//go:build test

package integration_test

import (
	"os"
	"testing"
)

// Companion to testtag_required_test.go. When the 'test' build tag is
// present, install a passthrough TestMain that simply runs the package's
// tests. (The no-tag variant fails fast to flag the missing tag.)
func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
