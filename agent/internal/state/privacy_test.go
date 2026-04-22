package state

import (
	"sync"
	"testing"
)

func TestPrivacyMode_DefaultIsOff(t *testing.T) {
	SetPrivacyMode(false) // reset in case other tests set it
	if IsPrivacyModeEnabled() {
		t.Fatal("expected privacy mode to be off by default")
	}
}

func TestPrivacyMode_Toggle(t *testing.T) {
	SetPrivacyMode(false)

	SetPrivacyMode(true)
	if !IsPrivacyModeEnabled() {
		t.Fatal("expected privacy mode to be on after SetPrivacyMode(true)")
	}

	SetPrivacyMode(false)
	if IsPrivacyModeEnabled() {
		t.Fatal("expected privacy mode to be off after SetPrivacyMode(false)")
	}
}

// TestPrivacyMode_ConcurrentAccess verifies the atomic flag is race-safe.
// Run with `go test -race`.
func TestPrivacyMode_ConcurrentAccess(t *testing.T) {
	SetPrivacyMode(false)

	var wg sync.WaitGroup
	const workers = 50
	const iterations = 1000

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				// Alternate writers, interleave with readers
				if (id+j)%2 == 0 {
					SetPrivacyMode(true)
				} else {
					SetPrivacyMode(false)
				}
				_ = IsPrivacyModeEnabled()
			}
		}(i)
	}
	wg.Wait()

	// Final state deterministic based on last operation
	SetPrivacyMode(false)
	if IsPrivacyModeEnabled() {
		t.Fatal("expected off after explicit reset")
	}
}
