# 🖥️ Remote Desktop Application

A lightweight, serverless remote desktop solution built with **Supabase**, **WebRTC**, and **GitHub Pages**.

## ✅ Status: **Production Ready** (v1.1.7 - Updated 2025-01-09)

### Core Features
- ✅ **High-quality screen streaming** (1920px @ 15 FPS, optimized quality)
- ✅ **User approval system** - Admin controls who can register
- ✅ **System tray integration** - Enhanced menu with console & log viewer
- ✅ **Stable reconnection** - Automatic cleanup and recovery
- ✅ **Mouse & keyboard control** - Full remote input (double-click fixed!)
- ✅ **External access** - Works across networks via TURN relay
- ✅ **Automated releases** - GitHub Actions CI/CD
- ✅ **Admin panel** - Approve users, monitor access
- ✅ **Console mode** - View live logs anytime

### 🆕 New: Multiple Agent Options
- ✅ **Windows Native Agent** - Full-featured, production-ready
- 🆕 **Web Agent** - Browser-based, no installation required!
- 🆕 **Browser Extension** - Add remote control to web agent
- 🚧 **Electron Agent** - Cross-platform desktop app (prototype)

## Architecture

- **Backend**: Supabase (Database, Realtime, Storage, Edge Functions, Auth)
- **Dashboard**: GitHub Pages (Static hosting)
- **Agent**: Go + Pion WebRTC (Single Windows EXE)
- **Connectivity**: WebRTC P2P with TURN fallback

## ✨ Key Features

### Security & Access Control
- **🔒 WebRTC Encryption** - P2P encryption with DTLS-SRTP
- **👥 User Approval** - Admin must approve all new users
- **🛡️ Admin Panel** - Centralized user management at `/admin.html`
- **🔐 RLS Policies** - Database-level security enforcement

### Performance & Reliability
- **🚀 Fast P2P** - Direct connection when possible, TURN fallback
- **⚡ Optimized Streaming** - JPEG quality 60, frame dropping on congestion
- **🔄 Auto-Reconnect** - Handles network interruptions gracefully
- **🌐 Cross-Network** - Works behind NAT/firewalls via TURN

### User Experience
- **📦 Portable** - Single EXE file, no installation required
- **🔔 Enhanced Tray** - Console window, log viewer, version display
- **🪟 Console Mode** - View live logs in real-time
- **🎮 Fixed Input** - No more double-clicks or arrow key issues
- **📊 Live Monitoring** - PowerShell window with tailed logs

## 📥 Quick Start (For Users)

### 1. Sign Up & Get Approved

1. **Visit Dashboard**: `https://stangtennis.github.io/Remote/`
2. **Create Account** - Sign up with your email
3. **Verify Email** - Click the verification link
4. **Wait for Approval** - Admin must approve your account
5. **Login** - Once approved, you can access the dashboard

### 2. Choose Your Agent

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
│   └── workflows/         # GitHub Actions (automated releases)
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
│   ├── agent.html        # 🆕 Web agent (browser-based)
│   ├── admin.html        # Admin panel
│   ├── css/
│   └── js/
│       ├── app.js
│       ├── webrtc.js
│       └── web-agent.js  # 🆕 Web agent logic
├── extension/             # 🆕 Browser extension
│   ├── manifest.json
│   ├── background.js
│   ├── content.js
│   └── icons/
├── native-host/           # 🆕 Native messaging helper
│   ├── main.go           # Input control helper
│   ├── build.bat
│   └── install-*.sh/bat  # Platform installers
├── electron-agent/        # 🚧 Electron agent (prototype)
│   └── ...
└── supabase/              # Supabase backend
    ├── migrations/        # Database schema
    └── functions/         # Edge Functions
```

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

### ✅ Completed Features (v1.1.7)

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

#### New in v1.1.7
- [x] **User Approval System** - Admin controls who can register
- [x] **Admin Panel** - Web UI for approving users
- [x] **Enhanced Tray Menu** - Console window + log viewer
- [x] **Console Mode** - Live log viewing (PowerShell tail)
- [x] **Input Fixes** - No more double-clicks or arrow key issues
- [x] **Performance** - Optimized JPEG quality (60) with frame dropping
- [x] **Documentation** - USER_APPROVAL_GUIDE.md, CONSOLE_MODE.md

### 🚧 Planned Enhancements

- [ ] **Controller Application** - 🆕 Standalone Windows EXE (TeamViewer-style) for admins
- [ ] **Chrome Web Store** - Publish browser extension
- [ ] **Video Encoding** - H.264/VP8 for better performance
- [ ] **File Transfer** - Send/receive files during session
- [ ] **Multi-Monitor** - Select which screen to stream
- [ ] **Code Signing** - Windows EXE certificate
- [ ] **Audio Streaming** - Remote audio support
- [ ] **Role-Based Access** - Separate admin vs user roles
- [ ] **Mobile Apps** - Android/iOS agents

## ⚠️ Known Limitations

- **Platform**: Windows only (agent)
- **Video Format**: JPEG frames @ 15 FPS (H.264/VP8 planned)
- **Multiple Tabs**: Use one dashboard tab per session
- **Code Signing**: Not implemented (Windows SmartScreen warning)

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

### Project Status
- **[PROJECT_STATUS.md](./PROJECT_STATUS.md)** - 🆕 Current status & forward plan
- **[CONTROLLER_APP_PLAN.md](./CONTROLLER_APP_PLAN.md)** - 🆕 Standalone controller application plan

### Setup & Deployment
- **[BRANCHING_STRATEGY.md](./BRANCHING_STRATEGY.md)** - Git workflow and branch structure
- **[RELEASE.md](./RELEASE.md)** - Automated release process
- **[DEPLOYMENT.md](./DEPLOYMENT.md)** - Detailed deployment guide

### User Guides
- **[USER_APPROVAL_GUIDE.md](./USER_APPROVAL_GUIDE.md)** - Complete guide to user approval system
- **[QUICKSTART-EXTENSION.md](./QUICKSTART-EXTENSION.md)** - 🆕 Browser extension quick start
- **[CONSOLE_MODE.md](./agent/CONSOLE_MODE.md)** - How to use debug/console mode

### Implementation Plans
- **[WEB_AGENT_IMPLEMENTATION_PLAN.md](./WEB_AGENT_IMPLEMENTATION_PLAN.md)** - 🆕 Web agent design
- **[WEB_AGENT_CONTROL_SOLUTION.md](./WEB_AGENT_CONTROL_SOLUTION.md)** - 🆕 Control solution analysis
- **[ANDROID_IMPLEMENTATION_PLAN.md](./ANDROID_IMPLEMENTATION_PLAN.md)** - Android agent plan

### Troubleshooting & Optimization
- **[TESTING_GUIDE.md](./TESTING_GUIDE.md)** - Testing and troubleshooting
- **[OPTIMIZATION.md](./OPTIMIZATION.md)** - Performance tuning (H.264/VP8)

### Release History
- **[CHANGELOG.md](./CHANGELOG.md)** - Version history
- **[RELEASE_NOTES_v1.1.7.md](./RELEASE_NOTES_v1.1.7.md)** - Latest release notes

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
