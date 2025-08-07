// Local Agent Builder for Testing
// Simulates the Supabase Edge Function locally for testing purposes

const express = require('express');
const path = require('path');
const fs = require('fs');
const crypto = require('crypto');

const app = express();
const PORT = 3001;

// Middleware
app.use(express.json());
app.use(express.static('public'));

// CORS middleware
app.use((req, res, next) => {
    res.header('Access-Control-Allow-Origin', '*');
    res.header('Access-Control-Allow-Headers', 'authorization, x-client-info, apikey, content-type');
    res.header('Access-Control-Allow-Methods', 'GET, POST, OPTIONS');
    next();
});

// Agent builder endpoint
app.get('/functions/v1/agent-builder', async (req, res) => {
    try {
        const {
            platform = 'windows',
            deviceName = 'Remote Device',
            autoStart = 'true',
            hideWindow = 'true',
            orgId = 'default'
        } = req.query;

        console.log(`🔧 Generating agent for platform: ${platform}, device: ${deviceName}`);

        // Validate platform
        const supportedPlatforms = ['windows', 'macos', 'linux'];
        if (!supportedPlatforms.includes(platform)) {
            return res.status(400).json({ error: 'Unsupported platform' });
        }

        // Generate unique device token
        const deviceToken = generateSecureToken(32);
        
        // Create agent configuration
        const agentConfig = {
            supabaseUrl: 'https://ptrtibzwokjcjjxvjpin.supabase.co',
            supabaseKey: 'sb_publishable_OwJ04BrJ6gZZ57fSeT-q5Q_mSsVaNia',
            deviceToken,
            deviceName,
            orgId,
            serverName: 'Remote Desktop System',
            autoStart: autoStart === 'true',
            hideWindow: hideWindow === 'true',
            platform,
            generatedAt: new Date().toISOString()
        };

        // Generate the agent script
        const agentScript = createAgentScript(platform, agentConfig);
        
        // Create platform-specific executable
        const agentExecutable = createExecutable(platform, agentScript, agentConfig);
        
        // Get filename
        const filename = getFilename(platform, deviceName);
        
        console.log(`✅ Generated agent: ${filename}`);

        // Return the executable file
        res.set({
            'Content-Type': 'application/octet-stream',
            'Content-Disposition': `attachment; filename="${filename}"`,
            'Content-Length': agentExecutable.length
        });
        
        res.send(agentExecutable);

    } catch (error) {
        console.error('❌ Agent builder error:', error);
        res.status(500).json({ 
            error: 'Failed to generate agent', 
            details: error.message 
        });
    }
});

// Test endpoint
app.get('/test', (req, res) => {
    res.json({ 
        message: 'Agent Builder Test Server Running',
        timestamp: new Date().toISOString(),
        endpoints: [
            'GET /functions/v1/agent-builder?platform=windows&deviceName=TestPC',
            'GET /test'
        ]
    });
});

function generateSecureToken(length) {
    return crypto.randomBytes(length).toString('hex').substring(0, length);
}

