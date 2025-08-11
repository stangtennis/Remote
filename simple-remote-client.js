#!/usr/bin/env node

/**
 * Simple Remote Desktop Client - TeamViewer Style
 * Single EXE that auto-registers with dashboard for remote control
 */

const { createClient } = require('@supabase/supabase-js');
const os = require('os');
const crypto = require('crypto');
const express = require('express');
const WebSocket = require('ws');
const http = require('http');

// Configuration
const SUPABASE_URL = 'https://ptrtibzwokjcjjxvjpin.supabase.co';
const SUPABASE_ANON_KEY = 'REDACTED_JWT';

class SimpleRemoteClient {
    constructor() {
        this.supabase = createClient(SUPABASE_URL, SUPABASE_ANON_KEY);
        this.deviceId = this.generateDeviceId();
        this.isRunning = false;
        this.mode = 'one-time'; // 'one-time' or 'service'
        this.heartbeatInterval = null;
        this.webServer = null;
        this.wsServer = null;
        
        console.log('🚀 Simple Remote Desktop Client');
        console.log(`📱 Device ID: ${this.deviceId}`);
    }
    
    // Generate hardware-based device ID
    generateDeviceId() {
        const hostname = os.hostname();
        const platform = os.platform();
        const arch = os.arch();
        const cpus = os.cpus().length;
        const totalMem = os.totalmem();
        
        const fingerprint = `${hostname}-${platform}-${arch}-${cpus}-${totalMem}`;
        return crypto.createHash('sha256').update(fingerprint).digest('hex').substring(0, 16);
    }
    
    // Start the client
    async start(mode = 'one-time') {
        this.mode = mode;
        console.log(`🔄 Starting in ${mode} mode...`);
        
        try {
            // Register device
            await this.registerDevice();
            
            // Start web server for local control
            await this.startWebServer();
            
            // Start heartbeat
            this.startHeartbeat();
            
            // Start WebRTC signaling
            this.startSignaling();
            
            this.isRunning = true;
            console.log('✅ Client started successfully');
            console.log(`🌐 Local control: http://localhost:8080`);
            console.log(`📱 Device ID: ${this.deviceId}`);
            
            if (mode === 'service') {
                console.log('🔄 Running as service (always on)');
            } else {
                console.log('🔗 Running for one-time sharing');
            }
            
        } catch (error) {
            console.error('❌ Failed to start client:', error.message);
        }
    }
    
    // Register device with Supabase
    async registerDevice() {
        const deviceInfo = {
            id: this.deviceId,
            device_name: os.hostname(),
            operating_system: `${os.platform()} ${os.release()}`,
            ip_address: this.getLocalIP(),
            status: 'online',
            is_online: true,
            last_seen: new Date().toISOString(),
            agent_version: '6.0.0-simple',
            mode: this.mode,
            local_port: 8080
        };
        
        console.log('📝 Registering device...');
        
        const { error } = await this.supabase
            .from('remote_devices')
            .upsert(deviceInfo, { onConflict: 'id' });
            
        if (error) {
            throw new Error(`Registration failed: ${error.message}`);
        }
        
        console.log('✅ Device registered successfully');
    }
    
