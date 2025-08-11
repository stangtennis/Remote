#!/usr/bin/env node

/**
 * Stable Remote Desktop Client - No Crash Version
 * Shows all errors and keeps console open
 */

const { createClient } = require('@supabase/supabase-js');
const os = require('os');
const crypto = require('crypto');
const express = require('express');
const WebSocket = require('ws');
const http = require('http');

// Configuration
const SUPABASE_URL = 'https://ptrtibzwokjcjjxvjpin.supabase.co';
const SUPABASE_ANON_KEY = 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJzdXBhYmFzZSIsInJlZiI6InB0cnRpYnp3b2tqY2pqeHZqcGluIiwicm9sZSI6ImFub24iLCJpYXQiOjE3NTQ0MzE1NzEsImV4cCI6MjA3MDAwNzU3MX0.DPNxkQul1-13tqJ89mqYJAx7NSJjabOP4q8c6KgOnWk';

// Global error handlers to prevent crashes
process.on('uncaughtException', (error) => {
    console.error('❌ UNCAUGHT EXCEPTION:', error.message);
    console.error('Stack:', error.stack);
    console.log('⚠️ Application will continue running...');
});

process.on('unhandledRejection', (reason, promise) => {
    console.error('❌ UNHANDLED REJECTION:', reason);
    console.log('⚠️ Application will continue running...');
});

class StableRemoteClient {
    constructor() {
        this.deviceId = this.generateDeviceId();
        this.isRunning = false;
        this.mode = 'one-time';
        this.heartbeatInterval = null;
        this.webServer = null;
        this.wsServer = null;
        this.screenShareInterval = null;
        this.connectedClients = new Set();
        this.supabase = null;
        
        console.log('🚀 Stable Remote Desktop Client v1.0');
        console.log(`📱 Device ID: ${this.deviceId}`);
        console.log(`🏠 Computer: ${os.hostname()}`);
        console.log(`💻 Platform: ${os.platform()} ${os.arch()}`);
        console.log('');
    }
    
    // Generate hardware-based device ID
    generateDeviceId() {
        try {
            const hostname = os.hostname();
            const platform = os.platform();
            const arch = os.arch();
            const cpus = os.cpus().length;
            const totalMem = os.totalmem();
            
            const fingerprint = `${hostname}-${platform}-${arch}-${cpus}-${totalMem}`;
            return crypto.createHash('sha256').update(fingerprint).digest('hex').substring(0, 16);
        } catch (error) {
            console.error('❌ Error generating device ID:', error.message);
            return 'fallback-' + Date.now().toString(36);
        }
    }
    
    // Initialize Supabase connection
    async initializeSupabase() {
        try {
            console.log('🔗 Connecting to Supabase...');
            this.supabase = createClient(SUPABASE_URL, SUPABASE_ANON_KEY);
            
            // Test connection
            const { data, error } = await this.supabase
                .from('remote_devices')
                .select('count')
                .limit(1);
                
            if (error) {
                throw new Error(`Supabase connection failed: ${error.message}`);
            }
            
            console.log('✅ Supabase connected successfully');
            return true;
        } catch (error) {
            console.error('❌ Supabase initialization failed:', error.message);
            console.log('⚠️ Will continue in offline mode');
            return false;
        }
    }
    
    // Start the client
    async start(mode = 'one-time') {
        this.mode = mode;
        console.log(`🔄 Starting in ${mode} mode...`);
        console.log('');
        
        try {
            // Initialize Supabase
            const supabaseOk = await this.initializeSupabase();
            
            // Register device (if Supabase is available)
            if (supabaseOk) {
                await this.registerDevice();
            }
            
            // Start web server
            await this.startWebServer();
            
            // Start heartbeat (if Supabase is available)
            if (supabaseOk) {
                this.startHeartbeat();
            }
            
            this.isRunning = true;
            console.log('✅ Client started successfully!');
            console.log('');
            console.log('📋 Connection Info:');
            console.log(`   🌐 Local control: http://localhost:8080`);
            console.log(`   📱 Device ID: ${this.deviceId}`);
            console.log(`   🏠 Computer: ${os.hostname()}`);
            console.log(`   🔄 Mode: ${mode}`);
            console.log('');
            console.log('🎯 Dashboard: https://stangtennis.github.io/remote-desktop/dashboard.html');
            console.log('');
            console.log('⚠️ Keep this window open for remote access');
            console.log('💡 Press Ctrl+C to stop the client');
            console.log('');
            
        } catch (error) {
            console.error('❌ Failed to start client:', error.message);
            console.log('⚠️ Will retry in 5 seconds...');
            setTimeout(() => this.start(mode), 5000);
        }
    }
    
