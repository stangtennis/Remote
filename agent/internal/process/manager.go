package process

// ProcessInfo represents a running process
type ProcessInfo struct {
	PID      int     `json:"pid"`
	Name     string  `json:"name"`
	CPU      float64 `json:"cpu"`
	MemoryMB float64 `json:"memory_mb"`
	User     string  `json:"user"`
}
