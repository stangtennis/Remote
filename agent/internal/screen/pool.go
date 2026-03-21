package screen

import "sync"

// pixelBufferPool reuses large byte slices for capture buffers,
// reducing GC pressure from allocating multi-megabyte buffers every frame.
var pixelBufferPool = sync.Pool{}

// getPixelBuffer returns a byte slice of at least the given size.
// The returned slice may be larger than requested — always use [:size].
func getPixelBuffer(size int) []byte {
	if v := pixelBufferPool.Get(); v != nil {
		buf := v.([]byte)
		if cap(buf) >= size {
			return buf[:size]
		}
	}
	return make([]byte, size)
}

// putPixelBuffer returns a buffer to the pool for reuse.
func putPixelBuffer(buf []byte) {
	pixelBufferPool.Put(buf)
}
