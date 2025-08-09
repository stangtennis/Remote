/**
 * Professional Screen Capture Module
 * Real native screen capture with compression and multi-monitor support
 * Replaces mock screen capture with actual desktop capture like TeamViewer
 */

const screenshot = require('screenshot-desktop');
const sharp = require('sharp');
const { performance } = require('perf_hooks');

class ProfessionalScreenCapture {
    constructor(options = {}) {
        this.quality = options.quality || 80; // JPEG quality (1-100)
        this.maxWidth = options.maxWidth || 1920;
        this.maxHeight = options.maxHeight || 1080;
        this.format = options.format || 'jpeg';
        this.frameRate = options.frameRate || 10; // FPS
        this.compressionEnabled = options.compression !== false;
        
        // Performance tracking
        this.stats = {
            totalFrames: 0,
            totalCaptureTime: 0,
            totalCompressionTime: 0,
            averageFPS: 0,
            lastFrameTime: 0
        };
        
        // Multi-monitor support
        this.displays = [];
        this.currentDisplay = 0;
        
        console.log('🎥 Professional Screen Capture initialized');
        console.log(`📊 Settings: ${this.maxWidth}x${this.maxHeight}, Quality: ${this.quality}%, FPS: ${this.frameRate}`);
    }

    /**
     * Initialize screen capture system
     * Detect available displays and set up capture parameters
     */
    async initialize() {
        try {
            // Get available displays
            this.displays = await screenshot.listDisplays();
            console.log(`🖥️  Detected ${this.displays.length} display(s):`);
            
            this.displays.forEach((display, index) => {
                console.log(`   Display ${index}: ${display.width}x${display.height} at (${display.x}, ${display.y})`);
            });
            
            return true;
        } catch (error) {
            console.error('❌ Failed to initialize screen capture:', error.message);
            return false;
        }
    }

    /**
     * Capture screen from specified display
     * @param {number} displayIndex - Display to capture (default: 0)
     * @returns {Promise<string>} Base64 encoded image data
     */
    async captureScreen(displayIndex = 0) {
        const startTime = performance.now();
        
        try {
            // Capture screenshot from specified display
            const captureOptions = {
                screen: displayIndex,
                format: 'png' // Always capture as PNG first for quality
            };
            
            const imageBuffer = await screenshot(captureOptions);
            const captureTime = performance.now() - startTime;
            
            // Compress and resize if enabled
            let processedBuffer = imageBuffer;
            let compressionTime = 0;
            
            if (this.compressionEnabled) {
                const compressionStart = performance.now();
                
                processedBuffer = await sharp(imageBuffer)
                    .resize(this.maxWidth, this.maxHeight, {
                        fit: 'inside',
                        withoutEnlargement: true
                    })
                    .jpeg({
                        quality: this.quality,
                        progressive: true,
                        mozjpeg: true
                    })
                    .toBuffer();
                    
                compressionTime = performance.now() - compressionStart;
            }
            
            // Update performance stats
            this.updateStats(captureTime, compressionTime);
            
            // Convert to base64
            const base64Data = processedBuffer.toString('base64');
            const mimeType = this.compressionEnabled ? 'image/jpeg' : 'image/png';
            
            // Log performance every 30 frames
            if (this.stats.totalFrames % 30 === 0) {
                this.logPerformanceStats();
            }
            
            return `data:${mimeType};base64,${base64Data}`;
            
        } catch (error) {
            console.error('❌ Screen capture failed:', error.message);
            
            // Return fallback placeholder on error
            return this.createErrorPlaceholder();
        }
    }

    /**
     * Capture screen with automatic display detection
     * Uses primary display by default
     */
    async captureScreenAuto() {
        return await this.captureScreen(this.currentDisplay);
    }

    /**
     * Switch to different display for capture
     * @param {number} displayIndex - Display index to switch to
     */
    switchDisplay(displayIndex) {
        if (displayIndex >= 0 && displayIndex < this.displays.length) {
            this.currentDisplay = displayIndex;
            console.log(`🖥️  Switched to display ${displayIndex}`);
            return true;
        }
        return false;
    }

    /**
     * Update quality settings dynamically
     * @param {number} quality - JPEG quality (1-100)
     */
    setQuality(quality) {
        if (quality >= 1 && quality <= 100) {
            this.quality = quality;
            console.log(`🎛️  Quality updated to ${quality}%`);
            return true;
        }
        return false;
    }

    /**
     * Update frame rate settings
     * @param {number} fps - Target frames per second
     */
    setFrameRate(fps) {
        if (fps >= 1 && fps <= 60) {
            this.frameRate = fps;
            console.log(`⚡ Frame rate updated to ${fps} FPS`);
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
            currentQuality: this.quality,
            currentFPS: this.frameRate,
            displayCount: this.displays.length,
            currentDisplay: this.currentDisplay
        };
    }

    /**
     * Update internal performance statistics
     */
    updateStats(captureTime, compressionTime) {
        this.stats.totalFrames++;
        this.stats.totalCaptureTime += captureTime;
        this.stats.totalCompressionTime += compressionTime;
        
        const now = performance.now();
        if (this.stats.lastFrameTime > 0) {
            const timeDiff = now - this.stats.lastFrameTime;
            this.stats.averageFPS = 1000 / timeDiff;
        }
        this.stats.lastFrameTime = now;
    }

    /**
     * Log performance statistics
     */
    logPerformanceStats() {
        const avgCapture = (this.stats.totalCaptureTime / this.stats.totalFrames).toFixed(2);
        const avgCompression = (this.stats.totalCompressionTime / this.stats.totalFrames).toFixed(2);
        const currentFPS = this.stats.averageFPS.toFixed(1);
        
        console.log(`📊 Performance Stats (${this.stats.totalFrames} frames):`);
        console.log(`   Capture: ${avgCapture}ms avg, Compression: ${avgCompression}ms avg`);
        console.log(`   Current FPS: ${currentFPS}, Target: ${this.frameRate}`);
    }

    /**
     * Create error placeholder image
     */
    createErrorPlaceholder() {
        // Create a simple error image using Sharp
        const width = 800;
        const height = 600;
        
        const svg = `
            <svg width="${width}" height="${height}" xmlns="http://www.w3.org/2000/svg">
                <rect width="100%" height="100%" fill="#1a1a1a"/>
                <text x="50%" y="45%" text-anchor="middle" fill="#ff6b6b" font-family="Arial" font-size="24">
                    Screen Capture Error
                </text>
                <text x="50%" y="55%" text-anchor="middle" fill="#888" font-family="Arial" font-size="16">
                    Unable to capture screen
                </text>
            </svg>
        `;
        
        return sharp(Buffer.from(svg))
            .png()
            .toBuffer()
            .then(buffer => `data:image/png;base64,${buffer.toString('base64')}`)
            .catch(() => 'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg==');
    }

    /**
     * Cleanup resources
     */
    destroy() {
        console.log('🧹 Professional Screen Capture destroyed');
        this.displays = [];
        this.stats = null;
    }
}

module.exports = ProfessionalScreenCapture;
