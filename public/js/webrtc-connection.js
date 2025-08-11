// WebRTC Connection Management Module for Remote Desktop
console.log('🌐 Loading WebRTC Connection Module...');

// WebRTC Configuration
const WEBRTC_CONFIG = {
    iceServers: [
        { urls: 'stun:stun.l.google.com:19302' },
        { urls: 'stun:stun1.l.google.com:19302' },
        { urls: 'stun:stun2.l.google.com:19302' }
    ],
    iceCandidatePoolSize: 10
};

// Signaling server configuration
const SIGNALING_SERVER_URL = 'ws://localhost:8081';

// Global WebRTC state
let peerConnection = null;
let signalingSocket = null;
let localStream = null;
let remoteStream = null;
let dataChannel = null;
let currentRoomId = null;
let connectionState = 'disconnected';

// Event handlers
let onConnectionStateChange = null;
let onRemoteStream = null;
let onDataChannelMessage = null;
let onError = null;

// Initialize WebRTC connection
async function initializeWebRTC(roomId, isHost = false) {
    try {
        console.log(`🔄 Initializing WebRTC connection for room: ${roomId}`);
        currentRoomId = roomId;
        
        // Create peer connection
        peerConnection = new RTCPeerConnection(WEBRTC_CONFIG);
        
        // Set up event handlers
        setupPeerConnectionHandlers();
        
        // Connect to signaling server
        await connectToSignalingServer();
        
        if (isHost) {
            // Host: Set up screen capture and create room
            await setupScreenCapture();
            await createRoom(roomId);
        } else {
            // Viewer: Join existing room
            await joinRoom(roomId);
        }
        
        console.log('✅ WebRTC connection initialized successfully');
        updateConnectionState('connecting');
        
    } catch (error) {
        console.error('❌ Failed to initialize WebRTC:', error);
        if (onError) onError(error);
        throw error;
    }
}

// Set up peer connection event handlers
function setupPeerConnectionHandlers() {
    console.log('🔧 Setting up peer connection handlers...');
    
    // Connection state changes
    peerConnection.onconnectionstatechange = () => {
        const state = peerConnection.connectionState;
        console.log(`📡 Connection state changed: ${state}`);
        updateConnectionState(state);
    };
    
    // ICE connection state changes
    peerConnection.oniceconnectionstatechange = () => {
        const state = peerConnection.iceConnectionState;
        console.log(`🧊 ICE connection state: ${state}`);
        
        if (state === 'failed' || state === 'disconnected') {
            console.log('🔄 Attempting ICE restart...');
            peerConnection.restartIce();
        }
    };
    
    // ICE candidates
    peerConnection.onicecandidate = (event) => {
        if (event.candidate) {
            console.log('🧊 Sending ICE candidate');
            sendSignalingMessage({
                type: 'ice-candidate',
                candidate: event.candidate,
                roomId: currentRoomId
            });
        }
    };
    
    // Remote stream
    peerConnection.ontrack = (event) => {
        console.log('📺 Received remote stream');
        remoteStream = event.streams[0];
        if (onRemoteStream) {
            onRemoteStream(remoteStream);
        }
    };
    
    // Data channel (for input commands)
    peerConnection.ondatachannel = (event) => {
        console.log('📡 Data channel received');
        const channel = event.channel;
        setupDataChannelHandlers(channel);
    };
}

// Connect to signaling server
function connectToSignalingServer() {
    return new Promise((resolve, reject) => {
        console.log('🔌 Connecting to signaling server...');
        
        signalingSocket = new WebSocket(SIGNALING_SERVER_URL);
        
        signalingSocket.onopen = () => {
            console.log('✅ Connected to signaling server');
            resolve();
        };
        
        signalingSocket.onerror = (error) => {
            console.error('❌ Signaling server connection error:', error);
            reject(error);
        };
        
        signalingSocket.onclose = () => {
            console.log('🔌 Signaling server connection closed');
            updateConnectionState('disconnected');
        };
        
        signalingSocket.onmessage = handleSignalingMessage;
    });
}

