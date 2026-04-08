# Remote Desktop

A **professional remote desktop solution** built with **Supabase**, **WebRTC**, and **Go** — like TeamViewer, but self-hosted and open-source.

**Current version: v2.99.57** | [Download](https://github.com/stangtennis/Remote/releases/latest) | [Dashboard](https://dashboard.hawkeye123.dk)

## Features

### Streaming & Performance
- **libjpeg-turbo SIMD encoding** — 2-6x faster JPEG via hardware SIMD (SSE2/AVX2/NEON)
- **DXGI screen capture** — GPU-accelerated, works over RDP
- **GDI fallback** — Session 0 / login screen capture
- **macOS Quartz capture** — CoreGraphics with hardware-scaled resize
- **OpenH264 video** — H.264 encoding via Cisco OpenH264
- **Adaptive streaming** — auto-adjusts FPS/quality/scale based on CPU, RTT, loss
- **Dirty region detection** — tile-based motion detection, 50-80% bandwidth savings on static desktop
- **Double-buffer frame comparison** — zero-allocation motion detection
- **Zero-copy capture (macOS)** — `unsafe.Slice` eliminates intermediate buffer copies
- **BGRA direct encode (Windows)** — skips pixel format conversion entirely

### Remote Control
- **Full mouse & keyboard** — click, drag, scroll, modifiers, unicode
- **UIPI bypass** — controls admin windows and Winlogon desktop via SYSTEM token
- **Session 0 support** — pre-login, post-login, lock screen (Win+L) — all verified
- **macOS input** — CGEvent-based mouse & keyboard with `kCGSessionEventTap`
- **Clipboard sync** — copy/paste text and images between machines
- **File transfer** — browse remote drives, upload/download files
- **Multi-monitor** — switch between displays, per-monitor streaming

### Platforms

| Platform | Agent | Controller |
|----------|-------|------------|
| **Windows** | Native EXE + Windows Service + NSIS installer | Native EXE + NSIS installer |
| **macOS** | Universal binary (Intel + Apple Silicon) | Universal binary |
| **Web** | Browser-based screen sharing | Dashboard (self-hosted) |

### Connectivity
- **WebRTC P2P** — direct connection for lowest latency (DTLS-SRTP encrypted)
- **Cloudflare TURN** — managed TURN as primary relay
- **Coturn fallback** — self-hosted Docker coturn as backup
- **STUN** — Google + Cloudflare STUN servers for NAT traversal
- **Connection-type badge** — viewer toolbar shows P2P/STUN/Relay icon
- **Auto-reconnect** — handles network interruptions gracefully

### Infrastructure
- **Cloudflare Tunnel** — all HTTP traffic via tunnel (no port forwarding needed)
- **Cloudflare Zero Trust** — Dashboard, Dockge, Beszel, Supabase behind email OTP
- **Caddy reverse proxy** — serves dashboard + downloads at `updates.hawkeye123.dk`
- **Dockge** — Docker Compose stack manager (replaces Portainer)
- **Beszel** — system monitoring (CPU, RAM, disk, Docker stats)
- **Glance** — home dashboard with service monitors, releases, device status
- **Auto-update** — agent + controller check `version.json` on startup (SHA256 verified)
- **Self-hosted Supabase** — runs in Docker, not cloud
- **GitHub Actions CI** — automated macOS builds, releases with auto-generated notes

### Security
- **Supabase Auth** — email verification + admin approval required
- **RLS policies** — database-level row security on all tables
- **Edge Function auth** — device registration requires API key
- **HMAC credentials** — time-limited TURN credentials via HMAC-SHA1
- **SYSTEM token** — Session 0 helper process for login screen access
- **Rate limiting** — 100 requests/min per user/device

### Management
- **Admin panel** — centralized user & device management at `/admin.html`
- **Device approval** — two-factor: user approval + device approval
- **System tray** — live status updates (connection mode, bitrate)
- **Console mode** — real-time log output for debugging
- **CLI tool** — `remote-desktop-cli` for scripted remote control
- **Claude Code integration** — `/remote-desktop` slash command for AI-assisted remote control

## Architecture

```
Controller (Windows/macOS)          Agent (Windows/macOS)
    |                                    |
    |-- WebRTC DataChannel (P2P) --------|
    |   - JPEG frames (libjpeg-turbo)    |
    |   - H.264 NAL units (OpenH264)     |
    |   - Mouse/keyboard events          |
    |   - File transfer                  |
    |   - Clipboard sync                 |
    |                                    |
    |-- Supabase Realtime (signaling) ---|
    |                                    |
    +-- TURN relay (Cloudflare/coturn) --+
```

### Key Components
- **Screen capture**: DXGI (Windows GPU) → GDI (fallback) → Quartz (macOS)
- **JPEG encoding**: libjpeg-turbo with SIMD (`-tags turbo`) → standard `image/jpeg` (fallback)
- **H.264 encoding**: OpenH264 via video track → JPEG tiles (fallback)
- **Input injection**: SendInput + SYSTEM token (Windows) → CGEvent (macOS)
- **Streaming modes**: idle-tiles (2 FPS, Q85) → active-tiles (20-25 FPS) → H.264

## Quick Start

### Download
Grab the latest installers from [GitHub Releases](https://github.com/stangtennis/Remote/releases/latest):
- `RemoteDesktopAgent-Setup.exe` — Windows agent (GUI + console + OpenH264 + TurboJPEG)
- `RemoteDesktopController-Setup.exe` — Windows controller

### Agent Setup
1. Run the installer or portable EXE as Administrator
2. Agent auto-registers with Supabase backend
3. Approve the device in the admin panel or controller

### Controller Setup
1. Run the installer or portable EXE
2. Login with your approved email/password
3. Select a device and connect

### macOS Agent
```bash
# Build natively with turbo JPEG
brew install libjpeg-turbo
cd agent && CGO_ENABLED=1 go build -tags turbo -o remote-agent ./cmd/remote-agent
# Run in Terminal (required for TCC Accessibility permission)
./remote-agent
```

## Build

**Prerequisites:** Go 1.25+, MinGW cross-compiler (`x86_64-w64-mingw32-gcc`)

```bash
# Full build (all platforms + installers)
./build-local.sh v2.99.57

# Manual Windows agent (with turbo JPEG)
cd agent && GOOS=windows GOARCH=amd64 CGO_ENABLED=1 \
  CC=x86_64-w64-mingw32-gcc CXX=x86_64-w64-mingw32-g++ \
  go build -tags turbo -ldflags '-s -w -H windowsgui' \
  -o ../builds/remote-agent.exe ./cmd/remote-agent

# Manual Windows controller
cd controller && GOOS=windows GOARCH=amd64 CGO_ENABLED=1 \
  CC=x86_64-w64-mingw32-gcc \
  go build -tags desktop,production -ldflags '-s -w -H windowsgui' \
  -o ../builds/controller.exe .
```

Version is injected via `-ldflags -X` — no source code changes needed.

### Build Tags
- `turbo` — enables libjpeg-turbo SIMD JPEG encoding (requires `libturbojpeg` library)
- `desktop,production` — required for controller (Wails framework)

### Dependencies
- **libjpeg-turbo** — SIMD JPEG encoding (bundled as `libturbojpeg.dll` for Windows, `brew install` for macOS)
- **OpenH264** — H.264 encoding (bundled as `openh264-2.1.1-win64.dll`)
- **MinGW** — Windows cross-compilation from Linux

## Cost

| Service | Cost | Notes |
|---------|------|-------|
| Supabase (self-hosted) | $0/mo | Docker on local server |
| Cloudflare Tunnel + TURN | $0/mo | Free tier |
| Coturn (backup) | $0/mo | Docker on same server |
| Dockge + Beszel + Glance | $0/mo | Docker monitoring stack |
| **Total** | **$0/mo** | Fully self-hosted |

## License

MIT License — see [LICENSE](./LICENSE)
