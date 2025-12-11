// Web Agent - Browser-based remote desktop agent

// Supabase configuration
const SUPABASE_URL = 'https://supabase.hawkeye123.dk';
const SUPABASE_ANON_KEY = 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyAgCiAgICAicm9sZSI6ICJhbm9uIiwKICAgICJpc3MiOiAic3VwYWJhc2UtZGVtbyIsCiAgICAiaWF0IjogMTY0MTc2OTIwMCwKICAgICJleHAiOiAxNzk5NTM1NjAwCn0.dc_X5iR_VP_qT0zsiyj_I_OZ2T9FtRU2BBNWN8Bu4GE';

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

// ICE Configuration (STUN/TURN servers)
const iceConfig = {
  iceServers: [
    { urls: 'stun:stun.l.google.com:19302' },
    { urls: 'stun:stun1.l.google.com:19302' },
    // TURN server for NAT traversal
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
  ]
};

// ============================================================================
// Authentication
// ============================================================================

async function login() {
  const email = document.getElementById('email').value.trim();
  const password = document.getElementById('password').value;

  if (!email || !password) {
    showMessage('Please enter both email and password', 'error');
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
      showMessage('Error checking account approval. Please contact support.', 'error');
      await supabase.auth.signOut();
      return;
    }

    if (!approval || !approval.approved) {
      showMessage('⏸️ Your account is pending approval. Please wait for an administrator to approve your account.', 'error');
      await supabase.auth.signOut();
      return;
    }

    // Register device
    try {
      await registerDevice();
    } catch (regError) {
      console.warn('Device registration warning:', regError);
      // Continue anyway - device section will still show
    }

    // Update UI
    document.getElementById('loginSection').style.display = 'none';
    document.getElementById('deviceSection').style.display = 'block';
    const userEmailEl = document.getElementById('userEmail');
    if (userEmailEl) userEmailEl.textContent = currentUser.email;
    
    // Update header status
    updateHeaderStatus('online', 'Connected');

    // Check if input helper is running
    checkHelperStatus();

  } catch (error) {
    console.error('Login error:', error);
    showMessage('Login failed: ' + error.message, 'error');
  }
}

async function signup() {
  const email = document.getElementById('signupEmail').value.trim();
  const password = document.getElementById('signupPassword').value;
  const passwordConfirm = document.getElementById('signupPasswordConfirm').value;

  if (!email || !password) {
    showMessage('Please enter email and password', 'error');
    return;
  }

  if (password !== passwordConfirm) {
    showMessage('Passwords do not match', 'error');
    return;
  }

  if (password.length < 6) {
    showMessage('Password must be at least 6 characters', 'error');
    return;
  }

  try {
    const { data, error } = await supabase.auth.signUp({
      email,
      password
    });

    if (error) throw error;

    console.log('✅ Account created:', email);
    showMessage('✅ Account created! Please wait for admin approval before logging in.', 'success');
    
    // Clear form and switch back to login after 3 seconds
    document.getElementById('signupEmail').value = '';
    document.getElementById('signupPassword').value = '';
    document.getElementById('signupPasswordConfirm').value = '';
    
    setTimeout(() => {
      showLogin();
    }, 3000);

  } catch (error) {
    console.error('Signup error:', error);
    showMessage('Signup failed: ' + error.message, 'error');
  }
}

function showSignup() {
  document.getElementById('loginForm').style.display = 'none';
  document.getElementById('signupForm').style.display = 'block';
  document.getElementById('authTitle').textContent = 'Create Account';
  document.getElementById('authSubtitle').textContent = 'Register to share your screen';
  document.getElementById('authToggleText').innerHTML = 'Already have an account? <a href="#" onclick="showLogin(); return false;" class="link-primary">Sign in</a>';
  document.getElementById('loginMessage').style.display = 'none';
}

function showLogin() {
  document.getElementById('signupForm').style.display = 'none';
  document.getElementById('loginForm').style.display = 'block';
  document.getElementById('authTitle').textContent = 'Sign In';
  document.getElementById('authSubtitle').textContent = 'Login to start sharing your screen';
  document.getElementById('authToggleText').innerHTML = 'Don\'t have an account? <a href="#" onclick="showSignup(); return false;" class="link-primary">Create one</a>';
  document.getElementById('loginMessage').style.display = 'none';
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
  document.getElementById('deviceSection').style.display = 'none';
  document.getElementById('sessionSection').style.display = 'none';
  document.getElementById('connectedSection').style.display = 'none';
  document.getElementById('loginSection').style.display = 'block';
  document.getElementById('email').value = '';
  document.getElementById('password').value = '';
  
  // Update header status
  updateHeaderStatus('offline', 'Not Connected');

  currentUser = null;
  deviceId = null;

  console.log('✅ Logged out');
}

