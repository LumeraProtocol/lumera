package keeper_test

import (
	"testing"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
	"github.com/stretchr/testify/require"
)

func TestNodeSuspicionStateRoundTrip(t *testing.T) {
	f := initFixture(t)

	state := types.NodeSuspicionState{
		SupernodeAccount: "lumera1node000000000000000000000000000000l2sya",
		SuspicionScore:   15,
		LastUpdatedEpoch: 7,
	}

	require.False(t, f.keeper.HasNodeSuspicionState(f.ctx, state.SupernodeAccount))
	_, found := f.keeper.GetNodeSuspicionState(f.ctx, state.SupernodeAccount)
	require.False(t, found)

	require.NoError(t, f.keeper.SetNodeSuspicionState(f.ctx, state))
	require.True(t, f.keeper.HasNodeSuspicionState(f.ctx, state.SupernodeAccount))

	got, found := f.keeper.GetNodeSuspicionState(f.ctx, state.SupernodeAccount)
	require.True(t, found)
	require.Equal(t, state, got)
}

func TestReporterReliabilityStateRoundTrip(t *testing.T) {
	f := initFixture(t)

	state := types.ReporterReliabilityState{
		ReporterSupernodeAccount: "lumera1reporter0000000000000000000000000m09fa",
		ReliabilityScore:         -5,
		LastUpdatedEpoch:         8,
		TrustBand:                types.ReporterTrustBand_REPORTER_TRUST_BAND_LOW_TRUST,
		ContradictionCount:       3,
	}

	require.False(t, f.keeper.HasReporterReliabilityState(f.ctx, state.ReporterSupernodeAccount))
	require.NoError(t, f.keeper.SetReporterReliabilityState(f.ctx, state))

	got, found := f.keeper.GetReporterReliabilityState(f.ctx, state.ReporterSupernodeAccount)
	require.True(t, found)
	require.Equal(t, state, got)
}

func TestTicketDeteriorationStateRoundTrip(t *testing.T) {
	f := initFixture(t)

	state := types.TicketDeteriorationState{
		TicketId:                     "ticket-1",
		DeteriorationScore:           25,
		LastUpdatedEpoch:             9,
		ActiveHealOpId:               3,
		ProbationUntilEpoch:          12,
		LastHealEpoch:                10,
		LastFailureEpoch:             8,
		RecentFailureEpochCount:      2,
		ContradictionCount:           1,
		LastTargetSupernodeAccount:   "lumera1target0000000000000000000000000g6we",
		LastReporterSupernodeAccount: "lumera1reporter0000000000000000000000000m09fa",
		LastResultClass:              types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_HASH_MISMATCH,
		LastResultEpoch:              9,
	}

	require.False(t, f.keeper.HasTicketDeteriorationState(f.ctx, state.TicketId))
	require.NoError(t, f.keeper.SetTicketDeteriorationState(f.ctx, state))

	got, found := f.keeper.GetTicketDeteriorationState(f.ctx, state.TicketId)
	require.True(t, found)
	require.Equal(t, state, got)
}

func TestTicketArtifactCountStateRoundTrip(t *testing.T) {
	f := initFixture(t)

	state := types.TicketArtifactCountState{
		TicketId:            "ticket-artifacts-1",
		IndexArtifactCount:  32,
		SymbolArtifactCount: 128,
	}

	require.False(t, f.keeper.HasTicketArtifactCountState(f.ctx, state.TicketId))
	require.NoError(t, f.keeper.SetTicketArtifactCountState(f.ctx, state))
	require.True(t, f.keeper.HasTicketArtifactCountState(f.ctx, state.TicketId))

	got, found := f.keeper.GetTicketArtifactCountState(f.ctx, state.TicketId)
	require.True(t, found)
	require.Equal(t, state, got)
}

