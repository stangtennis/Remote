// Quick Support - Sharer (answerer) logic
// State machine: INIT → TOKEN_ENTRY → READY → SHARING → CONNECTED → ENDED

let supportState = 'INIT';
let supportSession = null;
let supportToken = null;
let supportInviteToken = null;
let peerConnection = null;
let mediaStream = null;
let signalingChannel = null;
let pollingInterval = null;
let processedSignalIds = new Set();
let pendingRemoteIceCandidates = [];
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
const consentSection = document.getElementById('consentSection');
const consentBtn = document.getElementById('consentBtn');

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

async function supportSignalRequest(action, extra = {}) {
  const auth = supportSession?.support_mode === 'ai'
    ? { client_grant_token: supportToken }
    : { token: supportToken };
  const response = await fetch(`${SUPABASE_CONFIG.url}/functions/v1/support-signal`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', 'apikey': SUPABASE_CONFIG.anonKey },
    body: JSON.stringify({ action, ...auth, ...extra }),
  });
  const data = await response.json();
  if (!response.ok || data.error) {
    const error = new Error(data.error || `Support request failed: ${action}`);
    error.status = response.status;
    throw error;
  }
  return data;
}

// Check for token or public mode in URL
const isPublicMode = new URLSearchParams(window.location.search).has('public');

(async function init() {
  const params = new URLSearchParams(window.location.search);
  const urlToken = params.get('token');

  if (isPublicMode) {
    // Public mode - check if enabled, then show direct share button
    pinSection.style.display = 'none';
    showStatus('Checker tilgængelighed...', 'info');
    try {
      const res = await fetch(`${SUPABASE_CONFIG.url}/functions/v1/support-signal`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'apikey': SUPABASE_CONFIG.anonKey },
        body: JSON.stringify({ action: 'check-public' }),
      });
      const { enabled } = await res.json();

      if (!enabled) {
        showStatus('Support link er ikke aktivt lige nu. Kontakt administrator.', 'error');
        supportDesc.textContent = 'Support er ikke tilgængeligt';
        return;
      }

      // Show direct share button (skip PIN step)
      hideStatus();
      supportDesc.textContent = 'Klik knappen for at dele din skærm med support';
      document.querySelector('.support-card h1').textContent = 'Support — Del din skærm';
      shareBtn.style.display = 'inline-flex';
      shareBtn.focus();
      setState('READY');
      setStep(2);
    } catch (error) {
      showStatus('Kunne ikke kontakte server: ' + error.message, 'error');
    }
  } else if (urlToken) {
    // Token provided in URL - validate it
    supportInviteToken = urlToken;
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
      body: JSON.stringify({
        action: 'validate',
        pin,
        ...(supportInviteToken ? { token: supportInviteToken } : {}),
      }),
    });

    const data = await response.json();
    if (!response.ok || data.error) {
      throw new Error(data.error || 'Ugyldig PIN');
    }

    supportSession = data;
    supportToken = data.client_grant_token || data.token;
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
    if (data.code_required) {
      pinSection.style.display = 'block';
      setState('TOKEN_ENTRY');
      setStep(1);
      showStatus('Indtast koden fra administratoren for at fortsætte.', 'info');
      return;
    }
    supportToken = data.client_grant_token || data.token;
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
  if (supportSession?.support_mode === 'ai' && supportSession?.requires_consent) {
    supportDesc.textContent = 'Koden er godkendt. Vælg tilladelser for AI-support.';
    consentSection.style.display = 'block';
    renderConsentScopes(supportSession.requested_scopes || ['screen']);
    consentBtn?.focus();
    return;
  }
  supportDesc.textContent = 'Session bekræftet! Klik for at dele din skærm.';
  shareBtn.style.display = 'inline-flex';
  shareBtn.focus();
}

function renderConsentScopes(requestedScopes) {
  document.querySelectorAll('#consentScopes [data-scope]').forEach((input) => {
    // The browser sharer has no input/file/terminal channel. Those scopes are
    // reserved for the native support client and must not be presented as active here.
    const allowed = input.dataset.scope === 'screen' && requestedScopes.includes(input.dataset.scope);
    input.closest('label').style.display = allowed ? 'block' : 'none';
    if (!allowed) input.checked = false;
  });
}

async function grantSupportConsent() {
  if (!supportToken || supportSession?.support_mode !== 'ai') return;
  const scopes = [...document.querySelectorAll('#consentScopes [data-scope]:checked')]
    .map((input) => input.dataset.scope);
  if (!scopes.includes('screen')) {
    showStatus('Skærmadgang skal være accepteret.', 'error');
    return;
  }

  consentBtn.disabled = true;
  try {
    const response = await fetch(`${SUPABASE_CONFIG.url}/functions/v1/support-signal`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'apikey': SUPABASE_CONFIG.anonKey },
      body: JSON.stringify({
        action: 'consent',
        client_grant_token: supportToken,
        approved: true,
        scopes,
      }),
    });
    const data = await response.json();
    if (!response.ok || data.error) throw new Error(data.error || 'Consent blev afvist');
    supportSession.client_consent_scopes = data.scopes;
    consentSection.style.display = 'none';
    supportDesc.textContent = 'Tilladelser godkendt. Klik for at starte support.';
    shareBtn.style.display = 'inline-flex';
    shareBtn.focus();
  } catch (error) {
    showStatus(error.message, 'error');
  } finally {
    consentBtn.disabled = false;
  }
}

