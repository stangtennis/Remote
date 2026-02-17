// Main Application Logic
// Coordinates between devices, sessions, and WebRTC

let currentDevice = null;

// Initialize app on dashboard load
document.addEventListener('DOMContentLoaded', async () => {
  debug('Dashboard initialized');

  // Verify authentication
  const session = await checkAuth();
  if (!session) return;

  // Initialize modules
  await initDevices();

  // Set up event listeners
  setupEventListeners();

  // Subscribe to realtime updates
  subscribeToRealtime();
});

function setupEventListeners() {
  // Refresh devices button
  const refreshBtn = document.getElementById('refreshDevicesBtn');
  if (refreshBtn) {
    refreshBtn.addEventListener('click', () => {
      loadDevices();
    });
  }

  // Download agent button
  const downloadBtn = document.getElementById('downloadAgentBtn');
  if (downloadBtn) {
    downloadBtn.addEventListener('click', () => {
      window.open('https://github.com/stangtennis/Remote/releases/latest/download/remote-agent.exe', '_blank');
    });
  }

  // Download controller button
  const downloadControllerBtn = document.getElementById('downloadControllerBtn');
  if (downloadControllerBtn) {
    downloadControllerBtn.addEventListener('click', () => {
      window.open('https://github.com/stangtennis/Remote/releases/latest/download/controller.exe', '_blank');
    });
  }

  // End session button (old UI)
  const endSessionBtn = document.getElementById('endSessionBtn');
  if (endSessionBtn) {
    endSessionBtn.addEventListener('click', async () => {
      await endSession();
    });
  }

  // Disconnect button (new preview toolbar)
  const disconnectBtn = document.getElementById('disconnectBtn');
  if (disconnectBtn) {
    disconnectBtn.addEventListener('click', async () => {
      await endSession();
    });
  }

  // Toggle stats button
  const toggleStatsBtn = document.getElementById('toggleStatsBtn');
  const statsContent = document.getElementById('statsContent');
  if (toggleStatsBtn && statsContent) {
    toggleStatsBtn.addEventListener('click', () => {
      statsContent.style.display =
        statsContent.style.display === 'none' ? 'block' : 'none';
    });
  }

  // Quality toggle button (H264/JPEG mode)
  const qualityToggleBtn = document.getElementById('qualityToggleBtn');
  if (qualityToggleBtn) {
    let currentMode = 'tiles';
    qualityToggleBtn.addEventListener('click', () => {
      const modes = ['tiles', 'h264', 'hybrid'];
      const modeNames = { tiles: 'JPEG Tiles', h264: 'H.264', hybrid: 'Hybrid' };
      const currentIndex = modes.indexOf(currentMode);
      currentMode = modes[(currentIndex + 1) % modes.length];

      if (typeof window.sendControlEvent === 'function') {
        window.sendControlEvent({
          type: 'set_mode',
          mode: currentMode
        });
        debug(`🎬 Switched to ${modeNames[currentMode]} mode`);

        qualityToggleBtn.title = `Mode: ${modeNames[currentMode]} (click to change)`;
        qualityToggleBtn.textContent = currentMode === 'tiles' ? '🎚️' : (currentMode === 'h264' ? '🎬' : '🔄');
      } else {
        console.warn('sendControlEvent not available - not connected?');
      }
    });
  }
}

function subscribeToRealtime() {
  // Device changes are handled by subscribeToDeviceUpdates() in devices.js
}

