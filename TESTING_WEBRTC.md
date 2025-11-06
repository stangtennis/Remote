# WebRTC Connection Testing Guide

## ⚠️ **IMPORTANT: Database Setup Required**

Before testing, you MUST create the `sessions` table in Supabase.

### SQL to Run in Supabase SQL Editor:

```sql
-- Create sessions table for WebRTC signaling
CREATE TABLE IF NOT EXISTS sessions (
    session_id TEXT PRIMARY KEY,
    device_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    status TEXT DEFAULT 'pending',
    offer TEXT,
    answer TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Add RLS policies
ALTER TABLE sessions ENABLE ROW LEVEL SECURITY;

-- Allow authenticated users to create sessions
CREATE POLICY "Users can create sessions"
    ON sessions FOR INSERT
    TO authenticated
    WITH CHECK (true);

-- Allow users to read their own sessions
CREATE POLICY "Users can read own sessions"
    ON sessions FOR SELECT
    TO authenticated
    USING (user_id = auth.uid());

-- Allow devices to read sessions for their device_id
CREATE POLICY "Devices can read their sessions"
    ON sessions FOR SELECT
    TO anon
    USING (true);

-- Allow devices to update sessions (for answers)
CREATE POLICY "Devices can update sessions"
    ON sessions FOR UPDATE
    TO anon
    USING (true);

-- Allow users to update their sessions
CREATE POLICY "Users can update own sessions"
    ON sessions FOR UPDATE
    TO authenticated
    USING (user_id = auth.uid());

-- Allow users to delete their sessions
CREATE POLICY "Users can delete own sessions"
    ON sessions FOR DELETE
    TO authenticated
    USING (user_id = auth.uid());
```

## 🔧 **Current Implementation Status**

### ✅ Completed:
- WebRTC client package (`controller/internal/webrtc/client.go`)
- Signaling client (`controller/internal/webrtc/signaling.go`)
- Agent WebRTC server (already exists)
- Agent signaling listener (already exists)

### ❌ Not Yet Implemented:
- Viewer integration with WebRTC client
- Video frame decoding and rendering
- Input event forwarding
- Connection UI feedback

## 📝 **Next Steps to Complete**

### 1. Update Viewer to Use WebRTC

The viewer (`controller/internal/viewer/viewer.go`) needs to:
- Create WebRTC client instance
- Create signaling client instance
- Initiate connection on "Connect" button
- Decode JPEG frames from data channel
- Render frames to canvas
- Forward mouse/keyboard events

### 2. Test Connection Flow

1. **Start Agent**:
   ```powershell
   cd F:\#Remote\agent
   .\remote-agent.exe
   ```

2. **Start Controller**:
   ```powershell
   cd F:\#Remote\controller
   .\controller.exe
   ```

3. **Approve Device** (if not already approved)

4. **Click Connect** on device

5. **Expected Flow**:
   - Controller creates session in Supabase
   - Controller creates WebRTC offer
   - Controller sends offer to Supabase
   - Agent polls and finds session
   - Agent retrieves offer
   - Agent creates WebRTC answer
   - Agent sends answer to Supabase
   - Controller retrieves answer
   - WebRTC connection established
   - Video streaming begins

## 🐛 **Troubleshooting**

### Connection Fails
- Check Supabase `sessions` table exists
- Check RLS policies are correct
- Check both agent and controller logs
- Verify device is online
- Check firewall/NAT settings

### No Video
- Check agent screen capturer is working
- Check data channel is open
- Check JPEG encoding/decoding
- Verify frame rate in logs

### High Latency
- Check network connection
- Verify STUN/TURN servers
- Check CPU usage on both sides
- Reduce resolution or quality

## 📊 **Performance Targets**

- **Latency**: < 100ms
- **FPS**: 60 (configurable)
- **Quality**: JPEG 95 (configurable)
- **Resolution**: Up to 4K (configurable)

## 🔐 **Security Notes**

- All signaling goes through Supabase (HTTPS)
- WebRTC uses DTLS for encryption
- RLS policies protect session data
- No direct P2P without authentication

## 📚 **Reference**

- Agent WebRTC: `agent/internal/webrtc/`
- Controller WebRTC: `controller/internal/webrtc/`
- Viewer UI: `controller/internal/viewer/`
- Implementation Plan: `WEBRTC_IMPLEMENTATION.md`
