package webrtc

import (
	"context"
	"encoding/json"
	"time"

	pionwebrtc "github.com/pion/webrtc/v3"
)

// sendStats sends streaming stats to controller
func (m *Manager) sendStats(fps, quality int, scale float64, mode string, rttMs int64, cpuPct float64) {
	if m.dataChannel == nil || m.dataChannel.ReadyState() != pionwebrtc.DataChannelStateOpen {
		return
	}

	stats := map[string]interface{}{
		"type":    "stats",
		"fps":     fps,
		"quality": quality,
		"scale":   scale,
		"mode":    mode,
		"rtt":     rttMs,
		"cpu":     cpuPct,
	}

	data, err := json.Marshal(stats)
	if err != nil {
		return
	}

	m.dataChannel.Send(data)
}

// collectStats collects WebRTC stats for adaptive streaming
func (m *Manager) collectStats(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		m.mu.Lock()
		pc := m.peerConnection
		m.mu.Unlock()

		if pc == nil {
			continue
		}

		stats := pc.GetStats()
		for _, stat := range stats {
			// Look for data channel stats
			if dcStats, ok := stat.(pionwebrtc.DataChannelStats); ok {
				sent := dcStats.MessagesSent
				// Calculate loss from retransmits (approximation)
				if sent > m.lastPacketsSent && m.lastPacketsSent > 0 {
					delta := sent - m.lastPacketsSent
					if delta > 0 {
						// Use buffered amount as proxy for congestion/loss
						buffered := float64(0)
						if m.dataChannel != nil {
							buffered = float64(m.dataChannel.BufferedAmount())
						}
						// High buffer = potential loss/congestion
						if buffered > 4*1024*1024 { // 4MB
							m.lossPct = (buffered / (16 * 1024 * 1024)) * 10 // 0-10% based on buffer
							if m.lossPct > 10 {
								m.lossPct = 10
							}
						} else {
							m.lossPct = 0
						}
					}
				}
				m.lastPacketsSent = sent
			}
		}
	}
}
