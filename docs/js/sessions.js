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

    debug('📑 Session Manager initialized');
  },

  // Create a new session for a device
  createSession(deviceId, deviceName) {
    if (this.sessions.has(deviceId)) {
      debug(`Session already exists for ${deviceName}, switching to it`);
      this.switchToSession(deviceId);
      return this.sessions.get(deviceId);
    }

    if (this.sessions.size >= this.maxSessions) {
      showToast(`Maksimalt ${this.maxSessions} sessioner tilladt. Luk venligst en session først.`, 'warning');
      return null;
    }

    const session = {
      // Identity
      id: deviceId,
      deviceName: deviceName,
      status: 'connecting', // connecting, connected, disconnected
      createdAt: Date.now(),
      // WebRTC
      peerConnection: null,
      dataChannel: null,
      // Session + signaling
      sessionData: null,           // fra session-token API
      signalingChannel: null,      // Supabase realtime channel
      pollingInterval: null,       // setInterval ID
      processedSignalIds: new Set(),
      pendingIceCandidates: [],
      // Frame display
      lastFrame: null,
      frameCount: 0,
      // Frame reassembly (per-session)
      frameChunks: [],
      expectedChunks: 0,
      currentFrameId: -1,
      frameTimeout: null,
      screenWidth: 0,
      screenHeight: 0,
      // Bandwidth/stats
      bytesReceived: 0,
      framesReceived: 0,
      framesDropped: 0,
      lastBandwidthCheck: Date.now(),
      currentBandwidthMbps: 0,
      bandwidthInterval: null,
      statsInterval: null,
      // Auto-reconnect state
      reconnectState: 'idle',       // 'idle' | 'reconnecting' | 'gave_up'
      reconnectAttempt: 0,
      reconnectTimer: null,
      reconnectStartedAt: null,
    };

    this.sessions.set(deviceId, session);
    this.createTab(session);
    this.switchToSession(deviceId);
    this.updateUI();

    debug(`📑 Created session for ${deviceName} (${deviceId})`);
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
      // Use endSession for full cleanup (signaling, DB, WebRTC)
      if (window.endSession) {
        window.endSession(session.id);
      }
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

    // Swap global refs so legacy code and input handlers work
    window.currentSession = session.sessionData;
    window.peerConnection = session.peerConnection;
    window.dataChannel = session.dataChannel;

    // Handle display based on session type
    const previewVideo = document.getElementById('previewVideo');
    const previewCanvas = document.getElementById('previewCanvas');

    if (deviceId === 'quick-support') {
      if (previewVideo) previewVideo.style.display = 'block';
      if (previewCanvas) previewCanvas.style.display = 'none';
    } else {
      if (previewVideo) previewVideo.style.display = '';
      if (previewCanvas) previewCanvas.style.display = '';
      // Restore last frame for this session
      if (session.lastFrame) {
        this.displayFrame(session.lastFrame);
      }
    }

    // Update toolbar with session info
    const connectedDeviceName = document.getElementById('connectedDeviceName');
    if (connectedDeviceName) {
      connectedDeviceName.textContent = session.deviceName;
    }

    // Update bandwidth display for this session
    const statsEl = document.getElementById('bandwidthStats');
    if (statsEl) {
      if (session.currentBandwidthMbps > 0) {
        statsEl.textContent = `${session.currentBandwidthMbps.toFixed(1)} Mbit/s`;
      } else {
        statsEl.textContent = '';
      }
    }

    // Show/hide idle state
    if (this.previewIdle) {
      this.previewIdle.style.display = session.status === 'connected' ? 'none' : 'flex';
    }

    debug(`📑 Switched to session: ${session.deviceName}`);
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

    debug(`📑 Session ${session.deviceName} status: ${status}`);
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

  // Display a frame on both canvases (preview + viewer)
  displayFrame(frameData) {
    const canvases = [
      document.getElementById('previewCanvas'),
      document.getElementById('remoteCanvas')
    ].filter(Boolean);
    if (canvases.length === 0) return;

    const img = new Image();
    img.onload = () => {
      canvases.forEach(canvas => {
        if (canvas.width !== img.width || canvas.height !== img.height) {
          canvas.width = img.width;
          canvas.height = img.height;
        }
        canvas.getContext('2d').drawImage(img, 0, 0);
      });
    };
    img.src = 'data:image/jpeg;base64,' + frameData;
  },

  // Close a session (tab + switching only — full cleanup done by endSession in app.js)
  closeSession(deviceId) {
    const session = this.sessions.get(deviceId);
    if (!session) return;

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
        window.currentSession = null;
        window.peerConnection = null;
        window.dataChannel = null;
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
    debug(`📑 Closed session: ${session.deviceName}`);
  },

  // Get the active session
  getActiveSession() {
    return this.sessions.get(this.activeSessionId);
  },

  // Check if a device has an active session
  hasSession(deviceId) {
    return this.sessions.has(deviceId);
  },

  // Lookup session by signaling session_id
  getSessionBySessionId(sessionId) {
    for (const session of this.sessions.values()) {
      if (session.sessionData && session.sessionData.session_id === sessionId) {
        return session;
      }
    }
    return null;
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
