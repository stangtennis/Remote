# Adaptive Streaming (v2.46.0)

Implementeret adaptiv streaming der justerer kvalitet baseret på netværksforhold.

## Parametre

| Parameter | Min | Max | Default | Beskrivelse |
|-----------|-----|-----|---------|-------------|
| FPS | 12 | 30 | 20 | Frames per second |
| Quality | 50 | 80 | 65 | JPEG kvalitet |
| Scale | 50% | 100% | 100% | Skalering af opløsning |

## Adaptation Logic

### Reducer kvalitet når:
- Buffer > 8 MB (netværk congested)
- Ændringer: FPS -4, Scale -10%, Quality -5

### Øg kvalitet når:
- Buffer < 1 MB OG ingen dropped frames
- Ændringer: Quality +2, Scale +5%, FPS +2

### Drop frames når:
- Buffer > 16 MB (kritisk congestion)

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
📊 FPS:20 Q:65 Scale:100% | 45.2KB/f ~7.2Mbit/s | Buf:0.5MB | Err:0 Drop:0
```

Ved congestion:
```
📊 FPS:12 Q:50 Scale:50% | 12.1KB/f ~1.2Mbit/s | Buf:6.2MB | Err:0 Drop:3
```

## Næste skridt

Se `H264_IMPLEMENTATION_PLAN.md` for H.264 video track implementation.
