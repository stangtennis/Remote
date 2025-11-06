package supabase

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/stangtennis/Remote/controller/internal/logger"
)

// Client represents a Supabase client
type Client struct {
	URL       string
	AnonKey   string
	AuthToken string
	client    *http.Client
}

// AuthResponse represents the authentication response
type AuthResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	User         User   `json:"user"`
}

// User represents a Supabase user
type User struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

// Device represents a remote device
type Device struct {
	DeviceID   string    `json:"device_id"`
	DeviceName string    `json:"device_name"`
	Platform   string    `json:"platform"`
	OwnerID    string    `json:"owner_id"`
	Status     string    `json:"status"`
	LastSeen   time.Time `json:"last_seen"`
	CreatedAt  time.Time `json:"created_at"`
	AssignedAt time.Time `json:"assigned_at"`
}

// NewClient creates a new Supabase client
func NewClient(url, anonKey string) *Client {
	logger.Debug("Creating Supabase client with URL: %s", url)
	logger.Debug("Anon key length: %d", len(anonKey))
	
	client := &Client{
		URL:     url,
		AnonKey: anonKey,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
	
	logger.Debug("Supabase client created successfully")
	return client
}

// SignIn authenticates a user with email and password
func (c *Client) SignIn(email, password string) (*AuthResponse, error) {
	logger.Debug("[SignIn] Starting authentication for email: %s", email)
	url := fmt.Sprintf("%s/auth/v1/token?grant_type=password", c.URL)
	logger.Debug("[SignIn] Auth URL: %s", url)

	payload := map[string]string{
		"email":    email,
		"password": password,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		logger.Error("[SignIn] Failed to marshal payload: %v", err)
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}
	logger.Debug("[SignIn] Payload marshaled, size: %d bytes", len(jsonData))

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		logger.Error("[SignIn] Failed to create HTTP request: %v", err)
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("apikey", c.AnonKey)
	logger.Debug("[SignIn] Request headers set, sending request...")

	resp, err := c.client.Do(req)
	if err != nil {
		logger.Error("[SignIn] HTTP request failed: %v", err)
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()
	logger.Debug("[SignIn] Received response with status: %d", resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Error("[SignIn] Failed to read response body: %v", err)
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	logger.Debug("[SignIn] Response body size: %d bytes", len(body))

	if resp.StatusCode != http.StatusOK {
		logger.Error("[SignIn] Authentication failed with status %d: %s", resp.StatusCode, string(body))
		return nil, fmt.Errorf("authentication failed: %s (status: %d)", string(body), resp.StatusCode)
	}

	var authResp AuthResponse
	if err := json.Unmarshal(body, &authResp); err != nil {
		logger.Error("[SignIn] Failed to unmarshal auth response: %v", err)
		logger.Debug("[SignIn] Response body: %s", string(body))
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Store the auth token
	c.AuthToken = authResp.AccessToken
	logger.Debug("[SignIn] Auth token stored, length: %d", len(c.AuthToken))
	logger.Info("[SignIn] Authentication successful for user: %s", authResp.User.Email)

	return &authResp, nil
}

// GetDevices fetches devices assigned to the authenticated user
func (c *Client) GetDevices(userID string) ([]Device, error) {
	logger.Debug("[GetDevices] Starting device fetch for user: %s", userID)
	
	if c.AuthToken == "" {
		logger.Error("[GetDevices] Not authenticated - auth token is empty")
		return nil, fmt.Errorf("not authenticated")
	}
	logger.Debug("[GetDevices] Auth token present, length: %d", len(c.AuthToken))

	// Call get_user_devices function
	url := fmt.Sprintf("%s/rest/v1/rpc/get_user_devices", c.URL)
	logger.Debug("[GetDevices] RPC URL: %s", url)

	payload := map[string]string{
		"p_user_id": userID,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		logger.Error("[GetDevices] Failed to marshal payload: %v", err)
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}
	logger.Debug("[GetDevices] Payload: %s", string(jsonData))

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		logger.Error("[GetDevices] Failed to create HTTP request: %v", err)
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("apikey", c.AnonKey)
	req.Header.Set("Authorization", "Bearer "+c.AuthToken)
	req.Header.Set("Content-Type", "application/json")
	logger.Debug("[GetDevices] Request headers set, sending request...")

	resp, err := c.client.Do(req)
	if err != nil {
		logger.Error("[GetDevices] HTTP request failed: %v", err)
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()
	logger.Debug("[GetDevices] Received response with status: %d", resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Error("[GetDevices] Failed to read response body: %v", err)
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	logger.Debug("[GetDevices] Response body: %s", string(body))

	if resp.StatusCode != http.StatusOK {
		logger.Error("[GetDevices] Failed to fetch devices with status %d: %s", resp.StatusCode, string(body))
		return nil, fmt.Errorf("failed to fetch devices: %s (status: %d)", string(body), resp.StatusCode)
	}

	var devices []Device
	if err := json.Unmarshal(body, &devices); err != nil {
		logger.Error("[GetDevices] Failed to unmarshal devices response: %v", err)
		logger.Debug("[GetDevices] Raw response: %s", string(body))
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	logger.Info("[GetDevices] Successfully fetched %d devices", len(devices))
	for i, device := range devices {
		logger.Debug("[GetDevices] Device %d: ID=%s, Name=%s, Platform=%s, Status=%s, Owner=%s",
			i+1, device.DeviceID, device.DeviceName, device.Platform, device.Status, device.OwnerID)
	}

	return devices, nil
}

// GetAllDevices fetches all devices (including unassigned ones)
func (c *Client) GetAllDevices() ([]Device, error) {
	logger.Debug("[GetAllDevices] Fetching all devices")
	
	if c.AuthToken == "" {
		logger.Error("[GetAllDevices] Not authenticated")
		return nil, fmt.Errorf("not authenticated")
	}

	url := fmt.Sprintf("%s/rest/v1/remote_devices?select=*", c.URL)
	logger.Debug("[GetAllDevices] URL: %s", url)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		logger.Error("[GetAllDevices] Failed to create request: %v", err)
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("apikey", c.AnonKey)
	req.Header.Set("Authorization", "Bearer "+c.AuthToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		logger.Error("[GetAllDevices] Request failed: %v", err)
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Error("[GetAllDevices] Failed to read response: %v", err)
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		logger.Error("[GetAllDevices] Failed with status %d: %s", resp.StatusCode, string(body))
		return nil, fmt.Errorf("failed to fetch devices: %s (status: %d)", string(body), resp.StatusCode)
	}

	var devices []Device
	if err := json.Unmarshal(body, &devices); err != nil {
		logger.Error("[GetAllDevices] Failed to unmarshal: %v", err)
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	logger.Info("[GetAllDevices] Successfully fetched %d devices", len(devices))
	return devices, nil
}

// AssignDevice assigns a device to the current user (approves it)
func (c *Client) AssignDevice(deviceID, userID string) error {
	logger.Debug("[AssignDevice] Assigning device %s to user %s", deviceID, userID)
	
	if c.AuthToken == "" {
		logger.Error("[AssignDevice] Not authenticated")
		return fmt.Errorf("not authenticated")
	}

	// Step 1: Update the device's owner_id
	deviceURL := fmt.Sprintf("%s/rest/v1/remote_devices?device_id=eq.%s", c.URL, deviceID)
	logger.Debug("[AssignDevice] Updating device owner, URL: %s", deviceURL)

	devicePayload := map[string]string{
		"owner_id": userID,
	}

	deviceJSON, err := json.Marshal(devicePayload)
	if err != nil {
		logger.Error("[AssignDevice] Failed to marshal device payload: %v", err)
		return fmt.Errorf("failed to marshal device payload: %w", err)
	}

	deviceReq, err := http.NewRequest("PATCH", deviceURL, bytes.NewBuffer(deviceJSON))
	if err != nil {
		logger.Error("[AssignDevice] Failed to create device update request: %v", err)
		return fmt.Errorf("failed to create device update request: %w", err)
	}

	deviceReq.Header.Set("apikey", c.AnonKey)
	deviceReq.Header.Set("Authorization", "Bearer "+c.AuthToken)
	deviceReq.Header.Set("Content-Type", "application/json")
	deviceReq.Header.Set("Prefer", "return=minimal")

	deviceResp, err := c.client.Do(deviceReq)
	if err != nil {
		logger.Error("[AssignDevice] Device update request failed: %v", err)
		return fmt.Errorf("device update request failed: %w", err)
	}
	defer deviceResp.Body.Close()

	deviceBody, _ := io.ReadAll(deviceResp.Body)
	
	if deviceResp.StatusCode != http.StatusNoContent && deviceResp.StatusCode != http.StatusOK {
		logger.Error("[AssignDevice] Failed to update device with status %d: %s", deviceResp.StatusCode, string(deviceBody))
		return fmt.Errorf("failed to update device: %s (status: %d)", string(deviceBody), deviceResp.StatusCode)
	}

	logger.Debug("[AssignDevice] Device owner updated successfully")

	// Step 2: Create device assignment record
	url := fmt.Sprintf("%s/rest/v1/device_assignments", c.URL)
	logger.Debug("[AssignDevice] Creating assignment, URL: %s", url)

	payload := map[string]string{
		"device_id":   deviceID,
		"user_id":     userID,
		"assigned_by": userID, // User assigns to themselves
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		logger.Error("[AssignDevice] Failed to marshal payload: %v", err)
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		logger.Error("[AssignDevice] Failed to create request: %v", err)
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("apikey", c.AnonKey)
	req.Header.Set("Authorization", "Bearer "+c.AuthToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Prefer", "return=minimal")

	resp, err := c.client.Do(req)
	if err != nil {
		logger.Error("[AssignDevice] Request failed: %v", err)
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		logger.Error("[AssignDevice] Failed with status %d: %s", resp.StatusCode, string(body))
		return fmt.Errorf("failed to assign device: %s (status: %d)", string(body), resp.StatusCode)
	}

	logger.Info("[AssignDevice] Successfully assigned device %s to user %s", deviceID, userID)
	return nil
}

// UnassignDevice removes a device assignment (unassigns from user)
func (c *Client) UnassignDevice(deviceID, userID string) error {
	logger.Debug("[UnassignDevice] Unassigning device %s from user %s", deviceID, userID)
	
	if c.AuthToken == "" {
		logger.Error("[UnassignDevice] Not authenticated")
		return fmt.Errorf("not authenticated")
	}

	url := fmt.Sprintf("%s/rest/v1/device_assignments?device_id=eq.%s&user_id=eq.%s", c.URL, deviceID, userID)
	logger.Debug("[UnassignDevice] URL: %s", url)

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		logger.Error("[UnassignDevice] Failed to create request: %v", err)
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("apikey", c.AnonKey)
	req.Header.Set("Authorization", "Bearer "+c.AuthToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		logger.Error("[UnassignDevice] Request failed: %v", err)
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		logger.Error("[UnassignDevice] Failed with status %d: %s", resp.StatusCode, string(body))
		return fmt.Errorf("failed to unassign device: %s (status: %d)", string(body), resp.StatusCode)
	}

	logger.Info("[UnassignDevice] Successfully unassigned device %s from user %s", deviceID, userID)
	return nil
}

// DeleteDevice permanently deletes a device from the database
func (c *Client) DeleteDevice(deviceID string) error {
	logger.Debug("[DeleteDevice] Deleting device %s", deviceID)
	
	if c.AuthToken == "" {
		logger.Error("[DeleteDevice] Not authenticated")
		return fmt.Errorf("not authenticated")
	}

	url := fmt.Sprintf("%s/rest/v1/remote_devices?device_id=eq.%s", c.URL, deviceID)
	logger.Debug("[DeleteDevice] URL: %s", url)

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		logger.Error("[DeleteDevice] Failed to create request: %v", err)
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("apikey", c.AnonKey)
	req.Header.Set("Authorization", "Bearer "+c.AuthToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		logger.Error("[DeleteDevice] Request failed: %v", err)
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		logger.Error("[DeleteDevice] Failed with status %d: %s", resp.StatusCode, string(body))
		return fmt.Errorf("failed to delete device: %s (status: %d)", string(body), resp.StatusCode)
	}

	logger.Info("[DeleteDevice] Successfully deleted device %s", deviceID)
	return nil
}

// CheckApproval checks if the user is approved
func (c *Client) CheckApproval(userID string) (bool, error) {
	logger.Debug("[CheckApproval] Checking approval for user: %s", userID)
	
	if c.AuthToken == "" {
		logger.Error("[CheckApproval] Not authenticated - auth token is empty")
		return false, fmt.Errorf("not authenticated")
	}

	url := fmt.Sprintf("%s/rest/v1/user_approvals?select=approved&user_id=eq.%s", c.URL, userID)
	logger.Debug("[CheckApproval] Query URL: %s", url)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		logger.Error("[CheckApproval] Failed to create HTTP request: %v", err)
		return false, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("apikey", c.AnonKey)
	req.Header.Set("Authorization", "Bearer "+c.AuthToken)
	req.Header.Set("Content-Type", "application/json")
	logger.Debug("[CheckApproval] Request headers set, sending request...")

	resp, err := c.client.Do(req)
	if err != nil {
		logger.Error("[CheckApproval] HTTP request failed: %v", err)
		return false, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()
	logger.Debug("[CheckApproval] Received response with status: %d", resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Error("[CheckApproval] Failed to read response body: %v", err)
		return false, fmt.Errorf("failed to read response: %w", err)
	}
	logger.Debug("[CheckApproval] Response body: %s", string(body))

	if resp.StatusCode != http.StatusOK {
		logger.Error("[CheckApproval] Failed to check approval with status %d: %s", resp.StatusCode, string(body))
		return false, fmt.Errorf("failed to check approval: %s (status: %d)", string(body), resp.StatusCode)
	}

	var approvals []struct {
		Approved bool `json:"approved"`
	}
	if err := json.Unmarshal(body, &approvals); err != nil {
		logger.Error("[CheckApproval] Failed to unmarshal approval response: %v", err)
		return false, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if len(approvals) == 0 {
		logger.Info("[CheckApproval] No approval record found for user: %s", userID)
		return false, nil
	}

	logger.Info("[CheckApproval] User %s approval status: %v", userID, approvals[0].Approved)
	return approvals[0].Approved, nil
}
