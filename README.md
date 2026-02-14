# 🖥️ Remote Desktop Application

A **professional remote desktop solution** built with **Supabase**, **WebRTC**, and **Go** - like TeamViewer, but self-hosted!

## ✅ Status: **FULLY FUNCTIONAL** (Updated 2026-02-14)

### 🎮 Controller Application v2.65.0 ✨ **WORKING!**
- ✅ **Standalone Windows EXE** - Native controller app (like TeamViewer)
- ✅ **Auto Admin Elevation** - UAC prompt on startup
- ✅ **Real Supabase Auth** - Login with email/password
- ✅ **Credential Storage** - Remember me functionality
- ✅ **Device Management** - Approve/remove devices directly in controller
- ✅ **WebRTC Video Streaming** - Live remote desktop view! 🎉
- ✅ **Full Input Control** - Mouse, keyboard, and scroll working!
- ✅ **Fullscreen Mode** - Auto-hide overlay toolbar (move mouse to top)
- ✅ **File Browser** - Browse and transfer files from remote machine
- ✅ **Clipboard Sync** - Copy/paste between machines
- ✅ **TURN Server Support** - Works across NAT/firewalls
- ✅ **Adaptive Streaming** - Auto-adjusts quality based on network
- 📦 **Auto-builds on GitHub** - Download from Releases
- 🏷️ **GitHub Releases** - Automated via GitHub Actions

### 🖥️ Agent Options
- ✅ **Windows Native Agent** (v2.65.0) - **FULLY WORKING!**
  - Auto admin elevation (UAC prompt if needed)
  - Adaptive FPS streaming (2-30 FPS based on activity)
  - **Bandwidth optimization** - Frame skipping on static desktop (50-80% savings)
  - JPEG Quality 50-85 (adaptive based on network)
  - DXGI screen capture (works over RDP!)
  - Full mouse & keyboard control
  - Accurate coordinate mapping
  - Auto-registers, no login required
  - Windows Service support (login screen capture)
  - Self-hosted TURN server for NAT traversal
  - File transfer support (browse remote drives)
  - Clipboard sync (text and images)
- ✅ **Web Agent** - Browser-based screen sharing agent at `/agent.html`
- ✅ **Web Dashboard** - Browser-based control interface
- ✅ **Browser Extension** - Remote control for web agent
- 🚧 **Electron Agent** - Cross-platform desktop (prototype)

### 🔧 Device Management
- ✅ **Authenticated Registration** - Agents register via secure edge function
- ✅ **Controller Approval** - Approve devices directly in controller app
- ✅ **Admin Assignment** - Assign devices to users via admin panel
- ✅ **User-Based Access** - Users see only assigned devices
- ✅ **Device Approval** - Admin approves devices for use
- ✅ **Reassignment** - Easy device reassignment between users

### 🌐 Connectivity
- ✅ **Self-hosted TURN Server** - Coturn on dedicated server
- ✅ **NAT Traversal** - Works behind firewalls
- ✅ **P2P when possible** - Direct connection for best latency
- ✅ **Automatic Fallback** - TURN relay when P2P fails

### 🌐 Web Dashboard & Admin Panel
- ✅ **GitHub Pages** - Live at https://stangtennis.github.io/Remote/
- ✅ **User Approval System** - Admin controls access
- ✅ **Device Management** - Assign/revoke device access
- ✅ **Admin Panel** - Centralized user & device management at `/admin.html`
- ✅ **RLS Policies** - Database-level security enforcement
- ✅ **Access Control** - Users see only assigned devices

### Performance & Reliability
- ✅ **Fast P2P** - Direct connection when possible, TURN fallback
- ✅ **High Quality** - 30 FPS @ JPEG 95, optimized for latency
- ✅ **Near-Lossless** - Exceptional visual fidelity for high bandwidth
- ✅ **Auto-Reconnect** - Handles network interruptions gracefully
- ✅ **Cross-Network** - Works behind NAT/firewalls via self-hosted TURN
- ✅ **Smart Buffering** - 10MB buffer prevents frame drops
- ✅ **Auto Admin** - Self-elevation with UAC prompt

### User Experience
- ✅ **Portable** - Single EXE file, no installation required
- ✅ **Enhanced Tray** - Console window, log viewer, version display
- ✅ **Console Mode** - View live logs in real-time

## 📋 Implementation Status

