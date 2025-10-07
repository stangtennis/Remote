# 🖥️ Remote Desktop Application

A lightweight, serverless remote desktop solution built with **Supabase**, **WebRTC**, and **GitHub Pages**.

## ✅ Status: **Production Ready** (Updated 2025-10-07)

- ✅ **High-quality screen streaming** (1920px @ 15 FPS)
- ✅ **System tray integration** - Runs silently in background
- ✅ **Stable reconnection** - Automatic cleanup and recovery
- ✅ **Mouse & keyboard control** - Full remote input
- ✅ **External access** - Works across networks via TURN relay
- ✅ **Automated releases** - GitHub Actions CI/CD
- ✅ **Clean logging** - Minimal, informative output

## Architecture

- **Backend**: Supabase (Database, Realtime, Storage, Edge Functions, Auth)
- **Dashboard**: GitHub Pages (Static hosting)
- **Agent**: Go + Pion WebRTC (Single Windows EXE)
- **Connectivity**: WebRTC P2P with TURN fallback

## ✨ Key Features

- **🔒 Secure** - WebRTC P2P encryption, Supabase RLS, short-lived tokens
- **🚀 Fast** - Direct P2P connection when possible, TURN fallback
- **📦 Portable** - Single EXE file, no installation required
- **🔔 System Tray** - Runs minimized in notification area
- **🔄 Auto-Reconnect** - Handles network interruptions gracefully
- **📊 Clean Logs** - View activity from system tray menu
- **🌐 Cross-Network** - Works behind NAT/firewalls via TURN

## 📥 Quick Start (For Users)

### Download & Install Agent

1. **Download** the latest release:
   ```
   https://github.com/stangtennis/Remote/releases/latest
   ```

2. **Extract** `remote-agent-windows.zip`

3. **Configure** - Create `.env` file with your Supabase credentials:
   ```env
   SUPABASE_URL=https://your-project.supabase.co
   SUPABASE_ANON_KEY=your-anon-key
   ```

4. **Run Setup** - Double-click `setup-startup.bat` to install as startup task

5. **Done!** - Agent runs on startup, visible in system tray

### Access Dashboard

Visit: `https://your-username.github.io/Remote/`

## 📁 Project Structure

```
Remote/
├── .github/
│   └── workflows/         # GitHub Actions (automated releases)
├── agent/                 # Go agent application
│   ├── cmd/remote-agent/  # Main entry point
│   ├── internal/          # Core packages
│   │   ├── webrtc/       # WebRTC peer connection
│   │   ├── screen/       # Screen capture
│   │   ├── input/        # Mouse/keyboard control
│   │   ├── tray/         # System tray integration
│   │   └── device/       # Device registration
│   ├── build.bat         # Local build script
│   └── setup-startup.bat # Installation script
├── docs/                  # GitHub Pages dashboard
│   ├── index.html
│   ├── css/
│   └── js/
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

### ✅ Completed Features

- [x] **Infrastructure** - Supabase backend, database, Edge Functions
- [x] **Authentication** - Supabase Auth with RLS policies
- [x] **Dashboard** - Web interface hosted on GitHub Pages
- [x] **Agent Core** - Screen capture, WebRTC streaming
- [x] **Input Control** - Mouse & keyboard remote control
- [x] **TURN Relay** - Cross-network connectivity via Twilio
- [x] **Reconnection** - Automatic cleanup and recovery
- [x] **System Tray** - Background operation with menu
- [x] **Automated Releases** - GitHub Actions CI/CD
- [x] **Session Cleanup** - Automatic via pg_cron

### 🚧 Planned Enhancements

- [ ] **Video Encoding** - H.264/VP8 for better performance
- [ ] **File Transfer** - Send/receive files during session
- [ ] **Multi-Monitor** - Select which screen to stream
- [ ] **Code Signing** - Windows EXE certificate
- [ ] **Audio Streaming** - Remote audio support

## ⚠️ Known Limitations

- **Platform**: Windows only (agent)
- **Video Format**: JPEG frames @ 15 FPS (H.264/VP8 planned)
- **Multiple Tabs**: Use one dashboard tab per session
- **Code Signing**: Not implemented (Windows SmartScreen warning)

## 🔒 Security Features

- **🔐 Encryption** - WebRTC P2P encryption (DTLS-SRTP)
- **👤 Authentication** - Supabase Auth with MFA support
- **🛡️ RLS Policies** - Row-level security on all database tables
- **🎟️ Short-lived Tokens** - JWT expiration (5-15 minutes)
- **🔑 API Key Rotation** - Per-device key management
- **⏱️ Rate Limiting** - 100 requests/min per user/device
- **📝 Audit Logs** - Session history and device tracking

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

- **[BRANCHING_STRATEGY.md](./BRANCHING_STRATEGY.md)** - Git workflow and branch structure
- **[RELEASE.md](./RELEASE.md)** - Automated release process
- **[DEPLOYMENT.md](./DEPLOYMENT.md)** - Detailed deployment guide
- **[TESTING_GUIDE.md](./TESTING_GUIDE.md)** - Testing and troubleshooting
- **[OPTIMIZATION.md](./OPTIMIZATION.md)** - Performance tuning (H.264/VP8)
- **[CHANGELOG.md](./CHANGELOG.md)** - Version history

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
