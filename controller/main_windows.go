//go:build windows

package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/stangtennis/Remote/controller/internal/logger"
)

// ==================== WINDOWS INSTALLATION CONSTANTS ====================

const (
	controllerInstallDir = `C:\Program Files\RemoteDesktopController`
	controllerExeName    = "controller.exe"
	controllerRegRunKey  = `SOFTWARE\Microsoft\Windows\CurrentVersion\Run`
	controllerRegValue   = "RemoteDesktopController"
)

// isAdmin checks if the current process has administrator privileges
func isAdmin() bool {
	_, err := os.Open("\\\\.\\PHYSICALDRIVE0")
	return err == nil
}

// runAsAdmin restarts the current process with elevated privileges
func runAsAdmin() {
	exe, _ := os.Executable()
	verb := "runas"
	cwd, _ := os.Getwd()

	verbPtr, _ := syscall.UTF16PtrFromString(verb)
	exePtr, _ := syscall.UTF16PtrFromString(exe)
	cwdPtr, _ := syscall.UTF16PtrFromString(cwd)
	argPtr, _ := syscall.UTF16PtrFromString(strings.Join(os.Args[1:], " "))

	shell32 := syscall.NewLazyDLL("shell32.dll")
	shellExecute := shell32.NewProc("ShellExecuteW")
	shellExecute.Call(0, uintptr(unsafe.Pointer(verbPtr)), uintptr(unsafe.Pointer(exePtr)),
		uintptr(unsafe.Pointer(argPtr)), uintptr(unsafe.Pointer(cwdPtr)), 1)
}

// isInstalledAsProgram checks if the controller is installed in Program Files
func isInstalledAsProgram() bool {
	targetExe := filepath.Join(controllerInstallDir, controllerExeName)
	_, err := os.Stat(targetExe)
	return err == nil
}

// getInstalledExePath returns the full path to the installed exe
func getInstalledExePath() string {
	return filepath.Join(controllerInstallDir, controllerExeName)
}

// stopRunningController kills any running controller.exe processes (except this one)
func stopRunningController() {
	myPID := os.Getpid()
	log.Printf("Stopper koererende controller-processer (vores PID: %d)...", myPID)

	cmd := exec.Command("tasklist", "/FI", fmt.Sprintf("IMAGENAME eq %s", controllerExeName), "/FO", "CSV", "/NH")
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("   tasklist fejl: %v", err)
		return
	}

	killed := 0
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.Contains(line, "No tasks") || strings.Contains(line, "INFO:") {
			continue
		}
		parts := strings.Split(line, ",")
		if len(parts) < 2 {
			continue
		}
		pidStr := strings.Trim(parts[1], "\" ")
		pid := 0
		fmt.Sscanf(pidStr, "%d", &pid)
		if pid == 0 || pid == myPID {
			continue
		}
		log.Printf("   Draeber PID %d...", pid)
		killCmd := exec.Command("taskkill", "/F", "/PID", fmt.Sprintf("%d", pid))
		killCmd.Run()
		killed++
	}

	if killed > 0 {
		log.Printf("   %d controller-proces(ser) stoppet", killed)
		time.Sleep(500 * time.Millisecond)
	} else {
		log.Printf("   Ingen andre controller-processer fundet")
	}
}

// installControllerAsProgram copies the exe to Program Files and sets up autostart
func installControllerAsProgram() error {
	if !isAdmin() {
		return fmt.Errorf("administrator rettigheder kraeves")
	}

	stopRunningController()

	if err := os.MkdirAll(controllerInstallDir, 0755); err != nil {
		return fmt.Errorf("kunne ikke oprette mappe: %w", err)
	}

	currentExe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("kunne ikke finde exe: %w", err)
	}

	targetExe := filepath.Join(controllerInstallDir, controllerExeName)

	if err := copyFileSimple(currentExe, targetExe); err != nil {
		return fmt.Errorf("kunne ikke kopiere: %w", err)
	}

	if err := setControllerAutostart(targetExe); err != nil {
		return fmt.Errorf("kunne ikke saette autostart: %w", err)
	}

	// Start the controller from Program Files
	log.Printf("Starter controller fra: %s", targetExe)
	startCmd := exec.Command(targetExe)
	startCmd.Dir = controllerInstallDir
	if err := startCmd.Start(); err != nil {
		log.Printf("Kunne ikke starte controller: %v", err)
	}

	log.Printf("Controller installeret: %s", targetExe)
	return nil
}

