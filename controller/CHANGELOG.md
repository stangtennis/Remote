# Controller Application Changelog

## [v0.2.0] - 2025-11-04

### Added
- ✅ **Supabase Authentication** - Real login with email/password
- ✅ **User Approval Check** - Verifies user is approved before allowing access
- ✅ **Real Device List** - Fetches actual devices from database
- ✅ **Device Status Indicators** - Shows online/offline/away status
- ✅ **Connection Dialog** - Shows device info when connecting
- ✅ **Configuration System** - Loads from .env file or uses defaults
- ✅ **Error Handling** - Proper error messages for auth failures

### Changed
- Device list now uses real data from Supabase
- Login button disables during authentication
- Status messages update in real-time

### Technical
- Added `internal/supabase/client.go` - Supabase API client
- Added `internal/config/config.go` - Configuration loader
- Updated `main.go` - Integrated Supabase authentication

---

## [v0.1.0] - 2025-11-04

### Added
- ✅ **Initial Prototype** - Working Fyne application
- ✅ **Login Window** - Email/password input fields
- ✅ **Device List** - Mock device list with 5 devices
- ✅ **Tab Navigation** - Login, Devices, Settings tabs
- ✅ **Build Scripts** - build.bat and run.bat
- ✅ **Documentation** - README.md and QUICKSTART.md

### Features
- Native Windows UI
- Tab-based interface
- Status indicators (mock)
- Connect buttons (mock)

---

## Roadmap

### v0.3.0 - WebRTC Viewer (Coming Soon)
- [ ] WebRTC connection setup
- [ ] Viewer window
- [ ] Display remote screen
- [ ] Connection management

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
