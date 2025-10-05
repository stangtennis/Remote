# 🔨 Build Instructions for Remote Desktop Agent

## Prerequisites

### **Windows Requirements:**
- ✅ Go 1.21 or later
- ✅ GCC (MinGW-w64) - Required for CGO (robotgo library)
- ✅ Git

---

## 🛠️ Setting Up Build Environment

### **Step 1: Install Go**
Download from: https://go.dev/dl/

```cmd
go version
```
Should show: `go version go1.21.x windows/amd64`

### **Step 2: Install GCC (MinGW-w64)**

#### **Option A: Chocolatey** (Easiest)
```cmd
choco install mingw
```

#### **Option B: Manual Installation**
1. Download from: https://sourceforge.net/projects/mingw-w64/
2. Choose: x86_64, posix, seh
3. Install to: `C:\mingw64`
4. Add to PATH: `C:\mingw64\bin`

#### **Verify GCC:**
```cmd
gcc --version
```
Should show: `gcc (x86_64-posix-seh-rev0, Built by MinGW-W64 project) ...`

### **Step 3: Clone Repository**
```cmd
git clone https://github.com/stangtennis/Remote.git
cd Remote\agent
```

---

## 🔧 Building

### **Simple Build:**
```cmd
.\build.bat
```

This will:
1. Check for GCC
2. Enable CGO
3. Build `remote-agent.exe`
4. Show success message

### **Manual Build:**
```cmd
set CGO_ENABLED=1
go build -o remote-agent.exe ./cmd/remote-agent
```

### **Clean Build:**
```cmd
go clean
go build -o remote-agent.exe ./cmd/remote-agent
```

---

## 📦 Build Output

**Success:**
```
🔨 Building Remote Desktop Agent with input control...
✅ GCC found
🔧 Building with CGO_ENABLED=1...
✅ Build successful!
📦 Binary: remote-agent.exe
🚀 Run with: .\remote-agent.exe
```

**Binary Location:** `agent\remote-agent.exe`

---

## 🐛 Troubleshooting

### **Error: GCC not found**
```
❌ ERROR: GCC not found!
```
**Fix:** Install MinGW-w64 and add to PATH

### **Error: undefined: robotgo.Move**
```
undefined: robotgo.Move
undefined: robotgo.KeyDown
```
**Fix:** CGO not enabled. Set `CGO_ENABLED=1`

### **Error: Package not found**
```
package github.com/... : no required module provides package
```
**Fix:** Run `go mod download`

### **Error: Too many errors in robotgo**
```
C:\Users\...\robotgo@v0.110.8\...: too many errors
```
**Fix:** GCC version mismatch or CGO issue
- Try: `go clean -cache`
- Reinstall MinGW-w64

---

## 🚀 Building for Production

### **Optimized Build:**
```cmd
set CGO_ENABLED=1
go build -ldflags="-s -w" -o remote-agent.exe ./cmd/remote-agent
```

Flags:
- `-s` : Strip symbol table
- `-w` : Strip DWARF debug info
- Result: Smaller binary size

### **With Version Info:**
```cmd
set VERSION=1.0.0
go build -ldflags="-s -w -X main.version=%VERSION%" -o remote-agent.exe ./cmd/remote-agent
```

---

## 📋 Build Checklist

Before deploying, verify:

- [ ] Binary builds without errors
- [ ] Binary size ~10-15 MB (with dependencies)
- [ ] Test run: `.\remote-agent.exe` shows startup message
- [ ] Service install: `install-service.bat` succeeds
- [ ] Service starts: `sc start RemoteDesktopAgent`
- [ ] Check logs: `agent.log` created
- [ ] Device registers: Check Supabase dashboard

---

## 🔄 Updating Existing Installation

### **Step 1: Stop Service**
```cmd
sc stop RemoteDesktopAgent
```

### **Step 2: Backup Old Version**
```cmd
copy remote-agent.exe remote-agent.exe.backup
```

### **Step 3: Copy New Version**
```cmd
copy /Y new\remote-agent.exe remote-agent.exe
```

### **Step 4: Start Service**
```cmd
sc start RemoteDesktopAgent
```

### **Step 5: Verify**
```cmd
view-logs.bat
```

---

## 🌐 Building on Different Machine

If you can't build on target machine:

### **Build Machine:**
```cmd
cd Remote\agent
.\build.bat
```

### **Transfer Files:**
Copy to USB/network drive:
- `remote-agent.exe`
- `install-service.bat`
- `uninstall-service.bat`
- All `.bat` diagnostic files

### **Target Machine:**
```cmd
cd C:\RemoteAgent
copy Z:\remote-agent.exe .
install-service.bat
```

---

## 🤖 CI/CD (GitHub Actions)

**Future:** Set up automated builds

### **Workflow File:** `.github/workflows/build.yml`
```yaml
name: Build Agent
on: [push, pull_request]
jobs:
  build:
    runs-on: windows-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      - name: Install MinGW
        run: choco install mingw
      - name: Build
        run: |
          cd agent
          .\build.bat
      - name: Upload Artifact
        uses: actions/upload-artifact@v3
        with:
          name: remote-agent.exe
          path: agent/remote-agent.exe
```

This would automatically build on every commit!

---

## 📚 Dependencies

The agent uses these Go packages (auto-downloaded):

### **CGO Dependencies:**
- `github.com/go-vgo/robotgo` - Keyboard/mouse control
- Requires: GCC, libpng, zlib

### **Pure Go Dependencies:**
- `github.com/pion/webrtc/v3` - WebRTC
- `github.com/kbinani/screenshot` - Screen capture
- `github.com/nfnt/resize` - Image resizing
- `golang.org/x/sys/windows` - Windows APIs

---

## ✅ Success Criteria

**You've successfully built when:**
- ✅ `remote-agent.exe` exists
- ✅ File size ~10-15 MB
- ✅ Running it shows startup message
- ✅ No "undefined" or "missing" errors
- ✅ Service installs and starts
- ✅ Logs are created

---

## 🆘 Need Help?

If you're stuck:

1. **Check logs:** `view-logs.bat`
2. **Verify GCC:** `gcc --version`
3. **Clean and retry:** `go clean -cache`
4. **Check Go version:** `go version`
5. **Ask for prebuilt binary** (as last resort)

---

**Good luck! The agent is complex but worth building from source for latest fixes!** 🎯
