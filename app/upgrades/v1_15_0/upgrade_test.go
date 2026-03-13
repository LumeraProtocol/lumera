package v1_15_0

import (
	"regexp"
	"testing"

	appParams "github.com/LumeraProtocol/lumera/app/upgrades/params"
	everlighttypes "github.com/LumeraProtocol/lumera/x/everlight/v1/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// AT42: Upgrade handler initializes Everlight store and params with defaults
// ---------------------------------------------------------------------------

func TestUpgradeName(t *testing.T) {
	require.Equal(t, "v1.15.0", UpgradeName)
}

func TestUpgradeNameFollowsSemverPattern(t *testing.T) {
	// UpgradeName must be a valid semver-style string "vMAJOR.MINOR.PATCH".
	matched, err := regexp.MatchString(`^v\d+\.\d+\.\d+$`, UpgradeName)
	require.NoError(t, err)
	require.True(t, matched,
		"UpgradeName %q should match the semver pattern vX.Y.Z", UpgradeName)
}

func TestStoreUpgradesAddsEverlight(t *testing.T) {
	require.Contains(t, StoreUpgrades.Added, everlighttypes.StoreKey,
		"StoreUpgrades.Added should contain the everlight store key")
}

func TestStoreUpgradesAddedContainsOnlyEverlight(t *testing.T) {
	// Exactly one store key is added — the everlight module and nothing else.
	require.Len(t, StoreUpgrades.Added, 1,
		"StoreUpgrades.Added should contain exactly one entry")
	require.Equal(t, everlighttypes.StoreKey, StoreUpgrades.Added[0],
		"The sole added store key should be the everlight store key")
}

func TestStoreUpgradesDeletedIsEmpty(t *testing.T) {
	require.Empty(t, StoreUpgrades.Deleted,
		"StoreUpgrades.Deleted should be empty — no existing stores are removed")
}

// ---------------------------------------------------------------------------
// AT43: Existing SN states and actions unaffected by Everlight upgrade
// ---------------------------------------------------------------------------

func TestStoreUpgradesDoesNotDeleteExistingModules(t *testing.T) {
	// The Deleted list must NOT contain any of the pre-existing module store
	// keys. This provides executable evidence that the upgrade preserves all
	// existing module stores.
	protectedKeys := []string{
		"supernode",
		"action",
		"claim",
		"lumeraid",
		"bank",
		"staking",
		"distribution",
	}

	for _, key := range protectedKeys {
		assert.NotContains(t, StoreUpgrades.Deleted, key,
			"StoreUpgrades.Deleted must not contain %q — existing stores must be preserved", key)
	}
}

func TestStoreUpgradesDoesNotRenameExistingModules(t *testing.T) {
	// Renamed list should be empty (or at least not reference any existing stores).
	require.Empty(t, StoreUpgrades.Renamed,
		"StoreUpgrades.Renamed should be empty — no existing stores are renamed")
}

func TestCreateUpgradeHandlerReturnsNonNil(t *testing.T) {
	// Verify that CreateUpgradeHandler can be called with a zero-value params
	// struct without panicking, and that it returns a non-nil handler function.
	handler := CreateUpgradeHandler(appParams.AppUpgradeParams{})
	require.NotNil(t, handler,
		"CreateUpgradeHandler should return a non-nil upgrade handler function")
}
