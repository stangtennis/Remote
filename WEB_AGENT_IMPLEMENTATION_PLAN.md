# 🌐 Web-Based Agent Implementation Plan

## Executive Summary

Create a browser-based screen sharing agent that works **without installing anything** - perfect for locked-down computers where you can't run executables.

**Feasibility:** ✅ **HIGH** (Screen Share) / ⚠️ **MEDIUM** (Remote Control)  
**Timeline:** 2-3 weeks (view-only) / 4-5 weeks (with control)  
**Priority:** **HIGH** - Solves major use case (work computers, restricted systems)

---

## 🎯 Two Approaches

### **Option A: View-Only Mode (Recommended - Phase 1)**
✅ **Fully feasible with just browser JavaScript**  
✅ No installation required  
✅ Works on any computer with a browser  
❌ Cannot control the remote computer (view only)

**Use Cases:**
- Monitor a locked-down work computer
- Share your screen in presentations
- Get help by letting someone see your screen
- Watch activity on another machine

### **Option B: Full Control Mode (Advanced - Phase 2)**
⚠️ **Requires browser extension**  
⚠️ User must install extension  
✅ Can control remote computer (mouse + keyboard)  
⚠️ Limited by browser security policies

**Use Cases:**
- Remote support on work computers
- Control locked-down systems
- Bypass executable restrictions

---

## 🚀 Phase 1: View-Only Web Agent (Recommended Start)

### Architecture

```
┌────────────────────────────────────┐
│  Remote Computer (Source)         │
│  ┌──────────────────────────────┐ │
│  │  Web Browser (Chrome/Edge)   │ │
│  │  ┌────────────────────────┐  │ │
│  │  │  Web Agent Page        │  │ │
│  │  │  - getDisplayMedia()   │  │ │
│  │  │  - WebRTC Peer         │  │ │
│  │  │  - Supabase Client     │  │ │
│  │  └────────────────────────┘  │ │
│  └──────────────────────────────┘ │
└─────────────────┬──────────────────┘
                  │
                  │ WebRTC P2P
                  │
┌─────────────────▼──────────────────┐
│  Dashboard (Viewer)                │
│  - Same existing dashboard         │
│  - No changes needed!              │
└────────────────────────────────────┘
```

---

## 📋 Technical Implementation

### 1. Web Agent Page (`/agent.html`)

**URL:** `https://stangtennis.github.io/Remote/agent.html`

**Features:**
- Login with email (same as dashboard)
- Register as device
- Start screen sharing (one click)
- Show connection status
- PIN entry for sessions
- Automatic reconnection

**Code Structure:**
```html
<!DOCTYPE html>
<html>
<head>
  <title>Remote Agent - Web</title>
  <link rel="stylesheet" href="css/styles.css">
</head>
<body>
  <div class="agent-container">
    <h1>🌐 Web Agent</h1>
    
    <!-- Login Section -->
    <div id="loginSection">
      <input type="email" id="email" placeholder="Your email">
      <input type="password" id="password" placeholder="Password">
      <button onclick="login()">Login</button>
    </div>
    
    <!-- Device Section -->
    <div id="deviceSection" style="display:none">
      <h2>Device: <span id="deviceName"></span></h2>
      <p>Status: <span id="status">Offline</span></p>
      <button id="startBtn" onclick="startSharing()">Start Screen Share</button>
      <button id="stopBtn" onclick="stopSharing()" style="display:none">Stop</button>
    </div>
    
    <!-- Session Section -->
    <div id="sessionSection" style="display:none">
      <h3>🔔 Connection Request</h3>
      <p>Enter PIN to accept:</p>
      <input type="text" id="pinInput" placeholder="6-digit PIN">
      <button onclick="acceptSession()">Accept</button>
    </div>
    
    <!-- Preview -->
    <video id="preview" autoplay muted style="width:400px"></video>
  </div>
  
  <script type="module" src="js/web-agent.js"></script>
</body>
</html>
```

---

### 2. Core JavaScript (`web-agent.js`)

