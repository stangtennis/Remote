// Remote Desktop Viewer — Multi-Session Support
// Each ViewerSession is an independent WebRTC connection with its own canvas/video

class ViewerSession {
  constructor(deviceId, deviceName, container) {
    this.id = crypto.randomUUID();
    this.deviceId = deviceId;
    this.deviceName = deviceName;
    this.peerConnection = null;
    this.dataChannel = null;
    this.fileChannel = null;
    this.videoChannel = null;
    this.signalingChannel = null;
    this.pollingInterval = null;
    this.processedSignalIds = new Set();
    this.pendingIceCandidates = [];
    this.sessionData = null;
    this.supabase = null;
    this.config = null;
    this.connected = false;
    this.iceConfig = { iceServers: [{ urls: 'stun:stun.l.google.com:19302' }] };
    this.frameChunks = {};
    this.screenWidth = 0;
    this.screenHeight = 0;
    this.usingH264 = false;
    this.isFullscreen = false;
    this.inputSetup = false;

    // Create DOM elements for this session
    this.wrapper = document.createElement('div');
    this.wrapper.className = 'session-wrapper';
    this.wrapper.dataset.sessionId = this.id;
    this.wrapper.innerHTML = `
      <div class="viewer-connecting" style="display:flex;">
        <div class="connecting-spinner"></div>
        <p>Opretter forbindelse til <span class="connecting-name">${deviceName}</span>...</p>
      </div>
      <div class="viewer-active" style="display:none;">
        <div class="viewer-toolbar">
          <span class="viewer-device-label">${deviceName}</span>
          <button class="btn btn-sm btn-icon session-fullscreen-btn" title="Fuldskærm"><i class="fas fa-expand"></i></button>
          <button class="btn btn-sm btn-danger session-disconnect-btn">Afbryd</button>
        </div>
        <div class="viewer-screen">
          <video autoplay playsinline muted></video>
          <canvas tabindex="0"></canvas>
        </div>
      </div>
    `;
    container.appendChild(this.wrapper);

    this.connectingEl = this.wrapper.querySelector('.viewer-connecting');
    this.activeEl = this.wrapper.querySelector('.viewer-active');
    this.videoEl = this.wrapper.querySelector('video');
    this.canvasEl = this.wrapper.querySelector('canvas');
  }

  async connect() {
    try {
      const config = await window.go.main.App.GetConnectionConfig();
      const cfg = {
        supabase_url: config.supabase_url || config.SupabaseURL,
        anon_key: config.anon_key || config.AnonKey,
        auth_token: config.auth_token || config.AuthToken,
        user_id: config.user_id || config.UserID,
        refresh_token: config.refresh_token || config.RefreshToken,
      };

      this.supabase = window.supabase
        ? window.supabase.createClient(cfg.supabase_url, cfg.anon_key, {
            auth: { persistSession: false }
          })
        : null;

      if (this.supabase) {
        await this.supabase.auth.setSession({
          access_token: cfg.auth_token,
          refresh_token: cfg.refresh_token
        });
      }

      this.config = cfg;
      await this.fetchTurnCredentials(cfg);
      await this.createSession(cfg);
      await this.setupPeerConnection();
      this.subscribeToSignaling();
      await this.createOffer();
    } catch (err) {
      console.error(`[${this.deviceName}] Connection failed:`, err);
      showToast(`Forbindelse til ${this.deviceName} fejlede: ${err.message}`, 'error');
      this.disconnect();
    }
  }

