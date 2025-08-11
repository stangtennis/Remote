// Simple WebRTC Test Agent
const WebSocket = require('ws');
const readline = require('readline');
const os = require('os');

// Configuration
const SIGNALING_SERVER_URL = 'ws://localhost:8081';
const DEVICE_ID = 'device_660df7a9d7015dc8'; // Match the first mock device ID
const DEVICE_NAME = 'Test Windows PC';

// Create console interface for user input
const rl = readline.createInterface({
  input: process.stdin,
  output: process.stdout
});

console.log('🚀 Starting WebRTC Test Agent...');
console.log(`📱 Device ID: ${DEVICE_ID}`);
console.log(`💻 Device Name: ${DEVICE_NAME}`);
console.log(`🌐 Connecting to: ${SIGNALING_SERVER_URL}`);

// Connect to signaling server
let socket;
let heartbeatInterval;
let roomId = `room_${DEVICE_ID}`;

function connectToSignalingServer() {
  try {
    socket = new WebSocket(SIGNALING_SERVER_URL);
    
    socket.on('open', () => {
      console.log('✅ Connected to signaling server');
      sendHeartbeat();
      heartbeatInterval = setInterval(sendHeartbeat, 30000);
      
      // Create a room on the signaling server
      const createRoomMessage = {
        type: 'create-room',
        roomId: roomId,
        deviceId: DEVICE_ID,
        deviceName: DEVICE_NAME,
        osType: process.platform
      };
      
      socket.send(JSON.stringify(createRoomMessage));
      console.log(`📝 Creating room ${roomId} as host`);
    });
    
    socket.on('message', (data) => {
      try {
        const message = JSON.parse(data);
        console.log(`📥 Received: ${message.type}`);
        
        switch (message.type) {
          case 'client-id':
            // Store client ID if needed
            break;
            
          case 'room-created':
            roomId = message.roomId;
            console.log(`🔑 Joined room: ${roomId}`);
            // Auto-start screen sharing simulation when room is created
            startScreenSharingSimulation();
            break;
            
          case 'viewer-connected':
            console.log('👀 Viewer connected, starting screen sharing simulation');
            startScreenSharingSimulation();
            break;
            
          case 'offer':
            console.log('📞 Received connection offer');
            // Handle the WebRTC offer properly with a valid SDP answer
            const offerSdp = message.sdp;
            console.log(`📞 Processing offer with SDP: ${offerSdp ? offerSdp.substring(0, 30) + '...' : 'missing'}`);
            
            // Create a proper SDP answer that follows WebRTC protocol
            const answerSdp = createMockSdpAnswer();
            
            // Send the answer with a slight delay to simulate processing time
            setTimeout(() => {
              console.log('📞 Sending SDP answer');
              socket.send(JSON.stringify({
                type: 'answer',
                roomId: roomId,
                sdp: answerSdp
              }));
              
              // Send some ICE candidates after the answer
              setTimeout(() => sendMockIceCandidates(), 500);
            }, 1000);
            break;
            
          case 'ice-candidate':
            console.log('❄️ Received ICE candidate');
            // In a real agent, we would add this ICE candidate
            break;
            
          case 'command':
            console.log('🎮 Received remote command:', JSON.stringify(message));
            try {
              // Handle both formats: data as string or direct object
              if (typeof message.data === 'string') {
                console.log('📦 Command data is string, parsing:', message.data);
                const commandData = JSON.parse(message.data);
                handleRemoteCommand(commandData);
              } else {
                console.log('📦 Command data is object:', JSON.stringify(message.data));
                handleRemoteCommand(message);
              }
              
              // Send acknowledgment back to dashboard
              socket.send(JSON.stringify({
                type: 'command-ack',
                roomId: roomId,
                status: 'success',
                timestamp: Date.now()
              }));
            } catch (error) {
              console.error('❌ Error processing command:', error.message, error.stack);
              
              // Send error back to dashboard
              socket.send(JSON.stringify({
                type: 'command-ack',
                roomId: roomId,
                status: 'error',
                error: error.message,
                timestamp: Date.now()
              }));
            }
            break;
            
          case 'disconnect':
            console.log('🔌 Viewer disconnected');
            stopScreenSharingSimulation();
            break;
        }
      } catch (error) {
        console.error('❌ Error processing message:', error);
      }
    });
    
    socket.on('error', (error) => {
      console.error('❌ WebSocket error:', error);
    });
    
    socket.on('close', () => {
      console.log('🔌 Disconnected from signaling server');
      clearInterval(heartbeatInterval);
      
      // Try to reconnect after a delay
      setTimeout(() => {
        console.log('🔄 Attempting to reconnect...');
        connectToSignalingServer();
      }, 5000);
    });
  } catch (error) {
    console.error('❌ Failed to connect to signaling server:', error);
    
    // Try to reconnect after a delay
    setTimeout(() => {
      console.log('🔄 Attempting to reconnect...');
      connectToSignalingServer();
    }, 5000);
  }
}

