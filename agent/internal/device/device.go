package device

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"time"

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
	// Retry registration with exponential backoff (for boot scenarios)
	maxRetries := 5
	baseDelay := 2 * time.Second
	
	for attempt := 1; attempt <= maxRetries; attempt++ {
		err := d.attemptRegister()
		if err == nil {
			return nil // Success
		}
		
		if attempt < maxRetries {
			delay := baseDelay * time.Duration(attempt)
			fmt.Printf("⚠️  Registration attempt %d failed: %v\n", attempt, err)
			fmt.Printf("   Retrying in %v...\n", delay)
			time.Sleep(delay)
		} else {
			return fmt.Errorf("registration failed after %d attempts: %w", maxRetries, err)
		}
	}
	
	return fmt.Errorf("registration failed")
}

func (d *Device) attemptRegister() error {
	// Prepare registration request
	reqBody := map[string]interface{}{
		"device_id":   d.ID,
		"device_name": d.Name,
		"platform":    d.Platform,
		"arch":        d.Arch,
		"cpu_count":   d.CPUCount,
		"ram_bytes":   d.RAMBytes,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Call device-register Edge Function
	url := d.cfg.SupabaseURL + "/functions/v1/device-register"
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("apikey", d.cfg.SupabaseAnonKey)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to register device: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	var regResp RegisterResponse
	if err := json.Unmarshal(body, &regResp); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	// Handle response
	switch regResp.Status {
	case "approved":
		d.APIKey = regResp.APIKey
		d.cfg.APIKey = regResp.APIKey
		fmt.Println("✅ Device approved and ready")
		return nil
	case "pending_approval":
		fmt.Println("⏳ Device registered. Waiting for owner approval...")
		fmt.Println("   Go to dashboard and approve this device")
		
		// Poll for approval
		return d.waitForApproval()
	default:
		return fmt.Errorf("unexpected status: %s", regResp.Status)
	}
}

func (d *Device) waitForApproval() error {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	timeout := time.After(10 * time.Minute)

	for {
		select {
		case <-timeout:
			return fmt.Errorf("approval timeout - please approve device in dashboard and restart agent")
		case <-ticker.C:
			// Check database directly for approval
			url := d.cfg.SupabaseURL + "/rest/v1/remote_devices"
			
			req, err := http.NewRequest("GET", url, nil)
			if err != nil {
				continue
			}

			req.Header.Set("apikey", d.cfg.SupabaseAnonKey)
			req.Header.Set("Authorization", "Bearer "+d.cfg.SupabaseAnonKey)
			
			q := req.URL.Query()
			q.Add("device_id", "eq."+d.ID)
			q.Add("select", "approved_at,api_key,is_online")
			req.URL.RawQuery = q.Encode()

			client := &http.Client{Timeout: 5 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				fmt.Printf("⚠️  Check failed: %v\n", err)
				continue
			}

			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			var devices []map[string]interface{}
			if json.Unmarshal(body, &devices) == nil && len(devices) > 0 {
				device := devices[0]
				if approvedAt, ok := device["approved_at"]; ok && approvedAt != nil {
					// Device is approved!
					if apiKey, ok := device["api_key"].(string); ok && apiKey != "" {
						d.APIKey = apiKey
						d.cfg.APIKey = apiKey
					}
					fmt.Println("\n✅ Device approved!")
					
					// Set device online
					d.setOnline()
					return nil
				}
			}
		}
	}
}

func (d *Device) setOnline() error {
	url := d.cfg.SupabaseURL + "/rest/v1/remote_devices"

	reqBody := map[string]interface{}{
		"is_online": true,
		"last_seen": time.Now().UTC().Format(time.RFC3339),
	}

	jsonData, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("PATCH", url, bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("apikey", d.cfg.SupabaseAnonKey)
	req.Header.Set("Authorization", "Bearer "+d.cfg.SupabaseAnonKey)
	req.Header.Set("Prefer", "return=minimal")
	
	q := req.URL.Query()
	q.Add("device_id", "eq."+d.ID)
	req.URL.RawQuery = q.Encode()

	client := &http.Client{Timeout: 5 * time.Second}
	resp, _ := client.Do(req)
	if resp != nil {
		resp.Body.Close()
	}
	
	return nil
}

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
