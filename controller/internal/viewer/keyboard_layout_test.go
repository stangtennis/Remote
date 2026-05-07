package viewer

import (
	"testing"

	"fyne.io/fyne/v2"
)

func TestShouldForwardTypedRune(t *testing.T) {
	v := &Viewer{}

	tests := []struct {
		name         string
		r            rune
		rightAltDown bool
		want         bool
	}{
		{name: "ascii without altgr uses key events", r: 'a', want: false},
		{name: "danish ae uses unicode text path", r: 'æ', want: true},
		{name: "danish oe uses unicode text path", r: 'ø', want: true},
		{name: "danish aa uses unicode text path", r: 'å', want: true},
		{name: "altgr ascii uses unicode text path", r: '@', rightAltDown: true, want: true},
		{name: "control characters ignored", r: '\n', rightAltDown: true, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v.rightAltDown = tt.rightAltDown
			if got := v.shouldForwardTypedRune(tt.r); got != tt.want {
				t.Fatalf("shouldForwardTypedRune(%q) = %v, want %v", tt.r, got, tt.want)
			}
		})
	}
}

func TestIsPotentialAltGrTextKey(t *testing.T) {
	tests := []struct {
		key  fyne.KeyName
		want bool
	}{
		{key: fyne.KeyName("2"), want: true},
		{key: fyne.KeyName("8"), want: true},
		{key: fyne.KeyName("e"), want: true},
		{key: fyne.KeyF4, want: false},
		{key: fyne.KeyEscape, want: false},
	}

	for _, tt := range tests {
		if got := isPotentialAltGrTextKey(tt.key); got != tt.want {
			t.Fatalf("isPotentialAltGrTextKey(%q) = %v, want %v", tt.key, got, tt.want)
		}
	}
}
