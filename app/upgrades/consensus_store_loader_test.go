package upgrades

import (
	"testing"

	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"
	"github.com/stretchr/testify/require"
)

func TestComputeConsensusStoreUpgrades_RenameWhenLegacyOnly(t *testing.T) {
	expected := setOf("consensus")
	existing := setOf("Consensus", "crisis")
	base := &storetypes.StoreUpgrades{
		Deleted: []string{"crisis"},
	}

	effective := computeConsensusStoreUpgrades(base, expected, existing, log.NewNopLogger())

	require.Len(t, effective.Renamed, 1)
	require.Equal(t, storetypes.StoreRename{OldKey: "Consensus", NewKey: "consensus"}, effective.Renamed[0])
	require.NotContains(t, effective.Added, "consensus")
	require.NotContains(t, effective.Deleted, "Consensus")
	require.Contains(t, effective.Deleted, "crisis")
}

func TestComputeConsensusStoreUpgrades_NoRenameWhenNewExists(t *testing.T) {
	expected := setOf("consensus")
	existing := setOf("consensus")

	effective := computeConsensusStoreUpgrades(nil, expected, existing, log.NewNopLogger())

	require.Empty(t, effective.Renamed)
	require.Empty(t, effective.Added)
	require.Empty(t, effective.Deleted)
}

func TestComputeConsensusStoreUpgrades_AddsConsensusWhenMissingNonAdaptive(t *testing.T) {
	existing := map[string]struct{}{}

	effective := computeConsensusStoreUpgrades(nil, nil, existing, log.NewNopLogger())

	require.Contains(t, effective.Added, "consensus")
	require.Empty(t, effective.Renamed)
}

func TestComputeConsensusStoreUpgrades_NoRenameWhenBothExist(t *testing.T) {
	expected := setOf("consensus")
	existing := setOf("Consensus", "consensus")

	effective := computeConsensusStoreUpgrades(nil, expected, existing, log.NewNopLogger())

	require.Empty(t, effective.Renamed)
	require.NotContains(t, effective.Deleted, "Consensus")
}
