package device

import (
	"fmt"
	"time"
)

func (d *Device) StartPresence() {
	interval := time.Duration(d.cfg.HeartbeatInterval) * time.Second
	if interval == 0 {
		interval = 30 * time.Second // Default 30 seconds
	}

	// Send initial heartbeat with authenticated token
	token, err := d.tokenProvider.GetToken()
	if err != nil {
		fmt.Printf("⚠️  Failed to get auth token for heartbeat: %v\n", err)
	}

	config := RegistrationConfig{
		SupabaseURL: d.cfg.SupabaseURL,
		AnonKey:     d.cfg.SupabaseAnonKey,
		AccessToken: token,
	}

	if err := UpdateHeartbeat(config, d.ID); err != nil {
		fmt.Printf("⚠️  Initial heartbeat failed: %v\n", err)
	}

	// Start periodic heartbeats with token refresh
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		// Get fresh token for each heartbeat
		token, err := d.tokenProvider.GetToken()
		if err != nil {
			fmt.Printf("⚠️  Heartbeat auth failed: %v\n", err)
			continue
		}

		config := RegistrationConfig{
			SupabaseURL: d.cfg.SupabaseURL,
			AnonKey:     d.cfg.SupabaseAnonKey,
			AccessToken: token,
		}

		if err := UpdateHeartbeat(config, d.ID); err != nil {
			fmt.Printf("⚠️  Heartbeat failed: %v\n", err)
		}
	}
}
