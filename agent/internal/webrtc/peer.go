package webrtc

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pion/webrtc/v3"
	"github.com/stangtennis/remote-agent/internal/clipboard"
	"github.com/stangtennis/remote-agent/internal/config"
	"github.com/stangtennis/remote-agent/internal/desktop"
	"github.com/stangtennis/remote-agent/internal/device"
	"github.com/stangtennis/remote-agent/internal/filetransfer"
	"github.com/stangtennis/remote-agent/internal/input"
	"github.com/stangtennis/remote-agent/internal/monitor"
	"github.com/stangtennis/remote-agent/internal/screen"
	"github.com/stangtennis/remote-agent/internal/video"
	"github.com/stangtennis/remote-agent/internal/video/encoder"
)

type Manager struct {
	cfg                 *config.Config
	device              *device.Device
	peerConnection      *webrtc.PeerConnection
	dataChannel         *webrtc.DataChannel
	controlChannel      *webrtc.DataChannel // Separate channel for input (low latency)
	screenCapturer      *screen.Capturer
	dirtyDetector       *screen.DirtyRegionDetector // For bandwidth optimization
	mouseController     *input.MouseController
	keyController       *input.KeyboardController
	fileTransferHandler *filetransfer.Handler
	clipboardMonitor    *clipboard.Monitor
	clipboardReceiver   *clipboard.Receiver
	sessionID           string
	isStreaming         bool
	isSession0          bool // Running in Session 0 (before user login)
	currentDesktop      desktop.DesktopType
	pendingCandidates   []*webrtc.ICECandidate // Buffer ICE candidates until answer is sent
	answerSent          bool                   // Flag to track if answer has been sent

	// RTT measurement
	lastRTT       time.Duration // Last measured round-trip time
	lastInputTime time.Time     // Last input event time (for idle detection)

	// Stats tracking
	lastPacketsSent uint32  // For loss calculation
	lossPct         float64 // Current packet loss percentage
	sendBps         float64 // Current send bitrate (bits per second)
	lastBytesSent   int64   // For sendBps calculation
	lastSendTime    time.Time

	// Video encoding (H.264)
	videoTrack   *video.Track
	videoEncoder *encoder.Manager
	useH264      bool // Whether to use H.264 video track

	// System monitoring
	cpuMonitor *monitor.CPUMonitor

	// Input-triggered frame refresh
	inputFrameTrigger chan struct{}
}

func New(cfg *config.Config, dev *device.Device) (*Manager, error) {
	// Check if we're in Session 0 (login screen / no user desktop)
	isSession0 := false
	currentDesktopType := desktop.DesktopDefault

	desktopName, err := desktop.GetInputDesktop()
	if err != nil {
		log.Printf("⚠️  Cannot detect desktop: %v", err)
		log.Println("   Assuming Session 0 (pre-login) mode")
		isSession0 = true
	} else {
		currentDesktopType = desktop.GetDesktopType(desktopName)
		if currentDesktopType == desktop.DesktopWinlogon {
			log.Println("🔒 Running on login screen (Winlogon desktop)")
			isSession0 = true
		} else {
			log.Printf("🖥️  Running on desktop: %s", desktopName)
		}
	}

	// Try to initialize screen capturer
	// For Session 0, use GDI mode which works better
	var capturer *screen.Capturer
	if isSession0 {
		capturer, err = screen.NewCapturerForSession0()
	} else {
		capturer, err = screen.NewCapturer()
	}

	if err != nil {
		log.Printf("⚠️  Screen capturer not available: %v", err)
		log.Println("   Screen capture will be initialized on first connection")
	}

	// Get screen dimensions for input mapping (default to 1920x1080 if capturer not available)
	width, height := 1920, 1080
	if capturer != nil {
		width, height = capturer.GetResolution()
		log.Printf("✅ Screen capturer initialized: %dx%d (Session0: %v)", width, height, isSession0)
	}

	// Set up file transfer handler
	// Use Downloads folder as default (or temp folder for Session 0)
	var downloadDir string
	if isSession0 {
		downloadDir = filepath.Join(os.TempDir(), "RemoteDesktop")
	} else {
		homeDir, _ := os.UserHomeDir()
		downloadDir = filepath.Join(homeDir, "Downloads", "RemoteDesktop")
	}
	fileTransferHandler := filetransfer.NewHandler(downloadDir)
	log.Printf("✅ File transfer handler initialized: %s", downloadDir)

	// Initialize video encoder
	videoEncoder := encoder.NewManager()
	if err := videoEncoder.Init(encoder.Config{
		Width:            width,
		Height:           height,
		Bitrate:          2000, // 2 Mbps default
		Framerate:        30,
		KeyframeInterval: 90, // Keyframe every 3 seconds at 30fps
	}); err != nil {
		log.Printf("⚠️ Video encoder init failed: %v (H.264 disabled)", err)
	}

	// Initialize CPU monitor
	cpuMon := monitor.NewCPUMonitor()
	cpuMon.Start()

	mgr := &Manager{
		cfg:                 cfg,
		device:              dev,
		screenCapturer:      capturer,
		mouseController:     input.NewMouseController(width, height),
		keyController:       input.NewKeyboardController(),
		fileTransferHandler: fileTransferHandler,
		isSession0:          isSession0,
		currentDesktop:      currentDesktopType,
		videoEncoder:        videoEncoder,
		useH264:             false, // Disabled by default, enable via signaling
		cpuMonitor:          cpuMon,
		inputFrameTrigger:   make(chan struct{}, 1), // Buffered to avoid blocking
	}

	log.Printf("🎬 Video encoder: %s", videoEncoder.GetEncoderName())

	// Start desktop monitoring to handle login/logout transitions
	go mgr.monitorDesktopChanges()

	return mgr, nil
}

