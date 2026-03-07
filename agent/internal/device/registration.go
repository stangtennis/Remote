package device

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/stangtennis/remote-agent/internal/version"
)

// ipInfo caches the public IP and ISP
var cachedIPInfo struct {
	IP  string
	ISP string
}

// fetchPublicIPInfo gets public IP and ISP from ip-api.com (free, no key needed)
func fetchPublicIPInfo() (ip, isp string) {
	if cachedIPInfo.IP != "" {
		return cachedIPInfo.IP, cachedIPInfo.ISP
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("http://ip-api.com/json/?fields=query,isp")
	if err != nil {
		log.Printf("⚠️ IP lookup failed: %v", err)
		return "", ""
	}
	defer resp.Body.Close()

	var result struct {
		Query string `json:"query"`
		ISP   string `json:"isp"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("⚠️ IP lookup parse failed: %v", err)
		return "", ""
	}

	cachedIPInfo.IP = strings.TrimSpace(result.Query)
	cachedIPInfo.ISP = strings.TrimSpace(result.ISP)
	log.Printf("🌐 Public IP: %s (%s)", cachedIPInfo.IP, cachedIPInfo.ISP)
	return cachedIPInfo.IP, cachedIPInfo.ISP
}

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

	// Fetch public IP and ISP
	publicIP, isp := fetchPublicIPInfo()

	// Create payload
	payload := map[string]interface{}{
		"device_id":      device.DeviceID,
		"device_name":    device.DeviceName,
		"platform":       device.Platform,
		"arch":           runtime.GOARCH,
		"owner_id":       config.UserID,
		"is_online":      true,
		"last_seen":      time.Now().Format(time.RFC3339),
		"agent_version":  version.Version,
		"public_ip":      publicIP,
		"isp":            isp,
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
			"device_name":   device.DeviceName,
			"owner_id":      config.UserID,
			"is_online":     true,
			"last_seen":     time.Now().Format(time.RFC3339),
			"agent_version": version.Version,
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

// HeartbeatResult contains information read back from the heartbeat response.
type HeartbeatResult struct {
	PendingCommand string // Non-empty if dashboard sent a command (e.g. "force_update")
}

// UpdateHeartbeat updates the device heartbeat using authenticated token.
// isOnline indicates whether the agent is healthy and reachable.
// Returns a HeartbeatResult with any pending commands from the dashboard.
func UpdateHeartbeat(config RegistrationConfig, deviceID string, isOnline bool) (*HeartbeatResult, error) {
	result := &HeartbeatResult{}

	url := fmt.Sprintf("%s/rest/v1/remote_devices?device_id=eq.%s", config.SupabaseURL, deviceID)

	now := time.Now().Format(time.RFC3339)
	publicIP, isp := fetchPublicIPInfo()
	payload := map[string]interface{}{
		"is_online":     isOnline,
		"last_seen":     now,
		"agent_version": version.Version,
		"public_ip":     publicIP,
		"isp":           isp,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return result, fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequest("PATCH", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return result, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("apikey", config.AnonKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Prefer", "return=representation")
	if config.AccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+config.AccessToken)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return result, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return result, fmt.Errorf("heartbeat failed: %s (status: %d)", string(body), resp.StatusCode)
	}

	// Parse response to check for pending commands
	if len(body) > 0 {
		var rows []struct {
			PendingCommand *string `json:"pending_command"`
		}
		if err := json.Unmarshal(body, &rows); err == nil && len(rows) > 0 && rows[0].PendingCommand != nil {
			result.PendingCommand = *rows[0].PendingCommand
		}
	}

	return result, nil
}

// ClearPendingCommand clears the pending_command column after processing.
func ClearPendingCommand(config RegistrationConfig, deviceID string) error {
	url := fmt.Sprintf("%s/rest/v1/remote_devices?device_id=eq.%s", config.SupabaseURL, deviceID)

	payload := map[string]interface{}{
		"pending_command": nil,
	}

	jsonData, _ := json.Marshal(payload)
	req, err := http.NewRequest("PATCH", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	req.Header.Set("apikey", config.AnonKey)
	req.Header.Set("Content-Type", "application/json")
	if config.AccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+config.AccessToken)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// SetOffline marks the device as offline in the database using authenticated token
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
	if config.AccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+config.AccessToken)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	return nil
}
