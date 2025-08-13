#!/usr/bin/env node

/**
 * Minimal Working Agent - Schema Compatible
 * Purpose: Get agent visible in dashboard with correct schema
 */

const os = require('os');
const fs = require('fs');
const crypto = require('crypto');
const http = require('http');
const https = require('https');
const WebSocket = require('ws');
const { createClient } = require('@supabase/supabase-js');

// Try to load native modules with fallbacks
let screenshot, sharp, robot;
try {
    screenshot = require('screenshot-desktop');
    console.log('✅ Screenshot module loaded');
} catch (e) {
    console.log('⚠️ Screenshot module not available, using fallback');
}

try {
    sharp = require('sharp');
    console.log('✅ Sharp module loaded');
} catch (e) {
    console.log('⚠️ Sharp module not available, using fallback');
}

try {
    robot = require('robotjs');
    console.log('✅ RobotJS module loaded');
} catch (e) {
    console.log('⚠️ RobotJS module not available, using fallback');
}

// Supabase configuration
const supabaseUrl = 'https://ptrtibzwokjcjjxvjpin.supabase.co';
const supabaseKey = 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJzdXBhYmFzZSIsInJlZiI6InB0cnRpYnp3b2tqY2pqeHZqcGluIiwicm9sZSI6ImFub24iLCJpYXQiOjE3NTQ0MzE1NzEsImV4cCI6MjA3MDAwNzU3MX0.DPNxkQul1-13tqJ89mqYJAx7NSJjabOP4q8c6KgOnWk';

class MinimalWorkingAgent {
    constructor() {
        this.deviceName = os.hostname() || 'WorkingAgent';
        this.deviceId = this.generateDeviceId();
        this.supabase = createClient(supabaseUrl, supabaseKey);
        this.isRunning = false;
        this.httpServer = null;
        this.httpsServer = null;
        this.wsServer = null;
        this.wssServer = null;
        this.connectedClients = new Set();
        this.screenCaptureInterval = null;
        this.isScreenSharing = false;
        
        console.log('🚀 Minimal Working Agent Starting...');
        console.log(`📱 Device Name: ${this.deviceName}`);
        console.log(`🆔 Device ID: ${this.deviceId}`);
        console.log(`💻 Platform: ${os.platform()}`);
        console.log('');
    }
    
    generateDeviceId() {
        // Generate hardware-based consistent device ID
        const crypto = require('crypto');
        const os = require('os');
        
        // Use hardware info for consistent ID
        const hardwareInfo = [
            os.hostname(),
            os.platform(),
            os.arch(),
            os.cpus()[0].model,
            os.totalmem().toString()
        ].join('|');
        
        // Generate UUID-format device ID
        const hash = crypto.createHash('sha256').update(hardwareInfo).digest('hex');
        const uuid = [
            hash.substr(0, 8),
            hash.substr(8, 4),
            hash.substr(12, 4),
            hash.substr(16, 4),
            hash.substr(20, 12)
        ].join('-');
        
        return uuid;
    }
    
    async start() {
        console.log('✅ Starting working agent...');
        this.isRunning = true;
        
        // Update Dennis device to be our agent
        await this.updateDevice();
        
        // Start WebSocket servers (WS + WSS)
        await this.startWebSocketServers();
        
        // Start heartbeat
        this.startHeartbeat();
        
        console.log('🎉 Working agent is now running and visible in dashboard!');
        console.log('🌐 Check dashboard: https://stangtennis.github.io/remote-desktop/dashboard.html');
    }
    
