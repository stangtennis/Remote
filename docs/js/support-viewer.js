// Quick Support Viewer - Dashboard side
// Creates support sessions, connects via WebRTC (offerer), displays video

let supportViewerPC = null;
let supportSignalingChannel = null;
let supportPollingInterval = null;
let supportProcessedIds = new Set();
let supportPendingIce = [];
let currentSupportSession = null;

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
    const response = await fetch(`${SUPABASE_CONFIG.url}/functions/v1/create-support-session`, {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${session.access_token}`,
        'Content-Type': 'application/json',
      },
      body: '{}',
    });

    const data = await response.json();
    if (!response.ok || data.error) {
      throw new Error(data.error || 'Kunne ikke oprette support session');
    }

    currentSupportSession = data;
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
    showSupportStep('create');
  }
}

function closeSupportModal() {
  const modal = document.getElementById('supportModal');
  if (modal) modal.style.display = 'none';
  cleanupSupportViewer();
}

function showSupportStep(step) {
  document.querySelectorAll('.support-step').forEach(el => el.style.display = 'none');
  const el = document.getElementById(`supportStep_${step}`);
  if (el) el.style.display = 'block';
}

async function onCreateSupportSession() {
  const btn = document.getElementById('supportCreateBtn');
  if (btn) {
    btn.disabled = true;
    btn.textContent = 'Opretter...';
  }

  const session = await createSupportSession();

  if (btn) {
    btn.disabled = false;
    btn.textContent = 'Opret ny session';
  }

  if (!session) return;

  // Show share step with PIN and link
  showSupportStep('share');
  document.getElementById('supportPin').textContent = session.pin;
  document.getElementById('supportLink').value = session.share_url;

  // Calculate expiry time
  const expiresAt = new Date(session.expires_at);
  document.getElementById('supportExpiry').textContent =
    `Udløber kl. ${expiresAt.toLocaleTimeString('da-DK', { hour: '2-digit', minute: '2-digit' })}`;

  // Subscribe for sharer ready signal
  waitForSharerReady(session.session_id);
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

// ============================================================================
// Wait for Sharer Ready
// ============================================================================

function waitForSharerReady(sessionId) {
  const statusEl = document.getElementById('supportShareStatus');
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
      });
      if (turnResp.ok) {
        const turnData = await turnResp.json();
        iceServers = turnData.iceServers;
      }
    } catch (e) {
      console.warn('Failed to fetch TURN credentials:', e);
    }

    const forceRelay = new URLSearchParams(window.location.search).get('relay') === 'true';
    const configuration = {
      iceServers,
      ...(forceRelay && { iceTransportPolicy: 'relay' }),
    };

    debug('Support viewer: creating peer connection');
    supportViewerPC = new RTCPeerConnection(configuration);

    // Handle remote video track
    supportViewerPC.ontrack = (event) => {
      debug('Support viewer: received remote track', event.track.kind);
      const video = document.getElementById('supportVideo');
      if (video && event.streams[0]) {
        video.srcObject = event.streams[0];
      }
    };

    // Send ICE candidates
    supportViewerPC.onicecandidate = async (event) => {
      if (event.candidate) {
        await supabase
          .from('session_signaling')
          .insert({
            session_id: sessionId,
            from_side: 'dashboard',
            msg_type: 'ice',
            payload: event.candidate,
          });
      }
    };

    // Connection state
    supportViewerPC.onconnectionstatechange = () => {
      const state = supportViewerPC.connectionState;
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
          break;
        case 'disconnected':
        case 'failed':
          if (statusEl) statusEl.textContent = 'Afbrudt';
          break;
      }
    };

    // Create offer (receive video only, no data channel)
    const offer = await supportViewerPC.createOffer({
      offerToReceiveVideo: true,
      offerToReceiveAudio: false,
    });
    await supportViewerPC.setLocalDescription(offer);

    // Send offer to sharer
    await supabase
      .from('session_signaling')
      .insert({
        session_id: sessionId,
        from_side: 'dashboard',
        msg_type: 'offer',
        payload: { type: 'offer', sdp: offer.sdp },
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

  supportPollingInterval = setInterval(async () => {
    try {
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
        await handleSupportViewerSignal(signal);
      }
    } catch (err) {
      console.error('Support polling error:', err);
    }
  }, 500);
}

async function handleSupportViewerSignal(signal) {
  if (signal.from_side !== 'support') return;
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
  if (e.key === 'Escape' && supportFullscreen) {
    e.preventDefault();
    toggleSupportFullscreen();
  }
});

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
// Cleanup
// ============================================================================

function cleanupSupportViewer() {
  if (supportPollingInterval) {
    clearInterval(supportPollingInterval);
    supportPollingInterval = null;
  }

  if (supportSignalingChannel) {
    supabase.removeChannel(supportSignalingChannel);
    supportSignalingChannel = null;
  }

  if (supportViewerPC) {
    try { supportViewerPC.close(); } catch (e) {}
    supportViewerPC = null;
  }

  supportProcessedIds.clear();
  supportPendingIce = [];
  currentSupportSession = null;
}

function endSupportSession() {
  if (currentSupportSession) {
    // Send bye signal
    supabase.from('session_signaling').insert({
      session_id: currentSupportSession.session_id,
      from_side: 'dashboard',
      msg_type: 'bye',
      payload: { reason: 'viewer_closed' },
    });
  }
  cleanupSupportViewer();
  closeSupportModal();
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

// Export
window.showSupportModal = showSupportModal;
window.closeSupportModal = closeSupportModal;
window.onCreateSupportSession = onCreateSupportSession;
window.copySupportLink = copySupportLink;
window.endSupportSession = endSupportSession;
window.toggleSupportFullscreen = toggleSupportFullscreen;
