// Main Application Logic
// Coordinates between devices, sessions, and WebRTC

let currentSession = null;
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
    let currentMode = 'tiles'; // Default to JPEG tiles
    qualityToggleBtn.addEventListener('click', () => {
      // Cycle through modes: tiles -> h264 -> hybrid -> tiles
      const modes = ['tiles', 'h264', 'hybrid'];
      const modeNames = { tiles: 'JPEG Tiles', h264: 'H.264', hybrid: 'Hybrid' };
      const currentIndex = modes.indexOf(currentMode);
      currentMode = modes[(currentIndex + 1) % modes.length];
      
      // Send mode change to agent
      if (typeof window.sendControlEvent === 'function') {
        window.sendControlEvent({
          type: 'set_mode',
          mode: currentMode
        });
        debug(`🎬 Switched to ${modeNames[currentMode]} mode`);
        
        // Update button tooltip and text
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

  // Subscribe to session signaling
  if (currentSession) {
    subscribeToSessionSignaling(currentSession.id);
  }
}

async function startSession(device) {
  currentDevice = device;
  
  // Create session tab if SessionManager is available
  if (window.SessionManager) {
    const existingSession = window.SessionManager.createSession(device.device_id, device.device_name);
    if (!existingSession) {
      return; // Max sessions reached or already exists
    }
  }
  
  try {
    // Get current session token for authorization
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
      // Fallback: Clean up any old pending sessions for this device
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
    
    // Call session-token Edge Function with explicit authorization
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
      
      // Try to get the actual error message from response
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

    currentSession = data;
    currentSession.device_id = device.device_id; // Store device_id for SessionManager
    window.currentSession = currentSession; // Expose globally for WebRTC module
    
    // Show preview UI elements
    const previewIdle = document.getElementById('previewIdle');
    const previewConnecting = document.getElementById('previewConnecting');
    const connectingDeviceName = document.getElementById('connectingDeviceName');
    const previewToolbar = document.getElementById('previewToolbar');
    const connectedDeviceName = document.getElementById('connectedDeviceName');
    
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

    // Subscribe to signaling FIRST (before sending offer)
    subscribeToSessionSignaling(currentSession.session_id);
    
    // Small delay to ensure subscription is active
    await new Promise(resolve => setTimeout(resolve, 200));
    
    // Initialize WebRTC (sends offer)
    await initWebRTC(currentSession);

  } catch (error) {
    console.error('Failed to start session:', error);
    // Remove session tab on error
    if (window.SessionManager) {
      window.SessionManager.closeSession(device.device_id);
    }
    showToast('Kunne ikke starte session: ' + error.message, 'error');
  }
}

async function endSession() {
  if (!currentSession) return;

  const sessionId = currentSession.session_id;
  const deviceId = currentSession.device_id;
  
  debug('🔌 Ending session:', sessionId);

  try {
    // Update session status in database
    await supabase
      .from('remote_sessions')
      .update({ status: 'ended', ended_at: new Date().toISOString() })
      .eq('id', sessionId);

    // Clean up WebRTC connection
    if (typeof window.cleanupWebRTC === 'function') {
      window.cleanupWebRTC();
    }
    
    // Stop signaling polling
    if (typeof window.stopPolling === 'function') {
      window.stopPolling();
    }

    // Close session tab if SessionManager is available
    if (window.SessionManager && deviceId) {
      window.SessionManager.closeSession(deviceId);
    }

    // Hide session UI elements
    const sessionSection = document.getElementById('sessionSection');
    if (sessionSection) sessionSection.style.display = 'none';
    
    const previewToolbar = document.getElementById('previewToolbar');
    if (previewToolbar) previewToolbar.style.display = 'none';
    
    const previewIdle = document.getElementById('previewIdle');
    if (previewIdle) previewIdle.style.display = 'flex';
    
    const previewConnecting = document.getElementById('previewConnecting');
    if (previewConnecting) previewConnecting.style.display = 'none';
    
    // Clear session state
    currentSession = null;
    window.currentSession = null;
    currentDevice = null;
    
    debug('✅ Session ended successfully');

  } catch (error) {
    console.error('❌ Failed to end session:', error);
  }
}

// Expose for disconnect button
window.disconnectFromDevice = endSession;

// Clean up session on page unload/refresh
window.addEventListener('beforeunload', (e) => {
  if (currentSession) {
    debug('Page unloading - cleaning up session:', currentSession.session_id);
    
    // Use navigator.sendBeacon for reliable session cleanup
    const payload = {
      status: 'ended',
      ended_at: new Date().toISOString()
    };
    
    // Try supabase update first (async, may not complete)
    supabase
      .from('remote_sessions')
      .update(payload)
      .eq('id', currentSession.session_id)
      .then(() => debug('Session ended successfully'))
      .catch(err => console.error('Session end error:', err));
    
    // Clean up input capture
    if (typeof cleanupInputCapture === 'function') {
      cleanupInputCapture();
    }
    
    // Close peer connection
    if (window.peerConnection) {
      window.peerConnection.close();
      window.peerConnection = null;
    }
    
    // Close data channel
    if (window.dataChannel) {
      window.dataChannel.close();
      window.dataChannel = null;
    }
  }
});

// Also clean up when page is hidden (e.g., switching tabs)
document.addEventListener('visibilitychange', () => {
  if (document.hidden && currentSession) {
    debug('Page hidden - keeping session alive but monitoring');
    // Don't end session immediately - user might come back
    // The 15-minute timeout will handle cleanup if they don't return
  }
});

// Export for other modules
window.startSession = startSession;
window.endSession = endSession;
window.currentSession = currentSession;
