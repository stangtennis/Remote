// Web Agent - Browser-based remote desktop agent
import { supabase } from './supabase.js';

// State
let currentUser = null;
let deviceId = null;
let currentSession = null;
let mediaStream = null;
let peerConnection = null;
let dataChannel = null;
let heartbeatInterval = null;
let sessionPollInterval = null;
let signalingChannel = null;

// ICE Configuration (STUN/TURN servers)
const iceConfig = {
  iceServers: [
    { urls: 'stun:stun.l.google.com:19302' },
    { urls: 'stun:stun1.l.google.com:19302' },
    { urls: 'stun:stun2.l.google.com:19302' }
    // Add TURN servers here if needed
  ]
};

// ============================================================================
// Authentication
// ============================================================================

async function login() {
  const email = document.getElementById('email').value.trim();
  const password = document.getElementById('password').value;

  if (!email || !password) {
    alert('Please enter both email and password');
    return;
  }

  try {
    const { data, error } = await supabase.auth.signInWithPassword({
      email,
      password
    });

    if (error) throw error;

    currentUser = data.user;
    console.log('✅ Logged in as:', currentUser.email);

    // Check if user is approved
    const { data: approval, error: approvalError } = await supabase
      .from('user_approvals')
      .select('approved')
      .eq('user_id', currentUser.id)
      .single();

    if (approvalError) {
      console.error('Error checking approval:', approvalError);
      alert('Error checking account approval. Please contact support.');
      await supabase.auth.signOut();
      return;
    }

    if (!approval || !approval.approved) {
      alert('⏸️ Your account is pending approval.\n\nPlease wait for an administrator to approve your account before you can use the web agent.');
      await supabase.auth.signOut();
      return;
    }

    // Register device
    await registerDevice();

    // Update UI
    document.getElementById('loginSection').classList.add('hidden');
    document.getElementById('deviceSection').classList.remove('hidden');
    document.getElementById('userEmail').textContent = currentUser.email;

  } catch (error) {
    console.error('Login error:', error);
    alert('Login failed: ' + error.message);
  }
}

async function logout() {
  // Stop sharing if active
  await stopSharing();

  // Clear device
  if (deviceId) {
    await supabase
      .from('remote_devices')
      .delete()
      .eq('device_id', deviceId);
  }

  // Clear intervals
  if (heartbeatInterval) clearInterval(heartbeatInterval);
  if (sessionPollInterval) clearInterval(sessionPollInterval);

  // Sign out
  await supabase.auth.signOut();

  // Reset UI
  document.getElementById('deviceSection').classList.add('hidden');
  document.getElementById('loginSection').classList.remove('hidden');
  document.getElementById('email').value = '';
  document.getElementById('password').value = '';

  currentUser = null;
  deviceId = null;

  console.log('✅ Logged out');
}

// ============================================================================
// Device Registration
// ============================================================================

async function registerDevice() {
  const deviceName = `Web - ${navigator.platform}`;
  const browserInfo = getBrowserInfo();

  try {
    const { data, error } = await supabase
      .from('remote_devices')
      .insert({
        device_name: deviceName,
        platform: 'web',
        browser: browserInfo,
        owner_id: currentUser.id,
        last_heartbeat: new Date().toISOString()
      })
      .select()
      .single();

    if (error) throw error;

    deviceId = data.device_id;
    document.getElementById('deviceName').textContent = deviceName;
    document.getElementById('browserInfo').textContent = browserInfo;
    document.getElementById('statusBadge').textContent = 'Online';
    document.getElementById('statusBadge').className = 'status-badge online';

    console.log('✅ Device registered:', deviceId);

    // Start heartbeat
    startHeartbeat();

    // Start polling for sessions
    startSessionPolling();

  } catch (error) {
    console.error('Device registration failed:', error);
    alert('Failed to register device: ' + error.message);
  }
}

function getBrowserInfo() {
  const ua = navigator.userAgent;
  let browser = 'Unknown';

  if (ua.includes('Chrome') && !ua.includes('Edg')) browser = 'Chrome';
  else if (ua.includes('Edg')) browser = 'Edge';
  else if (ua.includes('Firefox')) browser = 'Firefox';
  else if (ua.includes('Safari') && !ua.includes('Chrome')) browser = 'Safari';

  return `${browser} (${navigator.platform})`;
}

function startHeartbeat() {
  heartbeatInterval = setInterval(async () => {
    if (!deviceId) return;

    try {
      await supabase
        .from('remote_devices')
        .update({ last_heartbeat: new Date().toISOString() })
        .eq('device_id', deviceId);
    } catch (error) {
      console.error('Heartbeat failed:', error);
    }
  }, 30000); // Every 30 seconds

  console.log('✅ Heartbeat started');
}

// ============================================================================
// Screen Capture
// ============================================================================

