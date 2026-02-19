//go:build !test
// +build !test

package evm_test

import (
	"testing"

	"github.com/LumeraProtocol/lumera/app/evm"
	"github.com/stretchr/testify/require"
)

// TestResetGlobalStateRequiresTestTag documents the production-build guard:
// test binaries built without `-tags=test` must panic with guidance.
func TestResetGlobalStateRequiresTestTag(t *testing.T) {
	defer func() {
		recovered := recover()
		require.True(t, evm.IsTestTagRequiredPanic(recovered))

		err, ok := recovered.(error)
		require.True(t, ok)
		require.Equal(t, evm.TestTagRequiredMessage(), err.Error())
	}()

	evm.ResetGlobalState()
}

// TestSetKeeperDefaultsRequiresTestTag documents the same guard for keeper
// defaults initialization in non-test-tag builds.
func TestSetKeeperDefaultsRequiresTestTag(t *testing.T) {
	defer func() {
		recovered := recover()
		require.True(t, evm.IsTestTagRequiredPanic(recovered))

		err, ok := recovered.(error)
		require.True(t, ok)
		require.Equal(t, evm.TestTagRequiredMessage(), err.Error())
	}()

	// Panic is triggered before keeper access, so nil is fine here.
	evm.SetKeeperDefaults(nil)
}
