package device

import (
	"fmt"
	"log"
	"os"
	"runtime"

	"github.com/stangtennis/remote-agent/internal/auth"
	"github.com/stangtennis/remote-agent/internal/config"
)

// HealthChecker is a callback that returns true if the agent's polling is healthy.
type HealthChecker func() bool

// ConnInfoProvider returns current WebRTC connection info for heartbeat
type ConnInfoProvider func() (connType string, bytesSent, bytesReceived uint64)

type Device struct {
	ID            string
	Name          string
	Platform      string
	Arch          string
	CPUCount      int
	RAMBytes      int64
	APIKey        string // Stable per-device key — survives JWT expiry. Loaded from credentials at startup.
	cfg           *config.Config
	tokenProvider *auth.TokenProvider
	userID        string
	healthCheck   HealthChecker
	connInfoFunc  ConnInfoProvider

	// Force-update-handler injected af main.go. Hvis sat, kaldes denne
	// i stedet for den brudte --update-from-flow når dashboard sender
	// pending_command=force_update. Service-mode bruger rename-trick
	// som virker pålideligt på Windows.
	forceUpdateHandler func() bool

	// Heartbeat health telemetry (atomic for lock-free reads from any goroutine)
	consecutiveHeartbeatFailures int32 // updated by StartPresence
	lastHeartbeatSuccess         int64 // unix timestamp; updated by StartPresence
	lastHeartbeatErr             error // last error observed; only read inside StartPresence
}

// SetConnInfoProvider sets a callback to get WebRTC connection info for heartbeat
func (d *Device) SetConnInfoProvider(fn ConnInfoProvider) {
	d.connInfoFunc = fn
}

// SetHealthCheck sets a callback used by heartbeat to determine if polling is healthy.
// If the health check returns false, the heartbeat will report is_online=false.
func (d *Device) SetHealthCheck(fn HealthChecker) {
	d.healthCheck = fn
}

// SetForceUpdateHandler sets the function that gets called when dashboard
// sends pending_command=force_update. Returns true if update was applied
// and service is exiting for restart.
func (d *Device) SetForceUpdateHandler(fn func() bool) {
	d.forceUpdateHandler = fn
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

	// Load existing api_key from credentials (saved during prior registration)
	if creds, err := auth.LoadCredentials(); err == nil {
		dev.APIKey = creds.APIKey
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
	if deviceInfo.APIKey != "" {
		d.APIKey = deviceInfo.APIKey
		// Persist api_key alongside the JWT credentials so it survives restarts.
		creds.APIKey = deviceInfo.APIKey
		if err := auth.SaveCredentials(creds); err != nil {
			log.Printf("⚠️  Failed to persist api_key to credentials: %v", err)
		} else {
			log.Printf("🔑 api_key persisted to credentials")
		}
	}

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

	// Try to get a fresh JWT for the call but don't fail if we can't —
	// the api_key path keeps working even when refresh tokens are dead.
	token := ""
	if d.tokenProvider != nil {
		if t, err := d.tokenProvider.GetToken(); err == nil {
			token = t
		}
	}

	regCfg := RegistrationConfig{
		SupabaseURL: d.cfg.SupabaseURL,
		AnonKey:     d.cfg.SupabaseAnonKey,
		AccessToken: token,
		APIKey:      d.APIKey,
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
