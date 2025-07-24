package v1_6_1

import (
	"context"
	"fmt"
	storetypes "cosmossdk.io/store/types"
	"github.com/LumeraProtocol/lumera/x/action/v1/types"

	"cosmossdk.io/log"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
)

const UpgradeName = "v1.6.1"

// CreateUpgradeHandler creates an upgrade handler for v1_6_1
func CreateUpgradeHandler(
	logger log.Logger,
	mm *module.Manager,
	configurator module.Configurator,
) upgradetypes.UpgradeHandler {
	return func(goCtx context.Context, _ upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		logger.Info(fmt.Sprintf("Starting upgrade %s...", UpgradeName))

		ctx := sdk.UnwrapSDKContext(goCtx)

		// 1. Run Migrations for Existing Modules (if any needed for this upgrade)
		// Use the unwrapped sdk.Context (ctx)
		logger.Info("Running module migrations...")
		newVM, err := mm.RunMigrations(ctx, configurator, fromVM)
		if err != nil {
			logger.Error("Failed to run migrations", "error", err)
			return nil, fmt.Errorf("failed to run migrations: %w", err)
		}
		logger.Info("Module migrations completed.")

		// 3. Add the New Module to the Version Map
		newVM[types.ModuleName] = types.ConsensusVersion

		logger.Info(fmt.Sprintf("Successfully completed upgrade %s", UpgradeName))

		// Return the UPDATED version map
		return newVM, nil
	}
}

var StoreUpgrades = storetypes.StoreUpgrades{
	Added: []string{},
	// Deleted: []string{...},
}
