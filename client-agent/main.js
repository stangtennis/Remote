// Remote Desktop Agent - Main Process (Simplified)
const { app, BrowserWindow, Tray, Menu, ipcMain, dialog, shell } = require('electron');
const path = require('path');
const os = require('os');
const { autoUpdater } = require('electron-updater');
const AutoLaunch = require('auto-launch');
const { 
    createGlobalSupabaseClient, 
    registerDeviceGlobally, 
    GlobalPresenceManager, 
    GlobalDeviceCommunication,
    SUPABASE_CONFIG 
} = require('../config/supabase-config');
// Temporarily disabled native modules for testing
// const AutoLaunch = require('auto-launch');
// const robot = require('robotjs');
// const screenshot = require('screenshot-desktop');

class RemoteDesktopAgent {
    constructor() {
        this.mainWindow = null;
        // Initialize global Supabase connection
        this.supabase = null;
        this.presenceManager = null;
        this.communicationManager = null;
        this.isConnected = false;
        this.reconnectAttempts = 0;
        this.maxReconnectAttempts = SUPABASE_CONFIG.global.maxReconnectAttempts;
        this.deviceId = null;
        this.deviceInfo = null;
        this.isControlled = false;
        this.serverUrl = 'http://localhost:3000';
        this.screenStreamInterval = null;
        
        this.init();
    }

    async init() {
        console.log('🤖 Remote Desktop Agent starting...');
        
        // Generate unique device ID
        this.deviceId = this.generateDeviceId();
        
        // Gather device information
        this.deviceInfo = await this.getDeviceInfo();
        
        // Set up auto-launch
        await this.setupAutoLaunch();
        
        // Start global connection
        this.connectToSupabase();
        
        // Set up IPC handlers
        this.setupIpcHandlers();
        
        console.log(`🤖 Agent initialized with Device ID: ${this.deviceId}`);
    }

    generateDeviceId() {
        // Generate a 9-digit ID similar to TeamViewer
        return Math.floor(100000000 + Math.random() * 900000000).toString();
    }

    async getDeviceInfo() {
        const displays = screen.getAllDisplays();
        const primaryDisplay = screen.getPrimaryDisplay();
        
        return {
            deviceId: this.deviceId,
            deviceName: os.hostname(),
            operatingSystem: `${os.type()} ${os.release()}`,
            version: '1.0.0',
            screenSize: {
                width: primaryDisplay.bounds.width,
                height: primaryDisplay.bounds.height
            },
            displays: displays.length,
            isConnected: this.isConnected,
            isControlled: this.isControlled
        };
    }

    async setupAutoLaunch() {
        // Auto-launch temporarily disabled for testing
        console.log('⚠️ Auto-launch disabled (testing mode)');
    }

