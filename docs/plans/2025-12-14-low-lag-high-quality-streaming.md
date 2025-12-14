# Low-Lag, High-Quality Streaming Implementation Plan

> **For Codex/Claude:** REQUIRED SUB-SKILL: Use `superpowers:executing-plans` to implement this plan task-by-task.

**Goal:** Make remote viewing feel smooth (low input-to-photon latency) while keeping text/UI crisp at reasonable bandwidth.

**Architecture:** Prefer WebRTC H.264 video track for continuous motion (congestion control + no datachannel chunking). Use data channels for input/clipboard/file transfer, and keep JPEG tiles as fallback + optional “crispness boost” for static scenes.

**Tech Stack:** Go (agent/controller), Pion WebRTC, JPEG tiles over datachannel, optional H.264 encoder (`agent/internal/video/encoder`), Supabase signaling (unchanged for this plan).

---

## Success Criteria (measurable)

- **Latency:** RTT < 150ms feels responsive; input “snappiness” improves by prioritizing control channel and avoiding video backlog.
- **Smoothness:** Stable 30 fps on typical WAN; 60 fps optional on LAN/good machines.
- **Picture:** Static UI/text looks sharp (raise quality when idle/static); motion uses H.264 at tuned bitrate.
- **Stability:** No multi-second freezes from missing datachannel chunks; old/incomplete frames are dropped quickly.

## Target Environment Preset (Your Choice: B = Internet + TURN/UDP)

- **Default mode:** `hybrid`
- **Target FPS cap:** 30
- **H.264 bitrate:** start 2500–4000 kbps (use `appSettings.MaxBitrate` as the hard cap)
- **Idle behavior:** 1–3 fps, `quality>=80`, `scale≈1.0` (crisp text without bandwidth spikes)

---

## Task 1: Add a single “Streaming HUD” source of truth (controller)

**Files:**
- Modify: `controller/internal/viewer/connection.go`
- Modify: `controller/internal/viewer/viewer.go`

**Step 1: Add parsing for all stats fields (fps/quality/scale/mode/rtt/cpu)**

In `controller/internal/viewer/connection.go`, extend the `"stats"` case to read:
- `quality` (float64 → int)
- `scale` (float64)
- `cpu` (float64)

**Step 2: Add UI labels for quality/scale/cpu (and show mode already exists)**

In `controller/internal/viewer/viewer.go`, add labels next to existing `FPS/RTT/Mbit/s/mode`.

**Step 3: Manual verification**

Run controller, connect, confirm HUD updates every ~1s from agent logs and UI.

---

## Task 2: Make “Quality” slider and presets actually control the agent

**Files:**
- Modify: `controller/internal/webrtc/client.go`
- Modify: `controller/internal/viewer/viewer.go`
- Modify: `agent/internal/webrtc/peer.go`

**Design:**
Add a new control message sent over data/control channel:
```json
{ "type": "set_stream_params", "max_fps": 60, "max_quality": 90, "max_scale": 1.0, "h264_bitrate_kbps": 4000 }
```

**Step 1: Add `SetStreamParams(...)` API on controller WebRTC client**

In `controller/internal/webrtc/client.go`, add:
- `type StreamParams struct { MaxFPS int; MaxQuality int; MaxScale float64; H264BitrateKbps int }`
- `func (c *Client) SetStreamParams(p StreamParams) error` that marshals/sends JSON with `type=set_stream_params`.

**Step 2: Wire `handleQualityChange` to send new params**

In `controller/internal/viewer/viewer.go`, implement `handleQualityChange` to call:
- `client.SetStreamParams(StreamParams{ MaxQuality: int(value), ... })`

**Step 3: Agent: store “caps” and apply them in the adaptive loop**

In `agent/internal/webrtc/peer.go`:
- Add fields on `Manager` like `maxFPS`, `maxQuality`, `maxScale`, `targetH264BitrateKbps`
- In `handleControlEvent`, add a `case "set_stream_params": ...` parser
- In `startScreenStreaming`, replace hardcoded `maxFPS/maxQuality/maxScale` with these caps (with safe defaults).

