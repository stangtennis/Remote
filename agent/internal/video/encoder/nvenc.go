package encoder

import (
	"bytes"
	"fmt"
	"image"
	"io"
	"log"
	"os/exec"
	"runtime"
	"sync"
	"time"
)

// NVENCEncoder implements H.264 encoding using FFmpeg's NVENC backend.
// It uses a background goroutine to read FFmpeg stdout continuously,
// preventing the Encode() call from blocking on pipe reads.
type NVENCEncoder struct {
	config  Config
	mu      sync.Mutex
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stdout  io.ReadCloser
	stderr  *bytes.Buffer
	started bool

	// Background reader goroutine sends encoded chunks here
	outCh   chan []byte
	errCh   chan error
	stopCh  chan struct{}
}

// NewNVENCEncoder creates a new NVENC encoder
func NewNVENCEncoder() *NVENCEncoder {
	return &NVENCEncoder{}
}

// IsNVENCAvailable checks if NVENC encoding is available
func IsNVENCAvailable() bool {
	if runtime.GOOS != "windows" && runtime.GOOS != "linux" {
		return false
	}

	// Check if ffmpeg is available
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return false
	}

	// Probe for NVENC support
	cmd := exec.Command("ffmpeg", "-hide_banner", "-encoders")
	out, err := cmd.Output()
	if err != nil {
		return false
	}

	return bytes.Contains(out, []byte("h264_nvenc"))
}

// Init initializes the NVENC encoder via FFmpeg subprocess
func (e *NVENCEncoder) Init(cfg Config) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !IsNVENCAvailable() {
		return fmt.Errorf("NVENC not available (ffmpeg or h264_nvenc not found)")
	}

	e.config = cfg
	// Cap bitrate at 4 Mbps for testing - high bitrates cause packetization issues
	// CBR mode instead of VBR to prevent bitrate spikes
	if e.config.Bitrate > 4000 {
		log.Printf("NVENC: capping bitrate %d -> 4000 kbps CBR for stable H.264 streaming", e.config.Bitrate)
		e.config.Bitrate = 4000
	}
	return e.startFFmpeg()
}

func (e *NVENCEncoder) stopProcess() {
	if e.stopCh != nil {
		close(e.stopCh)
		e.stopCh = nil
	}
	if e.stdin != nil {
		e.stdin.Close()
		e.stdin = nil
	}
	if e.cmd != nil && e.cmd.Process != nil {
		e.cmd.Process.Kill()
		e.cmd.Wait()
		e.cmd = nil
	}
}

func (e *NVENCEncoder) startFFmpeg() error {
	// Clean up previous instance
	e.stopProcess()

	// FFmpeg command: read raw RGBA from stdin, encode to H.264 NVENC, output raw H.264 to stdout
	//
	// QUALITY-TUNED for desktop content (tekst, skarpe kanter):
	//   - profile high (i stedet for baseline) → bedre encoding-kvalitet
	//     ved samme bitrate; alle moderne browsere understøtter High.
	//   - preset p4 (medium) — endnu lav-latency på GPU men højere kvalitet
	//     end p1 (fastest); kun ~1ms ekstra encode-tid på GTX 1060.
	//   - VBR-rate control med 2x maxrate-headroom — lader bitrate stige
	//     midlertidigt under hurtige scene-changes (mindre kompressions-
	//     artefakter ved bevægelse), falder igen ved statisk indhold.
	//   - color_range tv + colorspace bt709 — eksplicit BT.709 (sRGB-mapped)
	//     i stedet for FFmpeg default BT.601. Fjerner farve-shift hvor
	//     rød/grøn blev "lidt off" på tekst og UI-elementer.
	//
	// LATENCY-CRITICAL flags bibeholdt:
	//   - tune ull (ultra-low latency) — 0 lookahead, 0 reorder
	//   - bf 0 — ingen B-frames
	//   - forced-idr — tving IDR ved keyframe-interval
	//   - dump_extra=freq=keyframe — repeat SPS+PPS før hver IDR (KRITISK
	//     for WebRTC: browser-decoder kan ellers ikke initialisere efter
	//     pakketab eller late-join → black screen)
	args := []string{
		"-hide_banner", "-loglevel", "error",
		"-f", "rawvideo",
		"-pix_fmt", "rgba",
		"-s", fmt.Sprintf("%dx%d", e.config.Width, e.config.Height),
		"-r", fmt.Sprintf("%d", e.config.Framerate),
		"-i", "pipe:0",
		"-c:v", "h264_nvenc",
		"-preset", "p4",          // Medium kvalitet (var p1=fastest); ~1ms ekstra
		"-tune", "ull",           // Ultra-low latency (no lookahead/reorder)
		"-rc", "cbr",             // Constant bitrate - NO spikes, stable packet size
		"-b:v", fmt.Sprintf("%dk", e.config.Bitrate),
		"-maxrate", fmt.Sprintf("%dk", e.config.Bitrate), // Same as bitrate for true CBR
		"-bufsize", fmt.Sprintf("%dk", e.config.Bitrate*2), // Larger buffer for smoothness
		"-profile:v", "high",     // High profile (4:2:0 men fuld H.264 feature set)
		"-g", fmt.Sprintf("%d", e.config.KeyframeInterval),
		"-bf", "0",               // No B-frames for low latency
		"-forced-idr", "1",
		"-spatial_aq", "1",       // Adaptive quantization PÅ — bedre kvalitet på flade områder
		"-temporal_aq", "1",      // Temporal AQ — bedre kvalitet ved bevægelse
		"-rc-lookahead", "0",     // No lookahead = no latency tradeoff
		"-color_range", "tv",
		"-colorspace", "bt709",   // BT.709 (sRGB-mapped) i stedet for FFmpeg default BT.601
		"-color_primaries", "bt709",
		"-color_trc", "bt709",
		"-flags", "+low_delay",
		"-flush_packets", "1",    // Flush stdout efter HVER frame (forhindrer partial reads)
		"-bsf:v", "dump_extra=freq=keyframe",
		"-f", "h264",
		"pipe:1",
	}

	e.cmd = exec.Command("ffmpeg", args...)

	var err error
	e.stdin, err = e.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}

	e.stdout, err = e.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}

	// Capture stderr for diagnostics
	e.stderr = &bytes.Buffer{}
	e.cmd.Stderr = e.stderr

	if err := e.cmd.Start(); err != nil {
		return fmt.Errorf("ffmpeg start: %w", err)
	}

	// Start background reader goroutine
	e.outCh = make(chan []byte, 4)   // Small buffer so reader doesn't block
	e.errCh = make(chan error, 1)
	e.stopCh = make(chan struct{})
	go e.readLoop()

	e.started = true
	log.Printf("✅ NVENC encoder started (FFmpeg PID: %d, %dx%d @ %d kbps)",
		e.cmd.Process.Pid, e.config.Width, e.config.Height, e.config.Bitrate)

	return nil
}

