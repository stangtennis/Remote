# Console Mode - See What Your Agent Is Doing

The Remote Desktop Agent has **two versions** you can run:

---

## 🖥️ **Normal Mode** (System Tray Only)

**File:** `remote-agent.exe`

**Build:** `build.bat`

**What it does:**
- ✅ Runs in system tray (no console window)
- ✅ Professional, clean appearance
- ✅ Perfect for daily use
- ✅ Starts minimized to tray

**When to use:**
- Normal daily usage
- When you want it running in the background
- After you've verified everything works

**How to run:**
- Double-click `remote-agent.exe`
- Icon appears in system tray
- Right-click icon for menu

---

## 🪟 **Debug/Console Mode** (With Live Logs)

**File:** `remote-agent-debug.exe`

**Build:** `build-debug.bat`

**What it does:**
- ✅ Shows console window with live logs
- ✅ See exactly what's happening in real-time
- ✅ Perfect for troubleshooting
- ✅ Watch connections, errors, and activity

**When to use:**
- First time setup
- Troubleshooting issues
- Watching what the agent is doing
- Debugging connection problems

**How to run:**

### Option 1: Quick Start (Recommended)
Double-click **`run-with-console.bat`**
- Automatically builds debug version if needed
- Opens console with live logs
- Press Ctrl+C to stop

### Option 2: Build Then Run
1. Run `build-debug.bat` (builds `remote-agent-debug.exe`)
2. Double-click `remote-agent-debug.exe`
3. Console window shows live activity

---

## 📊 **What You'll See in Console Mode**

```
========================================
🖥️  Remote Desktop Agent Starting...
📦 Version: v1.1.6
📝 Log file: C:\path\to\agent.log
========================================
🔧 Running in interactive mode
📱 Registering device...
✅ Device registered: Dennis-bærbar
🔄 Starting heartbeat (every 30s)...
❤️ Heartbeat sent - device is online
🔍 Polling for sessions (every 2s)...
🎥 Starting screen streaming at 15 FPS...
📊 Sent 100 frames (latest: 120 KB, 0 errors, 0 dropped)
```

You can see everything that's happening in real-time!

---

## 🎯 **Quick Comparison**

| Feature | Normal Mode | Debug/Console Mode |
|---------|-------------|-------------------|
| **File** | `remote-agent.exe` | `remote-agent-debug.exe` |
| **Console** | ❌ Hidden | ✅ Visible |
| **Tray Icon** | ✅ Yes | ✅ Yes |
| **Live Logs** | ❌ No (check log file) | ✅ Yes (in console) |
| **Use For** | Daily use | Debugging/Setup |
| **Double-Click** | Starts hidden | Shows console |

---

## 💡 **Pro Tips**

### See Logs Anytime
Even in normal mode, logs are saved to `agent.log`. View them with:
- Double-click `view-logs.bat` (live tail)
- Open `agent.log` in any text editor

### Switch Between Modes
You can use both versions! They're the same agent, just different display:
- **Normal mode** for daily use
- **Debug mode** when you want to see what's happening

### Stop the Agent
- **Console mode:** Press `Ctrl+C` in the console window
- **Normal mode:** Right-click tray icon → Exit

---

## 🚀 **Recommended Workflow**

### First Time Setup
1. **Use Console Mode** (`run-with-console.bat`)
2. Watch it register your device
3. See the heartbeat working
4. Verify connections

### Daily Use
1. **Use Normal Mode** (double-click `remote-agent.exe`)
2. Runs quietly in system tray
3. Check logs if needed with `view-logs.bat`

### Troubleshooting
1. **Switch to Console Mode** (`run-with-console.bat`)
2. Watch real-time logs
3. See exact error messages
4. Fix the issue
5. Switch back to Normal Mode

---

**Now you can see exactly what your agent is doing!** 🎉
