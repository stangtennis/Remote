// WebRTC Connection Module
// Handles peer connection, media tracks, and data channels
// All per-session state lives on ctx (session object from SessionManager)

// ICE Configuration - fetched dynamically for security
let iceConfig = {
  iceServers: [
    { urls: 'stun:stun.l.google.com:19302' },
    { urls: 'stun:stun1.l.google.com:19302' }
  ]
};

// Fetch dynamic TURN credentials from backend
async function fetchTurnCredentials() {
  try {
    const { data: { session } } = await supabase.auth.getSession();
    if (!session) {
      debug('⚠️ No session, using STUN only');
      return;
    }

    const response = await fetch(`${SUPABASE_CONFIG.url}/functions/v1/turn-credentials`, {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${session.access_token}`,
        'Content-Type': 'application/json'
      }
    });

    if (response.ok) {
      const data = await response.json();
      iceConfig = { iceServers: data.iceServers };
      debug(`✅ TURN credentials fetched (expires in ${data.ttl}s)`);
    } else {
      console.warn('⚠️ Failed to fetch TURN credentials, using STUN only');
    }
  } catch (error) {
    console.warn('⚠️ Error fetching TURN credentials:', error);
  }
}

// Clean up a specific session's WebRTC resources
function cleanupSessionWebRTC(ctx) {
  if (!ctx) return;
  debug('🧹 Cleaning up WebRTC for session:', ctx.id);

  // Stop bandwidth interval
  if (ctx.bandwidthInterval) {
    clearInterval(ctx.bandwidthInterval);
    ctx.bandwidthInterval = null;
  }

  // Stop stats interval
  if (ctx.statsInterval) {
    clearInterval(ctx.statsInterval);
    ctx.statsInterval = null;
  }

  // Clear frame timeout
  if (ctx.frameTimeout) {
    clearTimeout(ctx.frameTimeout);
    ctx.frameTimeout = null;
  }

  // Close data channel
  if (ctx.dataChannel) {
    try { ctx.dataChannel.close(); } catch (e) {}
    ctx.dataChannel = null;
  }

  // Close peer connection
  if (ctx.peerConnection) {
    try { ctx.peerConnection.close(); } catch (e) {}
    ctx.peerConnection = null;
  }

  // Reset frame state
  ctx.frameChunks = [];
  ctx.expectedChunks = 0;
  ctx.currentFrameId = -1;

  // Update globals if this was the active session
  if (window.SessionManager && window.SessionManager.activeSessionId === ctx.id) {
    window.peerConnection = null;
    window.dataChannel = null;
  }

  debug('✅ WebRTC cleanup complete for session:', ctx.id);
}

// Legacy cleanupWebRTC - cleans up active session
function cleanupWebRTC() {
  const ctx = window.SessionManager?.getActiveSession();
  if (ctx) {
    cleanupSessionWebRTC(ctx);
  }
  // Also clean up input capture
  if (typeof cleanupInputCapture === 'function') {
    cleanupInputCapture();
  }
}

// Expose cleanup globally
window.cleanupWebRTC = cleanupWebRTC;
window.cleanupSessionWebRTC = cleanupSessionWebRTC;

async function initWebRTC(sessionData, ctx) {
  try {
    debug('🚀 initWebRTC called for device:', ctx.id);

    if (!sessionData || !sessionData.session_id) {
      throw new Error('Invalid session object - missing session_id');
    }

    // Check if we should force relay mode (for testing TURN)
    const forceRelay = new URLSearchParams(window.location.search).get('relay') === 'true';

    // Fetch TURN credentials if not already fetched
    if (iceConfig.iceServers.length <= 2) {
      await fetchTurnCredentials();
    }

    // Use dynamically fetched ICE configuration
    const configuration = {
      ...iceConfig,
      // Force relay mode if ?relay=true in URL (for testing)
      ...(forceRelay && { iceTransportPolicy: 'relay' })
    };

    if (forceRelay) {
      debug('⚠️ RELAY-ONLY MODE ENABLED (for testing)');
    }

    debug('🔐 Dashboard TURN config:', JSON.stringify(configuration, null, 2));

    // Create peer connection on ctx
    ctx.peerConnection = new RTCPeerConnection(configuration);
    // Set global ref for active session
    if (window.SessionManager.activeSessionId === ctx.id) {
      window.peerConnection = ctx.peerConnection;
    }
    debug('✅ PeerConnection created for', ctx.id);

    // Set up event handlers
    setupPeerConnectionHandlers(ctx);
    debug('✅ Event handlers set up');

    // Create data channel for control inputs
    ctx.dataChannel = ctx.peerConnection.createDataChannel('control', {
      ordered: true
    });
    setupDataChannelHandlers(ctx);
    // Set global ref for active session
    if (window.SessionManager.activeSessionId === ctx.id) {
      window.dataChannel = ctx.dataChannel;
    }
    debug('✅ Data channel created');

    // Create offer
    debug('📝 Creating offer...');
    const offer = await ctx.peerConnection.createOffer({
      offerToReceiveVideo: true,
      offerToReceiveAudio: false
    });
    debug('✅ Offer created');

    debug('📝 Setting local description...');
    await ctx.peerConnection.setLocalDescription(offer);
    debug('✅ Local description set');

    // Send offer via signaling
    debug('📤 Sending offer to agent via signaling...');
    await sendSignal({
      session_id: sessionData.session_id,
      from: 'dashboard',
      type: 'offer',
      sdp: offer.sdp
    });
    debug('✅ WebRTC offer sent successfully!');

  } catch (error) {
    console.error('❌ WebRTC initialization failed:', error);
    console.error('Error stack:', error.stack);
    throw error;
  }
}

function setupPeerConnectionHandlers(ctx) {
  const pc = ctx.peerConnection;

  // ICE candidate handler
  pc.onicecandidate = async (event) => {
    if (event.candidate) {
      // Determine candidate type for logging
      const candidateStr = event.candidate.candidate;
      let candidateType = 'unknown';
      if (candidateStr.includes('typ relay')) {
        candidateType = 'RELAY (TURN)';
      } else if (candidateStr.includes('typ srflx')) {
        candidateType = 'SRFLX (STUN)';
      } else if (candidateStr.includes('typ host')) {
        candidateType = 'HOST (local)';
      } else if (candidateStr.includes('typ prflx')) {
        candidateType = 'PRFLX (peer)';
      }

      debug(`📤 Sending ICE candidate [${candidateType}]:`, candidateStr.substring(0, 80) + '...');

      if (!ctx.sessionData) {
        console.error('⚠️ Cannot send ICE candidate: sessionData is null for', ctx.id);
        return;
      }

      await sendSignal({
        session_id: ctx.sessionData.session_id,
        from: 'dashboard',
        type: 'ice',
        candidate: event.candidate
      });
    } else {
      debug('📤 ICE gathering complete (null candidate)');
    }
  };

  // ICE gathering state handler
  pc.onicegatheringstatechange = () => {
    debug('ICE gathering state:', pc.iceGatheringState);
  };

  // Connection state handler
  pc.onconnectionstatechange = () => {
    const state = pc.connectionState;
    debug('❗ Connection state:', state, 'for device:', ctx.id);
    debug('❗ ICE state:', pc.iceConnectionState);
    debug('❗ Signaling state:', pc.signalingState);

    // Update SessionManager
    if (window.SessionManager) {
      const sessionStatus = state === 'connected' ? 'connected' :
                           state === 'connecting' ? 'connecting' : 'disconnected';
      window.SessionManager.updateSessionStatus(ctx.id, sessionStatus);
    }

    // Only update DOM elements if this is the active session
    const isActive = window.SessionManager?.activeSessionId === ctx.id;

    if (isActive) {
      const statusElement = document.getElementById('sessionStatus');
      const overlay = document.getElementById('viewerOverlay');
      const reconnectOverlay = document.getElementById('previewReconnecting');

      switch (state) {
        case 'connecting':
          if (statusElement) {
            statusElement.textContent = 'Connecting...';
            statusElement.className = 'status-badge pending';
          }
          break;
        case 'connected':
          if (statusElement) {
            statusElement.textContent = 'Connected';
            statusElement.className = 'status-badge online';
          }
          if (overlay) overlay.style.display = 'none';
          // Hide reconnect overlay if we just reconnected
          if (ctx.reconnectState === 'reconnecting') {
            ctx.reconnectState = 'idle';
            ctx.reconnectAttempt = 0;
            ctx.reconnectStartedAt = null;
            if (reconnectOverlay) reconnectOverlay.style.display = 'none';
            showToast('Forbindelse genoprettet!', 'success');
            debug('✅ Reconnect successful for', ctx.id);
          }
          updateConnectionStats(ctx);
          break;
        case 'disconnected':
        case 'failed':
          if (statusElement) {
            statusElement.textContent = state === 'failed' ? 'Connection Failed' : 'Disconnected';
            statusElement.className = 'status-badge offline';
          }
          // Trigger auto-reconnect if not already in progress
          if (ctx.reconnectState === 'idle' && ctx.sessionData) {
            debug('🔄 Starting auto-reconnect for', ctx.id);
            ctx.reconnectState = 'reconnecting';
            ctx.reconnectStartedAt = Date.now();
            ctx.reconnectAttempt = 0;
            if (reconnectOverlay) {
              reconnectOverlay.style.display = 'flex';
              const statusEl = document.getElementById('reconnectStatus');
              if (statusEl) statusEl.textContent = 'Forsøg 1/8';
            }
            // Start reconnect from app.js
            if (typeof window.reconnectSession === 'function') {
              window.reconnectSession(ctx.id);
            }
          }
          break;
      }
    } else {
      // Not active session, but still trigger reconnect
      if ((state === 'disconnected' || state === 'failed') && ctx.reconnectState === 'idle' && ctx.sessionData) {
        ctx.reconnectState = 'reconnecting';
        ctx.reconnectStartedAt = Date.now();
        ctx.reconnectAttempt = 0;
        if (typeof window.reconnectSession === 'function') {
          window.reconnectSession(ctx.id);
        }
      }
    }

    // Stop polling when connected (per-session)
    if (state === 'connected') {
      if (window.stopSessionPolling) {
        window.stopSessionPolling(ctx);
        debug('🛑 Stopped signaling polling for', ctx.id, '(connection established)');
      }
    }
  };

  // Track handler (remote video/canvas)
  pc.ontrack = (event) => {
    debug('Remote track received:', event.track.kind, 'for device:', ctx.id);
    // Only set video srcObject if this is the active session
    if (window.SessionManager?.activeSessionId === ctx.id) {
      const remoteVideo = document.getElementById('remoteVideo');
      if (remoteVideo && event.streams[0]) {
        remoteVideo.srcObject = event.streams[0];
      }
    }
  };

  // ICE connection state handler
  pc.oniceconnectionstatechange = () => {
    debug('ICE state:', pc.iceConnectionState, 'for device:', ctx.id);

    if (pc.iceConnectionState === 'connected') {
      updateConnectionType(ctx);
    }
  };
}

function setupDataChannelHandlers(ctx) {
  const dc = ctx.dataChannel;

  dc.onopen = () => {
    debug('Data channel opened for', ctx.id);
    // Enable mouse/keyboard input (only once, shared across sessions)
    setupInputCapture();
  };

  dc.onclose = () => {
    debug('Data channel closed for', ctx.id);
    // Only cleanup input if no sessions remain
    if (window.SessionManager && window.SessionManager.getSessionCount() <= 1) {
      cleanupInputCapture();
    }
  };

  dc.onerror = (error) => {
    console.error('Data channel error for', ctx.id, ':', error);
  };

  dc.onmessage = async (event) => {
    // Track bandwidth per-session
    let dataSize = 0;
    if (event.data instanceof ArrayBuffer) {
      dataSize = event.data.byteLength;
    } else if (event.data instanceof Blob) {
      dataSize = event.data.size;
    } else if (typeof event.data === 'string') {
      dataSize = event.data.length;
    }
    ctx.bytesReceived += dataSize;

    // Receive JPEG frame from agent (possibly chunked)
    if (event.data instanceof ArrayBuffer) {
      const data = new Uint8Array(event.data);

      // Check if this is JSON (starts with '{' = 0x7B)
      if (data.length > 0 && data[0] === 0x7B) {
        try {
          const text = new TextDecoder().decode(data);
          const msg = JSON.parse(text);
          handleAgentMessage(msg);
        } catch (e) {
          console.warn('Failed to parse JSON message from ArrayBuffer:', e);
        }
        return;
      }

      // Frame type detection
      const FRAME_TYPE_REGION = 0x02;
      const CHUNK_MAGIC_OLD = 0xFF;
      const CHUNK_MAGIC_NEW = 0xFE;

      // Check for dirty region (type 0x02)
      if (data.length > 9 && data[0] === FRAME_TYPE_REGION) {
        const x = data[1] | (data[2] << 8);
        const y = data[3] | (data[4] << 8);
        const w = data[5] | (data[6] << 8);
        const h = data[7] | (data[8] << 8);
        const jpegData = data.slice(9);
        // Only display if active session
        if (window.SessionManager?.activeSessionId === ctx.id) {
          displayDirtyRegion(jpegData.buffer, x, y, w, h);
        }
        ctx.framesReceived++;
      }
      // Check for NEW chunked frame format
      else if (data.length > 5 && data[0] === CHUNK_MAGIC_NEW) {
        const frameId = (data[1] << 8) | data[2];
        const chunkIndex = data[3];
        const totalChunks = data[4];
        const chunkData = data.slice(5);

        // If frame ID changed, start a new frame
        if (ctx.currentFrameId !== frameId) {
          if (ctx.frameChunks.length > 0 && ctx.expectedChunks > 0) {
            ctx.framesDropped++;
          }
          ctx.frameChunks = new Array(totalChunks);
          ctx.expectedChunks = totalChunks;
          ctx.currentFrameId = frameId;

          if (ctx.frameTimeout) clearTimeout(ctx.frameTimeout);
          ctx.frameTimeout = setTimeout(() => {
            if (ctx.expectedChunks > 0 && ctx.currentFrameId === frameId) {
              ctx.framesDropped++;
              ctx.frameChunks = [];
              ctx.expectedChunks = 0;
            }
          }, 500);
        }

        // Store this chunk
        if (ctx.expectedChunks > 0 && chunkIndex < ctx.expectedChunks) {
          ctx.frameChunks[chunkIndex] = chunkData;

          const receivedCount = ctx.frameChunks.filter(c => c).length;
          if (receivedCount === ctx.expectedChunks) {
            if (ctx.frameTimeout) {
              clearTimeout(ctx.frameTimeout);
              ctx.frameTimeout = null;
            }

            // Reassemble frame
            const totalLength = ctx.frameChunks.reduce((sum, chunk) => sum + chunk.length, 0);
            const completeFrame = new Uint8Array(totalLength);
            let offset = 0;
            for (const chunk of ctx.frameChunks) {
              completeFrame.set(chunk, offset);
              offset += chunk.length;
            }

            // Only display if active session
            if (window.SessionManager?.activeSessionId === ctx.id) {
              displayVideoFrame(completeFrame.buffer, ctx);
            } else {
              // Store frame for tab switching even when not active
              storeFrameForSession(completeFrame.buffer, ctx);
            }
            ctx.framesReceived++;

            ctx.frameChunks = [];
            ctx.expectedChunks = 0;
          }
        }
      }
      // Check for OLD chunked frame format
      else if (data.length > 3 && data[0] === CHUNK_MAGIC_OLD && data[1] !== 0xD8) {
        const chunkIndex = data[1];
        const totalChunks = data[2];
        const chunkData = data.slice(3);

        if (chunkIndex === 0) {
          if (ctx.frameChunks.length > 0 && ctx.expectedChunks > 0) {
            ctx.framesDropped++;
          }
          ctx.frameChunks = new Array(totalChunks);
          ctx.expectedChunks = totalChunks;

          if (ctx.frameTimeout) clearTimeout(ctx.frameTimeout);
          ctx.frameTimeout = setTimeout(() => {
            if (ctx.expectedChunks > 0) {
              ctx.framesDropped++;
              ctx.frameChunks = [];
              ctx.expectedChunks = 0;
            }
          }, 500);
        }

        if (ctx.expectedChunks > 0 && chunkIndex < ctx.expectedChunks) {
          ctx.frameChunks[chunkIndex] = chunkData;

          const receivedCount = ctx.frameChunks.filter(c => c).length;
          if (receivedCount === ctx.expectedChunks) {
            if (ctx.frameTimeout) {
              clearTimeout(ctx.frameTimeout);
              ctx.frameTimeout = null;
            }

            const totalLength = ctx.frameChunks.reduce((sum, chunk) => sum + chunk.length, 0);
            const completeFrame = new Uint8Array(totalLength);
            let offset = 0;
            for (const chunk of ctx.frameChunks) {
              completeFrame.set(chunk, offset);
              offset += chunk.length;
            }

            if (window.SessionManager?.activeSessionId === ctx.id) {
              displayVideoFrame(completeFrame.buffer, ctx);
            } else {
              storeFrameForSession(completeFrame.buffer, ctx);
            }
            ctx.framesReceived++;

            ctx.frameChunks = [];
            ctx.expectedChunks = 0;
          }
        }
      } else {
        // Single-packet frame (raw JPEG starting with FF D8)
        if (window.SessionManager?.activeSessionId === ctx.id) {
          displayVideoFrame(event.data, ctx);
        } else {
          storeFrameForSession(event.data, ctx);
        }
        ctx.framesReceived++;
      }
    } else if (event.data instanceof Blob) {
      if (window.SessionManager?.activeSessionId === ctx.id) {
        displayVideoFrame(event.data, ctx);
      }
      ctx.framesReceived++;
    } else if (typeof event.data === 'string') {
      try {
        const msg = JSON.parse(event.data);
        handleAgentMessage(msg);
      } catch (e) {
        console.warn('Failed to parse message:', e);
      }
    }
  };

  // Per-session bandwidth interval
  ctx.bandwidthInterval = setInterval(() => {
    const now = Date.now();
    const elapsed = (now - ctx.lastBandwidthCheck) / 1000;

    if (elapsed > 0 && ctx.bytesReceived > 0) {
      const bitsPerSecond = (ctx.bytesReceived * 8) / elapsed;
      ctx.currentBandwidthMbps = bitsPerSecond / 1000000;

      // Only update UI for active session
      if (window.SessionManager?.activeSessionId === ctx.id) {
        updateBandwidthDisplay(ctx.currentBandwidthMbps, ctx.framesReceived / elapsed);
        debug(`📊 Bandwidth: ${ctx.currentBandwidthMbps.toFixed(2)} Mbit/s | FPS: ${(ctx.framesReceived / elapsed).toFixed(1)} | Dropped: ${ctx.framesDropped}`);
      }
    }

    // Reset counters
    ctx.bytesReceived = 0;
    ctx.framesReceived = 0;
    ctx.framesDropped = 0;
    ctx.lastBandwidthCheck = now;
  }, 1000);

  // Per-session stats interval
  ctx.statsInterval = setInterval(() => {
    if (ctx.peerConnection && ctx.peerConnection.connectionState === 'connected') {
      if (window.SessionManager?.activeSessionId === ctx.id) {
        updateConnectionStats(ctx);
      }
    }
  }, 2000);
}

// Store a frame as base64 for tab-switching (non-active sessions)
function storeFrameForSession(data, ctx) {
  // Only store every ~10th frame to save memory
  if (Math.random() >= 0.1) return;

  const blob = data instanceof Blob ? data : new Blob([data], { type: 'image/jpeg' });
  const reader = new FileReader();
  reader.onloadend = () => {
    const base64 = reader.result.split(',')[1];
    if (base64 && window.SessionManager) {
      window.SessionManager.storeFrame(ctx.id, base64);
    }
  };
  reader.readAsDataURL(blob);
}

// Update bandwidth display in UI
function updateBandwidthDisplay(mbps, fps) {
  const statsEl = document.getElementById('bandwidthStats');
  if (statsEl) {
    statsEl.textContent = `${mbps.toFixed(1)} Mbit/s | ${fps.toFixed(0)} FPS`;
  }

  const connectionInfo = document.getElementById('connectionInfo');
  if (connectionInfo) {
    const bwSpan = connectionInfo.querySelector('.bandwidth');
    if (bwSpan) {
      bwSpan.textContent = `${mbps.toFixed(1)} Mbit/s`;
    }
  }
}

// Get current bandwidth (for external use)
function getCurrentBandwidth() {
  const ctx = window.SessionManager?.getActiveSession();
  return ctx ? ctx.currentBandwidthMbps : 0;
}

// Helper function to calculate actual image area within canvas (accounting for object-fit: contain)
function getImageCoordinates(element, clientX, clientY) {
  const rect = element.getBoundingClientRect();

  const displayWidth = rect.width;
  const displayHeight = rect.height;

  let actualWidth, actualHeight;
  if (element.tagName === 'CANVAS') {
    actualWidth = element.width;
    actualHeight = element.height;
  } else if (element.tagName === 'VIDEO') {
    actualWidth = element.videoWidth || displayWidth;
    actualHeight = element.videoHeight || displayHeight;
  } else {
    actualWidth = displayWidth;
    actualHeight = displayHeight;
  }

  if (actualWidth === 0 || actualHeight === 0) {
    actualWidth = displayWidth;
    actualHeight = displayHeight;
  }

  const relX = clientX - rect.left;
  const relY = clientY - rect.top;

  const scaleX = displayWidth / actualWidth;
  const scaleY = displayHeight / actualHeight;
  const scale = Math.min(scaleX, scaleY);

  const renderWidth = actualWidth * scale;
  const renderHeight = actualHeight * scale;

  const offsetX = (displayWidth - renderWidth) / 2;
  const offsetY = (displayHeight - renderHeight) / 2;

  const x = Math.max(0, Math.min(1, (relX - offsetX) / renderWidth));
  const y = Math.max(0, Math.min(1, (relY - offsetY) / renderHeight));

  return { x, y };
}

// Store event listeners so we can clean them up
let inputListenersAttached = false;
let inputEventHandlers = {};

function setupInputCapture() {
  const remoteVideo = document.getElementById('remoteVideo');
  const remoteCanvas = document.getElementById('remoteCanvas') || document.getElementById('previewCanvas');
  const target = remoteCanvas || remoteVideo;

  if (!target) return;

  // Prevent duplicate event listeners
  if (inputListenersAttached) {
    debug('Input capture already enabled, skipping duplicate setup');
    return;
  }
  inputListenersAttached = true;

  target.focus();
  debug('🎯 Canvas focused for keyboard input');

  const contextMenuHandler = (e) => {
    e.preventDefault();
    e.stopPropagation();
  };
  target.addEventListener('contextmenu', contextMenuHandler);
  inputEventHandlers.contextMenu = contextMenuHandler;

  let lastMouseMove = 0;
  const mouseMoveHandler = (e) => {
    const dc = getActiveDataChannel();
    if (!dc || dc.readyState !== 'open') return;

    const now = Date.now();
    if (now - lastMouseMove < 16) return;
    lastMouseMove = now;

    const coords = getImageCoordinates(target, e.clientX, e.clientY);

    sendControlEvent({
      t: 'mouse_move',
      x: Math.round(coords.x * 10000) / 10000,
      y: Math.round(coords.y * 10000) / 10000,
      rel: true
    });
  };
  target.addEventListener('mousemove', mouseMoveHandler);
  inputEventHandlers.mouseMove = mouseMoveHandler;

  const mouseDownHandler = (e) => {
    const dc = getActiveDataChannel();
    if (!dc || dc.readyState !== 'open') return;

    const button = ['left', 'middle', 'right'][e.button] || 'left';
    sendControlEvent({
      t: 'mouse_click',
      button,
      down: true
    });
    e.preventDefault();
  };
  target.addEventListener('mousedown', mouseDownHandler);
  inputEventHandlers.mouseDown = mouseDownHandler;

  const mouseUpHandler = (e) => {
    const dc = getActiveDataChannel();
    if (!dc || dc.readyState !== 'open') return;

    const button = ['left', 'middle', 'right'][e.button] || 'left';
    sendControlEvent({
      t: 'mouse_click',
      button,
      down: false
    });
    e.preventDefault();
  };
  target.addEventListener('mouseup', mouseUpHandler);
  inputEventHandlers.mouseUp = mouseUpHandler;

  const wheelHandler = (e) => {
    const dc = getActiveDataChannel();
    if (!dc || dc.readyState !== 'open') return;

    sendControlEvent({
      t: 'mouse_scroll',
      delta: e.deltaY > 0 ? -1 : 1
    });
    e.preventDefault();
  };
  target.addEventListener('wheel', wheelHandler);
  inputEventHandlers.wheel = wheelHandler;

  target.tabIndex = 0;
  target.style.outline = 'none';

  const clickHandler = () => {
    target.focus();
  };
  target.addEventListener('click', clickHandler);
  inputEventHandlers.click = clickHandler;

  const pressedKeys = new Set();

  const keyDownHandler = async (e) => {
    const dc = getActiveDataChannel();
    if (!dc || dc.readyState !== 'open') return;

    if (e.ctrlKey && e.code === 'KeyV') {
      e.preventDefault();
      e.stopPropagation();
      await sendClipboardToAgent();
      return;
    }

    if (pressedKeys.has(e.code)) return;
    pressedKeys.add(e.code);

    sendControlEvent({
      t: 'key',
      code: e.code,
      down: true,
      ctrl: e.ctrlKey,
      shift: e.shiftKey,
      alt: e.altKey
    });
    e.preventDefault();
    e.stopPropagation();
  };
  target.addEventListener('keydown', keyDownHandler);
  inputEventHandlers.keyDown = keyDownHandler;

  const keyUpHandler = (e) => {
    const dc = getActiveDataChannel();
    if (!dc || dc.readyState !== 'open') return;

    pressedKeys.delete(e.code);

    sendControlEvent({
      t: 'key',
      code: e.code,
      down: false
    });
    e.preventDefault();
    e.stopPropagation();
  };
  target.addEventListener('keyup', keyUpHandler);
  inputEventHandlers.keyUp = keyUpHandler;

  debug('✅ Input capture enabled (routes to active session)');
}

function cleanupInputCapture() {
  if (!inputListenersAttached) return;

  const remoteVideo = document.getElementById('remoteVideo');
  const remoteCanvas = document.getElementById('remoteCanvas');
  const target = remoteCanvas || remoteVideo;

  if (target && inputEventHandlers) {
    Object.entries(inputEventHandlers).forEach(([name, handler]) => {
      const eventName = {
        contextMenu: 'contextmenu',
        mouseMove: 'mousemove',
        mouseDown: 'mousedown',
        mouseUp: 'mouseup',
        wheel: 'wheel',
        click: 'click',
        keyDown: 'keydown',
        keyUp: 'keyup'
      }[name];
      if (eventName) {
        target.removeEventListener(eventName, handler);
      }
    });
  }

  inputListenersAttached = false;
  inputEventHandlers = {};
  debug('🧹 Input capture cleaned up');
}

// Get the active session's data channel
function getActiveDataChannel() {
  const session = window.SessionManager?.getActiveSession();
  return session?.dataChannel || null;
}

function sendControlEvent(event) {
  const dc = getActiveDataChannel();
  if (dc && dc.readyState === 'open') {
    dc.send(JSON.stringify(event));
  }
}

// Export for use in other modules
window.sendControlEvent = sendControlEvent;

async function updateConnectionStats(ctx) {
  const pc = ctx ? ctx.peerConnection : window.peerConnection;
  if (!pc) return;

  try {
    const stats = await pc.getStats();
    let bitrate = 0;
    let rtt = 0;
    let packetLoss = 0;

    stats.forEach(report => {
      if (report.type === 'inbound-rtp' && report.kind === 'video') {
        bitrate = Math.round((report.bytesReceived * 8) / 1000);
        packetLoss = report.packetsLost || 0;
      }
      if (report.type === 'candidate-pair' && report.state === 'succeeded') {
        rtt = report.currentRoundTripTime ?
          Math.round(report.currentRoundTripTime * 1000) : 0;
      }
    });

    document.getElementById('statBitrate').textContent = bitrate + ' kbps';
    document.getElementById('statRtt').textContent = rtt + ' ms';
    document.getElementById('statPacketLoss').textContent = packetLoss + ' packets';

  } catch (error) {
    console.error('Failed to get stats:', error);
  }
}

async function updateConnectionType(ctx) {
  const pc = ctx ? ctx.peerConnection : window.peerConnection;
  if (!pc) return;

  // Only update UI for active session
  if (ctx && window.SessionManager?.activeSessionId !== ctx.id) return;

  try {
    const stats = await pc.getStats();
    let connectionType = 'Unknown';

    let activePair = null;
    stats.forEach(report => {
      if (report.type === 'candidate-pair' && report.state === 'succeeded' && report.nominated) {
        activePair = report;
      }
    });

    if (activePair) {
      let localType = 'unknown';
      let remoteType = 'unknown';

      stats.forEach(report => {
        if (report.type === 'local-candidate' && report.id === activePair.localCandidateId) {
          localType = report.candidateType || 'unknown';
        }
        if (report.type === 'remote-candidate' && report.id === activePair.remoteCandidateId) {
          remoteType = report.candidateType || 'unknown';
        }
      });

      debug(`🔗 Connection: local=${localType}, remote=${remoteType}`);

      if (localType === 'relay' || remoteType === 'relay') {
        connectionType = 'TURN (Relayed)';
      } else if (localType === 'srflx' || remoteType === 'srflx') {
        connectionType = 'P2P (STUN)';
      } else if (localType === 'host' && remoteType === 'host') {
        connectionType = 'P2P (Direct)';
      }
    }

    document.getElementById('statConnectionType').textContent = connectionType;

  } catch (error) {
    console.error('Failed to get connection type:', error);
  }
}

// Display video frame on canvas
function displayVideoFrame(data, ctx) {
  const canvas = document.getElementById('previewCanvas') || document.getElementById('remoteCanvas');
  if (!canvas) {
    console.error('Canvas not found!');
    return;
  }

  const canvasCtx = canvas.getContext('2d');

  let dataSize = 0;
  if (data instanceof Blob) {
    dataSize = data.size;
  } else if (data instanceof ArrayBuffer) {
    dataSize = data.byteLength;
  } else if (data && data.byteLength) {
    dataSize = data.byteLength;
  }

  let isJpeg = false;
  let headerHex = '';
  let jpegData = data;

  if (data instanceof ArrayBuffer && data.byteLength > 10) {
    const header = new Uint8Array(data, 0, 10);
    isJpeg = header[0] === 0xFF && header[1] === 0xD8;
    headerHex = Array.from(header).map(b => b.toString(16).padStart(2, '0')).join(' ');

    if (!isJpeg && header[4] === 0xFF && header[5] === 0xD8) {
      jpegData = data.slice(4);
      isJpeg = true;
      debug('📷 Stripped 4-byte prefix from frame');
    }
  }

  debug(`📷 Frame received: ${dataSize} bytes, isJPEG: ${isJpeg}, header: ${headerHex}`);

  if (dataSize < 100) {
    console.error('❌ Frame too small, likely corrupt:', dataSize);
    return;
  }

  const blob = jpegData instanceof Blob ? jpegData : new Blob([jpegData], { type: 'image/jpeg' });

  // Hide overlays
  const previewIdle = document.getElementById('previewIdle');
  const previewConnecting = document.getElementById('previewConnecting');
  if (previewIdle) previewIdle.style.display = 'none';
  if (previewConnecting) previewConnecting.style.display = 'none';

  const img = new Image();
  img.onload = () => {
    // Store screen size on ctx
    if (ctx) {
      ctx.screenWidth = img.width;
      ctx.screenHeight = img.height;
    }

    if (canvas.width !== img.width || canvas.height !== img.height) {
      canvas.width = img.width;
      canvas.height = img.height;
      debug(`📐 Canvas resized to ${img.width}x${img.height}`);
    }

    canvasCtx.drawImage(img, 0, 0);

    // Store frame in SessionManager for tab switching (every ~10th frame)
    if (ctx && window.SessionManager && Math.random() < 0.1) {
      const reader = new FileReader();
      reader.onloadend = () => {
        const base64 = reader.result.split(',')[1];
        if (base64) {
          window.SessionManager.storeFrame(ctx.id, base64);
        }
      };
      reader.readAsDataURL(blob);
    }

    URL.revokeObjectURL(img.src);
  };

  img.onerror = (e) => {
    console.error('Failed to load image:', e);
    URL.revokeObjectURL(img.src);
  };

  img.src = URL.createObjectURL(blob);
}

// Display a dirty region (partial screen update) on canvas
function displayDirtyRegion(data, x, y, w, h) {
  const canvas = document.getElementById('previewCanvas') || document.getElementById('remoteCanvas');
  if (!canvas) {
    console.error('Canvas not found!');
    return;
  }

  if (canvas.width === 0 || canvas.height === 0) {
    console.warn('Canvas not initialized, skipping dirty region');
    return;
  }

  const ctx = canvas.getContext('2d');
  const blob = new Blob([data], { type: 'image/jpeg' });

  const img = new Image();
  img.onload = () => {
    ctx.drawImage(img, x, y);
    URL.revokeObjectURL(img.src);
  };

  img.onerror = (e) => {
    console.error('Failed to load dirty region:', e);
    URL.revokeObjectURL(img.src);
  };

  img.src = URL.createObjectURL(blob);
}

// Fullscreen functionality
document.addEventListener('DOMContentLoaded', () => {
  const fullscreenBtn = document.getElementById('fullscreenBtn');
  const viewerContainer = document.getElementById('viewerContainer');

  if (fullscreenBtn && viewerContainer) {
    fullscreenBtn.addEventListener('click', () => {
      if (!document.fullscreenElement) {
        viewerContainer.requestFullscreen().catch(err => {
          console.error('Failed to enter fullscreen:', err);
        });
      } else {
        document.exitFullscreen();
      }
    });

    document.addEventListener('fullscreenchange', () => {
      if (document.fullscreenElement) {
        fullscreenBtn.textContent = '⛶';
        fullscreenBtn.title = 'Exit Fullscreen (Esc)';
      } else {
        fullscreenBtn.textContent = '⛶';
        fullscreenBtn.title = 'Fullscreen (F11)';
      }
    });
  }
});

// ==================== CLIPBOARD SYNC ====================

function handleAgentMessage(msg) {
  if (!msg.type) return;

  switch (msg.type) {
    case 'monitor_list':
      handleMonitorList(msg);
      break;

    case 'monitor_switched':
      handleMonitorSwitched(msg);
      break;

    case 'clipboard_text':
      if (msg.content) {
        navigator.clipboard.writeText(msg.content).then(() => {
          debug('📋 Clipboard received from agent (text:', msg.content.length, 'bytes)');
        }).catch(err => {
          console.warn('Failed to write clipboard:', err);
        });
      }
      break;

    case 'clipboard_image':
      if (msg.content) {
        try {
          const binary = atob(msg.content);
          const bytes = new Uint8Array(binary.length);
          for (let i = 0; i < binary.length; i++) {
            bytes[i] = binary.charCodeAt(i);
          }
          const blob = new Blob([bytes], { type: 'image/png' });
          navigator.clipboard.write([
            new ClipboardItem({ 'image/png': blob })
          ]).then(() => {
            debug('📋 Clipboard received from agent (image:', bytes.length, 'bytes)');
          }).catch(err => {
            console.warn('Failed to write image clipboard:', err);
          });
        } catch (e) {
          console.warn('Failed to decode clipboard image:', e);
        }
      }
      break;
  }
}

// Send clipboard to agent (uses active session's data channel)
async function sendClipboardToAgent() {
  const dc = getActiveDataChannel();
  if (!dc || dc.readyState !== 'open') return;

  try {
    const text = await navigator.clipboard.readText();
    if (text) {
      sendControlEvent({
        type: 'clipboard_text',
        content: text
      });
      debug('📋 Clipboard sent to agent (text:', text.length, 'bytes)');
      return;
    }
  } catch (e) {
    // Text read failed, try image
  }

  try {
    const items = await navigator.clipboard.read();
    for (const item of items) {
      if (item.types.includes('image/png')) {
        const blob = await item.getType('image/png');
        const buffer = await blob.arrayBuffer();
        const base64 = btoa(String.fromCharCode(...new Uint8Array(buffer)));
        sendControlEvent({
          type: 'clipboard_image',
          content: base64
        });
        debug('📋 Clipboard sent to agent (image:', buffer.byteLength, 'bytes');
        return;
      }
    }
  } catch (e) {
    console.warn('Failed to read clipboard:', e);
  }
}

// ==================== MULTI-MONITOR ====================

function handleMonitorList(msg) {
  const monitors = msg.monitors || [];
  const active = msg.active || 0;

  debug(`📺 Monitor list received: ${monitors.length} monitors, active: ${active}`);

  // Store on active session ctx
  const ctx = window.SessionManager?.getActiveSession();
  if (ctx) {
    ctx.monitors = monitors;
    ctx.activeMonitor = active;
  }

  // Populate monitor selector dropdown
  const select = document.getElementById('monitorSelect');
  if (!select) return;

  select.innerHTML = '';
  monitors.forEach(mon => {
    const opt = document.createElement('option');
    opt.value = mon.index;
    const label = mon.primary ? `${mon.name} (${mon.width}x${mon.height}) ★` : `${mon.name} (${mon.width}x${mon.height})`;
    opt.textContent = label;
    if (mon.index === active) opt.selected = true;
    select.appendChild(opt);
  });

  // Show/hide selector based on monitor count
  const container = document.getElementById('monitorSelectContainer');
  if (container) {
    container.style.display = monitors.length > 1 ? 'inline-flex' : 'none';
  }
}

function handleMonitorSwitched(msg) {
  const index = msg.index;
  const width = msg.width;
  const height = msg.height;

  debug(`📺 Monitor switched to ${index}: ${width}x${height}`);

  // Update canvas size
  const canvas = document.getElementById('previewCanvas');
  if (canvas) {
    canvas.width = width;
    canvas.height = height;
  }

  // Update active session ctx
  const ctx = window.SessionManager?.getActiveSession();
  if (ctx) {
    ctx.activeMonitor = index;
    ctx.screenWidth = width;
    ctx.screenHeight = height;
  }

  // Update dropdown selection
  const select = document.getElementById('monitorSelect');
  if (select) select.value = index;

  showToast(`Skiftet til monitor ${index + 1} (${width}x${height})`, 'success');
}

// Export
window.initWebRTC = initWebRTC;
window.sendClipboardToAgent = sendClipboardToAgent;
