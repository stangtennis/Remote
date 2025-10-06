package webrtc

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/pion/webrtc/v3"
	"github.com/stangtennis/remote-agent/internal/config"
	"github.com/stangtennis/remote-agent/internal/device"
	"github.com/stangtennis/remote-agent/internal/input"
	"github.com/stangtennis/remote-agent/internal/screen"
)

type Manager struct {
	cfg             *config.Config
	device          *device.Device
	peerConnection  *webrtc.PeerConnection
	dataChannel     *webrtc.DataChannel
	screenCapturer  *screen.Capturer
	mouseController *input.MouseController
	keyController   *input.KeyboardController
	sessionID       string
	isStreaming     bool
}

func New(cfg *config.Config, dev *device.Device) (*Manager, error) {
	// Try to initialize screen capturer, but don't fail if it's not available
	// (happens in Session 0 / before user login)
	capturer, err := screen.NewCapturer()
	if err != nil {
		log.Printf("⚠️  Screen capturer not available: %v", err)
		log.Println("   This is normal before user login (Session 0)")
		log.Println("   Screen capture will be initialized on first connection")
	}

	// Get screen dimensions for input mapping (default to 1920x1080 if capturer not available)
	width, height := 1920, 1080
	if capturer != nil {
		width, height = capturer.GetResolution()
		log.Printf("✅ Screen capturer initialized: %dx%d", width, height)
	}

	return &Manager{
		cfg:             cfg,
		device:          dev,
		screenCapturer:  capturer,
		mouseController: input.NewMouseController(width, height),
		keyController:   input.NewKeyboardController(),
	}, nil
}

func (m *Manager) CreatePeerConnection(iceServers []webrtc.ICEServer) error {
	config := webrtc.Configuration{
		ICEServers: iceServers,
	}

	pc, err := webrtc.NewPeerConnection(config)
	if err != nil {
		return fmt.Errorf("failed to create peer connection: %w", err)
	}

	m.peerConnection = pc

	// Set up connection state handler
	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		log.Printf("Connection state: %s", state.String())
		
		switch state {
		case webrtc.PeerConnectionStateConnected:
			log.Println("✅ WebRTC connected!")
			m.isStreaming = true
			go m.startScreenStreaming()
		case webrtc.PeerConnectionStateDisconnected:
			log.Println("⚠️  WebRTC disconnected")
			m.cleanupConnection("Disconnected")
		case webrtc.PeerConnectionStateFailed:
			log.Println("❌ WebRTC connection failed")
			m.cleanupConnection("Failed")
		case webrtc.PeerConnectionStateClosed:
			log.Println("🔒 WebRTC connection closed")
			m.cleanupConnection("Closed")
		}
	})

	// Set up ICE candidate handler
	pc.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate != nil {
			log.Printf("📤 Generated ICE candidate: %s", candidate.Typ.String())
			m.sendICECandidate(candidate)
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
		log.Println("✅ Data channel ready")
	})

	dc.OnClose(func() {
		log.Println("❌ Data channel closed")
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
	eventType, ok := event["t"].(string)
	if !ok {
		return
	}

	switch eventType {
	case "mouse_move":
		x, _ := event["x"].(float64)
		y, _ := event["y"].(float64)
		if err := m.mouseController.Move(x, y); err != nil {
			log.Printf("Mouse move error: %v", err)
		}

	case "mouse_click":
		button, _ := event["button"].(string)
		down, _ := event["down"].(bool)
		if err := m.mouseController.Click(button, down); err != nil {
			log.Printf("Mouse click error: %v", err)
		}

	case "mouse_scroll":
		delta, _ := event["delta"].(float64)
		if err := m.mouseController.Scroll(int(delta)); err != nil {
			log.Printf("Mouse scroll error: %v", err)
		}

	case "key":
		code, _ := event["code"].(string)
		down, _ := event["down"].(bool)
		if err := m.keyController.SendKey(code, down); err != nil {
			log.Printf("Key event error: %v", err)
		}
	}
}

func (m *Manager) startScreenStreaming() {
	// Stream JPEG frames over data channel
	// 15 FPS (66ms) = smooth experience with good bandwidth
	ticker := time.NewTicker(66 * time.Millisecond)
	defer ticker.Stop()

	log.Println("🎥 Starting screen streaming at 15 FPS (high quality)...")
	
	// If screen capturer not initialized (Session 0), try to initialize now
	if m.screenCapturer == nil {
		log.Println("⚠️  Screen capturer not initialized, attempting to initialize now...")
		capturer, err := screen.NewCapturer()
		if err != nil {
			log.Printf("❌ Failed to initialize screen capturer: %v", err)
			log.Println("   Cannot stream screen - user might need to log in first")
			return
		}
		m.screenCapturer = capturer
		log.Println("✅ Screen capturer initialized successfully!")
		
		// Update screen dimensions
		width, height := capturer.GetResolution()
		m.mouseController = input.NewMouseController(width, height)
		log.Printf("✅ Updated screen resolution: %dx%d", width, height)
	}

	frameCount := 0
	errorCount := 0
	consecutiveErrors := 0
	
	for m.isStreaming {
		<-ticker.C

		if m.dataChannel == nil || m.dataChannel.ReadyState() != webrtc.DataChannelStateOpen {
			continue
		}

		// Capture every frame with high quality
		jpeg, err := m.screenCapturer.CaptureJPEG(75) // Quality 75 (high quality for good bandwidth)
		if err != nil {
			errorCount++
			consecutiveErrors++
			
			// Only log every 50th error to avoid spam, but ALWAYS show the error message
			if errorCount%50 == 1 {
				log.Printf("⚠️ Screen capture failing (total: %d errors) - Error: %v", errorCount, err)
			}
			
			// If too many consecutive errors, something is seriously wrong
			if consecutiveErrors > 100 {
				log.Printf("❌ Too many consecutive capture failures (%d), stopping stream", consecutiveErrors)
				break
			}
			
			continue
		}
		
		// Reset consecutive error counter on success
		consecutiveErrors = 0

		// Send frame over data channel (with chunking if needed)
		if err := m.sendFrameChunked(jpeg); err != nil {
			log.Printf("Failed to send frame: %v", err)
		} else {
			frameCount++
			// Log every 100 frames instead of 50 to reduce logging overhead
			if frameCount%100 == 0 {
				log.Printf("📊 Sent %d frames (latest size: %d KB, %d errors)", frameCount, len(jpeg)/1024, errorCount)
			}
		}
	}

	log.Printf("🛑 Screen streaming stopped (sent %d frames, %d errors)", frameCount, errorCount)
}

func (m *Manager) sendFrameChunked(data []byte) error {
	const maxChunkSize = 60000 // 60KB chunks (safely under 64KB limit)
	const chunkMagic = 0xFF // Magic byte to identify chunked frames
	
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
		
		// Small delay between chunks to avoid overwhelming the channel
		time.Sleep(1 * time.Millisecond)
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
