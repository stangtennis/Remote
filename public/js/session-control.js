// Session Control Module for WebRTC Remote Desktop
console.log('🎮 Loading Session Control Module...');

// Session state
let isSessionActive = false;
let currentSessionId = null;
let currentDeviceId = null;
let inputEnabled = false;
let mousePosition = { x: 0, y: 0 };
let keyboardModifiers = {
    ctrl: false,
    alt: false,
    shift: false,
    meta: false
};

// Input event handlers
let onInputCommand = null;
let onSessionStateChange = null;

// Initialize session control
function initializeSessionControl(deviceId, sessionId) {
    console.log(`🎮 Initializing session control for device: ${deviceId}`);
    
    currentDeviceId = deviceId;
    currentSessionId = sessionId;
    isSessionActive = true;
    
    // Set up input event listeners
    setupInputEventListeners();
    
    console.log('✅ Session control initialized');
    updateSessionState('active');
}

// Set up input event listeners
function setupInputEventListeners() {
    console.log('🔧 Setting up input event listeners...');
    
    // Get video element for coordinate calculation
    const videoElement = document.getElementById('remoteVideo');
    if (!videoElement) {
        console.error('❌ Remote video element not found');
        return;
    }
    
    // Mouse events
    videoElement.addEventListener('mousedown', handleMouseDown);
    videoElement.addEventListener('mouseup', handleMouseUp);
    videoElement.addEventListener('mousemove', handleMouseMove);
    videoElement.addEventListener('wheel', handleMouseWheel);
    videoElement.addEventListener('contextmenu', (e) => e.preventDefault());
    
    // Keyboard events (document level)
    document.addEventListener('keydown', handleKeyDown);
    document.addEventListener('keyup', handleKeyUp);
    
    // Focus management
    videoElement.addEventListener('click', () => {
        videoElement.focus();
        enableInput();
    });
    
    videoElement.addEventListener('blur', () => {
        disableInput();
    });
    
    // Make video element focusable
    videoElement.tabIndex = 0;
    
    console.log('✅ Input event listeners set up');
}

// Handle mouse down events
function handleMouseDown(event) {
    if (!inputEnabled || !isSessionActive) return;
    
    event.preventDefault();
    
    const coords = getRelativeCoordinates(event);
    const button = getMouseButton(event.button);
    
    console.log(`🖱️ Mouse down: ${button} at (${coords.x}, ${coords.y})`);
    
    const command = {
        type: 'mouse_input',
        action: 'down',
        button: button,
        x: coords.x,
        y: coords.y,
        timestamp: Date.now()
    };
    
    sendInputCommand(command);
}

// Handle mouse up events
function handleMouseUp(event) {
    if (!inputEnabled || !isSessionActive) return;
    
    event.preventDefault();
    
    const coords = getRelativeCoordinates(event);
    const button = getMouseButton(event.button);
    
    console.log(`🖱️ Mouse up: ${button} at (${coords.x}, ${coords.y})`);
    
    const command = {
        type: 'mouse_input',
        action: 'up',
        button: button,
        x: coords.x,
        y: coords.y,
        timestamp: Date.now()
    };
    
    sendInputCommand(command);
}

// Handle mouse move events
function handleMouseMove(event) {
    if (!inputEnabled || !isSessionActive) return;
    
    const coords = getRelativeCoordinates(event);
    mousePosition = coords;
    
    // Throttle mouse move events
    if (!handleMouseMove.lastSent || Date.now() - handleMouseMove.lastSent > 16) {
        const command = {
            type: 'mouse_input',
            action: 'move',
            x: coords.x,
            y: coords.y,
            timestamp: Date.now()
        };
        
        sendInputCommand(command);
        handleMouseMove.lastSent = Date.now();
    }
}