**Step 4: Verification**

- Connect, move the quality slider, confirm agent logs show updated caps within 1–2 seconds and the HUD reflects it.

---

## Task 3: Fix “idle mode hurts picture” (sharp static UI)

**Files:**
- Modify: `agent/internal/webrtc/peer.go`

**Rationale:** When idle (low motion), bandwidth is low, so we can afford higher JPEG quality and full scale to make text crisp.

**Step 1: Change idle defaults**

In `startScreenStreaming`:
- Change `idleQuality` from `50` → `80` (or `85`)
- Change `idleScale` from `0.75` → `1.0` (if CPU allows; otherwise `0.85`)

**Step 2: Keep “idle FPS low”**

Keep `idleFPS` low (1–3), but prioritize quality/scale.

**Step 3: Verification**

Open a text-heavy UI on the agent machine, stop moving mouse:
- Image should become sharper within ~1s
- Bandwidth should remain low due to low FPS

---

## Task 4: Reduce freezes from lost chunks (latest-frame-wins)

**Files:**
- Modify: `controller/internal/webrtc/client.go`

**Rationale:** With unreliable channels, missing a single chunk can stall display until a full later frame arrives. We should drop incomplete frames quickly.

**Step 1: Track per-frame deadlines**

Add a small struct to track:
- firstSeen time for each `frameID`
- expected `totalChunks`

If `now - firstSeen > 200ms` (tunable), drop the incomplete frame and free memory.

**Step 2: Prefer smaller frames when using unreliable video channel**

If you detect the incoming stream is frequently chunked:
- lower `max_fps` or `max_scale` via `set_stream_params` (Task 2) rather than relying on retransmits.

**Step 3: Verification**

Simulate loss (or test via TURN TCP/high RTT):
- UI should “skip” frames but keep updating (no multi-second freezes).

---

## Task 5: Default to H.264 (smooth motion) with fast fallback

**Files:**
- Modify: `controller/internal/viewer/connection.go`
- Modify: `controller/internal/viewer/viewer.go`
- Modify: `agent/internal/webrtc/peer.go`

**Step 1: On connect, set mode to `hybrid` (or `h264`)**

In controller connection success callback, call:
- `client.SetStreamingMode("hybrid", appSettings.MaxBitrate*1000)`

**Step 2: Confirm agent H.264 encoder init status is surfaced**

If encoder init failed, the agent already falls back; ensure the HUD shows `mode=jpeg` and consider sending an explicit “mode changed” message.

**Step 3: Add a “Low lag / Balanced / Crisp” preset mapping**

Implement presets (controller UI) that send `SetStreamingMode` + `SetStreamParams`:
- **Low lag:** `h264`, 30 fps cap, moderate bitrate, lower scale under RTT
- **Balanced:** `hybrid`, 30 fps, adaptive caps
- **Crisp (static):** `hybrid`, low FPS idle + high JPEG idle quality/scale

**Step 4: Verification**

LAN test: should hold 60 fps (if enabled) with low RTT.
WAN/TURN test: should hold 20–30 fps without chunking stalls.

---

## Task 6: Add minimal tests for the new “drop incomplete frame” logic

**Files:**
- Create: `controller/internal/webrtc/client_chunk_test.go`

**Step 1: Write failing test**

Test case: send only some chunks for `frameID=123`, wait >200ms in a fake clock or by injecting time provider, verify internal storage is cleared and no callback fires.

**Step 2: Implement minimal time provider (if needed)**

If you can’t easily fake time, inject `now func() time.Time` into `Client`.

**Step 3: Run tests**

Run:
- `cd controller && go test ./...`

---

## Execution Notes (what to do first)

1) Task 3 (idle sharper) is the fastest “picture looks better” win.
2) Task 2 (controller->agent params) makes tuning iterative instead of code changes.
3) Task 4 prevents the worst UX failure mode (freezes).
4) Task 5 makes motion smooth by default.