  async fetchTurnCredentials(cfg) {
    try {
      const response = await fetch(`${cfg.supabase_url}/functions/v1/turn-credentials`, {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${cfg.auth_token}`,
          'Content-Type': 'application/json'
        }
      });
      if (response.ok) {
        const data = await response.json();
        this.iceConfig = { iceServers: data.iceServers };
        console.log(`[${this.deviceName}] TURN credentials fetched`);
      }
    } catch (e) {
      console.warn(`[${this.deviceName}] TURN fetch failed, using STUN only:`, e);
    }
  }

  async createSession(cfg) {
    const response = await fetch(`${cfg.supabase_url}/functions/v1/session-token`, {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${cfg.auth_token}`,
        'Content-Type': 'application/json'
      },
      body: JSON.stringify({ device_id: this.deviceId })
    });

    if (!response.ok) throw new Error('Kunne ikke oprette session');
    this.sessionData = await response.json();
    console.log(`[${this.deviceName}] Session created:`, this.sessionData.session_id);
  }

  async setupPeerConnection() {
    const config = { ...this.iceConfig, iceTransportPolicy: 'relay' };
    this.peerConnection = new RTCPeerConnection(config);

    this.peerConnection.onicecandidate = (event) => {
      if (event.candidate) {
        this.sendSignal({
          session_id: this.sessionData.session_id,
          from: 'dashboard',
          type: 'ice',
          candidate: event.candidate.toJSON()
        });
      }
    };

    this.peerConnection.onconnectionstatechange = () => {
      const state = this.peerConnection?.connectionState;
      console.log(`[${this.deviceName}] Connection state:`, state);
      if (state === 'connected') {
        this.onConnected();
      } else if (state === 'disconnected' || state === 'failed') {
        this.onDisconnected();
      }
    };

    this.peerConnection.ontrack = (event) => {
      console.log(`[${this.deviceName}] Track received:`, event.track.kind);
      if (event.track.kind === 'video') {
        this.usingH264 = true;
        this.videoEl.srcObject = event.streams[0];
        this.videoEl.style.display = '';
        this.canvasEl.style.pointerEvents = 'auto';
        this.canvasEl.style.background = 'transparent';
      }
    };

    this.peerConnection.ondatachannel = (event) => {
      const dc = event.channel;
      console.log(`[${this.deviceName}] Data channel received:`, dc.label);

      if (dc.label === 'video') {
        this.videoChannel = dc;
        dc.binaryType = 'arraybuffer';
        dc.onmessage = (e) => this.handleDataMessage(e);
      } else if (dc.label === 'control' || dc.label === 'screen') {
        this.dataChannel = dc;
        dc.binaryType = 'arraybuffer';
        dc.onmessage = (e) => this.handleDataMessage(e);
      } else if (dc.label === 'file-transfer' || dc.label === 'file') {
        this.fileChannel = dc;
      }
    };

    // Create all data channels the agent expects
    const controlDC = this.peerConnection.createDataChannel('control', { ordered: true });
    controlDC.binaryType = 'arraybuffer';
    controlDC.onopen = () => {
      console.log(`[${this.deviceName}] Control data channel open`);
      this.dataChannel = controlDC;
    };
    controlDC.onmessage = (e) => this.handleDataMessage(e);

    // Unreliable video channel for low-latency small frames
    const videoDC = this.peerConnection.createDataChannel('video', { ordered: false, maxRetransmits: 0 });
    videoDC.binaryType = 'arraybuffer';
    videoDC.onopen = () => console.log(`[${this.deviceName}] Video data channel open`);
    videoDC.onmessage = (e) => this.handleDataMessage(e);

    // Reliable file transfer channel
    const fileDC = this.peerConnection.createDataChannel('file', { ordered: true });
    fileDC.binaryType = 'arraybuffer';
    fileDC.onopen = () => {
      console.log(`[${this.deviceName}] File data channel open`);
      this.fileChannel = fileDC;
    };
  }

