package encoder

import (
	"fmt"
	"image"
	"log"
	"runtime"
	"sync"
	"unsafe"

	openh264 "github.com/y9o/go-openh264"
)

// OpenH264Encoder implements H.264 encoding using Cisco's OpenH264
// This uses purego for dynamic loading - no CGO required!
type OpenH264Encoder struct {
	config    Config
	mu        sync.Mutex
	encoder   *openh264.ISVCEncoder
	pinner    *runtime.Pinner
	frameNum  int64
	initialized bool
}

// NewOpenH264Encoder creates a new OpenH264 encoder
func NewOpenH264Encoder() *OpenH264Encoder {
	return &OpenH264Encoder{}
}

// Init initializes the OpenH264 encoder
func (e *OpenH264Encoder) Init(cfg Config) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Ensure OpenH264 DLL exists (download if needed)
	dllPath, err := EnsureOpenH264DLL()
	if err != nil {
		return fmt.Errorf("failed to ensure OpenH264 DLL: %w", err)
	}

	// Load OpenH264 DLL
	if err := openh264.Open(dllPath); err != nil {
		return fmt.Errorf("failed to load OpenH264 DLL: %w", err)
	}
	log.Printf("🎬 OpenH264 loaded from: %s", dllPath)

	e.config = cfg

	// Create encoder
	var ppEnc *openh264.ISVCEncoder
	if ret := openh264.WelsCreateSVCEncoder(&ppEnc); ret != 0 || ppEnc == nil {
		return fmt.Errorf("failed to create OpenH264 encoder: %d", ret)
	}
	e.encoder = ppEnc

	// Initialize encoder parameters
	encParam := openh264.SEncParamBase{
		IUsageType:     openh264.SCREEN_CONTENT_REAL_TIME, // Optimized for screen content
		IPicWidth:      int32(cfg.Width),
		IPicHeight:     int32(cfg.Height),
		ITargetBitrate: int32(cfg.Bitrate * 1000), // Convert kbps to bps
		FMaxFrameRate:  float32(cfg.Framerate),
	}

	if ret := e.encoder.Initialize(&encParam); ret != 0 {
		openh264.WelsDestroySVCEncoder(e.encoder)
		return fmt.Errorf("failed to initialize OpenH264 encoder: %d", ret)
	}

	// Set additional options for low latency
	var videoFormat int = int(openh264.VideoFormatI420)
	e.encoder.SetOption(openh264.ENCODER_OPTION_DATAFORMAT, &videoFormat)

	e.pinner = &runtime.Pinner{}
	e.initialized = true
	e.frameNum = 0

	log.Printf("🎬 OpenH264 encoder initialized: %dx%d @ %d fps, %d kbps",
		cfg.Width, cfg.Height, cfg.Framerate, cfg.Bitrate)

	return nil
}

