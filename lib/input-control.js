/**
 * Professional Input Control Module
 * Real mouse and keyboard control with robotjs for precise remote input
 * Replaces mock input handling with actual system control like TeamViewer
 */

const robot = require('robotjs');
const { performance } = require('perf_hooks');

class ProfessionalInputControl {
    constructor(options = {}) {
        this.enabled = options.enabled !== false;
        this.mouseSensitivity = options.mouseSensitivity || 1.0;
        this.keyboardDelay = options.keyboardDelay || 0;
        this.inputLagCompensation = options.inputLagCompensation !== false;
        
        // Performance tracking
        this.stats = {
            totalMouseEvents: 0,
            totalKeyboardEvents: 0,
            totalInputTime: 0,
            averageInputLag: 0,
            lastInputTime: 0
        };
        
        // Screen dimensions for coordinate mapping
        this.screenSize = null;
        
        // Input validation
        this.maxMouseSpeed = 10000; // pixels per second
        this.lastMousePosition = { x: 0, y: 0 };
        this.lastMouseTime = 0;
        
        console.log('🖱️ Professional Input Control initialized');
        console.log(`⚙️ Settings: Sensitivity: ${this.mouseSensitivity}x, Keyboard Delay: ${this.keyboardDelay}ms`);
    }

    /**
     * Initialize input control system
     * Get screen dimensions and set up robotjs
     */
    async initialize() {
        try {
            // Get screen dimensions for coordinate mapping
            this.screenSize = robot.getScreenSize();
            console.log(`🖥️ Screen size detected: ${this.screenSize.width}x${this.screenSize.height}`);
            
            // Configure robotjs settings for optimal performance
            robot.setMouseDelay(2); // Minimal delay for responsiveness
            robot.setKeyboardDelay(this.keyboardDelay);
            
            // Test mouse and keyboard functionality
            const currentMousePos = robot.getMousePos();
            console.log(`🖱️ Current mouse position: ${currentMousePos.x}, ${currentMousePos.y}`);
            
            console.log('✅ Professional Input Control initialized successfully');
            return true;
            
        } catch (error) {
            console.error('❌ Failed to initialize input control:', error.message);
            return false;
        }
    }

