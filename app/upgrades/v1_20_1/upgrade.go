package v1_20_1

import (
	"context"
	"fmt"

	upgradetypes "cosmossdk.io/x/upgrade/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	appParams "github.com/LumeraProtocol/lumera/app/upgrades/params"
	upgrade_v1_20_0 "github.com/LumeraProtocol/lumera/app/upgrades/v1_20_0"
)

// UpgradeName is the on-chain name used for this upgrade.
const UpgradeName = "v1.20.1"

// evmAlreadyInitialized reports whether the EVM stack was already brought up on
// this chain, using the presence of the EVM (vm) module in the from-version map
// as the signal. v1.20.0 registers all four EVM modules, so a chain that ran it
// carries evmtypes.ModuleName in fromVM; a chain that skipped straight to v1.20.1
// (e.g. a direct 1.12.0 -> 1.20.1 one-hop) does not. Upgrades are atomic, so this
// is consistent with whether the EVM stores were mounted.
func evmAlreadyInitialized(fromVM module.VersionMap) bool {
	_, ok := fromVM[evmtypes.ModuleName]
	return ok
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
		if !evmAlreadyInitialized(fromVM) {
			p.Logger.Info(fmt.Sprintf("Starting upgrade %s: EVM not yet initialized, running full v1.20.0 bring-up", UpgradeName))
			return upgrade_v1_20_0.CreateUpgradeHandler(p)(goCtx, plan, fromVM)
		}

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
