package device

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

const (
	registryPath = `SOFTWARE\RemoteDesktop`
	registryKey  = "DeviceID"
	configDir    = "RemoteDesktop"
	configFile   = "device.id"
)

// GetOrCreateDeviceID returns a persistent device ID
// First tries Windows Registry, then falls back to file storage
func GetOrCreateDeviceID() (string, error) {
	// Try to load from registry first (Windows)
	id, err := loadFromRegistry()
	if err == nil && id != "" {
		return id, nil
	}

	// Try to load from file
	id, err = loadFromFile()
	if err == nil && id != "" {
		return id, nil
	}

	// Generate new ID
	id, err = generateDeviceID()
	if err != nil {
		return "", fmt.Errorf("failed to generate device ID: %w", err)
	}

	// Save to both registry and file
	saveToRegistry(id)
	saveToFile(id)

	return id, nil
}

// generateDeviceID creates a unique device ID based on hardware info
func generateDeviceID() (string, error) {
	// Get hostname
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	// Get username
	username := os.Getenv("USERNAME")
	if username == "" {
		username = "unknown"
	}

	// Get computer name
	computerName := os.Getenv("COMPUTERNAME")
	if computerName == "" {
		computerName = hostname
	}

	// Combine info and hash
	data := fmt.Sprintf("%s-%s-%s", computerName, username, hostname)
	hash := sha256.Sum256([]byte(data))

	// Create device ID
	deviceID := fmt.Sprintf("device_%x", hash[:16])

	return deviceID, nil
}

// loadFromRegistry loads device ID from Windows Registry
func loadFromRegistry() (string, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, registryPath, registry.QUERY_VALUE)
	if err != nil {
		return "", err
	}
	defer key.Close()

	id, _, err := key.GetStringValue(registryKey)
	if err != nil {
		return "", err
	}

	return id, nil
}

// saveToRegistry saves device ID to Windows Registry
func saveToRegistry(id string) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, registryPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()

	return key.SetStringValue(registryKey, id)
}

// loadFromFile loads device ID from file
func loadFromFile() (string, error) {
	path, err := getConfigPath()
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// saveToFile saves device ID to file
func saveToFile(id string) error {
	path, err := getConfigPath()
	if err != nil {
		return err
	}

	// Create directory if not exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(path, []byte(id), 0644)
}

// getConfigPath returns the path to the config file
func getConfigPath() (string, error) {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return "", fmt.Errorf("APPDATA environment variable not set")
	}

	return filepath.Join(appData, configDir, configFile), nil
}

// GetDeviceName returns a friendly device name
func GetDeviceName() string {
	computerName := os.Getenv("COMPUTERNAME")
	if computerName == "" {
		hostname, err := os.Hostname()
		if err != nil {
			return "Unknown Device"
		}
		return hostname
	}
	return computerName
}

// GetPlatform returns the platform name
func GetPlatform() string {
	return "Windows"
}