async function startSharing() {
  shareBtn.disabled = true;
  shareBtn.style.display = 'none';
  connectingSpinner.classList.add('visible');
  showStatus('Vælg den skærm du vil dele...', 'info');

  try {
    // In public mode, create session first
    if (isPublicMode && !supportToken) {
      showStatus('Opretter session...', 'info');
      const res = await fetch(`${SUPABASE_CONFIG.url}/functions/v1/support-signal`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'apikey': SUPABASE_CONFIG.anonKey },
        body: JSON.stringify({ action: 'create-public' }),
      });
      const data = await res.json();
      if (!res.ok || data.error) {
        throw new Error(data.error || 'Kunne ikke oprette session');
      }
      supportSession = { session_id: data.session_id, token: data.token };
      supportToken = data.token;
      showStatus('Vælg den skærm du vil dele...', 'info');
    }

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
    await supportSignalRequest('ready');

    // Fetch TURN credentials
    const turnData = await supportSignalRequest('turn');

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
    if (mediaStream) {
      mediaStream.getTracks().forEach((track) => track.stop());
      mediaStream = null;
      localPreview.srcObject = null;
    }
    if (supportToken) {
      try { await supportSignalRequest('end'); } catch (endError) { console.error('Failed to end support session:', endError); }
    }
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

window.addEventListener('pagehide', () => {
  if (!supportToken || !supportSession) return;
  const auth = supportSession.support_mode === 'ai'
    ? { client_grant_token: supportToken }
    : { token: supportToken };
  const body = JSON.stringify({ action: 'end', ...auth });
  navigator.sendBeacon(
    `${SUPABASE_CONFIG.url}/functions/v1/support-signal`,
    new Blob([body], { type: 'text/plain;charset=UTF-8' }),
  );
});

function startSignaling(turnData) {
  // Store turn data for when we create the peer connection
  window._turnData = turnData;

  // Private support signaling goes through the Edge Function so the browser
  // never needs anonymous RLS access to session_signaling.
  pollingInterval = setInterval(async () => {
    try {
      const { signals } = await supportSignalRequest('signal-read');

      for (const signal of signals || []) {
        if (processedSignalIds.has(signal.id)) continue;
        debug('Polled signal:', signal.msg_type);
        await handleSignal(signal);
      }
    } catch (err) {
      console.error('Polling error:', err);
      if ([400, 401, 403, 404].includes(err.status) || String(err.message).includes('no longer active')) {
        await stopSharing();
        return;
      }
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
      await supportSignalRequest('signal-write', {
        signal_type: 'ice',
        signal_payload: {
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

  for (const candidate of pendingRemoteIceCandidates.splice(0)) {
    await peerConnection.addIceCandidate(new RTCIceCandidate(candidate));
  }

  // Create answer
  const answer = await peerConnection.createAnswer();
  await peerConnection.setLocalDescription(answer);
  debug('Sending answer to dashboard');

  // Send answer via signaling
  try {
    await supportSignalRequest('signal-write', {
      signal_type: 'answer',
      signal_payload: { type: 'answer', sdp: answer.sdp },
    });
  } catch (error) {
    console.error('Failed to send answer:', error);
    return;
  }

  // Flush buffered ICE candidates
  answerSent = true;
  if (pendingCandidates.length > 0) {
    debug(`Flushing ${pendingCandidates.length} buffered ICE candidates`);
    for (const candidate of pendingCandidates) {
      await supportSignalRequest('signal-write', {
        signal_type: 'ice',
        signal_payload: {
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
  let iceCandidate;
  if (payload.candidate && typeof payload.candidate === 'object') {
    iceCandidate = payload.candidate;
  } else {
    iceCandidate = payload;
  }

  if (iceCandidate && iceCandidate.candidate) {
    if (!peerConnection) {
      debug('Buffering ICE candidate until offer creates peer connection');
      pendingRemoteIceCandidates.push(iceCandidate);
      return;
    }
    if (!peerConnection.remoteDescription) {
      debug('Buffering ICE candidate (remote description not set)');
      pendingRemoteIceCandidates.push(iceCandidate);
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

async function terminateSupportRemotely() {
  if (!supportSession || !supportToken) return true;
  const auth = supportSession.support_mode === 'ai'
    ? { client_grant_token: supportToken }
    : { token: supportToken };
  const endBody = JSON.stringify({ action: 'end', ...auth });

  try {
    await supportSignalRequest('signal-write', {
      signal_type: 'bye',
      signal_payload: { reason: 'sharer_stopped' },
    });
  } catch (error) {
    console.warn('Support bye signal failed; retrying end:', error);
  }

  for (let attempt = 0; attempt < 3; attempt += 1) {
    try {
      const response = await fetch(`${SUPABASE_CONFIG.url}/functions/v1/support-signal`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'apikey': SUPABASE_CONFIG.anonKey },
        body: endBody,
      });
      if (response.ok) return true;
    } catch (error) {
      console.warn('Support end attempt failed:', error);
    }
    await new Promise(resolve => setTimeout(resolve, 500 * (attempt + 1)));
  }

  navigator.sendBeacon(
    `${SUPABASE_CONFIG.url}/functions/v1/support-signal`,
    new Blob([endBody], { type: 'text/plain;charset=UTF-8' }),
  );
  return false;
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

  const endedRemotely = await terminateSupportRemotely();

  // Update UI
  previewSection.classList.remove('visible');
  stopBtn.classList.remove('visible');
  sessionInfo.style.display = 'none';
  localPreview.srcObject = null;
  if (consentSection) consentSection.style.display = 'none';
  supportDesc.textContent = 'Skærmdelingen er afsluttet';
  showStatus(endedRemotely ? 'Skærmdelingen er stoppet.' : 'Skærmdelingen er stoppet lokalt; serveren prøver stadig at afslutte sessionen.', 'info');
  supportSession = null;
  supportToken = null;
  supportInviteToken = null;
  window._turnData = null;
  processedSignalIds.clear();
  pendingRemoteIceCandidates = [];
  setStep(1);
}
