package upgrades

import (
	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"

	"github.com/cosmos/cosmos-sdk/baseapp"
)

// AuditStoreLoader builds a store loader that safely adds the audit store only
// when it is missing on-disk. This allows v1.11.1 to work for both:
//   - chains that already ran v1.11.0 (audit store exists)
//   - chains upgrading directly from v1.10.1 (audit store missing)
func AuditStoreLoader(
	upgradeHeight int64,
	baseUpgrades *storetypes.StoreUpgrades,
	logger log.Logger,
) baseapp.StoreLoader {
	fallbackLoader := upgradetypes.UpgradeStoreLoader(upgradeHeight, baseUpgrades)

	return func(ms storetypes.CommitMultiStore) error {
		if upgradeHeight != ms.LastCommitID().Version+1 {
			return baseapp.DefaultStoreLoader(ms)
		}

		existingStoreNames, err := loadExistingStoreNames(ms)
		if err != nil {
			logger.Error("Failed to load existing stores; falling back to standard upgrade loader", "error", err)
			return fallbackLoader(ms)
		}

		effective := computeAuditStoreUpgrades(baseUpgrades, existingStoreNames)
		if len(effective.Added) == 0 && len(effective.Deleted) == 0 && len(effective.Renamed) == 0 {
			logger.Info("No store upgrades required; loading latest version", "height", upgradeHeight)
			return baseapp.DefaultStoreLoader(ms)
		}

		logger.Info(
			"Applying store upgrades",
			"height", upgradeHeight,
			"added", effective.Added,
			"deleted", effective.Deleted,
			"renamed", formatStoreRenames(effective.Renamed),
		)

		return ms.LoadLatestVersionAndUpgrade(&effective)
	}
}

func computeAuditStoreUpgrades(baseUpgrades *storetypes.StoreUpgrades, existingStoreNames map[string]struct{}) storetypes.StoreUpgrades {
	effective := cloneStoreUpgrades(baseUpgrades)
	effective.Added = filterStoreAdds(effective.Added, existingStoreNames)
	effective.Deleted = filterStoreDeletes(effective.Deleted, existingStoreNames)
	effective.Renamed = filterStoreRenames(effective.Renamed, existingStoreNames)
	effective.Added = uniqueSortedStores(effective.Added)
	effective.Deleted = uniqueSortedStores(effective.Deleted)
	return effective
}