    /**
     * Handle mouse input events
     * @param {number} x - X coordinate (0-1 normalized or absolute pixels)
     * @param {number} y - Y coordinate (0-1 normalized or absolute pixels)
     * @param {string} button - Mouse button ('left', 'right', 'middle')
     * @param {string} action - Action type ('move', 'down', 'up', 'click', 'scroll')
     * @param {object} options - Additional options (scrollDirection, scrollAmount, etc.)
     */
    async handleMouseInput(x, y, button = 'left', action = 'move', options = {}) {
        if (!this.enabled) {
            return { success: false, message: 'Input control disabled' };
        }

        const startTime = performance.now();

        try {
            // Normalize coordinates if they appear to be relative (0-1 range)
            let absoluteX = x;
            let absoluteY = y;
            
            if (x <= 1 && y <= 1 && x >= 0 && y >= 0) {
                absoluteX = Math.round(x * this.screenSize.width);
                absoluteY = Math.round(y * this.screenSize.height);
            }

            // Apply sensitivity adjustment
            if (action === 'move' && this.mouseSensitivity !== 1.0) {
                const currentPos = robot.getMousePos();
                const deltaX = (absoluteX - currentPos.x) * this.mouseSensitivity;
                const deltaY = (absoluteY - currentPos.y) * this.mouseSensitivity;
                absoluteX = currentPos.x + deltaX;
                absoluteY = currentPos.y + deltaY;
            }

            // Validate mouse speed to prevent excessive movement
            if (this.inputLagCompensation && action === 'move') {
                const now = performance.now();
                if (this.lastMouseTime > 0) {
                    const timeDiff = now - this.lastMouseTime;
                    const distance = Math.sqrt(
                        Math.pow(absoluteX - this.lastMousePosition.x, 2) + 
                        Math.pow(absoluteY - this.lastMousePosition.y, 2)
                    );
                    const speed = distance / (timeDiff / 1000); // pixels per second
                    
                    if (speed > this.maxMouseSpeed) {
                        console.warn(`⚠️ Mouse speed too high: ${speed.toFixed(0)} px/s, limiting movement`);
                        return { success: false, message: 'Mouse movement too fast, ignored for safety' };
                    }
                }
                
                this.lastMousePosition = { x: absoluteX, y: absoluteY };
                this.lastMouseTime = now;
            }

            // Ensure coordinates are within screen bounds
            absoluteX = Math.max(0, Math.min(this.screenSize.width - 1, absoluteX));
            absoluteY = Math.max(0, Math.min(this.screenSize.height - 1, absoluteY));

            // Execute mouse action
            let result = { success: true, message: '' };
            
            switch (action) {
                case 'move':
                    robot.moveMouse(absoluteX, absoluteY);
                    result.message = `Mouse moved to ${absoluteX}, ${absoluteY}`;
                    break;
                    
                case 'down':
                    robot.moveMouse(absoluteX, absoluteY);
                    robot.mouseToggle('down', button);
                    result.message = `Mouse ${button} button pressed at ${absoluteX}, ${absoluteY}`;
                    break;
                    
                case 'up':
                    robot.moveMouse(absoluteX, absoluteY);
                    robot.mouseToggle('up', button);
                    result.message = `Mouse ${button} button released at ${absoluteX}, ${absoluteY}`;
                    break;
                    
                case 'click':
                    robot.moveMouse(absoluteX, absoluteY);
                    robot.mouseClick(button);
                    result.message = `Mouse ${button} clicked at ${absoluteX}, ${absoluteY}`;
                    break;
                    
                case 'scroll':
                    robot.moveMouse(absoluteX, absoluteY);
                    const scrollAmount = options.scrollAmount || 3;
                    const scrollDirection = options.scrollDirection || 'up';
                    robot.scrollMouse(scrollAmount, scrollDirection);
                    result.message = `Mouse scrolled ${scrollDirection} by ${scrollAmount} at ${absoluteX}, ${absoluteY}`;
                    break;
                    
                default:
                    result = { success: false, message: `Unknown mouse action: ${action}` };
            }

            // Update performance stats
            this.updateMouseStats(performance.now() - startTime);
            
            return result;

        } catch (error) {
            console.error('❌ Mouse input error:', error.message);
            return { success: false, message: `Mouse input failed: ${error.message}` };
        }
    }

    /**
     * Handle keyboard input events
     * @param {string} key - Key to press (single character, key name, or key code)
     * @param {string} action - Action type ('press', 'down', 'up', 'type')
     * @param {object} options - Additional options (modifiers, etc.)
     */
    async handleKeyboardInput(key, action = 'press', options = {}) {
        if (!this.enabled) {
            return { success: false, message: 'Input control disabled' };
        }

        const startTime = performance.now();

        try {
            let result = { success: true, message: '' };
            const modifiers = options.modifiers || [];

            switch (action) {
                case 'press':
                    if (modifiers.length > 0) {
                        robot.keyTap(key, modifiers);
                        result.message = `Key combination ${modifiers.join('+')}+${key} pressed`;
                    } else {
                        robot.keyTap(key);
                        result.message = `Key '${key}' pressed`;
                    }
                    break;
                    
                case 'down':
                    robot.keyToggle(key, 'down', modifiers);
                    result.message = `Key '${key}' pressed down`;
                    break;
                    
                case 'up':
                    robot.keyToggle(key, 'up', modifiers);
                    result.message = `Key '${key}' released`;
                    break;
                    
                case 'type':
                    robot.typeString(key);
                    result.message = `Text typed: '${key}'`;
                    break;
                    
                default:
                    result = { success: false, message: `Unknown keyboard action: ${action}` };
            }

            // Update performance stats
            this.updateKeyboardStats(performance.now() - startTime);
            
            return result;

        } catch (error) {
            console.error('❌ Keyboard input error:', error.message);
            return { success: false, message: `Keyboard input failed: ${error.message}` };
        }
    }

