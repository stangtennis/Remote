# Bandwidth og CPU-optimering (agent streaming)

**Dato:** 2025-12-19  
**Kilde:** Codex analyse af nuvaerende pipeline

## Maal (balanceret)

- Standard budget: 3-8 Mbit/s (typisk WAN)
- Lav CPU paa billig hardware, men stadig brugbart billede
- Skarp tekst ved statisk desktop (idle)
- Stabil inputfoelelse og lav latency
- Robust fallback ved hoej RTT, tab, eller CPU-spikes

## Non-goals

- Ingen schema- eller signaling-aendringer i Supabase
- Ingen stor UI-ombygning i controlleren
- Ingen hard afhaengighed af GPU-encode (men forberedt)
- Ingen garanti for perfekt video ved gaming (maal er fjernarbejde)

## Nuvaerende pipeline (observationer fra kode)

Primaer loop ligger i `agent/internal/webrtc/peer.go`:

- Capture: `CaptureRGBA()` hver tick
- Motion detection: dirty regions + `GetChangePercentage(...)`
- Adaptive loop: justerer `fps`, `quality`, `scale` ud fra
  - `BufferedAmount`
  - `lossPct`, `RTT`
  - CPU status
- Idle-mode: `fps=2`, `quality=85`, `scale=1.0` ved lav motion
- Frame skipping: spring encode/sending over ved `motionPct < 0.1`
- Full-frame refresh: hver ~5s eller ved hoej motion
- H.264: video track anvendes hvis aktiv, fallback ved hoej CPU/RTT
- Stats: sender `fps/quality/scale/mode/rtt/cpu` til controller

Nuvaerende default graenser (i loop):

- `minFPS=12`, `maxFPS=30`
- `minQuality=50`, `maxQuality=80`
- `minScale=0.5`, `maxScale=1.0`
- Idle: `fps=2`, `quality=85`, `scale=1.0`

## Flaskehalse (CPU + baandbredde)

CPU-tunge trin:

1) RGBA-capture i fuld oploesning hver tick
2) Dirty-region detection paa fuld frame
3) JPEG encoding (tiles)
4) H.264 encoding (naar aktiv)

Bandbredde-tunge trin:

- Fulde JPEG-frames ved bevaegelse
- Datachannel chunking overhead og retransmits
- Hyppige full-frame refreshes ved lav motion

## Anbefalet strategi: adaptiv hybrid med CPU-budget

Behold hybrid, men lad en simpel mode-model styre adfaerd. Default er
idle-tiles, og H.264 aktiveres kun naar CPU/RTT/loss er stabile.

### Mode 1: Idle Tiles (default)

**Trigger:** `motionPct < 0.3%` og ingen input i > 1s

**Parametre:**
- FPS: 1-2
- Quality: 80-90
- Scale: 1.0
- Full-frame cadence: 8-10s
- Encoder: JPEG tiles

**Effekt:** Minimal CPU/bw, skarp tekst ved lav bevaegelse.

### Mode 2: Active Tiles

**Trigger:** input indenfor 250ms eller `motionPct >= 1%`

**Parametre:**
- FPS: 12-24
- Quality: 60-75
- Scale: 0.85-1.0
- Encoder: JPEG tiles

**Effekt:** Stabil brugerfoelelse, lavere CPU end H.264.

### Mode 3: Active H.264

**Trigger:** CPU < 70-75% (stabilt), RTT < 200ms, loss < 3%

**Parametre:**
- FPS: 20-30
- Target bitrate cap: 3-8 Mbit/s
- Encoder: H.264 video track

**Fallback:** CPU > 85% eller RTT > 300ms -> tilbage til tiles.

### Skiftelogik (med hysterese)

- Mode skifter kun efter stabilt vindue (fx 2-3 sekunders sliding window)
- Skift ned (til tiles) sker hurtigere end skift op (til H.264)
- Undgaa flapping ved at indfoere minimums-tid i hver mode (fx 2s)

Pseudo-logik:

```
if idle && motionPct < 0.3% && timeSinceInput > 1s:
    mode = idle_tiles
else if cpuOk && rttOk && lossOk:
    mode = h264
else:
    mode = active_tiles
```

