# 🖥️ Remote Desktop Application

A **professional remote desktop solution** built with **Supabase**, **WebRTC**, and **Go** - like TeamViewer, but self-hosted!

## ✅ Status: **FULLY FUNCTIONAL** (Updated 2025-11-11)

### 🎮 Controller Application v2.2.0 (2025-11-11) ✨ **WORKING!**
- ✅ **Standalone Windows EXE** - Native controller app (like TeamViewer)
- ✅ **Real Supabase Auth** - Login with email/password
- ✅ **Device Management** - Approve/remove devices directly in controller
- ✅ **WebRTC Video Streaming** - Live remote desktop view! 🎉
- ✅ **Full Input Control** - Mouse, keyboard, and scroll working!
- ✅ **Frame Chunk Reassembly** - Handles large frames correctly
- ✅ **Proper Disconnect** - Clean connection termination
- ✅ **30 FPS Streaming** - Optimized for low latency
- 📦 **Auto-builds on GitHub** - Download from Releases
- 🏷️ **GitHub Releases** - Automated via GitHub Actions

### 🖥️ Agent Options
- ✅ **Windows Native Agent** (v2.2.0) - **FULLY WORKING!**
  - 30 FPS streaming (optimized for latency)
  - JPEG Quality 95 (near-lossless)
  - DXGI screen capture (works over RDP!)
  - Full mouse & keyboard control
  - Accurate coordinate mapping
  - Auto-registers, no login required
  - Windows Service support (login screen capture)
- ✅ **Web Agent** - Browser-based, no installation required
- ✅ **Browser Extension** - Remote control for web agent
- 🚧 **Electron Agent** - Cross-platform desktop (prototype)

### 🔧 Device Management
- ✅ **Anonymous Registration** - Agents auto-register without login
- ✅ **Controller Approval** - Approve devices directly in controller app (NEW!)
- ✅ **Admin Assignment** - Assign devices to users via admin panel
- ✅ **User-Based Access** - Users see only assigned devices
- ✅ **Device Approval** - Admin approves devices for use
- ✅ **Reassignment** - Easy device reassignment between users

### 🌐 Web Dashboard & Admin Panel
- ✅ **GitHub Pages** - Live at https://stangtennis.github.io/Remote/
- ✅ **User Approval System** - Admin controls access
- ✅ **Device Management** - Assign/revoke device access
- ✅ **Tabbed Interface** - Users & Devices management
- ✅ **Real-time Updates** - Supabase Realtime integration
- ✅ **Visual Indicators** - Color-coded status and assignments

## 🏗️ Architecture

```
┌─────────────────────────────────────────────────────┐
│  CONTROLLER.EXE (Admin - NEW!)                      │
│  - Native Windows application                       │
│  - Login & device management                        │
│  - WebRTC viewer (coming soon)                      │
└────────────────┬────────────────────────────────────┘
                 │ WebRTC P2P
                 ↓
┌─────────────────────────────────────────────────────┐
│  AGENTS (Multiple Options)                          │
│  ├─ Windows Agent (Go EXE) - Production             │
│  ├─ Web Agent (Browser) - No install                │
│  ├─ Extension + Native Host - Full control          │
│  └─ Electron Agent - Cross-platform (prototype)     │
└────────────────┬────────────────────────────────────┘
                 │
                 ↓
┌─────────────────────────────────────────────────────┐
│  BACKEND (Supabase)                                 │
│  ├─ PostgreSQL - Devices, sessions, users           │
│  ├─ Realtime - WebRTC signaling                     │
│  ├─ Auth - User authentication                      │
│  └─ Edge Functions - Session cleanup                │
└─────────────────────────────────────────────────────┘
```

### Technology Stack
- **Controller**: Go + Fyne (Native Windows UI)
- **Backend**: Supabase (PostgreSQL, Realtime, Auth, Edge Functions)
- **Dashboard**: HTML/CSS/JS hosted on GitHub Pages
- **Agents**: Go (Windows), JavaScript (Web/Extension), Electron
- **WebRTC**: Pion (Go), Browser WebRTC API
- **Connectivity**: P2P with TURN fallback

## ✨ Key Features

### Security & Access Control
- **🔒 WebRTC Encryption** - P2P encryption with DTLS-SRTP
- **👥 User Approval** - Admin must approve all new users
- **📱 Device Assignment** - Admin assigns devices to users
- **🛡️ Admin Panel** - Centralized user & device management at `/admin.html`
- **🔐 RLS Policies** - Database-level security enforcement
- **🎯 Access Control** - Users see only assigned devices

### Performance & Reliability
- **🚀 Fast P2P** - Direct connection when possible, TURN fallback
- **⚡ MAXIMUM QUALITY** - 60 FPS @ JPEG 95, 4K support (v2.0.0)
- **💎 Near-Lossless** - Exceptional visual fidelity for high bandwidth
- **🔄 Auto-Reconnect** - Handles network interruptions gracefully
- **🌐 Cross-Network** - Works behind NAT/firewalls via TURN
- **📊 Smart Buffering** - 10MB buffer prevents frame drops

