// Electron Agent - Remote Desktop with Full Control
// Based on web-agent.js but with Electron native control APIs

// Supabase configuration
const SUPABASE_URL = 'https://mnqtdugcvfyenjuqruol.supabase.co';
const SUPABASE_ANON_KEY = 'REDACTED_JWT';

// Initialize Supabase client
const supabase = window.supabase.createClient(SUPABASE_URL, SUPABASE_ANON_KEY);

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
let controlEnabled = true; // Remote control is always enabled in Electron

// ICE Configuration
const iceConfig = {
  iceServers: [
    { urls: 'stun:stun.l.google.com:19302' },
    { urls: 'stun:stun1.l.google.com:19302' },
    { urls: 'stun:stun2.l.google.com:19302' }
  ]
};

// Display platform info
document.getElementById('platformInfo').textContent = 
  `Electron on ${window.electronAPI.platform}`;

console.log('💻 Electron Agent loaded');
console.log('🎮 Remote Control: ENABLED');

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
      alert('⏸️ Your account is pending approval.\\n\\nPlease wait for an administrator to approve your account.');
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

async function generateDeviceID() {
  // Generate unique device ID for Electron
  const data = `electron-${window.electronAPI.platform}-${currentUser.id}`;
  
  const encoder = new TextEncoder();
  const dataBuffer = encoder.encode(data);
  const hashBuffer = await crypto.subtle.digest('SHA-256', dataBuffer);
  const hashArray = Array.from(new Uint8Array(hashBuffer));
  const hashHex = hashArray.map(b => b.toString(16).padStart(2, '0')).join('');
  
  return 'electron-' + hashHex.substring(0, 16);
}

async function registerDevice() {
  const deviceName = `Electron - ${window.electronAPI.platform}`;

  try {
    const generatedDeviceId = await generateDeviceID();
    
    const { data: existing, error: checkError } = await supabase
      .from('remote_devices')
      .select('device_id')
      .eq('device_id', generatedDeviceId)
      .maybeSingle();
    
    if (existing) {
      console.log('Device already exists, updating...');
      const { data, error } = await supabase
        .from('remote_devices')
        .update({
          device_name: deviceName,
          last_seen: new Date().toISOString(),
          is_online: true
        })
        .eq('device_id', generatedDeviceId)
        .select()
        .single();
      
      if (error) throw error;
      deviceId = data.device_id;
    } else {
      const { data, error} = await supabase
        .from('remote_devices')
        .insert({
          device_id: generatedDeviceId,
          device_name: deviceName,
          platform: 'electron',
          owner_id: currentUser.id,
          last_seen: new Date().toISOString(),
          is_online: true
        })
        .select()
        .single();

      if (error) throw error;
      deviceId = data.device_id;
    }

    document.getElementById('deviceName').textContent = deviceName;
    document.getElementById('statusBadge').textContent = 'Online';
    document.getElementById('statusBadge').className = 'status-badge online';

    console.log('✅ Device registered:', deviceId);

    startHeartbeat();
    startSessionPolling();

  } catch (error) {
    console.error('Device registration failed:', error);
    alert('Failed to register device: ' + error.message);
  }
}

function startHeartbeat() {
  heartbeatInterval = setInterval(async () => {
    if (!deviceId) return;

    try {
      await supabase
        .from('remote_devices')
        .update({ 
          last_seen: new Date().toISOString(),
          is_online: true
        })
        .eq('device_id', deviceId);
    } catch (error) {
      console.error('Heartbeat failed:', error);
    }
  }, 30000);

  console.log('✅ Heartbeat started');
}

// ============================================================================
// Screen Capture (Electron)
// ============================================================================

async function startSharing() {
  try {
    console.log('📹 Requesting screen capture...');

    // Get available sources from Electron
    const sources = await window.electronAPI.getSources();
    
    if (!sources || sources.length === 0) {
      throw new Error('No screen sources available');
    }

    // Use the first screen (primary monitor)
    const primaryScreen = sources.find(s => s.id.startsWith('screen')) || sources[0];

    // Get media stream using Electron's desktopCapturer
    mediaStream = await navigator.mediaDevices.getUserMedia({
      audio: false,
      video: {
        mandatory: {
          chromeMediaSource: 'desktop',
          chromeMediaSourceId: primaryScreen.id,
          minWidth: 1280,
          maxWidth: 1920,
          minHeight: 720,
          maxHeight: 1080
        }
      }
    });

    console.log('✅ Screen capture started');

    const preview = document.getElementById('preview');
    preview.srcObject = mediaStream;

    document.getElementById('startBtn').classList.add('hidden');
    document.getElementById('stopBtn').classList.remove('hidden');
    document.getElementById('statusBadge').textContent = 'Sharing';
    document.getElementById('statusBadge').className = 'status-badge sharing';

    mediaStream.getVideoTracks()[0].addEventListener('ended', () => {
      console.log('Screen sharing stopped');
      stopSharing();
    });

  } catch (error) {
    console.error('Screen capture failed:', error);
    alert('Failed to start screen sharing: ' + error.message);
  }
}

