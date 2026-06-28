package encoder

import "testing"

func TestPopAnnexBAccessUnitWithAUD(t *testing.T) {
	frame1 := []byte{
		0x00, 0x00, 0x00, 0x01, 0x09, 0xf0,
		0x00, 0x00, 0x00, 0x01, 0x67, 0x01,
		0x00, 0x00, 0x00, 0x01, 0x68, 0x02,
		0x00, 0x00, 0x00, 0x01, 0x65, 0x03,
	}
	frame2 := []byte{
		0x00, 0x00, 0x01, 0x09, 0xf0,
		0x00, 0x00, 0x01, 0x41, 0x04,
	}
	stream := append(append([]byte{}, frame1...), frame2...)

	au, rest := popAnnexBAccessUnit(stream)
	if string(au) != string(frame1) {
		t.Fatalf("first AU mismatch: got %x want %x", au, frame1)
	}
	if string(rest) != string(frame2) {
		t.Fatalf("rest mismatch: got %x want %x", rest, frame2)
	}
}

func TestPopAnnexBAccessUnitWaitsForNextAUD(t *testing.T) {
	stream := []byte{
		0x00, 0x00, 0x00, 0x01, 0x09, 0xf0,
		0x00, 0x00, 0x00, 0x01, 0x65, 0x03,
	}

	au, rest := popAnnexBAccessUnit(stream)
	if au != nil {
		t.Fatalf("expected no complete AU, got %x", au)
	}
	if string(rest) != string(stream) {
		t.Fatalf("rest mismatch: got %x want %x", rest, stream)
	}
}
