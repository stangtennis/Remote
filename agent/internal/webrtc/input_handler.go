package webrtc

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	pionwebrtc "github.com/pion/webrtc/v3"
	"github.com/stangtennis/remote-agent/internal/desktop"
	"github.com/stangtennis/remote-agent/internal/screen"
)

// handleInputEvent handles input events (mouse, keyboard) with priority
func (m *Manager) handleInputEvent(event map[string]interface{}) {
	eventType, ok := event["t"].(string)
	if !ok {
		return
	}

	switch eventType {
	case "mouse_move", "mouse_click", "mouse_scroll", "key":
		m.noteInputPriority()
		m.inputEvents.Add(1)
	}

	// Track last input time for idle detection
	m.setLastInputTime(time.Now())

	// Trigger immediate frame capture for click events (visual feedback)
	if eventType == "mouse_click" || eventType == "key" {
		select {
		case m.inputFrameTrigger <- struct{}{}:
			// Triggered
		default:
			// Already pending, skip
		}
	}

	// Handle ping/pong for RTT measurement
	if eventType == "ping" {
		ts, _ := event["ts"].(float64)
		pong := map[string]interface{}{
			"t":  "pong",
			"ts": ts,
		}
		if data, err := json.Marshal(pong); err == nil {
			// Send pong on control channel for accurate RTT
			if m.controlChannel != nil && m.controlChannel.ReadyState() == pionwebrtc.DataChannelStateOpen {
				m.controlChannel.Send(data)
			} else if m.dataChannel != nil && m.dataChannel.ReadyState() == pionwebrtc.DataChannelStateOpen {
				m.dataChannel.Send(data)
			}
		}
		return
	}

	// Switch to input desktop before handling input (only for direct input, not pipe-forwarded)
	if m.isSession0 && !(m.screenCapturer != nil && m.screenCapturer.HasInputForwarder()) {
		if err := desktop.SwitchToInputDesktop(); err != nil {
			// Rate-limit this warning (noisy on Hyper-V VMs)
			if eventType == "mouse_click" || eventType == "key" {
				log.Printf("⚠️  Failed to switch to input desktop: %v", err)
			}
		}
	}

	// Route input via pipe when in Session 0 with a pipe-based capturer
	if m.isSession0 && m.screenCapturer != nil && m.screenCapturer.HasInputForwarder() {
		var forwardErr error
		// Helper: convert relative (0.0-1.0) coordinates to absolute pixel coordinates
		resolveCoords := func(x, y float64) (int, int) {
			isRelative, _ := event["rel"].(bool)
			if isRelative {
				w, h := m.screenCapturer.GetResolution()
				return int(x * float64(w)), int(y * float64(h))
			}
			return int(x), int(y)
		}

		switch eventType {
		case "mouse_move":
			x, _ := event["x"].(float64)
			y, _ := event["y"].(float64)
			absX, absY := resolveCoords(x, y)
			forwardErr = m.screenCapturer.ForwardMouseMove(absX, absY)

		case "mouse_click":
			button, _ := event["button"].(string)
			down, _ := event["down"].(bool)
			x, _ := event["x"].(float64)
			y, _ := event["y"].(float64)
			btnCode := 0 // left
			if button == "right" {
				btnCode = 1
			} else if button == "middle" {
				btnCode = 2
			}
			downVal := 0
			if down {
				downVal = 1
			}
			absX, absY := resolveCoords(x, y)
			forwardErr = m.screenCapturer.ForwardMouseClick(btnCode, downVal, absX, absY)

		case "mouse_scroll":
			delta, _ := event["delta"].(float64)
			forwardErr = m.screenCapturer.ForwardScroll(int(delta), 0, 0)

		case "key":
			code, _ := event["code"].(string)
			down, _ := event["down"].(bool)
			ctrl, _ := event["ctrl"].(bool)
			shift, _ := event["shift"].(bool)
			alt, _ := event["alt"].(bool)
			meta, _ := event["meta"].(bool)

			// For type_text: send each character via ForwardUnicodeChar.
			// The helper's handleUnicode uses charToVK() for ASCII chars
			// (VK codes work with admin windows) and falls back to
			// KEYEVENTF_UNICODE for non-ASCII chars.
			if charStr, ok := event["char"].(string); ok && charStr != "" && down {
				for _, ch := range charStr {
					if err := m.screenCapturer.ForwardUnicodeChar(ch); err != nil {
						forwardErr = err
						break
					}
				}
			} else {
				forwardErr = m.screenCapturer.ForwardKeyEvent(code, down, ctrl, shift, alt, meta)
			}
		}
		if forwardErr != nil {
			m.inputForwardErrors.Add(1)
			if eventType != "mouse_move" {
				log.Printf("⚠️ Session0 input forward failed (%s): %v", eventType, forwardErr)
			}
			m.sendInputStatus(eventType, "forward_error", forwardErr.Error(), true)
		} else {
			m.inputForwarded.Add(1)
			force := eventType == "mouse_click" || eventType == "key"
			m.sendInputStatus(eventType, "forwarded", "", force)
		}
		return
	}

	// Handle input events (direct — not Session 0 or no pipe capturer)
	switch eventType {
	case "mouse_move":
		x, _ := event["x"].(float64)
		y, _ := event["y"].(float64)
		isRelative, _ := event["rel"].(bool)
		if isRelative {
			m.mouseController.MoveRelative(x, y)
		} else {
			m.mouseController.Move(x, y)
		}

	case "mouse_click":
		button, _ := event["button"].(string)
		down, _ := event["down"].(bool)
		x, hasX := event["x"].(float64)
		y, hasY := event["y"].(float64)
		isRelative, _ := event["rel"].(bool)
		if hasX && hasY {
			if isRelative {
				m.mouseController.MoveRelative(x, y)
			} else {
				m.mouseController.Move(x, y)
			}
		}
		m.mouseController.Click(button, down)

	case "mouse_scroll":
		delta, _ := event["delta"].(float64)
		m.mouseController.Scroll(int(delta))

	case "key":
		code, _ := event["code"].(string)
		down, _ := event["down"].(bool)
		ctrl, _ := event["ctrl"].(bool)
		shift, _ := event["shift"].(bool)
		alt, _ := event["alt"].(bool)
		meta, _ := event["meta"].(bool)

		// If "char" field is present, use Unicode input (bypasses keyboard layout)
		if charStr, ok := event["char"].(string); ok && charStr != "" && down {
			for _, ch := range charStr {
				if err := m.keyController.SendUnicodeChar(ch); err != nil {
					// Fallback to key code approach
					m.keyController.SendKeyWithModifiers(code, down, ctrl, shift, alt, meta)
				}
			}
		} else if m.keyController != nil {
			m.keyController.SendKeyWithModifiers(code, down, ctrl, shift, alt, meta)
		}
	}
}

