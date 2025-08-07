const WebSocket = require('ws');
const http = require('http');

/**
 * WebSocket Server for Real-Time Remote Desktop Communication
 */

class RemoteDesktopWebSocketServer {
    constructor(port = 3001) {
        this.port = port;
        this.server = null;
        this.wss = null;
        this.devices = new Map();
        this.controllers = new Map();
        this.sessions = new Map();
    }

    start() {
        console.log('🚀 Starting WebSocket Server for Remote Desktop...');

        // Create HTTP server
        this.server = http.createServer();

        // Create WebSocket server
        this.wss = new WebSocket.Server({ server: this.server });

        this.wss.on('connection', (ws, req) => {
            console.log('🔌 New WebSocket connection');

            ws.on('message', (data) => {
                try {
                    const message = JSON.parse(data.toString());
                    this.handleMessage(ws, message);
                } catch (error) {
                    console.error('❌ Invalid message format:', error);
                }
            });

            ws.on('close', () => {
                console.log('🔌 WebSocket connection closed');
                this.handleDisconnection(ws);
            });

            ws.on('error', (error) => {
                console.error('❌ WebSocket error:', error);
            });

            // Send welcome message
            this.sendToClient(ws, {
                type: 'welcome',
                message: 'Connected to Remote Desktop WebSocket Server'
            });
        });

        this.server.listen(this.port, () => {
            console.log(`✅ WebSocket Server running on port ${this.port}`);
            console.log(`🌐 WebSocket URL: ws://localhost:${this.port}`);
        });
    }

    handleMessage(ws, message) {
        console.log(`📨 Received: ${message.type}`);

        switch (message.type) {
            case 'device_connect':
                this.handleDeviceConnect(ws, message);
                break;
            case 'controller_connect':
                this.handleControllerConnect(ws, message);
                break;
            case 'start_session':
                this.handleStartSession(ws, message);
                break;
            case 'end_session':
                this.handleEndSession(ws, message);
                break;
            case 'screen_frame':
                this.handleScreenFrame(ws, message);
                break;
            case 'mouse_move':
            case 'mouse_click':
            case 'key_press':
                this.handleInputEvent(ws, message);
                break;
            case 'ping':
                this.sendToClient(ws, { type: 'pong' });
                break;
            default:
                console.log(`❓ Unknown message type: ${message.type}`);
        }
    }

    handleDeviceConnect(ws, message) {
        const deviceId = message.deviceId;
        const deviceName = message.deviceName || 'Unknown Device';

        this.devices.set(deviceId, {
            ws: ws,
            deviceId: deviceId,
            deviceName: deviceName,
            status: 'online',
            connectedAt: new Date()
        });

        ws.deviceId = deviceId;
        ws.clientType = 'device';

        console.log(`📱 Device connected: ${deviceName} (${deviceId})`);

        this.sendToClient(ws, {
            type: 'device_registered',
            deviceId: deviceId,
            message: 'Device registered successfully'
        });

        // Notify all controllers about new device
        this.broadcastToControllers({
            type: 'device_online',
            deviceId: deviceId,
            deviceName: deviceName
        });
    }

    handleControllerConnect(ws, message) {
        const controllerId = message.controllerId || `controller_${Date.now()}`;

        this.controllers.set(controllerId, {
            ws: ws,
            controllerId: controllerId,
            connectedAt: new Date()
        });

        ws.controllerId = controllerId;
        ws.clientType = 'controller';

        console.log(`🎮 Controller connected: ${controllerId}`);

        // Send list of available devices
        const deviceList = Array.from(this.devices.values()).map(device => ({
            deviceId: device.deviceId,
            deviceName: device.deviceName,
            status: device.status
        }));

        this.sendToClient(ws, {
            type: 'device_list',
            devices: deviceList
        });
    }

