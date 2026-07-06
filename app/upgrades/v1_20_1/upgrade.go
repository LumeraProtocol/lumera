package v1_20_1

import (
	"context"
	"fmt"

	upgradetypes "cosmossdk.io/x/upgrade/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	erc20types "github.com/cosmos/evm/x/erc20/types"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	precisebanktypes "github.com/cosmos/evm/x/precisebank/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	appParams "github.com/LumeraProtocol/lumera/app/upgrades/params"
	upgrade_v1_20_0 "github.com/LumeraProtocol/lumera/app/upgrades/v1_20_0"
)

// UpgradeName is the on-chain name used for this upgrade.
const UpgradeName = "v1.20.1"

// evmBringUpModules are the four cosmos/evm modules that the v1.20.0 EVM bring-up
// registers together (see v1_20_0.CreateUpgradeHandler). They are the signal for
// whether the bring-up already ran: a chain that ran v1.20.0 carries all four in
// fromVM; a chain that skipped straight to v1.20.1 (a direct 1.12.0 -> 1.20.1
// one-hop) carries none. Because v1.20.0 registers them atomically, "some but not
// all present" is not a state any correct upgrade path can produce.
var evmBringUpModules = []string{
	evmtypes.ModuleName,
	feemarkettypes.ModuleName,
	precisebanktypes.ModuleName,
	erc20types.ModuleName,
}

// evmModuleState partitions evmBringUpModules by their presence in fromVM.
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

// CreateUpgradeHandler returns a state-driven v1.20.1 handler. When the chain has
// not yet initialized the EVM stack it delegates to the full v1.20.0 EVM bring-up
// (params finalization, coin info, ERC20 policy, migration_end_time, and the
// InitGenesis skip). When the EVM stack is already present it is a plain
// migration-only hotfix. This replaces the previous IsMainnet-based routing, so a
// direct 1.12.0 -> 1.20.1 upgrade works on any network. The matching add-only
// store loader mounts the EVM stores when absent, so the bring-up path has them.
func CreateUpgradeHandler(p appParams.AppUpgradeParams) upgradetypes.UpgradeHandler {
	return func(goCtx context.Context, plan upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		present, absent := evmModuleState(fromVM)

		switch {
		case len(present) == 0:
			// No EVM modules yet — this is a direct one-hop onto v1.20.1. Run the
			// full v1.20.0 bring-up (the add-only store loader mounts the stores).
			p.Logger.Info(fmt.Sprintf("Starting upgrade %s: EVM not yet initialized, running full v1.20.0 bring-up", UpgradeName))
			return upgrade_v1_20_0.CreateUpgradeHandler(p)(goCtx, plan, fromVM)
		case len(absent) > 0:
			// Partial EVM state cannot arise from any correct upgrade path (v1.20.0
			// registers all four modules atomically). Neither branch is safe here:
			// the bring-up path would double-init the present modules, and the hotfix
			// path would skip param finalization for the absent ones. Fail closed.
			return nil, fmt.Errorf(
				"%s: inconsistent EVM module state, refusing to run — present=%v absent=%v; expected all EVM modules present (migration-only hotfix) or all absent (full bring-up)",
				UpgradeName, present, absent,
			)
		}

		// All EVM modules present: the chain already ran v1.20.0, so v1.20.1 is a
		// plain migration-only hotfix.
		p.Logger.Info(fmt.Sprintf("Starting upgrade %s: EVM already initialized, running migration-only hotfix", UpgradeName))
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
