# 🎥 Controller v0.3.0 - WebRTC Viewer Design

**Goal:** Enable real-time remote screen viewing through WebRTC peer-to-peer connection.

---

## 📋 Overview

### What We're Building

A **viewer window** that opens when clicking "Connect" on a device, establishing a WebRTC connection and displaying the remote screen in real-time.

### Key Features
- ✅ Viewer window with video canvas
- ✅ WebRTC peer connection
- ✅ Session management via Supabase
- ✅ Real-time video streaming
- ✅ Connection status indicators
- ✅ Clean disconnect handling

---

## 🏗️ Architecture

### High-Level Flow

```
┌─────────────────────────────────────────────────────┐
│  CONTROLLER (Admin)                                 │
│  1. Click "Connect" on device                       │
│  2. Create session in database                      │
│  3. Send WebRTC offer via Realtime                  │
│  4. Wait for agent response                         │
│  5. Establish WebRTC connection                     │
│  6. Receive video stream                            │
│  7. Display in viewer window                        │
└────────────────┬────────────────────────────────────┘
                 │
                 │ Supabase Realtime (Signaling)
                 │ WebRTC P2P (Video Stream)
                 │
                 ↓
┌─────────────────────────────────────────────────────┐
│  AGENT (Remote Device)                              │
│  1. Listen for session requests                     │
│  2. Show PIN prompt                                 │
│  3. User enters PIN                                 │
│  4. Send WebRTC answer                              │
│  5. Establish connection                            │
│  6. Stream screen via WebRTC                        │
└─────────────────────────────────────────────────────┘
```

### Component Structure

```
controller/
├── internal/
│   ├── viewer/              # 🆕 New package
│   │   ├── window.go        # Viewer window UI
│   │   ├── webrtc.go        # WebRTC connection
│   │   ├── session.go       # Session management
│   │   ├── renderer.go      # Video rendering
│   │   └── controls.go      # UI controls
│   ├── supabase/            # Existing
│   └── config/              # Existing
└── main.go                  # Update to use viewer
```

---

## 📦 Package Design

### 1. Viewer Window (`window.go`)

**Purpose:** Manage the viewer window UI and lifecycle.

```go
package viewer

import (
    "fyne.io/fyne/v2"
    "fyne.io/fyne/v2/canvas"
    "fyne.io/fyne/v2/container"
    "fyne.io/fyne/v2/widget"
    "github.com/stangtennis/Remote/controller/internal/supabase"
)

type ViewerWindow struct {
    app        fyne.App
    window     fyne.Window
    device     supabase.Device
    
    // UI components
    videoCanvas *canvas.Raster
    statusLabel *widget.Label
    statsLabel  *widget.Label
    
    // Connection
    connection  *WebRTCConnection
    session     *Session
    
    // State
    isConnected bool
    frameCount  int
}

// NewViewerWindow creates a new viewer window for a device
func NewViewerWindow(app fyne.App, device supabase.Device) *ViewerWindow {
    vw := &ViewerWindow{
        app:    app,
        device: device,
    }
    
    vw.createWindow()
    return vw
}

// createWindow builds the UI
func (vw *ViewerWindow) createWindow() {
    vw.window = vw.app.NewWindow(vw.device.DeviceName + " - Remote Desktop")
    vw.window.Resize(fyne.NewSize(1280, 720))
    
    // Video canvas
    vw.videoCanvas = canvas.NewRaster(vw.drawFrame)
    
    // Controls
    controls := vw.createControls()
    
    // Layout
    content := container.NewBorder(
        controls,      // Top
        vw.createStatusBar(), // Bottom
        nil, nil,      // Left, Right
        vw.videoCanvas, // Center
    )
    
    vw.window.SetContent(content)
    vw.window.SetOnClosed(vw.handleClose)
}

// Connect initiates the connection to the remote device
func (vw *ViewerWindow) Connect() error {
    vw.statusLabel.SetText("🔄 Connecting...")
    
    // 1. Create session
    session, err := CreateSession(vw.device)
    if err != nil {
        return err
    }
    vw.session = session
    
    // 2. Create WebRTC connection
    conn, err := NewWebRTCConnection(vw.device, vw.session)
    if err != nil {
        return err
    }
    vw.connection = conn
    
    // 3. Set up callbacks
    conn.OnConnected = vw.handleConnected
    conn.OnDisconnected = vw.handleDisconnected
    conn.OnFrame = vw.handleFrame
    
    // 4. Start connection
    return conn.Connect()
}

// Show displays the viewer window
func (vw *ViewerWindow) Show() {
    vw.window.Show()
}
```

