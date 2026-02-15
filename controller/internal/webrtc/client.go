package webrtc

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/pion/interceptor"
	"github.com/pion/interceptor/pkg/intervalpli"
	"github.com/pion/rtp/codecs"
	"github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/pkg/media/samplebuilder"
)

// Client represents a WebRTC client for the controller
type Client struct {
	peerConnection       *webrtc.PeerConnection
	dataChannel          *webrtc.DataChannel
	controlChannel       *webrtc.DataChannel // Separate channel for input (low latency)
	fileChannel          *webrtc.DataChannel // Reliable channel for file transfer
	videoTrack           *webrtc.TrackRemote
	onFrame              func([]byte)
	onH264Frame          func([]byte) // Callback for decoded H.264 frames
	onConnected          func()
	onDisconnected       func()
	onDataChannelMessage func([]byte)
	onFileMessage        func([]byte) // Callback for file transfer messages
	mu                   sync.Mutex
	connected            bool

	// Frame reassembly with timeout tracking
	frameChunks    map[int][][]byte  // frameID -> chunk data
	frameFirstSeen map[int]time.Time // frameID -> first chunk arrival time
	frameChunksMu  sync.Mutex

	// RTT measurement
	lastPingTime time.Time
	lastRTT      time.Duration
	onRTTUpdate  func(time.Duration)

	// H.264 decoding
	h264Decoder   *H264Decoder
	useH264       bool
	h264Receiving bool
}

// NewClient creates a new WebRTC client
func NewClient() (*Client, error) {
	return &Client{
		connected:      false,
		frameChunks:    make(map[int][][]byte),
		frameFirstSeen: make(map[int]time.Time),
	}, nil
}