// Handle signaling messages
async function handleSignalingMessage(event) {
    try {
        const message = JSON.parse(event.data);
        console.log('📨 Received signaling message:', message.type);
        
        switch (message.type) {
            case 'room-created':
                console.log(`✅ Room created: ${message.roomId}`);
                break;
                
            case 'room-joined':
                console.log(`✅ Joined room: ${message.roomId}`);
                // Create and send offer
                await createAndSendOffer();
                break;
                
            case 'offer':
                console.log('📥 Received offer');
                await handleOffer(message.offer);
                break;
                
            case 'answer':
                console.log('📥 Received answer');
                await handleAnswer(message.answer);
                break;
                
            case 'ice-candidate':
                console.log('🧊 Received ICE candidate');
                await handleIceCandidate(message.candidate);
                break;
                
            case 'error':
                console.error('❌ Signaling error:', message.error);
                if (onError) onError(new Error(message.error));
                break;
                
            default:
                console.log('⚠️ Unknown signaling message type:', message.type);
        }
    } catch (error) {
        console.error('❌ Error handling signaling message:', error);
    }
}

// Set up screen capture (for host)
async function setupScreenCapture() {
    try {
        console.log('📺 Setting up screen capture...');
        
        localStream = await navigator.mediaDevices.getDisplayMedia({
            video: {
                width: { ideal: 1920 },
                height: { ideal: 1080 },
                frameRate: { ideal: 30, max: 60 }
            },
            audio: false
        });
        
        // Add stream to peer connection
        localStream.getTracks().forEach(track => {
            peerConnection.addTrack(track, localStream);
        });
        
        // Set up data channel for input commands
        dataChannel = peerConnection.createDataChannel('input', {
            ordered: true
        });
        setupDataChannelHandlers(dataChannel);
        
        console.log('✅ Screen capture set up successfully');
        
    } catch (error) {
        console.error('❌ Failed to set up screen capture:', error);
        throw error;
    }
}

// Set up data channel handlers
function setupDataChannelHandlers(channel) {
    console.log('📡 Setting up data channel handlers...');
    
    channel.onopen = () => {
        console.log('✅ Data channel opened');
    };
    
    channel.onclose = () => {
        console.log('📡 Data channel closed');
    };
    
    channel.onerror = (error) => {
        console.error('❌ Data channel error:', error);
    };
    
    channel.onmessage = (event) => {
        console.log('📨 Data channel message received');
        if (onDataChannelMessage) {
            try {
                const message = JSON.parse(event.data);
                onDataChannelMessage(message);
            } catch (error) {
                console.error('❌ Error parsing data channel message:', error);
            }
        }
    };
    
    // Store reference for sending messages
    if (!dataChannel) {
        dataChannel = channel;
    }
}

// Create room (host)
function createRoom(roomId) {
    console.log(`🏠 Creating room: ${roomId}`);
    sendSignalingMessage({
        type: 'create-room',
        roomId: roomId
    });
}

// Join room (viewer)
function joinRoom(roomId) {
    console.log(`🚪 Joining room: ${roomId}`);
    sendSignalingMessage({
        type: 'join-room',
        roomId: roomId
    });
}

// Create and send offer
async function createAndSendOffer() {
    try {
        console.log('📤 Creating and sending offer...');
        
        const offer = await peerConnection.createOffer({
            offerToReceiveVideo: true,
            offerToReceiveAudio: false
        });
        
        await peerConnection.setLocalDescription(offer);
        
        sendSignalingMessage({
            type: 'offer',
            offer: offer,
            roomId: currentRoomId
        });
        
        console.log('✅ Offer sent successfully');
        
    } catch (error) {
        console.error('❌ Failed to create/send offer:', error);
        throw error;
    }
}

