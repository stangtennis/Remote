#!/usr/bin/env python3

import eventlet
eventlet.monkey_patch(socket=True, select=True)

import os
import sys
import json
import time
import socket
import logging
import secrets
import datetime
import traceback
import threading
import ipaddress
import uuid
from flask import Flask, render_template, request, redirect, url_for, session, Response
from flask_socketio import SocketIO, emit
from PIL import Image
import io
import base64
import numpy as np
import requests

from session_manager import SessionManager
from input_handler import InputHandler

# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

def get_local_ip():
    """Get local IP address"""
    try:
        # Get all network interfaces
        hostname = socket.gethostname()
        ip_list = socket.gethostbyname_ex(hostname)[2]
        
        # Filter out localhost and try to find the LAN IP
        for ip in ip_list:
            if not ip.startswith('127.'):
                return ip
        return '127.0.0.1'  # Fallback to localhost
    except Exception as e:
        logger.error(f"Error getting local IP: {e}")
        return '127.0.0.1'

def get_public_ip():
    """Get public IP address"""
    try:
        response = requests.get('https://api.ipify.org')
        return response.text
    except:
        return None

# Initialize Flask app
app = Flask(__name__)
app.config['SECRET_KEY'] = 'secret!'

# Initialize SocketIO with CORS allowed
socketio = SocketIO(app, cors_allowed_origins="*", async_mode='eventlet')

# Initialize managers
session_manager = SessionManager()
input_handler = InputHandler()

# Store active sessions
active_sessions = {}

@app.before_request
def before_request():
    pass

@app.after_request
def add_header(response):
    """Add headers to allow screen capture API."""
    # Allow screen capture
    response.headers['Feature-Policy'] = 'display-capture *'
    response.headers['Permissions-Policy'] = 'display-capture=*'
    
    # Allow necessary features
    response.headers['Feature-Policy'] += '; microphone *; camera *'
    response.headers['Permissions-Policy'] += '; microphone=*; camera=*'
    
    # CORS headers
    response.headers['Access-Control-Allow-Origin'] = '*'
    response.headers['Access-Control-Allow-Methods'] = 'GET, POST, OPTIONS'
    response.headers['Access-Control-Allow-Headers'] = 'Content-Type'
    
    # Security headers
    response.headers['Cross-Origin-Embedder-Policy'] = 'require-corp'
    response.headers['Cross-Origin-Opener-Policy'] = 'same-origin'
    response.headers['Cross-Origin-Resource-Policy'] = 'cross-origin'
    
    return response

@app.route('/')
def index():
    """Render the index page"""
    local_ip = get_local_ip()
    return render_template('index.html', local_ip=local_ip)

@app.route('/controller')
def controller():
    """Render the controller page"""
    local_ip = get_local_ip()
    return render_template('controller.html', local_ip=local_ip)

@socketio.on('connect')
def handle_connect():
    """Handle client connection"""
    logger.info(f"Client connected: {request.sid}")

@socketio.on('disconnect')
def handle_disconnect():
    """Handle client disconnection"""
    logger.info(f"Client disconnected: {request.sid}")
    # Clean up any active sessions
    for session_id, session in list(active_sessions.items()):
        if session.get('sharer_sid') == request.sid:
            del active_sessions[session_id]
            broadcast_sessions()
            logger.info(f"Removed session {session_id} due to sharer disconnect")
        elif session.get('controller_sid') == request.sid:
            session['controller_sid'] = None
            broadcast_sessions()
            logger.info(f"Removed controller from session {session_id}")

@socketio.on('start_sharing')
def handle_start_sharing():
    """Handle start of screen sharing"""
    session_id = str(uuid.uuid4())
    active_sessions[session_id] = {
        'sharer_sid': request.sid,
        'controller_sid': None,
        'timestamp': time.time()
    }
    socketio.emit('session_id', session_id, room=request.sid)
    logger.info(f"Started sharing session: {session_id}, total sessions: {len(active_sessions)}")
    # Broadcast updated sessions list to all clients
    broadcast_sessions()

@socketio.on('stop_sharing')
def handle_stop_sharing(data):
    """Handle end of screen sharing"""
    session_id = data.get('session_id')
    if session_id in active_sessions:
        session = active_sessions[session_id]
        if session.get('sharer_sid') == request.sid:
            del active_sessions[session_id]
            # Notify controller if connected
            if session.get('controller_sid'):
                socketio.emit('session_ended', room=session['controller_sid'])
            # Broadcast updated sessions list
            broadcast_sessions()
            logger.info(f"Stopped sharing session: {session_id}, remaining sessions: {len(active_sessions)}")

@socketio.on('request_sessions')
def handle_request_sessions():
    """Handle request for active sessions"""
    logger.info(f"Sessions requested by {request.sid}, active sessions: {len(active_sessions)}")
    # Clean up stale sessions
    current_time = time.time()
    for session_id, session in list(active_sessions.items()):
        if current_time - session.get('timestamp', 0) > 300:  # 5 minutes
            del active_sessions[session_id]
            logger.info(f"Removed stale session {session_id}")
    
    # Get available sessions
    sessions_list = [
        session_id for session_id, session in active_sessions.items()
        if not session.get('controller_sid')
    ]
    
    logger.info(f"Sending sessions list to {request.sid}: {sessions_list}")
    socketio.emit('sessions_list', sessions_list, room=request.sid)

def broadcast_sessions(target_sid=None):
    """Broadcast active sessions to controllers"""
    # Get available sessions
    sessions_list = [
        session_id for session_id, session in active_sessions.items()
        if not session.get('controller_sid')
    ]
    
    logger.info(f"Broadcasting sessions list: {sessions_list}")
    if target_sid:
        socketio.emit('sessions_list', sessions_list, room=target_sid)
    else:
        socketio.emit('sessions_list', sessions_list, room=None)

