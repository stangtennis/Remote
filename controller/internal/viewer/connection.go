package viewer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"log"
	"time"

	"github.com/pion/webrtc/v3"
	rtc "github.com/stangtennis/Remote/controller/internal/webrtc"
)

// ConnectWebRTC initiates a WebRTC connection to the remote device
func (v *Viewer) ConnectWebRTC(supabaseURL, anonKey, authToken, userID string) error {
	log.Printf("🔗 Initiating WebRTC connection to device: %s", v.deviceID)
	
	// Create WebRTC client
	client, err := rtc.NewClient()
	if err != nil {
		return fmt.Errorf("failed to create WebRTC client: %w", err)
	}
	
	// Store client reference for input forwarding
	v.webrtcClient = client

	// Create signaling client
	signalingClient := rtc.NewSignalingClient(supabaseURL, anonKey, authToken)

	// Set up callbacks
	client.SetOnFrame(func(frameData []byte) {
		v.handleVideoFrame(frameData)
	})

	client.SetOnConnected(func() {
		log.Println("✅ WebRTC connected!")
		v.connected = true
		v.statusLabel.SetText("🟢 Connected")
		
		// Enable input forwarding
		v.setupInputForwarding()
	})

	client.SetOnDisconnected(func() {
		log.Println("❌ WebRTC disconnected")
		v.connected = false
		v.statusLabel.SetText("🔴 Disconnected")
	})

	// Create peer connection with STUN servers
	iceServers := []webrtc.ICEServer{
		{URLs: []string{"stun:stun.l.google.com:19302"}},
		{URLs: []string{"stun:stun1.l.google.com:19302"}},
	}

	if err := client.CreatePeerConnection(iceServers); err != nil {
		return fmt.Errorf("failed to create peer connection: %w", err)
	}

	// Create session in Supabase
	log.Println("📝 Creating WebRTC session...")
	session, err := signalingClient.CreateSession(v.deviceID, userID)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	log.Printf("✅ Session created: %s", session.SessionID)

	// Create offer
	log.Println("📤 Creating WebRTC offer...")
	offerJSON, err := client.CreateOffer()
	if err != nil {
		return fmt.Errorf("failed to create offer: %w", err)
	}

	// Send offer to Supabase
	log.Println("📤 Sending offer to agent...")
	if err := signalingClient.SendOffer(session.SessionID, offerJSON); err != nil {
		return fmt.Errorf("failed to send offer: %w", err)
	}

	// Wait for answer from agent
	log.Println("⏳ Waiting for answer from agent...")
	v.statusLabel.SetText("⏳ Waiting for agent...")
	
	go func() {
		answerJSON, err := signalingClient.WaitForAnswer(session.SessionID, 30*time.Second)
		if err != nil {
			log.Printf("❌ Failed to get answer: %v", err)
			v.statusLabel.SetText("❌ Connection failed")
			return
		}

		log.Println("📨 Received answer from agent")

		// Set answer
		if err := client.SetAnswer(answerJSON); err != nil {
			log.Printf("❌ Failed to set answer: %v", err)
			v.statusLabel.SetText("❌ Connection failed")
			return
		}

		log.Println("🎉 WebRTC handshake complete, waiting for connection...")
	}()

	return nil
}

// handleVideoFrame processes incoming video frames
func (v *Viewer) handleVideoFrame(frameData []byte) {
	// Decode JPEG frame
	img, err := jpeg.Decode(bytes.NewReader(frameData))
	if err != nil {
		log.Printf("⚠️  Failed to decode frame: %v", err)
		return
	}

	// Update canvas on UI thread
	v.updateCanvas(img)
	
	// Update FPS counter
	v.updateFPS()
}

var lastFrameTime time.Time
var frameCount int
var currentFPS int

