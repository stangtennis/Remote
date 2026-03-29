package webrtc

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"log"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	pionwebrtc "github.com/pion/webrtc/v3"
	"github.com/stangtennis/remote-agent/internal/desktop"
	"github.com/stangtennis/remote-agent/internal/input"
	"github.com/stangtennis/remote-agent/internal/screen"
)

// StreamMode represents the current streaming mode
type StreamMode int

const (
	ModeIdleTiles   StreamMode = iota // Low motion, high quality tiles
	ModeActiveTiles                   // Active use, moderate quality tiles
	ModeActiveH264                    // Active use, H.264 encoding
)

func (sm StreamMode) String() string {
	switch sm {
	case ModeIdleTiles:
		return "idle-tiles"
	case ModeActiveTiles:
		return "active-tiles"
	case ModeActiveH264:
		return "h264"
	default:
		return "unknown"
	}
}

// ModeState tracks mode switching with hysteresis
type ModeState struct {
	current       StreamMode
	lastSwitch    time.Time
	minModeDur    time.Duration // Minimum time in mode before switching
	switchHistory []time.Time   // Recent switches for flapping detection
}

// SetH264Mode enables or disables H.264 video track mode
func (m *Manager) SetH264Mode(enabled bool) {
	if enabled {
		// Check prerequisites
		if m.videoTrack == nil {
			log.Println("⚠️ Kan ikke aktivere H.264 - video track ikke oprettet")
			return
		}
		if m.videoEncoder == nil {
			log.Println("⚠️ Kan ikke aktivere H.264 - video encoder ikke initialiseret")
			return
		}
		encName := m.videoEncoder.GetEncoderName()
		if encName != "openh264" {
			log.Printf("⚠️ Kan ikke aktivere H.264 - encoder understøtter ikke H.264 (encoder: %s)", encName)
			return
		}

		// Start video track
		m.videoTrack.Start()
		m.useH264 = true
		m.videoEncoder.ForceKeyframe()
		log.Printf("🎬 H.264 tilstand aktiveret (encoder: %s)", encName)
	} else {
		m.useH264 = false
		// Stop video track
		if m.videoTrack != nil {
			m.videoTrack.Stop()
		}
		log.Println("🎬 H.264 tilstand deaktiveret (bruger JPEG tiles)")
	}
}

// SetVideoBitrate adjusts the video encoder bitrate
func (m *Manager) SetVideoBitrate(kbps int) {
	if m.videoEncoder != nil {
		if err := m.videoEncoder.SetBitrate(kbps); err != nil {
			log.Printf("⚠️ Failed to set bitrate: %v", err)
		} else {
			log.Printf("🎬 Video bitrate set to %d kbps", kbps)
		}
	}
}

// updateMovingAverage updates a moving average array (max 6 samples = 3 seconds at 500ms)
func updateMovingAverage(arr *[]float64, val float64, maxLen int) float64 {
	*arr = append(*arr, val)
	if len(*arr) > maxLen {
		*arr = (*arr)[1:]
	}
	sum := 0.0
	for _, v := range *arr {
		sum += v
	}
	return sum / float64(len(*arr))
}

func updateMovingAverageDuration(arr *[]time.Duration, val time.Duration, maxLen int) time.Duration {
	*arr = append(*arr, val)
	if len(*arr) > maxLen {
		*arr = (*arr)[1:]
	}
	sum := time.Duration(0)
	for _, v := range *arr {
		sum += v
	}
	return sum / time.Duration(len(*arr))
}

// determineMode decides which mode to use based on current conditions
func (m *Manager) determineMode(motionPct float64, timeSinceInput time.Duration, avgCPU float64, avgRTT time.Duration, lossPct float64) StreamMode {
	// Can't switch if minimum duration not elapsed
	if time.Since(m.modeState.lastSwitch) < m.modeState.minModeDur {
		return m.modeState.current
	}

	// When H.264 is active, stay in H.264 mode (encoder handles static screens with tiny P-frames)
	// Only fall back if conditions are bad (high CPU/RTT/loss)
	if m.useH264 {
		if avgCPU < 75 && avgRTT < 200*time.Millisecond && lossPct < 3 {
			return ModeActiveH264
		}
		// Bad conditions - fall back to tiles
		return ModeActiveTiles
	}

	// Mode 1: Idle Tiles (default for low motion + no recent input) - only for JPEG mode
	if motionPct < 0.3 && timeSinceInput > 1*time.Second {
		return ModeIdleTiles
	}

	// Mode 2: Active Tiles (default for active use)
	return ModeActiveTiles
}

