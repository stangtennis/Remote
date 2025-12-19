# 🎮 Remote Desktop Controller

**Standalone Windows application for controlling remote clients** - Like TeamViewer!

## ✅ Status: Production Ready (v2.63.9)

The controller application is **fully functional** and ready for use!

## 🚀 Quick Start

### Download
Get the latest release from [GitHub Releases](https://github.com/stangtennis/Remote/releases)

### Run
1. Download `controller-v2.63.9.exe` or `RemoteDesktopController-Setup-v2.63.9.exe`
2. Run the installer or EXE directly
3. Login with your credentials
4. Select a device and click Connect

---

## 📋 Features (v2.63.9)

### Core
- ✅ **Native Windows UI** - Built with Fyne
- ✅ **Supabase Auth** - Login with email/password
- ✅ **Device Management** - View and approve devices
- ✅ **WebRTC Streaming** - Real-time remote desktop view

### Remote Control
- ✅ **Full Input Control** - Mouse, keyboard, scroll
- ✅ **Fullscreen Mode** - F11/ESC with auto-hide toolbar
- ✅ **Adaptive Quality** - Auto-adjusts based on network

### File & Clipboard
- ✅ **File Browser** - Browse remote drives and folders
- ✅ **File Transfer** - Download files from remote machine
- ✅ **Clipboard Sync** - Copy/paste text and images

### Stats & Monitoring
- ✅ **Real-time Stats** - FPS, quality, RTT, CPU display
- ✅ **Connection Status** - Online/offline indicators

---

## 🛠️ Development

### Prerequisites

- Go 1.21+
- Windows (for now)

### Build Locally

```bash
# Development mode
go run main.go

# Build executable
go build -ldflags "-s -w -H windowsgui" -o controller.exe

# Or use build script
.\build.bat
```

### Build on GitHub

**Automatic builds via GitHub Actions:**

1. **Push to `controller` branch** - Triggers build
2. **Download artifact** - From Actions tab
3. **Create release** - Tag with `controller-v0.2.0`

```bash
# Push to controller branch
git checkout controller
git push origin controller

# GitHub Actions builds controller.exe automatically
# Download from: Actions → Build Controller Application → Artifacts
```

### Release Process

```bash
# Create and push tag
git tag controller-v0.2.0
git push origin controller-v0.2.0

# GitHub Actions will:
# 1. Build controller.exe
# 2. Create GitHub Release
# 3. Upload controller.exe to release
```

### Project Structure

```
controller/
├── main.go              # Main application
├── go.mod               # Dependencies
├── build.bat            # Build script
├── run.bat              # Run script
├── README.md            # This file
├── QUICKSTART.md        # Quick start guide
└── .env.example         # Configuration template
```

---

## 🎯 Roadmap

### Week 1-2: Core Functionality
- [x] Create prototype UI
- [ ] Add Supabase authentication
- [ ] Fetch real device list
- [ ] Implement WebRTC viewer

### Week 3-4: Remote Control
- [ ] Capture mouse/keyboard input
- [ ] Send via WebRTC data channel
- [ ] Test with existing agents
- [ ] Add connection management

### Week 5-6: Polish
- [ ] System tray integration
- [ ] Multi-session support
- [ ] File transfer
- [ ] Settings panel

---

## 📚 Documentation

- **[QUICKSTART.md](./QUICKSTART.md)** - Quick start guide
- **[../CONTROLLER_APP_PLAN.md](../CONTROLLER_APP_PLAN.md)** - Complete implementation plan

---

## 🎉 Try It Now!

```bash
cd controller
.\run.bat
```

The app will open and you can test the UI!
