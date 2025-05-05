package v1_0_0

import (
	"context"
	storetypes "cosmossdk.io/store/types"
	"fmt"
	actionkeeper "github.com/LumeraProtocol/lumera/x/action/v1/keeper"
	"github.com/LumeraProtocol/lumera/x/action/v1/types"
	"time"

	"cosmossdk.io/log"
	"cosmossdk.io/math"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
)

const UpgradeName = "v1.0.0"

// CreateUpgradeHandler creates an upgrade handler for v1_0_0
func CreateUpgradeHandler(
	logger log.Logger,
	mm *module.Manager,
	configurator module.Configurator,
	actionKeeper actionkeeper.Keeper,
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

		// 2. Initialize Genesis State (Parameters) for the New 'x/action' Module
		logger.Info("Initializing genesis parameters for x/action module...")

		// Set the Action module parameters
		initialParams := types.Params{
			BaseActionFee:        sdk.NewCoin("ulume", math.NewInt(10000)),
			FeePerByte:           sdk.NewCoin("ulume", math.NewInt(100)),
			MaxActionsPerBlock:   10,
			MinSuperNodes:        1,
			MaxDdAndFingerprints: 50,
			MaxRaptorQSymbols:    50,
			ExpirationDuration:   24 * time.Hour,
			MinProcessingTime:    1 * time.Minute,
			MaxProcessingTime:    1 * time.Hour,
			SuperNodeFeeShare:    "1.000000000000000000",
			FoundationFeeShare:   "0.000000000000000000",
		}

		// Validate parameters
		if err := initialParams.Validate(); err != nil {
			logger.Error("Invalid initial x/action parameters", "error", err)
			return nil, fmt.Errorf("invalid initial x/action parameters: %w", err)
		}

		// Set the initial parameters using the action keeper and the unwrapped sdk.Context (ctx)
		if err := actionKeeper.SetParams(ctx, initialParams); err != nil {
			logger.Error("Failed to set x/action params", "error", err)
			return nil, fmt.Errorf("failed to set x/action params: %w", err)
		}

		logger.Info("Successfully set initial x/action parameters.")

		// 3. Add the New Module to the Version Map
		newVM[types.ModuleName] = types.ConsensusVersion

		logger.Info(fmt.Sprintf("Successfully completed upgrade %s", UpgradeName))

		// Return the UPDATED version map
		return newVM, nil
	}
}

var StoreUpgrades = storetypes.StoreUpgrades{
	Added: []string{
		types.StoreKey,
		types.MemStoreKey,
	},
	// Deleted: []string{...},
}
