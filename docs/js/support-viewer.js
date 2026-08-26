// Quick Support Viewer - Dashboard side
// Creates support sessions, connects via WebRTC (offerer), displays video

let supportViewerPC = null;
let supportSignalingChannel = null;
let supportControllerChannel = null;
let supportControllerPollInterval = null;
let supportPollingInterval = null;
let supportProcessedIds = new Set();
let supportPendingIce = [];
let supportReadyHandled = false;
let currentSupportSession = null;
let supportInPreview = false;
let supportCreationPending = false;
let resolveSupportRestore;
const supportRestoreReady = new Promise((resolve) => { resolveSupportRestore = resolve; });
const ACTIVE_AI_SUPPORT_STORAGE_KEY = 'remoteDesktopActiveAISupport';
const SUPPORT_SITE_BASE = window.location.pathname.startsWith('/Remote/')
  ? `${window.location.origin}/Remote`
  : window.location.origin;

function rememberActiveAISupportSession(sessionId, userId) {
  localStorage.setItem(ACTIVE_AI_SUPPORT_STORAGE_KEY, JSON.stringify({ session_id: sessionId, user_id: userId }));
}

function forgetActiveAISupportSession() {
  localStorage.removeItem(ACTIVE_AI_SUPPORT_STORAGE_KEY);
}

function setAISupportStatus(state, message) {
  const container = document.getElementById('aiSupportStatus');
  if (!container) return;
  if (state === 'hidden') {
    container.style.display = 'none';
    return;
  }

  const dot = document.getElementById('aiSupportStatusDot');
  const title = document.getElementById('aiSupportStatusTitle');
  const text = document.getElementById('aiSupportStatusText');
  const connected = state === 'connected';
  const ended = state === 'ended';
  const color = connected ? '#22c55e' : ended ? '#f59e0b' : '#60a5fa';
  container.style.display = 'flex';
  container.style.borderColor = `${color}59`;
  container.style.background = `${color}14`;
  if (dot) dot.style.background = color;
  if (title) title.textContent = connected ? 'AI-klient forbundet' : ended ? 'AI-support afsluttet' : 'AI-klient venter';
  if (text) text.textContent = message || '';
}

function setSupportExpiry(session) {
  const expiry = document.getElementById('supportExpiry');
  if (!expiry) return;
  if (session.support_mode === 'ai') {
    expiry.textContent = 'Ingen automatisk udløb — afsluttes manuelt';
    return;
  }
  const expiresAt = new Date(session.expires_at);
  expiry.textContent = `Udløber kl. ${expiresAt.toLocaleTimeString('da-DK', { hour: '2-digit', minute: '2-digit' })}`;
}

// ============================================================================
// Session Creation
// ============================================================================

