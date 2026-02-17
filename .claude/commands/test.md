---
name: test
description: Kør headless browser-tests mod dashboard JavaScript-koden
argument-hint: [fil eller modul at teste, f.eks. "sessions" eller "webrtc"]
---

# Test Dashboard JavaScript

Kør automatiserede browser-tests mod dashboard JS-modulerne med Puppeteer.

## Fremgangsmåde

### 1. Start lokal HTTP-server
Start en Python HTTP-server fra `docs/`-mappen. Brug en ledig port (prøv 9191, undgå 8080/9000/9090 som er optaget):
```bash
cd docs && python3 -m http.server 9191 &
```
Verificer med `curl` at serveren svarer.

### 2. Opret test-HTML side
Opret en midlertidig `docs/test-dashboard.html` med:
- Alle nødvendige DOM-elementer (tabsContainer, previewCanvas, previewIdle, previewToolbar, previewConnecting, connectedDeviceName, bandwidthStats, sessionStatus, viewerOverlay, connectionInfo, statBitrate, statRtt, statPacketLoss, statConnectionType, sessionTabs, tabAddBtn, previewVideo, etc.)
- Stubs for `debug()`, `showToast()`, `supabase`, `SUPABASE_CONFIG`
- Script-tags der loader JS-filerne i korrekt rækkefølge: sessions.js, webrtc.js, signaling.js, app.js

### 3. Opret og kør Puppeteer-test
Opret `/tmp/test-dashboard.mjs` med tests. Installer puppeteer hvis nødvendigt (`npm install puppeteer` i /tmp).

**Tests der SKAL køres:**

#### Basis: Globale objekter eksisterer
- `SessionManager` med korrekte metoder (createSession, switchToSession, closeSession, getActiveSession, getSessionBySessionId, hasSession)
- `window.startSession`, `window.endSession`, `window.initWebRTC`, `window.sendSignal`
- `window.subscribeToSessionSignaling`, `window.stopPolling`, `window.stopSessionPolling`
- `window.sendControlEvent`, `window.cleanupWebRTC`, `window.cleanupSessionWebRTC`

#### Session lifecycle
- `createSession()` returnerer objekt med alle per-session felter (sessionData, signalingChannel, pollingInterval, processedSignalIds, pendingIceCandidates, frameChunks, expectedChunks, currentFrameId, bytesReceived, framesReceived, framesDropped, bandwidthInterval, statsInterval)
- Duplicate session returnerer eksisterende
- Max 6 sessions håndhæves

#### Multi-session + switching
- Opret 2+ sessions, verificer tab-switching og activeSessionId
- `switchToSession()` sætter korrekte globale refs (window.currentSession, window.peerConnection, window.dataChannel)
- `closeSession()` switcher til remaining session
- Sidste session lukket → globaler ryddet (null)

#### Input routing
- `sendControlEvent()` sender til aktiv sessions dataChannel (mock)
- Ingen fejl når ingen session er aktiv

#### Specifikt modul (hvis argument givet)
Hvis brugeren angiver `$ARGUMENTS`, fokuser tests på det specifikke modul:
- `sessions` → test SessionManager detaljeret
- `webrtc` → test initWebRTC, cleanup, frame handling
- `signaling` → test signaling subscription/polling
- `app` → test startSession/endSession lifecycle

### 4. Rapportér resultater
Vis tydelig oversigt med ✅/❌ for hver test. Rapportér eventuelle JavaScript console-errors (ignorer `checkAuth is not defined` og netværksfejl da auth/supabase er stubbede).

### 5. Ryd op
- Slet `docs/test-dashboard.html`
- Stop HTTP-serveren
- Slet test-filer i /tmp

## Vigtige noter
- Port 8080 er optaget (Docker). Brug 9191 eller lignende.
- `checkAuth is not defined` er forventet fejl (auth-modul ikke loaded i tests).
- Puppeteer's `page.waitForTimeout()` virker ikke i nyere versioner — brug `await new Promise(r => setTimeout(r, ms))`.
