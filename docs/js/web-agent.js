// Web Agent - Browser-based remote desktop agent

// Supabase client is initialized in config.js
// Access via window.supabase and window.SUPABASE_CONFIG

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
    debug('✅ Logged in as:', currentUser.email);

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

    // Fetch TURN credentials for WebRTC
    await fetchTurnCredentials();

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

    debug('✅ Account created:', email);
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

  debug('✅ Logged out');
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
      debug('Device already exists, updating...');
      const { data, error } = await supabase
        .from('remote_devices')
        .update({
          device_name: deviceName,
          last_seen: new Date().toISOString(),
          is_online: true,
          approved: true
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
          is_online: true,
          approved: true
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

    debug('✅ Device registered:', deviceId);

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

  debug('✅ Heartbeat started');
}

// ============================================================================
// Screen Capture
// ============================================================================

async function startSharing() {
  try {
    debug('📹 Requesting screen capture...');

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

    debug('✅ Screen capture started');

    // Show preview
    const preview = document.getElementById('preview');
    preview.srcObject = mediaStream;

    // Update UI
    document.getElementById('startBtn').style.display = 'none';
    document.getElementById('stopBtn').style.display = 'inline-flex';
    document.getElementById('previewWindow').style.display = 'block';
    document.getElementById('sessionSection').style.display = 'block';
    updateHeaderStatus('online', 'Sharing Screen');

    // Listen for user stopping the share (via browser controls)
    mediaStream.getVideoTracks()[0].addEventListener('ended', () => {
      debug('User stopped sharing via browser');
      stopSharing();
    });

    // Check for extension (for remote control)
    checkExtensionAvailable();

  } catch (error) {
    console.error('Screen capture failed:', error);
    if (error.name === 'NotAllowedError') {
      showToast('Skærmdeling blev afvist. Tillad venligst skærmdeling for at bruge web-agenten.', 'error', 6000);
    } else {
      showToast('Kunne ikke starte skærmdeling: ' + error.message, 'error');
    }
  }
}

async function stopSharing() {
  debug('🛑 Stopping screen share...');

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
  document.getElementById('sessionSection').style.display = 'none';
  updateHeaderStatus('online', 'Connected');
  stopSessionTimer();
  
  // Reset offer tracking so new sessions can be detected
  processedOfferIds.clear();

  debug('✅ Screen sharing stopped');
}

function checkExtensionAvailable() {
  // Check if browser extension is installed
  // This will be used for remote control in Phase 2
  window.addEventListener('message', (event) => {
    if (event.data.type === 'extension_ready') {
      debug('✅ Extension detected - remote control available');
      // Extension detected - could enable additional features here
    }
  });
}

// ============================================================================
// Session Management
// ============================================================================

function startSessionPolling() {
  // Poll session_signaling for offers from dashboard/controller targeting our device.
  // This mirrors the native Go agent's fetchWebDashboardSessions() pattern.
  console.log('[WebAgent] Starting session polling for device:', deviceId);
  
  sessionPollInterval = setInterval(async () => {
    if (!deviceId || currentSession) return;
    if (!mediaStream) return; // Only accept sessions while sharing

    try {
      // Look for offers in session_signaling from dashboard
      const { data: signals, error: sigError } = await supabase
        .from('session_signaling')
        .select('*')
        .eq('msg_type', 'offer')
        .in('from_side', ['dashboard', 'controller'])
        .order('created_at', { ascending: false })
        .limit(10);

      if (sigError) {
        console.error('[WebAgent] Error fetching signals:', sigError);
        throw sigError;
      }
      if (!signals || signals.length === 0) return;

      // For each offer, check if the session belongs to our device
      for (const sig of signals) {
        if (processedOfferIds.has(sig.id)) continue;

        // Look up the session in remote_sessions to verify it targets our device
        const { data: sessions, error: sessError } = await supabase
          .from('remote_sessions')
          .select('id, status, device_id')
          .eq('id', sig.session_id)
          .eq('device_id', deviceId)
          .in('status', ['pending', 'active'])
          .limit(1);

        if (sessError) {
          console.warn('[WebAgent] Session lookup error for', sig.session_id, sessError);
          continue; // Don't mark as processed — retry next cycle
        }
        
        if (!sessions || sessions.length === 0) {
          // Mark as processed only if lookup succeeded but no match (not our device or wrong status)
          processedOfferIds.add(sig.id);
          continue;
        }

        console.log('[WebAgent] 📞 Incoming offer from dashboard for session:', sig.session_id);
        processedOfferIds.add(sig.id);

        // Auto-accept: set session active and start WebRTC
        currentSession = { id: sig.session_id, session_id: sig.session_id };

        await supabase
          .from('remote_sessions')
          .update({ status: 'active' })
          .eq('id', sig.session_id);

        // Parse the offer SDP from the signaling payload
        const offerPayload = sig.payload;
        await handleIncomingOffer(sig.session_id, offerPayload);
        break; // Handle one offer at a time
      }
    } catch (error) {
      console.error('[WebAgent] Session poll error:', error);
    }
  }, 1500);

  console.log('[WebAgent] ✅ Session polling started (listening for dashboard offers)');
}

let processedOfferIds = new Set();

async function handleIncomingOffer(sessionId, offerPayload) {
  console.log('[WebAgent] 🔧 Setting up WebRTC (answering dashboard offer)...');
  console.log('[WebAgent] Offer payload keys:', Object.keys(offerPayload || {}));

  try {
    // Show connected UI, hide waiting card
    document.getElementById('deviceSection').style.display = 'none';
    document.getElementById('sessionSection').style.display = 'none';
    document.getElementById('connectedSection').style.display = 'block';
    updateHeaderStatus('online', 'Session Active');
    startSessionTimer();

    // Start WebRTC as answerer
    await startWebRTC(sessionId, offerPayload);

    // Connect to local input helper (for remote control)
    connectToHelper();

  } catch (error) {
    console.error('Failed to handle incoming offer:', error);
    showToast('Kunne ikke starte session: ' + error.message, 'error');
    await endSession();
  }
}

async function endSession() {
  debug('🛑 Ending session...');

  // Close data channel
  if (dataChannel) {
    try { dataChannel.close(); } catch (e) {}
    dataChannel = null;
  }

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
  
  // Stop signaling polling
  stopSignalingPolling();

  // Disconnect helper
  if (helperWs) {
    try { helperWs.close(); } catch (e) {}
    helperWs = null;
    helperConnected = false;
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
  if (mediaStream) {
    // Still sharing — show waiting card and device section
    document.getElementById('deviceSection').style.display = 'block';
    document.getElementById('sessionSection').style.display = 'block';
    updateHeaderStatus('online', 'Sharing Screen');
  } else {
    document.getElementById('deviceSection').style.display = 'block';
    document.getElementById('sessionSection').style.display = 'none';
    updateHeaderStatus('online', 'Connected');
  }
  stopSessionTimer();
  
  // Reset offer tracking so new sessions can be detected
  processedOfferIds.clear();

  debug('✅ Session ended');
}

// ============================================================================
// WebRTC Connection
// ============================================================================

async function startWebRTC(sessionId, offerPayload) {
  console.log('[WebAgent] 🔗 Starting WebRTC connection (answerer role)...');
  console.log('[WebAgent] Session ID:', sessionId);

  try {
    // Refresh TURN credentials before creating peer connection
    await fetchTurnCredentials();
    console.log('[WebAgent] ICE config:', JSON.stringify(iceConfig.iceServers.map(s => s.urls)));

    // Create peer connection
    peerConnection = new RTCPeerConnection(iceConfig);

    // Add screen stream to peer connection
    if (!mediaStream) {
      throw new Error('No media stream available');
    }

    mediaStream.getTracks().forEach(track => {
      peerConnection.addTrack(track, mediaStream);
      debug('Added track:', track.kind);
    });

    // Buffer ICE candidates until answer is sent
    let answerSent = false;
    let pendingCandidates = [];

    // Handle incoming data channels from dashboard (for receiving input)
    peerConnection.ondatachannel = (event) => {
      debug('📥 Received data channel:', event.channel.label);
      dataChannel = event.channel;
      dataChannel.onopen = () => debug('✅ Data channel opened');
      dataChannel.onclose = () => debug('🔌 Data channel closed');
      dataChannel.onmessage = handleRemoteInput;
    };

    // Handle ICE candidates — buffer until answer is sent
    peerConnection.onicecandidate = async (event) => {
      if (event.candidate) {
        if (!answerSent) {
          debug('⏸️ Buffering ICE candidate (answer not sent yet)');
          pendingCandidates.push(event.candidate);
          return;
        }
        debug('📤 Sending ICE candidate');
        await supabase
          .from('session_signaling')
          .insert({
            session_id: sessionId,
            from_side: 'agent',
            msg_type: 'ice',
            payload: {
              candidate: event.candidate.candidate,
              sdpMid: event.candidate.sdpMid || '0',
              sdpMLineIndex: event.candidate.sdpMLineIndex || 0
            }
          });
      }
    };

    // Handle connection state changes
    peerConnection.onconnectionstatechange = () => {
      debug('Connection state:', peerConnection.connectionState);
      const qualityEl = document.getElementById('connectionQuality');
      if (qualityEl) {
        const stateMap = { connected: 'Fremragende', connecting: 'Forbinder...', disconnected: 'Afbrudt', failed: 'Fejlet' };
        qualityEl.textContent = stateMap[peerConnection.connectionState] || peerConnection.connectionState;
      }

      if (peerConnection.connectionState === 'connected') {
        debug('✅ WebRTC CONNECTED!');
        // Stop signaling polling once connected
        stopSignalingPolling();
      }

      if (peerConnection.connectionState === 'disconnected' ||
          peerConnection.connectionState === 'failed') {
        console.warn('⚠️ Connection lost');
        endSession();
      }
    };

    // Set remote description (the dashboard's offer)
    const offerSDP = offerPayload.sdp || offerPayload.SDP;
    console.log('[WebAgent] Offer SDP found:', !!offerSDP, 'payload keys:', Object.keys(offerPayload || {}));
    if (!offerSDP) {
      console.error('[WebAgent] Full offer payload:', JSON.stringify(offerPayload));
      throw new Error('No SDP in offer payload');
    }

    const offer = new RTCSessionDescription({
      type: 'offer',
      sdp: offerSDP
    });
    await peerConnection.setRemoteDescription(offer);
    console.log('[WebAgent] ✅ Remote description set (dashboard offer)');

    // Create answer
    const answer = await peerConnection.createAnswer();
    await peerConnection.setLocalDescription(answer);
    console.log('[WebAgent] 📤 Sending answer to dashboard');

    // Send answer via session_signaling
    const { error: answerError } = await supabase
      .from('session_signaling')
      .insert({
        session_id: sessionId,
        from_side: 'agent',
        msg_type: 'answer',
        payload: {
          type: 'answer',
          sdp: answer.sdp
        }
      });
    
    if (answerError) {
      console.error('[WebAgent] Failed to send answer:', answerError);
      throw answerError;
    }
    console.log('[WebAgent] ✅ Answer sent successfully');

    // Mark answer as sent and flush buffered ICE candidates
    answerSent = true;
    if (pendingCandidates.length > 0) {
      debug(`📤 Flushing ${pendingCandidates.length} buffered ICE candidates`);
      for (const candidate of pendingCandidates) {
        await supabase
          .from('session_signaling')
          .insert({
            session_id: sessionId,
            from_side: 'agent',
            msg_type: 'ice',
            payload: {
              candidate: candidate.candidate,
              sdpMid: candidate.sdpMid || '0',
              sdpMLineIndex: candidate.sdpMLineIndex || 0
            }
          });
      }
      pendingCandidates = [];
    }

    // Listen for ICE candidates from dashboard
    listenForSignaling();

    debug('✅ WebRTC answer sent, waiting for ICE exchange');

  } catch (error) {
    console.error('WebRTC setup failed:', error);
    throw error;
  }
}

let signalingPollingInterval = null;
let processedSignalIds = new Set();

function listenForSignaling() {
  const sessionId = currentSession.session_id || currentSession.id;
  
  // Subscribe to signaling messages via Realtime
  signalingChannel = supabase
    .channel(`session_${sessionId}`)
    .on('postgres_changes', {
      event: 'INSERT',
      schema: 'public',
      table: 'session_signaling',
      filter: `session_id=eq.${sessionId}`
    }, async (payload) => {
      const msg = payload.new;
      await handleSignalingMessage(msg);
    })
    .subscribe();

  debug('✅ Listening for signaling messages (realtime)');
  
  // Start polling fallback
  startSignalingPolling();
}

function startSignalingPolling() {
  debug('🔄 Starting signaling polling fallback...');
  
  signalingPollingInterval = setInterval(async () => {
    if (!currentSession) return;
    
    const sessionId = currentSession.session_id || currentSession.id;
    
    try {
      const { data, error } = await supabase
        .from('session_signaling')
        .select('*')
        .eq('session_id', sessionId)
        .in('from_side', ['dashboard', 'controller'])
        .order('created_at', { ascending: true });

      if (error) {
        console.error('❌ Signaling polling error:', error);
        return;
      }

      if (data && data.length > 0) {
        for (const msg of data) {
          if (processedSignalIds.has(msg.id)) continue;
          processedSignalIds.add(msg.id);
          debug('📥 Polled signaling:', msg.msg_type);
          await handleSignalingMessage(msg);
        }
      }
    } catch (err) {
      console.error('❌ Signaling polling exception:', err);
    }
  }, 500);
}

function stopSignalingPolling() {
  if (signalingPollingInterval) {
    clearInterval(signalingPollingInterval);
    signalingPollingInterval = null;
  }
  processedSignalIds.clear();
}

async function handleSignalingMessage(msg) {
  // Skip our own messages (from agent)
  if (msg.from_side === 'agent') return;
  
  // Skip already processed
  if (processedSignalIds.has(msg.id)) return;
  processedSignalIds.add(msg.id);

  // Skip offers — we already handled the initial offer in handleIncomingOffer
  if (msg.msg_type === 'offer') return;

  const data = msg.payload;

  try {
    if (msg.msg_type === 'ice') {
      // Handle both formats: flat { candidate, sdpMid, sdpMLineIndex } and nested { candidate: {...} }
      let iceData = data;
      if (data.candidate && typeof data.candidate === 'object') {
        iceData = data.candidate;
      }
      
      if (iceData && iceData.candidate) {
        debug('📥 ICE candidate from dashboard');
        await peerConnection.addIceCandidate(new RTCIceCandidate({
          candidate: iceData.candidate,
          sdpMid: iceData.sdpMid,
          sdpMLineIndex: iceData.sdpMLineIndex
        }));
        debug('✅ ICE candidate added');
      }
    }
  } catch (error) {
    console.error('Signaling error:', error);
  }
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
        debug('✅ Connected to Input Helper');
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
        debug('🔌 Input Helper disconnected');
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
      debug('📊 Helper status:', msg);
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
    debug('🎮 Remote input:', input.type);

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
window.endSession = endSession;

// ============================================================================
// Initialization
// ============================================================================

debug('🌐 Web Agent loaded');
debug('Platform:', navigator.platform);
debug('Browser:', getBrowserInfo());

// Check if already logged in
supabase.auth.getSession().then(({ data: { session } }) => {
  if (session) {
    debug('Already logged in, initializing...');
    currentUser = session.user;
    fetchTurnCredentials().then(() => registerDevice()).then(() => {
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