// switchMode changes the streaming mode and logs the transition
func (m *Manager) switchMode(newMode StreamMode, fps *int, quality *int, scale *float64, frameInterval *time.Duration, ticker *time.Ticker) {
	if newMode == m.modeState.current {
		return
	}

	oldMode := m.modeState.current
	m.modeState.current = newMode
	m.modeState.lastSwitch = time.Now()

	// Track switch history for flapping detection
	m.modeState.switchHistory = append(m.modeState.switchHistory, time.Now())
	if len(m.modeState.switchHistory) > 10 {
		m.modeState.switchHistory = m.modeState.switchHistory[1:]
	}

	// Apply mode-specific parameters
	switch newMode {
	case ModeIdleTiles:
		*fps = 2
		*quality = 85
		*scale = 1.0
		log.Printf("🔄 Mode switch: %s -> %s (FPS:%d Q:%d Scale:%.0f%%)", oldMode, newMode, *fps, *quality, *scale*100)
	case ModeActiveTiles:
		if runtime.GOOS == "darwin" {
			*fps = 25
			*quality = 65
			*scale = 0.75
		} else {
			*fps = 20
			*quality = 65
			*scale = 0.75
		}
		log.Printf("🔄 Mode switch: %s -> %s (FPS:%d Q:%d Scale:%.0f%%)", oldMode, newMode, *fps, *quality, *scale*100)
	case ModeActiveH264:
		*fps = 25
		*quality = 70 // Not used for H.264, but keep reasonable
		*scale = 1.0
		log.Printf("🔄 Mode switch: %s -> %s (FPS:%d H.264 active)", oldMode, newMode, *fps)
	}

	// Update ticker with new FPS
	*frameInterval = time.Duration(1000 / *fps) * time.Millisecond
	ticker.Reset(*frameInterval)
}

