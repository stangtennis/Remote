#!/usr/bin/env node

/**
 * Real Remote Desktop Agent
 * Provides actual screen capture, input control, and real-time communication
 */

const { createClient } = require('@supabase/supabase-js');
const WebSocket = require('ws');
const screenshot = require('screenshot-desktop');
const robot = require('robotjs');
const fs = require('fs');
const path = require('path');
const os = require('os');

class RealRemoteAgent {
    constructor() {
        this.deviceId = this.generateDeviceId();
        this.deviceName = process.env.DEVICE_NAME || os.hostname();
        this.orgId = process.env.ORG_ID || 'default';
        this.isConnected = false;
        this.isStreaming = false;
        this.streamInterval = null;
        this.supabase = null;
        this.websocket = null;
        
        // Supabase configuration
        this.supabaseUrl = 'https://ptrtibzwokjcjjxvjpin.supabase.co';
        this.supabaseKey = 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJzdXBhYmFzZSIsInJlZiI6InB0cnRpYnp3b2tqY2pqeHZqcGluIiwicm9sZSI6ImFub24iLCJpYXQiOjE3MzM1MTI0NTYsImV4cCI6MjA0OTA4ODQ1Nn0.OwJ04BrJ6gZZ57fSeT-q5Q_mSsVaNia';
        
        console.log('🌍 Real Remote Desktop Agent Initializing...');
        console.log(`📱 Device ID: ${this.deviceId}`);
        console.log(`💻 Device Name: ${this.deviceName}`);
        console.log(`🏢 Organization: ${this.orgId}`);
    }

    generateDeviceId() {
        return `device_${Math.random().toString(36).substr(2, 9)}_${Date.now()}`;
    }

    async initialize() {
        try {
            console.log('🔧 Initializing Real Remote Agent...');
            
            // Initialize Supabase client
            await this.initializeSupabase();
            
            // Register device
            await this.registerDevice();
            
            // Start WebSocket connection
            await this.startWebSocketConnection();
            
            // Set up screen capture capabilities
            this.setupScreenCapture();
            
            // Set up input control
            this.setupInputControl();
            
            console.log('✅ Real Remote Agent initialized successfully');
            console.log('🎯 Agent is ready for remote control sessions');
            
            // Keep the process alive
            this.keepAlive();
            
        } catch (error) {
            console.error('❌ Failed to initialize agent:', error);
            process.exit(1);
        }
    }

    async initializeSupabase() {
        try {
            console.log('📡 Connecting to Supabase...');
            this.supabase = createClient(this.supabaseUrl, this.supabaseKey);
            
            // Test connection
            const { data, error } = await this.supabase.from('devices').select('count').limit(1);
            if (error) throw error;
            
            console.log('✅ Supabase connection established');
        } catch (error) {
            console.error('❌ Supabase connection failed:', error);
            throw error;
        }
    }

    async registerDevice() {
        try {
            console.log('📝 Registering device...');
            
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
                    os_version: os.release()
                }
            };

            const { data, error } = await this.supabase
                .from('devices')
                .upsert(deviceInfo, { onConflict: 'device_id' });

            if (error) throw error;

