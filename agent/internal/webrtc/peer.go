package webrtc

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pion/interceptor"
	"github.com/pion/rtcp"
	pionwebrtc "github.com/pion/webrtc/v3"
	"github.com/stangtennis/remote-agent/internal/auth"
	"github.com/stangtennis/remote-agent/internal/terminal"
	"github.com/stangtennis/remote-agent/internal/clipboard"
	"github.com/stangtennis/remote-agent/internal/config"
	"github.com/stangtennis/remote-agent/internal/desktop"
	"github.com/stangtennis/remote-agent/internal/device"
	"github.com/stangtennis/remote-agent/internal/filetransfer"
	"github.com/stangtennis/remote-agent/internal/input"
	"github.com/stangtennis/remote-agent/internal/monitor"
	"github.com/stangtennis/remote-agent/internal/screen"
	"github.com/stangtennis/remote-agent/internal/updater"
	"github.com/stangtennis/remote-agent/internal/version"
	"github.com/stangtennis/remote-agent/internal/video"
	"github.com/stangtennis/remote-agent/internal/video/encoder"
)

type Manager struct {
	cfg                 *config.Config
	device              *device.Device
	tokenProvider       *auth.TokenProvider
	peerConnection      *pionwebrtc.PeerConnection
	dataChannel         *pionwebrtc.DataChannel
	controlChannel      *pionwebrtc.DataChannel // Separate channel for input (low latency)
	videoChannel        *pionwebrtc.DataChannel // Unreliable channel for video (less latency)
	fileChannel         *pionwebrtc.DataChannel // Reliable channel for file transfer
	terminalChannel     *pionwebrtc.DataChannel // Data channel for remote terminal
	terminal            *terminal.Terminal
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
	startedInSession0   bool // Process was started in Session 0 (never changes)

	// Concurrency control
	mu         sync.Mutex             // Protects peerConnection, dataChannel, controlChannel
	connCtx    context.Context        // Lifecycle context for current connection (streaming + grace period)
	connCancel context.CancelFunc     // Cancel function for connCtx
	currentDesktop      desktop.DesktopType
	pendingCandidates   []*pionwebrtc.ICECandidate // Buffer ICE candidates until answer is sent
	answerSent          bool                       // Flag to track if answer has been sent
	iceStopCh           chan struct{}               // Closed to stop ICE polling goroutine

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

	// Status callback for tray updates
	StatusCallback func(string)

	// Polling health tracking (for heartbeat awareness)
	pollingHealthy  atomic.Bool  // true = polling is working
	lastPollSuccess atomic.Int64 // Unix timestamp of last successful poll

	// Shared HTTP client with connection pooling (reused across all requests)
	httpClient *http.Client
}

// IsPollingHealthy returns true if session polling is working normally.
func (m *Manager) IsPollingHealthy() bool {
	return m.pollingHealthy.Load()
}

// LastPollSuccess returns the time of the last successful poll.
func (m *Manager) LastPollSuccess() time.Time {
	return time.Unix(m.lastPollSuccess.Load(), 0)
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
		currentDesktopType = desktop.DesktopWinlogon // So WTS fallback can detect transition to DesktopDefault
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
		startedInSession0:   isSession0,
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
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        10,
				MaxIdleConnsPerHost: 5,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
	mgr.pollingHealthy.Store(true) // Start healthy
	mgr.lastPollSuccess.Store(time.Now().Unix())

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
			if m.startedInSession0 {
				// We're still in Session 0 process but user has logged in
				// Keep isSession0 = true so input goes through pipe forwarder
				log.Println("🔓 User session detected from Session 0 — reinitializing capturer with pipe forwarder")
				if m.screenCapturer != nil {
					if err := m.screenCapturer.Reinitialize(true); err != nil {
						log.Printf("❌ Failed to reinitialize capturer: %v", err)
					}
					width, height := m.screenCapturer.GetResolution()
					m.mouseController = input.NewMouseController(width, height)
					hasForwarder := m.screenCapturer.HasInputForwarder()
					log.Printf("✅ Capturer reinitialized: %dx%d, HasInputForwarder=%v", width, height, hasForwarder)
				}
			} else {
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
			}
		default:
			log.Printf("⚠️  Desktop switched to unknown type: %d (was: %d)", dt, oldDesktop)
		}
	})
}