// monitorDesktopChanges watches for desktop switches (login screen <-> user desktop)
func (m *Manager) monitorDesktopChanges() {
	log.Println("👁️  Starting desktop change monitor...")

	desktop.MonitorDesktopSwitch(func(dt desktop.DesktopType) {
		if dt == m.currentDesktop {
			return // No change
		}

		oldDesktop := m.currentDesktop
		m.currentDesktop = dt

		switch dt {
		case desktop.DesktopWinlogon:
			log.Println("🔒 Desktop switched to login screen")
			m.isSession0 = true
			// Reinitialize capturer for login screen
			if m.screenCapturer != nil {
				log.Println("🔄 Reinitializing screen capturer for login screen...")
				if err := m.screenCapturer.Reinitialize(true); err != nil {
					log.Printf("❌ Failed to reinitialize capturer: %v", err)
				}
			}
		case desktop.DesktopDefault:
			log.Println("🔓 Desktop switched to user desktop")
			m.isSession0 = false
			// Reinitialize capturer for user desktop (prefer DXGI)
			if m.screenCapturer != nil {
				log.Println("🔄 Reinitializing screen capturer for user desktop...")
				if err := m.screenCapturer.Reinitialize(false); err != nil {
					log.Printf("❌ Failed to reinitialize capturer: %v", err)
				}
				// Update mouse controller with new resolution
				width, height := m.screenCapturer.GetResolution()
				m.mouseController = input.NewMouseController(width, height)
				log.Printf("✅ Updated screen resolution: %dx%d", width, height)
			}
		default:
			log.Printf("⚠️  Desktop switched to unknown type: %d (was: %d)", dt, oldDesktop)
		}
	})
}