### User Experience
- **📦 Portable** - Single EXE file, no installation required
- **🔔 Enhanced Tray** - Console window, log viewer, version display
- **🪟 Console Mode** - View live logs in real-time
- **🎮 Fixed Input** - No more double-clicks or arrow key issues
- **📊 Live Monitoring** - PowerShell window with tailed logs

## 📥 Quick Start

### For Admins: Controller Application v2.0.0

**Best for:** Controlling multiple remote computers (like TeamViewer)

1. **Download Controller** (from GitHub Releases)
   ```
   https://github.com/stangtennis/Remote/releases/latest
   → Download controller.exe
   ```

2. **Run Controller**
   ```bash
   controller.exe
   ```

3. **Login** - Use your approved credentials

4. **Approve Devices** - Go to "Approve Devices" tab and approve pending devices

5. **See Devices** - View all assigned devices in "My Devices" tab

6. **Connect** - Click Connect to start remote session (WebRTC viewer coming in v2.1.0)

**See:** [controller/README.md](./controller/README.md) for details

---

### For Users: Choose Your Agent

#### 1. Sign Up & Get Approved

1. **Visit Dashboard**: `https://stangtennis.github.io/Remote/`
2. **Create Account** - Sign up with your email
3. **Verify Email** - Click the verification link
4. **Wait for Approval** - Admin must approve your account
5. **Login** - Once approved, you can access the dashboard

#### 2. Choose Your Agent

#### Option A: Windows Native Agent (Recommended)
**Best for:** Full control, always-on monitoring, Windows systems

1. **Download** the latest release:
   ```
   https://github.com/stangtennis/Remote/releases/latest
   ```

2. **Run Agent** - Double-click `remote-agent.exe`

3. **Enter Email** - On first run, enter your registered email

4. **Approve Device** - Go to dashboard and approve your device

5. **Connect!** - Click "Connect" in dashboard, enter PIN on agent

**System Tray Features:**
Right-click the tray icon to:
- **Show Console Window** - View live logs in PowerShell
- **View Log File** - Open full log in Notepad
- **Exit** - Stop the agent

#### Option B: Web Agent (No Installation!)
**Best for:** Locked-down computers, quick access, cross-platform

1. **Open Web Agent**: `https://stangtennis.github.io/Remote/agent.html`

2. **Login** - Use your approved email/password

3. **Start Screen Share** - Click button and select screen

4. **Connect!** - Device appears in dashboard, enter PIN when prompted

**Note:** View-only mode. For remote control, install the browser extension.

#### Option C: Web Agent + Extension (Full Control)
**Best for:** Remote control on locked-down systems

1. **Install Extension** - [Chrome Web Store link] (Coming soon)

2. **Install Native Helper** - Run installer from extension

3. **Open Web Agent** - Follow Option B steps above

4. **Full Control** - Mouse & keyboard control now enabled!

### Access Dashboard

Visit: `https://stangtennis.github.io/Remote/`

**Admin Panel**: `https://stangtennis.github.io/Remote/admin.html`

## 📁 Project Structure

```
Remote/
├── .github/
│   └── workflows/         # GitHub Actions
│       ├── release.yml    # Windows agent releases
│       └── build-controller.yml  # 🆕 Controller builds
├── controller/            # 🆕 Controller application (v0.2.0)
│   ├── main.go           # Main application
│   ├── internal/
│   │   ├── supabase/     # Supabase client
│   │   └── config/       # Configuration
│   ├── build.bat         # Build script
│   ├── run.bat           # Run script
│   ├── README.md         # Controller docs
│   ├── QUICKSTART.md     # Quick start guide
│   ├── CHANGELOG.md      # Version history
│   └── TESTING.md        # Testing guide
├── agent/                 # Windows native agent (Go)
│   ├── cmd/remote-agent/  # Main entry point
│   ├── internal/          # Core packages
│   │   ├── webrtc/       # WebRTC peer connection
│   │   ├── screen/       # Screen capture
│   │   ├── input/        # Mouse/keyboard control
│   │   ├── tray/         # System tray integration
│   │   └── device/       # Device registration
│   ├── build.bat         # Local build script
│   └── setup-startup.bat # Installation script
├── docs/                  # GitHub Pages dashboard + web agent
│   ├── index.html        # Dashboard
│   ├── agent.html        # Web agent (browser-based)
│   ├── admin.html        # Admin panel
│   ├── css/
│   └── js/
│       ├── app.js
│       ├── webrtc.js
│       └── web-agent.js  # Web agent logic
├── extension/             # Browser extension
│   ├── manifest.json
│   ├── background.js
│   ├── content.js
│   └── icons/
├── native-host/           # Native messaging helper
│   ├── main.go           # Input control helper
│   ├── build.bat
│   └── install-*.sh/bat  # Platform installers
├── electron-agent/        # Electron agent (prototype)
│   └── ...
└── supabase/              # Supabase backend
    ├── migrations/        # Database schema
    └── functions/         # Edge Functions
```

## 🌿 Development Branches

This project uses feature branches for organized development:

