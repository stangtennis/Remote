# H.264 Optimization Analysis

**Dato:** 2025-12-17  
**Kilde:** Claude analyse af nuværende H.264 pipeline

## Problem: Nuværende Pipeline er Ineffektiv

Den nuværende H.264 pipeline i controlleren:

```
H.264 (RTP/WebRTC) → FFmpeg decode → MJPEG encode → Go jpeg.Decode → Fyne render
```

Dette er funktionelt, men **ikke optimalt** for:
- Latency
- CPU forbrug
- Stabil frame pacing

Især ved høj opløsning får vi "video inde i video" (re-encode), ekstra buffering og meget unødvendig arbejde.

## Løsninger (Prioriteret)

### 1) Bedste Slutløsning: Hardware Decode/Render på Controller (Ingen MJPEG)

På Windows er den mest optimale løsning at lade controlleren decode H.264 med **Media Foundation / D3D11VA** (hardware acceleration) og rendere direkte til en texture/canvas.

**Fordele:**
- Lav CPU
- Lav latency
- Stabil 60fps
- Ingen "FFmpeg-process" overhead

**Ulemper:**
- Mere arbejde at implementere (Windows-specifikt)
- Per-OS implementering hvis cross-platform

### 2) Næstbedst: FFmpeg Decode til Raw Frames (Ikke MJPEG)

Hvis vi beholder FFmpeg som backend, er det store win at **stoppe med at re-encode til MJPEG**.

```
FFmpeg decode → rawvideo (RGB/BGRA) → render direkte
```

**Fordele:**
- Mindre latency
- Mindre CPU end MJPEG-runden
- Færre "mystiske" delays

**Ulemper:**
- Skal håndtere frame størrelse/pixelformat
- Pipe-båndbredde bliver høj (men lokalt på controller-PC)

**Ekstra:** FFmpeg kan køre med GPU decode (d3d11va, cuda, qsv) på controller-PC'en.

### 3) Pragmatisk Løsning: Browser-Renderer (WebView2/Chromium)

Den mest pragmatiske "det spiller bare" løsning er at lade en embedded browser vise WebRTC video track (som en normal `<video>`):

**Fordele:**
- Hardware decode/render "gratis"
- Rigtig god timing/jitter-buffer adfærd
- Mindre Go-kode omkring decode/render

**Implementering:**
- Controlleren forbliver Go/Fyne til login/device-liste
- Når du forbinder, åbner den en "viewer" i WebView2/Chromium

## Hvordan TeamViewer og MS RDP Gør Det

### TeamViewer (Proprietær)
- Direkte UDP når muligt (lav latency), fallback til TCP/relay
- Adaptiv kvalitet/FPS/bitrate baseret på RTT, tab og CPU
- Differentieret content: tekst/UI behandles anderledes end video/3D
- Caching og delta: sender oftest kun ændringer
- Separat prioritering af input: input går på lav-latency kanal

### Microsoft RDP (Dokumenteret)
- **Orders/Graphics pipeline:** Sender "tegn denne tekst, flyt dette vindue" i stedet for pixels
- **Bitmap/glyph caching:** Gentagende elementer caches på klienten
- **Moderne codec-mode (H.264/AVC):**
  - AVC 420: klassisk video (billigere båndbredde)
  - AVC 444: bedre tekstfarver/skarpt UI
- **UDP multitransport:** UDP-kanaler til grafik + separate kanaler til clipboard, audio
- **Aggressiv pacing:** Drop frames hellere end at øge latency

## Hemmeligheden

**De re-encoder ALDRIG** - de decoder direkte til GPU/render.

Vores nuværende pipeline gør ekstra arbejde:
```
H.264 → FFmpeg → MJPEG → JPEG decode → render
```

Det er det modsatte af hvad RDP/TeamViewer prøver at opnå.

## Vigtigste Principper for "RDP/TeamViewer Feel"

1. **Ingen re-encode til MJPEG** på controlleren (decode direkte til raw/GPU)
2. **Hardware decode/render** på controlleren (Media Foundation/D3D11VA)
3. **Adaptivt:** opløsning/FPS/bitrate styres efter net + agent CPU
4. **Hybrid strategi:** pixels/video til bevægelse, "smarte updates/caching" til UI/tekst

## Praktiske Råd

- Undgå 4K/60 som standard (kræver høj bitrate)
- Brug moderne 64-bit FFmpeg build
- Mål CPU for ffmpeg.exe + app - hvis ffmpeg spiser CPU → decode/encode-kæden er problemet

## Valgt Løsning

**Løsning #1: Hardware Decode/Render med Media Foundation/D3D11VA**

Dette giver størst effekt på både latency og smoothness.
