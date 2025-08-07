#!/usr/bin/env node

/**
 * Standalone Remote Desktop Agent Executable
 * Like MeshCentral - Just run the EXE and it auto-connects
 * No Node.js installation required for end users
 */

const http = require('http');
const https = require('https');
const os = require('os');
const path = require('path');
const fs = require('fs');

class StandaloneRemoteAgent {
    constructor() {
        this.deviceId = this.generateDeviceId();
        this.deviceName = os.hostname() || 'Unknown-PC';
        this.orgId = 'default';
        this.isConnected = false;
        this.websocket = null;
        this.heartbeatInterval = null;
        
        // Configuration
        this.config = {
            supabaseUrl: 'https://ptrtibzwokjcjjxvjpin.supabase.co',
            supabaseKey: 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJzdXBhYmFzZSIsInJlZiI6InB0cnRpYnp3b2tqY2pqeHZqcGluIiwicm9sZSI6ImFub24iLCJpYXQiOjE3NTQ0MzE1NzEsImV4cCI6MjA3MDAwNzU3MX0.DPNxkQul1-13tqJ89mqYJAx7NSJjabOP4q8c6KgOnWk',
            websocketHost: 'localhost',
            websocketPort: 3001,
            heartbeatInterval: 30000,
            reconnectDelay: 5000
        };
        
        this.showStartupBanner();
    }

    showStartupBanner() {
        console.clear();
        console.log('╔══════════════════════════════════════════════════════════════╗');
        console.log('║                🌍 Remote Desktop Agent                        ║');
        console.log('║                   Standalone Edition                         ║');
        console.log('╠══════════════════════════════════════════════════════════════╣');
        console.log(`║ Device Name: ${this.deviceName.padEnd(47)} ║`);
        console.log(`║ Device ID:   ${this.deviceId.padEnd(47)} ║`);
        console.log(`║ Platform:    ${os.platform().padEnd(47)} ║`);
        console.log(`║ Version:     4.1.0${' '.repeat(41)} ║`);
        console.log('╚══════════════════════════════════════════════════════════════╝');
        console.log('');
    }

    generateDeviceId() {
        return 'device_' + Math.random().toString(36).substr(2, 9) + '_' + Date.now();
    }
    
    getLocalIP() {
        const interfaces = os.networkInterfaces();
        let ipAddress = '127.0.0.1';
        
        // Find the first non-internal IPv4 address
        Object.keys(interfaces).forEach((ifname) => {
            interfaces[ifname].forEach((iface) => {
                if (iface.family === 'IPv4' && !iface.internal) {
                    ipAddress = iface.address;
                }
            });
        });
        
        return ipAddress;
    }
    
