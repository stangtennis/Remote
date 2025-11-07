package webrtc

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"github.com/pion/webrtc/v3"
)

// Client represents a WebRTC client for the controller
type Client struct {
	peerConnection      *webrtc.PeerConnection
	dataChannel         *webrtc.DataChannel
	videoTrack          *webrtc.TrackRemote
	onFrame             func([]byte)
	onConnected         func()
	onDisconnected      func()
	onDataChannelMessage func([]byte)
	mu                  sync.Mutex
	connected           bool
}

// NewClient creates a new WebRTC client
func NewClient() (*Client, error) {
	return &Client{
		connected: false,
	}, nil
}

// CreatePeerConnection initializes the peer connection
func (c *Client) CreatePeerConnection(iceServers []webrtc.ICEServer) error {
	config := webrtc.Configuration{
		ICEServers: iceServers,
	}

	pc, err := webrtc.NewPeerConnection(config)
	if err != nil {
		return fmt.Errorf("failed to create peer connection: %w", err)
	}

	c.peerConnection = pc

	// Handle connection state changes
	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		log.Printf("WebRTC connection state: %s", state.String())
		
		switch state {
		case webrtc.PeerConnectionStateConnected:
			log.Println("✅ WebRTC connected!")
			c.mu.Lock()
			c.connected = true
			c.mu.Unlock()
			if c.onConnected != nil {
				c.onConnected()
			}
		case webrtc.PeerConnectionStateDisconnected, webrtc.PeerConnectionStateFailed:
			log.Println("❌ WebRTC disconnected")
			c.mu.Lock()
			c.connected = false
			c.mu.Unlock()
			if c.onDisconnected != nil {
				c.onDisconnected()
			}
		}
	})

	// Handle incoming tracks (video)
	pc.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		log.Printf("📺 Received track: %s", track.Kind().String())
		c.videoTrack = track
		
		// This would be for RTP video, but we're using data channel for JPEG
		// Keep for future H.264 implementation
	})

	// Handle data channel from remote
	pc.OnDataChannel(func(dc *webrtc.DataChannel) {
		log.Printf("📡 Data channel opened: %s", dc.Label())
		c.dataChannel = dc

		dc.OnMessage(func(msg webrtc.DataChannelMessage) {
			// Try to parse as JSON first (for clipboard and other messages)
			var jsonMsg map[string]interface{}
			if err := json.Unmarshal(msg.Data, &jsonMsg); err == nil {
				// It's a JSON message (clipboard, file transfer, etc.)
				if c.onDataChannelMessage != nil {
					c.onDataChannelMessage(msg.Data)
				}
			} else {
				// It's binary data (JPEG frame)
				if c.onFrame != nil {
					c.onFrame(msg.Data)
				}
			}
		})
	})

	return nil
}

// CreateOffer creates an SDP offer
func (c *Client) CreateOffer() (string, error) {
	if c.peerConnection == nil {
		return "", fmt.Errorf("peer connection not initialized")
	}

	offer, err := c.peerConnection.CreateOffer(nil)
	if err != nil {
		return "", fmt.Errorf("failed to create offer: %w", err)
	}

	if err := c.peerConnection.SetLocalDescription(offer); err != nil {
		return "", fmt.Errorf("failed to set local description: %w", err)
	}

	offerJSON, err := json.Marshal(offer)
	if err != nil {
		return "", fmt.Errorf("failed to marshal offer: %w", err)
	}

	return string(offerJSON), nil
}

// SetAnswer sets the remote SDP answer
func (c *Client) SetAnswer(answerJSON string) error {
	if c.peerConnection == nil {
		return fmt.Errorf("peer connection not initialized")
	}

	var answer webrtc.SessionDescription
	if err := json.Unmarshal([]byte(answerJSON), &answer); err != nil {
		return fmt.Errorf("failed to unmarshal answer: %w", err)
	}

	if err := c.peerConnection.SetRemoteDescription(answer); err != nil {
		return fmt.Errorf("failed to set remote description: %w", err)
	}

	return nil
}

// SendInput sends mouse/keyboard input to the agent
func (c *Client) SendInput(inputJSON string) error {
	if c.dataChannel == nil || c.dataChannel.ReadyState() != webrtc.DataChannelStateOpen {
		return fmt.Errorf("data channel not ready")
	}

	return c.dataChannel.SendText(inputJSON)
}

// SetOnFrame sets the callback for received video frames
func (c *Client) SetOnFrame(callback func([]byte)) {
	c.onFrame = callback
}

// SetOnConnected sets the callback for connection established
func (c *Client) SetOnConnected(callback func()) {
	c.onConnected = callback
}

// SetOnDisconnected sets the callback for disconnection
func (c *Client) SetOnDisconnected(callback func()) {
	c.onDisconnected = callback
}

// SetOnDataChannelMessage sets the callback for data channel messages
func (c *Client) SetOnDataChannelMessage(callback func([]byte)) {
	c.onDataChannelMessage = callback
}

// IsConnected returns the connection status
func (c *Client) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.connected
}

// Close closes the WebRTC connection
func (c *Client) Close() error {
	if c.peerConnection != nil {
		return c.peerConnection.Close()
	}
	return nil
}