async function createSupportSession() {
  const { data: { session } } = await supabase.auth.getSession();
  if (!session) {
    showToast('Du skal være logget ind', 'error');
    return null;
  }

  try {
    const modeSelect = document.getElementById('supportMode');
    const supportMode = modeSelect?.value === 'screen' ? 'screen' : 'ai';
    const requestedScopes = [...document.querySelectorAll('[data-support-scope]:checked')]
      .map((input) => input.dataset.supportScope);

    const response = await fetch(`${SUPABASE_CONFIG.url}/functions/v1/create-support-session`, {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${session.access_token}`,
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        support_mode: supportMode,
        requested_scopes: supportMode === 'ai' ? requestedScopes : ['screen'],
      }),
    });

    const data = await response.json();
    if (!response.ok || data.error) {
      throw new Error(data.error || 'Kunne ikke oprette support session');
    }

    currentSupportSession = data;
    if (data.support_mode === 'ai') {
      const { data: authData } = await supabase.auth.getSession();
      if (authData?.session?.user?.id) {
        rememberActiveAISupportSession(data.session_id, authData.session.user.id);
      }
    }
    return data;
  } catch (error) {
    console.error('Create support session error:', error);
    showToast('Fejl: ' + error.message, 'error');
    return null;
  }
}

// ============================================================================
// Support Modal UI
// ============================================================================

function showSupportModal() {
  const modal = document.getElementById('supportModal');
  if (modal) {
    modal.style.display = 'flex';
    const hasActiveAISession = currentSupportSession?.support_mode === 'ai';
    showSupportStep(hasActiveAISession ? 'share' : 'create');
    if (hasActiveAISession) return;
    const modeSelect = document.getElementById('supportMode');
    const scopes = document.getElementById('aiSupportScopes');
    if (modeSelect && !modeSelect.dataset.bound) {
      modeSelect.dataset.bound = 'true';
      modeSelect.addEventListener('change', () => {
        if (scopes) scopes.style.display = modeSelect.value === 'ai' ? 'block' : 'none';
        updateSupportCreateButton();
      });
    }
    if (modeSelect && scopes) scopes.style.display = modeSelect.value === 'ai' ? 'block' : 'none';
    updateSupportCreateButton();
    loadPublicSupportState();
  }
}

function updateSupportCreateButton() {
  const button = document.getElementById('supportCreateBtn');
  const mode = document.getElementById('supportMode')?.value;
  if (button && !supportCreationPending) {
    button.textContent = mode === 'screen' ? 'Opret skærmdelingssession' : 'Generér AI-supportkode';
  }
}

async function generateAISupportCode() {
  await supportRestoreReady;
  showSupportModal();
  if (currentSupportSession?.support_mode === 'ai' || supportCreationPending) return;

  const modeSelect = document.getElementById('supportMode');
  if (modeSelect) modeSelect.value = 'ai';
  const scopes = document.getElementById('aiSupportScopes');
  if (scopes) scopes.style.display = 'block';
  updateSupportCreateButton();
  await onCreateSupportSession();
}

function closeSupportModal() {
  const modal = document.getElementById('supportModal');
  if (currentSupportSession?.support_mode === 'ai' && !supportInPreview) {
    if (modal) modal.style.display = 'none';
    return;
  }
  if (currentSupportSession && !supportInPreview) {
    endSupportSession();
    return;
  }
  if (modal) modal.style.display = 'none';
  if (!supportInPreview) {
    cleanupSupportViewer();
  }
}

function showSupportStep(step) {
  document.querySelectorAll('.support-step').forEach(el => el.style.display = 'none');
  const el = document.getElementById(`supportStep_${step}`);
  if (el) el.style.display = 'block';
}

async function onCreateSupportSession() {
  await supportRestoreReady;
  if (currentSupportSession?.support_mode === 'ai') {
    showSupportStep('share');
    return currentSupportSession;
  }
  if (supportCreationPending) return null;
  supportCreationPending = true;
  const btn = document.getElementById('supportCreateBtn');
  const quickBtn = document.getElementById('aiSupportCodeBtn');
  if (btn) {
    btn.disabled = true;
    btn.textContent = 'Opretter...';
  }
  if (quickBtn) {
    quickBtn.disabled = true;
    quickBtn.innerHTML = '<i class="fas fa-spinner fa-spin"></i> Opretter...';
  }

  let session;
  try {
    session = await createSupportSession();
  } finally {
    supportCreationPending = false;
    if (btn) {
      btn.disabled = false;
      updateSupportCreateButton();
    }
    if (quickBtn) {
      quickBtn.disabled = false;
      quickBtn.innerHTML = '<i class="fas fa-robot"></i> Generér AI-kode';
    }
  }

  if (!session) return null;

  // Show share step with PIN and link
  showSupportStep('share');
  document.getElementById('supportPin').textContent = session.pin;
  document.getElementById('supportLink').value = session.share_url;
  const sessionIdEl = document.getElementById('supportSessionId');
  if (sessionIdEl) sessionIdEl.textContent = session.session_id;
  const aiRevokeBtn = document.getElementById('supportAiRevokeBtn');
  if (aiRevokeBtn) aiRevokeBtn.style.display = session.support_mode === 'ai' ? 'block' : 'none';
  const ubuntuBtn = document.getElementById('supportUbuntuConnectBtn');
  if (ubuntuBtn) ubuntuBtn.style.display = session.support_mode === 'ai' ? 'block' : 'none';

  // Calculate expiry time
  setSupportExpiry(session);

  if (session.support_mode === 'ai') {
    setAISupportStatus('waiting', 'Venter på PIN-godkendelse og forbindelse fra Ubuntu AI...');
    watchUbuntuController(session.session_id);
    document.getElementById('supportShareStatus').textContent =
      'Send koden til personen. De åbner AI Supportklient, indtaster koden og accepterer tilladelserne.';
    // Queue the Ubuntu watcher immediately. It only claims the session after
    // the native client has completed consent and marked itself ready.
    requestUbuntuAI();
    return session;
  }

  // Browser screen-share sessions use the dashboard viewer.
  waitForSharerReady(session.session_id);
  return session;
}

async function requestUbuntuAI() {
  const sessionId = currentSupportSession?.session_id;
  const button = document.getElementById('supportUbuntuConnectBtn');
  const status = document.getElementById('supportShareStatus');
  if (!sessionId) return;

  if (button) {
    button.disabled = true;
    button.textContent = 'Forbinder Ubuntu AI...';
  }

  const { data: authData } = await supabase.auth.getSession();
  const authSession = authData?.session;
  if (!authSession) {
    if (status) status.textContent = 'Du skal være logget ind for at forbinde Ubuntu AI.';
    return;
  }
  const response = await fetch(`${SUPABASE_CONFIG.url}/functions/v1/support-signal`, {
    method: 'POST',
    headers: {
      'apikey': SUPABASE_CONFIG.anonKey,
      'Authorization': `Bearer ${authSession.access_token}`,
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ action: 'request-controller', session_id: sessionId }),
  });
  const result = await response.json().catch(() => ({}));
  if (!response.ok || result.error) {
    if (button) {
      button.disabled = false;
      button.textContent = 'Forbind Ubuntu AI';
    }
    if (status) status.textContent = `Kunne ikke vælge Ubuntu AI-target: ${result.error || response.status}`;
    return;
  }
  if (status) status.textContent = 'Ubuntu AI er valgt. Venter på forbindelse fra Ubuntu...';
  setAISupportStatus('waiting', 'Ubuntu AI er valgt. Venter på forbindelse...');
}

function watchUbuntuController(sessionId) {
  if (supportControllerChannel) supabase.removeChannel(supportControllerChannel);
  if (supportControllerPollInterval) clearInterval(supportControllerPollInterval);
  const applyControllerState = (session) => {
    const status = document.getElementById('supportShareStatus');
    const button = document.getElementById('supportUbuntuConnectBtn');
    if (!session) return;
    if (session.controller_claimed_by) {
      if (status) status.textContent = 'Ubuntu AI er forbundet til denne client.';
      setAISupportStatus('connected', `Session ${sessionId} er forbundet til Ubuntu AI.`);
      if (button) {
        button.disabled = true;
        button.textContent = 'Ubuntu AI forbundet';
      }
    } else if (session.status === 'ended' || session.status === 'expired') {
      if (status) status.textContent = 'Supportsessionen er afsluttet.';
      setAISupportStatus('ended', 'Supportsessionen er afsluttet.');
    } else if (currentSupportSession?.support_mode === 'ai') {
      setAISupportStatus('waiting', 'AI-klienten er godkendt. Venter på Ubuntu AI...');
    }
  };
  const refreshControllerState = async () => {
    const { data } = await supabase
      .from('support_sessions')
      .select('status, controller_requested, controller_claimed_by')
      .eq('id', sessionId)
      .maybeSingle();
    applyControllerState(data);
  };
  supportControllerChannel = supabase
    .channel(`support-controller-${sessionId}`)
    .on('postgres_changes', {
      event: 'UPDATE',
      schema: 'public',
      table: 'support_sessions',
      filter: `id=eq.${sessionId}`,
    }, (payload) => applyControllerState(payload.new))
    .subscribe();
  refreshControllerState();
  supportControllerPollInterval = setInterval(refreshControllerState, 3000);
}

function copySupportLink() {
  const linkInput = document.getElementById('supportLink');
  if (!linkInput) return;
  navigator.clipboard.writeText(linkInput.value).then(() => {
    const btn = document.getElementById('supportCopyBtn');
    if (btn) {
      const orig = btn.textContent;
      btn.textContent = 'Kopieret!';
      setTimeout(() => btn.textContent = orig, 2000);
    }
  });
}

function copySupportPin() {
  const pin = document.getElementById('supportPin')?.textContent?.trim();
  if (!pin) return;
  navigator.clipboard.writeText(pin).then(() => {
    showToast('AI-supportkoden er kopieret', 'success');
  }).catch(() => {
    showToast('Kunne ikke kopiere koden', 'error');
  });
}

// ============================================================================
// Wait for Sharer Ready
// ============================================================================

function waitForSharerReady(sessionId) {
  const statusEl = document.getElementById('supportShareStatus');
  supportReadyHandled = false;
  if (statusEl) statusEl.textContent = 'Venter på at personen deler sin skærm...';

  // Subscribe to session_signaling for ready signal from support
  supportSignalingChannel = supabase
    .channel(`support_viewer_${sessionId}`)
    .on('postgres_changes', {
      event: 'INSERT',
      schema: 'public',
      table: 'session_signaling',
      filter: `session_id=eq.${sessionId}`,
    }, async (payload) => {
      const signal = payload.new;
      if (signal.from_side === 'support' && signal.msg_type === 'answer' && signal.payload?.type === 'ready') {
        if (supportReadyHandled) return;
        supportReadyHandled = true;
        debug('Sharer is ready!');
        if (statusEl) statusEl.textContent = 'Personen er klar! Starter forbindelse...';
        connectToSupport(sessionId);
      }
    })
    .subscribe();

  // Polling fallback for ready signal
  supportPollingInterval = setInterval(async () => {
    const { data } = await supabase
      .from('session_signaling')
      .select('*')
      .eq('session_id', sessionId)
      .eq('from_side', 'support')
      .order('created_at', { ascending: true });

    if (!data) return;

    for (const signal of data) {
      if (supportProcessedIds.has(signal.id)) continue;
      supportProcessedIds.add(signal.id);

      if (signal.msg_type === 'answer' && signal.payload?.type === 'ready') {
        if (supportReadyHandled) continue;
        supportReadyHandled = true;
        debug('Polled: Sharer is ready!');
        if (statusEl) statusEl.textContent = 'Personen er klar! Starter forbindelse...';
        connectToSupport(sessionId);
      }
    }
  }, 1000);
}

// ============================================================================
// WebRTC Connection (Offerer)
// ============================================================================

async function connectToSupport(sessionId) {
  // Stop polling for ready
  if (supportPollingInterval) {
    clearInterval(supportPollingInterval);
    supportPollingInterval = null;
  }

  // Show viewer step
  showSupportStep('viewer');

  try {
    // Fetch TURN credentials
    const { data: { session: authSession } } = await supabase.auth.getSession();
    const requireRelay = false;
    let iceServers = [
      { urls: 'stun:stun.l.google.com:19302' },
      { urls: 'stun:stun1.l.google.com:19302' },
    ];

    try {
      const turnResp = await fetch(`${SUPABASE_CONFIG.url}/functions/v1/turn-credentials`, {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${authSession.access_token}`,
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ require_relay: requireRelay }),
      });
      if (!turnResp.ok && requireRelay) throw new Error('Cloudflare TURN relay unavailable');
      if (turnResp.ok) {
        const turnData = await turnResp.json();
        iceServers = turnData.iceServers;
      }
    } catch (e) {
      if (requireRelay) throw e;
      console.warn('Failed to fetch TURN credentials:', e);
    }

    const forceRelay = requireRelay || new URLSearchParams(window.location.search).get('relay') === 'true';
    const configuration = {
      iceServers,
      ...(forceRelay && { iceTransportPolicy: 'relay' }),
    };

    debug('Support viewer: creating peer connection');
    supportViewerPC = new RTCPeerConnection(configuration);
    const viewerPC = supportViewerPC;

    // Handle remote video track - pipe to both modal video and main preview
    supportViewerPC.ontrack = (event) => {
      if (supportViewerPC !== viewerPC) return;
      debug('Support viewer: received remote track', event.track.kind);
      const supportVideo = document.getElementById('supportVideo');
      const previewVideo = document.getElementById('previewVideo');
      if (event.streams[0]) {
        for (const video of [supportVideo, previewVideo]) {
          if (!video) continue;
          video.srcObject = event.streams[0];
          video.muted = true;
          video.play().catch((error) => debug('Support viewer autoplay deferred:', error.message));
        }
      }
    };

    // Send ICE candidates
    viewerPC.onicecandidate = async (event) => {
      if (event.candidate) {
        const payload = {
          ...event.candidate.toJSON(),
        };
        await supabase
          .from('session_signaling')
          .insert({
            session_id: sessionId,
            from_side: 'dashboard',
            msg_type: 'ice',
            payload,
          });
      }
    };

    // Connection state
    viewerPC.onconnectionstatechange = () => {
      const state = viewerPC.connectionState;
      debug('Support viewer connection state:', state);
      const statusEl = document.getElementById('supportViewerStatus');

      switch (state) {
        case 'connecting':
          if (statusEl) statusEl.textContent = 'Forbinder...';
          break;
        case 'connected':
          if (statusEl) statusEl.textContent = 'Forbundet';
          // Stop signaling polling
          if (supportPollingInterval) {
            clearInterval(supportPollingInterval);
            supportPollingInterval = null;
          }
          // Show in main preview area
          showSupportInPreview();
          break;
        case 'disconnected':
        case 'failed':
          if (statusEl) statusEl.textContent = 'Afbrudt';
          if (currentSupportSession) void endSupportSession();
          else cleanupSupportViewer();
          break;
      }
    };

    // Create offer (receive video only, no data channel)
    const offer = await viewerPC.createOffer({
      offerToReceiveVideo: true,
      offerToReceiveAudio: false,
    });
    await viewerPC.setLocalDescription(offer);

    // Send offer to sharer
    await supabase
      .from('session_signaling')
      .insert({
        session_id: sessionId,
        from_side: 'dashboard',
        msg_type: 'offer',
        payload: {
          type: 'offer',
          sdp: offer.sdp,
        },
      });

    debug('Support viewer: offer sent');

    // Listen for answer and ICE from sharer
    subscribeToSupportSignaling(sessionId);

  } catch (error) {
    console.error('Support connection error:', error);
    const statusEl = document.getElementById('supportViewerStatus');
    if (statusEl) statusEl.textContent = 'Forbindelsesfejl: ' + error.message;
  }
}

