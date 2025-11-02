# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

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