async function startSession(device) {
  currentDevice = device;

  // Create session in SessionManager (gets ctx)
  if (!window.SessionManager) return;

  const ctx = window.SessionManager.createSession(device.device_id, device.device_name);
  if (!ctx) {
    return; // Max sessions reached or already exists (switched to it)
  }

  try {
    // Get current auth session token
    const { data: { session } } = await supabase.auth.getSession();

    if (!session) {
      throw new Error('Not authenticated. Please log in again.');
    }

    // Generate a unique controller ID for this dashboard instance
    const controllerId = `dashboard-${session.user.id}-${Date.now()}`;

    // Use claim_device_connection to atomically take over any existing sessions
    debug('🔒 Claiming device connection (will kick any existing controllers)...');
    const { data: claimResult, error: claimError } = await supabase.rpc('claim_device_connection', {
      p_device_id: device.device_id,
      p_controller_id: controllerId,
      p_controller_type: 'dashboard'
    });

    if (claimError) {
      console.warn('⚠️ claim_device_connection not available, falling back to old method:', claimError);
      await supabase
        .from('remote_sessions')
        .update({ status: 'expired' })
        .eq('device_id', device.device_id)
        .in('status', ['pending', 'active']);
    } else {
      debug('✅ Device claimed:', claimResult);
      if (claimResult.kicked_sessions > 0) {
        debug(`🔴 Kicked ${claimResult.kicked_sessions} existing session(s)`);
      }
    }

    // Call session-token Edge Function
    const { data, error } = await supabase.functions.invoke('session-token', {
      body: {
        device_id: device.device_id,
        use_pin: true
      },
      headers: {
        Authorization: `Bearer ${session.access_token}`
      }
    });

    if (error) {
      console.error('Full error object:', error);
      console.error('Error context:', error.context);

      if (error.context && error.context instanceof Response) {
        const errorText = await error.context.text();
        console.error('Response body:', errorText);
        try {
          const errorJson = JSON.parse(errorText);
          console.error('Parsed error:', errorJson);
          throw new Error(errorJson.error || errorJson.message || 'Unknown error');
        } catch (e) {
          throw new Error(errorText || error.message);
        }
      }
      throw error;
    }

    // Store session data on ctx
    ctx.sessionData = data;
    ctx.sessionData.device_id = device.device_id;
    window.currentSession = ctx.sessionData;

    // Show preview UI elements
    const previewIdle = document.getElementById('previewIdle');
    const previewConnecting = document.getElementById('previewConnecting');
    const connectingDeviceName = document.getElementById('connectingDeviceName');

    if (previewIdle) previewIdle.style.display = 'none';
    if (previewConnecting) {
      previewConnecting.style.display = 'flex';
      if (connectingDeviceName) connectingDeviceName.textContent = device.device_name;
    }

    // Also update old session UI if present
    const sessionSection = document.getElementById('sessionSection');
    if (sessionSection) {
      sessionSection.style.display = 'block';
      const sessionDeviceName = document.getElementById('sessionDeviceName');
      const sessionPin = document.getElementById('sessionPin');
      const sessionStatus = document.getElementById('sessionStatus');
      if (sessionDeviceName) sessionDeviceName.textContent = device.device_name;
      if (sessionPin) sessionPin.textContent = data.pin;
      if (sessionStatus) {
        sessionStatus.textContent = 'Connecting...';
        sessionStatus.className = 'status-badge pending';
      }
    }

    // Subscribe to signaling FIRST (before sending offer) — per-session
    subscribeToSessionSignaling(ctx.sessionData.session_id, ctx);

    // Small delay to ensure subscription is active
    await new Promise(resolve => setTimeout(resolve, 200));

    // Initialize WebRTC (sends offer) — per-session
    await initWebRTC(ctx.sessionData, ctx);

  } catch (error) {
    console.error('Failed to start session:', error);
    // Remove session tab on error
    if (window.SessionManager) {
      window.SessionManager.closeSession(device.device_id);
    }
    showToast('Kunne ikke starte session: ' + error.message, 'error');
  }
}

async function endSession(deviceId) {
  // Default to active session if no deviceId specified
  if (!deviceId) {
    deviceId = window.SessionManager?.activeSessionId;
  }
  if (!deviceId) return;

  const ctx = window.SessionManager?.sessions.get(deviceId);
  if (!ctx) return;

  // Cancel any in-progress reconnect
  cancelReconnect(deviceId);

  const sessionId = ctx.sessionData?.session_id;
  debug('🔌 Ending session:', sessionId, 'for device:', deviceId);

  try {
    // Update session status in database
    if (sessionId) {
      await supabase
        .from('remote_sessions')
        .update({ status: 'ended', ended_at: new Date().toISOString() })
        .eq('id', sessionId);
    }

    // Stop signaling for this session
    if (window.stopSessionPolling) {
      window.stopSessionPolling(ctx);
    }
    if (ctx.signalingChannel) {
      supabase.removeChannel(ctx.signalingChannel);
      ctx.signalingChannel = null;
    }

    // Clean up WebRTC for this session
    if (typeof window.cleanupSessionWebRTC === 'function') {
      window.cleanupSessionWebRTC(ctx);
    }

    // Close session tab + switch to another
    if (window.SessionManager) {
      window.SessionManager.closeSession(deviceId);
    }

    // If no sessions remain, clean up input and show idle
    if (!window.SessionManager || window.SessionManager.getSessionCount() === 0) {
      if (typeof cleanupInputCapture === 'function') {
        cleanupInputCapture();
      }

      const sessionSection = document.getElementById('sessionSection');
      if (sessionSection) sessionSection.style.display = 'none';

      const previewToolbar = document.getElementById('previewToolbar');
      if (previewToolbar) previewToolbar.style.display = 'none';

      const previewIdle = document.getElementById('previewIdle');
      if (previewIdle) previewIdle.style.display = 'flex';

      const previewConnecting = document.getElementById('previewConnecting');
      if (previewConnecting) previewConnecting.style.display = 'none';

      currentDevice = null;
    }

    debug('✅ Session ended successfully for device:', deviceId);

  } catch (error) {
    console.error('❌ Failed to end session:', error);
  }
}

