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
		TicketId:            "ticket-1",
		DeteriorationScore:  25,
		LastUpdatedEpoch:    9,
		ActiveHealOpId:      3,
		ProbationUntilEpoch: 12,
		LastHealEpoch:       10,
	}

	require.False(t, f.keeper.HasTicketDeteriorationState(f.ctx, state.TicketId))
	require.NoError(t, f.keeper.SetTicketDeteriorationState(f.ctx, state))

	got, found := f.keeper.GetTicketDeteriorationState(f.ctx, state.TicketId)
	require.True(t, found)
	require.Equal(t, state, got)
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
