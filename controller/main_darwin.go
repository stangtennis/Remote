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

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/stangtennis/Remote/controller/internal/logger"
)

// ==================== macOS INSTALLATION CONSTANTS ====================

const (
	controllerInstallDir = "/Applications"
	controllerExeName    = "RemoteDesktopController"
	controllerRegRunKey  = "" // Not used on macOS
	controllerRegValue   = "" // Not used on macOS
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
	// Also check for .app bundle
	targetApp := filepath.Join(controllerInstallDir, controllerExeName+".app")
	_, err = os.Stat(targetApp)
	return err == nil
}

// stopRunningController kills any running controller processes (except this one)
func stopRunningController() {
	myPID := os.Getpid()
	log.Printf("Stopper koerende controller-processer (vores PID: %d)...", myPID)

	// Use pkill to kill all instances except ours
	// First try killall which is more reliable on macOS
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

	// Stop any running controller first
	stopRunningController()

	// Get current exe path
	currentExe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("kunne ikke finde exe: %w", err)
	}

	targetExe := filepath.Join(controllerInstallDir, controllerExeName)

	// Copy exe to /Applications
	if err := copyFileSimple(currentExe, targetExe); err != nil {
		return fmt.Errorf("kunne ikke kopiere: %w", err)
	}

	// Make executable
	if err := os.Chmod(targetExe, 0755); err != nil {
		return fmt.Errorf("kunne ikke saette eksekverbar: %w", err)
	}

	// Add LaunchAgent for autostart
	if err := setControllerAutostart(targetExe); err != nil {
		return fmt.Errorf("kunne ikke saette autostart: %w", err)
	}

	// Start the controller from /Applications
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

	// Stop any running controller first
	stopRunningController()

	// Remove autostart LaunchAgent
	removeControllerAutostart()

	// Remove shortcuts (symlinks)
	removeShortcuts()

	// Remove executable from /Applications
	targetExe := filepath.Join(controllerInstallDir, controllerExeName)
	if err := os.Remove(targetExe); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("kunne ikke fjerne executable: %w", err)
	}

	// Also try removing .app bundle if it exists
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

	// Ensure LaunchAgents directory exists
	if err := os.MkdirAll(filepath.Dir(plistPath), 0755); err != nil {
		return fmt.Errorf("kunne ikke oprette LaunchAgents mappe: %w", err)
	}

	if err := os.WriteFile(plistPath, []byte(plistContent), 0644); err != nil {
		return fmt.Errorf("kunne ikke skrive plist: %w", err)
	}

	// Load the LaunchAgent
	cmd := exec.Command("launchctl", "load", plistPath)
	cmd.Run() // Ignore errors - may already be loaded

	return nil
}