### ✅ Completed Features (v2.65.0 - 2026-02-14)

#### Core Functionality
- [x] **Infrastructure** - Supabase backend, database, Edge Functions
- [x] **Authentication** - Supabase Auth with RLS policies
- [x] **Dashboard** - Web interface hosted on GitHub Pages
- [x] **Agent Core** - Screen capture, WebRTC streaming
- [x] **Input Control** - Mouse & keyboard remote control
- [x] **Self-hosted TURN** - Coturn server for NAT traversal
- [x] **Reconnection** - Automatic cleanup and recovery
- [x] **Automated Releases** - GitHub Actions CI/CD
- [x] **Session Cleanup** - Automatic via pg_cron

#### New in v2.65.0 (2026-02-14) 🔒
- [x] **Security Hardening** - RLS on all tables, auth on edge functions, origin-restricted WebSocket
- [x] **Web Agent** - Browser-based screen sharing via WebRTC
- [x] **Build Fixes** - All cross-compile errors resolved
- [x] **Self-hosted Supabase** - Runs locally in Docker, not cloud

#### v2.64.0 (2025-12-18) 🎉
- [x] **Bandwidth Optimization** - Frame skipping on static desktop (50-80% savings)
- [x] **Fullscreen Overlay Toolbar** - Auto-hide toolbar in fullscreen mode
- [x] **File Browser** - Browse remote drives and transfer files
- [x] **Clipboard Sync** - Copy/paste text and images between machines
- [x] **Adaptive Streaming** - Auto-adjusts FPS/quality based on network & CPU
- [x] **Idle Mode** - 2 FPS + high quality when desktop is static
- [x] **Motion Detection** - Dirty region detection for bandwidth optimization
- [x] **Stats Display** - Real-time FPS, quality, RTT, CPU in controller

#### Previous Features (v2.38.0 - v2.63.x)
- [x] **WebRTC Video Streaming** - Live remote desktop view
- [x] **Frame Chunk Reassembly** - Handles large frames correctly
- [x] **Full Input Control** - Mouse, keyboard, and scroll
- [x] **Adaptive FPS Streaming** - 2-30 FPS based on activity
- [x] **DXGI Screen Capture** - Works over RDP sessions
- [x] **Windows Service Support** - Can run at login screen
- [x] **Device Approval in Controller** - Approve devices from controller app
- [x] **Admin Panel** - Web UI for approving users
- [x] **Enhanced Tray Menu** - Console window + log viewer
- [x] **Self-Elevation** - Auto UAC prompt if not running as admin
- [x] **Credential Storage** - Remember me with secure storage

### 🚧 Planned Enhancements

#### High Priority
- [ ] **Hardware H.264 Encoding** - GPU-accelerated video for better compression
- [ ] **Multi-Monitor** - Select which screen to stream
- [ ] **Session 0 Screen Capture** - View login screen (requires helper process)

#### Medium Priority
- [ ] **Audio Streaming** - Remote audio support
- [ ] **Code Signing** - Windows EXE certificate
- [ ] **Chrome Web Store** - Publish browser extension

#### Future Enhancements
- [ ] **Role-Based Access** - Separate admin vs user roles
- [ ] **Mobile Apps** - Android/iOS agents
- [ ] **Linux Agent** - Cross-platform support
- [ ] **macOS Agent** - Apple platform support

## ⚠️ Known Limitations

- **Platform**: Windows only (agent)
- **Video Format**: JPEG frames with adaptive quality (H.264 software available, hardware planned)
- **Login Screen**: Cannot capture Session 0 without helper process
- **Latency**: ~50-150ms typical (depends on network)
- **Code Signing**: Not implemented (Windows SmartScreen warning)
- **Bandwidth**: ~0.5-2 Mbit/s static, ~10-25 Mbit/s active (with frame skipping optimization)

## 🔒 Security Features

### Authentication & Access Control
- **👤 Supabase Auth** - Email verification required
- **👥 User Approval** - Admin must approve all new users
- **🔐 Admin Panel** - Centralized user management
- **🛡️ RLS Policies** - Enabled on all tables (devices, sessions, signaling)
- **🔑 Edge Function Auth** - Device registration requires API key verification
- **⏱️ Rate Limiting** - 100 requests/min per user/device

