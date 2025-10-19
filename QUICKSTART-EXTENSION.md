# 🚀 Quick Start - Browser Extension + Native Helper

Get remote control working in 5 minutes!

---

## 📋 What You Need

- Go 1.21+ installed
- Chrome or Edge browser
- 5 minutes of your time

---

## ⚡ Installation (Windows)

### **1. Build Native Host (2 minutes)**

```bash
cd native-host
go mod download
build.bat
```

### **2. Install Native Host (1 minute)**

```bash
install-windows.bat
```

### **3. Load Extension (1 minute)**

1. Open Chrome: `chrome://extensions/`
2. Enable **Developer mode** (top right switch)
3. Click **Load unpacked**
4. Select the `extension` folder
5. **Copy the Extension ID** (looks like: `abcdefghijklmnopqrstuvwxyz123456`)

### **4. Update Manifest (30 seconds)**

1. Open `native-host\com.remote.desktop.control.json`
2. Replace `EXTENSION_ID_HERE` with your copied ID
3. Run `install-windows.bat` again

### **5. Test It! (30 seconds)**

1. Open web agent: https://stangtennis.github.io/Remote/agent.html
2. Check console (F12) - should see: `✅ Extension content script initialized`
3. Start sharing → Connect from dashboard → **Remote control works!** 🎉

---

## ⚡ Installation (macOS)

### **1. Build Native Host**

```bash
cd native-host
go mod download
chmod +x build.sh
./build.sh
```

### **2. Install Native Host**

```bash
chmod +x install-macos.sh
./install-macos.sh
```

### **3. Load Extension**

Same as Windows (steps 3-5 above)

**Note:** macOS may require Accessibility permissions:
- System Preferences → Security & Privacy → Accessibility
- Add Chrome/Edge

---

## ⚡ Installation (Linux)

### **1. Install Dependencies**

```bash
sudo apt-get install libxtst-dev libpng++-dev
```

### **2. Build Native Host**

```bash
cd native-host
go mod download
chmod +x build.sh
./build.sh
```

### **3. Install Native Host**

```bash
chmod +x install-linux.sh
./install-linux.sh
```

### **4. Load Extension**

Same as Windows (steps 3-5 above)

---

## ✅ Verification

Open the web agent page and check the browser console (F12):

**✅ Working:**
```
🔌 Remote Desktop Control Extension - Content Script Loaded
✅ Extension content script initialized
🚀 Remote Desktop Control Extension - Background Script Loaded
🔗 Attempting to connect to native host...
✅ Native host connected successfully
```

**❌ Not Working:**
```
⚠️ Native host disconnected: Specified native messaging host not found
```
→ Re-run install script with correct Extension ID

---

## 🎮 Using Remote Control

1. **Open agent** page
2. **Login** with your account
3. **Start screen sharing**
4. **Connect** from dashboard
5. **Enter PIN**
6. **✅ Control the remote computer!**

### **What Works:**

- ✅ Mouse movement
- ✅ Mouse clicks (left, right, middle)
- ✅ Mouse scroll
- ✅ Keyboard typing
- ✅ Keyboard shortcuts (Ctrl+C, Alt+Tab, etc.)
- ✅ Function keys
- ✅ All special keys

---

## 🐛 Common Issues

### **"Native host not connected"**

**Fix:**
1. Verify extension ID matches in manifest
2. Re-run install script
3. Restart browser

### **"Extension not loading"**

**Fix:**
1. Check `manifest.json` for syntax errors
2. Make sure you selected the `extension` folder
3. Try disabling/re-enabling the extension

### **"Input not working"**

**Windows Fix:**
- Run Chrome as administrator (first time)

**macOS Fix:**
- Grant Accessibility permissions

**Linux Fix:**
- Install required libraries:
  ```bash
  sudo apt-get install libxtst-dev libpng++-dev
  ```

---

## 📦 File Sizes

- **Extension:** ~50 KB
- **Native Host:** ~5 MB (includes robotgo library)
- **Total:** ~5 MB one-time download

---

## 🔒 Security

- ✅ **Local only** - No external servers
- ✅ **PIN verification** - User must approve each session
- ✅ **Chrome Native Messaging** - Secure communication
- ✅ **Open source** - Fully auditable code

---

## 🔄 Updates

**Extension updates:**
- Just reload extension in `chrome://extensions/`

**Native host updates:**
- Rebuild and no need to re-register

---

## 💡 Tips

- **First time?** Test on a virtual machine or secondary computer
- **Production?** Build release versions and distribute
- **Issues?** Check browser console (F12) for error messages
- **Permissions?** macOS/Linux may require special permissions

---

## 📚 More Info

- **Full README:** See `extension/README.md`
- **Troubleshooting:** See extension README
- **Support:** Check GitHub issues

---

**You're all set! Enjoy full remote control.** 🎉🎮🚀
