# 📱 Android Agent Implementation Plan

## Executive Summary

Add Android support to the Remote Desktop system, allowing users to remotely view and control Android devices from the web dashboard.

**Estimated Timeline:** 4-6 weeks  
**Complexity:** High (Android permissions, screen capture, input injection)  
**Priority:** Medium-High (expand platform support)

---

## 🎯 Goals

### Primary Objectives
- ✅ Screen streaming from Android device to web dashboard
- ✅ Remote touch input (tap, swipe, pinch)
- ✅ Remote keyboard input
- ✅ Device registration and approval (same as Windows)
- ✅ WebRTC P2P connection (reuse existing infrastructure)

### Secondary Objectives
- ✅ Battery optimization
- ✅ Quick settings tile for easy access
- ✅ Picture-in-picture mode support
- ✅ Android 8.0+ compatibility

---

## 📊 Technical Architecture

### Platform Comparison

| Feature | Windows Agent | Android Agent |
|---------|---------------|---------------|
| **Language** | Go | Kotlin |
| **Screen Capture** | Win32 API (GDI/DXGI) | MediaProjection API |
| **Input Control** | robotgo library | AccessibilityService API |
| **WebRTC** | Pion WebRTC (Go) | Google WebRTC (Android SDK) |
| **Background Service** | System Tray | Foreground Service |
| **Permissions** | Admin (optional) | User approval required |
| **Deployment** | Single EXE | APK (Google Play / Direct) |

### Android Stack

```
┌─────────────────────────────────────┐
│   Web Dashboard (Existing)         │
│   - WebRTC Peer                    │
│   - Signaling via Supabase         │
└────────────────┬────────────────────┘
                 │ WebRTC + Signaling
                 │
┌────────────────▼────────────────────┐
│   Android Agent (NEW)              │
├─────────────────────────────────────┤
│  App Layer (Kotlin)                │
│  - MainActivity                    │
│  - ForegroundService               │
│  - AccessibilityService            │
│  - MediaProjection Manager         │
├─────────────────────────────────────┤
│  Core Components                   │
│  - WebRTC Manager                  │
│  - Screen Capturer                 │
│  - Input Controller                │
│  - Device Manager                  │
├─────────────────────────────────────┤
│  Android APIs                      │
│  - MediaProjection API             │
│  - AccessibilityService API        │
│  - WebRTC Android SDK              │
│  - Supabase Kotlin SDK             │
└─────────────────────────────────────┘
```

---

## 🔧 Core Components

### 1. Screen Capture (MediaProjection API)

**API:** `android.media.projection.MediaProjection`

**Requirements:**
- ✅ Android 5.0+ (API 21+)
- ✅ User consent dialog (per session)
- ✅ Foreground service with notification
- ✅ Permission: `FOREGROUND_SERVICE_MEDIA_PROJECTION`

**Implementation:**
```kotlin
class ScreenCaptureService : Service() {
    private lateinit var mediaProjection: MediaProjection
    private lateinit var virtualDisplay: VirtualDisplay
    private lateinit var imageReader: ImageReader
    
    fun startCapture(resultCode: Int, data: Intent) {
        // Create MediaProjection
        mediaProjection = mediaProjectionManager
            .getMediaProjection(resultCode, data)
        
        // Create ImageReader for frame capture
        imageReader = ImageReader.newInstance(
            width, height,
            PixelFormat.RGBA_8888,
            2 // Max images
        )
        
        // Create VirtualDisplay
        virtualDisplay = mediaProjection.createVirtualDisplay(
            "ScreenCapture",
            width, height, density,
            DisplayManager.VIRTUAL_DISPLAY_FLAG_AUTO_MIRROR,
            imageReader.surface,
            null, null
        )
        
        // Set up frame callback
        imageReader.setOnImageAvailableListener({ reader ->
            val image = reader.acquireLatestImage()
            // Convert to JPEG and send via WebRTC
            sendFrameToWebRTC(image)
            image.close()
        }, handler)
    }
}
```

**Challenges:**
- ❌ **User consent required** - Every session needs approval dialog
- ❌ **Foreground notification** - Must show persistent notification
- ❌ **Battery drain** - Continuous screen capture is power-intensive
- ✅ **Solution:** Lower FPS (10-15), optimize encoding, stop when idle

---

### 2. Input Control (AccessibilityService)

**API:** `android.accessibilityservice.AccessibilityService`

