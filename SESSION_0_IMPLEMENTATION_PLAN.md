# 🪟 Session 0 Helper Process - Implementation Plan

**Goal:** Enable screen capture of both login screen (Session 0) and user desktop (Session 1+)  
**Estimated Time:** 8-12 hours  
**Priority:** High (for complete remote desktop functionality)

---

## 🎯 **Objective**

Create a helper process that runs in the user session to:
1. Capture user desktop when logged in
2. Show system tray icon for user interaction
3. Communicate with the service in Session 0
4. Forward frames and input events bidirectionally

---

## 🏗️ **Architecture**

### **Two-Process Model**

```
┌─────────────────────────────────────────────────────────┐
│  Session 0 (System Services)                            │
│  ┌───────────────────────────────────────────────────┐  │
│  │  Remote Agent Service (remote-agent-service.exe)  │  │
│  │  - Runs as Windows Service                        │  │
│  │  - Starts on boot                                 │  │
│  │  - Captures Session 0 (login screen)              │  │
│  │  - WebRTC connection to controller                │  │
│  │  - Named pipe server                              │  │
│  └─────────────────┬─────────────────────────────────┘  │
│                    │ Named Pipe: \\.\pipe\RemoteAgentIPC│
└────────────────────┼─────────────────────────────────────┘
                     │
┌────────────────────┼─────────────────────────────────────┐
│  Session 1+ (User Desktop)                              │
│  ┌─────────────────▼─────────────────────────────────┐  │
│  │  Remote Agent Helper (remote-agent-helper.exe)    │  │
│  │  - Auto-starts with user login                    │  │
│  │  - Captures user desktop                          │  │
│  │  - Shows system tray icon                         │  │
│  │  - Named pipe client                              │  │
│  │  - Forwards frames to service                     │  │
│  │  - Receives input from service                    │  │
│  └───────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────┘
```

---

## 📋 **Implementation Steps**

### **Phase 1: Named Pipe Communication (2-3 hours)**

#### **1.1 Create IPC Package**
```
agent/internal/ipc/
├── pipe.go          # Named pipe server/client
├── messages.go      # Message protocol
└── protocol.go      # Message encoding/decoding
```

**Message Types:**
```go
type MessageType byte

const (
    MsgTypeFrame       MessageType = 0x01  // Screen frame data
    MsgTypeInput       MessageType = 0x02  // Input event
    MsgTypeCommand     MessageType = 0x03  // Control command
    MsgTypeHeartbeat   MessageType = 0x04  // Keep-alive
    MsgTypeStatus      MessageType = 0x05  // Status update
)

type Message struct {
    Type    MessageType
    Length  uint32
    Payload []byte
}
```

**Named Pipe Server (Service):**
```go
// agent/internal/ipc/server.go
type PipeServer struct {
    pipeName string
    listener net.Listener
    onFrame  func([]byte)
    onInput  func([]byte)
}

func NewPipeServer(name string) *PipeServer
func (s *PipeServer) Start() error
func (s *PipeServer) SendInput(data []byte) error
func (s *PipeServer) Close() error
```

**Named Pipe Client (Helper):**
```go
// agent/internal/ipc/client.go
type PipeClient struct {
    pipeName string
    conn     net.Conn
    onInput  func([]byte)
}

func NewPipeClient(name string) *PipeClient
func (c *PipeClient) Connect() error
func (c *PipeClient) SendFrame(data []byte) error
func (c *PipeClient) Close() error
```

#### **1.2 Implement Protocol**
- Binary message format: `[Type:1][Length:4][Payload:N]`
- Frame compression (optional)
- Error handling and reconnection
- Heartbeat mechanism (every 5 seconds)

---

### **Phase 2: Helper Application (3-4 hours)**

#### **2.1 Create Helper Entry Point**
```
agent/cmd/remote-agent-helper/
└── main.go          # Helper application entry point
```

**Main Features:**
```go
func main() {
    // 1. Initialize logging
    log.SetOutput(helperLogFile)
    
    // 2. Connect to service via named pipe
    client := ipc.NewPipeClient("\\\\.\\pipe\\RemoteAgentIPC")
    if err := client.Connect(); err != nil {
        log.Fatal("Failed to connect to service:", err)
    }
    
    // 3. Initialize screen capture
    capturer := screen.NewCapturer()
    
    // 4. Initialize system tray
    tray := tray.NewTray()
    tray.SetIcon(iconData)
    tray.SetTooltip("Remote Agent Helper")
    
    // 5. Start capture loop
    go startCaptureLoop(capturer, client)
    
    // 6. Handle input from service
    client.SetOnInput(func(data []byte) {
        handleInput(data)
    })
    
    // 7. Run tray (blocks)
    tray.Run()
}
```

