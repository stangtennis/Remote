# Testing Guide - Mouse/Keyboard Control & Session Cleanup

## 🎯 What We're Testing

1. **Mouse Control**: Movement, clicking, scrolling
2. **Keyboard Control**: Typing, shortcuts
3. **Automatic Session Cleanup**: pg_cron cleanup every 5 minutes
4. **Connection Stability**: With input control enabled

## ✅ Prerequisites

- [x] Migration deployed (`20250102000012_session_cleanup_cron.sql`)
- [x] Edge Function deployed (`session-cleanup`)
- [x] Agent built with CGO (`remote-agent.exe`)
- [ ] Old sessions cleaned up

## 🧹 Step 1: Clean Old Sessions

**Run in Supabase SQL Editor**:
```sql
DELETE FROM session_signaling;
DELETE FROM remote_sessions WHERE status IN ('pending', 'active');
UPDATE remote_devices SET is_online = false;
```

Or use: `clean_sessions.sql`

## 🚀 Step 2: Start Agent

```powershell
cd f:\#Remote\agent
.\remote-agent.exe
```

**Expected output**:
```
🖥️  Remote Desktop Agent Starting...
=====================================
📱 Registering device...
✅ Device registered: dev-xxxxx
👂 Listening for incoming connections...
```

## 🌐 Step 3: Connect from Dashboard

1. **Close ALL other dashboard tabs** (critical!)
2. Open: https://stangtennis.github.io/Remote/dashboard.html
3. Login with your credentials
4. Device should show as "Online"
5. Click **Connect**
6. Enter PIN from agent terminal

**Expected**:
- Connection establishes in <5 seconds
- Screen streaming starts
- No errors in browser console

## 🖱️ Step 4: Test Mouse Control

### Test 4.1: Mouse Movement
1. Move cursor around in browser
2. **Watch remote screen** - cursor should move
3. Try corners and edges

**Expected**: 
- Smooth cursor movement on remote screen
- No lag >200ms (LAN) or >500ms (TURN)
- Coordinates map correctly

### Test 4.2: Mouse Clicking
1. **Left click** on remote desktop icon
2. **Right click** to open context menu
3. **Middle click** (scroll wheel)
4. **Double click** to open folder

**Expected**:
- All click types register correctly
- Context menus appear
- Programs launch

### Test 4.3: Mouse Scrolling
1. Open a long webpage on remote
2. **Scroll up/down** in browser
3. Try fast scrolling
4. Try precise scrolling

**Expected**:
- Page scrolls smoothly
- Direction is correct
- Speed feels natural

## ⌨️ Step 5: Test Keyboard Control

### Test 5.1: Basic Typing
1. Click on Notepad on remote screen
2. Type: `Hello World! 123 !@#$%`
3. Try uppercase and lowercase

**Expected**:
- All characters appear correctly
- Special characters work
- No missed keystrokes

### Test 5.2: Keyboard Shortcuts
Test these shortcuts:

- **Ctrl+C** (copy)
- **Ctrl+V** (paste)
- **Ctrl+A** (select all)
- **Alt+Tab** (switch windows)
- **Win+E** (open Explorer)
- **Ctrl+Shift+Esc** (Task Manager)

**Expected**:
- All shortcuts work
- Modifier keys register correctly
- No stuck keys

### Test 5.3: Function Keys
- **F2** (rename)
- **F5** (refresh)
- **F11** (fullscreen)

**Expected**:
- Function keys work as expected

## 🧪 Step 6: Test Session Cleanup

### Test 6.1: Manual Cleanup
Run in Supabase SQL Editor:
```sql
SELECT cleanup_old_sessions_direct();
```

**Expected**:
```
✅ Cleaned up old signaling messages
✅ Expired old sessions
✅ Deleted old completed sessions
✅ Marked inactive devices as offline
```

### Test 6.2: Verify Cron Job
```sql
-- Check job exists
SELECT * FROM cron.job WHERE jobname = 'session-cleanup';

-- Check recent runs
SELECT * FROM cron.job_run_details 
WHERE jobid = (SELECT jobid FROM cron.job WHERE jobname = 'session-cleanup')
ORDER BY start_time DESC LIMIT 5;
```

**Expected**:
- Job exists with schedule `*/5 * * * *`
- Recent runs show success
- No error messages

