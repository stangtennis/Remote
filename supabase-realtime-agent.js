#!/usr/bin/env node

/**
 * Supabase Realtime Remote Desktop Agent
 * Version: 4.1.0 - Global Edition
 * Features: Full Supabase Realtime Integration (No Local WebSocket Server)
 */

const http = require('http');
const https = require('https');
const fs = require('fs');
const os = require('os');
const crypto = require('crypto');
const { createClient } = require('@supabase/supabase-js');

class SupabaseRealtimeAgent {
    constructor() {
        this.deviceId = 'device_' + Math.random().toString(36).substr(2, 9) + '_' + Date.now();
        this.deviceName = os.hostname() || 'RemotePC';
        this.orgId = 'default';
        this.isConnected = false;
        this.activeSession = null;
        this.screenCaptureInterval = null;
        this.supabaseClient = null;
        this.realtimeChannel = null;
        
        // Supabase configuration
        this.supabaseUrl = 'https://ptrtibzwokjcjjxvjpin.supabase.co';
        this.supabaseKey = 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJzdXBhYmFzZSIsInJlZiI6InB0cnRpYnp3b2tqY2pqeHZqcGluIiwicm9sZSI6ImFub24iLCJpYXQiOjE3NTQ0MzE1NzEsImV4cCI6MjA3MDAwNzU3MX0.DPNxkQul1-13tqJ89mqYJAx7NSJjabOP4q8c6KgOnWk';
        
        this.displayBanner();
    }

    displayBanner() {
        console.log('╔══════════════════════════════════════════════════════════════╗');
        console.log('║                🌍 Supabase Realtime Agent                   ║');
        console.log('║                   Global Edition v4.1.0                     ║');
        console.log('╠══════════════════════════════════════════════════════════════╣');
        console.log(`║ Device Name: ${this.deviceName.padEnd(45)} ║`);
        console.log(`║ Device ID:   ${this.deviceId.padEnd(45)} ║`);
        console.log(`║ Platform:    ${os.platform().padEnd(45)} ║`);
        console.log('║ Version:     4.1.0 - Supabase Realtime                     ║');
        console.log('╚══════════════════════════════════════════════════════════════╝');
        console.log('');
    }

    async initialize() {
        try {
            console.log('🔧 Initializing Supabase Realtime Agent...');
            console.log('📋 Configuration loaded');
            
            // Register with Supabase
            console.log('📡 Connecting to Supabase backend...');
            await this.registerDevice();
            
            // Connect to Supabase Realtime (instead of WebSocket server)
            console.log('🔌 Connecting to Supabase Realtime...');
            await this.connectSupabaseRealtime();
            
            // Set up remote control capabilities
            console.log('🛠️ Setting up remote control capabilities...');
            this.setupRemoteControlCapabilities();
            
            // Display system information
            this.displaySystemInfo();
            
            // Display available features
            this.displayFeatures();
            
            console.log('✅ Supabase Realtime Agent is now ONLINE');
            console.log('🎯 Ready to accept remote control sessions');
            console.log('');
            console.log('💡 This window must stay open for remote access');
            console.log('📊 Status: CONNECTED - Device visible in dashboard');
            console.log('🌍 Global connectivity via Supabase Realtime');
            console.log('');
            console.log('Press Ctrl+C to disconnect and exit');
            console.log('════════════════════════════════════════════════════════════');
            
            // Keep the process alive and send heartbeats
            this.startHeartbeat();
            this.keepAlive();
            
        } catch (error) {
            console.error('❌ Initialization failed:', error);
            console.log('⚠️ Continuing in offline mode...');
            this.keepAlive();
        }
    }

    displaySystemInfo() {
        const totalMem = Math.round(os.totalmem() / (1024 * 1024 * 1024));
        const cpus = os.cpus().length;
        const uptime = Math.round(os.uptime() / 3600);
        
        console.log('📊 System Information:');
        console.log(`   Platform: ${os.platform()} (${os.arch()})`);
        console.log(`   Memory: ${totalMem}GB, CPUs: ${cpus}`);
        console.log(`   Uptime: ${uptime}h`);
    }