func (m *Manager) startScreenStreaming(ctx context.Context) {
	log.Println("🎥 Starting adaptive screen streaming...")

	// If screen capturer not initialized, try to initialize now
	if m.screenCapturer == nil {
		log.Println("⚠️  Screen capturer not initialized, attempting to initialize now...")
		var capturer *screen.Capturer
		var err error

		// Use appropriate capturer based on current desktop
		if m.isSession0 {
			log.Println("   Using GDI mode for Session 0...")
			capturer, err = screen.NewCapturerForSession0()
		} else {
			capturer, err = screen.NewCapturer()
		}

		if err != nil {
			log.Printf("❌ Failed to initialize screen capturer: %v", err)
			log.Println("   Cannot stream screen - user might need to log in first")
			return
		}
		m.screenCapturer = capturer
		log.Printf("✅ Screen capturer initialized successfully! (GDI mode: %v)", capturer.IsGDIMode())

		// Update screen dimensions
		width, height := capturer.GetResolution()
		m.mouseController = input.NewMouseController(width, height)
		log.Printf("✅ Updated screen resolution: %dx%d", width, height)
	}

	// Initialize dirty region detector for motion detection
	if m.dirtyDetector == nil {
		m.dirtyDetector = screen.NewDirtyRegionDetector(128, 128)
	}

	// Log capturer state for debugging
	if m.screenCapturer != nil {
		log.Printf("📸 Capturer state: GDI=%v, Session0=%v, resolution=%dx%d",
			m.screenCapturer.IsGDIMode(),
			m.isSession0,
			m.screenCapturer.GetBounds().Dx(), m.screenCapturer.GetBounds().Dy())
	}

	// Send monitor list to dashboard (skip in Session 0 - DXGI enumeration can crash)
	if !m.isSession0 {
		m.sendMonitorList()
	} else {
		log.Println("⚠️  Skipping monitor enumeration in Session 0 (DXGI not available)")
	}

	// Adaptive streaming parameters — lower defaults on macOS (software JPEG is CPU-heavy)
	fps := 20
	quality := 60
	scale := 0.75
	maxFPS := 20
	maxQuality := 75
	maxScale := 1.0
	if runtime.GOOS == "darwin" {
		fps = 25
		quality = 65
		scale = 0.75
		maxFPS = 30
		maxQuality = 80
		maxScale = 1.0
	}
	frameInterval := time.Duration(1000/fps) * time.Millisecond

	// Thresholds for adaptation (use controller caps if set)
	bufferHigh := uint64(1 * 1024 * 1024)    // 1MB - reduce quality sooner (was 2MB)
	bufferMedium := uint64(512 * 1024)        // 512KB - skip every 2nd frame (was 1MB)
	bufferLow := uint64(256 * 1024)           // 256KB - can increase quality (was 512KB)
	minFPS := 10
	minQuality := 30  // Lower floor for aggressive quality reduction under load
	minScale := 0.4   // Lower floor for aggressive scaling under load
	frameSkipCounter := 0 // Counter for frame skipping under buffer pressure

	// Apply controller caps if set
	if m.streamMaxFPS > 0 {
		maxFPS = m.streamMaxFPS
	}
	if m.streamMaxQuality > 0 {
		maxQuality = m.streamMaxQuality
	}
	if m.streamMaxScale > 0 {
		maxScale = m.streamMaxScale
	}

	frameCount := 0
	errorCount := 0
	droppedFrames := 0
	skippedFrames := 0 // Frames skipped due to no change (bandwidth optimization)
	bytesSent := int64(0)
	var lastFrame []byte
	var lastRGBA *image.RGBA
	lastAdaptTime := time.Now()
	lastLogTime := time.Now()
	lastFullFrame := time.Now() // For full-frame refresh cadence
	isIdle := false             // Tracked from modeState for logging
	motionPct := 0.0
	forceFullFrame := false

	// H.264 auto-enable state
	streamStart := time.Now()
	h264AutoEnabled := false
	lastH264Check := time.Now()

	ticker := time.NewTicker(frameInterval)
	defer ticker.Stop()

	// Auto-start video track if H.264 is enabled
	if m.useH264 && m.videoTrack != nil {
		m.videoTrack.Start()
		if m.videoEncoder != nil {
			m.videoEncoder.ForceKeyframe()
		}
		log.Println("🎬 H.264 auto-enabled (OpenH264 encoder available)")
	}

	// Start stats collection goroutine
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("🔥 STATS COLLECTOR PANIC: %v", r)
				log.Printf("🔥 Stack: %s", string(debug.Stack()))
			}
		}()
		m.collectStats(ctx)
	}()

	dcWaitLogged := false
	dcWaitCount := 0

	for m.isStreaming.Load() {
		// Wait for either ticker, input-triggered frame request, or context cancellation
		select {
		case <-ctx.Done():
			log.Println("🛑 Streaming stopped (context cancelled)")
			return
		case <-ticker.C:
			// Normal frame interval
		case <-m.inputFrameTrigger:
			// Input triggered - send frame immediately for visual feedback
			// Small delay to let the input take effect
			time.Sleep(10 * time.Millisecond)
		}

		if m.dataChannel == nil || m.dataChannel.ReadyState() != pionwebrtc.DataChannelStateOpen {
			dcWaitCount++
			if !dcWaitLogged {
				dcWaitLogged = true
				if m.dataChannel == nil {
					log.Println("⏳ Waiting for data channel (nil) - streaming loop running but no data channel yet")
				} else {
					log.Printf("⏳ Waiting for data channel (state: %s)", m.dataChannel.ReadyState().String())
				}
			} else if dcWaitCount%200 == 0 {
				// Log every ~10s at 20fps
				if m.dataChannel == nil {
					log.Printf("⏳ Still waiting for data channel (nil) - %d ticks", dcWaitCount)
				} else {
					log.Printf("⏳ Still waiting for data channel (state: %s) - %d ticks", m.dataChannel.ReadyState().String(), dcWaitCount)
				}
				// Also check SCTP transport state
				if m.peerConnection != nil {
					log.Printf("   PC state: %s, ICE: %s, signaling: %s",
						m.peerConnection.ConnectionState().String(),
						m.peerConnection.ICEConnectionState().String(),
						m.peerConnection.SignalingState().String())
					sctp := m.peerConnection.SCTP()
					if sctp != nil {
						transport := sctp.Transport()
						if transport != nil {
							log.Printf("   SCTP transport state: %s", transport.State().String())
						} else {
							log.Println("   SCTP transport: nil")
						}
					} else {
						log.Println("   SCTP: nil")
					}
				}
			}
			continue
		}

		// Safety: re-check dataChannel and screenCapturer after potential cleanup race
		dc := m.dataChannel
		sc := m.screenCapturer
		if dc == nil || sc == nil {
			continue
		}

		// Switch to input desktop before capture (important for Session 0/login screen)
		if m.isSession0 {
			desktop.SwitchToInputDesktop()
		}

		bufferedAmount := dc.BufferedAmount()

		// Capture RGBA for motion detection (also used for JPEG encoding - single capture)
		rgbaFrame, err := sc.CaptureRGBA()
		if err != nil {
			errorCount++

			// Check if capture needs reinitialization (DXGI errors, GDI errors, pipe errors)
			errStr := err.Error()
			isCaptureError := strings.Contains(errStr, "AcquireNextFrame") ||
				strings.Contains(errStr, "error -2") ||
				strings.Contains(errStr, "error -3") ||
				strings.Contains(errStr, "DXGI") ||
				strings.Contains(errStr, "capture failed") ||
				strings.Contains(errStr, "capture helper") ||
				strings.Contains(errStr, "pipe") ||
				strings.Contains(errStr, "timeout")

			// Log every error for the first 10, then every 10th
			if errorCount <= 10 || errorCount%10 == 0 {
				log.Printf("⚠️ Capture error #%d: %s (retryable: %v)", errorCount, errStr, isCaptureError)
			}

			if isCaptureError {
				// Rate-limit reinit attempts: first error, then exponential backoff
				// (every 5, 10, 20, 50, 100 errors — capped at every 100)
				reinitInterval := 5
				if errorCount > 50 {
					reinitInterval = 100
				} else if errorCount > 20 {
					reinitInterval = 50
				} else if errorCount > 10 {
					reinitInterval = 20
				}
				if errorCount == 1 || errorCount%reinitInterval == 0 {
					log.Printf("🔄 Reinitializing screen capturer (error #%d, next reinit in %d errors)...", errorCount, reinitInterval)
					time.Sleep(500 * time.Millisecond)

					if reinitErr := sc.Reinitialize(false); reinitErr != nil {
						log.Printf("⚠️ Reinit failed: %v", reinitErr)
					} else {
						log.Printf("✅ Screen capturer reinitialized!")
					}
				}
				time.Sleep(200 * time.Millisecond)
			} else if errorCount%50 == 1 {
				log.Printf("⚠️ Unknown capture error: %v", err)
			}
			continue
		}

		// Detect motion using dirty regions
		width, height := sc.GetResolution()
		if lastRGBA != nil {
			regions, _ := m.dirtyDetector.DetectDirtyRegions(rgbaFrame, quality)
			motionPct = m.dirtyDetector.GetChangePercentage(regions, width, height)
		}
		lastRGBA = rgbaFrame

		// Full-frame refresh cadence: every 5s or when motion > 30%
		if time.Since(lastFullFrame) > 5*time.Second || motionPct > 30 {
			forceFullFrame = true
			lastFullFrame = time.Now()
		}

		// Update moving averages for mode switching
		timeSinceInput := time.Since(m.lastInputTime)
		cpuPct := float64(0)
		if m.cpuMonitor != nil {
			cpuPct = m.cpuMonitor.GetCPUPercent()
		}
		avgCPU := updateMovingAverage(&m.cpuAvg, cpuPct, 6)
		avgRTT := updateMovingAverageDuration(&m.rttAvg, m.lastRTT, 6)

		// Determine and switch mode based on conditions
		desiredMode := m.determineMode(motionPct, timeSinceInput, avgCPU, avgRTT, m.lossPct)
		m.switchMode(desiredMode, &fps, &quality, &scale, &frameInterval, ticker)

		// Track if we're in idle mode for logging
		isIdle = (m.modeState.current == ModeIdleTiles)

		// Adaptive quality adjustment (every 500ms, skip if idle)
		if !isIdle && time.Since(lastAdaptTime) > 500*time.Millisecond {
			lastAdaptTime = time.Now()
			changed := false

			// Get CPU status
			cpuHigh := false
			cpuPct := float64(0)
			if m.cpuMonitor != nil {
				cpuHigh = m.cpuMonitor.IsHighCPU()
				cpuPct = m.cpuMonitor.GetCPUPercent()
			}

			// Check for congestion: high buffer OR high loss OR high RTT OR high CPU
			congested := bufferedAmount > bufferHigh || m.lossPct > 5 || m.lastRTT > 250*time.Millisecond || cpuHigh

			// CPU-guard: reduce quality if CPU is high
			if cpuHigh && !congested {
				log.Printf("🔥 CPU-guard triggered (%.1f%%) - reducing quality", cpuPct)
				congested = true
			}

			// Auto-switch to tiles-only if conditions are bad
			if m.useH264 {
				criticalCPU := m.cpuMonitor != nil && m.cpuMonitor.IsCriticalCPU()
				highRTT := m.lastRTT > 300*time.Millisecond
				if criticalCPU || highRTT {
					m.useH264 = false
					if criticalCPU {
						log.Println("⚠️ Auto-switch to tiles-only (CPU > 90%)")
					} else {
						log.Println("⚠️ Auto-switch to tiles-only (RTT > 300ms)")
					}
				}
			}

			if congested {
				// Network congested - SCALE-FIRST strategy for better text readability
				// Reduce scale first, then FPS, then quality
				// Use larger steps when buffer is very high (> 1.5MB)
				severeCongestion := bufferedAmount > (bufferHigh * 3 / 4)
				scaleStep := 0.1
				fpsStep := 4
				qualityStep := 5
				if severeCongestion {
					scaleStep = 0.15
					fpsStep = 6
					qualityStep = 10
				}

				if scale > minScale {
					scale -= scaleStep
					if scale < minScale {
						scale = minScale
					}
					changed = true
				} else if fps > minFPS {
					fps -= fpsStep
					if fps < minFPS {
						fps = minFPS
					}
					changed = true
				} else if quality > minQuality {
					quality -= qualityStep
					if quality < minQuality {
						quality = minQuality
					}
					changed = true
				}
			} else if bufferedAmount < bufferLow && droppedFrames == 0 && m.lossPct < 1 && m.lastRTT < 120*time.Millisecond {
				// Network clear - can increase quality (reverse order: quality, scale, fps)
				if quality < maxQuality {
					quality += 2
					if quality > maxQuality {
						quality = maxQuality
					}
					changed = true
				} else if scale < maxScale {
					scale += 0.05
					if scale > maxScale {
						scale = maxScale
					}
					changed = true
				} else if fps < maxFPS {
					fps += 2
					if fps > maxFPS {
						fps = maxFPS
					}
					changed = true
				}
			}

			if changed {
				// Update ticker with new FPS
				frameInterval = time.Duration(1000/fps) * time.Millisecond
				ticker.Reset(frameInterval)
			}
		}

		// H.264 auto-enable: after 10s of streaming, check if conditions are good
		if !m.useH264 && !h264AutoEnabled && time.Since(streamStart) > 10*time.Second && time.Since(lastH264Check) > 5*time.Second {
			lastH264Check = time.Now()
			// Check prerequisites
			if m.videoTrack != nil && m.videoEncoder != nil && m.videoEncoder.GetEncoderName() == "openh264" {
				// Check network conditions
				if m.lastRTT < 100*time.Millisecond && m.lossPct < 1.0 && avgCPU < 60 {
					log.Printf("🎬 Auto-enabling H.264 (good network conditions: RTT=%dms, loss=%.1f%%, CPU=%.0f%%)",
						m.lastRTT.Milliseconds(), m.lossPct, avgCPU)
					m.SetH264Mode(true)
					h264AutoEnabled = true
				}
			}
		}

		// H.264 auto-disable: fall back if CPU goes high
		if m.useH264 && h264AutoEnabled && avgCPU > 60 {
			log.Printf("⚠️ H.264 auto-disable: CPU too high (%.0f%%) — falling back to JPEG tiles", avgCPU)
			m.SetH264Mode(false)
			h264AutoEnabled = false
		}

		// EARLY-DROP: Drop frames before encode if buffer is filling up
		// This keeps the stream responsive instead of building up latency
		criticalBuffer := bufferedAmount > bufferHigh
		criticalCPU := m.cpuMonitor != nil && m.cpuMonitor.IsCriticalCPU()

		if criticalBuffer || criticalCPU {
			droppedFrames++
			if droppedFrames%10 == 1 {
				log.Printf("⚠️ Early-drop #%d: buffer=%dKB cpu=%.1f%%", droppedFrames, bufferedAmount/1024, cpuPct)
			}
			// Short sleep to let buffer drain instead of busy-looping
			time.Sleep(20 * time.Millisecond)
			continue
		}

		// FRAME SKIPPING under buffer pressure — prevents stalls on slow connections/CPUs
		// When buffer > 1MB, only send every 2nd frame. When > 2MB threshold approaching, every 4th.
		frameSkipCounter++
		if bufferedAmount > bufferMedium {
			skipInterval := 2
			if bufferedAmount > (bufferHigh * 3 / 4) { // > 1.5MB
				skipInterval = 4
			}
			if frameSkipCounter%skipInterval != 0 {
				skippedFrames++
				continue
			}
		}

		// H.264 mode: encode and send via video track
		if m.useH264 && m.videoTrack != nil && m.videoEncoder != nil {
			// Wrap H.264 encoding in panic recovery
			func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("❌ PANIC i H.264 encoding: %v", r)
						m.useH264 = false // Disable H.264 on panic
						errorCount++
					}
				}()

				// Encode RGBA to H.264
				nalUnits, encErr := m.videoEncoder.Encode(rgbaFrame)
				if encErr != nil {
					errorCount++
					if errorCount%100 == 1 {
						log.Printf("⚠️ H.264 encode fejl: %v", encErr)
					}
					return
				}

				if nalUnits != nil && len(nalUnits) > 0 {
					// Write H.264 NAL units to video track
					frameDuration := time.Second / time.Duration(fps)
					if writeErr := m.videoTrack.WriteFrame(nalUnits, frameDuration); writeErr != nil {
						errorCount++
						if errorCount%100 == 1 {
							log.Printf("⚠️ Video track write fejl: %v", writeErr)
						}
					} else {
						frameCount++
						bytesSent += int64(len(nalUnits))
						// Log every 100th frame to track H.264 streaming
						if frameCount%100 == 0 {
							log.Printf("🎬 H.264: %d frames sendt, %d bytes total", frameCount, bytesSent)
						}
					}
				}
			}()
			continue
		}

		// BANDWIDTH OPTIMIZATION: Skip frame if no change detected (except forced refresh)
		// This can save 50-80% bandwidth on static desktop
		if !forceFullFrame && motionPct < 0.1 && lastFrame != nil {
			// No significant change - skip encoding and sending
			skippedFrames++
			continue
		}

		// Tiles mode: encode RGBA to JPEG with scaling (reuse rgbaFrame to avoid double-capture)
		jpeg, scaledW, scaledH, encErr := sc.EncodeRGBAToJPEG(rgbaFrame, quality, scale)
		if encErr != nil {
			errorCount++
			if errorCount%50 == 1 {
				log.Printf("⚠️ JPEG encode error: %v", encErr)
			}

			if lastFrame != nil {
				m.sendFrameChunked(lastFrame)
			}
			continue
		}

		_ = scaledW
		_ = scaledH

		// POST-ENCODE buffer check: if buffer grew during encoding, drop the frame
		postEncodeBuffer := dc.BufferedAmount()
		if postEncodeBuffer > bufferHigh {
			droppedFrames++
			if droppedFrames%10 == 1 {
				log.Printf("⚠️ Post-encode drop #%d: buffer grew to %dKB during encode", droppedFrames, postEncodeBuffer/1024)
			}
			continue
		}

		lastFrame = jpeg

		// Send frame (use full frame marker if forced refresh)
		if forceFullFrame {
			if sendErr := m.sendFullFrame(jpeg); sendErr != nil {
				log.Printf("Failed to send full frame: %v", sendErr)
			} else {
				frameCount++
				bytesSent += int64(len(jpeg))
			}
			forceFullFrame = false
		} else {
			if sendErr := m.sendFrameChunked(jpeg); sendErr != nil {
				log.Printf("Failed to send frame: %v", sendErr)
			} else {
				frameCount++
				bytesSent += int64(len(jpeg))
			}
		}

		// Log every second and calculate sendBps
		if time.Since(lastLogTime) >= time.Second {
			// Calculate actual send bitrate
			elapsed := time.Since(m.lastSendTime).Seconds()
			if elapsed > 0 && m.lastBytesSent > 0 {
				bytesDelta := bytesSent - m.lastBytesSent
				m.sendBps = float64(bytesDelta*8) / elapsed
			}
			m.lastBytesSent = bytesSent
			m.lastSendTime = time.Now()

			lastLogTime = time.Now()
			avgKBPerFrame := float64(bytesSent) / float64(frameCount) / 1024
			sendMbps := m.sendBps / 1000000
			rttMs := m.lastRTT.Milliseconds()
			cpuPct := float64(0)
			if m.cpuMonitor != nil {
				cpuPct = m.cpuMonitor.GetCPUPercent()
			}

			// Get current mode string
			mode := m.modeState.current.String()

			log.Printf("📊 Mode:%s FPS:%d Q:%d Scale:%.0f%% Motion:%.1f%% RTT:%dms Loss:%.1f%% CPU:%.0f%% | %.1fKB/f %.1fMbit/s | Buf:%.1fMB | Err:%d Drop:%d Skip:%d",
				mode, fps, quality, scale*100, motionPct, rttMs, m.lossPct, cpuPct, avgKBPerFrame, sendMbps,
				float64(bufferedAmount)/1024/1024, errorCount, droppedFrames, skippedFrames)
			droppedFrames = 0 // Reset per-second counter
			skippedFrames = 0 // Reset per-second counter

			// Send stats to controller
			m.sendStats(fps, quality, scale, mode, rttMs, cpuPct)

			// Update tray status
			if m.StatusCallback != nil {
				trayStatus := fmt.Sprintf("Forbundet | %s | %.1f Mbit/s", mode, sendMbps)
				m.StatusCallback(trayStatus)
			}
		}
	}

	log.Printf("🛑 Screen streaming stopped (sent %d frames, %.1f MB total, %d errors)",
		frameCount, float64(bytesSent)/1024/1024, errorCount)
}

