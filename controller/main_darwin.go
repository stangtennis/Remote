//go:build darwin

package main

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/stangtennis/Remote/controller/internal/logger"
)

// ==================== macOS INSTALLATION CONSTANTS ====================

const (
	controllerInstallDir = "/Applications"
	controllerAppName    = "Remote Desktop Controller.app"
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
	targetApp := filepath.Join(controllerInstallDir, controllerAppName)
	_, err := os.Stat(targetApp)
	if err == nil {
		return true
	}
	// Also check raw binary (legacy)
	targetExe := filepath.Join(controllerInstallDir, controllerExeName)
	_, err = os.Stat(targetExe)
	return err == nil
}

// getInstalledExePath returns the full path to the installed exe inside the .app bundle
func getInstalledExePath() string {
	return filepath.Join(controllerInstallDir, controllerAppName, "Contents", "MacOS", controllerExeName)
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

// installControllerAsProgram copies the .app bundle or binary to /Applications
func installControllerAsProgram() error {
	if !isAdmin() {
		return fmt.Errorf("administrator rettigheder kraeves")
	}

	stopRunningController()

	currentExe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("kunne ikke finde exe: %w", err)
	}

	// Check if we're running from inside an .app bundle
	appBundlePath := findAppBundle(currentExe)
	if appBundlePath != "" {
		// Install .app bundle
		targetApp := filepath.Join(controllerInstallDir, controllerAppName)
		log.Printf("Installerer .app bundle: %s -> %s", appBundlePath, targetApp)

		// Remove existing
		os.RemoveAll(targetApp)

		// Copy entire .app bundle
		if err := copyDir(appBundlePath, targetApp); err != nil {
			return fmt.Errorf("kunne ikke kopiere .app bundle: %w", err)
		}

		exePath := filepath.Join(targetApp, "Contents", "MacOS", controllerExeName)
		if err := os.Chmod(exePath, 0755); err != nil {
			return fmt.Errorf("kunne ikke saette eksekverbar: %w", err)
		}

		if err := setControllerAutostart(exePath); err != nil {
			return fmt.Errorf("kunne ikke saette autostart: %w", err)
		}

		log.Printf("Starter controller fra: %s", exePath)
		startCmd := exec.Command("open", targetApp)
		if err := startCmd.Start(); err != nil {
			log.Printf("Kunne ikke starte controller: %v", err)
		}

		log.Printf("Controller .app installeret: %s", targetApp)
	} else {
		// Fallback: install raw binary
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
	}

	return nil
}

// findAppBundle returns the .app bundle path if the executable is inside one, or "" if not
func findAppBundle(exePath string) string {
	// Walk up from exe to find .app
	// e.g. /path/to/Foo.app/Contents/MacOS/Foo -> /path/to/Foo.app
	dir := exePath
	for i := 0; i < 5; i++ {
		dir = filepath.Dir(dir)
		if strings.HasSuffix(dir, ".app") {
			return dir
		}
		if dir == "/" || dir == "." {
			break
		}
	}
	return ""
}

// uninstallControllerProgram removes the controller installation and autostart
func uninstallControllerProgram() error {
	if !isAdmin() {
		return fmt.Errorf("administrator rettigheder kraeves")
	}

	stopRunningController()
	removeControllerAutostart()
	removeShortcuts()

	// Remove .app bundle
	targetApp := filepath.Join(controllerInstallDir, controllerAppName)
	os.RemoveAll(targetApp)

	// Remove legacy raw binary
	targetExe := filepath.Join(controllerInstallDir, controllerExeName)
	if err := os.Remove(targetExe); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("kunne ikke fjerne executable: %w", err)
	}

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

// copyDir recursively copies a directory
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(targetPath, info.Mode())
		}

		return copyFileSimple(path, targetPath)
	})
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

	// If inside .app bundle, use 'open' to launch properly
	appBundle := findAppBundle(executable)
	if appBundle != "" {
		cmd := exec.Command("open", appBundle)
		if err := cmd.Start(); err != nil {
			logger.Error("Failed to start new instance: %v", err)
			return
		}
	} else {
		cmd := exec.Command(executable)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err != nil {
			logger.Error("Failed to start new instance: %v", err)
			return
		}
	}

	logger.Info("New instance started, exiting current")
	os.Exit(0)
}

