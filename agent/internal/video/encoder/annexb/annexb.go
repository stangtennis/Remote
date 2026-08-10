// Package annexb provides pure-Go helpers for parsing H.264 Annex-B byte
// streams into access units delimited by Access Unit Delimiter (AUD) NAL units
// (NAL type 9). The logic is isolated from the encoder package so it can be
// unit-tested on platforms where the screen-capture (CGO/C++) dependency is
// unavailable.
package annexb

// PopAccessUnit consumes the first complete access unit from data.
//
// A complete access unit starts with an AUD NAL unit (type 9) and ends right
// before the next AUD. If no second AUD is present, no complete unit can be
// emitted yet and the (possibly trimmed) remaining bytes are returned so the
// caller can buffer them for the next call.
//
// The returned au slice is a copy; rest aliases the tail of data.
func PopAccessUnit(data []byte) (au []byte, rest []byte) {
	if len(data) < 6 {
		return nil, data
	}

	firstStart := FindStartCode(data, 0)
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
		start := FindStartCode(data, pos)
		if start < 0 {
			break
		}
		nalStart := start + StartCodeLen(data[start:])
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

// FindStartCode returns the byte offset of the next Annex-B start code
// (00 00 01 or 00 00 00 01) at or after from, or -1 if none is found.
func FindStartCode(data []byte, from int) int {
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

// StartCodeLen returns 4 for a 00 00 00 01 start code and 3 otherwise
// (00 00 01). Callers must ensure data begins at a start code position.
func StartCodeLen(data []byte) int {
	if len(data) >= 4 && data[0] == 0 && data[1] == 0 && data[2] == 0 && data[3] == 1 {
		return 4
	}
	return 3
}
