// Supabase Edge Function: Agent Builder
// Generates downloadable, pre-configured client agents like MeshCentral

import { serve } from "https://deno.land/std@0.168.0/http/server.ts"
import { createClient } from 'https://esm.sh/@supabase/supabase-js@2'

const corsHeaders = {
  'Access-Control-Allow-Origin': '*',
  'Access-Control-Allow-Headers': 'authorization, x-client-info, apikey, content-type',
}

interface AgentConfig {
  supabaseUrl: string
  supabaseKey: string
  deviceToken: string
  deviceName: string
  orgId: string
  serverName: string
  autoStart: boolean
  hideWindow: boolean
  platform: string
}

serve(async (req) => {
  // Handle CORS preflight requests
  if (req.method === 'OPTIONS') {
    return new Response('ok', { headers: corsHeaders })
  }

  try {
    const url = new URL(req.url)
    const platform = url.searchParams.get('platform') || 'windows'
    const deviceName = url.searchParams.get('deviceName') || 'Remote Device'
    const orgId = url.searchParams.get('orgId') || 'default'
    const autoStart = url.searchParams.get('autoStart') === 'true'
    const hideWindow = url.searchParams.get('hideWindow') !== 'false'

    // Validate platform
    const supportedPlatforms = ['windows', 'macos', 'linux']
    if (!supportedPlatforms.includes(platform)) {
      return new Response(
        JSON.stringify({ error: 'Unsupported platform' }),
        { status: 400, headers: { ...corsHeaders, 'Content-Type': 'application/json' } }
      )
    }

    // Generate unique device token
    const deviceToken = generateSecureToken(32)
    
    // Create agent configuration
    const agentConfig: AgentConfig = {
      supabaseUrl: Deno.env.get('SUPABASE_URL') || 'https://ptrtibzwokjcjjxvjpin.supabase.co',
      supabaseKey: Deno.env.get('SUPABASE_ANON_KEY') || 'sb_publishable_OwJ04BrJ6gZZ57fSeT-q5Q_mSsVaNia',
      deviceToken,
      deviceName,
      orgId,
      serverName: 'Remote Desktop System',
      autoStart,
      hideWindow,
      platform
    }

    // Generate the agent executable
    const agentExecutable = await generateAgentExecutable(platform, agentConfig)
    
    // Log agent generation
    console.log(`Generated agent for platform: ${platform}, device: ${deviceName}`)

    // Return the executable file
    const filename = getFilename(platform, deviceName)
    return new Response(agentExecutable, {
      headers: {
        ...corsHeaders,
        'Content-Type': 'application/octet-stream',
        'Content-Disposition': `attachment; filename="${filename}"`,
        'Content-Length': agentExecutable.byteLength.toString()
      }
    })

  } catch (error) {
    console.error('Agent builder error:', error)
    return new Response(
      JSON.stringify({ error: 'Failed to generate agent', details: error.message }),
      { status: 500, headers: { ...corsHeaders, 'Content-Type': 'application/json' } }
    )
  }
})

function generateSecureToken(length: number): string {
  const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789'
  let result = ''
  const randomArray = new Uint8Array(length)
  crypto.getRandomValues(randomArray)
  
  for (let i = 0; i < length; i++) {
    result += chars[randomArray[i] % chars.length]
  }
  return result
}

async function generateAgentExecutable(platform: string, config: AgentConfig): Promise<Uint8Array> {
  // In a real implementation, this would:
  // 1. Load the base executable template for the platform
  // 2. Inject the configuration into the executable
  // 3. Sign the executable (for security)
  // 4. Return the customized executable
  
  // For now, we'll create a script-based agent that can be easily distributed
  const agentScript = createAgentScript(platform, config)
  
  switch (platform) {
    case 'windows':
      return createWindowsExecutable(agentScript, config)
    case 'macos':
      return createMacOSApp(agentScript, config)
    case 'linux':
      return createLinuxPackage(agentScript, config)
    default:
      throw new Error(`Unsupported platform: ${platform}`)
  }
}

