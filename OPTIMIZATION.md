# Video Encoding Optimization Guide

## Current State: JPEG Frame Streaming

**Performance**:
- Frame rate: ~10 FPS (100ms interval)
- Quality: 50% JPEG compression
- Bandwidth: ~500 KB/s
- Latency: ~150ms (LAN), ~300ms (TURN)

**Limitations**:
- High bandwidth usage
- Low frame rate
- No inter-frame compression
- Inefficient for video content

## Goal: H.264/VP8 Video Track

**Target Performance**:
- Frame rate: 30-60 FPS
- Quality: Adaptive (720p-1080p)
- Bandwidth: ~1-3 Mbps (vs current ~4 Mbps)
- Latency: ~50-100ms (LAN), ~150-250ms (TURN)

## Implementation Plan

### Phase 1: Add VP8 Support (Easier)

VP8 has better Go support via Pion and doesn't require external encoders.

#### 1.1 Install Dependencies

```powershell
go get github.com/pion/mediadevices
go get github.com/pion/mediadevices/pkg/codec/vpx
```

#### 1.2 Create Video Track

```go
// internal/webrtc/video.go
package webrtc

import (
	"github.com/pion/mediadevices"
	"github.com/pion/mediadevices/pkg/codec/vpx"
	"github.com/pion/webrtc/v3"
)

func (m *Manager) createVideoTrack() (*webrtc.TrackLocalStaticSample, error) {
	// Configure VP8 codec
	vp8Params, err := vpx.NewVP8Params()
	if err != nil {
		return nil, err
	}
	vp8Params.BitRate = 2_000_000 // 2 Mbps
	
	// Create video track
	codec := mediadevices.NewVP8Codec(vp8Params)
	track, err := webrtc.NewTrackLocalStaticSample(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8},
		"video",
		"pion",
	)
	
	return track, err
}
```

#### 1.3 Update Streaming Logic

```go
func (m *Manager) startVideoStreaming(track *webrtc.TrackLocalStaticSample) {
	ticker := time.NewTicker(33 * time.Millisecond) // ~30 FPS
	defer ticker.Stop()

	for m.isStreaming {
		<-ticker.C

		// Capture screen
		img, err := screenshot.CaptureRect(m.screenCapturer.GetBounds())
		if err != nil {
			continue
		}

		// Encode as VP8 and write to track
		// (Implementation details depend on mediadevices API)
		sample := media.Sample{
			Data:      encodedFrame,
			Duration:  time.Second / 30,
		}
		
		if err := track.WriteSample(sample); err != nil {
			log.Printf("Failed to write sample: %v", err)
		}
	}
}
```

### Phase 2: Add H.264 Support (Better Performance)

H.264 offers better compression but requires external encoder (FFmpeg or hardware encoder).

#### 2.1 Install FFmpeg

```powershell
# Option 1: Chocolatey
choco install ffmpeg

# Option 2: Manual
# Download from https://ffmpeg.org/download.html
# Add to PATH
```

#### 2.2 Use Hardware Encoding (NVIDIA NVENC / Intel Quick Sync)

```go
// internal/video/encoder.go
package video

import (
	"os/exec"
)

type H264Encoder struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
}

func NewH264Encoder(width, height, fps, bitrate int) (*H264Encoder, error) {
	// Use hardware encoder if available
	cmd := exec.Command("ffmpeg",
		"-f", "rawvideo",
		"-pix_fmt", "bgra",
		"-s", fmt.Sprintf("%dx%d", width, height),
		"-r", fmt.Sprintf("%d", fps),
		"-i", "pipe:0",
		
		// NVIDIA NVENC (if available)
		"-c:v", "h264_nvenc",
		// OR Intel Quick Sync
		// "-c:v", "h264_qsv",
		// OR software
		// "-c:v", "libx264",
		
		"-preset", "llhp", // low latency high performance
		"-tune", "zerolatency",
		"-b:v", fmt.Sprintf("%dk", bitrate),
		"-maxrate", fmt.Sprintf("%dk", bitrate),
		"-bufsize", fmt.Sprintf("%dk", bitrate*2),
		"-g", fmt.Sprintf("%d", fps), // keyframe interval
		"-f", "h264",
		"-an", // no audio
		"pipe:1",
	)
	
	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	
	return &H264Encoder{
		cmd:    cmd,
		stdin:  stdin,
		stdout: stdout,
	}, nil
}
```

#### 2.3 Alternative: Pure Go H.264 (x264-go)

```powershell
go get github.com/gen2brain/x264-go
```

```go
import "github.com/gen2brain/x264-go"

type Encoder struct {
	enc *x264.Encoder
}

func NewEncoder(width, height, fps int) (*Encoder, error) {
	opts := &x264.Options{
		Width:     width,
		Height:    height,
		FrameRate: fps,
		Tune:      "zerolatency",
		Preset:    "veryfast",
		Profile:   "baseline",
		LogLevel:  x264.LogNone,
	}
	
	enc, err := x264.NewEncoder(opts)
	if err != nil {
		return nil, err
	}
	
	return &Encoder{enc: enc}, nil
}

func (e *Encoder) Encode(img image.Image) ([]byte, error) {
	return e.enc.Encode(img)
}
```

