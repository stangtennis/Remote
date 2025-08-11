const { createClient } = require('@supabase/supabase-js');
const os = require('os');
const crypto = require('crypto');
const fs = require('fs');
const path = require('path');
const http = require('http');
const https = require('https');

// WebSocket support with graceful fallback
let WebSocket;
try {
    WebSocket = require('ws');
    console.log('✅ WebSocket module loaded successfully');
} catch (error) {
    console.log('⚠️ WebSocket module not available:', error.message);
}

// Native modules with graceful fallback
let screenshotDesktop, sharp, robotjs;
try {
    screenshotDesktop = require('screenshot-desktop');
    console.log('✅ screenshot-desktop loaded');
} catch (error) {
    console.log('⚠️ screenshot-desktop not available:', error.message);
}

try {
    sharp = require('sharp');
    console.log('✅ sharp loaded');
} catch (error) {
    console.log('⚠️ sharp not available:', error.message);
}

try {
    robotjs = require('robotjs');
    console.log('✅ robotjs loaded');
} catch (error) {
    console.log('⚠️ robotjs not available:', error.message);
}

class StandaloneAgent {
    constructor() {
        // Supabase configuration
        this.supabaseUrl = 'https://ptrtibzwokjcjjxvjpin.supabase.co';
        this.supabaseKey = 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJzdXBhYmFzZSIsInJlZiI6InB0cnRpYnp3b2tqY2pqeHZqcGluIiwicm9sZSI6ImFub24iLCJpYXQiOjE3NTQ0MzE1NzEsImV4cCI6MjA3MDAwNzU3MX0.DPNxkQul1-13tqJ89mqYJAx7NSJjabOP4q8c6KgOnWk';
        
        // Device identification
        this.deviceId = this.generateHardwareBasedDeviceId();
        this.deviceName = os.hostname();
        
        // WebSocket server configuration
        this.httpPort = 8080;
        this.httpsPort = 8443;
        this.connectedClients = new Set();
        
        // Intervals
        this.screenCaptureInterval = null;
        this.screenShareInterval = null;
        
        console.log(`🖥️ Device ID: ${this.deviceId}`);
        console.log(`🏷️ Device Name: ${this.deviceName}`);
    }

    generateHardwareBasedDeviceId() {
        try {
            // Get hardware identifiers
            const hostname = os.hostname();
            const platform = os.platform();
            const arch = os.arch();
            const cpus = os.cpus();
            const totalMemory = os.totalmem();
            
            // Create hardware fingerprint
            const hardwareString = `${hostname}-${platform}-${arch}-${cpus.length}-${totalMemory}`;
            
            // Generate consistent UUID from hardware
            const hash = crypto.createHash('sha256').update(hardwareString).digest('hex');
            
            // Format as UUID v4
            const uuid = [
                hash.substr(0, 8),
                hash.substr(8, 4),
                '4' + hash.substr(13, 3), // Version 4
                ((parseInt(hash.substr(16, 1), 16) & 0x3) | 0x8).toString(16) + hash.substr(17, 3), // Variant bits
                hash.substr(20, 12)
            ].join('-');
            
            return uuid;
        } catch (error) {
            console.error('❌ Hardware ID generation failed:', error.message);
            // Fallback to random UUID
            return crypto.randomUUID();
        }
    }

    async initialize() {
        try {
            console.log('🚀 Initializing Standalone Remote Desktop Agent v6.2.0 (WSS Edition)...');
            
            // Initialize Supabase client
            this.supabaseClient = createClient(this.supabaseUrl, this.supabaseKey);
            console.log('✅ Supabase client initialized');
            
            // Register device
            await this.registerDevice();
            
            // Setup realtime communication
            await this.setupRealtimeChannel();
            
            // Start WebSocket servers (HTTP and HTTPS)
            await this.startWebSocketServers();
            
            // Start heartbeat
            this.startHeartbeat();
            
            // Keep alive
            this.keepAlive();
            
            console.log('✅ Agent initialized successfully');
            console.log('🔗 Ready for remote connections');
            
        } catch (error) {
            console.error('❌ Initialization failed:', error.message);
            process.exit(1);
        }
    }

