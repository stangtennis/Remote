// Quick Support - Sharer (answerer) logic
// State machine: INIT → TOKEN_ENTRY → READY → SHARING → CONNECTED → ENDED

let supportState = 'INIT';
let supportSession = null;
let supportToken = null;
let peerConnection = null;
let mediaStream = null;
let signalingChannel = null;
let pollingInterval = null;
let processedSignalIds = new Set();
let sharingStartTime = null;
let durationInterval = null;

// UI elements
const pinSection = document.getElementById('pinSection');
const pinInput = document.getElementById('pinInput');
const shareBtn = document.getElementById('shareBtn');
const statusMsg = document.getElementById('statusMsg');
const previewSection = document.getElementById('previewSection');
const localPreview = document.getElementById('localPreview');
const sessionInfo = document.getElementById('sessionInfo');
const stopBtn = document.getElementById('stopBtn');
const connectingSpinner = document.getElementById('connectingSpinner');
const supportDesc = document.getElementById('supportDesc');

// Step indicators
const steps = [
  document.getElementById('step1'),
  document.getElementById('step2'),
  document.getElementById('step3'),
];

function setStep(n) {
  steps.forEach((s, i) => {
    s.classList.remove('active', 'done');
    if (i < n - 1) s.classList.add('done');
    if (i === n - 1) s.classList.add('active');
  });
}

function showStatus(msg, type) {
  statusMsg.textContent = msg;
  statusMsg.className = `status-msg ${type} visible`;
}

function hideStatus() {
  statusMsg.className = 'status-msg';
}

function setState(newState) {
  supportState = newState;
  debug('Support state:', newState);
}

// Check for token in URL
(function init() {
  const params = new URLSearchParams(window.location.search);
  const urlToken = params.get('token');

  if (urlToken) {
    // Token provided in URL - validate it
    supportToken = urlToken;
    pinSection.style.display = 'none';
    showStatus('Validerer session...', 'info');
    validateToken(urlToken);
  } else {
    // No token - show PIN entry
    setState('TOKEN_ENTRY');
    setStep(1);
    pinInput.focus();
    pinInput.addEventListener('keydown', (e) => {
      if (e.key === 'Enter') validatePin();
    });
  }
})();

async function validatePin() {
  const pin = pinInput.value.trim();
  if (pin.length !== 6 || !/^\d{6}$/.test(pin)) {
    showStatus('Indtast en gyldig 6-cifret PIN', 'error');
    return;
  }

  showStatus('Validerer PIN...', 'info');

  try {
    const response = await fetch(`${SUPABASE_CONFIG.url}/functions/v1/support-signal`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'apikey': SUPABASE_CONFIG.anonKey,
      },
      body: JSON.stringify({ action: 'validate', pin }),
    });

    const data = await response.json();
    if (!response.ok || data.error) {
      throw new Error(data.error || 'Ugyldig PIN');
    }

    supportSession = data;
    supportToken = data.token;
    onSessionValidated();
  } catch (error) {
    showStatus(error.message, 'error');
  }
}

async function validateToken(token) {
  try {
    const response = await fetch(`${SUPABASE_CONFIG.url}/functions/v1/support-signal`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'apikey': SUPABASE_CONFIG.anonKey,
      },
      body: JSON.stringify({ action: 'validate', token }),
    });

    const data = await response.json();
    if (!response.ok || data.error) {
      throw new Error(data.error || 'Ugyldig eller udløbet session');
    }

    supportSession = data;
    onSessionValidated();
  } catch (error) {
    showStatus(error.message, 'error');
    // Show PIN entry as fallback
    pinSection.style.display = 'block';
    setState('TOKEN_ENTRY');
    setStep(1);
  }
}

function onSessionValidated() {
  setState('READY');
  setStep(2);
  pinSection.style.display = 'none';
  hideStatus();
  supportDesc.textContent = 'Session bekræftet! Klik for at dele din skærm.';
  shareBtn.style.display = 'inline-flex';
  shareBtn.focus();
}

