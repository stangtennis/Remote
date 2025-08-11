const WebSocket = require('ws');
const http = require('http');
const path = require('path');
const fs = require('fs');

console.log('🚀 Starting WebRTC Signaling Server...');

// Create HTTP server for serving static files
const server = http.createServer((req, res) => {
    let filePath = path.join(__dirname, 'public', req.url === '/' ? 'webrtc-dashboard.html' : req.url);
    
    // Security: prevent directory traversal
    if (!filePath.startsWith(path.join(__dirname, 'public'))) {
        res.writeHead(403);
        res.end('Forbidden');
        return;
    }
    
    fs.readFile(filePath, (err, data) => {
        if (err) {
            res.writeHead(404);
            res.end('File not found');
            return;
        }
        
        // Set content type based on file extension
        const ext = path.extname(filePath);
        const contentTypes = {
            '.html': 'text/html',
            '.js': 'application/javascript',
            '.css': 'text/css',
            '.json': 'application/json'
        };
        
        res.writeHead(200, { 'Content-Type': contentTypes[ext] || 'text/plain' });
        res.end(data);
    });
});

// Create WebSocket server for signaling
const wss = new WebSocket.Server({ server });

// Store connected clients
const clients = new Map();
const rooms = new Map();

console.log('✅ WebSocket server created');

wss.on('connection', (ws, req) => {
    const clientId = generateClientId();
    clients.set(clientId, ws);
    
    console.log(`🔗 Client connected: ${clientId} (${clients.size} total)`);
    
    // Send client their ID
    ws.send(JSON.stringify({
        type: 'client-id',
        clientId: clientId
    }));
    
    ws.on('message', (message) => {
        try {
            const data = JSON.parse(message);
            handleSignalingMessage(clientId, data);
        } catch (error) {
            console.error(`❌ Invalid message from ${clientId}:`, error.message);
            ws.send(JSON.stringify({
                type: 'error',
                message: 'Invalid JSON message'
            }));
        }
    });
    
    ws.on('close', () => {
        console.log(`🔌 Client disconnected: ${clientId}`);
        clients.delete(clientId);
        
        // Remove from any rooms
        for (const [roomId, room] of rooms.entries()) {
            if (room.host === clientId || room.viewers.has(clientId)) {
                if (room.host === clientId) {
                    // Host disconnected, notify all viewers
                    room.viewers.forEach(viewerId => {
                        const viewerWs = clients.get(viewerId);
                        if (viewerWs) {
                            viewerWs.send(JSON.stringify({
                                type: 'host-disconnected'
                            }));
                        }
                    });
                    rooms.delete(roomId);
                    console.log(`🏠 Room ${roomId} closed (host disconnected)`);
                } else {
                    room.viewers.delete(clientId);
                    console.log(`👁️ Viewer left room ${roomId}`);
                }
            }
        }
    });
    
    ws.on('error', (error) => {
        console.error(`❌ WebSocket error for ${clientId}:`, error.message);
    });
});

function handleSignalingMessage(clientId, data) {
    const ws = clients.get(clientId);
    
    switch (data.type) {
        case 'create-room':
            createRoom(clientId, data.roomId);
            break;
            
        case 'join-room':
            joinRoom(clientId, data.roomId);
            break;
            
        case 'offer':
            forwardToRoom(clientId, data, 'offer');
            break;
            
        case 'answer':
            forwardToRoom(clientId, data, 'answer');
            break;
            
        case 'ice-candidate':
            forwardToRoom(clientId, data, 'ice-candidate');
            break;
            
        case 'list-rooms':
            listRooms(clientId);
            break;
            
        default:
            console.log(`⚠️ Unknown message type from ${clientId}: ${data.type}`);
            ws.send(JSON.stringify({
                type: 'error',
                message: `Unknown message type: ${data.type}`
            }));
    }
}