@socketio.on('join_session')
def handle_join_session(data):
    """Handle controller joining a session"""
    session_id = data.get('session_id')
    if session_id in active_sessions:
        session = active_sessions[session_id]
        if not session.get('controller_sid'):
            session['controller_sid'] = request.sid
            logger.info(f"Controller {request.sid} joined session {session_id}")
            socketio.emit('controller_joined', room=session['sharer_sid'])
            # Broadcast updated sessions list
            broadcast_sessions()
        else:
            logger.warning(f"Session {session_id} already has a controller")
    else:
        logger.warning(f"Invalid session ID: {session_id}")

@socketio.on('leave_session')
def handle_leave_session(data):
    """Handle controller leaving a session"""
    session_id = data.get('session_id')
    if session_id in active_sessions:
        session = active_sessions[session_id]
        if session.get('controller_sid') == request.sid:
            session['controller_sid'] = None
            logger.info(f"Controller {request.sid} left session {session_id}")
            socketio.emit('controller_left', room=session['sharer_sid'])
            # Broadcast updated sessions list
            broadcast_sessions()

@socketio.on('screen_data')
def handle_screen_data(data):
    """Handle screen data from sharer"""
    try:
        session_id = data.get('session_id')
        if session_id in active_sessions:
            session = active_sessions[session_id]
            controller_sid = session.get('controller_sid')
            
            if controller_sid:
                # Forward screen data to controller without unnecessary logging
                socketio.emit('screen_data', {
                    'image': data.get('image'),
                    'session_id': session_id
                }, room=controller_sid)
                
                # Update session timestamp
                session['timestamp'] = time.time()
    except Exception as e:
        logger.error(f"Error handling screen data: {str(e)}")

@socketio.on('mouse_event')
def handle_mouse_event(data):
    """Handle mouse events from controller"""
    try:
        session_id = data.get('session_id')
        if session_id in active_sessions:
            session = active_sessions[session_id]
            if session.get('sharer_sid'):
                # Forward mouse event to sharer
                socketio.emit('mouse_event', data, room=session['sharer_sid'])
                
                # Handle the mouse event
                event_type = data.get('type')
                x = data.get('x', 0)
                y = data.get('y', 0)
                button = data.get('button', 'left')
                
                # Use eventlet for non-blocking input handling
                if event_type == 'wheel':
                    wheel_delta = -120 if data.get('deltaY', 0) > 0 else 120
                    eventlet.spawn(input_handler.handle_mouse_event, x, y, event_type, wheel_delta=wheel_delta)
                else:
                    eventlet.spawn(input_handler.handle_mouse_event, x, y, event_type, button)
    except Exception as e:
        logger.error(f"Error handling mouse event: {str(e)}")

@socketio.on('keyboard_event')
def handle_keyboard_event(data):
    """Handle keyboard events from controller"""
    try:
        session_id = data.get('session_id')
        if session_id in active_sessions:
            session = active_sessions[session_id]
            if session.get('sharer_sid'):
                # Forward keyboard event to sharer
                socketio.emit('keyboard_event', data, room=session['sharer_sid'])
                
                # Handle the keyboard event locally
                key = data.get('key')
                event_type = data.get('type')
                ctrl = data.get('ctrl', False)
                alt = data.get('alt', False)
                shift = data.get('shift', False)
                
                input_handler.handle_key(key, event_type, ctrl, alt, shift)
            else:
                logger.warning(f"No sharer connected for session {session_id}")
        else:
            logger.warning(f"Invalid session ID for keyboard event: {session_id}")
    except Exception as e:
        logger.error(f"Error handling keyboard event: {str(e)}")
        logger.exception(e)

@socketio.on('mouse_move')
def handle_mouse_move(data):
    try:
        x = data.get('x', 0)
        y = data.get('y', 0)
        input_handler.move_mouse(x, y)
    except Exception as e:
        logger.error(f"Error moving mouse: {str(e)}")

@socketio.on('mouse_click')
def handle_mouse_click(data):
    try:
        button = data.get('button', 'left')
        action = data.get('action', 'up')
        x = data.get('x', 0)
        y = data.get('y', 0)
        
        input_handler.move_mouse(x, y)
        
        if button == 'left':
            if action == 'down':
                input_handler.click_mouse('left', 'down')
            else:
                input_handler.click_mouse('left', 'up')
        elif button == 'right':
            if action == 'down':
                input_handler.click_mouse('right', 'down')
            else:
                input_handler.click_mouse('right', 'up')
    except Exception as e:
        logger.error(f"Error clicking mouse: {str(e)}")

@socketio.on('key_press')
def handle_key_press(data):
    try:
        key = data.get('key')
        if key:
            input_handler.press_key(key, data.get('action'))
    except Exception as e:
        logger.error(f"Error handling key press: {str(e)}")

if __name__ == "__main__":
    try:
        # Initialize logging
        logging.basicConfig(level=logging.INFO)
        
        # Get IP addresses
        local_ip = get_local_ip()
        public_ip = get_public_ip()
        logger.info(f"Local IP: {local_ip}")
        logger.info(f"Public IP: {public_ip}")
        
        # Configure Flask
        app.config['SESSION_TYPE'] = 'filesystem'
        
        # Print access information
        PORT = 7080
        print("\nAccess the application at:")
        print(f"   Local: http://localhost:{PORT}")
        print(f"   LAN:   http://{local_ip}:{PORT}\n")
        
        # Run server
        socketio.run(app, host='0.0.0.0', port=PORT, debug=True)
        
    except Exception as e:
        logger.error(f"Server error: {str(e)}")
        traceback.print_exc()
        sys.exit(1)