// updateFPS calculates and updates the FPS counter
func (v *Viewer) updateFPS() {
	frameCount++
	
	now := time.Now()
	if lastFrameTime.IsZero() {
		lastFrameTime = now
		return
	}

	elapsed := now.Sub(lastFrameTime)
	if elapsed >= time.Second {
		currentFPS = frameCount
		frameCount = 0
		lastFrameTime = now
		
		// Update FPS label
		v.fpsLabel.SetText(fmt.Sprintf("FPS: %d", currentFPS))
	}
}

// updateCanvas updates the video canvas with a new frame
func (v *Viewer) updateCanvas(img image.Image) {
	// Convert to canvas image
	v.videoCanvas.Image = img
	v.videoCanvas.Refresh()
}

// setupInputForwarding configures input event forwarding to the agent
func (v *Viewer) setupInputForwarding() {
	log.Println("🎮 Setting up input forwarding...")
	
	// Create input handler if not exists
	if v.inputHandler == nil {
		v.inputHandler = NewInputHandler(v)
	}
	
	// Set up mouse move callback
	v.inputHandler.SetOnMouseMove(func(x, y float32) {
		v.SendMouseMove(x, y)
	})
	
	// Set up mouse button callback
	v.inputHandler.SetOnMouseButton(func(button int, pressed bool) {
		v.SendMouseButton(button, pressed)
	})
	
	// Set up mouse scroll callback
	v.inputHandler.SetOnMouseScroll(func(deltaX, deltaY float32) {
		v.SendMouseScroll(deltaX, deltaY)
	})
	
	// Set up keyboard callback
	v.inputHandler.SetOnKeyPress(func(key string, pressed bool) {
		v.SendKeyPress(key, pressed)
	})
	
	// Attach to canvas
	v.inputHandler.AttachToCanvas()
	
	log.Println("✅ Input forwarding enabled")
}

// SendMouseMove sends a mouse move event to the agent
func (v *Viewer) SendMouseMove(x, y float32) {
	if v.webrtcClient == nil {
		return
	}
	
	event := map[string]interface{}{
		"t": "mouse_move",
		"x": x,
		"y": y,
	}

	eventJSON, _ := json.Marshal(event)
	
	// Send via WebRTC data channel
	if client, ok := v.webrtcClient.(*rtc.Client); ok {
		client.SendInput(string(eventJSON))
	}
}

// SendMouseButton sends a mouse button event to the agent
func (v *Viewer) SendMouseButton(button int, pressed bool) {
	if v.webrtcClient == nil {
		return
	}
	
	// Map button number to string
	buttonStr := "left"
	if button == 1 {
		buttonStr = "middle"
	} else if button == 2 {
		buttonStr = "right"
	}
	
	event := map[string]interface{}{
		"t":      "mouse_click",
		"button": buttonStr,
		"down":   pressed,
	}

	eventJSON, _ := json.Marshal(event)
	
	if client, ok := v.webrtcClient.(*rtc.Client); ok {
		client.SendInput(string(eventJSON))
	}
}

// SendMouseScroll sends a mouse scroll event to the agent
func (v *Viewer) SendMouseScroll(deltaX, deltaY float32) {
	if v.webrtcClient == nil {
		return
	}
	
	event := map[string]interface{}{
		"t":     "mouse_scroll",
		"delta": deltaY, // Use Y delta for vertical scrolling
	}

	eventJSON, _ := json.Marshal(event)
	
	if client, ok := v.webrtcClient.(*rtc.Client); ok {
		client.SendInput(string(eventJSON))
	}
}

// SendKeyPress sends a keyboard event to the agent
func (v *Viewer) SendKeyPress(key string, pressed bool) {
	if v.webrtcClient == nil {
		return
	}
	
	event := map[string]interface{}{
		"t":    "key",
		"code": key,
		"down": pressed,
	}

	eventJSON, _ := json.Marshal(event)
	
	if client, ok := v.webrtcClient.(*rtc.Client); ok {
		client.SendInput(string(eventJSON))
	}
}