// ============================================================================
// Signaling Subscription (Viewer side)
// ============================================================================

function subscribeToSupportSignaling(sessionId) {
  // We already have the realtime channel from waitForSharerReady
  // Just reset the polling for answer/ice signals
  supportProcessedIds.clear();
  const signalSide = 'support';

  supportPollingInterval = setInterval(async () => {
    try {
      const { data } = await supabase
        .from('session_signaling')
        .select('*')
        .eq('session_id', sessionId)
        .eq('from_side', signalSide)
        .order('created_at', { ascending: true });

      if (!data) return;

      for (const signal of data) {
        if (supportProcessedIds.has(signal.id)) continue;
        supportProcessedIds.add(signal.id);
        await handleSupportViewerSignal(signal);
      }
    } catch (err) {
      console.error('Support polling error:', err);
    }
  }, 500);
}

async function handleSupportViewerSignal(signal) {
  const expectedSide = 'support';
  if (signal.from_side !== expectedSide) return;
  if (!supportViewerPC) return;

  debug('Support viewer: processing signal', signal.msg_type);

  try {
    switch (signal.msg_type) {
      case 'answer': {
        // Skip ready signals
        if (signal.payload?.type === 'ready') return;

        if (supportViewerPC.signalingState !== 'have-local-offer') {
          debug('Skipping answer, state:', supportViewerPC.signalingState);
          return;
        }

        const answer = new RTCSessionDescription(signal.payload);
        await supportViewerPC.setRemoteDescription(answer);
        debug('Support viewer: remote description set');

        // Flush buffered ICE
        if (supportPendingIce.length > 0) {
          debug(`Flushing ${supportPendingIce.length} buffered ICE candidates`);
          for (const ice of supportPendingIce) {
            await supportViewerPC.addIceCandidate(new RTCIceCandidate(ice));
          }
          supportPendingIce = [];
        }
        break;
      }

      case 'ice': {
        let iceCandidate;
        if (signal.payload.candidate && typeof signal.payload.candidate === 'object') {
          iceCandidate = signal.payload.candidate;
        } else {
          iceCandidate = signal.payload;
        }

        if (iceCandidate && iceCandidate.candidate) {
          if (!supportViewerPC.remoteDescription) {
            supportPendingIce.push({
              candidate: iceCandidate.candidate,
              sdpMid: iceCandidate.sdpMid,
              sdpMLineIndex: iceCandidate.sdpMLineIndex,
            });
          } else {
            await supportViewerPC.addIceCandidate(
              new RTCIceCandidate({
                candidate: iceCandidate.candidate,
                sdpMid: iceCandidate.sdpMid,
                sdpMLineIndex: iceCandidate.sdpMLineIndex,
              })
            );
          }
        }
        break;
      }

      case 'bye':
        debug('Support sharer disconnected');
        cleanupSupportViewer();
        const statusEl = document.getElementById('supportViewerStatus');
        if (statusEl) statusEl.textContent = 'Personen stoppede deling';
        break;
    }
  } catch (error) {
    console.error('Error handling support signal:', error);
  }
}

