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
	systray.SetIcon(getIcon())
	systray.SetTitle("Remote Agent")
	systray.SetTooltip(fmt.Sprintf("Remote Desktop Agent\n%s", t.device.Name))

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

// getIcon returns a simple ICO file bytes for the system tray
// This is a minimal 16x16 icon (Windows .ico format)
func getIcon() []byte {
	// Simple blue square icon (16x16, 32-bit RGBA)
	// ICO file format: ICONDIR header + ICONDIRENTRY + BMP data
	return []byte{
		// ICO Header (6 bytes)
		0x00, 0x00, // Reserved (must be 0)
		0x01, 0x00, // Type (1 = ICO)
		0x01, 0x00, // Number of images
		
		// ICONDIRENTRY (16 bytes)
		0x10,       // Width (16 pixels)
		0x10,       // Height (16 pixels)
		0x00,       // Color palette (0 = no palette)
		0x00,       // Reserved
		0x01, 0x00, // Color planes
		0x20, 0x00, // Bits per pixel (32)
		0x00, 0x04, 0x00, 0x00, // Size of image data (1024 bytes)
		0x16, 0x00, 0x00, 0x00, // Offset to image data
		
		// BMP data would go here (simplified for now)
		// For production, use an actual icon file or embed one
	}
}
