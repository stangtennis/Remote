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

// upsertDevice registers device using Edge Function (better security)
func upsertDevice(config RegistrationConfig, device *DeviceInfo) error {
	// Use Edge Function instead of direct DB access
	edgeFunctionURL := fmt.Sprintf("%s/functions/v1/device-register", config.SupabaseURL)
	
	// Create payload for Edge Function
	payload := map[string]interface{}{
		"device_id":   device.DeviceID,
		"device_name": device.DeviceName,
		"platform":    device.Platform,
		"arch":        "amd64", // TODO: Get from runtime
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequest("POST", edgeFunctionURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("apikey", config.AnonKey)
	req.Header.Set("Authorization", "Bearer "+config.AnonKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to call Edge Function: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	// Edge Function returns 201 for new registration, 200 for approved, 202 for pending
	if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusAccepted {
		var result map[string]interface{}
		if err := json.Unmarshal(body, &result); err == nil {
			if status, ok := result["status"].(string); ok {
				if status == "pending_approval" {
					fmt.Println("✅ Device registered successfully (awaiting approval)")
				} else if status == "approved" {
					fmt.Println("✅ Device registered and approved")
				}
			}
		}
		return nil
	}

	return fmt.Errorf("registration failed: %s (status: %d)", string(body), resp.StatusCode)
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