// Expose for disconnect button and session tab close
window.disconnectFromDevice = endSession;

// Clean up ALL sessions on page unload/refresh
window.addEventListener('beforeunload', (e) => {
  if (!window.SessionManager || window.SessionManager.getSessionCount() === 0) return;

  debug('Page unloading - cleaning up all sessions');

  // Clean up input capture once
  if (typeof cleanupInputCapture === 'function') {
    cleanupInputCapture();
  }

  // Iterate all sessions and clean up
  for (const [deviceId, ctx] of window.SessionManager.sessions) {
    const sessionId = ctx.sessionData?.session_id;
    if (sessionId) {
      // Try to update DB (async, may not complete)
      supabase
        .from('remote_sessions')
        .update({ status: 'ended', ended_at: new Date().toISOString() })
        .eq('id', sessionId)
        .then(() => debug('Session ended:', sessionId))
        .catch(err => console.error('Session end error:', err));
    }

    // Close peer connection
    if (ctx.peerConnection) {
      try { ctx.peerConnection.close(); } catch (e) {}
    }

    // Close data channel
    if (ctx.dataChannel) {
      try { ctx.dataChannel.close(); } catch (e) {}
    }

    // Stop polling
    if (ctx.pollingInterval) {
      clearInterval(ctx.pollingInterval);
    }

    // Stop intervals
    if (ctx.bandwidthInterval) {
      clearInterval(ctx.bandwidthInterval);
    }
    if (ctx.statsInterval) {
      clearInterval(ctx.statsInterval);
    }
  }
});

// Monitor visibility changes
document.addEventListener('visibilitychange', () => {
  if (document.hidden && window.SessionManager?.getSessionCount() > 0) {
    debug('Page hidden - keeping sessions alive but monitoring');
  }
});

// ==================== AUTO-RECONNECT ====================

const RECONNECT_MAX_DURATION = 2 * 60 * 1000; // 2 minutes
const RECONNECT_MAX_ATTEMPTS = 8;
const RECONNECT_BACKOFF = [1000, 2000, 4000, 8000, 16000, 30000, 30000, 30000]; // ms