    handleStartSession(ws, message) {
        const deviceId = message.deviceId;
        const device = this.devices.get(deviceId);

        if (!device) {
            this.sendToClient(ws, {
                type: 'error',
                message: 'Device not found'
            });
            return;
        }

        const sessionId = `session_${Date.now()}`;
        this.sessions.set(sessionId, {
            sessionId: sessionId,
            deviceId: deviceId,
            controllerId: ws.controllerId,
            startedAt: new Date()
        });

        console.log(`🎯 Starting session: ${sessionId} (${ws.controllerId} -> ${deviceId})`);

        // Notify device to start screen sharing
        this.sendToClient(device.ws, {
            type: 'start_screen_share',
            sessionId: sessionId,
            controllerId: ws.controllerId
        });

        // Notify controller
        this.sendToClient(ws, {
            type: 'session_started',
            sessionId: sessionId,
            deviceId: deviceId
        });
    }

    handleEndSession(ws, message) {
        const sessionId = message.sessionId;
        const session = this.sessions.get(sessionId);

        if (session) {
            const device = this.devices.get(session.deviceId);
            if (device) {
                this.sendToClient(device.ws, {
                    type: 'stop_screen_share',
                    sessionId: sessionId
                });
            }

            this.sessions.delete(sessionId);
            console.log(`🛑 Session ended: ${sessionId}`);
        }
    }

    handleScreenFrame(ws, message) {
        // Forward screen frame to controller
        const deviceId = ws.deviceId;
        const activeSession = Array.from(this.sessions.values())
            .find(session => session.deviceId === deviceId);

        if (activeSession) {
            const controller = this.controllers.get(activeSession.controllerId);
            if (controller) {
                this.sendToClient(controller.ws, message);
            }
        }
    }

    handleInputEvent(ws, message) {
        // Forward input event to device
        if (ws.clientType === 'controller') {
            const activeSession = Array.from(this.sessions.values())
                .find(session => session.controllerId === ws.controllerId);

            if (activeSession) {
                const device = this.devices.get(activeSession.deviceId);
                if (device) {
                    this.sendToClient(device.ws, message);
                }
            }
        }
    }

    handleDisconnection(ws) {
        if (ws.clientType === 'device' && ws.deviceId) {
            this.devices.delete(ws.deviceId);
            console.log(`📱 Device disconnected: ${ws.deviceId}`);

            // Notify controllers
            this.broadcastToControllers({
                type: 'device_offline',
                deviceId: ws.deviceId
            });

        } else if (ws.clientType === 'controller' && ws.controllerId) {
            this.controllers.delete(ws.controllerId);
            console.log(`🎮 Controller disconnected: ${ws.controllerId}`);

            // End any active sessions
            const activeSessions = Array.from(this.sessions.values())
                .filter(session => session.controllerId === ws.controllerId);

            activeSessions.forEach(session => {
                this.handleEndSession(ws, { sessionId: session.sessionId });
            });
        }
    }

    sendToClient(ws, message) {
        if (ws.readyState === WebSocket.OPEN) {
            ws.send(JSON.stringify(message));
        }
    }

    broadcastToControllers(message) {
        this.controllers.forEach(controller => {
            this.sendToClient(controller.ws, message);
        });
    }

    getStatus() {
        return {
            devices: this.devices.size,
            controllers: this.controllers.size,
            sessions: this.sessions.size,
            uptime: process.uptime()
        };
    }
}

// Start the server
if (require.main === module) {
    const server = new RemoteDesktopWebSocketServer();
    server.start();

    // Handle graceful shutdown
    process.on('SIGINT', () => {
        console.log('\n🛑 Shutting down WebSocket server...');
        process.exit(0);
    });

    // Status logging
    setInterval(() => {
        const status = server.getStatus();
        console.log(`📊 Status: ${status.devices} devices, ${status.controllers} controllers, ${status.sessions} sessions`);
    }, 30000);
}

module.exports = RemoteDesktopWebSocketServer;