// SetH264Mode enables or disables H.264 video track mode
func (m *Manager) SetH264Mode(enabled bool) {
	m.useH264 = enabled
	if enabled {
		// Start video track if available
		if m.videoTrack != nil {
			m.videoTrack.Start()
		}
		log.Println("🎬 H.264 mode enabled")
	} else {
		// Stop video track
		if m.videoTrack != nil {
			m.videoTrack.Stop()
		}
		log.Println("🎬 H.264 mode disabled (using JPEG tiles)")
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

func (m *Manager) CreatePeerConnection(iceServers []webrtc.ICEServer) error {
	// Close any existing connection first
	if m.peerConnection != nil {
		log.Println("🔄 Closing existing peer connection for new connection...")
		m.cleanupConnection("New connection requested")
	}

	config := webrtc.Configuration{
		ICEServers: iceServers,
	}

	pc, err := webrtc.NewPeerConnection(config)
	if err != nil {
		return fmt.Errorf("failed to create peer connection: %w", err)
	}

	// Reset state for new connection
	m.answerSent = false
	m.pendingCandidates = nil
	m.peerConnection = pc

	// Add video track if H.264 mode is enabled
	if m.useH264 {
		videoTrack, err := video.NewTrack()
		if err != nil {
			log.Printf("⚠️ Failed to create video track: %v", err)
		} else {
			m.videoTrack = videoTrack
			if _, err := pc.AddTrack(videoTrack.GetTrack()); err != nil {
				log.Printf("⚠️ Failed to add video track: %v", err)
			} else {
				log.Println("🎬 Video track added to peer connection")
			}
		}
	}

	// Set up ICE connection state handler (more granular than connection state)
	pc.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		log.Printf("🧊 ICE connection state: %s", state.String())
		if state == webrtc.ICEConnectionStateConnected {
			log.Println("🧊 ICE layer connected!")
		}
	})

	// Set up connection state handler
	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		log.Printf("🔄 Connection state changed: %s", state.String())

		switch state {
		case webrtc.PeerConnectionStateConnected:
			log.Println("✅ WebRTC CONNECTED! Starting screen streaming...")
			m.isStreaming = true
			// Hide local cursor during remote session
			if m.mouseController != nil {
				m.mouseController.HideCursor()
			}
			go m.startScreenStreaming()
		case webrtc.PeerConnectionStateDisconnected:
			log.Println("⚠️  WebRTC DISCONNECTED")
			// Restore local cursor
			if m.mouseController != nil {
				m.mouseController.ShowCursor()
			}
			m.cleanupConnection("Disconnected")
		case webrtc.PeerConnectionStateFailed:
			log.Println("❌ WebRTC CONNECTION FAILED")
			// Restore local cursor
			if m.mouseController != nil {
				m.mouseController.ShowCursor()
			}
			m.cleanupConnection("Failed")
		case webrtc.PeerConnectionStateClosed:
			log.Println("🔒 WebRTC CONNECTION CLOSED")
			// Restore local cursor
			if m.mouseController != nil {
				m.mouseController.ShowCursor()
			}
			m.cleanupConnection("Closed")
		}
	})

	// Set up ICE candidate handler
	pc.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate != nil {
			log.Printf("📤 Generated ICE candidate: %s", candidate.Typ.String())
			// Buffer candidates until answer is sent (for web dashboard compatibility)
			if !m.answerSent {
				m.pendingCandidates = append(m.pendingCandidates, candidate)
				log.Printf("   Buffered (waiting for answer to be sent first)")
			} else {
				m.sendICECandidate(candidate)
			}
		}
	})

	// Set up data channel handler
	pc.OnDataChannel(func(dc *webrtc.DataChannel) {
		log.Printf("📡 Data channel opened: %s", dc.Label())

		// Route to appropriate handler based on channel label
		if dc.Label() == "control" {
			log.Println("🎮 Control channel ready (low-latency input)")
			m.controlChannel = dc
			m.setupControlChannelHandlers(dc)
			// Also use control channel for streaming if no separate data channel
			// Dashboard only creates one "control" channel for both input and frames
			if m.dataChannel == nil {
				m.dataChannel = dc
				log.Println("📺 Using control channel for frame streaming")
			}
		} else {
			m.dataChannel = dc
			m.setupDataChannelHandlers(dc)
		}
	})

	return nil
}

func (m *Manager) setupDataChannelHandlers(dc *webrtc.DataChannel) {
	dc.OnOpen(func() {
		log.Println("✅ DATA CHANNEL READY - Controller can now receive frames!")

		// Set up file transfer send callback
		if m.fileTransferHandler != nil {
			m.fileTransferHandler.SetSendDataCallback(func(data []byte) error {
				if m.dataChannel != nil && m.dataChannel.ReadyState() == webrtc.DataChannelStateOpen {
					return m.dataChannel.Send(data)
				}
				return fmt.Errorf("data channel not ready")
			})
		}

		// Start clipboard monitoring
		log.Println("📋 Starting clipboard monitoring...")
		m.startClipboardMonitoring()
	})

	dc.OnClose(func() {
		log.Println("❌ DATA CHANNEL CLOSED")

		// Stop clipboard monitoring
		if m.clipboardMonitor != nil {
			m.clipboardMonitor.Stop()
		}
	})

	dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		// Handle control events from dashboard
		var event map[string]interface{}
		if err := json.Unmarshal(msg.Data, &event); err != nil {
			log.Printf("Failed to parse control event: %v", err)
			return
		}

		m.handleControlEvent(event)
	})
}

