package v1_20_2

import (
	"testing"

	"cosmossdk.io/log"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	erc20types "github.com/cosmos/evm/x/erc20/types"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	precisebanktypes "github.com/cosmos/evm/x/precisebank/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/stretchr/testify/require"

	appParams "github.com/LumeraProtocol/lumera/app/upgrades/params"
	upgrade_v1_20_0 "github.com/LumeraProtocol/lumera/app/upgrades/v1_20_0"
	evmigrationtypes "github.com/LumeraProtocol/lumera/x/evmigration/types"
)

// TestUpgradeName pins the on-chain identifier. The governance proposal --name,
// the cosmovisor upgrades/<NAME>/ directory, and `q upgrade applied <NAME>` all
// key off this exact string, so a typo here is an operational outage.
func TestUpgradeName(t *testing.T) {
	require.Equal(t, "v1.20.2", UpgradeName)
}

// TestEvmModuleStateArrivalShapes covers the two shapes this one binary must
// serve, plus the inconsistent shape it must refuse.
//
// Live state on 2026-08-01: mainnet (1.12.0) carries none of these modules,
// testnet (1.20.1) carries all four.
func TestEvmModuleStateArrivalShapes(t *testing.T) {
	t.Run("pre-EVM chain (mainnet at 1.12.0) reports all absent", func(t *testing.T) {
		present, absent := evmModuleState(module.VersionMap{
			"bank":      1,
			"staking":   5,
			"audit":     2,
			"supernode": 1,
		})
		require.Empty(t, present, "a pre-EVM chain must report no EVM modules present")
		require.Len(t, absent, 4, "all four EVM bring-up modules must be reported absent")
	})

	t.Run("EVM chain (testnet at 1.20.1) reports all present", func(t *testing.T) {
		present, absent := evmModuleState(module.VersionMap{
			evmtypes.ModuleName:         1,
			feemarkettypes.ModuleName:   1,
			precisebanktypes.ModuleName: 1,
			erc20types.ModuleName:       1,
			"audit":                     2,
		})
		require.Len(t, present, 4, "a chain that ran the bring-up must report all four present")
		require.Empty(t, absent)
	})

	t.Run("partial EVM state is reported as mixed so the handler can fail closed", func(t *testing.T) {
		present, absent := evmModuleState(module.VersionMap{
			evmtypes.ModuleName:       1,
			feemarkettypes.ModuleName: 1,
		})
		require.NotEmpty(t, present)
		require.NotEmpty(t, absent)
	})
}

// TestPartialEVMStateFailsClosed proves the handler REFUSES an inconsistent
// arrival shape rather than guessing a branch.
//
// Partial EVM state cannot arise from any correct upgrade path (v1.20.0
// registers all four modules atomically), so reaching it means something is
// already wrong. Neither branch is safe from there: the bring-up would
// double-initialize the modules that exist, and the migrations-only path would
// skip param finalization for the ones that do not. Halting the upgrade is the
// only correct action.
//
// This test carries the assertion on its own -- with it removed, deleting the
// fail-closed branch entirely still passes every other test in the package.
func TestPartialEVMStateFailsClosed(t *testing.T) {
	params := appParams.AppUpgradeParams{
		ChainID: "lumera-mainnet-1",
		Logger:  log.NewNopLogger(),
	}

	partial := module.VersionMap{
		evmtypes.ModuleName:       1,
		feemarkettypes.ModuleName: 1,
		// precisebank and erc20 deliberately missing.
	}

	vm, err := CreateUpgradeHandler(params)(
		sdk.WrapSDKContext(sdk.NewContext(nil, tmproto.Header{ChainID: "lumera-mainnet-1"}, false, log.NewNopLogger())),
		upgradetypes.Plan{},
		partial,
	)

	require.Error(t, err, "partial EVM module state must abort the upgrade")
	require.Nil(t, vm, "a refused upgrade must not return a version map")
	require.Contains(t, err.Error(), "inconsistent EVM module state")
	require.Contains(t, err.Error(), UpgradeName)
	require.Contains(t, err.Error(), precisebanktypes.ModuleName,
		"the error must name the modules that are missing so an operator can act on it")
}

// TestStoreUpgradesCoverEVMBringUp asserts v1.20.2 declares the SAME store set
// as the v1.20.0 bring-up rather than a hand-copied list that can drift.
//
// evmigration in particular must be present: a direct 1.12.0 -> 1.20.2 one-hop
// mounts that store for the first time, and omitting it panics at load with
// "version of store evmigration mismatch root store's version".
func TestStoreUpgradesCoverEVMBringUp(t *testing.T) {
	require.Equal(t, upgrade_v1_20_0.StoreUpgrades, StoreUpgrades,
		"v1.20.2 must reuse the v1.20.0 store declaration, not a divergent copy")

	require.Contains(t, StoreUpgrades.Added, evmigrationtypes.StoreKey)
	require.Contains(t, StoreUpgrades.Added, evmtypes.StoreKey)
	require.Contains(t, StoreUpgrades.Added, feemarkettypes.StoreKey)
	require.Contains(t, StoreUpgrades.Added, precisebanktypes.StoreKey)
	require.Contains(t, StoreUpgrades.Added, erc20types.StoreKey)

	require.Empty(t, StoreUpgrades.Deleted, "the upgrade must never delete a store")
	require.Empty(t, StoreUpgrades.Renamed, "the upgrade must never rename a store")
}
