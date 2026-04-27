//go:build darwin

package sysinfo

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

func collectPlatform() (Info, error) {
	info := Info{}
	info.Hostname, _ = os.Hostname()
	info.OS = readMacOS()
	info.CPU, info.CPUCores = readCPU()
	info.RAMTotalGB, info.RAMFreeGB = readMemory()
	info.Disks = readDisks()
	info.UptimeSec = readUptime()
	info.InstalledApps = readApps()
	return info, nil
}

func readMacOS() string {
	out, err := exec.Command("sw_vers").Output()
	if err != nil {
		return "macOS"
	}
	productName, productVersion, buildVersion := "macOS", "", ""
	for _, line := range strings.Split(string(out), "\n") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		switch key {
		case "ProductName":
			productName = val
		case "ProductVersion":
			productVersion = val
		case "BuildVersion":
			buildVersion = val
		}
	}
	parts := []string{productName}
	if productVersion != "" {
		parts = append(parts, productVersion)
	}
	if buildVersion != "" {
		parts = append(parts, "("+buildVersion+")")
	}
	return strings.Join(parts, " ")
}

func readCPU() (string, int) {
	name := sysctlString("machdep.cpu.brand_string")
	if name == "" {
		name = sysctlString("hw.model")
	}
	cores := sysctlInt("hw.logicalcpu")
	return name, cores
}

func readMemory() (float64, float64) {
	totalBytes := sysctlUint64("hw.memsize")
	pageSize := sysctlUint64("hw.pagesize")
	if pageSize == 0 {
		pageSize = 4096
	}

	// Available memory via vm_stat (free + inactive pages)
	out, err := exec.Command("vm_stat").Output()
	freePages := uint64(0)
	if err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			if strings.HasPrefix(line, "Pages free:") || strings.HasPrefix(line, "Pages inactive:") {
				digits := strings.Trim(strings.TrimSpace(strings.SplitN(line, ":", 2)[1]), ".")
				v, _ := strconv.ParseUint(digits, 10, 64)
				freePages += v
			}
		}
	}
	gb := func(b uint64) float64 { return float64(b) / 1024 / 1024 / 1024 }
	return roundGB(gb(totalBytes)), roundGB(gb(freePages * pageSize))
}

func readDisks() []DiskInfo {
	var disks []DiskInfo
	mounts := []string{"/"}
	if entries, err := os.ReadDir("/Volumes"); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				mounts = append(mounts, "/Volumes/"+e.Name())
			}
		}
	}
	seen := map[string]bool{}
	for _, m := range mounts {
		var stat syscall.Statfs_t
		if err := syscall.Statfs(m, &stat); err != nil {
			continue
		}
		key := fmt.Sprintf("%d-%d", stat.Fsid.Val[0], stat.Fsid.Val[1])
		if seen[key] {
			continue
		}
		seen[key] = true
		bs := uint64(stat.Bsize)
		total := stat.Blocks * bs
		free := stat.Bfree * bs
		gb := func(b uint64) float64 { return float64(b) / 1024 / 1024 / 1024 }
		disks = append(disks, DiskInfo{
			Mount:   m,
			TotalGB: roundGB(gb(total)),
			FreeGB:  roundGB(gb(free)),
		})
	}
	return disks
}

func readUptime() int64 {
	out, err := exec.Command("sysctl", "-n", "kern.boottime").Output()
	if err != nil {
		return 0
	}
	// Format: { sec = 1700000000, usec = 0 } Wed Nov 15 ...
	s := string(out)
	idx := strings.Index(s, "sec = ")
	if idx < 0 {
		return 0
	}
	tail := s[idx+len("sec = "):]
	end := strings.IndexAny(tail, ", ")
	if end < 0 {
		return 0
	}
	bootSec, err := strconv.ParseInt(tail[:end], 10, 64)
	if err != nil {
		return 0
	}
	now := exec.Command("date", "+%s")
	nowOut, err := now.Output()
	if err != nil {
		return 0
	}
	nowSec, _ := strconv.ParseInt(strings.TrimSpace(string(nowOut)), 10, 64)
	if nowSec == 0 {
		return 0
	}
	return nowSec - bootSec
}

func readApps() []AppInfo {
	// /Applications/*.app — simple, no system_profiler dependency
	var apps []AppInfo
	entries, err := os.ReadDir("/Applications")
	if err != nil {
		return nil
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".app") {
			continue
		}
		apps = append(apps, AppInfo{Name: strings.TrimSuffix(name, ".app")})
	}
	return apps
}

func sysctlString(key string) string {
	out, err := exec.Command("sysctl", "-n", key).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func sysctlInt(key string) int {
	v, _ := strconv.Atoi(sysctlString(key))
	return v
}

func sysctlUint64(key string) uint64 {
	v, _ := strconv.ParseUint(sysctlString(key), 10, 64)
	return v
}

func roundGB(v float64) float64 {
	return float64(int64(v*100+0.5)) / 100
}

// Silence unused import
var _ = bytes.NewReader
