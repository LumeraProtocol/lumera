package v1_8_2

import (
	"context"
	"fmt"

	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
)

// UpgradeName is the on-chain name used for this upgrade.
const UpgradeName = "v1.8.2"

// CreateUpgradeHandler creates an upgrade handler for v1_8_2
func CreateUpgradeHandler(
	logger log.Logger,
	mm *module.Manager,
	configurator module.Configurator,
) upgradetypes.UpgradeHandler {
	return func(goCtx context.Context, _ upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		logger.Info(fmt.Sprintf("Starting upgrade %s...", UpgradeName))

		ctx := sdk.UnwrapSDKContext(goCtx)

		// Run module migrations
		logger.Info("Running module migrations...")
		newVM, err := mm.RunMigrations(ctx, configurator, fromVM)
		if err != nil {
			logger.Error("Failed to run migrations", "error", err)
			return nil, fmt.Errorf("failed to run migrations: %w", err)
		}
		logger.Info("Module migrations completed.")

		logger.Info(fmt.Sprintf("Successfully completed upgrade %s", UpgradeName))
		return newVM, nil
	}
}

// StoreUpgrades declares any store additions/deletions for this upgrade.
// No KV store key changes are required for v1.8.2.
var StoreUpgrades = storetypes.StoreUpgrades{
	Added:   []string{},
	Deleted: []string{},
}
