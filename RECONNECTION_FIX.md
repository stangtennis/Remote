# Reconnection Issue Fix

## Problem

After refreshing the dashboard, you can't connect again because:
1. Old session stays in "active" state
2. Agent is still listening for the old session
3. New connection attempt conflicts with old session

## ✅ Fix Applied

Added `beforeunload` event handler to:
- End session in database when page refreshes
- Close peer connection properly
- Clean up data channel

## 🧪 Test the Fix

### After deploying (in 2-3 minutes):

1. **Hard refresh dashboard** (Ctrl+F5)
2. **Connect to agent**
3. **Test**: Refresh page (F5)
4. **Connect again** - should work now!

## 🔧 Temporary Workaround (Until Fix Deploys)

If you need to reconnect NOW before the fix deploys:

### Option 1: Clean Sessions in Supabase

Run in SQL Editor:
```sql
DELETE FROM session_signaling;
DELETE FROM remote_sessions WHERE status IN ('pending', 'active');
```

Then refresh dashboard and try again.

### Option 2: Restart Agent

1. Stop agent (Ctrl+C)
2. Start agent again
3. Refresh dashboard
4. Connect

### Option 3: Use "End Session" Button

1. Before refreshing, click "End Session" button
2. Wait 2 seconds
3. Then refresh
4. Connect again

## 🎯 How the Fix Works

**Before** (broken):
```
1. User connects → Session created (status: active)
2. User refreshes page → Session still active ❌
3. User tries to connect → Conflict! Old session blocks new one
```

**After** (fixed):
```
1. User connects → Session created (status: active)
2. User refreshes page → beforeunload fires → Session set to 'ended' ✅
3. User tries to connect → New session created, no conflict ✅
```

## 📋 What Was Added

```javascript
// In docs/js/app.js
window.addEventListener('beforeunload', (e) => {
  if (currentSession) {
    // Update session status to 'ended'
    supabase
      .from('remote_sessions')
      .update({ status: 'ended', ended_at: new Date().toISOString() })
      .eq('id', currentSession.session_id);
    
    // Close connections
    if (window.peerConnection) window.peerConnection.close();
    if (window.dataChannel) window.dataChannel.close();
  }
});
```

## 🔍 Why It Sometimes Works

File Explorer works because:
- It's a simple case with no conflicts
- Or you happened to wait long enough (15 min) for auto-cleanup
- Or the old session timed out naturally

## ⏱️ Auto-Cleanup Also Helps

The pg_cron job (runs every 5 min) will clean up:
- Sessions older than 15 minutes
- Stale signaling messages

So even without this fix, waiting 5-15 minutes would eventually let you reconnect.

## 🚀 Deploy the Fix

```powershell
cd f:\#Remote
git push origin main
```

Wait 1-2 minutes for GitHub Pages to update.

## ✅ Verify Fix is Deployed

1. Hard refresh dashboard (Ctrl+F5)
2. Open DevTools Console (F12)
3. Look for log: `"Page unloading - cleaning up session: ..."`
4. If you see it → fix is active ✅

## 📊 Test Results

After deploying, test this flow:

1. **Connect** → ✅ Should work
2. **Refresh** (F5) → ✅ Session should end
3. **Connect again** → ✅ Should work
4. **Repeat** → ✅ Should always work

**Before fix**: Only step 1 worked  
**After fix**: All steps should work

## 🐛 If Still Not Working

### Check session status:
```sql
SELECT id, status, created_at, ended_at 
FROM remote_sessions 
ORDER BY created_at DESC 
LIMIT 5;
```

### Check for stuck active sessions:
```sql
SELECT COUNT(*) FROM remote_sessions WHERE status = 'active';
```

If > 0 after refresh → fix not working, clear manually

### Check browser console:
- Should see: `"Page unloading - cleaning up session: ..."`
- Should see: `"Session ended successfully"`
- If not → hard refresh (Ctrl+Shift+R)

## 💡 Additional Improvements

Future enhancements could include:
- [ ] Agent auto-reset after disconnect
- [ ] Dashboard shows "Reconnecting..." instead of failing
- [ ] Session heartbeat (mark as active every 30s)
- [ ] Dashboard warns "Another tab is connected"

## 🎯 Success Criteria

- ✅ Can refresh dashboard and reconnect immediately
- ✅ No need to manually clean database
- ✅ No need to restart agent
- ✅ Sessions clean up automatically on page close

---

**Status**: ✅ Fix committed  
**Commit**: `257159f` - "Add session cleanup on page unload/refresh"  
**Deploy**: Push to GitHub Pages (in progress)  
**Test**: Hard refresh dashboard after 2 minutes  

---

**Last Updated**: 2025-10-03 00:22
