# 🖥️ Remote Desktop Application

A **professional remote desktop solution** built with **Supabase**, **WebRTC**, and **Go** - like TeamViewer, but self-hosted!

## ✅ Status: **FULLY FUNCTIONAL** (Updated 2025-12-10)

### 🎮 Controller Application v2.38.0 ✨ **WORKING!**
- ✅ **Standalone Windows EXE** - Native controller app (like TeamViewer)
- ✅ **Auto Admin Elevation** - UAC prompt on startup
- ✅ **Real Supabase Auth** - Login with email/password
- ✅ **Credential Storage** - Remember me functionality
- ✅ **Device Management** - Approve/remove devices directly in controller
- ✅ **WebRTC Video Streaming** - Live remote desktop view! 🎉
- ✅ **Full Input Control** - Mouse, keyboard, and scroll working!
- ✅ **Fullscreen Toggle** - Immersive viewing mode
- ✅ **TURN Server Support** - Works across NAT/firewalls
- 📦 **Auto-builds on GitHub** - Download from Releases
- 🏷️ **GitHub Releases** - Automated via GitHub Actions

### 🖥️ Agent Options
- ✅ **Windows Native Agent** (v2.38.0) - **FULLY WORKING!**
  - Auto admin elevation (UAC prompt if needed)
  - 30 FPS streaming (optimized for latency)
  - JPEG Quality 95 (near-lossless)
  - DXGI screen capture (works over RDP!)
  - Full mouse & keyboard control
  - Accurate coordinate mapping
  - Auto-registers, no login required
  - Windows Service support (login screen capture)
  - Self-hosted TURN server for NAT traversal
- ✅ **Web Dashboard** - Browser-based control interface
- ✅ **Browser Extension** - Remote control for web agent
- 🚧 **Electron Agent** - Cross-platform desktop (prototype)

### 🔧 Device Management
- ✅ **Anonymous Registration** - Agents auto-register without login
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

### ✅ Completed Features (v2.38.0 - 2025-12-10)

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

#### New in v2.38.0 (2025-12-10) 🎉
- [x] **Self-Elevation** - Auto UAC prompt if not running as admin
- [x] **Admin Manifest** - Embedded in both agent and controller
- [x] **Credential Storage** - Remember me with secure storage
- [x] **Fullscreen Toggle** - In controller viewer
- [x] **Dashboard Scrolling** - Fixed UI scaling issues
- [x] **Logout Button** - Working dashboard logout
- [x] **Self-hosted TURN** - Coturn on dedicated server (188.228.14.94)
- [x] **Connection Cleanup** - Disconnect old sessions on new connect
- [x] **macOS Controller** - Builds for Intel and Apple Silicon

#### Previous Features
- [x] **WebRTC Video Streaming** - Live remote desktop view
- [x] **Frame Chunk Reassembly** - Handles large frames correctly
- [x] **Full Input Control** - Mouse, keyboard, and scroll
- [x] **30 FPS Streaming** - Optimized for latency
- [x] **DXGI Screen Capture** - Works over RDP sessions
- [x] **Windows Service Support** - Can run at login screen
- [x] **Device Approval in Controller** - Approve devices from controller app
- [x] **Admin Panel** - Web UI for approving users
- [x] **Enhanced Tray Menu** - Console window + log viewer

### 🚧 Planned Enhancements (v2.3.0+)

#### High Priority
- [ ] **Session 0 Screen Capture** - View login screen (requires helper process)
- [ ] **Clipboard Sync** - Copy/paste between machines
- [ ] **File Transfer** - Send/receive files during session
- [ ] **Multi-Monitor** - Select which screen to stream
- [ ] **Video Encoding** - H.264/VP8 for better compression
- [ ] **Quality Settings** - Adjustable FPS and quality in UI

#### Medium Priority
- [ ] **Audio Streaming** - Remote audio support
- [ ] **Code Signing** - Windows EXE certificate
- [ ] **Chrome Web Store** - Publish browser extension
- [ ] **Connection Stats** - FPS, latency, bandwidth display
- [ ] **Fullscreen Mode** - Immersive remote desktop view

#### Future Enhancements
- [ ] **Role-Based Access** - Separate admin vs user roles
- [ ] **Mobile Apps** - Android/iOS agents
- [ ] **Linux Agent** - Cross-platform support
- [ ] **macOS Agent** - Apple platform support

## ⚠️ Known Limitations

- **Platform**: Windows only (agent)
- **Video Format**: JPEG frames @ 30 FPS (H.264/VP8 planned for v2.3.0)
- **Login Screen**: Cannot capture Session 0 without helper process
- **Latency**: ~1 second delay (typical for JPEG streaming)
- **Code Signing**: Not implemented (Windows SmartScreen warning)
- **Bandwidth**: 3-8 MB/s required for 30 FPS @ quality 95

## 🔒 Security Features