function createAgentScript(platform, config) {
    return `
// Auto-generated Remote Desktop Agent
// Platform: ${platform}
// Generated: ${config.generatedAt}
// Device: ${config.deviceName}

const { app, BrowserWindow, Tray, Menu, ipcMain, dialog, nativeImage } = require('electron');
const path = require('path');
const os = require('os');
const { createClient } = require('@supabase/supabase-js');

// Embedded configuration (injected during build)
const AGENT_CONFIG = ${JSON.stringify(config, null, 2)};

console.log('🚀 Remote Desktop Agent starting...');
console.log('📱 Device Token:', AGENT_CONFIG.deviceToken);
console.log('💻 Device Name:', AGENT_CONFIG.deviceName);
console.log('🌍 Server:', AGENT_CONFIG.supabaseUrl);

class RemoteDesktopAgent {
    constructor() {
        this.deviceId = this.generateDeviceId();
        this.supabase = createClient(AGENT_CONFIG.supabaseUrl, AGENT_CONFIG.supabaseKey);
        this.isConnected = false;
        this.tray = null;
        this.mainWindow = null;
        
        console.log('📱 Generated Device ID:', this.deviceId);
    }

    async initialize() {
        try {
            console.log('🔧 Initializing Remote Desktop Agent...');
            
            // Create system tray
            this.createTray();
            
            // Connect to Supabase
            await this.connectToSupabase();
            
            // Register device
            await this.registerDevice();
            
            // Create hidden window if needed
            if (!AGENT_CONFIG.hideWindow) {
                this.createWindow();
            }
            
            console.log('✅ Agent initialized successfully');
            
        } catch (error) {
            console.error('❌ Failed to initialize agent:', error);
            this.showErrorDialog('Initialization Failed', error.message);
        }
    }

    generateDeviceId() {
        return Math.floor(100000000 + Math.random() * 900000000).toString();
    }

    async connectToSupabase() {
        try {
            console.log('🌍 Connecting to global Supabase...');
            
            const { data, error } = await this.supabase
                .from('remote_devices')
                .select('count')
                .limit(1);
                
            if (error) throw error;
            
            this.isConnected = true;
            console.log('✅ Connected to Supabase globally');
            
        } catch (error) {
            console.error('❌ Supabase connection failed:', error);
            throw error;
        }
    }

    async registerDevice() {
        try {
            console.log('🌍 Registering device globally...');
            
            const deviceInfo = {
                device_id: this.deviceId,
                device_name: AGENT_CONFIG.deviceName || os.hostname(),
                operating_system: this.getOperatingSystem(),
                status: 'online',
                last_seen: new Date().toISOString(),
                metadata: {
                    version: '1.0.0',
                    platform: AGENT_CONFIG.platform,
                    auto_generated: true,
                    org_id: AGENT_CONFIG.orgId,
                    device_token: AGENT_CONFIG.deviceToken,
                    generated_at: AGENT_CONFIG.generatedAt,
                    agent_type: 'downloadable'
                }
            };

            const { error } = await this.supabase
                .from('remote_devices')
                .upsert(deviceInfo);

            if (error) throw error;

            // Update presence
            await this.supabase
                .from('device_presence')
                .upsert({
                    device_id: this.deviceId,
                    status: 'online',
                    last_seen: new Date().toISOString(),
                    metadata: {
                        agent_type: 'downloadable',
                        platform: AGENT_CONFIG.platform
                    }
                });

            console.log('✅ Device registered globally');
            this.updateTrayTooltip(\`Online - ID: \${this.deviceId}\`);
            
        } catch (error) {
            console.error('❌ Device registration failed:', error);
            throw error;
        }
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

    createTray() {
        try {
            // Create a simple tray icon
            const icon = this.createTrayIcon();
            this.tray = new Tray(icon);
            
            const contextMenu = Menu.buildFromTemplate([
                {
                    label: \`Device ID: \${this.deviceId || 'Generating...'}\`,
                    enabled: false
                },
                {
                    label: \`Device: \${AGENT_CONFIG.deviceName}\`,
                    enabled: false
                },
                {
                    label: 'Status: Connecting...',
                    enabled: false
                },
                { type: 'separator' },
                {
                    label: 'Copy Device ID',
                    click: () => {
                        if (this.deviceId) {
                            require('electron').clipboard.writeText(this.deviceId);
                            console.log('📋 Device ID copied to clipboard');
                        }
                    }
                },
                {
                    label: 'Show Window',
                    click: () => this.showWindow()
                },
                {
                    label: 'Quit',
                    click: () => app.quit()
                }
            ]);
            
            this.tray.setContextMenu(contextMenu);
            this.tray.setToolTip('Remote Desktop Agent - Starting...');
            
        } catch (error) {
            console.error('⚠️ Failed to create tray icon:', error);
        }
    }

    createTrayIcon() {
        // Create a simple colored square as tray icon
        const canvas = require('canvas');
        const { createCanvas } = canvas;
        const canvasSize = 16;
        const canvasInstance = createCanvas(canvasSize, canvasSize);
        const ctx = canvasInstance.getContext('2d');
        
        // Draw a simple icon
        ctx.fillStyle = '#4CAF50';
        ctx.fillRect(0, 0, canvasSize, canvasSize);
        ctx.fillStyle = '#FFFFFF';
        ctx.fillRect(2, 2, canvasSize-4, canvasSize-4);
        ctx.fillStyle = '#2196F3';
        ctx.fillRect(4, 4, canvasSize-8, canvasSize-8);
        
        const buffer = canvasInstance.toBuffer('image/png');
        return nativeImage.createFromBuffer(buffer);
    }

    createWindow() {
        this.mainWindow = new BrowserWindow({
            width: 400,
            height: 300,
            webPreferences: {
                nodeIntegration: true,
                contextIsolation: false
            },
            title: \`Remote Desktop Agent - \${AGENT_CONFIG.deviceName}\`,
            resizable: false,
            show: !AGENT_CONFIG.hideWindow
        });

        this.mainWindow.loadURL('data:text/html;charset=utf-8,' + encodeURIComponent(\`
            <!DOCTYPE html>
            <html>
            <head>
                <title>Remote Desktop Agent</title>
                <style>
                    body { font-family: Arial, sans-serif; padding: 20px; background: #f5f5f5; }
                    .container { background: white; padding: 20px; border-radius: 8px; box-shadow: 0 2px 10px rgba(0,0,0,0.1); }
                    .status { color: #4CAF50; font-weight: bold; }
                    .device-id { font-family: monospace; background: #f0f0f0; padding: 5px; border-radius: 4px; }
                </style>
            </head>
            <body>
                <div class="container">
                    <h2>🌍 Remote Desktop Agent</h2>
                    <p><strong>Device Name:</strong> \${AGENT_CONFIG.deviceName}</p>
                    <p><strong>Device ID:</strong> <span class="device-id">\${this.deviceId}</span></p>
                    <p><strong>Platform:</strong> \${AGENT_CONFIG.platform}</p>
                    <p><strong>Status:</strong> <span class="status">Connected Globally</span></p>
                    <p><strong>Server:</strong> \${AGENT_CONFIG.supabaseUrl}</p>
                    <hr>
                    <p><small>This device is now available for remote control from the admin dashboard.</small></p>
                </div>
            </body>
            </html>
        \`));

        this.mainWindow.on('close', (event) => {
            if (AGENT_CONFIG.hideWindow) {
                event.preventDefault();
                this.mainWindow.hide();
            }
        });
    }

    showWindow() {
        if (!this.mainWindow) {
            this.createWindow();
        } else {
            this.mainWindow.show();
            this.mainWindow.focus();
        }
    }

    updateTrayTooltip(status) {
        if (this.tray) {
            this.tray.setToolTip(\`Remote Desktop Agent - \${status}\`);
            
            // Update context menu
            const contextMenu = Menu.buildFromTemplate([
                {
                    label: \`Device ID: \${this.deviceId}\`,
                    enabled: false
                },
                {
                    label: \`Device: \${AGENT_CONFIG.deviceName}\`,
                    enabled: false
                },
                {
                    label: \`Status: \${status}\`,
                    enabled: false
                },
                { type: 'separator' },
                {
                    label: 'Copy Device ID',
                    click: () => {
                        require('electron').clipboard.writeText(this.deviceId);
                        console.log('📋 Device ID copied to clipboard');
                    }
                },
                {
                    label: 'Show Window',
                    click: () => this.showWindow()
                },
                {
                    label: 'Quit',
                    click: () => app.quit()
                }
            ]);
            
            this.tray.setContextMenu(contextMenu);
        }
    }

    showErrorDialog(title, message) {
        dialog.showErrorBox(title, message);
    }
}

// App initialization
app.whenReady().then(async () => {
    const agent = new RemoteDesktopAgent();
    await agent.initialize();
});

app.on('window-all-closed', () => {
    // Keep running in background
});

console.log('🚀 Remote Desktop Agent loaded and ready');
`;
}

