package device

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

func (d *Device) StartPresence() {
	ticker := time.NewTicker(time.Duration(d.cfg.HeartbeatInterval) * time.Second)
	defer ticker.Stop()

	// Send initial heartbeat
	d.sendHeartbeat()

	for range ticker.C {
		if err := d.sendHeartbeat(); err != nil {
			fmt.Printf("⚠️  Heartbeat failed: %v\n", err)
		}
	}
}

func (d *Device) sendHeartbeat() error {
	// Update device last_seen and is_online status via Supabase
	url := d.cfg.SupabaseURL + "/rest/v1/remote_devices"

	reqBody := map[string]interface{}{
		"is_online": true,
		"last_seen": time.Now().UTC().Format(time.RFC3339),
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("PATCH", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("apikey", d.cfg.SupabaseAnonKey)
	req.Header.Set("Authorization", "Bearer "+d.cfg.SupabaseAnonKey)
	req.Header.Set("Prefer", "return=minimal")
	
	// Filter by device_id
	q := req.URL.Query()
	q.Add("device_id", "eq."+d.ID)
	req.URL.RawQuery = q.Encode()

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("heartbeat failed with status: %d", resp.StatusCode)
	}

	return nil
}
