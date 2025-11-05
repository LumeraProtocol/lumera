package v1_7_2

import (
    "context"
    "fmt"

    upgradetypes "cosmossdk.io/x/upgrade/types"

    sdk "github.com/cosmos/cosmos-sdk/types"
    "github.com/cosmos/cosmos-sdk/types/module"

    appParams "github.com/LumeraProtocol/lumera/app/upgrades/params"
)

// UpgradeName is the on-chain name used for this combined upgrade.
const UpgradeName = "v1.7.2"

// CreateUpgradeHandler creates the upgrade handler for v1_7_2 which
// combines all updates targeted for the 1.7.2 release.
func CreateUpgradeHandler(p appParams.AppUpgradeParams) upgradetypes.UpgradeHandler {
    return func(goCtx context.Context, _ upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
        p.Logger.Info(fmt.Sprintf("Starting upgrade %s...", UpgradeName))

        // Unwrap the SDK context from the Go context
        ctx := sdk.UnwrapSDKContext(goCtx)

        // Run module migrations to bring all modules to their latest consensus versions
        p.Logger.Info("Running module migrations...")
        newVM, err := p.ModuleManager.RunMigrations(ctx, p.Configurator, fromVM)
        if err != nil {
            p.Logger.Error("Failed to run migrations", "error", err)
            return nil, fmt.Errorf("failed to run migrations: %w", err)
        }
        p.Logger.Info("Module migrations completed.")

        p.Logger.Info(fmt.Sprintf("Successfully completed upgrade %s", UpgradeName))
        return newVM, nil
    }
}