function createExecutable(platform, script, config) {
    switch (platform) {
        case 'windows':
            return createWindowsExecutable(script, config);
        case 'macos':
            return createMacOSExecutable(script, config);
        case 'linux':
            return createLinuxExecutable(script, config);
        default:
            throw new Error(`Unsupported platform: ${platform}`);
    }
}

function createWindowsExecutable(script, config) {
    const batchScript = `@echo off
title Remote Desktop Agent Installer
echo.
echo ========================================
echo   Remote Desktop Agent Installer
echo ========================================
echo.
echo Device Name: ${config.deviceName}
echo Platform: Windows
echo Generated: ${config.generatedAt}
echo.

echo Installing Remote Desktop Agent...
echo.

REM Create agent directory
set AGENT_DIR=%USERPROFILE%\\RemoteDesktopAgent
if not exist "%AGENT_DIR%" mkdir "%AGENT_DIR%"
cd /d "%AGENT_DIR%"

echo Creating agent files...

REM Create package.json
(
echo {
echo   "name": "remote-desktop-agent",
echo   "version": "1.0.0",
echo   "description": "Auto-generated Remote Desktop Agent",
echo   "main": "agent.js",
echo   "scripts": {
echo     "start": "electron ."
echo   },
echo   "dependencies": {
echo     "electron": "^22.0.0",
echo     "@supabase/supabase-js": "^2.39.0",
echo     "canvas": "^2.11.2"
echo   }
echo }
) > package.json

REM Create the agent script
(
${script.split('\n').map(line => `echo ${line.replace(/"/g, '\\"')}`).join('\n')}
) > agent.js

echo.
echo Installing dependencies...
call npm install --silent

if errorlevel 1 (
    echo.
    echo ERROR: Failed to install dependencies
    echo Please make sure Node.js and npm are installed
    pause
    exit /b 1
)

echo.
echo Starting Remote Desktop Agent...
echo The agent will run in the background.
echo Check the system tray for the agent icon.
echo.

start "" npm start

echo.
echo Agent started successfully!
echo Device ID will be shown in the system tray.
echo.
pause
`;

    return Buffer.from(batchScript, 'utf8');
}

function createMacOSExecutable(script, config) {
    const shellScript = `#!/bin/bash
echo "========================================"
echo "  Remote Desktop Agent Installer"
echo "========================================"
echo ""
echo "Device Name: ${config.deviceName}"
echo "Platform: macOS"
echo "Generated: ${config.generatedAt}"
echo ""

echo "Installing Remote Desktop Agent..."
echo ""

# Create agent directory
AGENT_DIR="$HOME/Applications/RemoteDesktopAgent"
mkdir -p "$AGENT_DIR"
cd "$AGENT_DIR"

echo "Creating agent files..."

# Create package.json
cat > package.json << 'EOF'
{
  "name": "remote-desktop-agent",
  "version": "1.0.0",
  "description": "Auto-generated Remote Desktop Agent",
  "main": "agent.js",
  "scripts": {
    "start": "electron ."
  },
  "dependencies": {
    "electron": "^22.0.0",
    "@supabase/supabase-js": "^2.39.0",
    "canvas": "^2.11.2"
  }
}
EOF

# Create the agent script
cat > agent.js << 'EOF'
${script}
EOF

echo ""
echo "Installing dependencies..."
npm install --silent

if [ $? -ne 0 ]; then
    echo ""
    echo "ERROR: Failed to install dependencies"
    echo "Please make sure Node.js and npm are installed"
    exit 1
fi

echo ""
echo "Starting Remote Desktop Agent..."
echo "The agent will run in the background."
echo "Check the menu bar for the agent icon."
echo ""

npm start &

echo ""
echo "Agent started successfully!"
echo "Device ID will be shown in the menu bar."
echo ""
`;

    return Buffer.from(shellScript, 'utf8');
}

function createLinuxExecutable(script, config) {
    const shellScript = `#!/bin/bash
echo "========================================"
echo "  Remote Desktop Agent Installer"
echo "========================================"
echo ""
echo "Device Name: ${config.deviceName}"
echo "Platform: Linux"
echo "Generated: ${config.generatedAt}"
echo ""

echo "Installing Remote Desktop Agent..."
echo ""

# Create agent directory
AGENT_DIR="$HOME/.local/share/RemoteDesktopAgent"
mkdir -p "$AGENT_DIR"
cd "$AGENT_DIR"

echo "Creating agent files..."

# Create package.json
cat > package.json << 'EOF'
{
  "name": "remote-desktop-agent",
  "version": "1.0.0",
  "description": "Auto-generated Remote Desktop Agent",
  "main": "agent.js",
  "scripts": {
    "start": "electron ."
  },
  "dependencies": {
    "electron": "^22.0.0",
    "@supabase/supabase-js": "^2.39.0",
    "canvas": "^2.11.2"
  }
}
EOF

# Create the agent script
cat > agent.js << 'EOF'
${script}
EOF

echo ""
echo "Installing dependencies..."
npm install --silent

if [ $? -ne 0 ]; then
    echo ""
    echo "ERROR: Failed to install dependencies"
    echo "Please make sure Node.js and npm are installed"
    exit 1
fi

echo ""
echo "Starting Remote Desktop Agent..."
echo "The agent will run in the background."
echo "Check the system tray for the agent icon."
echo ""

npm start &

echo ""
echo "Agent started successfully!"
echo "Device ID will be shown in the system tray."
echo ""
`;

    return Buffer.from(shellScript, 'utf8');
}

function getFilename(platform, deviceName) {
    const safeName = deviceName.replace(/[^a-zA-Z0-9]/g, '_');
    const timestamp = new Date().toISOString().slice(0, 10);
    
    switch (platform) {
        case 'windows':
            return `RemoteAgent_${safeName}_${timestamp}.bat`;
        case 'macos':
            return `RemoteAgent_${safeName}_${timestamp}.sh`;
        case 'linux':
            return `RemoteAgent_${safeName}_${timestamp}.sh`;
        default:
            return `RemoteAgent_${safeName}_${timestamp}.txt`;
    }
}

// Start the test server
app.listen(PORT, () => {
    console.log(`🚀 Agent Builder Test Server running on http://localhost:${PORT}`);
    console.log(`📋 Test endpoint: http://localhost:${PORT}/test`);
    console.log(`🔧 Agent builder: http://localhost:${PORT}/functions/v1/agent-builder?platform=windows&deviceName=TestPC`);
    console.log('');
    console.log('Ready to test agent generation! 🎉');
});