// uninstallControllerProgram removes the controller installation and autostart
func uninstallControllerProgram() error {
	if !isAdmin() {
		return fmt.Errorf("administrator rettigheder kraeves")
	}

	stopRunningController()
	removeControllerAutostart()
	removeShortcuts()

	if err := os.RemoveAll(controllerInstallDir); err != nil {
		return fmt.Errorf("kunne ikke fjerne mappe: %w", err)
	}

	log.Println("Controller afinstalleret")
	return nil
}

// setControllerAutostart adds the controller to Windows autostart via registry
func setControllerAutostart(exePath string) error {
	cmd := exec.Command("reg", "add",
		`HKCU\`+controllerRegRunKey,
		"/v", controllerRegValue,
		"/t", "REG_SZ",
		"/d", `"`+exePath+`"`,
		"/f")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("reg add failed: %w", err)
	}
	return nil
}

// removeControllerAutostart removes the controller from Windows autostart
func removeControllerAutostart() {
	cmd := exec.Command("reg", "delete",
		`HKCU\`+controllerRegRunKey,
		"/v", controllerRegValue,
		"/f")
	cmd.Run()
}

// copyFileSimple copies a file from src to dst
func copyFileSimple(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// createShortcut creates a Windows .lnk shortcut file using PowerShell
func createShortcut(shortcutPath, targetExe, description string) error {
	psScript := fmt.Sprintf(`
$ws = New-Object -ComObject WScript.Shell
$s = $ws.CreateShortcut('%s')
$s.TargetPath = '%s'
$s.WorkingDirectory = '%s'
$s.Description = '%s'
$s.Save()
`, shortcutPath, targetExe, filepath.Dir(targetExe), description)

	cmd := exec.Command("powershell", "-NoProfile", "-Command", psScript)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("powershell shortcut failed: %w (%s)", err, strings.TrimSpace(string(output)))
	}
	return nil
}

// createStartMenuShortcut creates a Start Menu shortcut for the controller
func createStartMenuShortcut(targetExe string) error {
	startMenuDir := filepath.Join(os.Getenv("ProgramData"), "Microsoft", "Windows", "Start Menu", "Programs")
	if _, err := os.Stat(startMenuDir); os.IsNotExist(err) {
		startMenuDir = filepath.Join(os.Getenv("APPDATA"), "Microsoft", "Windows", "Start Menu", "Programs")
	}

	shortcutPath := filepath.Join(startMenuDir, "Remote Desktop Controller.lnk")
	log.Printf("Opretter Start Menu genvej: %s", shortcutPath)
	return createShortcut(shortcutPath, targetExe, "Remote Desktop Controller")
}

// createDesktopShortcut creates a Desktop shortcut for the controller
func createDesktopShortcut(targetExe string) error {
	desktopDir := filepath.Join(os.Getenv("USERPROFILE"), "Desktop")
	shortcutPath := filepath.Join(desktopDir, "Remote Desktop Controller.lnk")
	log.Printf("Opretter Desktop genvej: %s", shortcutPath)
	return createShortcut(shortcutPath, targetExe, "Remote Desktop Controller")
}

// removeShortcuts removes Start Menu and Desktop shortcuts
func removeShortcuts() {
	startMenuAll := filepath.Join(os.Getenv("ProgramData"), "Microsoft", "Windows", "Start Menu", "Programs", "Remote Desktop Controller.lnk")
	if err := os.Remove(startMenuAll); err == nil {
		log.Printf("Start Menu genvej fjernet (alle brugere)")
	}
	startMenuUser := filepath.Join(os.Getenv("APPDATA"), "Microsoft", "Windows", "Start Menu", "Programs", "Remote Desktop Controller.lnk")
	if err := os.Remove(startMenuUser); err == nil {
		log.Printf("Start Menu genvej fjernet (bruger)")
	}
	desktopShortcut := filepath.Join(os.Getenv("USERPROFILE"), "Desktop", "Remote Desktop Controller.lnk")
	if err := os.Remove(desktopShortcut); err == nil {
		log.Printf("Desktop genvej fjernet")
	}
}

// openBrowser opens a URL in the default browser
func openBrowser(url string) {
	logger.Info("Opening browser: %s", url)
	cmd := exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	if err := cmd.Start(); err != nil {
		logger.Error("Failed to open browser: %v", err)
	}
}

// restartApplication restarts the application
func restartApplication() {
	logger.Info("Restarting application...")

	executable, err := os.Executable()
	if err != nil {
		logger.Error("Failed to get executable path: %v", err)
		return
	}

	// Use cmd /c start to launch detached process
	cmd := exec.Command("cmd", "/c", "start", "", executable)
	if err := cmd.Run(); err != nil {
		logger.Error("Failed to start new instance: %v", err)
		return
	}

	logger.Info("New instance started, exiting current")
	os.Exit(0)
}

// runUpdateMode runs when started with --update-from flag
func runUpdateMode(oldExePath string) {
	logFile := oldExePath + ".update.log"
	f, _ := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if f != nil {
		defer f.Close()
	}

	logMsg := func(msg string) {
		line := fmt.Sprintf("[%s] %s\n", time.Now().Format("15:04:05"), msg)
		if f != nil {
			f.WriteString(line)
		}
	}

	logMsg("Update mode started")
	logMsg(fmt.Sprintf("Old exe: %s", oldExePath))

	currentExe, err := os.Executable()
	if err != nil {
		logMsg(fmt.Sprintf("ERROR: Failed to get current exe path: %v", err))
		return
	}
	logMsg(fmt.Sprintf("New exe: %s", currentExe))

	// Wait for old exe to exit
	logMsg("Waiting for old exe to exit...")
	for i := 0; i < 100; i++ {
		file, err := os.OpenFile(oldExePath, os.O_RDWR, 0)
		if err == nil {
			file.Close()
			logMsg("Old exe is unlocked")
			break
		}
		if os.IsNotExist(err) {
			logMsg("Old exe already deleted")
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Delete old exe
	logMsg("Deleting old exe...")
	if err := os.Remove(oldExePath); err != nil {
		if !os.IsNotExist(err) {
			logMsg(fmt.Sprintf("WARNING: Failed to delete old exe: %v", err))
		}
	} else {
		logMsg("Old exe deleted")
	}

	// Copy current exe to old location
	logMsg(fmt.Sprintf("Moving %s to %s", currentExe, oldExePath))
	srcFile, err := os.Open(currentExe)
	if err != nil {
		logMsg(fmt.Sprintf("ERROR: Failed to open source: %v", err))
		return
	}
	dstFile, err := os.Create(oldExePath)
	if err != nil {
		srcFile.Close()
		logMsg(fmt.Sprintf("ERROR: Failed to create destination: %v", err))
		return
	}
	_, err = dstFile.ReadFrom(srcFile)
	srcFile.Close()
	dstFile.Close()
	if err != nil {
		logMsg(fmt.Sprintf("ERROR: Failed to copy: %v", err))
		return
	}
	logMsg("Copy complete")

	// Start from original location
	logMsg("Starting controller from original location...")
	cmd := exec.Command(oldExePath)
	if err := cmd.Start(); err != nil {
		logMsg(fmt.Sprintf("ERROR: Failed to start: %v", err))
		return
	}

	logMsg("Update complete!")
}
