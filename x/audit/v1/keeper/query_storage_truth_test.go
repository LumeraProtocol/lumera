package keeper_test

import (
	"testing"

	"github.com/LumeraProtocol/lumera/x/audit/v1/keeper"
	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestNodeSuspicionStateQuery(t *testing.T) {
	f := initFixture(t)
	qs := keeper.NewQueryServerImpl(f.keeper)

	_, err := qs.NodeSuspicionState(f.ctx, nil)
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))

	_, err = qs.NodeSuspicionState(f.ctx, &types.QueryNodeSuspicionStateRequest{})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))

	_, err = qs.NodeSuspicionState(f.ctx, &types.QueryNodeSuspicionStateRequest{SupernodeAccount: "lumera1missing"})
	require.Error(t, err)
	require.Equal(t, codes.NotFound, status.Code(err))

	state := types.NodeSuspicionState{
		SupernodeAccount: "lumera1node111111111111111111111111111111dlux4",
		SuspicionScore:   21,
		LastUpdatedEpoch: 19,
	}
	require.NoError(t, f.keeper.SetNodeSuspicionState(f.ctx, state))

	resp, err := qs.NodeSuspicionState(f.ctx, &types.QueryNodeSuspicionStateRequest{SupernodeAccount: state.SupernodeAccount})
	require.NoError(t, err)
	require.Equal(t, state, resp.State)
}

func TestReporterReliabilityStateQuery(t *testing.T) {
	f := initFixture(t)
	qs := keeper.NewQueryServerImpl(f.keeper)

	state := types.ReporterReliabilityState{
		ReporterSupernodeAccount: "lumera1reporter111111111111111111111111lyv93",
		ReliabilityScore:         -9,
		LastUpdatedEpoch:         20,
	}
	require.NoError(t, f.keeper.SetReporterReliabilityState(f.ctx, state))

	resp, err := qs.ReporterReliabilityState(f.ctx, &types.QueryReporterReliabilityStateRequest{
		ReporterSupernodeAccount: state.ReporterSupernodeAccount,
	})
	require.NoError(t, err)
	require.Equal(t, state, resp.State)
}

func TestTicketDeteriorationStateQuery(t *testing.T) {
	f := initFixture(t)
	qs := keeper.NewQueryServerImpl(f.keeper)

	state := types.TicketDeteriorationState{
		TicketId:            "ticket-query-1",
		DeteriorationScore:  30,
		LastUpdatedEpoch:    21,
		ProbationUntilEpoch: 23,
	}
	require.NoError(t, f.keeper.SetTicketDeteriorationState(f.ctx, state))

	resp, err := qs.TicketDeteriorationState(f.ctx, &types.QueryTicketDeteriorationStateRequest{
		TicketId: state.TicketId,
	})
	require.NoError(t, err)
	require.Equal(t, state, resp.State)
}

func TestHealOpQueries(t *testing.T) {
	f := initFixture(t)
	qs := keeper.NewQueryServerImpl(f.keeper)

	healOp1 := types.HealOp{
		HealOpId:               1,
		TicketId:               "ticket-abc",
		ScheduledEpochId:       3,
		HealerSupernodeAccount: "lumera1healer1111111111111111111111111399f2",
		Status:                 types.HealOpStatus_HEAL_OP_STATUS_SCHEDULED,
	}
	healOp2 := types.HealOp{
		HealOpId:               2,
		TicketId:               "ticket-abc",
		ScheduledEpochId:       4,
		HealerSupernodeAccount: "lumera1healer2222222222222222222222222v5r0s",
		Status:                 types.HealOpStatus_HEAL_OP_STATUS_IN_PROGRESS,
	}
	healOp3 := types.HealOp{
		HealOpId:               3,
		TicketId:               "ticket-def",
		ScheduledEpochId:       5,
		HealerSupernodeAccount: "lumera1healer3333333333333333333333333ea4u8",
		Status:                 types.HealOpStatus_HEAL_OP_STATUS_IN_PROGRESS,
	}

	require.NoError(t, f.keeper.SetHealOp(f.ctx, healOp1))
	require.NoError(t, f.keeper.SetHealOp(f.ctx, healOp2))
	require.NoError(t, f.keeper.SetHealOp(f.ctx, healOp3))

	respOne, err := qs.HealOp(f.ctx, &types.QueryHealOpRequest{HealOpId: 2})
	require.NoError(t, err)
	require.Equal(t, healOp2, respOne.HealOp)

	respByTicket, err := qs.HealOpsByTicket(f.ctx, &types.QueryHealOpsByTicketRequest{TicketId: "ticket-abc"})
	require.NoError(t, err)
	require.Len(t, respByTicket.HealOps, 2)

	respByStatus, err := qs.HealOpsByStatus(f.ctx, &types.QueryHealOpsByStatusRequest{Status: types.HealOpStatus_HEAL_OP_STATUS_IN_PROGRESS})
	require.NoError(t, err)
	require.Len(t, respByStatus.HealOps, 2)

	_, err = qs.HealOpsByStatus(f.ctx, &types.QueryHealOpsByStatusRequest{Status: types.HealOpStatus_HEAL_OP_STATUS_UNSPECIFIED})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}
