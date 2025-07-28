package v1_8_0

import (
	"context"
	"fmt"

	storetypes "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"cosmossdk.io/log"
)

const UpgradeName = "v1.8.0"

// CreateUpgradeHandler creates an upgrade handler for v1_8_0
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

		// You may add more custom upgrade logic here (if needed)

		logger.Info(fmt.Sprintf("Successfully completed upgrade %s", UpgradeName))
		return newVM, nil
	}
}

var StoreUpgrades = storetypes.StoreUpgrades{
	// No new store keys needed if you are only updating existing modules
	Added:   []string{},
	Deleted: []string{
		"nft",	
	},
}