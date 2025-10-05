# 🚀 Remote Desktop Agent - Easy Setup Guide

## For Users Who Need Remote Support

### ⚡ Quick Start (One-Time Setup)

1. **Right-click** `setup-startup.bat`
2. **Select** "Run as Administrator"
3. **Click** "Yes" when Windows asks for permission
4. **Done!** The agent will start automatically with Windows

That's it! Your computer is now ready for remote support.

---

## 🎯 What Each File Does

### `setup-startup.bat` ⭐ (Recommended)
- **Configures agent to start with Windows**
- Only needs to be run ONCE
- Agent runs in the background automatically
- Works even at the login screen

### `run-agent.bat`
- **Runs agent manually when you need it**
- Good for one-time support sessions
- Doesn't set up automatic startup

### `remove-startup.bat`
- **Removes automatic startup**
- Use this if you no longer need remote support
- Agent won't start with Windows anymore

### `remote-agent.exe`
- **The actual agent program**
- Usually you don't run this directly
- Use the `.bat` files instead

---

## 💡 For the Person Providing Support

### Setting Up a User for Remote Support

**Tell them to:**

1. Download the agent files to their computer
2. Right-click `setup-startup.bat`
3. Choose "Run as Administrator"
4. Restart their computer (or just run the agent immediately)

**That's it!** They're ready to connect.

### Getting the Connection PIN

The user should:
1. Open Task Manager (Ctrl+Shift+Esc)
2. Look for "remote-agent.exe" process
3. Or check the agent window if visible

The PIN will be shown in the agent logs/window.

---

## 🔧 Troubleshooting

### "Access Denied" or "Admin Required"
- Right-click the .bat file
- Select "Run as Administrator"
- Click "Yes" on the security prompt

### Agent Not Starting at Boot
- Run `setup-startup.bat` again as Administrator
- Make sure the agent files are in a permanent location (not Downloads folder)

### Black Screen on Dashboard
- Make sure someone is logged into the computer
- The agent captures the current screen (login screen or desktop)
- If the computer is locked, you'll see the lock screen

### Remove Everything
- Right-click `remove-startup.bat`
- Run as Administrator
- Delete the agent folder

---

## 📋 Technical Details (Optional)

The agent uses **Windows Task Scheduler** to:
- Run as SYSTEM user (full permissions)
- Start automatically at boot
- Work with locked/login screens
- Bypass RDP restrictions

This is the same approach used by TeamViewer, AnyDesk, and other remote support tools.

---

## ✅ Security Notes

- Agent only runs when you set it up
- Requires Administrator permission to install
- Uses secure WebRTC connections
- No permanent backdoors
- Easy to remove completely

---

## 🆘 Need Help?

If you have issues:
1. Check if antivirus is blocking the agent
2. Make sure you ran as Administrator
3. Try rebooting the computer
4. Check Windows Event Viewer for errors

---

**That's it! Simple, secure, and user-friendly remote support.**