#### **2.2 Screen Capture in Helper**
```go
func startCaptureLoop(capturer *screen.Capturer, client *ipc.PipeClient) {
    ticker := time.NewTicker(33 * time.Millisecond) // 30 FPS
    defer ticker.Stop()
    
    for range ticker.C {
        frame, err := capturer.CaptureScreen()
        if err != nil {
            log.Printf("Capture error: %v", err)
            continue
        }
        
        // Send frame to service via pipe
        if err := client.SendFrame(frame); err != nil {
            log.Printf("Send frame error: %v", err)
        }
    }
}
```

#### **2.3 System Tray in Helper**
```go
// Use existing tray package
tray := tray.NewTray()
tray.AddMenuItem("Status: Connected", nil)
tray.AddSeparator()
tray.AddMenuItem("Show Logs", onShowLogs)
tray.AddMenuItem("Exit Helper", onExit)
```

---

### **Phase 3: Service Modifications (2-3 hours)**

#### **3.1 Session Detection**
```go
// agent/internal/service/session.go
func GetCurrentSession() (uint32, error) {
    // Use Windows API to get current session ID
    var sessionId uint32
    err := windows.ProcessIdToSessionId(
        windows.GetCurrentProcessId(),
        &sessionId,
    )
    return sessionId, err
}

func IsUserLoggedIn() bool {
    // Check if any user session exists
    sessions, _ := wts.EnumerateSessions()
    for _, session := range sessions {
        if session.State == wts.WTSActive && session.SessionId > 0 {
            return true
        }
    }
    return false
}
```

#### **3.2 Service Logic Update**
```go
func (s *Service) Run() {
    sessionId, _ := GetCurrentSession()
    
    if sessionId == 0 {
        // Session 0 - Capture login screen directly
        log.Println("Running in Session 0 - capturing login screen")
        s.captureMode = CaptureModeDirect
        s.startDirectCapture()
    } else {
        // User session - Wait for helper
        log.Println("Running in user session - waiting for helper")
        s.captureMode = CaptureModeHelper
        s.startPipeServer()
        s.waitForHelper()
    }
    
    // Start WebRTC connection
    s.startWebRTC()
}
```

#### **3.3 Pipe Server in Service**
```go
func (s *Service) startPipeServer() {
    s.pipeServer = ipc.NewPipeServer("\\\\.\\pipe\\RemoteAgentIPC")
    
    // Handle frames from helper
    s.pipeServer.SetOnFrame(func(frame []byte) {
        // Send frame via WebRTC
        s.webrtcManager.SendFrame(frame)
    })
    
    // Forward input to helper
    s.webrtcManager.SetOnInput(func(input []byte) {
        s.pipeServer.SendInput(input)
    })
    
    if err := s.pipeServer.Start(); err != nil {
        log.Printf("Failed to start pipe server: %v", err)
    }
}
```

---

### **Phase 4: Auto-Start Helper (1-2 hours)**

#### **4.1 Registry Startup Entry**
```go
// agent/internal/startup/registry.go
func InstallHelperStartup() error {
    key, err := registry.OpenKey(
        registry.CURRENT_USER,
        `Software\Microsoft\Windows\CurrentVersion\Run`,
        registry.SET_VALUE,
    )
    if err != nil {
        return err
    }
    defer key.Close()
    
    exePath, _ := os.Executable()
    helperPath := filepath.Join(filepath.Dir(exePath), "remote-agent-helper.exe")
    
    return key.SetStringValue("RemoteAgentHelper", helperPath)
}

func UninstallHelperStartup() error {
    key, err := registry.OpenKey(
        registry.CURRENT_USER,
        `Software\Microsoft\Windows\CurrentVersion\Run`,
        registry.SET_VALUE,
    )
    if err != nil {
        return err
    }
    defer key.Close()
    
    return key.DeleteValue("RemoteAgentHelper")
}
```

#### **4.2 Service Installer Update**
```batch
REM install-service.bat
@echo off
echo Installing Remote Agent Service with Helper...

REM Copy helper to Program Files
copy remote-agent-helper.exe "%ProgramFiles%\RemoteAgent\" /Y

REM Install service
sc create RemoteAgent binPath= "%ProgramFiles%\RemoteAgent\remote-agent.exe" start= auto

REM Add helper to startup (will run when user logs in)
reg add "HKCU\Software\Microsoft\Windows\CurrentVersion\Run" /v RemoteAgentHelper /t REG_SZ /d "%ProgramFiles%\RemoteAgent\remote-agent-helper.exe" /f

echo Installation complete!
```

---

### **Phase 5: Testing & Debugging (1-2 hours)**

#### **5.1 Test Scenarios**

