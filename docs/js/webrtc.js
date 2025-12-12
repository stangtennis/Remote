// WebRTC Connection Module
// Handles peer connection, media tracks, and data channels
let peerConnection = null;
let dataChannel = null;

// Clean up existing connection before creating new one
function cleanupWebRTC() {
  console.log('🧹 Cleaning up WebRTC connection...');
  
  // Clean up input capture
  if (typeof cleanupInputCapture === 'function') {
    cleanupInputCapture();
  }
  
  // Close data channel
  if (dataChannel) {
    try {
      dataChannel.close();
    } catch (e) {}
    dataChannel = null;
  }
  window.dataChannel = null;
  
  // Close peer connection
  if (peerConnection) {
    try {
      peerConnection.close();
    } catch (e) {}
    peerConnection = null;
  }
  window.peerConnection = null;
  
  // Reset frame state
  frameChunks = [];
  expectedChunks = 0;
  if (frameTimeout) {
    clearTimeout(frameTimeout);
    frameTimeout = null;
  }
  
  console.log('✅ WebRTC cleanup complete');
}

// Expose cleanup globally
window.cleanupWebRTC = cleanupWebRTC;

async function initWebRTC(session) {
  try {
    console.log('🚀 initWebRTC called with session:', session);
    
    // Clean up any existing connection first
    cleanupWebRTC();
    
    if (!session || !session.session_id) {
      throw new Error('Invalid session object - missing session_id');
    }
    
    // Check if we should force relay mode (for testing TURN)
    const forceRelay = new URLSearchParams(window.location.search).get('relay') === 'true';
    
    // Always use our own TURN server configuration
    const configuration = {
      iceServers: [
        { urls: 'stun:stun.l.google.com:19302' },
        { urls: 'stun:stun1.l.google.com:19302' },
        // Egen TURN server på hawkeye123.dk
        {
          urls: 'turn:188.228.14.94:3478',
          username: 'remotedesktop',
          credential: 'Hawkeye2025Turn!'
        },
        {
          urls: 'turn:188.228.14.94:3478?transport=tcp',
          username: 'remotedesktop',
          credential: 'Hawkeye2025Turn!'
        }
      ],
      // Force relay mode if ?relay=true in URL (for testing)
      ...(forceRelay && { iceTransportPolicy: 'relay' })
    };
    
    if (forceRelay) {
      console.log('⚠️ RELAY-ONLY MODE ENABLED (for testing)');
    }

    console.log('🔐 Dashboard TURN config:', JSON.stringify(configuration, null, 2));

    peerConnection = new RTCPeerConnection(configuration);
    window.peerConnection = peerConnection; // Expose globally for signaling module
    console.log('✅ PeerConnection created');

    // Set up event handlers
    setupPeerConnectionHandlers();
    console.log('✅ Event handlers set up');

    // Create data channel for control inputs
    dataChannel = peerConnection.createDataChannel('control', {
      ordered: true
    });
    setupDataChannelHandlers();
    console.log('✅ Data channel created');

    // Create offer
    console.log('📝 Creating offer...');
    const offer = await peerConnection.createOffer({
      offerToReceiveVideo: true,
      offerToReceiveAudio: false
    });
    console.log('✅ Offer created');

    console.log('📝 Setting local description...');
    await peerConnection.setLocalDescription(offer);
    console.log('✅ Local description set');

    // Send offer via signaling
    console.log('📤 Sending offer to agent via signaling...');
    await sendSignal({
      session_id: session.session_id,
      from: 'dashboard',
      type: 'offer',
      sdp: offer.sdp
    });
    console.log('✅ WebRTC offer sent successfully!');

  } catch (error) {
    console.error('❌ WebRTC initialization failed:', error);
    console.error('Error stack:', error.stack);
    throw error;
  }
}