---

### 2. WebRTC Connection (`webrtc.go`)

**Purpose:** Handle WebRTC peer connection, ICE, and media streams.

```go
package viewer

import (
    "github.com/pion/webrtc/v3"
    "log"
)

type WebRTCConnection struct {
    device supabase.Device
    session *Session
    
    // WebRTC
    pc          *webrtc.PeerConnection
    dataChannel *webrtc.DataChannel
    videoTrack  *webrtc.TrackRemote
    
    // Callbacks
    OnConnected    func()
    OnDisconnected func()
    OnFrame        func(frame []byte)
    
    // State
    connected bool
}

// NewWebRTCConnection creates a new WebRTC connection
func NewWebRTCConnection(device supabase.Device, session *Session) (*WebRTCConnection, error) {
    conn := &WebRTCConnection{
        device:  device,
        session: session,
    }
    
    return conn, nil
}

// Connect establishes the WebRTC connection
func (c *WebRTCConnection) Connect() error {
    // 1. Create peer connection
    config := webrtc.Configuration{
        ICEServers: []webrtc.ICEServer{
            {
                URLs: []string{"stun:stun.l.google.com:19302"},
            },
            {
                URLs:       []string{"turn:global.relay.metered.ca:80"},
                Username:   "your-username",
                Credential: "your-credential",
            },
        },
    }
    
    pc, err := webrtc.NewPeerConnection(config)
    if err != nil {
        return err
    }
    c.pc = pc
    
    // 2. Set up handlers
    c.setupHandlers()
    
    // 3. Create data channel for control (future use)
    dc, err := pc.CreateDataChannel("control", nil)
    if err != nil {
        return err
    }
    c.dataChannel = dc
    
    // 4. Create offer
    offer, err := pc.CreateOffer(nil)
    if err != nil {
        return err
    }
    
    // 5. Set local description
    if err := pc.SetLocalDescription(offer); err != nil {
        return err
    }
    
    // 6. Send offer to agent via Supabase
    return c.session.SendOffer(offer)
}

// setupHandlers configures WebRTC event handlers
func (c *WebRTCConnection) setupHandlers() {
    // ICE candidate handler
    c.pc.OnICECandidate(func(candidate *webrtc.ICECandidate) {
        if candidate != nil {
            c.session.SendICECandidate(candidate)
        }
    })
    
    // Connection state handler
    c.pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
        log.Printf("Connection state: %s", state.String())
        
        switch state {
        case webrtc.PeerConnectionStateConnected:
            c.connected = true
            if c.OnConnected != nil {
                c.OnConnected()
            }
        case webrtc.PeerConnectionStateDisconnected,
             webrtc.PeerConnectionStateFailed,
             webrtc.PeerConnectionStateClosed:
            c.connected = false
            if c.OnDisconnected != nil {
                c.OnDisconnected()
            }
        }
    })
    
    // Track handler (receive video)
    c.pc.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
        log.Printf("Received track: %s", track.Kind().String())
        
        if track.Kind() == webrtc.RTPCodecTypeVideo {
            c.videoTrack = track
            go c.handleVideoTrack(track)
        }
    })
}

// handleVideoTrack processes incoming video frames
func (c *WebRTCConnection) handleVideoTrack(track *webrtc.TrackRemote) {
    for {
        // Read RTP packet
        packet, _, err := track.ReadRTP()
        if err != nil {
            log.Printf("Error reading RTP: %v", err)
            return
        }
        
        // Extract frame data
        // Note: This is simplified - actual implementation needs codec handling
        if c.OnFrame != nil {
            c.OnFrame(packet.Payload)
        }
    }
}

// HandleAnswer processes the WebRTC answer from the agent
func (c *WebRTCConnection) HandleAnswer(answer webrtc.SessionDescription) error {
    return c.pc.SetRemoteDescription(answer)
}

// HandleICECandidate adds an ICE candidate from the agent
func (c *WebRTCConnection) HandleICECandidate(candidate webrtc.ICECandidateInit) error {
    return c.pc.AddICECandidate(candidate)
}

// Disconnect closes the WebRTC connection
func (c *WebRTCConnection) Disconnect() error {
    if c.pc != nil {
        return c.pc.Close()
    }
    return nil
}
```

---

### 3. Session Management (`session.go`)

**Purpose:** Manage session lifecycle and Supabase Realtime signaling.