### Connection Security
- **🔐 WebRTC Encryption** - P2P encryption (DTLS-SRTP)
- **🔑 Device Approval** - Two-factor: user approval + device approval
- **🌐 Origin Restrictions** - WebSocket input-helper only accepts known origins
- **🚫 Automatic Timeout** - Sessions expire after inactivity
- **🔒 Session Takeover RPCs** - Only authenticated users can call

### Monitoring & Audit
- **📝 Audit Logs** - Session history and device tracking
- **📊 User Activity** - Track sign-ups and approvals
- **🔍 Admin Oversight** - View all pending users

## 💰 Cost Estimation

| Service | Cost | Notes |
|---------|------|-------|
| Supabase (self-hosted) | $0/mo | Runs in Docker on local server |
| TURN (Self-hosted) | ~$5/mo | Coturn on VPS |
| GitHub Pages | Free | Static hosting for dashboard |
| Caddy Reverse Proxy | $0/mo | Runs on same server |
| **Total** | **~$5/mo** | Self-hosted setup |

**Note**: Supabase runs self-hosted in Docker, not on Supabase cloud.

## 📚 Documentation

### Setup & Configuration
- **[CONFIGURATION.md](./CONFIGURATION.md)** - Environment variables and setup
- **[AGENTS.md](./AGENTS.md)** - Contributor guide (repo layout, build, test)
- **[ULTIMATE_GUIDE.md](./ULTIMATE_GUIDE.md)** - Comprehensive project guide

### Component Docs
- **[agent/README.md](./agent/README.md)** - Agent documentation
- **[agent/CONSOLE_MODE.md](./agent/CONSOLE_MODE.md)** - Debug/console mode
- **[controller/README.md](./controller/README.md)** - Controller documentation
- **[controller/QUICKSTART.md](./controller/QUICKSTART.md)** - Quick start guide
- **[controller/CHANGELOG.md](./controller/CHANGELOG.md)** - Version history
- **[docs/README.md](./docs/README.md)** - Dashboard/web agent docs
- **[extension/README.md](./extension/README.md)** - Browser extension
- **[supabase/README-MIGRATIONS.md](./supabase/README-MIGRATIONS.md)** - Database migrations

### Infrastructure
- **[caddy/README.md](./caddy/README.md)** - Caddy reverse proxy setup
- **[docs/INPUT_HELPER_PROTOCOL.md](./docs/INPUT_HELPER_PROTOCOL.md)** - Input helper WebSocket protocol

### Release History
- **[GitHub Releases](https://github.com/stangtennis/Remote/releases)** - All release notes and downloads

## 🔨 Build & Release (Linux → Windows Cross-Compile)

**Prereqs:** Go 1.25+, MinGW cross-compiler (`x86_64-w64-mingw32-gcc`).

```bash
# Agent (GUI - no console window)
cd agent && GOOS=windows GOARCH=amd64 CGO_ENABLED=1 \
  CC=x86_64-w64-mingw32-gcc CXX=x86_64-w64-mingw32-g++ \
  go build -ldflags '-s -w -H windowsgui' -o ../builds/remote-agent.exe ./cmd/remote-agent

# Agent (Console - with logging output)
cd agent && GOOS=windows GOARCH=amd64 CGO_ENABLED=1 \
  CC=x86_64-w64-mingw32-gcc CXX=x86_64-w64-mingw32-g++ \
  go build -ldflags '-s -w' -o ../builds/remote-agent-console.exe ./cmd/remote-agent

# Controller
cd controller && GOOS=windows GOARCH=amd64 CGO_ENABLED=1 \
  CC=x86_64-w64-mingw32-gcc \
  go build -ldflags '-s -w -H windowsgui' -o ../builds/controller.exe .
```

**Version:** Update version in `agent/internal/tray/tray.go` and `controller/main.go` before building.

**Dashboard:** Push to `main` branch → GitHub Pages auto-deploys.

**Downloads:** Copy built EXEs to Caddy downloads server for auto-update.

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch (`agent` or `dashboard`)
3. Make your changes
4. Test thoroughly
5. Submit a pull request

## 📄 License

MIT License - See [LICENSE](./LICENSE) for details

## 📞 Support

- **Issues**: [GitHub Issues](https://github.com/stangtennis/Remote/issues)
- **Discussions**: [GitHub Discussions](https://github.com/stangtennis/Remote/discussions)

---

**Made with ❤️ using Supabase, WebRTC, and Go**
