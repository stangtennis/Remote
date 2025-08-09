const WebSocket = require('ws');
const { createRequire } = require('module');

console.log('🚀 Starting WebRTC Remote Desktop Agent...');

// Try to load native modules for professional screen capture
let ProfessionalScreenCapture = null;
let ProfessionalInputControl = null;

try {
    const screenshotDesktop = createRequire(import.meta.url)('screenshot-desktop');
    const sharp = createRequire(import.meta.url)('sharp');
    const robotjs = createRequire(import.meta.url)('robotjs');
    
    ProfessionalScreenCapture = { screenshotDesktop, sharp };
    ProfessionalInputControl = { robotjs };
    console.log('✅ Professional native modules loaded successfully');
} catch (error) {
    console.log('⚠️ Professional modules not available, using compatibility mode');
    console.log('💡 Install: npm install screenshot-desktop sharp robotjs');
}

class WebRTCAgent {
    constructor() {
        this.ws = null;
        this.clientId = null;
        this.roomId = null;
        this.isHost = false;
        this.peerConnections = new Map(); // Multiple viewers
        this.localStream = null;
        this.screenCapture = null;
        this.inputControl = null;
        
        // WebRTC configuration
        this.rtcConfig = {
            iceServers: [
                { urls: 'stun:stun.l.google.com:19302' },
                { urls: 'stun:stun1.l.google.com:19302' }
            ]
        };
        
        this.initialize();
    }
    
    async initialize() {
        console.log('🔧 Initializing WebRTC Agent...');
        
        // Setup screen capture
        await this.setupScreenCapture();
        
        // Setup input control
        await this.setupInputControl();
        
        // Connect to signaling server
        this.connectToSignalingServer();
        
        console.log('✅ WebRTC Agent initialized successfully');
    }
    
    async setupScreenCapture() {
        if (ProfessionalScreenCapture) {
            try {
                console.log('🖥️ Setting up professional screen capture...');
                
                this.screenCapture = {
                    async captureScreen() {
                        const { screenshotDesktop, sharp } = ProfessionalScreenCapture;
                        
                        // Capture screenshot
                        const screenshot = await screenshotDesktop({ format: 'png' });
                        
                        // Convert to JPEG with compression for WebRTC
                        const jpegBuffer = await sharp(screenshot)
                            .jpeg({ quality: 80 })
                            .toBuffer();
                        
                        return jpegBuffer;
                    }
                };
                
                console.log('✅ Professional screen capture ready');
            } catch (error) {
                console.log('⚠️ Professional screen capture failed, using compatibility mode');
                this.setupCompatibilityScreenCapture();
            }
        } else {
            this.setupCompatibilityScreenCapture();
        }
    }
    
    setupCompatibilityScreenCapture() {
        console.log('🔄 Setting up compatibility screen capture...');
        
        this.screenCapture = {
            async captureScreen() {
                // Generate a placeholder image for compatibility mode
                const canvas = require('canvas');
                const { createCanvas } = canvas;
                
                const width = 1920;
                const height = 1080;
                const canvasElement = createCanvas(width, height);
                const ctx = canvasElement.getContext('2d');
                
                // Create a gradient background
                const gradient = ctx.createLinearGradient(0, 0, width, height);
                gradient.addColorStop(0, '#667eea');
                gradient.addColorStop(1, '#764ba2');
                ctx.fillStyle = gradient;
                ctx.fillRect(0, 0, width, height);
                
                // Add text
                ctx.fillStyle = 'white';
                ctx.font = '48px Arial';
                ctx.textAlign = 'center';
                ctx.fillText('WebRTC Agent - Compatibility Mode', width / 2, height / 2 - 50);
                ctx.font = '24px Arial';
                ctx.fillText(`${new Date().toLocaleTimeString()}`, width / 2, height / 2 + 50);
                ctx.fillText('Install native modules for real screen capture', width / 2, height / 2 + 100);
                
                return canvasElement.toBuffer('image/jpeg', { quality: 0.8 });
            }
        };
        
        console.log('✅ Compatibility screen capture ready');
    }
    
