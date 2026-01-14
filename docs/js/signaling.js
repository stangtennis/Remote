// Signaling Module
// Handles WebRTC signaling via Supabase Realtime

let signalingChannel = null;
let pollingInterval = null;
let processedSignalIds = new Set();
let pendingIceCandidates = []; // Buffer for ICE candidates received before remote description

async function sendSignal(payload) {
  try {
    console.log('📤 Attempting to send signal:', payload.type, 'for session:', payload.session_id);
    
    // Prepare payload based on type
    let signalPayload;
    if (payload.type === 'ice') {
      // For ICE candidates, send the candidate object directly
      signalPayload = payload.candidate;
    } else {
      // For offer/answer, send as-is with type and sdp
      signalPayload = {
        type: payload.type,
        sdp: payload.sdp
      };
    }
    
    // Insert signaling message into database
    const { data, error } = await supabase
      .from('session_signaling')
      .insert({
        session_id: payload.session_id,
        from_side: payload.from,
        msg_type: payload.type,
        payload: signalPayload
      })
      .select();

    if (error) {
      console.error('❌ Database error:', error);
      throw error;
    }

    console.log('✅ Signal sent successfully:', payload.type, data);

  } catch (error) {
    console.error('❌ Failed to send signal:', error);
    console.error('❌ Error details:', JSON.stringify(error, null, 2));
    throw error;
  }
}

function subscribeToSessionSignaling(sessionId) {
  if (signalingChannel) {
    supabase.removeChannel(signalingChannel);
  }

  // Clear previous polling
  if (pollingInterval) {
    clearInterval(pollingInterval);
  }
  processedSignalIds.clear();

  // Subscribe to signaling messages for this session (Realtime)
  signalingChannel = supabase
    .channel(`session:${sessionId}`)
    .on('postgres_changes',
      {
        event: 'INSERT',
        schema: 'public',
        table: 'session_signaling',
        filter: `session_id=eq.${sessionId}`
      },
      async (payload) => {
        console.log('✅ Realtime signal received:', payload.new.msg_type);
        await handleSignal(payload.new);
      }
    )
    .subscribe();

  console.log('Subscribed to signaling for session:', sessionId);

  // Start polling as fallback (in case Realtime is slow/broken)
  startPollingForSignals(sessionId);
}

async function startPollingForSignals(sessionId) {
  console.log('🔄 Starting polling fallback for signals...');
  
  // Poll every 500ms
  pollingInterval = setInterval(async () => {
    try {
      const { data, error } = await supabase
        .from('session_signaling')
        .select('*')
        .eq('session_id', sessionId)
        .in('from_side', ['agent', 'system']) // Include system for kick signals
        .order('created_at', { ascending: true });

      if (error) {
        console.error('❌ Polling error:', error);
        return;
      }

      // Debug: log all fetched signals
      if (data && data.length > 0) {
        // Log ALL signal types received (for debugging)
        const signalTypes = data.map(s => `${s.msg_type}(${s.from_side})`).join(', ');
        console.log(`🔍 Polled ${data.length} signals: ${signalTypes}`);
        
        for (const signal of data) {
          // Skip already processed signals
          if (processedSignalIds.has(signal.id)) {
            continue;
          }
          processedSignalIds.add(signal.id);
          
          console.log('📥 Polled NEW signal:', signal.msg_type, 'from:', signal.from_side, 'id:', signal.id);
          await handleSignal(signal);
        }
      }
    } catch (err) {
      console.error('❌ Polling exception:', err);
    }
  }, 500);
}

function stopPolling() {
  if (pollingInterval) {
    clearInterval(pollingInterval);
    pollingInterval = null;
  }
  processedSignalIds.clear();
}

