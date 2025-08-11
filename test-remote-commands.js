/**
 * Test Remote Commands Script
 * 
 * This script sends test commands to the WebRTC agent to verify command handling functionality.
 * It connects to the signaling server and sends various command types to test the agent's
 * ability to handle mouse, keyboard, file transfer, and system commands.
 */

const WebSocket = require('ws');
const readline = require('readline');

// Configuration
const SIGNALING_SERVER = 'ws://localhost:8081';
const DEVICE_ID = 'device_660df7a9d7015dc8';
const ROOM_ID = `room_${DEVICE_ID}`;

// Create WebSocket connection
let socket;
let connected = false;

// Create readline interface for command input
const rl = readline.createInterface({
  input: process.stdin,
  output: process.stdout
});

// Connect to signaling server
function connect() {
  console.log(`🌐 Connecting to ${SIGNALING_SERVER}...`);
  
  socket = new WebSocket(SIGNALING_SERVER);
  
  socket.on('open', () => {
    console.log('✅ Connected to signaling server');
    connected = true;
    
    // Join the room
    socket.send(JSON.stringify({
      type: 'join-room',
      roomId: ROOM_ID
    }));
    
    console.log(`🔑 Joining room: ${ROOM_ID}`);
    showCommandMenu();
  });
  
  socket.on('message', (data) => {
    try {
      const message = JSON.parse(data);
      console.log(`📥 Received: ${message.type}`);
      
      if (message.type === 'command-ack') {
        console.log(`✅ Command acknowledged: ${message.status}`);
      }
    } catch (error) {
      console.error('❌ Error parsing message:', error.message);
    }
  });
  
  socket.on('close', () => {
    console.log('🔌 Disconnected from signaling server');
    connected = false;
  });
  
  socket.on('error', (error) => {
    console.error('❌ WebSocket error:', error.message);
  });
}

// Send a command to the agent
function sendCommand(command, data) {
  if (!connected) {
    console.error('❌ Not connected to signaling server');
    return;
  }
  
  const message = {
    type: 'message',
    roomId: ROOM_ID,
    content: JSON.stringify({
      type: command,
      data: data
    })
  };
  
  console.log(`📤 Sending command: ${command}`);
  socket.send(JSON.stringify(message));
}

// Show command menu
function showCommandMenu() {
  console.log('\n📋 Available Commands:');
  console.log('1. Mouse Click (left button at x=500, y=300)');
  console.log('2. Mouse Move (to x=800, y=400)');
  console.log('3. Keyboard Press (press "A" key)');
  console.log('4. File Upload (test.txt, 1MB)');
  console.log('5. File Download (document.pdf)');
  console.log('6. System Info');
  console.log('7. Start Screen Sharing');
  console.log('8. Stop Screen Sharing');
  console.log('9. Exit');
  
  promptCommand();
}

// Prompt for command
function promptCommand() {
  rl.question('\n🎮 Enter command number: ', (answer) => {
    switch (answer) {
      case '1':
        sendCommand('mouse', { type: 'click', button: 'left', x: 500, y: 300 });
        break;
        
      case '2':
        sendCommand('mouse', { type: 'move', x: 800, y: 400 });
        break;
        
      case '3':
        sendCommand('keyboard', { type: 'press', key: 'A' });
        break;
        
      case '4':
        sendCommand('file-transfer', { 
          operation: 'upload', 
          fileName: 'test.txt', 
          fileSize: 1024 * 1024 // 1MB
        });
        break;
        
      case '5':
        sendCommand('file-transfer', { 
          operation: 'download', 
          fileName: 'document.pdf'
        });
        break;
        
      case '6':
        sendCommand('system', { action: 'info' });
        break;
        
      case '7':
        sendCommand('start-screen', {});
        break;
        
      case '8':
        sendCommand('stop-screen', {});
        break;
        
      case '9':
        console.log('👋 Exiting...');
        socket.close();
        rl.close();
        process.exit(0);
        break;
        
      default:
        console.log('❌ Invalid command');
    }
    
    // Show menu again after command
    setTimeout(showCommandMenu, 1000);
  });
}

// Start the application
console.log('🚀 Starting WebRTC Remote Command Test');
connect();

// Handle exit
process.on('SIGINT', () => {
  console.log('\n👋 Exiting...');
  if (socket) socket.close();
  rl.close();
  process.exit(0);
});
