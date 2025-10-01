package v1_7_2

import (
    "context"
    "fmt"

    "cosmossdk.io/log"
    storetypes "cosmossdk.io/store/types"
    upgradetypes "cosmossdk.io/x/upgrade/types"

    sdk "github.com/cosmos/cosmos-sdk/types"
    "github.com/cosmos/cosmos-sdk/types/module"
)

// UpgradeName is the on-chain name used for this combined upgrade.
const UpgradeName = "v1.7.2"

// CreateUpgradeHandler creates the upgrade handler for v1_7_2 which
// combines all updates targeted for the 1.7.2 release.
func CreateUpgradeHandler(
    logger log.Logger,
    mm *module.Manager,
    configurator module.Configurator,
) upgradetypes.UpgradeHandler {
    return func(goCtx context.Context, _ upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
        logger.Info(fmt.Sprintf("Starting upgrade %s...", UpgradeName))

        // Unwrap the SDK context from the Go context
        ctx := sdk.UnwrapSDKContext(goCtx)

        // Run module migrations to bring all modules to their latest consensus versions
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
// No KV store key changes are required for v1.7.2 at this time.
// Note: v1.7.2 includes a module migration in x/action that converts
// legacy Action.price (string) to cosmos.base.v1beta1.Coin and bumps
// the module consensus version. Supernode protobuf additions are
// backward compatible and do not require a store upgrade.
var StoreUpgrades = storetypes.StoreUpgrades{
    Added:   []string{},
    Deleted: []string{},
}
