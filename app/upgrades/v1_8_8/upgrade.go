package v1_8_8

import (
	"context"
	"fmt"

	upgradetypes "cosmossdk.io/x/upgrade/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"

	appParams "github.com/LumeraProtocol/lumera/app/upgrades/params"
	actiontypes "github.com/LumeraProtocol/lumera/x/action/v1/types"
)

// UpgradeName is the on-chain name used for this upgrade.
const UpgradeName = "v1.8.8"

// CreateUpgradeHandler creates an upgrade handler for v1.8.8.
//
// This upgrade backfills secondary indices introduced in prior releases:
// - action module: state/creator/type/block/supernode indices
// - supernode module: supernodeAccount -> validator operator address index
//
// The index keys are derived from existing primary records, so no StoreUpgrades
// (added/removed module store keys) are required.
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

		if p.ActionKeeper == nil || p.SupernodeKeeper == nil {
			return nil, fmt.Errorf("%s upgrade requires keepers to be wired (action=%v supernode=%v)", UpgradeName, p.ActionKeeper != nil, p.SupernodeKeeper != nil)
		}

		// Backfill action indices by replaying SetAction for each existing action.
		var actionIDs []string
		if err := p.ActionKeeper.IterateActions(ctx, func(a *actiontypes.Action) bool {
			actionIDs = append(actionIDs, a.ActionID)
			return false
		}); err != nil {
			return nil, fmt.Errorf("failed to iterate actions for index backfill: %w", err)
		}
		for _, id := range actionIDs {
			a, found := p.ActionKeeper.GetActionByID(ctx, id)
			if !found {
				continue
			}
			if err := p.ActionKeeper.SetAction(ctx, a); err != nil {
				return nil, fmt.Errorf("failed to backfill action indices for action_id=%s: %w", id, err)
			}
		}

		// Backfill supernode account index by replaying SetSuperNode for each supernode.
		supernodes, err := p.SupernodeKeeper.GetAllSuperNodes(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list supernodes for account index backfill: %w", err)
		}
		for _, sn := range supernodes {
			if err := p.SupernodeKeeper.SetSuperNode(ctx, sn); err != nil {
				return nil, fmt.Errorf("failed to backfill supernode account index for validator=%s: %w", sn.ValidatorAddress, err)
			}
		}

		p.Logger.Info(fmt.Sprintf("Successfully completed upgrade %s", UpgradeName))
		return newVM, nil
	}
}
