package metrics

import (
	"os"
	"testing"
)

func TestEnabled_Default(t *testing.T) {
	os.Unsetenv("RD_METRICS_ENABLED")
	if Enabled() {
		t.Fatal("expected metrics disabled by default")
	}
}

func TestEnabled_Truthy(t *testing.T) {
	for _, v := range []string{"1", "true", "TRUE", "yes", "YES"} {
		os.Setenv("RD_METRICS_ENABLED", v)
		if !Enabled() {
			t.Fatalf("expected Enabled() for RD_METRICS_ENABLED=%q", v)
		}
	}
	os.Unsetenv("RD_METRICS_ENABLED")
}

func TestEnabled_Falsy(t *testing.T) {
	for _, v := range []string{"", "0", "false", "off", "no"} {
		os.Setenv("RD_METRICS_ENABLED", v)
		if Enabled() {
			t.Fatalf("expected disabled for RD_METRICS_ENABLED=%q", v)
		}
	}
	os.Unsetenv("RD_METRICS_ENABLED")
}

func TestHelpersAreNoopsWhenDisabled(t *testing.T) {
	os.Unsetenv("RD_METRICS_ENABLED")
	// These must not panic or error even without Init.
	RecordRTT(42)
	RecordReconnect()
	SetActiveSessions(3)
	SetPrivacyMode(true)
	SetPrivacyMode(false)
	AddBytesSent("video", 1024)

	// TimeFrameEncode returns a callable that measures duration
	done := TimeFrameEncode("jpeg")
	done() // should be a no-op when disabled
}

func TestAddBytesSent_NegativeIsNoop(t *testing.T) {
	// Should not panic even if metrics enabled
	os.Setenv("RD_METRICS_ENABLED", "1")
	defer os.Unsetenv("RD_METRICS_ENABLED")
	AddBytesSent("video", -100)
	AddBytesSent("video", 0)
}
