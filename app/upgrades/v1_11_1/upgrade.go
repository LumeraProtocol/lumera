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

func withMinDiskFreePercentFloor(params audittypes.Params, floor uint32) (audittypes.Params, bool) {
	params = params.WithDefaults()
	if params.MinDiskFreePercent >= floor {
		return params, false
	}
	params.MinDiskFreePercent = floor
	return params, true
}