async function stopSharing() {
  console.log('🛑 Stopping screen share...');

  if (mediaStream) {
    mediaStream.getTracks().forEach(track => track.stop());
    mediaStream = null;
  }

  if (peerConnection) {
    peerConnection.close();
    peerConnection = null;
  }

  const preview = document.getElementById('preview');
  preview.srcObject = null;

  if (currentSession) {
    await endSession();
  }

  document.getElementById('startBtn').classList.remove('hidden');
  document.getElementById('stopBtn').classList.add('hidden');
  document.getElementById('statusBadge').textContent = 'Online';
  document.getElementById('statusBadge').className = 'status-badge online';
  document.getElementById('connectedSection').classList.add('hidden');

  console.log('✅ Screen sharing stopped');
}

// ============================================================================
// Session Management
// ============================================================================

function startSessionPolling() {
  sessionPollInterval = setInterval(async () => {
    if (!deviceId || currentSession || !mediaStream) return;

    try {
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
  }, 2000);

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
    await supabase
      .from('remote_sessions')
      .update({ status: 'active' })
      .eq('id', currentSession.id);

    document.getElementById('sessionSection').classList.add('hidden');
    await startWebRTC();
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

  if (peerConnection) {
    peerConnection.close();
    peerConnection = null;
  }

  if (signalingChannel) {
    supabase.removeChannel(signalingChannel);
    signalingChannel = null;
  }

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
    peerConnection = new RTCPeerConnection(iceConfig);

    if (!mediaStream) {
      throw new Error('No media stream available');
    }

    mediaStream.getTracks().forEach(track => {
      peerConnection.addTrack(track, mediaStream);
      console.log('Added track:', track.kind);
    });

    // Create data channel for remote control
    dataChannel = peerConnection.createDataChannel('input');
    dataChannel.onopen = () => {
      console.log('✅ Data channel opened - remote control ready');
    };
    dataChannel.onmessage = handleRemoteInput;

    peerConnection.onicecandidate = async (event) => {
      if (event.candidate) {
        console.log('📤 Sending ICE candidate');
        await supabase
          .from('session_signaling')
          .insert({
            session_id: currentSession.id,
            from_side: 'agent',
            msg_type: 'ice',
            payload: event.candidate
          });
      }
    };

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

    const offer = await peerConnection.createOffer({
      offerToReceiveVideo: false,
      offerToReceiveAudio: false
    });

    await peerConnection.setLocalDescription(offer);
    console.log('📤 Sending offer');

    await supabase
      .from('session_signaling')
      .insert({
        session_id: currentSession.id,
        from_side: 'agent',
        msg_type: 'offer',
        payload: offer
      });

    listenForSignaling();

    console.log('✅ WebRTC connection initiated');

  } catch (error) {
    console.error('WebRTC setup failed:', error);
    throw error;
  }
}

function listenForSignaling() {
  signalingChannel = supabase
    .channel(`session_${currentSession.id}`)
    .on('postgres_changes', {
      event: 'INSERT',
      schema: 'public',
      table: 'session_signaling',
      filter: `session_id=eq.${currentSession.id}`
    }, async (payload) => {
      const msg = payload.new;

      if (msg.from_side === 'agent') return;

      console.log('📥 Received signaling:', msg.msg_type);

      const data = msg.payload;

      try {
        if (msg.msg_type === 'answer') {
          await peerConnection.setRemoteDescription(new RTCSessionDescription(data));
          console.log('✅ Answer received and set');
        } else if (msg.msg_type === 'ice') {
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

// ============================================================================
// Remote Control Input Handler (Electron-specific)
// ============================================================================

async function handleRemoteInput(event) {
  if (!controlEnabled) {
    console.warn('⚠️ Remote control is disabled');
    return;
  }

  try {
    const data = JSON.parse(event.data);
    console.log('🎮 Input event received:', data.type);

    switch (data.type) {
      case 'mouse_move':
        await window.electronAPI.control.mouseMove(data.x, data.y);
        break;

      case 'mouse_click':
        await window.electronAPI.control.mouseClick(data.button || 'left', data.double || false);
        break;

      case 'mouse_down':
        await window.electronAPI.control.mouseButton(data.button || 'left', 'down');
        break;

      case 'mouse_up':
        await window.electronAPI.control.mouseButton(data.button || 'left', 'up');
        break;

      case 'mouse_scroll':
        await window.electronAPI.control.mouseScroll(data.deltaX || 0, data.deltaY || 0);
        break;

      case 'keyboard_press':
        await window.electronAPI.control.keyboardPress(data.key, data.modifiers);
        break;

      case 'keyboard_type':
        await window.electronAPI.control.keyboardType(data.text);
        break;

      default:
        console.warn('Unknown input type:', data.type);
    }
  } catch (error) {
    console.error('❌ Failed to handle input:', error);
  }
}

// ============================================================================
// Global Functions
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
