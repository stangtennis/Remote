package reconnection

import (
	"log"
	"math"
	"sync"
	"time"
)

// Manager handles automatic reconnection with exponential backoff
type Manager struct {
	maxRetries        int
	baseDelay         time.Duration
	maxDelay          time.Duration
	backoffMultiplier float64
	currentAttempt    int
	isReconnecting    bool
	isCancelled       bool
	mu                sync.Mutex
	
	// Callbacks
	onReconnecting    func(attempt int, maxAttempts int, nextDelay time.Duration)
	onReconnected     func()
	onReconnectFailed func()
	reconnectFunc     func() error
}

// NewManager creates a new reconnection manager
func NewManager() *Manager {
	return &Manager{
		maxRetries:        10,
		baseDelay:         1 * time.Second,
		maxDelay:          30 * time.Second,
		backoffMultiplier: 2.0,
		currentAttempt:    0,
		isReconnecting:    false,
		isCancelled:       false,
	}
}

// SetMaxRetries sets the maximum number of retry attempts
func (m *Manager) SetMaxRetries(max int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.maxRetries = max
}

// SetBaseDelay sets the initial delay before first retry
func (m *Manager) SetBaseDelay(delay time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.baseDelay = delay
}

// SetReconnectFunc sets the function to call for reconnection
func (m *Manager) SetReconnectFunc(fn func() error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reconnectFunc = fn
}

// SetOnReconnecting sets the callback for reconnection attempts
func (m *Manager) SetOnReconnecting(fn func(attempt int, maxAttempts int, nextDelay time.Duration)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onReconnecting = fn
}

// SetOnReconnected sets the callback for successful reconnection
func (m *Manager) SetOnReconnected(fn func()) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onReconnected = fn
}

// SetOnReconnectFailed sets the callback for failed reconnection
func (m *Manager) SetOnReconnectFailed(fn func()) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onReconnectFailed = fn
}

// StartReconnection begins the reconnection process
func (m *Manager) StartReconnection() {
	m.mu.Lock()
	if m.isReconnecting {
		m.mu.Unlock()
		return // Already reconnecting
	}
	m.isReconnecting = true
	m.isCancelled = false
	m.currentAttempt = 0
	m.mu.Unlock()

	go m.reconnectionLoop()
}

// reconnectionLoop performs the reconnection attempts
func (m *Manager) reconnectionLoop() {
	defer func() {
		m.mu.Lock()
		m.isReconnecting = false
		m.mu.Unlock()
	}()

	for {
		m.mu.Lock()
		if m.isCancelled {
			m.mu.Unlock()
			log.Println("🛑 Reconnection cancelled")
			return
		}
		
		m.currentAttempt++
		attempt := m.currentAttempt
		maxRetries := m.maxRetries
		reconnectFunc := m.reconnectFunc
		m.mu.Unlock()

		if attempt > maxRetries {
			log.Printf("❌ Reconnection failed after %d attempts", maxRetries)
			m.mu.Lock()
			callback := m.onReconnectFailed
			m.mu.Unlock()
			if callback != nil {
				callback()
			}
			return
		}

		// Calculate delay with exponential backoff
		delay := m.calculateDelay(attempt)
		
		log.Printf("🔄 Reconnection attempt %d/%d in %v", attempt, maxRetries, delay)
		
		// Notify UI
		m.mu.Lock()
		callback := m.onReconnecting
		m.mu.Unlock()
		if callback != nil {
			callback(attempt, maxRetries, delay)
		}

		// Wait before attempting
		time.Sleep(delay)

		// Check if cancelled during sleep
		m.mu.Lock()
		if m.isCancelled {
			m.mu.Unlock()
			log.Println("🛑 Reconnection cancelled during wait")
			return
		}
		m.mu.Unlock()

		// Attempt reconnection
		if reconnectFunc != nil {
			err := reconnectFunc()
			if err == nil {
				log.Println("✅ Reconnected successfully!")
				m.reset()
				m.mu.Lock()
				callback := m.onReconnected
				m.mu.Unlock()
				if callback != nil {
					callback()
				}
				return
			}
			log.Printf("⚠️  Reconnection attempt %d failed: %v", attempt, err)
		}
	}
}

// calculateDelay calculates the delay for the next attempt using exponential backoff
func (m *Manager) calculateDelay(attempt int) time.Duration {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Exponential backoff: baseDelay * (multiplier ^ (attempt - 1))
	delay := time.Duration(float64(m.baseDelay) * math.Pow(m.backoffMultiplier, float64(attempt-1)))
	
	// Cap at max delay
	if delay > m.maxDelay {
		delay = m.maxDelay
	}
	
	return delay
}

// Cancel stops the reconnection process
func (m *Manager) Cancel() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.isReconnecting {
		m.isCancelled = true
		log.Println("🛑 Cancelling reconnection...")
	}
}

// reset resets the reconnection state
func (m *Manager) reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.currentAttempt = 0
	m.isReconnecting = false
	m.isCancelled = false
}

// IsReconnecting returns whether reconnection is in progress
func (m *Manager) IsReconnecting() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.isReconnecting
}

// GetCurrentAttempt returns the current attempt number
func (m *Manager) GetCurrentAttempt() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.currentAttempt
}

// GetMaxRetries returns the maximum number of retries
func (m *Manager) GetMaxRetries() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.maxRetries
}
