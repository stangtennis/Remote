//go:build darwin

package service

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"
)

const serviceName = "dk.hawkeye.remote-agent"
const serviceDisplayName = "Remote Desktop Agent"
const serviceDescription = "Remote desktop access agent"

const launchdPlistTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>{{.Label}}</string>
    <key>ProgramArguments</key>
    <array>
        <string>{{.ExePath}}</string>
        <string>--console</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>{{.LogDir}}/remote-agent.log</string>
    <key>StandardErrorPath</key>
    <string>{{.LogDir}}/remote-agent-error.log</string>
    <key>WorkingDirectory</key>
    <string>{{.WorkDir}}</string>
</dict>
</plist>`

type Service struct {
	onStart func() error
	onStop  func() error
}

func NewService(onStart, onStop func() error) *Service {
	return &Service{
		onStart: onStart,
		onStop:  onStop,
	}
}

// RunService checks if running as launchd daemon.
// On macOS, launchd daemons are just normal processes launched by launchd.
func RunService() error {
	// macOS services are just processes — no special service API needed
	return nil
}

func getPlistPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "/tmp/" + serviceName + ".plist"
	}
	return filepath.Join(home, "Library", "LaunchAgents", serviceName+".plist")
}

func getLogDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "/tmp"
	}
	logDir := filepath.Join(home, "Library", "Logs", "RemoteDesktopAgent")
	os.MkdirAll(logDir, 0755)
	return logDir
}

func InstallService(exePath string) error {
	plistPath := getPlistPath()

	// Ensure LaunchAgents directory exists
	dir := filepath.Dir(plistPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create LaunchAgents dir: %w", err)
	}

	// Generate plist from template
	tmpl, err := template.New("plist").Parse(launchdPlistTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse plist template: %w", err)
	}

	f, err := os.Create(plistPath)
	if err != nil {
		return fmt.Errorf("failed to create plist file: %w", err)
	}
	defer f.Close()

	data := struct {
		Label   string
		ExePath string
		LogDir  string
		WorkDir string
	}{
		Label:   serviceName,
		ExePath: exePath,
		LogDir:  getLogDir(),
		WorkDir: filepath.Dir(exePath),
	}

	if err := tmpl.Execute(f, data); err != nil {
		return fmt.Errorf("failed to write plist: %w", err)
	}

	log.Printf("Launchd plist installed: %s", plistPath)
	return nil
}

func UninstallService() error {
	// Unload first
	StopService()

	plistPath := getPlistPath()
	if err := os.Remove(plistPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove plist: %w", err)
	}

	log.Println("Launchd plist removed")
	return nil
}

func StartService() error {
	plistPath := getPlistPath()
	cmd := exec.Command("launchctl", "load", plistPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("launchctl load failed: %w", err)
	}
	log.Println("Launchd service started")
	return nil
}

func StopService() error {
	plistPath := getPlistPath()
	cmd := exec.Command("launchctl", "unload", plistPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("launchctl unload failed: %w", err)
	}
	log.Println("Launchd service stopped")
	return nil
}
