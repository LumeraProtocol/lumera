package upgrades

import (
	"testing"

	storetypes "cosmossdk.io/store/types"
	"github.com/stretchr/testify/require"
)

func TestComputeAdaptiveStoreUpgrades(t *testing.T) {
	expected := setOf("auth", "bank", "pfm")
	existing := setOf("auth", "bank", "nft")

	base := &storetypes.StoreUpgrades{
		Added:   []string{"pfm"},
		Deleted: []string{"nft", "crisis"},
		Renamed: []storetypes.StoreRename{{OldKey: "old", NewKey: "new"}},
	}

	effective := computeAdaptiveStoreUpgrades(base, expected, existing)

	require.ElementsMatch(t, []string{"pfm"}, effective.Added)
	require.ElementsMatch(t, []string{"nft"}, effective.Deleted)
	require.Equal(t, base.Renamed, effective.Renamed)
}

func TestComputeAdaptiveStoreUpgradesFiltersExistingAdds(t *testing.T) {
	expected := setOf("auth", "bank")
	existing := setOf("auth", "bank")

	base := &storetypes.StoreUpgrades{
		Added:   []string{"auth"},
		Deleted: []string{"crisis"},
	}

	effective := computeAdaptiveStoreUpgrades(base, expected, existing)

	require.Empty(t, effective.Added)
	require.Empty(t, effective.Deleted)
}

func TestComputeAdaptiveStoreUpgradesKeepsMultipleMissingEVMStores(t *testing.T) {
	expected := setOf("auth", "bank", "feemarket", "precisebank", "evm", "erc20")
	existing := setOf("auth", "bank")

	effective := computeAdaptiveStoreUpgrades(nil, expected, existing)

	require.ElementsMatch(t, []string{"feemarket", "precisebank", "evm", "erc20"}, effective.Added)
	require.Empty(t, effective.Deleted)
}

func TestComputeAddOnlyStoreUpgradesAddsMissing(t *testing.T) {
	existing := setOf("auth", "bank")
	base := &storetypes.StoreUpgrades{
		Added: []string{"feemarket", "precisebank", "evm", "erc20", "evmigration"},
	}

	effective := computeAddOnlyStoreUpgrades(base, existing)

	require.ElementsMatch(t, []string{"feemarket", "precisebank", "evm", "erc20", "evmigration"}, effective.Added)
	require.Empty(t, effective.Deleted)
	require.Empty(t, effective.Renamed)
}

func TestComputeAddOnlyStoreUpgradesSkipsPresent(t *testing.T) {
	existing := setOf("auth", "bank", "feemarket", "evm")
	base := &storetypes.StoreUpgrades{
		Added: []string{"feemarket", "precisebank", "evm", "erc20", "evmigration"},
	}

	effective := computeAddOnlyStoreUpgrades(base, existing)

	require.ElementsMatch(t, []string{"precisebank", "erc20", "evmigration"}, effective.Added)
	require.Empty(t, effective.Deleted)
}

func TestComputeAddOnlyStoreUpgradesNoopWhenAllPresent(t *testing.T) {
	existing := setOf("auth", "bank", "feemarket", "precisebank", "evm", "erc20", "evmigration")
	base := &storetypes.StoreUpgrades{
		Added: []string{"feemarket", "precisebank", "evm", "erc20", "evmigration"},
	}

	effective := computeAddOnlyStoreUpgrades(base, existing)

	require.Empty(t, effective.Added)
	require.Empty(t, effective.Deleted)
}

// The add-only computation must NEVER delete or rename a store, even if the base
// upgrade declares deletions/renames. This is the safety property that prevents a
// binary-misregistration bug from wiping a live store on mainnet.
func TestComputeAddOnlyStoreUpgradesIgnoresDeletesAndRenames(t *testing.T) {
	existing := setOf("auth", "bank", "nft", "crisis")
	base := &storetypes.StoreUpgrades{
		Added:   []string{"evm"},
		Deleted: []string{"nft", "crisis"},
		Renamed: []storetypes.StoreRename{{OldKey: "old", NewKey: "new"}},
	}

	effective := computeAddOnlyStoreUpgrades(base, existing)

	require.ElementsMatch(t, []string{"evm"}, effective.Added)
	require.Empty(t, effective.Deleted, "add-only must never delete a store")
	require.Empty(t, effective.Renamed, "add-only must never rename a store")
}

func TestComputeAddOnlyStoreUpgradesNilBase(t *testing.T) {
	effective := computeAddOnlyStoreUpgrades(nil, setOf("auth"))

	require.Empty(t, effective.Added)
	require.Empty(t, effective.Deleted)
}

func setOf(names ...string) map[string]struct{} {
	out := make(map[string]struct{}, len(names))
	for _, name := range names {
		out[name] = struct{}{}
	}
	return out
}
