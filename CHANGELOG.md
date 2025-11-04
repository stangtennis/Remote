# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

## [0.2.0] - 2025-11-05 - Device Assignment System

### Added - TeamViewer-Style Device Management
- **Device Assignment System**: Complete admin-managed device workflow
  - Agents auto-register without login (anonymous registration)
  - Admins assign devices to users via admin panel
  - Users see only devices assigned to them
  - Professional IT management workflow

### Database
- **New Table**: `device_assignments` - Tracks device-to-user assignments
- **New Columns**: 
  - `role` in `user_approvals` (user/admin)
  - `status` in `remote_devices` (online/offline)
  - `approved`, `assigned_by`, `assigned_at` in `remote_devices`
- **New Functions**:
  - `get_user_devices(user_id)` - Get devices assigned to user
  - `get_unassigned_devices()` - List unassigned devices (admin only)
  - `assign_device(device_id, user_id)` - Assign device to user
  - `revoke_device_assignment(device_id, user_id)` - Revoke assignment
- **RLS Policies**: Anonymous device registration, admin-only management

### Controller Application
- **Assignment-Based Access**: Users see only assigned devices
- **Updated API**: Uses `get_user_devices()` RPC function
- **GitHub Actions**: Automated Windows EXE builds on push
- **Artifact Downloads**: 30-day retention for builds
- **Release Tags**: `controller-v*` creates GitHub releases

### Agent Application
- **Anonymous Registration**: No login required on devices
- **Persistent Device ID**: Saved to Windows Registry + file
- **Hardware-Based ID**: Stable across restarts
- **Auto-Registration**: Registers on startup
- **New Files**:
  - `agent/internal/device/id.go` - Device ID management
  - `agent/internal/device/registration.go` - Anonymous registration
- **Updated Heartbeat**: Uses new anonymous update system

### Admin Panel
- **Tabbed Interface**: User Approvals + Device Management
- **Device Management Tab**:
  - View all devices with real-time status
  - Filter: All / Unassigned / Assigned
  - Device stats: Total, Unassigned, Online
  - Device cards with status badges (🟢 Online / 🔴 Offline)
  - Assignment modal with user selection
  - Approve devices during assignment
  - Revoke assignments
- **Visual Indicators**:
  - Orange border: Unassigned devices
  - Green border: Assigned devices
  - Status badges with color coding

### Documentation
- **DEVICE_ASSIGNMENT_DESIGN.md**: Complete system design
- **Migration Guide**: Database migration instructions
- **Workflow Documentation**: Step-by-step usage guide

### Changed
- **Controller**: Now queries assigned devices only
- **Agent**: Removed login requirement
- **Admin Panel**: Expanded with device management
- **Database**: Owner-based to assignment-based model

### Benefits
- ✅ Deploy agents via GPO/scripts
- ✅ No user interaction needed
- ✅ Central device management
- ✅ Easy assignment/reassignment
- ✅ Professional IT workflow

## [Previous Releases]

### Added
- **Web Agent**: Browser-based agent using getDisplayMedia() - no installation required!
- **Browser Extension**: Chrome extension for remote control capabilities
- **Native Messaging Host**: Tiny native helper (Go) for mouse/keyboard control
- **Electron Agent**: Cross-platform desktop agent prototype
- **Extension Installation Scripts**: Automated setup for Windows/Mac/Linux

### Changed
- **Native Host Manifest**: Updated with actual extension ID
- **Web Agent UI**: Modern, responsive design with status indicators
- **Documentation**: Added comprehensive implementation plans for web agent

## [1.1.7] - 2025-01-09

### Added
- **Automatic Session Cleanup**: PostgreSQL cron job runs every 5 minutes to clean stale sessions
- **Mouse/Keyboard Control**: Re-enabled with robotgo (requires CGO build)
- **Build Script**: `build.bat` for easy CGO-enabled compilation
- **Deployment Guide**: Comprehensive `DEPLOYMENT.md` with setup instructions
- **Optimization Guide**: `OPTIMIZATION.md` for H.264/VP8 video encoding upgrade path
- **Session Cleanup Edge Function**: Manual trigger for emergency cleanup
- **Session Cleanup Migration**: SQL migration with pg_cron integration

### Changed
- **Input Handling**: Uncommented and enabled mouse/keyboard control in `peer.go`
- **Screen Resolution**: Now passes dimensions to input controllers for coordinate mapping
- **Documentation**: Updated README and plan.md with current status

### Fixed
- **Session Deadlocks**: Automatic cleanup prevents old sessions from clogging signaling
- **Input Control**: Previously disabled, now functional (with CGO)

## [2025-10-02] - WebRTC Connection Working

### Added
- **TURN Relay**: Twilio TURN servers integrated and working
- **External Access**: Confirmed working from outside local network
- **Error Handling**: HTTP status code checking in signaling functions
- **Debugging**: Enhanced logging for connection states and ICE candidates

### Changed
- **Session Management**: Improved cleanup of stuck sessions
- **WebRTC Config**: TURN servers properly configured with credentials

### Fixed
- **Connection Failures**: Agent can now connect reliably across networks
- **Signaling Issues**: Database write errors now properly logged
- **Session Reuse**: Stale sessions no longer cause connection problems

## [2025-10-01] - Initial Working Implementation

### Added
- **Supabase Backend**: Database schema, Edge Functions, RLS policies
- **Dashboard**: GitHub Pages deployment with authentication
- **Agent**: Go + Pion WebRTC implementation
- **Screen Capture**: JPEG frame streaming over data channel
- **Device Registration**: Hardware-based device ID
- **Authentication**: Supabase Auth with email/password

### Known Limitations
- Input control disabled (pending CGO setup)
- Manual session cleanup required
- JPEG-only video (10 FPS)
- No code signing

---

## Versioning Strategy

This project uses Semantic Versioning (SemVer):
- **MAJOR**: Breaking changes to API or database schema
- **MINOR**: New features, backwards compatible
- **PATCH**: Bug fixes, backwards compatible

Current versions:
- **Windows Agent**: v1.1.7 (Production)
- **Web Agent**: v1.0.0-beta (Unreleased)
- **Browser Extension**: v1.0.0 (Unreleased)
- **Dashboard**: v1.1.7 (Production)

Next release: **v1.2.0** (Web Agent + Extension)
