/**
 * Direct Command Test Script
 * 
 * This script directly sends WebRTC commands to the test agent using the exact
 * message format expected by the agent's handleRemoteCommand function.
 */

const WebSocket = require('ws');

// Configuration
const SIGNALING_SERVER = 'ws://localhost:8081';
const DEVICE_ID = 'device_660df7a9d7015dc8';
const ROOM_ID = `room_${DEVICE_ID}`;

// Connect to signaling server
console.log(`🌐 Connecting to ${SIGNALING_SERVER}...`);
const socket = new WebSocket(SIGNALING_SERVER);

// Test commands to send (in sequence)
const testCommands = [
  // Test mouse click command - Format 1: { type: 'message', content: JSON string with command }
  {
    type: 'message',
    roomId: ROOM_ID,
    content: JSON.stringify({
      type: 'command',
      command: 'mouse',
      data: { type: 'click', button: 'left', x: 500, y: 300 }
    })
  },
  
  // Test keyboard press command - Format 2: { type: 'command', command: string, data: object }
  {
    type: 'command',
    roomId: ROOM_ID,
    command: 'keyboard',
    data: { type: 'press', key: 'A' }
  },
  
  // Test file transfer command - Format 3: { command: string, data: object }
  {
    command: 'file-transfer',
    roomId: ROOM_ID,
    data: { operation: 'upload', fileName: 'test.txt', fileSize: 1024 * 1024 }
  },
  
  // Test system command - Format 4: { type: 'command', data: JSON string with command info }
  {
    type: 'command',
    roomId: ROOM_ID,
    data: JSON.stringify({
      command: 'system',
      data: { action: 'info' }
    })
  }
];

// Handle WebSocket events
socket.on('open', () => {
  console.log('✅ Connected to signaling server');
  
  // Join the room
  socket.send(JSON.stringify({
    type: 'join-room',
    roomId: ROOM_ID
  }));
  
  console.log(`🔑 Joining room: ${ROOM_ID}`);
  
  // Wait for room join confirmation before sending commands
  setTimeout(() => {
    console.log('📤 Sending test commands...');
    
    // Send each test command with a delay between them
    testCommands.forEach((command, index) => {
      setTimeout(() => {
        // Log different formats appropriately
        if (command.type === 'message') {
          const content = JSON.parse(command.content);
          console.log(`📤 Sending command ${index + 1}/${testCommands.length}: ${content.command} (Format 1)`);
        } else if (command.type === 'command' && command.command) {
          console.log(`📤 Sending command ${index + 1}/${testCommands.length}: ${command.command} (Format 2)`);
        } else if (command.command && !command.type) {
          console.log(`📤 Sending command ${index + 1}/${testCommands.length}: ${command.command} (Format 3)`);
        } else if (command.type === 'command' && command.data && typeof command.data === 'string') {
          const data = JSON.parse(command.data);
          console.log(`📤 Sending command ${index + 1}/${testCommands.length}: ${data.command} (Format 4)`);
        }
        socket.send(JSON.stringify(command));
      }, index * 2000); // 2 second delay between commands
    });
    
    // Close connection after all commands are sent
    setTimeout(() => {
      console.log('👋 Test complete, closing connection');
      socket.close();
      process.exit(0);
    }, testCommands.length * 2000 + 1000);
  }, 2000);
});

socket.on('message', (data) => {
  try {
    const message = JSON.parse(data);
    console.log(`📥 Received: ${message.type}`);
  } catch (error) {
    console.error('❌ Error parsing message:', error.message);
  }
});

socket.on('close', () => {
  console.log('🔌 Disconnected from signaling server');
});

socket.on('error', (error) => {
  console.error('❌ WebSocket error:', error.message);
});

// Handle process termination
process.on('SIGINT', () => {
  console.log('\n👋 Exiting...');
  socket.close();
  process.exit(0);
});
