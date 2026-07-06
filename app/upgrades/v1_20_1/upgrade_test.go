package v1_20_1

import (
	"testing"

	"github.com/cosmos/cosmos-sdk/types/module"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/stretchr/testify/require"
)

// A chain that already ran v1.20.0 carries the EVM module in its version map, so
// v1.20.1 must treat it as already-brought-up (migration-only hotfix path).
func TestEVMAlreadyInitializedTrueWhenEVMPresent(t *testing.T) {
	fromVM := module.VersionMap{
		"auth":              1,
		"bank":              1,
		evmtypes.ModuleName: 1,
	}
	require.True(t, evmAlreadyInitialized(fromVM))
}

// A chain that skipped v1.20.0 (direct 1.12.0 -> 1.20.1 one-hop) has no EVM
// module in its version map, so v1.20.1 must run the full bring-up.
func TestEVMAlreadyInitializedFalseWhenEVMAbsent(t *testing.T) {
	fromVM := module.VersionMap{
		"auth": 1,
		"bank": 1,
	}
	require.False(t, evmAlreadyInitialized(fromVM))
}

func TestEVMAlreadyInitializedFalseForNilMap(t *testing.T) {
	require.False(t, evmAlreadyInitialized(nil))
}
