package webrtc

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/pion/webrtc/v3"
)

// Client represents a WebRTC client for the controller
type Client struct {
	peerConnection       *webrtc.PeerConnection
	dataChannel          *webrtc.DataChannel
	videoTrack           *webrtc.TrackRemote
	onFrame              func([]byte)
	onConnected          func()
	onDisconnected       func()
	onDataChannelMessage func([]byte)
	mu                   sync.Mutex
	connected            bool

	// Frame reassembly
	frameChunks   map[int][][]byte // chunk index -> chunk data
	frameChunksMu sync.Mutex

	// RTT measurement
	lastPingTime time.Time
	lastRTT      time.Duration
	onRTTUpdate  func(time.Duration)
}

// NewClient creates a new WebRTC client
func NewClient() (*Client, error) {
	return &Client{
		connected:   false,
		frameChunks: make(map[int][][]byte),
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
			c.handleDataChannelMessage(msg.Data)
		})
	})

	return nil
}

// CreateOffer creates an SDP offer
func (c *Client) CreateOffer() (string, error) {
	if c.peerConnection == nil {
		return "", fmt.Errorf("peer connection not initialized")
	}

	// Create a data channel to trigger ICE gathering
	// The agent will use this channel to send video frames
	dc, err := c.peerConnection.CreateDataChannel("data", nil)
	if err != nil {
		return "", fmt.Errorf("failed to create data channel: %w", err)
	}
	c.dataChannel = dc

	// Add OnOpen handler to see when the channel opens
	dc.OnOpen(func() {
		log.Println("📡 Data channel OPENED (controller side)")
	})

	// Add OnMessage handler
	dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		c.handleDataChannelMessage(msg.Data)
	})

	log.Println("📡 Data channel created")

	offer, err := c.peerConnection.CreateOffer(nil)
	if err != nil {
		return "", fmt.Errorf("failed to create offer: %w", err)
	}

	if err := c.peerConnection.SetLocalDescription(offer); err != nil {
		return "", fmt.Errorf("failed to set local description: %w", err)
	}

	log.Println("⏳ Waiting for ICE gathering to complete...")

	// Wait for ICE gathering to complete
	gatherComplete := webrtc.GatheringCompletePromise(c.peerConnection)
	<-gatherComplete

	log.Println("✅ ICE gathering complete!")

	// Get the complete offer with all ICE candidates
	completeOffer := c.peerConnection.LocalDescription()
	log.Printf("📊 Complete offer SDP length: %d", len(completeOffer.SDP))
	log.Printf("📊 Full SDP:\n%s", completeOffer.SDP)

	offerJSON, err := json.Marshal(completeOffer)
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

	log.Printf("📥 Received answer length: %d", len(answerJSON))
	log.Printf("📥 First 100 chars: %.100s", answerJSON)

	var answer webrtc.SessionDescription
	if err := json.Unmarshal([]byte(answerJSON), &answer); err != nil {
		log.Printf("❌ Failed to unmarshal. Full answer: %s", answerJSON)
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

// SendData sends raw bytes over the data channel
func (c *Client) SendData(data []byte) error {
	if c.dataChannel == nil || c.dataChannel.ReadyState() != webrtc.DataChannelStateOpen {
		return fmt.Errorf("data channel not ready")
	}

	return c.dataChannel.Send(data)
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

// handleDataChannelMessage processes incoming data channel messages with chunk reassembly
func (c *Client) handleDataChannelMessage(data []byte) {
	const chunkMagic = 0xFF

	// Check if this is a chunked frame
	if len(data) >= 3 && data[0] == chunkMagic {
		// Extract chunk metadata
		chunkIndex := int(data[1])
		totalChunks := int(data[2])
		chunkData := data[3:]

		c.frameChunksMu.Lock()

		// Initialize chunk array if needed
		if c.frameChunks[totalChunks] == nil {
			c.frameChunks[totalChunks] = make([][]byte, totalChunks)
		}

		// Store this chunk
		c.frameChunks[totalChunks][chunkIndex] = chunkData

		// Check if we have all chunks
		allChunks := true
		for i := 0; i < totalChunks; i++ {
			if c.frameChunks[totalChunks][i] == nil {
				allChunks = false
				break
			}
		}

		if allChunks {
			// Reassemble the frame
			var completeFrame []byte
			for i := 0; i < totalChunks; i++ {
				completeFrame = append(completeFrame, c.frameChunks[totalChunks][i]...)
			}

			// Clear chunks for this frame
			delete(c.frameChunks, totalChunks)
			c.frameChunksMu.Unlock()

			// Send complete frame
			if c.onFrame != nil {
				c.onFrame(completeFrame)
			}
		} else {
			c.frameChunksMu.Unlock()
		}
		return
	}

	// Not a chunked frame - try to parse as JSON first (for clipboard and other messages)
	var jsonMsg map[string]interface{}
	if err := json.Unmarshal(data, &jsonMsg); err == nil {
		// Check for pong response (RTT measurement)
		if msgType, ok := jsonMsg["t"].(string); ok && msgType == "pong" {
			if !c.lastPingTime.IsZero() {
				c.lastRTT = time.Since(c.lastPingTime)
				if c.onRTTUpdate != nil {
					c.onRTTUpdate(c.lastRTT)
				}
			}
			return
		}

		// It's a JSON message (clipboard, file transfer, etc.)
		if c.onDataChannelMessage != nil {
			c.onDataChannelMessage(data)
		}
	} else {
		// It's binary data (JPEG frame sent as single message)
		if c.onFrame != nil {
			c.onFrame(data)
		}
	}
}

// StartPingLoop starts sending ping messages for RTT measurement
func (c *Client) StartPingLoop() {
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			c.mu.Lock()
			connected := c.connected
			c.mu.Unlock()

			if !connected {
				return
			}

			c.SendPing()
		}
	}()
}

// SendPing sends a ping message to measure RTT
func (c *Client) SendPing() {
	if c.dataChannel == nil || c.dataChannel.ReadyState() != webrtc.DataChannelStateOpen {
		return
	}

	c.lastPingTime = time.Now()
	ping := map[string]interface{}{
		"t":  "ping",
		"ts": float64(c.lastPingTime.UnixNano()) / 1e6, // ms timestamp
	}

	if data, err := json.Marshal(ping); err == nil {
		c.dataChannel.Send(data)
	}
}

// SetOnRTTUpdate sets the callback for RTT updates
func (c *Client) SetOnRTTUpdate(callback func(time.Duration)) {
	c.onRTTUpdate = callback
}

// GetLastRTT returns the last measured RTT
func (c *Client) GetLastRTT() time.Duration {
	return c.lastRTT
}