async function startSharing() {
  shareBtn.disabled = true;
  shareBtn.style.display = 'none';
  connectingSpinner.classList.add('visible');
  showStatus('Vælg den skærm du vil dele...', 'info');

  try {
    // Request screen capture
    mediaStream = await navigator.mediaDevices.getDisplayMedia({
      video: { cursor: 'always' },
      audio: false,
    });

    // Show local preview
    localPreview.srcObject = mediaStream;
    previewSection.classList.add('visible');
    hideStatus();

    // Handle user stopping screen share via browser UI
    mediaStream.getVideoTracks()[0].onended = () => {
      debug('Screen sharing stopped by user via browser UI');
      stopSharing();
    };

    // Notify backend that sharer is ready
    await fetch(`${SUPABASE_CONFIG.url}/functions/v1/support-signal`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'apikey': SUPABASE_CONFIG.anonKey,
      },
      body: JSON.stringify({ action: 'ready', token: supportToken }),
    });

    // Fetch TURN credentials
    const turnResponse = await fetch(`${SUPABASE_CONFIG.url}/functions/v1/support-signal`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'apikey': SUPABASE_CONFIG.anonKey,
      },
      body: JSON.stringify({ action: 'turn', token: supportToken }),
    });
    const turnData = await turnResponse.json();

    setState('SHARING');
    setStep(3);
    supportDesc.textContent = 'Venter på at supportmedarbejder forbinder...';
    connectingSpinner.classList.remove('visible');
    stopBtn.classList.add('visible');
    sessionInfo.style.display = 'flex';
    document.getElementById('connectionState').textContent = 'Venter på forbindelse...';

    // Start signaling - listen for offer from dashboard
    startSignaling(turnData);
  } catch (error) {
    connectingSpinner.classList.remove('visible');
    if (error.name === 'NotAllowedError') {
      showStatus('Skærmdeling blev afvist. Prøv igen.', 'error');
    } else {
      showStatus('Fejl: ' + error.message, 'error');
    }
    shareBtn.style.display = 'inline-flex';
    shareBtn.disabled = false;
    setState('READY');
    setStep(2);
  }
}

function startSignaling(turnData) {
  const sessionId = supportSession.session_id;

  // Store turn data for when we create the peer connection
  window._turnData = turnData;

  // Subscribe to signaling via Realtime
  signalingChannel = supabase
    .channel(`support_${sessionId}`)
    .on('postgres_changes', {
      event: 'INSERT',
      schema: 'public',
      table: 'session_signaling',
      filter: `session_id=eq.${sessionId}`,
    }, async (payload) => {
      debug('Realtime signal received:', payload.new.msg_type);
      await handleSignal(payload.new);
    })
    .subscribe();

  // Start polling fallback
  pollingInterval = setInterval(async () => {
    try {
      const { data, error } = await supabase
        .from('session_signaling')
        .select('*')
        .eq('session_id', sessionId)
        .eq('from_side', 'dashboard')
        .order('created_at', { ascending: true });

      if (error || !data) return;

      for (const signal of data) {
        if (processedSignalIds.has(signal.id)) continue;
        processedSignalIds.add(signal.id);
        debug('Polled signal:', signal.msg_type);
        await handleSignal(signal);
      }
    } catch (err) {
      console.error('Polling error:', err);
    }
  }, 500);
}

async function handleSignal(signal) {
  // Ignore own signals
  if (signal.from_side === 'support') return;
  // Only process dashboard signals
  if (signal.from_side !== 'dashboard') return;

  // Deduplicate
  if (processedSignalIds.has(signal.id)) return;
  processedSignalIds.add(signal.id);

  debug('Processing signal:', signal.msg_type, signal.payload);

  try {
    switch (signal.msg_type) {
      case 'offer':
        await handleOffer(signal.payload);
        break;

      case 'ice':
        await handleIceCandidate(signal.payload);
        break;

      case 'bye':
        stopSharing();
        break;
    }
  } catch (error) {
    console.error('Error handling signal:', error);
  }
}

