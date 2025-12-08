# v2.4.0 - Mouse Drift Fix + Multi-Connection

## 🐛 Critical Bug Fixes

### Mouse Movement Drift - FIXED
The mouse drift issue has been completely resolved with multiple improvements:

- **60 FPS Throttling** - Mouse events limited to 60/sec (16ms interval) to prevent event flooding
- **Precision Rounding** - Changed from int truncation to math.Round for accurate coordinate conversion
- **Float64 Calculations** - Higher precision in coordinate transformations to eliminate cumulative errors
- **Bounds Clamping** - Screen boundary checking prevents out-of-bounds coordinates
- **Division by Zero Protection** - Added safety checks for edge cases

**Result**: Mouse cursor now tracks perfectly without drift, even during extended sessions.

---

## 🎯 Major Features

### Multi-Connection Support (v2.4.0)
- Controller can now manage multiple agent connections simultaneously
- Session switching UI for easy navigation between devices
- Per-session routing and state management
- Concurrent WebRTC connections with independent video/input streams

### Bidirectional Clipboard Sync
- Full two-way clipboard synchronization (agent ↔ controller)
- Text and image support
- Automatic conflict resolution
- Low latency sync

---

## 📦 Downloads

### Remote Agent (Client)
For the computer you want to control:
- **remote-agent.exe** - Windows executable (optimized build)

### Remote Controller
For the computer you control from:
- **remote-controller.exe** - Windows executable (optimized build)

---

## 🚀 Quick Start

### Agent Setup
1. Download **remote-agent.exe**
2. Run **install-service.bat** (as Admin)
3. Agent will register and wait for approval

### Controller Setup
1. Download **remote-controller.exe**
2. Login with your credentials
3. Go to "Approve Devices" tab
4. Approve your agent
5. Connect from "My Devices" tab

---

## 🔧 Technical Details

**Files Modified:**
- controller/internal/viewer/interactive_canvas.go (throttling)
- controller/internal/viewer/connection.go (precision)
- agent/internal/input/mouse.go (rounding & clamping)

**Infrastructure:**
- Supabase: https://supabase.hawkeye123.dk
- Dashboard: https://stangtennis.github.io/Remote

---

## 📊 Version History
- v2.4.0 - Mouse drift fix + Multi-connection
- v2.3.0 - Maximum quality mode (60 FPS, 4K support)
- v2.2.0 - Clipboard sync (agent → controller)
- v2.1.0 - File transfer + auto-reconnection
- v2.0.0 - Core remote desktop functionality