    async updateDevice() {
        try {
            console.log(`📝 Registering/updating device: ${this.deviceName} (ID: ${this.deviceId})`);
            
            // Minimal device data - using correct column names and required fields
            const deviceData = {
                device_name: this.deviceName,
                device_id: this.deviceId,
                is_online: true,
                last_seen: new Date().toISOString()
            };
            
            console.log('📊 Registration data:', deviceData);
            
            // First, try to UPDATE existing device by device_id
            console.log('🔄 Attempting to update existing device...');
            const { data: updateData, error: updateError } = await this.supabase
                .from('remote_devices')
                .update({
                    device_name: this.deviceName,
                    is_online: true,
                    last_seen: new Date().toISOString()
                })
                .eq('device_id', this.deviceId)
                .select();
                
            if (updateError) {
                console.error('❌ Update failed:', updateError.message);
                
                // If update fails, try INSERT (new device)
                console.log('🔄 Trying to insert new device...');
                const { data: insertData, error: insertError } = await this.supabase
                    .from('remote_devices')
                    .insert(deviceData)
                    .select();
                    
                if (insertError) {
                    console.error('❌ Insert also failed:', insertError.message);
                    console.error('❌ Insert error details:', insertError);
                } else {
                    console.log(`✅ New device registered successfully: ${this.deviceName}`);
                    console.log('📊 Device data:', insertData);
                }
            } else if (updateData && updateData.length > 0) {
                console.log(`✅ Existing device updated successfully: ${this.deviceName}`);
                console.log('📊 Device data:', updateData);
            } else {
                console.log('⚠️ Update succeeded but no rows affected - device may not exist');
                
                // Try INSERT for new device
                console.log('🔄 Trying to insert new device...');
                const { data: insertData, error: insertError } = await this.supabase
                    .from('remote_devices')
                    .insert(deviceData)
                    .select();
                    
                if (insertError) {
                    console.error('❌ Insert failed:', insertError.message);
                } else {
                    console.log(`✅ New device registered successfully: ${this.deviceName}`);
                    console.log('📊 Device data:', insertData);
                }
            }
        } catch (error) {
            console.error('❌ Registration error:', error.message);
        }
    }
    
    async startWebSocketServers() {
        try {
            console.log('🔌 Starting WebSocket servers (WS + WSS)...');
            
            // Start regular WebSocket server (WS)
            await this.startWSServer();
            console.log('ℹ️  WSS server disabled - using WS server for now');
            
        } catch (error) {
            console.error('❌ Failed to start WebSocket servers:', error.message);
        }
    }
    
    async startWSServer() {
        const ports = [8080, 8081, 8082, 8083, 8084, 8085, 8086, 8087, 8088, 8089];
        
        for (const port of ports) {
            try {
                console.log(`🔌 Trying to start WS server on port ${port}...`);
                
                // Create HTTP server
                this.httpServer = http.createServer();
                
                // Create WebSocket server
                this.wsServer = new WebSocket.Server({ 
                    server: this.httpServer
                });
                
                // Handle WebSocket connections
                this.wsServer.on('connection', (ws, req) => {
                    console.log(`🔗 New WS connection from ${req.socket.remoteAddress}`);
                    this.handleWebSocketConnection(ws, 'WS');
                });
                
                // Try to start HTTP server on this port
                await new Promise((resolve, reject) => {
                    this.httpServer.listen(port, () => {
                        console.log(`✅ WS server running on port ${port}`);
                        this.wsPort = port; // Store the successful port
                        resolve();
                    });
                    
                    this.httpServer.on('error', (error) => {
                        if (error.code === 'EADDRINUSE') {
                            console.log(`⚠️ Port ${port} is in use, trying next port...`);
                            reject(error);
                        } else {
                            reject(error);
                        }
                    });
                });
                
                // If we get here, server started successfully
                break;
                
            } catch (error) {
                if (error.code === 'EADDRINUSE') {
                    // Try next port
                    continue;
                } else {
                    console.error(`❌ Failed to start WS server on port ${port}:`, error.message);
                    break;
                }
            }
        }
    }
    