**Requirements:**
- ✅ User must enable accessibility service in Settings
- ✅ One-time permission (not per session)
- ✅ Can inject gestures programmatically

**Implementation:**
```kotlin
class RemoteInputService : AccessibilityService() {
    
    override fun onAccessibilityEvent(event: AccessibilityEvent?) {
        // Listen for system events (optional)
    }
    
    override fun onInterrupt() {}
    
    fun performTap(x: Float, y: Float) {
        val path = Path().apply {
            moveTo(x, y)
        }
        
        val gesture = GestureDescription.Builder()
            .addStroke(GestureDescription.StrokeDescription(
                path, 0, 10 // 10ms tap
            ))
            .build()
        
        dispatchGesture(gesture, null, null)
    }
    
    fun performSwipe(x1: Float, y1: Float, x2: Float, y2: Float) {
        val path = Path().apply {
            moveTo(x1, y1)
            lineTo(x2, y2)
        }
        
        val gesture = GestureDescription.Builder()
            .addStroke(GestureDescription.StrokeDescription(
                path, 0, 300 // 300ms swipe
            ))
            .build()
        
        dispatchGesture(gesture, null, null)
    }
    
    fun performPinch(centerX: Float, centerY: Float, scale: Float) {
        // Two-finger pinch gesture
        // Requires continued gestures (API 26+)
    }
}
```

**Challenges:**
- ❌ **Setup friction** - Users must manually enable in Settings
- ❌ **Security warning** - Android warns about accessibility risks
- ❌ **Complex gestures** - Multi-touch requires Android 8.0+
- ✅ **Solution:** Clear onboarding UI, show step-by-step guide

---

### 3. WebRTC Connection (Google WebRTC SDK)

**Library:** `org.webrtc:google-webrtc`

**Gradle Dependency:**
```gradle
implementation 'org.webrtc:google-webrtc:1.0.32006'
```

**Implementation:**
```kotlin
class WebRTCManager(private val context: Context) {
    private var peerConnection: PeerConnection? = null
    private val videoSource: VideoSource
    private val videoTrack: VideoTrack
    
    init {
        // Initialize PeerConnectionFactory
        val options = PeerConnectionFactory.InitializationOptions.builder(context)
            .setEnableInternalTracer(true)
            .createInitializationOptions()
        PeerConnectionFactory.initialize(options)
        
        val factory = PeerConnectionFactory.builder()
            .setVideoEncoderFactory(
                DefaultVideoEncoderFactory(
                    rootEglBase.eglBaseContext,
                    true, // Enable hardware encoding
                    true  // Enable H264 high profile
                )
            )
            .setVideoDecoderFactory(
                DefaultVideoDecoderFactory(rootEglBase.eglBaseContext)
            )
            .createPeerConnectionFactory()
        
        // Create video source from screen capture
        videoSource = factory.createVideoSource(false)
        videoTrack = factory.createVideoTrack("screen", videoSource)
    }
    
    fun createOffer(callback: (SessionDescription) -> Unit) {
        val constraints = MediaConstraints().apply {
            mandatory.add(MediaConstraints.KeyValuePair("OfferToReceiveVideo", "false"))
            mandatory.add(MediaConstraints.KeyValuePair("OfferToReceiveAudio", "false"))
        }
        
        peerConnection?.createOffer(object : SdpObserver {
            override fun onCreateSuccess(sdp: SessionDescription) {
                peerConnection?.setLocalDescription(SdpObserver(), sdp)
                callback(sdp)
            }
            // ... other callbacks
        }, constraints)
    }
    
    fun sendVideoFrame(image: Image) {
        // Convert Image to VideoFrame
        val buffer = convertImageToI420(image)
        val videoFrame = VideoFrame(buffer, 0, System.nanoTime())
        videoSource.capturerObserver.onFrameCaptured(videoFrame)
        videoFrame.release()
    }
}
```

**Features:**
- ✅ Hardware encoding (H264, VP8, VP9)
- ✅ Adaptive bitrate
- ✅ Same signaling as Windows agent (Supabase)
- ✅ Data channel for input events

---

### 4. Foreground Service (Background Operation)

**API:** `android.app.Service`

**Requirements:**
- ✅ Persistent notification
- ✅ Service type: `mediaProjection`
- ✅ Cannot be dismissed by user

