package keeper_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/LumeraProtocol/lumera/x/evmigration/keeper"
	"github.com/LumeraProtocol/lumera/x/evmigration/types"
)

// TestCheckMigrationActivationEmptyCanaryAllowsEveryone pins the semantics that
// determine whether a live network keeps working across this upgrade.
//
// # WHY THIS MATTERS
//
// An empty CanaryLegacyAddresses list means ALLOW ALL, not deny all. Combined
// with EnableMigration=true that is "migration fully open", which is exactly
// the state lumera-testnet-2 is in today (verified live 2026-07-30:
// enable_migration=true, no canary field set).
//
// I previously recommended flipping testnet to enable_migration=false before
// rollout. That recommendation was WRONG: it would have switched off a working
// testnet migration for no benefit. Migration being open is the point of the
// feature; the continuity work exists to make it SAFE, not to switch it off.
//
// This test exists so nobody "hardens" the empty-list case into a deny and
// silently breaks every open network at the next upgrade.
func TestCheckMigrationActivationEmptyCanaryAllowsEveryone(t *testing.T) {
	legacy := sdk.AccAddress([]byte("legacy______________"))
	other := sdk.AccAddress([]byte("other_______________"))

	t.Run("enabled with empty canary allows any address", func(t *testing.T) {
		params := types.Params{EnableMigration: true}
		require.NoError(t, keeper.CheckMigrationActivation(params, legacy),
			"empty canary list must ALLOW ALL - this is live testnet's state")
		require.NoError(t, keeper.CheckMigrationActivation(params, other),
			"empty canary list must not discriminate between addresses")
	})

	t.Run("disabled blocks even a listed address", func(t *testing.T) {
		params := types.Params{
			EnableMigration:       false,
			CanaryLegacyAddresses: []string{legacy.String()},
		}
		err := keeper.CheckMigrationActivation(params, legacy)
		require.ErrorIs(t, err, types.ErrMigrationDisabled,
			"EnableMigration=false must dominate the allowlist")
	})

	t.Run("non-empty canary restricts to listed addresses", func(t *testing.T) {
		params := types.Params{
			EnableMigration:       true,
			CanaryLegacyAddresses: []string{legacy.String()},
		}
		require.NoError(t, keeper.CheckMigrationActivation(params, legacy),
			"listed address must be admitted during canary")
		require.ErrorIs(t, keeper.CheckMigrationActivation(params, other),
			types.ErrMigrationNotCanary,
			"unlisted address must be rejected during canary")
	})
}

// TestCanaryIsOptIn documents the resulting three-state model so the operational
// meaning of each params combination is unambiguous:
//
//	EnableMigration=false                       -> closed  (mainnet's post-upgrade default)
//	EnableMigration=true,  canary empty         -> open    (testnet today, unchanged by this upgrade)
//	EnableMigration=true,  canary non-empty     -> canary  (opt-in narrowing)
//
// Canary is a NARROWING of an open network, entered by ADDING addresses. It is
// not a stage every network must pass through, and a network already running
// open does not need to be closed first.
func TestCanaryIsOptIn(t *testing.T) {
	addr := sdk.AccAddress([]byte("someaddress_________"))

	closed := types.Params{EnableMigration: false}
	open := types.Params{EnableMigration: true}
	canary := types.Params{EnableMigration: true, CanaryLegacyAddresses: []string{"lumera1someoneelse"}}

	require.Error(t, keeper.CheckMigrationActivation(closed, addr), "closed rejects")
	require.NoError(t, keeper.CheckMigrationActivation(open, addr), "open admits")
	require.Error(t, keeper.CheckMigrationActivation(canary, addr), "canary excludes unlisted")
}