// We don't need a heartbeat for the WebRTC signaling server
function sendHeartbeat() {
  // Just log that we're still connected
  console.log('💓 Agent still connected');
}

function getLocalIpAddress() {
  const interfaces = os.networkInterfaces();
  for (const name of Object.keys(interfaces)) {
    for (const iface of interfaces[name]) {
      if (iface.family === 'IPv4' && !iface.internal) {
        return iface.address;
      }
    }
  }
  return '127.0.0.1';
}

// Simulate screen sharing by sending frame updates
let screenSharingInterval;
let frameCount = 0;

function startScreenSharingSimulation() {
  console.log('🖼️ Starting screen frame simulation');
  
  screenSharingInterval = setInterval(() => {
    if (socket && socket.readyState === WebSocket.OPEN && roomId) {
      frameCount++;
      
      // Log locally that we're sending frames, but don't actually send them
      // through the signaling server as it doesn't support custom message types
      // In a real implementation, this would be sent through the WebRTC data channel
      console.log(`📤 Simulating screen frame #${frameCount}`);
      
      if (frameCount % 10 === 0) {
        console.log(`🖼️ Sent ${frameCount} frames`);
        
        // Send a heartbeat message to keep the connection alive
        // using a supported message type
        socket.send(JSON.stringify({
          type: 'ice-candidate', // Using a supported message type
          roomId: roomId,
          candidate: null, // Null candidate is valid and indicates end of candidates
          deviceId: DEVICE_ID,
          frameCount: frameCount // Including frame count as metadata
        }));
      }
    }
  }, 1000); // 1 frame per second for testing
}

function stopScreenSharingSimulation() {
  if (screenSharingInterval) {
    clearInterval(screenSharingInterval);
    screenSharingInterval = null;
    console.log('🖼️ Stopped screen frame simulation');
  }
}

// Handle user commands
rl.on('line', (input) => {
  const command = input.trim().toLowerCase();
  
  switch (command) {
    case 'exit':
    case 'quit':
      console.log('👋 Shutting down agent...');
      if (socket) socket.close();
      clearInterval(heartbeatInterval);
      clearInterval(screenSharingInterval);
      process.exit(0);
      break;
      
    case 'status':
      console.log(`📊 Status: ${socket ? (socket.readyState === WebSocket.OPEN ? 'Connected' : 'Disconnected') : 'Not initialized'}`);
      console.log(`🔑 Room ID: ${roomId || 'None'}`);
      break;
      
    case 'help':
      console.log('📚 Available commands:');
      console.log('  status - Show connection status');
      console.log('  start - Start screen sharing simulation');
      console.log('  stop - Stop screen sharing simulation');
      console.log('  help - Show this help message');
      console.log('  exit/quit - Shut down the agent');
      break;
      
    case 'start':
      startScreenSharingSimulation();
      break;
      
    case 'stop':
      stopScreenSharingSimulation();
      break;
      
    default:
      console.log('❓ Unknown command. Type "help" for available commands.');
  }
});

// Start the connection
connectToSignalingServer();

// Handle process termination
process.on('SIGINT', () => {
  console.log('👋 Shutting down agent...');
  if (socket) socket.close();
  clearInterval(heartbeatInterval);
  clearInterval(screenSharingInterval);
  process.exit(0);
});

