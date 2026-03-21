package screen

import (
	"image"
	"sync"
)

// DirtyRegion represents a changed area of the screen
type DirtyRegion struct {
	X, Y, Width, Height int
	Data                []byte // JPEG encoded region
}

// DirtyRegionDetector detects changed regions between frames
type DirtyRegionDetector struct {
	lastFrame  *image.RGBA
	buffers    [2]*image.RGBA // Double-buffer: avoids re-allocating per frame
	currentBuf int
	tileWidth  int
	tileHeight int
	mu         sync.Mutex
}

// NewDirtyRegionDetector creates a new detector with specified tile size
// Smaller tiles = more precision but more overhead
// Recommended: 64x64 or 128x128
func NewDirtyRegionDetector(tileWidth, tileHeight int) *DirtyRegionDetector {
	return &DirtyRegionDetector{
		tileWidth:  tileWidth,
		tileHeight: tileHeight,
	}
}

// DetectDirtyRegions compares current frame with last frame and returns changed regions
// Returns nil if this is the first frame (full frame should be sent)
func (d *DirtyRegionDetector) DetectDirtyRegions(current *image.RGBA, quality int) ([]DirtyRegion, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()

	bounds := current.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// First frame or resolution change - allocate double-buffers
	if d.lastFrame == nil || d.lastFrame.Bounds() != current.Bounds() {
		d.buffers[0] = image.NewRGBA(current.Bounds())
		d.buffers[1] = image.NewRGBA(current.Bounds())
		d.currentBuf = 0
		copy(d.buffers[0].Pix, current.Pix)
		d.lastFrame = d.buffers[0]
		return nil, true // isFirstFrame = true
	}

	var dirtyRegions []DirtyRegion

	// Compare tiles
	for y := 0; y < height; y += d.tileHeight {
		for x := 0; x < width; x += d.tileWidth {
			// Calculate tile bounds
			tileW := min(d.tileWidth, width-x)
			tileH := min(d.tileHeight, height-y)

			// Check if tile has changed
			if d.isTileDirty(current, x, y, tileW, tileH) {
				// Extract and encode the dirty region
				region := extractRegion(current, x, y, tileW, tileH)
				encoded, err := encodeRegionJPEG(region, quality)
				if err == nil {
					dirtyRegions = append(dirtyRegions, DirtyRegion{
						X:      x,
						Y:      y,
						Width:  tileW,
						Height: tileH,
						Data:   encoded,
					})
				}
			}
		}
	}

	// Swap double-buffers (reuse pre-allocated buffer, avoid allocation)
	nextBuf := 1 - d.currentBuf
	copy(d.buffers[nextBuf].Pix, current.Pix)
	d.currentBuf = nextBuf
	d.lastFrame = d.buffers[nextBuf]

	return dirtyRegions, false
}

// isTileDirty checks if a tile has changed between frames
// Uses sampling for speed - checks every 4th pixel
func (d *DirtyRegionDetector) isTileDirty(current *image.RGBA, x, y, w, h int) bool {
	stride := current.Stride
	lastStride := d.lastFrame.Stride

	// Sample every 4th pixel for speed
	for dy := 0; dy < h; dy += 4 {
		for dx := 0; dx < w; dx += 4 {
			px := x + dx
			py := y + dy

			currIdx := py*stride + px*4
			lastIdx := py*lastStride + px*4

			// Compare RGBA values (with small threshold for noise)
			if absDiff(current.Pix[currIdx], d.lastFrame.Pix[lastIdx]) > 2 ||
				absDiff(current.Pix[currIdx+1], d.lastFrame.Pix[lastIdx+1]) > 2 ||
				absDiff(current.Pix[currIdx+2], d.lastFrame.Pix[lastIdx+2]) > 2 {
				return true
			}
		}
	}
	return false
}

// GetChangePercentage returns what percentage of the screen changed
func (d *DirtyRegionDetector) GetChangePercentage(regions []DirtyRegion, screenWidth, screenHeight int) float64 {
	if len(regions) == 0 {
		return 0
	}

	totalPixels := screenWidth * screenHeight
	changedPixels := 0
	for _, r := range regions {
		changedPixels += r.Width * r.Height
	}

	return float64(changedPixels) / float64(totalPixels) * 100
}

// Reset clears the last frame (forces full frame on next capture)
func (d *DirtyRegionDetector) Reset() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.lastFrame = nil
}

// Helper functions

func extractRegion(img *image.RGBA, x, y, w, h int) *image.RGBA {
	region := image.NewRGBA(image.Rect(0, 0, w, h))
	srcStride := img.Stride
	dstStride := region.Stride

	for dy := 0; dy < h; dy++ {
		srcStart := (y+dy)*srcStride + x*4
		dstStart := dy * dstStride
		copy(region.Pix[dstStart:dstStart+w*4], img.Pix[srcStart:srcStart+w*4])
	}

	return region
}

func encodeRegionJPEG(img *image.RGBA, quality int) ([]byte, error) {
	return EncodeJPEG(img.Pix, img.Bounds().Dx(), img.Bounds().Dy(), img.Stride, quality, false)
}

func absDiff(a, b uint8) uint8 {
	if a > b {
		return a - b
	}
	return b - a
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
