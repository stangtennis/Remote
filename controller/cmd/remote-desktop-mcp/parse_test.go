package main

import (
	"encoding/json"
	"testing"
)

func TestParseDevicesList(t *testing.T) {
	stdout := "SERVER (windows) online  ID: 8b0e6a5e-1111\nwin11dl (windows) offline  ID: dev-2\n\n"
	entries, ok := parseDevicesList(stdout)
	if !ok {
		t.Fatal("expected successful parse")
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	want := deviceEntry{Name: "SERVER", Platform: "windows", Status: "online", DeviceID: "8b0e6a5e-1111"}
	if entries[0] != want {
		t.Fatalf("entries[0] = %+v, want %+v", entries[0], want)
	}
	if entries[1].Status != "offline" {
		t.Fatalf("entries[1].Status = %q, want offline", entries[1].Status)
	}
}

func TestParseDevicesListEmpty(t *testing.T) {
	entries, ok := parseDevicesList("")
	if !ok || len(entries) != 0 {
		t.Fatalf("empty output should parse to zero entries: %+v ok=%v", entries, ok)
	}
}

func TestParseDevicesListRejectsUnknownLines(t *testing.T) {
	for name, stdout := range map[string]string{
		"human text":      "No devices registered.",
		"malformed":       "SERVER (windows) maybe  ID: abc",
		"missing id":      "SERVER (windows) online",
		"unexpected json": `{"devices":[]}`,
	} {
		if _, ok := parseDevicesList(stdout); ok {
			t.Errorf("%s: expected conservative parse failure", name)
		}
	}
}

func TestParseSupportList(t *testing.T) {
	stdout := "Key 123456  active expires 14:22  session 8b0e6a5e-2222\nKey abc129  pending expires 09:01  session 8b0e6a5e-3333\n"
	entries, ok := parseSupportList(stdout)
	if !ok {
		t.Fatal("expected successful parse")
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	want := supportSessionEntry{Key: "123456", Status: "active", ExpiresAt: "14:22", SessionID: "8b0e6a5e-2222"}
	if entries[0] != want {
		t.Fatalf("entries[0] = %+v, want %+v", entries[0], want)
	}
}

func TestParseSupportListRejectsUnknownLines(t *testing.T) {
	for name, stdout := range map[string]string{
		"human text": "No active AI support clients.",
		"malformed":  "Key 123456 active session abc",
	} {
		if _, ok := parseSupportList(stdout); ok {
			t.Errorf("%s: expected conservative parse failure", name)
		}
	}
}

func TestMarshalJSONRoundTrip(t *testing.T) {
	data, err := marshalJSON([]deviceEntry{{Name: "SERVER", Platform: "windows", Status: "online", DeviceID: "abc-1"}})
	if err != nil {
		t.Fatal(err)
	}
	var got []deviceEntry
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].DeviceID != "abc-1" {
		t.Fatalf("round trip mismatch: %+v", got)
	}
}
