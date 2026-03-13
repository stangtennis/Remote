// Remote Desktop Viewer — Phase 2
// Handles WebRTC connection, video display, and input capture
// Uses browser-native WebRTC (no Go pion stack needed)

const Viewer = {
  deviceId: null,
  deviceName: null,
  peerConnection: null,
  dataChannel: null,
  fileChannel: null,
  signalingChannel: null,
  pollingInterval: null,
  processedSignalIds: new Set(),
  pendingIceCandidates: [],
  sessionData: null,
  supabase: null,
  connected: false,
  iceConfig: { iceServers: [{ urls: 'stun:stun.l.google.com:19302' }] },

  // Frame state
  frameChunks: {},
  currentFrameId: -1,
  screenWidth: 0,
  screenHeight: 0,
  videoChannel: null,
  usingH264: false,
  isFullscreen: false,

  async connect(deviceId, deviceName) {
    this.deviceId = deviceId;
    this.deviceName = deviceName;

    // Show connecting state
    document.getElementById('viewerIdle').style.display = 'none';
    document.getElementById('viewerConnecting').style.display = 'flex';
    document.getElementById('viewerActive').style.display = 'none';
    document.getElementById('connectingDeviceName').textContent = deviceName;

    try {
      // Get connection config from Go backend
      const config = await window.go.main.App.GetConnectionConfig();

      // Normalize config keys (Wails returns PascalCase, JSON returns snake_case)
      const cfg = {
        supabase_url: config.supabase_url || config.SupabaseURL,
        anon_key: config.anon_key || config.AnonKey,
        auth_token: config.auth_token || config.AuthToken,
        user_id: config.user_id || config.UserID,
        refresh_token: config.refresh_token || config.RefreshToken,
      };

      // Initialize Supabase client for signaling
      this.supabase = window.supabase
        ? window.supabase.createClient(cfg.supabase_url, cfg.anon_key, {
            auth: { persistSession: false }
          })
        : null;

      if (this.supabase) {
        // Set auth token
        await this.supabase.auth.setSession({
          access_token: cfg.auth_token,
          refresh_token: cfg.refresh_token
        });
      }

      // Store normalized config for later use
      this.config = cfg;

      // Fetch TURN credentials
      await this.fetchTurnCredentials(cfg);

      // Create session token
      await this.createSession(cfg);

      // Setup WebRTC
      await this.setupPeerConnection();

      // Subscribe to signaling
      this.subscribeToSignaling();

      // Create and send offer
      await this.createOffer();

    } catch (err) {
      console.error('Connection failed:', err);
      showToast('Forbindelse fejlede: ' + err.message, 'error');
      this.disconnect();
    }
  },

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
        console.log('TURN credentials fetched');
      }
    } catch (e) {
      console.warn('TURN fetch failed, using STUN only:', e);
    }
  },

  async createSession(cfg) {
    const response = await fetch(`${cfg.supabase_url}/functions/v1/session-token`, {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${cfg.auth_token}`,
        'Content-Type': 'application/json'
      },
      body: JSON.stringify({ device_id: this.deviceId })
    });

    if (!response.ok) {
      throw new Error('Kunne ikke oprette session');
    }

    this.sessionData = await response.json();
    console.log('Session created:', this.sessionData.session_id);
  },

  async setupPeerConnection() {
    this.peerConnection = new RTCPeerConnection(this.iceConfig);

    // ICE candidates
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

    // Connection state
    this.peerConnection.onconnectionstatechange = () => {
      const state = this.peerConnection.connectionState;
      console.log('Connection state:', state);
      if (state === 'connected') {
        this.onConnected();
      } else if (state === 'disconnected' || state === 'failed') {
        this.onDisconnected();
      }
    };

    // Media tracks (H.264 video)
    this.peerConnection.ontrack = (event) => {
      console.log('Track received:', event.track.kind);
      if (event.track.kind === 'video') {
        this.usingH264 = true;
        const video = document.getElementById('viewerVideo');
        const canvas = document.getElementById('viewerCanvas');
        video.srcObject = event.streams[0];
        video.style.display = '';
        // H.264 track: show video, hide canvas overlay
        canvas.style.pointerEvents = 'auto';
        canvas.style.background = 'transparent';
      }
    };

    // Data channels from agent (agent-created channels)
    this.peerConnection.ondatachannel = (event) => {
      const dc = event.channel;
      console.log('Data channel received from agent:', dc.label);

      if (dc.label === 'video') {
        this.videoChannel = dc;
        dc.binaryType = 'arraybuffer';
        dc.onmessage = (event) => this.handleDataMessage(event);
        dc.onopen = () => console.log('Video data channel open (unreliable)');
      } else if (dc.label === 'control' || dc.label === 'screen') {
        this.dataChannel = dc;
        dc.binaryType = 'arraybuffer';
        dc.onmessage = (event) => this.handleDataMessage(event);
      } else if (dc.label === 'file-transfer' || dc.label === 'file') {
        this.fileChannel = dc;
      }
    };

    // Create control data channel (for sending input + receiving frames)
    const controlDC = this.peerConnection.createDataChannel('control', { ordered: true });
    controlDC.binaryType = 'arraybuffer';
    controlDC.onopen = () => {
      console.log('Control data channel open');
      this.dataChannel = controlDC;
    };
    // Agent sends frames back on this same channel
    controlDC.onmessage = (event) => this.handleDataMessage(event);
  },

  handleDataMessage(event) {
    if (event.data instanceof ArrayBuffer) {
      const data = new Uint8Array(event.data);
      if (data.length === 0) return;

      // Check if this is JSON (starts with '{' = 0x7B)
      if (data[0] === 0x7B) {
        try {
          const text = new TextDecoder().decode(data);
          const msg = JSON.parse(text);
          if (msg.type === 'screen_info' || msg.type === 'frame_meta') {
            this.screenWidth = msg.width;
            this.screenHeight = msg.height;
          }
        } catch (e) { /* not JSON */ }
        return;
      }

      const CHUNK_MAGIC = 0xFE;
      const FRAME_TYPE_REGION = 0x02;

      // Dirty region update (type 0x02)
      if (data.length > 9 && data[0] === FRAME_TYPE_REGION) {
        const x = data[1] | (data[2] << 8);
        const y = data[3] | (data[4] << 8);
        const w = data[5] | (data[6] << 8);
        const h = data[7] | (data[8] << 8);
        this.renderRegion(data.slice(9).buffer, x, y, w, h);
        return;
      }

      // Chunked frame (magic byte 0xFE)
      if (data.length > 5 && data[0] === CHUNK_MAGIC) {
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
          // Clean up old incomplete frames
          for (const id of Object.keys(this.frameChunks)) {
            if (Number(id) < frameId - 5) delete this.frameChunks[id];
          }
          this.renderFrame(assembled.buffer);
        }
        return;
      }

      // Single JPEG frame (raw, starts with FF D8)
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
  },

  renderFrame(data) {
    const blob = data instanceof Blob ? data : new Blob([data], { type: 'image/jpeg' });
    if (blob.size < 100) return; // Too small, corrupt

    const img = new Image();
    img.onload = () => {
      const canvas = document.getElementById('viewerCanvas');
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
  },

  renderRegion(data, x, y, w, h) {
    const canvas = document.getElementById('viewerCanvas');
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
  },

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
    console.log('Offer sent');
  },

  // ==================== SIGNALING ====================
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
  },

  subscribeToSignaling() {
    if (!this.supabase || !this.sessionData) return;
    const sessionId = this.sessionData.session_id;

    // Realtime subscription
    this.signalingChannel = this.supabase
      .channel(`session:${sessionId}`)
      .on('postgres_changes',
        { event: 'INSERT', schema: 'public', table: 'session_signaling', filter: `session_id=eq.${sessionId}` },
        (payload) => this.handleSignal(payload.new)
      )
      .subscribe();

    // Polling fallback
    this.pollingInterval = setInterval(async () => {
      const { data } = await this.supabase
        .from('session_signaling')
        .select('*')
        .eq('session_id', sessionId)
        .in('from_side', ['agent', 'system'])
        .order('created_at', { ascending: true });

      if (data) {
        for (const signal of data) {
          if (!this.processedSignalIds.has(signal.id)) {
            this.processedSignalIds.add(signal.id);
            await this.handleSignal(signal);
          }
        }
      }
    }, 500);
  },

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
            // Flush buffered ICE
            for (const c of this.pendingIceCandidates) {
              await pc.addIceCandidate(new RTCIceCandidate(c));
            }
            this.pendingIceCandidates = [];
          }
          break;

        case 'ice':
          const candidate = signal.payload.candidate ? signal.payload : signal.payload;
          if (candidate && candidate.candidate) {
            if (!pc.remoteDescription) {
              this.pendingIceCandidates.push(candidate);
            } else {
              await pc.addIceCandidate(new RTCIceCandidate(candidate));
            }
          }
          break;

        case 'kick':
          showToast('Afkoblet — en anden controller har overtaget.', 'warning');
          this.disconnect();
          break;
      }
    } catch (e) {
      console.error('Signal handling error:', e);
    }
  },

  // ==================== CONNECTED STATE ====================
  onConnected() {
    this.connected = true;
    document.getElementById('viewerConnecting').style.display = 'none';
    document.getElementById('viewerActive').style.display = 'flex';
    document.getElementById('viewerDeviceName').textContent = this.deviceName;
    showToast(`Forbundet til ${this.deviceName}`, 'success');

    // Setup input handlers
    this.setupInput();
  },

  onDisconnected() {
    if (!this.connected) return;
    this.connected = false;
    showToast('Forbindelse tabt', 'warning');
    this.disconnect();
  },

  disconnect() {
    // Cleanup
    if (this.pollingInterval) { clearInterval(this.pollingInterval); this.pollingInterval = null; }
    if (this.signalingChannel) { this.supabase?.removeChannel(this.signalingChannel); }
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

    // Reset UI
    document.getElementById('viewerIdle').style.display = 'flex';
    document.getElementById('viewerConnecting').style.display = 'none';
    document.getElementById('viewerActive').style.display = 'none';
  },

  // ==================== INPUT ====================
  setupInput() {
    const canvas = document.getElementById('viewerCanvas');
    const video = document.getElementById('viewerVideo');
    canvas.focus();

    // Ensure canvas gets focus for keyboard input
    canvas.addEventListener('click', () => canvas.focus());

    // Mouse events
    canvas.addEventListener('mousemove', (e) => this.sendMouseEvent('mousemove', e));
    canvas.addEventListener('mousedown', (e) => { e.preventDefault(); this.sendMouseEvent('mousedown', e); });
    canvas.addEventListener('mouseup', (e) => this.sendMouseEvent('mouseup', e));
    canvas.addEventListener('wheel', (e) => { e.preventDefault(); this.sendWheelEvent(e); }, { passive: false });
    canvas.addEventListener('contextmenu', (e) => e.preventDefault());

    // Keyboard events
    canvas.addEventListener('keydown', (e) => { e.preventDefault(); this.sendKeyEvent('keydown', e); });
    canvas.addEventListener('keyup', (e) => { e.preventDefault(); this.sendKeyEvent('keyup', e); });

    // Disconnect button
    document.getElementById('viewerDisconnectBtn').addEventListener('click', () => this.disconnect());

    // Fullscreen via Wails runtime
    document.getElementById('viewerFullscreenBtn').addEventListener('click', () => this.toggleFullscreen());
  },

  async toggleFullscreen() {
    try {
      await window.go.main.App.ToggleFullscreen();
      this.isFullscreen = !this.isFullscreen;
      const toolbar = document.querySelector('.viewer-toolbar');
      if (toolbar) {
        if (this.isFullscreen) {
          toolbar.classList.add('fullscreen-autohide');
        } else {
          toolbar.classList.remove('fullscreen-autohide');
        }
      }
    } catch (e) {
      console.error('Fullscreen toggle failed:', e);
    }
  },

  sendMouseEvent(type, e) {
    if (!this.dataChannel || this.dataChannel.readyState !== 'open') return;
    const canvas = e.target;
    const rect = canvas.getBoundingClientRect();
    const scaleX = (this.screenWidth || canvas.width) / rect.width;
    const scaleY = (this.screenHeight || canvas.height) / rect.height;
    const x = Math.round((e.clientX - rect.left) * scaleX);
    const y = Math.round((e.clientY - rect.top) * scaleY);

    this.dataChannel.send(JSON.stringify({
      type: type,
      x: x, y: y,
      button: e.button
    }));
  },

  sendWheelEvent(e) {
    if (!this.dataChannel || this.dataChannel.readyState !== 'open') return;
    this.dataChannel.send(JSON.stringify({
      type: 'wheel',
      deltaX: Math.round(e.deltaX),
      deltaY: Math.round(e.deltaY)
    }));
  },

  sendKeyEvent(type, e) {
    if (!this.dataChannel || this.dataChannel.readyState !== 'open') return;

    // Intercept F11 and ESC locally for fullscreen toggle
    if (type === 'keydown') {
      if (e.code === 'F11') {
        this.toggleFullscreen();
        return;
      }
      if (e.code === 'Escape' && this.isFullscreen) {
        this.toggleFullscreen();
        return;
      }
    }

    this.dataChannel.send(JSON.stringify({
      type: type,
      code: e.code,
      key: e.key,
      shift: e.shiftKey,
      ctrl: e.ctrlKey,
      alt: e.altKey,
      meta: e.metaKey
    }));
  }
};

window.Viewer = Viewer;