    async registerDevice() {
        try {
            // Use only columns that definitely exist in the database
            const deviceInfo = {
                id: this.deviceId,
                device_name: this.deviceName,
                device_type: 'desktop',
                operating_system: os.platform(),
                ip_address: this.getLocalIP(),
                status: 'online',
                last_seen: new Date().toISOString(),
                metadata: {
                    architecture: os.arch(),
                    cpu_count: os.cpus().length,
                    total_memory_gb: Math.round(os.totalmem() / (1024 * 1024 * 1024)),
                    agent_version: '6.2.0',
                    wss_enabled: true,
                    http_port: this.httpPort,
                    https_port: this.httpsPort
                }
            };

            const { data, error } = await this.supabaseClient
                .from('remote_devices')
                .upsert(deviceInfo, { onConflict: 'id' });

            if (error) {
                console.error('❌ Device registration failed:', error.message);
            } else {
                console.log('✅ Device registered successfully');
                console.log(`📱 Device ID: ${this.deviceId}`);
                console.log(`🏷️ Device Name: ${this.deviceName}`);
            }
        } catch (error) {
            console.error('❌ Registration error:', error.message);
        }
    }

    async setupRealtimeChannel() {
        try {
            this.realtimeChannel = this.supabaseClient
                .channel(`device-${this.deviceId}`)
                .on('broadcast', { event: 'remote-command' }, (payload) => {
                    this.handleRemoteCommand(payload.payload);
                })
                .subscribe();

            console.log('✅ Realtime channel established');
        } catch (error) {
            console.error('❌ Realtime setup failed:', error.message);
        }
    }

    async handleRemoteCommand(message) {
        console.log(`💬 Received command: ${JSON.stringify(message)}`);
        
        try {
            switch (message.type) {
                case 'start-screen':
                    this.startScreenCapture();
                    break;
                case 'stop-screen':
                    this.stopScreenCapture();
                    break;
                case 'mouse':
                    await this.handleMouseCommand(message.data);
                    break;
                case 'keyboard':
                    await this.handleKeyboardCommand(message.data);
                    break;
                default:
                    console.log(`❓ Unknown command: ${message.type}`);
            }
        } catch (error) {
            console.error('❌ Command handling error:', error.message);
        }
    }

    async handleMouseCommand(data) {
        console.log(`🖱️ Mouse: ${data.type} at (${data.x}, ${data.y})`);
        
        if (robotjs) {
            try {
                switch (data.type) {
                    case 'move':
                        robotjs.moveMouse(data.x, data.y);
                        break;
                    case 'click':
                        robotjs.mouseClick(data.button || 'left');
                        break;
                }
            } catch (error) {
                console.error('❌ Mouse control error:', error.message);
            }
        } else {
            // Fallback: Windows API via PowerShell
            await this.executeWindowsMouseCommand(data);
        }
    }

    async handleKeyboardCommand(data) {
        console.log(`⌨️ Keyboard: ${data.key}`);
        
        if (robotjs) {
            try {
                robotjs.keyTap(data.key);
            } catch (error) {
                console.error('❌ Keyboard control error:', error.message);
            }
        } else {
            // Fallback: Windows API via PowerShell
            await this.executeWindowsKeyboardCommand(data);
        }
    }

    async executeWindowsMouseCommand(data) {
        try {
            const { exec } = require('child_process');
            let psCommand = '';
            
            switch (data.type) {
                case 'move':
                    psCommand = `[System.Windows.Forms.Cursor]::Position = New-Object System.Drawing.Point(${data.x}, ${data.y})`;
                    break;
                case 'click':
                    psCommand = `Add-Type -AssemblyName System.Windows.Forms; [System.Windows.Forms.SendKeys]::SendWait("{CLICK}")`;
                    break;
            }
            
            if (psCommand) {
                exec(`powershell -Command "${psCommand}"`, (error) => {
                    if (error) console.error('❌ PowerShell mouse error:', error.message);
                });
            }
        } catch (error) {
            console.error('❌ Windows mouse command error:', error.message);
        }
    }