async function handleSignal(signal) {
  // Ignore our own signals
  if (signal.from_side === 'dashboard') return;

  // Handle kick signals - another controller took over
  if (signal.msg_type === 'kick') {
    console.log('🔴 KICKED - another controller took over this device');
    console.log('   Kick reason:', signal.payload?.reason);
    console.log('   New controller:', signal.payload?.new_controller_type);
    
    // Clean up WebRTC connection
    if (typeof cleanupWebRTC === 'function') {
      cleanupWebRTC();
    }
    
    // Stop polling
    stopPolling();
    
    // Show user message
    const statusEl = document.getElementById('sessionStatus');
    if (statusEl) {
      statusEl.textContent = 'Disconnected - another controller connected';
      statusEl.className = 'status-badge disconnected';
    }
    
    // Update preview UI
    const previewConnecting = document.getElementById('previewConnecting');
    const previewIdle = document.getElementById('previewIdle');
    if (previewConnecting) previewConnecting.style.display = 'none';
    if (previewIdle) previewIdle.style.display = 'flex';
    
    // Notify SessionManager if available
    if (window.SessionManager && window.currentSession) {
      window.SessionManager.closeSession(window.currentSession.device_id);
    }
    
    alert('Du blev afkoblet - en anden controller har overtaget forbindelsen.');
    return;
  }

  // Skip already processed signals (prevents duplicates from realtime + polling)
  if (processedSignalIds.has(signal.id)) {
    return;
  }
  processedSignalIds.add(signal.id);

  console.log('🔵 Processing signal:', signal.msg_type, 'from', signal.from_side);

  const peerConnection = window.peerConnection;
  if (!peerConnection) {
    console.warn('No peer connection available');
    return;
  }

  try {
    switch (signal.msg_type) {
      case 'answer':
        // Only set answer if we're in have-local-offer state
        if (peerConnection.signalingState !== 'have-local-offer') {
          console.log('⏭️ Skipping answer - already in state:', peerConnection.signalingState);
          return;
        }
        const answer = new RTCSessionDescription(signal.payload);
        await peerConnection.setRemoteDescription(answer);
        console.log('✅ Remote description set (answer)');
        
        // Process any buffered ICE candidates now that remote description is set
        if (pendingIceCandidates.length > 0) {
          console.log(`🔄 Processing ${pendingIceCandidates.length} buffered ICE candidates`);
          for (const candidate of pendingIceCandidates) {
            try {
              await peerConnection.addIceCandidate(
                new RTCIceCandidate({
                  candidate: candidate.candidate,
                  sdpMid: candidate.sdpMid,
                  sdpMLineIndex: candidate.sdpMLineIndex
                })
              );
              console.log('✅ Buffered ICE candidate added');
            } catch (err) {
              console.warn('⚠️ Failed to add buffered ICE candidate:', err);
            }
          }
          pendingIceCandidates = []; // Clear buffer
        }
        break;

      case 'ice':
        // Agent sent ICE candidate
        // Handle both formats:
        // Old: payload = { candidate: { candidate: "...", sdpMid, sdpMLineIndex } }
        // New: payload = { candidate: "...", sdpMid: "0", sdpMLineIndex: 0 }
        let iceCandidate;
        if (signal.payload.candidate && typeof signal.payload.candidate === 'object') {
          // Old nested format
          iceCandidate = signal.payload.candidate;
        } else {
          // New flat format
          iceCandidate = signal.payload;
        }
        
        if (iceCandidate && iceCandidate.candidate) {
          const candidateStr = typeof iceCandidate.candidate === 'string' 
            ? iceCandidate.candidate.substring(0, 50) + '...'
            : JSON.stringify(iceCandidate.candidate).substring(0, 50);
          console.log('📥 Received ICE candidate from agent:', candidateStr);
          
          // Check if remote description is set
          if (!peerConnection.remoteDescription) {
            console.log('⏸️ Buffering ICE candidate (remote description not set yet)');
            pendingIceCandidates.push(iceCandidate);
          } else {
            await peerConnection.addIceCandidate(
              new RTCIceCandidate({
                candidate: iceCandidate.candidate,
                sdpMid: iceCandidate.sdpMid,
                sdpMLineIndex: iceCandidate.sdpMLineIndex
              })
            );
            console.log('✅ ICE candidate added successfully');
          }
        }
        break;

      case 'offer':
        // Agent sent offer (reconnection scenario)
        // Payload has {type, sdp} structure
        const offer = new RTCSessionDescription(signal.payload);
        await peerConnection.setRemoteDescription(offer);
        console.log('✅ Remote description set from offer');
        
        // Process any buffered ICE candidates now that remote description is set
        if (pendingIceCandidates.length > 0) {
          console.log(`🔄 Processing ${pendingIceCandidates.length} buffered ICE candidates`);
          for (const candidate of pendingIceCandidates) {
            try {
              await peerConnection.addIceCandidate(
                new RTCIceCandidate({
                  candidate: candidate.candidate,
                  sdpMid: candidate.sdpMid,
                  sdpMLineIndex: candidate.sdpMLineIndex
                })
              );
              console.log('✅ Buffered ICE candidate added');
            } catch (err) {
              console.warn('⚠️ Failed to add buffered ICE candidate:', err);
            }
          }
          pendingIceCandidates = []; // Clear buffer
        }
        
        // Create and send answer
        const answerSdp = await peerConnection.createAnswer();
        await peerConnection.setLocalDescription(answerSdp);
        
        await sendSignal({
          session_id: window.currentSession.session_id,
          from: 'dashboard',
          type: 'answer',
          sdp: answerSdp.sdp
        });
        console.log('Answer sent in response to offer');
        break;

      default:
        console.warn('Unknown signal type:', signal.msg_type);
    }
  } catch (error) {
    console.error('Failed to handle signal:', error);
  }
}

// Export
window.sendSignal = sendSignal;
window.subscribeToSessionSignaling = subscribeToSessionSignaling;
window.stopPolling = stopPolling;
