# 💻 Remote Desktop Agent - Electron

Full-featured remote desktop agent with **complete remote control capabilities** powered by Electron and Nut.js.

---

## ✨ Features

- ✅ **Full Remote Control** - Mouse and keyboard control via Nut.js
- ✅ **Screen Sharing** - High-quality WebRTC screen streaming
- ✅ **Secure Authentication** - Supabase-based user management
- ✅ **PIN-based Sessions** - Secure connection approval
- ✅ **Cross-Platform** - Windows, macOS, Linux support
- ✅ **Native Performance** - Electron + Node.js integration

---

## 🚀 Quick Start

### **Prerequisites**

- Node.js 18+ (https://nodejs.org/)
- npm or yarn package manager

### **Installation**

```bash
# Navigate to electron-agent directory
cd electron-agent

# Install dependencies
npm install

# Start in development mode
npm start
```

---

## 📦 Building for Production

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

Output will be in the `dist/` directory.

---

## 🎮 How Remote Control Works

### **Architecture:**

```
Dashboard → WebRTC Data Channel → Electron Agent → Nut.js → System Input
```

1. **Dashboard captures** mouse/keyboard events
2. **WebRTC Data Channel** sends events to agent
3. **Electron IPC** forwards to main process
4. **Nut.js** injects system-level input

### **Supported Input:**

- ✅ Mouse movement
- ✅ Mouse clicks (left, right, middle)
- ✅ Mouse double-click
- ✅ Mouse scroll
- ✅ Keyboard keys
- ✅ Keyboard modifiers (Ctrl, Alt, Shift, Meta)
- ✅ Text typing

---

## 🔒 Security

- **User approval required** - PIN verification for each session
- **Supabase RLS** - Row-level security policies
- **Secure WebRTC** - Peer-to-peer encrypted connection
- **Context isolation** - Electron security best practices

---

## 🛠️ Development

### **Project Structure**

```
electron-agent/
├── main.js              # Electron main process
├── preload.js           # IPC bridge (secure)
├── renderer/
│   ├── index.html       # UI
│   ├── agent.js         # WebRTC + Control logic
│   └── styles.css       # Styling
├── assets/              # Icons
└── package.json         # Dependencies
```

### **Key Dependencies**

- `electron` - Desktop app framework
- `@nut-tree/nut-js` - Native input control
- `@supabase/supabase-js` - Backend/auth

---

## 📝 Configuration

Update Supabase credentials in `renderer/agent.js`:

```javascript
const SUPABASE_URL = 'your-project-url';
const SUPABASE_ANON_KEY = 'your-anon-key';
```

---

## 🐛 Troubleshooting

### **Nut.js Installation Issues**

Nut.js requires native dependencies. On some systems:

**Windows:**
- Install Visual Studio Build Tools
- May need Windows SDK

**macOS:**
- Install Xcode Command Line Tools:
  ```bash
  xcode-select --install
  ```

**Linux:**
- Install libxtst-dev:
  ```bash
  sudo apt-get install libxtst-dev
  ```

### **Screen Capture Permission**

**macOS:**
- Grant Screen Recording permission in System Preferences → Security & Privacy

**Linux:**
- May need to configure Wayland/X11 permissions

---

## 📄 License

MIT

---

## 🔗 Related

- **Dashboard:** https://stangtennis.github.io/Remote/dashboard.html
- **Web Agent** (view-only): https://stangtennis.github.io/Remote/agent.html
