package upgrades

import (
	"context"
	"fmt"

	storetypes "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"

	appParams "github.com/LumeraProtocol/lumera/app/upgrades/params"
	upgrade_v1_10_0 "github.com/LumeraProtocol/lumera/app/upgrades/v1_10_0"
	upgrade_v1_10_1 "github.com/LumeraProtocol/lumera/app/upgrades/v1_10_1"
	upgrade_v1_11_0 "github.com/LumeraProtocol/lumera/app/upgrades/v1_11_0"
	upgrade_v1_11_1 "github.com/LumeraProtocol/lumera/app/upgrades/v1_11_1"
	upgrade_v1_12_0 "github.com/LumeraProtocol/lumera/app/upgrades/v1_12_0"
	upgrade_v1_20_0 "github.com/LumeraProtocol/lumera/app/upgrades/v1_20_0"
	upgrade_v1_20_1 "github.com/LumeraProtocol/lumera/app/upgrades/v1_20_1"
	upgrade_v1_6_1 "github.com/LumeraProtocol/lumera/app/upgrades/v1_6_1"
	upgrade_v1_8_0 "github.com/LumeraProtocol/lumera/app/upgrades/v1_8_0"
	upgrade_v1_8_4 "github.com/LumeraProtocol/lumera/app/upgrades/v1_8_4"
	upgrade_v1_9_0 "github.com/LumeraProtocol/lumera/app/upgrades/v1_9_0"
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
// | v1.10.0 | custom   | drop crisis                       | Migrate consensus params from x/params to x/consensus; remove x/crisis
// | v1.10.1 | custom   | drop crisis (if not already)      | Ensure consensus params are present in x/consensus
// | v1.11.0 | custom   | add audit store                   | Initializes audit params with dynamic epoch_zero_height
// | v1.11.1 | custom   | conditional add audit store       | Supports direct v1.10.1->v1.11.1 and enforces audit min_disk_free_percent floor (>=15)
// | v1.12.0 | custom   | none (Everlight in supernode)     | Runs migrations; Everlight logic embedded in x/supernode
// | v1.20.0 | custom   | non-mainnet: add feemarket, precisebank, vm, erc20 | EVM bring-up; gated to non-mainnet (mainnet runs it via v1.20.1)
// | v1.20.1 | custom   | state-driven add-only: feemarket, precisebank, vm, erc20 | EVM bring-up when EVM absent (any network, incl. direct 1.12.0->1.20.1); migrations-only hotfix when EVM already present. Add-only store loader mounts only missing keys.
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

// List of all known upgrade names, in chronological order.
var upgradeNames = []string{
	upgrade_v1_6_1.UpgradeName,
	upgradeNameV170,
	upgradeNameV172,
	upgrade_v1_8_0.UpgradeName,
	upgrade_v1_8_4.UpgradeName,
	upgradeNameV185,
	upgrade_v1_9_0.UpgradeName,
	upgradeNameV191,
	upgrade_v1_10_0.UpgradeName,
	upgrade_v1_10_1.UpgradeName,
	upgrade_v1_11_0.UpgradeName,
	upgrade_v1_11_1.UpgradeName,
	upgrade_v1_12_0.UpgradeName,
	upgrade_v1_20_0.UpgradeName,
	upgrade_v1_20_1.UpgradeName,
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
	case upgrade_v1_10_0.UpgradeName:
		return UpgradeConfig{
			StoreUpgrade: &upgrade_v1_10_0.StoreUpgrades,
			Handler:      upgrade_v1_10_0.CreateUpgradeHandler(params),
		}, true
	case upgrade_v1_10_1.UpgradeName:
		return UpgradeConfig{
			StoreUpgrade: &upgrade_v1_10_1.StoreUpgrades,
			Handler:      upgrade_v1_10_1.CreateUpgradeHandler(params),
		}, true
	case upgrade_v1_11_0.UpgradeName:
		return UpgradeConfig{
			StoreUpgrade: &upgrade_v1_11_0.StoreUpgrades,
			Handler:      upgrade_v1_11_0.CreateUpgradeHandler(params),
		}, true
	case upgrade_v1_11_1.UpgradeName:
		return UpgradeConfig{
			StoreUpgrade: &upgrade_v1_11_1.StoreUpgrades,
			Handler:      upgrade_v1_11_1.CreateUpgradeHandler(params),
		}, true
	case upgrade_v1_12_0.UpgradeName:
		return UpgradeConfig{
			StoreUpgrade: &upgrade_v1_12_0.StoreUpgrades,
			Handler:      upgrade_v1_12_0.CreateUpgradeHandler(params),
		}, true
	case upgrade_v1_20_0.UpgradeName:
		// Mainnet skips v1.20.0 entirely and runs the EVM bring-up via v1.20.1
		// instead (see the v1.20.1 case). Testnet and devnet already ran v1.20.0,
		// so they keep it. Mirrors the v1.8.0/v1.8.4 mainnet-skip precedent.
		if IsMainnet(params.ChainID) {
			return NoUpgradeConfig, true
		}
		return UpgradeConfig{
			StoreUpgrade: &upgrade_v1_20_0.StoreUpgrades,
			Handler:      upgrade_v1_20_0.CreateUpgradeHandler(params),
		}, true
	case upgrade_v1_20_1.UpgradeName:
		// v1.20.1 carries the EVM bring-up based on chain STATE, not chain-id.
		// It declares the same EVM store additions as v1.20.0 on every network;
		// the add-only store loader (see StoreLoaderForUpgrade) mounts only the
		// keys missing from committed state, so this is a no-op on chains that
		// already ran v1.20.0 and mounts the EVM stores on a direct 1.12.0->1.20.1
		// one-hop. The handler is likewise state-driven: it runs the full v1.20.0
		// bring-up when the EVM stack is absent from fromVM, else migrations only.
		return UpgradeConfig{
			StoreUpgrade: &upgrade_v1_20_0.StoreUpgrades,
			Handler:      upgrade_v1_20_1.CreateUpgradeHandler(params),
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