function createAgentScript(platform: string, config: AgentConfig): string {
  return `
// Auto-generated Remote Desktop Agent
// Platform: ${platform}
// Generated: ${new Date().toISOString()}

const { app, BrowserWindow, Tray, Menu, ipcMain, dialog } = require('electron');
const path = require('path');
const os = require('os');
const { createClient } = require('@supabase/supabase-js');

// Embedded configuration (injected during build)
const AGENT_CONFIG = ${JSON.stringify(config, null, 2)};

class RemoteDesktopAgent {
    constructor() {
        this.deviceId = this.generateDeviceId();
        this.supabase = createClient(AGENT_CONFIG.supabaseUrl, AGENT_CONFIG.supabaseKey);
        this.isConnected = false;
        this.tray = null;
        
        console.log('🚀 Remote Desktop Agent starting...');
        console.log('📱 Device ID:', this.deviceId);
        console.log('🌍 Server:', AGENT_CONFIG.supabaseUrl);
    }

    async initialize() {
        try {
            // Create system tray
            this.createTray();
            
            // Connect to Supabase
            await this.connectToSupabase();
            
            // Register device
            await this.registerDevice();
            
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
            const deviceInfo = {
                device_id: this.deviceId,
                device_name: AGENT_CONFIG.deviceName || os.hostname(),
                operating_system: process.platform,
                status: 'online',
                last_seen: new Date().toISOString(),
                metadata: {
                    version: '1.0.0',
                    platform: '${platform}',
                    auto_generated: true,
                    org_id: AGENT_CONFIG.orgId,
                    device_token: AGENT_CONFIG.deviceToken
                }
            };

            const { error } = await this.supabase
                .from('remote_devices')
                .upsert(deviceInfo);

            if (error) throw error;

            console.log('✅ Device registered globally');
            this.updateTrayTooltip('Online - Ready for remote control');
            
        } catch (error) {
            console.error('❌ Device registration failed:', error);
            throw error;
        }
    }

    createTray() {
        try {
            // Create a simple tray icon (in real implementation, this would be embedded)
            this.tray = new Tray(this.createTrayIcon());
            
            const contextMenu = Menu.buildFromTemplate([
                {
                    label: \`Device ID: \${this.deviceId}\`,
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
                        require('electron').clipboard.writeText(this.deviceId);
                    }
                },
                {
                    label: 'Quit',
                    click: () => app.quit()
                }
            ]);
            
            this.tray.setContextMenu(contextMenu);
            this.tray.setToolTip('Remote Desktop Agent - Connecting...');
            
        } catch (error) {
            console.error('⚠️ Failed to create tray icon:', error);
        }
    }

    createTrayIcon() {
        // Create a simple base64 icon (in real implementation, this would be a proper icon file)
        const iconData = 'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg==';
        return nativeImage.createFromDataURL(iconData);
    }

    updateTrayTooltip(status) {
        if (this.tray) {
            this.tray.setToolTip(\`Remote Desktop Agent - \${status}\`);
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

${platform === 'windows' ? '// Windows-specific code' : ''}
${platform === 'macos' ? '// macOS-specific code' : ''}
${platform === 'linux' ? '// Linux-specific code' : ''}
`;
}

function createWindowsExecutable(script: string, config: AgentConfig): Uint8Array {
  // Create a Windows batch file that downloads and runs the agent
  const batchScript = `
@echo off
echo Installing Remote Desktop Agent...
echo Device Name: ${config.deviceName}
echo.

REM Create temp directory
mkdir "%TEMP%\\RemoteDesktopAgent" 2>nul
cd /d "%TEMP%\\RemoteDesktopAgent"

REM Download Node.js portable if not exists
if not exist "node.exe" (
    echo Downloading Node.js...
    powershell -Command "Invoke-WebRequest -Uri 'https://nodejs.org/dist/v18.17.0/win-x64/node.exe' -OutFile 'node.exe'"
)

REM Create the agent script
echo Creating agent...
(
echo ${script.replace(/\n/g, '\necho ')}
) > agent.js

REM Create package.json
(
echo {
echo   "name": "remote-desktop-agent",
echo   "version": "1.0.0",
echo   "main": "agent.js",
echo   "dependencies": {
echo     "electron": "^22.0.0",
echo     "@supabase/supabase-js": "^2.39.0"
echo   }
echo }
) > package.json

REM Install dependencies and run
echo Installing dependencies...
npm install --silent

echo Starting Remote Desktop Agent...
npm start

pause
`;

  return new TextEncoder().encode(batchScript);
}

function createMacOSApp(script: string, config: AgentConfig): Uint8Array {
  // Create a macOS shell script
  const shellScript = `#!/bin/bash
echo "Installing Remote Desktop Agent..."
echo "Device Name: ${config.deviceName}"
echo

# Create app directory
mkdir -p ~/Applications/RemoteDesktopAgent.app/Contents/MacOS
cd ~/Applications/RemoteDesktopAgent.app/Contents/MacOS

# Create the agent script
cat > agent.js << 'EOF'
${script}
EOF

# Create package.json
cat > package.json << 'EOF'
{
  "name": "remote-desktop-agent",
  "version": "1.0.0",
  "main": "agent.js",
  "dependencies": {
    "electron": "^22.0.0",
    "@supabase/supabase-js": "^2.39.0"
  }
}
EOF

# Install dependencies
echo "Installing dependencies..."
npm install --silent

echo "Starting Remote Desktop Agent..."
npm start
`;

  return new TextEncoder().encode(shellScript);
}

function createLinuxPackage(script: string, config: AgentConfig): Uint8Array {
  // Create a Linux shell script
  const shellScript = `#!/bin/bash
echo "Installing Remote Desktop Agent..."
echo "Device Name: ${config.deviceName}"
echo

# Create app directory
mkdir -p ~/.local/share/RemoteDesktopAgent
cd ~/.local/share/RemoteDesktopAgent

# Create the agent script
cat > agent.js << 'EOF'
${script}
EOF

# Create package.json
cat > package.json << 'EOF'
{
  "name": "remote-desktop-agent",
  "version": "1.0.0",
  "main": "agent.js",
  "dependencies": {
    "electron": "^22.0.0",
    "@supabase/supabase-js": "^2.39.0"
  }
}
EOF

# Install dependencies
echo "Installing dependencies..."
npm install --silent

echo "Starting Remote Desktop Agent..."
npm start
`;

  return new TextEncoder().encode(shellScript);
}

function getFilename(platform: string, deviceName: string): string {
  const safeName = deviceName.replace(/[^a-zA-Z0-9]/g, '_');
  
  switch (platform) {
    case 'windows':
      return `RemoteDesktopAgent_${safeName}.bat`;
    case 'macos':
      return `RemoteDesktopAgent_${safeName}.sh`;
    case 'linux':
      return `RemoteDesktopAgent_${safeName}.sh`;
    default:
      return `RemoteDesktopAgent_${safeName}.txt`;
  }
}