// Handle remote commands from the dashboard
function handleRemoteCommand(message) {
  console.log(`💬 Received command message: ${JSON.stringify(message)}`);
  
  // Handle different message formats
  let command, data;
  
  if (message.command && message.data) {
    // Format: { command: 'type', data: {...} }
    console.log(`💡 Detected format 1: command=${message.command}`);
    command = message.command;
    data = message.data;
  } else if (message.type === 'command' && message.data) {
    // Format: { type: 'command', data: { command: 'type', ... } }
    console.log(`💡 Detected format 2: type=command with data`);
    try {
      const parsedData = typeof message.data === 'string' ? JSON.parse(message.data) : message.data;
      console.log(`💡 Parsed data:`, parsedData);
      command = parsedData.command || parsedData.type;
      data = parsedData;
    } catch (error) {
      console.error(`❌ Error parsing command data: ${error.message}`);
      return;
    }
  } else if (message.type === 'command') {
    // Format: { type: 'command', command: 'type', ... }
    console.log(`💡 Detected format 3: type=command with command=${message.command}`);
    command = message.command;
    data = message;
  } else {
    // Unknown format
    console.log(`❓ Unknown command format: ${JSON.stringify(message)}`);
    return;
  }
  
  console.log(`💬 Processing command: ${command}`);
  
  switch (command) {
    case 'mouse':
      console.log(`🖱️ Mouse event: ${data.type || 'move'} at position (${data.x || 0}, ${data.y || 0})`);
      break;
      
    case 'keyboard':
      console.log(`⌨️ Keyboard event: ${data.type || 'press'} key=${data.key || 'unknown'}`);
      break;
      
    case 'file-transfer':
      console.log(`💾 File transfer request: ${data.operation || 'unknown'} - ${data.fileName || 'unnamed'}`);
      simulateFileTransfer(data);
      break;
      
    case 'system':
      console.log(`💻 System command: ${data.action || 'unknown'}`);
      handleSystemCommand(data);
      break;
      
    case 'start-screen':
      console.log(`💻 Starting screen sharing`);
      startScreenSharingSimulation();
      break;
      
    case 'stop-screen':
      console.log(`💻 Stopping screen sharing`);
      stopScreenSharingSimulation();
      break;
      
    default:
      console.log(`❓ Unknown command type: ${command}`);
  }
}

// Handle system commands
function handleSystemCommand(data) {
  const action = data.action || 'unknown';
  
  switch (action) {
    case 'restart':
      console.log('🔄 Simulating system restart');
      break;
      
    case 'shutdown':
      console.log('🔴 Simulating system shutdown');
      break;
      
    case 'sleep':
      console.log('💤 Simulating system sleep');
      break;
      
    case 'info':
      console.log('📈 Simulating system info request');
      const systemInfo = {
        os: 'Windows 11',
        hostname: 'TEST-PC',
        cpus: 8,
        memory: {
          total: 16 * 1024 * 1024 * 1024, // 16GB
          free: 8 * 1024 * 1024 * 1024,   // 8GB
        },
        network: {
          interfaces: [
            { name: 'Ethernet', ip: '192.168.1.100', mac: '00:11:22:33:44:55' },
            { name: 'Wi-Fi', ip: '192.168.1.101', mac: '66:77:88:99:AA:BB' }
          ]
        }
      };
      console.log(`📈 System info: ${JSON.stringify(systemInfo)}`);
      break;
      
    default:
      console.log(`❓ Unknown system action: ${action}`);
  }
}

// Simulate file transfer operations
function simulateFileTransfer(data) {
  const { operation, fileName, fileSize } = data;
  
  if (operation === 'upload') {
    console.log(`📥 Simulating file upload: ${fileName} (${formatFileSize(fileSize)})`);
    
    // Simulate progress updates
    let progress = 0;
    const interval = setInterval(() => {
      progress += 10;
      console.log(`📥 Upload progress: ${progress}% - ${fileName}`);
      
      if (progress >= 100) {
        console.log(`✅ Upload complete: ${fileName}`);
        clearInterval(interval);
      }
    }, 500);
  } else if (operation === 'download') {
    console.log(`📤 Simulating file download: ${fileName}`);
    
    // Simulate progress updates
    let progress = 0;
    const interval = setInterval(() => {
      progress += 10;
      console.log(`📤 Download progress: ${progress}% - ${fileName}`);
      
      if (progress >= 100) {
        console.log(`✅ Download complete: ${fileName}`);
        clearInterval(interval);
      }
    }, 500);
  }
}

// Format file size in human-readable format
function formatFileSize(bytes) {
  if (bytes < 1024) return bytes + ' B';
  else if (bytes < 1048576) return (bytes / 1024).toFixed(2) + ' KB';
  else if (bytes < 1073741824) return (bytes / 1048576).toFixed(2) + ' MB';
  else return (bytes / 1073741824).toFixed(2) + ' GB';
}

