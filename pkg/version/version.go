package version

import "time"

// Version information - single source of truth
// Update these values before building a new release
var (
	Version   = "2.65.0"
	BuildDate = time.Now().Format("2006-01-02 15:04:05")
)

// GetVersion returns the current version string
func GetVersion() string {
	return Version
}

// GetBuildDate returns the build date string
func GetBuildDate() string {
	return BuildDate
}

// GetFullVersion returns version with build date
func GetFullVersion() string {
	return Version + " (built " + BuildDate + ")"
}
