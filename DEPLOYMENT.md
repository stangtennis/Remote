# Deployment Guide

## ✅ Prerequisites

### 1. Install Required Tools

#### MinGW-w64 (For CGO/robotgo support)
```powershell
# Option 1: Chocolatey
choco install mingw

# Option 2: Manual download
# Download from: https://sourceforge.net/projects/mingw-w64/
# Add to PATH: C:\mingw64\bin
```

#### Verify GCC Installation
```powershell
gcc --version
# Should output: gcc (MinGW-w64) ...
```

#### Go 1.21+
```powershell
go version
# Should output: go version go1.21.x windows/amd64
```

### 2. Supabase Setup

```powershell
# Install Supabase CLI
scoop install supabase

# Login
supabase login

# Link to your project
cd f:\#Remote
supabase link --project-ref your-project-ref
```

## 🚀 Deployment Steps

### Step 1: Deploy Database Migrations

```powershell
cd f:\#Remote\supabase
supabase db push
```

This will create:
- All database tables
- RLS policies
- Triggers and functions
- Session cleanup cron job (runs every 5 minutes)

### Step 2: Deploy Edge Functions

```powershell
# Deploy all edge functions
supabase functions deploy session-token
supabase functions deploy device-register
supabase functions deploy file-transfer
supabase functions deploy session-cleanup
```

### Step 3: Configure Environment Variables

```powershell
# Set secrets for Edge Functions
supabase secrets set TWILIO_ACCOUNT_SID=your_sid
supabase secrets set TWILIO_AUTH_TOKEN=your_token
```

### Step 4: Build Agent (WITH Input Control)

```powershell
cd f:\#Remote\agent

# Using build script (recommended)
.\build.bat

# OR manual build with CGO
set CGO_ENABLED=1
go build -o remote-agent.exe ./cmd/remote-agent
```

**Important**: If build fails with robotgo errors:
1. Ensure MinGW-w64 GCC is installed and in PATH
2. Ensure CGO_ENABLED=1 is set
3. Check that go-vgo/robotgo dependencies are satisfied

### Step 5: Deploy Dashboard to GitHub Pages

```powershell
cd f:\#Remote

# Commit and push
git add .
git commit -m "Deploy updates"
git push origin main
```

Then in GitHub repository settings:
1. Go to Settings → Pages
2. Source: Deploy from branch
3. Branch: `main` / `dashboard` folder
4. Save

Dashboard will be available at: `https://yourusername.github.io/Remote/`

## 🧪 Testing

### Test Session Cleanup

```sql
-- Run manual cleanup in Supabase SQL Editor
SELECT cleanup_old_sessions_direct();

-- View cron jobs
SELECT * FROM cron.job;

-- View cron run history
SELECT * FROM cron.job_run_details ORDER BY start_time DESC LIMIT 10;
```

### Test Agent

```powershell
cd f:\#Remote\agent
.\remote-agent.exe
```

Expected output:
```
🖥️  Remote Desktop Agent Starting...
=====================================
📱 Registering device...
✅ Device approved and ready
✅ Device registered: dev-xxxxx
   Name: your-computer
   Platform: windows
   Arch: amd64
👂 Listening for incoming connections...
```

### Test Dashboard Connection

1. Open https://yourusername.github.io/Remote/dashboard.html
2. Login with your credentials
3. You should see your device online
4. Click "Connect"
5. Enter PIN shown in agent terminal
6. You should see screen streaming and be able to control mouse/keyboard

## 🔧 Troubleshooting

### Agent Build Errors

**Error: `gcc: command not found`**
- Install MinGW-w64 and add to PATH
- Restart terminal after installation

**Error: `undefined reference to 'robotgo.XXX'`**
- Ensure CGO_ENABLED=1
- Run `go clean -cache`
- Try `go get -u github.com/go-vgo/robotgo`

**Error: `cgo: C compiler not found`**
- Install GCC via MinGW-w64
- Verify with `gcc --version`

### Connection Issues

