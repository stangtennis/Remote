// Main Application Logic
// Coordinates between devices, sessions, and WebRTC

let currentSession = null;
let currentDevice = null;

// Initialize app on dashboard load
document.addEventListener('DOMContentLoaded', async () => {
  console.log('Dashboard initialized');
  
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
      alert('Agent download will be available once the Windows agent is built.');
      // TODO: Link to agents bucket in Supabase Storage
    });
  }

  // End session button
  const endSessionBtn = document.getElementById('endSessionBtn');
  if (endSessionBtn) {
    endSessionBtn.addEventListener('click', async () => {
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
}

function subscribeToRealtime() {
  // Subscribe to device changes
  supabase
    .channel('devices')
    .on('postgres_changes', 
      { event: '*', schema: 'public', table: 'remote_devices' },
      (payload) => {
        console.log('Device changed:', payload);
        loadDevices();
      }
    )
    .subscribe();

  // Subscribe to session signaling
  if (currentSession) {
    subscribeToSessionSignaling(currentSession.id);
  }
}

async function startSession(device) {
  currentDevice = device;
  
  try {
    // Clean up any old pending sessions for this device first
    console.log('🧹 Cleaning old pending sessions for device:', device.device_id);
    await supabase
      .from('remote_sessions')
      .update({ status: 'expired' })
      .eq('device_id', device.device_id)
      .in('status', ['pending', 'active']);
    
    // Get current session token for authorization
    const { data: { session } } = await supabase.auth.getSession();
    
    if (!session) {
      throw new Error('Not authenticated. Please log in again.');
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
    window.currentSession = data; // Expose globally for WebRTC module
    
    // Show session UI
    document.getElementById('sessionSection').style.display = 'block';
    document.getElementById('sessionDeviceName').textContent = device.device_name;
    document.getElementById('sessionPin').textContent = data.pin;
    document.getElementById('sessionStatus').textContent = 'Connecting...';
    document.getElementById('sessionStatus').className = 'status-badge pending';

    // Initialize WebRTC
    await initWebRTC(currentSession);
    
    // Subscribe to signaling for this session
    subscribeToSessionSignaling(currentSession.session_id);

  } catch (error) {
    console.error('Failed to start session:', error);
    alert('Failed to start session: ' + error.message);
  }
}

async function endSession() {
  if (!currentSession) return;

  try {
    // Update session status
    await supabase
      .from('remote_sessions')
      .update({ status: 'ended', ended_at: new Date().toISOString() })
      .eq('id', currentSession.session_id);

    // Clean up input capture
    if (typeof cleanupInputCapture === 'function') {
      cleanupInputCapture();
    }
    
    // Close WebRTC connection
    if (window.peerConnection) {
      window.peerConnection.close();
      window.peerConnection = null;
    }

    // Hide session UI
    document.getElementById('sessionSection').style.display = 'none';
    
    currentSession = null;
    currentDevice = null;

  } catch (error) {
    console.error('Failed to end session:', error);
  }
}

// Clean up session on page unload/refresh
window.addEventListener('beforeunload', (e) => {
  if (currentSession) {
    console.log('Page unloading - cleaning up session:', currentSession.session_id);
    
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
      .then(() => console.log('Session ended successfully'))
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
    console.log('Page hidden - keeping session alive but monitoring');
    // Don't end session immediately - user might come back
    // The 15-minute timeout will handle cleanup if they don't return
  }
});

// Export for other modules
window.startSession = startSession;
window.endSession = endSession;
window.currentSession = currentSession;