// Handle received offer
async function handleOffer(offer) {
    try {
        console.log('📥 Handling received offer...');
        
        await peerConnection.setRemoteDescription(offer);
        
        const answer = await peerConnection.createAnswer();
        await peerConnection.setLocalDescription(answer);
        
        sendSignalingMessage({
            type: 'answer',
            answer: answer,
            roomId: currentRoomId
        });
        
        console.log('✅ Answer sent successfully');
        
    } catch (error) {
        console.error('❌ Failed to handle offer:', error);
        throw error;
    }
}

// Handle received answer
async function handleAnswer(answer) {
    try {
        console.log('📥 Handling received answer...');
        await peerConnection.setRemoteDescription(answer);
        console.log('✅ Answer processed successfully');
        
    } catch (error) {
        console.error('❌ Failed to handle answer:', error);
        throw error;
    }
}

// Handle ICE candidate
async function handleIceCandidate(candidate) {
    try {
        console.log('🧊 Adding ICE candidate...');
        await peerConnection.addIceCandidate(candidate);
        console.log('✅ ICE candidate added successfully');
        
    } catch (error) {
        console.error('❌ Failed to add ICE candidate:', error);
    }
}

// Send signaling message
function sendSignalingMessage(message) {
    if (signalingSocket && signalingSocket.readyState === WebSocket.OPEN) {
        signalingSocket.send(JSON.stringify(message));
    } else {
        console.error('❌ Signaling socket not connected');
    }
}

// Send input command via data channel
function sendInputCommand(command) {
    if (dataChannel && dataChannel.readyState === 'open') {
        console.log('📤 Sending input command:', command.type);
        dataChannel.send(JSON.stringify(command));
    } else {
        console.error('❌ Data channel not open');
    }
}

// Update connection state
function updateConnectionState(newState) {
    if (connectionState !== newState) {
        connectionState = newState;
        console.log(`📊 Connection state updated: ${newState}`);
        if (onConnectionStateChange) {
            onConnectionStateChange(newState);
        }
    }
}

// Disconnect and cleanup
function disconnect() {
    console.log('🔌 Disconnecting WebRTC connection...');
    
    // Stop local stream
    if (localStream) {
        localStream.getTracks().forEach(track => track.stop());
        localStream = null;
    }
    
    // Close data channel
    if (dataChannel) {
        dataChannel.close();
        dataChannel = null;
    }
    
    // Close peer connection
    if (peerConnection) {
        peerConnection.close();
        peerConnection = null;
    }
    
    // Close signaling socket
    if (signalingSocket) {
        signalingSocket.close();
        signalingSocket = null;
    }
    
    updateConnectionState('disconnected');
    console.log('✅ WebRTC connection disconnected');
}

// Get connection statistics
async function getConnectionStats() {
    if (!peerConnection) {
        return null;
    }
    
    try {
        const stats = await peerConnection.getStats();
        const statsReport = {};
        
        stats.forEach((report) => {
            if (report.type === 'inbound-rtp' && report.mediaType === 'video') {
                statsReport.video = {
                    bytesReceived: report.bytesReceived,
                    packetsReceived: report.packetsReceived,
                    packetsLost: report.packetsLost,
                    framesReceived: report.framesReceived,
                    frameWidth: report.frameWidth,
                    frameHeight: report.frameHeight
                };
            }
        });
        
        return statsReport;
    } catch (error) {
        console.error('❌ Failed to get connection stats:', error);
        return null;
    }
}

// Export functions for use in other modules
window.WebRTCConnection = {
    initializeWebRTC,
    disconnect,
    sendInputCommand,
    getConnectionStats,
    
    // Event handler setters
    setConnectionStateHandler: (handler) => { onConnectionStateChange = handler; },
    setRemoteStreamHandler: (handler) => { onRemoteStream = handler; },
    setDataChannelMessageHandler: (handler) => { onDataChannelMessage = handler; },
    setErrorHandler: (handler) => { onError = handler; },
    
    // State getters
    getConnectionState: () => connectionState,
    getCurrentRoomId: () => currentRoomId,
    isConnected: () => connectionState === 'connected'
};

console.log('✅ WebRTC Connection Module loaded successfully');
