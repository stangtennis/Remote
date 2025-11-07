# 🧪 Clipboard Sync Testing Guide

**Version:** v2.2.0  
**Feature:** Clipboard Synchronization (Agent → Controller)  
**Status:** Ready for Testing

---

## 📋 **Quick Test Checklist**

### **Basic Tests:**
- [ ] Text clipboard sync (small)
- [ ] Text clipboard sync (large)
- [ ] Image clipboard sync (screenshot)
- [ ] Image clipboard sync (file)
- [ ] Multiple clipboard changes
- [ ] Connection/disconnection
- [ ] Reconnection

### **Edge Cases:**
- [ ] Very large text (>10MB)
- [ ] Very large image (>50MB)
- [ ] Empty clipboard
- [ ] Special characters
- [ ] Unicode text
- [ ] Multiple rapid changes

---

## 🚀 **Setup**

### **Prerequisites:**
1. Build agent: `cd agent && go build -o remote-agent.exe .\cmd\remote-agent`
2. Build controller: `cd controller && go build -o controller.exe .`
3. Start agent on remote machine
4. Start controller on local machine
5. Connect to remote device

---

## ✅ **Test Cases**

### **Test 1: Basic Text Clipboard**

**Objective:** Verify text clipboard sync works

**Steps:**
1. Connect to remote device
2. On remote machine: Open Notepad
3. Type: "Hello from remote!"
4. Select all (Ctrl+A) and copy (Ctrl+C)
5. On local machine: Open Notepad
6. Paste (Ctrl+V)

**Expected Result:**
- ✅ Text "Hello from remote!" appears in local Notepad
- ✅ Sync happens within ~500ms
- ✅ Agent logs: "📋 Text clipboard changed (X bytes)"
- ✅ Controller logs: "📋 Clipboard updated with text (X bytes)"

**Status:** [ ] Pass [ ] Fail

---

### **Test 2: Large Text Clipboard**

**Objective:** Verify large text clipboard sync

**Steps:**
1. Connect to remote device
2. On remote machine: Open a large text file (1-5MB)
3. Select all (Ctrl+A) and copy (Ctrl+C)
4. On local machine: Open Notepad
5. Paste (Ctrl+V)

**Expected Result:**
- ✅ Large text appears in local Notepad
- ✅ Sync completes successfully
- ✅ No errors in logs

**Status:** [ ] Pass [ ] Fail

---

### **Test 3: Screenshot Clipboard**

**Objective:** Verify image clipboard sync (screenshot)

**Steps:**
1. Connect to remote device
2. On remote machine: Take screenshot (Win+Shift+S)
3. Select an area to capture
4. On local machine: Open Paint
5. Paste (Ctrl+V)

**Expected Result:**
- ✅ Screenshot appears in Paint
- ✅ Image quality is good
- ✅ Agent logs: "📋 Image clipboard changed (X bytes)"
- ✅ Controller logs: "📋 Clipboard updated with raw image (X bytes)"

**Status:** [ ] Pass [ ] Fail

---

### **Test 4: Image File Clipboard**

**Objective:** Verify image clipboard sync (file)

**Steps:**
1. Connect to remote device
2. On remote machine: Open an image file in Paint
3. Select all (Ctrl+A) and copy (Ctrl+C)
4. On local machine: Open Paint
5. Paste (Ctrl+V)

**Expected Result:**
- ✅ Image appears in Paint
- ✅ Image quality is preserved
- ✅ Colors are correct

**Status:** [ ] Pass [ ] Fail

---

### **Test 5: Multiple Clipboard Changes**

**Objective:** Verify multiple clipboard changes sync correctly

**Steps:**
1. Connect to remote device
2. On remote machine: Copy "Text 1"
3. Wait 1 second
4. On local machine: Paste - should see "Text 1"
5. On remote machine: Copy "Text 2"
6. Wait 1 second
7. On local machine: Paste - should see "Text 2"
8. On remote machine: Copy "Text 3"
9. Wait 1 second
10. On local machine: Paste - should see "Text 3"

**Expected Result:**
- ✅ Each clipboard change syncs correctly
- ✅ No duplicates
- ✅ No missed changes
- ✅ Correct order

**Status:** [ ] Pass [ ] Fail

---

### **Test 6: Connection/Disconnection**

**Objective:** Verify clipboard monitoring starts/stops correctly

**Steps:**
1. Connect to remote device
2. Verify agent logs: "📋 Clipboard receiver initialized"
3. Copy text on remote
4. Verify sync works
5. Disconnect from device
6. Verify agent logs: "❌ Data channel closed"
7. Copy text on remote (should not sync)
8. Reconnect to device
9. Copy text on remote
10. Verify sync works again

**Expected Result:**
- ✅ Monitoring starts on connection
- ✅ Monitoring stops on disconnection
- ✅ Monitoring restarts on reconnection
- ✅ No errors in logs

**Status:** [ ] Pass [ ] Fail

---

### **Test 7: Very Large Text (>10MB)**

**Objective:** Verify size limit handling

**Steps:**
1. Connect to remote device
2. On remote machine: Create a text file >10MB
3. Open in Notepad and copy all
4. Check agent logs

**Expected Result:**
- ✅ Agent logs: "⚠️ Clipboard text too large (>10MB), skipping"
- ✅ No crash or error
- ✅ Connection remains stable