// readLoop continuously reads from FFmpeg stdout in a background goroutine.
// Each Read() may return partial data; we send whatever we get to outCh.
// With tune=ull and no B-frames, FFmpeg outputs one access unit per input frame.
func (e *NVENCEncoder) readLoop() {
	buf := make([]byte, 512*1024) // 512KB read buffer
	for {
		n, err := e.stdout.Read(buf)
		if n > 0 {
			data := make([]byte, n)
			copy(data, buf[:n])
			select {
			case e.outCh <- data:
			case <-e.stopCh:
				return
			}
		}
		if err != nil {
			select {
			case e.errCh <- err:
			default:
			}
			return
		}
	}
}

// Encode encodes an RGBA frame to H.264 NAL units via NVENC
func (e *NVENCEncoder) Encode(frame *image.RGBA, forceKeyframe bool) ([]byte, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.started || e.stdin == nil {
		return nil, fmt.Errorf("encoder not started")
	}

	// Check if frame dimensions changed
	bounds := frame.Bounds()
	if bounds.Dx() != e.config.Width || bounds.Dy() != e.config.Height {
		e.config.Width = bounds.Dx()
		e.config.Height = bounds.Dy()
		log.Printf("NVENC: resolution changed to %dx%d, restarting ffmpeg", e.config.Width, e.config.Height)
		if err := e.startFFmpeg(); err != nil {
			return nil, err
		}
	}

	// Write raw RGBA frame to FFmpeg stdin
	_, err := e.stdin.Write(frame.Pix)
	if err != nil {
		stderrMsg := ""
		if e.stderr != nil && e.stderr.Len() > 0 {
			stderrMsg = e.stderr.String()
			log.Printf("NVENC FFmpeg stderr: %s", stderrMsg)
		}
		return nil, fmt.Errorf("write frame: %w (stderr: %s)", err, stderrMsg)
	}

	// Wait for encoded output from the background reader.
	// With tune=ull, FFmpeg should produce output within a few ms after receiving a frame.
	// We collect all available chunks within the timeout window.
	var result []byte
	timeout := time.After(100 * time.Millisecond)

	// Wait for first chunk (blocking with timeout)
	select {
	case data := <-e.outCh:
		result = data
	case err := <-e.errCh:
		return nil, fmt.Errorf("ffmpeg read error: %w", err)
	case <-timeout:
		return nil, nil // No output yet (first few frames may buffer)
	}

	// Give a short pause for any remaining data to arrive (FFmpeg may write
	// a frame in multiple pipe writes even with flush_packets=1)
	time.Sleep(2 * time.Millisecond)

	// Drain any additional chunks that are already available (non-blocking)
	for {
		select {
		case data := <-e.outCh:
			result = append(result, data...)
		default:
			return result, nil
		}
	}
}

// SetBitrate adjusts the encoding bitrate (requires FFmpeg restart)
func (e *NVENCEncoder) SetBitrate(kbps int) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	oldBitrate := e.config.Bitrate
	e.config.Bitrate = kbps

	// Restart FFmpeg with new bitrate for immediate effect
	if e.started && oldBitrate != kbps {
		log.Printf("NVENC: bitrate changed %d -> %d kbps, restarting encoder", oldBitrate, kbps)
		return e.startFFmpeg()
	}
	return nil
}

// Close releases encoder resources
func (e *NVENCEncoder) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.started = false
	e.stopProcess()
	log.Println("NVENC encoder closed")
	return nil
}

// Name returns the encoder name
func (e *NVENCEncoder) Name() string {
	return "nvenc"
}
