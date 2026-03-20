package device

import (
	"fmt"
	"log"
	"os/exec"
	"runtime"
	"time"

	"github.com/stangtennis/remote-agent/internal/updater"
	"github.com/stangtennis/remote-agent/internal/version"
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

	if result, err := UpdateHeartbeat(config, d.ID, true); err != nil {
		fmt.Printf("⚠️  Initial heartbeat failed: %v\n", err)
	} else {
		d.handlePendingCommand(config, result)
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

		// Determine health status — if polling is unhealthy, report offline
		isHealthy := true
		if d.healthCheck != nil {
			isHealthy = d.healthCheck()
		}

		if !isHealthy {
			fmt.Println("⚠️  Heartbeat: polling is unhealthy — reporting offline")
		}

		if result, err := UpdateHeartbeat(config, d.ID, isHealthy); err != nil {
			fmt.Printf("⚠️  Heartbeat failed: %v\n", err)
		} else {
			d.handlePendingCommand(config, result)
		}
	}
}

// handlePendingCommand processes any pending command from the dashboard.
func (d *Device) handlePendingCommand(config RegistrationConfig, result *HeartbeatResult) {
	if result == nil || result.PendingCommand == "" {
		return
	}

	log.Printf("📬 Pending command received: %s", result.PendingCommand)

	// Clear command immediately so it doesn't re-trigger
	if err := ClearPendingCommand(config, d.ID); err != nil {
		log.Printf("⚠️  Failed to clear pending command: %v", err)
	}

	switch result.PendingCommand {
	case "force_update":
		go d.executeForceUpdate()
	case "restart":
		go d.executeRestart()
	case "lock":
		go d.executeLock()
	case "shutdown":
		go d.executeShutdown()
	default:
		log.Printf("⚠️  Unknown pending command: %s", result.PendingCommand)
	}
}

// executeForceUpdate downloads and installs agent update.
func (d *Device) executeForceUpdate() {
	log.Println("🔄 Force update triggered via dashboard command")

	u, err := updater.NewUpdater(version.Version)
	if err != nil {
		log.Printf("❌ Force update: could not create updater: %v", err)
		return
	}

	if err := u.CheckForUpdate(); err != nil {
		log.Printf("❌ Force update: check failed: %v", err)
		return
	}

	if u.GetAvailableUpdate() == nil {
		log.Println("✅ Force update: already up to date (" + version.Version + ")")
		return
	}

	info := u.GetAvailableUpdate()
	log.Printf("📥 Downloading %s...", info.TagName)

	if err := u.DownloadUpdate(); err != nil {
		log.Printf("❌ Force update: download failed: %v", err)
		return
	}

	log.Printf("📦 Installing %s...", info.TagName)
	if err := u.InstallUpdate(); err != nil {
		log.Printf("❌ Force update: install failed: %v", err)
		return
	}

	log.Printf("✅ Force update: installed %s, agent will restart", info.TagName)
}

// executeRestart triggers an OS restart.
func (d *Device) executeRestart() {
	log.Println("🔄 Restart triggered via dashboard command")
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("shutdown", "/r", "/t", "5", "/c", "Remote Desktop: Genstart anmodet")
	} else if runtime.GOOS == "darwin" {
		cmd = exec.Command("sudo", "shutdown", "-r", "+1")
	} else {
		cmd = exec.Command("sudo", "shutdown", "-r", "+1")
	}
	if err := cmd.Run(); err != nil {
		log.Printf("❌ Restart failed: %v", err)
	}
}

// executeLock locks the workstation screen.
func (d *Device) executeLock() {
	log.Println("🔒 Lock triggered via dashboard command")
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("rundll32.exe", "user32.dll,LockWorkStation")
	} else if runtime.GOOS == "darwin" {
		cmd = exec.Command("pmset", "displaysleepnow")
	} else {
		cmd = exec.Command("loginctl", "lock-session")
	}
	if err := cmd.Run(); err != nil {
		log.Printf("❌ Lock failed: %v", err)
	}
}

// executeShutdown triggers an OS shutdown.
func (d *Device) executeShutdown() {
	log.Println("⏻ Shutdown triggered via dashboard command")
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("shutdown", "/s", "/t", "10", "/c", "Remote Desktop: Nedlukning anmodet")
	} else if runtime.GOOS == "darwin" {
		cmd = exec.Command("sudo", "shutdown", "-h", "+1")
	} else {
		cmd = exec.Command("sudo", "shutdown", "-h", "+1")
	}
	if err := cmd.Run(); err != nil {
		log.Printf("❌ Shutdown failed: %v", err)
	}
}