// runUpdateMode runs when started with --update-from flag
// On macOS, the downloaded file is a .tar.gz containing the .app bundle
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

	logMsg("Update mode started (macOS)")
	logMsg(fmt.Sprintf("Old exe/path: %s", oldExePath))

	currentExe, err := os.Executable()
	if err != nil {
		logMsg(fmt.Sprintf("ERROR: Failed to get current exe path: %v", err))
		return
	}
	logMsg(fmt.Sprintf("New exe: %s", currentExe))

	// Determine if we're updating an .app bundle or a raw binary
	oldAppBundle := findAppBundle(oldExePath)

	if oldAppBundle != "" {
		// .app bundle update: the downloaded file should be a .tar.gz
		// Check if current exe's parent has a .tar.gz nearby (downloaded update)
		updateDir := filepath.Dir(currentExe)
		tarFiles, _ := filepath.Glob(filepath.Join(updateDir, "*.tar.gz"))

		if len(tarFiles) > 0 {
			tarPath := tarFiles[0]
			logMsg(fmt.Sprintf("Extracting .tar.gz: %s", tarPath))

			// Extract to temp dir
			tempDir, err := os.MkdirTemp("", "controller-update-*")
			if err != nil {
				logMsg(fmt.Sprintf("ERROR: Failed to create temp dir: %v", err))
				return
			}
			defer os.RemoveAll(tempDir)

			if err := extractTarGz(tarPath, tempDir); err != nil {
				logMsg(fmt.Sprintf("ERROR: Failed to extract tar.gz: %v", err))
				return
			}

			// Find the .app inside extracted
			extractedApp := ""
			filepath.Walk(tempDir, func(path string, info os.FileInfo, err error) error {
				if err == nil && info.IsDir() && strings.HasSuffix(path, ".app") {
					extractedApp = path
					return filepath.SkipAll
				}
				return nil
			})

			if extractedApp == "" {
				logMsg("ERROR: No .app found in tar.gz")
				return
			}

			logMsg(fmt.Sprintf("Found .app: %s", extractedApp))

			// Wait for old app to close
			logMsg("Waiting for old app to exit...")
			time.Sleep(2 * time.Second)

			// Replace the old .app
			logMsg(fmt.Sprintf("Removing old .app: %s", oldAppBundle))
			if err := os.RemoveAll(oldAppBundle); err != nil {
				logMsg(fmt.Sprintf("WARNING: Failed to remove old .app: %v", err))
			}

			logMsg(fmt.Sprintf("Installing new .app to: %s", oldAppBundle))
			if err := copyDirForUpdate(extractedApp, oldAppBundle); err != nil {
				logMsg(fmt.Sprintf("ERROR: Failed to copy .app: %v", err))
				return
			}

			// Make binary executable
			newExe := filepath.Join(oldAppBundle, "Contents", "MacOS", controllerExeName)
			os.Chmod(newExe, 0755)

			// Launch the new app
			logMsg(fmt.Sprintf("Launching updated app: %s", oldAppBundle))
			cmd := exec.Command("open", oldAppBundle)
			if err := cmd.Start(); err != nil {
				logMsg(fmt.Sprintf("ERROR: Failed to launch: %v", err))
				return
			}

			logMsg("Update complete!")
			return
		}
	}

	// Fallback: raw binary update (legacy)
	logMsg("Fallback: raw binary update mode")

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

// extractTarGz extracts a .tar.gz archive to the destination directory
func extractTarGz(tarGzPath, destDir string) error {
	file, err := os.Open(tarGzPath)
	if err != nil {
		return fmt.Errorf("failed to open tar.gz: %w", err)
	}
	defer file.Close()

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar read error: %w", err)
		}

		// Security: prevent path traversal
		targetPath := filepath.Join(destDir, header.Name)
		if !strings.HasPrefix(targetPath, filepath.Clean(destDir)+string(os.PathSeparator)) && targetPath != filepath.Clean(destDir) {
			return fmt.Errorf("invalid tar path: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, os.FileMode(header.Mode)); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return err
			}
			outFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return err
			}
			outFile.Close()
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return err
			}
			os.Remove(targetPath)
			if err := os.Symlink(header.Linkname, targetPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// copyDirForUpdate recursively copies a directory (used during update)
func copyDirForUpdate(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(targetPath, info.Mode())
		}

		// Handle symlinks
		if info.Mode()&os.ModeSymlink != 0 {
			linkTarget, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(linkTarget, targetPath)
		}

		return copyFileSimple(path, targetPath)
	})
}
