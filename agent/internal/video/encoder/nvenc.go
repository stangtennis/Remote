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
	pending []byte
}

const (
	nvencFirstChunkTimeout = 120 * time.Millisecond
	nvencMaxDrainDuration  = 90 * time.Millisecond
)

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
	if e.config.Bitrate > 50000 {
		log.Printf("NVENC: capping bitrate %d -> 50000 kbps", e.config.Bitrate)
		e.config.Bitrate = 50000
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
	//   - profile baseline — lidt mindre effektiv kompression end High,
	//     men langt mere robust i browser-dekodere ved store UI-skift
	//     som Start-menuen i dashboardets native browser-H.264 pipeline.
	//   - preset p4 (medium) — endnu lav-latency på GPU men højere kvalitet
	//     end p1 (fastest); kun ~1ms ekstra encode-tid på GTX 1060.
	//   - CBR ved eksplicit klient-bitrate — holder browser/TURN path
	//     forudsigelig. Dashboard sender 10 Mbps fra v3.1.91, så agenten
	//     ikke bliver stående på init-default 16 Mbps i browser-H.264.
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
		"-preset", "p4", // Medium kvalitet (var p1=fastest); ~1ms ekstra
		"-tune", "ull", // Ultra-low latency (no lookahead/reorder)
		"-rc", "cbr", // Constant bitrate for stable quality
		"-b:v", fmt.Sprintf("%dk", e.config.Bitrate),
		"-maxrate", fmt.Sprintf("%dk", e.config.Bitrate),
		"-bufsize", fmt.Sprintf("%dk", e.config.Bitrate*3),
		"-profile:v", "baseline", // Browser-safe profile for dashboard H.264 decode
		"-g", fmt.Sprintf("%d", e.config.KeyframeInterval),
		"-bf", "0", // No B-frames for low latency
		"-forced-idr", "1",
		"-aud", "1", // Emit access unit delimiters so frame boundaries are parseable.
		"-spatial_aq", "1", // Adaptive quantization PÅ — bedre kvalitet på flade områder
		"-temporal_aq", "1", // Temporal AQ — bedre kvalitet ved bevægelse
		"-rc-lookahead", "0", // No lookahead = no latency tradeoff
		"-color_range", "tv",
		"-colorspace", "bt709", // BT.709 (sRGB-mapped) i stedet for FFmpeg default BT.601
		"-color_primaries", "bt709",
		"-color_trc", "bt709",
		"-flags", "+low_delay",
		"-flush_packets", "1", // Flush stdout efter HVER frame (forhindrer partial reads)
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
	e.outCh = make(chan []byte, 32) // Large enough for bursty keyframes
	e.errCh = make(chan error, 1)
	e.stopCh = make(chan struct{})
	e.pending = nil
	go e.readLoop()

	e.started = true
	log.Printf("✅ NVENC encoder started (FFmpeg PID: %d, %dx%d @ %d kbps)",
		e.cmd.Process.Pid, e.config.Width, e.config.Height, e.config.Bitrate)

	return nil
}

// readLoop continuously reads from FFmpeg stdout in a background goroutine.
// Each Read() may return partial data; Encode() reassembles the burst before
// writing one WebRTC sample. This matters for browser decoders: sending a
// partial H.264 access unit can smear/corrupt the lower part of the frame.
func (e *NVENCEncoder) readLoop() {
	buf := make([]byte, 1024*1024)
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

	if au := e.popAccessUnitLocked(); len(au) > 0 {
		return au, nil
	}

	// Wait for first chunk (blocking with timeout)
	select {
	case data := <-e.outCh:
		e.pending = append(e.pending, data...)
	case err := <-e.errCh:
		return nil, fmt.Errorf("ffmpeg read error: %w", err)
	case <-time.After(nvencFirstChunkTimeout):
		return nil, ErrNoFrameReady // First few frames may buffer.
	}

	deadline := time.NewTimer(nvencMaxDrainDuration)
	defer deadline.Stop()

	for {
		if au := e.popAccessUnitLocked(); len(au) > 0 {
			return au, nil
		}

		select {
		case data := <-e.outCh:
			e.pending = append(e.pending, data...)
		case err := <-e.errCh:
			if len(e.pending) > 0 {
				log.Printf("NVENC: returning buffered H.264 after read error: %v", err)
				result := e.pending
				e.pending = nil
				return result, nil
			}
			return nil, fmt.Errorf("ffmpeg read error: %w", err)
		case <-deadline.C:
			if len(e.pending) == 0 {
				return nil, ErrNoFrameReady
			}
			// Fail open if FFmpeg does not emit AUD despite -aud 1. This keeps
			// video alive, but the normal path above is delimiter-based.
			result := e.pending
			e.pending = nil
			log.Printf("NVENC: H.264 access-unit deadline hit, returning %d bytes", len(result))
			return result, nil
		}
	}
}

func (e *NVENCEncoder) popAccessUnitLocked() []byte {
	au, rest := popAnnexBAccessUnit(e.pending)
	if len(au) == 0 {
		return nil
	}
	e.pending = rest
	return au
}

func popAnnexBAccessUnit(data []byte) ([]byte, []byte) {
	if len(data) < 6 {
		return nil, data
	}

	firstStart := findStartCode(data, 0)
	if firstStart < 0 {
		if len(data) > 3 {
			return nil, append([]byte(nil), data[len(data)-3:]...)
		}
		return nil, data
	}
	if firstStart > 0 {
		data = data[firstStart:]
	}

	secondAUD := -1
	seenAUD := false
	for pos := 0; ; {
		start := findStartCode(data, pos)
		if start < 0 {
			break
		}
		nalStart := start + startCodeLen(data[start:])
		if nalStart >= len(data) {
			break
		}
		if data[nalStart]&0x1f == 9 {
			if seenAUD {
				secondAUD = start
				break
			}
			seenAUD = true
		}
		pos = nalStart + 1
	}

	if secondAUD < 0 {
		return nil, data
	}
	return append([]byte(nil), data[:secondAUD]...), append([]byte(nil), data[secondAUD:]...)
}

func findStartCode(data []byte, from int) int {
	for i := from; i+3 < len(data); i++ {
		if data[i] == 0 && data[i+1] == 0 {
			if data[i+2] == 1 {
				return i
			}
			if i+4 <= len(data) && data[i+2] == 0 && data[i+3] == 1 {
				return i
			}
		}
	}
	return -1
}

func startCodeLen(data []byte) int {
	if len(data) >= 4 && data[0] == 0 && data[1] == 0 && data[2] == 0 && data[3] == 1 {
		return 4
	}
	return 3
}

// SetBitrate records the requested bitrate without restarting FFmpeg.
//
// FFmpeg/NVENC does not accept this bitrate change through the current stdin
// pipeline. Restarting here causes a visible keyframe burst and short capture
// stall on every dashboard quality change, so the new value takes effect on the
// next encoder restart, e.g. a resolution change or new session.
func (e *NVENCEncoder) SetBitrate(kbps int) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	oldBitrate := e.config.Bitrate
	e.config.Bitrate = kbps

	if e.started && oldBitrate != kbps {
		log.Printf("NVENC: bitrate requested %d -> %d kbps (effective on next encoder restart)", oldBitrate, kbps)
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
