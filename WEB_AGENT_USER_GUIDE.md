# 🌐 Web Agent User Guide

## What is the Web Agent?

The **Web Agent** is a browser-based remote desktop solution that lets you share your screen **without installing any software**. Perfect for:

- 🔒 **Locked-down computers** where you can't install executables
- 💼 **Work computers** with restricted permissions
- 🚀 **Quick demos** or presentations
- 🆘 **Emergency access** when you need it now

---

## ✅ Features

### View-Only Mode (No Installation)
- ✅ Screen sharing via browser
- ✅ High-quality video streaming (up to 1080p @ 30fps)
- ✅ WebRTC P2P connection
- ✅ PIN-based session approval
- ✅ Works on Chrome, Edge, Firefox

### Future: Remote Control Mode
- ⏳ Requires browser extension (Phase 2)
- ⏳ Full mouse & keyboard control
- ⏳ 5KB native helper

---

## 🚀 Quick Start

### Step 1: Access the Web Agent

**URL:** `https://stangtennis.github.io/Remote/agent.html`

Open this in your browser (Chrome, Edge, or Firefox recommended).

---

### Step 2: Login

1. Enter your email and password
2. Click **Login**

**⚠️ Important:** Your account must be approved by an administrator before you can use the web agent. If you see "pending approval", contact your admin.

---

### Step 3: Start Screen Sharing

1. Click **🎥 Start Screen Sharing**
2. Browser will ask for permission:
   ```
   "agent.html wants to share your screen"
   ```
3. Choose what to share:
   - **Entire Screen** (recommended)
   - **Window** (specific application)
   - **Browser Tab** (just one tab)
4. Click **Share**

**✅ Your device is now online and visible in the dashboard!**

---

### Step 4: Accept Connection Request

When someone wants to view your screen:

1. You'll see: **"🔔 Connection Request"**
2. A **6-digit PIN** prompt will appear
3. The person on the dashboard will tell you the PIN
4. Enter the PIN
5. Click **✅ Accept Connection**

**🟢 Your screen is now being shared!**

---

### Step 5: End Session

To stop sharing:

- Click **🛑 End Session** (while connected)
- Or click **🛑 Stop Sharing** (to stop completely)
- Or close the browser tab

---

## 📋 Common Scenarios

### Scenario 1: Monitor Your Work Computer

**Goal:** View your work PC from home (view-only)

**Steps:**
1. At work: Open `agent.html` in browser
2. Login with your account
3. Start screen sharing
4. Go home
5. Open dashboard, see your work PC
6. Click "Connect", enter PIN on work PC
7. ✅ View your work PC screen from home!

---

### Scenario 2: Remote Support

**Goal:** Help someone by viewing their screen

**Steps:**
1. Send them: `https://stangtennis.github.io/Remote/agent.html`
2. They login and start sharing
3. You open dashboard, see their device
4. Click "Connect"
5. Tell them the PIN
6. They enter PIN
7. ✅ You can now see their screen and guide them!

---

### Scenario 3: Presentation Mode

**Goal:** Share your screen in a meeting

**Steps:**
1. Open `agent.html`
2. Start screen sharing
3. Share dashboard link with viewers
4. They connect and see your screen
5. ✅ No screen-share apps needed!

---

## 🔍 Troubleshooting

### "Screen sharing permission denied"

**Cause:** You clicked "Cancel" on the permission dialog

**Solution:** Click "Start Screen Sharing" again and click "Share"

---

### "Account pending approval"

**Cause:** Your account hasn't been approved yet

**Solution:** Contact administrator (hansemand@gmail.com)

---

### "Device not showing in dashboard"

**Cause:** Not logged in or device registration failed

**Solution:**
1. Check you're logged in (see your email in device info)
2. Refresh the page
3. Try logging out and back in

---

### "Connection failed"

**Possible causes:**
- Network issues
- Firewall blocking WebRTC
- Browser not supported

**Solution:**
1. Check internet connection
2. Try different browser (Chrome recommended)
3. Check firewall settings

---

### "Session disconnected"

**Cause:** Tab was closed, network lost, or browser went to sleep

**Solution:**
- Keep the tab open and active
- Don't let computer sleep
- Refresh page if needed

---

## ⚠️ Important Notes

### Keep Tab Open
❗ The web agent only works while the browser tab is open. Closing the tab stops sharing.

**Tip:** Open in a separate window and minimize it

---

### Permission Each Time
❗ Browser asks for permission every time you start sharing (security feature)

**Why?** Prevents malicious websites from secretly recording your screen

---

### What Viewers See
❗ Viewers can see **everything** on your screen

**Including:**
- All windows and applications
- Notifications
- Passwords if you type them
- Personal files if you open them

**Tip:** Close sensitive applications before sharing

---