// ============================================================================
// Fullscreen
// ============================================================================

let supportFullscreen = false;

function toggleSupportFullscreen() {
  const modal = document.getElementById('supportModal');
  const content = modal?.querySelector('.modal-content');
  const container = document.getElementById('supportVideoContainer');
  const video = document.getElementById('supportVideo');
  if (!modal || !content || !container) return;

  supportFullscreen = !supportFullscreen;

  if (supportFullscreen) {
    // Try native fullscreen on video container first
    if (container.requestFullscreen) {
      container.requestFullscreen().catch(() => {
        // Fallback: expand modal to fill screen
        applyFullscreenStyle(modal, content, container, video);
      });
    } else {
      applyFullscreenStyle(modal, content, container, video);
    }
  } else {
    if (document.fullscreenElement) {
      document.exitFullscreen().catch(() => {});
    }
    removeFullscreenStyle(modal, content, container, video);
  }
}

function applyFullscreenStyle(modal, content, container, video) {
  content.style.maxWidth = '100vw';
  content.style.width = '100vw';
  content.style.height = '100vh';
  content.style.margin = '0';
  content.style.borderRadius = '0';
  content.style.padding = '0';
  container.style.borderRadius = '0';
  container.style.marginBottom = '0';
  container.style.height = 'calc(100vh - 40px)';
  video.style.height = '100%';
  video.style.objectFit = 'contain';
  // Toolbar overlay
  modal.dataset.fullscreen = 'true';
}

