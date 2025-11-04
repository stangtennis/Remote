package device

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"runtime"

	"github.com/stangtennis/remote-agent/internal/config"
)

type Device struct {
	ID       string
	Name     string
	Platform string
	Arch     string
	CPUCount int
	RAMBytes int64
	APIKey   string
	cfg      *config.Config
}

type RegisterResponse struct {
	Status   string `json:"status"`
	DeviceID string `json:"device_id"`
	APIKey   string `json:"api_key"`
	Message  string `json:"message"`
}

func New(cfg *config.Config) (*Device, error) {
	// Try to load existing device ID from file
	deviceID, err := loadDeviceID()
	if err != nil || deviceID == "" {
		// Generate new device ID and save it
		deviceID = generateDeviceID()
		saveDeviceID(deviceID)
	}

	dev := &Device{
		ID:       deviceID,
		Platform: runtime.GOOS,
		Arch:     runtime.GOARCH,
		CPUCount: runtime.NumCPU(),
		cfg:      cfg,
	}

	// Get device name
	if cfg.DeviceName != "" {
		dev.Name = cfg.DeviceName
	} else {
		hostname, _ := os.Hostname()
		if hostname != "" {
			dev.Name = hostname
		} else {
			dev.Name = fmt.Sprintf("%s-%s", dev.Platform, dev.Arch)
		}
	}

	// Get RAM (approximate)
	dev.RAMBytes = getRAMBytes()

	return dev, nil
}

func (d *Device) Register() error {
	// Use new anonymous registration system
	config := RegistrationConfig{
		SupabaseURL: d.cfg.SupabaseURL,
		AnonKey:     d.cfg.SupabaseAnonKey,
	}

	// Register device anonymously
	deviceInfo, err := RegisterDevice(config)
	if err != nil {
		return fmt.Errorf("failed to register device: %w", err)
	}

	// Update device info
	d.ID = deviceInfo.DeviceID
	d.Name = deviceInfo.DeviceName

	fmt.Println("✅ Device registered successfully!")
	fmt.Printf("   Device ID: %s\n", d.ID)
	fmt.Printf("   Device Name: %s\n", d.Name)
	fmt.Println("   ⏳ Waiting for admin to assign this device...")
	fmt.Println("   💡 Admin can assign this device in the admin panel")

	return nil
}

// Old registration code removed - using new anonymous registration system

func (d *Device) SetOffline() error {
	// Update device status to offline
	// This will be handled by heartbeat timeout in presence.go
	fmt.Println("📴 Setting device offline...")
	return nil
}

func generateDeviceID() string {
	// Generate unique device ID based on hostname (without timestamp for stability)
	hostname, _ := os.Hostname()
	data := fmt.Sprintf("%s-%s-%s", hostname, runtime.GOOS, runtime.GOARCH)
	hash := sha256.Sum256([]byte(data))
	return "dev-" + hex.EncodeToString(hash[:8])
}

func loadDeviceID() (string, error) {
	// Load device ID from file
	data, err := os.ReadFile(".device_id")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func saveDeviceID(deviceID string) error {
	// Save device ID to file
	return os.WriteFile(".device_id", []byte(deviceID), 0600)
}

func getRAMBytes() int64 {
	// Placeholder - would need platform-specific code
	// For Windows, can use GlobalMemoryStatusEx
	return 8 * 1024 * 1024 * 1024 // Default 8GB
}