    // Register device with Supabase
    async registerDevice() {
        try {
            const deviceInfo = {
                id: this.deviceId,
                device_name: os.hostname(),
                operating_system: `${os.platform()} ${os.release()}`,
                ip_address: this.getLocalIP(),
                status: 'online',
                is_online: true,
                last_seen: new Date().toISOString(),
                agent_version: '8.0.0-stable',
                mode: this.mode,
                local_port: 8080
            };
            
            console.log('📝 Registering device with dashboard...');
            
            const { error } = await this.supabase
                .from('remote_devices')
                .upsert(deviceInfo, { onConflict: 'id' });
                
            if (error) {
                throw new Error(`Registration failed: ${error.message}`);
            }
            
            console.log('✅ Device registered successfully');
        } catch (error) {
            console.error('❌ Device registration failed:', error.message);
            console.log('⚠️ Will continue without dashboard registration');
        }
    }
    
    // Get local IP address
    getLocalIP() {
        try {
            const interfaces = os.networkInterfaces();
            for (const name of Object.keys(interfaces)) {
                for (const iface of interfaces[name]) {
                    if (iface.family === 'IPv4' && !iface.internal) {
                        return iface.address;
                    }
                }
            }
            return '127.0.0.1';
        } catch (error) {
            console.error('❌ Error getting local IP:', error.message);
            return '127.0.0.1';
        }
    }
    
    // Start web server
    async startWebServer() {
        try {
            const app = express();
            const server = http.createServer(app);
            
            // Enable CORS
            app.use((req, res, next) => {
                res.header('Access-Control-Allow-Origin', '*');
                res.header('Access-Control-Allow-Headers', 'Origin, X-Requested-With, Content-Type, Accept');
                next();
            });
            
            // API endpoints
            app.get('/api/status', (req, res) => {
                res.json({
                    deviceId: this.deviceId,
                    status: 'online',
                    mode: this.mode,
                    hostname: os.hostname(),
                    platform: os.platform(),
                    uptime: process.uptime(),
                    connectedClients: this.connectedClients.size,
                    version: '8.0.0-stable'
                });
            });
            
            app.get('/api/screenshot', async (req, res) => {
                try {
                    const screenshot = this.createMockScreenshot();
                    res.set('Content-Type', 'image/png');
                    res.send(screenshot);
                } catch (error) {
                    res.status(500).json({ error: error.message });
                }
            });
            
            // Start server
            server.listen(8080, '0.0.0.0', () => {
                console.log('🌐 Web server started on port 8080');
            });
            
            this.webServer = server;
            
            // Start WebSocket server
            this.wsServer = new WebSocket.Server({ server });
            this.wsServer.on('connection', (ws, req) => {
                const clientIP = req.socket.remoteAddress;
                console.log(`🔗 Dashboard connected from ${clientIP}`);
                
                this.connectedClients.add(ws);
                
                ws.on('message', (data) => {
                    try {
                        const message = JSON.parse(data);
                        this.handleControlMessage(message, ws);
                    } catch (error) {
                        console.error('❌ Invalid message:', error.message);
                    }
                });
                
                ws.on('close', () => {
                    console.log(`🔌 Dashboard disconnected from ${clientIP}`);
                    this.connectedClients.delete(ws);
                });
                
                ws.on('error', (error) => {
                    console.error('❌ WebSocket error:', error.message);
                    this.connectedClients.delete(ws);
                });
            });
            
        } catch (error) {
            console.error('❌ Failed to start web server:', error.message);
            throw error;
        }
    }
    
    // Handle control messages
    handleControlMessage(message, ws) {
        console.log(`📨 Control message: ${message.type}`);
        
        try {
            switch (message.type) {
                case 'mouse-move':
                    console.log(`🖱️ Mouse move: (${message.x}, ${message.y})`);
                    this.handleRealMouseMove(message.x, message.y);
                    break;
                    
                case 'mouse-click':
                    console.log(`🖱️ Mouse click: (${message.x}, ${message.y}) ${message.button}`);
                    this.handleRealMouseClick(message.x, message.y, message.button);
                    break;
                    
                case 'keyboard':
                    console.log(`⌨️ Keyboard: ${message.key}`);
                    this.handleRealKeyboard(message.key);
                    break;
                    
                case 'start-screen-share':
                    this.startScreenShare(ws);
                    break;
                    
                case 'stop-screen-share':
                    this.stopScreenShare();
                    break;
                    
                default:
                    console.log(`❓ Unknown control message: ${message.type}`);
            }
        } catch (error) {
            console.error('❌ Error handling control message:', error.message);
        }
    }
    