function removeFullscreenStyle(modal, content, container, video) {
  content.style.maxWidth = '';
  content.style.width = '';
  content.style.height = '';
  content.style.margin = '';
  content.style.borderRadius = '';
  content.style.padding = '';
  container.style.borderRadius = '';
  container.style.marginBottom = '';
  container.style.height = '';
  video.style.height = '';
  video.style.objectFit = '';
  modal.dataset.fullscreen = 'false';
}

// Exit fullscreen when native fullscreen ends (Escape key)
document.addEventListener('fullscreenchange', () => {
  if (!document.fullscreenElement && supportFullscreen) {
    supportFullscreen = false;
    const modal = document.getElementById('supportModal');
    const content = modal?.querySelector('.modal-content');
    const container = document.getElementById('supportVideoContainer');
    const video = document.getElementById('supportVideo');
    if (modal && content && container && video) {
      removeFullscreenStyle(modal, content, container, video);
    }
  }
});

// Keyboard shortcut: F for fullscreen, Escape to exit
document.addEventListener('keydown', (e) => {
  const modal = document.getElementById('supportModal');
  if (!modal || modal.style.display === 'none') return;
  const viewerStep = document.getElementById('supportStep_viewer');
  if (!viewerStep || viewerStep.style.display === 'none') return;

  if (e.key === 'f' || e.key === 'F') {
    e.preventDefault();
    toggleSupportFullscreen();
  }
  // When in fullscreen (native or CSS), Escape should only exit fullscreen, not close modal
  if (e.key === 'Escape' && (supportFullscreen || document.fullscreenElement)) {
    e.preventDefault();
    e.stopPropagation();
    e.stopImmediatePropagation();
    if (supportFullscreen) toggleSupportFullscreen();
  }
}, true); // capture phase to intercept before modal close handlers

