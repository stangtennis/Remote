package clipboard

import "testing"

func TestHashString(t *testing.T) {
	h1 := hashString("hello")
	h2 := hashString("hello")
	h3 := hashString("world")

	if h1 != h2 {
		t.Error("same input should produce same hash")
	}
	if h1 == h3 {
		t.Error("different input should produce different hash")
	}
	if len(h1) != 64 {
		t.Errorf("hash length should be 64 hex chars, got %d", len(h1))
	}
}

func TestHashBytes(t *testing.T) {
	h1 := hashBytes([]byte{1, 2, 3})
	h2 := hashBytes([]byte{1, 2, 3})
	h3 := hashBytes([]byte{4, 5, 6})

	if h1 != h2 {
		t.Error("same input should produce same hash")
	}
	if h1 == h3 {
		t.Error("different input should produce different hash")
	}
}

func TestTruncateString(t *testing.T) {
	if got := truncateString("hello", 10); got != "hello" {
		t.Errorf("truncateString short = %q, want 'hello'", got)
	}
	if got := truncateString("hello world", 5); got != "hello" {
		t.Errorf("truncateString long = %q, want 'hello'", got)
	}
}

func TestRememberText(t *testing.T) {
	m := NewMonitor()
	m.RememberText("test")
	if m.lastTextHash != hashString("test") {
		t.Error("RememberText should update lastTextHash")
	}
}

func TestRememberImage(t *testing.T) {
	m := NewMonitor()
	data := []byte{0xFF, 0xD8, 0xFF}
	m.RememberImage(data)
	if m.lastImageHash != hashBytes(data) {
		t.Error("RememberImage should update lastImageHash")
	}
}