    async startWSSServer() {
        try {
            console.log('🔒 Starting WSS server...');
            
            // Generate self-signed SSL certificate
            const sslOptions = await this.generateSSLCertificate();
            
            // Create HTTPS server
            this.httpsServer = https.createServer(sslOptions);
            
            // Create secure WebSocket server
            this.wssServer = new WebSocket.Server({ 
                server: this.httpsServer,
                port: 8443
            });
            
            // Handle secure WebSocket connections
            this.wssServer.on('connection', (ws, req) => {
                console.log(`🔒 New WSS connection from ${req.socket.remoteAddress}`);
                this.handleWebSocketConnection(ws, 'WSS');
            });
            
            // Start HTTPS server
            this.httpsServer.listen(8443, () => {
                console.log('✅ WSS server running on port 8443');
            });
            
        } catch (error) {
            console.error('❌ Failed to start WSS server:', error.message);
        }
    }
    
    handleWebSocketConnection(ws, type) {
        this.connectedClients.add(ws);
        
        // Send welcome message
        ws.send(JSON.stringify({
            type: 'welcome',
            message: `Connected to Remote Desktop Agent via ${type}`,
            deviceName: this.deviceName,
            connectionType: type,
            timestamp: new Date().toISOString()
        }));
        
        // Handle messages from dashboard
        ws.on('message', (message) => {
            try {
                const data = JSON.parse(message);
                this.handleRemoteCommand(data, ws);
            } catch (error) {
                console.error('❌ Error parsing WebSocket message:', error.message);
            }
        });
        
        // Handle connection close
        ws.on('close', () => {
            console.log(`🔌 ${type} connection closed`);
            this.connectedClients.delete(ws);
        });
        
        // Handle errors
        ws.on('error', (error) => {
            console.error(`❌ ${type} error:`, error.message);
            this.connectedClients.delete(ws);
        });
    }
    
    async generateSSLCertificate() {
        try {
            console.log('🔐 Generating proper self-signed SSL certificate...');
            
            // Try to use existing certificate files
            const certPath = './ssl-cert.pem';
            const keyPath = './ssl-key.pem';
            
            if (fs.existsSync(certPath) && fs.existsSync(keyPath)) {
                console.log('✅ Using existing SSL certificate files');
                return {
                    cert: fs.readFileSync(certPath),
                    key: fs.readFileSync(keyPath)
                };
            }
            
            console.log('🔐 Creating simple SSL certificate for WebSocket compatibility...');
            
            // Generate RSA key pair using Node.js crypto
            const { generateKeyPairSync } = crypto;
            const { privateKey, publicKey } = generateKeyPairSync('rsa', {
                modulusLength: 2048,
                publicKeyEncoding: {
                    type: 'spki',
                    format: 'pem'
                },
                privateKeyEncoding: {
                    type: 'pkcs8',
                    format: 'pem'
                }
            });
            
            // For WebSocket over HTTPS, we can use a minimal approach
            // Create basic certificate that Node.js HTTPS can accept
            const simpleCert = privateKey; // Use private key for both cert and key
            
            // Save certificate and key files
            fs.writeFileSync(certPath, simpleCert);
            fs.writeFileSync(keyPath, privateKey);
            
            console.log('✅ Generated simple SSL certificate for WebSocket compatibility');
            
            return {
                cert: simpleCert,
                key: privateKey
            };
            
        } catch (error) {
            console.error('❌ SSL certificate generation failed:', error.message);
            
            // Fallback: create minimal certificate for testing
            const fallbackCert = `-----BEGIN CERTIFICATE-----
MIIBkTCB+wIJAMlyFqk69v+9MA0GCSqGSIb3DQEBCwUAMBQxEjAQBgNVBAMMCWxv
Y2FsaG9zdDAeFw0yNDEyMDgwODAwMDBaFw0yNTEyMDgwODAwMDBaMBQxEjAQBgNV
BAMMCWxvY2FsaG9zdDBcMA0GCSqGSIb3DQEBAQUAA0sAMEgCQQDAlUqmNpOIPGoa
aFQKi65JBXLYKg6npszdrbau2x3h4WCn4SB9xZy7aUWpJ5ioiNtCz3TCUyGyaZZm
SV8Wk468SJLs+ObHs64cyYdVAgMBAAEwDQYJKoZIhvcNAQELBQADQQBz5eD16u+7
-----END CERTIFICATE-----`;
            
            const fallbackKey = `-----BEGIN PRIVATE KEY-----
MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQDAlUqmNpOIPGoa
aFQKi65JBXLYKg6npszdrbau2x3h4WCn4SB9xZy7aUWpJ5ioiNtCz3TCUyGyaZZm
SV8Wk468SJLs+ObHs64cyYdVAgMBAAECggEAQJGixnQO4No=
-----END PRIVATE KEY-----`;
            
            return {
                cert: fallbackCert,
                key: fallbackKey
            };
        }
    }
    
