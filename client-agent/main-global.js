// Remote Desktop Agent - Global Supabase Version
// This version connects directly to Supabase for worldwide accessibility

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

class GlobalRemoteDesktopAgent {
    constructor() {
        this.mainWindow = null;
        this.tray = null;
        
        // Global Supabase connection
        this.supabase = null;
        this.presenceManager = null;
        this.communicationManager = null;
        this.isConnected = false;
        this.reconnectAttempts = 0;
        this.maxReconnectAttempts = SUPABASE_CONFIG.global.maxReconnectAttempts;
        
        // Device information
        this.deviceId = this.generateDeviceId();
        this.deviceName = os.hostname();
        this.operatingSystem = this.getOperatingSystem();
        this.version = '1.0.0';
        this.isControlled = false;
        this.currentSessionId = null;
        
        // Screen streaming (simplified for now)
        this.screenStreamInterval = null;
        this.isStreaming = false;
        
        console.log(`🚀 Global Remote Desktop Agent starting...`);
        console.log(`📱 Device ID: ${this.deviceId}`);
        console.log(`💻 Device Name: ${this.deviceName}`);
        console.log(`🖥️ OS: ${this.operatingSystem}`);
    }

    async initialize() {
        try {
            console.log('🔧 Initializing Global Remote Desktop Agent...');
            
            // Set up auto-launch
            await this.setupAutoLaunch();
            
            // Create system tray
            this.createTray();
            
            // Connect to global Supabase
            await this.connectToSupabase();
            
            // Set up IPC handlers
            this.setupIpcHandlers();
            
            // Set up auto-updater
            this.setupAutoUpdater();
            
            console.log('✅ Global Remote Desktop Agent initialized successfully');
            
        } catch (error) {
            console.error('❌ Failed to initialize agent:', error);
            this.showErrorDialog('Initialization Failed', error.message);
        }
    }

    generateDeviceId() {
        // Generate a 9-digit device ID like TeamViewer
        return Math.floor(100000000 + Math.random() * 900000000).toString();
    }

    getOperatingSystem() {
        const platform = process.platform;
        switch (platform) {
            case 'win32': return 'Windows';
            case 'darwin': return 'macOS';
            case 'linux': return 'Linux';
            default: return platform;
        }
    }

    getScreenResolution() {
        try {
            const { screen } = require('electron');
            const primaryDisplay = screen.getPrimaryDisplay();
            return {
                width: primaryDisplay.bounds.width,
                height: primaryDisplay.bounds.height,
                scaleFactor: primaryDisplay.scaleFactor
            };
        } catch (error) {
            return { width: 1920, height: 1080, scaleFactor: 1 };
        }
    }

    async setupAutoLaunch() {
        try {
            console.log('🔄 Setting up auto-launch...');
            
            const autoLauncher = new AutoLaunch({
                name: 'Remote Desktop Agent',
                path: process.execPath,
                isHidden: true
            });

            const isEnabled = await autoLauncher.isEnabled();
            if (!isEnabled) {
                await autoLauncher.enable();
                console.log('✅ Auto-launch enabled');
            } else {
                console.log('✅ Auto-launch already enabled');
            }
            
        } catch (error) {
            console.error('⚠️ Auto-launch setup failed:', error);
            // Continue without auto-launch
        }
    }

    async connectToSupabase() {
        try {
            console.log('🌍 Connecting to global Supabase...');
            this.updateStatus('Connecting globally...');
            
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
            
            this.updateTrayIcon();
            
        } catch (error) {
            console.error('❌ Global Supabase connection failed:', error);
            this.handleConnectionError();
        }
    }

    async setupGlobalHandlers() {
        try {
            console.log('🔧 Setting up global Supabase handlers...');
            
            // Start presence tracking
            await this.presenceManager.startPresenceTracking();
            
            // Subscribe to control events
            await this.communicationManager.subscribeToControlEvents();
            
            // Subscribe to database changes for this device
            this.supabase
                .channel('device_updates')
                .on('postgres_changes', {
                    event: 'UPDATE',
                    schema: 'public',
                    table: 'remote_devices',
                    filter: `device_id=eq.${this.deviceId}`
                }, (payload) => {
                    console.log('📱 Device updated globally:', payload.new);
                    this.handleDeviceUpdate(payload.new);
                })
                .subscribe();
            
            // Subscribe to session requests
            this.supabase
                .channel('session_requests')
                .on('postgres_changes', {
                    event: 'INSERT',
                    schema: 'public',
                    table: 'remote_sessions',
                    filter: `device_id=eq.${this.deviceId}`
                }, (payload) => {
                    console.log('🎯 New session request:', payload.new);
                    this.handleSessionRequest(payload.new);
                })
                .subscribe();
            
            console.log('✅ Global handlers setup complete');
            
        } catch (error) {
            console.error('❌ Failed to setup global handlers:', error);
            throw error;
        }
    }

