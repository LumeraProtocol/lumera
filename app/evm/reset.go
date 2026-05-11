//go:build !test
// +build !test

package evm

import "testing"

// ResetGlobalState is a no-op in production builds.
// In test binaries compiled without -tags=test, cosmos/evm's global singletons
// (coin info, chain config, EIP activators) cannot be reset, causing "already set"
// panics when multiple App instances are created in the same process.
func ResetGlobalState() {
	if testing.Testing() {
		panicTestTagRequired()
	}
}