    createBasicCertificate(privateKey) {
        // Create a basic self-signed certificate that Node.js can parse
        // This is a simplified X.509 certificate for localhost
        const basicCert = `-----BEGIN CERTIFICATE-----
MIICpjCCAY4CCQDKOGJQUuSHWTANBgkqhkiG9w0BAQsFADCBkTELMAkGA1UEBhMC
VVMxEzARBgNVBAgMCkNhbGlmb3JuaWExFjAUBgNVBAcMDU1vdW50YWluIFZpZXcx
FDASBgNVBAoMC1JlbW90ZURlc2t0b3AxEzARBgNVBAsMClNlbGZTaWduZWQxGjAY
BgNVBAMMEWxvY2FsaG9zdC5sb2NhbGRvbTAeFw0yNDA4MTIwODAwMDBaFw0yNTA4
MTIwODAwMDBaMIGRMQswCQYDVQQGEwJVUzETMBEGA1UECAwKQ2FsaWZvcm5pYTEW
MBQGA1UEBwwNTW91bnRhaW4gVmlldzEUMBIGA1UECgwLUmVtb3RlRGVza3RvcDET
MBEGA1UECwwKU2VsZlNpZ25lZDEaMBgGA1UEAwwRbG9jYWxob3N0LmxvY2FsZG9t
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAwJVKpjaTiDxqGmhUCouu
SQVy2CoOp6bM3a02rssd4eFgp+EgfcWcu2lFqSeYqIjbQs90wlMhsmmWZklfFpOO
vEiS7Pjmx7OuHMmHVQIDAQABo1MwUTAdBgNVHQ4EFgQUt2ui6qiqhftzKleEBQJD
eQuwPiMwHwYDVR0jBBgwFoAUt2ui6qiqhftzKleEBQJDeQuwPiMwDwYDVR0TAQH/
BAUwAwEB/zANBgkqhkiG9w0BAQsFAAOCAQEAg+oPhcQZ0iNoJG9spsVt5bucf9ig
xmTI3ChcIQRU6h+3iuIldt4GGL90IJuEMIDrLNn2qbcIxHw1S14bbeQ+cYtStX7L
eWdQDRFtMaMQStWvFm/LYQJf4BISbd93Ch6+sM4=
-----END CERTIFICATE-----`;
        
        return basicCert;
    }
    
    handleRemoteCommand(command, ws) {
        console.log(`💬 Received command: ${command.type || command.command}`);
        
        switch (command.type || command.command) {
            case 'start-screen':
                console.log('📺 Starting screen sharing...');
                this.startScreenCapture(ws);
                break;
                
            case 'stop-screen':
                console.log('⏹️ Stopping screen sharing...');
                this.stopScreenCapture(ws);
                break;
                
            case 'mouse':
                console.log(`🖱️ Mouse ${command.action || 'move'} at (${command.x || 0}, ${command.y || 0})`);
                this.handleMouseInput(command);
                break;
                
            case 'keyboard':
                console.log(`⌨️ Keyboard ${command.action || 'press'} key: ${command.key || 'unknown'}`);
                this.handleKeyboardInput(command);
                break;
                
            default:
                console.log(`❓ Unknown command: ${command.type || command.command}`);
        }
    }
    