func (m *Manager) CreatePeerConnection(iceServers []pionwebrtc.ICEServer) error {
	// Close any existing connection first
	if m.peerConnection != nil {
		log.Println("🔄 Closing existing peer connection for new connection...")
		m.cleanupConnection("New connection requested")
	}

	config := pionwebrtc.Configuration{
		ICEServers: iceServers,
	}

	// Create MediaEngine with H.264 codec support (required for H.264 track negotiation).
	me := &pionwebrtc.MediaEngine{}
	_ = me.RegisterCodec(pionwebrtc.RTPCodecParameters{
		RTPCodecCapability: pionwebrtc.RTPCodecCapability{
			MimeType:    pionwebrtc.MimeTypeH264,
			ClockRate:   90000,
			SDPFmtpLine: "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f",
		},
		PayloadType: 96,
	}, pionwebrtc.RTPCodecTypeVideo)

	ir := &interceptor.Registry{}
	// Default interceptors are needed for RTCP feedback, NACK/PLI plumbing, etc.
	_ = pionwebrtc.RegisterDefaultInterceptors(me, ir)

	api := pionwebrtc.NewAPI(pionwebrtc.WithMediaEngine(me), pionwebrtc.WithInterceptorRegistry(ir))
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
	m.terminalChannel = nil
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
	pc.OnICEConnectionStateChange(func(state pionwebrtc.ICEConnectionState) {
		log.Printf("🧊 ICE connection state: %s", state.String())
		if state == pionwebrtc.ICEConnectionStateConnected {
			log.Println("🧊 ICE layer connected!")
		}
	})

	// Set up connection state handler
	// Capture current peer connection to detect stale callbacks from old connections
	thisPC := pc
	pc.OnConnectionStateChange(func(state pionwebrtc.PeerConnectionState) {
		log.Printf("🔄 Connection state changed: %s", state.String())

		// Ignore callbacks from old peer connections that fired after a new connection was created
		m.mu.Lock()
		isCurrentPC := m.peerConnection == thisPC
		m.mu.Unlock()
		if !isCurrentPC && (state == pionwebrtc.PeerConnectionStateClosed || state == pionwebrtc.PeerConnectionStateFailed) {
			log.Printf("🔄 Ignoring stale %s callback from old peer connection", state.String())
			return
		}

		switch state {
		case pionwebrtc.PeerConnectionStateConnected:
			log.Println("✅ WebRTC CONNECTED! Starting screen streaming...")
			if m.StatusCallback != nil {
				m.StatusCallback("Forbundet")
			}
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
		case pionwebrtc.PeerConnectionStateDisconnected:
			log.Println("⚠️  WebRTC DISCONNECTED - waiting for ICE recovery...")
			if m.StatusCallback != nil {
				m.StatusCallback("Afbrudt — venter på genforbindelse...")
			}
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
		case pionwebrtc.PeerConnectionStateFailed:
			log.Println("❌ WebRTC CONNECTION FAILED")
			if m.StatusCallback != nil {
				m.StatusCallback("Status: Online (ingen forbindelse)")
			}
			// Restore local cursor
			if m.mouseController != nil {
				m.mouseController.ShowCursor()
			}
			if m.connCancel != nil {
				m.connCancel()
			}
			m.cleanupConnection("Failed")
		case pionwebrtc.PeerConnectionStateClosed:
			log.Println("🔒 WebRTC CONNECTION CLOSED")
			if m.StatusCallback != nil {
				m.StatusCallback("Status: Online (ingen forbindelse)")
			}
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
	pc.OnICECandidate(func(candidate *pionwebrtc.ICECandidate) {
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
	pc.OnDataChannel(func(dc *pionwebrtc.DataChannel) {
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
		case "terminal":
			log.Println("🖥️ Terminal data channel opened")
			m.terminalChannel = dc
			m.setupTerminalChannelHandlers(dc)
		default:
			m.dataChannel = dc
			m.setupDataChannelHandlers(dc)
		}
	})

	return nil
}

func (m *Manager) setupDataChannelHandlers(dc *pionwebrtc.DataChannel) {
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

	dc.OnMessage(func(msg pionwebrtc.DataChannelMessage) {
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
func (m *Manager) setupFileChannelHandlers(dc *pionwebrtc.DataChannel) {
	dc.OnOpen(func() {
		log.Println("📁 FILE CHANNEL READY - File transfer enabled!")

		// Set up file transfer send callback to use file channel
		if m.fileTransferHandler != nil {
			m.fileTransferHandler.SetSendDataCallback(func(data []byte) error {
				if dc.ReadyState() == pionwebrtc.DataChannelStateOpen {
					return dc.Send(data)
				}
				return fmt.Errorf("file channel not ready")
			})
		}
	})

	dc.OnClose(func() {
		log.Println("📁 File channel closed")
	})

	dc.OnMessage(func(msg pionwebrtc.DataChannelMessage) {
		// Handle file transfer messages
		if m.fileTransferHandler != nil {
			if err := m.fileTransferHandler.HandleIncomingData(msg.Data); err != nil {
				log.Printf("❌ File transfer error: %v", err)
			}
		}
	})
}

// setupTerminalChannelHandlers sets up the terminal data channel for remote shell access
func (m *Manager) setupTerminalChannelHandlers(dc *pionwebrtc.DataChannel) {
	dc.OnOpen(func() {
		log.Println("🖥️ TERMINAL CHANNEL READY")
	})

	dc.OnClose(func() {
		log.Println("🖥️ Terminal channel closed")
		if m.terminal != nil {
			m.terminal.Close()
			m.terminal = nil
		}
	})

	dc.OnMessage(func(msg pionwebrtc.DataChannelMessage) {
		var termMsg struct {
			Type string `json:"type"`
			Data string `json:"data"`
		}
		if err := json.Unmarshal(msg.Data, &termMsg); err != nil {
			return
		}
		switch termMsg.Type {
		case "input":
			if m.terminal != nil {
				m.terminal.Write([]byte(termMsg.Data))
			}
		case "start":
			// Start new terminal session
			if m.terminal != nil {
				m.terminal.Close()
			}
			term, err := terminal.New()
			if err != nil {
				errMsg, _ := json.Marshal(map[string]string{"type": "error", "data": err.Error()})
				dc.Send(errMsg)
				return
			}
			m.terminal = term
			// Forward output to data channel
			go term.ReadOutput(func(data []byte) {
				outMsg, _ := json.Marshal(map[string]string{"type": "output", "data": string(data)})
				dc.Send(outMsg)
			})
			go term.ReadStderr(func(data []byte) {
				outMsg, _ := json.Marshal(map[string]string{"type": "output", "data": string(data)})
				dc.Send(outMsg)
			})
		case "close":
			if m.terminal != nil {
				m.terminal.Close()
				m.terminal = nil
			}
		}
	})
}

// setupControlChannelHandlers sets up the low-latency control channel for input
func (m *Manager) setupControlChannelHandlers(dc *pionwebrtc.DataChannel) {
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

	dc.OnMessage(func(msg pionwebrtc.DataChannelMessage) {
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
		case pionwebrtc.PeerConnectionStateConnected:
			// NOTE: The OnConnectionStateChange handler will also fire for Connected,
			// which cancels this context and starts fresh streaming. We just return here
			// to avoid duplicate streaming goroutines.
			log.Println("✅ ICE recovered during grace period (handler will start streaming)")
			return
		case pionwebrtc.PeerConnectionStateFailed, pionwebrtc.PeerConnectionStateClosed:
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

	// Stop ICE polling goroutine from previous session
	if m.iceStopCh != nil {
		close(m.iceStopCh)
		m.iceStopCh = nil
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
	if m.terminalChannel != nil {
		m.terminalChannel.Close()
		m.terminalChannel = nil
	}

	pc := m.peerConnection
	m.peerConnection = nil
	m.mu.Unlock()

	// Close terminal session if active
	if m.terminal != nil {
		m.terminal.Close()
		m.terminal = nil
	}

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
	if m.controlChannel != nil {
		m.controlChannel.Close()
	}
	if m.videoChannel != nil {
		m.videoChannel.Close()
	}
	if m.fileChannel != nil {
		m.fileChannel.Close()
	}
	if m.terminalChannel != nil {
		m.terminalChannel.Close()
	}
	pc := m.peerConnection
	m.peerConnection = nil
	m.mu.Unlock()

	// Close terminal session if active
	if m.terminal != nil {
		m.terminal.Close()
		m.terminal = nil
	}

	if pc != nil {
		done := make(chan struct{})
		go func() {
			pc.Close()
			close(done)
		}()
		select {
		case <-done:
			log.Println("WebRTC peer connection closed gracefully")
		case <-time.After(5 * time.Second):
			log.Println("WebRTC peer connection close timed out (5s)")
		}
	}
}

// handleForceUpdate checks for updates, downloads and installs if available
func (m *Manager) handleForceUpdate() {
	// Send status back to dashboard
	sendStatus := func(status, message string) {
		msg := map[string]interface{}{
			"type":    "update_status",
			"status":  status,
			"message": message,
		}
		if data, err := json.Marshal(msg); err == nil {
			if m.dataChannel != nil && m.dataChannel.ReadyState() == pionwebrtc.DataChannelStateOpen {
				m.dataChannel.Send(data)
			}
		}
	}

	sendStatus("checking", "Tjekker for opdateringer...")

	u, err := updater.NewUpdater(version.Version)
	if err != nil {
		log.Printf("❌ Force update: could not create updater: %v", err)
		sendStatus("error", "Kunne ikke initialisere opdatering: "+err.Error())
		return
	}

	if err := u.CheckForUpdate(); err != nil {
		log.Printf("❌ Force update: check failed: %v", err)
		sendStatus("error", "Fejl ved tjek: "+err.Error())
		return
	}

	if u.GetAvailableUpdate() == nil {
		log.Println("✅ Force update: already up to date")
		sendStatus("up_to_date", "Agent er allerede opdateret ("+version.Version+")")
		return
	}

	info := u.GetAvailableUpdate()
	sendStatus("downloading", "Downloader "+info.TagName+"...")

	if err := u.DownloadUpdate(); err != nil {
		log.Printf("❌ Force update: download failed: %v", err)
		sendStatus("error", "Download fejlede: "+err.Error())
		return
	}

	sendStatus("installing", "Installerer "+info.TagName+"...")

	if err := u.InstallUpdate(); err != nil {
		log.Printf("❌ Force update: install failed: %v", err)
		sendStatus("error", "Installation fejlede: "+err.Error())
		return
	}

	sendStatus("restarting", "Genstarter agent med "+info.TagName+"...")
	log.Printf("✅ Force update: installed %s, agent will restart", info.TagName)
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