// Handle mouse wheel events
function handleMouseWheel(event) {
    if (!inputEnabled || !isSessionActive) return;
    
    event.preventDefault();
    
    const coords = getRelativeCoordinates(event);
    
    console.log(`🖱️ Mouse wheel: ${event.deltaY} at (${coords.x}, ${coords.y})`);
    
    const command = {
        type: 'mouse_input',
        action: 'wheel',
        x: coords.x,
        y: coords.y,
        deltaX: event.deltaX,
        deltaY: event.deltaY,
        timestamp: Date.now()
    };
    
    sendInputCommand(command);
}

// Handle key down events
function handleKeyDown(event) {
    if (!inputEnabled || !isSessionActive) return;
    
    // Update modifier state
    updateModifiers(event);
    
    // Don't prevent certain browser shortcuts
    if (shouldAllowBrowserShortcut(event)) {
        return;
    }
    
    event.preventDefault();
    
    console.log(`⌨️ Key down: ${event.key} (code: ${event.code})`);
    
    const command = {
        type: 'keyboard_input',
        action: 'down',
        key: event.key,
        code: event.code,
        modifiers: { ...keyboardModifiers },
        timestamp: Date.now()
    };
    
    sendInputCommand(command);
}

// Handle key up events
function handleKeyUp(event) {
    if (!inputEnabled || !isSessionActive) return;
    
    // Update modifier state
    updateModifiers(event);
    
    // Don't prevent certain browser shortcuts
    if (shouldAllowBrowserShortcut(event)) {
        return;
    }
    
    event.preventDefault();
    
    console.log(`⌨️ Key up: ${event.key} (code: ${event.code})`);
    
    const command = {
        type: 'keyboard_input',
        action: 'up',
        key: event.key,
        code: event.code,
        modifiers: { ...keyboardModifiers },
        timestamp: Date.now()
    };
    
    sendInputCommand(command);
}

// Get relative coordinates within video element
function getRelativeCoordinates(event) {
    const videoElement = event.target;
    const rect = videoElement.getBoundingClientRect();
    
    // Calculate relative position (0-1)
    const relativeX = (event.clientX - rect.left) / rect.width;
    const relativeY = (event.clientY - rect.top) / rect.height;
    
    // Clamp to valid range
    const x = Math.max(0, Math.min(1, relativeX));
    const y = Math.max(0, Math.min(1, relativeY));
    
    return { x, y };
}

// Get mouse button name
function getMouseButton(buttonCode) {
    switch (buttonCode) {
        case 0: return 'left';
        case 1: return 'middle';
        case 2: return 'right';
        default: return 'unknown';
    }
}

// Update keyboard modifiers
function updateModifiers(event) {
    keyboardModifiers.ctrl = event.ctrlKey;
    keyboardModifiers.alt = event.altKey;
    keyboardModifiers.shift = event.shiftKey;
    keyboardModifiers.meta = event.metaKey;
}

// Check if browser shortcut should be allowed
function shouldAllowBrowserShortcut(event) {
    // Allow F12 (dev tools)
    if (event.key === 'F12') return true;
    
    // Allow Ctrl+Shift+I (dev tools)
    if (event.ctrlKey && event.shiftKey && event.key === 'I') return true;
    
    // Allow Ctrl+R (refresh)
    if (event.ctrlKey && event.key === 'r') return true;
    
    // Allow Alt+Tab (window switching)
    if (event.altKey && event.key === 'Tab') return true;
    
    return false;
}

// Send input command
function sendInputCommand(command) {
    if (onInputCommand) {
        onInputCommand(command);
    } else {
        console.warn('⚠️ No input command handler set');
    }
}

// Enable input capture
function enableInput() {
    inputEnabled = true;
    console.log('✅ Input capture enabled');
    
    // Visual feedback
    const videoElement = document.getElementById('remoteVideo');
    if (videoElement) {
        videoElement.style.cursor = 'none';
        videoElement.style.border = '2px solid #00ff00';
    }
    
    updateSessionState('input_enabled');
}