// Frame type markers for dirty region protocol
const (
	frameTypeFull   = 0x01 // Full frame JPEG
	frameTypeRegion = 0x02 // Dirty region update
	frameTypeChunk  = 0xFF // Chunked frame (legacy)
)

// sendFullFrame sends a complete frame with header
func (m *Manager) sendFullFrame(data []byte) error {
	// Header: [type(1), reserved(3), ...jpeg_data]
	// For full frames, we use chunking if needed
	header := []byte{frameTypeFull, 0, 0, 0}
	fullData := append(header, data...)
	return m.sendFrameChunked(fullData)
}

// sendDirtyRegion sends a single dirty region update
func (m *Manager) sendDirtyRegion(region screen.DirtyRegion) error {
	// Header: [type(1), x(2), y(2), w(2), h(2), ...jpeg_data]
	// Total header: 9 bytes
	header := make([]byte, 9)
	header[0] = frameTypeRegion
	// X position (16-bit little endian)
	header[1] = byte(region.X & 0xFF)
	header[2] = byte((region.X >> 8) & 0xFF)
	// Y position (16-bit little endian)
	header[3] = byte(region.Y & 0xFF)
	header[4] = byte((region.Y >> 8) & 0xFF)
	// Width (16-bit little endian)
	header[5] = byte(region.Width & 0xFF)
	header[6] = byte((region.Width >> 8) & 0xFF)
	// Height (16-bit little endian)
	header[7] = byte(region.Height & 0xFF)
	header[8] = byte((region.Height >> 8) & 0xFF)

	fullData := append(header, region.Data...)

	// Dirty regions are usually small, send directly if possible
	if len(fullData) <= 60000 {
		return m.dataChannel.Send(fullData)
	}
	// Otherwise chunk it
	return m.sendFrameChunked(fullData)
}

