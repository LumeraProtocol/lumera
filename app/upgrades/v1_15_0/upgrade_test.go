package v1_15_0

import (
	"testing"

	everlighttypes "github.com/LumeraProtocol/lumera/x/everlight/v1/types"
	"github.com/stretchr/testify/require"
)

func TestUpgradeName(t *testing.T) {
	require.Equal(t, "v1.15.0", UpgradeName)
}

func TestStoreUpgradesAddsEverlight(t *testing.T) {
	require.Contains(t, StoreUpgrades.Added, everlighttypes.StoreKey,
		"StoreUpgrades.Added should contain the everlight store key")
}

func TestStoreUpgradesDeletedIsEmpty(t *testing.T) {
	require.Empty(t, StoreUpgrades.Deleted,
		"StoreUpgrades.Deleted should be empty — no existing stores are removed")
}
