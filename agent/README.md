# Remote Desktop Agent

Windows agent application for remote desktop access. **Current version: v2.64.0**

## 🚀 Quick Start

### Download
Get the latest release from [GitHub Releases](https://github.com/stangtennis/Remote/releases)

### Install
1. Download `RemoteDesktopAgent-Setup-v2.64.0.exe`
2. Run the installer
3. Agent starts automatically and appears in system tray

### Manual Run
```bash
# Double-click or run:
.\remote-agent.exe
```

---

## 📋 Features (v2.64.0)

### Streaming
- ✅ **Adaptive FPS** - 2-30 FPS based on activity
- ✅ **Bandwidth Optimization** - Frame skipping on static desktop (50-80% savings)
- ✅ **DXGI Capture** - Works over RDP sessions
- ✅ **Idle Mode** - 2 FPS + high quality when static

### Remote Control
- ✅ **Full Input** - Mouse, keyboard, scroll
- ✅ **Accurate Mapping** - Correct coordinate translation

### File & Clipboard
- ✅ **File Browser** - Browse drives and folders
- ✅ **File Transfer** - Send files to controller
- ✅ **Clipboard Sync** - Text and images

### System
- ✅ **System Tray** - Minimize to tray
- ✅ **Auto-Registration** - No login required
- ✅ **Windows Service** - Run at login screen
- ✅ **Auto-Elevation** - UAC prompt if needed

---

## 🛠️ Development

### Prerequisites
- Go 1.21+
- MinGW (for CGO)

### Build
```bash
cd agent
go build -ldflags="-s -w -H windowsgui" -o remote-agent.exe ./cmd/remote-agent
```

### Windows Service
```bash
# Install as service (requires Admin)
install-service.bat

# Uninstall service
uninstall-service.bat

# See SERVICE_GUIDE.md for details
```

**Option 2: Startup Task (Run on user login)**
```bash
# Setup auto-start on login (requires Admin)
setup-startup.bat

# Remove auto-start
schtasks /delete /tn "RemoteDesktopAgent" /f
```

### Development Mode
```bash
# Run from source
go run ./cmd/remote-agent
```

## Features

- ✅ Device registration with Supabase
- ✅ Screen capture (JPEG over data channel, 30 FPS, 1920px)
- ✅ WebRTC P2P connection with TURN fallback
- ✅ Mouse and keyboard input
- ✅ Mouse wheel/scroll support
- ✅ Realtime presence tracking
- ✅ Auto-reconnection
- ✅ Windows Service support
- ✅ Lock screen capture (Windows login screen access)
- ✅ Desktop switching detection
- 🔄 Video track (coming in Phase 4)
- 🔄 File transfer (coming in Phase 5)

## Architecture

```
agent/
├── cmd/
│   └── remote-agent/
│       └── main.go           # Entry point (service-aware)
├── internal/
│   ├── device/
│   │   ├── device.go         # Device info & registration
│   │   └── presence.go       # Heartbeat & online status
│   ├── screen/
│   │   └── capture.go        # Screen capture (multi-desktop)
│   ├── input/
│   │   ├── mouse.go          # Mouse input simulation
│   │   └── keyboard.go       # Keyboard input simulation
│   ├── desktop/
│   │   └── desktop_windows.go # Desktop switching & detection
│   ├── service/
│   │   └── service_windows.go # Windows Service wrapper
│   ├── webrtc/
│   │   ├── peer.go           # WebRTC peer connection
│   │   ├── datachannel.go    # Data channel for frames & input
│   │   └── signaling.go      # Signaling via Supabase
│   └── config/
│       └── config.go         # Configuration
├── run-agent-once.bat        # Manual one-time start
├── setup-startup.bat         # Auto-start on login
├── install-service.bat       # Install as Windows Service
├── uninstall-service.bat     # Remove Windows Service
├── SERVICE_GUIDE.md          # Service installation guide
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
