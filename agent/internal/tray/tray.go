package tray

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/getlantern/systray"
	"github.com/stangtennis/remote-agent/internal/device"
)

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
	// Set up the system tray icon and menu
	// Note: Icon is optional - systray will use a default if not set
	// systray.SetIcon(getIcon()) // Commented out - icon format needs proper implementation
	systray.SetTitle("Remote")
	systray.SetTooltip(fmt.Sprintf("Remote Desktop Agent\nDevice: %s", t.device.Name))

	// Add menu items
	mDeviceName := systray.AddMenuItem(fmt.Sprintf("Device: %s", t.device.Name), "")
	mDeviceName.Disable()
	
	mStatus := systray.AddMenuItem("Status: Online", "Current connection status")
	mStatus.Disable()
	
	systray.AddSeparator()
	
	mLogs := systray.AddMenuItem("View Logs", "Open log file")
	mVersion := systray.AddMenuItem("Version 1.0.0", "Agent version")
	mVersion.Disable()
	
	systray.AddSeparator()
	
	mQuit := systray.AddMenuItem("Exit", "Quit the agent")

	// Handle menu clicks
	go func() {
		for {
			select {
			case <-mLogs.ClickedCh:
				openLogs()
			case <-mQuit.ClickedCh:
				log.Println("🛑 Exit requested from system tray")
				systray.Quit()
				return
			}
		}
	}()
}

func openLogs() {
	// Get the executable directory
	exePath, err := os.Executable()
	if err != nil {
		log.Printf("Failed to get executable path: %v", err)
		return
	}
	exeDir := filepath.Dir(exePath)
	logPath := filepath.Join(exeDir, "agent.log")

	// Open log file with default text editor
	log.Printf("Opening log file: %s", logPath)
	
	// Use cmd /c start to open with default editor
	cmd := exec.Command("cmd", "/c", "start", "", logPath)
	if err := cmd.Start(); err != nil {
		log.Printf("Failed to open log file: %v", err)
	}
}

// TODO: Add proper icon support
// getIcon() should return valid ICO file bytes for the system tray
// For now, we rely on the default Windows icon
