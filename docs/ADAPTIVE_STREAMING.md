# Adaptive Streaming (v2.51.1)

Implementeret adaptiv streaming der justerer kvalitet baseret på netværksforhold.

## Parametre

| Parameter | Range | Default | Beskrivelse |
|-----------|-------|---------|-------------|
| FPS | 12-30 | 20 | Frames per second |
| Quality | 50-80 | 65 | JPEG kvalitet |
| Scale | 50-100% (0.5-1.0) | 100% (1.0) | Skalering af opløsning |

## Målinger

### Implementeret (v2.51.1)
- `bufBytes` - DataChannel buffered amount (bytes)
- `RTT` - Round-trip time via ping/pong (ms)
- `motionPct` - Procent af skærm ændret (fra DirtyRegionDetector)
- `lossPct` - Packet loss percentage (estimeret fra buffer congestion, 0-10%)

### Planlagt
- `sendBps` - Aktuel send bitrate (bytes sendt / tid siden sidste måling)
- `cpuPct` - Process CPU forbrug; threshold 85% over 3 målinger → sænk FPS/Scale

## Adaptation Logic

### Nuværende regler (v2.51.1)

**Reducer kvalitet når:**
```
bufBytes > 8MB ELLER lossPct > 5% ELLER RTT > 250ms
→ FPS -= 4, Scale -= 0.1, Quality -= 5
```

**Øg kvalitet når:**
```
bufBytes < 1MB OG lossPct < 1% OG RTT < 120ms OG ingen dropped frames
→ Quality += 2, Scale += 0.05, FPS += 2
```

**Drop frames når:**
- `bufBytes > 16 MB` (kritisk congestion)

**Idle-mode:**
```
motionPct < 1% i 1 sekund OG ingen input i 500ms
→ FPS = 2, Scale = 0.75, Quality = 50
Exit idle ved motion > 1% eller input-event
```

**Full-frame refresh (v2.49.0):**
```
Hvert 5. sekund ELLER motionPct > 30%
→ Send komplet frame (ikke delta)
```

### Planlagte regler (v2.50+)

**CPU-guard:**
```
cpuPct > 85% over 3 målinger
→ Sænk FPS og Scale et trin
```

## Input-prioritet (v2.48.0)

Separat data channel for input:
- `ordered = false`
- `maxRetransmits = 0`
- Pausér frame-send hvis backlog > 8-16 MB

## Full-frame refresh (v2.49.0)

- Send full frame hver 5 sekunder
- Eller når `motionPct > 30%`
- Sikrer resync for dirty tiles/foveated mode

## Modes (v2.51.1)

| Mode | Beskrivelse |
|------|-------------|
| `tiles-only` | Kun JPEG frames over data channel (default, altid tilgængelig) |
| `hybrid` | H.264 video track + tiles/foveated over data channel |
| `h264-only` | Kun H.264 video track |

**CPU-only hosts:** `tiles-only` er default fallback. `hybrid` bruger software H.264 med 720p/15-24 fps som startværdier.

**Auto-switch til tiles-only:**
- TURN TCP med RTT > 300ms
- Encoder init fejl
- CPU > 90% sustained

Se `H264_IMPLEMENTATION_PLAN.md` for detaljer.

## Kodeændringer

### `agent/internal/screen/capture.go`
Ny funktion `CaptureJPEGScaled(quality int, scale float64)`:
- Capturer skærm som RGBA
- Skalerer med Bilinear (hurtig)
- Encoder til JPEG
- Returnerer bytes + skalerede dimensioner

### `agent/internal/webrtc/peer.go`
Opdateret `startScreenStreaming()`:
- Adaptive parametre (fps, quality, scale)
- Buffer-baseret justering hver 500ms
- Dynamisk ticker reset ved FPS-ændring
- Logging hvert sekund med aktuelle settings

## Log Output

Agent logger nu:
```
📊 FPS:20 Q:65 Scale:100% Motion:5.2% | 45.2KB/f ~7.2Mbit/s | Buf:0.5MB | Err:0 Drop:0
```

Ved idle:
```
📊 FPS:2 Q:50 Scale:75% Motion:0.3% 💤IDLE | 8.1KB/f ~0.1Mbit/s | Buf:0.1MB | Err:0 Drop:0
```

Ved congestion:
```
📊 FPS:12 Q:50 Scale:50% Motion:15.0% | 12.1KB/f ~1.2Mbit/s | Buf:6.2MB | Err:0 Drop:3
```

## Roadmap

1. **v2.46.0** ✅ - Buffer-baseret adaptation
2. **v2.47.0** ✅ - RTT measurement + idle-mode + motion detection
3. **v2.48.0** ✅ - Input-prioritet (separat data channel)
4. **v2.48.1** ✅ - Loss/RTT-baseret adaptive streaming
5. **v2.49.0** ✅ - Full-frame refresh cadence
6. **v2.50.0** ✅ - H.264 encoder infrastructure
7. **v2.51.0** ✅ - Video track integration
8. **v2.51.1** ✅ - Hybrid mode signaling

### Skipped (ikke prioritet)
- NVENC hardware encoder (kræver NVIDIA GPU + CUDA)
- Audio streaming