**Implementation:**
```kotlin
class RemoteAgentService : Service() {
    
    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        createNotificationChannel()
        
        val notification = NotificationCompat.Builder(this, CHANNEL_ID)
            .setContentTitle("Remote Agent Active")
            .setContentText("Device is accessible remotely")
            .setSmallIcon(R.drawable.ic_remote)
            .setOngoing(true)
            .setPriority(NotificationCompat.PRIORITY_LOW)
            .build()
        
        startForeground(NOTIFICATION_ID, notification)
        
        // Start screen capture and WebRTC
        startRemoteSession()
        
        return START_STICKY // Restart if killed
    }
    
    private fun createNotificationChannel() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            val channel = NotificationChannel(
                CHANNEL_ID,
                "Remote Agent",
                NotificationManager.IMPORTANCE_LOW
            ).apply {
                description = "Background service for remote access"
            }
            notificationManager.createNotificationChannel(channel)
        }
    }
}
```

**Manifest:**
```xml
<service
    android:name=".RemoteAgentService"
    android:foregroundServiceType="mediaProjection"
    android:exported="false" />

<uses-permission android:name="android.permission.FOREGROUND_SERVICE" />
<uses-permission android:name="android.permission.FOREGROUND_SERVICE_MEDIA_PROJECTION" />
```

---

## 📱 User Experience

### Onboarding Flow

```
1. Install APK
   ↓
2. Open app → Login with email
   ↓
3. Grant accessibility permission
   (Settings → Accessibility → Remote Agent → Enable)
   ↓
4. Start service
   ↓
5. Tap "Start Remote Access"
   (Shows MediaProjection consent dialog)
   ↓
6. Device registered & online
   ↓
7. Dashboard shows device
   ↓
8. User clicks "Connect"
   ↓
9. Agent shows PIN prompt
   ↓
10. User enters PIN → Session starts!
```

### UI Components

**MainActivity:**
- Device status (Online/Offline)
- Start/Stop button
- Current session info
- Settings (quality, FPS)

**Quick Settings Tile:**
- Toggle remote access on/off
- Show current status
- Fast access without opening app

**Notification:**
- "Remote Agent Active"
- Tap to open app
- Stop button (ends session)

---

## 🔒 Permissions & Security

### Required Permissions

```xml
<!-- Screen Capture -->
<uses-permission android:name="android.permission.FOREGROUND_SERVICE" />
<uses-permission android:name="android.permission.FOREGROUND_SERVICE_MEDIA_PROJECTION" />

<!-- Accessibility (Input Control) -->
<!-- Declared in accessibility_service_config.xml -->

<!-- Network -->
<uses-permission android:name="android.permission.INTERNET" />
<uses-permission android:name="android.permission.ACCESS_NETWORK_STATE" />

<!-- Wake Lock (Keep screen on during session) -->
<uses-permission android:name="android.permission.WAKE_LOCK" />
```

### Security Considerations

**✅ Strengths:**
- User must explicitly enable accessibility
- Screen capture requires per-session consent
- Same PIN-based session approval as Windows
- WebRTC encryption (DTLS-SRTP)

**⚠️ Concerns:**
- Accessibility service can be misused (explain clearly in UI)
- Screen capture shows everything (passwords, notifications)
- Battery drain if left running

**🛡️ Mitigations:**
- Clear privacy policy
- Auto-stop after 30min inactivity
- Show warning about visible content
- Option to pause notifications during session

---

## 📦 Project Structure

```
android-agent/
├── app/
│   ├── src/
│   │   ├── main/
│   │   │   ├── java/com/stangtennis/remoteagent/
│   │   │   │   ├── ui/
│   │   │   │   │   ├── MainActivity.kt
│   │   │   │   │   ├── OnboardingActivity.kt
│   │   │   │   │   └── SettingsActivity.kt
│   │   │   │   ├── service/
│   │   │   │   │   ├── RemoteAgentService.kt
│   │   │   │   │   ├── RemoteInputService.kt (Accessibility)
│   │   │   │   │   └── QuickSettingsTileService.kt
│   │   │   │   ├── core/
│   │   │   │   │   ├── ScreenCapturer.kt
│   │   │   │   │   ├── InputController.kt
│   │   │   │   │   ├── WebRTCManager.kt
│   │   │   │   │   └── DeviceManager.kt
│   │   │   │   ├── data/
│   │   │   │   │   ├── SupabaseClient.kt
│   │   │   │   │   ├── Device.kt
│   │   │   │   │   └── Session.kt
│   │   │   │   └── util/
│   │   │   │       ├── ImageConverter.kt
│   │   │   │       └── NetworkMonitor.kt
│   │   │   ├── res/
│   │   │   │   ├── layout/
│   │   │   │   ├── values/
│   │   │   │   └── xml/
│   │   │   │       └── accessibility_service_config.xml
│   │   │   └── AndroidManifest.xml
│   │   └── build.gradle.kts
│   └── build.gradle.kts
├── gradle/
├── build.gradle.kts
└── settings.gradle.kts
```