### Network Usage
❗ Streaming video uses bandwidth (~1-5 Mbps)

**On mobile hotspot?** Quality may be lower due to bandwidth

---

## 🔒 Privacy & Security

### Data Protection
- ✅ WebRTC P2P encryption (DTLS-SRTP)
- ✅ PIN-based session approval
- ✅ You control when to accept connections
- ✅ Can end session anytime

### What's Transmitted
- ✅ **Screen video** - What you choose to share
- ❌ **NOT saved** - No recording by system
- ❌ **NOT stored** - Streams directly to viewer

### Who Can Connect
- ✅ **Only approved users** - Admin controls access
- ✅ **Only with your PIN** - You approve each session
- ✅ **Only when you allow** - You must click "Accept"

---

## 📱 Browser Compatibility

| Browser | Screen Capture | WebRTC | Status |
|---------|---------------|--------|--------|
| **Chrome 72+** | ✅ Full | ✅ Full | ✅ **Recommended** |
| **Edge 79+** | ✅ Full | ✅ Full | ✅ **Recommended** |
| **Firefox 66+** | ✅ Full | ✅ Full | ✅ Supported |
| **Safari 13+** | ⚠️ Limited | ✅ Full | ⚠️ Partial |
| **Mobile** | ❌ No | ✅ Full | ❌ Desktop only |

---

## 🆚 Web Agent vs Native Agent

| Feature | Web Agent | Windows EXE |
|---------|-----------|-------------|
| **Installation** | None | Required |
| **Screen Capture** | ✅ Full | ✅ Full |
| **Remote Control** | ❌ View only | ✅ Full control |
| **Locked Computers** | ✅ Works | ❌ Blocked |
| **Background** | ❌ Tab only | ✅ Service |
| **Auto-Start** | ❌ Manual | ✅ Startup |
| **Cross-Platform** | ✅ Any OS | ❌ Windows |

**When to use which:**
- **Web Agent:** Locked computer, quick access, demo mode
- **Native Agent:** Personal computer, 24/7 access, full control

---

## 🎯 Tips & Best Practices

### For Best Quality
✅ Use wired internet (not WiFi)
✅ Close unnecessary applications
✅ Use Chrome or Edge browser
✅ Share "Entire Screen" not window

### For Privacy
✅ Close sensitive windows before sharing
✅ Disable notifications temporarily
✅ Don't share if you'll type passwords
✅ Always end session when done

### For Reliability
✅ Keep tab in foreground (don't minimize)
✅ Don't let computer sleep
✅ Use AC power (not battery)
✅ Stable internet connection

---

## 🆘 Getting Help

### Check Logs
Open browser console (F12) to see detailed logs:
- ✅ Green checkmarks = Success
- ❌ Red errors = Problems
- 📤 📥 = Network activity

### Common Log Messages
```
✅ Logged in as: your@email.com
✅ Device registered: [device-id]
📹 Requesting screen capture...
✅ Screen capture started
📞 Incoming connection request
✅ PIN accepted, starting session...
🔗 Starting WebRTC connection...
✅ WebRTC connection initiated
```

### Support Channels
- **GitHub Issues:** https://github.com/stangtennis/Remote/issues
- **Email:** hansemand@gmail.com
- **Documentation:** Check README.md

---

## 📚 Technical Details

### System Requirements
- **OS:** Windows, macOS, Linux
- **Browser:** Chrome 72+, Edge 79+, Firefox 66+
- **Internet:** 2+ Mbps for good quality
- **Account:** Approved user

### Network Ports
- **STUN:** UDP 3478, 19302
- **TURN:** TCP/UDP 443, 3478 (if P2P fails)
- **WebRTC:** Random UDP ports (49152-65535)

### Video Quality
- **Resolution:** Up to 1920x1080
- **Frame Rate:** 15-30 FPS (adaptive)
- **Bandwidth:** 1-5 Mbps (depends on content)
- **Latency:** 100-500ms (depends on network)

---

## 🚀 Future Enhancements (Phase 2)

### Remote Control Mode (Coming Soon)
When available, you'll be able to:
- 🎮 Control computer remotely (mouse + keyboard)
- 📦 Install tiny helper (5KB) + browser extension
- ✅ Still much lighter than full agent

Stay tuned for updates!

---

## ✅ Conclusion

The Web Agent is perfect for:
- ✅ Viewing screens on locked computers
- ✅ Quick demonstrations
- ✅ Remote assistance (guide mode)
- ✅ Emergency access

It's not meant for:
- ❌ 24/7 unattended monitoring (use native agent)
- ❌ Full remote control (Phase 2 coming)
- ❌ Background operation (requires open tab)

**Start using it now:**  
https://stangtennis.github.io/Remote/agent.html

---

**Happy Screen Sharing!** 🌐✨