    // Get local IP address
    getLocalIP() {
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
    
    // Start web server for local control
    async startWebServer() {
        const app = express();
        const server = http.createServer(app);
        
        // Serve static files
        app.use(express.static('public'));
        
        // API endpoints
        app.get('/api/status', (req, res) => {
            res.json({
                deviceId: this.deviceId,
                status: 'online',
                mode: this.mode,
                hostname: os.hostname(),
                platform: os.platform(),
                uptime: process.uptime()
            });
        });
        
        app.get('/api/screenshot', async (req, res) => {
            try {
                const screenshot = await this.takeScreenshot();
                res.set('Content-Type', 'image/jpeg');
                res.send(screenshot);
            } catch (error) {
                res.status(500).json({ error: error.message });
            }
        });
        
        // Start server
        server.listen(8080, () => {
            console.log('🌐 Web server started on port 8080');
        });
        
        this.webServer = server;
        
        // Start WebSocket server for real-time control
        this.wsServer = new WebSocket.Server({ server });
        this.wsServer.on('connection', (ws) => {
            console.log('🔗 Dashboard connected');
            
            ws.on('message', (data) => {
                try {
                    const message = JSON.parse(data);
                    this.handleControlMessage(message, ws);
                } catch (error) {
                    console.error('❌ Invalid message:', error.message);
                }
            });
            
            ws.on('close', () => {
                console.log('🔌 Dashboard disconnected');
            });
        });
    }
    
    // Handle control messages from dashboard
    handleControlMessage(message, ws) {
        console.log(`📨 Control message: ${message.type}`);
        
        switch (message.type) {
            case 'mouse-move':
                this.handleMouseMove(message.x, message.y);
                break;
                
            case 'mouse-click':
                this.handleMouseClick(message.x, message.y, message.button);
                break;
                
            case 'keyboard':
                this.handleKeyboard(message.key, message.type);
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
    }
    
    // Screen capture (mock for now, will be replaced with real capture)
    async takeScreenshot() {
        // Mock screenshot - replace with real screen capture
        const mockImage = Buffer.from('mock-screenshot-data');
        return mockImage;
    }
    
    // Mouse control (mock for now)
    handleMouseMove(x, y) {
        console.log(`🖱️ Mouse move: (${x}, ${y})`);
        // TODO: Implement real mouse control with robotjs
    }
    
    handleMouseClick(x, y, button = 'left') {
        console.log(`🖱️ Mouse click: (${x}, ${y}) ${button}`);
        // TODO: Implement real mouse click with robotjs
    }
    
    // Keyboard control (mock for now)
    handleKeyboard(key, type = 'press') {
        console.log(`⌨️ Keyboard ${type}: ${key}`);
        // TODO: Implement real keyboard control with robotjs
    }
    
    // Screen sharing
    startScreenShare(ws) {
        console.log('📺 Starting screen sharing...');
        
        // Send screen frames every 100ms (10 FPS)
        this.screenShareInterval = setInterval(async () => {
            try {
                const screenshot = await this.takeScreenshot();
                const frame = {
                    type: 'screen-frame',
                    data: screenshot.toString('base64'),
                    timestamp: Date.now()
                };
                
                if (ws.readyState === WebSocket.OPEN) {
                    ws.send(JSON.stringify(frame));
                }
            } catch (error) {
                console.error('❌ Screen capture error:', error.message);
            }
        }, 100);
    }
    
    stopScreenShare() {
        console.log('⏹️ Stopping screen sharing...');
        if (this.screenShareInterval) {
            clearInterval(this.screenShareInterval);
            this.screenShareInterval = null;
        }
    }
    
    // Start signaling for WebRTC
    startSignaling() {
        // TODO: Implement WebRTC signaling for peer-to-peer connection
        console.log('🔗 WebRTC signaling ready');
    }
    
    // Start heartbeat to keep device online
    startHeartbeat() {
        this.heartbeatInterval = setInterval(async () => {
            try {
                const { error } = await this.supabase
                    .from('remote_devices')
                    .update({ 
                        last_seen: new Date().toISOString(),
                        status: 'online',
                        is_online: true
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
        }, 30000); // Every 30 seconds
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
        
        // Close servers
        if (this.webServer) {
            this.webServer.close();
        }
        
        if (this.wsServer) {
            this.wsServer.close();
        }
        
        // Update device status
        try {
            await this.supabase
                .from('remote_devices')
                .update({ 
                    status: 'offline',
                    is_online: false,
                    last_seen: new Date().toISOString()
                })
                .eq('id', this.deviceId);
        } catch (error) {
            console.error('❌ Failed to update offline status:', error.message);
        }
        
        console.log('✅ Client stopped');
    }
}

// CLI Interface
if (require.main === module) {
    const client = new SimpleRemoteClient();
    
    // Parse command line arguments
    const args = process.argv.slice(2);
    const mode = args.includes('--service') ? 'service' : 'one-time';
    
    // Start client
    client.start(mode);
    
    // Handle graceful shutdown
    process.on('SIGINT', async () => {
        console.log('\n🛑 Received shutdown signal...');
        await client.stop();
        process.exit(0);
    });
    
    process.on('SIGTERM', async () => {
        console.log('\n🛑 Received termination signal...');
        await client.stop();
        process.exit(0);
    });
}

module.exports = SimpleRemoteClient;