  handleDataMessage(event) {
    if (event.data instanceof ArrayBuffer) {
      const data = new Uint8Array(event.data);
      if (data.length === 0) return;

      if (data[0] === 0x7B) {
        try {
          const msg = JSON.parse(new TextDecoder().decode(data));
          if (msg.type === 'screen_info' || msg.type === 'frame_meta') {
            this.screenWidth = msg.width;
            this.screenHeight = msg.height;
          }
        } catch (e) { /* not JSON */ }
        return;
      }

      if (data.length > 3 && data[0] === 0x01) {
        this.renderFrame(data.slice(3).buffer);
        return;
      }

      if (data.length > 9 && data[0] === 0x02) {
        const x = data[1] | (data[2] << 8);
        const y = data[3] | (data[4] << 8);
        const w = data[5] | (data[6] << 8);
        const h = data[7] | (data[8] << 8);
        this.renderRegion(data.slice(9).buffer, x, y, w, h);
        return;
      }

      if (data.length > 5 && data[0] === 0xFE) {
        const frameId = (data[1] << 8) | data[2];
        const chunkIndex = data[3];
        const totalChunks = data[4];
        const chunkData = data.slice(5);

        if (!this.frameChunks[frameId]) {
          this.frameChunks[frameId] = { chunks: new Array(totalChunks), received: 0, total: totalChunks };
        }
        const frame = this.frameChunks[frameId];
        if (!frame.chunks[chunkIndex]) {
          frame.chunks[chunkIndex] = chunkData;
          frame.received++;
        }
        if (frame.received === frame.total) {
          const totalSize = frame.chunks.reduce((s, c) => s + c.length, 0);
          const assembled = new Uint8Array(totalSize);
          let offset = 0;
          for (const chunk of frame.chunks) {
            assembled.set(chunk, offset);
            offset += chunk.length;
          }
          delete this.frameChunks[frameId];
          for (const id of Object.keys(this.frameChunks)) {
            if (Number(id) < frameId - 5) delete this.frameChunks[id];
          }
          this.renderFrame(assembled.buffer);
        }
        return;
      }

      this.renderFrame(event.data);
    } else if (event.data instanceof Blob) {
      event.data.arrayBuffer().then(buf => this.handleDataMessage({ data: buf }));
    } else if (typeof event.data === 'string') {
      try {
        const msg = JSON.parse(event.data);
        if (msg.type === 'screen_info' || msg.type === 'frame_meta') {
          this.screenWidth = msg.width;
          this.screenHeight = msg.height;
        }
      } catch (e) { /* not JSON */ }
    }
  }

  renderFrame(data) {
    const blob = data instanceof Blob ? data : new Blob([data], { type: 'image/jpeg' });
    if (blob.size < 100) return;

    const img = new Image();
    img.onload = () => {
      const canvas = this.canvasEl;
      if (!canvas) return;
      const ctx = canvas.getContext('2d');
      if (canvas.width !== img.width || canvas.height !== img.height) {
        canvas.width = img.width;
        canvas.height = img.height;
        this.screenWidth = img.width;
        this.screenHeight = img.height;
      }
      ctx.drawImage(img, 0, 0);
      URL.revokeObjectURL(img.src);
    };
    img.onerror = () => URL.revokeObjectURL(img.src);
    img.src = URL.createObjectURL(blob);
  }

  renderRegion(data, x, y, w, h) {
    const canvas = this.canvasEl;
    if (canvas.width === 0 || canvas.height === 0) return;
    const ctx = canvas.getContext('2d');
    const blob = new Blob([data], { type: 'image/jpeg' });
    const img = new Image();
    img.onload = () => {
      ctx.drawImage(img, x, y);
      URL.revokeObjectURL(img.src);
    };
    img.onerror = () => URL.revokeObjectURL(img.src);
    img.src = URL.createObjectURL(blob);
  }

  async createOffer() {
    const offer = await this.peerConnection.createOffer({
      offerToReceiveVideo: true,
      offerToReceiveAudio: false
    });
    await this.peerConnection.setLocalDescription(offer);

    await this.sendSignal({
      session_id: this.sessionData.session_id,
      from: 'dashboard',
      type: 'offer',
      sdp: offer.sdp
    });
    console.log(`[${this.deviceName}] Offer sent`);
  }

