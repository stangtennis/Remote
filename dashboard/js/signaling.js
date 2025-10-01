// Signaling Module
// Handles WebRTC signaling via Supabase Realtime

let signalingChannel = null;

async function sendSignal(payload) {
  try {
    // Insert signaling message into database
    const { error } = await supabase
      .from('session_signaling')
      .insert({
        session_id: payload.session_id,
        from_side: payload.from,
        msg_type: payload.type,
        payload: {
          sdp: payload.sdp,
          candidate: payload.candidate,
          ts: new Date().toISOString()
        }
      });

    if (error) throw error;

    console.log('Signal sent:', payload.type);

  } catch (error) {
    console.error('Failed to send signal:', error);
    throw error;
  }
}

function subscribeToSessionSignaling(sessionId) {
  if (signalingChannel) {
    supabase.removeChannel(signalingChannel);
  }

  // Subscribe to signaling messages for this session
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
        console.log('Signal received:', payload);
        await handleSignal(payload.new);
      }
    )
    .subscribe();

  console.log('Subscribed to signaling for session:', sessionId);
}

async function handleSignal(signal) {
  // Ignore our own signals
  if (signal.from_side === 'dashboard') return;

  const peerConnection = window.peerConnection;
  if (!peerConnection) {
    console.warn('No peer connection available');
    return;
  }

  try {
    switch (signal.msg_type) {
      case 'answer':
        // Agent sent answer to our offer
        const answer = new RTCSessionDescription({
          type: 'answer',
          sdp: signal.payload.sdp
        });
        await peerConnection.setRemoteDescription(answer);
        console.log('Remote description set (answer)');
        break;

      case 'ice':
        // Agent sent ICE candidate
        if (signal.payload.candidate) {
          await peerConnection.addIceCandidate(
            new RTCIceCandidate(signal.payload.candidate)
          );
          console.log('ICE candidate added');
        }
        break;

      case 'offer':
        // Agent sent offer (reconnection scenario)
        const offer = new RTCSessionDescription({
          type: 'offer',
          sdp: signal.payload.sdp
        });
        await peerConnection.setRemoteDescription(offer);
        
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