func (m *Manager) sendInputStatus(eventType, route, errMsg string, force bool) {
	now := time.Now()
	last := time.Unix(0, m.lastInputStatusAt.Load())
	if !force && now.Sub(last) < 2*time.Second {
		return
	}
	m.lastInputStatusAt.Store(now.UnixNano())

	status := map[string]interface{}{
		"type":      "input_status",
		"event":     eventType,
		"route":     route,
		"session0":  m.isSession0,
		"forwarder": m.screenCapturer != nil && m.screenCapturer.HasInputForwarder(),
		"events":    m.inputEvents.Load(),
		"forwarded": m.inputForwarded.Load(),
		"errors":    m.inputForwardErrors.Load(),
		"error":     errMsg,
	}
	data, jsonErr := json.Marshal(status)
	if jsonErr != nil {
		return
	}
	if m.controlChannel != nil && m.controlChannel.ReadyState() == pionwebrtc.DataChannelStateOpen {
		_ = m.controlChannel.Send(data)
		return
	}
	if m.dataChannel != nil && m.dataChannel.ReadyState() == pionwebrtc.DataChannelStateOpen {
		_ = m.dataChannel.Send(data)
	}
}

func (m *Manager) noteInputPriority() {
	if m.isSession0 && m.screenCapturer != nil && m.screenCapturer.HasInputForwarder() {
		m.inputPriorityUntil.Store(time.Now().Add(80 * time.Millisecond).UnixNano())
	}
}