  async sendSignal(payload) {
    if (!this.supabase) return;

    let signalPayload;
    if (payload.type === 'ice') {
      signalPayload = payload.candidate;
    } else {
      signalPayload = { type: payload.type, sdp: payload.sdp };
    }

    await this.supabase.from('session_signaling').insert({
      session_id: payload.session_id,
      from_side: payload.from,
      msg_type: payload.type,
      payload: signalPayload
    });
  }

  subscribeToSignaling() {
    if (!this.supabase || !this.sessionData) return;
    const sessionId = this.sessionData.session_id;

    this.signalingChannel = this.supabase
      .channel(`session:${sessionId}`)
      .on('postgres_changes',
        { event: 'INSERT', schema: 'public', table: 'session_signaling', filter: `session_id=eq.${sessionId}` },
        (payload) => this.handleSignal(payload.new)
      )
      .subscribe();

    this.pollingInterval = setInterval(async () => {
      try {
        const { data } = await this.supabase
          .from('session_signaling')
          .select('*')
          .eq('session_id', sessionId)
          .in('from_side', ['agent', 'system'])
          .order('created_at', { ascending: true });

        if (data) {
          for (const signal of data) {
            if (!this.processedSignalIds.has(signal.id)) {
              await this.handleSignal(signal);
            }
          }
        }
      } catch (e) { console.error('Poll error:', e); }
    }, 500);
  }

  async handleSignal(signal) {
    if (signal.from_side === 'dashboard') return;
    if (this.processedSignalIds.has(signal.id)) return;
    this.processedSignalIds.add(signal.id);

    const pc = this.peerConnection;
    if (!pc) return;

    try {
      switch (signal.msg_type) {
        case 'answer':
          if (pc.signalingState === 'have-local-offer') {
            await pc.setRemoteDescription(new RTCSessionDescription(signal.payload));
            for (const c of this.pendingIceCandidates) {
              await pc.addIceCandidate(new RTCIceCandidate(c));
            }
            this.pendingIceCandidates = [];
          }
          break;

        case 'ice':
          if (signal.payload && signal.payload.candidate) {
            if (!pc.remoteDescription) {
              this.pendingIceCandidates.push(signal.payload);
            } else {
              await pc.addIceCandidate(new RTCIceCandidate(signal.payload));
            }
          }
          break;

        case 'kick':
          showToast(`${this.deviceName}: Afkoblet — en anden controller har overtaget.`, 'warning');
          this.disconnect();
          break;
      }
    } catch (e) {
      console.error(`[${this.deviceName}] Signal handling error:`, e);
    }
  }

  onConnected() {
    this.connected = true;
    this.connectingEl.style.display = 'none';
    this.activeEl.style.display = 'flex';
    showToast(`Forbundet til ${this.deviceName}`, 'success');
    this.setupInput();
    // Notify session manager
    if (window.SessionManager) {
      window.SessionManager.onSessionConnected(this.id);
    }
  }

  onDisconnected() {
    if (!this.connected) return;
    this.connected = false;
    showToast(`Forbindelse til ${this.deviceName} tabt`, 'warning');
    this.disconnect();
  }

  disconnect() {
    if (this.pollingInterval) { clearInterval(this.pollingInterval); this.pollingInterval = null; }
    if (this.signalingChannel && this.supabase) { this.supabase.removeChannel(this.signalingChannel); }
    if (this.peerConnection) { this.peerConnection.close(); this.peerConnection = null; }
    this.dataChannel = null;
    this.fileChannel = null;
    this.videoChannel = null;
    this.frameChunks = {};
    this.usingH264 = false;
    this.processedSignalIds.clear();
    this.pendingIceCandidates = [];
    this.connected = false;
    this.isFullscreen = false;

    // Remove DOM
    if (this.wrapper && this.wrapper.parentNode) {
      this.wrapper.remove();
    }

    // Notify session manager
    if (window.SessionManager) {
      window.SessionManager.onSessionDisconnected(this.id);
    }
  }

