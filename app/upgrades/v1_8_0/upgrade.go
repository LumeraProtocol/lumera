package v1_8_0

import (
	"context"
	"fmt"

	storetypes "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"

	appParams "github.com/LumeraProtocol/lumera/app/upgrades/params"
	pfmtypes "github.com/cosmos/ibc-apps/middleware/packet-forward-middleware/v10/packetforward/types"
)

const UpgradeName = "v1.8.0"

// CreateUpgradeHandler creates an upgrade handler for v1_8_0
func CreateUpgradeHandler(p appParams.AppUpgradeParams) upgradetypes.UpgradeHandler {
	return func(goCtx context.Context, _ upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		p.Logger.Info(fmt.Sprintf("Starting upgrade %s...", UpgradeName))

		ctx := sdk.UnwrapSDKContext(goCtx)

		// Run module migrations
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

var StoreUpgrades = storetypes.StoreUpgrades{
	Added:   []string{
		pfmtypes.StoreKey, // added Packet Forwarding Middleware (PFM) store key
	},
	Deleted: []string{
		"nft",			   // deleted NFT module store key
	},
}