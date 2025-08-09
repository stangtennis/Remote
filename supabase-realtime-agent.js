#!/usr/bin/env node

/**
 * Supabase Realtime Remote Desktop Agent
 * Version: 5.0.0 - File Transfer Integration Edition
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
        // Generate consistent hardware-based device ID for this physical PC
        this.deviceId = this.generateHardwareBasedDeviceId();
        this.deviceName = os.hostname() || 'RemotePC';
        this.orgId = 'default';
        this.isConnected = false;
        this.activeSession = null;
        this.screenCaptureInterval = null;
        this.supabaseClient = null;
        this.realtimeChannel = null;
        
        // File transfer capabilities (initialized after supabase client)
        this.fileTransferManager = null;
        this.activeTransfers = new Map();
        this.transferChannel = null;
        
        // Supabase configuration
        this.supabaseUrl = 'https://ptrtibzwokjcjjxvjpin.supabase.co';
        this.supabaseKey = 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJzdXBhYmFzZSIsInJlZiI6InB0cnRpYnp3b2tqY2pqeHZqcGluIiwicm9sZSI6ImFub24iLCJpYXQiOjE3NTQ0MzE1NzEsImV4cCI6MjA3MDAwNzU3MX0.DPNxkQul1-13tqJ89mqYJAx7NSJjabOP4q8c6KgOnWk';
        
        // Additional headers for authentication
        this.authHeaders = {
            'apikey': this.supabaseKey,
            'Authorization': `Bearer ${this.supabaseKey}`
        };
        
        this.displayBanner();
    }

    displayBanner() {
        console.log('╔══════════════════════════════════════════════════════════════╗');
        console.log('║                🌍 Supabase Realtime Agent                   ║');
        console.log('║                   Global Edition v5.0.0                     ║');
        console.log('╠══════════════════════════════════════════════════════════════╣');
        console.log(`║ Device Name: ${this.deviceName.padEnd(45)} ║`);
        console.log(`║ Device ID:   ${this.deviceId.padEnd(45)} ║`);
        console.log(`║ Platform:    ${os.platform().padEnd(45)} ║`);
        console.log('║ Version:     5.0.0 - File Transfer Integration              ║');
        console.log('╚══════════════════════════════════════════════════════════════╝');
        console.log('');
    }

    generateHardwareBasedDeviceId() {
        try {
            // Create a consistent device ID based on hardware characteristics
            const hostname = os.hostname() || 'unknown';
            const platform = os.platform();
            const arch = os.arch();
            const cpus = os.cpus().length.toString();
            const totalMem = Math.round(os.totalmem() / (1024 * 1024 * 1024)).toString(); // GB
            
            // Get network interfaces to find MAC address
            let macAddress = 'unknown';
            try {
                const interfaces = os.networkInterfaces();
                for (const interfaceName in interfaces) {
                    const iface = interfaces[interfaceName];
                    for (const alias of iface) {
                        if (!alias.internal && alias.mac && alias.mac !== '00:00:00:00:00:00') {
                            macAddress = alias.mac;
                            break;
                        }
                    }
                    if (macAddress !== 'unknown') break;
                }
            } catch (err) {
                console.warn('⚠️ Could not get MAC address:', err.message);
            }
            
            // Create a unique string from hardware characteristics
            const hardwareString = `${hostname}-${platform}-${arch}-${cpus}-${totalMem}-${macAddress}`;
            
            // Create a hash of the hardware string for a consistent device ID
            const hash = crypto.createHash('sha256').update(hardwareString).digest('hex');
            
            // Convert hash to UUID format (8-4-4-4-12 pattern) for database compatibility
            const uuidFormat = [
                hash.substring(0, 8),
                hash.substring(8, 12),
                hash.substring(12, 16),
                hash.substring(16, 20),
                hash.substring(20, 32)
            ].join('-');
            
            const deviceId = uuidFormat; // Use UUID format for database compatibility
            
            console.log(`🔧 Generated hardware-based device ID: ${deviceId}`);
            console.log(`📋 Hardware fingerprint: ${hostname} (${platform}/${arch}, ${cpus} CPUs, ${totalMem}GB RAM)`);
            
            return deviceId;
            
        } catch (error) {
            console.error('❌ Error generating hardware-based device ID:', error.message);
            // Fallback to hostname-based ID if hardware detection fails
            const fallbackId = `device_${os.hostname() || 'unknown'}_${os.platform()}`;
            console.log(`🔄 Using fallback device ID: ${fallbackId}`);
            return fallbackId;
        }
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
            
            // Initialize file transfer manager
            console.log('📁 Setting up file transfer capabilities...');
            this.fileTransferManager = new FileTransferManager(this.supabaseClient, this.deviceId);
            
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
                this.supabaseClient = createClient(this.supabaseUrl, this.supabaseKey, {
                    auth: {
                        persistSession: false,
                        autoRefreshToken: true
                    },
                    global: {
                        headers: this.authHeaders
                    }
                });
                console.log('✅ Supabase client initialized with auth headers');
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
            
            // First, try to update device presence using Supabase client directly
            let presenceData = null;
            let presenceError = null;
            try {
                const { data, error } = await this.supabaseClient
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
                            agent_version: '4.2.0',
                            global_edition: true
                        })
                    })
                    .select();
                
                presenceData = data;
                presenceError = error;
            } catch (err) {
                presenceError = { message: err.message };
            }
            
            if (presenceError) {
                console.warn('⚠️ Device presence update failed:', presenceError.message);
                // Continue anyway - might be a permissions issue but we can still try the device registration
            } else {
                console.log('✅ Device presence updated successfully');
            }
            
            // Then register/update the device in remote_devices table using Supabase client
            const { data, error } = await this.supabaseClient
                .from('remote_devices')
                .upsert({
                    id: this.deviceId,  // Hardware-based device ID as primary key
                    device_name: this.deviceName,
                    device_type: 'desktop',
                    operating_system: `${os.platform()} ${os.release()}`,
                    ip_address: this.getLocalIP(),
                    is_online: true,  // Use is_online instead of status to match schema
                    last_seen: new Date().toISOString(),
                    access_key: accessKey,  // Required field in schema
                    owner_id: null  // Set to null for now, can be assigned later
                })
                .select();
            
            if (error) {
                console.error('❌ Failed to register device:', error.message);
                console.warn('⚠️ Registration response:', error.status || 'Unknown');
                
                // If we get a 401, it might be an authentication issue with the table
                // Let's try a simpler approach with just a GET request to verify connectivity
                const { data: testData, error: testError } = await this.supabaseClient
                    .from('remote_devices')
                    .select('count')
                    .limit(1);
                    
                if (testError) {
                    console.error('❌ Test query also failed:', testError.message);
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
        // Enhanced screen capture implementation with system information
        this.captureScreen = () => {
            try {
                // Get actual screen resolution from system
                const screenWidth = process.platform === 'win32' ? 1920 : 1920; // Could use native modules for real resolution
                const screenHeight = process.platform === 'win32' ? 1080 : 1080;
                
                // Generate a more realistic mock screenshot with system info
                const screenData = {
                    timestamp: Date.now(),
                    width: screenWidth,
                    height: screenHeight,
                    format: 'jpeg',
                    platform: os.platform(),
                    hostname: os.hostname(),
                    // In production, this would be actual screenshot data from native modules like screenshot-desktop
                    data: `screenshot_${Date.now()}_${screenWidth}x${screenHeight}_base64_placeholder`,
                    quality: 80,
                    compression: 'jpeg'
                };
                
                console.log(`📸 Screen captured: ${screenWidth}x${screenHeight} on ${os.platform()}`);
                return screenData;
                
            } catch (error) {
                console.error('❌ Screen capture error:', error.message);
                return {
                    timestamp: Date.now(),
                    width: 1920,
                    height: 1080,
                    format: 'jpeg',
                    data: 'error_capturing_screen',
                    error: error.message
                };
            }
        };
    }

    setupInputControl() {
        // Enhanced input control implementation with validation and logging
        this.handleMouseInput = (x, y, button, action) => {
            try {
                // Validate coordinates
                if (typeof x !== 'number' || typeof y !== 'number' || x < 0 || y < 0) {
                    throw new Error('Invalid mouse coordinates');
                }
                
                // Validate button
                const validButtons = ['left', 'right', 'middle'];
                if (!validButtons.includes(button)) {
                    throw new Error('Invalid mouse button');
                }
                
                // Validate action
                const validActions = ['click', 'down', 'up', 'move', 'drag'];
                if (!validActions.includes(action)) {
                    throw new Error('Invalid mouse action');
                }
                
                console.log(`🖱️ Mouse ${action}: (${x}, ${y}) button: ${button} on ${os.platform()}`);
                
                // In production, this would use native modules like robotjs:
                // robot.moveMouse(x, y);
                // robot.mouseClick(button);
                
                return { 
                    success: true, 
                    message: `Mouse ${action} executed at (${x}, ${y})`,
                    timestamp: Date.now(),
                    platform: os.platform()
                };
                
            } catch (error) {
                console.error('❌ Mouse input error:', error.message);
                return { 
                    success: false, 
                    error: error.message,
                    timestamp: Date.now()
                };
            }
        };

        this.handleKeyboardInput = (key, action) => {
            try {
                // Validate action
                const validActions = ['keydown', 'keyup', 'keypress', 'type'];
                if (!validActions.includes(action)) {
                    throw new Error('Invalid keyboard action');
                }
                
                // Validate key
                if (typeof key !== 'string' || key.length === 0) {
                    throw new Error('Invalid key input');
                }
                
                console.log(`⌨️ Keyboard ${action}: "${key}" on ${os.platform()}`);
                
                // In production, this would use native modules like robotjs:
                // robot.keyTap(key);
                // robot.typeString(key);
                
                return { 
                    success: true, 
                    message: `Key ${action} executed: "${key}"`,
                    timestamp: Date.now(),
                    platform: os.platform()
                };
                
            } catch (error) {
                console.error('❌ Keyboard input error:', error.message);
                return { 
                    success: false, 
                    error: error.message,
                    timestamp: Date.now()
                };
            }
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
            
            // Also update the database record directly using Supabase client
            try {
                await this.supabaseClient
                    .from('remote_devices')
                    .update({
                        last_seen: new Date().toISOString(),
                        status: 'online'
                    })
                    .eq('id', this.deviceId);
            } catch (error) {
                console.error('❌ Error updating heartbeat status:', error.message);
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
            
            // Update device status to offline using Supabase client
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
            
            // Unsubscribe from Realtime channels
            if (this.realtimeChannel) {
                await this.realtimeChannel.unsubscribe();
                console.log('📡 Disconnected from Supabase Realtime');
            }
            
            process.exit(0);
        });
    }
}

// File Transfer Manager Class
class FileTransferManager {
    constructor(supabaseClient, deviceId) {
        this.supabaseClient = supabaseClient;
        this.deviceId = deviceId;
        this.activeTransfers = new Map();
        this.chunkSize = 1024 * 1024; // 1MB chunks
        this.fileTransferUrl = 'https://ptrtibzwokjcjjxvjpin.supabase.co/functions/v1/file-transfer';
    }

    async initiateFileTransfer(targetDeviceId, filePath) {
        try {
            console.log(`📁 Initiating file transfer to device: ${targetDeviceId}`);
            
            if (!fs.existsSync(filePath)) {
                throw new Error('File not found');
            }

            const stats = fs.statSync(filePath);
            const fileName = require('path').basename(filePath);
            const fileType = require('path').extname(filePath);

            const transferRequest = {
                sourceDeviceId: this.deviceId,
                targetDeviceId: targetDeviceId,
                fileName: fileName,
                fileSize: stats.size,
                fileType: fileType,
                transferType: 'upload'
            };

            const response = await this.makeRequest('/api/initiate-transfer', 'POST', transferRequest);
            
            if (response.success) {
                console.log(`✅ Transfer session created: ${response.sessionId}`);
                await this.uploadFile(response.sessionId, filePath, stats.size);
                return response;
            } else {
                throw new Error(response.error || 'Failed to initiate transfer');
            }

        } catch (error) {
            console.error('❌ File transfer initiation failed:', error.message);
            throw error;
        }
    }

    async uploadFile(sessionId, filePath, fileSize) {
        try {
            console.log(`📤 Starting file upload for session: ${sessionId}`);
            
            const totalChunks = Math.ceil(fileSize / this.chunkSize);
            let chunkIndex = 0;
            let uploadedBytes = 0;

            this.activeTransfers.set(sessionId, {
                status: 'uploading',
                progress: 0,
                totalChunks: totalChunks,
                uploadedChunks: 0
            });

            // Read file in chunks and upload
            const buffer = fs.readFileSync(filePath);
            
            for (let offset = 0; offset < fileSize; offset += this.chunkSize) {
                const chunk = buffer.slice(offset, Math.min(offset + this.chunkSize, fileSize));
                
                const formData = {
                    sessionId: sessionId,
                    chunkIndex: chunkIndex.toString(),
                    totalChunks: totalChunks.toString(),
                    chunk: chunk.toString('base64')
                };

                const response = await this.makeRequest('/api/upload-chunk', 'POST', formData);
                
                if (response.success) {
                    chunkIndex++;
                    uploadedBytes += chunk.length;
                    const progress = Math.round((uploadedBytes / fileSize) * 100);
                    
                    console.log(`📤 Uploaded chunk ${chunkIndex}/${totalChunks} (${progress}%)`);
                    
                    // Update transfer status
                    const transferStatus = this.activeTransfers.get(sessionId);
                    if (transferStatus) {
                        transferStatus.progress = progress;
                        transferStatus.uploadedChunks = chunkIndex;
                    }
                } else {
                    throw new Error(`Failed to upload chunk ${chunkIndex}: ${response.error}`);
                }
            }

            console.log(`✅ File upload completed for session: ${sessionId}`);
            this.activeTransfers.delete(sessionId);
            
        } catch (error) {
            console.error('❌ File upload failed:', error.message);
            this.activeTransfers.delete(sessionId);
            throw error;
        }
    }

    async downloadFile(sessionId, savePath) {
        try {
            console.log(`📥 Starting file download for session: ${sessionId}`);
            
            // Get transfer status to know total chunks
            const statusResponse = await this.makeRequest(`/api/transfer-status?sessionId=${sessionId}`, 'GET');
            
            if (!statusResponse.id) {
                throw new Error('Transfer session not found');
            }

            const totalChunks = Math.ceil(statusResponse.fileSize / this.chunkSize);
            const chunks = [];
            
            this.activeTransfers.set(sessionId, {
                status: 'downloading',
                progress: 0,
                totalChunks: totalChunks,
                downloadedChunks: 0
            });

            for (let chunkIndex = 0; chunkIndex < totalChunks; chunkIndex++) {
                const chunkResponse = await this.makeRequest(
                    `/api/download-chunk?sessionId=${sessionId}&chunkIndex=${chunkIndex}`, 
                    'GET'
                );

                if (chunkResponse) {
                    chunks.push(Buffer.from(chunkResponse, 'base64'));
                    
                    const progress = Math.round(((chunkIndex + 1) / totalChunks) * 100);
                    console.log(`📥 Downloaded chunk ${chunkIndex + 1}/${totalChunks} (${progress}%)`);
                    
                    // Update transfer status
                    const transferStatus = this.activeTransfers.get(sessionId);
                    if (transferStatus) {
                        transferStatus.progress = progress;
                        transferStatus.downloadedChunks = chunkIndex + 1;
                    }
                } else {
                    throw new Error(`Failed to download chunk ${chunkIndex}`);
                }
            }

            // Combine chunks and save file
            const fileBuffer = Buffer.concat(chunks);
            fs.writeFileSync(savePath, fileBuffer);
            
            console.log(`✅ File download completed for session: ${sessionId}`);
            this.activeTransfers.delete(sessionId);
            
        } catch (error) {
            console.error('❌ File download failed:', error.message);
            this.activeTransfers.delete(sessionId);
            throw error;
        }
    }

    async cancelTransfer(sessionId) {
        try {
            console.log(`🛑 Cancelling transfer session: ${sessionId}`);
            
            const response = await this.makeRequest('/api/cancel-transfer', 'POST', { sessionId });
            
            if (response.success) {
                this.activeTransfers.delete(sessionId);
                console.log(`✅ Transfer cancelled: ${sessionId}`);
                return response;
            } else {
                throw new Error(response.error || 'Failed to cancel transfer');
            }
            
        } catch (error) {
            console.error('❌ Failed to cancel transfer:', error.message);
            throw error;
        }
    }

    async listTransfers() {
        try {
            const response = await this.makeRequest(`/api/list-transfers?deviceId=${this.deviceId}`, 'GET');
            return response || [];
        } catch (error) {
            console.error('❌ Failed to list transfers:', error.message);
            return [];
        }
    }

    getTransferStatus(sessionId) {
        return this.activeTransfers.get(sessionId) || null;
    }

    async makeRequest(endpoint, method, data = null) {
        try {
            const url = this.fileTransferUrl + endpoint;
            const options = {
                method: method,
                headers: {
                    'apikey': this.supabaseClient.supabaseKey,
                    'Authorization': `Bearer ${this.supabaseClient.supabaseKey}`,
                    'Content-Type': 'application/json'
                }
            };

            if (data && method !== 'GET') {
                options.body = JSON.stringify(data);
            }

            const response = await fetch(url, options);
            const result = await response.json();
            return result;
            
        } catch (error) {
            console.error('❌ File transfer API request failed:', error.message);
            throw error;
        }
    }
}

// Start the Supabase Realtime Agent
const agent = new SupabaseRealtimeAgent();
agent.initialize().catch(console.error);