    async executeWindowsKeyboardCommand(data) {
        try {
            const { exec } = require('child_process');
            const psCommand = `Add-Type -AssemblyName System.Windows.Forms; [System.Windows.Forms.SendKeys]::SendWait("${data.key}")`;
            
            exec(`powershell -Command "${psCommand}"`, (error) => {
                if (error) console.error('❌ PowerShell keyboard error:', error.message);
            });
        } catch (error) {
            console.error('❌ Windows keyboard command error:', error.message);
        }
    }

    async takeScreenshot() {
        if (screenshotDesktop) {
            try {
                const screenshot = await screenshotDesktop();
                
                if (sharp) {
                    // Compress with sharp
                    return await sharp(screenshot)
                        .resize(1280, 720, { fit: 'inside' })
                        .jpeg({ quality: 60 })
                        .toBuffer();
                } else {
                    // Return raw screenshot
                    return screenshot;
                }
            } catch (error) {
                console.error('❌ Screenshot error:', error.message);
                return this.createMockScreenshot();
            }
        } else {
            return this.createMockScreenshot();
        }
    }

    createMockScreenshot() {
        // Create SVG mock screenshot
        const svg = `
            <svg width="800" height="600" xmlns="http://www.w3.org/2000/svg">
                <rect width="100%" height="100%" fill="#1a1a1a"/>
                <text x="50%" y="45%" text-anchor="middle" fill="#4CAF50" font-family="Arial" font-size="24">
                    ${this.deviceName}
                </text>
                <text x="50%" y="55%" text-anchor="middle" fill="#888" font-family="Arial" font-size="16">
                    WSS Remote Desktop Agent v6.2.0
                </text>
                <text x="50%" y="65%" text-anchor="middle" fill="#666" font-family="Arial" font-size="14">
                    ${new Date().toLocaleTimeString()}
                </text>
            </svg>
        `;
        
        return Buffer.from(svg);
    }

    startScreenCapture() {
        if (this.screenCaptureInterval) {
            console.log('📺 Screen capture already running');
            return;
        }
        
        console.log('📺 Starting screen capture...');
        
        this.screenCaptureInterval = setInterval(async () => {
            try {
                const screenshot = await this.takeScreenshot();
                const base64Data = screenshot.toString('base64');
                
                // Send via Supabase Realtime
                await this.sendRealtimeResponse({
                    type: 'screen_frame',
                    frameData: `data:image/jpeg;base64,${base64Data}`,
                    timestamp: Date.now()
                });
                
                console.log('📸 Screen frame sent');
            } catch (error) {
                console.error('❌ Screen capture error:', error.message);
            }
        }, 1000); // 1 FPS for Supabase
    }

    stopScreenCapture() {
        if (this.screenCaptureInterval) {
            clearInterval(this.screenCaptureInterval);
            this.screenCaptureInterval = null;
            console.log('📺 Screen capture stopped');
        }
    }

    async sendRealtimeResponse(data) {
        try {
            await this.realtimeChannel.send({
                type: 'broadcast',
                event: 'agent-response',
                payload: data
            });
        } catch (error) {
            console.error('❌ Realtime send error:', error.message);
        }
    }

    startHeartbeat() {
        console.log('💓 Starting heartbeat...');
        setInterval(() => {
            this.sendHeartbeat();
        }, 30000); // Every 30 seconds
    }

    async sendHeartbeat() {
        try {
            const { error } = await this.supabaseClient
                .from('remote_devices')
                .update({ 
                    last_seen: new Date().toISOString(),
                    status: 'online',
                    ip_address: this.getLocalIP()
                })
                .eq('id', this.deviceId);

            if (error) {
                console.error('❌ Heartbeat failed:', error.message);
            } else {
                console.log('💓 Heartbeat sent');
            }
        } catch (error) {
            console.error('❌ Heartbeat error:', error.message);
        }
    }

    getLocalIP() {
        try {
            const interfaces = os.networkInterfaces();
            for (const interfaceName in interfaces) {
                const iface = interfaces[interfaceName];
                for (const alias of iface) {
                    if (alias.family === 'IPv4' && !alias.internal) {
                        return alias.address;
                    }
                }
            }
            return '127.0.0.1';
        } catch (error) {
            return '127.0.0.1';
        }
    }

