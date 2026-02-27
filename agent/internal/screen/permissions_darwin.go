//go:build darwin

package screen

/*
#cgo LDFLAGS: -framework CoreGraphics
#include <CoreGraphics/CoreGraphics.h>

// CGPreflightScreenCaptureAccess was added in macOS 10.15
// We target 10.15+ so call directly (no __builtin_available needed)
static int checkScreenRecording() {
    return CGPreflightScreenCaptureAccess();
}

// CGRequestScreenCaptureAccess prompts user if not already granted
static int requestScreenRecording() {
    return CGRequestScreenCaptureAccess();
}
*/
import "C"

// CheckScreenRecordingPermission checks if screen recording permission is granted.
func CheckScreenRecordingPermission() bool {
	return C.checkScreenRecording() != 0
}

// RequestScreenRecordingPermission prompts the user for screen recording permission
// if not already granted. Returns true if permission is granted.
func RequestScreenRecordingPermission() bool {
	return C.requestScreenRecording() != 0
}
