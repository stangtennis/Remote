# Remote Desktop Agent

Windows agent application for remote desktop access.

## Prerequisites

**Install Go:**
1. Download: https://go.dev/dl/
2. Install Go 1.21 or higher
3. Verify: `go version`

## Build

```bash
cd f:\#Remote\agent

# Initialize module
go mod init github.com/stangtennis/remote-agent

# Download dependencies
go mod tidy

# Build
go build -o remote-agent.exe ./cmd/remote-agent

# Build for production (smaller, optimized)
go build -ldflags="-s -w" -o remote-agent.exe ./cmd/remote-agent
```

## Run

```bash
# Development mode
go run ./cmd/remote-agent

# Or run the built executable
.\remote-agent.exe
```

## Features

- ✅ Device registration with Supabase
- ✅ Screen capture (JPEG over data channel)
- ✅ WebRTC P2P connection
- ✅ Mouse and keyboard input
- ✅ Realtime presence tracking
- ✅ Auto-reconnection
- 🔄 Video track (coming in Fase 4)
- 🔄 File transfer (coming in Fase 5)

## Architecture

```
agent/
├── cmd/
│   └── remote-agent/
│       └── main.go           # Entry point
├── internal/
│   ├── device/
│   │   ├── device.go         # Device info & registration
│   │   └── presence.go       # Heartbeat & online status
│   ├── screen/
│   │   ├── capture.go        # Screen capture
│   │   └── encoder.go        # JPEG encoding
│   ├── input/
│   │   ├── mouse.go          # Mouse input simulation
│   │   └── keyboard.go       # Keyboard input simulation
│   ├── webrtc/
│   │   ├── peer.go           # WebRTC peer connection
│   │   ├── datachannel.go    # Data channel for frames & input
│   │   └── signaling.go      # Signaling via Supabase
│   └── config/
│       └── config.go         # Configuration
├── go.mod
├── go.sum
└── README.md
```

## Configuration

Create `.env` file or set environment variables:

```env
SUPABASE_URL=https://mnqtdugcvfyenjuqruol.supabase.co
SUPABASE_ANON_KEY=your-anon-key
DEVICE_NAME=My PC
```

## Dependencies

- **Pion WebRTC** - WebRTC implementation
- **kbinani/screenshot** - Screen capture
- **robotgo** - Mouse/keyboard simulation
- **supabase-go** - Supabase client

## Testing

```bash
# Run agent
go run ./cmd/remote-agent

# Should see:
# - Device registered
# - Waiting for connection
# - Go to dashboard and click "Connect"
```

## Troubleshooting

### "go: command not found"
- Install Go from https://go.dev/dl/
- Add to PATH

### "cannot find package"
- Run `go mod tidy`
- Check internet connection

### Screen capture fails
- Requires Windows desktop session
- Won't work in RDP without GPU

### WebRTC connection fails
- Check firewall
- Verify Supabase Edge Functions are deployed
- Check TURN credentials

## Next Steps

1. Install Go
2. Run `go mod init github.com/stangtennis/remote-agent`
3. Create the source files
4. Run `go mod tidy` to download dependencies
5. Test with `go run ./cmd/remote-agent`
