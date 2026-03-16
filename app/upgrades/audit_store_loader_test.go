package upgrades

import (
	"testing"

	storetypes "cosmossdk.io/store/types"
	"github.com/stretchr/testify/require"
)

func TestComputeAuditStoreUpgrades_AddsAuditWhenMissing(t *testing.T) {
	base := &storetypes.StoreUpgrades{Added: []string{"audit"}}
	existing := setOf("auth", "bank")

	effective := computeAuditStoreUpgrades(base, existing)
	require.ElementsMatch(t, []string{"audit"}, effective.Added)
	require.Empty(t, effective.Deleted)
	require.Empty(t, effective.Renamed)
}

func TestComputeAuditStoreUpgrades_SkipsAuditWhenAlreadyPresent(t *testing.T) {
	base := &storetypes.StoreUpgrades{Added: []string{"audit"}}
	existing := setOf("auth", "bank", "audit")

	effective := computeAuditStoreUpgrades(base, existing)
	require.Empty(t, effective.Added)
	require.Empty(t, effective.Deleted)
	require.Empty(t, effective.Renamed)
}
