package state

import (
	"sync/atomic"

	"github.com/stangtennis/remote-agent/internal/metrics"
)

var privacyMode atomic.Bool

// SetPrivacyMode toggles privacy mode. When enabled, screen capture
// returns solid black frames instead of the real screen content.
// Both GUI and capture must run in the same process for this to work.
func SetPrivacyMode(enabled bool) {
	privacyMode.Store(enabled)
	metrics.SetPrivacyMode(enabled)
}

// IsPrivacyModeEnabled returns true if capture should return black frames.
func IsPrivacyModeEnabled() bool {
	return privacyMode.Load()
}