func TestSetStorageTruthTicketArtifactCounts_ImmutableOnceSet(t *testing.T) {
	f := initFixture(t)

	require.NoError(t, f.keeper.SetStorageTruthTicketArtifactCounts(f.ctx, "ticket-artifacts-2", 10, 40))

	// Exact replay is allowed.
	require.NoError(t, f.keeper.SetStorageTruthTicketArtifactCounts(f.ctx, "ticket-artifacts-2", 10, 40))

	// Divergent values are rejected.
	err := f.keeper.SetStorageTruthTicketArtifactCounts(f.ctx, "ticket-artifacts-2", 11, 40)
	require.Error(t, err)
	require.Contains(t, err.Error(), "immutable")
}

func TestHealOpAndNextIDRoundTrip(t *testing.T) {
	f := initFixture(t)

	require.Equal(t, uint64(1), f.keeper.GetNextHealOpID(f.ctx))
	f.keeper.SetNextHealOpID(f.ctx, 22)
	require.Equal(t, uint64(22), f.keeper.GetNextHealOpID(f.ctx))

	healOp := types.HealOp{
		HealOpId:               5,
		TicketId:               "ticket-5",
		ScheduledEpochId:       11,
		HealerSupernodeAccount: "lumera1healer00000000000000000000000004qyrj",
		VerifierSupernodeAccounts: []string{
			"lumera1verifier1000000000000000000000005tzzg",
			"lumera1verifier200000000000000000000000w2x4k",
		},
		Status:          types.HealOpStatus_HEAL_OP_STATUS_SCHEDULED,
		CreatedHeight:   100,
		UpdatedHeight:   100,
		DeadlineEpochId: 14,
		ResultHash:      "hash-1",
		Notes:           "initial",
	}

	require.False(t, f.keeper.HasHealOp(f.ctx, healOp.HealOpId))
	require.NoError(t, f.keeper.SetHealOp(f.ctx, healOp))
	require.True(t, f.keeper.HasHealOp(f.ctx, healOp.HealOpId))

	got, found := f.keeper.GetHealOp(f.ctx, healOp.HealOpId)
	require.True(t, found)
	require.Equal(t, healOp, got)

	// Update should replace status/ticket indices without duplicating stale ones.
	healOp.Status = types.HealOpStatus_HEAL_OP_STATUS_IN_PROGRESS
	healOp.TicketId = "ticket-5b"
	healOp.UpdatedHeight = 101
	require.NoError(t, f.keeper.SetHealOp(f.ctx, healOp))

	got, found = f.keeper.GetHealOp(f.ctx, healOp.HealOpId)
	require.True(t, found)
	require.Equal(t, healOp, got)
}

func TestHealOpVerificationRoundTrip(t *testing.T) {
	f := initFixture(t)

	healOpID := uint64(44)
	verifierA := "lumera1verifiera00000000000000000000000h7v3e"
	verifierB := "lumera1verifierb00000000000000000000000z9f3r"

	require.False(t, f.keeper.HasHealOpVerification(f.ctx, healOpID, verifierA))
	_, found := f.keeper.GetHealOpVerification(f.ctx, healOpID, verifierA)
	require.False(t, found)

	f.keeper.SetHealOpVerification(f.ctx, healOpID, verifierA, true)
	f.keeper.SetHealOpVerification(f.ctx, healOpID, verifierB, false)

	require.True(t, f.keeper.HasHealOpVerification(f.ctx, healOpID, verifierA))
	value, found := f.keeper.GetHealOpVerification(f.ctx, healOpID, verifierA)
	require.True(t, found)
	require.True(t, value)

	value, found = f.keeper.GetHealOpVerification(f.ctx, healOpID, verifierB)
	require.True(t, found)
	require.False(t, value)

	all, err := f.keeper.GetAllHealOpVerifications(f.ctx, healOpID)
	require.NoError(t, err)
	require.Len(t, all, 2)
	require.True(t, all[verifierA])
	require.False(t, all[verifierB])
}
