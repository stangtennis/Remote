package webrtc

import (
	"context"
	"encoding/json"
	"time"

	pionwebrtc "github.com/pion/webrtc/v3"
	"github.com/stangtennis/remote-agent/internal/metrics"
)

// sendStats sends streaming stats to controller
func (m *Manager) sendStats(fps, quality int, scale float64, mode string, rttMs int64, cpuPct float64) {
	if m.dataChannel == nil || m.dataChannel.ReadyState() != pionwebrtc.DataChannelStateOpen {
		return
	}

	stats := map[string]interface{}{
		"type":      "stats",
		"fps":       fps,
		"quality":   quality,
		"scale":     scale,
		"mode":      mode,
		"rtt":       rttMs,
		"cpu":       cpuPct,
		"conn_type": m.getConnectionType(),
		"bytes_tx":  m.getTotalBytesSent(),
		"bytes_rx":  m.getTotalBytesReceived(),
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

	var prevPacketsSent, prevPacketsReceived uint32

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
		// Collect candidate type lookup table
		localCandidates := map[string]string{} // id → candidateType
		for _, stat := range stats {
			if cs, ok := stat.(pionwebrtc.ICECandidateStats); ok {
				localCandidates[cs.ID] = cs.CandidateType.String()
			}
		}

		for _, stat := range stats {
			// Use ICE candidate pair stats for loss and RTT
			if pairStats, ok := stat.(pionwebrtc.ICECandidatePairStats); ok {
				// Only use the nominated (active) pair
				if !pairStats.Nominated {
					continue
				}

				// RTT from ICE (more accurate than app-level ping)
				m.statsMu.Lock()
				if pairStats.CurrentRoundTripTime > 0 {
					m.lastRTT = time.Duration(pairStats.CurrentRoundTripTime * float64(time.Second))
					metrics.RecordRTT(int(m.lastRTT.Milliseconds()))
				}

				// Track connection type from local candidate
				if ct, ok := localCandidates[pairStats.LocalCandidateID]; ok {
					m.connectionType = ct
				}

				// Track cumulative bytes for TURN usage monitoring
				m.totalBytesSent = uint64(pairStats.BytesSent)
				m.totalBytesReceived = uint64(pairStats.BytesReceived)

				// Calculate loss from sent vs received delta
				sent := pairStats.PacketsSent
				received := pairStats.PacketsReceived
				if prevPacketsSent > 0 && sent > prevPacketsSent {
					deltaSent := sent - prevPacketsSent
					deltaReceived := received - prevPacketsReceived
					if deltaSent > 0 {
						lost := float64(0)
						if deltaSent > deltaReceived {
							lost = float64(deltaSent - deltaReceived)
						}
						m.lossPct = (lost / float64(deltaSent)) * 100
						if m.lossPct > 100 {
							m.lossPct = 100
						}
					}
				}
				m.statsMu.Unlock()
				prevPacketsSent = sent
				prevPacketsReceived = received
				break // Only process one nominated pair
			}
		}

		// Fallback: also check buffer as secondary congestion signal
		if m.dataChannel != nil {
			buffered := float64(m.dataChannel.BufferedAmount())
			m.statsMu.Lock()
			if buffered > 4*1024*1024 && m.lossPct < 1 {
				// Buffer is very high but ICE says no loss — report minor congestion
				m.lossPct = (buffered / (16 * 1024 * 1024)) * 5
				if m.lossPct > 5 {
					m.lossPct = 5
				}
			}
			m.statsMu.Unlock()
		}
	}
}
