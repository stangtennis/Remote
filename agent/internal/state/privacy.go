package state

import "sync/atomic"

var privacyMode atomic.Bool

// SetPrivacyMode toggles privacy mode. When enabled, screen capture
// returns solid black frames instead of the real screen content.
// Both GUI and capture must run in the same process for this to work.
func SetPrivacyMode(enabled bool) {
	privacyMode.Store(enabled)
}

// IsPrivacyModeEnabled returns true if capture should return black frames.
func IsPrivacyModeEnabled() bool {
	return privacyMode.Load()
}