// setupControlChannelHandlers sets up the low-latency control channel for input
func (m *Manager) setupControlChannelHandlers(dc *webrtc.DataChannel) {
	dc.OnOpen(func() {
		log.Println("🎮 CONTROL CHANNEL READY - Low-latency input enabled!")
	})

	dc.OnClose(func() {
		log.Println("🎮 Control channel closed")
	})

	dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		// Handle input events with priority
		var event map[string]interface{}
		if err := json.Unmarshal(msg.Data, &event); err != nil {
			return
		}

		// Only handle input events on control channel
		m.handleInputEvent(event)
	})
}

// handleInputEvent handles input events (mouse, keyboard) with priority
func (m *Manager) handleInputEvent(event map[string]interface{}) {
	eventType, ok := event["t"].(string)
	if !ok {
		return
	}

	// Track last input time for idle detection
	m.lastInputTime = time.Now()

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
			if m.controlChannel != nil && m.controlChannel.ReadyState() == webrtc.DataChannelStateOpen {
				m.controlChannel.Send(data)
			} else if m.dataChannel != nil && m.dataChannel.ReadyState() == webrtc.DataChannelStateOpen {
				m.dataChannel.Send(data)
			}
		}
		return
	}

	// Switch to input desktop before handling input
	if m.isSession0 {
		if err := desktop.SwitchToInputDesktop(); err != nil {
			log.Printf("⚠️  Failed to switch to input desktop: %v", err)
		}
	}

	// Handle input events
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
		if m.keyController != nil {
			m.keyController.SendKeyWithModifiers(code, down, ctrl, shift, alt)
		}
	}
}

func (m *Manager) handleControlEvent(event map[string]interface{}) {
	// Handle streaming mode changes
	if msgType, ok := event["type"].(string); ok && msgType == "set_mode" {
		if mode, ok := event["mode"].(string); ok {
			switch mode {
			case "h264":
				m.SetH264Mode(true)
				log.Println("🎬 Switched to H.264 mode")
			case "tiles":
				m.SetH264Mode(false)
				log.Println("🎬 Switched to tiles-only mode")
			case "hybrid":
				m.SetH264Mode(true)
				log.Println("🎬 Switched to hybrid mode (H.264 + tiles)")
			}
		}
		if bitrate, ok := event["bitrate"].(float64); ok && bitrate > 0 {
			m.SetVideoBitrate(int(bitrate))
		}
		return
	}

	// Clipboard messages (controller -> agent)
	if msgType, ok := event["type"].(string); ok {
		switch msgType {
		case "clipboard_text":
			if content, ok := event["content"].(string); ok {
				if m.clipboardReceiver == nil {
					m.clipboardReceiver = clipboard.NewReceiver()
				}
				if err := m.clipboardReceiver.SetText(content); err != nil {
					log.Printf("? Failed to set clipboard text on agent: %v", err)
				} else if m.clipboardMonitor != nil {
					m.clipboardMonitor.RememberText(content)
				}
			}
			return
		case "clipboard_image":
			if contentB64, ok := event["content"].(string); ok {
				imageData, err := base64.StdEncoding.DecodeString(contentB64)
				if err != nil {
					log.Printf("? Failed to decode clipboard image: %v", err)
					return
				}
				if m.clipboardReceiver == nil {
					m.clipboardReceiver = clipboard.NewReceiver()
				}
				if err := m.clipboardReceiver.SetImage(imageData); err != nil {
					log.Printf("? Failed to set clipboard image on agent: %v", err)
				} else if m.clipboardMonitor != nil {
					m.clipboardMonitor.RememberImage(imageData)
				}
			}
			return
		}
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
			if m.dataChannel != nil && m.dataChannel.ReadyState() == webrtc.DataChannelStateOpen {
				m.dataChannel.Send(data)
			}
		}
		return
	}

	// Track last input time for idle detection
	m.lastInputTime = time.Now()

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

		// Send key with modifiers
		if err := m.keyController.SendKeyWithModifiers(code, down, ctrl, shift, alt); err != nil {
			log.Printf("Key event error: %v", err)
		}
	}
}