// Create a mock SDP answer that follows WebRTC protocol format
function createMockSdpAnswer() {
  return `v=0
o=- 12345678 2 IN IP4 127.0.0.1
s=-
t=0 0
a=group:BUNDLE 0
a=msid-semantic: WMS stream_id
m=video 9 UDP/TLS/RTP/SAVPF 96 97 98 99 100 101 102
c=IN IP4 0.0.0.0
a=rtcp:9 IN IP4 0.0.0.0
a=ice-ufrag:mock
a=ice-pwd:mockpassword
a=ice-options:trickle
a=fingerprint:sha-256 AA:BB:CC:DD:EE:FF:00:11:22:33:44:55:66:77:88:99:AA:BB:CC:DD:EE:FF:00:11:22:33:44:55:66:77:88:99
a=setup:active
a=mid:0
a=extmap:1 urn:ietf:params:rtp-hdrext:toffset
a=extmap:2 http://www.webrtc.org/experiments/rtp-hdrext/abs-send-time
a=sendrecv
a=rtcp-mux
a=rtcp-rsize
a=rtpmap:96 VP8/90000
a=rtcp-fb:96 goog-remb
a=rtcp-fb:96 transport-cc
a=rtcp-fb:96 ccm fir
a=rtcp-fb:96 nack
a=rtcp-fb:96 nack pli
a=rtpmap:97 rtx/90000
a=fmtp:97 apt=96
a=rtpmap:98 VP9/90000
a=rtcp-fb:98 goog-remb
a=rtcp-fb:98 transport-cc
a=rtcp-fb:98 ccm fir
a=rtcp-fb:98 nack
a=rtcp-fb:98 nack pli
a=rtpmap:99 rtx/90000
a=fmtp:99 apt=98
a=rtpmap:100 H264/90000
a=rtcp-fb:100 goog-remb
a=rtcp-fb:100 transport-cc
a=rtcp-fb:100 ccm fir
a=rtcp-fb:100 nack
a=rtcp-fb:100 nack pli
a=fmtp:100 level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f
a=rtpmap:101 rtx/90000
a=fmtp:101 apt=100
a=rtpmap:102 red/90000
a=rtpmap:103 rtx/90000
a=fmtp:103 apt=102
a=ssrc-group:FID 1001 1002
a=ssrc:1001 cname:mock-cname
a=ssrc:1001 msid:stream_id video_label
a=ssrc:1001 mslabel:stream_id
a=ssrc:1001 label:video_label
a=ssrc:1002 cname:mock-cname
a=ssrc:1002 msid:stream_id video_label
a=ssrc:1002 mslabel:stream_id
a=ssrc:1002 label:video_label`;
}

// Send mock ICE candidates
function sendMockIceCandidates() {
  if (!socket || socket.readyState !== WebSocket.OPEN || !roomId) return;
  
  // Send a few mock ICE candidates
  const candidates = [
    { candidate: 'candidate:1 1 UDP 2122252543 192.168.1.100 49152 typ host', sdpMid: '0', sdpMLineIndex: 0 },
    { candidate: 'candidate:2 1 UDP 1686052863 203.0.113.5 49153 typ srflx raddr 192.168.1.100 rport 49152', sdpMid: '0', sdpMLineIndex: 0 },
    { candidate: 'candidate:3 1 UDP 41885695 198.51.100.5 49154 typ relay raddr 203.0.113.5 rport 49153', sdpMid: '0', sdpMLineIndex: 0 },
    { candidate: '', sdpMid: '0', sdpMLineIndex: 0 } // End-of-candidates
  ];
  
  // Send candidates with slight delays to simulate gathering
  candidates.forEach((candidate, index) => {
    setTimeout(() => {
      console.log(`❄️ Sending ICE candidate ${index + 1}/${candidates.length}`);
      socket.send(JSON.stringify({
        type: 'ice-candidate',
        roomId: roomId,
        candidate: candidate.candidate,
        sdpMid: candidate.sdpMid,
        sdpMLineIndex: candidate.sdpMLineIndex
      }));
    }, index * 300);
  });
}

console.log('✅ Agent initialized and ready');
console.log('📚 Type "help" for available commands');
