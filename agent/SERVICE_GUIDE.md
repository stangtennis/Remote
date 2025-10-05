# Windows Service Installation Guide

## 🎯 What This Enables

Installing the agent as a Windows Service provides:

✅ **Lock Screen Access** - Capture and control the Windows login screen  
✅ **Auto-Start on Boot** - Runs automatically when computer starts (before login)  
✅ **Background Operation** - Runs even when no one is logged in  
✅ **Full Desktop Switching** - Works across user desktop ↔ login screen transitions  
✅ **Network Wait** - Waits for network to be ready before connecting  
✅ **Auto-Recovery** - Restarts automatically if it crashes  
✅ **Remote Reboot Access** - Connect immediately after restart without physical access  

---

## 📋 Requirements

- Windows 10/11 or Windows Server
- Administrator privileges
- Agent executable (`remote-agent.exe`)

---

## 🚀 Installation Steps

### **Option 1: Windows Service (Recommended for servers/unattended access)**

1. **Open PowerShell/CMD as Administrator**
   - Right-click Start → "Terminal (Admin)" or "Command Prompt (Admin)"

2. **Navigate to agent folder**
   ```cmd
   cd path\to\agent
   ```

3. **Run installation script**
   ```cmd
   install-service.bat
   ```

4. **Service is now running!**
   - Starts automatically on boot
   - Can capture login screen when locked
   - Runs with SYSTEM privileges

---

### **Option 2: Startup Task (Standard user access)**

Use this for machines where you want it to run only when a user logs in:

```cmd
setup-startup.bat
```

- Runs when user logs in
- User-level permissions
- Cannot capture login screen

---

## 🔧 Service Management

### Check Service Status
```cmd
sc query RemoteDesktopAgent
```

### Start Service
```cmd
sc start RemoteDesktopAgent
```

### Stop Service
```cmd
sc stop RemoteDesktopAgent
```

### Uninstall Service
```cmd
uninstall-service.bat
```

Or manually:
```cmd
sc stop RemoteDesktopAgent
sc delete RemoteDesktopAgent
```

---

## 📊 How It Works

### Boot/Restart Sequence
When computer starts up:

1. **Windows boots** - Hardware initialization
2. **Network starts** - Service waits for LanmanWorkstation (network)
3. **Agent starts** - Runs as LocalSystem with delayed start
4. **Retry connection** - Attempts to connect up to 5 times with exponential backoff (2s, 4s, 6s, 8s, 10s)
5. **Registration** - Connects to Supabase and registers device
6. **Ready to connect** - Device shows as online in dashboard, ready before user login

**This means you can:**
- Restart a remote machine
- Connect immediately after it boots
- Access the login screen to log in
- No physical access needed!

### Desktop Switching
The agent automatically detects and switches between:

- **User Desktop** (`Default`) - Normal desktop when logged in
- **Login Screen** (`Winlogon`) - Windows login/lock screen
- **Screen Saver** - Screen saver desktop

### When Locked/Logged Out
1. Machine is locked or no one logged in
2. Agent detects switch to `Winlogon` desktop
3. Captures login screen instead of user desktop
4. You can send keyboard/mouse input to login
5. After login, automatically switches to user desktop

### Auto-Recovery
If the agent crashes or fails:
- **First failure** - Restarts after 5 seconds
- **Second failure** - Restarts after 10 seconds
- **Third failure** - Restarts after 30 seconds
- Resets counter after 24 hours

---

## 🔐 Security Considerations

### Service Mode (SYSTEM Account)
- ✅ Runs with highest privileges
- ✅ Can access login screen
- ⚠️ Ensure `.env` file has restricted permissions
- ⚠️ Only install on trusted machines

### Startup Task Mode (User Account)
- ✅ Limited to user permissions
- ✅ More secure for shared machines
- ❌ Cannot capture login screen
- ❌ Only works when user logged in

---

## 🐛 Troubleshooting

### Service won't start
- Check Event Viewer → Windows Logs → Application
- Look for `RemoteDesktopAgent` errors
- Verify `.env` file exists with correct credentials

### Can't see login screen
- Ensure service is running (not startup task)
- Check service is configured with `interact` flag
- Verify SYSTEM account permissions

### Screen capture fails
- If on RDP: Keep RDP session connected (minimized)
- If console: Ensure monitor connected
- Check agent logs for specific errors

---

## 📝 Logs

### Interactive Mode
Logs appear in console window

### Service Mode
Check Windows Event Viewer:
- Windows Logs → Application
- Source: `RemoteDesktopAgent`

---

## 🔄 Switching Between Modes

### From Startup Task → Service
```cmd
schtasks /delete /tn "RemoteDesktopAgent" /f
install-service.bat
```

### From Service → Startup Task
```cmd
uninstall-service.bat
setup-startup.bat
```

---

## ✅ Verification

After installation, verify it works:

1. **Service is running**
   ```cmd
   sc query RemoteDesktopAgent
   ```
   Should show `STATE: RUNNING`

2. **Agent is registered**
   - Check dashboard - device should appear online

3. **Test lock screen**
   - Lock Windows (Win+L)
   - Connect from dashboard
   - You should see login screen!

---

## 🎯 Best Practices

### For Servers
- ✅ Use Windows Service mode
- ✅ Configure `.env` with strong credentials
- ✅ Restrict `.env` file permissions
- ✅ Enable Windows Firewall

### For Desktops/Laptops
- ✅ Use Startup Task mode (unless need lock screen)
- ✅ Only install on personal machines
- ✅ Use strong Supabase credentials

### For RDP Access
- ⚠️ Keep RDP session connected (can minimize)
- ⚠️ Or use service mode for lock screen support
- ⚠️ Disconnecting RDP may limit functionality

---

## 📞 Support

If you encounter issues:

1. Check Event Viewer logs
2. Verify `.env` configuration
3. Test in interactive mode first (`.\remote-agent.exe`)
4. Check Supabase dashboard for device status