func (m *Manager) inputPriorityActive() bool {
	deadline := m.inputPriorityUntil.Load()
	return deadline > 0 && time.Now().UnixNano() < deadline
}

// handleControlEvent handles control events from the dashboard data channel
func (m *Manager) handleControlEvent(event map[string]interface{}) {
	getMsgType := func(ev map[string]interface{}) string {
		if v, ok := ev["type"].(string); ok && v != "" {
			return v
		}
		if v, ok := ev["t"].(string); ok && v != "" {
			return v
		}
		return ""
	}
	sendCodecStatus := func(requested string, active string, accepted bool, reason string) {
		status := map[string]interface{}{
			"type":      "codec_status",
			"requested": requested,
			"active":    active,
			"accepted":  accepted,
			"reason":    reason,
		}
		if data, err := json.Marshal(status); err == nil {
			if m.controlChannel != nil && m.controlChannel.ReadyState() == pionwebrtc.DataChannelStateOpen {
				_ = m.controlChannel.Send(data)
				return
			}
			if m.dataChannel != nil && m.dataChannel.ReadyState() == pionwebrtc.DataChannelStateOpen {
				_ = m.dataChannel.Send(data)
			}
		}
	}

	// Handle streaming mode changes
	if msgType := getMsgType(event); msgType == "set_mode" {
		if mode, ok := event["mode"].(string); ok {
			log.Printf("🎛️ Received set_mode request: mode=%s bitrate=%v", mode, event["bitrate"])
			switch mode {
			case "h264":
				if m.SetH264Mode(true) {
					log.Println("🎬 Switched to H.264 mode")
					sendCodecStatus("h264", "h264", true, "")
				} else {
					log.Println("🎬 H.264 request ignored; staying on JPEG tiles")
					sendCodecStatus("h264", "jpeg", false, "h264_unavailable_for_session0_gdi")
				}
			case "tiles":
				m.SetH264Mode(false)
				log.Println("🎬 Switched to tiles-only mode")
				sendCodecStatus("tiles", "jpeg", true, "")
			case "hybrid":
				if m.SetH264Mode(true) {
					log.Println("🎬 Switched to hybrid mode (H.264 + tiles)")
					sendCodecStatus("hybrid", "h264", true, "")
				} else {
					log.Println("🎬 Hybrid/H.264 request ignored; staying on JPEG tiles")
					sendCodecStatus("hybrid", "jpeg", false, "h264_unavailable_for_session0_gdi")
				}
			}
		}
		if bitrate, ok := event["bitrate"].(float64); ok && bitrate > 0 {
			kbps := int(bitrate)
			if kbps > 50000 {
				kbps = 50000
			}
			m.SetVideoBitrate(kbps)
		}
		return
	}

	// Handle switch_monitor
	if msgType := getMsgType(event); msgType == "switch_monitor" {
		m.handleSwitchMonitor(event)
		return
	}

	// Clipboard messages (controller -> agent)
	if msgType := getMsgType(event); msgType != "" {
		switch msgType {
		case "clipboard_text":
			if content, ok := event["content"].(string); ok {
				m.handleClipboardText(content)
			}
			return
		case "clipboard_image":
			if contentB64, ok := event["content"].(string); ok {
				m.handleClipboardImage(contentB64)
			}
			return
		case "stream_pause":
			// Controller signalled user-idle — stop sending frames until
			// stream_resume. Connection + heartbeat stay alive.
			if !m.pausedByController.Swap(true) {
				log.Println("⏸️  Streaming paused by controller (idle)")
			}
			return
		case "stream_resume":
			if m.pausedByController.Swap(false) {
				log.Println("▶️  Streaming resumed by controller")
				// Trigger an immediate frame so the viewer sees fresh
				// content rather than waiting for the next tick.
				select {
				case m.inputFrameTrigger <- struct{}{}:
				default:
				}
			}
			return
		case "release_all_keys":
			m.releaseAllKeys()
			return
		case "remote_login":
			m.handleRemoteLogin(event)
			return
		}
	}

	// Handle force update from dashboard
	if msgType, ok := event["type"].(string); ok && msgType == "force_update" {
		log.Println("🔄 Force update requested from dashboard")
		go m.handleForceUpdate()
		return
	}

	// Check if this is a file transfer or file browser message
	if msgType, ok := event["type"].(string); ok {
		switch msgType {
		case "file_transfer_start", "file_chunk", "file_transfer_complete", "file_transfer_error":
			if m.fileTransferHandler != nil {
				data, _ := json.Marshal(event)
				if err := m.fileTransferHandler.HandleIncomingData(data); err != nil {
					log.Printf("File transfer error: %v", err)
				}
			}
			return

		case "dir_list":
			// Handle directory listing request
			path, _ := event["path"].(string)
			m.handleDirListRequest(path)
			return

		case "drives_list":
			// Handle drives listing request
			m.handleDrivesListRequest()
			return

		case "file_request":
			// Handle file download request from controller
			remotePath, _ := event["remotePath"].(string)
			m.handleFileRequest(remotePath)
			return
		}
	}

	// Handle input events
	eventType, ok := event["t"].(string)
	if !ok {
		return
	}

	// Handle ping/pong for RTT measurement
	if eventType == "ping" {
		// Respond with pong immediately
		ts, _ := event["ts"].(float64)
		pong := map[string]interface{}{
			"t":  "pong",
			"ts": ts, // Echo back the timestamp
		}
		if data, err := json.Marshal(pong); err == nil {
			if m.dataChannel != nil && m.dataChannel.ReadyState() == pionwebrtc.DataChannelStateOpen {
				m.dataChannel.Send(data)
			}
		}
		return
	}

	// Track last input time for idle detection
	m.setLastInputTime(time.Now())

	// Switch to input desktop before handling input (required for Session 0 / login screen)
	if m.isSession0 {
		if err := desktop.SwitchToInputDesktop(); err != nil {
			log.Printf("⚠️  Failed to switch to input desktop: %v", err)
		}
	}

	switch eventType {
	case "mouse_move":
		x, _ := event["x"].(float64)
		y, _ := event["y"].(float64)
		isRelative, _ := event["rel"].(bool)

		// Use rel flag to determine coordinate type
		if isRelative {
			if err := m.mouseController.MoveRelative(x, y); err != nil {
				log.Printf("❌ Mouse move error: %v", err)
			}
		} else {
			if err := m.mouseController.Move(x, y); err != nil {
				log.Printf("❌ Mouse move error: %v", err)
			}
		}

	case "mouse_click":
		button, _ := event["button"].(string)
		down, _ := event["down"].(bool)
		x, hasX := event["x"].(float64)
		y, hasY := event["y"].(float64)
		isRelative, _ := event["rel"].(bool)

		// Move mouse to click position if coordinates are provided
		if hasX && hasY {
			if isRelative {
				m.mouseController.MoveRelative(x, y)
			} else {
				m.mouseController.Move(x, y)
			}
		}

		if err := m.mouseController.Click(button, down); err != nil {
			log.Printf("❌ Mouse click error: %v", err)
		}

	case "mouse_scroll":
		delta, _ := event["delta"].(float64)
		if err := m.mouseController.Scroll(int(delta)); err != nil {
			log.Printf("Mouse scroll error: %v", err)
		}

	case "key":
		code, _ := event["code"].(string)
		down, _ := event["down"].(bool)
		ctrl, _ := event["ctrl"].(bool)
		shift, _ := event["shift"].(bool)
		alt, _ := event["alt"].(bool)
		meta, _ := event["meta"].(bool)

		// If "char" field is present, use Unicode input (bypasses keyboard layout)
		if charStr, ok := event["char"].(string); ok && charStr != "" && down {
			for _, ch := range charStr {
				if err := m.keyController.SendUnicodeChar(ch); err != nil {
					// Fallback to key code approach
					if err2 := m.keyController.SendKeyWithModifiers(code, down, ctrl, shift, alt, meta); err2 != nil {
						log.Printf("Key event error: %v", err2)
					}
				}
			}
		} else {
			// Send key with modifiers
			if err := m.keyController.SendKeyWithModifiers(code, down, ctrl, shift, alt, meta); err != nil {
				log.Printf("Key event error: %v", err)
			}
		}
	}
}

