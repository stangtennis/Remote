//go:build windows

package device

import (
	"crypto/sha256"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

const (
	// Use LOCAL_MACHINE for machine-wide ID (same for all users and services)
	registryPathLM = `SOFTWARE\RemoteDesktop`
	// Fallback to CURRENT_USER if no admin rights
	registryPathCU = `SOFTWARE\RemoteDesktop`
	registryKey    = "DeviceID"
	configDir      = "RemoteDesktop"
	configFile     = "device.id"
)

// GetOrCreateDeviceID returns a persistent device ID
// Uses Windows MachineGUID as base for consistent ID across all users/services
func GetOrCreateDeviceID() (string, error) {
	// Try to load existing ID from LOCAL_MACHINE registry (machine-wide)
	id, err := loadFromRegistryLM()
	if err == nil && id != "" {
		log.Printf("📋 Loaded device ID from LOCAL_MACHINE registry: %s", id[:20]+"...")
		return id, nil
	}

	// Try CURRENT_USER registry (fallback for non-admin)
	id, err = loadFromRegistryCU()
	if err == nil && id != "" {
		log.Printf("📋 Loaded device ID from CURRENT_USER registry: %s", id[:20]+"...")
		return id, nil
	}

	// Try to load from ProgramData file (machine-wide)
	id, err = loadFromProgramData()
	if err == nil && id != "" {
		log.Printf("📋 Loaded device ID from ProgramData: %s", id[:20]+"...")
		return id, nil
	}

	// Try to load from AppData file (user-specific fallback)
	id, err = loadFromAppData()
	if err == nil && id != "" {
		log.Printf("📋 Loaded device ID from AppData: %s", id[:20]+"...")
		return id, nil
	}

	// Generate new ID based on Windows MachineGUID (hardware-based, never changes)
	id, err = generateDeviceID()
	if err != nil {
		return "", fmt.Errorf("failed to generate device ID: %w", err)
	}

	log.Printf("🆕 Generated new device ID: %s", id[:20]+"...")

	// Save to all locations for persistence
	if err := saveToRegistryLM(id); err != nil {
		log.Printf("⚠️  Could not save to LOCAL_MACHINE registry (need admin): %v", err)
		// Try CURRENT_USER as fallback
		if err := saveToRegistryCU(id); err != nil {
			log.Printf("⚠️  Could not save to CURRENT_USER registry: %v", err)
		}
	}

	// Also save to file as backup
	if err := saveToProgramData(id); err != nil {
		log.Printf("⚠️  Could not save to ProgramData: %v", err)
		// Try AppData as fallback
		saveToAppData(id)
	}

	return id, nil
}

// generateDeviceID creates a unique device ID based on Windows MachineGUID
// MachineGUID is set during Windows installation and never changes
func generateDeviceID() (string, error) {
	// Try to get Windows MachineGUID (most reliable, never changes)
	machineGUID, err := getWindowsMachineGUID()
	if err == nil && machineGUID != "" {
		hash := sha256.Sum256([]byte(machineGUID))
		return fmt.Sprintf("device_%x", hash[:16]), nil
	}

	log.Printf("⚠️  Could not get MachineGUID: %v, using fallback", err)

	// Fallback: use computer name + hostname (less reliable but still works)
	hostname, _ := os.Hostname()
	computerName := os.Getenv("COMPUTERNAME")
	if computerName == "" {
		computerName = hostname
	}

	// DO NOT include USERNAME - it changes between user and SYSTEM service
	data := fmt.Sprintf("machine-%s-%s", computerName, hostname)
	hash := sha256.Sum256([]byte(data))

	return fmt.Sprintf("device_%x", hash[:16]), nil
}

// getWindowsMachineGUID gets the Windows MachineGUID from registry
// This GUID is created during Windows installation and never changes
func getWindowsMachineGUID() (string, error) {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, 
		`SOFTWARE\Microsoft\Cryptography`, registry.QUERY_VALUE)
	if err != nil {
		return "", err
	}
	defer key.Close()

	guid, _, err := key.GetStringValue("MachineGuid")
	if err != nil {
		return "", err
	}

	return guid, nil
}

// loadFromRegistryLM loads device ID from LOCAL_MACHINE registry (machine-wide)
func loadFromRegistryLM() (string, error) {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, registryPathLM, registry.QUERY_VALUE)
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

// loadFromRegistryCU loads device ID from CURRENT_USER registry (user-specific fallback)
func loadFromRegistryCU() (string, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, registryPathCU, registry.QUERY_VALUE)
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

// saveToRegistryLM saves device ID to LOCAL_MACHINE registry (requires admin)
func saveToRegistryLM(id string) error {
	key, _, err := registry.CreateKey(registry.LOCAL_MACHINE, registryPathLM, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()

	return key.SetStringValue(registryKey, id)
}

// saveToRegistryCU saves device ID to CURRENT_USER registry (fallback)
func saveToRegistryCU(id string) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, registryPathCU, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()

	return key.SetStringValue(registryKey, id)
}

// loadFromProgramData loads device ID from ProgramData (machine-wide file)
func loadFromProgramData() (string, error) {
	programData := os.Getenv("ProgramData")
	if programData == "" {
		programData = `C:\ProgramData`
	}
	path := filepath.Join(programData, configDir, configFile)

	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// saveToProgramData saves device ID to ProgramData (machine-wide file)
func saveToProgramData(id string) error {
	programData := os.Getenv("ProgramData")
	if programData == "" {
		programData = `C:\ProgramData`
	}
	path := filepath.Join(programData, configDir, configFile)

	// Create directory if not exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(path, []byte(id), 0644)
}

// loadFromAppData loads device ID from AppData (user-specific fallback)
func loadFromAppData() (string, error) {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return "", fmt.Errorf("APPDATA environment variable not set")
	}
	path := filepath.Join(appData, configDir, configFile)

	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// saveToAppData saves device ID to AppData (user-specific fallback)
func saveToAppData(id string) error {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return fmt.Errorf("APPDATA environment variable not set")
	}
	path := filepath.Join(appData, configDir, configFile)

	// Create directory if not exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(path, []byte(id), 0644)
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