// ============================================================================
// Device Registration
// ============================================================================

async function generateDeviceID() {
  // Generate unique device ID based on browser fingerprint
  const data = `${navigator.userAgent}-${navigator.platform}-${currentUser.id}`;
  
  // Create SHA-256 hash
  const encoder = new TextEncoder();
  const dataBuffer = encoder.encode(data);
  const hashBuffer = await crypto.subtle.digest('SHA-256', dataBuffer);
  const hashArray = Array.from(new Uint8Array(hashBuffer));
  const hashHex = hashArray.map(b => b.toString(16).padStart(2, '0')).join('');
  
  return 'web-' + hashHex.substring(0, 16); // First 16 chars
}

async function registerDevice() {
  const browserInfo = getBrowserInfo();
  const deviceName = `Web - ${browserInfo}`;

  try {
    // Generate device ID
    const generatedDeviceId = await generateDeviceID();
    
    // Check if device already exists
    const { data: existing, error: checkError } = await supabase
      .from('remote_devices')
      .select('device_id')
      .eq('device_id', generatedDeviceId)
      .maybeSingle(); // Use maybeSingle instead of single to handle 0 or 1 results
    
    if (existing) {
      // Device exists - update it
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
      // Device doesn't exist - insert it
      const { data, error } = await supabase
        .from('remote_devices')
        .insert({
          device_id: generatedDeviceId,
          device_name: deviceName,
          platform: 'web',
          owner_id: currentUser.id,
          last_seen: new Date().toISOString(),
          is_online: true
        })
        .select()
        .single();

      if (error) throw error;
      deviceId = data.device_id;
    }

    // Update UI elements (with null checks for missing elements)
    const deviceNameEl = document.getElementById('deviceName');
    const browserInfoEl = document.getElementById('browserInfo');
    if (deviceNameEl) deviceNameEl.textContent = deviceName;
    if (browserInfoEl) browserInfoEl.textContent = browserInfo;

    console.log('✅ Device registered:', deviceId);

    // Start heartbeat
    startHeartbeat();

    // Start polling for sessions
    startSessionPolling();

  } catch (error) {
    console.error('Device registration failed:', error);
    // Don't show alert - just log the error
    throw error; // Re-throw so caller can handle it
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
        .update({ 
          last_seen: new Date().toISOString(),
          is_online: true
        })
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
    document.getElementById('startBtn').style.display = 'none';
    document.getElementById('stopBtn').style.display = 'inline-flex';
    document.getElementById('previewWindow').style.display = 'block';
    updateHeaderStatus('online', 'Sharing Screen');

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
  document.getElementById('startBtn').style.display = 'inline-flex';
  document.getElementById('stopBtn').style.display = 'none';
  document.getElementById('previewWindow').style.display = 'none';
  document.getElementById('connectedSection').style.display = 'none';
  updateHeaderStatus('online', 'Connected');
  stopSessionTimer();

  console.log('✅ Screen sharing stopped');
}

function checkExtensionAvailable() {
  // Check if browser extension is installed
  // This will be used for remote control in Phase 2
  window.addEventListener('message', (event) => {
    if (event.data.type === 'extension_ready') {
      console.log('✅ Extension detected - remote control available');
      // Extension detected - could enable additional features here
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
  document.getElementById('deviceSection').style.display = 'none';
  document.getElementById('sessionSection').style.display = 'block';
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
        status: 'active'
      })
      .eq('id', currentSession.id);

    // Hide PIN prompt
    document.getElementById('sessionSection').style.display = 'none';

    // Start WebRTC connection
    await startWebRTC();

    // Connect to local input helper (for remote control)
    connectToHelper();

    // Show connected section
    document.getElementById('connectedSection').style.display = 'block';
    updateHeaderStatus('online', 'Session Active');
    startSessionTimer();

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
  document.getElementById('sessionSection').style.display = 'none';
  document.getElementById('deviceSection').style.display = 'block';
  document.getElementById('pinInput').value = '';
  updateHeaderStatus('online', 'Connected');
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
  document.getElementById('connectedSection').style.display = 'none';
  document.getElementById('deviceSection').style.display = 'block';
  updateHeaderStatus('online', 'Connected');
  stopSessionTimer();

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
            from_side: 'agent',
            msg_type: 'ice',
            payload: event.candidate
          });
      }
    };

    // Handle connection state changes
    peerConnection.onconnectionstatechange = () => {
      console.log('Connection state:', peerConnection.connectionState);
      const connStatusEl = document.getElementById('connectionStatus');
      if (connStatusEl) {
        connStatusEl.textContent = `Connection: ${peerConnection.connectionState}`;
      }
      // Update connection quality indicator
      const qualityEl = document.getElementById('connectionQuality');
      if (qualityEl) {
        qualityEl.textContent = peerConnection.connectionState === 'connected' ? 'Excellent' : peerConnection.connectionState;
      }

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
        from_side: 'agent',
        msg_type: 'offer',
        payload: offer
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

      // Skip our own messages (from agent)
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
// Input Helper Connection (Local WebSocket)
// ============================================================================

let helperWs = null;
let helperConnected = false;
let helperReconnectTimer = null;
let inputSeq = 0;

const HELPER_URL = 'ws://127.0.0.1:9877/input';
const HELPER_STATUS_URL = 'http://127.0.0.1:9877/status';

// Check if helper is running (HTTP status check)
async function checkHelperStatus() {
  try {
    const response = await fetch(HELPER_STATUS_URL, { 
      method: 'GET',
      mode: 'cors'
    });
    if (response.ok) {
      const data = await response.json();
      updateHelperUI(true, data.version || 'v1.0.0');
      return true;
    }
  } catch (e) {
    // Helper not running
  }
  updateHelperUI(false);
  return false;
}

// Update helper UI elements
function updateHelperUI(connected, version = null) {
  const statusText = document.getElementById('helperStatusText');
  const downloadBtn = document.getElementById('downloadHelperBtn');
  const connectedBadge = document.getElementById('helperConnectedBadge');
  const helperHint = document.getElementById('helperHint');
  const helperIcon = document.querySelector('.helper-icon');

  if (connected) {
    if (statusText) statusText.textContent = `Connected (${version})`;
    if (downloadBtn) downloadBtn.style.display = 'none';
    if (connectedBadge) connectedBadge.style.display = 'inline-flex';
    if (helperHint) helperHint.style.display = 'none';
    if (helperIcon) {
      helperIcon.classList.remove('disconnected');
      helperIcon.classList.add('connected');
    }
  } else {
    if (statusText) statusText.textContent = 'Not Running';
    if (downloadBtn) downloadBtn.style.display = 'inline-flex';
    if (connectedBadge) connectedBadge.style.display = 'none';
    if (helperHint) helperHint.style.display = 'block';
    if (helperIcon) {
      helperIcon.classList.remove('connected');
      helperIcon.classList.add('disconnected');
    }
  }
}

async function connectToHelper() {
  if (helperWs && helperWs.readyState === WebSocket.OPEN) {
    return true;
  }

  return new Promise((resolve) => {
    try {
      helperWs = new WebSocket(HELPER_URL);

      helperWs.onopen = () => {
        console.log('✅ Connected to Input Helper');
        helperConnected = true;
        updateHelperUI(true);

        // Authenticate with helper
        const authMsg = {
          type: 'auth',
          token: currentUser?.id || 'anonymous',
          device_id: deviceId,
          session_id: currentSession?.id || ''
        };
        helperWs.send(JSON.stringify(authMsg));

        resolve(true);
      };

      helperWs.onclose = () => {
        console.log('🔌 Input Helper disconnected');
        helperConnected = false;
        helperWs = null;
        updateHelperUI(false);

        // Retry connection after 5 seconds
        if (helperReconnectTimer) clearTimeout(helperReconnectTimer);
        helperReconnectTimer = setTimeout(() => {
          if (currentSession) {
            connectToHelper();
          }
        }, 5000);
      };

      helperWs.onerror = (err) => {
        console.warn('⚠️ Input Helper not available (run input-helper.exe locally)');
        helperConnected = false;
        updateHelperUI(false);
        resolve(false);
      };

      helperWs.onmessage = (event) => {
        try {
          const msg = JSON.parse(event.data);
          handleHelperMessage(msg);
        } catch (e) {
          console.error('Invalid helper message:', e);
        }
      };

    } catch (e) {
      console.warn('Failed to connect to helper:', e);
      resolve(false);
    }
  });
}

function handleHelperMessage(msg) {
  switch (msg.type) {
    case 'status':
      console.log('📊 Helper status:', msg);
      break;
    case 'ack':
      if (!msg.ok && msg.error) {
        console.warn('Helper error:', msg.error);
      }
      break;
    case 'clipboard_content':
      // Send clipboard to remote viewer
      if (dataChannel && dataChannel.readyState === 'open') {
        dataChannel.send(JSON.stringify({
          type: 'clipboard_text',
          content: msg.content
        }));
      }
      break;
  }
}

function sendToHelper(event) {
  if (!helperWs || helperWs.readyState !== WebSocket.OPEN) {
    return false;
  }

  event.seq = ++inputSeq;
  event.ts = Date.now();

  try {
    helperWs.send(JSON.stringify(event));
    return true;
  } catch (e) {
    console.error('Failed to send to helper:', e);
    return false;
  }
}

function handleRemoteInput(event) {
  // Handle remote input commands - forward to local helper
  try {
    const input = JSON.parse(event.data);
    console.log('🎮 Remote input:', input.type);

    // Forward to local helper if connected
    if (helperConnected) {
      sendToHelper(input);
    } else {
      // Try to connect
      connectToHelper().then(connected => {
        if (connected) {
          sendToHelper(input);
        }
      });
    }

  } catch (error) {
    console.error('Input handling error:', error);
  }
}

// ============================================================================
// UI Helper Functions
// ============================================================================

function updateHeaderStatus(status, text) {
  const headerStatus = document.getElementById('headerStatus');
  if (headerStatus) {
    const dot = headerStatus.querySelector('.status-dot');
    const textEl = headerStatus.querySelector('.status-text');
    if (dot) {
      dot.className = 'status-dot ' + status;
    }
    if (textEl) {
      textEl.textContent = text;
    }
  }
}

function showMessage(message, type = 'error') {
  const msgBox = document.getElementById('loginMessage');
  if (msgBox) {
    msgBox.textContent = message;
    msgBox.className = 'message-box ' + type;
    msgBox.style.display = 'block';
    setTimeout(() => {
      msgBox.style.display = 'none';
    }, 5000);
  }
}

let sessionTimer = null;
let sessionStartTime = null;

function startSessionTimer() {
  sessionStartTime = Date.now();
  sessionTimer = setInterval(() => {
    const elapsed = Math.floor((Date.now() - sessionStartTime) / 1000);
    const minutes = Math.floor(elapsed / 60).toString().padStart(2, '0');
    const seconds = (elapsed % 60).toString().padStart(2, '0');
    const durationEl = document.getElementById('sessionDuration');
    if (durationEl) {
      durationEl.textContent = `${minutes}:${seconds}`;
    }
  }, 1000);
}

function stopSessionTimer() {
  if (sessionTimer) {
    clearInterval(sessionTimer);
    sessionTimer = null;
  }
}

// ============================================================================
// Global Functions (called from HTML)
// ============================================================================

window.login = login;
window.logout = logout;
window.signup = signup;
window.showSignup = showSignup;
window.showLogin = showLogin;
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
      document.getElementById('loginSection').style.display = 'none';
      document.getElementById('deviceSection').style.display = 'block';
      const userEmailEl = document.getElementById('userEmail');
      if (userEmailEl) userEmailEl.textContent = currentUser.email;
      updateHeaderStatus('online', 'Connected');
      // Check if input helper is running
      checkHelperStatus();
    }).catch(err => {
      console.error('Device registration error:', err);
      // Still show device section even if registration fails
      document.getElementById('loginSection').style.display = 'none';
      document.getElementById('deviceSection').style.display = 'block';
      // Check if input helper is running
      checkHelperStatus();
      const userEmailEl = document.getElementById('userEmail');
      if (userEmailEl) userEmailEl.textContent = currentUser.email;
      updateHeaderStatus('online', 'Connected');
    });
  }
});
