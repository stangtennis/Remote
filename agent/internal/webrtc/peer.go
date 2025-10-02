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

	width, height := capturer.GetResolution()

	return &Manager{
		cfg:             cfg,
		device:          dev,
		screenCapturer:  capturer,
		mouseController: input.NewMouseController(width, height),
		keyController:   input.NewKeyboardController(),
		isStreaming: false,
	}, nil
}

func (m *Manager) CreatePeerConnection(iceServers []webrtc.ICEServer) error {
	config := webrtc.Configuration{
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
		m.mouseController.Move(x, y)

	case "mouse_click":
		button, _ := event["button"].(string)
		down, _ := event["down"].(bool)
		m.mouseController.Click(button, down)

	case "key":
		code, _ := event["code"].(string)
		down, _ := event["down"].(bool)
		m.keyController.SendKey(code, down)
	}
}

func (m *Manager) startScreenStreaming() {
	// Stream JPEG frames over data channel
	ticker := time.NewTicker(33 * time.Millisecond) // ~30 FPS
	defer ticker.Stop()

	log.Println("🎥 Starting screen streaming...")

	for m.isStreaming {
		<-ticker.C

		if m.dataChannel == nil || m.dataChannel.ReadyState() != webrtc.DataChannelStateOpen {
			continue
		}

		// Capture screen as JPEG
		jpeg, err := m.screenCapturer.CaptureJPEG(60) // Quality 60
		if err != nil {
			log.Printf("Failed to capture screen: %v", err)
			continue
		}

		// Send frame over data channel
		if err := m.dataChannel.Send(jpeg); err != nil {
			log.Printf("Failed to send frame: %v", err)
		}
	}

	log.Println("🛑 Screen streaming stopped")
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
