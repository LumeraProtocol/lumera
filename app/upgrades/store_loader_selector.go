package upgrades

import (
	"fmt"

	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"

	"github.com/cosmos/cosmos-sdk/baseapp"

	upgrade_v1_10_1 "github.com/LumeraProtocol/lumera/app/upgrades/v1_10_1"
	upgrade_v1_11_1 "github.com/LumeraProtocol/lumera/app/upgrades/v1_11_1"
	upgrade_v1_20_1 "github.com/LumeraProtocol/lumera/app/upgrades/v1_20_1"
	upgrade_v1_20_2 "github.com/LumeraProtocol/lumera/app/upgrades/v1_20_2"
)

type StoreLoaderSelection struct {
	Loader   baseapp.StoreLoader
	LogLabel string
}

// StoreLoaderForUpgrade returns the store loader to use for a given upgrade plan.
// When adaptive mode is enabled, expectedStoreNames should be provided.
func StoreLoaderForUpgrade(
	upgradeName string,
	upgradeHeight int64,
	baseUpgrades *storetypes.StoreUpgrades,
	expectedStoreNames map[string]struct{},
	logger log.Logger,
	adaptive bool,
) StoreLoaderSelection {
	// v1.20.1 always uses the add-only store loader, on every network and
	// regardless of the adaptive-store-manager env flag. It mounts the declared
	// EVM store keys that are absent from committed state and never deletes a
	// store, so it is safe on mainnet and a no-op on chains that already ran
	// v1.20.0. See the v1.20.1 case in SetupUpgrades.
	// v1.20.2 carries the same EVM store additions for the same reason: a chain
	// upgrading straight from 1.12.0 (mainnet's position) has none of the EVM
	// stores, while one arriving from 1.20.1 has all of them. The add-only loader
	// makes a single binary correct on both.
	if upgradeName == upgrade_v1_20_1.UpgradeName || upgradeName == upgrade_v1_20_2.UpgradeName {
		return StoreLoaderSelection{
			Loader:   AddOnlyStoreLoader(upgradeHeight, baseUpgrades, logger),
			LogLabel: "add-only EVM bring-up",
		}
	}

	if adaptive {
		if upgradeName == upgrade_v1_10_1.UpgradeName {
			return StoreLoaderSelection{
				Loader:   ConsensusStoreLoader(upgradeHeight, baseUpgrades, expectedStoreNames, logger),
				LogLabel: "consensus rename",
			}
		}
		if upgradeName == upgrade_v1_11_1.UpgradeName {
			return StoreLoaderSelection{
				Loader:   AuditStoreLoader(upgradeHeight, baseUpgrades, logger),
				LogLabel: "conditional audit store",
			}
		}
		return StoreLoaderSelection{
			Loader:   AdaptiveStoreLoader(upgradeHeight, baseUpgrades, expectedStoreNames, logger),
			LogLabel: "adaptive mode",
		}
	}

	if upgradeName == upgrade_v1_10_1.UpgradeName {
		return StoreLoaderSelection{
			Loader:   ConsensusStoreLoader(upgradeHeight, baseUpgrades, nil, logger),
			LogLabel: "consensus rename",
		}
	}
	if upgradeName == upgrade_v1_11_1.UpgradeName {
		return StoreLoaderSelection{
			Loader:   AuditStoreLoader(upgradeHeight, baseUpgrades, logger),
			LogLabel: "conditional audit store",
		}
	}

	return StoreLoaderSelection{
		Loader: upgradetypes.UpgradeStoreLoader(upgradeHeight, baseUpgrades),
	}
}

func (s StoreLoaderSelection) LogMessage() string {
	if s.LogLabel == "" {
		return "Configured store loader for upgrade"
	}
	return fmt.Sprintf("Configured store loader for upgrade (%s)", s.LogLabel)
}