  setupInput() {
    if (this.inputSetup) return;
    this.inputSetup = true;

    const canvas = this.canvasEl;
    canvas.focus();

    canvas.addEventListener('click', () => canvas.focus());
    canvas.addEventListener('mousemove', (e) => this.sendMouseEvent('mousemove', e));
    canvas.addEventListener('mousedown', (e) => { e.preventDefault(); this.sendMouseEvent('mousedown', e); });
    canvas.addEventListener('mouseup', (e) => this.sendMouseEvent('mouseup', e));
    canvas.addEventListener('wheel', (e) => { e.preventDefault(); this.sendWheelEvent(e); }, { passive: false });
    canvas.addEventListener('contextmenu', (e) => e.preventDefault());
    canvas.addEventListener('keydown', (e) => { e.preventDefault(); this.sendKeyEvent('keydown', e); });
    canvas.addEventListener('keyup', (e) => { e.preventDefault(); this.sendKeyEvent('keyup', e); });

    this.wrapper.querySelector('.session-disconnect-btn').addEventListener('click', () => this.disconnect());
    this.wrapper.querySelector('.session-fullscreen-btn').addEventListener('click', () => this.toggleFullscreen());
  }

  async toggleFullscreen() {
    try {
      await window.go.main.App.ToggleFullscreen();
      this.isFullscreen = !this.isFullscreen;
      const toolbar = this.wrapper.querySelector('.viewer-toolbar');
      if (toolbar) {
        toolbar.classList.toggle('fullscreen-autohide', this.isFullscreen);
      }
    } catch (e) {
      console.error('Fullscreen toggle failed:', e);
    }
  }

  sendMouseEvent(type, e) {
    if (!this.dataChannel || this.dataChannel.readyState !== 'open') return;
    const canvas = this.canvasEl;
    const rect = canvas.getBoundingClientRect();
    const scaleX = (this.screenWidth || canvas.width) / rect.width;
    const scaleY = (this.screenHeight || canvas.height) / rect.height;
    const x = Math.round((e.clientX - rect.left) * scaleX);
    const y = Math.round((e.clientY - rect.top) * scaleY);

    // Agent expects: t=mouse_move/mouse_click, button as string, down as bool
    const buttonNames = ['left', 'right', 'middle'];
    if (type === 'mousemove') {
      this.dataChannel.send(JSON.stringify({ t: 'mouse_move', x: x, y: y }));
    } else if (type === 'mousedown') {
      this.dataChannel.send(JSON.stringify({ t: 'mouse_click', x: x, y: y, button: buttonNames[e.button] || 'left', down: true }));
    } else if (type === 'mouseup') {
      this.dataChannel.send(JSON.stringify({ t: 'mouse_click', x: x, y: y, button: buttonNames[e.button] || 'left', down: false }));
    }
  }

  sendWheelEvent(e) {
    if (!this.dataChannel || this.dataChannel.readyState !== 'open') return;
    // Agent expects: t=mouse_scroll, delta (positive=up, negative=down)
    this.dataChannel.send(JSON.stringify({
      t: 'mouse_scroll',
      delta: -Math.round(e.deltaY)
    }));
  }

  sendKeyEvent(type, e) {
    if (!this.dataChannel || this.dataChannel.readyState !== 'open') return;

    if (type === 'keydown') {
      if (e.code === 'F11') { this.toggleFullscreen(); return; }
      if (e.code === 'Escape' && this.isFullscreen) { this.toggleFullscreen(); return; }
    }

    // Agent expects: t=key, down=bool, code, char for text input
    this.dataChannel.send(JSON.stringify({
      t: 'key',
      code: e.code,
      key: e.key,
      down: type === 'keydown',
      shift: e.shiftKey,
      ctrl: e.ctrlKey,
      alt: e.altKey,
      meta: e.metaKey,
      char: (type === 'keydown' && e.key.length === 1) ? e.key : ''
    }));
  }

