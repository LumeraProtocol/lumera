package upgrades

import (
	"context"
	"fmt"

	storetypes "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"

	appParams "github.com/LumeraProtocol/lumera/app/upgrades/params"
	upgrade_v1_6_1 "github.com/LumeraProtocol/lumera/app/upgrades/v1_6_1"
	upgrade_v1_8_0 "github.com/LumeraProtocol/lumera/app/upgrades/v1_8_0"
	upgrade_v1_8_4 "github.com/LumeraProtocol/lumera/app/upgrades/v1_8_4"
	upgrade_v1_9_0 "github.com/LumeraProtocol/lumera/app/upgrades/v1_9_0"
	upgrade_v1_12_0 "github.com/LumeraProtocol/lumera/app/upgrades/v1_12_0"
)

// =================================================================================================================================
// Upgrade overview:
// =================================================================================================================================
// | Name    | Handler  | Store changes                     | Notes
// | v1.6.1  | custom   | none                              | Adds action module consensus version after migrations
// | v1.7.0  | standard | none                              | Migrations only
// | v1.7.2  | standard | none                              | Migrations only
// | v1.8.0  | standard | testnet/devnet: add PFM, drop NFT | Store upgrade gated to non-mainnet; handler is migrations only
// | v1.8.4  | standard | mainnet: add PFM, drop NFT        | Store upgrade gated to mainnet; handler is migrations only
// | v1.8.5  | standard | none                              | Migrations only
// | v1.9.0  | custom   | none                              | Backfills action/supernode secondary indices
// | v1.9.1  | standard | none                              | Migrations only
// | v1.12.0 | custom   | drop crisis                       | Migrate consensus params from x/params to x/consensus; remove x/crisis
// =================================================================================================================================

type UpgradeConfig struct {
	StoreUpgrade *storetypes.StoreUpgrades
	Handler      upgradetypes.UpgradeHandler
}

// Migration-only upgrades that use the standard handler.
const (
	upgradeNameV170 = "v1.7.0"
	upgradeNameV172 = "v1.7.2"
	upgradeNameV185 = "v1.8.5"
	upgradeNameV191 = "v1.9.1"
)

var upgradeNames = []string{
	upgrade_v1_6_1.UpgradeName,
	upgradeNameV170,
	upgradeNameV172,
	upgrade_v1_8_0.UpgradeName,
	upgrade_v1_8_4.UpgradeName,
	upgradeNameV185,
	upgrade_v1_9_0.UpgradeName,
	upgradeNameV191,
	upgrade_v1_12_0.UpgradeName,
}

var NoUpgradeConfig = UpgradeConfig{
	StoreUpgrade: nil,
	Handler:      nil,
}

// SetupUpgrades returns the configuration for the requested upgrade (if any).
// Do not define StoreUpgrades if there are no store changes for that upgrade.
// Do not define an UpgradeHandler if no custom logic is needed beyond module migrations.
// Prefer standardUpgradeHandler for migration-only upgrades; only create a custom
// handler when bespoke state changes are required beyond RunMigrations.
func SetupUpgrades(upgradeName string, params appParams.AppUpgradeParams) (UpgradeConfig, bool) {
	switch upgradeName {
	case upgrade_v1_6_1.UpgradeName:
		return UpgradeConfig{
			Handler: upgrade_v1_6_1.CreateUpgradeHandler(params),
		}, true
	case upgradeNameV170:
		return UpgradeConfig{
			Handler: standardUpgradeHandler(upgradeNameV170, params),
		}, true
	case upgradeNameV172:
		return UpgradeConfig{
			Handler: standardUpgradeHandler(upgradeNameV172, params),
		}, true
	case upgrade_v1_8_0.UpgradeName:
		if IsTestnet(params.ChainID) || IsDevnet(params.ChainID) {
			return UpgradeConfig{
				StoreUpgrade: &upgrade_v1_8_0.StoreUpgrades,
				Handler:      standardUpgradeHandler(upgrade_v1_8_0.UpgradeName, params),
			}, true
		}
		return NoUpgradeConfig, true
	case upgrade_v1_8_4.UpgradeName:
		config := UpgradeConfig{
			Handler: standardUpgradeHandler(upgrade_v1_8_4.UpgradeName, params),
		}
		if IsMainnet(params.ChainID) {
			config.StoreUpgrade = &upgrade_v1_8_4.StoreUpgrades
		}
		return config, true
	case upgradeNameV185:
		return UpgradeConfig{
			Handler: standardUpgradeHandler(upgradeNameV185, params),
		}, true
	case upgrade_v1_9_0.UpgradeName:
		return UpgradeConfig{
			Handler: upgrade_v1_9_0.CreateUpgradeHandler(params),
		}, true
	case upgradeNameV191:
		return UpgradeConfig{
			Handler: standardUpgradeHandler(upgradeNameV191, params),
		}, true
	case upgrade_v1_12_0.UpgradeName:
		return UpgradeConfig{
			StoreUpgrade: &upgrade_v1_12_0.StoreUpgrades,
			Handler: upgrade_v1_12_0.CreateUpgradeHandler(params),
		}, true

	// add future upgrades here
	default:
		return UpgradeConfig{}, false
	}
}

// AllUpgrades returns the upgrade configuration for every known upgrade name.
func AllUpgrades(params appParams.AppUpgradeParams) map[string]UpgradeConfig {
	configs := make(map[string]UpgradeConfig, len(upgradeNames))
	for _, upgradeName := range upgradeNames {
		if config, found := SetupUpgrades(upgradeName, params); found {
			configs[upgradeName] = config
		}
	}
	return configs
}

// standardUpgradeHandler returns a migration-only upgrade handler for simple upgrades.
// Use this helper when an upgrade needs only RunMigrations (no custom logic) and does
// not modify store keys; it keeps minimal upgrades boilerplate-free. When an upgrade
// requires bespoke state changes or store additions/removals, define a dedicated
// handler (and StoreUpgrades when applicable).
func standardUpgradeHandler(upgradeName string, p appParams.AppUpgradeParams) upgradetypes.UpgradeHandler {
	return func(goCtx context.Context, _ upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		p.Logger.Info(fmt.Sprintf("Starting upgrade %s...", upgradeName))

		ctx := sdk.UnwrapSDKContext(goCtx)

		p.Logger.Info("Running module migrations...")
		newVM, err := p.ModuleManager.RunMigrations(ctx, p.Configurator, fromVM)
		if err != nil {
			p.Logger.Error("Failed to run migrations", "error", err)
			return nil, fmt.Errorf("failed to run migrations: %w", err)
		}
		p.Logger.Info("Module migrations completed.")

		p.Logger.Info(fmt.Sprintf("Successfully completed upgrade %s", upgradeName))
		return newVM, nil
	}
}
