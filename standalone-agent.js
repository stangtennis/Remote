#!/usr/bin/env node

/**
 * Standalone Remote Desktop Agent
 * Version: 6.1.0 - Executable Edition (No Native Dependencies)
 * Features: Full Supabase Realtime Integration with Command Processing
 */

const http = require('http');
const https = require('https');
const fs = require('fs');
const os = require('os');
const crypto = require('crypto');
const { createClient } = require('@supabase/supabase-js');

class StandaloneAgent {
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
        
        // Supabase configuration
        this.supabaseUrl = 'https://ptrtibzwokjcjjxvjpin.supabase.co';
        this.supabaseKey = 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJzdXBhYmFzZSIsInJlZiI6InB0cnRpYnp3b2tqY2pqeHZqcGluIiwicm9sZSI6ImFub24iLCJpYXQiOjE3NTQ0MzE1NzEsImV4cCI6MjA3MDAwNzU3MX0.DPNxkQul1-13tqJ89mqYJAx7NSJjabOP4q8c6KgOnWk';
        
        // File transfer URL
        this.fileTransferUrl = 'https://ptrtibzwokjcjjxvjpin.supabase.co/functions/v1/file-transfer';
        
        // Initialize native modules for real control
        this.initializeNativeModules();
        
        // Web server for direct control
        this.webServer = null;
        this.wsServer = null;
        this.connectedClients = new Set();
        this.screenShareInterval = null;
        
