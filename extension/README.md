# 🎮 Remote Desktop Control - Browser Extension

Browser extension + tiny native helper that enables **full remote control** for the web-based remote desktop agent.

---

## 🌟 Features

- ✅ **Full Remote Control** - Mouse and keyboard control
- ✅ **Minimal Footprint** - ~1MB extension + ~5KB native helper
- ✅ **Native Performance** - Direct system-level input injection
- ✅ **Cross-Platform** - Windows, macOS, Linux
- ✅ **Secure** - Chrome Native Messaging API
- ✅ **Works with Web Agent** - No need to change your existing setup

---

## 📋 Prerequisites

- **Go 1.21+** (to build native host)
- **Chrome or Edge browser**
- **Web agent** running at https://stangtennis.github.io/Remote/agent.html

---

## 🚀 Installation

### **Step 1: Build Native Host**

Navigate to the `native-host` directory:

```bash
cd native-host
```

**Windows:**
```bash
# Install Go dependencies
go mod download

# Build
build.bat
```

**macOS/Linux:**
```bash
# Install Go dependencies
go mod download

# Build
chmod +x build.sh
./build.sh
```

### **Step 2: Install Native Host**

**Windows:**
```bash
install-windows.bat
```

**macOS:**
```bash
chmod +x install-macos.sh
./install-macos.sh
```

**Linux:**
```bash
chmod +x install-linux.sh
./install-linux.sh
```

This registers the native messaging host with Chrome/Edge.

### **Step 3: Load Extension in Chrome**

1. Open Chrome/Edge
2. Go to `chrome://extensions/` (or `edge://extensions/`)
3. Enable **Developer mode** (top right)
4. Click **Load unpacked**
5. Select the `extension` folder
6. Note the **Extension ID** (you'll need it)

### **Step 4: Update Native Host Manifest**

1. Open `native-host/com.remote.desktop.control.json`
2. Replace `EXTENSION_ID_HERE` with your actual extension ID
3. Re-run the install script:
   - Windows: `install-windows.bat`
   - macOS: `./install-macos.sh`
   - Linux: `./install-linux.sh`

---

## ✅ Testing

1. Open the **web agent**: https://stangtennis.github.io/Remote/agent.html
2. Check browser console - you should see:
   ```
   🔌 Remote Desktop Control Extension - Content Script Loaded
   ✅ Extension content script initialized
   ```
3. The extension icon should appear in your browser toolbar
4. Start screen sharing and connect from dashboard
5. **Remote control should now work!** 🎉

---

## 🔧 How It Works

```
Dashboard → WebRTC Data Channel → Web Agent (Browser)
                                        ↓
                                   Window Message
                                        ↓
                                   Extension Content Script
                                        ↓
                                   Chrome Runtime Message
                                        ↓
                                   Extension Background Script
                                        ↓
                                   Native Messaging API
                                        ↓
                                   Native Host (Go)
                                        ↓
                                   Robotgo Library
                                        ↓
                                   System Input (Mouse/Keyboard)
```

---

## 📁 File Structure

```
extension/
├── manifest.json       # Extension manifest
├── background.js       # Service worker (native messaging)
├── content.js          # Content script (page communication)
├── icons/              # Extension icons
└── README.md           # This file

native-host/
├── main.go             # Native host implementation
├── go.mod              # Go dependencies
├── build.bat           # Windows build script
├── build.sh            # macOS/Linux build script
├── install-windows.bat # Windows installer
├── install-macos.sh    # macOS installer
├── install-linux.sh    # Linux installer
└── com.remote.desktop.control.json  # Native host manifest
```

---

## 🐛 Troubleshooting

### **"Native host not connected" error**

1. Check if native host is built:
   ```bash
   cd native-host
   ls remote-desktop-control*
   ```

2. Check if native host is registered:
   - **Windows:** Open Registry Editor → `HKCU\Software\Google\Chrome\NativeMessagingHosts\com.remote.desktop.control`
   - **macOS/Linux:** Check `~/.config/google-chrome/NativeMessagingHosts/`

3. Verify extension ID in manifest matches actual ID

4. Check browser console for errors

### **Native host builds but doesn't work**

**Windows:**
- May need Visual C++ redistributables
- Try running as administrator

**macOS:**
- Grant Accessibility permissions:
  - System Preferences → Security & Privacy → Accessibility
  - Add Terminal or Chrome

**Linux:**
- Install required libraries:
  ```bash
  sudo apt-get install libxtst-dev libpng++-dev
  ```

### **Extension not loading**

1. Make sure manifest.json is valid
2. Check for JSON syntax errors
3. Reload extension after changes

---

## 🔒 Security

- **Native Messaging** - Secure Chrome API for extension ↔ native communication
- **Isolated Communication** - Extension only communicates with allowed origins
- **No Network Access** - Native host operates locally only
- **User Approval** - Sessions require PIN verification

---

## 📦 Building Releases

To create distributable packages:

**Windows:**
```bash
cd native-host
build.bat
# Distribute: remote-desktop-control.exe + install-windows.bat + manifest
```

**macOS:**
```bash
cd native-host
./build.sh
# Distribute: remote-desktop-control + install-macos.sh + manifest
```

**Linux:**
```bash
cd native-host
./build.sh
# Distribute: remote-desktop-control + install-linux.sh + manifest
```

---

## 🔄 Updates

To update the extension:
1. Make changes to extension files
2. Go to `chrome://extensions/`
3. Click **Reload** button on the extension

To update the native host:
1. Rebuild: `build.bat` or `./build.sh`
2. No need to re-register

---

## 📝 Development

**Testing native host standalone:**
```bash
cd native-host
echo '{"type":"ping"}' | ./remote-desktop-control
```

**Console logging:**
- Extension: Browser DevTools Console
- Native host: Outputs to stderr (visible in terminal if run manually)

---

## ✨ Next Steps

1. ✅ **Test locally** - Verify remote control works
2. ✅ **Package for distribution** - Create installation packages
3. ✅ **Publish extension** - Submit to Chrome Web Store (optional)
4. ✅ **Documentation** - Share setup guide with users

---

**Enjoy full remote control with minimal installation!** 🎉🚀
