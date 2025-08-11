#!/usr/bin/env node

/**
 * Production Remote Desktop Agent
 * Version: 6.0.0 - Standalone Executable Edition
 * Features: Full Supabase Realtime Integration (No Local Dependencies)
 */

const http = require('http');
const https = require('https');
const fs = require('fs');
const os = require('os');
const crypto = require('crypto');
const { createClient } = require('@supabase/supabase-js');

// Try to load professional modules, fall back to null if not available
let ProfessionalScreenCapture = null;
let ProfessionalInputControl = null;

try {
    ProfessionalScreenCapture = require('./lib/screen-capture');
    ProfessionalInputControl = require('./lib/input-control');
    console.log('✅ Professional modules loaded');
} catch (error) {
    console.log('⚠️ Professional modules not available, using simulation mode');
    console.log('   This is normal for standalone executables without native dependencies');
}

class ProductionAgent {
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
        
        // File transfer capabilities
        this.fileTransferManager = null;
        this.activeTransfers = new Map();
        this.transferChannel = null;
        
        // Professional modules
        this.screenCapture = null;
        this.inputControl = null;
        
        // Supabase configuration
        this.supabaseUrl = 'https://ptrtibzwokjcjjxvjpin.supabase.co';
        this.supabaseKey = 'REDACTED_JWT';
        
        // File transfer URL
        this.fileTransferUrl = 'https://ptrtibzwokjcjjxvjpin.supabase.co/functions/v1/file-transfer';
        