---

## 🗓️ Implementation Phases

### Phase 1: Foundation (Week 1-2)
- [ ] Set up Android project (Kotlin + Gradle)
- [ ] Add dependencies (WebRTC, Supabase, Coroutines)
- [ ] Implement device registration
- [ ] Basic UI (MainActivity, status display)
- [ ] Supabase integration (auth, device CRUD)

**Deliverable:** App can register device and show online status

### Phase 2: Screen Capture (Week 2-3)
- [ ] Implement MediaProjection screen capture
- [ ] Foreground service with notification
- [ ] Convert frames to JPEG/H264
- [ ] Test frame rate and quality optimization
- [ ] Add FPS counter and stats

**Deliverable:** App can capture and encode screen

### Phase 3: WebRTC Streaming (Week 3-4)
- [ ] Implement WebRTC peer connection
- [ ] Integrate with Supabase signaling
- [ ] Send video frames via WebRTC
- [ ] Test P2P connection
- [ ] Add TURN fallback support

**Deliverable:** Dashboard can see Android screen

### Phase 4: Input Control (Week 4-5)
- [ ] Implement AccessibilityService
- [ ] Handle tap, swipe, long-press events
- [ ] Receive input from WebRTC data channel
- [ ] Map web coordinates to screen coordinates
- [ ] Test input latency and accuracy

**Deliverable:** Dashboard can control Android device

### Phase 5: Polish & Testing (Week 5-6)
- [ ] Onboarding flow with permission guide
- [ ] Quick Settings tile
- [ ] Settings screen (quality, FPS, auto-stop)
- [ ] Battery optimization
- [ ] Error handling and reconnection
- [ ] User testing on multiple devices

**Deliverable:** Production-ready Android agent

### Phase 6: Release (Week 6)
- [ ] Code signing (Android keystore)
- [ ] Generate APK and AAB (Android App Bundle)
- [ ] Create GitHub release
- [ ] Update documentation
- [ ] Optional: Publish to Google Play Store

**Deliverable:** Public release

---

## 📊 Technology Stack

| Component | Technology | Notes |
|-----------|-----------|-------|
| **Language** | Kotlin | Modern, recommended by Google |
| **UI** | Jetpack Compose | Declarative UI (optional, can use XML) |
| **Async** | Kotlin Coroutines | Async/await pattern |
| **WebRTC** | `org.webrtc:google-webrtc` | Official Android WebRTC SDK |
| **Supabase** | `io.github.jan-tennert.supabase:postgrest-kt` | Kotlin client |
| **Video Encoding** | MediaCodec API | Hardware H264 encoding |
| **DI** | Hilt/Koin | Dependency injection (optional) |
| **Min SDK** | API 26 (Android 8.0) | For continued gestures |
| **Target SDK** | API 34 (Android 14) | Latest features |

---

## ⚡ Performance Optimization

### Screen Capture
- **Resolution:** Max 1920x1080 (scale down if higher)
- **FPS:** 10-15 FPS (lower on battery)
- **Encoding:** H264 hardware encoding
- **Quality:** CRF 28-32 (balanced)
- **Frame dropping:** Skip frames if WebRTC buffer full

### Battery Life
- **Stop when idle:** Auto-stop after 30min no input
- **Lower FPS on battery:** 10 FPS when not charging
- **Wake lock:** Only during active session
- **Background:** Stop screen capture when dashboard disconnected

### Network
- **Adaptive bitrate:** Adjust quality based on RTT
- **Frame skipping:** Drop frames on high latency
- **Compression:** Use H264 instead of JPEG when possible

---

## ❓ Challenges & Solutions

### Challenge 1: User Consent for Screen Capture
**Problem:** Every session requires MediaProjection approval dialog  
**Solution:** 
- Cache the result intent (works for single session only on Android 14+)
- Clear UI explaining why consent is needed
- Auto-reconnect if dialog dismissed

