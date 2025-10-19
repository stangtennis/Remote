# 🚀 Quick Start Guide - Electron Agent

## ⚡ Get Started in 3 Steps

### **1. Install Dependencies**

```bash
cd electron-agent
npm install
```

This will install:
- Electron (desktop framework)
- @nut-tree/nut-js (native input control)
- @supabase/supabase-js (backend)

### **2. Run the App**

```bash
npm start
```

The Electron agent will launch!

### **3. Login & Start Sharing**

1. **Login** with your Supabase credentials
2. **Start Screen Sharing** 
3. Open **Dashboard** on another device
4. **Connect** and enter PIN
5. **✅ Full remote control is now active!**

---

## 🎮 What Works

### **✅ Viewing**
- Screen streaming via WebRTC
- Real-time video feed
- Low latency

### **✅ Remote Control** 
- 🖱️ **Mouse Control**
  - Move cursor
  - Click (left, right, middle)
  - Double-click
  - Scroll
  
- ⌨️ **Keyboard Control**
  - All keys
  - Modifiers (Ctrl, Alt, Shift, Cmd)
  - Text typing
  - Function keys

---

## 🔧 Development Mode

Enable DevTools for debugging:

```bash
# Set environment variable
set NODE_ENV=development

# Run
npm start
```

DevTools will open automatically.

---

## 📦 Building Executables

### **Build for Current Platform**

```bash
npm run build
```

### **Build for Specific Platform**

```bash
# Windows
npm run build:win

# macOS
npm run build:mac

# Linux
npm run build:linux
```

**Output:** `dist/` directory

---

## 🐛 Troubleshooting

### **"Nut.js install failed"**

**Windows:**
```bash
npm install --global --production windows-build-tools
```

**macOS:**
```bash
xcode-select --install
```

**Linux (Ubuntu/Debian):**
```bash
sudo apt-get install libxtst-dev libpng++-dev
```

### **"Screen capture permission denied"**

**macOS:**
- System Preferences → Security & Privacy → Screen Recording
- Enable for Electron

**Linux:**
- May need X11/Wayland permissions

### **"Cannot connect to dashboard"**

1. Check internet connection
2. Verify Supabase credentials in `renderer/agent.js`
3. Check firewall settings

---

## 📋 Next Steps

1. **Test locally** - Run agent and connect from dashboard
2. **Test remote** - Try over internet
3. **Build executable** - Create distributable
4. **Deploy** - Share with users!

---

## 🆚 Web Agent vs Electron Agent

| Feature | Web Agent | Electron Agent |
|---------|-----------|----------------|
| **Installation** | None (browser) | Small app download |
| **Remote Control** | ❌ No | ✅ Yes |
| **Viewing** | ✅ Yes | ✅ Yes |
| **Works on locked PCs** | ✅ Yes | ⚠️ Depends |
| **Platform** | Any browser | Win/Mac/Linux |

**Use Web Agent for:** Quick viewing, no installation
**Use Electron Agent for:** Full control, better performance

---

## 💡 Tips

- **Performance:** Lower video quality in Settings for slower connections
- **Security:** Always verify PIN before accepting connections
- **Updates:** Pull latest from GitHub regularly

---

**Ready to go! Run `npm start` to launch the agent.** 🚀