  show() {
    this.wrapper.style.display = 'flex';
    if (this.connected && this.canvasEl) {
      this.canvasEl.focus();
    }
  }

  hide() {
    this.wrapper.style.display = 'none';
  }
}

// Session Manager — handles multiple concurrent sessions
const SessionManager = {
  sessions: new Map(),  // id -> ViewerSession
  activeSessionId: null,

  getContainer() {
    return document.getElementById('sessionContainer');
  },

  getTabBar() {
    return document.getElementById('sessionTabs');
  },

  connect(deviceId, deviceName) {
    console.log('SessionManager.connect:', deviceId, deviceName);
    showToast(`Forbinder til ${deviceName}...`, 'info');

    // Check if already connected to this device
    for (const [id, session] of this.sessions) {
      if (session.deviceId === deviceId) {
        this.switchTo(id);
        showToast(`Allerede forbundet til ${deviceName}`, 'info');
        return;
      }
    }

    // Hide idle state
    document.getElementById('viewerIdle').style.display = 'none';

    // Create new session
    const container = this.getContainer();
    if (!container) {
      showToast('FEJL: sessionContainer element ikke fundet!', 'error');
      return;
    }
    const session = new ViewerSession(deviceId, deviceName, container);
    this.sessions.set(session.id, session);

    // Add tab
    this.addTab(session);

    // Switch to new session
    this.switchTo(session.id);

    // Start connection
    session.connect();
  },

  addTab(session) {
    const tabBar = this.getTabBar();
    tabBar.style.display = 'flex';

    const tab = document.createElement('button');
    tab.className = 'session-tab active';
    tab.dataset.sessionId = session.id;
    tab.innerHTML = `
      <span class="session-tab-dot connecting"></span>
      <span class="session-tab-name">${session.deviceName}</span>
      <span class="session-tab-close" title="Afbryd">&times;</span>
    `;

    tab.addEventListener('click', (e) => {
      if (e.target.classList.contains('session-tab-close')) {
        session.disconnect();
      } else {
        this.switchTo(session.id);
      }
    });

    tabBar.appendChild(tab);
  },

  switchTo(sessionId) {
    this.activeSessionId = sessionId;

    // Update tabs
    this.getTabBar().querySelectorAll('.session-tab').forEach(tab => {
      tab.classList.toggle('active', tab.dataset.sessionId === sessionId);
    });

    // Show/hide session wrappers
    for (const [id, session] of this.sessions) {
      if (id === sessionId) {
        session.show();
      } else {
        session.hide();
      }
    }
  },

  onSessionConnected(sessionId) {
    const tab = this.getTabBar().querySelector(`[data-session-id="${sessionId}"]`);
    if (tab) {
      const dot = tab.querySelector('.session-tab-dot');
      dot.classList.remove('connecting');
      dot.classList.add('connected');
    }
  },

  onSessionDisconnected(sessionId) {
    this.sessions.delete(sessionId);

    // Remove tab
    const tab = this.getTabBar().querySelector(`[data-session-id="${sessionId}"]`);
    if (tab) tab.remove();

    // If no sessions left, show idle
    if (this.sessions.size === 0) {
      document.getElementById('viewerIdle').style.display = 'flex';
      this.getTabBar().style.display = 'none';
      this.activeSessionId = null;
    } else if (this.activeSessionId === sessionId) {
      // Switch to another session
      const nextId = this.sessions.keys().next().value;
      this.switchTo(nextId);
    }
  },

  disconnectAll() {
    for (const [id, session] of this.sessions) {
      session.disconnect();
    }
  }
};

window.SessionManager = SessionManager;

// Backwards compat — Viewer.connect still works
window.Viewer = {
  connect(deviceId, deviceName) {
    SessionManager.connect(deviceId, deviceName);
  },
  disconnect() {
    SessionManager.disconnectAll();
  }
};
