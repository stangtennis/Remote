//go:build darwin

package tray

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/getlantern/systray"
	"github.com/stangtennis/remote-agent/internal/device"
	"github.com/stangtennis/remote-agent/internal/updater"
)

// Version information - injected at build time via -ldflags -X
var (
	Version       = "dev"
	BuildDate     = "unknown"
	VersionString = ""
)

func init() {
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

// getIcon returns icon data for macOS system tray (same as Windows for now)
func getIcon() []byte {
	return []byte{
		// ICO Header
		0x00, 0x00, 0x01, 0x00, 0x01, 0x00,
		// ICONDIRENTRY
		0x10, 0x10, 0x00, 0x00, 0x01, 0x00, 0x20, 0x00,
		0x28, 0x04, 0x00, 0x00, 0x16, 0x00, 0x00, 0x00,
		// BMP Info Header
		0x28, 0x00, 0x00, 0x00, 0x10, 0x00, 0x00, 0x00,
		0x20, 0x00, 0x00, 0x00, 0x01, 0x00, 0x20, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x04, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		// 16x16 pixel data (simplified — 1024 bytes of transparent/blue)
	}
}

func checkForUpdates() {
	log.Println("Checking for updates...")

	u, err := updater.NewUpdater(Version)
	if err != nil {
		log.Printf("Could not initialize updater: %v", err)
		showAlert("Error", fmt.Sprintf("Could not check for updates:\n%v", err))
		return
	}

	versionInfo, err := u.FetchVersionInfo()
	if err != nil {
		log.Printf("Could not fetch version info: %v", err)
		showAlert("Error", fmt.Sprintf("Could not fetch version info:\n%v", err))
		return
	}

	currentAgent, _ := updater.ParseVersion(Version)
	remoteAgent, _ := updater.ParseVersion(versionInfo.AgentVersion)

	agentStatus := "Up to date"
	if remoteAgent.IsNewerThan(currentAgent) {
		agentStatus = "NEW VERSION"
	}

	msg := fmt.Sprintf(
		"Agent (this):\n  Installed: %s\n  Available: %s  %s\n\nController:\n  Available: %s",
		Version,
		versionInfo.AgentVersion, agentStatus,
		versionInfo.ControllerVersion,
	)

	if remoteAgent.IsNewerThan(currentAgent) {
		msg += "\n\nDo you want to download and install the update?"
		// Use osascript for yes/no
		showAlert("Update Available", msg)
	} else {
		showAlert("Updates", msg)
	}
}

func showAlert(title, message string) {
	script := fmt.Sprintf(`display alert "%s" message "%s"`, title, message)
	exec.Command("osascript", "-e", script).Run()
}