    async setupInputControl() {
        if (ProfessionalInputControl) {
            try {
                console.log('🖱️ Setting up professional input control...');
                
                const { robotjs } = ProfessionalInputControl;
                
                this.inputControl = {
                    mouseClick: (x, y, button = 'left') => {
                        robotjs.moveMouse(x, y);
                        robotjs.mouseClick(button);
                        console.log(`🖱️ Mouse click at (${x}, ${y}) with ${button} button`);
                    },
                    
                    mouseMove: (x, y) => {
                        robotjs.moveMouse(x, y);
                    },
                    
                    keyPress: (key) => {
                        robotjs.keyTap(key);
                        console.log(`⌨️ Key press: ${key}`);
                    },
                    
                    keyCombo: (keys) => {
                        robotjs.keyTap(keys[keys.length - 1], keys.slice(0, -1));
                        console.log(`⌨️ Key combo: ${keys.join('+')}`);
                    }
                };
                
                console.log('✅ Professional input control ready');
            } catch (error) {
                console.log('⚠️ Professional input control failed, using mock mode');
                this.setupMockInputControl();
            }
        } else {
            this.setupMockInputControl();
        }
    }
    
    setupMockInputControl() {
        this.inputControl = {
            mouseClick: (x, y, button = 'left') => {
                console.log(`🖱️ [MOCK] Mouse click at (${x}, ${y}) with ${button} button`);
            },
            
            mouseMove: (x, y) => {
                console.log(`🖱️ [MOCK] Mouse move to (${x}, ${y})`);
            },
            
            keyPress: (key) => {
                console.log(`⌨️ [MOCK] Key press: ${key}`);
            },
            
            keyCombo: (keys) => {
                console.log(`⌨️ [MOCK] Key combo: ${keys.join('+')}`);
            }
        };
        
        console.log('✅ Mock input control ready');
    }
    
    connectToSignalingServer() {
        const serverUrl = process.env.SIGNALING_SERVER || 'ws://localhost:8080';
        console.log(`🔗 Connecting to signaling server: ${serverUrl}`);
        
        this.ws = new WebSocket(serverUrl);
        
        this.ws.on('open', () => {
            console.log('✅ Connected to signaling server');
        });
        
        this.ws.on('message', (message) => {
            try {
                const data = JSON.parse(message);
                this.handleSignalingMessage(data);
            } catch (error) {
                console.error('❌ Invalid signaling message:', error.message);
            }
        });
        
        this.ws.on('close', () => {
            console.log('🔌 Disconnected from signaling server');
            // Attempt to reconnect after 5 seconds
            setTimeout(() => {
                console.log('🔄 Attempting to reconnect...');
                this.connectToSignalingServer();
            }, 5000);
        });
        
        this.ws.on('error', (error) => {
            console.error('❌ WebSocket error:', error.message);
        });
    }
    
    handleSignalingMessage(data) {
        switch (data.type) {
            case 'client-id':
                this.clientId = data.clientId;
                console.log(`🆔 Received client ID: ${this.clientId}`);
                this.createRoom();
                break;
                
            case 'room-created':
                this.roomId = data.roomId;
                this.isHost = true;
                console.log(`🏠 Room created: ${this.roomId}`);
                console.log(`🌐 Share this room ID for remote access: ${this.roomId}`);
                break;
                
            case 'viewer-joined':
                console.log(`👁️ Viewer joined: ${data.viewerId}`);
                this.handleNewViewer(data.viewerId);
                break;
                
            case 'offer':
                this.handleOffer(data.from, data.offer);
                break;
                
            case 'answer':
                this.handleAnswer(data.from, data.answer);
                break;
                
            case 'ice-candidate':
                this.handleIceCandidate(data.from, data.candidate);
                break;
                
            case 'input-command':
                this.handleInputCommand(data.command);
                break;
                
            default:
                console.log(`⚠️ Unknown signaling message: ${data.type}`);
        }
    }
    
    createRoom() {
        if (this.ws && this.ws.readyState === WebSocket.OPEN) {
            this.ws.send(JSON.stringify({
                type: 'create-room'
            }));
        }
    }
    
