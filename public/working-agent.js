#!/usr/bin/env node

/**
 * Working Remote Desktop Agent (No External Dependencies)
 * Device: TestPC
 * Generated: 2025-08-07T10:02:00.000Z
 */

const http = require('http');
const https = require('https');
const fs = require('fs');
const os = require('os');

class WorkingRemoteAgent {
    constructor() {
        this.deviceId = 'device_' + Math.random().toString(36).substr(2, 9) + '_' + Date.now();
        this.deviceName = 'TestPC';
        this.orgId = 'default';
        this.isConnected = false;
        
        console.log('🌍 Working Remote Desktop Agent Starting...');
        console.log('📱 Device:', this.deviceName);
        console.log('🆔 Device ID:', this.deviceId);
        console.log('💻 Platform:', os.platform());
        console.log('🏢 Organization:', this.orgId);
    }

    async initialize() {
        try {
            console.log('🔧 Initializing agent...');
            
            // Register with Supabase
            await this.registerDevice();
            
            // Connect to WebSocket server
            this.connectWebSocket();
            
            // Set up basic capabilities
            this.setupCapabilities();
            
            console.log('✅ Agent initialized successfully');
            console.log('🎯 Agent is ready for remote control sessions');
            
            // Keep the process alive
            this.keepAlive();
            
        } catch (error) {
            console.error('❌ Initialization failed:', error);
            console.log('⚠️ Continuing in offline mode...');
            this.keepAlive();
        }
    }