    async connectToSupabase() {
        try {
            console.log('🌍 Connecting to global Supabase...');
            
            // Create global Supabase client
            this.supabase = createGlobalSupabaseClient();
            
            // Test connection
            const { data, error } = await this.supabase
                .from('remote_devices')
                .select('count')
                .limit(1);
                
            if (error) {
                throw new Error(`Supabase connection failed: ${error.message}`);
            }
            
            console.log('✅ Global Supabase connection established');
            
            // Initialize presence and communication managers
            this.presenceManager = new GlobalPresenceManager(this.supabase, this.deviceId);
            this.communicationManager = new GlobalDeviceCommunication(this.supabase, this.deviceId);
            
            // Setup global handlers
            await this.setupGlobalHandlers();
            
            this.isConnected = true;
            this.reconnectAttempts = 0;
            
            // Register device globally
            await this.registerDeviceGlobally();
            
        } catch (error) {
            console.error('❌ Global Supabase connection failed:', error);
            this.handleConnectionError();
        
        console.log(`🎯 Control request from user: ${userId}`);
        
        // Show permission dialog
        const response = await dialog.showMessageBox(null, {
            type: 'question',
            buttons: ['Allow', 'Deny'],
            defaultId: 1,
            title: 'Remote Control Request',
            message: 'Remote Control Request',
            detail: `A user wants to control this computer remotely.\n\nUser ID: ${userId}\n\nDo you want to allow this connection?`,
            icon: path.join(__dirname, 'assets', 'app-icon.png')
        });

        const granted = response.response === 0;
        
        if (granted) {
            this.isControlled = true;
            this.startScreenStream();
            console.log('✅ Control permission granted');
        } else {
            console.log('❌ Control permission denied');
        }

        // Send response back to server
        this.socket.emit('control-permission', {
            deviceId: this.deviceId,
            granted,
            requestId
        });

        this.updateTrayIcon();
    }

    startScreenStream() {
        console.log('📺 Starting screen stream (mock mode)...');
        
        this.screenStreamInterval = setInterval(() => {
            // Send mock screen data for testing
            this.socket.emit('screen-stream', {
                deviceId: this.deviceId,
                imageData: 'data:image/svg+xml;base64,' + Buffer.from('<svg width="800" height="600" xmlns="http://www.w3.org/2000/svg"><rect width="100%" height="100%" fill="#f0f0f0"/><text x="50%" y="50%" text-anchor="middle" font-size="24" fill="#333">Mock Screen - Device ' + this.deviceId + '</text></svg>').toString('base64'),
                timestamp: Date.now()
            });
        }, 1000); // 1 FPS for testing
    }

    stopScreenStream() {
        if (this.screenStreamInterval) {
            clearInterval(this.screenStreamInterval);
            this.screenStreamInterval = null;
            console.log('📺 Screen stream stopped');
        }
    }

    handleRemoteInput(data) {
        const { type, x, y, button, key, keyCode } = data;
        
        // Mock input handling for testing
        console.log('🖱️ Mock input received:', { type, x, y, button, key });
        
        // In a real implementation, this would control the actual mouse/keyboard
        // For now, just log the input for testing purposes
    }

    endControlSession() {
        console.log('🛑 Control session ended');
        this.isControlled = false;
        this.stopScreenStream();
        this.updateTrayIcon();
    }

    createWindow() {
        this.mainWindow = new BrowserWindow({
            width: 400,
            height: 600,
            webPreferences: {
                nodeIntegration: true,
                contextIsolation: false
            },
            icon: path.join(__dirname, 'assets', 'app-icon.png'),
            title: 'Remote Desktop Agent',
            resizable: false,
            maximizable: false,
            show: false
        });

        this.mainWindow.loadFile('renderer.html');

        this.mainWindow.on('close', (event) => {
            event.preventDefault();
            this.mainWindow.hide();
        });

        this.mainWindow.on('closed', () => {
            this.mainWindow = null;
        });
    }

    createTray() {
        const iconPath = path.join(__dirname, 'assets', 'tray-icon.png');
        this.tray = new Tray(iconPath);
        
        this.updateTrayIcon();
        
        this.tray.on('click', () => {
            this.showWindow();
        });

        this.tray.on('right-click', () => {
            this.showContextMenu();
        });
    }

    updateTrayIcon() {
        if (!this.tray) return;
        
        let tooltip = `Remote Desktop Agent\nDevice ID: ${this.deviceId}`;
        
        if (this.isControlled) {
            tooltip += '\n⚠️ Being Controlled';
        } else if (this.isConnected) {
            tooltip += '\n✅ Connected';
        } else {
            tooltip += '\n❌ Disconnected';
        }
        
        this.tray.setToolTip(tooltip);
    }

    showContextMenu() {
        const contextMenu = Menu.buildFromTemplate([
            {
                label: `Device ID: ${this.deviceId}`,
                enabled: false
            },
            { type: 'separator' },
            {
                label: 'Show Window',
                click: () => this.showWindow()
            },
            {
                label: 'Copy Device ID',
                click: () => {
                    clipboard.writeText(this.deviceId);
                }
            },
            { type: 'separator' },
            {
                label: 'Quit',
                click: () => {
                    app.quit();
                }
            }
        ]);
        
        this.tray.popUpContextMenu(contextMenu);
    }

    showWindow() {
        if (!this.mainWindow) {
            this.createWindow();
        }
        
        this.mainWindow.show();
        this.mainWindow.focus();
    }

    setupIpcHandlers() {
        ipcMain.handle('get-device-info', async () => {
            return await this.getDeviceInfo();
        });

        ipcMain.handle('copy-device-id', async () => {
            clipboard.writeText(this.deviceId);
        });
    }
}

// App event handlers
app.whenReady().then(() => {
    const agent = new RemoteDesktopAgent();
    
    agent.createTray();
    
    // Show window on first run (optional)
    // agent.showWindow();
});

app.on('window-all-closed', () => {
    // Keep app running in background
});

app.on('activate', () => {
    // macOS specific behavior
});

app.on('before-quit', () => {
    if (global.agent) {
        global.agent.socket?.disconnect();
    }
});