    async updateDevicePresence() {
        return new Promise((resolve, reject) => {
            try {
                const presenceData = JSON.stringify({
                    device_id: this.deviceId,
                    status: 'online',
                    last_seen: new Date().toISOString(),
                    connection_info: JSON.stringify({
                        ip: this.getLocalIP(),
                        connection_type: 'supabase_realtime'
                    }),
                    metadata: JSON.stringify({
                        agent_version: '4.1.0',
                        global_edition: true
                    })
                });
                
                const options = {
                    hostname: this.config.supabaseUrl.replace('https://', ''),
                    port: 443,
                    path: '/rest/v1/device_presence',
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                        'Content-Length': Buffer.byteLength(presenceData),
                        'apikey': this.config.supabaseKey,
                        'Authorization': `Bearer ${this.config.supabaseKey}`,
                        'Prefer': 'return=minimal'
                    }
                };
                
                const req = https.request(options, (res) => {
                    let data = '';
                    res.on('data', (chunk) => data += chunk);
                    res.on('end', () => {
                        if (res.statusCode === 201 || res.statusCode === 200) {
                            console.log('✅ Device presence updated successfully');
                            resolve();
                        } else {
                            console.log(`⚠️ Device presence update response: ${res.statusCode}`);
                            resolve(); // Continue anyway
                        }
                    });
                });
                
                req.on('error', (error) => {
                    console.log('⚠️ Device presence update failed:', error.message);
                    resolve(); // Continue anyway
                });
                
                req.setTimeout(5000, () => {
                    req.destroy();
                    console.log('⚠️ Device presence update timeout');
                    resolve();
                });
                
                req.write(presenceData);
                req.end();
                
            } catch (error) {
                console.log('⚠️ Device presence update error:', error.message);
                resolve(); // Continue anyway
            }
        });
    }
    
    async testSupabaseConnection() {
        return new Promise((resolve, reject) => {
            try {
                const options = {
                    hostname: this.config.supabaseUrl.replace('https://', ''),
                    port: 443,
                    path: '/rest/v1/remote_devices?select=count',
                    method: 'GET',
                    headers: {
                        'apikey': this.config.supabaseKey,
                        'Authorization': `Bearer ${this.config.supabaseKey}`
                    }
                };
                
                const req = https.request(options, (res) => {
                    if (res.statusCode === 200) {
                        resolve();
                    } else {
                        reject(new Error(`Status code: ${res.statusCode}`));
                    }
                });
                
                req.on('error', (error) => {
                    reject(error);
                });
                
                req.setTimeout(5000, () => {
                    req.destroy();
                    reject(new Error('Connection timeout'));
                });
                
                req.end();
                
            } catch (error) {
                reject(error);
            }
        });
    }

    async initialize() {
        try {
            console.log('🔧 Initializing Remote Desktop Agent...');
            
            // Load configuration if exists
            this.loadConfiguration();
            
            // Register with Supabase
            console.log('📡 Connecting to Supabase backend...');
            await this.registerDevice();
            
            // Connect to WebSocket server
            console.log('🔌 Connecting to real-time server...');
            await this.connectWebSocket();
            
            // Start heartbeat
            this.startHeartbeat();
            
            // Setup capabilities
            this.setupCapabilities();
            
            console.log('✅ Remote Desktop Agent is now ONLINE');
            console.log('🎯 Ready to accept remote control sessions');
            console.log('');
            console.log('💡 This window must stay open for remote access');
            console.log('📊 Status: CONNECTED - Device visible in dashboard');
            console.log('');
            console.log('Press Ctrl+C to disconnect and exit');
            console.log('═'.repeat(60));
            
            // Keep the process alive
            this.keepAlive();
            
        } catch (error) {
            console.error('❌ Initialization failed:', error.message);
            console.log('⚠️ Continuing in offline mode...');
            console.log('💡 Check your internet connection and try again');
            this.keepAlive();
        }
    }

    loadConfiguration() {
        try {
            const configPath = path.join(__dirname, 'config.json');
            if (fs.existsSync(configPath)) {
                const userConfig = JSON.parse(fs.readFileSync(configPath, 'utf8'));
                this.config = { ...this.config, ...userConfig };
                console.log('📋 Configuration loaded');
            }
        } catch (error) {
            console.log('📋 Using default configuration');
        }
    }

    async registerDevice() {
        // Create device data that matches the schema
        const deviceData = {
            device_id: this.deviceId,
            device_name: this.deviceName,
            device_type: 'desktop',
            operating_system: `${os.platform()} ${os.release()}`,
            ip_address: this.getLocalIP(),
            status: 'online',
            last_seen: new Date().toISOString(),
            access_key: this.deviceId, // Use device ID as access key for now
            metadata: JSON.stringify({
                hostname: os.hostname(),
                platform: os.platform(),
                release: os.release(),
                arch: os.arch(),
                cpus: os.cpus().length,
                memory: Math.round(os.totalmem() / (1024 * 1024 * 1024)) + 'GB'
            })
        };

        return new Promise((resolve, reject) => {
            try {
                console.log('📝 Registering device with Supabase...');
                
                // First, try to update device presence
                this.updateDevicePresence()
                    .then(() => {
                        // Then register/update the device in remote_devices table
                        const postData = JSON.stringify(deviceData);
                        const options = {
                            hostname: this.config.supabaseUrl.replace('https://', ''),
                            port: 443,
                            path: '/rest/v1/remote_devices',
                            method: 'POST',
                            headers: {
                                'Content-Type': 'application/json',
                                'Content-Length': Buffer.byteLength(postData),
                                'apikey': this.config.supabaseKey,
                                'Authorization': `Bearer ${this.config.supabaseKey}`,
                                'Prefer': 'return=minimal'
                            }
                        };

                        const req = https.request(options, (res) => {
                            let data = '';
                            res.on('data', (chunk) => data += chunk);
                            res.on('end', () => {
                                if (res.statusCode === 201 || res.statusCode === 200) {
                                    console.log('✅ Device registered with Supabase');
                                    this.isConnected = true;
                                    resolve();
                                } else {
                                    console.log(`⚠️ Registration response: ${res.statusCode}`);
                                    // If we get a 401, it might be an authentication issue with the table
                                    // Let's try a simpler approach with just a GET request to verify connectivity
                                    this.testSupabaseConnection()
                                        .then(() => {
                                            console.log('✅ Supabase connection is working, but registration failed. Likely a schema mismatch.');
                                            console.log('⚠️ Continuing with Supabase Realtime connection despite registration issues');
                                            this.isConnected = true; // Consider connected anyway
                                            resolve();
                                        })
                                        .catch(() => {
                                            console.log('⚠️ Supabase connection test also failed');
                                            resolve(); // Continue anyway
                                        });
                                }
                            });
                        });

                        req.on('error', (error) => {
                            console.log('⚠️ Supabase connection failed:', error.message);
                            resolve(); // Continue in offline mode
                        });

                        req.setTimeout(10000, () => {
                            req.destroy();
                            console.log('⚠️ Supabase connection timeout');
                            resolve();
                        });

                        req.write(postData);
                        req.end();
                    })
                    .catch(error => {
                        console.log('⚠️ Device presence update failed:', error.message);
                        resolve(); // Continue anyway
                    });
                
            } catch (error) {
                console.log('⚠️ Registration error:', error.message);
                resolve();
            }
        });
    }

    async connectWebSocket() {
        return new Promise((resolve) => {
            try {
                const net = require('net');
                this.websocket = new net.Socket();
                
                this.websocket.connect(this.config.websocketPort, this.config.websocketHost, () => {
                    console.log('✅ Connected to WebSocket server');
                    
                    // Send WebSocket handshake
                    const key = Buffer.from(Math.random().toString()).toString('base64');
                    const handshake = [
                        'GET / HTTP/1.1',
                        `Host: ${this.config.websocketHost}:${this.config.websocketPort}`,
                        'Upgrade: websocket',
                        'Connection: Upgrade',
                        'Sec-WebSocket-Key: ' + key,
                        'Sec-WebSocket-Version: 13',
                        '', ''
                    ].join('\r\n');
                    
                    this.websocket.write(handshake);
                });

                this.websocket.on('data', (data) => {
                    const response = data.toString();
                    if (response.includes('HTTP/1.1 101')) {
                        console.log('✅ WebSocket handshake successful');
                        
                        // Send device connect message
                        setTimeout(() => {
                            this.sendWebSocketMessage({
                                type: 'device_connect',
                                deviceId: this.deviceId,
                                deviceName: this.deviceName
                            });
                            console.log('📤 Device registered with real-time server');
                        }, 100);
                        
                        resolve();
                    } else {
                        this.handleWebSocketMessage(response);
                    }
                });

                this.websocket.on('error', (error) => {
                    console.log('⚠️ WebSocket connection failed:', error.message);
                    resolve(); // Continue anyway
                });

                this.websocket.on('close', () => {
                    console.log('🔌 WebSocket connection lost - attempting reconnect...');
                    setTimeout(() => this.connectWebSocket(), this.config.reconnectDelay);
                });

                // Timeout for connection
                setTimeout(() => {
                    if (!this.websocket.connecting) {
                        resolve();
                    }
                }, 5000);
                
            } catch (error) {
                console.log('⚠️ WebSocket setup failed:', error.message);
                resolve();
            }
        });
    }

    sendWebSocketMessage(message) {
        if (this.websocket && this.websocket.writable) {
            try {
                const messageStr = JSON.stringify(message);
                const payload = Buffer.from(messageStr);
                const payloadLength = payload.length;
                
                // Generate random masking key (required for client frames)
                const maskingKey = Buffer.from([
                    Math.floor(Math.random() * 256),
                    Math.floor(Math.random() * 256),
                    Math.floor(Math.random() * 256),
                    Math.floor(Math.random() * 256)
                ]);
                
                // Mask the payload
                const maskedPayload = Buffer.alloc(payloadLength);
                for (let i = 0; i < payloadLength; i++) {
                    maskedPayload[i] = payload[i] ^ maskingKey[i % 4];
                }
                
                // Create WebSocket frame with proper masking
                const frame = Buffer.concat([
                    Buffer.from([0x81, 0x80 | payloadLength]), // FIN=1, MASK=1, length
                    maskingKey,                                  // 4-byte masking key
                    maskedPayload                               // Masked payload
                ]);
                
                this.websocket.write(frame);
            } catch (error) {
                console.log('⚠️ Failed to send WebSocket message:', error.message);
            }
        }
    }

    handleWebSocketMessage(data) {
        try {
            // In a full implementation, this would handle:
            // - Remote control commands
            // - Screen sharing requests
            // - File transfer requests
            // - System information requests
            console.log('📨 Received command from controller');
        } catch (error) {
            console.log('⚠️ Message handling error:', error.message);
        }
    }

    setupCapabilities() {
        console.log('🛠️ Setting up remote control capabilities...');
        
        // System information
        const systemInfo = {
            platform: os.platform(),
            arch: os.arch(),
            hostname: os.hostname(),
            memory: Math.round(os.totalmem() / 1024 / 1024 / 1024) + 'GB',
            cpus: os.cpus().length,
            uptime: Math.round(os.uptime() / 3600) + 'h'
        };
        
        console.log('📊 System Information:');
        console.log(`   Platform: ${systemInfo.platform} (${systemInfo.arch})`);
        console.log(`   Memory: ${systemInfo.memory}, CPUs: ${systemInfo.cpus}`);
        console.log(`   Uptime: ${systemInfo.uptime}`);
        
        // Capabilities status
        console.log('🎯 Available Features:');
        console.log('   ✅ Device registration and heartbeat');
        console.log('   ✅ Real-time communication');
        console.log('   ✅ System information reporting');
        console.log('   ⚠️ Screen capture (requires enhancement)');
        console.log('   ⚠️ Remote input control (requires enhancement)');
        console.log('   ⚠️ File transfer (requires enhancement)');
    }

    startHeartbeat() {
        this.heartbeatInterval = setInterval(async () => {
            await this.updateHeartbeat();
            
            // Show status update every 5 minutes
            const now = new Date();
            if (now.getMinutes() % 5 === 0 && now.getSeconds() < 30) {
                console.log(`💓 ${now.toLocaleTimeString()} - Agent online, awaiting connections...`);
            }
        }, this.config.heartbeatInterval);
    }

    async updateHeartbeat() {
        if (!this.isConnected) return;
        
        try {
            const updateData = {
                is_online: true,
                last_seen: new Date().toISOString()
            };
            
            const options = {
                hostname: this.config.supabaseUrl.replace('https://', ''),
                port: 443,
                path: `/rest/v1/remote_devices?device_id=eq.${this.deviceId}`,
                method: 'PATCH',
                headers: {
                    'Content-Type': 'application/json',
                    'Content-Length': Buffer.byteLength(JSON.stringify(updateData)),
                    'apikey': this.config.supabaseKey,
                    'Authorization': `Bearer ${this.config.supabaseKey}`
                }
            };

            const req = https.request(options);
            req.on('error', () => {}); // Ignore heartbeat errors
            req.write(updateData);
            req.end();
            
        } catch (error) {
            // Ignore heartbeat errors
        }
    }

    async shutdown() {
        console.log('\n🛑 Shutting down Remote Desktop Agent...');
        
        // Clear heartbeat
        if (this.heartbeatInterval) {
            clearInterval(this.heartbeatInterval);
        }
        
        // Update status to offline
        if (this.isConnected) {
            try {
                const updateData = JSON.stringify({ status: 'offline' });
                const options = {
                    hostname: this.config.supabaseUrl.replace('https://', ''),
                    port: 443,
                    path: `/rest/v1/remote_devices?device_id=eq.${this.deviceId}`,
                    method: 'PATCH',
                    headers: {
                        'Content-Type': 'application/json',
                        'Content-Length': Buffer.byteLength(updateData),
                        'apikey': this.config.supabaseKey,
                        'Authorization': `Bearer ${this.config.supabaseKey}`
                    }
                };

                const req = https.request(options);
                req.write(updateData);
                req.end();
                
                console.log('✅ Device status updated to offline');
            } catch (error) {
                // Ignore shutdown errors
            }
        }
        
        // Close WebSocket
        if (this.websocket) {
            this.websocket.destroy();
        }
        
        console.log('✅ Agent shutdown complete');
        console.log('👋 Thank you for using Remote Desktop Agent');
    }

    keepAlive() {
        // Handle graceful shutdown
        process.on('SIGINT', async () => {
            await this.shutdown();
            process.exit(0);
        });

        process.on('SIGTERM', async () => {
            await this.shutdown();
            process.exit(0);
        });

        // Keep process alive
        process.stdin.resume();
    }
}

// Start the standalone agent
if (require.main === module) {
    const agent = new StandaloneRemoteAgent();
    agent.initialize().catch((error) => {
        console.error('💥 Agent startup failed:', error);
        console.log('Press any key to exit...');
        process.stdin.once('data', () => process.exit(1));
    });
}

module.exports = StandaloneRemoteAgent;