func (m *Manager) sendFrameChunked(data []byte) error {
	const maxChunkSize = 60000 // 60KB chunks (safely under 64KB limit)
	const chunkMagic = 0xFE   // Magic byte for chunked frames with frame ID (new format)

	// Increment frame ID for each new frame
	m.frameID++
	frameID := m.frameID

	// Channel selection strategy:
	// - Single-message frames (< 60KB): use video channel (unreliable = lower latency)
	// - Chunked frames (>= 60KB): use data channel (reliable) because lost chunks
	//   on unreliable channels cause entire frames to be dropped (especially over WiFi)
	reliableChannel := m.dataChannel
	if reliableChannel == nil || reliableChannel.ReadyState() != pionwebrtc.DataChannelStateOpen {
		return fmt.Errorf("no channel available for sending")
	}

	// If data fits in one message, send directly (no chunking needed)
	if len(data) <= maxChunkSize {
		// Prefer unreliable video channel for single messages (lower latency)
		sendChannel := reliableChannel
		if m.videoChannel != nil && m.videoChannel.ReadyState() == pionwebrtc.DataChannelStateOpen {
			sendChannel = m.videoChannel
		}
		return sendChannel.Send(data)
	}

	// Chunked frames MUST use reliable channel to ensure all chunks arrive
	totalChunks := (len(data) + maxChunkSize - 1) / maxChunkSize

	for i := 0; i < totalChunks; i++ {
		start := i * maxChunkSize
		end := start + maxChunkSize
		if end > len(data) {
			end = len(data)
		}

		// New header format: [magic, frame_id_hi, frame_id_lo, chunk_index, total_chunks, ...data]
		// This allows receiver to distinguish frames and handle out-of-order delivery
		chunk := make([]byte, 5+len(data[start:end]))
		chunk[0] = chunkMagic
		chunk[1] = byte(frameID >> 8)   // Frame ID high byte
		chunk[2] = byte(frameID & 0xFF) // Frame ID low byte
		chunk[3] = byte(i)              // Chunk index
		chunk[4] = byte(totalChunks)    // Total chunks
		copy(chunk[5:], data[start:end])

		if err := reliableChannel.Send(chunk); err != nil {
			return err
		}
	}

	return nil
}