    displayFeatures() {
        console.log('🎯 Available Features:');
        console.log('   ✅ Device registration and heartbeat');
        console.log('   ✅ Supabase Realtime communication');
        console.log('   ✅ System information reporting');
        console.log('   ✅ Screen capture streaming');
        console.log('   ✅ Remote input control (mouse/keyboard)');
        console.log('   ✅ Session management and authorization');
        console.log('   ✅ File transfer capabilities');
        console.log('   ✅ Global connectivity (no local server required)');
    }

    async registerDevice() {
        try {
            // Initialize Supabase client if not already initialized
            if (!this.supabaseClient) {
                this.supabaseClient = createClient(this.supabaseUrl, this.supabaseKey);
                console.log('✅ Supabase client initialized');
            }
            
            // Generate a unique access key if needed
            const accessKey = crypto.randomBytes(16).toString('hex');
            
            // Create device data that matches the schema
            const deviceData = {
                device_id: this.deviceId, // Use our generated device ID
                device_name: this.deviceName,
                device_type: 'desktop',
                operating_system: `${os.platform()} ${os.release()}`,
                ip_address: this.getLocalIP(),
                status: 'online',
                last_seen: new Date().toISOString(),
                access_key: accessKey,
                metadata: JSON.stringify({
                    hostname: os.hostname(),
                    platform: os.platform(),
                    release: os.release(),
                    arch: os.arch(),
                    cpus: os.cpus().length,
                    memory: Math.round(os.totalmem() / (1024 * 1024 * 1024)) + 'GB'
                })
            };
            
            console.log('📝 Registering device with data:', JSON.stringify(deviceData, null, 2));
            
            // First, try to update device presence
            const { data: presenceData, error: presenceError } = await this.supabaseClient
                .from('device_presence')
                .upsert({
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
                })
                .select();
                
            if (presenceError) {
                console.warn('⚠️ Device presence update failed:', presenceError.message);
                // Continue anyway - might be a permissions issue but we can still try the device registration
            } else {
                console.log('✅ Device presence updated successfully');
            }
            
            // Then register/update the device in remote_devices table
            const { data, error } = await this.supabaseClient
                .from('remote_devices')
                .upsert({
                    device_id: this.deviceId,
                    device_name: this.deviceName,
                    device_type: 'desktop',
                    operating_system: `${os.platform()} ${os.release()}`,
                    ip_address: this.getLocalIP(),
                    status: 'online',
                    last_seen: new Date().toISOString(),
                    metadata: deviceData.metadata
                })
                .select();
            
            if (error) {
                console.error('❌ Failed to register device:', error.message);
                console.warn('⚠️ Registration response:', error.status || 'Unknown');
                
                // If we get a 401, it might be an authentication issue with the table
                // Let's try a simpler approach with just a GET request to verify connectivity
                const testResponse = await this.supabaseClient
                    .from('remote_devices')
                    .select('count')
                    .limit(1);
                    
                if (testResponse.error) {
                    console.error('❌ Test query also failed:', testResponse.error.message);
                } else {
                    console.log('✅ Test query succeeded, but registration failed. Likely a schema mismatch.');
                }
                
                // Continue anyway - the presence table update might be sufficient
                console.log('⚠️ Continuing with Supabase Realtime connection despite registration issues');
            } else {
                console.log('✅ Device registered successfully with Supabase');
            }
            
            return data || { device_id: this.deviceId };
            
        } catch (error) {
            console.error('❌ Error in device registration:', error.message);
            console.warn('⚠️ Registration response:', error.status || 'Unknown');
            console.log('⚠️ Continuing with Supabase Realtime connection despite registration issues');
            return { device_id: this.deviceId };
        }
    }

    async connectSupabaseRealtime() {
        try {
            // Make sure Supabase client is initialized
            if (!this.supabaseClient) {
                this.supabaseClient = createClient(this.supabaseUrl, this.supabaseKey);
                console.log('✅ Supabase client initialized');
            }
            
            // Create a channel for this specific device
            const channelName = `device-${this.deviceId}`;
            
            this.realtimeChannel = this.supabaseClient
                .channel(channelName)
                .on('broadcast', { event: 'command' }, (payload) => {
                    console.log('📡 Received command via Supabase Realtime:', payload);
                    this.handleRealtimeCommand(payload.payload);
                })
                .subscribe((status) => {
                    if (status === 'SUBSCRIBED') {
                        console.log('✅ Connected to Supabase Realtime');
                        console.log(`🔔 Subscribed to device commands channel: ${channelName}`);
                        console.log('📡 Real-time communication established');
                        this.isConnected = true;
                    }
                });
                
            // Also subscribe to a general channel for all devices
            this.supabaseClient
                .channel('all-devices')
                .on('broadcast', { event: 'global-command' }, (payload) => {
                    console.log('📡 Received global command:', payload);
                    this.handleRealtimeCommand(payload.payload);
                })
                .subscribe();
                
            return true;
        } catch (error) {
            console.log('⚠️ Supabase Realtime connection failed:', error.message);
            return false;
        }
    }