// Show toolbar on hover over video
document.addEventListener('DOMContentLoaded', () => {
  const container = document.getElementById('supportVideoContainer');
  const toolbar = document.getElementById('supportVideoToolbar');
  if (container && toolbar) {
    container.addEventListener('mouseenter', () => toolbar.style.opacity = '1');
    container.addEventListener('mouseleave', () => toolbar.style.opacity = '0');
  }
});

// Update resolution display
function updateSupportResolution() {
  const video = document.getElementById('supportVideo');
  const resEl = document.getElementById('supportViewerRes');
  if (video && resEl && video.videoWidth > 0) {
    resEl.textContent = `${video.videoWidth}×${video.videoHeight}`;
  }
  requestAnimationFrame(updateSupportResolution);
}
requestAnimationFrame(updateSupportResolution);

// ============================================================================
// Main Preview Integration
// ============================================================================

function showSupportInPreview() {
  supportInPreview = true;

  // Create session tab
  SessionManager.createSession('quick-support', '🆘 Quick Support');
  SessionManager.updateSessionStatus('quick-support', 'connected');

  // Show video element, hide canvas (support uses WebRTC video track, not canvas)
  const previewVideo = document.getElementById('previewVideo');
  const previewCanvas = document.getElementById('previewCanvas');
  if (previewVideo) previewVideo.style.display = 'block';
  if (previewCanvas) previewCanvas.style.display = 'none';

  // Update device name in toolbar
  const connectedDeviceName = document.getElementById('connectedDeviceName');
  if (connectedDeviceName) connectedDeviceName.textContent = '🆘 Quick Support';

  // Hide toolbar center buttons (view-only, no remote control)
  const toolbarCenter = document.querySelector('.toolbar-center');
  if (toolbarCenter) toolbarCenter.style.display = 'none';

  // Change disconnect button
  const disconnectBtn = document.getElementById('disconnectBtn');
  if (disconnectBtn) {
    disconnectBtn.textContent = 'Afslut Support';
    disconnectBtn.onclick = endSupportSession;
  }

  // Override tab close button for quick-support
  const tab = document.querySelector('[data-session-id="quick-support"]');
  if (tab) {
    const closeBtn = tab.querySelector('.tab-close');
    if (closeBtn) {
      closeBtn.onclick = (e) => {
        e.stopPropagation();
        endSupportSession();
      };
    }
  }

  // Close the modal (just hide, don't cleanup - supportInPreview flag prevents it)
  closeSupportModal();
}

function removeSupportFromPreview() {
  if (!supportInPreview) return;
  supportInPreview = false;

  // Clear preview video
  const previewVideo = document.getElementById('previewVideo');
  const previewCanvas = document.getElementById('previewCanvas');
  if (previewVideo) {
    previewVideo.srcObject = null;
    previewVideo.style.display = '';
  }
  if (previewCanvas) previewCanvas.style.display = '';

  // Remove quick-support session from SessionManager
  // (avoid closeSession which calls disconnectFromDevice)
  const tab = document.querySelector('[data-session-id="quick-support"]');
  if (tab) tab.remove();
  SessionManager.sessions.delete('quick-support');

  if (SessionManager.activeSessionId === 'quick-support') {
    const remaining = Array.from(SessionManager.sessions.keys());
    if (remaining.length > 0) {
      SessionManager.switchToSession(remaining[0]);
    } else {
      SessionManager.activeSessionId = null;
      const previewIdle = document.getElementById('previewIdle');
      if (previewIdle) previewIdle.style.display = 'flex';
      const previewToolbar = document.getElementById('previewToolbar');
      if (previewToolbar) previewToolbar.style.display = 'none';
    }
  }
  SessionManager.updateUI();

  // Restore toolbar center buttons
  const toolbarCenter = document.querySelector('.toolbar-center');
  if (toolbarCenter) toolbarCenter.style.display = '';

  // Restore disconnect button
  const disconnectBtn = document.getElementById('disconnectBtn');
  if (disconnectBtn) {
    disconnectBtn.textContent = 'Afbryd';
    disconnectBtn.onclick = null;
  }
}

// ============================================================================
// Cleanup
// ============================================================================

function cleanupSupportViewer() {
  removeSupportFromPreview();

  if (supportPollingInterval) {
    clearInterval(supportPollingInterval);
    supportPollingInterval = null;
  }

  if (supportSignalingChannel) {
    supabase.removeChannel(supportSignalingChannel);
    supportSignalingChannel = null;
  }

  if (supportControllerChannel) {
    supabase.removeChannel(supportControllerChannel);
    supportControllerChannel = null;
  }
  if (supportControllerPollInterval) {
    clearInterval(supportControllerPollInterval);
    supportControllerPollInterval = null;
  }

  if (supportViewerPC) {
    try { supportViewerPC.close(); } catch (e) {}
    supportViewerPC = null;
  }

  supportProcessedIds.clear();
  supportPendingIce = [];
  currentSupportSession = null;
  forgetActiveAISupportSession();
  setAISupportStatus('hidden');
}

