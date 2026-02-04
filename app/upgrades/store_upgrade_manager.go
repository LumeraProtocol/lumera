package upgrades

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"

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
	return envBool(EnvEnableStoreUpgradeManager)
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

func envBool(key string) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return false
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false
	}
	return parsed
}
