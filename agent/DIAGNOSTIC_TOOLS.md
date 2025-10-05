# 🔧 Diagnostic Tools for Remote Agent

## 📋 Quick Reference

### **check-duplicates.bat**
Check if multiple agent instances are running
- Shows all running processes
- Checks for startup task
- Checks Windows Service
- **Run this first!**

### **view-logs.bat**
View agent logs in real-time
- Shows last 50 lines
- Updates automatically as new logs arrive
- Press Ctrl+C to stop

### **watch-service.bat**
Monitor service status changes
- Updates every 2 seconds
- Shows if service starts/stops
- Useful to see when service crashes

### **full-diagnostic.bat**
Complete system diagnostic
- All checks in one place
- Shows processes, service, logs, device ID
- Saves output for troubleshooting

### **fix-duplicates.bat** ⭐
**Fix the duplicate agent problem**
- Stops all agent processes
- Removes startup task
- Keeps only Windows Service
- **Run this if you see 2 devices in dashboard!**

---

## 🚨 Common Problems

### **Problem: See 2 identical devices in dashboard**
**Cause:** Both service AND startup task are running  
**Fix:** Run `fix-duplicates.bat`

### **Problem: Service keeps restarting**
**Cause:** Desktop monitoring or screen capture crashing  
**Check:** Run `view-logs.bat` to see errors  
**Check:** Run `watch-service.bat` to see restart pattern

### **Problem: Can't connect before login**
**Cause:** Session 0 isolation (services can't access desktop easily)  
**Status:** Known limitation, requires more work

---

## 📝 How to Use

1. **Copy all .bat files** to Dennis's machine (same folder as remote-agent.exe)

2. **Run full-diagnostic.bat** to see current state

3. **If multiple processes found**, run `fix-duplicates.bat`

4. **To monitor in real-time**, run `watch-service.bat` or `view-logs.bat`

5. **Send diagnostic output** to help troubleshoot

---

## 📊 Expected Results

**✅ Healthy System:**
```
Processes running: 1
Service: RUNNING
No startup task
```

**⚠️ Problem System:**
```
Processes running: 2 or more
Service: RUNNING
Startup task: EXISTS
```

---

## 💡 Tips

- Always run as **Administrator**
- Check logs after every connection attempt
- If service keeps stopping, check Event Viewer → System logs
- Keep only ONE method: Service OR Startup Task (not both)

---

## 🎯 Recommended Setup

**For lock screen support:**
- ✅ Use Windows Service
- ❌ Don't use Startup Task

**For normal use only:**
- ❌ Don't use Windows Service
- ✅ Use Startup Task

**NEVER use both at the same time!**
