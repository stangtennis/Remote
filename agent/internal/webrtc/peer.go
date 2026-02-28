package webrtc

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"log"
	"net/http"
	"runtime/debug"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pion/interceptor"
	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v3"
	"github.com/stangtennis/remote-agent/internal/auth"
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

// StreamMode represents the current streaming mode
type StreamMode int

const (
	ModeIdleTiles   StreamMode = iota // Low motion, high quality tiles
	ModeActiveTiles                   // Active use, moderate quality tiles
	ModeActiveH264                    // Active use, H.264 encoding
)

func (m StreamMode) String() string {
	switch m {
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

type Manager struct {
	cfg                 *config.Config
	device              *device.Device
	tokenProvider       *auth.TokenProvider
	peerConnection      *webrtc.PeerConnection
	dataChannel         *webrtc.DataChannel
	controlChannel      *webrtc.DataChannel // Separate channel for input (low latency)
	videoChannel        *webrtc.DataChannel // Unreliable channel for video (less latency)
	fileChannel         *webrtc.DataChannel // Reliable channel for file transfer
	screenCapturer      *screen.Capturer
	dirtyDetector       *screen.DirtyRegionDetector // For bandwidth optimization
	mouseController     *input.MouseController
	keyController       *input.KeyboardController
	fileTransferHandler *filetransfer.Handler
	clipboardMonitor    *clipboard.Monitor
	clipboardReceiver   *clipboard.Receiver
	sessionID           string
	isStreaming         atomic.Bool
	isSession0          bool // Running in Session 0 (before user login)

	// Concurrency control
	mu         sync.Mutex             // Protects peerConnection, dataChannel, controlChannel
	connCtx    context.Context        // Lifecycle context for current connection (streaming + grace period)
	connCancel context.CancelFunc     // Cancel function for connCtx
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

	// Frame ID for chunking (robustness against out-of-order delivery)
	frameID uint16

	// Stream params from controller (caps)
	streamMaxFPS     int
	streamMaxQuality int
	streamMaxScale   float64
	streamH264Kbps   int

	// Mode management
	modeState *ModeState

	// CPU/RTT moving averages for mode switching
	cpuAvg []float64
	rttAvg []time.Duration
}

// setAuthHeaders sets apikey and Authorization headers using authenticated JWT token.
func (m *Manager) setAuthHeaders(req *http.Request) error {
	req.Header.Set("apikey", m.cfg.SupabaseAnonKey)
	token, err := m.tokenProvider.GetToken()
	if err != nil {
		return fmt.Errorf("failed to get auth token: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	return nil
}

func New(cfg *config.Config, dev *device.Device, tokenProvider *auth.TokenProvider) (*Manager, error) {
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
		Bitrate:          8000, // 8 Mbps - good quality for 1080p screen content
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
		tokenProvider:       tokenProvider,
		screenCapturer:      capturer,
		mouseController:     input.NewMouseController(width, height),
		keyController:       input.NewKeyboardController(),
		fileTransferHandler: fileTransferHandler,
		isSession0:          isSession0,
		currentDesktop:      currentDesktopType,
		videoEncoder:        videoEncoder,
		useH264:             false, // Start with JPEG tiles; enable H.264 via set_mode when ready
		cpuMonitor:          cpuMon,
		inputFrameTrigger:   make(chan struct{}, 1), // Buffered to avoid blocking
		modeState: &ModeState{
			current:    ModeIdleTiles,
			lastSwitch: time.Now(),
			minModeDur: 2 * time.Second, // Minimum 2s in mode before switching
		},
		cpuAvg: make([]float64, 0, 6),       // 3 seconds at 500ms intervals
		rttAvg: make([]time.Duration, 0, 6), // 3 seconds at 500ms intervals
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

func (m *Manager) CreatePeerConnection(iceServers []webrtc.ICEServer) error {
	// Close any existing connection first
	if m.peerConnection != nil {
		log.Println("🔄 Closing existing peer connection for new connection...")
		m.cleanupConnection("New connection requested")
	}

	config := webrtc.Configuration{
		ICEServers: iceServers,
	}

	// Create MediaEngine with H.264 codec support (required for H.264 track negotiation).
	me := &webrtc.MediaEngine{}
	_ = me.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{
			MimeType:    webrtc.MimeTypeH264,
			ClockRate:   90000,
			SDPFmtpLine: "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f",
		},
		PayloadType: 96,
	}, webrtc.RTPCodecTypeVideo)

	ir := &interceptor.Registry{}
	// Default interceptors are needed for RTCP feedback, NACK/PLI plumbing, etc.
	_ = webrtc.RegisterDefaultInterceptors(me, ir)

	api := webrtc.NewAPI(webrtc.WithMediaEngine(me), webrtc.WithInterceptorRegistry(ir))
	pc, err := api.NewPeerConnection(config)
	if err != nil {
		return fmt.Errorf("failed to create peer connection: %w", err)
	}

	// Reset state for new connection
	m.answerSent = false
	m.pendingCandidates = nil
	m.mu.Lock()
	m.dataChannel = nil
	m.controlChannel = nil
	m.videoChannel = nil
	m.fileChannel = nil
	m.mu.Unlock()
	m.peerConnection = pc

	// Always add video track (even if not using H.264 yet)
	// This allows mode switching without renegotiation
	videoTrack, err := video.NewTrack()
	if err != nil {
		log.Printf("⚠️ Failed to create video track: %v", err)
	} else {
		m.videoTrack = videoTrack
		sender, err := pc.AddTrack(videoTrack.GetTrack())
		if err != nil {
			log.Printf("⚠️ Failed to add video track: %v", err)
		} else {
			log.Println("🎬 Video track added (H.264 ready, mode switch without renegotiation)")

			// Drain RTCP and react to keyframe requests (PLI/FIR) to avoid stalls and improve recovery.
			go func() {
				for {
					pkts, _, rtcpErr := sender.ReadRTCP()
					if rtcpErr != nil {
						return
					}
					for _, pkt := range pkts {
						switch pkt.(type) {
						case *rtcp.PictureLossIndication, *rtcp.FullIntraRequest:
							if m.videoEncoder != nil {
								m.videoEncoder.ForceKeyframe()
							}
						}
					}
				}
			}()
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
			// Cancel any previous streaming/grace period goroutines
			if m.connCancel != nil {
				m.connCancel()
			}
			m.connCtx, m.connCancel = context.WithCancel(context.Background())
			m.isStreaming.Store(true)
			// Hide local cursor during remote session
			if m.mouseController != nil {
				m.mouseController.HideCursor()
			}
			go func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("🔥 STREAMING PANIC: %v", r)
						log.Printf("🔥 Stack: %s", string(debug.Stack()))
					}
				}()
				m.startScreenStreaming(m.connCtx)
			}()
		case webrtc.PeerConnectionStateDisconnected:
			log.Println("⚠️  WebRTC DISCONNECTED - waiting for ICE recovery...")
			m.isStreaming.Store(false) // Stop sending frames during recovery
			// Cancel previous streaming, start grace period with new context
			if m.connCancel != nil {
				m.connCancel()
			}
			m.connCtx, m.connCancel = context.WithCancel(context.Background())
			go func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("🔥 DISCONNECT HANDLER PANIC: %v", r)
						log.Printf("🔥 Stack: %s", string(debug.Stack()))
					}
				}()
				m.handleDisconnectGracePeriod(m.connCtx)
			}()
		case webrtc.PeerConnectionStateFailed:
			log.Println("❌ WebRTC CONNECTION FAILED")
			// Restore local cursor
			if m.mouseController != nil {
				m.mouseController.ShowCursor()
			}
			if m.connCancel != nil {
				m.connCancel()
			}
			m.cleanupConnection("Failed")
		case webrtc.PeerConnectionStateClosed:
			log.Println("🔒 WebRTC CONNECTION CLOSED")
			// Restore local cursor
			if m.mouseController != nil {
				m.mouseController.ShowCursor()
			}
			if m.connCancel != nil {
				m.connCancel()
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
		switch dc.Label() {
		case "control":
			log.Println("🎮 Control channel ready (low-latency input)")
			m.controlChannel = dc
			m.setupControlChannelHandlers(dc)
			// Also use control channel for streaming if no separate data channel
			// Dashboard only creates one "control" channel for both input and frames
			if m.dataChannel == nil {
				m.dataChannel = dc
				log.Println("📺 Using control channel for frame streaming")
			}
		case "video":
			log.Println("🎬 Video channel ready (unreliable, low-latency)")
			m.videoChannel = dc
			dc.OnOpen(func() {
				log.Println("✅ VIDEO CHANNEL OPEN - Using unreliable channel for frames")
			})
		case "file":
			log.Println("📁 File channel ready (reliable, ordered)")
			m.fileChannel = dc
			m.setupFileChannelHandlers(dc)
		default:
			m.dataChannel = dc
			m.setupDataChannelHandlers(dc)
		}
	})

	return nil
}

