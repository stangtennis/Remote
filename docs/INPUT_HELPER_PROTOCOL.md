# Web Agent Input Helper Protocol

Browser-only remote input via WebRTC + local helper process.

## Architecture

```
┌─────────────────┐     WebRTC      ┌─────────────────┐
│  Dashboard      │◄───────────────►│  Web Agent      │
│  (viewer)       │   video stream  │  (agent.html)   │
└─────────────────┘                 └────────┬────────┘
                                             │
                                    WebSocket│localhost:9877
                                             │
                                    ┌────────▼────────┐
                                    │  Input Helper   │
                                    │  (Go process)   │
                                    │                 │
                                    │  - Mouse inject │
                                    │  - Key inject   │
                                    │  - Clipboard    │
                                    └─────────────────┘
```

## Message Protocol (JSON over WebSocket)

### Authentication (first message)
```json
{
  "type": "auth",
  "token": "session-token-from-supabase",
  "device_id": "web-abc123",
  "session_id": "uuid"
}
```

### Mouse Move (relative, pointer lock)
```json
{
  "type": "mouse_move",
  "dx": 10,
  "dy": -5,
  "seq": 1234,
  "ts": 1702300000000
}
```

### Mouse Move (absolute, normalized 0-1)
```json
{
  "type": "mouse_abs",
  "x": 0.5,
  "y": 0.3,
  "seq": 1235,
  "ts": 1702300000001
}
```

### Mouse Button
```json
{
  "type": "mouse_button",
  "button": "left",
  "down": true,
  "seq": 1236,
  "ts": 1702300000002
}
```
- `button`: "left", "right", "middle"

### Mouse Wheel
```json
{
  "type": "wheel",
  "dx": 0,
  "dy": -120,
  "seq": 1237,
  "ts": 1702300000003
}
```

### Key Event
```json
{
  "type": "key",
  "code": "KeyA",
  "key": "a",
  "down": true,
  "ctrl": false,
  "shift": false,
  "alt": false,
  "seq": 1238,
  "ts": 1702300000004
}
```

### Clipboard
```json
{
  "type": "clipboard",
  "direction": "to_system",
  "content": "text to paste",
  "seq": 1239,
  "ts": 1702300000005
}
```
- `direction`: "to_system" (paste) or "from_system" (copy request)

### Control
```json
{
  "type": "control",
  "action": "pause",
  "scope": "all"
}
```
- `action`: "pause", "resume", "stop"
- `scope`: "mouse", "keyboard", "clipboard", "all"

### Response from Helper
```json
{
  "type": "ack",
  "seq": 1234,
  "ok": true,
  "error": null
}
```

```json
{
  "type": "clipboard_content",
  "content": "copied text",
  "seq": 1240
}
```

```json
{
  "type": "status",
  "connected": true,
  "scope": {
    "mouse": true,
    "keyboard": false,
    "clipboard": false
  }
}
```

## Security

1. **Token validation**: Helper validates session token against Supabase
2. **Device binding**: Token tied to specific device_id
3. **Scope control**: Default mouse-only, explicit toggle for keyboard/clipboard
4. **Rate limiting**: Max 1000 events/sec, coalesce mouse moves
5. **Inactivity timeout**: Auto-pause after 30s no input
6. **Emergency stop**: Ctrl+Shift+Escape in browser pauses all

## Flow Control

1. **Sequence numbers**: Monotonic, detect gaps/reorder
2. **Backpressure**: If WS buffer > 100 events, coalesce mouse moves
3. **Stale detection**: Drop mouse_move older than 100ms
4. **State preservation**: Never drop button/key up/down events

## Browser Capture

1. **Pointer lock**: Request on click, relative mouse while locked
2. **Fallback**: Absolute coords when unlocked
3. **Keyboard**: keydown/keyup with code, suppress repeat
4. **Blur handling**: Send "all keys up" on blur/visibility change
5. **Clipboard**: Require user gesture (Ctrl+V to paste)

## Helper Endpoints

- `ws://127.0.0.1:9877/input` - Main input WebSocket
- `GET http://127.0.0.1:9877/status` - Health check
- `POST http://127.0.0.1:9877/stop` - Emergency stop

## Build & Run

```bash
cd input-helper
go build -o input-helper.exe .
./input-helper.exe
```

Or development:
```bash
go run .
```
