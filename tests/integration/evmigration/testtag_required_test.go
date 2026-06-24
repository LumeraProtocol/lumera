//go:build !test

package integration_test

import (
	"fmt"
	"os"
	"testing"
)

// Without the 'test' build tag, app.Setup hits the cosmos-evm chainConfig
// guard and every subtest in the integration suite silently calls t.Skip
// (see app/test_helpers.go: runOrSkipEVMTestTag). `go test
// ./tests/integration/evmigration` then reports "ok" with zero coverage
// of the actual integration assertions — the multisig and mirror-source
// invariants we ship are unverified.
//
// This file is compiled only when the 'test' tag is ABSENT. Its TestMain
// fails the test binary before any subtest runs, surfacing the
// configuration error as a hard CI failure rather than a silent pass.
//
// To run the integration suite correctly:
//
//	go test -tags='integration test' ./tests/integration/evmigration/...
//
// The companion file testtag_required_present_test.go (built when the
// tag IS present) installs the canonical TestMain that lets the suite run.
func TestMain(m *testing.M) {
	fmt.Fprintln(os.Stderr,
		"integration/evmigration tests require the 'test' build tag — "+
			"the cosmos-evm chainConfig guard otherwise silently skips every subtest.\n"+
			"Run: go test -tags='integration test' ./tests/integration/evmigration/...")
	os.Exit(1)
}
