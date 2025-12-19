# Controller Application Changelog

## [v2.63.9] - 2025-12-18

### Added
- ✅ **Fullscreen Overlay Toolbar** - Auto-hide toolbar when mouse at top of screen
- ✅ **File Browser** - Browse remote drives and folders
- ✅ **File Transfer** - Download files from remote machine
- ✅ **Clipboard Sync** - Copy/paste text and images between machines
- ✅ **Stats Display** - Real-time FPS, quality, RTT, CPU in status bar

### Fixed
- Fixed file transfer channel callback override issue
- Fixed file browser filenames not displaying correctly
- Fixed JPEG decode errors with chunked frames
- Fixed Fyne thread errors with fyne.Do() wrapping

---

## [v2.63.x] - 2025-12-17

### Added
- ✅ **Adaptive Streaming** - Auto-adjusts quality based on network/CPU
- ✅ **Quality Slider** - Adjust streaming quality in real-time
- ✅ **H.264 Toggle** - Switch between JPEG and H.264 modes

---

## [v2.38.0] - 2025-12-10

### Added
- ✅ **Self-Elevation** - Auto UAC prompt if not running as admin
- ✅ **Credential Storage** - Remember me with secure storage
- ✅ **Fullscreen Toggle** - F11/ESC for fullscreen mode
- ✅ **TURN Server Support** - Works across NAT/firewalls

---

## [v0.2.0] - 2025-11-04

### Added
- ✅ **Supabase Authentication** - Real login with email/password
- ✅ **Real Device List** - Fetches actual devices from database
- ✅ **Device Status Indicators** - Shows online/offline/away status

---

## [v0.1.0] - 2025-11-04

### Added
- ✅ **Initial Prototype** - Working Fyne application
- ✅ **Login Window** - Email/password input fields
- ✅ **Device List** - Mock device list with status indicators

### v0.4.0 - Remote Control
- [ ] Mouse input capture
- [ ] Keyboard input capture
- [ ] Send via WebRTC data channel
- [ ] Test with existing agents

### v1.0.0 - Production Release
- [ ] System tray integration
- [ ] Multi-session support
- [ ] File transfer
- [ ] Auto-updates
- [ ] Code signing
