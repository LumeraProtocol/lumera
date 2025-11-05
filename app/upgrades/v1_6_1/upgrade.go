package v1_6_1

import (
	"context"
	"fmt"

	upgradetypes "cosmossdk.io/x/upgrade/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"

	appParams "github.com/LumeraProtocol/lumera/app/upgrades/params"
	"github.com/LumeraProtocol/lumera/x/action/v1/types"
)

const UpgradeName = "v1.6.1"

// CreateUpgradeHandler creates an upgrade handler for v1_6_1
func CreateUpgradeHandler(p appParams.AppUpgradeParams) upgradetypes.UpgradeHandler {
	return func(goCtx context.Context, _ upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		p.Logger.Info(fmt.Sprintf("Starting upgrade %s...", UpgradeName))

		ctx := sdk.UnwrapSDKContext(goCtx)

		// 1. Run Migrations for Existing Modules (if any needed for this upgrade)
		// Use the unwrapped sdk.Context (ctx)
		p.Logger.Info("Running module migrations...")
		newVM, err := p.ModuleManager.RunMigrations(ctx, p.Configurator, fromVM)
		if err != nil {
			p.Logger.Error("Failed to run migrations", "error", err)
			return nil, fmt.Errorf("failed to run migrations: %w", err)
		}
		p.Logger.Info("Module migrations completed.")

		// 3. Add the New Module to the Version Map
		newVM[types.ModuleName] = types.ConsensusVersion

		p.Logger.Info(fmt.Sprintf("Successfully completed upgrade %s", UpgradeName))

		// Return the UPDATED version map
		return newVM, nil
	}
}