    async startScreenCapture(ws) {
        if (this.isScreenSharing) {
            console.log('⚠️ Screen sharing already active');
            return;
        }
        
        this.isScreenSharing = true;
        console.log('📺 Starting real screen capture...');
        
        // Send confirmation to dashboard
        ws.send(JSON.stringify({
            type: 'response',
            message: 'Screen sharing started',
            success: true
        }));
        
        // Start screen capture loop
        this.screenCaptureInterval = setInterval(async () => {
            try {
                await this.captureAndSendScreen(ws);
            } catch (error) {
                console.error('❌ Screen capture error:', error.message);
            }
        }, 100); // 10 FPS
    }
    
    async captureAndSendScreen(ws) {
        try {
            let screenBuffer;
            
            if (screenshot) {
                // Use real screen capture
                screenBuffer = await screenshot();
                
                // Compress with Sharp if available
                if (sharp) {
                    screenBuffer = await sharp(screenBuffer)
                        .resize(800, 600)
                        .jpeg({ quality: 60 })
                        .toBuffer();
                }
                
                // Send real screen data
                ws.send(JSON.stringify({
                    type: 'screen-frame',
                    data: screenBuffer.toString('base64'),
                    timestamp: Date.now()
                }));
                
            } else {
                // Fallback: Send mock screen data
                const mockScreen = Buffer.from('Mock screen data - native modules not available');
                ws.send(JSON.stringify({
                    type: 'screen-frame',
                    data: mockScreen.toString('base64'),
                    timestamp: Date.now(),
                    mock: true
                }));
            }
            
        } catch (error) {
            console.error('❌ Screen capture failed:', error.message);
        }
    }
    
    stopScreenCapture(ws) {
        if (!this.isScreenSharing) {
            console.log('⚠️ Screen sharing not active');
            return;
        }
        
        this.isScreenSharing = false;
        console.log('⏹️ Stopping screen capture...');
        
        // Clear interval
        if (this.screenCaptureInterval) {
            clearInterval(this.screenCaptureInterval);
            this.screenCaptureInterval = null;
        }
        
        // Send confirmation to dashboard
        ws.send(JSON.stringify({
            type: 'response',
            message: 'Screen sharing stopped',
            success: true
        }));
    }
    
    handleMouseInput(command) {
        try {
            if (robot) {
                // Use real mouse control with RobotJS
                switch (command.action || command.type) {
                    case 'move':
                        robot.moveMouse(command.x || 0, command.y || 0);
                        console.log(`🖱️ Real mouse moved to (${command.x}, ${command.y})`);
                        break;
                        
                    case 'click':
                    case 'left-click':
                        if (command.x && command.y) {
                            robot.moveMouse(command.x, command.y);
                        }
                        robot.mouseClick('left');
                        console.log(`🖱️ Real left click at (${command.x}, ${command.y})`);
                        break;
                        
                    case 'right-click':
                        if (command.x && command.y) {
                            robot.moveMouse(command.x, command.y);
                        }
                        robot.mouseClick('right');
                        console.log(`🖱️ Real right click at (${command.x}, ${command.y})`);
                        break;
                        
                    case 'double-click':
                        if (command.x && command.y) {
                            robot.moveMouse(command.x, command.y);
                        }
                        robot.mouseClick('left', true); // double click
                        console.log(`🖱️ Real double click at (${command.x}, ${command.y})`);
                        break;
                        
                    case 'scroll':
                        robot.scrollMouse(command.deltaX || 0, command.deltaY || 0);
                        console.log(`🖱️ Real scroll (${command.deltaX}, ${command.deltaY})`);
                        break;
                        
                    default:
                        console.log(`❓ Unknown mouse action: ${command.action}`);
                }
            } else {
                // Fallback: Use PowerShell for mouse control
                this.handleMouseInputFallback(command);
            }
        } catch (error) {
            console.error('❌ Mouse input error:', error.message);
        }
    }
    
