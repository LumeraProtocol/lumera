package version

import (
	"strconv"
	"strings"
)

// GTE reports whether the semantic version current is greater than or equal to
// floor.  Both values may carry a leading "v"/"V", pre-release suffixes
// ("-rc1"), and build metadata ("+build1") — these are stripped before
// comparison.  If either string is not a valid semver, the function falls back
// to a case-insensitive string comparison.
func GTE(current, floor string) bool {
	cMaj, cMin, cPatch, okC := Parse(current)
	fMaj, fMin, fPatch, okF := Parse(floor)
	if !okC || !okF {
		return strings.EqualFold(strings.TrimSpace(current), strings.TrimSpace(floor))
	}
	if cMaj != fMaj {
		return cMaj > fMaj
	}
	if cMin != fMin {
		return cMin > fMin
	}
	return cPatch >= fPatch
}

// Parse extracts major, minor, and patch integers from a semver string.
// It returns false if the string cannot be parsed.  Leading "v"/"V",
// pre-release suffixes ("-…"), and build metadata ("+…") are stripped.
func Parse(v string) (major, minor, patch int, ok bool) {
	norm := strings.TrimSpace(v)
	norm = strings.TrimPrefix(strings.TrimPrefix(norm, "v"), "V")
	if idx := strings.Index(norm, "-"); idx >= 0 {
		norm = norm[:idx]
	}
	if idx := strings.Index(norm, "+"); idx >= 0 {
		norm = norm[:idx]
	}
	parts := strings.Split(norm, ".")
	if len(parts) < 2 {
		return 0, 0, 0, false
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, 0, false
	}
	minor, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, 0, false
	}
	if len(parts) > 2 {
		patch, err = strconv.Atoi(parts[2])
		if err != nil {
			return 0, 0, 0, false
		}
	}
	return major, minor, patch, true
}