async function startSharing() {
  try {
    console.log('📹 Requesting screen capture...');

    // Request screen capture permission
    mediaStream = await navigator.mediaDevices.getDisplayMedia({
      video: {
        cursor: 'always', // Show cursor
        displaySurface: 'monitor', // Prefer full screen
        width: { ideal: 1920 },
        height: { ideal: 1080 },
        frameRate: { ideal: 15, max: 30 }
      },
      audio: false
    });

    console.log('✅ Screen capture started');

    // Show preview
    const preview = document.getElementById('preview');
    preview.srcObject = mediaStream;

    // Update UI
    document.getElementById('startBtn').classList.add('hidden');
    document.getElementById('stopBtn').classList.remove('hidden');
    document.getElementById('statusBadge').textContent = 'Sharing';
    document.getElementById('statusBadge').className = 'status-badge sharing';

    // Listen for user stopping the share (via browser controls)
    mediaStream.getVideoTracks()[0].addEventListener('ended', () => {
      console.log('User stopped sharing via browser');
      stopSharing();
    });

    // Check for extension (for remote control)
    checkExtensionAvailable();

  } catch (error) {
    console.error('Screen capture failed:', error);
    if (error.name === 'NotAllowedError') {
      alert('❌ Screen sharing permission denied.\n\nPlease allow screen sharing to use the web agent.');
    } else {
      alert('Failed to start screen sharing: ' + error.message);
    }
  }
}

async function stopSharing() {
  console.log('🛑 Stopping screen share...');

  // Stop media stream
  if (mediaStream) {
    mediaStream.getTracks().forEach(track => track.stop());
    mediaStream = null;
  }

  // Close peer connection
  if (peerConnection) {
    peerConnection.close();
    peerConnection = null;
  }

  // Clear preview
  const preview = document.getElementById('preview');
  preview.srcObject = null;

  // End session if active
  if (currentSession) {
    await endSession();
  }

  // Update UI
  document.getElementById('startBtn').classList.remove('hidden');
  document.getElementById('stopBtn').classList.add('hidden');
  document.getElementById('statusBadge').textContent = 'Online';
  document.getElementById('statusBadge').className = 'status-badge online';
  document.getElementById('connectedSection').classList.add('hidden');

  console.log('✅ Screen sharing stopped');
}

function checkExtensionAvailable() {
  // Check if browser extension is installed
  // This will be used for remote control in Phase 2
  window.addEventListener('message', (event) => {
    if (event.data.type === 'extension_ready') {
      console.log('✅ Extension detected - remote control available');
      const controlStatus = document.getElementById('controlStatus');
      controlStatus.classList.remove('hidden', 'disabled');
      controlStatus.classList.add('enabled');
      document.getElementById('controlText').textContent = '🎮 Remote Control: Enabled';
      controlStatus.querySelector('small').textContent = 'Full remote control available';
    }
  });
}

// ============================================================================
// Session Management
// ============================================================================

function startSessionPolling() {
  sessionPollInterval = setInterval(async () => {
    if (!deviceId || currentSession || !mediaStream) return;

    try {
      // Check for pending sessions
      const { data, error } = await supabase
        .from('remote_sessions')
        .select('*')
        .eq('device_id', deviceId)
        .eq('status', 'pending')
        .order('created_at', { ascending: false })
        .limit(1);

      if (error) throw error;

      if (data && data.length > 0) {
        currentSession = data[0];
        console.log('📞 Incoming connection request');
        showPinPrompt();
      }
    } catch (error) {
      console.error('Session poll error:', error);
    }
  }, 2000); // Check every 2 seconds

  console.log('✅ Session polling started');
}

function showPinPrompt() {
  document.getElementById('deviceSection').classList.add('hidden');
  document.getElementById('sessionSection').classList.remove('hidden');
  document.getElementById('pinInput').focus();
}

async function acceptSession() {
  const pin = document.getElementById('pinInput').value.trim();

  if (!pin || pin.length !== 6) {
    alert('Please enter the 6-digit PIN');
    return;
  }

  if (pin !== currentSession.pin) {
    alert('❌ Invalid PIN. Please check and try again.');
    document.getElementById('pinInput').value = '';
    return;
  }

  console.log('✅ PIN accepted, starting session...');

  try {
    // Update session status
    await supabase
      .from('remote_sessions')
      .update({
        status: 'active',
        started_at: new Date().toISOString()
      })
      .eq('id', currentSession.id);

    // Hide PIN prompt
    document.getElementById('sessionSection').classList.add('hidden');

    // Start WebRTC connection
    await startWebRTC();

    // Show connected section
    document.getElementById('connectedSection').classList.remove('hidden');
    document.getElementById('sessionStart').textContent = new Date().toLocaleString();

  } catch (error) {
    console.error('Failed to accept session:', error);
    alert('Failed to start session: ' + error.message);
    rejectSession();
  }
}

function rejectSession() {
  console.log('❌ Session rejected');

  if (currentSession) {
    supabase
      .from('remote_sessions')
      .update({ status: 'rejected' })
      .eq('id', currentSession.id)
      .then(() => console.log('Session marked as rejected'));
  }

  currentSession = null;
  document.getElementById('sessionSection').classList.add('hidden');
  document.getElementById('deviceSection').classList.remove('hidden');
  document.getElementById('pinInput').value = '';
}

