# FFmpeg Setup for Remote Desktop Agent

## Why FFmpeg?

FFmpeg's GDI capture works reliably with RDP sessions, unlike DXGI which is blocked by Windows security.

## Quick Install (Recommended)

### Option 1: Download Binary (Fastest)

1. **Download FFmpeg:**
   - Visit: https://github.com/BtbN/FFmpeg-Builds/releases
   - Download: `ffmpeg-master-latest-win64-gpl.zip`

2. **Extract:**
   ```powershell
   # Extract to C:\ffmpeg
   Expand-Archive -Path "ffmpeg-master-latest-win64-gpl.zip" -DestinationPath "C:\ffmpeg"
   ```

3. **Add to PATH:**
   ```powershell
   # Add to system PATH (requires admin)
   [Environment]::SetEnvironmentVariable("Path", $env:Path + ";C:\ffmpeg\bin", [EnvironmentVariableTarget]::Machine)
   
   # OR add to user PATH (no admin needed)
   [Environment]::SetEnvironmentVariable("Path", $env:Path + ";C:\ffmpeg\bin", [EnvironmentVariableTarget]::User)
   ```

4. **Restart PowerShell** and verify:
   ```powershell
   ffmpeg -version
   ```

### Option 2: Use Chocolatey (If installed)

```powershell
choco install ffmpeg
```

### Option 3: Use Winget (Windows 11)

```powershell
winget install ffmpeg
```

## Alternative: Place FFmpeg in Agent Directory

If you don't want to modify PATH, just place `ffmpeg.exe` in the same directory as `remote-agent.exe`:

```
F:\#Remote\agent\
  ├── remote-agent.exe
  └── ffmpeg.exe
```

The agent will automatically find it!

## Verify Installation

Run the agent and look for:
```
✅ Using FFmpeg GDI capture (works with RDP)
```

If you see this, FFmpeg is working correctly!

## Troubleshooting

### "ffmpeg not found"
- Make sure `ffmpeg.exe` is in PATH or in the agent directory
- Restart your terminal/PowerShell after PATH changes
- Try running `ffmpeg -version` to test

### Still getting errors?
The agent will automatically fall back to other capture methods. Check the startup logs.
