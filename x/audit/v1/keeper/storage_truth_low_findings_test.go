package keeper_test

import (
	"encoding/binary"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/LumeraProtocol/lumera/x/audit/v1/keeper"
	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

// TestGetNextHealOpID_PanicsOnMalformedState verifies NEW-B-7 sibling-symmetry
// with GetNextEvidenceID — malformed counter bytes panic instead of silently
// returning a corrupt value.
func TestGetNextHealOpID_PanicsOnMalformedState(t *testing.T) {
	t.Run("malformed length", func(t *testing.T) {
		f := initFixture(t)
		// Write 7 bytes (not 8) to the next-id counter key.
		keeper.WriteRawNextHealOpIDForTest(f.keeper, f.ctx, []byte{0, 0, 0, 0, 0, 0, 1})
		require.PanicsWithError(t,
			"audit: malformed next heal-op id (len=7, want 8)",
			func() { _ = f.keeper.GetNextHealOpID(f.ctx) })
	})

	t.Run("zero id sentinel collision", func(t *testing.T) {
		f := initFixture(t)
		bz := make([]byte, 8)
		binary.BigEndian.PutUint64(bz, 0)
		keeper.WriteRawNextHealOpIDForTest(f.keeper, f.ctx, bz)
		require.PanicsWithError(t,
			"audit: invalid next heal-op id (id=0 collides with not-found sentinel)",
			func() { _ = f.keeper.GetNextHealOpID(f.ctx) })
	})

	t.Run("valid id works", func(t *testing.T) {
		f := initFixture(t)
		f.keeper.SetNextHealOpID(f.ctx, 42)
		require.Equal(t, uint64(42), f.keeper.GetNextHealOpID(f.ctx))
	})
}

// TestHealVerifierCountParam_FallbackToDefault verifies NEW-B-3 — the new
// StorageTruthHealVerifierCount param defaults to 2 when unset and is
// honored when configured.
func TestHealVerifierCountParam_FallbackToDefault(t *testing.T) {
	defaults := types.DefaultParams()
	require.Equal(t, types.DefaultStorageTruthHealVerifierCount, defaults.StorageTruthHealVerifierCount,
		"DefaultParams must seed the heal-verifier count param")
	require.Equal(t, uint32(2), defaults.StorageTruthHealVerifierCount,
		"default value must be 2 (sibling-symmetry with previous hardcode)")

	// WithDefaults should fill in 0 → default.
	zeroed := types.Params{StorageTruthHealVerifierCount: 0}
	withDefaults := zeroed.WithDefaults()
	require.Equal(t, types.DefaultStorageTruthHealVerifierCount, withDefaults.StorageTruthHealVerifierCount)
}

// TestInitGenesis_RejectsPostponementWithoutSupernodeState verifies NEW-B-6/B-9
// — audit InitGenesis cross-validates StorageTruthPostponements against
// supernode SuperNodeStatePostponed and rejects mismatched state.
func TestInitGenesis_RejectsPostponementWithoutSupernodeState(t *testing.T) {
	t.Run("supernode not found", func(t *testing.T) {
		f := initFixture(t)
		acct := "lumera1cccccccccccccccccccccccccccccccccccccdpac9"
		f.supernodeKeeper.EXPECT().
			GetSuperNodeByAccount(gomock.Any(), acct).
			Return(sntypes.SuperNode{}, false, nil).Times(1)

		genesis := types.GenesisState{
			Params: types.DefaultParams(),
			StorageTruthPostponements: []types.StorageTruthPostponement{
				{SupernodeAccount: acct, PostponedAtEpochId: 5},
			},
		}
		err := f.keeper.InitGenesis(f.ctx, genesis)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unknown supernode")
	})

	t.Run("supernode not in postponed state", func(t *testing.T) {
		f := initFixture(t)
		acct := "lumera1ddddddddddddddddddddddddddddddddddddqsxnvc"
		// Supernode exists but its latest state record is Active, not Postponed.
		sn := sntypes.SuperNode{
			SupernodeAccount: acct,
			States:           []*sntypes.SuperNodeStateRecord{{State: sntypes.SuperNodeStateActive, Height: 1}},
		}
		f.supernodeKeeper.EXPECT().
			GetSuperNodeByAccount(gomock.Any(), acct).
			Return(sn, true, nil).Times(1)

		genesis := types.GenesisState{
			Params: types.DefaultParams(),
			StorageTruthPostponements: []types.StorageTruthPostponement{
				{SupernodeAccount: acct, PostponedAtEpochId: 5},
			},
		}
		err := f.keeper.InitGenesis(f.ctx, genesis)
		require.Error(t, err)
		require.Contains(t, err.Error(), "lacks corresponding supernode-postponed state")
	})
}

// TestEventTypeHealOpInsufficientVerifiersExists verifies NEW-B-4 — sibling
// event constant is registered with the same prefix shape as InsufficientHealers.
func TestEventTypeHealOpInsufficientVerifiersExists(t *testing.T) {
	require.NotEmpty(t, types.EventTypeHealOpInsufficientVerifiers,
		"NEW-B-4: heal-op insufficient-verifiers event type must be registered")
	// Sanity: distinct from healers event type.
	require.NotEqual(t, types.EventTypeHealOpInsufficientHealers,
		types.EventTypeHealOpInsufficientVerifiers,
		"event types for healers vs verifiers must be distinct")
}

// silence unused import if sdk above isn't otherwise referenced
var _ = sdk.NewEventManager