// removeControllerAutostart removes the LaunchAgent plist and unloads it
func removeControllerAutostart() {
	plistPath := launchAgentPath()

	// Unload the LaunchAgent
	cmd := exec.Command("launchctl", "unload", plistPath)
	cmd.Run() // Ignore errors

	// Remove the plist file
	os.Remove(plistPath) // Ignore errors
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

// createShortcut is a no-op on macOS (shortcuts are not a concept)
func createShortcut(shortcutPath, targetExe, description string) error {
	// macOS does not use .lnk shortcuts; create a symlink instead
	return os.Symlink(targetExe, shortcutPath)
}

// createStartMenuShortcut is a no-op on macOS (no Start Menu)
func createStartMenuShortcut(targetExe string) error {
	// macOS has no Start Menu concept
	return nil
}

// createDesktopShortcut creates a symlink on the Desktop
func createDesktopShortcut(targetExe string) error {
	homeDir, _ := os.UserHomeDir()
	desktopDir := filepath.Join(homeDir, "Desktop")
	symlinkPath := filepath.Join(desktopDir, controllerExeName)
	log.Printf("Opretter Desktop symlink: %s", symlinkPath)

	// Remove existing symlink if present
	os.Remove(symlinkPath)

	return os.Symlink(targetExe, symlinkPath)
}

// removeShortcuts removes Desktop symlinks
func removeShortcuts() {
	homeDir, _ := os.UserHomeDir()

	// Desktop symlink
	desktopLink := filepath.Join(homeDir, "Desktop", controllerExeName)
	if err := os.Remove(desktopLink); err == nil {
		log.Printf("Desktop symlink fjernet")
	}
}

// showInstallDialog shows the install/uninstall dialog for the controller
func showInstallDialog(window fyne.Window) {
	installed := isInstalledAsProgram()

	if installed {
		// Show uninstall option
		dialog.ShowConfirm("Afinstaller Controller",
			"Controller er installeret i:\n"+controllerInstallDir+"/"+controllerExeName+"\n\n"+
				"Dette vil:\n"+
				"  Stoppe koerende controller\n"+
				"  Fjerne autostart (LaunchAgent)\n"+
				"  Slette installationen\n\n"+
				"Fortsaet?",
			func(ok bool) {
				if !ok {
					return
				}
				if !isAdmin() {
					dialog.ShowConfirm("Administrator kraeves",
						"Afinstallation kraever administrator rettigheder.\n\nGenstart som administrator?",
						func(ok bool) {
							if ok {
								runAsAdmin()
								myApp.Quit()
							}
						}, window)
					return
				}
				go func() {
					err := uninstallControllerProgram()
					fyne.Do(func() {
						if err != nil {
							dialog.ShowError(fmt.Errorf("Kunne ikke afinstallere: %v", err), window)
						} else {
							dialog.ShowInformation("Afinstallation faerdig",
								"Controller afinstalleret.\n\nAutostart og genveje er fjernet.", window)
						}
					})
				}()
			}, window)
	} else {
		// Show install option with desktop shortcut checkbox
		desktopCheck := widget.NewCheck("Opret genvej paa skrivebordet", nil)
		desktopCheck.SetChecked(true)

		content := container.NewVBox(
			widget.NewLabelWithStyle("Installer Controller", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
			widget.NewSeparator(),
			widget.NewLabel("Dette vil:"),
			widget.NewLabel("  Kopiere controller til /Applications"),
			widget.NewLabel("  Oprette LaunchAgent til autostart"),
			widget.NewLabel("  Stoppe evt. koerende gammel version"),
			widget.NewSeparator(),
			desktopCheck,
			widget.NewSeparator(),
			widget.NewLabel("Installationsmappe:"),
			widget.NewLabel(controllerInstallDir),
		)

		d := dialog.NewCustomConfirm("Installer Controller", "Installer", "Annuller", content,
			func(ok bool) {
				if !ok {
					return
				}
				createDesktop := desktopCheck.Checked
				if !isAdmin() {
					dialog.ShowConfirm("Administrator kraeves",
						"Installation kraever administrator rettigheder.\n\nGenstart som administrator?",
						func(ok bool) {
							if ok {
								runAsAdmin()
								myApp.Quit()
							}
						}, window)
					return
				}
				go func() {
					err := installControllerAsProgram()
					if err == nil {
						targetExe := filepath.Join(controllerInstallDir, controllerExeName)
						// Optionally create Desktop shortcut (symlink)
						if createDesktop {
							if dErr := createDesktopShortcut(targetExe); dErr != nil {
								log.Printf("Desktop genvej fejlede: %v", dErr)
							}
						}
					}
					fyne.Do(func() {
						if err != nil {
							dialog.ShowError(fmt.Errorf("Kunne ikke installere: %v", err), window)
						} else {
							d := dialog.NewInformation("Installation faerdig",
								"Controller installeret og startet!\n\n"+
									"Starter automatisk ved login (LaunchAgent)\n"+
									"Installeret i: "+controllerInstallDir+"\n\n"+
									"Dette vindue lukkes nu.", window)
							d.SetOnClosed(func() {
								myApp.Quit()
							})
							d.Show()
						}
					})
				}()
			}, window)
		d.Resize(fyne.NewSize(400, 350))
		d.Show()
	}
}

// openBrowser opens a URL in the default browser (macOS implementation)
func openBrowser(url string) {
	logger.Info("Opening browser: %s", url)
	cmd := exec.Command("open", url)
	if err := cmd.Start(); err != nil {
		logger.Error("Failed to open browser: %v", err)
	}
}

// restartApplication restarts the application (macOS implementation)
func restartApplication() {
	logger.Info("Restarting application...")

	// Show progress dialog
	progressDialog := dialog.NewCustom("Restarting", "Cancel",
		container.NewVBox(
			widget.NewLabel("Restarting application..."),
			widget.NewProgressBarInfinite(),
		), myWindow)
	progressDialog.Show()

	// Get the current executable path
	executable, err := os.Executable()
	if err != nil {
		logger.Error("Failed to get executable path: %v", err)
		progressDialog.Hide()
		dialog.ShowError(fmt.Errorf("Failed to restart: %v", err), myWindow)
		return
	}

	logger.Info("Executable path: %s", executable)

	// Start a new instance in background
	go func() {
		// Small delay to ensure UI updates
		time.Sleep(500 * time.Millisecond)

		cmd := exec.Command(executable)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		logger.Info("Starting new instance with command: %v", cmd.Args)

		if err := cmd.Start(); err != nil {
			logger.Error("Failed to start new instance: %v", err)
			progressDialog.Hide()
			dialog.ShowError(fmt.Errorf("Failed to restart: %v", err), myWindow)
			return
		}

		logger.Info("New instance started successfully, shutting down current instance")

		// Exit current instance immediately
		myApp.Quit()
	}()
}