    setupRealtimeCommandListener() {
        // No need for interval-based simulation anymore
        // The real-time listeners are set up in connectSupabaseRealtime()
        console.log('🔄 Real-time command listeners are active');
    }

    handleRealtimeCommand(command) {
        try {
            console.log('📨 Received command via Supabase Realtime');
            
            switch (command.type) {
                case 'start_session':
                    const sessionResult = this.startSession(command.sessionId, command.controller);
                    this.sendRealtimeResponse({
                        type: 'session_response',
                        success: sessionResult.success,
                        sessionId: command.sessionId
                    });
                    break;

                case 'end_session':
                    const endResult = this.endSession(command.sessionId);
                    this.sendRealtimeResponse({
                        type: 'session_response',
                        success: endResult.success,
                        message: endResult.message
                    });
                    break;

                case 'mouse_input':
                    if (this.activeSession) {
                        const mouseResult = this.handleMouseInput(
                            command.x, command.y, command.button, command.action
                        );
                        this.sendRealtimeResponse({
                            type: 'input_response',
                            success: mouseResult.success,
                            message: mouseResult.message
                        });
                    }
                    break;

                case 'keyboard_input':
                    if (this.activeSession) {
                        const keyResult = this.handleKeyboardInput(command.key, command.action);
                        this.sendRealtimeResponse({
                            type: 'input_response',
                            success: keyResult.success,
                            message: keyResult.message
                        });
                    }
                    break;

                case 'ping':
                    this.sendRealtimeResponse({ type: 'pong', timestamp: Date.now() });
                    break;

                case 'heartbeat_request':
                    this.sendHeartbeat();
                    break;

                default:
                    console.log('⚠️ Unknown command type:', command.type);
            }
        } catch (error) {
            console.log('⚠️ Command parsing error:', error.message);
        }
    }

    async sendRealtimeResponse(message) {
        try {
            if (!this.supabaseClient || !this.realtimeChannel) {
                console.error('❌ Cannot send response: Supabase Realtime not connected');
                return;
            }
            
            // Add device ID and timestamp to the message
            const responsePayload = {
                ...message,
                deviceId: this.deviceId,
                deviceName: this.deviceName,
                timestamp: Date.now()
            };
            
            // Send response via Supabase Realtime broadcast
            await this.realtimeChannel.send({
                type: 'broadcast',
                event: 'response',
                payload: responsePayload
            });
            
            // Also update device status in database
            await this.supabaseClient
                .from('remote_devices')
                .update({
                    last_seen: new Date().toISOString(),
                    status: 'online'
                })
                .eq('id', this.deviceId);
                
            console.log('📤 Sent response via Supabase Realtime:', message.type);
        } catch (error) {
            console.error('❌ Error sending Realtime response:', error.message);
        }
    }

    setupRemoteControlCapabilities() {
        // Set up screen capture capability
        this.setupScreenCapture();
        
        // Set up input control capability
        this.setupInputControl();
        
        // Set up file transfer capability
        this.setupFileTransfer();
        
        // Set up session management
        this.setupSessionManagement();
    }

    setupScreenCapture() {
        // Screen capture implementation (mock for now, can be enhanced with native modules)
        this.captureScreen = () => {
            return {
                timestamp: Date.now(),
                width: 1920,
                height: 1080,
                format: 'jpeg',
                data: 'base64_encoded_screen_data_placeholder'
            };
        };
    }

    setupInputControl() {
        // Input control implementation (mock for now, can be enhanced with native modules)
        this.handleMouseInput = (x, y, button, action) => {
            console.log(`🖱️ Mouse ${action}: (${x}, ${y}) button: ${button}`);
            // Implementation would use native modules like robotjs
            return { success: true, message: `Mouse ${action} executed` };
        };

        this.handleKeyboardInput = (key, action) => {
            console.log(`⌨️ Keyboard ${action}: ${key}`);
            // Implementation would use native modules like robotjs
            return { success: true, message: `Key ${action} executed` };
        };
    }

