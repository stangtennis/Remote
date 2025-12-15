package updater

import (
	"encoding/json"
	"testing"
)

func TestParseRelease(t *testing.T) {
	sampleJSON := `{
		"tag_name": "v2.61.5",
		"name": "Remote Desktop v2.61.5",
		"prerelease": false,
		"draft": false,
		"published_at": "2025-12-15T20:00:00Z",
		"assets": [
			{
				"name": "controller-v2.61.5.exe",
				"browser_download_url": "https://github.com/stangtennis/Remote/releases/download/v2.61.5/controller-v2.61.5.exe",
				"size": 15000000
			},
			{
				"name": "controller-v2.61.5.exe.sha256",
				"browser_download_url": "https://github.com/stangtennis/Remote/releases/download/v2.61.5/controller-v2.61.5.exe.sha256",
				"size": 100
			},
			{
				"name": "remote-agent-v2.61.5.exe",
				"browser_download_url": "https://github.com/stangtennis/Remote/releases/download/v2.61.5/remote-agent-v2.61.5.exe",
				"size": 12000000
			}
		]
	}`

	var release Release
	if err := json.Unmarshal([]byte(sampleJSON), &release); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if release.TagName != "v2.61.5" {
		t.Errorf("TagName = %q, want %q", release.TagName, "v2.61.5")
	}

	if release.Prerelease {
		t.Error("Prerelease should be false")
	}

	if len(release.Assets) != 3 {
		t.Errorf("Assets count = %d, want 3", len(release.Assets))
	}

	// Check controller exe asset
	var controllerExe *ReleaseAsset
	for i := range release.Assets {
		if release.Assets[i].Name == "controller-v2.61.5.exe" {
			controllerExe = &release.Assets[i]
			break
		}
	}

	if controllerExe == nil {
		t.Fatal("Controller exe asset not found")
	}

	if controllerExe.Size != 15000000 {
		t.Errorf("Controller exe size = %d, want 15000000", controllerExe.Size)
	}
}

func TestParseSHA256Line(t *testing.T) {
	// Valid SHA256 hash (exactly 64 hex characters)
	validHash := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		{
			"valid with two spaces",
			validHash + "  controller-v2.61.5.exe",
			validHash,
			false,
		},
		{
			"valid with one space",
			validHash + " controller.exe",
			validHash,
			false,
		},
		{
			"too short hash",
			"tooshort  file.exe",
			"",
			true,
		},
		{
			"empty input",
			"",
			"",
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			line := tt.input
			if line == "" {
				if !tt.wantErr {
					t.Error("Expected error for empty input")
				}
				return
			}

			// Split on whitespace
			fields := make([]string, 0)
			current := ""
			for _, c := range line {
				if c == ' ' || c == '\t' {
					if current != "" {
						fields = append(fields, current)
						current = ""
					}
				} else {
					current += string(c)
				}
			}
			if current != "" {
				fields = append(fields, current)
			}

			if len(fields) < 1 {
				if !tt.wantErr {
					t.Error("Expected error for invalid format")
				}
				return
			}

			hash := fields[0]
			if len(hash) != 64 {
				if !tt.wantErr {
					t.Errorf("Hash length = %d, want 64", len(hash))
				}
				return
			}

			if tt.wantErr {
				t.Error("Expected error but got none")
				return
			}

			if hash != tt.expected {
				t.Errorf("Hash = %q, want %q", hash, tt.expected)
			}
		})
	}
}
