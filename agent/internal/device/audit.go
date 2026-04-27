package device

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// AuditEvent describes a single audit_logs row written by the agent.
type AuditEvent struct {
	Event    string                 `json:"event"`
	Severity string                 `json:"severity,omitempty"` // info | warning | error
	Details  map[string]interface{} `json:"details,omitempty"`
}

// WriteAudit posts an entry to the public.audit_logs table. Failures are
// logged locally to ~/.remote-desktop/audit.log on the agent host but never
// returned — auditing is fire-and-forget.
func (d *Device) WriteAudit(ev AuditEvent) {
	go d.writeAuditSync(ev)
}

func (d *Device) writeAuditSync(ev AuditEvent) {
	if d == nil || d.cfg == nil {
		return
	}
	severity := ev.Severity
	if severity == "" {
		severity = "info"
	}
	payload := map[string]interface{}{
		"device_id": d.ID,
		"event":     ev.Event,
		"severity":  severity,
		"details":   ev.Details,
	}
	body, _ := json.Marshal(payload)

	url := fmt.Sprintf("%s/rest/v1/audit_logs", d.cfg.SupabaseURL)
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		appendLocalAudit(payload)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("apikey", d.cfg.SupabaseAnonKey)

	if d.tokenProvider != nil {
		if tok, err := d.tokenProvider.GetToken(); err == nil && tok != "" {
			req.Header.Set("Authorization", "Bearer "+tok)
		}
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("⚠️ audit POST failed: %v — falling back to local log", err)
		appendLocalAudit(payload)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("⚠️ audit POST status=%d body=%s", resp.StatusCode, string(body))
		appendLocalAudit(payload)
	}
}

// appendLocalAudit writes a JSON line to ~/.remote-desktop/audit.log so that
// audit history survives Supabase outages.
func appendLocalAudit(payload map[string]interface{}) {
	home, _ := os.UserHomeDir()
	if home == "" {
		return
	}
	dir := filepath.Join(home, ".remote-desktop")
	_ = os.MkdirAll(dir, 0700)
	logPath := filepath.Join(dir, "audit.log")

	payload["ts"] = time.Now().UTC().Format(time.RFC3339)
	line, _ := json.Marshal(payload)
	line = append(line, '\n')

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(line)
}