## CPU-optimering (detaljer)

1) **Gate full capture i idle**
   - I idle-mode: brug en let motion-probe (downscale/sampling) pr. tick
   - Kun hvis motion over taerskel -> fuld RGBA capture + encode

2) **Billigere motion-probe**
   - Downscale til fx 1/4 eller 1/8
   - Eller sample 32x32 grid af pixels
   - Koer fuld dirty-region detection kun ved mistanke om aendring

3) **Drop foer encode**
   - Hvis `BufferedAmount` er hoej eller CPU hoej: skip encode tidligt
   - Sparer CPU sammenlignet med at encode og saa droppe

4) **Backoff paa DXGI timeouts**
   - Ved gentagne "no new frame": oeg sleep interval gradvist
   - Undgaa spildt arbejde naar skaermen er helt statisk

5) **H.264 CPU-budget**
   - Krav: CPU under 70-75% stabilt for at aktivere H.264
   - Force-off ved CPU > 85% eller spikes over 90%

## Baandbredde-optimering (detaljer)

1) **Skala foerst, kvalitet bagefter**
   - Hvis budget overskrides: saenk `scale` (1.0 -> 0.85 -> 0.7)
   - Kvalitet holdes moderat for bedre laeselighed

2) **FPS bundet til motion**
   - Idle: 1-2 FPS
   - Aktivt arbejde: 12-24 FPS
   - H.264: 20-30 FPS, men under bitrate cap

3) **Full-frame cadence dynamisk**
   - Idle: 8-10s
   - Aktivt: 3-5s
   - Hoej motion: full frames efter behov

4) **Region updates fremfor fuld frame**
   - Send dirty regions hyppigt
   - Fulde frames sjaeldnere ved lav motion

5) **Chunk sizing og drop**
   - Ved hoej RTT/loss: mindre chunks
   - Drop ufuldstaendige frames hurtigt (latest-frame-wins)

## Konkrete startparametre (foreslaaet)

- Idle threshold: 0.3% motion
- Active threshold: 1.0% motion
- Idle enter delay: 1s
- Idle exit delay: 200-300ms
- H.264 enable: CPU < 75% i 3s, RTT < 200ms, loss < 3%
- H.264 disable: CPU > 85% eller RTT > 300ms
- Frame skip: `motionPct < 0.1` (behold)

## Metrics og observabilitet

Udover eksisterende stats anbefales:

- Moving average af CPU, RTT, loss (2-3s vindue)
- Mode-switch events (log + HUD)
- Bytes sent per mode (tiles vs H.264)

Eksisterende loglinje viser allerede: FPS, quality, scale, motion, RTT,
loss, CPU, buffer og send Mbps. Det er tilstraekkeligt til tuning.

## Test og tuning (hurtig matrix)

1) **Statisk desktop (IDE/Docs)**
   - Maal: 1-3 Mbit/s, CPU < 10-15% paa billig laptop

2) **Normal arbejde (scroll/drag)**
   - Maal: 3-8 Mbit/s, CPU < 30-40%

3) **Tung motion (video/anim)**
   - Maal: H.264 hvis muligt, ellers tiles med lavere scale

## Risici og afboedning

- **Flapping mellem modes**: brug hysterese + minimumstid pr. mode
- **Tekst bliver utydelig**: prioriter scale foer quality
- **Stalls fra chunk loss**: drop ufuldstaendige frames hurtigt
- **CPU spikes**: early-drop foer encode + H.264 off

## Touchpoints i kode (hvis analysen implementeres)

- `agent/internal/webrtc/peer.go`
  - Mode-styring, motion probe, capture gating, adaptive loop
- `controller/internal/webrtc/client.go`
  - Drop ufuldstaendige frames, stream params
- `docs/js/webrtc.js` (hvis web viewer)
  - Chunk drop og bandwidth display

---

**Konklusion:** Den mest robuste vej er en adaptiv hybrid med
CPU-budget og tydelig mode-model. Default idle-tiles giver lav CPU og
lavt bandwidth, mens H.264 kun aktiveres paa maskiner og net der kan
baere det. Dette matcher kravet om minimal belastning og stabil oplevelse
paa tvaers af billig laptop og stationaer.
