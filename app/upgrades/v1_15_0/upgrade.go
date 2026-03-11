package v1_15_0

import (
	"context"
	"fmt"

	upgradetypes "cosmossdk.io/x/upgrade/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"

	consensusparams "github.com/LumeraProtocol/lumera/app/upgrades/internal/consensusparams"
	appParams "github.com/LumeraProtocol/lumera/app/upgrades/params"
)

// UpgradeName is the on-chain name used for this upgrade.
const UpgradeName = "v1.15.0"

// CreateUpgradeHandler adds the Everlight store and initializes its params
// via RunMigrations (the module's InitGenesis sets default params automatically).
func CreateUpgradeHandler(p appParams.AppUpgradeParams) upgradetypes.UpgradeHandler {
	return func(goCtx context.Context, _ upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		p.Logger.Info(fmt.Sprintf("Starting upgrade %s...", UpgradeName))

		ctx := sdk.UnwrapSDKContext(goCtx)

		if err := consensusparams.EnsurePresent(ctx, p, UpgradeName); err != nil {
			return nil, err
		}

		// Run all module migrations after consensus params have been verified.
		// This triggers Everlight's InitGenesis which sets default params.
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
