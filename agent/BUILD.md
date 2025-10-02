# Agent Build Guide

## Step 1: Install Go

1. Download Go: https://go.dev/dl/go1.22.0.windows-amd64.msi
2. Run the installer
3. Verify installation:
   ```powershell
   go version
   ```

## Step 2: Install Dependencies

```powershell
cd f:\#Remote\agent

# Download all Go dependencies
go mod tidy
```

## Step 3: Build the Agent

```powershell
# Development build (with debug info)
go build -o remote-agent.exe ./cmd/remote-agent

# Production build (optimized, smaller)
go build -ldflags="-s -w" -o remote-agent.exe ./cmd/remote-agent
```

## Step 4: Run the Agent

```powershell
# Run directly
.\remote-agent.exe

# Or run without building
go run ./cmd/remote-agent
```

## Expected Output

```
🖥️  Remote Desktop Agent Starting...
=====================================
📱 Registering device...
✅ Device registered: dev-abc123
   Name: YOUR-PC
   Platform: windows
   Arch: amd64
⏳ Device registered. Waiting for owner approval...
   Go to dashboard and approve this device
```

## Step 5: Approve Device in Dashboard

1. Go to: https://stangtennis.github.io/Remote/
2. Login
3. You'll see your device with "Pending Approval" badge
4. Click **Approve** button

Agent will show:
```
✅ Device approved!
👂 Listening for incoming connections...
```

## Step 6: Test Connection

1. In dashboard, click your device
2. Click **Connect** button
3. Agent will show:
   ```
   📞 Incoming session: abc-123 (PIN: 123456)
   🔧 Setting up WebRTC connection...
   ⏳ Waiting for offer from dashboard...
   📨 Received offer from dashboard
   📤 Sent answer to dashboard
   ✅ WebRTC connected!
   📡 Data channel opened: control
   ✅ Data channel ready
   🎥 Starting screen streaming...
   ```

4. Dashboard should show your screen!

## Troubleshooting

### "go: command not found"
- Restart PowerShell after installing Go
- Check PATH includes Go bin directory

### Build errors about missing packages
```powershell
go mod tidy
go clean -modcache
go mod download
```

### robotgo build errors
- robotgo requires gcc for Windows
- Install: https://jmeubank.github.io/tdm-gcc/download/
- Or use pre-built version (will try to auto-download)

### Screen capture not working
- Requires active desktop session
- Won't work in background service (Phase 6 feature)
- Check display permissions

### Device not appearing in dashboard
- Check SUPABASE_URL and ANON_KEY in config.go
- Verify agent shows "Device registered"
- Check network/firewall

### WebRTC not connecting
- Check dashboard console for errors
- Verify TURN credentials (if behind strict NAT)
- Check firewall allows UDP traffic

### Input (mouse/keyboard) not working
- robotgo needs admin privileges on some systems
- Run agent as administrator: Right-click → Run as administrator

## Build for Distribution

```powershell
# Clean build
go clean
go build -ldflags="-s -w" -o remote-agent.exe ./cmd/remote-agent

# Optional: Compress with UPX
# Download UPX: https://github.com/upx/upx/releases
upx --best remote-agent.exe

# File size: ~15-20MB (without UPX), ~5-8MB (with UPX)
```

## Next Steps

After successful connection:
- Test mouse movement and clicks
- Test keyboard input
- Check connection stats in dashboard
- Test reconnection (close dashboard and reopen)

Then proceed to Fase 3: TURN integration for NAT traversal!
