//go:build windows

package sysinfo

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

func collectPlatform() (Info, error) {
	info := Info{}
	info.Hostname, _ = os.Hostname()
	info.OS = readOSCaption()
	info.CPU, info.CPUCores = readCPUInfo()
	info.RAMTotalGB, info.RAMFreeGB = readMemory()
	info.Disks = readDrives()
	info.UptimeSec = readUptime()
	info.InstalledApps = readInstalledApps()
	return info, nil
}

func readOSCaption() string {
	// Pull the friendly name + version from the registry (no WMI dependency)
	k, err := registry.OpenKey(registry.LOCAL_MACHINE,
		`SOFTWARE\Microsoft\Windows NT\CurrentVersion`, registry.QUERY_VALUE|registry.WOW64_64KEY)
	if err != nil {
		return "Windows"
	}
	defer k.Close()
	name, _, _ := k.GetStringValue("ProductName")
	displayVer, _, _ := k.GetStringValue("DisplayVersion")
	build, _, _ := k.GetStringValue("CurrentBuildNumber")
	parts := []string{name}
	if displayVer != "" {
		parts = append(parts, displayVer)
	}
	if build != "" {
		parts = append(parts, "(build "+build+")")
	}
	return strings.Join(parts, " ")
}

func readCPUInfo() (string, int) {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE,
		`HARDWARE\DESCRIPTION\System\CentralProcessor\0`, registry.QUERY_VALUE)
	if err != nil {
		return "", 0
	}
	defer k.Close()
	name, _, _ := k.GetStringValue("ProcessorNameString")

	// Count logical processors via NumberOfProcessors env var fallback
	cores := 0
	if v := os.Getenv("NUMBER_OF_PROCESSORS"); v != "" {
		fmt.Sscanf(v, "%d", &cores)
	}
	return strings.TrimSpace(name), cores
}

type memoryStatusEx struct {
	Length               uint32
	MemoryLoad           uint32
	TotalPhys            uint64
	AvailPhys            uint64
	TotalPageFile        uint64
	AvailPageFile        uint64
	TotalVirtual         uint64
	AvailVirtual         uint64
	AvailExtendedVirtual uint64
}

func readMemory() (float64, float64) {
	mod := windows.NewLazySystemDLL("kernel32.dll")
	proc := mod.NewProc("GlobalMemoryStatusEx")
	var ms memoryStatusEx
	ms.Length = uint32(unsafe.Sizeof(ms))
	ret, _, _ := proc.Call(uintptr(unsafe.Pointer(&ms)))
	if ret == 0 {
		return 0, 0
	}
	gb := func(b uint64) float64 { return float64(b) / 1024 / 1024 / 1024 }
	return roundGB(gb(ms.TotalPhys)), roundGB(gb(ms.AvailPhys))
}

func readDrives() []DiskInfo {
	mod := windows.NewLazySystemDLL("kernel32.dll")
	getDriveType := mod.NewProc("GetDriveTypeW")
	getDiskFreeSpace := mod.NewProc("GetDiskFreeSpaceExW")

	var disks []DiskInfo
	for c := 'A'; c <= 'Z'; c++ {
		root := string(c) + ":\\"
		rootPtr, _ := syscall.UTF16PtrFromString(root)
		dt, _, _ := getDriveType.Call(uintptr(unsafe.Pointer(rootPtr)))
		// DRIVE_FIXED == 3, DRIVE_REMOVABLE == 2; only report fixed local drives
		if dt != 3 {
			continue
		}
		var freeAvail, total, totalFree uint64
		ret, _, _ := getDiskFreeSpace.Call(
			uintptr(unsafe.Pointer(rootPtr)),
			uintptr(unsafe.Pointer(&freeAvail)),
			uintptr(unsafe.Pointer(&total)),
			uintptr(unsafe.Pointer(&totalFree)),
		)
		if ret == 0 || total == 0 {
			continue
		}
		gb := func(b uint64) float64 { return float64(b) / 1024 / 1024 / 1024 }
		disks = append(disks, DiskInfo{
			Mount:   string(c) + ":",
			TotalGB: roundGB(gb(total)),
			FreeGB:  roundGB(gb(totalFree)),
		})
	}
	return disks
}

func readUptime() int64 {
	mod := windows.NewLazySystemDLL("kernel32.dll")
	proc := mod.NewProc("GetTickCount64")
	ret, _, _ := proc.Call()
	ms := uint64(ret)
	return int64(ms / 1000)
}

// readInstalledApps walks both 32 and 64 bit Uninstall keys in HKLM.
func readInstalledApps() []AppInfo {
	roots := []struct {
		root registry.Key
		path string
		view uint32
	}{
		{registry.LOCAL_MACHINE, `Software\Microsoft\Windows\CurrentVersion\Uninstall`, registry.WOW64_64KEY},
		{registry.LOCAL_MACHINE, `Software\Microsoft\Windows\CurrentVersion\Uninstall`, registry.WOW64_32KEY},
	}
	seen := map[string]bool{}
	var apps []AppInfo
	for _, r := range roots {
		k, err := registry.OpenKey(r.root, r.path, registry.READ|r.view)
		if err != nil {
			continue
		}
		names, _ := k.ReadSubKeyNames(-1)
		k.Close()
		for _, n := range names {
			sub, err := registry.OpenKey(r.root, r.path+`\`+n, registry.QUERY_VALUE|r.view)
			if err != nil {
				continue
			}
			displayName, _, _ := sub.GetStringValue("DisplayName")
			if displayName == "" || seen[displayName] {
				sub.Close()
				continue
			}
			version, _, _ := sub.GetStringValue("DisplayVersion")
			publisher, _, _ := sub.GetStringValue("Publisher")
			sub.Close()
			seen[displayName] = true
			apps = append(apps, AppInfo{
				Name:      displayName,
				Version:   version,
				Publisher: publisher,
			})
		}
	}
	return apps
}

// powershellQuiet runs a PowerShell snippet and returns trimmed stdout. Kept as a
// fallback helper if registry-based queries miss something.
func powershellQuiet(script string, timeout time.Duration) string {
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	done := make(chan struct{})
	var out []byte
	var err error
	go func() {
		out, err = cmd.Output()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(timeout):
		_ = cmd.Process.Kill()
		return ""
	}
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func roundGB(v float64) float64 {
	// Round to 2 decimals
	return float64(int64(v*100+0.5)) / 100
}
