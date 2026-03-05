package version

// Version information - injected at build time via -ldflags -X
var (
	Version   = "dev"
	BuildDate = "unknown"
)
