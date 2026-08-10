package annexb

import (
	"bytes"
	"testing"
)

// sc4 builds a 4-byte Annex-B start code.
func sc4() []byte { return []byte{0x00, 0x00, 0x00, 0x01} }

// sc3 builds a 3-byte Annex-B start code.
func sc3() []byte { return []byte{0x00, 0x00, 0x01} }

// aud appends an Access Unit Delimiter NAL (type 9) with the given start code.
func aud(code []byte) []byte {
	return append(append([]byte{}, code...), 0x09, 0xf0)
}

func TestStartCodeLen(t *testing.T) {
	if got := StartCodeLen(sc4()); got != 4 {
		t.Fatalf("4-byte start code: got %d want 4", got)
	}
	if got := StartCodeLen(sc3()); got != 3 {
		t.Fatalf("3-byte start code: got %d want 3", got)
	}
	// Unknown/garbage defaults to 3.
	if got := StartCodeLen([]byte{0x01, 0x02, 0x03}); got != 3 {
		t.Fatalf("garbage start code: got %d want 3", got)
	}
}

func TestFindStartCode(t *testing.T) {
	data := append([]byte{0xAA, 0xBB}, sc4()...)
	data = append(data, 0x67)
	if got := FindStartCode(data, 0); got != 2 {
		t.Fatalf("expected start at 2, got %d", got)
	}
	// From index 3 the same start code still matches as a 3-byte 00 00 01
	// (bytes 3,4,5), which is valid Annex-B behavior.
	if got := FindStartCode(data, 3); got != 3 {
		t.Fatalf("search from inside overlapping start code: got %d want 3", got)
	}
	if got := FindStartCode(data, len(data)); got != -1 {
		t.Fatalf("expected -1 for from past data, got %d", got)
	}
	if got := FindStartCode([]byte{0x01, 0x02, 0x03, 0x04}, 0); got != -1 {
		t.Fatalf("expected -1 for no start code, got %d", got)
	}
}

func TestPopAccessUnitTwoCompleteUnits(t *testing.T) {
	frame1 := append(append(append(append([]byte{}, aud(sc4())...), 0x67, 0x01), 0x68, 0x02), 0x65, 0x03)
	_ = frame1
	// Build: [AUD][SPS][PPS][IDR] [AUD'][slice]
	f1 := bytes.Join([][]byte{
		aud(sc4()), {0x67, 0x01},
		{0x00, 0x00, 0x00, 0x01, 0x68, 0x02},
		{0x00, 0x00, 0x00, 0x01, 0x65, 0x03},
	}, nil)
	f2 := bytes.Join([][]byte{
		aud(sc3()), {0x41, 0x04},
	}, nil)
	stream := append(append([]byte{}, f1...), f2...)

	au, rest := PopAccessUnit(stream)
	if !bytes.Equal(au, f1) {
		t.Fatalf("first AU mismatch:\n got %x\nwant %x", au, f1)
	}
	if !bytes.Equal(rest, f2) {
		t.Fatalf("rest mismatch:\n got %x\nwant %x", rest, f2)
	}

	// Popping again from rest yields f2 as a *complete* unit only if a second
	// AUD follows; with a single AUD it must buffer (return nil au).
	au2, rest2 := PopAccessUnit(rest)
	if au2 != nil {
		t.Fatalf("single-AUD stream must not yield a unit, got %x", au2)
	}
	if !bytes.Equal(rest2, f2) {
		t.Fatalf("single-AUD rest must be preserved:\n got %x\nwant %x", rest2, f2)
	}
}

func TestPopAccessUnitWaitsForSecondAUD(t *testing.T) {
	stream := bytes.Join([][]byte{aud(sc4()), {0x65, 0x03}}, nil)

	au, rest := PopAccessUnit(stream)
	if au != nil {
		t.Fatalf("expected no complete AU, got %x", au)
	}
	if !bytes.Equal(rest, stream) {
		t.Fatalf("rest must equal input when buffering, got %x", rest)
	}
}

func TestPopAccessUnitShortInput(t *testing.T) {
	au, rest := PopAccessUnit([]byte{0x00, 0x00, 0x01})
	if au != nil {
		t.Fatalf("expected nil au for <6 bytes, got %x", au)
	}
	if !bytes.Equal(rest, []byte{0x00, 0x00, 0x01}) {
		t.Fatalf("rest must equal short input, got %x", rest)
	}

	au, rest = PopAccessUnit(nil)
	if au != nil || rest != nil {
		t.Fatalf("nil input must return nil/nil, got au=%x rest=%x", au, rest)
	}
}

func TestPopAccessUnitNoStartCodeKeepsTail(t *testing.T) {
	// No start code anywhere: keep the last 3 bytes so a partial start code
	// that arrives later can still be recognized.
	stream := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}
	au, rest := PopAccessUnit(stream)
	if au != nil {
		t.Fatalf("expected nil au, got %x", au)
	}
	want := stream[len(stream)-3:]
	if !bytes.Equal(rest, want) {
		t.Fatalf("rest must be last 3 bytes:\n got %x\nwant %x", rest, want)
	}
}

func TestPopAccessUnitLeadingGarbageSkipped(t *testing.T) {
	f1 := bytes.Join([][]byte{aud(sc4()), {0x65, 0x03}}, nil)
	f2 := bytes.Join([][]byte{aud(sc4()), {0x41, 0x05}}, nil)
	stream := append(append([]byte{0xFF, 0xEE, 0xDD}, f1...), f2...)

	au, _ := PopAccessUnit(stream)
	if !bytes.Equal(au, f1) {
		t.Fatalf("leading garbage must be skipped:\n got %x\nwant %x", au, f1)
	}
}

func TestPopAccessUnitThreeUnitsDrain(t *testing.T) {
	u1 := bytes.Join([][]byte{aud(sc4()), {0x65, 0x01}}, nil)
	u2 := bytes.Join([][]byte{aud(sc3()), {0x41, 0x02}}, nil)
	u3 := bytes.Join([][]byte{aud(sc4()), {0x41, 0x03}}, nil)
	stream := append(append([]byte{}, u1...), append(u2, u3...)...)

	au1, rest1 := PopAccessUnit(stream)
	if !bytes.Equal(au1, u1) {
		t.Fatalf("u1 mismatch:\n got %x\nwant %x", au1, u1)
	}
	au2, rest2 := PopAccessUnit(rest1)
	if !bytes.Equal(au2, u2) {
		t.Fatalf("u2 mismatch:\n got %x\nwant %x", au2, u2)
	}
	// u3 has no trailing AUD → buffered.
	au3, rest3 := PopAccessUnit(rest2)
	if au3 != nil {
		t.Fatalf("u3 must be buffered without trailing AUD, got %x", au3)
	}
	if !bytes.Equal(rest3, u3) {
		t.Fatalf("u3 rest mismatch:\n got %x\nwant %x", rest3, u3)
	}
}
