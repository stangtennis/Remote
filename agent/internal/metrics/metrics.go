// Package metrics exposes Prometheus metrics for the agent.
//
// Metrics are only collected/exposed when RD_METRICS_ENABLED=true.
// The HTTP server listens on RD_METRICS_ADDR (default: 127.0.0.1:9090).
// Default bind to 127.0.0.1 avoids accidental exposure over LAN/WAN.
package metrics

import (
	"context"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"
)

var (
	// FramesTotal counts frames encoded and sent, labeled by codec.
	FramesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rd_frames_total",
			Help: "Total number of frames encoded and sent to controllers, by codec.",
		},
		[]string{"codec"}, // jpeg, h264
	)

	// FrameEncodeDuration observes encode duration in seconds.
	FrameEncodeDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "rd_frame_encode_duration_seconds",
			Help:    "Time spent encoding a single frame.",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5},
		},
		[]string{"codec"},
	)

	// WebRTCRTT is a gauge for current WebRTC RTT in milliseconds.
	WebRTCRTT = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "rd_webrtc_rtt_ms",
			Help: "Current WebRTC round-trip time in milliseconds.",
		},
	)

	// ReconnectsTotal counts WebRTC reconnection attempts.
	ReconnectsTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "rd_reconnects_total",
			Help: "Total number of WebRTC reconnection attempts.",
		},
	)

	// ActiveSessions is the current number of active controller sessions.
	ActiveSessions = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "rd_active_sessions",
			Help: "Number of currently active controller sessions.",
		},
	)

	// PrivacyModeEnabled is 1 when privacy mode (black frame) is on, else 0.
	PrivacyModeEnabled = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "rd_privacy_mode_enabled",
			Help: "1 if privacy mode is enabled (controller sees black frames), 0 otherwise.",
		},
	)

	// BytesSentTotal counts bytes sent to controllers, by channel type.
	BytesSentTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rd_bytes_sent_total",
			Help: "Total bytes sent to controllers.",
		},
		[]string{"channel"}, // video, data, file
	)

	started atomic.Bool
)

// Enabled returns true when the RD_METRICS_ENABLED env var is truthy.
func Enabled() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("RD_METRICS_ENABLED")))
	return v == "1" || v == "true" || v == "yes"
}

// Init registers all metrics and starts the HTTP server when enabled.
// Safe to call multiple times; subsequent calls are no-ops.
func Init(ctx context.Context) {
	if !Enabled() {
		return
	}
	if !started.CompareAndSwap(false, true) {
		return
	}

	reg := prometheus.NewRegistry()
	reg.MustRegister(
		FramesTotal,
		FrameEncodeDuration,
		WebRTCRTT,
		ReconnectsTotal,
		ActiveSessions,
		PrivacyModeEnabled,
		BytesSentTotal,
	)

	addr := os.Getenv("RD_METRICS_ADDR")
	if addr == "" {
		addr = "127.0.0.1:9090"
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	}))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Info().Str("addr", addr).Msg("metrics server listening on /metrics")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error().Err(err).Msg("metrics server stopped")
		}
	}()

	// Graceful shutdown
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
}

// TimeFrameEncode measures encoding duration and records to FrameEncodeDuration.
// Usage:
//
//	defer metrics.TimeFrameEncode("jpeg")()
func TimeFrameEncode(codec string) func() {
	if !Enabled() {
		return func() {}
	}
	start := time.Now()
	return func() {
		FrameEncodeDuration.WithLabelValues(codec).Observe(time.Since(start).Seconds())
		FramesTotal.WithLabelValues(codec).Inc()
	}
}

// RecordRTT updates the WebRTC RTT gauge (in milliseconds).
func RecordRTT(rttMs int) {
	if !Enabled() {
		return
	}
	WebRTCRTT.Set(float64(rttMs))
}

// RecordReconnect increments the reconnect counter.
func RecordReconnect() {
	if !Enabled() {
		return
	}
	ReconnectsTotal.Inc()
}

// SetActiveSessions updates the active session count.
func SetActiveSessions(n int) {
	if !Enabled() {
		return
	}
	ActiveSessions.Set(float64(n))
}

// SetPrivacyMode reflects the privacy toggle state.
func SetPrivacyMode(on bool) {
	if !Enabled() {
		return
	}
	if on {
		PrivacyModeEnabled.Set(1)
	} else {
		PrivacyModeEnabled.Set(0)
	}
}

// AddBytesSent records bytes sent on a channel (video, data, file).
func AddBytesSent(channel string, n int) {
	if !Enabled() || n <= 0 {
		return
	}
	BytesSentTotal.WithLabelValues(channel).Add(float64(n))
}