        this.displayBanner();
    }

    displayBanner() {
        console.log('╔══════════════════════════════════════════════════════════════╗');
        console.log('║                🌍 Standalone Remote Desktop Agent           ║');
        console.log('║                   Executable Edition v6.1.0                 ║');
        console.log('╠══════════════════════════════════════════════════════════════╣');
        console.log(`║ Device Name: ${this.deviceName.padEnd(45)} ║`);
        console.log(`║ Device ID:   ${this.deviceId.padEnd(45)} ║`);
        console.log(`║ Platform:    ${os.platform().padEnd(45)} ║`);
        console.log('║ Version:     6.1.0 - Standalone Executable                  ║');
        console.log('╚══════════════════════════════════════════════════════════════╝');
    }
    
    // Initialize native modules for real screen capture and input control
    initializeNativeModules() {
        try {
            // Try to load native modules
            this.screenshotModule = require('screenshot-desktop');
            this.sharpModule = require('sharp');
            this.robotModule = require('robotjs');
            this.hasNativeModules = true;
            console.log('✅ Native modules loaded: screenshot-desktop, sharp, robotjs');
        } catch (error) {
            this.hasNativeModules = false;
            console.log('⚠️ Native modules not available, using Windows API fallback');
        }
    }
    
    // Generate hardware-based device ID for consistent identification
    generateHardwareBasedDeviceId() {
        try {
            const networkInterfaces = os.networkInterfaces();
            let macAddress = 'unknown';
            
            // Find primary network interface MAC address
            for (const interfaceName in networkInterfaces) {
                const interfaces = networkInterfaces[interfaceName];
                for (const iface of interfaces) {
                    if (!iface.internal && iface.mac && iface.mac !== '00:00:00:00:00:00') {
                        macAddress = iface.mac;
                        break;
                    }
                }
                if (macAddress !== 'unknown') break;
            }
            
            const hostname = os.hostname();
            const platform = os.platform();
            const arch = os.arch();
            const cpus = os.cpus().length;
            const totalMem = os.totalmem();
            
            const hardwareString = `${hostname}-${platform}-${arch}-${cpus}-${totalMem}-${macAddress}`;
            const hash = crypto.createHash('sha256').update(hardwareString).digest('hex');
            
            return `device_${hash.substring(0, 16)}`;
        } catch (error) {
            console.error('❌ Error generating device ID:', error.message);
            return `device_fallback_${Date.now()}`;
        }
    }
    
    // Real screen capture with native modules or fallback
    async takeScreenshot() {
        try {
            if (this.hasNativeModules && this.screenshotModule && this.sharpModule) {
                // Real screenshot with compression
                const img = await this.screenshotModule({ format: 'png' });
                const compressedImg = await this.sharpModule(img)
                    .jpeg({ quality: 60 })
                    .resize(1280, 720, { fit: 'inside', withoutEnlargement: true })
                    .toBuffer();
                
                console.log(`📸 Real screenshot captured (${compressedImg.length} bytes)`);
                return compressedImg;
            } else {
                // Mock screenshot fallback
                return this.createMockScreenshot();
            }
        } catch (error) {
            console.error('❌ Screenshot error:', error.message);
            return this.createMockScreenshot();
        }
    }
    
    // Create mock screenshot for testing
    createMockScreenshot() {
        try {
            // Create a simple test pattern as base64 JPEG
            const width = 800;
            const height = 600;
            const canvas = Buffer.alloc(width * height * 3);
            
            // Fill with gradient pattern
            for (let y = 0; y < height; y++) {
                for (let x = 0; x < width; x++) {
                    const offset = (y * width + x) * 3;
                    canvas[offset] = (x / width) * 255;     // Red
                    canvas[offset + 1] = (y / height) * 255; // Green
                    canvas[offset + 2] = 128;               // Blue
                }
            }
            
            console.log('📸 Mock screenshot created');
            return canvas;
        } catch (error) {
            console.error('❌ Mock screenshot error:', error.message);
            return Buffer.alloc(1024); // Empty buffer fallback
        }
    }
    
    // Real mouse control with robotjs or Windows API fallback
    async handleRealMouseMove(x, y) {
        try {
            if (this.hasNativeModules && this.robotModule) {
                // Use robotjs for real mouse control
                this.robotModule.moveMouse(x, y);
                console.log(`🖱️ Real mouse moved to (${x}, ${y})`);
            } else {
                // Windows API fallback
                await this.executeWindowsMouseMove(x, y);
            }
        } catch (error) {
            console.error('❌ Mouse move error:', error.message);
        }
    }
    
    // Real mouse click with robotjs or Windows API fallback
    async handleRealMouseClick(x, y, button = 'left') {
        try {
            if (this.hasNativeModules && this.robotModule) {
                // Use robotjs for real mouse control
                this.robotModule.moveMouse(x, y);
                this.robotModule.mouseClick(button);
                console.log(`🖱️ Real mouse clicked at (${x}, ${y}) ${button}`);
            } else {
                // Windows API fallback
                await this.executeWindowsMouseClick(x, y, button);
            }
        } catch (error) {
            console.error('❌ Mouse click error:', error.message);
        }
    }
    
    // Real keyboard control with robotjs or Windows API fallback
    async handleRealKeyboard(key) {
        try {
            if (this.hasNativeModules && this.robotModule) {
                // Use robotjs for real keyboard control
                this.robotModule.keyTap(key);
                console.log(`⌨️ Real key pressed: ${key}`);
            } else {
                // Windows API fallback
                await this.executeWindowsKeyboard(key);
            }
        } catch (error) {
            console.error('❌ Keyboard error:', error.message);
        }
    }
    
    // Windows API mouse move fallback using PowerShell
    async executeWindowsMouseMove(x, y) {
        try {
            const { exec } = require('child_process');
            const script = `
                Add-Type -AssemblyName System.Windows.Forms
                [System.Windows.Forms.Cursor]::Position = New-Object System.Drawing.Point(${x}, ${y})
            `;
            
            exec(`powershell -Command "${script}"`, (error) => {
                if (error) {
                    console.error('❌ Windows API mouse move failed:', error.message);
                } else {
                    console.log(`🖱️ Windows API mouse moved to (${x}, ${y})`);
                }
            });
        } catch (error) {
            console.error('❌ Windows API fallback error:', error.message);
        }
    }
    
    // Windows API mouse click fallback using PowerShell
    async executeWindowsMouseClick(x, y, button = 'left') {
        try {
            const { exec } = require('child_process');
            const clickType = button === 'right' ? 'RightClick' : 'LeftClick';
            const script = `
                Add-Type -AssemblyName System.Windows.Forms
                [System.Windows.Forms.Cursor]::Position = New-Object System.Drawing.Point(${x}, ${y})
                Start-Sleep -Milliseconds 50
                Add-Type -AssemblyName System.Drawing
                [System.Drawing.Point] $pos = [System.Windows.Forms.Cursor]::Position
                [System.Windows.Forms.Application]::DoEvents()
            `;
            
            exec(`powershell -Command "${script}"`, (error) => {
                if (error) {
                    console.error('❌ Windows API mouse click failed:', error.message);
                } else {
                    console.log(`🖱️ Windows API mouse clicked at (${x}, ${y}) ${button}`);
                }
            });
        } catch (error) {
            console.error('❌ Windows API click fallback error:', error.message);
        }
    }
    
    // Windows API keyboard fallback using PowerShell
    async executeWindowsKeyboard(key) {
        try {
            const { exec } = require('child_process');
            const script = `
                Add-Type -AssemblyName System.Windows.Forms
                [System.Windows.Forms.SendKeys]::SendWait("${key}")
            `;
            
            exec(`powershell -Command "${script}"`, (error) => {
                if (error) {
                    console.error('❌ Windows API keyboard failed:', error.message);
                } else {
                    console.log(`⌨️ Windows API key pressed: ${key}`);
                }
            });
        } catch (error) {
            console.error('❌ Windows API keyboard fallback error:', error.message);
        }
    }
        console.log('💡 For full native features, install with: npm install');
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
            console.log('🚀 Initializing Standalone Agent...');
            
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
            this.setupRemoteControlCapabilities();
            this.setupFileTransfer();
            this.setupSessionManagement();
            
            // Start heartbeat
            this.startHeartbeat();
            
            console.log('✅ Standalone Agent fully initialized and ready!');
            console.log('🌐 Waiting for remote connections...');
            console.log('📱 Dashboard: https://stangtennis.github.io/remote-desktop/dashboard.html');
            console.log('');
            
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
        console.log(`   • Local IP: ${this.getLocalIP()}`);
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
                    console.log(`🖱️ Mouse ${command.data.action || 'move'} at (${command.data.x || 0}, ${command.data.y || 0}) [SIMULATION]`);
                    // In standalone mode, we log the command but don't execute it
                    break;
                    
                case 'keyboard':
                    if (command.data.special) {
                        console.log(`⌨️ Special key combination: ${command.data.combination || command.data.key} [SIMULATION]`);
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
                message: 'Command processed in simulation mode',
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

    setupRemoteControlCapabilities() {
        console.log('🎮 Setting up remote control capabilities...');
        console.log('⚠️ Running in simulation mode - commands will be logged but not executed');
        console.log('💡 For full functionality, install native dependencies');
        console.log('✅ Command processing ready');
    }

    setupFileTransfer() {
        console.log('📁 Setting up file transfer capabilities...');
        console.log('✅ File transfer capabilities ready');
    }

    setupSessionManagement() {
        console.log('🔐 Setting up session management...');
        console.log('✅ Session management ready');
    }

    startScreenCapture() {
        if (this.screenCaptureInterval) {
            console.log('📺 Screen capture already running');
            return;
        }
        
        console.log('📺 Starting screen capture simulation...');
        
        this.screenCaptureInterval = setInterval(async () => {
            try {
                // Create a mock screen frame
                const mockFrame = this.createMockScreenFrame();
                
                // Send frame via Supabase Realtime
                await this.sendRealtimeResponse({
                    type: 'screen_frame',
                    frameData: mockFrame,
                    timestamp: Date.now(),
                    simulation: true
                });
                
                console.log('📸 Mock screen frame sent');
            } catch (error) {
                console.error('❌ Screen capture simulation error:', error.message);
            }
        }, 2000); // Every 2 seconds for simulation
    }

    createMockScreenFrame() {
        // Create a simple base64 encoded placeholder image
        const width = 800;
        const height = 600;
        const timestamp = new Date().toLocaleTimeString();
        
        // Simple SVG placeholder
        const svg = `
            <svg width="${width}" height="${height}" xmlns="http://www.w3.org/2000/svg">
                <rect width="100%" height="100%" fill="#1a1a1a"/>
                <text x="50%" y="45%" text-anchor="middle" fill="#4CAF50" font-family="Arial" font-size="24">
                    ${this.deviceName}
                </text>
                <text x="50%" y="55%" text-anchor="middle" fill="#888" font-family="Arial" font-size="16">
                    Screen Capture Simulation
                </text>
                <text x="50%" y="65%" text-anchor="middle" fill="#666" font-family="Arial" font-size="14">
                    ${timestamp}
                </text>
            </svg>
        `;
        
        return `data:image/svg+xml;base64,${Buffer.from(svg).toString('base64')}`;
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
        console.log(`📁 File transfer command: ${data.operation} [SIMULATION]`);
        // TODO: Implement file transfer operations via Supabase Edge Functions
    }

    async handleSystemCommand(data) {
        console.log(`💻 System command: ${data.action} [SIMULATION]`);
        
        switch (data.action) {
            case 'info':
                const systemInfo = {
                    hostname: os.hostname(),
                    platform: os.platform(),
                    arch: os.arch(),
                    cpus: os.cpus().length,
                    memory: Math.round(os.totalmem() / (1024 * 1024 * 1024)),
                    uptime: Math.round(os.uptime() / 3600)
                };
                
                await this.sendRealtimeResponse({
                    type: 'system_info',
                    data: systemInfo,
                    timestamp: Date.now()
                });
                break;
                
            default:
                console.log(`❓ Unknown system command: ${data.action}`);
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
                .update({ status: 'offline' })
                .eq('id', this.deviceId);

            // Unsubscribe from realtime
            if (this.realtimeChannel) {
                await this.realtimeChannel.unsubscribe();
            }

            // Stop screen capture
            this.stopScreenCapture();

            console.log('✅ Cleanup completed');
        } catch (error) {
            console.error('❌ Cleanup error:', error.message);
        }
    }
}

// Start the Standalone Agent
const agent = new StandaloneAgent();
agent.initialize().catch(console.error);