### Test 6.3: Test Automatic Cleanup
1. Connect and disconnect from dashboard
2. Check database:
   ```sql
   SELECT * FROM session_signaling ORDER BY created_at DESC;
   SELECT * FROM remote_sessions ORDER BY created_at DESC;
   ```
3. Wait 5-10 minutes
4. Check again

**Expected**:
- Old signaling messages deleted (>1 min)
- Expired sessions updated (>15 min)
- Tables stay clean

## 🔍 Step 7: Connection Stability Test

### Test 7.1: Prolonged Session
1. Connect and stream for 10+ minutes
2. Continuously move mouse and type
3. Monitor frame rate and latency

**Expected**:
- No disconnections
- Stable frame rate (~10 FPS)
- No memory leaks

### Test 7.2: Reconnection
1. Connect
2. Close dashboard tab
3. Open new tab and reconnect
4. Should get new session

**Expected**:
- Old session cleans up automatically
- New session creates successfully
- No signaling conflicts

### Test 7.3: Network Change
1. Connect via WiFi
2. Switch to Ethernet (or vice versa)
3. Connection should adapt via ICE restart

**Expected**:
- Brief disconnection (<5 sec)
- Auto-reconnect
- New ICE candidates exchange

## 📊 Performance Metrics

Monitor these during testing:

| Metric | Target | Actual |
|--------|--------|--------|
| Connection Time | <5 sec | ___ |
| Frame Rate | ~10 FPS | ___ |
| Mouse Latency | <200ms (LAN) | ___ |
| Keyboard Latency | <150ms (LAN) | ___ |
| CPU Usage (Agent) | <10% | ___ |
| Memory Usage | <100MB | ___ |
| Bandwidth | ~500 KB/s | ___ |

## ❌ Common Issues

### Mouse not moving
- Check agent logs for errors
- Verify CGO was enabled during build
- Try running agent as Administrator
- Test: `go run` a simple robotgo example

### Keyboard not working
- Check key mapping in `keyboard.go`
- Try simple keys first (a-z, 0-9)
- Then test special keys
- Check for conflicting shortcuts

### Input lag
- Check network latency (ping)
- Verify TURN isn't being used on LAN
- Monitor CPU usage
- Try reducing screen resolution

### Cleanup not running
- Verify pg_cron is enabled:
  ```sql
  SELECT * FROM pg_extension WHERE extname = 'pg_cron';
  ```
- Check cron job schedule:
  ```sql
  SELECT * FROM cron.job WHERE jobname = 'session-cleanup';
  ```
- View error logs:
  ```sql
  SELECT * FROM cron.job_run_details ORDER BY start_time DESC;
  ```

## ✅ Test Completion Checklist

- [ ] Mouse movement works smoothly
- [ ] All mouse buttons work (left, right, middle)
- [ ] Mouse scrolling works
- [ ] Basic keyboard typing works
- [ ] Keyboard shortcuts work
- [ ] Function keys work
- [ ] Manual cleanup function works
- [ ] Cron job is scheduled correctly
- [ ] Automatic cleanup runs successfully
- [ ] Connection stable for 10+ minutes
- [ ] Reconnection works properly
- [ ] No signaling conflicts
- [ ] Performance metrics acceptable

## 🎉 Success Criteria

**Minimum**:
- ✅ Mouse control functional
- ✅ Keyboard control functional
- ✅ Sessions clean up (manually or auto)
- ✅ Stable connection

**Ideal**:
- ✅ All inputs feel natural (<200ms lag)
- ✅ No missed keystrokes or clicks
- ✅ Automatic cleanup runs every 5 min
- ✅ 10+ minute sessions stable
- ✅ Zero manual intervention needed

## 📝 Report Issues

If you find issues, collect:

1. **Agent logs** (terminal output)
2. **Browser console** (F12)
3. **Database state**:
   ```sql
   SELECT * FROM remote_sessions ORDER BY created_at DESC LIMIT 5;
   SELECT * FROM session_signaling ORDER BY created_at DESC LIMIT 10;
   ```
4. **Cron job status**:
   ```sql
   SELECT * FROM cron.job_run_details ORDER BY start_time DESC LIMIT 5;
   ```

---

**Ready to test?** Start with Step 1! 🚀

**Estimated testing time**: 30-45 minutes
