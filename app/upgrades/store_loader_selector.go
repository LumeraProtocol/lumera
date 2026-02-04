package upgrades

import (
	"fmt"

	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"

	"github.com/cosmos/cosmos-sdk/baseapp"

	upgrade_v1_10_1 "github.com/LumeraProtocol/lumera/app/upgrades/v1_10_1"
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
	if adaptive {
		if upgradeName == upgrade_v1_10_1.UpgradeName {
			return StoreLoaderSelection{
				Loader:   ConsensusStoreLoader(upgradeHeight, baseUpgrades, expectedStoreNames, logger),
				LogLabel: "consensus rename",
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