async function endSupportSession() {
  const sessionId = currentSupportSession?.session_id;
  const modal = document.getElementById('supportModal');
  if (!sessionId) {
    cleanupSupportViewer();
    if (modal) modal.style.display = 'none';
    return;
  }

  try {
    const { data: { session: authSession } } = await supabase.auth.getSession();
    if (!authSession) throw new Error('Administrator session expired');
    const revokeResponse = await fetch(`${SUPABASE_CONFIG.url}/functions/v1/support-signal`, {
      method: 'POST',
      headers: {
        Authorization: `Bearer ${authSession.access_token}`,
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        action: 'revoke',
        session_id: sessionId,
        reason: 'Ended by administrator',
      }),
    });
    if (!revokeResponse.ok) throw new Error(`Revoke failed (${revokeResponse.status})`);
  } catch (error) {
    console.error('Failed to revoke support session:', error);
    const statusEl = document.getElementById('supportShareStatus') || document.getElementById('supportViewerStatus');
    if (statusEl) statusEl.textContent = `Kunne ikke afslutte sikkert: ${error.message}`;
    const revokeButton = document.getElementById('supportAiRevokeBtn');
    if (revokeButton) {
      revokeButton.disabled = false;
      revokeButton.textContent = 'Prøv at afslutte igen';
    }
    return;
  }

  cleanupSupportViewer(); // includes removeSupportFromPreview
  const aiRevokeBtn = document.getElementById('supportAiRevokeBtn');
  if (aiRevokeBtn) aiRevokeBtn.style.display = 'none';
  if (modal) modal.style.display = 'flex';
  showSupportStep('audit');
  await loadSupportAudit(sessionId);
}

async function loadSupportAudit(sessionId) {
  const list = document.getElementById('supportAuditList');
  if (!list) return;
  list.textContent = 'Henter supportoversigt...';

  const { data, error } = await supabase
    .from('support_action_audit')
    .select('created_at, actor_type, action_type, status, summary, target, details, completed_at, verified')
    .eq('support_session_id', sessionId)
    .order('created_at', { ascending: true });
  if (error) {
    list.textContent = `Kunne ikke hente oversigten: ${error.message}`;
    return;
  }
  if (!data?.length) {
    list.textContent = 'Der blev ikke registreret nogen handlinger.';
    return;
  }

  list.replaceChildren(...data.map((entry) => {
    const row = document.createElement('div');
    row.style.cssText = 'padding: 0.65rem 0; border-bottom: 1px solid var(--border);';
    const time = new Date(entry.created_at).toLocaleString('da-DK');
    const verification = entry.verified ? 'verified' : 'reported';
    row.textContent = `${time} | ${entry.actor_type} | ${entry.action_type} | ${entry.status} | ${verification} | ${entry.summary}`;
    return row;
  }));
}

// ============================================================================
// Handle ?support=SESSION_ID parameter (from controller)
// ============================================================================

(function checkSupportParam() {
  const params = new URLSearchParams(window.location.search);
  const supportSessionId = params.get('support');
  if (supportSessionId) {
    // Auto-open support viewer for this session
    document.addEventListener('DOMContentLoaded', () => {
      setTimeout(() => {
        showSupportModal();
        showSupportStep('share');
        // Set session info
        currentSupportSession = { session_id: supportSessionId };
        document.getElementById('supportShareStatus').textContent = 'Venter på at personen deler sin skærm...';
        waitForSharerReady(supportSessionId);
      }, 500);
    });
  }
})();

// ============================================================================
// Public Support Link — Auto-connect listener
// ============================================================================

let publicSupportChannel = null;

function startPublicSupportListener() {
  if (publicSupportChannel) return; // already listening

  publicSupportChannel = supabase
    .channel('public-support-sessions')
    .on('postgres_changes', {
      event: 'UPDATE',
      schema: 'public',
      table: 'support_sessions',
      filter: 'is_public=eq.true',
    }, (payload) => {
      if (payload.new.status === 'active' && !currentSupportSession) {
        debug('Public support session became active:', payload.new.id);
        handleIncomingPublicSession(payload.new);
      }
    })
    .subscribe();

  debug('Public support listener started');
}

function stopPublicSupportListener() {
  if (publicSupportChannel) {
    supabase.removeChannel(publicSupportChannel);
    publicSupportChannel = null;
    debug('Public support listener stopped');
  }
}

async function handleIncomingPublicSession(session) {
  // Don't auto-connect if already viewing a support session
  if (currentSupportSession) {
    showToast('Ny public support session venter (allerede i session)', 'warning');
    return;
  }

  showToast('Indkommende support session...', 'info');

  currentSupportSession = {
    session_id: session.id,
    token: session.token,
  };

  // Show modal and go directly to viewer step
  showSupportModal();
  showSupportStep('viewer');

  // Auto-connect as offerer
  await connectToSupport(session.id);
}

