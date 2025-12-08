package device

import (
	"fmt"
	"time"
)

func (d *Device) StartPresence() {
	// Use new heartbeat system from registration.go
	config := RegistrationConfig{
		SupabaseURL: d.cfg.SupabaseURL,
		AnonKey:     d.cfg.SupabaseAnonKey,
	}

	interval := time.Duration(d.cfg.HeartbeatInterval) * time.Second
	if interval == 0 {
		interval = 30 * time.Second // Default 30 seconds
	}

	// Send initial heartbeat
	if err := UpdateHeartbeat(config, d.ID); err != nil {
		fmt.Printf("⚠️  Initial heartbeat failed: %v\n", err)
	}

	// Start periodic heartbeats
	StartHeartbeat(config, d.ID, interval)
}