function setupPeerConnectionHandlers() {
  // ICE candidate handler
  peerConnection.onicecandidate = async (event) => {
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
      
      console.log(`📤 Sending ICE candidate [${candidateType}]:`, candidateStr.substring(0, 80) + '...');
      
      if (!window.currentSession) {
        console.error('⚠️ Cannot send ICE candidate: currentSession is null');
        return;
      }
      
      await sendSignal({
        session_id: window.currentSession.session_id,
        from: 'dashboard',
        type: 'ice',
        candidate: event.candidate
      });
    } else {
      console.log('📤 ICE gathering complete (null candidate)');
    }
  };

  // ICE connection state handler
  peerConnection.oniceconnectionstatechange = () => {
    console.log('ICE connection state:', peerConnection.iceConnectionState);
  };

  // ICE gathering state handler
  peerConnection.onicegatheringstatechange = () => {
    console.log('ICE gathering state:', peerConnection.iceGatheringState);
  };

  // Connection state handler
  peerConnection.onconnectionstatechange = () => {
    const state = peerConnection.connectionState;
    console.log('❗ Connection state:', state);
    console.log('❗ ICE state:', peerConnection.iceConnectionState);
    console.log('❗ Signaling state:', peerConnection.signalingState);
    
    const statusElement = document.getElementById('sessionStatus');
    const overlay = document.getElementById('viewerOverlay');

    // Update SessionManager if available
    const deviceId = window.currentSession?.device_id;
    if (window.SessionManager && deviceId) {
      const sessionStatus = state === 'connected' ? 'connected' : 
                           state === 'connecting' ? 'connecting' : 'disconnected';
      window.SessionManager.updateSessionStatus(deviceId, sessionStatus);
    }

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
        updateConnectionStats();
        // Stop polling since we're connected
        if (window.stopPolling) {
          window.stopPolling();
          console.log('🛑 Stopped signaling polling (connection established)');
        }
        break;
      case 'disconnected':
        if (statusElement) {
          statusElement.textContent = 'Disconnected';
          statusElement.className = 'status-badge offline';
        }
        break;
      case 'failed':
        if (statusElement) {
          statusElement.textContent = 'Connection Failed';
          statusElement.className = 'status-badge offline';
        }
        if (overlay) {
          overlay.style.display = 'flex';
          overlay.innerHTML = '<p>Connection failed. Please try again.</p>';
        }
        break;
    }
  };

  // Track handler (remote video/canvas)
  peerConnection.ontrack = (event) => {
    console.log('Remote track received:', event.track.kind);
    const remoteVideo = document.getElementById('remoteVideo');
    if (remoteVideo && event.streams[0]) {
      remoteVideo.srcObject = event.streams[0];
    }
  };

  // ICE connection state handler
  peerConnection.oniceconnectionstatechange = () => {
    console.log('ICE state:', peerConnection.iceConnectionState);
    
    if (peerConnection.iceConnectionState === 'connected') {
      updateConnectionType();
    }
  };
}

// Frame reassembly state
let frameChunks = [];
let expectedChunks = 0;
let frameTimeout = null;
let framesReceived = 0;
let framesDropped = 0;

// Bandwidth tracking
let bytesReceived = 0;
let lastBandwidthCheck = Date.now();
let currentBandwidthMbps = 0;

// Screen size tracking for dirty regions (set by first full frame)
let screenWidth = 0;
let screenHeight = 0;