**Test 1: Login Screen Capture**
```
1. Install service
2. Restart computer
3. At login screen, connect from controller
4. Verify login screen is visible
5. Type password and login
```

**Test 2: User Desktop Capture**
```
1. Login to Windows
2. Verify helper starts automatically
3. Connect from controller
4. Verify user desktop is visible
5. Test mouse/keyboard control
```

**Test 3: Session Switching**
```
1. Connect while logged in
2. Lock computer (Win+L)
3. Verify switches to login screen capture
4. Unlock
5. Verify switches back to desktop capture
```

**Test 4: Helper Crash Recovery**
```
1. Kill helper process
2. Verify service detects disconnection
3. Verify helper auto-restarts on next login
```

#### **5.2 Logging & Debugging**
```
Service Log: C:\ProgramData\RemoteAgent\service.log
Helper Log:  C:\Users\<user>\AppData\Local\RemoteAgent\helper.log
```

---

## 📁 **File Structure**

```
agent/
├── cmd/
│   ├── remote-agent/          # Main agent (can run as service or app)
│   │   └── main.go
│   └── remote-agent-helper/   # NEW: Helper for user session
│       └── main.go
├── internal/
│   ├── ipc/                   # NEW: Inter-process communication
│   │   ├── pipe.go           # Named pipe implementation
│   │   ├── server.go         # Pipe server (service side)
│   │   ├── client.go         # Pipe client (helper side)
│   │   ├── messages.go       # Message types
│   │   └── protocol.go       # Message encoding/decoding
│   ├── service/
│   │   ├── service.go        # Service implementation
│   │   ├── session.go        # NEW: Session detection
│   │   └── capture_mode.go   # NEW: Direct vs Helper capture
│   ├── startup/              # NEW: Auto-start management
│   │   └── registry.go       # Registry startup entries
│   └── ...
├── install-service.bat        # Updated: Install service + helper
├── uninstall-service.bat      # Updated: Remove service + helper
└── build-helper.bat           # NEW: Build helper executable
```

---

## 🔧 **Build Scripts**

### **build-helper.bat**
```batch
@echo off
echo Building Remote Agent Helper...

set CGO_ENABLED=1
go build -ldflags="-s -w -H windowsgui" -o remote-agent-helper.exe .\cmd\remote-agent-helper

echo Build complete: remote-agent-helper.exe
```

### **build-all.bat**
```batch
@echo off
echo Building Remote Agent (Service + Helper)...

REM Build service
call build.bat

REM Build helper
call build-helper.bat

echo All builds complete!
```

---

## 🧪 **Testing Checklist**

- [ ] Named pipe communication works
- [ ] Service detects Session 0 correctly
- [ ] Service detects user session correctly
- [ ] Helper connects to service
- [ ] Helper captures user desktop
- [ ] Helper forwards frames to service
- [ ] Service forwards input to helper
- [ ] Helper shows system tray icon
- [ ] Helper auto-starts on login
- [ ] Login screen capture works
- [ ] Desktop capture works
- [ ] Session switching works
- [ ] Helper crash recovery works
- [ ] Uninstall removes everything

---

## ⚠️ **Known Challenges**

1. **UAC Elevation**
   - Helper needs to run with same privileges as user
   - Service runs as SYSTEM
   - Input injection may require elevation

2. **Session Switching**
   - Detect when user locks/unlocks
   - Switch between Session 0 and user session capture
   - Handle fast user switching

3. **Multiple Users**
   - Multiple user sessions on same machine
   - Each user needs their own helper instance
   - Service needs to manage multiple helpers

4. **Performance**
   - Named pipe overhead
   - Frame data copying
   - May need shared memory for better performance

---

## 🎯 **Success Criteria**

✅ Can capture login screen (Session 0)  
✅ Can capture user desktop (Session 1+)  
✅ Helper auto-starts on user login  
✅ System tray icon visible to user  
✅ Input control works in both modes  
✅ Smooth transition between sessions  
✅ Proper cleanup on uninstall  

---

## 📚 **References**

- [Windows Services and Session 0](https://docs.microsoft.com/en-us/windows/win32/services/services)
- [Named Pipes in Go](https://pkg.go.dev/github.com/Microsoft/go-winio)
- [Windows Session Management](https://docs.microsoft.com/en-us/windows/win32/termserv/terminal-services-portal)
- [Auto-start Applications](https://docs.microsoft.com/en-us/windows/win32/setupapi/run-and-runonce-registry-keys)

---

**Estimated Time:** 8-12 hours  
**Priority:** High  
**Complexity:** Medium-High  
**Dependencies:** None (can start immediately)

**This will complete the remote desktop functionality for all scenarios!** 🚀