    // Generate self-signed certificate for WSS
    generateSelfSignedCert() {
        try {
            const certDir = path.join(__dirname, 'certs');
            
            // Create certs directory if it doesn't exist
            if (!fs.existsSync(certDir)) {
                fs.mkdirSync(certDir, { recursive: true });
            }
            
            const keyPath = path.join(certDir, 'key.pem');
            const certPath = path.join(certDir, 'cert.pem');
            
            // Check if certificates already exist
            if (fs.existsSync(keyPath) && fs.existsSync(certPath)) {
                console.log('🔒 Using existing SSL certificates');
                return { keyPath, certPath };
            }
            
            // Generate self-signed certificate using OpenSSL (if available)
            try {
                const { execSync } = require('child_process');
                const opensslCmd = `openssl req -x509 -newkey rsa:2048 -keyout "${keyPath}" -out "${certPath}" -days 365 -nodes -subj "/C=US/ST=State/L=City/O=RemoteDesktop/CN=localhost"`;
                execSync(opensslCmd, { stdio: 'ignore' });
                console.log('🔒 Generated new SSL certificates with OpenSSL');
                return { keyPath, certPath };
            } catch (opensslError) {
                // Fallback: Create basic self-signed cert with Node.js crypto
                console.log('⚠️ OpenSSL not available, creating basic self-signed certificate');
                
                const { generateKeyPairSync } = require('crypto');
                const { publicKey, privateKey } = generateKeyPairSync('rsa', {
                    modulusLength: 2048,
                    publicKeyEncoding: { type: 'spki', format: 'pem' },
                    privateKeyEncoding: { type: 'pkcs8', format: 'pem' }
                });
                
                // Create a basic certificate (this is simplified)
                fs.writeFileSync(keyPath, privateKey);
                fs.writeFileSync(certPath, publicKey);
                
                console.log('🔒 Generated basic SSL certificates');
                return { keyPath, certPath };
            }
        } catch (error) {
            console.error('❌ Certificate generation failed:', error.message);
            return null;
        }
    }

    // Start WebSocket servers (both WS and WSS)
    async startWebSocketServers() {
        if (!WebSocket) {
            console.log('⚠️ WebSocket module not available, skipping WebSocket servers');
            return;
        }
        
        try {
            // Start HTTP WebSocket server (port 8080)
            this.httpServer = http.createServer();
            this.wsServer = new WebSocket.Server({ server: this.httpServer });
            
            this.httpServer.listen(this.httpPort, () => {
                console.log(`🌐 HTTP WebSocket server listening on port ${this.httpPort}`);
            });
            
            // Setup WebSocket handlers for HTTP server
            this.setupWebSocketHandlers(this.wsServer, 'WS');
            
            // Try to start HTTPS WebSocket server (port 8443)
            const certs = this.generateSelfSignedCert();
            if (certs && fs.existsSync(certs.keyPath) && fs.existsSync(certs.certPath)) {
                try {
                    const httpsOptions = {
                        key: fs.readFileSync(certs.keyPath),
                        cert: fs.readFileSync(certs.certPath)
                    };
                    
                    this.httpsServer = https.createServer(httpsOptions);
                    this.wssServer = new WebSocket.Server({ server: this.httpsServer });
                    
                    this.httpsServer.listen(this.httpsPort, () => {
                        console.log(`🔒 HTTPS WebSocket server (WSS) listening on port ${this.httpsPort}`);
                    });
                    
                    // Setup WebSocket handlers for HTTPS server
                    this.setupWebSocketHandlers(this.wssServer, 'WSS');
                } catch (httpsError) {
                    console.log('⚠️ HTTPS server failed, using HTTP only:', httpsError.message);
                }
            } else {
                console.log('⚠️ SSL certificates not available, using HTTP WebSocket only');
            }
            
        } catch (error) {
            console.error('❌ WebSocket server startup failed:', error.message);
        }
    }

