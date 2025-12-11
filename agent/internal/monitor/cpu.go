// Package monitor provides system monitoring for adaptive streaming
package monitor

import (
	"log"
	"runtime"
	"sync"
	"time"
)

// CPUMonitor tracks process CPU usage for adaptive streaming
type CPUMonitor struct {
	mu              sync.RWMutex
	cpuPct          float64   // Current CPU percentage (0-100)
	samples         []float64 // Recent samples for averaging
	maxSamples      int
	highCPUCount    int     // Count of consecutive high CPU readings
	threshold       float64 // Threshold for "high" CPU (default 85%)
	criticalCount   int     // Number of high readings before triggering guard
	running         bool
	lastMeasureTime time.Time
	lastCPUTime     time.Duration
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
	m.running = true
	m.lastMeasureTime = time.Now()
	m.mu.Unlock()

	go m.monitorLoop()
	log.Println("📊 CPU monitor started")
}

// Stop stops CPU monitoring
func (m *CPUMonitor) Stop() {
	m.mu.Lock()
	m.running = false
	m.mu.Unlock()
	log.Println("📊 CPU monitor stopped")
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

	var lastNumGC uint32
	var lastPauseTotal time.Duration

	for {
		m.mu.RLock()
		running := m.running
		m.mu.RUnlock()

		if !running {
			return
		}

		<-ticker.C

		// Use runtime stats as proxy for CPU usage
		// This is an approximation based on GC activity and goroutine count
		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)

		// Calculate CPU estimate based on:
		// 1. GC pause time (more GC = more CPU)
		// 2. Number of goroutines (more = potentially more CPU)
		// 3. Allocations (more allocs = more CPU)

		gcPauseIncrease := time.Duration(memStats.PauseTotalNs) - lastPauseTotal
		gcCountIncrease := memStats.NumGC - lastNumGC

		lastPauseTotal = time.Duration(memStats.PauseTotalNs)
		lastNumGC = memStats.NumGC

		// Estimate CPU based on GC activity (rough heuristic)
		// High GC activity often correlates with high CPU
		cpuEstimate := float64(0)

		if gcCountIncrease > 0 {
			// GC pause as percentage of interval
			intervalMs := float64(500)
			pauseMs := float64(gcPauseIncrease.Milliseconds())
			gcCPU := (pauseMs / intervalMs) * 100

			// Scale up since GC is just part of CPU usage
			cpuEstimate = gcCPU * 5 // Rough multiplier
		}

		// Add goroutine overhead estimate
		numGoroutines := runtime.NumGoroutine()
		if numGoroutines > 50 {
			cpuEstimate += float64(numGoroutines-50) * 0.5
		}

		// Clamp to 0-100
		if cpuEstimate < 0 {
			cpuEstimate = 0
		}
		if cpuEstimate > 100 {
			cpuEstimate = 100
		}

		m.mu.Lock()
		m.cpuPct = cpuEstimate

		// Track samples for averaging
		m.samples = append(m.samples, cpuEstimate)
		if len(m.samples) > m.maxSamples {
			m.samples = m.samples[1:]
		}

		// Check if high CPU
		if cpuEstimate > m.threshold {
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