func (m *Manager) setupDataChannelHandlers(dc *webrtc.DataChannel) {
	dc.OnOpen(func() {
		log.Println("✅ DATA CHANNEL READY - Controller can now receive frames!")

		// Clear any stuck modifier keys from previous sessions
		if m.keyController != nil {
			m.keyController.ClearModifiers()
		}

		// NOTE: File transfer callback is set in setupFileChannelHandlers
		// Do NOT set it here as it would override the file channel callback

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

// setupFileChannelHandlers sets up the reliable file transfer channel
func (m *Manager) setupFileChannelHandlers(dc *webrtc.DataChannel) {
	dc.OnOpen(func() {
		log.Println("📁 FILE CHANNEL READY - File transfer enabled!")
		
		// Set up file transfer send callback to use file channel
		if m.fileTransferHandler != nil {
			m.fileTransferHandler.SetSendDataCallback(func(data []byte) error {
				if dc.ReadyState() == webrtc.DataChannelStateOpen {
					return dc.Send(data)
				}
				return fmt.Errorf("file channel not ready")
			})
		}
	})

	dc.OnClose(func() {
		log.Println("📁 File channel closed")
	})

	dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		// Handle file transfer messages
		if m.fileTransferHandler != nil {
			if err := m.fileTransferHandler.HandleIncomingData(msg.Data); err != nil {
				log.Printf("❌ File transfer error: %v", err)
			}
		}
	})
}

// setupControlChannelHandlers sets up the low-latency control channel for input
func (m *Manager) setupControlChannelHandlers(dc *webrtc.DataChannel) {
	dc.OnOpen(func() {
		log.Println("🎮 CONTROL CHANNEL READY - Low-latency input enabled!")
		// Clear any stuck modifier keys from previous sessions
		if m.keyController != nil {
			m.keyController.ClearModifiers()
		}
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

		// Check for control messages from controller
		if msgType, ok := event["type"].(string); ok {
			switch msgType {
			case "clipboard_text":
				if content, ok := event["content"].(string); ok {
					log.Printf("📋 Received clipboard text from controller (%d bytes)", len(content))
					m.handleClipboardText(content)
				}
				return
			case "clipboard_image":
				if contentB64, ok := event["content"].(string); ok {
					log.Printf("📋 Received clipboard image from controller")
					m.handleClipboardImage(contentB64)
				}
				return
			case "set_stream_params":
				m.handleSetStreamParams(event)
				return
			}
		}

		// Handle input events on control channel
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

	// Route input via pipe when in Session 0 with a pipe-based capturer
	if m.isSession0 && m.screenCapturer != nil && m.screenCapturer.HasInputForwarder() {
		switch eventType {
		case "mouse_move":
			x, _ := event["x"].(float64)
			y, _ := event["y"].(float64)
			m.screenCapturer.ForwardMouseMove(int(x), int(y))

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
			m.screenCapturer.ForwardMouseClick(btnCode, downVal, int(x), int(y))

		case "mouse_scroll":
			delta, _ := event["delta"].(float64)
			m.screenCapturer.ForwardScroll(int(delta), 0, 0)

		case "key":
			code, _ := event["code"].(string)
			down, _ := event["down"].(bool)
			ctrl, _ := event["ctrl"].(bool)
			shift, _ := event["shift"].(bool)
			alt, _ := event["alt"].(bool)
			meta, _ := event["meta"].(bool)

			// Unicode char forwarding
			if charStr, ok := event["char"].(string); ok && charStr != "" && down {
				for _, ch := range charStr {
					m.screenCapturer.ForwardUnicodeChar(ch)
				}
			} else {
				m.screenCapturer.ForwardKeyEvent(code, down, ctrl, shift, alt, meta)
			}
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
			kbps := int(bitrate)
			if kbps > 50000 {
				kbps = 50000
			}
			m.SetVideoBitrate(kbps)
		}
		return
	}

	// Handle switch_monitor
	if msgType, ok := event["type"].(string); ok && msgType == "switch_monitor" {
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
				if m.dataChannel != nil && m.dataChannel.ReadyState() == webrtc.DataChannelStateOpen {
					m.dataChannel.Send(data)
				}
			}
			log.Printf("✅ Switched to monitor %d: %dx%d (offset: %d,%d)", index, width, height, offsetX, offsetY)
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

// handleClipboardText handles incoming clipboard text from controller
func (m *Manager) handleClipboardText(content string) {
	if m.clipboardReceiver == nil {
		m.clipboardReceiver = clipboard.NewReceiver()
	}
	if err := m.clipboardReceiver.SetText(content); err != nil {
		log.Printf("❌ Failed to set clipboard text on agent: %v", err)
	} else {
		log.Println("✅ Clipboard text set on agent")
		if m.clipboardMonitor != nil {
			m.clipboardMonitor.RememberText(content)
		}
	}
}

// handleClipboardImage handles incoming clipboard image from controller
func (m *Manager) handleClipboardImage(contentB64 string) {
	imageData, err := base64.StdEncoding.DecodeString(contentB64)
	if err != nil {
		log.Printf("❌ Failed to decode clipboard image: %v", err)
		return
	}
	if m.clipboardReceiver == nil {
		m.clipboardReceiver = clipboard.NewReceiver()
	}
	if err := m.clipboardReceiver.SetImage(imageData); err != nil {
		log.Printf("❌ Failed to set clipboard image on agent: %v", err)
	} else {
		log.Println("✅ Clipboard image set on agent")
		if m.clipboardMonitor != nil {
			m.clipboardMonitor.RememberImage(imageData)
		}
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

// lightMotionProbe performs a cheap motion detection by sampling pixels
// Returns estimated motion percentage without full RGBA capture
func (m *Manager) lightMotionProbe() (float64, error) {
	// For now, we'll use a downscaled capture (1/4 resolution)
	// This is much cheaper than full resolution capture
	width, height := m.screenCapturer.GetResolution()
	probeWidth := width / 4
	probeHeight := height / 4
	
	// TODO: Implement actual downscaled capture in screen.Capturer
	// For now, return 0 to indicate we need full capture
	_ = probeWidth
	_ = probeHeight
	return 0, nil
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
		*fps = 20
		*quality = 65
		*scale = 1.0
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

	// Adaptive streaming parameters
	fps := 20     // Current FPS (12-30)
	quality := 65 // JPEG quality (50-80)
	scale := 1.0  // Scale factor (0.5-1.0)
	frameInterval := time.Duration(1000/fps) * time.Millisecond

	// Thresholds for adaptation (use controller caps if set)
	bufferHigh := uint64(8 * 1024 * 1024) // 8MB - reduce quality
	bufferLow := uint64(1 * 1024 * 1024)  // 1MB - can increase quality
	minFPS := 12
	maxFPS := 30
	minQuality := 50
	maxQuality := 80
	minScale := 0.5
	maxScale := 1.0

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

		if m.dataChannel == nil || m.dataChannel.ReadyState() != webrtc.DataChannelStateOpen {
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

		// Switch to input desktop before capture (important for Session 0/login screen)
		if m.isSession0 {
			desktop.SwitchToInputDesktop()
		}

		bufferedAmount := m.dataChannel.BufferedAmount()

		// Capture RGBA for motion detection (also used for JPEG encoding - single capture)
		rgbaFrame, err := m.screenCapturer.CaptureRGBA()
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

					if reinitErr := m.screenCapturer.Reinitialize(false); reinitErr != nil {
						log.Printf("⚠️ Reinit failed: %v", reinitErr)
					} else {
						log.Printf("✅ Screen capturer reinitialized!")
						// Don't reset errorCount — prevents infinite reinit loop
						// when the same error keeps recurring (e.g., GDI -3 in Session 0)
					}
				}
				time.Sleep(200 * time.Millisecond)
			} else if errorCount%50 == 1 {
				log.Printf("⚠️ Unknown capture error: %v", err)
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
				if scale > minScale {
					scale -= 0.1
					if scale < minScale {
						scale = minScale
					}
					changed = true
				} else if fps > minFPS {
					fps -= 4
					if fps < minFPS {
						fps = minFPS
					}
					changed = true
				} else if quality > minQuality {
					quality -= 5
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

		// EARLY-DROP: Drop frames before encode if buffer/CPU is critically high
		// This saves CPU compared to encoding and then dropping
		criticalBuffer := bufferedAmount > bufferHigh*2
		criticalCPU := m.cpuMonitor != nil && m.cpuMonitor.IsCriticalCPU()
		
		if criticalBuffer || criticalCPU {
			droppedFrames++
			if criticalCPU && droppedFrames%10 == 1 {
				log.Printf("⚠️ Early-drop: CPU %.1f%% - skipping encode", cpuPct)
			}
			continue
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
				nalUnits, err := m.videoEncoder.Encode(rgbaFrame)
				if err != nil {
					errorCount++
					if errorCount%100 == 1 {
						log.Printf("⚠️ H.264 encode fejl: %v", err)
					}
					return
				}

				if nalUnits != nil && len(nalUnits) > 0 {
					// Write H.264 NAL units to video track
					frameDuration := time.Second / time.Duration(fps)
					if err := m.videoTrack.WriteFrame(nalUnits, frameDuration); err != nil {
						errorCount++
						if errorCount%100 == 1 {
							log.Printf("⚠️ Video track write fejl: %v", err)
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
		jpeg, scaledW, scaledH, err := m.screenCapturer.EncodeRGBAToJPEG(rgbaFrame, quality, scale)
		if err != nil {
			errorCount++
			if errorCount%50 == 1 {
				log.Printf("⚠️ JPEG encode error: %v", err)
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
			droppedFrames = 0  // Reset per-second counter
			skippedFrames = 0  // Reset per-second counter

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
	const chunkMagic = 0xFE    // Magic byte for chunked frames with frame ID (new format)

	// Increment frame ID for each new frame
	m.frameID++
	frameID := m.frameID

	// Use video channel if available (unreliable = lower latency)
	// Fall back to data channel for compatibility
	sendChannel := m.dataChannel
	if m.videoChannel != nil && m.videoChannel.ReadyState() == webrtc.DataChannelStateOpen {
		sendChannel = m.videoChannel
	}
	if sendChannel == nil || sendChannel.ReadyState() != webrtc.DataChannelStateOpen {
		return fmt.Errorf("no channel available for sending")
	}

	// If data fits in one message, send directly (no chunking needed)
	if len(data) <= maxChunkSize {
		return sendChannel.Send(data)
	}

	// Otherwise, chunk it with frame ID for robustness
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

		if err := sendChannel.Send(chunk); err != nil {
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
		if m.dataChannel != nil && m.dataChannel.ReadyState() == webrtc.DataChannelStateOpen {
			m.dataChannel.Send(data)
			log.Printf("📺 Sent monitor list: %d monitors (active: %d)", len(monitors), activeIndex)
		}
	}
}

// handleDisconnectGracePeriod waits up to 8 seconds for ICE to self-recover
// before cleaning up the connection. Checks every 500ms.
// The context is cancelled when a new connection state change supersedes this grace period.
func (m *Manager) handleDisconnectGracePeriod(ctx context.Context) {
	const gracePeriod = 8 * time.Second
	const checkInterval = 500 * time.Millisecond
	deadline := time.Now().Add(gracePeriod)

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			log.Println("🔌 Grace period cancelled (new state change)")
			return
		case <-time.After(checkInterval):
		}

		m.mu.Lock()
		pc := m.peerConnection
		m.mu.Unlock()

		if pc == nil {
			log.Println("🔌 PeerConnection is nil during grace period - cleaning up")
			break
		}

		state := pc.ConnectionState()
		switch state {
		case webrtc.PeerConnectionStateConnected:
			// NOTE: The OnConnectionStateChange handler will also fire for Connected,
			// which cancels this context and starts fresh streaming. We just return here
			// to avoid duplicate streaming goroutines.
			log.Println("✅ ICE recovered during grace period (handler will start streaming)")
			return
		case webrtc.PeerConnectionStateFailed, webrtc.PeerConnectionStateClosed:
			log.Printf("❌ Connection state became %s during grace period", state.String())
			if m.mouseController != nil {
				m.mouseController.ShowCursor()
			}
			m.cleanupConnection(state.String())
			return
		}
		// Still disconnected - keep waiting
	}

	log.Println("⏰ Grace period expired (8s) - cleaning up connection")
	if m.mouseController != nil {
		m.mouseController.ShowCursor()
	}
	m.cleanupConnection("Disconnected (grace period expired)")
}

func (m *Manager) cleanupConnection(reason string) {
	log.Printf("🧹 Cleaning up connection (reason: %s)", reason)

	// Stop streaming
	m.isStreaming.Store(false)

	// Update session status to ended
	if m.sessionID != "" {
		m.updateSessionStatus("ended")
	}

	// Close data channels and peer connection under mutex
	m.mu.Lock()
	if m.dataChannel != nil {
		m.dataChannel.Close()
		m.dataChannel = nil
	}
	if m.controlChannel != nil {
		m.controlChannel.Close()
		m.controlChannel = nil
	}
	if m.videoChannel != nil {
		m.videoChannel.Close()
		m.videoChannel = nil
	}
	if m.fileChannel != nil {
		m.fileChannel.Close()
		m.fileChannel = nil
	}

	pc := m.peerConnection
	m.peerConnection = nil
	m.mu.Unlock()

	// Close peer connection outside mutex to avoid deadlock with pion callbacks
	if pc != nil {
		pc.Close()
	}

	// Reset session ID for next connection
	m.sessionID = ""

	log.Println("✅ Connection cleaned up - ready for new connections")
}

func (m *Manager) Close() {
	m.isStreaming.Store(false)

	if m.connCancel != nil {
		m.connCancel()
	}

	m.mu.Lock()
	if m.dataChannel != nil {
		m.dataChannel.Close()
	}
	pc := m.peerConnection
	m.mu.Unlock()

	if pc != nil {
		pc.Close()
	}
}

// handleDirListRequest handles directory listing requests from controller
func (m *Manager) handleDirListRequest(path string) {
	log.Printf("📂 Directory listing requested: %s", path)

	// Sanitize path - reject traversal attempts
	cleanPath := filepath.Clean(path)
	if strings.Contains(cleanPath, "..") {
		log.Printf("⚠️ Path traversal rejected: %s", path)
		return
	}
	path = cleanPath

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

	// Sanitize path - reject traversal attempts
	cleanPath := filepath.Clean(remotePath)
	if strings.Contains(cleanPath, "..") {
		log.Printf("⚠️ Path traversal rejected: %s", remotePath)
		return
	}
	remotePath = cleanPath

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
func (m *Manager) collectStats(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		m.mu.Lock()
		pc := m.peerConnection
		m.mu.Unlock()

		if pc == nil {
			continue
		}

		stats := pc.GetStats()
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