### Challenge 2: Accessibility Permission Friction
**Problem:** Users must manually enable in Settings  
**Solution:**
- Step-by-step onboarding with screenshots
- Direct deep-link to Settings page
- Explain benefits clearly (remote control)

### Challenge 3: Battery Drain
**Problem:** Continuous screen capture drains battery fast  
**Solution:**
- Lower FPS (10-15 instead of 30)
- Auto-stop on idle
- Show battery warning in notification
- Pause capture when dashboard disconnects

### Challenge 4: Multi-Touch Gestures
**Problem:** Pinch-to-zoom requires Android 8.0+ and complex API  
**Solution:**
- Phase 1: Support tap and swipe only
- Phase 2: Add pinch/zoom for API 26+
- Use `continueStroke()` for multi-touch

### Challenge 5: Google Play Store Policy
**Problem:** Accessibility services have strict review  
**Solution:**
- Clear privacy policy
- Explain use case (remote desktop)
- Optional: Distribute via direct APK first
- Consider Google Play's Accessibility Policy

---

## 🎯 Success Metrics

### Functional
- ✅ Screen streaming works on 5+ different Android devices
- ✅ Input control works with <100ms latency
- ✅ App survives background (START_STICKY)
- ✅ Reconnection works after network change

### Performance
- ✅ Battery drain <10%/hour during active session
- ✅ Frame rate 10-15 FPS stable
- ✅ App size <20MB
- ✅ Memory usage <150MB

### UX
- ✅ Onboarding completed in <3 minutes
- ✅ User can start session in <30 seconds
- ✅ Clear error messages for missing permissions

---

## 📚 Documentation Needed

- [ ] **ANDROID_SETUP.md** - Installation and permissions guide
- [ ] **ANDROID_DEVELOPMENT.md** - Build and test locally
- [ ] **ANDROID_TROUBLESHOOTING.md** - Common issues
- [ ] Update **README.md** - Add Android support info
- [ ] Update **USER_APPROVAL_GUIDE.md** - Android-specific notes

---

## 🚀 Next Steps

### Immediate (Before Starting)
1. Research WebRTC Android SDK documentation
2. Set up Android Studio and test devices
3. Create prototype: Screen capture → Display locally
4. Test AccessibilityService input injection
5. Validate Supabase Kotlin SDK compatibility

### Short-term (Phase 1)
1. Create Android project skeleton
2. Implement device registration
3. Test WebRTC peer connection setup
4. Build basic UI

### Long-term (Post-Release)
1. Gather user feedback
2. Optimize battery life further
3. Add Android-specific features (Picture-in-Picture)
4. Consider publishing to Google Play Store
5. Support older Android versions (API 21-25)

---

## 💰 Cost Considerations

### Development
- **Time:** 4-6 weeks full-time
- **Test Devices:** 3-5 Android devices (various versions)
- **Google Play Developer Account:** $25 one-time (if publishing)

### Ongoing
- **Same backend as Windows** - No additional Supabase/TURN costs
- **APK Hosting** - Free via GitHub Releases
- **Play Store** - No ongoing fees

---

## ✅ Conclusion

**Feasibility:** ✅ **High** - All required APIs are available and well-documented

**Effort:** ⚠️ **Medium-High** - Significant development work, but no blockers

**Value:** ✅ **High** - Doubles the platform support, makes app more versatile

**Recommendation:** ✅ **Proceed** - Start with Phase 1 prototype to validate approach

---

## 📖 References

- [MediaProjection API Documentation](https://developer.android.com/media/grow/media-projection)
- [AccessibilityService Guide](https://developer.android.com/guide/topics/ui/accessibility/service)
- [WebRTC Android SDK](https://webrtc.github.io/webrtc-org/native-code/android/)
- [Foreground Services](https://developer.android.com/develop/background-work/services/foreground-services)
- [Supabase Kotlin Client](https://github.com/supabase-community/supabase-kt)
- [Android Gesture Dispatch](https://developer.android.com/reference/android/accessibilityservice/AccessibilityService#dispatchGesture(android.accessibilityservice.GestureDescription,%20android.accessibilityservice.AccessibilityService.GestureResultCallback,%20android.os.Handler))

---

**Created:** 2025-01-09  
**Version:** 1.0  
**Status:** Draft - Ready for Review