    /**
     * Handle special key combinations (Ctrl+Alt+Del, Windows key, etc.)
     * @param {string} combination - Special key combination name
     */
    async handleSpecialKeys(combination) {
        if (!this.enabled) {
            return { success: false, message: 'Input control disabled' };
        }

        try {
            let result = { success: true, message: '' };

            switch (combination.toLowerCase()) {
                case 'ctrl+alt+del':
                case 'ctrl+alt+delete':
                    robot.keyTap('delete', ['control', 'alt']);
                    result.message = 'Ctrl+Alt+Del sent';
                    break;
                    
                case 'win':
                case 'windows':
                case 'cmd':
                    robot.keyTap('command');
                    result.message = 'Windows key pressed';
                    break;
                    
                case 'alt+tab':
                    robot.keyTap('tab', ['alt']);
                    result.message = 'Alt+Tab sent';
                    break;
                    
                case 'ctrl+c':
                    robot.keyTap('c', ['control']);
                    result.message = 'Ctrl+C (Copy) sent';
                    break;
                    
                case 'ctrl+v':
                    robot.keyTap('v', ['control']);
                    result.message = 'Ctrl+V (Paste) sent';
                    break;
                    
                default:
                    result = { success: false, message: `Unknown special key combination: ${combination}` };
            }

            return result;

        } catch (error) {
            console.error('❌ Special keys error:', error.message);
            return { success: false, message: `Special keys failed: ${error.message}` };
        }
    }

    /**
     * Get current mouse position
     */
    getCurrentMousePosition() {
        try {
            return robot.getMousePos();
        } catch (error) {
            console.error('❌ Failed to get mouse position:', error.message);
            return { x: 0, y: 0 };
        }
    }

    /**
     * Enable or disable input control
     * @param {boolean} enabled - Whether to enable input control
     */
    setEnabled(enabled) {
        this.enabled = enabled;
        console.log(`🖱️ Input control ${enabled ? 'enabled' : 'disabled'}`);
    }

    /**
     * Update mouse sensitivity
     * @param {number} sensitivity - Mouse sensitivity multiplier
     */
    setMouseSensitivity(sensitivity) {
        if (sensitivity > 0 && sensitivity <= 5) {
            this.mouseSensitivity = sensitivity;
            console.log(`🖱️ Mouse sensitivity updated to ${sensitivity}x`);
            return true;
        }
        return false;
    }

    /**
     * Get current performance statistics
     */
    getStats() {
        return {
            ...this.stats,
            screenSize: this.screenSize,
            enabled: this.enabled,
            mouseSensitivity: this.mouseSensitivity,
            keyboardDelay: this.keyboardDelay
        };
    }

    /**
     * Update mouse performance statistics
     */
    updateMouseStats(inputTime) {
        this.stats.totalMouseEvents++;
        this.stats.totalInputTime += inputTime;
        this.stats.averageInputLag = this.stats.totalInputTime / (this.stats.totalMouseEvents + this.stats.totalKeyboardEvents);
    }

    /**
     * Update keyboard performance statistics
     */
    updateKeyboardStats(inputTime) {
        this.stats.totalKeyboardEvents++;
        this.stats.totalInputTime += inputTime;
        this.stats.averageInputLag = this.stats.totalInputTime / (this.stats.totalMouseEvents + this.stats.totalKeyboardEvents);
    }

    /**
     * Log performance statistics
     */
    logPerformanceStats() {
        const totalEvents = this.stats.totalMouseEvents + this.stats.totalKeyboardEvents;
        console.log(`📊 Input Performance Stats (${totalEvents} events):`);
        console.log(`   Mouse Events: ${this.stats.totalMouseEvents}, Keyboard Events: ${this.stats.totalKeyboardEvents}`);
        console.log(`   Average Input Lag: ${this.stats.averageInputLag.toFixed(2)}ms`);
        console.log(`   Screen Size: ${this.screenSize?.width}x${this.screenSize?.height}`);
    }

    /**
     * Cleanup resources
     */
    destroy() {
        console.log('🧹 Professional Input Control destroyed');
        this.enabled = false;
        this.stats = null;
    }
}

module.exports = ProfessionalInputControl;
