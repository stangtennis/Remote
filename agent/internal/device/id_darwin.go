//go:build darwin

package device

import (
	"crypto/sha256"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	configDir  = "RemoteDesktop"
	configFile = "device.id"
)

// GetOrCreateDeviceID returns a persistent device ID.
// On macOS, uses IOPlatformSerialNumber (hardware serial) as base.
func GetOrCreateDeviceID() (string, error) {
	// Try to load from Application Support
	id, err := loadFromAppSupport()
	if err == nil && id != "" {
		log.Printf("Loaded device ID from Application Support: %s", id[:20]+"...")
		return id, nil
	}

	// Generate new ID based on macOS hardware UUID
	id, err = generateDeviceID()
	if err != nil {
		return "", fmt.Errorf("failed to generate device ID: %w", err)
	}

	log.Printf("Generated new device ID: %s", id[:20]+"...")

	// Save for persistence
	if err := saveToAppSupport(id); err != nil {
		log.Printf("Could not save device ID: %v", err)
	}

	return id, nil
}

// generateDeviceID creates a unique device ID based on macOS hardware UUID
func generateDeviceID() (string, error) {
	// Get macOS hardware UUID via ioreg
	hwUUID, err := getMacHardwareUUID()
	if err == nil && hwUUID != "" {
		hash := sha256.Sum256([]byte(hwUUID))
		return fmt.Sprintf("device_%x", hash[:16]), nil
	}

	log.Printf("Could not get hardware UUID: %v, using fallback", err)

	// Fallback: use hostname
	hostname, _ := os.Hostname()
	data := fmt.Sprintf("machine-%s", hostname)
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("device_%x", hash[:16]), nil
}

// getMacHardwareUUID gets the macOS hardware UUID
func getMacHardwareUUID() (string, error) {
	cmd := exec.Command("ioreg", "-rd1", "-c", "IOPlatformExpertDevice")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	for _, line := range strings.Split(string(output), "\n") {
		if strings.Contains(line, "IOPlatformUUID") {
			parts := strings.Split(line, `"`)
			if len(parts) >= 4 {
				return parts[3], nil
			}
		}
	}

	return "", fmt.Errorf("IOPlatformUUID not found in ioreg output")
}

func loadFromAppSupport() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(home, "Library", "Application Support", configDir, configFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func saveToAppSupport(id string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dir := filepath.Join(home, "Library", "Application Support", configDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	path := filepath.Join(dir, configFile)
	return os.WriteFile(path, []byte(id), 0644)
}

// GetDeviceName returns a friendly device name
func GetDeviceName() string {
	// Try scutil for the ComputerName (user-friendly name)
	cmd := exec.Command("scutil", "--get", "ComputerName")
	output, err := cmd.Output()
	if err == nil {
		name := strings.TrimSpace(string(output))
		if name != "" {
			return name
		}
	}

	hostname, err := os.Hostname()
	if err != nil {
		return "Unknown Mac"
	}
	return hostname
}

// GetPlatform returns the platform name
func GetPlatform() string {
	return "macOS"
}