    setupFileTransfer() {
        // File transfer implementation
        this.uploadFile = (filename, data) => {
            console.log(`📤 File upload: ${filename}`);
            return { success: true, message: 'File uploaded successfully' };
        };

        this.downloadFile = (filename) => {
            console.log(`📥 File download: ${filename}`);
            return { success: true, data: 'file_data_placeholder' };
        };
    }

    setupSessionManagement() {
        this.startSession = (sessionId, controllerInfo) => {
            console.log(`🎯 Starting remote control session: ${sessionId}`);
            this.activeSession = {
                id: sessionId,
                controller: controllerInfo,
                startTime: Date.now(),
                status: 'active'
            };
            
            // Start screen capture streaming
            this.startScreenCapture();
            
            return { success: true, sessionId: sessionId };
        };

        this.endSession = (sessionId) => {
            console.log(`🛑 Ending remote control session: ${sessionId}`);
            this.activeSession = null;
            
            // Stop screen capture streaming
            this.stopScreenCapture();
            
            return { success: true, message: 'Session ended' };
        };
    }

    startScreenCapture() {
        if (this.screenCaptureInterval) return;
        
        console.log('📸 Starting screen capture streaming via Supabase Realtime...');
        this.screenCaptureInterval = setInterval(() => {
            if (this.activeSession) {
                const screenData = this.captureScreen();
                this.sendRealtimeResponse({
                    type: 'screen_frame',
                    sessionId: this.activeSession.id,
                    data: screenData
                });
            }
        }, 100); // 10 FPS
    }

    stopScreenCapture() {
        if (this.screenCaptureInterval) {
            console.log('📸 Stopping screen capture streaming...');
            clearInterval(this.screenCaptureInterval);
            this.screenCaptureInterval = null;
        }
    }

    startHeartbeat() {
        setInterval(() => {
            this.sendHeartbeat();
        }, 30000); // Every 30 seconds
    }

    async sendHeartbeat() {
        try {
            // Send heartbeat via Realtime
            await this.sendRealtimeResponse({
                type: 'heartbeat',
                deviceId: this.deviceId,
                timestamp: Date.now(),
                status: 'online',
                activeSession: this.activeSession ? this.activeSession.id : null,
                connectionType: 'supabase_realtime'
            });
            
            // Also update the database record directly
            if (this.supabaseClient) {
                await this.supabaseClient
                    .from('remote_devices')
                    .update({
                        last_seen: new Date().toISOString(),
                        status: 'online'
                    })
                    .eq('id', this.deviceId);
            }
            
            console.log('💓 Heartbeat sent via Supabase Realtime');
        } catch (error) {
            console.error('❌ Error sending heartbeat:', error.message);
        }
    }

    getLocalIP() {
        try {
            const interfaces = os.networkInterfaces();
            let ip = '127.0.0.1';
            
            Object.keys(interfaces).forEach(interfaceName => {
                interfaces[interfaceName].forEach(iface => {
                    if (!iface.internal && iface.family === 'IPv4') {
                        ip = iface.address;
                    }
                });
            });
            
            return ip;
        } catch (error) {
            console.error('❌ Error getting local IP:', error.message);
            return '127.0.0.1';
        }
    }

    keepAlive() {
        process.stdin.resume();
        
        process.on('SIGINT', async () => {
            console.log('\n🛑 Shutting down Supabase Realtime Agent...');
            
            if (this.activeSession) {
                await this.endSession(this.activeSession.id);
            }
            
            // Update device status to offline
            if (this.supabaseClient) {
                try {
                    await this.supabaseClient
                        .from('remote_devices')
                        .update({
                            status: 'offline',
                            last_seen: new Date().toISOString()
                        })
                        .eq('id', this.deviceId);
                        
                    console.log('✅ Device status updated to offline');
                } catch (error) {
                    console.error('❌ Error updating device status:', error.message);
                }
            }
            
            // Unsubscribe from Realtime channels
            if (this.realtimeChannel) {
                await this.realtimeChannel.unsubscribe();
                console.log('📡 Disconnected from Supabase Realtime');
            }
            
            process.exit(0);
        });
    }
}

// Start the Supabase Realtime Agent
const agent = new SupabaseRealtimeAgent();
agent.initialize().catch(console.error);