    async handleNewViewer(viewerId) {
        try {
            // Create new peer connection for this viewer
            const peerConnection = new RTCPeerConnection(this.rtcConfig);
            this.peerConnections.set(viewerId, peerConnection);
            
            // Add screen capture stream
            if (!this.localStream) {
                await this.startScreenSharing();
            }
            
            if (this.localStream) {
                this.localStream.getTracks().forEach(track => {
                    peerConnection.addTrack(track, this.localStream);
                });
            }
            
            // Setup data channel for input commands
            const dataChannel = peerConnection.createDataChannel('input', {
                ordered: true
            });
            
            dataChannel.onopen = () => {
                console.log(`📡 Data channel opened for viewer ${viewerId}`);
            };
            
            dataChannel.onmessage = (event) => {
                try {
                    const command = JSON.parse(event.data);
                    this.handleInputCommand(command);
                } catch (error) {
                    console.error('❌ Invalid input command:', error.message);
                }
            };
            
            // Handle ICE candidates
            peerConnection.onicecandidate = (event) => {
                if (event.candidate && this.ws) {
                    this.ws.send(JSON.stringify({
                        type: 'ice-candidate',
                        candidate: event.candidate,
                        target: viewerId
                    }));
                }
            };
            
            // Create and send offer
            const offer = await peerConnection.createOffer();
            await peerConnection.setLocalDescription(offer);
            
            if (this.ws) {
                this.ws.send(JSON.stringify({
                    type: 'offer',
                    offer: offer,
                    target: viewerId
                }));
            }
            
            console.log(`📤 Sent offer to viewer ${viewerId}`);
            
        } catch (error) {
            console.error(`❌ Failed to handle new viewer ${viewerId}:`, error.message);
        }
    }
    
    async handleOffer(from, offer) {
        // This would be used if agent acts as viewer (not implemented in this version)
        console.log(`📥 Received offer from ${from} (not implemented)`);
    }
    
    async handleAnswer(from, answer) {
        try {
            const peerConnection = this.peerConnections.get(from);
            if (peerConnection) {
                await peerConnection.setRemoteDescription(answer);
                console.log(`📥 Set remote description for ${from}`);
            }
        } catch (error) {
            console.error(`❌ Failed to handle answer from ${from}:`, error.message);
        }
    }
    
    async handleIceCandidate(from, candidate) {
        try {
            const peerConnection = this.peerConnections.get(from);
            if (peerConnection) {
                await peerConnection.addIceCandidate(candidate);
                console.log(`🧊 Added ICE candidate from ${from}`);
            }
        } catch (error) {
            console.error(`❌ Failed to handle ICE candidate from ${from}:`, error.message);
        }
    }
    
    handleInputCommand(command) {
        if (!this.inputControl) {
            console.log('⚠️ Input control not available');
            return;
        }
        
        switch (command.type) {
            case 'mouse-click':
                this.inputControl.mouseClick(command.x, command.y, command.button);
                break;
                
            case 'mouse-move':
                this.inputControl.mouseMove(command.x, command.y);
                break;
                
            case 'key-press':
                this.inputControl.keyPress(command.key);
                break;
                
            case 'key-combo':
                this.inputControl.keyCombo(command.keys);
                break;
                
            default:
                console.log(`⚠️ Unknown input command: ${command.type}`);
        }
    }
    
    async startScreenSharing() {
        try {
            console.log('🖥️ Starting screen sharing...');
            
            // For Node.js, we need to create a MediaStream from our screen capture
            // This is a simplified version - in practice, you'd need a more complex setup
            // to convert screen captures to WebRTC MediaStream
            
            console.log('⚠️ Note: Full WebRTC screen sharing from Node.js requires additional setup');
            console.log('💡 Consider using Electron or browser-based agent for full WebRTC support');
            
            // For now, we'll simulate having a stream
            this.localStream = {
                getTracks: () => [],
                addTrack: () => {},
                removeTrack: () => {}
            };
            
        } catch (error) {
            console.error('❌ Failed to start screen sharing:', error.message);
        }
    }
}

// Start the agent
const agent = new WebRTCAgent();

// Graceful shutdown
process.on('SIGINT', () => {
    console.log('\n🛑 Shutting down WebRTC Agent...');
    
    if (agent.ws) {
        agent.ws.close();
    }
    
    // Close all peer connections
    agent.peerConnections.forEach(pc => pc.close());
    
    console.log('✅ Agent shut down gracefully');
    process.exit(0);
});

// Error handling
process.on('uncaughtException', (error) => {
    console.error('❌ Uncaught Exception:', error);
    process.exit(1);
});

process.on('unhandledRejection', (reason, promise) => {
    console.error('❌ Unhandled Rejection at:', promise, 'reason:', reason);
});
