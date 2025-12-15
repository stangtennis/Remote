package updater

import "testing"

func TestParseVersion(t *testing.T) {
	tests := []struct {
		input    string
		expected Version
		wantErr  bool
	}{
		{"v2.61.4", Version{Major: 2, Minor: 61, Patch: 4}, false},
		{"2.61.4", Version{Major: 2, Minor: 61, Patch: 4}, false},
		{"v1.0.0", Version{Major: 1, Minor: 0, Patch: 0}, false},
		{"v10.20.30", Version{Major: 10, Minor: 20, Patch: 30}, false},
		{"invalid", Version{}, true},
		{"v1.2", Version{}, true},
		{"", Version{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseVersion(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseVersion(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if got.Major != tt.expected.Major || got.Minor != tt.expected.Minor || got.Patch != tt.expected.Patch {
					t.Errorf("ParseVersion(%q) = %v, want %v", tt.input, got, tt.expected)
				}
			}
		})
	}
}

func TestVersionCompare(t *testing.T) {
	tests := []struct {
		v1       string
		v2       string
		expected int
	}{
		{"v2.61.4", "v2.61.4", 0},
		{"v2.61.5", "v2.61.4", 1},
		{"v2.61.4", "v2.61.5", -1},
		{"v2.62.0", "v2.61.9", 1},
		{"v3.0.0", "v2.99.99", 1},
		{"v1.0.0", "v2.0.0", -1},
	}

	for _, tt := range tests {
		t.Run(tt.v1+" vs "+tt.v2, func(t *testing.T) {
			v1, _ := ParseVersion(tt.v1)
			v2, _ := ParseVersion(tt.v2)
			got := v1.Compare(v2)
			if got != tt.expected {
				t.Errorf("Compare(%q, %q) = %d, want %d", tt.v1, tt.v2, got, tt.expected)
			}
		})
	}
}

func TestVersionIsNewerThan(t *testing.T) {
	tests := []struct {
		v1       string
		v2       string
		expected bool
	}{
		{"v2.61.5", "v2.61.4", true},
		{"v2.61.4", "v2.61.5", false},
		{"v2.61.4", "v2.61.4", false},
		{"v3.0.0", "v2.99.99", true},
	}

	for _, tt := range tests {
		t.Run(tt.v1+" newer than "+tt.v2, func(t *testing.T) {
			v1, _ := ParseVersion(tt.v1)
			v2, _ := ParseVersion(tt.v2)
			got := v1.IsNewerThan(v2)
			if got != tt.expected {
				t.Errorf("IsNewerThan(%q, %q) = %v, want %v", tt.v1, tt.v2, got, tt.expected)
			}
		})
	}
}

func TestVersionString(t *testing.T) {
	v := Version{Major: 2, Minor: 61, Patch: 4}
	expected := "v2.61.4"
	if got := v.String(); got != expected {
		t.Errorf("String() = %q, want %q", got, expected)
	}
}
