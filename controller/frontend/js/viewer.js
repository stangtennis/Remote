// Remote Desktop Viewer — Multi-Session Support
// Each ViewerSession is an independent WebRTC connection with its own canvas/video

// Global console interceptor — fanger sidste 500 log-events til "Vis log"-knappen.
// Wails' WebView2 har ingen indbygget DevTools i prod-build, så uden dette
// ville support-debug kræve at udvikleren sad foran maskinen.
(function() {
  if (window._globalConsoleLog) return; // already wired
  window._globalConsoleLog = [];
  const RING_MAX = 500;
  const orig = { log: console.log, warn: console.warn, error: console.error, info: console.info };
  function record(level, args) {
    try {
      const ts = new Date().toISOString().substring(11, 23);
      const msg = Array.from(args).map(a => {
        if (a instanceof Error) return a.stack || a.message;
        if (typeof a === 'object') { try { return JSON.stringify(a); } catch (_) { return String(a); } }
        return String(a);
      }).join(' ');
      window._globalConsoleLog.push(`[${ts}] [${level}] ${msg}`);
      if (window._globalConsoleLog.length > RING_MAX) window._globalConsoleLog.shift();
    } catch (_) { /* never break logging */ }
  }
  console.log   = function() { record('log',   arguments); orig.log.apply(console, arguments); };
  console.info  = function() { record('info',  arguments); orig.info.apply(console, arguments); };
  console.warn  = function() { record('warn',  arguments); orig.warn.apply(console, arguments); };
  console.error = function() { record('error', arguments); orig.error.apply(console, arguments); };
})();

