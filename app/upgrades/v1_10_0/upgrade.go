package v1_10_0

import (
	"context"
	"fmt"

	upgradetypes "cosmossdk.io/x/upgrade/types"

	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types"

	appParams "github.com/LumeraProtocol/lumera/app/upgrades/params"
)

// UpgradeName is the on-chain name used for this upgrade.
const UpgradeName = "v1.10.0"

// CreateUpgradeHandler migrates consensus params from x/params to x/consensus
// and then runs module migrations.
func CreateUpgradeHandler(p appParams.AppUpgradeParams) upgradetypes.UpgradeHandler {
	return func(goCtx context.Context, _ upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		p.Logger.Info(fmt.Sprintf("Starting upgrade %s...", UpgradeName))

		ctx := sdk.UnwrapSDKContext(goCtx)

		if p.ParamsKeeper == nil || p.ConsensusParamsKeeper == nil {
			return nil, fmt.Errorf("%s upgrade requires ParamsKeeper and ConsensusParamsKeeper", UpgradeName)
		}

		// Use the legacy baseapp paramspace to read existing consensus params from x/params.
		// This is required for in-place upgrades where consensus params were historically stored in x/params.
		legacySubspace := p.ParamsKeeper.Subspace(baseapp.Paramspace).WithKeyTable(paramstypes.ConsensusParamsKeyTable())
		// Migrate consensus params into x/consensus (ConsensusParamsKeeper), which is collections-backed in v0.53+.
		if err := baseapp.MigrateParams(ctx, legacySubspace, p.ConsensusParamsKeeper.ParamsStore); err != nil {
			return nil, fmt.Errorf("failed to migrate consensus params: %w", err)
		}

		// Run all module migrations after consensus params have been moved.
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
