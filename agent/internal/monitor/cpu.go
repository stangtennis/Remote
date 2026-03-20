// Package monitor provides system monitoring for adaptive streaming
package monitor

import (
	"log"
	"os"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/process"
)

// CPUMonitor tracks process CPU usage for adaptive streaming
type CPUMonitor struct {
	mu           sync.RWMutex
	cpuPct       float64   // Current CPU percentage (0-100)
	samples      []float64 // Recent samples for averaging
	maxSamples   int
	highCPUCount int     // Count of consecutive high CPU readings
	threshold    float64 // Threshold for "high" CPU (default 85%)
	criticalCount int    // Number of high readings before triggering guard
	running      bool
	proc         *process.Process
}

// NewCPUMonitor creates a new CPU monitor
func NewCPUMonitor() *CPUMonitor {
	return &CPUMonitor{
		samples:       make([]float64, 0, 10),
		maxSamples:    10,
		threshold:     85.0,
		criticalCount: 3,
	}
}

// Start begins CPU monitoring in background
func (m *CPUMonitor) Start() {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}

	proc, err := process.NewProcess(int32(os.Getpid()))
	if err != nil {
		m.mu.Unlock()
		log.Printf("CPU monitor: failed to get process: %v", err)
		return
	}
	m.proc = proc
	m.running = true
	m.mu.Unlock()

	go m.monitorLoop()
	log.Println("CPU monitor started")
}

// Stop stops CPU monitoring
func (m *CPUMonitor) Stop() {
	m.mu.Lock()
	m.running = false
	m.mu.Unlock()
	log.Println("CPU monitor stopped")
}

// GetCPUPercent returns current CPU usage percentage
func (m *CPUMonitor) GetCPUPercent() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cpuPct
}

// IsHighCPU returns true if CPU has been high for criticalCount measurements
func (m *CPUMonitor) IsHighCPU() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.highCPUCount >= m.criticalCount
}

// IsCriticalCPU returns true if CPU is critically high (>90%)
func (m *CPUMonitor) IsCriticalCPU() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cpuPct > 90.0
}

// SetThreshold sets the high CPU threshold
func (m *CPUMonitor) SetThreshold(pct float64) {
	m.mu.Lock()
	m.threshold = pct
	m.mu.Unlock()
}

func (m *CPUMonitor) monitorLoop() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		m.mu.RLock()
		running := m.running
		m.mu.RUnlock()

		if !running {
			return
		}

		<-ticker.C

		// Get actual process CPU percentage via gopsutil
		pct, err := m.proc.Percent(0)
		if err != nil {
			continue
		}

		m.mu.Lock()
		m.cpuPct = pct

		// Track samples for averaging
		m.samples = append(m.samples, pct)
		if len(m.samples) > m.maxSamples {
			m.samples = m.samples[1:]
		}

		// Check if high CPU
		if pct > m.threshold {
			m.highCPUCount++
		} else {
			m.highCPUCount = 0
		}

		m.mu.Unlock()
	}
}

// GetAverageCPU returns average CPU over recent samples
func (m *CPUMonitor) GetAverageCPU() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.samples) == 0 {
		return 0
	}

	sum := float64(0)
	for _, s := range m.samples {
		sum += s
	}
	return sum / float64(len(m.samples))
}
