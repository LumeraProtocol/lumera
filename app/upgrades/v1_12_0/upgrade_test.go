package v1_12_0

import (
	"regexp"
	"testing"

	appParams "github.com/LumeraProtocol/lumera/app/upgrades/params"
	audittypes "github.com/LumeraProtocol/lumera/x/audit/v1/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// AT42: Upgrade handler initializes Everlight store and params with defaults
// ---------------------------------------------------------------------------

func TestUpgradeName(t *testing.T) {
	require.NotEmpty(t, UpgradeName)
	require.Contains(t, UpgradeName, "v1.")
}

func TestUpgradeNameFollowsSemverPattern(t *testing.T) {
	// UpgradeName must be a valid semver-style string "vMAJOR.MINOR.PATCH".
	matched, err := regexp.MatchString(`^v\d+\.\d+\.\d+$`, UpgradeName)
	require.NoError(t, err)
	require.True(t, matched,
		"UpgradeName %q should match the semver pattern vX.Y.Z", UpgradeName)
}

func TestStoreUpgradesAddsNoDedicatedStore(t *testing.T) {
	require.Empty(t, StoreUpgrades.Added,
		"StoreUpgrades.Added should be empty because Everlight now uses the supernode store")
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

// ---------------------------------------------------------------------------
// Activation policy constants — pin so future changes are intentional and
// surface in code review.
// ---------------------------------------------------------------------------

func TestActivationPolicyConstants(t *testing.T) {
	require.True(t, everlightEnabled,
		"everlightEnabled must be true at activation; Everlight defaults are persisted")
	require.Equal(t,
		audittypes.StorageTruthEnforcementMode_STORAGE_TRUTH_ENFORCEMENT_MODE_SHADOW,
		storageTruthEnforcementMode,
		"LEP-6 enforcement mode at activation must be SHADOW")
}