// Disable input capture
function disableInput() {
    inputEnabled = false;
    console.log('🚫 Input capture disabled');
    
    // Visual feedback
    const videoElement = document.getElementById('remoteVideo');
    if (videoElement) {
        videoElement.style.cursor = 'default';
        videoElement.style.border = '1px solid #ccc';
    }
    
    updateSessionState('input_disabled');
}

// Toggle input capture
function toggleInput() {
    if (inputEnabled) {
        disableInput();
    } else {
        enableInput();
    }
}

// Send special commands
function sendSpecialCommand(commandType, data = {}) {
    console.log(`🎯 Sending special command: ${commandType}`);
    
    const command = {
        type: 'special_command',
        command: commandType,
        data: data,
        timestamp: Date.now()
    };
    
    sendInputCommand(command);
}

// Common special commands
function sendCtrlAltDel() {
    sendSpecialCommand('ctrl_alt_del');
}

function sendAltTab() {
    sendSpecialCommand('alt_tab');
}

function sendWinKey() {
    sendSpecialCommand('win_key');
}

function sendScreenshot() {
    sendSpecialCommand('screenshot');
}

function sendClipboard(text) {
    sendSpecialCommand('clipboard', { text: text });
}

// Session management
function startSession(deviceId) {
    console.log(`🎮 Starting session with device: ${deviceId}`);
    
    currentDeviceId = deviceId;
    currentSessionId = `session_${Date.now()}`;
    isSessionActive = true;
    
    updateSessionState('starting');
    
    return currentSessionId;
}

function endSession() {
    console.log('🛑 Ending session');
    
    disableInput();
    isSessionActive = false;
    
    // Clean up event listeners
    cleanupEventListeners();
    
    updateSessionState('ended');
    
    const sessionId = currentSessionId;
    currentSessionId = null;
    currentDeviceId = null;
    
    return sessionId;
}

// Clean up event listeners
function cleanupEventListeners() {
    console.log('🧹 Cleaning up event listeners...');
    
    const videoElement = document.getElementById('remoteVideo');
    if (videoElement) {
        videoElement.removeEventListener('mousedown', handleMouseDown);
        videoElement.removeEventListener('mouseup', handleMouseUp);
        videoElement.removeEventListener('mousemove', handleMouseMove);
        videoElement.removeEventListener('wheel', handleMouseWheel);
        
        // Reset styles
        videoElement.style.cursor = 'default';
        videoElement.style.border = '1px solid #ccc';
    }
    
    document.removeEventListener('keydown', handleKeyDown);
    document.removeEventListener('keyup', handleKeyUp);
    
    console.log('✅ Event listeners cleaned up');
}

// Update session state
function updateSessionState(newState) {
    console.log(`📊 Session state updated: ${newState}`);
    if (onSessionStateChange) {
        onSessionStateChange(newState, currentSessionId, currentDeviceId);
    }
}

// Get session info
function getSessionInfo() {
    return {
        isActive: isSessionActive,
        sessionId: currentSessionId,
        deviceId: currentDeviceId,
        inputEnabled: inputEnabled,
        mousePosition: mousePosition,
        modifiers: keyboardModifiers
    };
}

// Export functions for use in other modules
window.SessionControl = {
    initializeSessionControl,
    startSession,
    endSession,
    enableInput,
    disableInput,
    toggleInput,
    
    // Special commands
    sendCtrlAltDel,
    sendAltTab,
    sendWinKey,
    sendScreenshot,
    sendClipboard,
    sendSpecialCommand,
    
    // Event handler setters
    setInputCommandHandler: (handler) => { onInputCommand = handler; },
    setSessionStateHandler: (handler) => { onSessionStateChange = handler; },
    
    // State getters
    getSessionInfo,
    isInputEnabled: () => inputEnabled,
    isSessionActive: () => isSessionActive
};

console.log('✅ Session Control Module loaded successfully');