```go
package viewer

import (
    "encoding/json"
    "fmt"
    "github.com/pion/webrtc/v3"
    "github.com/stangtennis/Remote/controller/internal/supabase"
    "time"
)

type Session struct {
    ID          string
    DeviceID    string
    ControllerID string
    Status      string
    CreatedAt   time.Time
    
    // Realtime channel
    channel     interface{} // Supabase Realtime channel
    
    // Callbacks
    OnAnswer    func(answer webrtc.SessionDescription)
    OnICE       func(candidate webrtc.ICECandidateInit)
}

// CreateSession creates a new session in the database
func CreateSession(device supabase.Device) (*Session, error) {
    // Generate session ID
    sessionID := generateSessionID()
    
    // Insert into remote_sessions table
    query := `
        INSERT INTO remote_sessions (
            session_id, device_id, controller_id, status, created_at
        ) VALUES ($1, $2, $3, 'pending', NOW())
        RETURNING session_id, created_at
    `
    
    // Execute query (using supabase client)
    // ... implementation
    
    session := &Session{
        ID:       sessionID,
        DeviceID: device.DeviceID,
        Status:   "pending",
    }
    
    // Set up Realtime listener
    session.setupRealtime()
    
    return session, nil
}

// setupRealtime configures Supabase Realtime for signaling
func (s *Session) setupRealtime() error {
    // Subscribe to session updates
    // Listen for:
    // - Agent acceptance
    // - WebRTC answer
    // - ICE candidates
    
    // Pseudo-code (actual implementation depends on Supabase Go client)
    /*
    channel := supabaseClient.Realtime.Channel("sessions:" + s.ID)
    
    channel.On("UPDATE", func(payload) {
        // Handle session status change
        if payload.Status == "accepted" {
            // Agent accepted
        }
    })
    
    channel.On("webrtc_answer", func(payload) {
        // Parse answer
        var answer webrtc.SessionDescription
        json.Unmarshal(payload.Answer, &answer)
        
        if s.OnAnswer != nil {
            s.OnAnswer(answer)
        }
    })
    
    channel.On("ice_candidate", func(payload) {
        // Parse ICE candidate
        var candidate webrtc.ICECandidateInit
        json.Unmarshal(payload.Candidate, &candidate)
        
        if s.OnICE != nil {
            s.OnICE(candidate)
        }
    })
    
    channel.Subscribe()
    */
    
    return nil
}

// SendOffer sends the WebRTC offer to the agent
func (s *Session) SendOffer(offer webrtc.SessionDescription) error {
    // Serialize offer
    offerJSON, err := json.Marshal(offer)
    if err != nil {
        return err
    }
    
    // Update session with offer
    query := `
        UPDATE remote_sessions
        SET webrtc_offer = $1, status = 'offer_sent'
        WHERE session_id = $2
    `
    
    // Execute query
    // ... implementation
    
    return nil
}

// SendICECandidate sends an ICE candidate to the agent
func (s *Session) SendICECandidate(candidate *webrtc.ICECandidate) error {
    if candidate == nil {
        return nil
    }
    
    candidateJSON, err := json.Marshal(candidate.ToJSON())
    if err != nil {
        return err
    }
    
    // Send via Realtime
    // ... implementation
    
    return nil
}

// Close ends the session
func (s *Session) Close() error {
    // Update session status
    query := `
        UPDATE remote_sessions
        SET status = 'closed', ended_at = NOW()
        WHERE session_id = $1
    `
    
    // Execute query
    // ... implementation
    
    // Unsubscribe from Realtime
    // ... implementation
    
    return nil
}

func generateSessionID() string {
    // Generate unique session ID
    return fmt.Sprintf("sess_%d", time.Now().UnixNano())
}
```

---

### 4. Video Renderer (`renderer.go`)

**Purpose:** Decode and render video frames to the canvas.

```go
package viewer

import (
    "image"
    "image/jpeg"
    "bytes"
    "log"
)

type Renderer struct {
    currentFrame image.Image
    frameCount   int
}

// NewRenderer creates a new video renderer
func NewRenderer() *Renderer {
    return &Renderer{}
}

// DecodeFrame decodes a JPEG frame
func (r *Renderer) DecodeFrame(data []byte) (image.Image, error) {
    // Decode JPEG
    img, err := jpeg.Decode(bytes.NewReader(data))
    if err != nil {
        return nil, err
    }
    
    r.currentFrame = img
    r.frameCount++
    
    return img, nil
}

// GetCurrentFrame returns the latest frame
func (r *Renderer) GetCurrentFrame() image.Image {
    return r.currentFrame
}

// GetFrameCount returns the total frames received
func (r *Renderer) GetFrameCount() int {
    return r.frameCount
}

// DrawFrame is called by Fyne to render the frame
func (r *Renderer) DrawFrame(w, h int) image.Image {
    if r.currentFrame == nil {
        // Return blank image
        return image.NewRGBA(image.Rect(0, 0, w, h))
    }
    
    // TODO: Scale frame to fit canvas if needed
    return r.currentFrame
}
```