**Sessions not cleaning up**
- Check pg_cron is enabled: `SELECT * FROM pg_extension WHERE extname = 'pg_cron';`
- Verify cron job exists: `SELECT * FROM cron.job;`
- Run manual cleanup: `SELECT cleanup_old_sessions_direct();`

**Multiple dashboard tabs causing conflicts**
- **Only use ONE browser tab** for the dashboard
- Close all other tabs before connecting

**TURN not working**
- Verify Twilio credentials in Edge Function secrets
- Check Edge Function logs: `supabase functions logs session-token`

**Input (mouse/keyboard) not working**
- Ensure agent was built WITH CGO enabled
- Check if robotgo is working: Test by moving mouse manually
- On Windows: May need to run as Administrator

## 🔐 Production Hardening

### Code Signing (Recommended)

```powershell
# Purchase certificate from DigiCert/Sectigo ($200-500/year)

# Sign the binary
signtool sign /f cert.pfx /p password /t http://timestamp.digicert.com remote-agent.exe

# Verify signature
signtool verify /pa remote-agent.exe
```

### Security Checklist

- [ ] Code signing certificate applied to agent EXE
- [ ] HTTPS enforced on dashboard (GitHub Pages does this automatically)
- [ ] RLS policies reviewed and tested
- [ ] Rate limiting enabled in Edge Functions
- [ ] MFA enabled for dashboard users
- [ ] API keys rotated regularly
- [ ] Audit logs monitored
- [ ] Session cleanup cron job verified working

## 📊 Monitoring

### Check Edge Function Logs

```powershell
# View recent logs
supabase functions logs session-token --tail
supabase functions logs session-cleanup --tail

# Check for errors
supabase functions logs session-token --level error
```

### Database Metrics

```sql
-- Active sessions
SELECT COUNT(*) FROM remote_sessions WHERE status = 'active';

-- Online devices
SELECT COUNT(*) FROM remote_devices WHERE is_online = true;

-- Session signaling queue size
SELECT COUNT(*) FROM session_signaling WHERE created_at > NOW() - INTERVAL '5 minutes';

-- Recent cleanup runs
SELECT * FROM cron.job_run_details 
WHERE jobid = (SELECT jobid FROM cron.job WHERE jobname = 'session-cleanup')
ORDER BY start_time DESC LIMIT 5;
```

## 🎯 Performance Optimization

### Current State (JPEG Streaming)
- Frame rate: ~10 FPS
- Quality: 50% JPEG
- Bandwidth: ~500 KB/s

### Next Steps (H.264/VP8)
See `OPTIMIZATION.md` for video encoding implementation guide.

## 📝 Manual Session Cleanup (Emergency)

If sessions get stuck:

```sql
-- Nuclear option: Delete all sessions and signaling
DELETE FROM session_signaling;
DELETE FROM remote_sessions WHERE status IN ('pending', 'active');

-- Softer option: Just expired ones
DELETE FROM session_signaling WHERE created_at < NOW() - INTERVAL '1 minute';
UPDATE remote_sessions SET status = 'expired', ended_at = NOW() 
WHERE status IN ('pending', 'active') AND created_at < NOW() - INTERVAL '15 minutes';
```

## 🚨 Emergency Procedures

### Agent Won't Connect
1. Stop agent (Ctrl+C)
2. Clean sessions in Supabase SQL Editor
3. Restart agent
4. Refresh dashboard (F5)
5. Try connecting again

### Connection Freezes
1. Check data channel state in browser console
2. Verify ICE candidates are exchanging
3. Check TURN credentials haven't expired
4. Restart both agent and dashboard

### High Database Load
1. Check session cleanup is running: `SELECT * FROM cron.job_run_details LIMIT 10;`
2. Manually run cleanup if needed: `SELECT cleanup_old_sessions_direct();`
3. Consider reducing cleanup interval in cron job

---

**Last Updated**: 2025-10-02
**Status**: ✅ Working (Screen streaming + Input control enabled)