### Authentication & Access Control
- **👤 Supabase Auth** - Email verification required
- **👥 User Approval** - Admin must approve all new users
- **🔐 Admin Panel** - Centralized user management
- **🛡️ RLS Policies** - Database-level security with approval checks
- **🎟️ Short-lived Tokens** - JWT expiration (5-15 minutes)
- **⏱️ Rate Limiting** - 100 requests/min per user/device

### Connection Security
- **🔐 WebRTC Encryption** - P2P encryption (DTLS-SRTP)
- **🔑 Device Approval** - Two-factor: user approval + device approval
- **📌 PIN Codes** - Random PIN for each session
- **🚫 Automatic Timeout** - Sessions expire after inactivity

### Monitoring & Audit
- **📝 Audit Logs** - Session history and device tracking
- **📊 User Activity** - Track sign-ups and approvals
- **🔍 Admin Oversight** - View all pending users

## 💰 Cost Estimation

| Service | Cost | Notes |
|---------|------|-------|
| Supabase Free Tier | $0/mo | Good for testing/personal use |
| Supabase Pro | $25/mo | Production (500GB bandwidth) |
| TURN (Self-hosted) | ~$5/mo | Coturn on VPS |
| GitHub Pages | Free | Static hosting |
| **Total** | **~$30/mo** | Production setup |

**Free Alternative**: Use Supabase free tier + free TURN services for personal use.

## 📚 Documentation

### 🎮 Controller Application (NEW!)
- **[controller/README.md](./controller/README.md)** - Main documentation
- **[controller/QUICKSTART.md](./controller/QUICKSTART.md)** - Quick start guide
- **[controller/CHANGELOG.md](./controller/CHANGELOG.md)** - Version history
- **[controller/TESTING.md](./controller/TESTING.md)** - Testing guide
- **[CONTROLLER_APP_PLAN.md](./CONTROLLER_APP_PLAN.md)** - Complete implementation plan

### Project Status & Planning
- **[PROJECT_STATUS.md](./PROJECT_STATUS.md)** - Current status & forward roadmap
- **[BRANCHING_STRATEGY.md](./BRANCHING_STRATEGY.md)** - Git workflow and branches

### Setup & Deployment
- **[RELEASE.md](./RELEASE.md)** - Automated release process
- **[DEPLOYMENT.md](./DEPLOYMENT.md)** - Detailed deployment guide

### User Guides
- **[USER_APPROVAL_GUIDE.md](./USER_APPROVAL_GUIDE.md)** - User approval system
- **[QUICKSTART-EXTENSION.md](./QUICKSTART-EXTENSION.md)** - Browser extension setup
- **[CONSOLE_MODE.md](./agent/CONSOLE_MODE.md)** - Debug/console mode

### Implementation Plans
- **[WEB_AGENT_IMPLEMENTATION_PLAN.md](./WEB_AGENT_IMPLEMENTATION_PLAN.md)** - Web agent design
- **[WEB_AGENT_CONTROL_SOLUTION.md](./WEB_AGENT_CONTROL_SOLUTION.md)** - Control solution
- **[ANDROID_IMPLEMENTATION_PLAN.md](./ANDROID_IMPLEMENTATION_PLAN.md)** - Android agent

### Troubleshooting & Optimization
- **[TESTING_GUIDE.md](./TESTING_GUIDE.md)** - Testing and troubleshooting
- **[OPTIMIZATION.md](./OPTIMIZATION.md)** - Performance tuning (H.264/VP8)

### Release History
- **[CHANGELOG.md](./CHANGELOG.md)** - Version history
- **[GitHub Releases](https://github.com/stangtennis/Remote/releases)** - All release notes and downloads

## 🔨 Build & Release (Linux Host)

- **Prereqs:** Go 1.25.x and MinGW for Windows CGO, or Docker (recommended). Repo root: `/home/dennis/projekter/Remote Desktop`.
- **Preferred (GitHub Actions):**
  1. `git tag v2.5.0`
  2. `git push origin v2.5.0`
  3. Workflows (`.github/workflows/release*.yml`) run on `windows-latest` with Go 1.25, build both EXEs, and attach them to the release.
- **Local cross-build via Docker (Linux → Windows):**
  ```
  docker run --rm -v "$PWD":/app -w /app golang:1.25 bash -lc '
    set -euo pipefail
    apt-get update -qq
    apt-get install -y -qq --no-install-recommends mingw-w64 > /dev/null
    export GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc
    mkdir -p /app/build
    cd agent && go build -ldflags "-s -w" -o /app/build/remote-agent.exe ./cmd/remote-agent
    cd /app/controller && go build -ldflags "-s -w -H windowsgui" -o /app/build/controller.exe .
  '
  ```
  Outputs: `build/remote-agent.exe`, `build/controller.exe`.
- **Flags:** `-ldflags "-s -w"` strips debug info; controller adds `-H windowsgui` to hide console. `CGO_ENABLED=1` is required for robotgo on the agent.
- **Releases:** Download from https://github.com/stangtennis/Remote/releases

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
