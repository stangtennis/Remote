package supabase

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
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
	DeviceID      string    `json:"device_id"`
	DeviceName    string    `json:"device_name"`
	Platform      string    `json:"platform"`
	OwnerID       string    `json:"owner_id"`
	Status        string    `json:"status"`
	LastHeartbeat time.Time `json:"last_heartbeat"`
	CreatedAt     time.Time `json:"created_at"`
}

// NewClient creates a new Supabase client
func NewClient(url, anonKey string) *Client {
	return &Client{
		URL:     url,
		AnonKey: anonKey,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// SignIn authenticates a user with email and password
func (c *Client) SignIn(email, password string) (*AuthResponse, error) {
	url := fmt.Sprintf("%s/auth/v1/token?grant_type=password", c.URL)

	payload := map[string]string{
		"email":    email,
		"password": password,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("apikey", c.AnonKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("authentication failed: %s (status: %d)", string(body), resp.StatusCode)
	}

	var authResp AuthResponse
	if err := json.Unmarshal(body, &authResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Store the auth token
	c.AuthToken = authResp.AccessToken

	return &authResp, nil
}

// GetDevices fetches all devices for the authenticated user
func (c *Client) GetDevices() ([]Device, error) {
	if c.AuthToken == "" {
		return nil, fmt.Errorf("not authenticated")
	}

	url := fmt.Sprintf("%s/rest/v1/remote_devices?select=*&order=last_heartbeat.desc", c.URL)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("apikey", c.AnonKey)
	req.Header.Set("Authorization", "Bearer "+c.AuthToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch devices: %s (status: %d)", string(body), resp.StatusCode)
	}

	var devices []Device
	if err := json.Unmarshal(body, &devices); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return devices, nil
}

// CheckApproval checks if the user is approved
func (c *Client) CheckApproval(userID string) (bool, error) {
	if c.AuthToken == "" {
		return false, fmt.Errorf("not authenticated")
	}

	url := fmt.Sprintf("%s/rest/v1/user_approvals?select=approved&user_id=eq.%s", c.URL, userID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("apikey", c.AnonKey)
	req.Header.Set("Authorization", "Bearer "+c.AuthToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("failed to check approval: %s (status: %d)", string(body), resp.StatusCode)
	}

	var approvals []struct {
		Approved bool `json:"approved"`
	}
	if err := json.Unmarshal(body, &approvals); err != nil {
		return false, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if len(approvals) == 0 {
		return false, nil
	}

	return approvals[0].Approved, nil
}
