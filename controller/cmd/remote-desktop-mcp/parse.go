package main

import (
	"encoding/json"
	"regexp"
	"strings"
)

// The CLI prints human-oriented text. These parsers are deliberately
// conservative: if any non-empty line does not match the expected shape, the
// adapter falls back to returning the bounded raw text instead of emitting a
// half-parsed structure.

type deviceEntry struct {
	Name     string `json:"name"`
	Platform string `json:"platform"`
	Status   string `json:"status"`
	DeviceID string `json:"device_id"`
}

type supportSessionEntry struct {
	Key       string `json:"key"`
	Status    string `json:"status"`
	ExpiresAt string `json:"expires_at"`
	SessionID string `json:"session_id"`
}

var (
	// Matches: "MyPC (windows) online  ID: 8b0e..."
	deviceLineRe = regexp.MustCompile(`^(.+?) \(([^)]+)\) (online|offline)\s+ID: ([A-Za-z0-9-_]+)$`)
	// Matches: "Key 123456  active expires 14:22  session 8b0e..."
	supportLineRe = regexp.MustCompile(`^Key ([A-Za-z0-9]+)\s+(\S+)\s+expires (\S+)\s+session ([A-Za-z0-9-]+)$`)
)

// parseDevicesList parses `remote-desktop-cli list` output. ok is false when
// the output does not fully match the known format.
func parseDevicesList(stdout string) (entries []deviceEntry, ok bool) {
	for _, raw := range strings.Split(stdout, "\n") {
		line := strings.TrimRight(raw, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		m := deviceLineRe.FindStringSubmatch(line)
		if m == nil {
			return nil, false
		}
		entries = append(entries, deviceEntry{
			Name:     strings.TrimSpace(m[1]),
			Platform: m[2],
			Status:   m[3],
			DeviceID: m[4],
		})
	}
	return entries, true
}

// parseSupportList parses `remote-desktop-cli support-list` output. ok is
// false when the output does not fully match the known format.
func parseSupportList(stdout string) (entries []supportSessionEntry, ok bool) {
	for _, raw := range strings.Split(stdout, "\n") {
		line := strings.TrimRight(raw, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		m := supportLineRe.FindStringSubmatch(line)
		if m == nil {
			return nil, false
		}
		entries = append(entries, supportSessionEntry{
			Key:       m[1],
			Status:    m[2],
			ExpiresAt: m[3],
			SessionID: m[4],
		})
	}
	return entries, true
}

func marshalJSON(v any) ([]byte, error) {
	return json.MarshalIndent(v, "", "  ")
}