    // Real screen capture
    async takeScreenshot() {
        try {
            // Try to load native modules dynamically
            if (!this.screenshotModule) {
                try {
                    this.screenshotModule = require('screenshot-desktop');
                    this.sharpModule = require('sharp');
                    console.log('✅ Native screenshot modules loaded');
                } catch (error) {
                    console.log('⚠️ Native modules not available, using mock');
                    this.screenshotModule = null;
                }
            }
            
            if (this.screenshotModule && this.sharpModule) {
                // Real screenshot
                const img = await this.screenshotModule({ format: 'png' });
                const compressedImg = await this.sharpModule(img)
                    .jpeg({ quality: 60 })
                    .resize(1280, 720, { fit: 'inside', withoutEnlargement: true })
                    .toBuffer();
                
                console.log(`📸 Real screenshot captured (${compressedImg.length} bytes)`);
                return compressedImg;
            } else {
                // Mock screenshot
                return this.createMockScreenshot();
            }
        } catch (error) {
            console.error('❌ Screenshot error:', error.message);
            return this.createMockScreenshot();
        }
    }
    
    // Create mock screenshot
    createMockScreenshot() {
        try {
            // Create a simple test pattern
            const width = 800;
            const height = 600;
            const canvas = Buffer.alloc(width * height * 4); // RGBA
            
            // Fill with test pattern
            for (let y = 0; y < height; y++) {
                for (let x = 0; x < width; x++) {
                    const offset = (y * width + x) * 4;
                    canvas[offset] = Math.floor((x / width) * 255);     // Red
                    canvas[offset + 1] = Math.floor((y / height) * 255); // Green
                    canvas[offset + 2] = 128;                            // Blue
                    canvas[offset + 3] = 255;                            // Alpha
                }
            }
            
            return canvas;
        } catch (error) {
            console.error('❌ Error creating screenshot:', error.message);
            return Buffer.alloc(1000); // Empty buffer
        }
    }
    
    // Screen sharing
    startScreenShare(ws) {
        console.log('📺 Starting screen sharing...');
        
        try {
            // Send screen frames every 200ms (5 FPS)
            this.screenShareInterval = setInterval(async () => {
                try {
                    const screenshot = await this.takeScreenshot();
                    const frame = {
                        type: 'screen-frame',
                        data: screenshot.toString('base64'),
                        timestamp: Date.now(),
                        width: 1280,
                        height: 720
                    };
                    
                    if (ws.readyState === WebSocket.OPEN) {
                        ws.send(JSON.stringify(frame));
                    }
                } catch (error) {
                    console.error('❌ Screen frame error:', error.message);
                }
            }, 200);
        } catch (error) {
            console.error('❌ Error starting screen share:', error.message);
        }
    }
    
    stopScreenShare() {
        console.log('⏹️ Stopping screen sharing...');
        if (this.screenShareInterval) {
            clearInterval(this.screenShareInterval);
            this.screenShareInterval = null;
        }
    }
    
    // Real mouse control methods
    handleRealMouseMove(x, y) {
        try {
            // Try to load robotjs dynamically
            if (!this.robotModule) {
                try {
                    this.robotModule = require('robotjs');
                    console.log('✅ RobotJS loaded for real mouse control');
                } catch (error) {
                    console.log('⚠️ RobotJS not available, using Windows API fallback');
                    this.robotModule = null;
                }
            }
            
            if (this.robotModule) {
                // Use robotjs for mouse control
                this.robotModule.moveMouse(x, y);
                console.log(`🖱️ Real mouse moved to (${x}, ${y})`);
            } else {
                // Fallback: Use Windows API via child_process
                this.moveMouseWindows(x, y);
            }
        } catch (error) {
            console.error('❌ Mouse move error:', error.message);
        }
    }
    
    handleRealMouseClick(x, y, button = 'left') {
        try {
            if (this.robotModule) {
                this.robotModule.moveMouse(x, y);
                this.robotModule.mouseClick(button);
                console.log(`🖱️ Real mouse clicked at (${x}, ${y}) ${button}`);
            } else {
                this.clickMouseWindows(x, y, button);
            }
        } catch (error) {
            console.error('❌ Mouse click error:', error.message);
        }
    }
    
    handleRealKeyboard(key) {
        try {
            if (this.robotModule) {
                this.robotModule.keyTap(key);
                console.log(`⌨️ Real key pressed: ${key}`);
            } else {
                this.pressKeyWindows(key);
            }
        } catch (error) {
            console.error('❌ Keyboard error:', error.message);
        }
    }
    
