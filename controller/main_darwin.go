//go:build darwin

package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/stangtennis/Remote/controller/internal/logger"
)

// ==================== macOS INSTALLATION CONSTANTS ====================

const (
	controllerInstallDir = "/Applications"
	controllerExeName    = "RemoteDesktopController"
)

const launchAgentLabel = "dk.hawkeye.remote-controller"

// isAdmin checks if the current process has root privileges
func isAdmin() bool {
	return os.Getuid() == 0
}

// runAsAdmin restarts the current process with elevated privileges using osascript
func runAsAdmin() {
	exe, _ := os.Executable()
	script := fmt.Sprintf(`do shell script "%s" with administrator privileges`, exe)
	cmd := exec.Command("osascript", "-e", script)
	cmd.Start()
}

// isInstalledAsProgram checks if the controller is installed in /Applications
func isInstalledAsProgram() bool {
	targetExe := filepath.Join(controllerInstallDir, controllerExeName)
	_, err := os.Stat(targetExe)
	if err == nil {
		return true
	}
	targetApp := filepath.Join(controllerInstallDir, controllerExeName+".app")
	_, err = os.Stat(targetApp)
	return err == nil
}

// getInstalledExePath returns the full path to the installed exe
func getInstalledExePath() string {
	return filepath.Join(controllerInstallDir, controllerExeName)
}

// stopRunningController kills any running controller processes (except this one)
func stopRunningController() {
	myPID := os.Getpid()
	log.Printf("Stopper koerende controller-processer (vores PID: %d)...", myPID)

	cmd := exec.Command("bash", "-c", fmt.Sprintf(
		"pgrep -f '%s' | grep -v '^%d$' | xargs -r kill -9 2>/dev/null || true",
		controllerExeName, myPID))
	cmd.Run()

	time.Sleep(500 * time.Millisecond)
	log.Printf("Controller-processer stoppet")
}

// installControllerAsProgram copies the executable to /Applications and sets up autostart
func installControllerAsProgram() error {
	if !isAdmin() {
		return fmt.Errorf("administrator rettigheder kraeves")
	}

	stopRunningController()

	currentExe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("kunne ikke finde exe: %w", err)
	}

	targetExe := filepath.Join(controllerInstallDir, controllerExeName)

	if err := copyFileSimple(currentExe, targetExe); err != nil {
		return fmt.Errorf("kunne ikke kopiere: %w", err)
	}

	if err := os.Chmod(targetExe, 0755); err != nil {
		return fmt.Errorf("kunne ikke saette eksekverbar: %w", err)
	}

	if err := setControllerAutostart(targetExe); err != nil {
		return fmt.Errorf("kunne ikke saette autostart: %w", err)
	}

	log.Printf("Starter controller fra: %s", targetExe)
	startCmd := exec.Command(targetExe)
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

	targetExe := filepath.Join(controllerInstallDir, controllerExeName)
	if err := os.Remove(targetExe); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("kunne ikke fjerne executable: %w", err)
	}
	targetApp := filepath.Join(controllerInstallDir, controllerExeName+".app")
	os.RemoveAll(targetApp)

	log.Println("Controller afinstalleret")
	return nil
}

// launchAgentPath returns the path to the LaunchAgent plist file
func launchAgentPath() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, "Library", "LaunchAgents", launchAgentLabel+".plist")
}

// setControllerAutostart creates a LaunchAgent plist for autostart
func setControllerAutostart(exePath string) error {
	plistContent := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>%s</string>
    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <false/>
</dict>
</plist>
`, launchAgentLabel, exePath)

	plistPath := launchAgentPath()

	if err := os.MkdirAll(filepath.Dir(plistPath), 0755); err != nil {
		return fmt.Errorf("kunne ikke oprette LaunchAgents mappe: %w", err)
	}

	if err := os.WriteFile(plistPath, []byte(plistContent), 0644); err != nil {
		return fmt.Errorf("kunne ikke skrive plist: %w", err)
	}

	cmd := exec.Command("launchctl", "load", plistPath)
	cmd.Run()

	return nil
}

// removeControllerAutostart removes the LaunchAgent plist and unloads it
func removeControllerAutostart() {
	plistPath := launchAgentPath()
	cmd := exec.Command("launchctl", "unload", plistPath)
	cmd.Run()
	os.Remove(plistPath)
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

// createStartMenuShortcut is a no-op on macOS
func createStartMenuShortcut(targetExe string) error {
	return nil
}

// createDesktopShortcut creates a symlink on the Desktop
func createDesktopShortcut(targetExe string) error {
	homeDir, _ := os.UserHomeDir()
	desktopDir := filepath.Join(homeDir, "Desktop")
	symlinkPath := filepath.Join(desktopDir, controllerExeName)
	log.Printf("Opretter Desktop symlink: %s", symlinkPath)
	os.Remove(symlinkPath)
	return os.Symlink(targetExe, symlinkPath)
}

// removeShortcuts removes Desktop symlinks
func removeShortcuts() {
	homeDir, _ := os.UserHomeDir()
	desktopLink := filepath.Join(homeDir, "Desktop", controllerExeName)
	if err := os.Remove(desktopLink); err == nil {
		log.Printf("Desktop symlink fjernet")
	}
}

// openBrowser opens a URL in the default browser
func openBrowser(url string) {
	logger.Info("Opening browser: %s", url)
	cmd := exec.Command("open", url)
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

	cmd := exec.Command(executable)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
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

	logMsg("Deleting old exe...")
	if err := os.Remove(oldExePath); err != nil {
		if !os.IsNotExist(err) {
			logMsg(fmt.Sprintf("WARNING: Failed to delete old exe: %v", err))
		}
	} else {
		logMsg("Old exe deleted")
	}

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

	if err := os.Chmod(oldExePath, 0755); err != nil {
		logMsg(fmt.Sprintf("WARNING: chmod failed: %v", err))
	}

	logMsg("Starting controller from original location...")
	cmd := exec.Command(oldExePath)
	if err := cmd.Start(); err != nil {
		logMsg(fmt.Sprintf("ERROR: Failed to start: %v", err))
		return
	}

	logMsg("Update complete!")
}
