//go:build darwin

package main

/*
#cgo darwin CFLAGS: -x objective-c
#cgo darwin LDFLAGS: -framework Foundation -framework UniformTypeIdentifiers

#import <Foundation/Foundation.h>
#include <stdint.h>

// osxcross builds do not currently ship the newer compiler-rt symbol that
// Clang emits for @available(...) checks. Wails uses those checks in its
// Objective-C glue, so provide a macOS-only fallback using Foundation's
// operatingSystemVersion APIs instead of compiler-rt availability helpers.
int32_t __isPlatformVersionAtLeast(uint32_t Platform, uint32_t Major, uint32_t Minor, uint32_t Subminor) {
	(void)Platform;
	NSProcessInfo *processInfo = [NSProcessInfo processInfo];
	NSOperatingSystemVersion requiredVersion;
	requiredVersion.majorVersion = (NSInteger)Major;
	requiredVersion.minorVersion = (NSInteger)Minor;
	requiredVersion.patchVersion = (NSInteger)Subminor;

	if ([processInfo respondsToSelector:@selector(isOperatingSystemAtLeastVersion:)]) {
		return [processInfo isOperatingSystemAtLeastVersion:requiredVersion] ? 1 : 0;
	}

	NSOperatingSystemVersion currentVersion = [processInfo operatingSystemVersion];
	if (currentVersion.majorVersion != requiredVersion.majorVersion) {
		return currentVersion.majorVersion > requiredVersion.majorVersion ? 1 : 0;
	}
	if (currentVersion.minorVersion != requiredVersion.minorVersion) {
		return currentVersion.minorVersion > requiredVersion.minorVersion ? 1 : 0;
	}
	return currentVersion.patchVersion >= requiredVersion.patchVersion ? 1 : 0;
}
*/
import "C"