    async registerDeviceGlobally() {
        try {
            const deviceInfo = {
                deviceId: this.deviceId,
                deviceName: this.deviceName,
                operatingSystem: this.operatingSystem,
                version: this.version,
                capabilities: ['screen_share', 'remote_input'],
                screenResolution: this.getScreenResolution()
            };

            console.log('🌍 Registering device globally:', deviceInfo);
            this.updateStatus('Registering globally...');
            
            const result = await registerDeviceGlobally(this.supabase, deviceInfo);
            
            if (result.success) {
                console.log('✅ Device registered globally successfully');
                this.updateStatus(`Connected globally - ID: ${this.deviceId}`);
            } else {
                console.error('❌ Global device registration failed:', result.error);
                this.updateStatus('Registration failed - Retrying...');
                // Retry after delay
                setTimeout(() => this.registerDeviceGlobally(), 5000);
            }
            
        } catch (error) {
            console.error('❌ Global device registration failed:', error);
            this.updateStatus('Registration error - Retrying...');
            setTimeout(() => this.registerDeviceGlobally(), 5000);
        }
    }

    async handleSessionRequest(sessionData) {
        try {
            console.log('🎯 Handling session request:', sessionData);
            
            // Show permission dialog
            const response = await dialog.showMessageBox(null, {
                type: 'question',
                buttons: ['Allow', 'Deny'],
                defaultId: 1,
                title: 'Remote Control Request',
                message: 'Remote Control Request',
                detail: `A user wants to control this computer remotely.\n\nSession ID: ${sessionData.session_id}\nRequested by: ${sessionData.created_by || 'Unknown'}\n\nDo you want to allow this connection?`,
                icon: path.join(__dirname, 'assets', 'app-icon.png')
            });

            const granted = response.response === 0;
            
            // Update session with permission response
            await this.supabase
                .from('remote_sessions')
                .update({
                    status: granted ? 'active' : 'denied',
                    started_at: granted ? new Date().toISOString() : null
                })
                .eq('session_id', sessionData.session_id);
            
            if (granted) {
                console.log('✅ Control permission granted');
                this.currentSessionId = sessionData.session_id;
                this.isControlled = true;
                await this.presenceManager.setStatus('controlled');
                this.startScreenStream();
            } else {
                console.log('❌ Control permission denied');
            }
            
            this.updateTrayIcon();
            
        } catch (error) {
            console.error('❌ Failed to handle session request:', error);
        }
    }

    handleDeviceUpdate(deviceData) {
        console.log('📱 Device update received:', deviceData);
        // Handle any device configuration updates
    }

    startScreenStream() {
        if (this.isStreaming) return;
        
        console.log('📺 Starting screen stream (mock mode)...');
        this.isStreaming = true;
        
        this.screenStreamInterval = setInterval(async () => {
            try {
                // Send mock screen data for testing
                const screenData = {
                    device_id: this.deviceId,
                    session_id: this.currentSessionId,
                    image_data: 'data:image/svg+xml;base64,' + Buffer.from(
                        `<svg width="800" height="600" xmlns="http://www.w3.org/2000/svg">
                            <rect width="100%" height="100%" fill="#f0f0f0"/>
                            <text x="50%" y="50%" text-anchor="middle" font-size="24" fill="#333">
                                Mock Screen - Device ${this.deviceId}
                            </text>
                            <text x="50%" y="60%" text-anchor="middle" font-size="16" fill="#666">
                                ${new Date().toLocaleTimeString()}
                            </text>
                        </svg>`
                    ).toString('base64'),
                    timestamp: new Date().toISOString()
                };
                
                // In Phase 2, this will send actual screen data via Supabase Realtime
                console.log('📺 Mock screen frame sent');
                
            } catch (error) {
                console.error('❌ Screen stream error:', error);
            }
        }, 1000); // 1 FPS for testing
    }

    stopScreenStream() {
        if (this.screenStreamInterval) {
            clearInterval(this.screenStreamInterval);
            this.screenStreamInterval = null;
            this.isStreaming = false;
            console.log('📺 Screen stream stopped');
        }
    }

    async endControlSession() {
        console.log('🛑 Ending control session');
        
        this.isControlled = false;
        this.stopScreenStream();
        
        if (this.currentSessionId) {
            // Update session status
            await this.supabase
                .from('remote_sessions')
                .update({
                    status: 'ended',
                    ended_at: new Date().toISOString()
                })
                .eq('session_id', this.currentSessionId);
            
            this.currentSessionId = null;
        }
        
        await this.presenceManager.setStatus('online');
        this.updateTrayIcon();
    }

    handleConnectionError() {
        this.isConnected = false;
        this.reconnectAttempts++;
        
        console.log(`❌ Connection error (attempt ${this.reconnectAttempts}/${this.maxReconnectAttempts})`);
        this.updateStatus('Connection lost - Reconnecting...');
        
        if (this.reconnectAttempts < this.maxReconnectAttempts) {
            const delay = Math.min(30000, 2000 * Math.pow(2, this.reconnectAttempts));
            setTimeout(() => this.connectToSupabase(), delay);
        } else {
            console.error('❌ Max reconnection attempts reached');
            this.updateStatus('Connection failed - Check internet');
            this.showErrorDialog('Connection Failed', 'Unable to connect to remote desktop service. Please check your internet connection.');
        }
        
        this.updateTrayIcon();
    }

