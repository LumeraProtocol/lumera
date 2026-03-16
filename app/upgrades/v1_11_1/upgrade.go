package v1_11_1

import (
	"context"
	"fmt"

	upgradetypes "cosmossdk.io/x/upgrade/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"

	appParams "github.com/LumeraProtocol/lumera/app/upgrades/params"
	audittypes "github.com/LumeraProtocol/lumera/x/audit/v1/types"
)

// UpgradeName is the on-chain name used for this upgrade.
const UpgradeName = "v1.11.1"

// auditMinDiskFreePercentFloor is the minimum acceptable value for
// audit.params.min_disk_free_percent after this upgrade.
const auditMinDiskFreePercentFloor = uint32(15)

// CreateUpgradeHandler runs module migrations and enforces a floor for
// audit.params.min_disk_free_percent.
//
// This handler supports both:
//   - direct upgrades from pre-audit binaries (e.g. v1.10.1), and
//   - upgrades from v1.11.0 where audit is already initialized.
func CreateUpgradeHandler(p appParams.AppUpgradeParams) upgradetypes.UpgradeHandler {
	return func(goCtx context.Context, _ upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		p.Logger.Info(fmt.Sprintf("Starting upgrade %s...", UpgradeName))

		ctx := sdk.UnwrapSDKContext(goCtx)
		if p.AuditKeeper == nil {
			return nil, fmt.Errorf("%s upgrade requires audit keeper to be wired", UpgradeName)
		}

		_, auditModuleAlreadyMigrated := fromVM[audittypes.ModuleName]
		migrationVM := prepareVersionMapForConditionalAuditInit(fromVM)

		p.Logger.Info("Running module migrations...")
		newVM, err := p.ModuleManager.RunMigrations(ctx, p.Configurator, migrationVM)
		if err != nil {
			p.Logger.Error("Failed to run migrations", "error", err)
			return nil, fmt.Errorf("failed to run migrations: %w", err)
		}
		p.Logger.Info("Module migrations completed.")

		if !auditModuleAlreadyMigrated {
			if err := initializeAuditForDirectUpgrade(ctx, p); err != nil {
				return nil, err
			}
		}

		params := p.AuditKeeper.GetParams(ctx).WithDefaults()
		updatedParams, changed := withMinDiskFreePercentFloor(params, auditMinDiskFreePercentFloor)
		if changed {
			if err := p.AuditKeeper.SetParams(ctx, updatedParams); err != nil {
				return nil, fmt.Errorf("set audit params: %w", err)
			}
			p.Logger.Info("Updated audit params min_disk_free_percent floor",
				"previous", params.MinDiskFreePercent,
				"current", updatedParams.MinDiskFreePercent,
			)
		} else {
			p.Logger.Info("Audit min_disk_free_percent already satisfies floor",
				"current", params.MinDiskFreePercent,
			)
		}

		p.Logger.Info(fmt.Sprintf("Successfully completed upgrade %s", UpgradeName))
		return newVM, nil
	}
}

func prepareVersionMapForConditionalAuditInit(fromVM module.VersionMap) module.VersionMap {
	migrationVM := make(module.VersionMap, len(fromVM)+1)
	for moduleName, version := range fromVM {
		migrationVM[moduleName] = version
	}

	migrationVM[audittypes.ModuleName] = audittypes.ConsensusVersion
	return migrationVM
}

func initializeAuditForDirectUpgrade(ctx sdk.Context, p appParams.AppUpgradeParams) error {
	gs := audittypes.DefaultGenesis()
	params := gs.Params.WithDefaults()

	// When enabling audit on an already-running chain, epoch zero must start at
	// the current upgrade height.
	params.EpochZeroHeight = uint64(ctx.BlockHeight())
	gs.Params = params

	if err := p.AuditKeeper.InitGenesis(ctx, *gs); err != nil {
		return fmt.Errorf("init audit module: %w", err)
	}

	// Create epoch-0 anchor immediately at upgrade height.
	epochID := uint64(0)
	epochStart := ctx.BlockHeight()
	epochEnd := epochStart + int64(params.EpochLengthBlocks) - 1
	if err := p.AuditKeeper.CreateEpochAnchorIfNeeded(ctx, epochID, epochStart, epochEnd, params); err != nil {
		return fmt.Errorf("create audit epoch anchor: %w", err)
	}

	return nil
}

func withMinDiskFreePercentFloor(params audittypes.Params, floor uint32) (audittypes.Params, bool) {
	params = params.WithDefaults()
	if params.MinDiskFreePercent >= floor {
		return params, false
	}
	params.MinDiskFreePercent = floor
	return params, true
}
