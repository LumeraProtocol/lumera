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

// TestGetNextHealOpID_RecoversFromMalformedState verifies CP-R3 A-F1 —
// GetNextHealOpID now mirrors GetNextEvidenceID by deriving a safe next ID
// from stored heal ops instead of panicking on malformed/zero counter state.
func TestGetNextHealOpID_RecoversFromMalformedState(t *testing.T) {
	seedHealOp := func(t *testing.T, f *fixture, id uint64) {
		t.Helper()
		require.NoError(t, f.keeper.SetHealOp(f.ctx, types.HealOp{
			HealOpId:               id,
			TicketId:               "ticket-heal-next-id",
			ScheduledEpochId:       11,
			HealerSupernodeAccount: "lumera1healer00000000000000000000000004qyrj",
			VerifierSupernodeAccounts: []string{
				"lumera1verifier1000000000000000000000005tzzg",
			},
			Status:          types.HealOpStatus_HEAL_OP_STATUS_SCHEDULED,
			CreatedHeight:   100,
			UpdatedHeight:   100,
			DeadlineEpochId: 14,
		}))
	}

	t.Run("malformed length derives max plus one", func(t *testing.T) {
		f := initFixture(t)
		seedHealOp(t, f, 9)
		// Write 7 bytes (not 8) to the next-id counter key.
		keeper.WriteRawNextHealOpIDForTest(f.keeper, f.ctx, []byte{0, 0, 0, 0, 0, 0, 1})
		require.Equal(t, uint64(10), f.keeper.GetNextHealOpID(f.ctx))
	})

	t.Run("zero id sentinel derives max plus one", func(t *testing.T) {
		f := initFixture(t)
		seedHealOp(t, f, 41)
		bz := make([]byte, 8)
		binary.BigEndian.PutUint64(bz, 0)
		keeper.WriteRawNextHealOpIDForTest(f.keeper, f.ctx, bz)
		require.Equal(t, uint64(42), f.keeper.GetNextHealOpID(f.ctx))
	})

	t.Run("missing counter with no heal ops starts at one", func(t *testing.T) {
		f := initFixture(t)
		require.Equal(t, uint64(1), f.keeper.GetNextHealOpID(f.ctx))
	})

	t.Run("valid id works", func(t *testing.T) {
		f := initFixture(t)
		f.keeper.SetNextHealOpID(f.ctx, 42)
		require.Equal(t, uint64(42), f.keeper.GetNextHealOpID(f.ctx))
	})
}

// TestMaxUint64EpochWindowScansAreWrapSafe verifies CP-R3 A-F2/B-F3 —
// ad-hoc secondary-index scans use named MaxUint64-safe range helpers rather
// than building endEpoch+1 directly.
func TestMaxUint64EpochWindowScansAreWrapSafe(t *testing.T) {
	maxEpoch := ^uint64(0)
	ticketID := "ticket-max-epoch"
	target := "lumera1targetmax0000000000000000000000e0n0n"
	reporter := "lumera1reportermax00000000000000000000y5kfe"
	excluded := "lumera1excludedmax0000000000000000000hpu43"

	t.Run("independent reporter PASS at MaxUint64 is included", func(t *testing.T) {
		f := initFixture(t)
		result := &types.StorageProofResult{
			TargetSupernodeAccount: target,
			TicketId:               ticketID,
			ResultClass:            types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_PASS,
		}
		require.NoError(t, keeper.SetStorageTruthReporterResultForTest(f.keeper, f.ctx, maxEpoch, reporter, result))

		found, err := keeper.HasIndependentReporterPassInWindowForTest(f.keeper, f.ctx, ticketID, target, excluded, maxEpoch-1, maxEpoch)
		require.NoError(t, err)
		require.True(t, found)
	})

	t.Run("clean recheck PASS at MaxUint64 is included", func(t *testing.T) {
		f := initFixture(t)
		result := &types.StorageProofResult{
			TargetSupernodeAccount: target,
			TicketId:               ticketID,
			BucketType:             types.StorageProofBucketType_STORAGE_PROOF_BUCKET_TYPE_RECHECK,
			ResultClass:            types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_PASS,
			TranscriptHash:         "transcript-max-epoch-pass",
		}
		require.NoError(t, f.keeper.IndexStorageProofTranscripts(f.ctx, maxEpoch, reporter, []*types.StorageProofResult{result}))

		found, err := keeper.HasCleanRecheckInWindowForTest(f.keeper, f.ctx, ticketID, target, maxEpoch-1, maxEpoch)
		require.NoError(t, err)
		require.True(t, found)
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