---

### 5. UI Controls (`controls.go`)

**Purpose:** Create UI controls for the viewer window.

```go
package viewer

import (
    "fyne.io/fyne/v2"
    "fyne.io/fyne/v2/container"
    "fyne.io/fyne/v2/widget"
)

// createControls creates the top control bar
func (vw *ViewerWindow) createControls() *fyne.Container {
    // Disconnect button
    disconnectBtn := widget.NewButton("Disconnect", func() {
        vw.Disconnect()
    })
    
    // Stats button
    statsBtn := widget.NewButton("Stats", func() {
        vw.showStats()
    })
    
    // Fullscreen button
    fullscreenBtn := widget.NewButton("Fullscreen", func() {
        vw.toggleFullscreen()
    })
    
    return container.NewHBox(
        disconnectBtn,
        statsBtn,
        fullscreenBtn,
    )
}

// createStatusBar creates the bottom status bar
func (vw *ViewerWindow) createStatusBar() *fyne.Container {
    vw.statusLabel = widget.NewLabel("Not connected")
    vw.statsLabel = widget.NewLabel("0 FPS | 0 Kbps")
    
    return container.NewHBox(
        vw.statusLabel,
        widget.NewLabel(" | "),
        vw.statsLabel,
    )
}

// Disconnect closes the connection
func (vw *ViewerWindow) Disconnect() {
    if vw.connection != nil {
        vw.connection.Disconnect()
    }
    if vw.session != nil {
        vw.session.Close()
    }
    vw.window.Close()
}

// showStats displays connection statistics
func (vw *ViewerWindow) showStats() {
    // TODO: Show detailed stats dialog
}

// toggleFullscreen toggles fullscreen mode
func (vw *ViewerWindow) toggleFullscreen() {
    vw.window.SetFullScreen(!vw.window.FullScreen())
}
```

---

## 🔄 Connection Flow

### Detailed Sequence

```
1. User clicks "Connect" on device
   ↓
2. Controller: Create ViewerWindow
   ↓
3. Controller: Call ViewerWindow.Connect()
   ↓
4. Controller: CreateSession() → Insert into remote_sessions
   ↓
5. Controller: NewWebRTCConnection()
   ↓
6. Controller: Create peer connection
   ↓
7. Controller: Create offer
   ↓
8. Controller: SendOffer() → Update session with offer
   ↓
9. Agent: Receive session notification (Realtime)
   ↓
10. Agent: Show PIN prompt
   ↓
11. User: Enter PIN on agent
   ↓
12. Agent: Validate PIN
   ↓
13. Agent: Create peer connection
   ↓
14. Agent: Set remote description (offer)
   ↓
15. Agent: Create answer
   ↓
16. Agent: Send answer via Realtime
   ↓
17. Controller: Receive answer
   ↓
18. Controller: Set remote description (answer)
   ↓
19. Both: Exchange ICE candidates
   ↓
20. Both: ICE connection established
   ↓
21. Agent: Start screen capture
   ↓
22. Agent: Send video track
   ↓
23. Controller: Receive video track
   ↓
24. Controller: Decode and display frames
   ↓
25. ✅ Connected and streaming!
```

---

## 📊 Database Schema

### Session Table (Already Exists)

```sql
-- remote_sessions table
CREATE TABLE remote_sessions (
    session_id TEXT PRIMARY KEY,
    device_id TEXT REFERENCES remote_devices(device_id),
    controller_id TEXT REFERENCES auth.users(id),
    status TEXT, -- 'pending', 'offer_sent', 'accepted', 'connected', 'closed'
    webrtc_offer JSONB,
    webrtc_answer JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    ended_at TIMESTAMPTZ
);
```

### Realtime Events