    createTray() {
        try {
            const iconPath = path.join(__dirname, 'assets', 'tray-icon.png');
            this.tray = new Tray(iconPath);
            
            this.updateTrayMenu();
            this.updateTrayIcon();
            
            this.tray.setToolTip('Remote Desktop Agent - Connecting...');
            
        } catch (error) {
            console.error('⚠️ Failed to create tray icon:', error);
        }
    }

    updateTrayMenu() {
        const contextMenu = Menu.buildFromTemplate([
            {
                label: `Device ID: ${this.deviceId}`,
                enabled: false
            },
            {
                label: `Status: ${this.isConnected ? (this.isControlled ? 'Being Controlled' : 'Online') : 'Offline'}`,
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
                    require('electron').clipboard.writeText(this.deviceId);
                    console.log('📋 Device ID copied to clipboard');
                }
            },
            { type: 'separator' },
            {
                label: 'Quit',
                click: () => this.quit()
            }
        ]);
        
        this.tray.setContextMenu(contextMenu);
    }

    updateTrayIcon() {
        if (!this.tray) return;
        
        let iconName = 'tray-icon.png';
        let tooltip = 'Remote Desktop Agent';
        
        if (!this.isConnected) {
            iconName = 'tray-icon-offline.png';
            tooltip += ' - Offline';
        } else if (this.isControlled) {
            iconName = 'tray-icon-controlled.png';
            tooltip += ' - Being Controlled';
        } else {
            iconName = 'tray-icon-online.png';
            tooltip += ' - Online';
        }
        
        try {
            const iconPath = path.join(__dirname, 'assets', iconName);
            this.tray.setImage(iconPath);
            this.tray.setToolTip(tooltip);
        } catch (error) {
            console.error('⚠️ Failed to update tray icon:', error);
        }
        
        this.updateTrayMenu();
    }

    updateStatus(status) {
        console.log(`📊 Status: ${status}`);
        if (this.mainWindow && !this.mainWindow.isDestroyed()) {
            this.mainWindow.webContents.send('status-update', status);
        }
    }

    showWindow() {
        if (!this.mainWindow || this.mainWindow.isDestroyed()) {
            this.createWindow();
        } else {
            this.mainWindow.show();
            this.mainWindow.focus();
        }
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
            minimizable: true,
            maximizable: false
        });

        this.mainWindow.loadFile(path.join(__dirname, 'renderer.html'));
        
        this.mainWindow.on('close', (event) => {
            event.preventDefault();
            this.mainWindow.hide();
        });

        this.mainWindow.on('closed', () => {
            this.mainWindow = null;
        });
    }

    setupIpcHandlers() {
        ipcMain.handle('get-device-info', () => ({
            deviceId: this.deviceId,
            deviceName: this.deviceName,
            operatingSystem: this.operatingSystem,
            version: this.version,
            isConnected: this.isConnected,
            isControlled: this.isControlled
        }));

        ipcMain.handle('end-session', async () => {
            await this.endControlSession();
        });

        ipcMain.handle('reconnect', async () => {
            this.reconnectAttempts = 0;
            await this.connectToSupabase();
        });
    }

    setupAutoUpdater() {
        autoUpdater.checkForUpdatesAndNotify();
        
        autoUpdater.on('update-available', () => {
            console.log('📦 Update available');
        });
        
        autoUpdater.on('update-downloaded', () => {
            console.log('📦 Update downloaded');
            dialog.showMessageBox(null, {
                type: 'info',
                title: 'Update Ready',
                message: 'Update downloaded. The application will restart to apply the update.',
                buttons: ['Restart Now', 'Later']
            }).then((result) => {
                if (result.response === 0) {
                    autoUpdater.quitAndInstall();
                }
            });
        });
    }

    showErrorDialog(title, message) {
        dialog.showErrorBox(title, message);
    }

    async quit() {
        console.log('🛑 Shutting down Global Remote Desktop Agent...');
        
        try {
            // Clean up connections
            if (this.presenceManager) {
                await this.presenceManager.stopPresenceTracking();
            }
            
            if (this.communicationManager) {
                await this.communicationManager.cleanup();
            }
            
            this.stopScreenStream();
            
        } catch (error) {
            console.error('⚠️ Error during cleanup:', error);
        }
        
        app.quit();
    }
}

// App event handlers
app.whenReady().then(async () => {
    const agent = new GlobalRemoteDesktopAgent();
    await agent.initialize();
    
    // Prevent app from quitting when all windows are closed (run in background)
    app.on('window-all-closed', () => {
        // Keep running in background
    });
    
    app.on('activate', () => {
        agent.showWindow();
    });
    
    app.on('before-quit', async () => {
        await agent.quit();
    });
});

app.on('window-all-closed', () => {
    // Keep running in background on all platforms
});

console.log('🚀 Global Remote Desktop Agent starting...');
