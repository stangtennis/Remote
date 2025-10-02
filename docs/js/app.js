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
    // Call session-token Edge Function
    const { data, error } = await supabase.functions.invoke('session-token', {
      body: {
        device_id: device.device_id,
        use_pin: true
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

// Export for other modules
window.startSession = startSession;
window.endSession = endSession;
window.currentSession = currentSession;
