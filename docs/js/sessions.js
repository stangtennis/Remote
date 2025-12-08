// Multi-Session Tab Manager
// Handles multiple simultaneous remote desktop connections

const SessionManager = {
  sessions: new Map(), // deviceId -> session object
  activeSessionId: null,
  maxSessions: 6,

  // Initialize the session manager
  init() {
    this.tabsContainer = document.getElementById('tabsContainer');
    this.tabAddBtn = document.getElementById('tabAddBtn');
    this.previewIdle = document.getElementById('previewIdle');
    
    // Tab add button scrolls to devices
    if (this.tabAddBtn) {
      this.tabAddBtn.addEventListener('click', () => {
        document.querySelector('.devices-section')?.scrollIntoView({ behavior: 'smooth' });
      });
    }

    console.log('📑 Session Manager initialized');
  },

  // Create a new session for a device
  createSession(deviceId, deviceName) {
    if (this.sessions.has(deviceId)) {
      console.log(`Session already exists for ${deviceName}, switching to it`);
      this.switchToSession(deviceId);
      return this.sessions.get(deviceId);
    }

    if (this.sessions.size >= this.maxSessions) {
      alert(`Maximum ${this.maxSessions} sessions allowed. Please close a session first.`);
      return null;
    }

    const session = {
      id: deviceId,
      deviceName: deviceName,
      status: 'connecting', // connecting, connected, disconnected
      peerConnection: null,
      dataChannel: null,
      lastFrame: null,
      frameCount: 0,
      createdAt: Date.now()
    };

    this.sessions.set(deviceId, session);
    this.createTab(session);
    this.switchToSession(deviceId);
    this.updateUI();

    console.log(`📑 Created session for ${deviceName} (${deviceId})`);
    return session;
  },

  // Create a tab element for a session
  createTab(session) {
    const tab = document.createElement('div');
    tab.className = 'session-tab connecting';
    tab.dataset.sessionId = session.id;
    tab.innerHTML = `
      <span class="tab-status"></span>
      <span class="tab-name" title="${session.deviceName}">${session.deviceName}</span>
      <button class="tab-close" title="Close session">×</button>
    `;

    // Click to switch
    tab.addEventListener('click', (e) => {
      if (!e.target.classList.contains('tab-close')) {
        this.switchToSession(session.id);
      }
    });

    // Close button
    tab.querySelector('.tab-close').addEventListener('click', (e) => {
      e.stopPropagation();
      this.closeSession(session.id);
    });

    this.tabsContainer.appendChild(tab);
  },

  // Switch to a different session
  switchToSession(deviceId) {
    const session = this.sessions.get(deviceId);
    if (!session) return;

    // Update active state
    this.activeSessionId = deviceId;

    // Update tab styles
    this.tabsContainer.querySelectorAll('.session-tab').forEach(tab => {
      tab.classList.toggle('active', tab.dataset.sessionId === deviceId);
    });

    // Show the session's last frame if available
    if (session.lastFrame) {
      this.displayFrame(session.lastFrame);
    }

    // Update toolbar with session info
    const connectedDeviceName = document.getElementById('connectedDeviceName');
    if (connectedDeviceName) {
      connectedDeviceName.textContent = session.deviceName;
    }

    // Show/hide idle state
    if (this.previewIdle) {
      this.previewIdle.style.display = session.status === 'connected' ? 'none' : 'flex';
    }

    console.log(`📑 Switched to session: ${session.deviceName}`);
  },

  // Update session status
  updateSessionStatus(deviceId, status) {
    const session = this.sessions.get(deviceId);
    if (!session) return;

    session.status = status;

    // Update tab class
    const tab = this.tabsContainer.querySelector(`[data-session-id="${deviceId}"]`);
    if (tab) {
      tab.classList.remove('connecting', 'connected', 'disconnected');
      tab.classList.add(status);
    }

    // Update UI if this is the active session
    if (deviceId === this.activeSessionId) {
      const previewToolbar = document.getElementById('previewToolbar');
      const previewIdle = document.getElementById('previewIdle');
      const previewConnecting = document.getElementById('previewConnecting');

      if (status === 'connected') {
        if (previewToolbar) previewToolbar.style.display = 'flex';
        if (previewIdle) previewIdle.style.display = 'none';
        if (previewConnecting) previewConnecting.style.display = 'none';
      } else if (status === 'connecting') {
        if (previewToolbar) previewToolbar.style.display = 'none';
        if (previewIdle) previewIdle.style.display = 'none';
        if (previewConnecting) previewConnecting.style.display = 'flex';
      } else {
        if (previewToolbar) previewToolbar.style.display = 'none';
        if (previewIdle) previewIdle.style.display = 'flex';
        if (previewConnecting) previewConnecting.style.display = 'none';
      }
    }

    console.log(`📑 Session ${session.deviceName} status: ${status}`);
  },

  // Store frame for a session
  storeFrame(deviceId, frameData) {
    const session = this.sessions.get(deviceId);
    if (!session) return;

    session.lastFrame = frameData;
    session.frameCount++;

    // Only display if this is the active session
    if (deviceId === this.activeSessionId) {
      this.displayFrame(frameData);
    }
  },

  // Display a frame on the canvas
  displayFrame(frameData) {
    const canvas = document.getElementById('previewCanvas');
    if (!canvas) return;

    const ctx = canvas.getContext('2d');
    const img = new Image();
    img.onload = () => {
      canvas.width = img.width;
      canvas.height = img.height;
      ctx.drawImage(img, 0, 0);
    };
    img.src = 'data:image/jpeg;base64,' + frameData;
  },

  // Close a session
  closeSession(deviceId) {
    const session = this.sessions.get(deviceId);
    if (!session) return;

    // Close WebRTC connection
    if (session.peerConnection) {
      session.peerConnection.close();
    }
    if (session.dataChannel) {
      session.dataChannel.close();
    }

    // Call the global disconnect if this is the active session
    if (deviceId === this.activeSessionId && window.disconnectFromDevice) {
      window.disconnectFromDevice();
    }

    // Remove tab
    const tab = this.tabsContainer.querySelector(`[data-session-id="${deviceId}"]`);
    if (tab) {
      tab.remove();
    }

    // Remove from sessions
    this.sessions.delete(deviceId);

    // Switch to another session if available
    if (deviceId === this.activeSessionId) {
      const remainingSessions = Array.from(this.sessions.keys());
      if (remainingSessions.length > 0) {
        this.switchToSession(remainingSessions[0]);
      } else {
        this.activeSessionId = null;
        // Show idle state
        if (this.previewIdle) {
          this.previewIdle.style.display = 'flex';
        }
        const previewToolbar = document.getElementById('previewToolbar');
        if (previewToolbar) {
          previewToolbar.style.display = 'none';
        }
      }
    }

    this.updateUI();
    console.log(`📑 Closed session: ${session.deviceName}`);
  },

  // Get the active session
  getActiveSession() {
    return this.sessions.get(this.activeSessionId);
  },

  // Check if a device has an active session
  hasSession(deviceId) {
    return this.sessions.has(deviceId);
  },

  // Update UI elements
  updateUI() {
    // Show/hide add button
    if (this.tabAddBtn) {
      this.tabAddBtn.style.display = this.sessions.size > 0 ? 'flex' : 'none';
    }

    // Show tabs container only if there are sessions
    const sessionTabs = document.getElementById('sessionTabs');
    if (sessionTabs) {
      sessionTabs.style.display = this.sessions.size > 0 ? 'flex' : 'none';
    }
  },

  // Get session count
  getSessionCount() {
    return this.sessions.size;
  }
};

// Initialize on DOM ready
document.addEventListener('DOMContentLoaded', () => {
  SessionManager.init();
});

// Export for use in other modules
window.SessionManager = SessionManager;
