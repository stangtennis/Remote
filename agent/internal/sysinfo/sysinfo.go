// Package sysinfo collects host diagnostics that the controller fetches via
// the existing process WebRTC data channel ("sysinfo" op).
package sysinfo

// DiskInfo describes a single mounted volume / drive letter.
type DiskInfo struct {
	Mount   string  `json:"mount"`
	TotalGB float64 `json:"total_gb"`
	FreeGB  float64 `json:"free_gb"`
}

// AppInfo describes one installed application.
type AppInfo struct {
	Name      string `json:"name"`
	Version   string `json:"version,omitempty"`
	Publisher string `json:"publisher,omitempty"`
}

// Info is the aggregated payload returned to the controller.
type Info struct {
	OS            string     `json:"os"`
	Hostname      string     `json:"hostname"`
	CPU           string     `json:"cpu"`
	CPUCores      int        `json:"cpu_cores"`
	RAMTotalGB    float64    `json:"ram_total_gb"`
	RAMFreeGB     float64    `json:"ram_free_gb"`
	Disks         []DiskInfo `json:"disks"`
	UptimeSec     int64      `json:"uptime_sec"`
	InstalledApps []AppInfo  `json:"installed_apps,omitempty"`
}

// Collect returns a snapshot of the current host. Implementation lives in
// sysinfo_windows.go / sysinfo_darwin.go.
func Collect() (Info, error) {
	return collectPlatform()
}
