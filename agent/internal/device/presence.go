package device

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/stangtennis/remote-agent/internal/updater"
	"github.com/stangtennis/remote-agent/internal/version"
)

func (d *Device) StartPresence() {
	interval := time.Duration(d.cfg.HeartbeatInterval) * time.Second
	if interval == 0 {
		interval = 30 * time.Second // Default 30 seconds
	}

	// Build heartbeat config — prefer per-device api_key (never expires);
	// JWT is best-effort and only used as a secondary auth path.
	makeConfig := func() RegistrationConfig {
		token := ""
		if d.tokenProvider != nil {
			if t, err := d.tokenProvider.GetToken(); err == nil {
				token = t
			}
		}
		return RegistrationConfig{
			SupabaseURL: d.cfg.SupabaseURL,
			AnonKey:     d.cfg.SupabaseAnonKey,
			AccessToken: token,
			APIKey:      d.APIKey,
		}
	}

	if d.APIKey == "" {
		fmt.Println("⚠️  Heartbeat: api_key missing — falling back to JWT-only auth (expect failures after token expiry)")
	}

	// Single heartbeat attempt. Returns true on success.
	doHeartbeat := func() bool {
		config := makeConfig()

		isHealthy := true
		if d.healthCheck != nil {
			isHealthy = d.healthCheck()
		}

		var ci []ConnectionInfo
		if d.connInfoFunc != nil {
			ct, bs, br := d.connInfoFunc()
			if ct != "" {
				ci = append(ci, ConnectionInfo{Type: ct, BytesSent: bs, BytesReceived: br})
			}
		}

		result, err := UpdateHeartbeat(config, d.ID, isHealthy, ci...)
		if err != nil {
			d.lastHeartbeatErr = err
			return false
		}
		d.lastHeartbeatErr = nil
		d.handlePendingCommand(config, result)
		return true
	}

	// Initial heartbeat (don't loop forever on first failure — we still
	// progress to the main loop which has its own retry logic).
	if doHeartbeat() {
		atomic.StoreInt64(&d.lastHeartbeatSuccess, time.Now().Unix())
	} else {
		fmt.Printf("⚠️  Initial heartbeat failed: %v\n", d.lastHeartbeatErr)
	}

	// Adaptive heartbeat loop with exponential backoff on failure.
	// On success: cadence stays at the configured interval (default 30s).
	// On failure: 30s → 60s → 120s → 240s → 300s (cap), reset on next
	// success. We log errors only on transitions and once a minute when
	// we're in degraded mode, so persistent outages don't spam the log.
	const maxBackoff = 5 * time.Minute
	consecutiveFailures := 0
	var lastErrLog time.Time
	var degradedAnnounced bool
	outageStart := time.Time{}

	for {
		var delay time.Duration
		if consecutiveFailures == 0 {
			delay = interval
		} else {
			// 30s × 2^(failures-1), capped
			delay = time.Duration(int64(interval) << uint(consecutiveFailures-1))
			if delay > maxBackoff || delay <= 0 {
				delay = maxBackoff
			}
		}

		time.Sleep(delay)

		ok := doHeartbeat()
		atomic.StoreInt32(&d.consecutiveHeartbeatFailures, int32(consecutiveFailures))
		if ok {
			now := time.Now()
			atomic.StoreInt64(&d.lastHeartbeatSuccess, now.Unix())
			if consecutiveFailures > 0 {
				downtime := time.Since(outageStart).Round(time.Second)
				fmt.Printf("✅ Heartbeat genoptaget efter %s nedetid (%d fejlede forsøg)\n", downtime, consecutiveFailures)
			}
			consecutiveFailures = 0
			degradedAnnounced = false
			outageStart = time.Time{}
			continue
		}

		consecutiveFailures++
		atomic.StoreInt32(&d.consecutiveHeartbeatFailures, int32(consecutiveFailures))

		if consecutiveFailures == 1 {
			outageStart = time.Now()
			fmt.Printf("⚠️  Heartbeat failed: %v (retry om %s)\n", d.lastHeartbeatErr, delay)
		} else if !degradedAnnounced && consecutiveFailures >= 3 {
			fmt.Printf("⚠️  Heartbeat: vedvarende nedetid (%d forsøg fejlet) — backoff %s\n", consecutiveFailures, delay)
			degradedAnnounced = true
			lastErrLog = time.Now()
		} else if time.Since(lastErrLog) >= time.Minute {
			fmt.Printf("⚠️  Heartbeat: stadig nede efter %d forsøg (sidste fejl: %v)\n", consecutiveFailures, d.lastHeartbeatErr)
			lastErrLog = time.Now()
		}
	}
}

// HeartbeatHealth returns degraded-mode telemetry for callers that want to
// surface "agent reachable but degraded" state in UI or metrics.
func (d *Device) HeartbeatHealth() (consecutiveFailures int, lastSuccess time.Time) {
	consecutiveFailures = int(atomic.LoadInt32(&d.consecutiveHeartbeatFailures))
	if ts := atomic.LoadInt64(&d.lastHeartbeatSuccess); ts > 0 {
		lastSuccess = time.Unix(ts, 0)
	}
	return
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

	// Exit the current process so the new version can replace the binary
	// Windows: SCM will restart the service; macOS: LaunchAgent KeepAlive restarts
	go func() {
		time.Sleep(1 * time.Second)
		log.Println("🔄 Exiting for update...")
		os.Exit(0)
	}()
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
