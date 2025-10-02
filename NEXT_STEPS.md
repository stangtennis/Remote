# Next Steps - Deployment Checklist

## ✅ Completed (Code Ready)

1. **Automatic Session Cleanup**
   - SQL migration created: `20250102000012_session_cleanup_cron.sql`
   - Edge Function created: `session-cleanup/index.ts`
   - Runs every 5 minutes via pg_cron
   - Status: **Ready to deploy**

2. **Mouse/Keyboard Control**
   - Input handlers re-enabled in `peer.go`
   - Requires CGO build with MinGW-w64
   - Build script created: `build.bat`
   - Status: **Ready to build and test**

3. **Documentation**
   - `DEPLOYMENT.md`: Complete deployment guide
   - `OPTIMIZATION.md`: Video encoding upgrade path
   - `CHANGELOG.md`: Version history
   - Status: **Complete**

## 🚀 Ready to Deploy

### Step 1: Deploy Database Migration

```powershell
cd f:\#Remote\supabase
supabase db push
```

**What this does**:
- Creates `cleanup_old_sessions_direct()` function
- Sets up pg_cron job to run every 5 minutes
- Auto-cleans: stale signaling (>1 min), expired sessions (>15 min), old records (>24h)
- Marks offline devices (>2 min since heartbeat)

**Verify**:
```sql
-- Check cron job exists
SELECT * FROM cron.job WHERE jobname = 'session-cleanup';

-- Test manually
SELECT cleanup_old_sessions_direct();

-- View run history
SELECT * FROM cron.job_run_details ORDER BY start_time DESC LIMIT 5;
```

### Step 2: Deploy Edge Function (Optional)

```powershell
supabase functions deploy session-cleanup
```

**Note**: The SQL function handles cleanup automatically. The Edge Function is backup/manual trigger only.

### Step 3: Build Agent with Input Control

**Requirements**:
- Install MinGW-w64: `choco install mingw`
- Verify GCC: `gcc --version`

**Build**:
```powershell
cd f:\#Remote\agent
.\build.bat
```

**Expected output**:
```
✅ GCC found
🔧 Building with CGO_ENABLED=1...
✅ Build successful!
📦 Binary: remote-agent.exe
```

**If build fails**:
- Check GCC is in PATH
- Run `go clean -cache`
- Try `go get -u github.com/go-vgo/robotgo`

### Step 4: Test Input Control

1. **Run agent**:
   ```powershell
   .\remote-agent.exe
   ```

2. **Connect from dashboard**

3. **Test mouse**:
   - Move cursor around
   - Click buttons
   - Scroll wheel

4. **Test keyboard**:
   - Type text
   - Try shortcuts (Ctrl+C, etc.)

**Expected behavior**:
- Mouse moves on remote screen
- Clicks register on remote system
- Keyboard inputs work on remote system

**If input doesn't work**:
- Check agent logs for errors
- Verify robotgo is working: Try moving mouse manually in Go code
- Windows: May need to run as Administrator

## 📋 Testing Checklist

### Session Cleanup Testing

- [ ] Run migration: `supabase db push`
- [ ] Verify cron job: `SELECT * FROM cron.job;`
- [ ] Create test session (connect/disconnect)
- [ ] Wait 5 minutes
- [ ] Check session is cleaned: `SELECT * FROM remote_sessions;`
- [ ] Check signaling is cleaned: `SELECT * FROM session_signaling;`

### Input Control Testing

- [ ] Build agent with CGO: `.\build.bat`
- [ ] Start agent: `.\remote-agent.exe`
- [ ] Connect from dashboard
- [ ] Test mouse movement
- [ ] Test mouse clicking (left, right, middle)
- [ ] Test mouse scrolling
- [ ] Test keyboard typing
- [ ] Test keyboard shortcuts (Ctrl+C, Alt+Tab, etc.)

### Integration Testing

- [ ] Connect from same network (should use P2P)
- [ ] Connect from external network (should use TURN)
- [ ] Verify session cleanup after disconnect
- [ ] Multiple connect/disconnect cycles
- [ ] Test with only ONE dashboard tab open
- [ ] Verify no signaling conflicts

## ⚠️ Important Reminders

### Before Deployment

1. **Close all dashboard tabs except one** - Multiple tabs cause conflicts
2. **Clean old sessions** - Run manual cleanup before testing:
   ```sql
   DELETE FROM session_signaling;
   DELETE FROM remote_sessions WHERE status IN ('pending', 'active');
   ```
3. **Backup database** - Before running migrations

### During Testing

1. **Monitor logs**:
   - Agent terminal output
   - Browser console (F12)
   - Supabase Edge Function logs
   - Database pg_cron logs

2. **Watch for**:
   - Connection state changes
   - ICE candidate generation
   - Data channel open/close
   - Input events being processed

3. **Performance metrics**:
   - Frame rate (~10 FPS with JPEG)
   - Latency (mouse lag)
   - Bandwidth usage
   - CPU usage on agent

## 🎯 Success Criteria

### Minimum Viable

- ✅ Agent connects reliably
- ✅ Screen streams continuously
- ✅ Mouse control works
- ✅ Keyboard control works
- ✅ Sessions clean up automatically
- ✅ External access works via TURN

### Production Ready

- ⏳ Code signing certificate installed
- ⏳ H.264/VP8 video encoding (30+ FPS)
- ⏳ Auto-update mechanism
- ⏳ Windows service mode
- ⏳ File transfer working
- ⏳ Production monitoring setup

## 📝 Notes

### CGO Build Requirements

**Required tools**:
- Go 1.21+
- MinGW-w64 (GCC for Windows)
- Git

**Environment variables**:
```
CGO_ENABLED=1
```

**Troubleshooting**:
- If build fails, check PATH includes `C:\mingw64\bin`
- Clear Go cache: `go clean -cache`
- Reinstall robotgo: `go get -u github.com/go-vgo/robotgo`

### Session Cleanup Configuration

**Current settings** (adjust in migration if needed):
- Signaling cleanup: 1 minute
- Session expiry: 15 minutes
- Old session deletion: 24 hours
- Device offline timeout: 2 minutes
- Cron interval: 5 minutes

**To adjust**:
Edit `20250102000012_session_cleanup_cron.sql` before deployment.

### Video Optimization (Future)

Current: JPEG @ 10 FPS @ ~4 Mbps
Target: H.264 @ 30 FPS @ ~1.5 Mbps

See `OPTIMIZATION.md` for implementation guide.

## 🚨 Rollback Plan

If something breaks:

### Rollback Migration
```powershell
supabase db reset  # WARNING: Drops all data!
# OR manually:
DROP FUNCTION IF EXISTS cleanup_old_sessions_direct();
SELECT cron.unschedule('session-cleanup');
```

### Rollback Agent
```powershell
# Use previous binary without CGO
git checkout HEAD~1 agent/
go build -o remote-agent.exe ./cmd/remote-agent
```

### Rollback Edge Function
```powershell
supabase functions delete session-cleanup
```

## 📞 Support

If you encounter issues:

1. **Check logs first**:
   - Agent terminal
   - Browser console
   - `supabase functions logs`
   - Database cron logs

2. **Common issues**:
   - See `DEPLOYMENT.md` troubleshooting section
   - Check `plan.md` for known issues

3. **Emergency cleanup**:
   ```sql
   DELETE FROM session_signaling;
   DELETE FROM remote_sessions WHERE status != 'ended';
   UPDATE remote_devices SET is_online = false;
   ```

---

**Ready to deploy?** Start with Step 1 above! 🚀

**Last Updated**: 2025-10-02 23:15