function createRoom(clientId, roomId) {
    const ws = clients.get(clientId);
    
    if (!roomId) {
        roomId = generateRoomId();
    }
    
    if (rooms.has(roomId)) {
        ws.send(JSON.stringify({
            type: 'error',
            message: 'Room already exists'
        }));
        return;
    }
    
    rooms.set(roomId, {
        id: roomId,
        host: clientId,
        viewers: new Set(),
        created: Date.now()
    });
    
    console.log(`🏠 Room created: ${roomId} by ${clientId}`);
    
    ws.send(JSON.stringify({
        type: 'room-created',
        roomId: roomId,
        role: 'host'
    }));
}

function joinRoom(clientId, roomId) {
    const ws = clients.get(clientId);
    const room = rooms.get(roomId);
    
    if (!room) {
        ws.send(JSON.stringify({
            type: 'error',
            message: 'Room not found'
        }));
        return;
    }
    
    if (room.host === clientId) {
        ws.send(JSON.stringify({
            type: 'error',
            message: 'You are already the host of this room'
        }));
        return;
    }
    
    room.viewers.add(clientId);
    console.log(`👁️ ${clientId} joined room ${roomId}`);
    
    ws.send(JSON.stringify({
        type: 'room-joined',
        roomId: roomId,
        role: 'viewer'
    }));
    
    // Notify host of new viewer
    const hostWs = clients.get(room.host);
    if (hostWs) {
        hostWs.send(JSON.stringify({
            type: 'viewer-joined',
            viewerId: clientId
        }));
    }
}

function forwardToRoom(clientId, data, messageType) {
    const room = findRoomForClient(clientId);
    
    if (!room) {
        const ws = clients.get(clientId);
        ws.send(JSON.stringify({
            type: 'error',
            message: 'Not in any room'
        }));
        return;
    }
    
    const message = {
        type: messageType,
        from: clientId,
        ...data
    };
    
    if (room.host === clientId) {
        // Host sending to viewers
        room.viewers.forEach(viewerId => {
            const viewerWs = clients.get(viewerId);
            if (viewerWs) {
                viewerWs.send(JSON.stringify(message));
            }
        });
    } else {
        // Viewer sending to host
        const hostWs = clients.get(room.host);
        if (hostWs) {
            hostWs.send(JSON.stringify(message));
        }
    }
    
    console.log(`📤 Forwarded ${messageType} from ${clientId} in room ${room.id}`);
}

function listRooms(clientId) {
    const ws = clients.get(clientId);
    const roomList = Array.from(rooms.values()).map(room => ({
        id: room.id,
        viewerCount: room.viewers.size,
        created: room.created
    }));
    
    ws.send(JSON.stringify({
        type: 'room-list',
        rooms: roomList
    }));
}

function findRoomForClient(clientId) {
    for (const room of rooms.values()) {
        if (room.host === clientId || room.viewers.has(clientId)) {
            return room;
        }
    }
    return null;
}

function generateClientId() {
    return 'client_' + Math.random().toString(36).substr(2, 9) + '_' + Date.now();
}

function generateRoomId() {
    return 'room_' + Math.random().toString(36).substr(2, 6).toUpperCase();
}

// Start server
const PORT = process.env.PORT || 8081;
server.listen(PORT, () => {
    console.log(`🌐 WebRTC Signaling Server running on http://localhost:${PORT}`);
    console.log(`📱 Dashboard available at: http://localhost:${PORT}/webrtc-dashboard.html`);
    console.log('🎯 Ready for WebRTC connections!');
});

// Graceful shutdown
process.on('SIGINT', () => {
    console.log('\n🛑 Shutting down signaling server...');
    wss.clients.forEach(ws => {
        ws.close(1000, 'Server shutting down');
    });
    server.close(() => {
        console.log('✅ Server closed gracefully');
        process.exit(0);
    });
});

// Error handling
process.on('uncaughtException', (error) => {
    console.error('❌ Uncaught Exception:', error);
    process.exit(1);
});

process.on('unhandledRejection', (reason, promise) => {
    console.error('❌ Unhandled Rejection at:', promise, 'reason:', reason);
});