async function reconnectSession(deviceId) {
  const ctx = window.SessionManager?.sessions.get(deviceId);
  if (!ctx || ctx.reconnectState !== 'reconnecting') return;

  // Check max duration
  if (Date.now() - ctx.reconnectStartedAt > RECONNECT_MAX_DURATION) {
    debug('⏰ Reconnect max duration exceeded for', deviceId);
    ctx.reconnectState = 'gave_up';
    const reconnectOverlay = document.getElementById('previewReconnecting');
    if (reconnectOverlay && window.SessionManager?.activeSessionId === deviceId) {
      reconnectOverlay.style.display = 'none';
    }
    showToast('Kunne ikke genoprette forbindelsen efter 2 minutter.', 'error');
    return;
  }

  // Check max attempts
  if (ctx.reconnectAttempt >= RECONNECT_MAX_ATTEMPTS) {
    debug('🛑 Reconnect max attempts reached for', deviceId);
    ctx.reconnectState = 'gave_up';
    const reconnectOverlay = document.getElementById('previewReconnecting');
    if (reconnectOverlay && window.SessionManager?.activeSessionId === deviceId) {
      reconnectOverlay.style.display = 'none';
    }
    showToast('Genopretter mislykkedes efter 8 forsøg.', 'error');
    return;
  }

  ctx.reconnectAttempt++;
  const delay = RECONNECT_BACKOFF[Math.min(ctx.reconnectAttempt - 1, RECONNECT_BACKOFF.length - 1)];

  debug(`🔄 Reconnect attempt ${ctx.reconnectAttempt}/${RECONNECT_MAX_ATTEMPTS} for ${deviceId} (delay: ${delay}ms)`);

  // Update UI
  if (window.SessionManager?.activeSessionId === deviceId) {
    const statusEl = document.getElementById('reconnectStatus');
    if (statusEl) statusEl.textContent = `Forsøg ${ctx.reconnectAttempt}/${RECONNECT_MAX_ATTEMPTS}`;
  }

  // Wait for backoff delay
  await new Promise(resolve => {
    ctx.reconnectTimer = setTimeout(resolve, delay);
  });

  // Check if reconnect was cancelled during wait
  if (ctx.reconnectState !== 'reconnecting') {
    debug('🛑 Reconnect cancelled during backoff for', deviceId);
    return;
  }

  try {
    // Clean up existing WebRTC
    if (typeof window.cleanupSessionWebRTC === 'function') {
      window.cleanupSessionWebRTC(ctx);
    }

    // Stop existing signaling
    if (window.stopSessionPolling) {
      window.stopSessionPolling(ctx);
    }
    if (ctx.signalingChannel) {
      supabase.removeChannel(ctx.signalingChannel);
      ctx.signalingChannel = null;
    }

    // Get fresh auth session
    const { data: { session } } = await supabase.auth.getSession();
    if (!session) {
      debug('❌ No auth session for reconnect');
      ctx.reconnectState = 'gave_up';
      return;
    }

    // Request new session token
    const { data, error } = await supabase.functions.invoke('session-token', {
      body: {
        device_id: deviceId,
        use_pin: true
      },
      headers: {
        Authorization: `Bearer ${session.access_token}`
      }
    });

    if (error) throw error;

    // Update session data
    ctx.sessionData = data;
    ctx.sessionData.device_id = deviceId;
    ctx.processedSignalIds = new Set();
    ctx.pendingIceCandidates = [];

    // Set global ref if active
    if (window.SessionManager?.activeSessionId === deviceId) {
      window.currentSession = ctx.sessionData;
    }

    // Subscribe to signaling
    subscribeToSessionSignaling(ctx.sessionData.session_id, ctx);
    await new Promise(resolve => setTimeout(resolve, 200));

    // Init new WebRTC connection
    await initWebRTC(ctx.sessionData, ctx);

    // Wait for connection (max 15s)
    const connected = await waitForConnection(ctx, 15000);
    if (connected) {
      debug('✅ Reconnect succeeded for', deviceId);
      // State will be updated by onconnectionstatechange handler
      return;
    }

    // Not connected yet - try again
    debug('⏳ Reconnect attempt timed out for', deviceId);
    if (ctx.reconnectState === 'reconnecting') {
      reconnectSession(deviceId);
    }

  } catch (err) {
    console.error('❌ Reconnect error for', deviceId, ':', err);
    if (ctx.reconnectState === 'reconnecting') {
      reconnectSession(deviceId);
    }
  }
}

function waitForConnection(ctx, timeout) {
  return new Promise(resolve => {
    const start = Date.now();
    const check = () => {
      if (!ctx.peerConnection) {
        resolve(false);
        return;
      }
      if (ctx.peerConnection.connectionState === 'connected') {
        resolve(true);
        return;
      }
      if (Date.now() - start > timeout) {
        resolve(false);
        return;
      }
      setTimeout(check, 500);
    };
    check();
  });
}

function cancelReconnect(deviceId) {
  const ctx = window.SessionManager?.sessions.get(deviceId);
  if (!ctx) return;

  if (ctx.reconnectTimer) {
    clearTimeout(ctx.reconnectTimer);
    ctx.reconnectTimer = null;
  }
  ctx.reconnectState = 'idle';
  ctx.reconnectAttempt = 0;
  ctx.reconnectStartedAt = null;

  const reconnectOverlay = document.getElementById('previewReconnecting');
  if (reconnectOverlay) reconnectOverlay.style.display = 'none';

  debug('🛑 Reconnect cancelled for', deviceId);
}

// Export reconnect functions
window.reconnectSession = reconnectSession;
window.cancelReconnect = cancelReconnect;

// Export for other modules
window.startSession = startSession;
window.endSession = endSession;
