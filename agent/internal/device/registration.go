package device

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// RegistrationConfig holds Supabase configuration
type RegistrationConfig struct {
	SupabaseURL string
	AnonKey     string
	AccessToken string // User's access token after login
	UserID      string // User's ID after login
}

// DeviceInfo represents device information
type DeviceInfo struct {
	DeviceID   string `json:"device_id"`
	DeviceName string `json:"device_name"`
	Platform   string `json:"platform"`
	Status     string `json:"status"`
}

// RegisterDevice registers the device anonymously with Supabase
func RegisterDevice(config RegistrationConfig) (*DeviceInfo, error) {
	// Get or create device ID
	deviceID, err := GetOrCreateDeviceID()
	if err != nil {
		return nil, fmt.Errorf("failed to get device ID: %w", err)
	}

	// Create device info
	device := &DeviceInfo{
		DeviceID:   deviceID,
		DeviceName: GetDeviceName(),
		Platform:   GetPlatform(),
		Status:     "online",
	}

	// Register with Supabase (upsert)
	if err := upsertDevice(config, device); err != nil {
		return nil, fmt.Errorf("failed to register device: %w", err)
	}

	return device, nil
}

// upsertDevice registers device with user authentication
func upsertDevice(config RegistrationConfig, device *DeviceInfo) error {
	// Use direct REST API with user's access token for authenticated registration
	url := fmt.Sprintf("%s/rest/v1/remote_devices", config.SupabaseURL)

	// Create payload
	payload := map[string]interface{}{
		"device_id":   device.DeviceID,
		"device_name": device.DeviceName,
		"platform":    device.Platform,
		"arch":        "amd64",
		"owner_id":    config.UserID,
		"is_online":   true,
		"last_seen":   time.Now().Format(time.RFC3339),
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Try upsert
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("apikey", config.AnonKey)
	req.Header.Set("Authorization", "Bearer "+config.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Prefer", "resolution=merge-duplicates")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to register device: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	// Check for conflict (device exists) - try update instead
	if resp.StatusCode == http.StatusConflict || resp.StatusCode == http.StatusBadRequest {
		// Device exists, update it
		updateURL := fmt.Sprintf("%s/rest/v1/remote_devices?device_id=eq.%s", config.SupabaseURL, device.DeviceID)

		updatePayload := map[string]interface{}{
			"device_name": device.DeviceName,
			"owner_id":    config.UserID,
			"is_online":   true,
			"last_seen":   time.Now().Format(time.RFC3339),
		}

		updateData, _ := json.Marshal(updatePayload)
		updateReq, _ := http.NewRequest("PATCH", updateURL, bytes.NewBuffer(updateData))
		updateReq.Header.Set("apikey", config.AnonKey)
		updateReq.Header.Set("Authorization", "Bearer "+config.AccessToken)
		updateReq.Header.Set("Content-Type", "application/json")

		updateResp, err := client.Do(updateReq)
		if err != nil {
			return fmt.Errorf("failed to update device: %w", err)
		}
		defer updateResp.Body.Close()

		if updateResp.StatusCode == http.StatusOK || updateResp.StatusCode == http.StatusNoContent {
			fmt.Println("✅ Device updated successfully (already registered)")
			return nil
		}

		updateBody, _ := io.ReadAll(updateResp.Body)
		return fmt.Errorf("device update failed: %s (status: %d)", string(updateBody), updateResp.StatusCode)
	}

	if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK {
		fmt.Println("✅ Device registered successfully")
		return nil
	}

	return fmt.Errorf("registration failed: %s (status: %d)", string(body), resp.StatusCode)
}

// UpdateHeartbeat updates the device heartbeat
func UpdateHeartbeat(config RegistrationConfig, deviceID string) error {
	url := fmt.Sprintf("%s/rest/v1/remote_devices?device_id=eq.%s", config.SupabaseURL, deviceID)

	now := time.Now().Format(time.RFC3339)
	payload := map[string]interface{}{
		"is_online": true,
		"last_seen": now,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequest("PATCH", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("apikey", config.AnonKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("heartbeat failed: %s (status: %d)", string(body), resp.StatusCode)
	}

	return nil
}

// StartHeartbeat starts sending periodic heartbeats
func StartHeartbeat(config RegistrationConfig, deviceID string, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		if err := UpdateHeartbeat(config, deviceID); err != nil {
			fmt.Printf("⚠️  Heartbeat failed: %v\n", err)
		}
	}
}

// SetOffline marks the device as offline in the database
func SetOffline(config RegistrationConfig, deviceID string) error {
	url := fmt.Sprintf("%s/rest/v1/remote_devices?device_id=eq.%s", config.SupabaseURL, deviceID)

	payload := map[string]interface{}{
		"is_online": false,
		"last_seen": time.Now().Format(time.RFC3339),
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequest("PATCH", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("apikey", config.AnonKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	return nil
}
