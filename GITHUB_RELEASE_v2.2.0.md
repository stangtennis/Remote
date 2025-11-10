# 🎉 v2.2.0 - Clipboard Sync Release

**Release Date:** November 10, 2025  
**Status:** Production Ready

---

## ✨ What's New

### 📋 **Clipboard Synchronization** (New!)
- ✅ Copy text on remote → paste on local (automatic)
- ✅ Copy images on remote → paste on local (automatic)
- ✅ Up to 10MB text, 50MB images
- ✅ 500ms polling with hash-based change detection
- ✅ **Just like RDP - completely automatic!**

---

## 🚀 All Features

### **Core Remote Desktop:**
- ✅ 60 FPS video streaming (JPEG quality 95)
- ✅ Full mouse control (move, click, scroll)
- ✅ Full keyboard control (all keys + shortcuts)
- ✅ Up to 4K resolution support
- ✅ < 200ms latency

### **File Management:**
- ✅ File transfer (controller → agent)
- ✅ Progress tracking
- ✅ Chunked transfer for large files

### **Reliability:**
- ✅ Auto-reconnection on disconnect
- ✅ Exponential backoff
- ✅ Connection status indicators

### **Clipboard Sync (v2.2.0):**
- ✅ Text clipboard sync
- ✅ Image clipboard sync
- ✅ Automatic monitoring
- ✅ One-way sync (agent → controller)

---

## 📦 Downloads

### **Controller** (Local Machine)
- `controller.exe` - 50 MB
- Windows 10/11 (64-bit)
- Run on your local computer

### **Agent** (Remote Machine)
- `remote-agent.exe` - 22 MB
- Windows 10/11 (64-bit)
- Run on the computer you want to control

---

## 🚀 Quick Start

### **1. Setup**
```powershell
# On remote machine (agent)
.\remote-agent.exe

# On local machine (controller)
.\controller.exe
```

### **2. Connect**
1. Login to controller
2. Approve device (if first time)
3. Click "Connect"

### **3. Use**
- **View** remote screen in real-time
- **Control** with mouse and keyboard
- **Copy** text/images on remote → **paste** on local
- **Send files** with "Send File" button
- **Fullscreen** with F11 (ESC to exit)

---

## 🎯 What You Can Do

✅ **View** remote desktop (60 FPS)  
✅ **Control** mouse and keyboard  
✅ **Copy/paste** between machines  
✅ **Send files** to remote  
✅ **Auto-reconnect** on disconnect  
✅ **Fullscreen mode**  

**Everything works like TeamViewer/RDP!** 🚀

---

## 📋 Clipboard Examples

### **Copy Code from Remote:**
1. Remote: Select and copy code (Ctrl+C)
2. Local: Paste into your editor (Ctrl+V)
3. ✨ Code appears instantly!

### **Copy Screenshot from Remote:**
1. Remote: Take screenshot (Win+Shift+S)
2. Local: Paste into Paint/Discord/Slack (Ctrl+V)
3. ✨ Screenshot appears instantly!

---

## 🔧 System Requirements

### **Controller:**
- Windows 10/11 (64-bit)
- 4GB RAM minimum
- Network connection

### **Agent:**
- Windows 10/11 (64-bit)
- 4GB RAM minimum
- Network connection
- User session (not Session 0)

---

## 📝 Known Issues

1. **Clipboard is one-way only** (agent → controller)
   - Workaround: Use file transfer for controller → agent

2. **Large clipboard data skipped** (>10MB text, >50MB images)
   - Workaround: Use file transfer for large files

---

## 🔄 Upgrade from v2.0.0

Simply download and replace the executables. No configuration changes needed!

All features from v2.0.0, v2.1.0, and v2.2.0 are included.

---

## 📚 Documentation

- [SUMMARY.md](https://github.com/stangtennis/Remote/blob/main/SUMMARY.md) - Project overview
- [RELEASE_NOTES_v2.2.0.md](https://github.com/stangtennis/Remote/blob/main/RELEASE_NOTES_v2.2.0.md) - Detailed release notes
- [TESTING_CLIPBOARD.md](https://github.com/stangtennis/Remote/blob/main/TESTING_CLIPBOARD.md) - Testing guide

---

## 🐛 Reporting Issues

Found a bug? [Create an issue](https://github.com/stangtennis/Remote/issues/new) with:
- Steps to reproduce
- Expected vs actual behavior
- Log output (if available)
- System information

---

## 🎉 Thank You!

Thank you for using Remote Desktop v2.2.0!

**Enjoy seamless clipboard sync!** 🚀

---

## 📊 Version History

- **v2.2.0** (Nov 2025) - Clipboard sync
- **v2.1.0** (Nov 2025) - File transfer + auto-reconnect
- **v2.0.0** (Nov 2025) - Core remote desktop

---

**Built with:** Go, Fyne, Pion WebRTC, Supabase, robotgo, golang.design/x/clipboard
