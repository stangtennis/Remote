// Remote Desktop Agent - Renderer Process
const { ipcRenderer } = require('electron');

class AgentRenderer {
    constructor() {
        this.deviceInfo = null;
        this.init();
    }

    async init() {
        console.log('🎨 Agent renderer starting...');
        
        // Load device information
        await this.loadDeviceInfo();
        
        // Set up event listeners
        this.setupEventListeners();
        
        // Start status updates
        this.startStatusUpdates();
    }

    async loadDeviceInfo() {
        try {
            this.deviceInfo = await ipcRenderer.invoke('get-device-info');
            this.updateUI();
        } catch (error) {
            console.error('Error loading device info:', error);
        }
    }

    updateUI() {
        if (!this.deviceInfo) return;

        // Format device ID with spaces (like TeamViewer)
        const formattedId = this.deviceInfo.deviceId.replace(/(\d{3})(\d{3})(\d{3})/, '$1 $2 $3');
        document.getElementById('device-id').textContent = formattedId;
        
        // Update device information
        document.getElementById('device-name').textContent = this.deviceInfo.deviceName;
        document.getElementById('operating-system').textContent = this.deviceInfo.operatingSystem;
        document.getElementById('version').textContent = this.deviceInfo.version;
        
        // Update connection status
        this.updateConnectionStatus();
        
        // Show/hide control warning
        this.updateControlStatus();
    }

    updateConnectionStatus() {
        const statusDot = document.getElementById('status-dot');
        const statusText = document.getElementById('status-text');
        
        statusDot.className = 'status-dot';
        
        if (this.deviceInfo.isControlled) {
            statusDot.classList.add('controlled');
            statusText.textContent = 'Being Controlled';
        } else if (this.deviceInfo.isConnected) {
            statusDot.classList.add('connected');
            statusText.textContent = 'Connected';
        } else {
            statusDot.classList.add('disconnected');
            statusText.textContent = 'Disconnected';
        }
    }

    updateControlStatus() {
        const warning = document.getElementById('controlled-warning');
        if (this.deviceInfo.isControlled) {
            warning.style.display = 'block';
        } else {
            warning.style.display = 'none';
        }
    }

    setupEventListeners() {
        // Copy Device ID button
        document.getElementById('copy-id-btn').addEventListener('click', async () => {
            try {
                await ipcRenderer.invoke('copy-device-id');
                this.showToast('Device ID copied to clipboard!');
            } catch (error) {
                console.error('Error copying device ID:', error);
                this.showToast('Failed to copy device ID', 'error');
            }
        });

        // Settings button
        document.getElementById('settings-btn').addEventListener('click', () => {
            // TODO: Implement settings
            this.showToast('Settings coming soon!');
        });

        // Hide to tray button
        document.getElementById('hide-btn').addEventListener('click', () => {
            window.close();
        });

        // Handle window focus
        window.addEventListener('focus', () => {
            this.loadDeviceInfo();
        });
    }

    startStatusUpdates() {
        // Update status every 2 seconds
        setInterval(async () => {
            await this.loadDeviceInfo();
        }, 2000);
    }

    showToast(message, type = 'success') {
        // Create toast notification
        const toast = document.createElement('div');
        toast.style.cssText = `
            position: fixed;
            top: 20px;
            right: 20px;
            background: ${type === 'error' ? '#ef4444' : '#10b981'};
            color: white;
            padding: 12px 20px;
            border-radius: 8px;
            font-size: 14px;
            font-weight: 500;
            z-index: 1000;
            opacity: 0;
            transform: translateX(100%);
            transition: all 0.3s ease;
        `;
        toast.textContent = message;
        
        document.body.appendChild(toast);
        
        // Animate in
        setTimeout(() => {
            toast.style.opacity = '1';
            toast.style.transform = 'translateX(0)';
        }, 100);
        
        // Remove after 3 seconds
        setTimeout(() => {
            toast.style.opacity = '0';
            toast.style.transform = 'translateX(100%)';
            setTimeout(() => {
                if (toast.parentNode) {
                    toast.parentNode.removeChild(toast);
                }
            }, 300);
        }, 3000);
    }
}

// Initialize when DOM is loaded
document.addEventListener('DOMContentLoaded', () => {
    new AgentRenderer();
});

// Handle keyboard shortcuts
document.addEventListener('keydown', (event) => {
    // Ctrl+C or Cmd+C to copy device ID
    if ((event.ctrlKey || event.metaKey) && event.key === 'c') {
        document.getElementById('copy-id-btn').click();
        event.preventDefault();
    }
    
    // Escape to hide window
    if (event.key === 'Escape') {
        window.close();
    }
});