func (m *Manager) startClipboardMonitoring() {
	// Initialize clipboard monitor
	m.clipboardMonitor = clipboard.NewMonitor()

	// Set up text clipboard callback
	m.clipboardMonitor.SetOnTextChange(func(text string) {
		if m.dataChannel == nil || m.dataChannel.ReadyState() != webrtc.DataChannelStateOpen {
			return
		}

		// Send text clipboard to controller
		msg := map[string]interface{}{
			"type":    "clipboard_text",
			"content": text,
		}

		data, err := json.Marshal(msg)
		if err != nil {
			log.Printf("❌ Failed to marshal clipboard text: %v", err)
			return
		}

		if err := m.dataChannel.Send(data); err != nil {
			log.Printf("❌ Failed to send clipboard text: %v", err)
		}
	})

	// Set up image clipboard callback
	m.clipboardMonitor.SetOnImageChange(func(imageData []byte) {
		if m.dataChannel == nil || m.dataChannel.ReadyState() != webrtc.DataChannelStateOpen {
			return
		}

		// Encode image to base64 for JSON transmission
		imageB64 := base64.StdEncoding.EncodeToString(imageData)

		// Send image clipboard to controller
		msg := map[string]interface{}{
			"type":    "clipboard_image",
			"content": imageB64,
		}

		data, err := json.Marshal(msg)
		if err != nil {
			log.Printf("❌ Failed to marshal clipboard image: %v", err)
			return
		}

		if err := m.dataChannel.Send(data); err != nil {
			log.Printf("❌ Failed to send clipboard image: %v", err)
		}
	})

	// Start monitoring
	if err := m.clipboardMonitor.Start(); err != nil {
		log.Printf("❌ Failed to start clipboard monitor: %v", err)
	}
}