// Encode encodes an RGBA frame to H.264 NAL units
func (e *OpenH264Encoder) Encode(frame *image.RGBA, forceKeyframe bool) ([]byte, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.initialized || e.encoder == nil {
		return nil, fmt.Errorf("encoder not initialized")
	}

	// Convert RGBA to YCbCr (I420)
	bounds := frame.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// Create YCbCr image
	ycbcr := image.NewYCbCr(bounds, image.YCbCrSubsampleRatio420)
	rgbaToYCbCr(frame, ycbcr)

	// Force keyframe if requested
	if forceKeyframe {
		var forceIDR int = 1
		e.encoder.SetOption(openh264.ENCODER_OPTION_IDR_INTERVAL, &forceIDR)
	}

	// Prepare source picture
	encSrcPic := openh264.SSourcePicture{
		IColorFormat: openh264.VideoFormatI420,
		IPicWidth:    int32(width),
		IPicHeight:   int32(height),
		UiTimeStamp:  e.frameNum,
	}

	// Set strides
	encSrcPic.IStride[0] = int32(ycbcr.YStride)
	encSrcPic.IStride[1] = int32(ycbcr.CStride)
	encSrcPic.IStride[2] = int32(ycbcr.CStride)

	// Pin memory and set data pointers
	e.pinner.Pin(&ycbcr.Y[0])
	e.pinner.Pin(&ycbcr.Cb[0])
	e.pinner.Pin(&ycbcr.Cr[0])
	defer e.pinner.Unpin()

	encSrcPic.PData[0] = (*uint8)(unsafe.Pointer(&ycbcr.Y[0]))
	encSrcPic.PData[1] = (*uint8)(unsafe.Pointer(&ycbcr.Cb[0]))
	encSrcPic.PData[2] = (*uint8)(unsafe.Pointer(&ycbcr.Cr[0]))

	// Encode frame
	encInfo := openh264.SFrameBSInfo{}
	if ret := e.encoder.EncodeFrame(&encSrcPic, &encInfo); ret != openh264.CmResultSuccess {
		return nil, fmt.Errorf("encode failed: %d", ret)
	}

	e.frameNum++

	// Skip frames produce no output
	if encInfo.EFrameType == openh264.VideoFrameTypeSkip {
		return nil, nil
	}

	// Collect NAL units
	var output []byte
	for iLayer := 0; iLayer < int(encInfo.ILayerNum); iLayer++ {
		pLayerBsInfo := &encInfo.SLayerInfo[iLayer]
		var iLayerSize int32
		nallens := unsafe.Slice(pLayerBsInfo.PNalLengthInByte, pLayerBsInfo.INalCount)
		for _, l := range nallens {
			iLayerSize += l
		}
		nals := unsafe.Slice(pLayerBsInfo.PBsBuf, iLayerSize)
		output = append(output, nals...)
	}

	return output, nil
}

// SetBitrate adjusts the encoding bitrate
func (e *OpenH264Encoder) SetBitrate(kbps int) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.initialized || e.encoder == nil {
		return fmt.Errorf("encoder not initialized")
	}

	e.config.Bitrate = kbps
	var bitrate int = kbps * 1000
	if ret := e.encoder.SetOption(openh264.ENCODER_OPTION_BITRATE, &bitrate); ret != 0 {
		return fmt.Errorf("failed to set bitrate: %d", ret)
	}

	log.Printf("🎬 OpenH264 bitrate set to %d kbps", kbps)
	return nil
}

// Close releases encoder resources
func (e *OpenH264Encoder) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.encoder != nil {
		e.encoder.Uninitialize()
		openh264.WelsDestroySVCEncoder(e.encoder)
		e.encoder = nil
	}

	e.initialized = false
	log.Println("🎬 OpenH264 encoder closed")
	return nil
}

// Name returns the encoder name
func (e *OpenH264Encoder) Name() string {
	return "openh264"
}

// rgbaToYCbCr converts RGBA image to YCbCr (I420)
func rgbaToYCbCr(rgba *image.RGBA, ycbcr *image.YCbCr) {
	bounds := rgba.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// Get RGBA pixel
			idx := y*rgba.Stride + x*4
			r := int32(rgba.Pix[idx])
			g := int32(rgba.Pix[idx+1])
			b := int32(rgba.Pix[idx+2])

			// Convert to YCbCr (BT.601)
			yy := (66*r + 129*g + 25*b + 128) >> 8
			ycbcr.Y[y*ycbcr.YStride+x] = uint8(yy + 16)

			// Subsample Cb and Cr (4:2:0)
			if x%2 == 0 && y%2 == 0 {
				cb := (-38*r - 74*g + 112*b + 128) >> 8
				cr := (112*r - 94*g - 18*b + 128) >> 8
				cx := x / 2
				cy := y / 2
				ycbcr.Cb[cy*ycbcr.CStride+cx] = uint8(cb + 128)
				ycbcr.Cr[cy*ycbcr.CStride+cx] = uint8(cr + 128)
			}
		}
	}
}
