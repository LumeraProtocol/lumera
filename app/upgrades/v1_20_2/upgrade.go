package v1_20_2

import (
	"context"
	"fmt"

	storetypes "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	erc20types "github.com/cosmos/evm/x/erc20/types"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	precisebanktypes "github.com/cosmos/evm/x/precisebank/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	appParams "github.com/LumeraProtocol/lumera/app/upgrades/params"
	upgrade_v1_20_0 "github.com/LumeraProtocol/lumera/app/upgrades/v1_20_0"
	evmigrationtypes "github.com/LumeraProtocol/lumera/x/evmigration/types"
)

// UpgradeName is the on-chain name used for this upgrade.
//
// This constant — not the git tag — is what the governance proposal `--name`,
// the cosmovisor `upgrades/<NAME>/` directory, and `q upgrade applied <NAME>`
// all key off. Keep them identical.
const UpgradeName = "v1.20.2"

// v1.20.2 carries module consensus-version bumps that shipped after v1.20.1 had
// already executed, and doubles as a safe one-hop target for a chain still on
// v1.12.0.
//
// # WHY THIS UPGRADE EXISTS
//
// The EVM-migration continuity work raises x/audit from ConsensusVersion 2 to 3
// and registers the 2->3 migration. RunMigrations only executes from inside an
// upgrade handler, so a version bump needs an upgrade that has NOT yet run on
// the target network in order to land.
//
// Verified live on 2026-07-30:
//
//	lumera-testnet-2   app_version 1.20.1   audit v2   EVM stack present
//	lumera-mainnet-1   app_version 1.12.0   audit v2   EVM stack ABSENT
//
// Testnet had already executed BOTH v1.20.0 and v1.20.1; neither can run again.
// Without this upgrade the binary would declare audit 3 while committed testnet
// state said 2, with no path between them.
//
// # TWO ARRIVAL SHAPES, ONE BINARY
//
// Because mainnet is still pre-EVM, this upgrade must be correct for two very
// different starting states:
//
//	from 1.20.1 (testnet)  EVM stores + modules present -> migrations only
//	from 1.12.0 (mainnet)  nothing present              -> full EVM bring-up
//
// Both are handled by STATE inspection, never by chain-id, so a network that
// takes an unexpected path still converges on the same result. This mirrors
// v1.20.1 deliberately: the two upgrades must not drift.
//
// This shape was not designed up front — it was forced by the Phase 2
// mainnet-shaped devnet rehearsal on 2026-07-30, which produced two successive
// crash-loops on a faithful 1.12.0 replica:
//
//	panic: failed to load latest version: version of store evmigration
//	mismatch root store's version; expected 155 got 0
//
//	panic: error initializing evm coin info: denom metadata aatom could not
//	be found
//
// The first is fixed by StoreUpgrades below, the second by delegating to the
// v1.20.0 bring-up when the EVM stack is absent.
var StoreUpgrades = storetypes.StoreUpgrades{
	Added: []string{
		feemarkettypes.StoreKey,
		precisebanktypes.StoreKey,
		evmtypes.StoreKey,
		erc20types.StoreKey,
		evmigrationtypes.StoreKey,
	},
}

// evmBringUpModules are the four cosmos/evm modules that the v1.20.0 EVM
// bring-up registers atomically. Their presence in fromVM is the signal for
// which arrival shape we are in. Because v1.20.0 registers them together,
// "some but not all present" is not a state any correct upgrade path produces.
var evmBringUpModules = []string{
	evmtypes.ModuleName,
	feemarkettypes.ModuleName,
	precisebanktypes.ModuleName,
	erc20types.ModuleName,
}

func evmModuleState(fromVM module.VersionMap) (present, absent []string) {
	for _, name := range evmBringUpModules {
		if _, ok := fromVM[name]; ok {
			present = append(present, name)
		} else {
			absent = append(absent, name)
		}
	}
	return present, absent
}

// CreateUpgradeHandler returns the state-driven v1.20.2 handler.
//
// When the EVM stack is absent it delegates to the full v1.20.0 bring-up, which
// upserts bank denom metadata, finalizes Lumera EVM params and initializes EVM
// coin info before running migrations. Skipping that on a pre-EVM chain panics
// with "denom metadata aatom could not be found", because cosmos/evm's defaults
// assume the upstream atom denom.
//
// When the EVM stack is present this is a plain migrations-only carrier, which
// is all testnet needs.
func CreateUpgradeHandler(p appParams.AppUpgradeParams) upgradetypes.UpgradeHandler {
	return func(goCtx context.Context, plan upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		present, absent := evmModuleState(fromVM)

		switch {
		case len(present) == 0:
			// Pre-EVM chain (mainnet's position at 1.12.0). Run the full v1.20.0
			// bring-up; the add-only store loader has already mounted the stores.
			p.Logger.Info(fmt.Sprintf("Starting upgrade %s: EVM not yet initialized, running full v1.20.0 bring-up", UpgradeName))
			return upgrade_v1_20_0.CreateUpgradeHandler(p)(goCtx, plan, fromVM)
		case len(absent) > 0:
			// Partial EVM state cannot arise from any correct upgrade path. Neither
			// branch is safe: the bring-up would double-init the present modules,
			// and the migrations-only path would skip param finalization for the
			// absent ones. Fail closed rather than corrupt state.
			return nil, fmt.Errorf(
				"%s: inconsistent EVM module state, refusing to run — present=%v absent=%v; expected all EVM modules present (migrations only) or all absent (full bring-up)",
				UpgradeName, present, absent,
			)
		}

		p.Logger.Info(fmt.Sprintf("Starting upgrade %s: EVM already initialized, running migrations only", UpgradeName))
		ctx := sdk.UnwrapSDKContext(goCtx)

		newVM, err := p.ModuleManager.RunMigrations(ctx, p.Configurator, fromVM)
		if err != nil {
			p.Logger.Error("Failed to run migrations", "error", err)
			return nil, fmt.Errorf("failed to run migrations: %w", err)
		}

		p.Logger.Info(fmt.Sprintf("Successfully completed upgrade %s", UpgradeName))
		return newVM, nil
	}
}