            console.log('✅ Device registered successfully');
            this.isConnected = true;
            
        } catch (error) {
            console.error('❌ Device registration failed:', error);
            throw error;
        }
    }

    async startWebSocketConnection() {
        try {
            console.log('🔌 Starting WebSocket connection...');
            
            // For now, we'll use a local WebSocket server
            // In production, this would connect to Supabase Realtime or a dedicated WebSocket server
            const wsUrl = 'ws://localhost:3001';
            
            this.websocket = new WebSocket(wsUrl);
            
            this.websocket.on('open', () => {
                console.log('✅ WebSocket connected');
                this.sendMessage({
                    type: 'device_connect',
                    deviceId: this.deviceId,
                    deviceName: this.deviceName
                });
            });

            this.websocket.on('message', (data) => {
                this.handleWebSocketMessage(JSON.parse(data.toString()));
            });

            this.websocket.on('error', (error) => {
                console.error('❌ WebSocket error:', error);
            });

            this.websocket.on('close', () => {
                console.log('🔌 WebSocket disconnected, attempting reconnect...');
                setTimeout(() => this.startWebSocketConnection(), 5000);
            });
            
        } catch (error) {
            console.log('⚠️ WebSocket server not available, running in offline mode');
            // Continue without WebSocket for now
        }
    }

    handleWebSocketMessage(message) {
        console.log('📨 Received message:', message.type);
        
        switch (message.type) {
            case 'start_screen_share':
                this.startScreenShare();
                break;
            case 'stop_screen_share':
                this.stopScreenShare();
                break;
            case 'mouse_move':
                this.handleMouseMove(message.data);
                break;
            case 'mouse_click':
                this.handleMouseClick(message.data);
                break;
            case 'key_press':
                this.handleKeyPress(message.data);
                break;
            case 'ping':
                this.sendMessage({ type: 'pong' });
                break;
        }
    }

    setupScreenCapture() {
        console.log('📸 Setting up screen capture...');
        
        // Test screen capture
        this.captureScreen().then(() => {
            console.log('✅ Screen capture ready');
        }).catch((error) => {
            console.error('❌ Screen capture setup failed:', error);
        });
    }

    async captureScreen() {
        try {
            const img = await screenshot({ format: 'png' });
            return img;
        } catch (error) {
            console.error('❌ Screen capture failed:', error);
            throw error;
        }
    }

    startScreenShare() {
        if (this.isStreaming) return;
        
        console.log('🎥 Starting screen share...');
        this.isStreaming = true;
        
        this.streamInterval = setInterval(async () => {
            try {
                const screenshot = await this.captureScreen();
                const base64 = screenshot.toString('base64');
                
                this.sendMessage({
                    type: 'screen_frame',
                    data: base64,
                    timestamp: Date.now()
                });
                
            } catch (error) {
                console.error('❌ Screen capture error:', error);
            }
        }, 1000 / 10); // 10 FPS
    }

    stopScreenShare() {
        if (!this.isStreaming) return;
        
        console.log('⏹️ Stopping screen share...');
        this.isStreaming = false;
        
        if (this.streamInterval) {
            clearInterval(this.streamInterval);
            this.streamInterval = null;
        }
    }

    setupInputControl() {
        console.log('🖱️ Setting up input control...');
        
        // Test robot.js
        try {
            const mousePos = robot.getMousePos();
            console.log(`✅ Input control ready (mouse at ${mousePos.x}, ${mousePos.y})`);
        } catch (error) {
            console.error('❌ Input control setup failed:', error);
        }
    }

    handleMouseMove(data) {
        try {
            robot.moveMouse(data.x, data.y);
        } catch (error) {
            console.error('❌ Mouse move failed:', error);
        }
    }

    handleMouseClick(data) {
        try {
            robot.mouseClick(data.button || 'left', data.double || false);
        } catch (error) {
            console.error('❌ Mouse click failed:', error);
        }
    }

    handleKeyPress(data) {
        try {
            if (data.key) {
                robot.keyTap(data.key, data.modifiers || []);
            } else if (data.text) {
                robot.typeString(data.text);
            }
        } catch (error) {
            console.error('❌ Key press failed:', error);
        }
    }

    sendMessage(message) {
        if (this.websocket && this.websocket.readyState === WebSocket.OPEN) {
            this.websocket.send(JSON.stringify(message));
        }
    }

    async updateHeartbeat() {
        try {
            await this.supabase
                .from('devices')
                .update({ 
                    last_seen: new Date().toISOString(),
                    status: 'online'
                })
                .eq('device_id', this.deviceId);
        } catch (error) {
            console.error('❌ Heartbeat update failed:', error);
        }
    }

    keepAlive() {
        // Update heartbeat every 30 seconds
        setInterval(() => {
            this.updateHeartbeat();
        }, 30000);

        // Handle graceful shutdown
        process.on('SIGINT', async () => {
            console.log('\n🛑 Shutting down agent...');
            
            try {
                await this.supabase
                    .from('devices')
                    .update({ status: 'offline' })
                    .eq('device_id', this.deviceId);
            } catch (error) {
                console.error('❌ Shutdown update failed:', error);
            }
            
            if (this.websocket) {
                this.websocket.close();
            }
            
            console.log('✅ Agent shutdown complete');
            process.exit(0);
        });

        console.log('💓 Agent heartbeat started');
        console.log('🔄 Agent running... (Press Ctrl+C to stop)');
    }
}

// Start the agent
if (require.main === module) {
    const agent = new RealRemoteAgent();
    agent.initialize().catch((error) => {
        console.error('💥 Agent startup failed:', error);
        process.exit(1);
    });
}

module.exports = RealRemoteAgent;