### Phase 3: Adaptive Bitrate

```go
// internal/video/adaptive.go
package video

type AdaptiveBitrateController struct {
	currentBitrate int
	targetBitrate  int
	minBitrate     int
	maxBitrate     int
	
	packetLoss     float64
	rtt            time.Duration
}

func (a *AdaptiveBitrateController) AdjustBitrate(stats *webrtc.Stats) int {
	// Get current stats
	a.packetLoss = stats.PacketLoss
	a.rtt = stats.RTT
	
	// Increase bitrate if network is good
	if a.packetLoss < 0.01 && a.rtt < 50*time.Millisecond {
		a.currentBitrate = min(a.currentBitrate*1.1, a.maxBitrate)
	}
	
	// Decrease bitrate if network is poor
	if a.packetLoss > 0.05 || a.rtt > 200*time.Millisecond {
		a.currentBitrate = max(a.currentBitrate*0.8, a.minBitrate)
	}
	
	return int(a.currentBitrate)
}
```

## Recommended Approach

### For MVP (Fastest to implement):
1. **Keep JPEG for now** - It works!
2. Focus on other features (file transfer, security hardening)

### For Production (Best performance):
1. **Use x264-go** for pure Go H.264 encoding
2. Fallback to JPEG if encoding fails
3. Implement adaptive bitrate based on network conditions

### For Best Performance (Requires FFmpeg):
1. **Use FFmpeg with hardware encoding** (NVENC/QSV)
2. Significantly better compression and FPS
3. Lower CPU usage on agent

## Migration Strategy

### Step 1: Create Feature Flag

```go
// config/config.go
type Config struct {
	// ...
	UseVideoTrack bool   `env:"USE_VIDEO_TRACK" default:"false"`
	VideoCodec    string `env:"VIDEO_CODEC" default:"jpeg"` // jpeg, vp8, h264
	VideoBitrate  int    `env:"VIDEO_BITRATE" default:"2000"` // kbps
	VideoFPS      int    `env:"VIDEO_FPS" default:"30"`
}
```

### Step 2: Implement Side-by-Side

```go
func (m *Manager) startStreaming() {
	if m.cfg.UseVideoTrack {
		go m.startVideoTrackStreaming()
	} else {
		go m.startJPEGDataChannelStreaming()
	}
}
```

### Step 3: Test and Compare

| Metric | JPEG | VP8 | H.264 (SW) | H.264 (HW) |
|--------|------|-----|------------|------------|
| FPS | 10 | 30 | 30 | 60 |
| Bitrate | ~4 Mbps | ~2 Mbps | ~1.5 Mbps | ~1.5 Mbps |
| CPU (Agent) | 5% | 15% | 25% | 8% |
| Latency (LAN) | 150ms | 80ms | 100ms | 50ms |
| Quality | Good | Excellent | Excellent | Excellent |

### Step 4: Gradual Rollout

1. Test with `USE_VIDEO_TRACK=true` locally
2. Deploy to limited users
3. Monitor performance metrics
4. Roll out to all users
5. Remove JPEG code path

## Bandwidth Comparison

**Current (JPEG @ 10 FPS, 50% quality)**:
- Per frame: ~40-50 KB
- Per second: ~400-500 KB
- Per minute: ~24-30 MB
- Per 10 min session: ~240-300 MB

**Target (H.264 @ 30 FPS, 2 Mbps)**:
- Per second: ~250 KB
- Per minute: ~15 MB
- Per 10 min session: ~150 MB

**Savings: ~50% bandwidth reduction with 3x higher FPS!**

## Code Signing Certificate Guide

### Recommended Providers

1. **Sectigo** (formerly Comodo)
   - Price: ~$200-300/year
   - Fast issuance (1-3 days)
   - Good support

2. **DigiCert**
   - Price: ~$400-500/year
   - Premium brand
   - Best for enterprise

3. **SSL.com**
   - Price: ~$250-350/year
   - Good middle ground

### Purchase Process

1. **Choose certificate type**: Code Signing Certificate (OV)
2. **Verify identity**: Company documents required
3. **Generate CSR**: Use certreq.exe on Windows
4. **Receive certificate**: Download .pfx file
5. **Install**: Import to Windows Certificate Store

### Signing the Agent

```powershell
# Sign with timestamp
signtool sign /f cert.pfx /p PASSWORD /t http://timestamp.digicert.com /fd SHA256 remote-agent.exe

# Verify signature
signtool verify /pa remote-agent.exe

# Expected output:
# Successfully verified: remote-agent.exe
```

### Without Certificate (Development Only)

**Warning**: Windows SmartScreen will block unsigned EXEs!

Temporary workarounds:
1. Right-click → Properties → "Unblock"
2. Add exclusion in Windows Defender
3. Run from trusted location

**DO NOT DISTRIBUTE** unsigned binaries to users!

## Next Steps

1. ✅ **Automatic session cleanup** - DONE
2. ✅ **Re-enable mouse/keyboard** - DONE  
3. ⏳ **Video encoding** - Use this guide when ready
4. ⏳ **Code signing** - Purchase certificate when deploying to users

**Current Priority**: Test the working system with input control enabled, then decide if video optimization is needed based on user feedback.

---

**Last Updated**: 2025-10-02