function setupDataChannelHandlers() {
  dataChannel.onopen = () => {
    console.log('Data channel opened');
    // Enable mouse/keyboard input
    setupInputCapture();
  };

  dataChannel.onclose = () => {
    console.log('Data channel closed');
    cleanupInputCapture();
  };

  dataChannel.onerror = (error) => {
    console.error('Data channel error:', error);
  };

  dataChannel.onmessage = async (event) => {
    // Track bandwidth
    let dataSize = 0;
    if (event.data instanceof ArrayBuffer) {
      dataSize = event.data.byteLength;
    } else if (event.data instanceof Blob) {
      dataSize = event.data.size;
    } else if (typeof event.data === 'string') {
      dataSize = event.data.length;
    }
    bytesReceived += dataSize;
    
    // Receive JPEG frame from agent (possibly chunked)
    if (event.data instanceof ArrayBuffer) {
      const data = new Uint8Array(event.data);
      
      // Check if this is JSON (starts with '{' = 0x7B)
      if (data.length > 0 && data[0] === 0x7B) {
        // This is a JSON message, not a frame
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
      const FRAME_TYPE_REGION = 0x02;  // Dirty region update
      const CHUNK_MAGIC = 0xFF;        // Chunked frame marker
      
      // Check for dirty region (type 0x02)
      if (data.length > 9 && data[0] === FRAME_TYPE_REGION) {
        // Dirty region: [type(1), x(2), y(2), w(2), h(2), ...jpeg_data]
        const x = data[1] | (data[2] << 8);
        const y = data[3] | (data[4] << 8);
        const w = data[5] | (data[6] << 8);
        const h = data[7] | (data[8] << 8);
        const jpegData = data.slice(9);
        displayDirtyRegion(jpegData.buffer, x, y, w, h);
        framesReceived++;
      }
      // Check for chunked frame (0xFF followed by chunk index, not 0xD8)
      else if (data.length > 3 && data[0] === CHUNK_MAGIC && data[1] !== 0xD8) {
        const chunkIndex = data[1];
        const totalChunks = data[2];
        const chunkData = data.slice(3);
        
        // Initialize chunk array if first chunk
        if (chunkIndex === 0) {
          if (frameChunks.length > 0 && expectedChunks > 0) {
            framesDropped++;
          }
          frameChunks = new Array(totalChunks);
          expectedChunks = totalChunks;
          
          if (frameTimeout) clearTimeout(frameTimeout);
          frameTimeout = setTimeout(() => {
            if (expectedChunks > 0) {
              framesDropped++;
              frameChunks = [];
              expectedChunks = 0;
            }
          }, 500);
        }
        
        // Store this chunk
        if (expectedChunks > 0 && chunkIndex < expectedChunks) {
          frameChunks[chunkIndex] = chunkData;
          
          const receivedCount = frameChunks.filter(c => c).length;
          if (receivedCount === expectedChunks) {
            if (frameTimeout) {
              clearTimeout(frameTimeout);
              frameTimeout = null;
            }
            
            // Reassemble frame
            const totalLength = frameChunks.reduce((sum, chunk) => sum + chunk.length, 0);
            const completeFrame = new Uint8Array(totalLength);
            let offset = 0;
            for (const chunk of frameChunks) {
              completeFrame.set(chunk, offset);
              offset += chunk.length;
            }
            
            // Display the complete reassembled frame (full JPEG)
            displayVideoFrame(completeFrame.buffer);
            framesReceived++;
            
            frameChunks = [];
            expectedChunks = 0;
          }
        }
      } else {
        // Single-packet frame (raw JPEG starting with FF D8)
        displayVideoFrame(event.data);
        framesReceived++;
      }
    } else if (event.data instanceof Blob) {
      displayVideoFrame(event.data);
      framesReceived++;
    } else if (typeof event.data === 'string') {
      try {
        const msg = JSON.parse(event.data);
        handleAgentMessage(msg);
      } catch (e) {
        console.warn('Failed to parse message:', e);
      }
    }
  };
  
  // Calculate and display bandwidth every second
  setInterval(() => {
    const now = Date.now();
    const elapsed = (now - lastBandwidthCheck) / 1000; // seconds
    
    if (elapsed > 0 && bytesReceived > 0) {
      const bitsPerSecond = (bytesReceived * 8) / elapsed;
      currentBandwidthMbps = bitsPerSecond / 1000000;
      
      // Update UI
      updateBandwidthDisplay(currentBandwidthMbps, framesReceived / elapsed);
      
      // Log to console
      console.log(`📊 Bandwidth: ${currentBandwidthMbps.toFixed(2)} Mbit/s | FPS: ${(framesReceived / elapsed).toFixed(1)} | Dropped: ${framesDropped}`);
    }
    
    // Reset counters
    bytesReceived = 0;
    framesReceived = 0;
    framesDropped = 0;
    lastBandwidthCheck = now;
  }, 1000);
}

// Update bandwidth display in UI
function updateBandwidthDisplay(mbps, fps) {
  // Update stats display if it exists
  const statsEl = document.getElementById('bandwidthStats');
  if (statsEl) {
    statsEl.textContent = `${mbps.toFixed(1)} Mbit/s | ${fps.toFixed(0)} FPS`;
  }
  
  // Also update connection info if available
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
  return currentBandwidthMbps;
}

// Helper function to calculate actual image area within canvas (accounting for object-fit: contain)
function getImageCoordinates(element, clientX, clientY) {
  const rect = element.getBoundingClientRect();
  
  // Get displayed size (CSS size on screen)
  const displayWidth = rect.width;
  const displayHeight = rect.height;
  
  // Get actual canvas/image size (internal pixel buffer)
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
  
  // If canvas has no content yet, use display size
  if (actualWidth === 0 || actualHeight === 0) {
    actualWidth = displayWidth;
    actualHeight = displayHeight;
  }
  
  // Calculate coordinates relative to element's top-left
  const relX = clientX - rect.left;
  const relY = clientY - rect.top;
  
  // For canvas with object-fit: contain, the image is scaled to fit
  // Calculate the scale factor and offset
  const scaleX = displayWidth / actualWidth;
  const scaleY = displayHeight / actualHeight;
  const scale = Math.min(scaleX, scaleY); // object-fit: contain uses the smaller scale
  
  // Calculate rendered size
  const renderWidth = actualWidth * scale;
  const renderHeight = actualHeight * scale;
  
  // Calculate offset (centering)
  const offsetX = (displayWidth - renderWidth) / 2;
  const offsetY = (displayHeight - renderHeight) / 2;
  
  // Map click position to normalized coordinates (0-1)
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
  
  // Prevent duplicate event listeners (would cause double input!)
  if (inputListenersAttached) {
    console.log('Input capture already enabled, skipping duplicate setup');
    return;
  }
  inputListenersAttached = true;
  
  // Focus canvas for keyboard input
  target.focus();
  console.log('🎯 Canvas focused for keyboard input');

  // Prevent context menu
  const contextMenuHandler = (e) => {
    e.preventDefault();
    e.stopPropagation();
  };
  target.addEventListener('contextmenu', contextMenuHandler);
  inputEventHandlers.contextMenu = contextMenuHandler;

  // Mouse move with throttling to prevent overwhelming the connection
  let lastMouseMove = 0;
  const mouseMoveHandler = (e) => {
    if (!dataChannel || dataChannel.readyState !== 'open') return;
    
    // Throttle to max 60 FPS (16ms) to reduce network load
    const now = Date.now();
    if (now - lastMouseMove < 16) return;
    lastMouseMove = now;
    
    const coords = getImageCoordinates(target, e.clientX, e.clientY);
    
    sendControlEvent({
      t: 'mouse_move',
      x: Math.round(coords.x * 10000) / 10000,
      y: Math.round(coords.y * 10000) / 10000,
      rel: true  // Flag for relative coordinates (0-1)
    });
  };
  target.addEventListener('mousemove', mouseMoveHandler);
  inputEventHandlers.mouseMove = mouseMoveHandler;

  // Mouse click
  const mouseDownHandler = (e) => {
    if (!dataChannel || dataChannel.readyState !== 'open') return;
    
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
    if (!dataChannel || dataChannel.readyState !== 'open') return;
    
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

  // Mouse wheel / scroll
  const wheelHandler = (e) => {
    if (!dataChannel || dataChannel.readyState !== 'open') return;
    
    sendControlEvent({
      t: 'mouse_scroll',
      delta: e.deltaY > 0 ? -1 : 1  // Negative for down, positive for up
    });
    e.preventDefault();
  };
  target.addEventListener('wheel', wheelHandler);
  inputEventHandlers.wheel = wheelHandler;

  // Keyboard (when viewer is focused)
  target.tabIndex = 0; // Make focusable
  target.style.outline = 'none'; // Remove focus outline
  
  // Auto-focus on click
  const clickHandler = () => {
    target.focus();
  };
  target.addEventListener('click', clickHandler);
  inputEventHandlers.click = clickHandler;
  
  // Track pressed keys to prevent duplicates from key repeat
  const pressedKeys = new Set();
  
  const keyDownHandler = async (e) => {
    if (!dataChannel || dataChannel.readyState !== 'open') return;
    
    // Handle Ctrl+V - paste from local clipboard to agent
    if (e.ctrlKey && e.code === 'KeyV') {
      e.preventDefault();
      e.stopPropagation();
      await sendClipboardToAgent();
      return;
    }
    
    // Ignore key repeat events (only send first press)
    if (pressedKeys.has(e.code)) return;
    pressedKeys.add(e.code);
    
    // Send modifier state with each key press for better compatibility
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
    if (!dataChannel || dataChannel.readyState !== 'open') return;
    
    // Remove from pressed keys
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

  console.log('✅ Input capture enabled (duplicate prevention active)');
}

// Clean up input capture when connection closes
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
  console.log('🧹 Input capture cleaned up');
}

function sendControlEvent(event) {
  if (dataChannel && dataChannel.readyState === 'open') {
    dataChannel.send(JSON.stringify(event));
  }
}

// Export for use in other modules
window.sendControlEvent = sendControlEvent;

async function updateConnectionStats() {
  if (!peerConnection) return;

  try {
    const stats = await peerConnection.getStats();
    let bitrate = 0;
    let rtt = 0;
    let packetLoss = 0;

    stats.forEach(report => {
      if (report.type === 'inbound-rtp' && report.kind === 'video') {
        bitrate = Math.round((report.bytesReceived * 8) / 1000); // kbps
        packetLoss = report.packetsLost || 0;
      }
      if (report.type === 'candidate-pair' && report.state === 'succeeded') {
        rtt = report.currentRoundTripTime ? 
          Math.round(report.currentRoundTripTime * 1000) : 0;
      }
    });

    // Update UI
    document.getElementById('statBitrate').textContent = bitrate + ' kbps';
    document.getElementById('statRtt').textContent = rtt + ' ms';
    document.getElementById('statPacketLoss').textContent = packetLoss + ' packets';

  } catch (error) {
    console.error('Failed to get stats:', error);
  }
}

async function updateConnectionType() {
  if (!peerConnection) return;

  try {
    const stats = await peerConnection.getStats();
    let connectionType = 'Unknown';

    // First, find the active candidate pair
    let activePair = null;
    stats.forEach(report => {
      if (report.type === 'candidate-pair' && report.state === 'succeeded' && report.nominated) {
        activePair = report;
      }
    });

    // If we found an active pair, look up the candidate details
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

      console.log(`🔗 Connection: local=${localType}, remote=${remoteType}`);

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
function displayVideoFrame(data) {
  // Use preview canvas (new dashboard) or remote canvas (old)
  const canvas = document.getElementById('previewCanvas') || document.getElementById('remoteCanvas');
  if (!canvas) {
    console.error('Canvas not found!');
    return;
  }

  const ctx = canvas.getContext('2d');
  
  // Debug: log data info
  let dataSize = 0;
  if (data instanceof Blob) {
    dataSize = data.size;
  } else if (data instanceof ArrayBuffer) {
    dataSize = data.byteLength;
  } else if (data && data.byteLength) {
    dataSize = data.byteLength;
  }
  
  // Check if data looks like JPEG (starts with 0xFF 0xD8)
  let isJpeg = false;
  let headerHex = '';
  let jpegData = data;
  
  if (data instanceof ArrayBuffer && data.byteLength > 10) {
    const header = new Uint8Array(data, 0, 10);
    isJpeg = header[0] === 0xFF && header[1] === 0xD8;
    headerHex = Array.from(header).map(b => b.toString(16).padStart(2, '0')).join(' ');
    
    // Check for 4-byte prefix before JPEG (agent sends frame type prefix)
    if (!isJpeg && header[4] === 0xFF && header[5] === 0xD8) {
      // Skip 4-byte prefix
      jpegData = data.slice(4);
      isJpeg = true;
      console.log('📷 Stripped 4-byte prefix from frame');
    }
  }
  
  console.log(`📷 Frame received: ${dataSize} bytes, isJPEG: ${isJpeg}, header: ${headerHex}`);
  
  if (dataSize < 100) {
    console.error('❌ Frame too small, likely corrupt:', dataSize);
    return;
  }
  
  // Convert data to blob if it's an ArrayBuffer
  const blob = jpegData instanceof Blob ? jpegData : new Blob([jpegData], { type: 'image/jpeg' });
  
  // Hide overlays immediately when we start receiving frames
  const previewIdle = document.getElementById('previewIdle');
  const previewConnecting = document.getElementById('previewConnecting');
  if (previewIdle) previewIdle.style.display = 'none';
  if (previewConnecting) previewConnecting.style.display = 'none';
  
  // Create image from blob
  const img = new Image();
  img.onload = () => {
    // Store screen size for dirty region calculations
    screenWidth = img.width;
    screenHeight = img.height;
    
    // Resize canvas to match image (only for full frames)
    if (canvas.width !== img.width || canvas.height !== img.height) {
      canvas.width = img.width;
      canvas.height = img.height;
      console.log(`📐 Canvas resized to ${img.width}x${img.height}`);
    }
    
    // Draw image on canvas
    ctx.drawImage(img, 0, 0);
    
    // Store frame in SessionManager for tab switching
    const deviceId = window.currentSession?.device_id;
    if (window.SessionManager && deviceId) {
      // Convert to base64 for storage (only every 10th frame to save memory)
      if (Math.random() < 0.1) {
        // Use FileReader to avoid stack overflow with large frames
        const reader = new FileReader();
        reader.onloadend = () => {
          const base64 = reader.result.split(',')[1]; // Remove data:image/jpeg;base64, prefix
          if (base64) {
            window.SessionManager.storeFrame(deviceId, base64);
          }
        };
        reader.readAsDataURL(blob);
      }
    }
    
    // Clean up
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

  // Don't draw if canvas hasn't been initialized with a full frame yet
  if (canvas.width === 0 || canvas.height === 0) {
    console.warn('Canvas not initialized, skipping dirty region');
    return;
  }

  const ctx = canvas.getContext('2d');
  
  // Convert data to blob
  const blob = new Blob([data], { type: 'image/jpeg' });
  
  // Create image from blob
  const img = new Image();
  img.onload = () => {
    // Draw the region at the specified position (don't resize canvas!)
    // The image should be drawn at its natural size at position (x, y)
    ctx.drawImage(img, x, y);
    
    // Clean up
    URL.revokeObjectURL(img.src);
  };
  
  img.onerror = (e) => {
    console.error('Failed to load dirty region:', e);
    URL.revokeObjectURL(img.src);
  };
  
  img.src = URL.createObjectURL(blob);
}

// Update stats every 2 seconds when connected
setInterval(() => {
  if (peerConnection && peerConnection.connectionState === 'connected') {
    updateConnectionStats();
  }
}, 2000);

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

    // Update button text when fullscreen changes
    document.addEventListener('fullscreenchange', () => {
      if (document.fullscreenElement) {
        fullscreenBtn.textContent = '⛶'; // Exit fullscreen icon
        fullscreenBtn.title = 'Exit Fullscreen (Esc)';
      } else {
        fullscreenBtn.textContent = '⛶'; // Fullscreen icon
        fullscreenBtn.title = 'Fullscreen (F11)';
      }
    });
  }
});

// ==================== CLIPBOARD SYNC ====================

// Handle messages from agent (clipboard, etc.)
function handleAgentMessage(msg) {
  if (!msg.type) return;
  
  switch (msg.type) {
    case 'clipboard_text':
      if (msg.content) {
        // Write text to local clipboard
        navigator.clipboard.writeText(msg.content).then(() => {
          console.log('📋 Clipboard received from agent (text:', msg.content.length, 'bytes)');
        }).catch(err => {
          console.warn('Failed to write clipboard:', err);
        });
      }
      break;
      
    case 'clipboard_image':
      if (msg.content) {
        // Decode base64 image and write to clipboard
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
            console.log('📋 Clipboard received from agent (image:', bytes.length, 'bytes)');
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

// Send clipboard to agent
async function sendClipboardToAgent() {
  if (!dataChannel || dataChannel.readyState !== 'open') return;
  
  try {
    // Try to read text first
    const text = await navigator.clipboard.readText();
    if (text) {
      sendControlEvent({
        type: 'clipboard_text',
        content: text
      });
      console.log('📋 Clipboard sent to agent (text:', text.length, 'bytes)');
      return;
    }
  } catch (e) {
    // Text read failed, try image
  }
  
  try {
    // Try to read image
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
        console.log('📋 Clipboard sent to agent (image:', buffer.byteLength, 'bytes)');
        return;
      }
    }
  } catch (e) {
    console.warn('Failed to read clipboard:', e);
  }
}

// Export
window.initWebRTC = initWebRTC;
window.peerConnection = peerConnection;
window.sendClipboardToAgent = sendClipboardToAgent;
