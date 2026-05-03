//go:build darwin

package encoder

import (
	"fmt"
	"image"
	"io"
	"log"
	"os/exec"
	"sync"
)

// VideoToolboxEncoder implementer H.264 encoding via Apples VideoToolbox
// (hardware-accelereret encoder på alle Mac'er — både Intel og Apple Silicon).
//
// Tidligere brugte vi kun OpenH264 software-encoder på macOS, hvilket
// hammede CPU'en (80-95% på Intel-MacBook ved 1280x800@30fps). VideoToolbox
// flytter encoding til AMD/Intel/ASIC-blokken og frigør CPU. Forventet
// CPU-besparelse: 60-80% på samme indhold.
//
// Bruger samme FFmpeg subprocess-pattern som NVENC for konsistens. RGBA
// in på stdin → H.264 NAL-units ud på stdout.
type VideoToolboxEncoder struct {
	config  Config
	mu      sync.Mutex
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stdout  io.ReadCloser
	buf     []byte
	started bool
}

func NewVideoToolboxEncoder() *VideoToolboxEncoder {
	return &VideoToolboxEncoder{}
}

// IsVideoToolboxAvailable returns true hvis FFmpeg + h264_videotoolbox
// er tilgængelig. På macOS er ffmpeg typisk installeret via brew eller
// bundlet i agent-pakken.
func IsVideoToolboxAvailable() bool {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return false
	}
	cmd := exec.Command("ffmpeg", "-hide_banner", "-encoders")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return containsBytes(out, []byte("h264_videotoolbox"))
}

func (e *VideoToolboxEncoder) Init(cfg Config) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !IsVideoToolboxAvailable() {
		return fmt.Errorf("VideoToolbox not available (ffmpeg or h264_videotoolbox not found)")
	}
	e.config = cfg
	return e.startFFmpeg()
}

func (e *VideoToolboxEncoder) startFFmpeg() error {
	if e.cmd != nil {
		e.cmd.Process.Kill()
		e.cmd.Wait()
	}

	// FFmpeg pipeline: rawvideo (RGBA) → h264_videotoolbox → annex-B NAL
	//
	// VideoToolbox-specifikke flags:
	//   - allow_sw 0 — kræv hardware accel (fail hvis ikke tilgængeligt)
	//   - realtime 1 — reduceret latency (no lookahead)
	//   - profile high — fuld H.264 feature set
	//   - dump_extra=keyframe (KRITISK): repeat SPS+PPS før hver IDR
	//     så browser-decoder kan altid initialisere
	//   - bt709 colorspace eksplicit — undgår farve-shift
	args := []string{
		"-hide_banner", "-loglevel", "error",
		"-f", "rawvideo",
		"-pix_fmt", "rgba",
		"-s", fmt.Sprintf("%dx%d", e.config.Width, e.config.Height),
		"-r", fmt.Sprintf("%d", e.config.Framerate),
		"-i", "pipe:0",
		"-c:v", "h264_videotoolbox",
		"-realtime", "1",            // Low latency mode
		"-allow_sw", "0",            // Kræv HW accel
		"-b:v", fmt.Sprintf("%dk", e.config.Bitrate),
		"-profile:v", "high",
		"-level", "4.1",
		"-g", fmt.Sprintf("%d", e.config.KeyframeInterval),
		"-bf", "0",                  // No B-frames for low latency
		"-color_range", "tv",
		"-colorspace", "bt709",
		"-color_primaries", "bt709",
		"-color_trc", "bt709",
		"-flags", "+low_delay",
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
	if err := e.cmd.Start(); err != nil {
		return fmt.Errorf("ffmpeg start: %w", err)
	}
	e.started = true
	log.Printf("VideoToolbox encoder started (FFmpeg PID: %d, %dx%d @ %d kbps)",
		e.cmd.Process.Pid, e.config.Width, e.config.Height, e.config.Bitrate)
	return nil
}

func (e *VideoToolboxEncoder) Encode(frame *image.RGBA, forceKeyframe bool) ([]byte, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.started || e.stdin == nil {
		return nil, fmt.Errorf("encoder not started")
	}
	bounds := frame.Bounds()
	if bounds.Dx() != e.config.Width || bounds.Dy() != e.config.Height {
		e.config.Width = bounds.Dx()
		e.config.Height = bounds.Dy()
		log.Printf("VideoToolbox: resolution changed to %dx%d, restarting ffmpeg", e.config.Width, e.config.Height)
		if err := e.startFFmpeg(); err != nil {
			return nil, err
		}
	}
	if _, err := e.stdin.Write(frame.Pix); err != nil {
		return nil, fmt.Errorf("write frame: %w", err)
	}
	if e.buf == nil {
		e.buf = make([]byte, 256*1024)
	}
	n, err := e.stdout.Read(e.buf)
	if err != nil {
		return nil, fmt.Errorf("read output: %w", err)
	}
	result := make([]byte, n)
	copy(result, e.buf[:n])
	return result, nil
}

func (e *VideoToolboxEncoder) SetBitrate(kbps int) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.config.Bitrate = kbps
	log.Printf("VideoToolbox: bitrate updated to %d kbps (effective on next restart)", kbps)
	return nil
}

func (e *VideoToolboxEncoder) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.cmd != nil && e.cmd.Process != nil {
		e.cmd.Process.Kill()
		e.cmd.Wait()
		e.cmd = nil
	}
	e.started = false
	return nil
}

func (e *VideoToolboxEncoder) Name() string {
	return "videotoolbox"
}
