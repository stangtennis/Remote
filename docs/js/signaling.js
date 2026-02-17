// Signaling Module
// Handles WebRTC signaling via Supabase Realtime
// All state is per-session via ctx (session object from SessionManager)

async function sendSignal(payload) {
  try {
    debug('📤 Attempting to send signal:', payload.type, 'for session:', payload.session_id);

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

    debug('✅ Signal sent successfully:', payload.type, data);

  } catch (error) {
    console.error('❌ Failed to send signal:', error);
    console.error('❌ Error details:', JSON.stringify(error, null, 2));
    throw error;
  }
}

function subscribeToSessionSignaling(sessionId, ctx) {
  // Clean up any previous channel on this ctx
  if (ctx.signalingChannel) {
    supabase.removeChannel(ctx.signalingChannel);
  }

  // Clear previous polling on this ctx
  if (ctx.pollingInterval) {
    clearInterval(ctx.pollingInterval);
  }
  ctx.processedSignalIds.clear();

  // Subscribe to signaling messages for this session (Realtime)
  ctx.signalingChannel = supabase
    .channel(`session:${sessionId}`)
    .on('postgres_changes',
      {
        event: 'INSERT',
        schema: 'public',
        table: 'session_signaling',
        filter: `session_id=eq.${sessionId}`
      },
      async (payload) => {
        debug('✅ Realtime signal received:', payload.new.msg_type);
        await handleSignal(payload.new, ctx);
      }
    )
    .subscribe();

  debug('Subscribed to signaling for session:', sessionId);

  // Start polling as fallback (in case Realtime is slow/broken)
  startPollingForSignals(sessionId, ctx);
}

async function startPollingForSignals(sessionId, ctx) {
  debug('🔄 Starting polling fallback for signals...');

  // Poll every 500ms
  ctx.pollingInterval = setInterval(async () => {
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
        debug(`🔍 Polled ${data.length} signals: ${signalTypes}`);

        for (const signal of data) {
          // Skip already processed signals
          if (ctx.processedSignalIds.has(signal.id)) {
            continue;
          }
          ctx.processedSignalIds.add(signal.id);

          debug('📥 Polled NEW signal:', signal.msg_type, 'from:', signal.from_side, 'id:', signal.id);
          await handleSignal(signal, ctx);
        }
      }
    } catch (err) {
      console.error('❌ Polling exception:', err);
    }
  }, 500);
}

// Stop polling for a specific session
function stopSessionPolling(ctx) {
  if (ctx.pollingInterval) {
    clearInterval(ctx.pollingInterval);
    ctx.pollingInterval = null;
  }
  ctx.processedSignalIds.clear();
}

// Stop all session polling (convenience for beforeunload etc.)
function stopPolling() {
  if (window.SessionManager) {
    for (const session of window.SessionManager.sessions.values()) {
      stopSessionPolling(session);
    }
  }
}

async function handleSignal(signal, ctx) {
  // Ignore our own signals
  if (signal.from_side === 'dashboard') return;

  // Handle kick signals - another controller took over
  if (signal.msg_type === 'kick') {
    debug('🔴 KICKED - another controller took over device:', ctx.id);
    debug('   Kick reason:', signal.payload?.reason);
    debug('   New controller:', signal.payload?.new_controller_type);

    showToast('Du blev afkoblet — en anden controller har overtaget forbindelsen.', 'warning', 6000);

    // End this specific session
    if (window.endSession) {
      window.endSession(ctx.id);
    }
    return;
  }

  // Skip already processed signals (prevents duplicates from realtime + polling)
  if (ctx.processedSignalIds.has(signal.id)) {
    return;
  }
  ctx.processedSignalIds.add(signal.id);

  debug('🔵 Processing signal:', signal.msg_type, 'from', signal.from_side, 'for device:', ctx.id);

  const peerConnection = ctx.peerConnection;
  if (!peerConnection) {
    console.warn('No peer connection available for session:', ctx.id);
    return;
  }

  try {
    switch (signal.msg_type) {
      case 'answer':
        // Only set answer if we're in have-local-offer state
        if (peerConnection.signalingState !== 'have-local-offer') {
          debug('⏭️ Skipping answer - already in state:', peerConnection.signalingState);
          return;
        }
        const answer = new RTCSessionDescription(signal.payload);
        await peerConnection.setRemoteDescription(answer);
        debug('✅ Remote description set (answer)');

        // Flush any buffered ICE candidates now that remote description is set
        if (ctx.pendingIceCandidates.length > 0) {
          debug(`📥 Flushing ${ctx.pendingIceCandidates.length} buffered ICE candidates`);
          for (const buffered of ctx.pendingIceCandidates) {
            await peerConnection.addIceCandidate(
              new RTCIceCandidate({
                candidate: buffered.candidate,
                sdpMid: buffered.sdpMid,
                sdpMLineIndex: buffered.sdpMLineIndex
              })
            );
          }
          ctx.pendingIceCandidates = [];
        }
        break;

      case 'ice':
        // Agent sent ICE candidate
        let iceCandidate;
        if (signal.payload.candidate && typeof signal.payload.candidate === 'object') {
          iceCandidate = signal.payload.candidate;
        } else {
          iceCandidate = signal.payload;
        }

        if (iceCandidate && iceCandidate.candidate) {
          const candidateStr = typeof iceCandidate.candidate === 'string'
            ? iceCandidate.candidate.substring(0, 50) + '...'
            : JSON.stringify(iceCandidate.candidate).substring(0, 50);
          debug('📥 Received ICE candidate from agent:', candidateStr);

          // Check if remote description is set
          if (!peerConnection.remoteDescription) {
            debug('⏸️ Buffering ICE candidate (remote description not set yet)');
            ctx.pendingIceCandidates.push(iceCandidate);
          } else {
            await peerConnection.addIceCandidate(
              new RTCIceCandidate({
                candidate: iceCandidate.candidate,
                sdpMid: iceCandidate.sdpMid,
                sdpMLineIndex: iceCandidate.sdpMLineIndex
              })
            );
            debug('✅ ICE candidate added successfully');
          }
        }
        break;

      case 'offer':
        // Agent sent offer (reconnection scenario)
        const offer = new RTCSessionDescription(signal.payload);
        await peerConnection.setRemoteDescription(offer);

        // Create and send answer
        const answerSdp = await peerConnection.createAnswer();
        await peerConnection.setLocalDescription(answerSdp);

        await sendSignal({
          session_id: ctx.sessionData.session_id,
          from: 'dashboard',
          type: 'answer',
          sdp: answerSdp.sdp
        });
        debug('Answer sent in response to offer');
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
window.stopSessionPolling = stopSessionPolling;