func (m *Manager) handleRemoteLogin(event map[string]interface{}) {
	username, _ := event["username"].(string)
	password, _ := event["password"].(string)
	domain, _ := event["domain"].(string)
	sendUsername := true
	if v, ok := event["send_username"].(bool); ok {
		sendUsername = v
	}

	username = strings.TrimSpace(username)
	domain = strings.TrimSpace(domain)

	if username == "" && password == "" {
		log.Println("⚠️ remote_login ignored: no username/password provided")
		return
	}

	hasForwarder := m.isSession0 && m.screenCapturer != nil && m.screenCapturer.HasInputForwarder()
	log.Printf("🔐 Remote login requested (username=%t, domain=%t, password=%t, session0=%t, forwarder=%t)", username != "", domain != "", password != "", m.isSession0, hasForwarder)

	if m.isSession0 && !hasForwarder {
		if err := desktop.SwitchToInputDesktop(); err != nil {
			log.Printf("⚠️ remote_login: failed to switch to input desktop: %v", err)
		}
	}

	loginUser := username
	if username != "" && domain != "" {
		loginUser = domain + `\` + username
	}

	if sendUsername && loginUser != "" {
		// Press Escape first to reset Windows login screen to a clean state.
		// This dismisses any pre-selected user and ensures cursor starts in
		// the username field. Without this, if Windows already shows a user
		// with cursor in the password field, the username would be typed into
		// the password field instead.
		if err := m.remoteTapKey("Escape"); err != nil {
			log.Printf("⚠️ remote_login escape key failed: %v", err)
		}
		time.Sleep(500 * time.Millisecond)

		if err := m.remoteTypeText(loginUser); err != nil {
			log.Printf("⚠️ remote_login username typing failed: %v", err)
		}
		if err := m.remoteTapKey("Tab"); err != nil {
			log.Printf("⚠️ remote_login tab key failed: %v", err)
		}
		time.Sleep(100 * time.Millisecond)
	}

	if password != "" {
		if err := m.remoteTypeText(password); err != nil {
			log.Printf("⚠️ remote_login password typing failed: %v", err)
		}
	}

	if err := m.remoteTapKey("Enter"); err != nil {
		log.Printf("⚠️ remote_login enter key failed: %v", err)
	}

	select {
	case m.inputFrameTrigger <- struct{}{}:
	default:
	}
}

func (m *Manager) releaseAllKeys() {
	if m.keyController != nil {
		m.keyController.ClearModifiers()
	}

	if m.isSession0 && m.screenCapturer != nil && m.screenCapturer.HasInputForwarder() {
		for _, code := range []string{
			"ControlLeft", "ControlRight",
			"ShiftLeft", "ShiftRight",
			"AltLeft", "AltRight",
			"MetaLeft", "MetaRight",
		} {
			if err := m.screenCapturer.ForwardKeyEvent(code, false, false, false, false, false); err != nil {
				log.Printf("⚠️ release_all_keys: forward %s keyup failed: %v", code, err)
			}
		}
	}
	log.Println("⌨️ Released remote modifier keys")
}

func (m *Manager) remoteTypeText(text string) error {
	if text == "" {
		return nil
	}

	hasForwarder := m.isSession0 && m.screenCapturer != nil && m.screenCapturer.HasInputForwarder()
	for _, ch := range text {
		if hasForwarder {
			if err := m.screenCapturer.ForwardUnicodeChar(ch); err != nil {
				return fmt.Errorf("forward unicode char failed: %w", err)
			}
		} else {
			if m.keyController == nil {
				return fmt.Errorf("keyboard controller unavailable")
			}
			if err := m.keyController.SendUnicodeChar(ch); err != nil {
				return fmt.Errorf("send unicode char failed: %w", err)
			}
		}
		time.Sleep(8 * time.Millisecond)
	}
	return nil
}

func (m *Manager) remoteTapKey(code string) error {
	hasForwarder := m.isSession0 && m.screenCapturer != nil && m.screenCapturer.HasInputForwarder()
	if hasForwarder {
		if err := m.screenCapturer.ForwardKeyEvent(code, true, false, false, false, false); err != nil {
			return fmt.Errorf("forward key down failed: %w", err)
		}
		time.Sleep(20 * time.Millisecond)
		if err := m.screenCapturer.ForwardKeyEvent(code, false, false, false, false, false); err != nil {
			return fmt.Errorf("forward key up failed: %w", err)
		}
		return nil
	}

	if m.keyController == nil {
		return fmt.Errorf("keyboard controller unavailable")
	}
	if err := m.keyController.SendKeyWithModifiers(code, true, false, false, false, false); err != nil {
		return fmt.Errorf("key down failed: %w", err)
	}
	time.Sleep(20 * time.Millisecond)
	if err := m.keyController.SendKeyWithModifiers(code, false, false, false, false, false); err != nil {
		return fmt.Errorf("key up failed: %w", err)
	}
	return nil
}

// handleSwitchMonitor handles monitor switching requests
func (m *Manager) handleSwitchMonitor(event map[string]interface{}) {
	indexF, ok := event["index"].(float64)
	if !ok {
		log.Println("⚠️ switch_monitor: missing index")
		return
	}
	index := int(indexF)
	if index < 0 || index > 15 {
		log.Printf("⚠️ switch_monitor: invalid index %d (must be 0-15)", index)
		return
	}
	log.Printf("🖥️ Switching to monitor %d...", index)

	if m.screenCapturer != nil {
		if err := m.screenCapturer.SwitchDisplay(index); err != nil {
			log.Printf("❌ Failed to switch display: %v", err)
			return
		}

		// Update mouse controller with new resolution + offset
		width, height := m.screenCapturer.GetResolution()
		monitors := screen.EnumerateDisplays()
		var offsetX, offsetY int
		for _, mon := range monitors {
			if mon.Index == index {
				offsetX = mon.OffsetX
				offsetY = mon.OffsetY
				break
			}
		}

		if m.mouseController != nil {
			m.mouseController.SetResolution(width, height)
			m.mouseController.SetMonitorOffset(offsetX, offsetY)
		}

		// Reset dirty region detector
		if m.dirtyDetector != nil {
			m.dirtyDetector = nil // Will be recreated on next frame
		}

		// Send confirmation
		confirmation := map[string]interface{}{
			"type":   "monitor_switched",
			"index":  index,
			"width":  width,
			"height": height,
		}
		if data, err := json.Marshal(confirmation); err == nil {
			if m.dataChannel != nil && m.dataChannel.ReadyState() == pionwebrtc.DataChannelStateOpen {
				m.dataChannel.Send(data)
			}
		}
		log.Printf("✅ Switched to monitor %d: %dx%d (offset: %d,%d)", index, width, height, offsetX, offsetY)
		logLowDisplayResolution("monitor switch", width, height)
	}
}

// handleSetStreamParams handles stream parameter updates from controller
func (m *Manager) handleSetStreamParams(event map[string]interface{}) {
	if maxQuality, ok := event["max_quality"].(float64); ok {
		q := int(maxQuality)
		if q < 10 {
			q = 10
		} else if q > 100 {
			q = 100
		}
		m.streamMaxQuality = q
	}
	if maxFPS, ok := event["max_fps"].(float64); ok {
		fps := int(maxFPS)
		if fps < 1 {
			fps = 1
		} else if fps > 60 {
			fps = 60
		}
		m.streamMaxFPS = fps
	}
	if maxScale, ok := event["max_scale"].(float64); ok {
		if maxScale >= 0.25 && maxScale <= 1.0 {
			m.streamMaxScale = maxScale
		}
	}
	if h264Kbps, ok := event["h264_bitrate_kbps"].(float64); ok {
		kbps := int(h264Kbps)
		if kbps < 100 {
			kbps = 100
		} else if kbps > 50000 {
			kbps = 50000
		}
		m.streamH264Kbps = kbps
	}
	log.Printf("📊 Stream params updated: Q=%d%% FPS=%d Scale=%.0f%% H264=%dkbps",
		m.streamMaxQuality, m.streamMaxFPS, m.streamMaxScale*100, m.streamH264Kbps)
}