// Load public support toggle state
async function loadPublicSupportState() {
  try {
    const res = await fetch(`${SUPABASE_CONFIG.url}/functions/v1/support-signal`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'apikey': SUPABASE_CONFIG.anonKey },
      body: JSON.stringify({ action: 'check-public' }),
    });
    const { enabled } = await res.json();
    const toggle = document.getElementById('publicSupportToggle');
    if (toggle) toggle.checked = enabled;
    updatePublicSupportUI(enabled);
    return enabled;
  } catch (e) {
    console.error('Failed to load public support state:', e);
    return false;
  }
}

async function togglePublicSupport() {
  const { data: { session } } = await supabase.auth.getSession();
  if (!session) {
    showToast('Du skal være logget ind', 'error');
    return;
  }

  try {
    const res = await fetch(`${SUPABASE_CONFIG.url}/functions/v1/support-signal`, {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${session.access_token}`,
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ action: 'toggle-public' }),
    });
    const data = await res.json();
    if (!res.ok || data.error) {
      throw new Error(data.error || 'Kunne ikke ændre indstilling');
    }
    updatePublicSupportUI(data.enabled);
    showToast(data.enabled ? 'Public support link aktiveret' : 'Public support link deaktiveret', 'success');
  } catch (error) {
    showToast('Fejl: ' + error.message, 'error');
    // Revert toggle
    const toggle = document.getElementById('publicSupportToggle');
    if (toggle) toggle.checked = !toggle.checked;
  }
}

function updatePublicSupportUI(enabled) {
  const linkSection = document.getElementById('publicLinkSection');
  if (linkSection) linkSection.style.display = enabled ? 'block' : 'none';

  // Start/stop listener based on enabled state
  if (enabled) {
    startPublicSupportListener();
  } else {
    stopPublicSupportListener();
  }
}

function copyPublicLink() {
  const input = document.getElementById('publicSupportLink');
  if (!input) return;
  navigator.clipboard.writeText(input.value).then(() => {
    const btn = input.nextElementSibling;
    if (btn) {
      const orig = btn.textContent;
      btn.textContent = 'Kopieret!';
      setTimeout(() => btn.textContent = orig, 2000);
    }
  });
}

async function restoreActiveAISupportSession(user) {
  let saved;
  try {
    saved = JSON.parse(localStorage.getItem(ACTIVE_AI_SUPPORT_STORAGE_KEY) || 'null');
  } catch (_) {
    forgetActiveAISupportSession();
    return;
  }
  if (!saved?.session_id || saved.user_id !== user.id) return;

  const { data: session, error } = await supabase
    .from('support_sessions')
    .select('id, pin, token, status, support_mode, requested_scopes, expires_at')
    .eq('id', saved.session_id)
    .eq('support_mode', 'ai')
    .in('status', ['pending', 'active'])
    .maybeSingle();
  if (error || !session) {
    forgetActiveAISupportSession();
    return;
  }

  currentSupportSession = {
    session_id: session.id,
    pin: session.pin,
    token: session.token,
    share_url: `${SUPPORT_SITE_BASE}/support.html?token=${encodeURIComponent(session.token)}`,
    expires_at: session.expires_at,
    support_mode: session.support_mode,
    requested_scopes: session.requested_scopes,
  };
  const modal = document.getElementById('supportModal');
  if (!modal) return;
  modal.style.display = 'none';
  showSupportStep('share');
  document.getElementById('supportPin').textContent = session.pin || '';
  document.getElementById('supportLink').value = currentSupportSession.share_url;
  document.getElementById('supportSessionId').textContent = session.id;
  setSupportExpiry(session);
  document.getElementById('supportShareStatus').textContent =
    'AI-session gendannet efter genindlæsning. Venter på klient/Ubuntu AI...';
  setAISupportStatus('waiting', 'AI-session gendannet. Venter på klient/Ubuntu AI...');
  const revokeButton = document.getElementById('supportAiRevokeBtn');
  if (revokeButton) revokeButton.style.display = 'block';
  const ubuntuButton = document.getElementById('supportUbuntuConnectBtn');
  if (ubuntuButton) ubuntuButton.style.display = 'block';
  watchUbuntuController(session.id);
  requestUbuntuAI();
}

// Restore any active AI session before allowing a new one to be created.
document.addEventListener('DOMContentLoaded', async () => {
  try {
    const { data: { session } } = await supabase.auth.getSession();
    if (session) {
      await restoreActiveAISupportSession(session.user);
      const enabled = await loadPublicSupportState();
      if (enabled) startPublicSupportListener();
    }
  } catch (error) {
    console.error('Support session restore failed:', error);
  } finally {
    resolveSupportRestore();
  }
});

// Export
window.showSupportModal = showSupportModal;
window.closeSupportModal = closeSupportModal;
window.onCreateSupportSession = onCreateSupportSession;
window.copySupportLink = copySupportLink;
window.copySupportPin = copySupportPin;
window.generateAISupportCode = generateAISupportCode;
window.endSupportSession = endSupportSession;
window.toggleSupportFullscreen = toggleSupportFullscreen;
window.togglePublicSupport = togglePublicSupport;
window.loadPublicSupportState = loadPublicSupportState;
window.copyPublicLink = copyPublicLink;