async function handleOffer(payload) {
  const sessionId = supportSession.session_id;
  const turnData = window._turnData;

  // Check for relay mode
  const forceRelay = new URLSearchParams(window.location.search).get('relay') === 'true';

  const configuration = {
    iceServers: turnData?.iceServers || [
      { urls: 'stun:stun.l.google.com:19302' },
      { urls: 'stun:stun1.l.google.com:19302' },
    ],
    ...(forceRelay && { iceTransportPolicy: 'relay' }),
  };

  debug('Creating peer connection with config:', JSON.stringify(configuration.iceServers.map(s => s.urls)));

  peerConnection = new RTCPeerConnection(configuration);

  // Add screen tracks
  if (mediaStream) {
    mediaStream.getTracks().forEach(track => {
      peerConnection.addTrack(track, mediaStream);
      debug('Added track:', track.kind);
    });
  }

  // Buffer ICE candidates until answer is sent
  let answerSent = false;
  let pendingCandidates = [];

  // Handle ICE candidates
  peerConnection.onicecandidate = async (event) => {
    if (event.candidate) {
      if (!answerSent) {
        pendingCandidates.push(event.candidate);
        return;
      }
      await supabase
        .from('session_signaling')
        .insert({
          session_id: sessionId,
          from_side: 'support',
          msg_type: 'ice',
          payload: {
            candidate: event.candidate.candidate,
            sdpMid: event.candidate.sdpMid || '0',
            sdpMLineIndex: event.candidate.sdpMLineIndex || 0,
          },
        });
    }
  };

  // Connection state changes
  peerConnection.onconnectionstatechange = () => {
    const state = peerConnection.connectionState;
    debug('Connection state:', state);
    const stateEl = document.getElementById('connectionState');

    switch (state) {
      case 'connecting':
        if (stateEl) stateEl.textContent = 'Forbinder...';
        break;
      case 'connected':
        setState('CONNECTED');
        if (stateEl) stateEl.textContent = 'Forbundet';
        supportDesc.textContent = 'Din skærm deles nu med supportmedarbejderen';
        showStatus('Forbundet! Din skærm deles.', 'success');
        // Stop polling
        if (pollingInterval) {
          clearInterval(pollingInterval);
          pollingInterval = null;
        }
        // Start duration timer
        sharingStartTime = Date.now();
        durationInterval = setInterval(updateDuration, 1000);
        break;
      case 'disconnected':
      case 'failed':
        if (stateEl) stateEl.textContent = 'Afbrudt';
        showStatus('Forbindelsen blev afbrudt.', 'error');
        stopSharing();
        break;
    }
  };

  // Set remote description (dashboard's offer)
  const offerSDP = payload.sdp || payload.SDP;
  if (!offerSDP) {
    console.error('No SDP in offer payload:', payload);
    return;
  }

  const offer = new RTCSessionDescription({ type: 'offer', sdp: offerSDP });
  await peerConnection.setRemoteDescription(offer);
  debug('Remote description set (dashboard offer)');

  // Create answer
  const answer = await peerConnection.createAnswer();
  await peerConnection.setLocalDescription(answer);
  debug('Sending answer to dashboard');

  // Send answer via signaling
  const { error: answerError } = await supabase
    .from('session_signaling')
    .insert({
      session_id: sessionId,
      from_side: 'support',
      msg_type: 'answer',
      payload: { type: 'answer', sdp: answer.sdp },
    });

  if (answerError) {
    console.error('Failed to send answer:', answerError);
    return;
  }

  // Flush buffered ICE candidates
  answerSent = true;
  if (pendingCandidates.length > 0) {
    debug(`Flushing ${pendingCandidates.length} buffered ICE candidates`);
    for (const candidate of pendingCandidates) {
      await supabase
        .from('session_signaling')
        .insert({
          session_id: sessionId,
          from_side: 'support',
          msg_type: 'ice',
          payload: {
            candidate: candidate.candidate,
            sdpMid: candidate.sdpMid || '0',
            sdpMLineIndex: candidate.sdpMLineIndex || 0,
          },
        });
    }
    pendingCandidates = [];
  }

  debug('Answer sent, waiting for ICE exchange');
}

async function handleIceCandidate(payload) {
  if (!peerConnection) {
    debug('No peer connection, ignoring ICE candidate');
    return;
  }

  let iceCandidate;
  if (payload.candidate && typeof payload.candidate === 'object') {
    iceCandidate = payload.candidate;
  } else {
    iceCandidate = payload;
  }

  if (iceCandidate && iceCandidate.candidate) {
    if (!peerConnection.remoteDescription) {
      debug('Buffering ICE candidate (remote description not set)');
      return;
    }
    await peerConnection.addIceCandidate(
      new RTCIceCandidate({
        candidate: iceCandidate.candidate,
        sdpMid: iceCandidate.sdpMid,
        sdpMLineIndex: iceCandidate.sdpMLineIndex,
      })
    );
    debug('ICE candidate added');
  }
}

function updateDuration() {
  if (!sharingStartTime) return;
  const elapsed = Math.floor((Date.now() - sharingStartTime) / 1000);
  const mins = Math.floor(elapsed / 60);
  const secs = elapsed % 60;
  const el = document.getElementById('sharingDuration');
  if (el) el.textContent = `${mins}:${secs.toString().padStart(2, '0')}`;
}

async function stopSharing() {
  setState('ENDED');

  // Stop duration timer
  if (durationInterval) {
    clearInterval(durationInterval);
    durationInterval = null;
  }

  // Stop media tracks
  if (mediaStream) {
    mediaStream.getTracks().forEach(t => t.stop());
    mediaStream = null;
  }

  // Close peer connection
  if (peerConnection) {
    try { peerConnection.close(); } catch (e) {}
    peerConnection = null;
  }

  // Stop polling
  if (pollingInterval) {
    clearInterval(pollingInterval);
    pollingInterval = null;
  }

  // Remove realtime channel
  if (signalingChannel) {
    supabase.removeChannel(signalingChannel);
    signalingChannel = null;
  }

  // Send bye signal
  if (supportSession) {
    try {
      await supabase
        .from('session_signaling')
        .insert({
          session_id: supportSession.session_id,
          from_side: 'support',
          msg_type: 'bye',
          payload: { reason: 'sharer_stopped' },
        });
    } catch (e) {}
  }

  // Update UI
  previewSection.classList.remove('visible');
  stopBtn.classList.remove('visible');
  sessionInfo.style.display = 'none';
  localPreview.srcObject = null;
  supportDesc.textContent = 'Skærmdelingen er afsluttet';
  showStatus('Skærmdelingen er stoppet.', 'info');
  setStep(1);
}