// ClipboardBroker centralises clipboard distribution between the controller's
// own OS clipboard and every connected session. A copy made on remote PC A
// is written to the controller's clipboard *and* forwarded to PC B, C, ...
// so a paste anywhere works without the user touching anything. A simple
// 5-second hash window prevents echo loops between sessions (agent →
// controller → broadcast back to agent → agent's monitor would otherwise
// fire again).
const ClipboardBroker = {
  recentHashes: new Map(),  // hash -> timestamp (ms)
  windowMs: 5000,

  hash(kind, payload) {
    let h = 0;
    const s = kind + ':' + payload;
    for (let i = 0; i < s.length; i++) {
      h = ((h << 5) - h + s.charCodeAt(i)) | 0;
    }
    return h.toString(36);
  },

  isRecent(hashKey) {
    const now = Date.now();
    // Drop entries older than the dedup window
    for (const [k, t] of this.recentHashes) {
      if (now - t > this.windowMs) this.recentHashes.delete(k);
    }
    return this.recentHashes.has(hashKey);
  },

  remember(hashKey) {
    this.recentHashes.set(hashKey, Date.now());
  },

  // spread: writes to local OS clipboard + sends to every other open
  // session. Called when one remote PC reports a clipboard change.
  spread(kind, payload, sourceSessionId) {
    const key = this.hash(kind, payload);
    if (this.isRecent(key)) return;
    this.remember(key);

    // 1. Update controller's local OS clipboard
    this._writeLocal(kind, payload);

    // 2. Forward to every other connected session
    if (typeof SessionManager !== 'undefined') {
      for (const [id, session] of SessionManager.sessions) {
        if (id === sourceSessionId) continue;
        this._sendToSession(session, kind, payload);
      }
    }
  },

  // broadcast: send to every connected session (no source to exclude).
  // Called on Ctrl+V — the local clipboard is the source of truth.
  broadcast(kind, payload, _ignored) {
    const key = this.hash(kind, payload);
    if (this.isRecent(key)) return;
    this.remember(key);

    if (typeof SessionManager !== 'undefined') {
      for (const [, session] of SessionManager.sessions) {
        this._sendToSession(session, kind, payload);
      }
    }
  },

  _writeLocal(kind, payload) {
    if (kind === 'text') {
      navigator.clipboard.writeText(payload).catch(err => {
        console.warn('clipboard.writeText failed:', err);
      });
      return;
    }
    if (kind === 'image') {
      try {
        const binary = atob(payload);
        const bytes = new Uint8Array(binary.length);
        for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i);
        const blob = new Blob([bytes], { type: 'image/png' });
        navigator.clipboard.write([new ClipboardItem({ 'image/png': blob })]).catch(err => {
          console.warn('clipboard.write image failed:', err);
        });
      } catch (e) {
        console.warn('decode clipboard image failed:', e);
      }
    }
  },

  _sendToSession(session, kind, payload) {
    const dc = session && session.dataChannel;
    if (!dc || dc.readyState !== 'open') return;
    const msgType = kind === 'text' ? 'clipboard_text' : 'clipboard_image';
    try {
      dc.send(JSON.stringify({ type: msgType, content: payload }));
    } catch (e) {
      console.warn('Failed to forward clipboard to session', session.id, e);
    }
  },
};

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

    // Auto-reconnect state
    this.reconnectState = 'idle';       // 'idle' | 'reconnecting' | 'gave_up'
    this.reconnectAttempt = 0;
    this.reconnectTimer = null;
    this.reconnectStartedAt = null;
    this.manualDisconnect = false;
    this._connectLog = [];

    // Create DOM elements for this session
    this.wrapper = document.createElement('div');
    this.wrapper.className = 'session-wrapper';
    this.wrapper.dataset.sessionId = this.id;
    // Stats tracking state
    this.statsInterval = null;
    this.prevBytesReceived = 0;
    this.prevTimestamp = 0;
    this.prevPacketsLost = 0;
    this.prevPacketsReceived = 0;
    this.statsHistory = { rtt: [], bw: [], fps: [], loss: [] }; // last 60 data points (1 per sec)
    this.currentQuality = { rtt: null, loss: null, bw: null, tier: 'unknown' };

    this.wrapper.innerHTML = `
      <div class="viewer-connecting" style="display:flex; flex-direction:column; align-items:center; justify-content:center; gap:0.5rem; overflow-y:auto; padding:1rem;">
        <div class="connecting-spinner"></div>
        <p style="font-size:0.75rem; text-align:left; line-height:1.5; font-family:monospace; max-height:50vh; overflow-y:auto; width:100%; max-width:500px; padding:0.5rem; background:rgba(0,0,0,0.3); border-radius:8px; user-select:text; -webkit-user-select:text;">Opretter forbindelse til ${deviceName}...</p>
        <button class="btn btn-sm btn-secondary connecting-copy-btn" style="display:none;" onclick="navigator.clipboard.writeText(this.previousElementSibling.textContent);"><i class="fas fa-copy"></i> Kopier log</button>
      </div>
      <div class="viewer-active" style="display:none;">
        <div class="viewer-toolbar">
          <span class="viewer-device-label">${deviceName}</span>
          <span class="viewer-stats" style="font-size:0.7rem; color:var(--text-muted); margin-left:auto; min-width:12rem; text-align:right;"></span>
          <span class="conn-type-badge" title="Forbindelsestype: ukendt"><i class="fas fa-circle-question"></i></span>
          <span class="session-timer" style="font-size:0.7rem; color:var(--text-muted); margin-left:0.5rem;" title="Session varighed">00:00</span>
          <div class="viewer-sparkline" title="Bitrate/Latency historie"></div>
          <select class="session-monitor-select" title="Vælg skærm" style="font-size:0.75rem; padding:0.2rem 0.4rem; background:var(--background-secondary); border:1px solid var(--border); border-radius:4px; color:var(--text); display:none;">
            <option value="0">Skærm 1</option>
          </select>
          <div class="quality-presets" style="display:flex; gap:2px; margin-right:0.25rem;">
            <button class="btn btn-sm quality-preset-btn" data-preset="low" title="Lav kvalitet (15 FPS, 45%)">Lav</button>
            <button class="btn btn-sm quality-preset-btn active" data-preset="medium" title="Mellem kvalitet (25 FPS, 70%)">Mellem</button>
            <button class="btn btn-sm quality-preset-btn" data-preset="high" title="Høj kvalitet (30 FPS, 95%)">Høj</button>
          </div>
          <button class="btn btn-sm btn-icon session-files-btn" title="Filoverførsel"><i class="fas fa-folder-open"></i></button>
          <button class="btn btn-sm btn-icon session-details-btn" title="Forbindelsesdetaljer"><i class="fas fa-info-circle"></i></button>
          <button class="btn btn-sm btn-icon session-update-btn" title="Opdater agent"><i class="fas fa-sync-alt"></i></button>
          <button class="btn btn-sm btn-icon session-screenshot-btn" title="Tag screenshot"><i class="fas fa-camera"></i></button>
          <button class="btn btn-sm btn-icon session-terminal-btn" title="Terminal"><i class="fas fa-terminal"></i></button>
          <button class="btn btn-sm btn-icon session-codec-btn" title="Skift codec (H.264 ⇄ JPEG)"><i class="fas fa-film"></i></button>
          <button class="btn btn-sm btn-icon session-log-btn" title="Vis session-log"><i class="fas fa-file-alt"></i></button>
          <button class="btn btn-sm btn-icon session-chat-btn" title="Chat"><i class="fas fa-comment"></i></button>
          <button class="btn btn-sm btn-icon session-fullscreen-btn" title="Fuldskærm"><i class="fas fa-expand"></i></button>
          <button class="btn btn-sm btn-danger session-disconnect-btn">Afbryd</button>
        </div>
        <div class="connection-details">
          <h4><i class="fas fa-signal"></i> Forbindelsesdetaljer</h4>
          <div class="detail-row"><span class="detail-label">Latency</span><span class="detail-value detail-rtt">—</span></div>
          <div class="detail-row"><span class="detail-label">FPS</span><span class="detail-value detail-fps">—</span></div>
          <div class="detail-row"><span class="detail-label">Båndbredde</span><span class="detail-value detail-bw">—</span></div>
          <div class="detail-row"><span class="detail-label">Opløsning</span><span class="detail-value detail-res">—</span></div>
          <div class="detail-row"><span class="detail-label">Forbindelsestype</span><span class="detail-value detail-type">—</span></div>
          <div class="detail-row"><span class="detail-label">Agent version</span><span class="detail-value detail-agent-ver">—</span></div>
        </div>
        <div class="chat-panel" style="display:none; position:absolute; right:0; top:2.5rem; bottom:0; width:280px; z-index:10; background:var(--background-secondary); border-left:1px solid var(--border); flex-direction:column;">
          <div class="chat-header" style="padding:0.5rem 0.75rem; border-bottom:1px solid var(--border); font-size:0.8rem; font-weight:600; display:flex; justify-content:space-between; align-items:center;">
            <span>Chat</span>
            <button class="btn btn-sm btn-icon chat-close-btn" style="font-size:0.7rem;">&times;</button>
          </div>
          <div class="chat-messages" style="flex:1; overflow-y:auto; padding:0.5rem; font-size:0.75rem;"></div>
          <div class="chat-input-row" style="padding:0.5rem; border-top:1px solid var(--border); display:flex; gap:0.25rem;">
            <input type="text" class="chat-input" placeholder="Skriv besked..." style="flex:1; padding:0.3rem 0.5rem; background:var(--surface); border:1px solid var(--border); border-radius:4px; color:var(--text); font-size:0.75rem;">
            <button class="btn btn-sm btn-primary chat-send-btn" style="padding:0.3rem 0.5rem; font-size:0.75rem;">Send</button>
          </div>
        </div>
        <div class="viewer-screen">
          <video autoplay playsinline muted></video>
          <canvas tabindex="0"></canvas>
        </div>
        <div class="terminal-container" style="display:none;">
          <div class="terminal-el" style="flex:1;"></div>
        </div>
      </div>
    `;
    container.appendChild(this.wrapper);

    this.connectingEl = this.wrapper.querySelector('.viewer-connecting');
    this.activeEl = this.wrapper.querySelector('.viewer-active');
    this.videoEl = this.wrapper.querySelector('video');
    this.canvasEl = this.wrapper.querySelector('canvas');
    this.setupChat();
  }

  setConnectStatus(msg) {
    if (!this._connectLog) this._connectLog = [];
    const ts = new Date().toLocaleTimeString();
    this._connectLog.push(`[${ts}] ${msg}`);
    console.log(`[CONNECT] ${msg}`);
    // Log to Go backend so it shows in controller log
    try { window.go?.main?.App?.LogFromFrontend?.('info', msg); } catch(e) {}
    const p = this.connectingEl?.querySelector('p');
    if (!p) return;
    p.innerText = this._connectLog.join('\n');
    p.scrollTop = p.scrollHeight;
    const copyBtn = this.connectingEl?.querySelector('.connecting-copy-btn');
    if (copyBtn) copyBtn.style.display = '';
  }

  setupChat() {
    this.chatChannel = null;
    const chatBtn = this.wrapper.querySelector('.session-chat-btn');
    const chatPanel = this.wrapper.querySelector('.chat-panel');
    const chatClose = this.wrapper.querySelector('.chat-close-btn');
    const chatInput = this.wrapper.querySelector('.chat-input');
    const chatSend = this.wrapper.querySelector('.chat-send-btn');
    const chatMessages = this.wrapper.querySelector('.chat-messages');

    if (chatBtn) {
      chatBtn.addEventListener('click', () => {
        const panel = this.wrapper.querySelector('.chat-panel');
        if (panel) {
          const isVisible = panel.style.display === 'flex';
          panel.style.display = isVisible ? 'none' : 'flex';
        }
      });
    }
    if (chatClose) {
      chatClose.addEventListener('click', () => {
        const panel = this.wrapper.querySelector('.chat-panel');
        if (panel) panel.style.display = 'none';
      });
    }

    const sendMessage = () => {
      const text = chatInput?.value?.trim();
      if (!text) return;
      this.sendChatMessage(text);
      this.addChatMessage('Du', text);
      if (chatInput) chatInput.value = '';
    };

    if (chatSend) chatSend.addEventListener('click', sendMessage);
    if (chatInput) chatInput.addEventListener('keydown', (e) => {
      if (e.key === 'Enter') sendMessage();
    });
  }

  sendChatMessage(text) {
    if (!this.chatChannel || this.chatChannel.readyState !== 'open') {
      // Fallback: send via control data channel
      if (this.dataChannel && this.dataChannel.readyState === 'open') {
        this.dataChannel.send(JSON.stringify({ type: 'chat', text, sender: 'controller' }));
      }
      return;
    }
    this.chatChannel.send(JSON.stringify({ type: 'chat', text, sender: 'controller' }));
  }

  addChatMessage(sender, text) {
    const chatMessages = this.wrapper.querySelector('.chat-messages');
    if (!chatMessages) return;
    const msg = document.createElement('div');
    msg.style.cssText = 'margin-bottom:0.3rem; padding:0.2rem 0.4rem; border-radius:4px; background:rgba(255,255,255,0.05);';
    const time = new Date().toLocaleTimeString('da-DK', { hour: '2-digit', minute: '2-digit' });
    msg.innerHTML = `<span style="color:var(--primary); font-weight:500;">${sender}</span> <span style="color:var(--text-dim); font-size:0.65rem;">${time}</span><br>${text}`;
    chatMessages.appendChild(msg);
    chatMessages.scrollTop = chatMessages.scrollHeight;
  }

  async connect() {
    try {
      this.setConnectStatus(`Henter forbindelsesconfig...`);
      const config = await window.go.main.App.GetConnectionConfig();
      const cfg = {
        supabase_url: config.supabase_url || config.SupabaseURL,
        anon_key: config.anon_key || config.AnonKey,
        auth_token: config.auth_token || config.AuthToken,
        user_id: config.user_id || config.UserID,
        refresh_token: config.refresh_token || config.RefreshToken,
      };

      this.setConnectStatus(`Initialiserer Supabase...`);
      this.supabase = window.supabase
        ? window.supabase.createClient(cfg.supabase_url, cfg.anon_key, {
            auth: { persistSession: false }
          })
        : null;

      if (!this.supabase) {
        this.setConnectStatus(`<span style="color:red;">FEJL: Supabase JS ikke loaded</span>`);
        showToast('Supabase bibliotek ikke loaded — tjek internetforbindelse', 'error');
        return;
      }

      await this.supabase.auth.setSession({
        access_token: cfg.auth_token,
        refresh_token: cfg.refresh_token
      });

      this.config = cfg;

      this.setConnectStatus(`Henter TURN credentials...`);
      await this.fetchTurnCredentials(cfg);

      this.setConnectStatus(`Opretter session...`);
      await this.createSession(cfg);

      this.setConnectStatus(`Opsætter WebRTC...`);
      await this.setupPeerConnection();

      this.setConnectStatus(`Lytter efter agent...`);
      this.subscribeToSignaling();

      this.setConnectStatus(`Sender tilbud til ${this.deviceName}...`);
      await this.createOffer();

      this.setConnectStatus(`Venter på svar fra ${this.deviceName}...`);

      // Timeout — if not connected after 30s, show error
      setTimeout(() => {
        if (!this.connected && this.connectingEl?.style.display !== 'none') {
          this.setConnectStatus(`<span style="color:orange;">Timeout — agent svarede ikke inden 30s<br>Tjek at agent kører på enheden</span>`);
        }
      }, 30000);

    } catch (err) {
      console.error(`[${this.deviceName}] Connection failed:`, err);
      this.setConnectStatus(`<span style="color:red;">FEJL: ${err.message}</span>`);
      showToast(`Forbindelse til ${this.deviceName} fejlede: ${err.message}`, 'error');
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
        // STUN for P2P + Cloudflare TURN + local coturn as backup
        // Generate coturn temp credentials (HMAC-based, valid 24h)
        const coturnUser = Math.floor(Date.now()/1000 + 86400) + ':remotedesktop';
        const coturnKey = 'HawkeyeTurnSecret2026x';
        const coturnCred = await this.hmacSHA1(coturnKey, coturnUser);
        this.iceConfig = { iceServers: [
          { urls: ['stun:stun.l.google.com:19302', 'stun:stun1.l.google.com:19302'] },
          ...data.iceServers,
          { urls: 'turn:turn.hawkeye123.dk:3478?transport=udp', username: coturnUser, credential: coturnCred },
          { urls: 'turn:turn.hawkeye123.dk:3478?transport=tcp', username: coturnUser, credential: coturnCred }
        ]};
        const hasTurn = JSON.stringify(data.iceServers).includes('turn:');
        this.setConnectStatus(`TURN: ${hasTurn ? 'OK' : 'KUN STUN'} (${data.iceServers.length} servere)`);
        console.log(`[${this.deviceName}] TURN credentials:`, JSON.stringify(data.iceServers).substring(0, 200));
      } else {
        this.setConnectStatus(`<span style="color:orange;">TURN fejlede (${response.status}) — kun STUN</span>`);
      }
    } catch (e) {
      this.setConnectStatus(`<span style="color:orange;">TURN fetch fejl: ${e.message}</span>`);
      console.warn(`[${this.deviceName}] TURN fetch failed:`, e);
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
    // Allow all ICE types — P2P preferred for speed, relay as fallback
    const config = { ...this.iceConfig };
    this.setConnectStatus(`ICE servers: ${config.iceServers?.length || 0}, policy: all`);
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
      const iceState = this.peerConnection?.iceConnectionState;
      console.log(`[${this.deviceName}] Connection: ${state}, ICE: ${iceState}`);
      this.setConnectStatus(`WebRTC: ${state} | ICE: ${iceState}`);
      if (state === 'connected') {
        this.onConnected();
      } else if (state === 'failed') {
        this.setConnectStatus(`<span style="color:red;">WebRTC FEJLET — ICE: ${iceState}<br>TURN relay virkede ikke</span>`);
        this.onDisconnected();
      } else if (state === 'disconnected') {
        this.onDisconnected();
      }
    };

    this.peerConnection.onicegatheringstatechange = () => {
      console.log(`[${this.deviceName}] ICE gathering: ${this.peerConnection?.iceGatheringState}`);
    };

    this.peerConnection.oniceconnectionstatechange = () => {
      const iceState = this.peerConnection?.iceConnectionState;
      console.log(`[${this.deviceName}] ICE connection: ${iceState}`);
      if (iceState === 'checking') {
        this.setConnectStatus(`ICE tjekker forbindelse...`);
      } else if (iceState === 'connected' || iceState === 'completed') {
        this.setConnectStatus(`ICE forbundet!`);
      } else if (iceState === 'failed') {
        this.setConnectStatus(`<span style="color:red;">ICE FEJLET — kan ikke nå agent<br>Firewall eller NAT blokerer</span>`);
      }
    };

    this.peerConnection.ontrack = (event) => {
      console.log(`[${this.deviceName}] Track received:`, event.track.kind);
      if (event.track.kind === 'video') {
        // Don't set usingH264 yet — agent always adds video track but only
        // sends frames when H.264 mode is active. Detect actual H.264 usage
        // by checking if video element receives frames (videoWidth > 0).
        this.videoEl.srcObject = event.streams[0];
        this.videoEl.style.display = '';
        this.canvasEl.style.pointerEvents = 'auto';
        this.canvasEl.style.background = 'transparent';
      } else if (event.track.kind === 'audio') {
        console.log(`[${this.deviceName}] Audio track received — playing`);
        const audio = new Audio();
        audio.srcObject = event.streams[0];
        audio.play().catch(e => console.warn('Audio autoplay blocked:', e));
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
      } else if (dc.label === 'chat') {
        this.chatChannel = dc;
        dc.onmessage = (e) => {
          try {
            const msg = JSON.parse(typeof e.data === 'string' ? e.data : new TextDecoder().decode(e.data));
            if (msg.type === 'chat') this.addChatMessage('Agent', msg.text || '');
          } catch(err) {}
        };
      }
    };

    // Create all data channels the agent expects
    const controlDC = this.peerConnection.createDataChannel('control', { ordered: true });
    controlDC.binaryType = 'arraybuffer';
    // Store reference immediately (like dashboard) — readyState checked before send
    this.dataChannel = controlDC;
    controlDC.onopen = () => {
      console.log(`[${this.deviceName}] Control data channel open — input enabled`);
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

    // Process manager channel
    const processDC = this.peerConnection.createDataChannel('process', { ordered: true });
    processDC.onopen = () => {
      console.log(`[${this.deviceName}] Process data channel open`);
      this.processChannel = processDC;
    };
    processDC.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data);
        window.dispatchEvent(new CustomEvent('process-message', { detail: msg }));
      } catch (e) { /* ignore */ }
    };

    // Chat channel
    const chatDC = this.peerConnection.createDataChannel('chat', { ordered: true });
    chatDC.onopen = () => {
      console.log(`[${this.deviceName}] Chat data channel open`);
      this.chatChannel = chatDC;
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
        } else if (msg.type === 'monitor_list') {
          this.updateMonitorList(msg.monitors || [], msg.active || 0);
        } else if (msg.type === 'update_status') {
          const type = msg.status === 'error' ? 'error' : msg.status === 'up_to_date' ? 'success' : 'info';
          showToast(msg.message || msg.status, type);
        } else if (msg.type === 'chat') {
          this.addChatMessage('Agent', msg.text || msg.message || '');
        } else if (msg.type === 'clipboard_text') {
          // Remote PC copied text — write it to the local OS clipboard,
          // and broadcast to every OTHER connected session so the user can
          // paste it on any of the connected machines (and on the
          // controller itself).
          if (msg.content) {
            ClipboardBroker.spread('text', msg.content, this.id);
          }
        } else if (msg.type === 'clipboard_image') {
          // Remote PC copied an image — decode base64 PNG, write to local
          // clipboard, and forward to every other open session.
          if (msg.content) {
            ClipboardBroker.spread('image', msg.content, this.id);
          }
        }
      } catch (e) { /* not JSON */ }
    }
  }

  // sendClipboardToAgent reads the controller's local OS clipboard and
  // broadcasts it to *every* connected session. Triggered on Ctrl+V over
  // any canvas: the user's intent is "make my local clipboard available
  // for paste here", and broadcasting to all sessions means a copy made
  // on any machine (or the controller itself) becomes pasteable on every
  // connected remote PC.
  async sendClipboardToAgent() {
    try {
      const text = await navigator.clipboard.readText();
      if (text) {
        ClipboardBroker.broadcast('text', text, null);
        return;
      }
    } catch (_) { /* fall through to image */ }
    try {
      const items = await navigator.clipboard.read();
      for (const item of items) {
        if (item.types.includes('image/png')) {
          const blob = await item.getType('image/png');
          const buffer = await blob.arrayBuffer();
          const base64 = btoa(String.fromCharCode(...new Uint8Array(buffer)));
          ClipboardBroker.broadcast('image', base64, null);
          return;
        }
      }
    } catch (err) {
      console.warn('readClipboard failed:', err);
    }
  }

  updateMonitorList(monitors, activeIndex) {
    const select = this.wrapper.querySelector('.session-monitor-select');
    if (!select || monitors.length <= 1) return;

    select.style.display = '';
    select.innerHTML = '';
    monitors.forEach((mon, i) => {
      const opt = document.createElement('option');
      opt.value = i;
      opt.textContent = mon.name || `Skærm ${i + 1}`;
      if (i === activeIndex) opt.selected = true;
      select.appendChild(opt);
    });
  }

  switchMonitor(index) {
    if (!this.dataChannel || this.dataChannel.readyState !== 'open') return;
    this.dataChannel.send(JSON.stringify({ type: 'switch_monitor', index: parseInt(index) }));
  }

  renderFrame(data) {
    const blob = data instanceof Blob ? data : new Blob([data], { type: 'image/jpeg' });
    if (blob.size < 100) return;
    // Count frames for FPS calculation
    if (!this._frameCount) this._frameCount = 0;
    this._frameCount++;

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
        this._ensureCanvasAutoFit(canvas);
        this._fitCanvasToContainer(canvas);
      }
      ctx.drawImage(img, 0, 0);
      URL.revokeObjectURL(img.src);
    };
    img.onerror = () => URL.revokeObjectURL(img.src);
    img.src = URL.createObjectURL(blob);
  }

  // ===================================================================
  // Canvas letterbox auto-fit
  //
  // CSS object-fit: contain på <canvas> er upålideligt på tværs af
  // browsere/Wails-WebView/HiDPI. I stedet sætter vi canvas' inline
  // CSS-dimensioner eksplicit ud fra parent-box og canvas-buffer-aspect.
  // ===================================================================
  _fitCanvasToContainer(canvas) {
    if (!canvas || !canvas.parentElement) return;
    const intW = canvas.width, intH = canvas.height;
    if (!intW || !intH) return;
    const parent = canvas.parentElement;
    const cw = parent.clientWidth, ch = parent.clientHeight;
    if (!cw || !ch) return;
    const scale = Math.min(cw / intW, ch / intH);
    const w = Math.floor(intW * scale);
    const h = Math.floor(intH * scale);
    canvas.style.width  = w + 'px';
    canvas.style.height = h + 'px';
    canvas.style.left   = Math.floor((cw - w) / 2) + 'px';
    canvas.style.top    = Math.floor((ch - h) / 2) + 'px';
    canvas.style.right  = 'auto';
    canvas.style.bottom = 'auto';
  }

  _ensureCanvasAutoFit(canvas) {
    if (!canvas || canvas._autoFitWired) return;
    canvas._autoFitWired = true;
    const fit = () => this._fitCanvasToContainer(canvas);
    fit();
    if (typeof ResizeObserver === 'function' && canvas.parentElement) {
      const ro = new ResizeObserver(() => fit());
      ro.observe(canvas.parentElement);
      canvas._fitObserver = ro;
    }
    if (!window._viewerFitWired) {
      window._viewerFitWired = true;
      window.addEventListener('resize', () => {
        document.querySelectorAll('.viewer-screen canvas').forEach(c => {
          if (c._fitObserver) {
            // observer fyrer; gør det også manuelt for sikkerheds skyld
          }
          // fitCanvasToContainer er en ren funktion — tilgå direkte
          const intW = c.width, intH = c.height;
          if (!intW || !intH || !c.parentElement) return;
          const cw = c.parentElement.clientWidth, ch = c.parentElement.clientHeight;
          if (!cw || !ch) return;
          const scale = Math.min(cw / intW, ch / intH);
          const w = Math.floor(intW * scale), h = Math.floor(intH * scale);
          c.style.width = w + 'px';
          c.style.height = h + 'px';
          c.style.left = Math.floor((cw - w) / 2) + 'px';
          c.style.top  = Math.floor((ch - h) / 2) + 'px';
        });
      });
    }
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
    if (!this.supabase) {
      console.error('sendSignal: supabase is null!');
      return;
    }

    let signalPayload;
    if (payload.type === 'ice') {
      signalPayload = payload.candidate;
    } else {
      signalPayload = { type: payload.type, sdp: payload.sdp };
    }

    const { error } = await this.supabase.from('session_signaling').insert({
      session_id: payload.session_id,
      from_side: payload.from,
      msg_type: payload.type,
      payload: signalPayload
    });

    if (error) {
      console.error(`sendSignal error (${payload.type}):`, error);
      this.setConnectStatus(`<span style="color:red;">Signaling fejl: ${error.message}</span>`);
    }
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

    let pollCount = 0;
    this.pollingInterval = setInterval(async () => {
      try {
        pollCount++;
        // Use neq instead of in() for compatibility
        const { data, error } = await this.supabase
          .from('session_signaling')
          .select('*')
          .eq('session_id', sessionId)
          .neq('from_side', 'dashboard')
          .order('created_at', { ascending: true });

        if (error) {
          this.setConnectStatus(`Polling fejl: ${error.message}`);
          return;
        }

        if (pollCount <= 5) {
          const cs = this.peerConnection?.connectionState || '?';
          const is = this.peerConnection?.iceConnectionState || '?';
          this.setConnectStatus(`Poll #${pollCount}: ${data ? data.length : 0} sig | conn=${cs} ice=${is}`);
        }

        if (data) {
          for (const signal of data) {
            if (!this.processedSignalIds.has(signal.id)) {
              this.setConnectStatus(`Signal: ${signal.msg_type} fra ${signal.from_side}`);
              await this.handleSignal(signal);
            }
          }
        }
      } catch (e) {
        this.setConnectStatus(`Poll fejl: ${e.message}`);
      }
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
          this.setConnectStatus(`Modtog svar — opsætter forbindelse...`);
          console.log(`[${this.deviceName}] Got answer, signalingState=${pc.signalingState}`);
          if (pc.signalingState === 'have-local-offer') {
            await pc.setRemoteDescription(new RTCSessionDescription(signal.payload));
            console.log(`[${this.deviceName}] Remote description set, flushing ${this.pendingIceCandidates.length} ICE candidates`);
            for (const c of this.pendingIceCandidates) {
              await pc.addIceCandidate(new RTCIceCandidate(c));
            }
            this.pendingIceCandidates = [];
            this.setConnectStatus(`WebRTC forbinder...`);
          } else {
            console.warn(`[${this.deviceName}] Unexpected signalingState for answer: ${pc.signalingState}`);
          }
          break;

        case 'ice':
          if (signal.payload && signal.payload.candidate) {
            if (!pc.remoteDescription) {
              this.pendingIceCandidates.push(signal.payload);
              this.setConnectStatus(`ICE buffered (venter på answer)`);
            } else {
              try {
                await pc.addIceCandidate(new RTCIceCandidate(signal.payload));
                this.setConnectStatus(`ICE tilføjet: ${signal.payload.candidate.substring(0, 50)}...`);
              } catch (iceErr) {
                this.setConnectStatus(`ICE fejl: ${iceErr.message}`);
              }
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

    // Handle successful reconnect
    if (this.reconnectState === 'reconnecting') {
      this.reconnectState = 'idle';
      this.reconnectAttempt = 0;
      this.reconnectStartedAt = null;
      showToast(`Forbindelse til ${this.deviceName} genoprettet!`, 'success');
      console.log(`[${this.deviceName}] Reconnect successful`);
    } else {
      showToast(`Forbundet til ${this.deviceName}`, 'success');
    }

    this.sessionStartTime = Date.now();
    this.sessionTimerInterval = setInterval(() => {
      const elapsed = Math.floor((Date.now() - this.sessionStartTime) / 1000);
      const m = Math.floor(elapsed / 60).toString().padStart(2, '0');
      const s = (elapsed % 60).toString().padStart(2, '0');
      const h = Math.floor(elapsed / 3600);
      const timerEl = this.wrapper.querySelector('.session-timer');
      if (timerEl) timerEl.textContent = h > 0 ? `${h}:${m}:${s}` : `${m}:${s}`;
    }, 1000);

    this.setupInput();
    this.startStats();
    this.sendSettingsToAgent();
    this.enableH264Mode();

    // Wire file channel to FileTransfer module
    if (this.fileChannel && window.FileTransfer) {
      window.FileTransfer.setChannel(this.fileChannel);
    }

    // Notify session manager
    if (window.SessionManager) {
      window.SessionManager.onSessionConnected(this.id);
    }
  }

  // Connection statistics — polls getStats() every second
  startStats() {
    this.prevBytesReceived = 0;
    this.prevTimestamp = 0;
    if (this.statsInterval) clearInterval(this.statsInterval);
    this.statsInterval = setInterval(() => this.updateStats(), 1000);
  }

  async updateStats() {
    if (!this.peerConnection || this.peerConnection.connectionState !== 'connected') return;

    try {
      const stats = await this.peerConnection.getStats();
      let rtt = null;
      let fps = null;
      let bytesReceived = 0;
      let timestamp = 0;
      let packetsLost = 0;
      let packetsReceived = 0;

      stats.forEach(report => {
        // RTT from active candidate pair
        if (report.type === 'candidate-pair' && report.state === 'succeeded' && report.currentRoundTripTime != null) {
          rtt = Math.round(report.currentRoundTripTime * 1000);
        }
        // FPS + packet loss from inbound video track
        if (report.type === 'inbound-rtp' && report.kind === 'video') {
          if (report.framesPerSecond != null) {
            fps = Math.round(report.framesPerSecond);
          }
          if (report.bytesReceived != null) {
            bytesReceived = report.bytesReceived;
            timestamp = report.timestamp;
          }
          if (report.packetsLost != null) packetsLost += report.packetsLost;
          if (report.packetsReceived != null) packetsReceived += report.packetsReceived;
        }
        // Also check transport-level for total bandwidth (includes data channels)
        if (report.type === 'transport' && report.bytesReceived != null) {
          if (report.bytesReceived > bytesReceived) {
            bytesReceived = report.bytesReceived;
            timestamp = report.timestamp;
          }
        }
      });

      // Calculate packet loss % (delta since last sample)
      let lossPercent = null;
      if (this.prevPacketsReceived > 0 || this.prevPacketsLost > 0) {
        const deltaLost = Math.max(0, packetsLost - this.prevPacketsLost);
        const deltaReceived = Math.max(0, packetsReceived - this.prevPacketsReceived);
        const total = deltaLost + deltaReceived;
        if (total > 0) lossPercent = (deltaLost / total) * 100;
      }
      this.prevPacketsLost = packetsLost;
      this.prevPacketsReceived = packetsReceived;

      // Calculate bandwidth
      let bwText = '';
      if (this.prevTimestamp > 0 && timestamp > this.prevTimestamp) {
        const deltaBytes = bytesReceived - this.prevBytesReceived;
        const deltaSec = (timestamp - this.prevTimestamp) / 1000;
        const mbps = (deltaBytes * 8) / (deltaSec * 1000000);
        bwText = mbps >= 1 ? `${mbps.toFixed(1)} Mbit/s` : `${(mbps * 1000).toFixed(0)} kbit/s`;
      }
      this.prevBytesReceived = bytesReceived;
      this.prevTimestamp = timestamp;

      // Calculate FPS from data channel frames (since no video track)
      if (fps == null && this._frameCount) {
        if (!this._lastFrameCount) this._lastFrameCount = 0;
        fps = this._frameCount - this._lastFrameCount;
        this._lastFrameCount = this._frameCount;
      }

      // Build display string
      const parts = [];
      if (rtt != null) parts.push(`${rtt}ms`);
      if (fps != null) parts.push(`${fps}fps`);
      if (bwText) parts.push(bwText);
      // Detect actual codec: H.264 if video element has frames, JPEG if canvas gets data channel frames
      const wasH264 = this.usingH264;
      this.usingH264 = !!(this.videoEl && this.videoEl.videoWidth > 0 && this.videoEl.videoHeight > 0);
      parts.push(this.usingH264 ? 'H.264' : 'JPEG');

      // Skjul/vis canvas afhængigt af mode. I JPEG-tile-mode tegner vi på
      // canvas (z-index 1, over video). I H.264-mode kommer frames via
      // video-track, og canvas'en sidder bare med frozen sidste-JPEG-content
      // og blokerer for video'en — derfor BLACK SCREEN. Skjul canvas så
      // video'en kommer igennem.
      if (this.usingH264 !== wasH264 && this.canvasEl) {
        this.canvasEl.style.display = this.usingH264 ? 'none' : '';
        if (this.videoEl) this.videoEl.style.display = this.usingH264 ? '' : 'none';
        console.log(`[${this.deviceName}] Codec switch → ${this.usingH264 ? 'H.264 (canvas hidden)' : 'JPEG (canvas shown)'}`);
        if (this._updateCodecBtn) this._updateCodecBtn();
      }

      const statsEl = this.wrapper.querySelector('.viewer-stats');
      if (statsEl) {
        statsEl.textContent = parts.length > 0 ? parts.join(' | ') : '';
      }

      // Update detail panel
      const panel = this.wrapper.querySelector('.connection-details');
      if (panel) {
        const set = (cls, val) => { const el = panel.querySelector(cls); if (el) el.textContent = val; };
        set('.detail-rtt', rtt != null ? `${rtt} ms` : '—');
        set('.detail-fps', fps != null ? `${fps}` : '—');
        set('.detail-bw', bwText || '—');
        // Resolution from canvas or video
        const w = this.canvasEl?.width || this.videoEl?.videoWidth || 0;
        const h = this.canvasEl?.height || this.videoEl?.videoHeight || 0;
        set('.detail-res', w > 0 ? `${w}x${h}` : '—');
        // Connection type from candidate pair
        let connType = '—';
        stats.forEach(report => {
          if (report.type === 'candidate-pair' && report.state === 'succeeded' && report.nominated) {
            stats.forEach(r => {
              if (r.type === 'local-candidate' && r.id === report.localCandidateId) {
                if (r.candidateType === 'relay') connType = 'TURN (Relay)';
                else if (r.candidateType === 'srflx') connType = 'P2P (STUN)';
                else if (r.candidateType === 'host') connType = 'P2P (Direkte)';
              }
            });
          }
        });
        set('.detail-type', connType);
        set('.detail-agent-ver', this.agentVersion || '—');
        // Update connection type badge in toolbar
        const badge = this.wrapper.querySelector('.conn-type-badge');
        if (badge && connType !== '—') {
          if (connType.includes('Direkte')) {
            badge.innerHTML = '<i class="fas fa-bolt"></i>';
            badge.style.color = '#22c55e';
            badge.title = 'P2P Direkte — lavest mulig latency';
          } else if (connType.includes('STUN')) {
            badge.innerHTML = '<i class="fas fa-arrow-right-arrow-left"></i>';
            badge.style.color = '#3b82f6';
            badge.title = 'P2P via STUN — lav latency';
          } else if (connType.includes('Relay')) {
            badge.innerHTML = '<i class="fas fa-cloud"></i>';
            badge.style.color = '#f59e0b';
            badge.title = 'TURN Relay — højere latency (trafik via server)';
          }
        }
      }

      // Track history for sparkline (max 60 entries)
      const mbpsVal = (this.prevTimestamp > 0 && bwText)
        ? parseFloat(bwText) || 0
        : 0;
      this.statsHistory.bw.push(mbpsVal);
      if (this.statsHistory.bw.length > 60) this.statsHistory.bw.shift();
      const rttVal = rtt != null ? rtt : (this.statsHistory.rtt.length > 0 ? this.statsHistory.rtt[this.statsHistory.rtt.length - 1] : 0);
      this.statsHistory.rtt.push(rttVal);
      if (this.statsHistory.rtt.length > 60) this.statsHistory.rtt.shift();
      if (fps != null) this.statsHistory.fps.push(fps);
      else this.statsHistory.fps.push(this.statsHistory.fps.length > 0 ? this.statsHistory.fps[this.statsHistory.fps.length - 1] : 0);
      if (this.statsHistory.fps.length > 60) this.statsHistory.fps.shift();
      this.statsHistory.loss.push(lossPercent != null ? lossPercent : 0);
      if (this.statsHistory.loss.length > 60) this.statsHistory.loss.shift();

      // Determine connection quality tier
      let tier = 'good';
      if (rttVal > 200 || (lossPercent != null && lossPercent > 5)) tier = 'poor';
      else if (rttVal > 100 || (lossPercent != null && lossPercent > 1)) tier = 'fair';
      this.currentQuality = { rtt: rttVal, loss: lossPercent, bw: mbpsVal, tier };

      this.renderSparkline();
    } catch (e) {
      // getStats() can fail during teardown — ignore
    }
  }

  // Render sparkline SVG for connection quality (RTT history) with color tier
  renderSparkline() {
    const container = this.wrapper.querySelector('.viewer-sparkline');
    if (!container) return;
    const data = this.statsHistory.rtt;
    if (data.length < 2) { container.innerHTML = ''; return; }

    const w = 120, h = 24, pad = 1;
    // Dynamisk Y-skala: mindst 100ms, mest max af data * 1.2
    const max = Math.max(100, Math.max(...data) * 1.2);
    const xStep = (w - pad * 2) / (data.length - 1);

    const points = data.map((v, i) => {
      const x = pad + i * xStep;
      const y = pad + (h - pad * 2) * (1 - Math.min(v, max) / max);
      return `${x.toFixed(1)},${y.toFixed(1)}`;
    });

    const fillPoints = `${pad},${h - pad} ${points.join(' ')} ${(pad + (data.length - 1) * xStep).toFixed(1)},${h - pad}`;

    // Quality tier → color class
    const tier = this.currentQuality.tier || 'good';
    container.setAttribute('data-tier', tier);

    // Format tooltip
    const q = this.currentQuality;
    const avgRtt = Math.round(data.reduce((a, b) => a + b, 0) / data.length);
    const avgLoss = this.statsHistory.loss.length > 0
      ? (this.statsHistory.loss.reduce((a, b) => a + b, 0) / this.statsHistory.loss.length).toFixed(1)
      : '—';
    const tooltipLines = [
      `RTT: ${q.rtt != null ? q.rtt + 'ms' : '—'} (avg ${avgRtt}ms)`,
      q.loss != null ? `Loss: ${q.loss.toFixed(2)}% (avg ${avgLoss}%)` : 'Loss: —',
      q.bw > 0 ? `Bitrate: ${q.bw.toFixed(1)} Mbit/s` : null,
      `Kvalitet: ${tier === 'good' ? 'God' : tier === 'fair' ? 'OK' : 'Dårlig'}`,
    ].filter(Boolean).join('\n');
    container.setAttribute('title', tooltipLines);

    container.innerHTML = `<svg width="${w}" height="${h}" viewBox="0 0 ${w} ${h}" aria-label="RTT historie: ${tooltipLines.replace(/\n/g, ', ')}">
      <polygon class="sparkline-fill" points="${fillPoints}" />
      <polyline points="${points.join(' ')}" />
      ${q.loss > 0.5 ? `<circle cx="${(w - 4).toFixed(1)}" cy="4" r="3" class="sparkline-loss-dot" />` : ''}
    </svg>`;
  }

  // Send current settings to agent via data channel
  async sendSettingsToAgent() {
    try {
      const settings = await window.go.main.App.GetSettings();
      const msg = {
        type: 'set_stream_params',
        max_quality: settings.video_quality,
        max_fps: settings.target_fps,
        max_scale: 1.0
      };

      // Wait for data channel to be open (may still be opening)
      const dc = this.dataChannel;
      if (!dc) return;

      const send = () => {
        if (dc.readyState === 'open') {
          dc.send(JSON.stringify(msg));
          console.log(`[${this.deviceName}] Sent stream params to agent:`, msg);
        }
      };

      if (dc.readyState === 'open') {
        send();
      } else {
        dc.addEventListener('open', send, { once: true });
      }
    } catch (e) {
      console.error(`[${this.deviceName}] Failed to send settings to agent:`, e);
    }
  }

  onDisconnected() {
    if (!this.connected) return;
    this.connected = false;

    // Don't auto-reconnect if user manually disconnected or was kicked
    if (this.manualDisconnect) return;

    // Start auto-reconnect if not already in progress
    if (this.reconnectState === 'idle' && this.sessionData) {
      console.log(`[${this.deviceName}] Connection lost — starting auto-reconnect`);
      showToast(`Forbindelse til ${this.deviceName} tabt — genopretter...`, 'warning');
      this.reconnectState = 'reconnecting';
      this.reconnectStartedAt = Date.now();
      this.reconnectAttempt = 0;

      // Show connecting overlay with reconnect status
      this.activeEl.style.display = 'none';
      this.connectingEl.style.display = 'flex';
      const statusP = this.connectingEl.querySelector('p');
      if (statusP) {
        statusP.innerHTML = `Genopretter forbindelse til <span class="connecting-name">${this.deviceName}</span>... <br><small>Forsøg 1/8</small>`;
      }

      // Update tab dot to show reconnecting
      if (window.SessionManager) {
        const tab = window.SessionManager.getTabBar().querySelector(`[data-session-id="${this.id}"]`);
        if (tab) {
          const dot = tab.querySelector('.session-tab-dot');
          if (dot) { dot.classList.remove('connected'); dot.classList.add('connecting'); }
        }
      }

      this.attemptReconnect();
    }
  }

  async attemptReconnect() {
    const RECONNECT_MAX_ATTEMPTS = 8;
    const RECONNECT_BACKOFF = [2000, 4000, 8000, 12000, 16000, 24000, 30000, 30000]; // ms, start 2s, max 30s

    if (this.reconnectState !== 'reconnecting') return;

    // Check max attempts
    if (this.reconnectAttempt >= RECONNECT_MAX_ATTEMPTS) {
      console.log(`[${this.deviceName}] Reconnect gave up after ${RECONNECT_MAX_ATTEMPTS} attempts`);
      this.reconnectState = 'gave_up';
      showToast(`Kunne ikke genoprette forbindelse til ${this.deviceName} efter ${RECONNECT_MAX_ATTEMPTS} forsøg.`, 'error');
      this.disconnect();
      return;
    }

    this.reconnectAttempt++;
    const delay = RECONNECT_BACKOFF[Math.min(this.reconnectAttempt - 1, RECONNECT_BACKOFF.length - 1)];

    console.log(`[${this.deviceName}] Reconnect attempt ${this.reconnectAttempt}/${RECONNECT_MAX_ATTEMPTS} (delay: ${delay}ms)`);

    // Update overlay text
    const statusP = this.connectingEl.querySelector('p');
    if (statusP) {
      statusP.innerHTML = `Genopretter forbindelse til <span class="connecting-name">${this.deviceName}</span>... <br><small>Forsøg ${this.reconnectAttempt}/${RECONNECT_MAX_ATTEMPTS}</small>`;
    }

    // Wait for backoff delay
    await new Promise(resolve => {
      this.reconnectTimer = setTimeout(resolve, delay);
    });

    // Check if reconnect was cancelled during wait
    if (this.reconnectState !== 'reconnecting') {
      console.log(`[${this.deviceName}] Reconnect cancelled during backoff`);
      return;
    }

    try {
      // Clean up existing WebRTC without removing DOM
      this.cleanupConnection();

      // Get fresh connection config (refreshes auth token)
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

      // Fetch fresh TURN credentials
      await this.fetchTurnCredentials(cfg);

      // Create new session
      await this.createSession(cfg);

      // Setup new peer connection
      await this.setupPeerConnection();

      // Subscribe to signaling
      this.subscribeToSignaling();

      // Create offer
      await this.createOffer();

      // Wait for connection (max 15s)
      const connected = await this.waitForConnection(15000);
      if (connected) {
        console.log(`[${this.deviceName}] Reconnect succeeded on attempt ${this.reconnectAttempt}`);
        // onConnected() handler will reset reconnect state
        return;
      }

      // Timed out — try again
      console.log(`[${this.deviceName}] Reconnect attempt ${this.reconnectAttempt} timed out`);
      if (this.reconnectState === 'reconnecting') {
        this.attemptReconnect();
      }

    } catch (err) {
      console.error(`[${this.deviceName}] Reconnect attempt ${this.reconnectAttempt} failed:`, err);
      if (this.reconnectState === 'reconnecting') {
        this.attemptReconnect();
      }
    }
  }

  waitForConnection(timeout) {
    return new Promise(resolve => {
      const start = Date.now();
      const check = () => {
        if (!this.peerConnection) {
          resolve(false);
          return;
        }
        if (this.peerConnection.connectionState === 'connected') {
          resolve(true);
          return;
        }
        if (Date.now() - start > timeout) {
          resolve(false);
          return;
        }
        setTimeout(check, 500);
      };
      check();
    });
  }

  async hmacSHA1(key, message) {
    const enc = new TextEncoder();
    const cryptoKey = await crypto.subtle.importKey('raw', enc.encode(key), { name: 'HMAC', hash: 'SHA-1' }, false, ['sign']);
    const sig = await crypto.subtle.sign('HMAC', cryptoKey, enc.encode(message));
    return btoa(String.fromCharCode(...new Uint8Array(sig)));
  }

  cleanupConnection() {
    if (this.sessionTimerInterval) { clearInterval(this.sessionTimerInterval); this.sessionTimerInterval = null; }
    if (this.statsInterval) { clearInterval(this.statsInterval); this.statsInterval = null; }
    if (this.pollingInterval) { clearInterval(this.pollingInterval); this.pollingInterval = null; }
    if (this.signalingChannel && this.supabase) { this.supabase.removeChannel(this.signalingChannel); this.signalingChannel = null; }
    if (this.peerConnection) { this.peerConnection.close(); this.peerConnection = null; }
    this.dataChannel = null;
    this.fileChannel = null;
    this.videoChannel = null;
    this.frameChunks = {};
    this.usingH264 = false;
    this.processedSignalIds.clear();
    this.pendingIceCandidates = [];
    this.stopTerminal();
  }

  cancelReconnect() {
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    this.reconnectState = 'idle';
    this.reconnectAttempt = 0;
    this.reconnectStartedAt = null;
    console.log(`[${this.deviceName}] Reconnect cancelled`);
  }

  toggleTerminal() {
    const termContainer = this.wrapper.querySelector('.terminal-container');
    if (!termContainer) return;

    if (termContainer.style.display === 'none' || !termContainer.style.display) {
      termContainer.style.display = 'flex';
      this.startTerminal();
    } else {
      termContainer.style.display = 'none';
      this.stopTerminal();
    }
  }

  startTerminal() {
    if (this.xterm) return; // Already running

    const termEl = this.wrapper.querySelector('.terminal-el');
    if (!termEl) return;

    // Create xterm instance
    this.xterm = new window.Terminal({
      cursorBlink: true,
      fontSize: 14,
      fontFamily: 'Consolas, "Courier New", monospace',
      theme: {
        background: '#0d1117',
        foreground: '#c9d1d9',
        cursor: '#58a6ff',
      }
    });

    const fitAddon = new window.FitAddon.FitAddon();
    this.xterm.loadAddon(fitAddon);
    this.xterm.open(termEl);
    fitAddon.fit();

    // Create terminal data channel
    this.terminalChannel = this.peerConnection.createDataChannel('terminal', { ordered: true });
    this.terminalChannel.onopen = () => {
      // Send start command
      this.terminalChannel.send(JSON.stringify({ type: 'start' }));
      this.xterm.writeln('\x1b[32mTerminal forbundet.\x1b[0m\r\n');
    };
    this.terminalChannel.onmessage = (e) => {
      try {
        const msg = JSON.parse(e.data);
        if (msg.type === 'output') {
          this.xterm.write(msg.data);
        } else if (msg.type === 'error') {
          this.xterm.writeln('\x1b[31mFejl: ' + msg.data + '\x1b[0m');
        }
      } catch (err) {
        // Raw data fallback
        this.xterm.write(e.data);
      }
    };

    // Send input to terminal channel
    this.xterm.onData((data) => {
      if (this.terminalChannel && this.terminalChannel.readyState === 'open') {
        this.terminalChannel.send(JSON.stringify({ type: 'input', data: data }));
      }
    });

    // Resize handler
    this._termResizeObserver = new ResizeObserver(() => fitAddon.fit());
    this._termResizeObserver.observe(termEl);
  }

  stopTerminal() {
    if (this.terminalChannel) {
      try {
        this.terminalChannel.send(JSON.stringify({ type: 'close' }));
        this.terminalChannel.close();
      } catch (e) {}
      this.terminalChannel = null;
    }
    if (this.xterm) {
      this.xterm.dispose();
      this.xterm = null;
    }
    if (this._termResizeObserver) {
      this._termResizeObserver.disconnect();
      this._termResizeObserver = null;
    }
  }

  disconnect() {
    // Mark as manual disconnect to prevent auto-reconnect triggering
    this.manualDisconnect = true;

    // Cancel any in-progress reconnect
    this.cancelReconnect();

    // Clean up connection resources
    this.cleanupConnection();

    this.connected = false;

    // Exit fullscreen if active
    if (this.isFullscreen) {
      this.isFullscreen = false;
      document.body.classList.remove('viewer-fullscreen');
      try { window.go.main.App.ToggleFullscreen(); } catch (e) {}
      const hint = document.querySelector('.fullscreen-exit-hint');
      if (hint) hint.classList.add('hidden');
    }

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

    canvas.tabIndex = 0;
    canvas.style.outline = 'none';
    canvas.addEventListener('click', () => canvas.focus());
    canvas.addEventListener('contextmenu', (e) => { e.preventDefault(); e.stopPropagation(); });
    canvas.addEventListener('mousemove', (e) => this.sendMouseEvent('mousemove', e));
    canvas.addEventListener('mousedown', (e) => { e.preventDefault(); this.sendMouseEvent('mousedown', e); });
    canvas.addEventListener('mouseup', (e) => { e.preventDefault(); this.sendMouseEvent('mouseup', e); });
    canvas.addEventListener('wheel', (e) => { e.preventDefault(); this.sendWheelEvent(e); }, { passive: false });
    canvas.addEventListener('keydown', (e) => this.sendKeyEvent('keydown', e));
    canvas.addEventListener('keyup', (e) => { e.preventDefault(); this.sendKeyEvent('keyup', e); });

    this.wrapper.querySelector('.session-disconnect-btn').addEventListener('click', () => { this.manualDisconnect = true; this.disconnect(); });
    this.wrapper.querySelector('.session-fullscreen-btn').addEventListener('click', () => this.toggleFullscreen());
    this.wrapper.querySelector('.session-terminal-btn').addEventListener('click', () => this.toggleTerminal());
    this.wrapper.querySelector('.session-files-btn').addEventListener('click', () => {
      if (window.FileTransfer) {
        if (this.fileChannel) window.FileTransfer.setChannel(this.fileChannel);
        window.FileTransfer.open();
      }
    });
    this.wrapper.querySelector('.session-monitor-select').addEventListener('change', (e) => {
      this.switchMonitor(e.target.value);
    });
    this.wrapper.querySelector('.session-update-btn').addEventListener('click', () => {
      this.forceUpdateAgent();
    });
    this.wrapper.querySelector('.session-details-btn').addEventListener('click', () => {
      const panel = this.wrapper.querySelector('.connection-details');
      if (panel) panel.classList.toggle('visible');
    });
    this.wrapper.querySelector('.session-screenshot-btn').addEventListener('click', () => this.takeScreenshot());
    const logBtn = this.wrapper.querySelector('.session-log-btn');
    if (logBtn) logBtn.addEventListener('click', () => this.showSessionLog());
    const codecBtn = this.wrapper.querySelector('.session-codec-btn');
    if (codecBtn) codecBtn.addEventListener('click', () => this.toggleCodec());
    this.wrapper.querySelectorAll('.quality-preset-btn').forEach(btn => {
      btn.addEventListener('click', () => this.applyQualityPreset(btn.dataset.preset));
    });
    // Initial codec-btn-state opdatering (default JPEG indtil agent skifter)
    this._updateCodecBtn();
  }

  // Skift mellem H.264 og JPEG-tile-mode. Sender set_mode-message til agent
  // via control-channel. Agenten skifter encoder + opdaterer streaming-loop.
  toggleCodec() {
    const dc = this.dataChannel;
    if (!dc || dc.readyState !== 'open') {
      showToast('Ikke forbundet til agent', 'error');
      return;
    }
    // Hvis vi er i H.264 nu, skift til tiles. Ellers prøv H.264.
    const newMode = this.usingH264 ? 'tiles' : 'h264';
    const bitrate = newMode === 'h264' ? 16000 : 0;
    try {
      dc.send(JSON.stringify({ type: 'set_mode', mode: newMode, bitrate: bitrate }));
      showToast(`Skifter til ${newMode === 'h264' ? 'H.264' : 'JPEG'}-mode...`, 'info');
      console.log(`[${this.deviceName}] Requested codec: ${newMode}`);
    } catch (e) {
      showToast(`Kunne ikke skifte codec: ${e.message}`, 'error');
    }
  }

  _updateCodecBtn() {
    const btn = this.wrapper && this.wrapper.querySelector('.session-codec-btn');
    if (!btn) return;
    if (this.usingH264) {
      btn.title = 'Skift til JPEG (nu: H.264)';
      btn.innerHTML = '<i class="fas fa-film"></i>';
      btn.classList.add('codec-active-h264');
      btn.classList.remove('codec-active-jpeg');
    } else {
      btn.title = 'Skift til H.264 (nu: JPEG)';
      btn.innerHTML = '<i class="fas fa-image"></i>';
      btn.classList.add('codec-active-jpeg');
      btn.classList.remove('codec-active-h264');
    }
  }

  // Vis session-log i en modal — samler connect-log + recent console-events
  // + WebRTC-stats. Brugeren kan kopiere alt med ét klik og pase til support.
  showSessionLog() {
    const lines = [];
    lines.push(`=== SESSION LOG: ${this.deviceName} ===`);
    lines.push(`Tid: ${new Date().toISOString()}`);
    lines.push(`Device ID: ${this.deviceId || '?'}`);
    lines.push(`Codec: ${this.usingH264 ? 'H.264' : 'JPEG'}`);
    lines.push(`Connection: ${this.peerConnection ? this.peerConnection.connectionState : '?'}`);
    lines.push(`ICE: ${this.peerConnection ? this.peerConnection.iceConnectionState : '?'}`);
    if (this.canvasEl) {
      lines.push(`Canvas: ${this.canvasEl.width}x${this.canvasEl.height} (display ${this.canvasEl.clientWidth}x${this.canvasEl.clientHeight})`);
    }
    if (this.videoEl) {
      lines.push(`Video: ${this.videoEl.videoWidth}x${this.videoEl.videoHeight}`);
    }
    lines.push('');
    lines.push('=== Connect log ===');
    lines.push((this._connectLog || []).join('\n'));
    if (window._globalConsoleLog && window._globalConsoleLog.length) {
      lines.push('');
      lines.push('=== Recent console events ===');
      lines.push(window._globalConsoleLog.slice(-100).join('\n'));
    }
    const text = lines.join('\n');

    // Modal med textarea + kopier-knap
    const modal = document.createElement('div');
    modal.style.cssText = 'position:fixed;inset:0;background:rgba(0,0,0,0.7);z-index:9999;display:flex;align-items:center;justify-content:center;';
    modal.innerHTML = `
      <div style="background:var(--surface,#1a2332);color:var(--text,#fff);border-radius:8px;padding:1rem;width:80vw;max-width:900px;max-height:80vh;display:flex;flex-direction:column;gap:0.5rem;">
        <div style="display:flex;justify-content:space-between;align-items:center;">
          <h3 style="margin:0;">Session-log: ${this.deviceName}</h3>
          <div>
            <button class="btn btn-sm btn-primary log-copy-btn"><i class="fas fa-copy"></i> Kopier alt</button>
            <button class="btn btn-sm btn-secondary log-close-btn">Luk</button>
          </div>
        </div>
        <textarea readonly style="flex:1;min-height:400px;font-family:monospace;font-size:0.8rem;background:#0a0e1a;color:#cde;border:1px solid #333;padding:0.5rem;resize:none;">${text.replace(/[<>&]/g, c => ({'<':'&lt;','>':'&gt;','&':'&amp;'}[c]))}</textarea>
      </div>`;
    document.body.appendChild(modal);
    modal.querySelector('.log-close-btn').onclick = () => modal.remove();
    modal.querySelector('.log-copy-btn').onclick = async () => {
      try {
        await navigator.clipboard.writeText(text);
        showToast('Log kopieret til clipboard', 'success');
      } catch (e) {
        modal.querySelector('textarea').select();
        document.execCommand('copy');
        showToast('Log kopieret', 'success');
      }
    };
    modal.onclick = (e) => { if (e.target === modal) modal.remove(); };
  }

  applyQualityPreset(preset) {
    const presets = {
      low:    { max_fps: 15, max_quality: 45, max_scale: 0.5 },
      medium: { max_fps: 25, max_quality: 70, max_scale: 0.75 },
      high:   { max_fps: 30, max_quality: 95, max_scale: 1.0 }
    };
    const params = presets[preset];
    if (!params) return;

    const dc = this.dataChannel;
    if (!dc || dc.readyState !== 'open') {
      showToast('Ikke forbundet', 'error');
      return;
    }

    dc.send(JSON.stringify({ type: 'set_stream_params', ...params }));

    // Update active button
    this.wrapper.querySelectorAll('.quality-preset-btn').forEach(b => b.classList.remove('active'));
    this.wrapper.querySelector(`.quality-preset-btn[data-preset="${preset}"]`).classList.add('active');

    const labels = { low: 'Lav', medium: 'Mellem', high: 'Høj' };
    showToast(`Kvalitet: ${labels[preset]} (${params.max_fps} FPS, ${params.max_quality}%)`, 'success');
    console.log(`[${this.deviceName}] Quality preset: ${preset}`, params);
  }

  takeScreenshot() {
    const canvas = this.canvasEl;
    const video = this.videoEl;
    let dataUrl;
    if (this.usingH264 && video && video.videoWidth > 0) {
      const c = document.createElement('canvas');
      c.width = video.videoWidth;
      c.height = video.videoHeight;
      c.getContext('2d').drawImage(video, 0, 0);
      dataUrl = c.toDataURL('image/png');
    } else if (canvas && canvas.width > 0) {
      dataUrl = canvas.toDataURL('image/png');
    }
    if (!dataUrl) { showToast('Ingen video at screenshotte', 'warning'); return; }
    const a = document.createElement('a');
    a.href = dataUrl;
    a.download = `screenshot-${this.deviceName}-${new Date().toISOString().slice(0,19).replace(/:/g,'-')}.png`;
    a.click();
    showToast('Screenshot gemt!', 'success');
  }

  enableH264Mode() {
    // Request H.264 streaming mode for better performance on large screen changes
    const dc = this.dataChannel;
    if (!dc) return;

    const send = () => {
      if (dc.readyState === 'open') {
        dc.send(JSON.stringify({ type: 'set_mode', mode: 'h264', bitrate: 16000 }));
        console.log(`[${this.deviceName}] Requested H.264 mode (8 Mbps)`);
      }
    };

    if (dc.readyState === 'open') {
      send();
    } else {
      dc.addEventListener('open', send, { once: true });
    }
  }

  forceUpdateAgent() {
    if (!this.dataChannel || this.dataChannel.readyState !== 'open') {
      showToast('Ikke forbundet', 'error');
      return;
    }
    showToast('Sender opdateringskommando til agent...', 'info');
    this.dataChannel.send(JSON.stringify({ type: 'force_update' }));
  }

  async toggleFullscreen() {
    try {
      await window.go.main.App.ToggleFullscreen();
      this.isFullscreen = !this.isFullscreen;

      const toolbar = this.wrapper.querySelector('.viewer-toolbar');
      if (toolbar) {
        toolbar.classList.toggle('fullscreen-autohide', this.isFullscreen);
      }

      // Toggle true fullscreen — hide all controller chrome
      document.body.classList.toggle('viewer-fullscreen', this.isFullscreen);

      // Show/hide exit hint
      let hint = document.querySelector('.fullscreen-exit-hint');
      if (this.isFullscreen) {
        if (!hint) {
          hint = document.createElement('div');
          hint.className = 'fullscreen-exit-hint';
          hint.textContent = 'Tryk ESC eller F11 for at forlade fuldskærm';
          document.body.appendChild(hint);
        }
        hint.classList.remove('hidden');
        // Auto-hide hint after 3s
        setTimeout(() => hint.classList.add('hidden'), 3000);
      } else {
        if (hint) hint.classList.add('hidden');
      }
    } catch (e) {
      console.error('Fullscreen toggle failed:', e);
    }
  }

  sendMouseEvent(type, e) {
    if (!this.dataChannel || this.dataChannel.readyState !== 'open') return;

    // Calculate coordinates accounting for object-fit: contain (black bars)
    const target = this.canvasEl;
    const rect = target.getBoundingClientRect();
    const displayW = rect.width;
    const displayH = rect.height;

    // Get actual image/canvas resolution
    let actualW = target.width || displayW;
    let actualH = target.height || displayH;
    // For H.264 video mode, use video element dimensions
    if (this.usingH264 && this.videoEl) {
      actualW = this.videoEl.videoWidth || actualW;
      actualH = this.videoEl.videoHeight || actualH;
    }
    if (actualW === 0) actualW = displayW;
    if (actualH === 0) actualH = displayH;

    // object-fit: contain — calculate rendered image area
    const scaleX = displayW / actualW;
    const scaleY = displayH / actualH;
    const scale = Math.min(scaleX, scaleY);
    const renderW = actualW * scale;
    const renderH = actualH * scale;
    const offsetX = (displayW - renderW) / 2;
    const offsetY = (displayH - renderH) / 2;

    // Map mouse position to normalized 0-1 within the actual image
    const relX = Math.max(0, Math.min(1, (e.clientX - rect.left - offsetX) / renderW));
    const relY = Math.max(0, Math.min(1, (e.clientY - rect.top - offsetY) / renderH));
    const x = Math.round(relX * 10000) / 10000;
    const y = Math.round(relY * 10000) / 10000;

    // e.button: 0=left, 1=middle, 2=right
    const buttonNames = ['left', 'middle', 'right'];
    if (type === 'mousemove') {
      this.dataChannel.send(JSON.stringify({ t: 'mouse_move', x, y, rel: true }));
    } else if (type === 'mousedown') {
      this.dataChannel.send(JSON.stringify({ t: 'mouse_click', x, y, button: buttonNames[e.button] || 'left', down: true, rel: true }));
    } else if (type === 'mouseup') {
      this.dataChannel.send(JSON.stringify({ t: 'mouse_click', x, y, button: buttonNames[e.button] || 'left', down: false, rel: true }));
    }
  }

  sendWheelEvent(e) {
    if (!this.dataChannel || this.dataChannel.readyState !== 'open') return;
    // Agent expects: t=mouse_scroll, delta (positive=up, negative=down)
    this.dataChannel.send(JSON.stringify({
      t: 'mouse_scroll',
      delta: e.deltaY > 0 ? -1 : 1
    }));
  }

  sendKeyEvent(type, e) {
    if (!this.dataChannel || this.dataChannel.readyState !== 'open') return;

    if (type === 'keydown') {
      if (e.code === 'F11') { this.toggleFullscreen(); return; }
      if (e.code === 'Escape' && this.isFullscreen) { this.toggleFullscreen(); return; }

      // Ctrl+V → first push the controller's local clipboard to the
      // agent (writes to remote PC clipboard), then fall through so the
      // Ctrl+V keystroke is also forwarded — the focused app on the
      // remote PC pastes the just-updated clipboard. Data channel is
      // ordered + reliable so the clipboard message arrives before the
      // keystroke.
      if (e.ctrlKey && e.code === 'KeyV') {
        this.sendClipboardToAgent();
        // do not return — let the keystroke forward below
      }
    }

    const evt = {
      t: 'key',
      code: e.code,
      down: type === 'keydown',
      shift: e.shiftKey,
      ctrl: e.ctrlKey,
      alt: e.altKey
    };

    // AltGr on Windows sends ctrlKey+altKey — include the resolved char
    // so agent uses ForwardUnicodeChar (hybrid AltGr handler)
    if (e.ctrlKey && e.altKey && !e.metaKey && e.key.length === 1) {
      evt.char = e.key;
    }

    this.dataChannel.send(JSON.stringify(evt));
    e.preventDefault();
    e.stopPropagation();
  }

  // ==================== SUPPORT SESSION (VIEW-ONLY) ====================

  async connectAsSupport(supportSessionId) {
    this.isSupportSession = true;
    this.supportSessionId = supportSessionId;

    try {
      // Get connection config for Supabase + TURN
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

      // Use support session ID directly (no session-token call needed)
      this.sessionData = { session_id: supportSessionId };

      // Fetch TURN credentials
      await this.fetchTurnCredentials(cfg);

      // Wait for sharer to be ready, then connect
      await this.waitForSupportReady(supportSessionId);

    } catch (err) {
      console.error(`[${this.deviceName}] Support connection failed:`, err);
      showToast(`Support forbindelse fejlede: ${err.message}`, 'error');
      this.disconnect();
    }
  }

  async waitForSupportReady(sessionId) {
    console.log(`[${this.deviceName}] Waiting for support sharer to be ready...`);

    // Check if session is already active (sharer already connected)
    const { data: existingSignals } = await this.supabase
      .from('session_signaling')
      .select('*')
      .eq('session_id', sessionId)
      .eq('from_side', 'support')
      .eq('msg_type', 'answer');

    const alreadyReady = existingSignals && existingSignals.some(s => s.payload?.type === 'ready');

    if (alreadyReady) {
      console.log(`[${this.deviceName}] Sharer already ready, connecting now`);
      await this.connectToSupportSession(sessionId);
      return;
    }

    // Subscribe to signaling for ready signal
    this.signalingChannel = this.supabase
      .channel(`support-viewer-${sessionId}`)
      .on('postgres_changes', {
        event: 'INSERT',
        schema: 'public',
        table: 'session_signaling',
        filter: `session_id=eq.${sessionId}`,
      }, async (payload) => {
        const signal = payload.new;
        if (signal.from_side === 'support' && signal.msg_type === 'answer' && signal.payload?.type === 'ready') {
          console.log(`[${this.deviceName}] Sharer is ready!`);
          if (this.pollingInterval) { clearInterval(this.pollingInterval); this.pollingInterval = null; }
          await this.connectToSupportSession(sessionId);
        }
      })
      .subscribe();

    // Polling fallback
    this.pollingInterval = setInterval(async () => {
      const { data } = await this.supabase
        .from('session_signaling')
        .select('*')
        .eq('session_id', sessionId)
        .eq('from_side', 'support')
        .eq('msg_type', 'answer');

      if (data && data.some(s => s.payload?.type === 'ready' && !this.processedSignalIds.has(s.id))) {
        console.log(`[${this.deviceName}] Polled: sharer is ready!`);
        clearInterval(this.pollingInterval);
        this.pollingInterval = null;
        await this.connectToSupportSession(sessionId);
      }
    }, 1500);
  }

  async connectToSupportSession(sessionId) {
    console.log(`[${this.deviceName}] Connecting to support session as viewer (offerer)`);

    // Setup peer connection (receive video only, no data channels for input)
    const config = { ...this.iceConfig };
    this.peerConnection = new RTCPeerConnection(config);

    // Handle remote video track
    this.peerConnection.ontrack = (event) => {
      console.log(`[${this.deviceName}] Support: received track`, event.track.kind);
      if (event.track.kind === 'video' && event.streams[0]) {
        this.usingH264 = true;
        this.videoEl.srcObject = event.streams[0];
        this.videoEl.style.display = '';
        // Hide canvas for video-track-based sessions
        this.canvasEl.style.display = 'none';
      }
    };

    // Send ICE candidates
    this.peerConnection.onicecandidate = async (event) => {
      if (event.candidate) {
        await this.supabase.from('session_signaling').insert({
          session_id: sessionId,
          from_side: 'dashboard',
          msg_type: 'ice',
          payload: event.candidate.toJSON(),
        });
      }
    };

    // Connection state
    this.peerConnection.onconnectionstatechange = () => {
      const state = this.peerConnection?.connectionState;
      console.log(`[${this.deviceName}] Support connection state:`, state);
      if (state === 'connected') {
        this.onSupportConnected();
      } else if (state === 'disconnected' || state === 'failed') {
        this.onDisconnected();
      }
    };

    // Create offer (receive video only)
    const offer = await this.peerConnection.createOffer({
      offerToReceiveVideo: true,
      offerToReceiveAudio: false,
    });
    await this.peerConnection.setLocalDescription(offer);

    // Send offer
    await this.supabase.from('session_signaling').insert({
      session_id: sessionId,
      from_side: 'dashboard',
      msg_type: 'offer',
      payload: { type: 'offer', sdp: offer.sdp },
    });

    console.log(`[${this.deviceName}] Support offer sent`);

    // Subscribe to answer/ICE from support sharer
    this.subscribeSupportSignaling(sessionId);
  }

  subscribeSupportSignaling(sessionId) {
    // Reset processed IDs for signaling (keep ready signal IDs)
    this.processedSignalIds.clear();

    // Stop any previous polling
    if (this.pollingInterval) { clearInterval(this.pollingInterval); this.pollingInterval = null; }

    this.pollingInterval = setInterval(async () => {
      try {
        const { data } = await this.supabase
          .from('session_signaling')
          .select('*')
          .eq('session_id', sessionId)
          .eq('from_side', 'support')
          .order('created_at', { ascending: true });

        if (!data) return;

        for (const signal of data) {
          if (this.processedSignalIds.has(signal.id)) continue;
          this.processedSignalIds.add(signal.id);
          await this.handleSupportSignal(signal);
        }
      } catch (e) {
        console.error('Support signaling poll error:', e);
      }
    }, 500);
  }

  async handleSupportSignal(signal) {
    if (signal.from_side !== 'support') return;
    if (!this.peerConnection) return;

    try {
      switch (signal.msg_type) {
        case 'answer': {
          // Skip ready signals
          if (signal.payload?.type === 'ready') return;

          if (this.peerConnection.signalingState !== 'have-local-offer') {
            console.log(`[${this.deviceName}] Skipping answer, state:`, this.peerConnection.signalingState);
            return;
          }

          const answer = new RTCSessionDescription(signal.payload);
          await this.peerConnection.setRemoteDescription(answer);
          console.log(`[${this.deviceName}] Support: remote description set`);

          // Flush buffered ICE
          for (const c of this.pendingIceCandidates) {
            await this.peerConnection.addIceCandidate(new RTCIceCandidate(c));
          }
          this.pendingIceCandidates = [];
          break;
        }

        case 'ice': {
          let iceCandidate = signal.payload;
          if (signal.payload.candidate && typeof signal.payload.candidate === 'object') {
            iceCandidate = signal.payload.candidate;
          }

          if (iceCandidate && iceCandidate.candidate) {
            const ice = {
              candidate: iceCandidate.candidate,
              sdpMid: iceCandidate.sdpMid,
              sdpMLineIndex: iceCandidate.sdpMLineIndex,
            };
            if (!this.peerConnection.remoteDescription) {
              this.pendingIceCandidates.push(ice);
            } else {
              await this.peerConnection.addIceCandidate(new RTCIceCandidate(ice));
            }
          }
          break;
        }

        case 'bye':
          console.log(`[${this.deviceName}] Support sharer disconnected`);
          showToast(`${this.deviceName}: Support session afsluttet`, 'info');
          this.disconnect();
          break;
      }
    } catch (e) {
      console.error(`[${this.deviceName}] Support signal error:`, e);
    }
  }

  onSupportConnected() {
    this.connected = true;
    this.connectingEl.style.display = 'none';
    this.activeEl.style.display = 'flex';

    showToast(`Forbundet til ${this.deviceName} (kun visning)`, 'success');
    this.startStats();

    // Hide input-related buttons (view-only session)
    const filesBtn = this.wrapper.querySelector('.session-files-btn');
    if (filesBtn) filesBtn.style.display = 'none';

    // Notify session manager
    if (window.SessionManager) {
      window.SessionManager.onSessionConnected(this.id);
    }
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

  connect(deviceId, deviceName, agentVersion) {
    console.log('SessionManager.connect:', deviceId, deviceName, agentVersion);
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
    session.agentVersion = agentVersion || '';
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
  connect(deviceId, deviceName, agentVersion) {
    SessionManager.connect(deviceId, deviceName, agentVersion);
  },
  disconnect() {
    SessionManager.disconnectAll();
  }
};