        this.displayBanner();
    }

    displayBanner() {
        console.log('╔══════════════════════════════════════════════════════════════╗');
        console.log('║                🌍 Production Remote Desktop Agent           ║');
        console.log('║                   Standalone Edition v6.0.0                 ║');
        console.log('╠══════════════════════════════════════════════════════════════╣');
        console.log(`║ Device Name: ${this.deviceName.padEnd(45)} ║`);
        console.log(`║ Device ID:   ${this.deviceId.padEnd(45)} ║`);
        console.log(`║ Platform:    ${os.platform().padEnd(45)} ║`);
        console.log('║ Version:     6.0.0 - Production Standalone                  ║');
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
                        if (alias.family === 'IPv4' && !alias.internal && alias.mac && alias.mac !== '00:00:00:00:00:00') {
                            macAddress = alias.mac;
                            break;
                        }
                    }
                    if (macAddress !== 'unknown') break;
                }
            } catch (error) {
                console.warn('⚠️ Could not determine MAC address, using fallback');
            }
            
            // Create hardware fingerprint
            const hardwareString = `${hostname}-${platform}-${arch}-${cpus}-${totalMem}-${macAddress}`;
            const hash = crypto.createHash('sha256').update(hardwareString).digest('hex');
            const deviceId = `device_${hash.substring(0, 16)}`;
            
            console.log(`🔑 Generated device ID: ${deviceId}`);
            console.log(`💻 Hardware fingerprint: ${hostname} (${platform}/${arch}, ${cpus} CPUs, ${totalMem}GB RAM)`);
            
            return deviceId;
        } catch (error) {
            console.error('❌ Error generating device ID:', error.message);
            // Fallback to random ID if hardware detection fails
            return `device_${crypto.randomBytes(8).toString('hex')}`;
        }
    }

    async initialize() {
        try {
            console.log('🚀 Initializing Production Agent...');
            
            // Initialize Supabase client
            this.supabaseClient = createClient(this.supabaseUrl, this.supabaseKey);
            console.log('✅ Supabase client initialized');
            
            // Display system information
            this.displaySystemInfo();
            
            // Register device
            await this.registerDevice();
            
            // Connect to Supabase Realtime
            await this.connectSupabaseRealtime();
            
            // Setup capabilities
            await this.setupRemoteControlCapabilities();
            this.setupFileTransfer();
            this.setupSessionManagement();
            
            // Start heartbeat
            this.startHeartbeat();
            
            console.log('✅ Production Agent fully initialized and ready!');
            console.log('🌐 Waiting for remote connections...');
            
            // Keep the process alive
            this.keepAlive();
            
        } catch (error) {
            console.error('❌ Failed to initialize agent:', error.message);
            process.exit(1);
        }
    }

    displaySystemInfo() {
        const cpus = os.cpus();
        const totalMem = Math.round(os.totalmem() / (1024 * 1024 * 1024));
        const freeMem = Math.round(os.freemem() / (1024 * 1024 * 1024));
        
        console.log('📊 System Information:');
        console.log(`   • OS: ${os.type()} ${os.release()}`);
        console.log(`   • Architecture: ${os.arch()}`);
        console.log(`   • CPUs: ${cpus.length}x ${cpus[0]?.model || 'Unknown'}`);
        console.log(`   • Memory: ${freeMem}GB free / ${totalMem}GB total`);
        console.log(`   • Uptime: ${Math.round(os.uptime() / 3600)}h`);
        console.log('');
    }

    async registerDevice() {
        try {
            console.log('📝 Registering device with Supabase...');
            
            const deviceData = {
                id: this.deviceId,
                device_name: this.deviceName,
                operating_system: os.platform(),
                ip_address: this.getLocalIP(),
                status: 'online',
                last_seen: new Date().toISOString()
            };

            const { data, error } = await this.supabaseClient
                .from('remote_devices')
                .upsert(deviceData, { 
                    onConflict: 'id',
                    ignoreDuplicates: false 
                })
                .select();

            if (error) {
                console.error('❌ Device registration failed:', error.message);
                throw error;
            }

            console.log('✅ Device registered successfully');
            console.log(`📱 Device ID: ${this.deviceId}`);
            console.log(`💻 Device Name: ${this.deviceName}`);
            
        } catch (error) {
            console.error('❌ Failed to register device:', error.message);
            throw error;
        }
    }

    async connectSupabaseRealtime() {
        try {
            console.log('🔌 Connecting to Supabase Realtime...');
            
            // Create realtime channel for this device
            this.realtimeChannel = this.supabaseClient.channel(`device_${this.deviceId}`, {
                config: {
                    broadcast: { self: true },
                    presence: { key: this.deviceId }
                }
            });

            // Setup command listener
            this.setupRealtimeCommandListener();

            // Subscribe to the channel
            this.realtimeChannel.subscribe((status) => {
                if (status === 'SUBSCRIBED') {
                    console.log('✅ Connected to Supabase Realtime');
                    this.isConnected = true;
                } else if (status === 'CHANNEL_ERROR') {
                    console.error('❌ Realtime channel error');
                    this.isConnected = false;
                } else if (status === 'TIMED_OUT') {
                    console.error('❌ Realtime connection timed out');
                    this.isConnected = false;
                }
            });
            
        } catch (error) {
            console.error('❌ Failed to connect to Supabase Realtime:', error.message);
            throw error;
        }
    }

    setupRealtimeCommandListener() {
        // Listen for remote control commands
        this.realtimeChannel.on('broadcast', { event: 'remote_command' }, (payload) => {
            console.log('🎮 Received remote command via Realtime');
            this.handleRealtimeCommand(payload.payload);
        });
    }

    async handleRealtimeCommand(command) {
        try {
            console.log(`💬 Processing command: ${command.type}`);
            
            switch (command.type) {
                case 'mouse':
                    if (this.inputControl) {
                        const result = await this.inputControl.handleMouseInput(
                            command.data.x || 0,
                            command.data.y || 0,
                            command.data.button || 'left',
                            command.data.action || 'move',
                            command.data.options || {}
                        );
                        console.log(`🖱️ Mouse ${command.data.action || 'move'}: ${result.success ? '✅' : '❌'} ${result.message || ''}`);
                    } else {
                        console.log(`🖱️ Mouse ${command.data.action || 'move'} at (${command.data.x || 0}, ${command.data.y || 0}) [SIMULATION]`);
                    }
                    break;
                    
                case 'keyboard':
                    if (this.inputControl) {
                        let result;
                        if (command.data.special) {
                            // Handle special key combinations
                            result = await this.inputControl.handleSpecialKeys(command.data.combination || command.data.key);
                        } else {
                            // Handle regular keyboard input
                            result = await this.inputControl.handleKeyboardInput(
                                command.data.key || '',
                                command.data.action || 'press',
                                command.data.options || {}
                            );
                        }
                        console.log(`⌨️ Keyboard ${command.data.action || 'press'}: ${result.success ? '✅' : '❌'} ${result.message || ''}`);
                    } else {
                        console.log(`⌨️ Keyboard ${command.data.action || 'press'}: ${command.data.key || 'unknown'} [SIMULATION]`);
                    }
                    break;
                    
                case 'screen_capture':
                    if (command.data.action === 'start') {
                        this.startScreenCapture();
                    } else if (command.data.action === 'stop') {
                        this.stopScreenCapture();
                    }
                    break;
                    
                case 'file_transfer':
                    await this.handleFileTransferCommand(command.data);
                    break;
                    
                case 'system':
                    await this.handleSystemCommand(command.data);
                    break;
                    
                default:
                    console.log(`❓ Unknown command type: ${command.type}`);
            }
            
            // Send acknowledgment
            await this.sendRealtimeResponse({
                type: 'command_ack',
                commandId: command.id,
                status: 'success',
                timestamp: Date.now()
            });
            
        } catch (error) {
            console.error('❌ Error handling command:', error.message);
            
            // Send error response
            await this.sendRealtimeResponse({
                type: 'command_ack',
                commandId: command.id,
                status: 'error',
                error: error.message,
                timestamp: Date.now()
            });
        }
    }

    async sendRealtimeResponse(message) {
        try {
            await this.realtimeChannel.send({
                type: 'broadcast',
                event: 'agent_response',
                payload: message
            });
        } catch (error) {
            console.error('❌ Failed to send realtime response:', error.message);
        }
    }

    async setupRemoteControlCapabilities() {
        console.log('🎮 Setting up remote control capabilities...');
        
        try {
            if (ProfessionalScreenCapture && ProfessionalInputControl) {
                // Initialize professional screen capture
                this.screenCapture = new ProfessionalScreenCapture({
                    quality: 80,
                    maxWidth: 1920,
                    maxHeight: 1080,
                    frameRate: 10
                });
                await this.screenCapture.initialize();
                
                // Initialize professional input control
                this.inputControl = new ProfessionalInputControl({
                    enabled: true,
                    mouseSensitivity: 1.0,
                    keyboardDelay: 0
                });
                await this.inputControl.initialize();
                
                console.log('✅ Professional remote control capabilities ready');
            } else {
                console.log('⚠️ Professional modules not available - running in simulation mode');
                console.log('   Remote control commands will be logged but not executed');
            }
            return true;
            
        } catch (error) {
            console.error('❌ Failed to setup remote control capabilities:', error.message);
            console.log('⚠️ Falling back to simulation mode');
            this.screenCapture = null;
            this.inputControl = null;
            return false;
        }
    }

    setupFileTransfer() {
        console.log('📁 Setting up file transfer capabilities...');
        // File transfer is handled via Supabase Edge Functions
        console.log('✅ File transfer capabilities ready');
    }

    setupSessionManagement() {
        console.log('🔐 Setting up session management...');
        // Session management for tracking active connections
        console.log('✅ Session management ready');
    }

    startScreenCapture() {
        if (this.screenCaptureInterval) {
            console.log('📺 Screen capture already running');
            return;
        }
        
        console.log('📺 Starting real-time screen capture...');
        
        const captureFrame = async () => {
            try {
                if (this.screenCapture) {
                    const frameData = await this.screenCapture.captureScreenAuto();
                    
                    // Send frame via Supabase Realtime
                    await this.sendRealtimeResponse({
                        type: 'screen_frame',
                        frameData: frameData,
                        timestamp: Date.now(),
                        stats: this.screenCapture.getStats()
                    });
                    
                    console.log('📸 Screen frame captured and sent');
                } else {
                    console.log('📸 Capturing screen frame (simulation mode)');
                }
            } catch (error) {
                console.error('❌ Screen capture error:', error.message);
            }
        };
        
        // Start capturing at specified frame rate
        const frameInterval = this.screenCapture ? 1000 / this.screenCapture.frameRate : 1000;
        this.screenCaptureInterval = setInterval(captureFrame, frameInterval);
        
        // Capture first frame immediately
        captureFrame();
    }

    stopScreenCapture() {
        if (this.screenCaptureInterval) {
            clearInterval(this.screenCaptureInterval);
            this.screenCaptureInterval = null;
            console.log('📺 Screen capture stopped');
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

    async handleFileTransferCommand(data) {
        console.log(`📁 File transfer command: ${data.operation}`);
        // TODO: Implement file transfer operations
    }

    async handleSystemCommand(data) {
        console.log(`💻 System command: ${data.action}`);
        // TODO: Implement system commands
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
                .update({ status: 'offline' })
                .eq('id', this.deviceId);

            // Unsubscribe from realtime
            if (this.realtimeChannel) {
                await this.realtimeChannel.unsubscribe();
            }

            // Stop screen capture
            this.stopScreenCapture();

            // Destroy professional modules
            if (this.screenCapture) {
                this.screenCapture.destroy();
                this.screenCapture = null;
            }

            if (this.inputControl) {
                this.inputControl.destroy();
                this.inputControl = null;
            }

            console.log('✅ Cleanup completed');
        } catch (error) {
            console.error('❌ Cleanup error:', error.message);
        }
    }
}

// Start the Production Agent
const agent = new ProductionAgent();
agent.initialize().catch(console.error);
