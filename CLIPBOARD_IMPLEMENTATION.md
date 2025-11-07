# 📋 Clipboard Synchronization Implementation Plan

**Feature:** One-way clipboard sync (agent → controller, like RDP)  
**Priority:** High (user requested) 🎯  
**Target Version:** v2.2.0  
**Estimated Work:** 3-4 hours (simplified)

---

## 🎯 **Overview**

Enable **one-way clipboard sync from remote agent to controller** (like RDP):
- **Primary Use Case:** Copy text/images on remote → paste on local computer
- Text clipboard content
- Image clipboard content (screenshots, etc.)
- Automatic sync on clipboard change
- Simple and reliable (like RDP)

**Future Enhancement:** Bidirectional sync (controller → agent) can be added later if needed.

---

## 📋 **Requirements**

### **Functional Requirements (Priority Order):**

**Phase 1 - Core (Like RDP):**
1. ✅ Copy text on **agent** → paste on **controller** (PRIMARY USE CASE)
2. ✅ Copy images on **agent** → paste on **controller** (screenshots, etc.)
3. ✅ Automatic clipboard monitoring on agent
4. ✅ Automatic sync to controller
5. ✅ Error handling for large clipboard data

**Phase 2 - Optional Enhancements:**
6. ⏳ Copy text on controller → paste on agent (reverse direction)
7. ⏳ Manual sync button
8. ⏳ Clipboard change notifications
9. ⏳ File clipboard support

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

### **Simplified Implementation (Agent → Controller Only)**

This matches RDP behavior: copy on remote, paste on local.

### **Phase 1: Agent Clipboard Monitor (2 hours)**

**File:** `agent/internal/clipboard/monitor.go`

```go
package clipboard

import (
    "time"
    "golang.design/x/clipboard"
)

type Monitor struct {
    lastContent []byte
    lastHash    string
    onTextChange  func(text string)
    onImageChange func(imageData []byte)
    stopChan    chan bool
}

func NewMonitor() *Monitor
func (m *Monitor) Start()
func (m *Monitor) Stop()
func (m *Monitor) checkClipboard()
func (m *Monitor) SetOnTextChange(callback func(string))
func (m *Monitor) SetOnImageChange(callback func([]byte))
```

**Features:**
- Poll clipboard every 500ms on **agent**
- Detect changes using hash comparison
- Extract text or images
- Send to controller via WebRTC
- Simple and reliable (like RDP)

---

### **Phase 2: Controller Clipboard Receiver (1 hour)**

**File:** `controller/internal/clipboard/receiver.go`

```go
package clipboard

import "golang.design/x/clipboard"

type Receiver struct {
    // Simple receiver - just sets local clipboard
}

func NewReceiver() *Receiver
func (r *Receiver) SetText(text string) error
func (r *Receiver) SetImage(imageData []byte) error
func (r *Receiver) HandleRemoteClipboard(data []byte) error
```

**Features:**
- Receive clipboard from agent via WebRTC
- Set local clipboard (text or image)
- Simple and fast
- No monitoring needed on controller (one-way only)

---

### **Phase 3: WebRTC Integration (1 hour)**

**Agent:** `agent/internal/webrtc/peer.go`
```go
// Start clipboard monitor on agent
m.clipboardMonitor = clipboard.NewMonitor()
m.clipboardMonitor.SetOnTextChange(func(text string) {
    // Send text to controller
    msg := map[string]interface{}{
        "type": "clipboard_text",
        "content": text,
    }
    data, _ := json.Marshal(msg)
    m.dataChannel.Send(data)
})
m.clipboardMonitor.SetOnImageChange(func(imageData []byte) {
    // Send image to controller
    msg := map[string]interface{}{
        "type": "clipboard_image",
        "content": base64.StdEncoding.EncodeToString(imageData),
    }
    data, _ := json.Marshal(msg)
    m.dataChannel.Send(data)
})
m.clipboardMonitor.Start()
```

**Controller:** `controller/internal/viewer/connection.go`
```go
// Initialize clipboard receiver
v.clipboardReceiver = clipboard.NewReceiver()

// Handle incoming clipboard from agent
client.SetOnDataChannelMessage(func(msg []byte) {
    var data map[string]interface{}
    json.Unmarshal(msg, &data)
    
    switch data["type"] {
    case "clipboard_text":
        text := data["content"].(string)
        v.clipboardReceiver.SetText(text)
        log.Println("📋 Clipboard synced (text)")
        
    case "clipboard_image":
        imageB64 := data["content"].(string)
        imageData, _ := base64.StdEncoding.DecodeString(imageB64)
        v.clipboardReceiver.SetImage(imageData)
        log.Println("📋 Clipboard synced (image)")
    }
})
```

---

### **Phase 4: UI Integration (Optional - 30 minutes)**

**Status Indicators:**
- "📋 Clipboard synced" log message
- "⚠️ Clipboard too large" warning
- "❌ Clipboard sync failed" error

**Optional UI:**
- Status label showing last clipboard sync
- Enable/disable clipboard sync toggle

---

## 📊 **Protocol Specification**

### **Message Types (Simplified):**

1. **clipboard_text** - Text clipboard content (agent → controller)
2. **clipboard_image** - Image clipboard content (agent → controller)

### **Message Flow (One-Way):**

```
Controller                          Agent
    |                                 |
    |                         [User copies text]
    |<-- clipboard_text -------------|
    |  (automatically sets local clipboard)
    |                                 |
    |                      [User copies image]
    |<-- clipboard_image ------------|
    |  (automatically sets local clipboard)
```

**Simple and automatic - just like RDP!**

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

### **Priority 1 (Core - Like RDP):**
1. ✅ Text clipboard sync (agent → controller)
2. ✅ Image clipboard sync (agent → controller)
3. ✅ Automatic clipboard monitoring on agent
4. ✅ WebRTC integration
5. ✅ Error handling

### **Priority 2 (Future Enhancements):**
6. ⏳ Reverse sync (controller → agent)
7. ⏳ Manual sync button
8. ⏳ UI notifications
9. ⏳ File clipboard sync
10. ⏳ Compression for large data

---

## 📝 **Implementation Checklist (Simplified)**

**Core Implementation (3-4 hours):**
- [ ] Create `agent/internal/clipboard/` package
- [ ] Implement clipboard monitor on agent (text + images)
- [ ] Create `controller/internal/clipboard/` package
- [ ] Implement clipboard receiver on controller
- [ ] Define simple message protocol (clipboard_text, clipboard_image)
- [ ] Integrate agent monitor with WebRTC data channel
- [ ] Integrate controller receiver with WebRTC data channel
- [ ] Add error handling for large clipboard data
- [ ] Test text clipboard sync
- [ ] Test image clipboard sync
- [ ] Update documentation

**Future Enhancements:**
- [ ] Add reverse sync (controller → agent)
- [ ] Add manual sync button
- [ ] Add UI notifications
- [ ] Add file clipboard support
- [ ] Add compression

---

## 🐛 **Known Challenges**

### **Challenge 1: Clipboard Monitoring**
- **Issue:** Polling vs event-driven
- **Solution:** Use `golang.design/x/clipboard` with polling (500ms)

### **Challenge 2: Large Images**
- **Issue:** Large images can be slow to transfer
- **Solution:** Compress images before sending, use JPEG for photos

### **Challenge 3: Sync Loops**
- **Issue:** Not applicable (one-way sync only)
- **Solution:** No sync loops possible with agent → controller only

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
