package device

import (
	"fmt"
	"os"
	"runtime"

	"github.com/stangtennis/remote-agent/internal/auth"
	"github.com/stangtennis/remote-agent/internal/config"
)

// HealthChecker is a callback that returns true if the agent's polling is healthy.
type HealthChecker func() bool

type Device struct {
	ID            string
	Name          string
	Platform      string
	Arch          string
	CPUCount      int
	RAMBytes      int64
	APIKey        string
	cfg           *config.Config
	tokenProvider *auth.TokenProvider
	userID        string
	healthCheck   HealthChecker
}

// SetHealthCheck sets a callback used by heartbeat to determine if polling is healthy.
// If the health check returns false, the heartbeat will report is_online=false.
func (d *Device) SetHealthCheck(fn HealthChecker) {
	d.healthCheck = fn
}

type RegisterResponse struct {
	Status   string `json:"status"`
	DeviceID string `json:"device_id"`
	APIKey   string `json:"api_key"`
	Message  string `json:"message"`
}

func New(cfg *config.Config, tokenProvider *auth.TokenProvider) (*Device, error) {
	// Get or create persistent device ID
	deviceID, err := GetOrCreateDeviceID()
	if err != nil {
		return nil, fmt.Errorf("failed to get device ID: %w", err)
	}

	dev := &Device{
		ID:            deviceID,
		Platform:      runtime.GOOS,
		Arch:          runtime.GOARCH,
		CPUCount:      runtime.NumCPU(),
		cfg:           cfg,
		tokenProvider: tokenProvider,
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
	// Get fresh token from provider
	token, err := d.tokenProvider.GetToken()
	if err != nil {
		return fmt.Errorf("failed to get auth token: %w", err)
	}

	creds, err := auth.GetCurrentUser()
	if err != nil {
		return fmt.Errorf("not logged in: %w", err)
	}

	// Use authenticated registration
	regConfig := RegistrationConfig{
		SupabaseURL: d.cfg.SupabaseURL,
		AnonKey:     d.cfg.SupabaseAnonKey,
		AccessToken: token,
		UserID:      creds.UserID,
	}

	// Register device with user authentication
	deviceInfo, err := RegisterDevice(regConfig)
	if err != nil {
		return fmt.Errorf("failed to register device: %w", err)
	}

	// Update device info
	d.ID = deviceInfo.DeviceID
	d.Name = deviceInfo.DeviceName
	d.userID = creds.UserID

	fmt.Println("✅ Device registered successfully!")
	fmt.Printf("   Device ID: %s\n", d.ID)
	fmt.Printf("   Device Name: %s\n", d.Name)
	fmt.Printf("   Owner: %s\n", creds.Email)

	return nil
}

// Old registration code removed - using new anonymous registration system

func (d *Device) SetOffline() error {
	// Update device status to offline in database
	fmt.Println("📴 Setting device offline...")

	token, err := d.tokenProvider.GetToken()
	if err != nil {
		fmt.Printf("⚠️  Failed to get auth token for offline: %v\n", err)
		// Fall back to unauthenticated call as last resort
		token = ""
	}

	regCfg := RegistrationConfig{
		SupabaseURL: d.cfg.SupabaseURL,
		AnonKey:     d.cfg.SupabaseAnonKey,
		AccessToken: token,
	}

	if err := SetOffline(regCfg, d.ID); err != nil {
		fmt.Printf("⚠️  Failed to set offline status: %v\n", err)
		return err
	}

	fmt.Println("✅ Device marked as offline")
	return nil
}

func getRAMBytes() int64 {
	// Placeholder - would need platform-specific code
	// For Windows, can use GlobalMemoryStatusEx
	return 8 * 1024 * 1024 * 1024 // Default 8GB
}
