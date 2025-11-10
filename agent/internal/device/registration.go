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

// upsertDevice inserts or updates device in Supabase
func upsertDevice(config RegistrationConfig, device *DeviceInfo) error {
	// Create payload with all required fields
	payload := map[string]interface{}{
		"device_id":   device.DeviceID,
		"device_name": device.DeviceName,
		"platform":    device.Platform,
		"status":      device.Status,
		"last_seen":   time.Now().Format(time.RFC3339),
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Try to update existing device first
	updateURL := fmt.Sprintf("%s/rest/v1/remote_devices?device_id=eq.%s", config.SupabaseURL, device.DeviceID)
	req, err := http.NewRequest("PATCH", updateURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create update request: %w", err)
	}

	req.Header.Set("apikey", config.AnonKey)
	req.Header.Set("Authorization", "Bearer "+config.AnonKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Prefer", "return=representation")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send update request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	// If update succeeded (device exists), we're done
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent {
		fmt.Println("✅ Device updated successfully (already registered)")
		return nil
	}

	// If device doesn't exist, try to insert it
	insertURL := fmt.Sprintf("%s/rest/v1/remote_devices", config.SupabaseURL)
	req, err = http.NewRequest("POST", insertURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create insert request: %w", err)
	}

	req.Header.Set("apikey", config.AnonKey)
	req.Header.Set("Authorization", "Bearer "+config.AnonKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err = client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send insert request: %w", err)
	}
	defer resp.Body.Close()

	body, _ = io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		// If we get a duplicate key error, that's actually OK - device exists
		if resp.StatusCode == http.StatusConflict || resp.StatusCode == 409 {
			fmt.Println("✅ Device already registered (conflict resolved)")
			return nil
		}
		return fmt.Errorf("registration failed: %s (status: %d)", string(body), resp.StatusCode)
	}

	return nil
}

// UpdateHeartbeat updates the device heartbeat
func UpdateHeartbeat(config RegistrationConfig, deviceID string) error {
	url := fmt.Sprintf("%s/rest/v1/remote_devices?device_id=eq.%s", config.SupabaseURL, deviceID)

	now := time.Now().Format(time.RFC3339)
	payload := map[string]interface{}{
		"status":    "online",
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