func (m *Manager) startScreenStreaming() {
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

	// Adaptive streaming parameters
	fps := 20              // Current FPS (12-30)
	quality := 65          // JPEG quality (50-80)
	scale := 1.0           // Scale factor (0.5-1.0)
	frameInterval := time.Duration(1000/fps) * time.Millisecond

	// Thresholds for adaptation
	const (
		bufferHigh    = 8 * 1024 * 1024  // 8MB - reduce quality
		bufferLow     = 1 * 1024 * 1024  // 1MB - can increase quality
		minFPS        = 12
		maxFPS        = 30
		minQuality    = 50
		maxQuality    = 80
		minScale      = 0.5
		maxScale      = 1.0
		idleFPS       = 2   // FPS when idle
		idleQuality   = 50  // Quality when idle
		idleScale     = 0.75 // Scale when idle
		idleThreshold = 1.0 // Motion % threshold for idle
	)

	frameCount := 0
	errorCount := 0
	droppedFrames := 0
	bytesSent := int64(0)
	var lastFrame []byte
	var lastRGBA *image.RGBA
	lastAdaptTime := time.Now()
	lastLogTime := time.Now()
	lastFullFrame := time.Now() // For full-frame refresh cadence
	lowMotionStart := time.Time{}  // When low motion started
	isIdle := false
	motionPct := 0.0
	forceFullFrame := false

	ticker := time.NewTicker(frameInterval)
	defer ticker.Stop()

	// Start stats collection goroutine
	go m.collectStats()

	for m.isStreaming {
		// Wait for either ticker or input-triggered frame request
		select {
		case <-ticker.C:
			// Normal frame interval
		case <-m.inputFrameTrigger:
			// Input triggered - send frame immediately for visual feedback
			// Small delay to let the input take effect
			time.Sleep(10 * time.Millisecond)
		}

		if m.dataChannel == nil || m.dataChannel.ReadyState() != webrtc.DataChannelStateOpen {
			continue
		}

		// Switch to input desktop before capture (important for Session 0/login screen)
		if m.isSession0 {
			desktop.SwitchToInputDesktop()
		}

		bufferedAmount := m.dataChannel.BufferedAmount()

		// Capture RGBA for motion detection
		rgbaFrame, err := m.screenCapturer.CaptureRGBA()
		if err != nil {
			errorCount++
			if errorCount%100 == 1 {
				log.Printf("⚠️ Screen capture error: %v", err)
			}
			continue
		}

		// Detect motion using dirty regions
		width, height := m.screenCapturer.GetResolution()
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

		// Idle mode detection
		timeSinceInput := time.Since(m.lastInputTime)
		if motionPct < idleThreshold && timeSinceInput > 500*time.Millisecond {
			if lowMotionStart.IsZero() {
				lowMotionStart = time.Now()
			} else if time.Since(lowMotionStart) > 1*time.Second && !isIdle {
				// Enter idle mode
				isIdle = true
				fps = idleFPS
				quality = idleQuality
				scale = idleScale
				frameInterval = time.Duration(1000/fps) * time.Millisecond
				ticker.Reset(frameInterval)
				log.Println("💤 Entering idle mode (low motion)")
			}
		} else {
			// Exit idle mode
			lowMotionStart = time.Time{}
			if isIdle {
				isIdle = false
				fps = 20
				quality = 65
				scale = 1.0
				frameInterval = time.Duration(1000/fps) * time.Millisecond
				ticker.Reset(frameInterval)
				log.Println("⚡ Exiting idle mode (activity detected)")
			}
		}

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
				// Network congested - reduce quality aggressively
				if fps > minFPS {
					fps -= 4
					if fps < minFPS {
						fps = minFPS
					}
					changed = true
				}
				if scale > minScale {
					scale -= 0.1
					if scale < minScale {
						scale = minScale
					}
					changed = true
				}
				if quality > minQuality {
					quality -= 5
					if quality < minQuality {
						quality = minQuality
					}
					changed = true
				}
			} else if bufferedAmount < bufferLow && droppedFrames == 0 && m.lossPct < 1 && m.lastRTT < 120*time.Millisecond {
				// Network clear - can increase quality
				if quality < maxQuality {
					quality += 2
					if quality > maxQuality {
						quality = maxQuality
					}
					changed = true
				}
				if scale < maxScale {
					scale += 0.05
					if scale > maxScale {
						scale = maxScale
					}
					changed = true
				}
				if fps < maxFPS {
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

		// Drop frames if buffer is critically high
		if bufferedAmount > bufferHigh*2 {
			droppedFrames++
			continue
		}

		// H.264 mode: encode and send via video track
		if m.useH264 && m.videoTrack != nil && m.videoEncoder != nil {
			// Encode RGBA to H.264
			nalUnits, err := m.videoEncoder.Encode(rgbaFrame)
			if err != nil {
				errorCount++
				if errorCount%100 == 1 {
					log.Printf("⚠️ H.264 encode error: %v", err)
				}
				continue
			}

			if nalUnits != nil && len(nalUnits) > 0 {
				// Write H.264 NAL units to video track
				frameDuration := time.Second / time.Duration(fps)
				if err := m.videoTrack.WriteFrame(nalUnits, frameDuration); err != nil {
					errorCount++
					if errorCount%100 == 1 {
						log.Printf("⚠️ Video track write error: %v", err)
					}
				} else {
					frameCount++
					bytesSent += int64(len(nalUnits))
				}
			}
			continue
		}

		// Tiles mode: encode RGBA to JPEG with scaling
		jpeg, scaledW, scaledH, err := m.screenCapturer.CaptureJPEGScaled(quality, scale)
		if err != nil {
			errorCount++
			
			// Check if DXGI needs reinitialization (screensaver, lock screen, power save)
			errStr := err.Error()
			isDXGIError := strings.Contains(errStr, "AcquireNextFrame") || 
				strings.Contains(errStr, "error -2") || 
				strings.Contains(errStr, "DXGI") ||
				strings.Contains(errStr, "capture failed")
			
			if isDXGIError {
				log.Printf("⚠️ DXGI error detected: %s (error #%d)", errStr, errorCount)
				
				// Try to reinitialize immediately on first error, then every 3 errors
				if errorCount == 1 || errorCount%3 == 0 {
					log.Printf("🔄 Reinitializing screen capturer...")
					time.Sleep(500 * time.Millisecond) // Brief wait
					
					if reinitErr := m.screenCapturer.Reinitialize(false); reinitErr != nil {
						log.Printf("⚠️ Reinit failed: %v - will retry", reinitErr)
					} else {
						log.Printf("✅ Screen capturer reinitialized!")
						errorCount = 0
					}
				}
				time.Sleep(200 * time.Millisecond) // Don't spam
			} else if errorCount%50 == 1 {
				log.Printf("⚠️ Capture error: %v", err)
			}
			
			if lastFrame != nil {
				m.sendFrameChunked(lastFrame)
			}
			continue
		}

		_ = scaledW
		_ = scaledH

		lastFrame = jpeg

		// Send frame (use full frame marker if forced refresh)
		if forceFullFrame {
			if err := m.sendFullFrame(jpeg); err != nil {
				log.Printf("Failed to send full frame: %v", err)
			} else {
				frameCount++
				bytesSent += int64(len(jpeg))
			}
			forceFullFrame = false
		} else {
			if err := m.sendFrameChunked(jpeg); err != nil {
				log.Printf("Failed to send frame: %v", err)
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
			idleStr := ""
			if isIdle {
				idleStr = " 💤IDLE"
			}
			rttMs := m.lastRTT.Milliseconds()
			cpuPct := float64(0)
			if m.cpuMonitor != nil {
				cpuPct = m.cpuMonitor.GetCPUPercent()
			}

			// Determine current mode
			mode := "jpeg"
			if m.useH264 {
				mode = "h264"
			}

			log.Printf("📊 FPS:%d Q:%d Scale:%.0f%% Motion:%.1f%% RTT:%dms Loss:%.1f%% CPU:%.0f%%%s | %.1fKB/f %.1fMbit/s | Buf:%.1fMB | Err:%d Drop:%d",
				fps, quality, scale*100, motionPct, rttMs, m.lossPct, cpuPct, idleStr, avgKBPerFrame, sendMbps,
				float64(bufferedAmount)/1024/1024, errorCount, droppedFrames)
			droppedFrames = 0 // Reset per-second counter

			// Send stats to controller
			m.sendStats(fps, quality, scale, mode, rttMs, cpuPct)
		}
	}

	log.Printf("🛑 Screen streaming stopped (sent %d frames, %.1f MB total, %d errors)",
		frameCount, float64(bytesSent)/1024/1024, errorCount)
}

// sendStats sends streaming stats to controller
func (m *Manager) sendStats(fps, quality int, scale float64, mode string, rttMs int64, cpuPct float64) {
	if m.dataChannel == nil || m.dataChannel.ReadyState() != webrtc.DataChannelStateOpen {
		return
	}

	stats := map[string]interface{}{
		"type":    "stats",
		"fps":     fps,
		"quality": quality,
		"scale":   scale,
		"mode":    mode,
		"rtt":     rttMs,
		"cpu":     cpuPct,
	}

	data, err := json.Marshal(stats)
	if err != nil {
		return
	}

	m.dataChannel.Send(data)
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
	const chunkMagic = 0xFF    // Magic byte to identify chunked frames

	// If data fits in one message, send directly
	if len(data) <= maxChunkSize {
		return m.dataChannel.Send(data)
	}

	// Otherwise, chunk it
	totalChunks := (len(data) + maxChunkSize - 1) / maxChunkSize

	for i := 0; i < totalChunks; i++ {
		start := i * maxChunkSize
		end := start + maxChunkSize
		if end > len(data) {
			end = len(data)
		}

		// Create chunk with header: [magic, chunk_index, total_chunks, ...data]
		chunk := make([]byte, 3+len(data[start:end]))
		chunk[0] = chunkMagic
		chunk[1] = byte(i)
		chunk[2] = byte(totalChunks)
		copy(chunk[3:], data[start:end])

		if err := m.dataChannel.Send(chunk); err != nil {
			return err
		}
	}

	return nil
}

func (m *Manager) cleanupConnection(reason string) {
	log.Printf("🧹 Cleaning up connection (reason: %s)", reason)

	// Stop streaming
	m.isStreaming = false

	// Update session status to ended
	if m.sessionID != "" {
		m.updateSessionStatus("ended")
	}

	// Close data channel
	if m.dataChannel != nil {
		m.dataChannel.Close()
		m.dataChannel = nil
	}

	// Close peer connection
	if m.peerConnection != nil {
		m.peerConnection.Close()
		m.peerConnection = nil
	}

	// Reset session ID for next connection
	m.sessionID = ""

	log.Println("✅ Connection cleaned up - ready for new connections")
}

func (m *Manager) Close() {
	m.isStreaming = false

	if m.dataChannel != nil {
		m.dataChannel.Close()
	}

	if m.peerConnection != nil {
		m.peerConnection.Close()
	}
}

// handleDirListRequest handles directory listing requests from controller
func (m *Manager) handleDirListRequest(path string) {
	log.Printf("📂 Directory listing requested: %s", path)

	type FileInfo struct {
		Name    string `json:"name"`
		Path    string `json:"path"`
		Size    int64  `json:"size"`
		IsDir   bool   `json:"is_dir"`
		ModTime int64  `json:"mod_time"`
	}

	response := map[string]interface{}{
		"type":  "dir_list_response",
		"path":  path,
		"files": []FileInfo{},
		"error": "",
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		response["error"] = err.Error()
		m.sendResponse(response)
		return
	}

	files := make([]FileInfo, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		files = append(files, FileInfo{
			Name:    entry.Name(),
			Path:    filepath.Join(path, entry.Name()),
			Size:    info.Size(),
			IsDir:   entry.IsDir(),
			ModTime: info.ModTime().Unix(),
		})
	}

	response["files"] = files
	m.sendResponse(response)
}

// handleDrivesListRequest handles drive listing requests from controller
func (m *Manager) handleDrivesListRequest() {
	log.Println("💾 Drives listing requested")

	drives := []string{}

	// Windows: Check all drive letters
	for letter := 'A'; letter <= 'Z'; letter++ {
		drive := string(letter) + ":\\"
		if _, err := os.Stat(drive); err == nil {
			drives = append(drives, drive)
		}
	}

	response := map[string]interface{}{
		"type":   "drives_list_response",
		"drives": drives,
	}

	m.sendResponse(response)
}

// handleFileRequest handles file download requests from controller
func (m *Manager) handleFileRequest(remotePath string) {
	log.Printf("📥 File request: %s", remotePath)

	// Read file
	data, err := os.ReadFile(remotePath)
	if err != nil {
		response := map[string]interface{}{
			"type":  "file_response_error",
			"path":  remotePath,
			"error": err.Error(),
		}
		m.sendResponse(response)
		return
	}

	// Send file in chunks via file transfer handler
	if m.fileTransferHandler != nil {
		// Use existing file transfer mechanism
		log.Printf("📤 Sending file: %s (%d bytes)", remotePath, len(data))
		// TODO: Implement proper chunked transfer
	}
}

// sendResponse sends a JSON response over the data channel
func (m *Manager) sendResponse(data map[string]interface{}) {
	if m.dataChannel == nil {
		log.Println("❌ Cannot send response: data channel not available")
		return
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Printf("❌ Failed to marshal response: %v", err)
		return
	}

	if err := m.dataChannel.Send(jsonData); err != nil {
		log.Printf("❌ Failed to send response: %v", err)
	}
}

// collectStats collects WebRTC stats for adaptive streaming
func (m *Manager) collectStats() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for m.isStreaming {
		<-ticker.C

		if m.peerConnection == nil {
			continue
		}

		stats := m.peerConnection.GetStats()
		for _, stat := range stats {
			// Look for data channel stats
			if dcStats, ok := stat.(webrtc.DataChannelStats); ok {
				sent := dcStats.MessagesSent
				// Calculate loss from retransmits (approximation)
				if sent > m.lastPacketsSent && m.lastPacketsSent > 0 {
					delta := sent - m.lastPacketsSent
					if delta > 0 {
						// Use buffered amount as proxy for congestion/loss
						buffered := float64(0)
						if m.dataChannel != nil {
							buffered = float64(m.dataChannel.BufferedAmount())
						}
						// High buffer = potential loss/congestion
						if buffered > 4*1024*1024 { // 4MB
							m.lossPct = (buffered / (16 * 1024 * 1024)) * 10 // 0-10% based on buffer
							if m.lossPct > 10 {
								m.lossPct = 10
							}
						} else {
							m.lossPct = 0
						}
					}
				}
				m.lastPacketsSent = sent
			}
		}
	}
}
