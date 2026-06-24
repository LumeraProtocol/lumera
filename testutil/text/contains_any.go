package text

import "strings"

// ContainsAny reports whether value contains any of the given needles.
func ContainsAny(value string, needles ...string) bool {
	for _, n := range needles {
		if strings.Contains(value, n) {
			return true
		}
	}
	return false
}
