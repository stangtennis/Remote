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

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

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

// stopRunningController kills any running controller.exe processes (except this one)
func stopRunningController() {
	myPID := os.Getpid()
	log.Printf("Stopper koerenede controller-processer (vores PID: %d)...", myPID)

	// Use tasklist to find all controller.exe PIDs, then kill each except ours
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
		// CSV format: "controller.exe","1234","Console","1","12,345 K"
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

	// Stop any running controller first
	stopRunningController()

	// Create install directory
	if err := os.MkdirAll(controllerInstallDir, 0755); err != nil {
		return fmt.Errorf("kunne ikke oprette mappe: %w", err)
	}

	// Get current exe path
	currentExe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("kunne ikke finde exe: %w", err)
	}

	targetExe := filepath.Join(controllerInstallDir, controllerExeName)

	// Copy exe to Program Files
	if err := copyFileSimple(currentExe, targetExe); err != nil {
		return fmt.Errorf("kunne ikke kopiere: %w", err)
	}

	// Add autostart registry entry
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

	// Stop any running controller first
	stopRunningController()

	// Remove autostart registry entry
	removeControllerAutostart()

	// Remove Start Menu and Desktop shortcuts
	removeShortcuts()

	// Remove install directory
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
	cmd.Run() // Ignore errors
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
	// Use PowerShell COM object to create .lnk file
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
	// Use ProgramData Start Menu (all users) if admin, otherwise per-user
	startMenuDir := filepath.Join(os.Getenv("ProgramData"), "Microsoft", "Windows", "Start Menu", "Programs")
	if _, err := os.Stat(startMenuDir); os.IsNotExist(err) {
		// Fallback to per-user
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
	// Start Menu (all users)
	startMenuAll := filepath.Join(os.Getenv("ProgramData"), "Microsoft", "Windows", "Start Menu", "Programs", "Remote Desktop Controller.lnk")
	if err := os.Remove(startMenuAll); err == nil {
		log.Printf("Start Menu genvej fjernet (alle brugere)")
	}
	// Start Menu (per-user)
	startMenuUser := filepath.Join(os.Getenv("APPDATA"), "Microsoft", "Windows", "Start Menu", "Programs", "Remote Desktop Controller.lnk")
	if err := os.Remove(startMenuUser); err == nil {
		log.Printf("Start Menu genvej fjernet (bruger)")
	}
	// Desktop
	desktopShortcut := filepath.Join(os.Getenv("USERPROFILE"), "Desktop", "Remote Desktop Controller.lnk")
	if err := os.Remove(desktopShortcut); err == nil {
		log.Printf("Desktop genvej fjernet")
	}
}

// showInstallDialog shows the install/uninstall dialog for the controller
func showInstallDialog(window fyne.Window) {
	installed := isInstalledAsProgram()

	if installed {
		// Show uninstall option
		dialog.ShowConfirm("Afinstaller Controller",
			"Controller er installeret i:\n"+controllerInstallDir+"\n\n"+
				"Dette vil:\n"+
				"  Stoppe koerende controller\n"+
				"  Fjerne autostart og genveje\n"+
				"  Slette installationen\n\n"+
				"Fortsaet?",
			func(ok bool) {
				if !ok {
					return
				}
				if !isAdmin() {
					dialog.ShowConfirm("Administrator kraeves",
						"Afinstallation kraever Administrator rettigheder.\n\nGenstart som Administrator?",
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
			widget.NewLabel("  Kopiere controller til Program Files"),
			widget.NewLabel("  Oprette Start Menu genvej"),
			widget.NewLabel("  Saette autostart ved Windows login"),
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
						"Installation kraever Administrator rettigheder.\n\nGenstart som Administrator?",
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
						// Always create Start Menu shortcut
						if smErr := createStartMenuShortcut(targetExe); smErr != nil {
							log.Printf("Start Menu genvej fejlede: %v", smErr)
						}
						// Optionally create Desktop shortcut
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
									"Start Menu genvej oprettet\n"+
									"Starter automatisk ved Windows login\n"+
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

// openBrowser opens a URL in the default browser (Windows implementation)
func openBrowser(url string) {
	logger.Info("Opening browser: %s", url)
	cmd := exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	if err := cmd.Start(); err != nil {
		logger.Error("Failed to open browser: %v", err)
	}
}

// restartApplication restarts the application (Windows implementation)
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

		// Use cmd /c start to launch detached process that survives parent exit
		cmd := exec.Command("cmd", "/c", "start", "", executable)

		logger.Info("Starting new instance with command: %v", cmd.Args)

		// Run synchronously to ensure process starts before we exit
		if err := cmd.Run(); err != nil {
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