    // Setup WebSocket connection handlers
    setupWebSocketHandlers(wsServer, serverType) {
        wsServer.on('connection', (ws, req) => {
            const clientIP = req.socket.remoteAddress;
            console.log(`🔌 ${serverType} client connected from ${clientIP}`);
            
            this.connectedClients.add(ws);
            
            // Send welcome message
            ws.send(JSON.stringify({
                type: 'welcome',
                deviceId: this.deviceId,
                deviceName: this.deviceName,
                serverType: serverType
            }));
            
            // Handle incoming messages
            ws.on('message', async (data) => {
                try {
                    const message = JSON.parse(data.toString());
                    await this.handleWebSocketMessage(ws, message);
                } catch (error) {
                    console.error('❌ WebSocket message error:', error.message);
                }
            });
            
            // Handle client disconnect
            ws.on('close', () => {
                console.log(`🔌 ${serverType} client disconnected`);
                this.connectedClients.delete(ws);
                
                // Stop screen sharing if no clients connected
                if (this.connectedClients.size === 0) {
                    this.stopScreenSharing();
                }
            });
            
            ws.on('error', (error) => {
                console.error(`❌ ${serverType} WebSocket error:`, error.message);
                this.connectedClients.delete(ws);
            });
        });
    }

    // Handle WebSocket messages from dashboard
    async handleWebSocketMessage(ws, message) {
        console.log(`📨 WebSocket message: ${message.type}`);
        
        switch (message.type) {
            case 'start-screen-share':
                this.startScreenSharing();
                break;
                
            case 'stop-screen-share':
                this.stopScreenSharing();
                break;
                
            case 'mouse-move':
                await this.handleMouseCommand({ type: 'move', x: message.x, y: message.y });
                break;
                
            case 'mouse-click':
                await this.handleMouseCommand({ type: 'click', x: message.x, y: message.y, button: message.button || 'left' });
                break;
                
            case 'keyboard':
                await this.handleKeyboardCommand({ key: message.key });
                break;
                
            default:
                console.log(`❓ Unknown WebSocket message type: ${message.type}`);
        }
    }

    // Start screen sharing via WebSocket
    startScreenSharing() {
        if (this.screenShareInterval) {
            console.log('📺 Screen sharing already active');
            return;
        }
        
        console.log('📺 Starting WebSocket screen sharing...');
        
        this.screenShareInterval = setInterval(async () => {
            try {
                const screenshot = await this.takeScreenshot();
                const base64Data = screenshot.toString('base64');
                
                // Send to all connected WebSocket clients
                const message = JSON.stringify({
                    type: 'screen-frame',
                    data: `data:image/jpeg;base64,${base64Data}`,
                    timestamp: Date.now()
                });
                
                this.connectedClients.forEach(client => {
                    if (client.readyState === WebSocket.OPEN) {
                        client.send(message);
                    }
                });
            } catch (error) {
                console.error('❌ Screen sharing error:', error.message);
            }
        }, 100); // 10 FPS for WebSocket
    }

    // Stop screen sharing
    stopScreenSharing() {
        if (this.screenShareInterval) {
            clearInterval(this.screenShareInterval);
            this.screenShareInterval = null;
            console.log('⏹️ Screen sharing stopped');
        }
    }

    keepAlive() {
        // Keep the process running
        process.on('SIGINT', () => {
            console.log('\n👋 Shutting down agent...');
            this.cleanup();
            process.exit(0);
        });

        process.on('SIGTERM', () => {
            console.log('\n👋 Shutting down agent...');
            this.cleanup();
            process.exit(0);
        });
    }

    async cleanup() {
        try {
            // Mark device as offline
            await this.supabaseClient
                .from('remote_devices')
                .update({ status: 'offline', is_online: false })
                .eq('id', this.deviceId);

            // Unsubscribe from realtime
            if (this.realtimeChannel) {
                await this.realtimeChannel.unsubscribe();
            }

            // Stop screen capture and sharing
            this.stopScreenCapture();
            this.stopScreenSharing();

            console.log('✅ Cleanup completed');
        } catch (error) {
            console.error('❌ Cleanup error:', error.message);
        }
    }
}

// Start the Standalone Agent
const agent = new StandaloneAgent();
agent.initialize().catch(console.error);
