import socketio
import uvicorn
from fastapi import FastAPI, WebSocket
from typing import Dict, Set
import json
import os
import platform
import logging
import hashlib

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

# Secure admin key hashing
ADMIN_KEY_HASH = hashlib.sha256(b'your_admin_key').hexdigest()  # Replace with environment variable

# Create FastAPI and Socket.IO instances
app = FastAPI()
sio = socketio.AsyncServer(async_mode='asgi')
socket_app = socketio.ASGIApp(sio, app)

class SessionManager:
    """
    Manages client and admin sessions.
    
    Attributes:
        client_sessions (dict): Maps client IDs to session IDs
        admin_sessions (dict): Maps admin IDs to session data
    """
    def __init__(self):
        self.client_sessions = {}
        self.admin_sessions = {}

    def register_client(self, sid, client_id):
        """
        Register a new client session.
        """
        self.client_sessions[sid] = client_id

    def register_admin(self, sid):
        """
        Register a new admin session.
        """
        self.admin_sessions[sid] = {'viewing': None}

    def get_client_list(self):
        """
        Get list of connected clients.
        """
        return list(set(self.client_sessions.values()))

session_manager = SessionManager()

# Store connected clients and admins
connected_clients: Dict[str, dict] = {}
connected_admins: Set[str] = set()

@app.get("/")
async def root():
    """
    Root endpoint for health check.
    
    Returns:
        dict: Status message
    """
    return {"message": "Remote Desktop Server Running"}

@sio.event
async def connect(sid, environ):
    logger.info(f"Client connected: {sid}")
    await sio.emit('request_registration', {}, room=sid)

@sio.event
async def disconnect(sid):
    try:
        if sid in connected_clients:
            del connected_clients[sid]
            await notify_admins("client_disconnected", {"client_id": sid})
        elif sid in connected_admins:
            connected_admins.remove(sid)
        logger.info(f"Client disconnected: {sid}")
    except Exception as e:
        logger.error(f"Error during disconnect: {e}")

@sio.event
async def register_client(sid, data):
    try:
        client_id = sid
        connected_clients[client_id] = {
            "hostname": data.get("hostname", platform.node()),
            "username": data.get("username", "Unknown"),
            "ip": data.get("ip", "Unknown")
        }
        session_manager.register_client(sid, client_id)
        await notify_admins("client_connected", {
            "client_id": client_id,
            "client_info": connected_clients[client_id]
        })
    except Exception as e:
        logger.error(f"Error registering client: {e}")
        await sio.emit('registration_error', {"message": "Registration failed"}, room=sid)

@sio.event
async def register_admin(sid, data=None):
    try:
        if not data:
            return False
        provided_key = data.get("admin_key")
        if hashlib.sha256(provided_key.encode()).hexdigest() == ADMIN_KEY_HASH:
            connected_admins.add(sid)
            session_manager.register_admin(sid)
            await sio.emit("client_list", session_manager.get_client_list(), room=sid)
            logger.info(f"Admin connected: {sid}")
            return True
        logger.warning(f"Invalid admin key attempt from {sid}")
        return False
    except Exception as e:
        logger.error(f"Error during admin registration: {e}")
        return False

async def notify_admins(event, data):
    try:
        for admin_sid in connected_admins:
            await sio.emit(event, data, room=admin_sid)
    except Exception as e:
        logger.error(f"Error notifying admins: {e}")

@sio.event
async def admin_command(sid, data):
    try:
        if sid in connected_admins:
            target_client = data.get('client_id')
            command = data.get('command')
            if target_client in connected_clients:
                await sio.emit('admin_command', {'command': command}, room=target_client)
                logger.info(f"Admin {sid} sent command to client {target_client}")
                return {"status": "success", "message": "Command sent"}
            return {"status": "error", "message": "Client not found"}
        return {"status": "error", "message": "Unauthorized"}
    except Exception as e:
        logger.error(f"Error processing admin command: {e}")
        return {"status": "error", "message": "Command failed"}

@sio.event
async def admin_view_client(sid, data):
    try:
        if sid in connected_admins:
            client_id = data.get('client_id')
            if client_id in connected_clients:
                session_manager.admin_sessions[sid]['viewing'] = client_id
                await sio.emit('admin_viewing_client', {'client_id': client_id}, room=sid)
                logger.info(f"Admin {sid} is viewing client {client_id}")
                return {"status": "success"}
            return {"status": "error", "message": "Client not found"}
        return {"status": "error", "message": "Unauthorized"}
    except Exception as e:
        logger.error(f"Error processing admin view request: {e}")
        return {"status": "error", "message": "View request failed"}

@sio.event
async def screen_data(sid, data):
    try:
        if sid in connected_clients:
            client_id = sid
            for admin_sid in connected_admins:
                admin_data = session_manager.admin_sessions.get(admin_sid, {})
                if admin_data.get('viewing') == client_id:
                    await sio.emit('screen_data', {
                        'client_id': client_id,
                        'image': data['image']
                    }, room=admin_sid)
    except Exception as e:
        logger.error(f"Error handling screen data: {e}")

if __name__ == "__main__":
    # Always use these settings for network access
    uvicorn.run(
        socket_app,
        host="192.168.1.90",  # Bind to specific IP address
        port=8000,
        log_level="info",
        proxy_headers=True,
        forwarded_allow_ips='*',
        timeout_keep_alive=60,
        timeout_graceful_shutdown=30
    )
