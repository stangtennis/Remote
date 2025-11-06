# Release v2.0.0 (2025-11-06) - Maximum Quality Update 🚀

## 🎯 Major Features

### **MAXIMUM QUALITY MODE** - Optimized for High Bandwidth
The agent has been completely upgraded for users with unlimited bandwidth:

- **60 FPS** streaming (4x smoother than before)
- **JPEG Quality 95** (near-lossless compression)
- **4K Resolution Support** (up to 3840px)
- **Lanczos3 Scaling** (highest quality algorithm)
- **10MB Buffer** (prevents frame drops on fast networks)

### **Device Approval in Controller**
- New "Approve Devices" tab in controller
- Approve pending devices directly from the UI
- Auto-refresh after approval
- Confirmation dialogs for safety

### **Improved Login Experience**
- Login form hides after successful login
- Clear visual indication of logged-in state
- Shows only status, logout, and restart buttons when logged in
- Cleaner, less cluttered interface

## 📊 Performance Specs

| Metric | Previous | v2.0.0 | Improvement |
|--------|----------|--------|-------------|
| Frame Rate | 15 FPS | **60 FPS** | 4x smoother |
| JPEG Quality | 60 | **95** | Near-lossless |
| Max Resolution | 1920px | **3840px** | 4K support |
| Buffer Size | 1MB | **10MB** | Less drops |
| Bandwidth | 0.5-2 MB/s | **5-15 MB/s** | High quality |

## 🔧 Technical Improvements

### Agent (Client)
- ✅ 60 FPS screen streaming
- ✅ JPEG quality 95 encoding
- ✅ 4K resolution support (3840x2160)
- ✅ Lanczos3 high-quality scaling
- ✅ Version info with build date
- ✅ Optimized for high bandwidth networks

### Controller
- ✅ Device approval UI
- ✅ Hide login form when logged in
- ✅ Version info with build date
- ✅ Improved user experience
- ✅ Auto-refresh device lists

## 📦 Downloads

### Remote Agent (Client) - v2.0.0
**For the computer you want to control**
- `remote-agent.exe` - Windows executable
- Install on the remote PC
- Runs as Windows Service or startup task

### Remote Controller - v2.0.0
**For the computer you control from**
- `controller.exe` - Windows executable
- Login and approve devices
- Connect to remote agents

## 🚀 Installation

### Agent Setup
1. Download `remote-agent.exe`
2. Run `install-service.bat` (as Admin) for auto-start
3. Agent will register and wait for approval

### Controller Setup
1. Download `controller.exe`
2. Run and login with your credentials
3. Go to "Approve Devices" tab
4. Approve your agent
5. Go to "My Devices" and click "Connect"

## 🎮 Perfect For

- ✅ High-end gaming
- ✅ Video editing
- ✅ Graphic design
- ✅ CAD/3D modeling
- ✅ Any visual work requiring precision
- ✅ Users with fast internet connections

## ⚠️ Requirements

- **Bandwidth**: 5-15 MB/s recommended for best quality
- **Windows**: Windows 10/11
- **Network**: Low latency connection preferred
- **CPU**: Modern multi-core processor for 60 FPS encoding

## 📝 Known Limitations

- Controller viewer WebRTC connection not yet implemented (coming soon)
- File transfer feature pending
- Clipboard sync pending

## 🔜 Coming Next

- WebRTC connection in controller viewer
- Video rendering and input forwarding
- File transfer between devices
- Clipboard synchronization
- Multi-monitor support

---

**Full Changelog**: https://github.com/stangtennis/Remote/compare/v1.1.7...v2.0.0
