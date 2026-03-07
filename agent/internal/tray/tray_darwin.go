//go:build darwin

package tray

import (
	_ "embed"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/getlantern/systray"
	"github.com/stangtennis/remote-agent/internal/device"
	"github.com/stangtennis/remote-agent/internal/updater"
	"github.com/stangtennis/remote-agent/internal/version"
)

//go:embed agent.png
var agentIconPNG []byte

// Version aliases for backwards compatibility (ldflags still inject here)
var (
	Version       = "dev"
	BuildDate     = "unknown"
	VersionString = ""
)

func init() {
	// Sync: if ldflags injected here, propagate to version package
	if Version != "dev" {
		version.Version = Version
		version.BuildDate = BuildDate
	} else if version.Version != "dev" {
		Version = version.Version
		BuildDate = version.BuildDate
	}
	if VersionString == "" {
		VersionString = Version + " (built " + BuildDate + ")"
	}
}

type TrayApp struct {
	device *device.Device
	onExit func()
}

func New(dev *device.Device, onExit func()) *TrayApp {
	return &TrayApp{
		device: dev,
		onExit: onExit,
	}
}

func (t *TrayApp) Run() {
	systray.Run(t.onReady, t.onExit)
}

func (t *TrayApp) onReady() {
	systray.SetIcon(getIcon())
	systray.SetTitle("Remote")
	systray.SetTooltip(fmt.Sprintf("Remote Desktop Agent %s\nDevice: %s", Version, t.device.Name))

	mDeviceName := systray.AddMenuItem(fmt.Sprintf("Device: %s", t.device.Name), "")
	mDeviceName.Disable()

	mStatus := systray.AddMenuItem("Status: Online", "")
	mStatus.Disable()

	systray.AddSeparator()

	mLogs := systray.AddMenuItem("View Log File", "Open log file")
	mVersion := systray.AddMenuItem(fmt.Sprintf("Version %s", Version), "")
	mVersion.Disable()

	systray.AddSeparator()

	mUpdate := systray.AddMenuItem("Check for Updates", "")
	mLogout := systray.AddMenuItem("Switch Account / Log Out", "")
	mQuit := systray.AddMenuItem("Quit", "")

	go func() {
		for {
			select {
			case <-mLogs.ClickedCh:
				openLogFile()
			case <-mUpdate.ClickedCh:
				log.Println("Update check requested from system tray")
				checkForUpdates()
			case <-mLogout.ClickedCh:
				log.Println("Logout requested from system tray")
				clearCredentialsAndRestart()
			case <-mQuit.ClickedCh:
				log.Println("Exit requested from system tray")
				systray.Quit()
				return
			}
		}
	}()
}

func openLogFile() {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Printf("Could not get home dir: %v", err)
		return
	}
	logPath := filepath.Join(home, "Library", "Logs", "RemoteDesktopAgent", "agent.log")

	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		log.Printf("Log file not found: %s", logPath)
		return
	}

	cmd := exec.Command("open", "-a", "Console", logPath)
	if err := cmd.Start(); err != nil {
		// Fallback to TextEdit
		exec.Command("open", "-e", logPath).Start()
	}
}

func clearCredentialsAndRestart() {
	// Remove credentials
	home, _ := os.UserHomeDir()
	credPath := filepath.Join(home, "Library", "Application Support", "RemoteDesktopAgent", ".credentials")
	if err := os.Remove(credPath); err != nil && !os.IsNotExist(err) {
		log.Printf("Could not remove credentials: %v", err)
	} else {
		log.Println("Credentials cleared")
	}

	// Restart agent
	exePath, err := os.Executable()
	if err != nil {
		log.Printf("Could not find executable path: %v", err)
		return
	}

	log.Println("Restarting agent for new login...")
	cmd := exec.Command(exePath)
	cmd.Start()
	systray.Quit()
}

// getIcon returns the embedded PNG file for the macOS system tray
func getIcon() []byte {
	return agentIconPNG
}

func checkForUpdates() {
	log.Println("Checking for updates...")

	u, err := updater.NewUpdater(Version)
	if err != nil {
		log.Printf("Could not initialize updater: %v", err)
		showAlert("Error", fmt.Sprintf("Could not check for updates:\n%v", err))
		return
	}

	if err := u.CheckForUpdate(); err != nil {
		log.Printf("Update check failed: %v", err)
		showAlert("Error", fmt.Sprintf("Could not check for updates:\n%v", err))
		return
	}

	info := u.GetAvailableUpdate()
	if info == nil {
		showAlert("Up to date", fmt.Sprintf("Agent %s is the latest version.", Version))
		return
	}

	// Ask user to confirm
	if !showConfirmDialog("Update Available",
		fmt.Sprintf("A new version is available: %s\nCurrent: %s\n\nDownload and install now?", info.TagName, Version)) {
		return
	}

	log.Printf("User confirmed update to %s", info.TagName)

	if err := u.DownloadUpdate(); err != nil {
		log.Printf("Download failed: %v", err)
		showAlert("Download Failed", fmt.Sprintf("Could not download update:\n%v", err))
		return
	}

	if err := u.InstallUpdate(); err != nil {
		log.Printf("Install failed: %v", err)
		showAlert("Install Failed", fmt.Sprintf("Could not install update:\n%v", err))
		return
	}
}

func showAlert(title, message string) {
	script := fmt.Sprintf(`display alert "%s" message "%s"`, title, message)
	exec.Command("osascript", "-e", script).Run()
}

func showConfirmDialog(title, message string) bool {
	script := fmt.Sprintf(`display alert "%s" message "%s" buttons {"Cancel", "Update"} default button "Update"`, title, message)
	err := exec.Command("osascript", "-e", script).Run()
	return err == nil // Cancel returns exit code 1
}