// CreatePeerConnection initializes the peer connection with H.264 video support
func (c *Client) CreatePeerConnection(iceServers []webrtc.ICEServer) error {
	// Create MediaEngine with H.264 codec support
	m := &webrtc.MediaEngine{}

	// Register H.264 codec (baseline profile for compatibility)
	if err := m.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{
			MimeType:    webrtc.MimeTypeH264,
			ClockRate:   90000,
			SDPFmtpLine: "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f",
		},
		PayloadType: 96,
	}, webrtc.RTPCodecTypeVideo); err != nil {
		log.Printf("⚠️ Failed to register H.264 codec: %v", err)
	}

	// Create interceptor registry for PLI (Picture Loss Indication)
	i := &interceptor.Registry{}

	// Add PLI interceptor to request keyframes
	pliFactory, err := intervalpli.NewReceiverInterceptor()
	if err != nil {
		log.Printf("⚠️ Failed to create PLI interceptor: %v", err)
	} else {
		i.Add(pliFactory)
	}

	// Use default interceptors
	if err := webrtc.RegisterDefaultInterceptors(m, i); err != nil {
		log.Printf("⚠️ Failed to register default interceptors: %v", err)
	}

	// Create API with custom MediaEngine
	api := webrtc.NewAPI(webrtc.WithMediaEngine(m), webrtc.WithInterceptorRegistry(i))

	config := webrtc.Configuration{
		ICEServers: iceServers,
	}

	// Use custom API to create peer connection
	pc, err := api.NewPeerConnection(config)
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

	// Handle incoming tracks (H.264 video)
	pc.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		codecMime := track.Codec().MimeType
		log.Printf("📺 OnTrack CALLED! Kind: %s, Codec: %s, SSRC: %d, PayloadType: %d",
			track.Kind().String(), codecMime, track.SSRC(), track.PayloadType())

		if track.Kind() == webrtc.RTPCodecTypeVideo {
			// Only start H.264 decoder for H.264 tracks
			if codecMime == webrtc.MimeTypeH264 {
				c.videoTrack = track
				c.h264Receiving = true
				log.Println("🎬 H.264 video track received - starting decoder goroutine NOW")

				// Start H.264 RTP receiver goroutine
				go c.receiveH264Track(track)
			} else {
				log.Printf("⚠️ Ignoring non-H.264 video track: %s", codecMime)
			}
		} else {
			log.Printf("⚠️ Ignoring non-video track: %s", track.Kind().String())
		}
	})

	// Handle data channel from remote - route by label like agent does
	pc.OnDataChannel(func(dc *webrtc.DataChannel) {
		log.Printf("📡 Data channel opened: %s", dc.Label())

		// Route to appropriate handler based on channel label
		switch dc.Label() {
		case "control":
			log.Println("🎮 Control channel ready (low-latency input)")
			c.controlChannel = dc
			dc.OnMessage(func(msg webrtc.DataChannelMessage) {
				c.handleDataChannelMessage(msg.Data)
			})
		case "video":
			log.Println("🎬 Video channel ready (unreliable, low-latency)")
			dc.OnMessage(func(msg webrtc.DataChannelMessage) {
				c.handleDataChannelMessage(msg.Data)
			})
		default:
			// Default data channel for general messages
			c.dataChannel = dc
			dc.OnMessage(func(msg webrtc.DataChannelMessage) {
				c.handleDataChannelMessage(msg.Data)
			})
		}
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

	// Create control channel for low-latency input (unordered, no retransmits)
	ordered := false
	maxRetransmits := uint16(0)
	controlOpts := &webrtc.DataChannelInit{
		Ordered:        &ordered,
		MaxRetransmits: &maxRetransmits,
	}
	cc, err := c.peerConnection.CreateDataChannel("control", controlOpts)
	if err != nil {
		log.Printf("⚠️ Failed to create control channel: %v (falling back to data channel)", err)
	} else {
		c.controlChannel = cc
		cc.OnOpen(func() {
			log.Println("🎮 Control channel OPENED (low-latency input)")
		})
		cc.OnMessage(func(msg webrtc.DataChannelMessage) {
			c.handleDataChannelMessage(msg.Data)
		})
		log.Println("🎮 Control channel created (ordered=false, maxRetransmits=0)")
	}

	// Create video channel for low-latency video (unreliable, unordered)
	// This avoids head-of-line blocking when packets are lost
	videoOpts := &webrtc.DataChannelInit{
		Ordered:        &ordered,
		MaxRetransmits: &maxRetransmits,
	}
	vc, err := c.peerConnection.CreateDataChannel("video", videoOpts)
	if err != nil {
		log.Printf("⚠️ Failed to create video channel: %v (using data channel for video)", err)
	} else {
		vc.OnOpen(func() {
			log.Println("🎬 Video channel OPENED (unreliable, low-latency)")
		})
		vc.OnMessage(func(msg webrtc.DataChannelMessage) {
			c.handleDataChannelMessage(msg.Data)
		})
		log.Println("🎬 Video channel created (ordered=false, maxRetransmits=0)")
	}

	// Create file channel for reliable file transfer (ordered, reliable)
	fileOrdered := true
	fileOpts := &webrtc.DataChannelInit{
		Ordered: &fileOrdered,
	}
	fc, err := c.peerConnection.CreateDataChannel("file", fileOpts)
	if err != nil {
		log.Printf("⚠️ Failed to create file channel: %v", err)
	} else {
		c.fileChannel = fc
		fc.OnOpen(func() {
			log.Println("📁 File channel OPENED (reliable, ordered)")
		})
		fc.OnMessage(func(msg webrtc.DataChannelMessage) {
			if c.onFileMessage != nil {
				c.onFileMessage(msg.Data)
			}
		})
		log.Println("📁 File channel created (ordered=true, reliable)")
	}

	// Add video transceiver for H.264 (recvonly) - enables agent to send video track
	// This is critical for H.264 support without renegotiation
	_, err = c.peerConnection.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo, webrtc.RTPTransceiverInit{
		Direction: webrtc.RTPTransceiverDirectionRecvonly,
	})
	if err != nil {
		log.Printf("⚠️ Failed to add video transceiver: %v", err)
	} else {
		log.Println("📺 Video transceiver added (recvonly) - H.264 ready")
	}

	offer, err := c.peerConnection.CreateOffer(nil)
	if err != nil {
		return "", fmt.Errorf("failed to create offer: %w", err)
	}

	if err := c.peerConnection.SetLocalDescription(offer); err != nil {
		return "", fmt.Errorf("failed to set local description: %w", err)
	}

	// Semi-trickle ICE: Wait max 5 seconds for ICE gathering, then proceed
	// 5s gives TURN/relay candidates time to gather (important for NAT traversal)
	// while still being faster than full gathering which can take 10-30s
	log.Println("⏳ ICE gathering (semi-trickle, max 5s)...")

	gatherComplete := webrtc.GatheringCompletePromise(c.peerConnection)
	select {
	case <-gatherComplete:
		log.Println("✅ ICE gathering complete!")
	case <-time.After(5 * time.Second):
		log.Println("⚡ Semi-trickle ICE: Proceeding with gathered candidates (5s timeout)")
	}

	// Get the offer with gathered ICE candidates
	completeOffer := c.peerConnection.LocalDescription()
	log.Printf("📊 Offer SDP length: %d", len(completeOffer.SDP))

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

// SendInput sends mouse/keyboard input to the agent via control channel (low latency)
func (c *Client) SendInput(inputJSON string) error {
	// Prefer control channel for low-latency input
	if c.controlChannel != nil && c.controlChannel.ReadyState() == webrtc.DataChannelStateOpen {
		return c.controlChannel.SendText(inputJSON)
	}

	// Fallback to data channel
	if c.dataChannel == nil || c.dataChannel.ReadyState() != webrtc.DataChannelStateOpen {
		return fmt.Errorf("no channel ready for input")
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

// SetStreamingMode sets the streaming mode on the agent
// mode: "tiles" | "h264" | "hybrid"
// bitrate: target bitrate in kbps (0 = use default)
func (c *Client) SetStreamingMode(mode string, bitrate int) error {
	log.Printf("🎬 SetStreamingMode called: mode=%s, bitrate=%d", mode, bitrate)
	
	msg := map[string]interface{}{
		"type": "set_mode",
		"mode": mode,
	}
	if bitrate > 0 {
		msg["bitrate"] = float64(bitrate)
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal mode message: %w", err)
	}

	log.Printf("🎬 Sending set_mode message to agent: %s", string(data))
	err = c.SendData(data)
	if err != nil {
		log.Printf("❌ Failed to send set_mode: %v", err)
	} else {
		log.Printf("✅ set_mode message sent successfully")
	}
	return err
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

// SetOnFileMessage sets the callback for file transfer messages
func (c *Client) SetOnFileMessage(callback func([]byte)) {
	c.onFileMessage = callback
}

// SendFileData sends data over the file channel
func (c *Client) SendFileData(data []byte) error {
	if c.fileChannel == nil || c.fileChannel.ReadyState() != webrtc.DataChannelStateOpen {
		return fmt.Errorf("file channel not ready")
	}
	return c.fileChannel.Send(data)
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

// stripFrameHeader removes frame type markers and returns the JPEG data
func stripFrameHeader(data []byte) []byte {
	const frameTypeFull = 0x01   // Full frame JPEG with 4-byte header
	const frameTypeRegion = 0x02 // Dirty region update with 9-byte header

	if len(data) > 4 && data[0] == frameTypeFull {
		return data[4:]
	}
	if len(data) > 9 && data[0] == frameTypeRegion {
		return data[9:]
	}
	return data
}

// handleDataChannelMessage processes incoming data channel messages with chunk reassembly
func (c *Client) handleDataChannelMessage(data []byte) {
	const chunkMagicOld = 0xFF // Old format: [magic, chunk_index, total_chunks, data]
	const chunkMagicNew = 0xFE // New format: [magic, frame_id_hi, frame_id_lo, chunk_index, total_chunks, data]

	// Check for new chunk format with frame ID (more robust)
	if len(data) >= 5 && data[0] == chunkMagicNew {
		frameID := int(data[1])<<8 | int(data[2])
		chunkIndex := int(data[3])
		totalChunks := int(data[4])
		chunkData := data[5:]

		c.frameChunksMu.Lock()

		// Use frameID as key for chunk storage (prevents mixing chunks from different frames)
		if c.frameChunks[frameID] == nil {
			c.frameChunks[frameID] = make([][]byte, totalChunks)
			c.frameFirstSeen[frameID] = time.Now()
		}

		// Store this chunk
		if chunkIndex < len(c.frameChunks[frameID]) {
			c.frameChunks[frameID][chunkIndex] = chunkData
		}

		// Check if we have all chunks
		allChunks := true
		for i := 0; i < totalChunks; i++ {
			if i >= len(c.frameChunks[frameID]) || c.frameChunks[frameID][i] == nil {
				allChunks = false
				break
			}
		}

		if allChunks {
			// Reassemble the frame
			var completeFrame []byte
			for i := 0; i < totalChunks; i++ {
				completeFrame = append(completeFrame, c.frameChunks[frameID][i]...)
			}

			// Clear chunks for this frame
			delete(c.frameChunks, frameID)
			delete(c.frameFirstSeen, frameID)

			// GC: Drop incomplete frames older than 200ms (prevents freezes)
			now := time.Now()
			for id, firstSeen := range c.frameFirstSeen {
				if now.Sub(firstSeen) > 200*time.Millisecond {
					delete(c.frameChunks, id)
					delete(c.frameFirstSeen, id)
				}
			}
			c.frameChunksMu.Unlock()

			// Send complete frame (strip frame type header if present)
			if c.onFrame != nil {
				c.onFrame(stripFrameHeader(completeFrame))
			}
		} else {
			c.frameChunksMu.Unlock()
		}
		return
	}

	// Check for old chunk format (backwards compatibility)
	if len(data) >= 3 && data[0] == chunkMagicOld {
		chunkIndex := int(data[1])
		totalChunks := int(data[2])
		chunkData := data[3:]

		c.frameChunksMu.Lock()

		// Use totalChunks as key (old behavior)
		if c.frameChunks[totalChunks] == nil {
			c.frameChunks[totalChunks] = make([][]byte, totalChunks)
		}

		c.frameChunks[totalChunks][chunkIndex] = chunkData

		allChunks := true
		for i := 0; i < totalChunks; i++ {
			if c.frameChunks[totalChunks][i] == nil {
				allChunks = false
				break
			}
		}

		if allChunks {
			var completeFrame []byte
			for i := 0; i < totalChunks; i++ {
				completeFrame = append(completeFrame, c.frameChunks[totalChunks][i]...)
			}
			delete(c.frameChunks, totalChunks)

			// GC: Clean up stale incomplete frames (older than 100 frame IDs)
			for id := range c.frameChunks {
				if totalChunks-id > 100 || id-totalChunks > 100 {
					delete(c.frameChunks, id)
				}
			}
			c.frameChunksMu.Unlock()

			// Send complete frame (strip frame type header if present)
			if c.onFrame != nil {
				c.onFrame(stripFrameHeader(completeFrame))
			}
		} else {
			c.frameChunksMu.Unlock()
		}
		return
	}

	// Check for frame type markers (dirty region protocol)
	const frameTypeFull = 0x01   // Full frame JPEG with 4-byte header
	const frameTypeRegion = 0x02 // Dirty region update with 9-byte header

	if len(data) > 4 && data[0] == frameTypeFull {
		// Full frame: [type(1), reserved(3), ...jpeg_data]
		jpegData := data[4:]
		if c.onFrame != nil {
			c.onFrame(jpegData)
		}
		return
	}

	if len(data) > 9 && data[0] == frameTypeRegion {
		// Dirty region: [type(1), x(2), y(2), w(2), h(2), ...jpeg_data]
		// For now, treat as full frame (dirty region compositing not implemented)
		jpegData := data[9:]
		if c.onFrame != nil {
			c.onFrame(jpegData)
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

// SendPing sends a ping message to measure RTT via control channel
func (c *Client) SendPing() {
	c.lastPingTime = time.Now()
	ping := map[string]interface{}{
		"t":  "ping",
		"ts": float64(c.lastPingTime.UnixNano()) / 1e6, // ms timestamp
	}

	data, err := json.Marshal(ping)
	if err != nil {
		return
	}

	// Prefer control channel for accurate RTT measurement
	if c.controlChannel != nil && c.controlChannel.ReadyState() == webrtc.DataChannelStateOpen {
		c.controlChannel.Send(data)
		return
	}

	// Fallback to data channel
	if c.dataChannel != nil && c.dataChannel.ReadyState() == webrtc.DataChannelStateOpen {
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

// receiveH264Track receives H.264 RTP packets and decodes them
func (c *Client) receiveH264Track(track *webrtc.TrackRemote) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("❌ receiveH264Track panic: %v", r)
		}
		c.h264Receiving = false
	}()

	log.Printf("🎬 receiveH264Track STARTED (codec: %s, SSRC: %d, ClockRate: %d)",
		track.Codec().MimeType, track.SSRC(), track.Codec().ClockRate)

	// Create sample builder for H.264 depacketization
	// MaxLate is packet buffer size. Keyframes for screen content can easily exceed 50 RTP packets,
	// which would prevent SampleBuilder from ever outputting a complete access unit (freeze).
	sb := samplebuilder.New(2000, &codecs.H264Packet{}, track.Codec().ClockRate)

	// Start FFmpeg decoder if not already running
	if c.h264Decoder == nil {
		var err error
		frameCount := 0
		c.h264Decoder, err = NewH264Decoder(func(jpegData []byte) {
			frameCount++
			if frameCount%30 == 1 {
				log.Printf("🎬 H.264 decoded frame #%d (%d bytes)", frameCount, len(jpegData))
			}
			// Forward decoded JPEG frame to onFrame callback
			if c.onFrame != nil {
				c.onFrame(jpegData)
			}
		})
		if err != nil {
			log.Printf("❌ Failed to start H.264 decoder: %v", err)
			log.Println("⚠️ Falling back to JPEG datachannel mode")
			// Ask agent to switch back to tiles so the user isn't left with a frozen view.
			if switchErr := c.SetStreamingMode("tiles", 0); switchErr != nil {
				log.Printf("⚠️ Failed to request tiles fallback: %v", switchErr)
			}
			return
		}
	}

	// Read RTP packets and decode
	rtpCount := 0
	sampleCount := 0
	log.Println("🎬 Entering RTP read loop...")
	for {
		// Read RTP packet
		rtpPacket, _, err := track.ReadRTP()
		if err != nil {
			log.Printf("⚠️ RTP read error (after %d packets): %v", rtpCount, err)
			break
		}

		rtpCount++
		if rtpCount == 1 {
			log.Printf("🎬 FIRST RTP packet received! SeqNum: %d, Timestamp: %d, PayloadLen: %d",
				rtpPacket.SequenceNumber, rtpPacket.Timestamp, len(rtpPacket.Payload))
		}
		if rtpCount%100 == 0 {
			log.Printf("🎬 RTP packets received: %d", rtpCount)
		}

		// Push to sample builder
		sb.Push(rtpPacket)

		// Pop complete samples (access units)
		for {
			sample := sb.Pop()
			if sample == nil {
				break
			}

			sampleCount++
			if sampleCount%30 == 1 {
				log.Printf("🎬 H.264 samples: %d (data size: %d bytes)", sampleCount, len(sample.Data))
			}

			// Restart decoder if it was stopped (e.g. after switching back from tiles)
			if c.h264Decoder == nil {
				log.Println("🎬 H.264 decoder was stopped - restarting for new data...")
				var decErr error
				decFrameCount := 0
				c.h264Decoder, decErr = NewH264Decoder(func(jpegData []byte) {
					decFrameCount++
					if decFrameCount%30 == 1 {
						log.Printf("🎬 H.264 decoded frame #%d (%d bytes)", decFrameCount, len(jpegData))
					}
					if c.onFrame != nil {
						c.onFrame(jpegData)
					}
				})
				if decErr != nil {
					log.Printf("❌ Failed to restart H.264 decoder: %v", decErr)
					continue
				}
				log.Println("✅ H.264 decoder restarted successfully")
			}

			// Ensure Annex-B format and send to decoder
			annexB := EnsureAnnexB(sample.Data)
			if err := c.h264Decoder.DecodeAnnexB(annexB); err != nil {
				log.Printf("⚠️ Decode error: %v", err)
			}
		}
	}

	log.Println("🎬 H.264 receiver stopped")
}

// SetOnH264Frame sets the callback for decoded H.264 frames
func (c *Client) SetOnH264Frame(callback func([]byte)) {
	c.onH264Frame = callback
}

// EnableH264 enables or disables H.264 mode
func (c *Client) EnableH264(enable bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.useH264 = enable
	log.Printf("🎬 H.264 mode: %v", enable)
}

// IsH264Receiving returns whether H.264 video is being received
func (c *Client) IsH264Receiving() bool {
	return c.h264Receiving
}

// StopH264Decoder stops the H.264 decoder
func (c *Client) StopH264Decoder() {
	if c.h264Decoder != nil {
		c.h264Decoder.Stop()
		c.h264Decoder = nil
	}
}
