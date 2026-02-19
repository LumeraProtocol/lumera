package text

import "strings"

// ContainsAny reports whether value contains at least one of the provided
// substring needles.
func ContainsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}
