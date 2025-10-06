# RDP Screen Capture - Research & Solutions

## The Problem
When running inside an RDP session, Windows blocks most screen capture APIs:
- ❌ DXGI Desktop Duplication - Access Denied (requires console session)
- ❌ BitBlt/GDI - Fails in RDP (returns black screen or fails)
- ❌ FFmpeg gdigrab - Access Denied error 5
- ❌ PrintWindow - Doesn't capture actual content in RDP

## Why This Happens
RDP creates a **virtual display adapter**. Screen capture APIs that work on physical hardware fail because:
1. No direct access to GPU in RDP session
2. Windows security model blocks session-crossing capture
3. RDP redirects display to network, not a real framebuffer

## Solutions That Actually Work

### Solution 1: Use Windows.Graphics.Capture API (WGC) ⭐ BEST
**Status**: Requires MSVC compiler (not available with MinGW/GCC)
- This API was designed for screen recording in all scenarios
- Works in RDP, works with DPI scaling, works everywhere
- Used by OBS, Microsoft Teams, etc.
- **Problem**: Needs C++/WinRT headers only available with MSVC

### Solution 2: Mirror Driver
**Status**: Deprecated in Windows 8+, not an option

### Solution 3: Run Agent on Console Session
**Status**: WORKS but requires different setup
- Install agent as Windows Service running on Session 0
- Or run agent on physical console (not via RDP)
- Then capture works normally
- **Limitation**: Can't run from RDP session

### Solution 4: Capture via RDP Protocol Itself
**Status**: Possible but complex
- Hook into RDP client/server communication
- Capture the actual RDP stream
- Not what user wants (wants to capture the remote machine's screen)

### Solution 5: Use Legacy GDI with Workarounds
**Status**: TESTING - May capture blank/black frames
- Try multiple GDI methods
- May only work for some windows, not full desktop
- Performance issues

## Recommended Solution

### For RDP Sessions: Install Agent as Service
The agent needs to run on the **console session** (Session 0 or physical console), not inside your RDP session.

**Steps**:
1. Create Windows Service wrapper for the agent
2. Service runs on console (always has screen access)
3. You connect via RDP to manage the machine
4. Agent running as service captures the console screen
5. You view the console screen via your dashboard

This is how TeamViewer, AnyDesk, etc. work - they run as services.

### Alternative: Switch to WGC (Requires MSVC)
Would need to:
1. Install Visual Studio Build Tools
2. Recompile with MSVC instead of GCC
3. Use Windows.Graphics.Capture API
4. Would work perfectly in RDP

## What We'll Try Next
Since we can't easily switch compilers, let's try:
1. Better GDI fallbacks with error handling
2. Detect RDP session and warn user
3. Try the `screenshot` library as last resort
4. Provide clear instructions for running as service
