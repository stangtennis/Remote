# H.264 Integrationsplan for Remote Desktop Agent (Go + Pion)

## Hvor vi står i dag (agent-side)
- **Transport**: Kun WebRTC data channel (`data`) + signaling via Supabase REST polling.
- **Video**: JPEG-billeder over data channel. Ingen WebRTC medietracks, ingen RTP.
- **Capture**: `screen.Capturer` med DXGI→JPEG (desktop) eller GDI (Session 0). Ingen skalering før encode.
- **Dirty regions**: Struktur findes (`DirtyRegionDetector`), men send-loop i `startScreenStreaming` bruger altid fulde JPEG-frames.
- **Input**: Mouse/keyboard-events via data channel; relative koordinater understøttes (`rel` flag).
- **Audio**: Ingen.

## Mål for H.264
- Hardware-accelereret H.264 (NVENC/QuickSync/AMF, fallback software) for lav båndbredde.
- Behold data channel til kontrol, clipboard, filoverførsel og evt. dirty-tiles/foveated spot tiles.
- Adaptiv bitrate/fps/skalering baseret på RTT, packet loss og buffer-tryk.
- Mulighed for fallback til JPEG/tiles-only, hvis encoder/transport ikke er tilgængelig.

## Arkitektur (foreslået hybrid)
- **Video track**: WebRTC video (H.264) til baseline kontinuerlig stream.
- **Tiles/foveated**: Valgfrit data channel-lag til små høj-kvalitets tiles omkring cursor/tekst.
- **Control channel**: Separat data channel (`ordered=false`, `maxRetransmits=0`) til input med prioritet.
- **Signaling**: Udvid Supabase session-felter med profil:
  - `mode`: `hybrid` | `h264-only` | `tiles-only`
  - `max_bitrate_kbps`, `min_bitrate_kbps`, `target_fps`, `max_scale_down`, `tile_size`, `foveated`.

## Implementeringsplan (opdelt i faser)
1) **Signaling-kontrakt**
   - Udvid session DTO i controller/dashboard + agent til at læse ovenstående felter.
   - Default: `mode=hybrid`, `max_bitrate_kbps=4000`, `target_fps=30`, `max_scale_down=0.5`.

2) **Capture + skalering**
   - Tilføj skalering i capture-pipelinen (Lanczos/Bilinear) før encode, styret af `state.scale`.
   - Eksponer aktuel opløsning til controller (send `resolution`-event ved init/reinit).

3) **H.264-encoderlag**
   - Introducér `video/encoder` package med hardware-first, software fallback.
   - API: `Init(config)`, `Encode(frame) -> RTP packets`, `SetBitrate(kbps)`, `ForceKeyframe()`.
   - Start med software (pion/mediadevices + x264) hvis hardware ikke er klar; udskift til NVENC/QuickSync når deps er på plads.

4) **WebRTC-track**
   - Opret video-transceiver (`AddTransceiverFromKind(webrtc.RTPCodecTypeVideo, ...H264 payload type...)`).
   - Hook encoder output til Pion `TrackLocalStaticRTP` eller custom `TrackLocal`.
   - Keyframe-interval 2–3 s, force ved scene change/motion > 25 %.

5) **Hybrid send-loop**
   - Beslut per tick (30–60 ms) om der skal sendes H.264 frame vs. dirty tiles:
     - Høj motion eller lav skalering → H.264 frame.
     - Lav motion → tiles/foveated i data channel.
   - Full-frame tile fallback hver 3–5 s eller hvis >30 % ændret.

6) **Adaptiv stige (QoS)**
   - Mål: RTT, packet loss, `BufferedAmount`, send-bps.
   - Regulér: `fps (12–30)`, `scale (0.5–1.0)`, `jpegQ (50–85)`, `h264_bitrate (500–4000 kbps)`.
   - Idle-mode: motion <1 % → fps 1–2, Q lav, scale 0.75; exit ved aktivitet.

7) **Foveated tiles**
   - På controller: send cursor-pos (relativ) med input.
   - På agent: hvis cursor flyttet inden for 300 ms, send høj-kvalitets tile (radius 128–192 px, Q=80–85) omkring cursor via data channel.

8) **Input-prioritet**
   - Separat data channel `control`, `ordered=false`, `maxRetransmits=0`.
   - Hvis backlog > 8–16 MB, pausér tile/full-frame send indtil kø er lav.