async function endSession() {
  console.log('🛑 Ending session...');

  // Close WebRTC
  if (peerConnection) {
    peerConnection.close();
    peerConnection = null;
  }

  // Unsubscribe from signaling
  if (signalingChannel) {
    supabase.removeChannel(signalingChannel);
    signalingChannel = null;
  }

  // Update session status
  if (currentSession) {
    await supabase
      .from('remote_sessions')
      .update({
        status: 'ended',
        ended_at: new Date().toISOString()
      })
      .eq('id', currentSession.id);

    currentSession = null;
  }

  // Update UI
  document.getElementById('connectedSection').classList.add('hidden');
  document.getElementById('deviceSection').classList.remove('hidden');

  console.log('✅ Session ended');
}

// ============================================================================
// WebRTC Connection
// ============================================================================

async function startWebRTC() {
  console.log('🔗 Starting WebRTC connection...');

  try {
    // Create peer connection
    peerConnection = new RTCPeerConnection(iceConfig);

    // Add screen stream to peer connection
    if (!mediaStream) {
      throw new Error('No media stream available');
    }

    mediaStream.getTracks().forEach(track => {
      peerConnection.addTrack(track, mediaStream);
      console.log('Added track:', track.kind);
    });

    // Create data channel for receiving input (Phase 2)
    dataChannel = peerConnection.createDataChannel('input');
    dataChannel.onopen = () => console.log('✅ Data channel opened');
    dataChannel.onmessage = handleRemoteInput;

    // Handle ICE candidates
    peerConnection.onicecandidate = async (event) => {
      if (event.candidate) {
        console.log('📤 Sending ICE candidate');
        await supabase
          .from('session_signaling')
          .insert({
            session_id: currentSession.id,
            type: 'ice_candidate',
            data: JSON.stringify(event.candidate),
            from_agent: true
          });
      }
    };

    // Handle connection state changes
    peerConnection.onconnectionstatechange = () => {
      console.log('Connection state:', peerConnection.connectionState);
      document.getElementById('connectionStatus').textContent =
        `Connection: ${peerConnection.connectionState}`;

      if (peerConnection.connectionState === 'disconnected' ||
          peerConnection.connectionState === 'failed') {
        console.warn('⚠️ Connection lost');
        endSession();
      }
    };

    // Create offer
    const offer = await peerConnection.createOffer({
      offerToReceiveVideo: false,
      offerToReceiveAudio: false
    });

    await peerConnection.setLocalDescription(offer);
    console.log('📤 Sending offer');

    // Send offer to dashboard
    await supabase
      .from('session_signaling')
      .insert({
        session_id: currentSession.id,
        type: 'offer',
        data: JSON.stringify(offer),
        from_agent: true
      });

    // Listen for answer and ICE candidates
    listenForSignaling();

    console.log('✅ WebRTC connection initiated');

  } catch (error) {
    console.error('WebRTC setup failed:', error);
    throw error;
  }
}

function listenForSignaling() {
  // Subscribe to signaling messages
  signalingChannel = supabase
    .channel(`session_${currentSession.id}`)
    .on('postgres_changes', {
      event: 'INSERT',
      schema: 'public',
      table: 'session_signaling',
      filter: `session_id=eq.${currentSession.id}`
    }, async (payload) => {
      const msg = payload.new;

      // Skip our own messages
      if (msg.from_agent) return;

      console.log('📥 Received signaling:', msg.type);

      const data = JSON.parse(msg.data);

      try {
        if (msg.type === 'answer') {
          await peerConnection.setRemoteDescription(new RTCSessionDescription(data));
          console.log('✅ Answer received and set');
        } else if (msg.type === 'ice_candidate') {
          await peerConnection.addIceCandidate(new RTCIceCandidate(data));
          console.log('✅ ICE candidate added');
        }
      } catch (error) {
        console.error('Signaling error:', error);
      }
    })
    .subscribe();

  console.log('✅ Listening for signaling messages');
}

function handleRemoteInput(event) {
  // Handle remote input commands (Phase 2 - requires extension + helper)
  try {
    const input = JSON.parse(event.data);
    console.log('🎮 Remote input:', input.type);

    // Forward to extension if available
    window.postMessage({
      type: 'input_command',
      command: input
    }, '*');

  } catch (error) {
    console.error('Input handling error:', error);
  }
}

// ============================================================================
// Global Functions (called from HTML)
// ============================================================================

window.login = login;
window.logout = logout;
window.startSharing = startSharing;
window.stopSharing = stopSharing;
window.acceptSession = acceptSession;
window.rejectSession = rejectSession;
window.endSession = endSession;

// ============================================================================
// Initialization
// ============================================================================

console.log('🌐 Web Agent loaded');
console.log('Platform:', navigator.platform);
console.log('Browser:', getBrowserInfo());

// Check if already logged in
supabase.auth.getSession().then(({ data: { session } }) => {
  if (session) {
    console.log('Already logged in, initializing...');
    currentUser = session.user;
    registerDevice().then(() => {
      document.getElementById('loginSection').classList.add('hidden');
      document.getElementById('deviceSection').classList.remove('hidden');
      document.getElementById('userEmail').textContent = currentUser.email;
    });
  }
});
