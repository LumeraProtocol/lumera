package upgrades

import (
	"fmt"
	"sort"

	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"

	textutil "github.com/LumeraProtocol/lumera/pkg/text"
	"github.com/cosmos/cosmos-sdk/baseapp"
)

// EnvEnableStoreUpgradeManager toggles the adaptive store upgrade manager.
// Intended for devnet environments where skipping intermediate upgrades is useful.
const EnvEnableStoreUpgradeManager = "LUMERA_ENABLE_STORE_UPGRADE_MANAGER"

// ShouldEnableStoreUpgradeManager returns true when the adaptive store upgrade
// manager should be used for this chain.
func ShouldEnableStoreUpgradeManager(chainID string) bool {
	if !IsDevnet(chainID) {
		return false
	}
	return textutil.EnvBool(EnvEnableStoreUpgradeManager)
}

// KVStoreNames returns the set of persistent KV store names registered in the app.
func KVStoreNames(storeKeys []storetypes.StoreKey) map[string]struct{} {
	names := make(map[string]struct{}, len(storeKeys))
	for _, key := range storeKeys {
		if _, ok := key.(*storetypes.KVStoreKey); !ok {
			continue
		}
		names[key.Name()] = struct{}{}
	}
	return names
}

// AdaptiveStoreLoader builds a store loader that merges explicit store upgrades with
// a diff between on-disk stores and the app's registered KV stores.
// This enables skipping intermediate upgrades in dev/test networks.
func AdaptiveStoreLoader(
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

		effective := computeAdaptiveStoreUpgrades(baseUpgrades, expectedStoreNames, existingStoreNames)
		if len(effective.Added) == 0 && len(effective.Deleted) == 0 && len(effective.Renamed) == 0 {
			logger.Info("No store upgrades required after diff; loading latest version", "height", upgradeHeight)
			return baseapp.DefaultStoreLoader(ms)
		}

		logger.Info(
			"Applying adaptive store upgrades",
			"height", upgradeHeight,
			"added", effective.Added,
			"deleted", effective.Deleted,
			"renamed", formatStoreRenames(effective.Renamed),
		)

		return ms.LoadLatestVersionAndUpgrade(&effective)
	}
}

// AddOnlyStoreLoader builds a store loader that mounts the store keys declared in
// baseUpgrades.Added that are NOT already present in committed state, and never
// deletes or renames anything. Unlike AdaptiveStoreLoader it does not diff against
// the app's expected store set, so it cannot be tricked by a binary that fails to
// register a store into wiping that store's data. It is safe on any network,
// which is why the state-driven v1.20.1 EVM bring-up uses it: on a chain that
// already mounted the EVM stores (ran v1.20.0) it is a no-op; on a one-hop chain
// it mounts the missing EVM stores.
func AddOnlyStoreLoader(
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

		effective := computeAddOnlyStoreUpgrades(baseUpgrades, existingStoreNames)
		if len(effective.Added) == 0 {
			logger.Info("No stores to add after diff; loading latest version", "height", upgradeHeight)
			return baseapp.DefaultStoreLoader(ms)
		}

		logger.Info(
			"Applying add-only store upgrades",
			"height", upgradeHeight,
			"added", effective.Added,
		)

		return ms.LoadLatestVersionAndUpgrade(&effective)
	}
}

// computeAddOnlyStoreUpgrades returns the subset of baseUpgrades.Added that is not
// already present in committed state. Deletions and renames declared on
// baseUpgrades are intentionally dropped: this loader can only add stores.
func computeAddOnlyStoreUpgrades(
	baseUpgrades *storetypes.StoreUpgrades,
	existingStoreNames map[string]struct{},
) storetypes.StoreUpgrades {
	if baseUpgrades == nil {
		return storetypes.StoreUpgrades{}
	}
	if existingStoreNames == nil {
		existingStoreNames = map[string]struct{}{}
	}

	added := make(map[string]struct{})
	for _, name := range baseUpgrades.Added {
		if _, exists := existingStoreNames[name]; !exists {
			added[name] = struct{}{}
		}
	}

	return storetypes.StoreUpgrades{Added: sortedKeys(added)}
}

type commitInfoReader interface {
	GetCommitInfo(int64) (*storetypes.CommitInfo, error)
}

func loadExistingStoreNames(ms storetypes.CommitMultiStore) (map[string]struct{}, error) {
	version := ms.LastCommitID().Version
	if version == 0 {
		return map[string]struct{}{}, nil
	}

	reader, ok := ms.(commitInfoReader)
	if !ok {
		return nil, fmt.Errorf("commit multistore does not expose commit info")
	}

	cInfo, err := reader.GetCommitInfo(version)
	if err != nil {
		return nil, fmt.Errorf("failed to read commit info for version %d: %w", version, err)
	}

	names := make(map[string]struct{}, len(cInfo.StoreInfos))
	for _, info := range cInfo.StoreInfos {
		names[info.Name] = struct{}{}
	}

	return names, nil
}

func computeAdaptiveStoreUpgrades(
	baseUpgrades *storetypes.StoreUpgrades,
	expectedStoreNames map[string]struct{},
	existingStoreNames map[string]struct{},
) storetypes.StoreUpgrades {
	if expectedStoreNames == nil {
		expectedStoreNames = map[string]struct{}{}
	}
	if existingStoreNames == nil {
		existingStoreNames = map[string]struct{}{}
	}

	effective := storetypes.StoreUpgrades{}
	if baseUpgrades != nil {
		effective.Renamed = append([]storetypes.StoreRename(nil), baseUpgrades.Renamed...)
	}

	added := make(map[string]struct{})
	deleted := make(map[string]struct{})

	if baseUpgrades != nil {
		for _, name := range baseUpgrades.Added {
			added[name] = struct{}{}
		}
		for _, name := range baseUpgrades.Deleted {
			deleted[name] = struct{}{}
		}
	}

	for name := range expectedStoreNames {
		if _, exists := existingStoreNames[name]; !exists {
			added[name] = struct{}{}
		}
	}

	for name := range existingStoreNames {
		if _, expected := expectedStoreNames[name]; !expected {
			deleted[name] = struct{}{}
		}
	}

	for name := range added {
		if _, exists := existingStoreNames[name]; exists {
			delete(added, name)
		}
	}

	for name := range deleted {
		if _, exists := existingStoreNames[name]; !exists {
			delete(deleted, name)
		}
	}

	for name := range added {
		if _, exists := deleted[name]; exists {
			delete(added, name)
		}
	}

	effective.Added = sortedKeys(added)
	effective.Deleted = sortedKeys(deleted)

	return effective
}

func sortedKeys(set map[string]struct{}) []string {
	if len(set) == 0 {
		return nil
	}
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func formatStoreRenames(renames []storetypes.StoreRename) []string {
	if len(renames) == 0 {
		return nil
	}
	out := make([]string, 0, len(renames))
	for _, rename := range renames {
		out = append(out, fmt.Sprintf("%s->%s", rename.OldKey, rename.NewKey))
	}
	return out
}