**Status:** [ ] Pass [ ] Fail

---

### **Test 8: Very Large Image (>50MB)**

**Objective:** Verify image size limit handling

**Steps:**
1. Connect to remote device
2. On remote machine: Open a very large image (>50MB) in Paint
3. Copy the image
4. Check agent logs

**Expected Result:**
- ✅ Agent logs: "⚠️ Clipboard image too large (>50MB), skipping"
- ✅ No crash or error
- ✅ Connection remains stable

**Status:** [ ] Pass [ ] Fail

---

### **Test 9: Special Characters**

**Objective:** Verify special character handling

**Steps:**
1. Connect to remote device
2. On remote machine: Copy text with special characters:
   - "Hello! @#$%^&*() 123"
   - "Quotes: 'single' \"double\""
   - "Symbols: €£¥₹"
3. On local machine: Paste each

**Expected Result:**
- ✅ All special characters appear correctly
- ✅ No encoding issues
- ✅ No corruption

**Status:** [ ] Pass [ ] Fail

---

### **Test 10: Unicode Text**

**Objective:** Verify Unicode text handling

**Steps:**
1. Connect to remote device
2. On remote machine: Copy Unicode text:
   - "Hello 世界 🌍"
   - "Привет мир"
   - "مرحبا بالعالم"
3. On local machine: Paste each

**Expected Result:**
- ✅ All Unicode characters appear correctly
- ✅ Emojis display properly
- ✅ No encoding issues

**Status:** [ ] Pass [ ] Fail

---

### **Test 11: Empty Clipboard**

**Objective:** Verify empty clipboard handling

**Steps:**
1. Connect to remote device
2. On remote machine: Clear clipboard (copy nothing)
3. Check logs

**Expected Result:**
- ✅ No errors in logs
- ✅ No unnecessary sync attempts
- ✅ Connection remains stable

**Status:** [ ] Pass [ ] Fail

---

### **Test 12: Rapid Clipboard Changes**

**Objective:** Verify rapid change handling

**Steps:**
1. Connect to remote device
2. On remote machine: Rapidly copy different texts (5-10 times quickly)
3. Wait 2 seconds
4. On local machine: Paste

**Expected Result:**
- ✅ Last copied text appears
- ✅ No crashes or errors
- ✅ Hash detection prevents duplicate sends
- ✅ Connection remains stable

**Status:** [ ] Pass [ ] Fail

---

## 📊 **Performance Tests**

### **Test P1: Latency**

**Objective:** Measure clipboard sync latency

**Steps:**
1. Connect to remote device
2. Copy text on remote
3. Immediately try to paste on local
4. Measure time until paste works

**Expected Result:**
- ✅ Latency < 1 second (typically ~500ms)

**Actual Latency:** _____ ms

---

### **Test P2: CPU Usage**

**Objective:** Verify CPU usage is acceptable

**Steps:**
1. Connect to remote device
2. Monitor agent CPU usage (Task Manager)
3. Let it run for 5 minutes

**Expected Result:**
- ✅ CPU usage < 5% when idle
- ✅ CPU usage < 20% during clipboard changes

**Actual CPU Usage:** _____ %

---

### **Test P3: Memory Usage**

**Objective:** Verify memory usage is acceptable

**Steps:**
1. Connect to remote device
2. Monitor agent memory usage (Task Manager)
3. Copy various clipboard content (text, images)
4. Let it run for 5 minutes

**Expected Result:**
- ✅ Memory usage < 100MB
- ✅ No memory leaks

**Actual Memory Usage:** _____ MB

---

## 🐛 **Bug Report Template**

If you find a bug, please report it with:

```
**Bug Title:** [Short description]

**Steps to Reproduce:**
1. [Step 1]
2. [Step 2]
3. [Step 3]

**Expected Behavior:**
[What should happen]

**Actual Behavior:**
[What actually happened]

**Logs:**
[Agent logs]
[Controller logs]

**System Info:**
- OS: [Windows version]
- Agent version: [version]
- Controller version: [version]

**Additional Context:**
[Any other relevant information]
```

---

## ✅ **Test Summary**

### **Test Results:**

| Test | Status | Notes |
|------|--------|-------|
| Basic Text | [ ] | |
| Large Text | [ ] | |
| Screenshot | [ ] | |
| Image File | [ ] | |
| Multiple Changes | [ ] | |
| Connection/Disconnection | [ ] | |
| Very Large Text | [ ] | |
| Very Large Image | [ ] | |
| Special Characters | [ ] | |
| Unicode Text | [ ] | |
| Empty Clipboard | [ ] | |
| Rapid Changes | [ ] | |

**Total Tests:** 12  
**Passed:** ___  
**Failed:** ___  
**Pass Rate:** ____%

---

## 📝 **Notes**

### **What Worked Well:**
- [Add notes]

### **Issues Found:**
- [Add notes]

### **Suggestions:**
- [Add notes]

---

## 🎯 **Sign-Off**

**Tester:** _______________  
**Date:** _______________  
**Version Tested:** v2.2.0  
**Overall Status:** [ ] Pass [ ] Fail  

**Ready for Release:** [ ] Yes [ ] No

---

**Happy Testing! 🧪**
