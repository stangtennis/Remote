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
	capturer, err := screen.NewCapturer()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize screen capturer: %w", err)
	}

	// Get primary screen dimensions for input mapping
	width, height := capturer.GetResolution()

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
			m.isStreaming = false
		case webrtc.PeerConnectionStateFailed:
			log.Println("❌ WebRTC connection failed")
			m.isStreaming = false
		case webrtc.PeerConnectionStateClosed:
			log.Println("🔒 WebRTC connection closed")
			m.isStreaming = false
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
	ticker := time.NewTicker(50 * time.Millisecond) // ~20 FPS (more realistic)
	defer ticker.Stop()

	log.Println("🎥 Starting screen streaming at 20 FPS...")

	for m.isStreaming {
		<-ticker.C

		if m.dataChannel == nil || m.dataChannel.ReadyState() != webrtc.DataChannelStateOpen {
			continue
		}

		// Capture screen as JPEG
		jpeg, err := m.screenCapturer.CaptureJPEG(60) // Quality 60 (optimized for speed)
		if err != nil {
			log.Printf("Failed to capture screen: %v", err)
			continue
		}

		// Send frame over data channel (with chunking if needed)
		if err := m.sendFrameChunked(jpeg); err != nil {
			log.Printf("Failed to send frame: %v", err)
		}
	}

	log.Println("🛑 Screen streaming stopped")
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
		
		// Create chunk with header: [chunk_index, total_chunks, ...data]
		chunk := make([]byte, 2+len(data[start:end]))
		chunk[0] = byte(i)
		chunk[1] = byte(totalChunks)
		copy(chunk[2:], data[start:end])
		
		if err := m.dataChannel.Send(chunk); err != nil {
			return err
		}
		
		// Small delay between chunks to avoid overwhelming the channel
		time.Sleep(1 * time.Millisecond)
	}
	
	return nil
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
