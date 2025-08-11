#!/usr/bin/env node

/**
 * Production Remote Desktop Client - TeamViewer Style
 * Standalone EXE with real screen capture and input control
 */

const { createClient } = require('@supabase/supabase-js');
const os = require('os');
const crypto = require('crypto');
const express = require('express');
const WebSocket = require('ws');
const http = require('http');

// Try to load native modules, fallback to simulation if not available
let screenshot, sharp, robot;
let hasNativeModules = false;

try {
    screenshot = require('screenshot-desktop');
    sharp = require('sharp');
    robot = require('robotjs');
    hasNativeModules = true;
    console.log('✅ Native modules loaded successfully');
} catch (error) {
    console.log('⚠️ Native modules not available, using simulation mode');
    hasNativeModules = false;
}

// Configuration
const SUPABASE_URL = 'https://ptrtibzwokjcjjxvjpin.supabase.co';
const SUPABASE_ANON_KEY = 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJzdXBhYmFzZSIsInJlZiI6InB0cnRpYnp3b2tqY2pqeHZqcGluIiwicm9sZSI6ImFub24iLCJpYXQiOjE3MzU2NzU5NzEsImV4cCI6MjA1MTI1MTk3MX0.TKzqpCqnhJMJzGHlxJz8X2vZ8FhqJhqJhqJhqJhqJhqJ';

class ProductionRemoteClient {
    constructor() {
        this.supabase = createClient(SUPABASE_URL, SUPABASE_ANON_KEY);
        this.deviceId = this.generateDeviceId();
        this.isRunning = false;
        this.mode = 'one-time';
        this.heartbeatInterval = null;
        this.webServer = null;
        this.wsServer = null;
        this.screenShareInterval = null;
        this.connectedClients = new Set();
        
        console.log('🚀 Production Remote Desktop Client');
        console.log(`📱 Device ID: ${this.deviceId}`);
        console.log(`🔧 Native modules: ${hasNativeModules ? 'Available' : 'Simulation mode'}`);
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
            
            // Start web server for control
            await this.startWebServer();
            
            // Start heartbeat
            this.startHeartbeat();
            
            this.isRunning = true;
            console.log('✅ Client started successfully');
            console.log(`🌐 Local control: http://localhost:8080`);
            console.log(`📱 Device ID: ${this.deviceId}`);
            console.log(`🏠 Computer: ${os.hostname()}`);
            
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
            agent_version: '7.0.0-production',
            mode: this.mode,
            local_port: 8080,
            has_native_modules: hasNativeModules
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
    
    // Start web server for control
    async startWebServer() {
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
                hasNativeModules: hasNativeModules,
                connectedClients: this.connectedClients.size
            });
        });
        
        app.get('/api/screenshot', async (req, res) => {
            try {
                const screenshotBuffer = await this.takeScreenshot();
                res.set('Content-Type', 'image/jpeg');
                res.send(screenshotBuffer);
            } catch (error) {
                res.status(500).json({ error: error.message });
            }
        });
        
        // Start server
        server.listen(8080, '0.0.0.0', () => {
            console.log('🌐 Web server started on port 8080 (all interfaces)');
        });
        
        this.webServer = server;
        
        // Start WebSocket server for real-time control
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
    
    // Real screen capture
    async takeScreenshot() {
        if (hasNativeModules) {
            try {
                // Capture screenshot
                const img = await screenshot({ format: 'png' });
                
                // Compress to JPEG with Sharp
                const compressedImg = await sharp(img)
                    .jpeg({ quality: 70 })
                    .resize(1920, 1080, { fit: 'inside', withoutEnlargement: true })
                    .toBuffer();
                
                return compressedImg;
            } catch (error) {
                console.error('❌ Screenshot error:', error.message);
                return this.createMockScreenshot();
            }
        } else {
            return this.createMockScreenshot();
        }
    }
    
    // Create mock screenshot for simulation
    createMockScreenshot() {
        // Create a simple colored rectangle as mock screenshot
        const width = 800;
        const height = 600;
        const canvas = Buffer.alloc(width * height * 3);
        
        // Fill with gradient colors
        for (let y = 0; y < height; y++) {
            for (let x = 0; x < width; x++) {
                const offset = (y * width + x) * 3;
                canvas[offset] = Math.floor((x / width) * 255);     // Red
                canvas[offset + 1] = Math.floor((y / height) * 255); // Green
                canvas[offset + 2] = 128;                            // Blue
            }
        }
        
        return canvas;
    }
    
    // Real mouse control
    handleMouseMove(x, y) {
        console.log(`🖱️ Mouse move: (${x}, ${y})`);
        
        if (hasNativeModules) {
            try {
                robot.moveMouse(x, y);
            } catch (error) {
                console.error('❌ Mouse move error:', error.message);
            }
        }
    }
    
    handleMouseClick(x, y, button = 'left') {
        console.log(`🖱️ Mouse click: (${x}, ${y}) ${button}`);
        
        if (hasNativeModules) {
            try {
                robot.moveMouse(x, y);
                robot.mouseClick(button);
            } catch (error) {
                console.error('❌ Mouse click error:', error.message);
            }
        }
    }
    
    // Real keyboard control
    handleKeyboard(key, type = 'press') {
        console.log(`⌨️ Keyboard ${type}: ${key}`);
        
        if (hasNativeModules) {
            try {
                if (type === 'press') {
                    robot.keyTap(key);
                } else if (type === 'down') {
                    robot.keyToggle(key, 'down');
                } else if (type === 'up') {
                    robot.keyToggle(key, 'up');
                }
            } catch (error) {
                console.error('❌ Keyboard error:', error.message);
            }
        }
    }
    
    // Screen sharing with real frames
    startScreenShare(ws) {
        console.log('📺 Starting screen sharing...');
        
        // Send screen frames every 200ms (5 FPS for better performance)
        this.screenShareInterval = setInterval(async () => {
            try {
                const screenshot = await this.takeScreenshot();
                const frame = {
                    type: 'screen-frame',
                    data: screenshot.toString('base64'),
                    timestamp: Date.now(),
                    width: hasNativeModules ? undefined : 800,
                    height: hasNativeModules ? undefined : 600
                };
                
                if (ws.readyState === WebSocket.OPEN) {
                    ws.send(JSON.stringify(frame));
                }
            } catch (error) {
                console.error('❌ Screen capture error:', error.message);
            }
        }, 200);
    }
    
    stopScreenShare() {
        console.log('⏹️ Stopping screen sharing...');
        if (this.screenShareInterval) {
            clearInterval(this.screenShareInterval);
            this.screenShareInterval = null;
        }
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
                        is_online: true,
                        connected_clients: this.connectedClients.size
                    })
                    .eq('id', this.deviceId);
                    
                if (error) {
                    console.error('❌ Heartbeat failed:', error.message);
                } else {
                    console.log(`💓 Heartbeat sent (${this.connectedClients.size} clients connected)`);
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
        
        // Close all client connections
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
    const client = new ProductionRemoteClient();
    
    // Parse command line arguments
    const args = process.argv.slice(2);
    const mode = args.includes('--service') ? 'service' : 'one-time';
    
    console.log('🎯 Production Remote Desktop Client');
    console.log('📋 Usage:');
    console.log('  node production-client.js          # One-time sharing');
    console.log('  node production-client.js --service # Always-on service');
    console.log('');
    
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

module.exports = ProductionRemoteClient;
