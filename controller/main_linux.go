//go:build linux

package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/stangtennis/Remote/controller/internal/logger"
)

// Linux stubs for platform-specific functions
// Controller GUI is primarily for Windows/macOS, but these stubs allow building on Linux

func isAdmin() bool {
	return os.Getuid() == 0
}

func runAsAdmin() {
	exe, _ := os.Executable()
	cmd := exec.Command("pkexec", exe)
	cmd.Start()
}

func isInstalledAsProgram() bool {
	return false
}

func getInstalledExePath() string {
	return "/usr/local/bin/remote-desktop-controller"
}

func installControllerAsProgram() error {
	return fmt.Errorf("installation not supported on Linux yet")
}

func uninstallControllerProgram() error {
	return fmt.Errorf("uninstallation not supported on Linux yet")
}

func createStartMenuShortcut(targetExe string) error {
	return nil
}

func createDesktopShortcut(targetExe string) error {
	return nil
}

func openBrowser(url string) {
	logger.Info("Opening browser: %s", url)
	cmd := exec.Command("xdg-open", url)
	cmd.Start()
}

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
	os.Exit(0)
}

func runUpdateMode(oldExePath string) {
	fmt.Println("Update mode not supported on Linux")
}