9) **Fallbacks**
   - Hvis encoder init fejler → `tiles-only` mode.
   - Hvis TURN TCP/RTT >300 ms → sænk fps/scale og kør tiles-only.

10) **Test- og måleplan**
    - Scenarier: LAN/UDP, WAN/UDP, TURN/UDP, TURN/TCP.
    - Mål: båndbredde (Mbps), RTT, packet loss, fps, subjektiv skarphed.
    - Log adapt-beslutninger i agent-log + UI visning i controller.

## Kodeberøringer (stier)
- `agent/internal/webrtc/peer.go`: Ny video track, adapt-loop, send-loop split (H.264 + tiles), resolution events.
- `agent/internal/screen/*`: Skalering før encode, optional RGBA for tiles.
- `agent/internal/webrtc/signaling.go`: Session-profiler/felter.
- `agent/internal/input/mouse.go`: Behold relative input; ingen ændring.
- `controller`/`dashboard`: UI presets + videregiv session-profiler, modtag resolution-event, anvend relative input (allerede understøttet).

## Pseudokode (kerner)
**Adapt tick**
```go
func AdaptTick(m Metrics) {
  if m.bufBytes > 16<<20 || m.loss > 5 || m.RTT > 250*time.Millisecond {
    fps = clamp(fps-5, 12, 30)
    scale = clamp(scale-0.1, 0.5, 1.0)
    jpegQ = clamp(jpegQ-5, 50, 85)
    h264 = max(h264-250, 500)
  } else if m.loss < 1 && m.RTT < 120*time.Millisecond && m.bufBytes < 4<<20 {
    fps = clamp(fps+5, 12, 30)
    scale = clamp(scale+0.1, 0.5, 1.0)
    jpegQ = clamp(jpegQ+5, 50, 85)
    h264 = min(h264+250, 4000)
  }
}
```

**Hybrid send**
```go
for range ticker {
  motion := detector.MotionPct()
  if mode == "h264-only" || motion > 20 || scale < 0.8 {
    sendH264Frame(scale, bitrate, forceKeyframeIfNeeded(motion))
  } else {
    sendTiles(detector, jpegQ, scale)
  }
  every(3s) { sendIFrame() }
}
```

**Foveated tile**
```go
if cursorMovedRecently() {
  tile := cutRegion(frame, cx-radius, cy-radius, r*2, r*2)
  sendTile(tile, qualityHigh)
}
```

## CPU-only (ingen GPU) strategi
- Brug software-encoder: `x264` med CLI (`ffmpeg -f rawvideo ... -c:v libx264`) eller Go-binding (pion/mediadevices + x264/openh264).
- Flags til lav latency: `-preset veryfast/superfast` (afhængig af CPU), `-tune zerolatency`, `-g 60`, `-keyint_min 30`, `-bf 0`, `-profile baseline/high`.
- Skaler ned for CPU-budget: start 1280x720 @ 24–30 fps; ved høj CPU → 0.75x/0.5x og 15–20 fps.
- Begræns bitrate til 1–3 Mbps default; hæv kun hvis CPU load < 70 %.
- Behold tiles/foveated som hovedvej for skarp tekst når H.264 er grovere/skaleret.
- Måling: log CPU-usage (process) pr. 5 s og sænk fps/scale når CPU > 85 % i 3 målinger.

## Afhængigheder/notes
- Hardware encode kræver adgang til NVENC/QuickSync/AMF; planlæg fallback til software for bred kompatibilitet (se CPU-only sektionen).
- TURN bør fortrinsvis være UDP; TCP fallback giver høj RTT → brug tiles-only i den case.
- Sørg for at relative input (`rel=true`) bruges i controller for korrekt mapping ved skalering.

## Prioriteret leveranceordre
1) Signaling-profiler + controller/dash UI presets.
2) Skalering + resolution event + adapt-loop skeleton.
3) Tiles batching + foveated tile på cursor (forbedrer kvalitet med det samme).
4) H.264 track (software først) + bitrate styring + keyframes.
5) Hardware-encoder integration + kvalitet/bitrate tuning.
6) End-to-end testmatrix og log/metrics visning.

## Succes-kriterier (måleligt)
- 1080p @ 30 fps under 3–4 Mbps på LAN/WAN med acceptabel skarphed.
- Input-latens < 80 ms LAN, < 150 ms typisk WAN.
- Tekst/UI skarp (foveated/høj-Q tiles) ved 0.5–0.75x skalering.
- Stabil fallback (tiles-only) når encoder eller net ikke kan følge med.