    // Windows API fallback methods
    moveMouseWindows(x, y) {
        try {
            const { exec } = require('child_process');
            // Use PowerShell to move mouse cursor
            const script = `Add-Type -AssemblyName System.Windows.Forms; [System.Windows.Forms.Cursor]::Position = New-Object System.Drawing.Point(${x}, ${y})`;
            exec(`powershell -Command "${script}"`, (error) => {
                if (error) {
                    console.error('❌ Windows mouse move error:', error.message);
                } else {
                    console.log(`🖱️ Windows API mouse moved to (${x}, ${y})`);
                }
            });
        } catch (error) {
            console.error('❌ Windows mouse move fallback error:', error.message);
        }
    }
    
    clickMouseWindows(x, y, button = 'left') {
        try {
            const { exec } = require('child_process');
            // Use PowerShell to click mouse
            const clickType = button === 'right' ? 'RightClick' : 'LeftClick';
            const script = `
                Add-Type -AssemblyName System.Windows.Forms;
                [System.Windows.Forms.Cursor]::Position = New-Object System.Drawing.Point(${x}, ${y});
                Start-Sleep -Milliseconds 50;
                [System.Windows.Forms.SendKeys]::SendWait("{${clickType}}")
            `;
            exec(`powershell -Command "${script}"`, (error) => {
                if (error) {
                    console.error('❌ Windows mouse click error:', error.message);
                } else {
                    console.log(`🖱️ Windows API mouse clicked at (${x}, ${y}) ${button}`);
                }
            });
        } catch (error) {
            console.error('❌ Windows mouse click fallback error:', error.message);
        }
    }
    
    pressKeyWindows(key) {
        try {
            const { exec } = require('child_process');
            // Use PowerShell to press key
            const script = `Add-Type -AssemblyName System.Windows.Forms; [System.Windows.Forms.SendKeys]::SendWait("${key}")`;
            exec(`powershell -Command "${script}"`, (error) => {
                if (error) {
                    console.error('❌ Windows key press error:', error.message);
                } else {
                    console.log(`⌨️ Windows API key pressed: ${key}`);
                }
            });
        } catch (error) {
            console.error('❌ Windows key press fallback error:', error.message);
        }
    }
    
    // Start heartbeat
    startHeartbeat() {
        this.heartbeatInterval = setInterval(async () => {
            try {
                if (this.supabase) {
                    const { error } = await this.supabase
                        .from('remote_devices')
                        .update({ 
                            last_seen: new Date().toISOString(),
                            status: 'online',
                            is_online: true,
                            connected_clients: this.connectedClients.size
                        })
                        .eq('id', this.deviceId);
                        
                    if (error) {
                        console.error('❌ Heartbeat failed:', error.message);
                    } else {
                        console.log(`💓 Heartbeat sent (${this.connectedClients.size} clients)`);
                    }
                }
            } catch (error) {
                console.error('❌ Heartbeat error:', error.message);
            }
        }, 30000);
    }
    
    // Stop the client
    async stop() {
        console.log('🛑 Stopping client...');
        
        this.isRunning = false;
        
        // Clear intervals
        if (this.heartbeatInterval) {
            clearInterval(this.heartbeatInterval);
        }
        
        if (this.screenShareInterval) {
            clearInterval(this.screenShareInterval);
        }
        
        // Close connections
        this.connectedClients.forEach(ws => {
            if (ws.readyState === WebSocket.OPEN) {
                ws.close();
            }
        });
        
        // Close servers
        if (this.webServer) {
            this.webServer.close();
        }
        
        if (this.wsServer) {
            this.wsServer.close();
        }
        
        // Update status
        try {
            if (this.supabase) {
                await this.supabase
                    .from('remote_devices')
                    .update({ 
                        status: 'offline',
                        is_online: false,
                        last_seen: new Date().toISOString()
                    })
                    .eq('id', this.deviceId);
            }
        } catch (error) {
            console.error('❌ Failed to update offline status:', error.message);
        }
        
        console.log('✅ Client stopped');
    }
}

// CLI Interface
if (require.main === module) {
    console.log('🎯 Stable Remote Desktop Client');
    console.log('📋 TeamViewer-style remote access');
    console.log('');
    
    const client = new StableRemoteClient();
    
    // Parse command line arguments
    const args = process.argv.slice(2);
    const mode = args.includes('--service') ? 'service' : 'one-time';
    
    // Start client
    client.start(mode).catch(error => {
        console.error('❌ Startup failed:', error.message);
        console.log('⚠️ Press any key to exit...');
        process.stdin.setRawMode(true);
        process.stdin.resume();
        process.stdin.on('data', () => process.exit(1));
    });
    
    // Handle graceful shutdown
    process.on('SIGINT', async () => {
        console.log('\n🛑 Shutting down...');
        await client.stop();
        console.log('👋 Goodbye!');
        process.exit(0);
    });
    
    process.on('SIGTERM', async () => {
        console.log('\n🛑 Terminating...');
        await client.stop();
        process.exit(0);
    });
}

module.exports = StableRemoteClient;
