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
  frameChunks: [],
  expectedChunks: 0,
  currentFrameId: -1,
  screenWidth: 0,
  screenHeight: 0,

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

      // Initialize Supabase client for signaling
      this.supabase = window.supabase
        ? window.supabase.createClient(config.supabase_url, config.anon_key, {
            auth: { persistSession: false }
          })
        : null;

      if (this.supabase) {
        // Set auth token
        await this.supabase.auth.setSession({
          access_token: config.auth_token,
          refresh_token: config.refresh_token
        });
      }

      // Fetch TURN credentials
      await this.fetchTurnCredentials(config);

      // Create session token
      await this.createSession(config);

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

  async fetchTurnCredentials(config) {
    try {
      const response = await fetch(`${config.supabase_url}/functions/v1/turn-credentials`, {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${config.auth_token}`,
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

  async createSession(config) {
    const response = await fetch(`${config.supabase_url}/functions/v1/session-token`, {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${config.auth_token}`,
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
        const video = document.getElementById('viewerVideo');
        video.srcObject = event.streams[0];
        video.style.display = '';
      }
    };

    // Data channels
    this.peerConnection.ondatachannel = (event) => {
      const dc = event.channel;
      console.log('Data channel received:', dc.label);

      if (dc.label === 'control' || dc.label === 'screen') {
        this.dataChannel = dc;
        this.setupDataChannel(dc);
      } else if (dc.label === 'file-transfer') {
        this.fileChannel = dc;
      }
    };

    // Create control data channel (for sending input)
    const controlDC = this.peerConnection.createDataChannel('control', { ordered: true });
    controlDC.onopen = () => {
      console.log('Control data channel open');
      this.dataChannel = controlDC;
    };
  },

  setupDataChannel(dc) {
    dc.onmessage = (event) => {
      this.handleDataMessage(event);
    };
  },

  handleDataMessage(event) {
    // Handle JPEG frame data from agent
    if (event.data instanceof ArrayBuffer || event.data instanceof Blob) {
      // Binary frame data
      this.handleFrameData(event.data);
    } else if (typeof event.data === 'string') {
      try {
        const msg = JSON.parse(event.data);
        if (msg.type === 'frame_meta') {
          this.screenWidth = msg.width;
          this.screenHeight = msg.height;
          this.expectedChunks = msg.chunks;
          this.currentFrameId = msg.frame_id;
          this.frameChunks = [];
        }
      } catch (e) {
        // Not JSON, might be frame data
      }
    }
  },

  async handleFrameData(data) {
    // Render on canvas
    const blob = data instanceof Blob ? data : new Blob([data], { type: 'image/jpeg' });
    const bitmap = await createImageBitmap(blob);
    const canvas = document.getElementById('viewerCanvas');
    const ctx = canvas.getContext('2d');

    if (canvas.width !== bitmap.width || canvas.height !== bitmap.height) {
      canvas.width = bitmap.width;
      canvas.height = bitmap.height;
    }
    ctx.drawImage(bitmap, 0, 0);
    bitmap.close();
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
    this.processedSignalIds.clear();
    this.pendingIceCandidates = [];
    this.connected = false;

    // Reset UI
    document.getElementById('viewerIdle').style.display = 'flex';
    document.getElementById('viewerConnecting').style.display = 'none';
    document.getElementById('viewerActive').style.display = 'none';
  },

  // ==================== INPUT ====================
  setupInput() {
    const canvas = document.getElementById('viewerCanvas');
    canvas.focus();

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

    // Fullscreen
    document.getElementById('viewerFullscreenBtn').addEventListener('click', () => {
      const container = document.getElementById('viewerContainer');
      if (document.fullscreenElement) {
        document.exitFullscreen();
      } else {
        container.requestFullscreen();
      }
    });
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
