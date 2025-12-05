// WebRTC Connection Module
// Handles peer connection, media tracks, and data channels
let peerConnection = null;
let dataChannel = null;

async function initWebRTC(session) {
  try {
    console.log('🚀 initWebRTC called with session:', session);
    
    if (!session || !session.session_id) {
      throw new Error('Invalid session object - missing session_id');
    }
    
    // Create peer connection with TURN servers from session
    const configuration = session.turn_config || {
      iceServers: [
        { urls: 'stun:stun.l.google.com:19302' },
        { urls: 'stun1.l.google.com:19302' }
      ]
    };

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
      console.log('📤 Sending ICE candidate:', event.candidate.type, event.candidate.candidate);
      
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

    switch (state) {
      case 'connecting':
        statusElement.textContent = 'Connecting...';
        statusElement.className = 'status-badge pending';
        break;
      case 'connected':
        statusElement.textContent = 'Connected';
        statusElement.className = 'status-badge online';
        overlay.style.display = 'none';
        updateConnectionStats();
        // Stop polling since we're connected
        if (window.stopPolling) {
          window.stopPolling();
          console.log('🛑 Stopped signaling polling (connection established)');
        }
        break;
      case 'disconnected':
        statusElement.textContent = 'Disconnected';
        statusElement.className = 'status-badge offline';
        break;
      case 'failed':
        statusElement.textContent = 'Connection Failed';
        statusElement.className = 'status-badge offline';
        overlay.style.display = 'flex';
        overlay.innerHTML = '<p>Connection failed. Please try again.</p>';
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
    // Receive JPEG frame from agent (possibly chunked)
    if (event.data instanceof ArrayBuffer) {
      const data = new Uint8Array(event.data);
      
      // Check if this is a chunked frame (magic byte 0xFF + 3-byte header)
      const chunkMagic = 0xFF;
      if (data.length > 3 && data[0] === chunkMagic) {
        const chunkIndex = data[1];
        const totalChunks = data[2];
        const chunkData = data.slice(3);
        
        // Initialize chunk array if first chunk
        if (chunkIndex === 0) {
          // Clear any previous incomplete frame
          if (frameChunks.length > 0 && expectedChunks > 0) {
            framesDropped++;
            console.warn('Dropped incomplete frame');
          }
          frameChunks = new Array(totalChunks);
          expectedChunks = totalChunks;
          
          // Clear old timeout
          if (frameTimeout) clearTimeout(frameTimeout);
          
          // Set timeout to discard incomplete frames (500ms)
          frameTimeout = setTimeout(() => {
            if (expectedChunks > 0) {
              framesDropped++;
              console.warn('Frame timeout - discarding incomplete frame');
              frameChunks = [];
              expectedChunks = 0;
            }
          }, 500);
        }
        
        // Store this chunk (if we have a valid array initialized)
        if (expectedChunks > 0 && chunkIndex < expectedChunks) {
          frameChunks[chunkIndex] = chunkData;
          
          // Check if we have all chunks
          const receivedCount = frameChunks.filter(c => c).length;
          if (receivedCount === expectedChunks) {
            // Clear timeout
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
            
            // Display the complete frame
            displayVideoFrame(completeFrame.buffer);
            framesReceived++;
            
            // Reset for next frame
            frameChunks = [];
            expectedChunks = 0;
          }
        }
      } else {
        // Single-packet frame (no chunking)
        displayVideoFrame(event.data);
        framesReceived++;
      }
    } else if (event.data instanceof Blob) {
      displayVideoFrame(event.data);
      framesReceived++;
    } else if (typeof event.data === 'string') {
      // Handle JSON messages (clipboard, etc.)
      try {
        const msg = JSON.parse(event.data);
        handleAgentMessage(msg);
      } catch (e) {
        console.warn('Failed to parse message:', e);
      }
    }
  };
  
  // Log stats every 5 seconds
  setInterval(() => {
    if (framesReceived > 0 || framesDropped > 0) {
      console.log(`📊 Frames: ${framesReceived} received, ${framesDropped} dropped`);
      framesReceived = 0;
      framesDropped = 0;
    }
  }, 5000);
}

// Helper function to calculate actual image area within canvas (accounting for object-fit: contain)
function getImageCoordinates(element, clientX, clientY) {
  const rect = element.getBoundingClientRect();
  
  // Get displayed size
  const displayWidth = rect.width;
  const displayHeight = rect.height;
  
  // Get actual canvas/image size
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
  
  // Calculate aspect ratios
  const displayAspect = displayWidth / displayHeight;
  const imageAspect = actualWidth / actualHeight;
  
  // Calculate actual rendered image dimensions within the element
  let renderWidth, renderHeight, offsetX, offsetY;
  
  if (imageAspect > displayAspect) {
    // Image is wider - letterboxing on top/bottom
    renderWidth = displayWidth;
    renderHeight = displayWidth / imageAspect;
    offsetX = 0;
    offsetY = (displayHeight - renderHeight) / 2;
  } else {
    // Image is taller - letterboxing on left/right
    renderHeight = displayHeight;
    renderWidth = displayHeight * imageAspect;
    offsetX = (displayWidth - renderWidth) / 2;
    offsetY = 0;
  }
  
  // Calculate coordinates relative to element
  const relX = clientX - rect.left;
  const relY = clientY - rect.top;
  
  // Map to actual image area (0-1 range)
  const x = Math.max(0, Math.min(1, (relX - offsetX) / renderWidth));
  const y = Math.max(0, Math.min(1, (relY - offsetY) / renderHeight));
  
  return { x, y };
}

// Store event listeners so we can clean them up
let inputListenersAttached = false;
let inputEventHandlers = {};

function setupInputCapture() {
  const remoteVideo = document.getElementById('remoteVideo');
  const remoteCanvas = document.getElementById('remoteCanvas');
  const target = remoteCanvas || remoteVideo;

  if (!target) return;
  
  // Prevent duplicate event listeners (would cause double input!)
  if (inputListenersAttached) {
    console.log('Input capture already enabled, skipping duplicate setup');
    return;
  }
  inputListenersAttached = true;

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
    
    // Handle Ctrl+C - let the key go through to agent, clipboard will sync back
    // (agent monitors its clipboard and sends changes)
    
    // Ignore key repeat events (only send first press)
    if (pressedKeys.has(e.code)) return;
    pressedKeys.add(e.code);
    
    sendControlEvent({
      t: 'key',
      code: e.code,
      down: true
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
  const canvas = document.getElementById('remoteCanvas');
  if (!canvas) {
    console.error('Canvas not found!');
    return;
  }

  const ctx = canvas.getContext('2d');
  
  // Convert data to blob if it's an ArrayBuffer
  const blob = data instanceof Blob ? data : new Blob([data], { type: 'image/jpeg' });
  
  console.log(`🖼️ Displaying frame: ${blob.size} bytes`);
  
  // Create image from blob
  const img = new Image();
  img.onload = () => {
    // Resize canvas to match image
    canvas.width = img.width;
    canvas.height = img.height;
    
    // Draw image on canvas
    ctx.drawImage(img, 0, 0);
    
    console.log(`✅ Frame drawn: ${img.width}x${img.height}`);
    
    // Clean up
    URL.revokeObjectURL(img.src);
  };
  
  img.onerror = (e) => {
    console.error('Failed to load image:', e);
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
