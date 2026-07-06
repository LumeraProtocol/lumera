package v1_20_1

import (
	"testing"

	"github.com/cosmos/cosmos-sdk/types/module"
	erc20types "github.com/cosmos/evm/x/erc20/types"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	precisebanktypes "github.com/cosmos/evm/x/precisebank/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/stretchr/testify/require"
)

// allEVMModules is a fromVM entry for every module the v1.20.0 bring-up registers.
func allEVMModules() module.VersionMap {
	vm := module.VersionMap{"auth": 1, "bank": 1}
	for _, name := range evmBringUpModules {
		vm[name] = 1
	}
	return vm
}

// A chain that ran v1.20.0 carries all four EVM modules -> nothing absent
// (migration-only hotfix path).
func TestEVMModuleStateAllPresent(t *testing.T) {
	present, absent := evmModuleState(allEVMModules())
	require.ElementsMatch(t, evmBringUpModules, present)
	require.Empty(t, absent)
}

// A direct 1.12.0 -> 1.20.1 one-hop carries no EVM modules -> all absent
// (full bring-up path).
func TestEVMModuleStateAllAbsent(t *testing.T) {
	present, absent := evmModuleState(module.VersionMap{"auth": 1, "bank": 1})
	require.Empty(t, present)
	require.ElementsMatch(t, evmBringUpModules, absent)
}

// Partial state (some EVM modules present, others not) must be detectable so the
// handler can fail closed rather than silently skip param finalization.
func TestEVMModuleStatePartial(t *testing.T) {
	fromVM := module.VersionMap{
		"auth":                    1,
		evmtypes.ModuleName:       1,
		feemarkettypes.ModuleName: 1,
		// precisebank and erc20 intentionally absent.
	}
	present, absent := evmModuleState(fromVM)
	require.ElementsMatch(t, []string{evmtypes.ModuleName, feemarkettypes.ModuleName}, present)
	require.ElementsMatch(t, []string{precisebanktypes.ModuleName, erc20types.ModuleName}, absent)
}