```javascript
import { supabase } from './supabase.js';

let mediaStream = null;
let peerConnection = null;
let deviceId = null;
let currentSession = null;

// Login and register device
async function login() {
  const email = document.getElementById('email').value;
  const password = document.getElementById('password').value;
  
  const { data, error } = await supabase.auth.signInWithPassword({
    email, password
  });
  
  if (error) {
    alert('Login failed: ' + error.message);
    return;
  }
  
  // Register device
  await registerDevice();
  
  // Show device section
  document.getElementById('loginSection').style.display = 'none';
  document.getElementById('deviceSection').style.display = 'block';
  
  // Start polling for sessions
  pollForSessions();
}

async function registerDevice() {
  const deviceName = `Web - ${navigator.platform}`;
  
  const { data, error } = await supabase
    .from('remote_devices')
    .insert({
      device_name: deviceName,
      platform: 'web',
      browser: navigator.userAgent,
      owner_id: (await supabase.auth.getUser()).data.user.id
    })
    .select()
    .single();
  
  if (error) {
    console.error('Device registration failed:', error);
    return;
  }
  
  deviceId = data.device_id;
  document.getElementById('deviceName').textContent = deviceName;
  document.getElementById('status').textContent = 'Online';
  
  // Send heartbeat
  startHeartbeat();
}

async function startSharing() {
  try {
    // Request screen capture
    mediaStream = await navigator.mediaDevices.getDisplayMedia({
      video: {
        cursor: 'always', // Show cursor in capture
        displaySurface: 'monitor' // Prefer full screen
      },
      audio: false
    });
    
    // Show preview
    document.getElementById('preview').srcObject = mediaStream;
    
    // Update UI
    document.getElementById('startBtn').style.display = 'none';
    document.getElementById('stopBtn').style.display = 'block';
    
    console.log('✅ Screen sharing started');
    
    // Listen for track ending (user stops sharing)
    mediaStream.getVideoTracks()[0].addEventListener('ended', () => {
      console.log('User stopped sharing');
      stopSharing();
    });
    
  } catch (error) {
    console.error('Failed to start screen sharing:', error);
    alert('Screen sharing permission denied');
  }
}

function stopSharing() {
  if (mediaStream) {
    mediaStream.getTracks().forEach(track => track.stop());
    mediaStream = null;
  }
  
  if (peerConnection) {
    peerConnection.close();
    peerConnection = null;
  }
  
  document.getElementById('preview').srcObject = null;
  document.getElementById('startBtn').style.display = 'block';
  document.getElementById('stopBtn').style.display = 'none';
  
  console.log('🛑 Screen sharing stopped');
}

// Poll for incoming sessions
async function pollForSessions() {
  setInterval(async () => {
    if (!deviceId || currentSession) return;
    
    const { data, error } = await supabase
      .from('remote_sessions')
      .select('*')
      .eq('device_id', deviceId)
      .eq('status', 'pending')
      .order('created_at', { ascending: false })
      .limit(1);
    
    if (data && data.length > 0) {
      currentSession = data[0];
      showPinPrompt();
    }
  }, 2000);
}

function showPinPrompt() {
  document.getElementById('sessionSection').style.display = 'block';
  document.getElementById('pinInput').focus();
}

async function acceptSession() {
  const pin = document.getElementById('pinInput').value;
  
  if (pin !== currentSession.pin) {
    alert('❌ Invalid PIN');
    return;
  }
  
  // Update session status
  await supabase
    .from('remote_sessions')
    .update({ status: 'active', started_at: new Date().toISOString() })
    .eq('id', currentSession.id);
  
  // Hide PIN prompt
  document.getElementById('sessionSection').style.display = 'none';
  
  // Start WebRTC connection
  await startWebRTC();
}

async function startWebRTC() {
  // Create peer connection
  const config = {
    iceServers: [
      { urls: 'stun:stun.l.google.com:19302' },
      // Add TURN server here
    ]
  };
  
  peerConnection = new RTCPeerConnection(config);
  
  // Add screen stream
  if (!mediaStream) {
    await startSharing();
  }
  
  mediaStream.getTracks().forEach(track => {
    peerConnection.addTrack(track, mediaStream);
  });
  
  // Handle ICE candidates
  peerConnection.onicecandidate = async (event) => {
    if (event.candidate) {
      await supabase
        .from('session_signaling')
        .insert({
          session_id: currentSession.id,
          type: 'ice_candidate',
          data: JSON.stringify(event.candidate),
          from_agent: true
        });
    }
  };
  
  // Create offer
  const offer = await peerConnection.createOffer();
  await peerConnection.setLocalDescription(offer);
  
  // Send offer to dashboard
  await supabase
    .from('session_signaling')
    .insert({
      session_id: currentSession.id,
      type: 'offer',
      data: JSON.stringify(offer),
      from_agent: true
    });
  
  console.log('✅ WebRTC offer sent');
  
  // Listen for answer and ICE candidates
  listenForSignaling();
}

async function listenForSignaling() {
  // Subscribe to signaling messages
  const channel = supabase
    .channel(`session_${currentSession.id}`)
    .on('postgres_changes', {
      event: 'INSERT',
      schema: 'public',
      table: 'session_signaling',
      filter: `session_id=eq.${currentSession.id}`
    }, async (payload) => {
      const msg = payload.new;
      
      if (msg.from_agent) return; // Skip our own messages
      
      const data = JSON.parse(msg.data);
      
      if (msg.type === 'answer') {
        await peerConnection.setRemoteDescription(new RTCSessionDescription(data));
        console.log('✅ WebRTC answer received');
      } else if (msg.type === 'ice_candidate') {
        await peerConnection.addIceCandidate(new RTCIceCandidate(data));
      }
    })
    .subscribe();
}

// Heartbeat
function startHeartbeat() {
  setInterval(async () => {
    if (!deviceId) return;
    
    await supabase
      .from('remote_devices')
      .update({ last_heartbeat: new Date().toISOString() })
      .eq('device_id', deviceId);
  }, 30000); // Every 30 seconds
}

// Export functions
window.login = login;
window.startSharing = startSharing;
window.stopSharing = stopSharing;
window.acceptSession = acceptSession;
```