- **`main`** - Stable, production-ready code
- **`agent`** - Windows agent development
- **`dashboard`** - Web dashboard & backend
- **`controller`** - Controller application (auto-builds on push) 🆕

See [BRANCHING_STRATEGY.md](./BRANCHING_STRATEGY.md) for details.

---

## 🛠️ Development Setup

### Prerequisites

- [Supabase CLI](https://supabase.com/docs/guides/cli) - Backend deployment
- [Go 1.24+](https://golang.org/dl/) - Agent compilation
- [MinGW-w64](https://www.mingw-w64.org/) - CGO support (for input control)
- [Git](https://git-scm.com/) - Version control
- Supabase account

### 1. Clone & Configure

```bash
git clone https://github.com/stangtennis/Remote.git
cd Remote

# Copy environment template
cp .env.example .env
# Edit .env with your Supabase credentials
```

### 2. Deploy Supabase Backend

```bash
# Login to Supabase
supabase login

# Link to your project
supabase link --project-ref your-project-ref

# Run migrations
cd supabase
supabase db push

# Deploy Edge Functions
supabase functions deploy session-token
supabase functions deploy device-register
```

### 3. Build Agent Locally

```bash
cd agent

# Install dependencies
go mod download

# Build (Windows)
.\build.bat

# Or manual build
$env:CGO_ENABLED=1
go build -ldflags "-s -w -H windowsgui" -o remote-agent.exe ./cmd/remote-agent
```

### 4. Deploy Dashboard

The dashboard is hosted on GitHub Pages:

1. Push to GitHub
2. Settings → Pages
3. Source: `main` branch, `/docs` folder
4. Save

Access at: `https://your-username.github.io/Remote/`

## 🔄 Branching Strategy

- **`main`** - Stable, production-ready code
- **`agent`** - Agent development
- **`dashboard`** - Dashboard development

See [BRANCHING_STRATEGY.md](./BRANCHING_STRATEGY.md) for details.

## 📦 Releases

Releases are **automated via GitHub Actions**:

```bash
# Create new version
git tag v1.2.0
git push origin v1.2.0

# GitHub Actions will:
# 1. Build agent with CGO
# 2. Create GitHub Release
# 3. Upload remote-agent.exe
# 4. Upload remote-agent-windows.zip (with scripts)
```

See [RELEASE.md](./RELEASE.md) for details.

## 📋 Implementation Status

### ✅ Completed Features (v2.2.0 - 2025-11-11)

#### Core Functionality
- [x] **Infrastructure** - Supabase backend, database, Edge Functions
- [x] **Authentication** - Supabase Auth with RLS policies
- [x] **Dashboard** - Web interface hosted on GitHub Pages
- [x] **Agent Core** - Screen capture, WebRTC streaming
- [x] **Input Control** - Mouse & keyboard remote control (fixed!)
- [x] **TURN Relay** - Cross-network connectivity via Twilio
- [x] **Reconnection** - Automatic cleanup and recovery
- [x] **Automated Releases** - GitHub Actions CI/CD
- [x] **Session Cleanup** - Automatic via pg_cron

#### New in v2.2.0 (2025-11-11) - **MAJOR MILESTONE!** 🎉
- [x] **WebRTC Video Streaming** - Live remote desktop view working!
- [x] **Frame Chunk Reassembly** - Fixed black screen issue
- [x] **Full Input Control** - Mouse, keyboard, and scroll fully functional
- [x] **Interactive Canvas** - Captures all mouse and keyboard events
- [x] **Coordinate Scaling** - Accurate mouse positioning
- [x] **Click with Position** - Mouse moves to click location
- [x] **Proper Disconnect** - Closes connection and stops streaming
- [x] **30 FPS Streaming** - Reduced from 60 FPS for better latency
- [x] **DXGI Screen Capture** - Works over RDP sessions
- [x] **Windows Service Support** - Can run at login screen
- [x] **Enhanced Logging** - Better visibility of agent activity
- [x] **Service Installer** - Easy Windows Service installation

#### Previous Features (v2.0.0 - 2025-11-06)
- [x] **Device Approval in Controller** - Approve devices from controller app
- [x] **Improved Login UX** - Login form hides when logged in
- [x] **Version Info** - Build dates in agent and controller
- [x] **GitHub Actions** - Automated releases on tag push
- [x] **User Approval System** - Admin controls who can register
- [x] **Admin Panel** - Web UI for approving users
- [x] **Enhanced Tray Menu** - Console window + log viewer
- [x] **Console Mode** - Live log viewing (PowerShell tail)

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
| TURN (Twilio) | ~$112/mo | 280GB @ $0.40/GB |
| GitHub Pages | Free | Static hosting |
| **Total** | **~$140/mo** | Production setup |

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
- **[RELEASE_NOTES_v2.2.0.md](./RELEASE_NOTES_v2.2.0.md)** - Latest release notes (2025-11-11) 🎉
- **[RELEASE_NOTES_v2.0.0.md](./RELEASE_NOTES_v2.0.0.md)** - Previous release notes (2025-11-06)
- **[RELEASE_NOTES_v1.1.7.md](./RELEASE_NOTES_v1.1.7.md)** - Earlier release notes

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
