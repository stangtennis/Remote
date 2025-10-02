// WebRTC Connection Module
// Handles peer connection, media tracks, and data channels

let peerConnection = null;
let dataChannel = null;

async function initWebRTC(session) {
  try {
    // Create peer connection with TURN servers from session
    const configuration = session.turn_config || {
      iceServers: [
        { urls: 'stun:stun.l.google.com:19302' },
        { urls: 'stun:stun1.l.google.com:19302' }
      ]
    };

    peerConnection = new RTCPeerConnection(configuration);
    window.peerConnection = peerConnection; // Expose globally for signaling module

    // Set up event handlers
    setupPeerConnectionHandlers();

    // Create data channel for control inputs
    dataChannel = peerConnection.createDataChannel('control', {
      ordered: true
    });
    setupDataChannelHandlers();

    // Create offer
    const offer = await peerConnection.createOffer({
      offerToReceiveVideo: true,
      offerToReceiveAudio: false
    });

    await peerConnection.setLocalDescription(offer);

    // Send offer via signaling
    await sendSignal({
      session_id: session.session_id,
      from: 'dashboard',
      type: 'offer',
      sdp: offer.sdp
    });

    console.log('WebRTC offer sent');

  } catch (error) {
    console.error('WebRTC initialization failed:', error);
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
    console.log('Connection state:', state);
    
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

function setupDataChannelHandlers() {
  dataChannel.onopen = () => {
    console.log('Data channel opened');
    // Enable mouse/keyboard input
    setupInputCapture();
  };

  dataChannel.onclose = () => {
    console.log('Data channel closed');
  };

  dataChannel.onerror = (error) => {
    console.error('Data channel error:', error);
  };

  dataChannel.onmessage = (event) => {
    // Receive JPEG frame from agent
    if (event.data instanceof ArrayBuffer || event.data instanceof Blob) {
      displayVideoFrame(event.data);
    }
  };
}

function setupInputCapture() {
  const remoteVideo = document.getElementById('remoteVideo');
  const remoteCanvas = document.getElementById('remoteCanvas');
  const target = remoteCanvas || remoteVideo;

  if (!target) return;

  // Mouse move
  target.addEventListener('mousemove', (e) => {
    if (!dataChannel || dataChannel.readyState !== 'open') return;
    
    const rect = target.getBoundingClientRect();
    const x = (e.clientX - rect.left) / rect.width;
    const y = (e.clientY - rect.top) / rect.height;
    
    sendControlEvent({
      t: 'mouse_move',
      x: Math.round(x * 10000) / 10000,
      y: Math.round(y * 10000) / 10000
    });
  });

  // Mouse click
  target.addEventListener('mousedown', (e) => {
    if (!dataChannel || dataChannel.readyState !== 'open') return;
    
    const button = ['left', 'middle', 'right'][e.button] || 'left';
    sendControlEvent({
      t: 'mouse_click',
      button,
      down: true
    });
    e.preventDefault();
  });

  target.addEventListener('mouseup', (e) => {
    if (!dataChannel || dataChannel.readyState !== 'open') return;
    
    const button = ['left', 'middle', 'right'][e.button] || 'left';
    sendControlEvent({
      t: 'mouse_click',
      button,
      down: false
    });
    e.preventDefault();
  });

  // Keyboard (when viewer is focused)
  target.tabIndex = 0; // Make focusable
  
  target.addEventListener('keydown', (e) => {
    if (!dataChannel || dataChannel.readyState !== 'open') return;
    
    sendControlEvent({
      t: 'key',
      code: e.code,
      down: true
    });
    e.preventDefault();
  });

  target.addEventListener('keyup', (e) => {
    if (!dataChannel || dataChannel.readyState !== 'open') return;
    
    sendControlEvent({
      t: 'key',
      code: e.code,
      down: false
    });
    e.preventDefault();
  });

  console.log('Input capture enabled');
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

    stats.forEach(report => {
      if (report.type === 'candidate-pair' && report.state === 'succeeded') {
        if (report.localCandidate && report.remoteCandidate) {
          const localType = report.localCandidate.candidateType || 'unknown';
          const remoteType = report.remoteCandidate.candidateType || 'unknown';
          
          if (localType === 'relay' || remoteType === 'relay') {
            connectionType = 'TURN (Relayed)';
          } else if (localType === 'srflx' || remoteType === 'srflx') {
            connectionType = 'P2P (STUN)';
          } else {
            connectionType = 'P2P (Direct)';
          }
        }
      }
    });

    document.getElementById('statConnectionType').textContent = connectionType;

  } catch (error) {
    console.error('Failed to get connection type:', error);
  }
}

// Display video frame on canvas
function displayVideoFrame(data) {
  const canvas = document.getElementById('remoteCanvas');
  if (!canvas) return;

  const ctx = canvas.getContext('2d');
  
  // Convert data to blob if it's an ArrayBuffer
  const blob = data instanceof Blob ? data : new Blob([data], { type: 'image/jpeg' });
  
  // Create image from blob
  const img = new Image();
  img.onload = () => {
    // Resize canvas to match image
    canvas.width = img.width;
    canvas.height = img.height;
    
    // Draw image on canvas
    ctx.drawImage(img, 0, 0);
    
    // Clean up
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

// Export
window.initWebRTC = initWebRTC;
window.peerConnection = peerConnection;
