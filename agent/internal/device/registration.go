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

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/stangtennis/remote-agent/internal/version"
)

// Package-level HTTP client with connection reuse
var httpClient = &http.Client{
	Timeout: 10 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        10,
		MaxIdleConnsPerHost: 5,
		IdleConnTimeout:     90 * time.Second,
	},
}

// systemMetrics holds collected system resource metrics.
type systemMetrics struct {
	CPUPercent   float64
	MemUsedMB    int
	MemTotalMB   int
	DiskUsedGB   int
	DiskTotalGB  int
}

// collectSystemMetrics gathers CPU, RAM and disk usage from the OS.
func collectSystemMetrics() systemMetrics {
	m := systemMetrics{}

	// CPU — instant sample (0 = no blocking interval, false = aggregate all cores)
	if percents, err := cpu.Percent(0, false); err == nil && len(percents) > 0 {
		m.CPUPercent = percents[0]
	}

	// RAM
	if v, err := mem.VirtualMemory(); err == nil {
		m.MemUsedMB = int(v.Used / 1024 / 1024)
		m.MemTotalMB = int(v.Total / 1024 / 1024)
	}

	// Disk — "C:" on Windows, "/" elsewhere
	diskPath := "/"
	if runtime.GOOS == "windows" {
		diskPath = "C:"
	}
	if d, err := disk.Usage(diskPath); err == nil {
		m.DiskUsedGB = int(d.Used / 1024 / 1024 / 1024)
		m.DiskTotalGB = int(d.Total / 1024 / 1024 / 1024)
	}

	return m
}

// ipInfo caches the public IP and ISP
var cachedIPInfo struct {
	IP  string
	ISP string
}

// fetchPublicIPInfo gets public IP and ISP via HTTPS (no API key needed)
func fetchPublicIPInfo() (ip, isp string) {
	if cachedIPInfo.IP != "" {
		return cachedIPInfo.IP, cachedIPInfo.ISP
	}

	resp, err := httpClient.Get("https://ipinfo.io/json")
	if err != nil {
		log.Printf("⚠️ IP lookup failed: %v", err)
		return "", ""
	}
	defer resp.Body.Close()

	var result struct {
		IP  string `json:"ip"`
		Org string `json:"org"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("⚠️ IP lookup parse failed: %v", err)
		return "", ""
	}

	cachedIPInfo.IP = strings.TrimSpace(result.IP)
	cachedIPInfo.ISP = strings.TrimSpace(result.Org)
	log.Printf("🌐 Public IP: %s (%s)", cachedIPInfo.IP, cachedIPInfo.ISP)
	return cachedIPInfo.IP, cachedIPInfo.ISP
}

// RegistrationConfig holds Supabase configuration
type RegistrationConfig struct {
	SupabaseURL string
	AnonKey     string
	AccessToken string // User's access token (may expire — fall back to APIKey)
	UserID      string // User's ID after login
	APIKey      string // Per-device api_key — preferred for agent → server calls
}

// applyAuthHeaders sets the right auth headers on req. Prefers the stable
// per-device api_key (x-device-key) when available — that path keeps working
// even if the user's JWT has expired. Falls back to Bearer token for the
// initial registration where the device row doesn't exist yet.
func (c *RegistrationConfig) applyAuthHeaders(req *http.Request) {
	req.Header.Set("apikey", c.AnonKey)
	if c.APIKey != "" {
		req.Header.Set("x-device-key", c.APIKey)
	}
	if c.AccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.AccessToken)
	}
}

// DeviceInfo represents device information
type DeviceInfo struct {
	DeviceID   string `json:"device_id"`
	DeviceName string `json:"device_name"`
	Platform   string `json:"platform"`
	Status     string `json:"status"`
	APIKey     string `json:"api_key,omitempty"` // Returned from upsert; persist to credentials
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

	// Register with Supabase (upsert) — populates device.APIKey on success
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

	config.applyAuthHeaders(req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Prefer", "resolution=merge-duplicates,return=representation")

	resp, err := httpClient.Do(req)
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
		config.applyAuthHeaders(updateReq)
		updateReq.Header.Set("Content-Type", "application/json")
		updateReq.Header.Set("Prefer", "return=representation")

		updateResp, err := httpClient.Do(updateReq)
		if err != nil {
			return fmt.Errorf("failed to update device: %w", err)
		}
		defer updateResp.Body.Close()

		updateBody, _ := io.ReadAll(updateResp.Body)
		if updateResp.StatusCode == http.StatusOK || updateResp.StatusCode == http.StatusNoContent {
			extractAPIKey(updateBody, device)
			fmt.Println("✅ Device updated successfully (already registered)")
			return nil
		}
		return fmt.Errorf("device update failed: %s (status: %d)", string(updateBody), updateResp.StatusCode)
	}

	if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK {
		extractAPIKey(body, device)
		fmt.Println("✅ Device registered successfully")
		return nil
	}

	return fmt.Errorf("registration failed: %s (status: %d)", string(body), resp.StatusCode)
}

// extractAPIKey pulls the api_key field out of a PostgREST representation
// response (`Prefer: return=representation`) into the device struct.
func extractAPIKey(body []byte, device *DeviceInfo) {
	if len(body) == 0 {
		return
	}
	var rows []struct {
		APIKey string `json:"api_key"`
	}
	if err := json.Unmarshal(body, &rows); err == nil && len(rows) > 0 {
		device.APIKey = rows[0].APIKey
	}
}

// HeartbeatResult contains information read back from the heartbeat response.
type HeartbeatResult struct {
	PendingCommand string // Non-empty if dashboard sent a command (e.g. "force_update")
}

// ConnectionInfo holds optional WebRTC connection metrics for heartbeat
type ConnectionInfo struct {
	Type          string // "host", "srflx", "relay"
	BytesSent     uint64
	BytesReceived uint64
}

// UpdateHeartbeat updates the device heartbeat using authenticated token.
// isOnline indicates whether the agent is healthy and reachable.
// Returns a HeartbeatResult with any pending commands from the dashboard.
func UpdateHeartbeat(config RegistrationConfig, deviceID string, isOnline bool, connInfo ...ConnectionInfo) (*HeartbeatResult, error) {
	result := &HeartbeatResult{}

	url := fmt.Sprintf("%s/rest/v1/remote_devices?device_id=eq.%s", config.SupabaseURL, deviceID)

	now := time.Now().Format(time.RFC3339)
	publicIP, isp := fetchPublicIPInfo()
	metrics := collectSystemMetrics()
	payload := map[string]interface{}{
		"is_online":       isOnline,
		"last_seen":       now,
		"agent_version":   version.Version,
		"public_ip":       publicIP,
		"isp":             isp,
		"cpu_percent":     metrics.CPUPercent,
		"memory_used_mb":  metrics.MemUsedMB,
		"memory_total_mb": metrics.MemTotalMB,
		"disk_used_gb":    metrics.DiskUsedGB,
		"disk_total_gb":   metrics.DiskTotalGB,
	}

	// Add connection info if provided
	if len(connInfo) > 0 && connInfo[0].Type != "" {
		payload["connection_type"] = connInfo[0].Type
		payload["session_bytes_sent"] = connInfo[0].BytesSent
		payload["session_bytes_received"] = connInfo[0].BytesReceived
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return result, fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequest("PATCH", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return result, fmt.Errorf("failed to create request: %w", err)
	}

	config.applyAuthHeaders(req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Prefer", "return=representation")

	resp, err := httpClient.Do(req)
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

	config.applyAuthHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
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

	config.applyAuthHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	return nil
}
