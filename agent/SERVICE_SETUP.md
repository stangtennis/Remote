# Remote Desktop Agent - Windows Service Setup

This guide explains how to install and run the Remote Desktop Agent as a Windows Service, enabling remote access even when no user is logged in (login screen / Session 0).

## Features

- **Pre-Login Access**: Connect to the PC even at the Windows login screen
- **Auto-Start**: Service starts automatically when Windows boots
- **Desktop Switching**: Automatically handles transitions between login screen and user desktop
- **Persistent Connection**: Maintains connection through user login/logout

## Quick Start

All service management is built into the agent executable. **Run as Administrator:**

```batch
# Install the service
remote-agent.exe -install

# Start the service
remote-agent.exe -start

# Check status
remote-agent.exe -status

# Stop the service
remote-agent.exe -stop

# Uninstall the service
remote-agent.exe -uninstall
```

## Command Reference

| Command | Description |
|---------|-------------|
| `remote-agent.exe` | Run interactively (with system tray) |
| `remote-agent.exe -install` | Install as Windows Service |
| `remote-agent.exe -uninstall` | Uninstall Windows Service |
| `remote-agent.exe -start` | Start the service |
| `remote-agent.exe -stop` | Stop the service |
| `remote-agent.exe -status` | Show service status |
| `remote-agent.exe -help` | Show help |

## Installation Steps

1. **Build the agent** (if not already built):
   ```batch
   cd agent
   .\build.bat
   ```

2. **Open Administrator Command Prompt**

3. **Install and start**:
   ```batch
   remote-agent.exe -install
   remote-agent.exe -start
   ```

That's it! The service will now:
- Start automatically when Windows boots
- Run before any user logs in
- Capture the login screen
- Continue running through user login/logout

## Viewing Logs

The agent writes logs to `agent.log` in the same directory as the executable.

```batch
# View logs
.\view-logs.bat

# Or manually
type agent.log

# Follow logs in real-time (PowerShell)
Get-Content agent.log -Wait -Tail 50
```

## How Session 0 Support Works

### Desktop Detection
The agent automatically detects which desktop is active:
- **Default**: Normal user desktop (after login)
- **Winlogon**: Windows login screen (before login)
- **Screen-saver**: Screen saver active

### Screen Capture Modes
- **DXGI** (Desktop Duplication API): Used for normal user desktop - best quality and performance
- **GDI**: Used for login screen (Session 0) - works when DXGI is unavailable

### Automatic Switching
When the desktop changes (e.g., user locks PC or logs out):
1. Agent detects desktop switch
2. Reinitializes screen capturer with appropriate mode
3. Continues streaming without disconnection

## Troubleshooting

### Service Won't Start

1. **Check if executable exists**:
   ```batch
   dir remote-agent.exe
   ```

2. **Check Windows Event Log**:
   - Open Event Viewer
   - Navigate to Windows Logs > Application
   - Look for "RemoteDesktopAgent" entries

3. **Check agent.log** for errors

### Can't Capture Login Screen

1. **Verify service is running as LocalSystem**:
   ```batch
   sc qc RemoteDesktopAgent
   ```
   Should show `SERVICE_START_NAME : LocalSystem`

2. **Check if GDI capture is working**:
   Look for "GDI capturer ready" in logs

### Connection Issues

1. **Verify device is registered**:
   Check Supabase dashboard for device entry

2. **Check network connectivity**:
   Ensure the PC can reach Supabase URL

3. **Firewall**:
   WebRTC uses UDP - ensure firewall allows outbound UDP

### Service Keeps Restarting

Check `agent.log` for crash reasons. Common issues:
- Missing dependencies (GCC runtime)
- Network connectivity problems
- Invalid configuration

## Security Considerations

1. **LocalSystem Account**: The service runs as LocalSystem for Session 0 access. This is a high-privilege account.

2. **Network Access**: The agent connects to Supabase for signaling. Ensure your network allows this.

3. **Screen Capture**: The agent can capture any screen content, including sensitive information.

4. **Input Control**: The agent can control mouse and keyboard. Only allow trusted controllers.

## Configuration

The agent reads configuration from environment variables or `.env` file:

```env
SUPABASE_URL=http://192.168.1.92:8888
SUPABASE_ANON_KEY=your-anon-key
DEVICE_NAME=MyPC
```

For service mode, set environment variables system-wide or modify the service configuration.

## Version

Current version: v2.3.0 (Built-in Service Management + Session 0 Support)