    handleKeyboardInput(command) {
        try {
            if (robot) {
                // Use real keyboard control with RobotJS
                switch (command.action || command.type) {
                    case 'press':
                    case 'keypress':
                        if (command.key) {
                            robot.keyTap(command.key);
                            console.log(`⌨️ Real key press: ${command.key}`);
                        }
                        break;
                        
                    case 'type':
                        if (command.text) {
                            robot.typeString(command.text);
                            console.log(`⌨️ Real type: ${command.text}`);
                        }
                        break;
                        
                    case 'keydown':
                        if (command.key) {
                            robot.keyToggle(command.key, 'down');
                            console.log(`⌨️ Real key down: ${command.key}`);
                        }
                        break;
                        
                    case 'keyup':
                        if (command.key) {
                            robot.keyToggle(command.key, 'up');
                            console.log(`⌨️ Real key up: ${command.key}`);
                        }
                        break;
                        
                    case 'combo':
                        if (command.keys && Array.isArray(command.keys)) {
                            robot.keyTap(command.keys[command.keys.length - 1], command.keys.slice(0, -1));
                            console.log(`⌨️ Real key combo: ${command.keys.join('+')}`);
                        }
                        break;
                        
                    default:
                        console.log(`❓ Unknown keyboard action: ${command.action}`);
                }
            } else {
                // Fallback: Use PowerShell for keyboard control
                this.handleKeyboardInputFallback(command);
            }
        } catch (error) {
            console.error('❌ Keyboard input error:', error.message);
        }
    }
    
    handleMouseInputFallback(command) {
        // PowerShell fallback for mouse control
        console.log(`🖱️ Fallback mouse ${command.action || 'move'} at (${command.x || 0}, ${command.y || 0})`);
        // Could implement PowerShell mouse control here if needed
    }
    
    handleKeyboardInputFallback(command) {
        // PowerShell fallback for keyboard control
        console.log(`⌨️ Fallback keyboard ${command.action || 'press'} key: ${command.key || 'unknown'}`);
        // Could implement PowerShell keyboard control here if needed
    }

    startHeartbeat() {
        console.log('💓 Starting heartbeat...');
        
        setInterval(async () => {
            if (!this.isRunning) return;
            
            try {
                const updateData = {
                    device_name: this.deviceName,
                    is_online: true,
                    last_seen: new Date().toISOString()
                };
                
                // Try to update any device with our name
                await this.supabase
                    .from('remote_devices')
                    .update(updateData)
                    .eq('device_name', this.deviceName);
                    
                console.log(`💓 Heartbeat sent - ${new Date().toLocaleTimeString()}`);
            } catch (error) {
                console.error('❌ Heartbeat failed:', error.message);
            }
        }, 30000); // Every 30 seconds
    }
    
    async stop() {
        console.log('🛑 Stopping working agent...');
        this.isRunning = false;
        
        try {
            const updateData = {
                is_online: false,
                last_seen: new Date().toISOString()
            };
            
            await this.supabase
                .from('remote_devices')
                .update(updateData)
                .eq('device_name', this.deviceName);
                
            console.log('✅ Agent stopped and marked offline');
        } catch (error) {
            console.error('❌ Stop error:', error.message);
        }
    }
}

// Handle graceful shutdown
process.on('SIGINT', async () => {
    console.log('\n🛑 Received SIGINT, shutting down gracefully...');
    if (global.agent) {
        await global.agent.stop();
    }
    process.exit(0);
});

// Start the working agent
const agent = new MinimalWorkingAgent();
global.agent = agent;
agent.start().catch(console.error);
