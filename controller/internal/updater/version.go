package updater

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Version represents a semantic version
type Version struct {
	Major int
	Minor int
	Patch int
	Raw   string
}

// ParseVersion parses a version string like "v2.61.4" or "2.61.4"
func ParseVersion(s string) (Version, error) {
	s = strings.TrimPrefix(s, "v")
	s = strings.TrimSpace(s)

	// Match X.Y.Z pattern
	re := regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)`)
	matches := re.FindStringSubmatch(s)
	if matches == nil {
		return Version{}, fmt.Errorf("invalid version format: %s", s)
	}

	major, _ := strconv.Atoi(matches[1])
	minor, _ := strconv.Atoi(matches[2])
	patch, _ := strconv.Atoi(matches[3])

	return Version{
		Major: major,
		Minor: minor,
		Patch: patch,
		Raw:   s,
	}, nil
}

// String returns the version as "vX.Y.Z"
func (v Version) String() string {
	return fmt.Sprintf("v%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// Compare compares two versions
// Returns: -1 if v < other, 0 if v == other, 1 if v > other
func (v Version) Compare(other Version) int {
	if v.Major != other.Major {
		if v.Major < other.Major {
			return -1
		}
		return 1
	}
	if v.Minor != other.Minor {
		if v.Minor < other.Minor {
			return -1
		}
		return 1
	}
	if v.Patch != other.Patch {
		if v.Patch < other.Patch {
			return -1
		}
		return 1
	}
	return 0
}

// IsNewerThan returns true if v is newer than other
func (v Version) IsNewerThan(other Version) bool {
	return v.Compare(other) > 0
}

// IsOlderThan returns true if v is older than other
func (v Version) IsOlderThan(other Version) bool {
	return v.Compare(other) < 0
}