---

## ✅ What Works (View-Only Mode)

### Browser Support
- ✅ **Chrome/Edge**: Full support (Chromium)
- ✅ **Firefox**: Full support
- ✅ **Safari**: Partial support (iOS restrictions)

### Capabilities
- ✅ Screen capture (full screen, window, or tab)
- ✅ High-quality video streaming (up to 4K)
- ✅ WebRTC P2P connection
- ✅ Same dashboard (no changes!)
- ✅ PIN-based session approval
- ✅ Works on locked-down computers
- ✅ No installation required
- ✅ Cross-platform (Windows, Mac, Linux)

### Limitations
- ❌ **Cannot control remote computer** (view only)
- ❌ No mouse/keyboard input
- ❌ User must grant permission each time
- ❌ User must keep browser tab open

---

## 🎮 Phase 2: Adding Remote Control (Optional)

### Problem: Browser Security

**Browsers cannot inject input for security reasons!**

JavaScript in a web page **cannot**:
- ❌ Send mouse clicks to other apps
- ❌ Send keyboard input to other apps
- ❌ Simulate system-level events

**Why?** This would be a major security vulnerability (malicious websites could control your computer!)

### Solutions

#### **Option 1: Browser Extension (Recommended)**

Create a Chrome/Edge extension that:
- ✅ Has elevated permissions
- ✅ Can inject input via Chrome Automation API
- ✅ Works alongside web agent page

**Pros:**
- More control than plain web page
- Still easier than EXE installation
- Cross-platform

