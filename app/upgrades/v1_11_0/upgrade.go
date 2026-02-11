package v1_11_0

import (
	"context"
	"fmt"

	storetypes "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"

	appParams "github.com/LumeraProtocol/lumera/app/upgrades/params"
	audittypes "github.com/LumeraProtocol/lumera/x/audit/v1/types"
)

// Storage challenge feature toggle at activation time.
//
// This is set exactly once during the upgrade; thereafter, params are governed by MsgUpdateParams.
// All other parameter defaults are owned by the audit module (types.DefaultParams()).
const auditScEnabled = true

// UpgradeName is the on-chain name used for this upgrade.
const UpgradeName = "v1.11.0"

// StoreUpgrades declares store additions/deletions for this upgrade.
//
// Audit is introduced for the first time on-chain, so its KV store must be added.
var StoreUpgrades = storetypes.StoreUpgrades{
	Added: []string{
		audittypes.StoreKey, // "audit"
	},
}

// CreateUpgradeHandler creates an upgrade handler for v1.11.0.
//
// This upgrade introduces the audit module and initializes its params in a way
// that requires no hard-coded epoch_zero_height for existing networks:
//   - epoch_zero_height is set to the upgrade block height (the first block the
//     new binary processes).
//   - epoch_length_blocks remains its default unless governance later changes it
//     (epoch cadence fields are immutable after initialization).
func CreateUpgradeHandler(p appParams.AppUpgradeParams) upgradetypes.UpgradeHandler {
	return func(goCtx context.Context, _ upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		p.Logger.Info(fmt.Sprintf("Starting upgrade %s...", UpgradeName))

		ctx := sdk.UnwrapSDKContext(goCtx)

		p.Logger.Info("Running module migrations...")
		newVM, err := p.ModuleManager.RunMigrations(ctx, p.Configurator, fromVM)
		if err != nil {
			p.Logger.Error("Failed to run migrations", "error", err)
			return nil, fmt.Errorf("failed to run migrations: %w", err)
		}
		p.Logger.Info("Module migrations completed.")

		if p.AuditKeeper == nil {
			return nil, fmt.Errorf("%s upgrade requires audit keeper to be wired", UpgradeName)
		}

		// Initialize audit genesis state with an automatically chosen epoch_zero_height.
		// Using the current block height ensures that the audit BeginBlocker will create
		// the epoch-0 anchor immediately at the epoch start height.
		gs := audittypes.DefaultGenesis()
		params := gs.Params.WithDefaults()

		// For an already-running chain, epoch zero must be chosen dynamically at the upgrade height.
		// This is set exactly once here; thereafter it is immutable.
		params.EpochZeroHeight = uint64(ctx.BlockHeight())

		// Feature gate for SC evidence validation/acceptance.
		params.ScEnabled = auditScEnabled

		gs.Params = params

		if err := p.AuditKeeper.InitGenesis(ctx, *gs); err != nil {
			return nil, fmt.Errorf("init audit module: %w", err)
		}

		// Create epoch-0 anchor immediately, so audit reporting can begin in the same upgrade block
		// without depending on BeginBlock ordering assumptions.
		epochID := uint64(0)
		epochStart := ctx.BlockHeight()
		epochEnd := epochStart + int64(params.EpochLengthBlocks) - 1
		if err := p.AuditKeeper.CreateEpochAnchorIfNeeded(ctx, epochID, epochStart, epochEnd, params); err != nil {
			return nil, fmt.Errorf("create audit epoch anchor: %w", err)
		}

		p.Logger.Info(fmt.Sprintf("Successfully completed upgrade %s", UpgradeName))
		return newVM, nil
	}
}
