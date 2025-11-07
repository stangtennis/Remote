# 📋 Clipboard Synchronization Implementation Plan

**Feature:** Bidirectional clipboard sync between controller and agent  
**Priority:** High (user requested) 🎯  
**Target Version:** v2.2.0  
**Estimated Work:** 6-8 hours

---

## 🎯 **Overview**

Enable seamless copy/paste between the controller and remote agent, supporting:
- Text clipboard content
- Image clipboard content
- File clipboard content (copy/paste files)
- Bidirectional sync (controller ↔ agent)
- Automatic sync on clipboard change
- Manual sync button option

---

## 📋 **Requirements**

### **Functional Requirements:**
1. Copy text on controller → paste on agent
2. Copy text on agent → paste on controller
3. Copy images on controller → paste on agent
4. Copy images on agent → paste on controller
5. Copy files on controller → paste on agent
6. Copy files on agent → paste on controller
7. Automatic clipboard monitoring
8. Manual sync button
9. Clipboard change notifications
10. Error handling for large clipboard data

### **Non-Functional Requirements:**
1. Low latency (< 500ms for text)
2. Efficient for large images (compression)
3. Secure transmission (encrypted via WebRTC)
4. Memory efficient
5. No clipboard pollution (don't sync back to source)

---

## 🏗️ **Architecture**

### **Components:**

```
Controller:
├── internal/clipboard/
│   ├── monitor.go       # Clipboard monitoring
│   ├── manager.go       # Clipboard operations
│   └── types.go         # Clipboard data types

Agent:
├── internal/clipboard/
│   ├── monitor.go       # Clipboard monitoring
│   ├── manager.go       # Clipboard operations
│   └── types.go         # Clipboard data types

Protocol:
├── clipboard_sync message type
├── JSON format: { type, content, format, size }
```

---

## 📦 **Data Structures**

### **Clipboard Message Format:**

```json
{
  "type": "clipboard_sync",
  "content_type": "text|image|files",
  "content": "base64_encoded_data",
  "format": "text/plain|image/png|files",
  "size": 1024,
  "timestamp": 1699334400,
  "source": "controller|agent"
}
```

### **Clipboard Data Types:**

```go
type ClipboardData struct {
    Type      string    // "text", "image", "files"
    Content   []byte    // Raw data or base64 encoded
    Format    string    // MIME type
    Size      int64     // Size in bytes
    Timestamp time.Time // When copied
    Source    string    // "controller" or "agent"
}
```

---

## 🔧 **Implementation Steps**

### **Phase 1: Controller Clipboard Monitor (2 hours)**

**File:** `controller/internal/clipboard/monitor.go`

```go
package clipboard

import (
    "time"
    "golang.design/x/clipboard"
)

type Monitor struct {
    lastContent []byte
    lastHash    string
    onChange    func(data *ClipboardData)
    stopChan    chan bool
}

func NewMonitor() *Monitor
func (m *Monitor) Start()
func (m *Monitor) Stop()
func (m *Monitor) checkClipboard()
func (m *Monitor) SetOnChange(callback func(*ClipboardData))
```

**Features:**
- Poll clipboard every 500ms
- Detect changes using hash comparison
- Extract text, images, or file paths
- Trigger callback on change
- Prevent sync loops (track source)

---

### **Phase 2: Controller Clipboard Manager (1 hour)**

**File:** `controller/internal/clipboard/manager.go`

```go
package clipboard

type Manager struct {
    monitor      *Monitor
    sendData     func([]byte) error
    lastSyncTime time.Time
}

func NewManager() *Manager
func (m *Manager) Start()
func (m *Manager) Stop()
func (m *Manager) SetClipboard(data *ClipboardData) error
func (m *Manager) GetClipboard() (*ClipboardData, error)
func (m *Manager) SyncToRemote(data *ClipboardData) error
func (m *Manager) HandleRemoteClipboard(data []byte) error
```

**Features:**
- Manage clipboard operations
- Send clipboard to agent via WebRTC
- Receive clipboard from agent
- Handle different data types
- Compression for large data

---

### **Phase 3: Agent Clipboard Monitor (2 hours)**

**File:** `agent/internal/clipboard/monitor.go`

Same structure as controller monitor, but for Windows agent.

**Windows API Integration:**
- Use `golang.design/x/clipboard` package
- Monitor clipboard changes
- Extract text, images, files
- Handle Windows-specific formats

---

### **Phase 4: Agent Clipboard Manager (1 hour)**

**File:** `agent/internal/clipboard/manager.go`

Same structure as controller manager.

**Features:**
- Receive clipboard from controller
- Set Windows clipboard
- Send clipboard to controller
- Handle format conversions

---

### **Phase 5: WebRTC Integration (1 hour)**

**Controller:** `controller/internal/viewer/connection.go`
```go
// Initialize clipboard manager
v.clipboardMgr = clipboard.NewManager()
v.clipboardMgr.SetSendDataCallback(func(data []byte) error {
    return v.webrtcClient.SendInput(string(data))
})
v.clipboardMgr.Start()
```

**Agent:** `agent/internal/webrtc/peer.go`
```go
// Handle clipboard messages
case "clipboard_sync":
    if m.clipboardManager != nil {
        m.clipboardManager.HandleRemoteClipboard(msg.Data)
    }
```

---

### **Phase 6: UI Integration (1 hour)**

**Controller Toolbar:**
- Add "📋 Sync Clipboard" button
- Show clipboard sync status
- Manual sync option
- Enable/disable auto-sync

**Status Indicators:**
- "📋 Clipboard synced" notification
- "⚠️ Clipboard too large" warning
- "❌ Clipboard sync failed" error

---

## 📊 **Protocol Specification**

### **Message Types:**

1. **clipboard_sync** - Sync clipboard content
2. **clipboard_request** - Request current clipboard
3. **clipboard_ack** - Acknowledge receipt
4. **clipboard_error** - Error message

### **Message Flow:**

```
Controller                          Agent
    |                                 |
    |-- clipboard_sync (text) ------->|
    |<-------- clipboard_ack ---------|
    |                                 |
    |<-- clipboard_sync (image) ------|
    |-------- clipboard_ack --------->|
```

---

## 🔐 **Security Considerations**

1. **Size Limits:**
   - Text: 10MB max
   - Images: 50MB max
   - Files: Use file transfer instead

2. **Validation:**
   - Validate data types
   - Check MIME types
   - Sanitize file paths

3. **Privacy:**
   - User can disable auto-sync
   - Show notification on sync
   - Option to exclude sensitive apps

---

## 🧪 **Testing Plan**

### **Test Cases:**

1. **Text Clipboard:**
   - Copy text on controller → paste on agent
   - Copy text on agent → paste on controller
   - Copy large text (1MB+)
   - Copy special characters, emojis

2. **Image Clipboard:**
   - Copy screenshot on controller → paste on agent
   - Copy image from Paint on agent → paste on controller
   - Copy large image (10MB+)

3. **File Clipboard:**
   - Copy file path on controller → paste on agent
   - Copy multiple files
   - Copy folder

4. **Edge Cases:**
   - Clipboard empty
   - Clipboard contains unsupported format
   - Network interruption during sync
   - Rapid clipboard changes

---

## 📚 **Dependencies**

### **Go Packages:**

```go
// Clipboard access
"golang.design/x/clipboard"

// Image processing
"image"
"image/png"
"image/jpeg"

// Compression
"compress/gzip"

// Encoding
"encoding/base64"
```

### **Installation:**

```bash
go get golang.design/x/clipboard
```

---

## 🎯 **Success Criteria**

1. ✅ Copy/paste text works bidirectionally
2. ✅ Copy/paste images works bidirectionally
3. ✅ Automatic sync with < 500ms latency
4. ✅ Manual sync button works
5. ✅ No clipboard pollution (sync loops)
6. ✅ Handles large clipboard data gracefully
7. ✅ UI feedback for sync status
8. ✅ Error handling for failures

---

## 🚀 **Implementation Order**

### **Priority 1 (Must Have):**
1. Text clipboard sync (controller → agent)
2. Text clipboard sync (agent → controller)
3. Automatic clipboard monitoring
4. WebRTC integration

### **Priority 2 (Should Have):**
5. Image clipboard sync
6. Manual sync button
7. UI notifications
8. Error handling

### **Priority 3 (Nice to Have):**
9. File clipboard sync
10. Compression for large data
11. Sync history
12. Clipboard preview

---

## 📝 **Implementation Checklist**

- [ ] Create `controller/internal/clipboard/` package
- [ ] Create `agent/internal/clipboard/` package
- [ ] Implement clipboard monitor (controller)
- [ ] Implement clipboard monitor (agent)
- [ ] Implement clipboard manager (controller)
- [ ] Implement clipboard manager (agent)
- [ ] Define clipboard message protocol
- [ ] Integrate with WebRTC data channel
- [ ] Add UI button and status indicators
- [ ] Handle text clipboard
- [ ] Handle image clipboard
- [ ] Handle file clipboard
- [ ] Add compression for large data
- [ ] Add error handling
- [ ] Write unit tests
- [ ] Write integration tests
- [ ] Update documentation

---

## 🐛 **Known Challenges**

### **Challenge 1: Clipboard Monitoring**
- **Issue:** Polling vs event-driven
- **Solution:** Use `golang.design/x/clipboard` with polling (500ms)

### **Challenge 2: Large Images**
- **Issue:** Large images can be slow to transfer
- **Solution:** Compress images before sending, use JPEG for photos

### **Challenge 3: Sync Loops**
- **Issue:** Setting clipboard triggers monitor, causing infinite loop
- **Solution:** Track source and timestamp, ignore self-generated changes

### **Challenge 4: Format Conversion**
- **Issue:** Different clipboard formats on Windows
- **Solution:** Normalize to standard formats (text, PNG, file paths)

---

## 📈 **Performance Targets**

| Metric | Target | Notes |
|--------|--------|-------|
| Text sync latency | < 500ms | For typical text (< 1KB) |
| Image sync latency | < 2s | For typical screenshot (< 1MB) |
| Memory usage | < 50MB | For clipboard manager |
| CPU usage | < 5% | During monitoring |
| Max text size | 10MB | Larger uses file transfer |
| Max image size | 50MB | Larger uses file transfer |

---

## 🎉 **Expected Outcome**

After implementation, users will be able to:
1. ✅ Copy text on controller, paste on agent seamlessly
2. ✅ Copy text on agent, paste on controller seamlessly
3. ✅ Copy images between controller and agent
4. ✅ Use manual sync button for on-demand sync
5. ✅ See notifications when clipboard is synced
6. ✅ Enjoy automatic clipboard sync with low latency

**This will significantly improve the remote desktop experience!** 🚀

---

**Document Version:** 1.0  
**Created:** November 7, 2025  
**Target Version:** v2.2.0  
**Estimated Completion:** December 2025