```javascript
// Controller sends
{
    type: 'webrtc_offer',
    session_id: 'sess_123',
    offer: { type: 'offer', sdp: '...' }
}

{
    type: 'ice_candidate',
    session_id: 'sess_123',
    candidate: { candidate: '...', sdpMid: '...', sdpMLineIndex: 0 }
}

// Agent sends
{
    type: 'webrtc_answer',
    session_id: 'sess_123',
    answer: { type: 'answer', sdp: '...' }
}

{
    type: 'ice_candidate',
    session_id: 'sess_123',
    candidate: { candidate: '...', sdpMid: '...', sdpMLineIndex: 0 }
}
```

---

## 🧪 Testing Strategy

### Unit Tests

```go
// viewer/window_test.go
func TestNewViewerWindow(t *testing.T) {
    // Test window creation
}

// viewer/webrtc_test.go
func TestWebRTCConnection(t *testing.T) {
    // Test connection setup
}

// viewer/session_test.go
func TestCreateSession(t *testing.T) {
    // Test session creation
}
```

### Integration Tests

1. **Test with Windows Agent**
   - Connect controller to Windows agent
   - Verify video streaming
   - Test disconnect

2. **Test with Web Agent**
   - Connect controller to web agent
   - Verify compatibility
   - Test reconnection

3. **Test Multiple Sessions**
   - Open multiple viewer windows
   - Verify independent connections
   - Test resource usage

---

## 🎯 Success Criteria

### Must Have (v0.3.0)
- ✅ Viewer window opens on "Connect"
- ✅ WebRTC connection established
- ✅ Video stream displays
- ✅ Can disconnect cleanly
- ✅ Connection status shown

### Nice to Have (v0.3.1)
- 🎁 Connection quality indicator
- 🎁 FPS counter
- 🎁 Bandwidth stats
- 🎁 Reconnection on failure
- 🎁 Fullscreen mode

### Future (v0.4.0+)
- 🚀 Mouse control
- 🚀 Keyboard control
- 🚀 File transfer
- 🚀 Multi-monitor support

---

## 📝 Implementation Checklist

### Phase 1: Foundation (Day 1)
- [ ] Create `internal/viewer/` package
- [ ] Implement `window.go` basic structure
- [ ] Create viewer window UI
- [ ] Add video canvas placeholder
- [ ] Test window opens/closes

### Phase 2: WebRTC (Day 2)
- [ ] Implement `webrtc.go`
- [ ] Port peer connection code from agent
- [ ] Set up ICE configuration
- [ ] Test offer creation
- [ ] Test ICE candidate exchange

### Phase 3: Session (Day 3)
- [ ] Implement `session.go`
- [ ] Create session in database
- [ ] Set up Realtime listeners
- [ ] Send/receive offers/answers
- [ ] Test signaling flow

### Phase 4: Rendering (Day 4)
- [ ] Implement `renderer.go`
- [ ] Decode JPEG frames
- [ ] Draw to canvas
- [ ] Test frame rate
- [ ] Optimize performance

### Phase 5: Integration (Day 5)
- [ ] Connect all components
- [ ] Test end-to-end flow
- [ ] Fix bugs
- [ ] Add error handling
- [ ] Update documentation

---

## 🚀 Getting Started

### Step 1: Create Package Structure
```bash
cd controller
mkdir -p internal/viewer
touch internal/viewer/window.go
touch internal/viewer/webrtc.go
touch internal/viewer/session.go
touch internal/viewer/renderer.go
touch internal/viewer/controls.go
```

### Step 2: Install Dependencies
```bash
go get github.com/pion/webrtc/v3
```

### Step 3: Start with Window
Begin implementing `window.go` with basic UI.

### Step 4: Port WebRTC Code
Copy and adapt from `agent/internal/webrtc/`.

### Step 5: Test Incrementally
Test each component as you build it.

---

## 📚 Resources

### Code References
- **Agent WebRTC:** `agent/internal/webrtc/`
- **Dashboard WebRTC:** `docs/js/webrtc.js`
- **Supabase Client:** `controller/internal/supabase/client.go`

### Documentation
- **Pion WebRTC:** https://github.com/pion/webrtc
- **Fyne Canvas:** https://developer.fyne.io/canvas/
- **Supabase Realtime:** https://supabase.com/docs/guides/realtime

---

## ✅ Summary

This design provides:
- ✅ Clear package structure
- ✅ Detailed component design
- ✅ Connection flow diagram
- ✅ Implementation checklist
- ✅ Testing strategy
- ✅ Success criteria

**Ready to start building!** 🚀

**Next:** Begin with Phase 1 - Create the viewer package structure and basic window UI.