// sendMonitorList sends the list of connected monitors to the dashboard
func (m *Manager) sendMonitorList() {
	monitors := screen.EnumerateDisplays()
	if len(monitors) == 0 {
		return
	}

	activeIndex := 0
	if m.screenCapturer != nil {
		activeIndex = m.screenCapturer.GetDisplayIndex()
	}

	type monitorMsg struct {
		Index   int    `json:"index"`
		Name    string `json:"name"`
		Width   int    `json:"width"`
		Height  int    `json:"height"`
		Primary bool   `json:"primary"`
		OffsetX int    `json:"offsetX"`
		OffsetY int    `json:"offsetY"`
	}

	var monList []monitorMsg
	for _, mon := range monitors {
		monList = append(monList, monitorMsg{
			Index:   mon.Index,
			Name:    mon.Name,
			Width:   mon.Width,
			Height:  mon.Height,
			Primary: mon.Primary,
			OffsetX: mon.OffsetX,
			OffsetY: mon.OffsetY,
		})
	}

	msg := map[string]interface{}{
		"type":     "monitor_list",
		"monitors": monList,
		"active":   activeIndex,
	}

	if data, err := json.Marshal(msg); err == nil {
		if m.dataChannel != nil && m.dataChannel.ReadyState() == pionwebrtc.DataChannelStateOpen {
			m.dataChannel.Send(data)
			log.Printf("📺 Sent monitor list: %d monitors (active: %d)", len(monitors), activeIndex)
		}
	}
}
