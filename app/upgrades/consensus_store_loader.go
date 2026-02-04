package upgrades

import (
	"sort"

	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"

	"github.com/cosmos/cosmos-sdk/baseapp"
)

const (
	legacyConsensusStoreKey = "Consensus"
	consensusStoreKey       = "consensus"
)

// ConsensusStoreLoader builds a store loader that safely renames the legacy
// consensus store (if present) and avoids panics when the new store already exists.
//
// If expectedStoreNames is provided, the loader will also compute adaptive store
// upgrades against the existing on-disk stores.
func ConsensusStoreLoader(
	upgradeHeight int64,
	baseUpgrades *storetypes.StoreUpgrades,
	expectedStoreNames map[string]struct{},
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

		effective := computeConsensusStoreUpgrades(baseUpgrades, expectedStoreNames, existingStoreNames, logger)
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

func computeConsensusStoreUpgrades(
	baseUpgrades *storetypes.StoreUpgrades,
	expectedStoreNames map[string]struct{},
	existingStoreNames map[string]struct{},
	logger log.Logger,
) storetypes.StoreUpgrades {
	var effective storetypes.StoreUpgrades
	if expectedStoreNames != nil {
		effective = computeAdaptiveStoreUpgrades(baseUpgrades, expectedStoreNames, existingStoreNames)
		effective.Renamed = filterStoreRenames(effective.Renamed, existingStoreNames)
	} else {
		effective = cloneStoreUpgrades(baseUpgrades)
		effective.Added = filterStoreAdds(effective.Added, existingStoreNames)
		effective.Deleted = filterStoreDeletes(effective.Deleted, existingStoreNames)
		effective.Renamed = filterStoreRenames(effective.Renamed, existingStoreNames)
	}

	hasLegacy := storeExists(existingStoreNames, legacyConsensusStoreKey)
	hasNew := storeExists(existingStoreNames, consensusStoreKey)

	switch {
	case hasLegacy && !hasNew:
		effective.Added = removeStoreName(effective.Added, consensusStoreKey)
		effective.Deleted = removeStoreName(effective.Deleted, legacyConsensusStoreKey)
		effective.Renamed = append(effective.Renamed, storetypes.StoreRename{
			OldKey: legacyConsensusStoreKey,
			NewKey: consensusStoreKey,
		})
	case !hasLegacy && !hasNew:
		if expectedStoreNames == nil {
			effective.Added = append(effective.Added, consensusStoreKey)
		}
	case hasLegacy && hasNew:
		effective.Deleted = removeStoreName(effective.Deleted, legacyConsensusStoreKey)
		logger.Info("Both legacy and new consensus stores exist; skipping rename", "old", legacyConsensusStoreKey, "new", consensusStoreKey)
	}

	effective.Added = uniqueSortedStores(effective.Added)
	effective.Deleted = uniqueSortedStores(effective.Deleted)

	return effective
}

func cloneStoreUpgrades(base *storetypes.StoreUpgrades) storetypes.StoreUpgrades {
	if base == nil {
		return storetypes.StoreUpgrades{}
	}
	return storetypes.StoreUpgrades{
		Added:   append([]string(nil), base.Added...),
		Deleted: append([]string(nil), base.Deleted...),
		Renamed: append([]storetypes.StoreRename(nil), base.Renamed...),
	}
}

func filterStoreAdds(added []string, existing map[string]struct{}) []string {
	if len(added) == 0 {
		return nil
	}
	out := make([]string, 0, len(added))
	for _, name := range added {
		if !storeExists(existing, name) {
			out = append(out, name)
		}
	}
	return out
}

func filterStoreDeletes(deleted []string, existing map[string]struct{}) []string {
	if len(deleted) == 0 {
		return nil
	}
	out := make([]string, 0, len(deleted))
	for _, name := range deleted {
		if storeExists(existing, name) {
			out = append(out, name)
		}
	}
	return out
}

func filterStoreRenames(renames []storetypes.StoreRename, existing map[string]struct{}) []storetypes.StoreRename {
	if len(renames) == 0 {
		return nil
	}
	out := make([]storetypes.StoreRename, 0, len(renames))
	for _, rename := range renames {
		if storeExists(existing, rename.OldKey) && !storeExists(existing, rename.NewKey) {
			out = append(out, rename)
		}
	}
	return out
}

func storeExists(existing map[string]struct{}, name string) bool {
	if existing == nil {
		return false
	}
	_, ok := existing[name]
	return ok
}

func removeStoreName(names []string, target string) []string {
	if len(names) == 0 {
		return nil
	}
	out := names[:0]
	for _, name := range names {
		if name != target {
			out = append(out, name)
		}
	}
	return out
}

func uniqueSortedStores(names []string) []string {
	if len(names) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(names))
	for _, name := range names {
		set[name] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for name := range set {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