**Cons:**
- User must install extension
- Chrome Web Store approval required
- Limited to browser windows (can't control desktop apps)

**Extension Manifest:**
```json
{
  "manifest_version": 3,
  "name": "Remote Agent Extension",
  "version": "1.0.0",
  "permissions": [
    "tabs",
    "scripting",
    "debugger"
  ],
  "background": {
    "service_worker": "background.js"
  },
  "content_scripts": [{
    "matches": ["<all_urls>"],
    "js": ["content.js"]
  }]
}
```

**Input Injection (Limited):**
```javascript
// Can inject events into web pages only
chrome.tabs.executeScript(tabId, {
  code: `
    document.dispatchEvent(new MouseEvent('click', {
      clientX: ${x},
      clientY: ${y}
    }));
  `
});
```

**Still Limited:**
- ❌ Can only control browser tabs
- ❌ Cannot control desktop apps
- ❌ Cannot control OS

#### **Option 2: WebDriver/Selenium**

Use browser automation tools:
- ✅ Full control over browser
- ❌ Requires local WebDriver installation (defeats purpose)
- ❌ Not suitable for web-only solution

#### **Option 3: View-Only + Native Helper**

Hybrid approach:
- Web page for screen sharing (no install)
- Optional tiny native helper for input (if needed)

**Best of both worlds:**
- ✅ Most users: View-only mode (no install)
- ✅ Advanced users: Install small helper for control

---

## 📊 Comparison: Web Agent vs Native Agent

| Feature | Web Agent | Windows EXE | Android App |
|---------|-----------|-------------|-------------|
| **Installation** | None | Required | Required |
| **Screen Capture** | ✅ Full | ✅ Full | ✅ Full |
| **Remote Control** | ❌ No* | ✅ Full | ✅ Full |
| **Permissions** | Low | Medium | High |
| **Cross-Platform** | ✅ Any OS | ❌ Windows | ❌ Android |
| **Locked Computers** | ✅ Works | ❌ Blocked | ❌ Blocked |
| **Auto-Start** | ❌ No | ✅ Yes | ✅ Yes |
| **Background** | ❌ Tab only | ✅ Yes | ✅ Yes |

*Requires browser extension for limited control

---

## 🎯 Recommended Approach

### **Phase 1: View-Only Web Agent (2-3 weeks)**

**Deliver immediately:**
1. Create `/agent.html` page
2. Implement getDisplayMedia screen capture
3. WebRTC streaming (reuse existing code)
4. Device registration (reuse existing backend)
5. PIN-based session approval

**Value:**
- ✅ Solves 80% of use cases
- ✅ No installation friction
- ✅ Works on locked computers
- ✅ Perfect for monitoring/support

### **Phase 2: Browser Extension (Optional - 2-3 weeks)**

**Add later if needed:**
1. Create Chrome extension
2. Add tab control capability
3. Publish to Chrome Web Store
4. Document installation

**Limited value:**
- ⚠️ Only controls browser tabs
- ⚠️ Doesn't solve locked computer control
- ⚠️ Native agent still better for full control

---

## 💡 Unique Use Cases

### **What Web Agent Solves:**

✅ **Work Computer Monitoring**
- Your work computer is locked down (no admin)
- You can't install EXE files
- But you CAN open a web page!
- Now you can monitor your work PC from home

✅ **Presentation/Demo Mode**
- Share your screen during presentations
- No software to install
- Just open agent.html
- Share the link

✅ **Emergency Access**
- Need to access a computer RIGHT NOW
- Don't have agent installed
- Open browser, load agent page
- Instant screen sharing

✅ **Cross-Platform Support**
- Works on Windows, Mac, Linux
- Same web page for all platforms
- No platform-specific builds

---

## 🗓️ Implementation Timeline

### Week 1: Core Setup
- [ ] Create `/agent.html` page
- [ ] Implement login/auth
- [ ] Device registration
- [ ] Basic UI styling

### Week 2: Screen Capture
- [ ] Implement getDisplayMedia
- [ ] Add video preview
- [ ] Handle permissions
- [ ] User stop handling

### Week 3: WebRTC Connection
- [ ] Create peer connection
- [ ] Send video stream
- [ ] Signaling (reuse existing)
- [ ] Session management
- [ ] PIN prompt

### Week 4: Polish & Testing
- [ ] Error handling
- [ ] Reconnection logic
- [ ] UI improvements
- [ ] Cross-browser testing
- [ ] Documentation

---

## 🚀 Quick Start (After Implementation)

### For Users:

1. **Open web agent:**
   ```
   https://stangtennis.github.io/Remote/agent.html
   ```

2. **Login** with your approved email

3. **Click "Start Screen Share"**
   - Browser shows permission dialog
   - Select screen/window/tab
   - Click "Share"

4. **Device appears in dashboard** (Online)

5. **Someone connects:**
   - Enter PIN on web agent
   - Screen streaming starts!

6. **To stop:**
   - Click "Stop" button
   - Or close browser tab

---

## ⚠️ Limitations & Workarounds

### Limitation 1: No Auto-Start
**Problem:** Must manually open page  
**Workaround:** Bookmark it, or set as homepage

### Limitation 2: Tab Must Stay Open
**Problem:** Closing tab stops sharing  
**Workaround:** Open in separate window, minimize

### Limitation 3: Permission Each Time
**Problem:** Browser asks for permission every time  
**Workaround:** None - this is browser security

### Limitation 4: No Background Operation
**Problem:** Browser tab must be active  
**Workaround:** Use native agent for always-on monitoring

### Limitation 5: No Remote Control
**Problem:** View-only mode  
**Workaround:** 
- Accept this for Phase 1
- Add browser extension for Phase 2 (limited)
- Use native agent for full control

---

## 📋 Browser Compatibility

| Browser | Screen Capture | WebRTC | Status |
|---------|---------------|--------|--------|
| **Chrome 72+** | ✅ Full | ✅ Full | ✅ **Recommended** |
| **Edge 79+** | ✅ Full | ✅ Full | ✅ **Recommended** |
| **Firefox 66+** | ✅ Full | ✅ Full | ✅ Supported |
| **Safari 13+** | ⚠️ Limited | ✅ Full | ⚠️ Partial |
| **Mobile** | ❌ Not supported | ✅ Full | ❌ Desktop only |

---

## 💰 Cost & Effort

### Development
- **Time:** 2-3 weeks
- **Complexity:** Low-Medium
- **Testing:** 3-5 browsers

### Infrastructure
- **Backend:** ✅ Reuse existing (Supabase)
- **Hosting:** ✅ Free (GitHub Pages)
- **Additional costs:** ❌ None

---

## ✅ Decision Matrix

### Should You Build This?

**YES if:**
- ✅ Users have locked-down computers
- ✅ You want cross-platform support
- ✅ Installation friction is a problem
- ✅ View-only monitoring is valuable

**NO if:**
- ❌ You MUST have remote control
- ❌ Background operation is required
- ❌ Native agent works fine

---

## 🎯 Success Metrics

### Functional
- ✅ Works on Chrome/Edge/Firefox
- ✅ Screen captures at 15+ FPS
- ✅ WebRTC connection establishes
- ✅ Dashboard can view stream
- ✅ PIN prompt works correctly

### Performance
- ✅ Startup time <10 seconds
- ✅ Video latency <500ms
- ✅ CPU usage <20% (browser)
- ✅ Bandwidth <5 Mbps

### UX
- ✅ Setup time <2 minutes
- ✅ Clear permission prompts
- ✅ Obvious connection status
- ✅ Easy stop mechanism

---

## 📚 Documentation Needed

- [ ] **WEB_AGENT_GUIDE.md** - User instructions
- [ ] **WEB_AGENT_FAQ.md** - Common questions
- [ ] Update **README.md** - Add web agent info
- [ ] **Browser Extension Guide** - If Phase 2

---

## 🚀 Next Steps

### Immediate
1. ✅ **Approve this plan** - Decide if valuable
2. Create prototype - Test getDisplayMedia
3. Test WebRTC in browser - Validate approach

### Short-term
1. Implement core web agent
2. Test on multiple browsers
3. Deploy to GitHub Pages

### Long-term
1. Gather user feedback
2. Consider browser extension
3. Optimize performance

---

## ✅ Conclusion

**Feasibility: ✅ VERY HIGH** (for view-only mode)

**Value: ✅ HIGH** - Solves real problem (locked computers)

**Effort: ✅ LOW-MEDIUM** - Simpler than native agents

**Recommendation: ✅ BUILD IT!**

### Why This is Valuable:

1. **No Installation** - Major advantage
2. **Cross-Platform** - One solution for all OS
3. **Locked Systems** - Works where native can't
4. **Quick Setup** - 2 minutes to start
5. **Same Backend** - Reuse everything

### Trade-offs:

- ❌ View-only (no control)
- ❌ Not background persistent
- ❌ Permission prompts

But for many use cases (monitoring work PCs, demos, emergency access), **view-only is perfect!**

---

**Start with Phase 1 (view-only), get feedback, then decide on Phase 2 (extension).**

---

**Created:** 2025-01-09  
**Version:** 1.0  
**Status:** Ready for Implementation
