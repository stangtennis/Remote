package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// device represents a remote desktop agent
type device struct {
	DeviceID   string    `json:"device_id"`
	DeviceName string    `json:"device_name"`
	Platform   string    `json:"platform"`
	Status     string    `json:"status"`
	LastSeen   time.Time `json:"last_seen"`
}

// fetchDevices retrieves all devices for the authenticated user
func fetchDevices(supabaseURL, anonKey string, auth *authInfo) ([]device, error) {
	token := auth.GetToken()
	url := fmt.Sprintf("%s/rest/v1/remote_devices?owner_id=eq.%s&select=*", supabaseURL, auth.userID)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("apikey", anonKey)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var devices []device
	if err := json.NewDecoder(resp.Body).Decode(&devices); err != nil {
		return nil, err
	}
	return devices, nil
}

// isOnline checks if a device was seen recently
func (d *device) isOnline() bool {
	return !d.LastSeen.IsZero() && time.Since(d.LastSeen) < 2*time.Minute
}

// statusString returns "online" or "offline"
func (d *device) statusString() string {
	if d.isOnline() {
		return "online"
	}
	return "offline"
}
