//go:build darwin

package input

/*
#cgo LDFLAGS: -framework ApplicationServices -framework CoreFoundation
#include <ApplicationServices/ApplicationServices.h>
#include <CoreFoundation/CoreFoundation.h>

// Check if process has Accessibility (TCC) permission
// prompt=1 opens System Preferences dialog if not trusted
static int checkAccessibility(int prompt) {
    if (prompt) {
        const void *keys[] = { kAXTrustedCheckOptionPrompt };
        const void *values[] = { kCFBooleanTrue };
        CFDictionaryRef options = CFDictionaryCreate(
            kCFAllocatorDefault, keys, values, 1,
            &kCFTypeDictionaryKeyCallBacks,
            &kCFTypeDictionaryValueCallBacks
        );
        Boolean trusted = AXIsProcessTrustedWithOptions(options);
        CFRelease(options);
        return trusted;
    }
    return AXIsProcessTrusted();
}
*/
import "C"

// CheckAccessibilityPermission checks if the process has Accessibility permission.
// If prompt is true, macOS will show a dialog asking the user to grant permission.
func CheckAccessibilityPermission(prompt bool) bool {
	p := C.int(0)
	if prompt {
		p = 1
	}
	return C.checkAccessibility(p) != 0
}

// IsAccessibilityTrusted checks Accessibility permission without prompting.
func IsAccessibilityTrusted() bool {
	return CheckAccessibilityPermission(false)
}
