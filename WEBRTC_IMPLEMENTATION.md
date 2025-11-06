# WebRTC Implementation Plan

## Current Status
- ✅ Agent has complete WebRTC server (peer.go, signaling.go)
- ✅ Agent listens for sessions via Supabase Realtime
- ✅ Agent streams screen at 60 FPS, JPEG 95, 4K
- ✅ Agent handles mouse/keyboard input
- ❌ Controller has viewer UI but no WebRTC client
- ❌ Controller cannot initiate connections

## Architecture

```
Controller (Viewer)          Supabase          Agent (Server)
      |                         |                    |
      |-- Create Session ------>|                    |
      |                         |<--- Poll Session --|
      |                         |                    |
      |-- Send Offer ---------->|                    |
      |                         |<--- Get Offer -----|
      |                         |                    |
      |<--- Send Answer --------|                    |
      |                         |-- Get Answer ----->|
      |                         |                    |
      |<========= WebRTC P2P Connection ==========>|
      |                         |                    |
      |<------- Video Stream (60 FPS) -------------|
      |-------- Mouse/Keyboard Input ------------->|
```

## Implementation Steps

### 1. Controller WebRTC Client Package
**File**: `controller/internal/webrtc/client.go`

**Features**:
- Create peer connection
- Handle ICE candidates
- Create SDP offer
- Handle SDP answer
- Receive video track
- Send input events via data channel

### 2. Signaling via Supabase
**File**: `controller/internal/webrtc/signaling.go`

**Features**:
- Create session in `sessions` table
- Send offer to Supabase
- Poll for answer from agent
- Exchange ICE candidates

### 3. Video Rendering
**File**: `controller/internal/viewer/video.go`

**Features**:
- Decode JPEG frames from data channel
- Render to Fyne canvas
- Handle resolution changes
- Display FPS counter

### 4. Input Forwarding
**File**: `controller/internal/viewer/input.go` (already exists)

**Features**:
- Capture mouse events
- Capture keyboard events
- Send via WebRTC data channel
- Normalize coordinates

## Database Schema Required

### sessions table
```sql
CREATE TABLE sessions (
    session_id TEXT PRIMARY KEY,
    device_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    status TEXT DEFAULT 'pending',
    offer JSONB,
    answer JSONB,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);
```

### ice_candidates table (optional, for trickle ICE)
```sql
CREATE TABLE ice_candidates (
    id SERIAL PRIMARY KEY,
    session_id TEXT NOT NULL,
    candidate JSONB NOT NULL,
    from_device BOOLEAN DEFAULT false,
    created_at TIMESTAMP DEFAULT NOW()
);
```

## Testing Plan

1. **Unit Tests**
   - Test peer connection creation
   - Test offer/answer exchange
   - Test data channel messages

2. **Integration Tests**
   - Start agent
   - Start controller
   - Initiate connection
   - Verify video stream
   - Test mouse/keyboard input

3. **Performance Tests**
   - Measure latency
   - Verify 60 FPS
   - Check bandwidth usage
   - Test 4K streaming

## Next Steps

1. Create controller WebRTC client package
2. Implement signaling in controller
3. Connect viewer to WebRTC stream
4. Test end-to-end connection
5. Add error handling and reconnection logic