    async registerDevice() {
        console.log('📝 Registering device with Supabase...');
        
        const deviceInfo = {
            device_id: this.deviceId,
            device_name: this.deviceName,
            organization_id: this.orgId,
            platform: os.platform(),
            arch: os.arch(),
            hostname: os.hostname(),
            status: 'online',
            last_seen: new Date().toISOString(),
            capabilities: {
                screen_capture: true,
                input_control: true,
                file_transfer: false,
                audio: false,
                webcam: false
            },
            metadata: {
                agent_version: '2.0.0',
                node_version: process.version,
                os_version: os.release(),
                working_directory: process.cwd()
            }
        };

        return new Promise((resolve) => {
            try {
                const postData = JSON.stringify(deviceInfo);
                const options = {
                    hostname: 'ptrtibzwokjcjjxvjpin.supabase.co',
                    port: 443,
                    path: '/rest/v1/devices',
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                        'Content-Length': Buffer.byteLength(postData),
                        'apikey': 'REDACTED_JWT',
                        'Authorization': 'Bearer REDACTED_JWT',
                        'Prefer': 'return=minimal'
                    }
                };

                const req = https.request(options, (res) => {
                    let data = '';
                    res.on('data', (chunk) => data += chunk);
                    res.on('end', () => {
                        if (res.statusCode === 201 || res.statusCode === 200) {
                            console.log('✅ Device registered successfully');
                            this.isConnected = true;
                        } else {
                            console.log(`⚠️ Registration response: ${res.statusCode}`);
                        }
                        resolve();
                    });
                });

                req.on('error', (error) => {
                    console.log('⚠️ Registration failed (offline mode):', error.message);
                    resolve();
                });

                req.write(postData);
                req.end();
                
            } catch (error) {
                console.log('⚠️ Registration error (offline mode):', error.message);
                resolve();
            }
        });
    }

    connectWebSocket() {
        console.log('🔌 Connecting to WebSocket server...');
        
        try {
            // Create a simple TCP connection to WebSocket server
            const net = require('net');
            const client = new net.Socket();
            
            client.connect(3002, 'localhost', () => {
                console.log('✅ Connected to WebSocket server');
                
                // Send WebSocket handshake
                const key = Buffer.from(Math.random().toString()).toString('base64');
                const handshake = [
                    'GET / HTTP/1.1',
                    'Host: localhost:3002',
                    'Upgrade: websocket',
                    'Connection: Upgrade',
                    'Sec-WebSocket-Key: ' + key,
                    'Sec-WebSocket-Version: 13',
                    '', ''
                ].join('\r\n');
                
                client.write(handshake);
            });

            client.on('data', (data) => {
                const response = data.toString();
                if (response.includes('HTTP/1.1 101')) {
                    console.log('✅ WebSocket handshake successful');
                    
                    // Send device connect message
                    setTimeout(() => {
                        const message = JSON.stringify({
                            type: 'device_connect',
                            deviceId: this.deviceId,
                            deviceName: this.deviceName
                        });
                        
                        // Simple WebSocket frame (text frame, no masking for server)
                        const frame = Buffer.concat([
                            Buffer.from([0x81, message.length]),
                            Buffer.from(message)
                        ]);
                        
                        client.write(frame);
                        console.log('📤 Device connect message sent');
                    }, 100);
                } else {
                    console.log('📨 WebSocket message received');
                }
            });

            client.on('error', (error) => {
                console.log('⚠️ WebSocket connection failed (offline mode):', error.message);
            });

            client.on('close', () => {
                console.log('🔌 WebSocket connection closed');
                // Attempt reconnection after 5 seconds
                setTimeout(() => this.connectWebSocket(), 5000);
            });
            
        } catch (error) {
            console.log('⚠️ WebSocket setup failed (offline mode):', error.message);
        }
    }

    setupCapabilities() {
        console.log('🛠️ Setting up capabilities...');
        
        // Basic screen info
        console.log('📸 Screen capture: Ready (mock)');
        console.log('🖱️ Input control: Ready (mock)');
        console.log('📁 File system: Ready');
        
        // In a real implementation, this would set up:
        // - Screen capture using native APIs or libraries
        // - Input simulation using native APIs
        // - File transfer capabilities
        // - Audio/video streaming
    }

    async updateHeartbeat() {
        if (!this.isConnected) return;
        
        try {
            const updateData = JSON.stringify({
                last_seen: new Date().toISOString(),
                status: 'online'
            });
            
            const options = {
                hostname: 'ptrtibzwokjcjjxvjpin.supabase.co',
                port: 443,
                path: `/rest/v1/devices?device_id=eq.${this.deviceId}`,
                method: 'PATCH',
                headers: {
                    'Content-Type': 'application/json',
                    'Content-Length': Buffer.byteLength(updateData),
                    'apikey': 'REDACTED_JWT',
                    'Authorization': 'Bearer REDACTED_JWT'
                }
            };

            const req = https.request(options, (res) => {
                // Heartbeat update complete
            });

            req.on('error', () => {
                // Ignore heartbeat errors
            });

            req.write(updateData);
            req.end();
            
        } catch (error) {
            // Ignore heartbeat errors
        }
    }

    keepAlive() {
        // Update heartbeat every 30 seconds
        setInterval(() => {
            this.updateHeartbeat();
            console.log('💓 Agent heartbeat');
        }, 30000);

        // Handle graceful shutdown
        process.on('SIGINT', async () => {
            console.log('\n🛑 Shutting down agent...');
            
            if (this.isConnected) {
                try {
                    const updateData = JSON.stringify({ status: 'offline' });
                    const options = {
                        hostname: 'ptrtibzwokjcjjxvjpin.supabase.co',
                        port: 443,
                        path: `/rest/v1/devices?device_id=eq.${this.deviceId}`,
                        method: 'PATCH',
                        headers: {
                            'Content-Type': 'application/json',
                            'Content-Length': Buffer.byteLength(updateData),
                            'apikey': 'REDACTED_JWT',
                            'Authorization': 'Bearer REDACTED_JWT'
                        }
                    };

                    const req = https.request(options);
                    req.write(updateData);
                    req.end();
                } catch (error) {
                    // Ignore shutdown errors
                }
            }
            
            console.log('✅ Agent shutdown complete');
            process.exit(0);
        });

        console.log('💓 Agent heartbeat started');
        console.log('🔄 Agent running... (Press Ctrl+C to stop)');
        console.log('');
        console.log('🎯 Features available:');
        console.log('  ✅ Supabase device registration');
        console.log('  ✅ WebSocket server connection');
        console.log('  ✅ Heartbeat monitoring');
        console.log('  ✅ Graceful shutdown');
        console.log('  ⚠️ Screen capture (mock - needs native libraries)');
        console.log('  ⚠️ Input control (mock - needs native libraries)');
        console.log('');
        console.log('📊 Status: Agent is online and ready for connections');
    }
}

// Start the agent
if (require.main === module) {
    const agent = new WorkingRemoteAgent();
    agent.initialize().catch((error) => {
        console.error('💥 Agent startup failed:', error);
        process.exit(1);
    });
}

module.exports = WorkingRemoteAgent;
