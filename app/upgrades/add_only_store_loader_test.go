package upgrades

import (
	"testing"

	"cosmossdk.io/log"
	"cosmossdk.io/store/metrics"
	pruningtypes "cosmossdk.io/store/pruning/types"
	"cosmossdk.io/store/rootmulti"
	storetypes "cosmossdk.io/store/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/stretchr/testify/require"

	upgrade_v1_20_0 "github.com/LumeraProtocol/lumera/app/upgrades/v1_20_0"
)

// newTestStore builds a committable in-memory rootmulti store (real no-op metrics
// + pruning options, both required before Commit).
func newTestStore(db dbm.DB) *rootmulti.Store {
	ms := rootmulti.NewStore(db, log.NewNopLogger(), metrics.NewNoOpMetrics())
	ms.SetPruning(pruningtypes.NewPruningOptions(pruningtypes.PruningNothing))
	return ms
}

// mountKeys mounts a KV store for each name into ms and returns the key objects.
// rootmulti indexes its stores by the StoreKey pointer, so callers must reuse the
// returned keys (not a fresh NewKVStoreKey of the same name) when calling
// GetKVStore, or the lookup panics with "store does not exist".
func mountKeys(ms *rootmulti.Store, names []string) map[string]*storetypes.KVStoreKey {
	keys := make(map[string]*storetypes.KVStoreKey, len(names))
	for _, name := range names {
		key := storetypes.NewKVStoreKey(name)
		keys[name] = key
		ms.MountStoreWithDB(key, storetypes.StoreTypeIAVL, nil)
	}
	return keys
}

func committedStoreNames(t *testing.T, ms *rootmulti.Store) map[string]struct{} {
	t.Helper()
	names, err := loadExistingStoreNames(ms)
	require.NoError(t, err)
	return names
}

// End-to-end through the real store loader: a chain committed WITHOUT the EVM
// stores (the pre-EVM 1.12.0 state) upgrades directly to v1.20.1. The add-only
// loader must mount every declared EVM store and preserve pre-existing data.
func TestAddOnlyStoreLoader_MountsMissingEVMStoresFromPreEVMState(t *testing.T) {
	db := dbm.NewMemDB()
	base := &upgrade_v1_20_0.StoreUpgrades

	// v1: pre-EVM committed state (auth + bank only), with data in bank.
	pre := newTestStore(db)
	preKeys := mountKeys(pre, []string{"auth", "bank"})
	require.NoError(t, pre.LoadLatestVersion())
	pre.GetKVStore(preKeys["bank"]).Set([]byte("balance"), []byte("42"))
	require.Equal(t, int64(1), pre.Commit().Version)

	// A new binary mounts auth + bank + all EVM stores and runs the v1.20.1
	// add-only loader at the upgrade height (committed version + 1).
	next := newTestStore(db)
	nextKeys := mountKeys(next, append([]string{"auth", "bank"}, base.Added...))
	require.NoError(t, AddOnlyStoreLoader(2, base, log.NewNopLogger())(next))

	// Pre-existing bank data survives the mount.
	require.Equal(t, []byte("42"), next.GetKVStore(nextKeys["bank"]).Get([]byte("balance")),
		"add-only loader must not disturb existing store data")

	// Every declared EVM store is now mounted and writable; committing succeeds.
	for _, name := range base.Added {
		next.GetKVStore(nextKeys[name]).Set([]byte("k"), []byte("v"))
	}
	require.Equal(t, int64(2), next.Commit().Version)

	// The committed store set now includes the EVM stores.
	committed := committedStoreNames(t, next)
	for _, name := range base.Added {
		require.Contains(t, committed, name, "committed store set should include %s after upgrade", name)
	}
}

// A chain that already ran v1.20.0 (EVM stores present) applying v1.20.1 must be a
// pure no-op at the store layer: the add-only loader adds nothing and does not
// disturb existing data. This is the hotfix path.
func TestAddOnlyStoreLoader_NoopWhenEVMStoresPresent(t *testing.T) {
	db := dbm.NewMemDB()
	base := &upgrade_v1_20_0.StoreUpgrades
	names := append([]string{"auth", "bank"}, base.Added...)

	// v1: committed state that already includes the EVM stores.
	pre := newTestStore(db)
	preKeys := mountKeys(pre, names)
	require.NoError(t, pre.LoadLatestVersion())
	pre.GetKVStore(preKeys["bank"]).Set([]byte("balance"), []byte("7"))
	pre.GetKVStore(preKeys[base.Added[0]]).Set([]byte("evm-key"), []byte("evm-val"))
	require.Equal(t, int64(1), pre.Commit().Version)

	// Re-open with the same mounted set and run the add-only loader.
	next := newTestStore(db)
	nextKeys := mountKeys(next, names)
	require.NoError(t, AddOnlyStoreLoader(2, base, log.NewNopLogger())(next))

	// Both stores' data survive untouched.
	require.Equal(t, []byte("7"), next.GetKVStore(nextKeys["bank"]).Get([]byte("balance")))
	require.Equal(t, []byte("evm-val"), next.GetKVStore(nextKeys[base.Added[0]]).Get([]byte("evm-key")))
}
