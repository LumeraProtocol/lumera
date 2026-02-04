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

func setOf(names ...string) map[string]struct{} {
	out := make(map[string]struct{}, len(names))
	for _, name := range names {
		out[name] = struct{}{}
	}
	return out
}
