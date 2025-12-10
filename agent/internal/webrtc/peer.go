package webrtc

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/pion/webrtc/v3"
	"github.com/stangtennis/remote-agent/internal/clipboard"
	"github.com/stangtennis/remote-agent/internal/config"
	"github.com/stangtennis/remote-agent/internal/desktop"
	"github.com/stangtennis/remote-agent/internal/device"
	"github.com/stangtennis/remote-agent/internal/filetransfer"
	"github.com/stangtennis/remote-agent/internal/input"
	"github.com/stangtennis/remote-agent/internal/screen"
)

type Manager struct {
	cfg                 *config.Config
	device              *device.Device
	peerConnection      *webrtc.PeerConnection
	dataChannel         *webrtc.DataChannel
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

	mgr := &Manager{
		cfg:                 cfg,
		device:              dev,
		screenCapturer:      capturer,
		mouseController:     input.NewMouseController(width, height),
		keyController:       input.NewKeyboardController(),
		fileTransferHandler: fileTransferHandler,
		isSession0:          isSession0,
		currentDesktop:      currentDesktopType,
	}

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
		m.dataChannel = dc
		m.setupDataChannelHandlers(dc)
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

func (m *Manager) handleControlEvent(event map[string]interface{}) {
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
	// Stream JPEG frames over data channel
	// 30 FPS (33ms) = smooth, reasonable bandwidth
	ticker := time.NewTicker(33 * time.Millisecond)
	defer ticker.Stop()

	log.Println("🎥 Starting screen streaming at 30 FPS with dirty region detection...")

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

	// Initialize dirty region detector (128x128 tiles for good balance)
	if m.dirtyDetector == nil {
		m.dirtyDetector = screen.NewDirtyRegionDetector(128, 128)
		log.Println("✅ Dirty region detector initialized (128x128 tiles)")
	}

	frameCount := 0
	errorCount := 0
	droppedFrames := 0
	bytesSent := int64(0)
	var lastFullFrame []byte // Cache last full frame for fallback

	for m.isStreaming {
		<-ticker.C

		if m.dataChannel == nil || m.dataChannel.ReadyState() != webrtc.DataChannelStateOpen {
			continue
		}

		// Switch to input desktop before capture (important for Session 0/login screen)
		if m.isSession0 {
			desktop.SwitchToInputDesktop()
		}

		// Check if data channel is backed up (buffered amount > 8MB)
		if m.dataChannel.BufferedAmount() > 8*1024*1024 {
			droppedFrames++
			if droppedFrames%10 == 1 {
				log.Printf("⚠️ Network congestion - dropped %d frames", droppedFrames)
			}
			continue
		}

		// Capture as RGBA for dirty region detection
		rgbaFrame, err := m.screenCapturer.CaptureRGBA()
		if err != nil {
			errorCount++
			if errorCount%100 == 1 {
				log.Printf("⚠️ Screen capture error: %v", err)
			}
			// Send last full frame as fallback
			if lastFullFrame != nil {
				m.sendFrameChunked(lastFullFrame)
			}
			continue
		}

		// Detect dirty regions
		dirtyRegions, isFirstFrame := m.dirtyDetector.DetectDirtyRegions(rgbaFrame, 75)

		if isFirstFrame || len(dirtyRegions) == 0 {
			// First frame or no changes - send full frame
			jpeg, err := m.screenCapturer.CaptureJPEG(75)
			if err != nil {
				continue
			}
			lastFullFrame = jpeg

			if isFirstFrame {
				// Send full frame with special header
				if err := m.sendFullFrame(jpeg); err != nil {
					log.Printf("Failed to send full frame: %v", err)
				} else {
					frameCount++
					bytesSent += int64(len(jpeg))
				}
			}
			// If no changes, don't send anything (save bandwidth!)
			continue
		}

		// Send dirty regions
		width, height := m.screenCapturer.GetResolution()
		changePercent := m.dirtyDetector.GetChangePercentage(dirtyRegions, width, height)

		// If more than 50% changed, send full frame instead (more efficient)
		if changePercent > 50 {
			jpeg, err := m.screenCapturer.CaptureJPEG(75)
			if err != nil {
				continue
			}
			lastFullFrame = jpeg
			if err := m.sendFullFrame(jpeg); err != nil {
				log.Printf("Failed to send full frame: %v", err)
			} else {
				frameCount++
				bytesSent += int64(len(jpeg))
			}
		} else {
			// Send only dirty regions
			regionBytes := 0
			for _, region := range dirtyRegions {
				if err := m.sendDirtyRegion(region); err != nil {
					log.Printf("Failed to send dirty region: %v", err)
					break
				}
				regionBytes += len(region.Data)
			}
			frameCount++
			bytesSent += int64(regionBytes)
		}

		// Log every 30 frames (once per second at 30 FPS)
		if frameCount%30 == 0 {
			avgKBPerFrame := float64(bytesSent) / float64(frameCount) / 1024
			log.Printf("📊 Streaming: %d frames | Avg: %.1f KB/frame | Regions: %d (%.1f%% changed) | Errors: %d | Dropped: %d",
				frameCount, avgKBPerFrame, len(dirtyRegions), changePercent, errorCount, droppedFrames)
		}
	}

	log.Printf("🛑 Screen streaming stopped (sent %d frames, %.1f MB total, %d errors, %d dropped)",
		frameCount, float64(bytesSent)/1024/1024, errorCount, droppedFrames)
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
		chunk[0] = frameTypeChunk
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
