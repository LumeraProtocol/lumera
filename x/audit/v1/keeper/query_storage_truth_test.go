package keeper_test

import (
	"testing"

	"github.com/LumeraProtocol/lumera/x/audit/v1/keeper"
	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
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
		TrustBand:                types.ReporterTrustBand_REPORTER_TRUST_BAND_LOW_TRUST,
		ContradictionCount:       2,
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
		TicketId:                     "ticket-query-1",
		DeteriorationScore:           30,
		LastUpdatedEpoch:             21,
		ProbationUntilEpoch:          23,
		LastFailureEpoch:             19,
		RecentFailureEpochCount:      2,
		ContradictionCount:           1,
		LastTargetSupernodeAccount:   "lumera1target1111111111111111111111111w4zx",
		LastReporterSupernodeAccount: "lumera1reporter111111111111111111111111lyv93",
		LastResultClass:              types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_HASH_MISMATCH,
		LastResultEpoch:              21,
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

func TestStorageTruthQueries_ReflectScoredReportIngestion(t *testing.T) {
	f := initFixture(t)
	f.ctx = f.ctx.WithBlockHeight(1).WithEventManager(sdk.NewEventManager())
	qs := keeper.NewQueryServerImpl(f.keeper)
	ms := keeper.NewMsgServerImpl(f.keeper)

	reporter := "sn-aaa-reporter"
	target := "sn-bbb-target"

	f.supernodeKeeper.EXPECT().
		GetSuperNodeByAccount(gomock.Any(), reporter).
		Return(sntypes.SuperNode{}, true, nil).
		AnyTimes()

	seedEpochAnchorForReportTest(t, f, 0, []string{reporter, target}, []string{reporter, target})

	portStates := fullOpenPortStates()
	_, err := ms.SubmitEpochReport(f.ctx, &types.MsgSubmitEpochReport{
		Creator: reporter,
		EpochId: 0,
		HostReport: types.HostReport{
			InboundPortStates: portStates,
		},
		StorageChallengeObservations: []*types.StorageChallengeObservation{
			{
				TargetSupernodeAccount: target,
				PortStates:             portStates,
			},
		},
		StorageProofResults: []*types.StorageProofResult{
			{
				TargetSupernodeAccount:     target,
				ChallengerSupernodeAccount: reporter,
				TicketId:                   "ticket-query-score-1",
				BucketType:                 types.StorageProofBucketType_STORAGE_PROOF_BUCKET_TYPE_RECENT,
				ArtifactClass:              types.StorageProofArtifactClass_STORAGE_PROOF_ARTIFACT_CLASS_INDEX,
				ArtifactOrdinal:            1,
				ArtifactKey:                "artifact-key-1",
				ResultClass:                types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_HASH_MISMATCH,
				TranscriptHash:             "transcript-hash-1",
			},
		},
	})
	require.NoError(t, err)

	nodeResp, err := qs.NodeSuspicionState(f.ctx, &types.QueryNodeSuspicionStateRequest{SupernodeAccount: target})
	require.NoError(t, err)
	// HASH_MISMATCH + INDEX artifact: node=+26 (spec-aligned value)
	require.Equal(t, int64(26), nodeResp.State.SuspicionScore)
	require.Equal(t, uint64(0), nodeResp.State.LastUpdatedEpoch)

	reporterResp, err := qs.ReporterReliabilityState(f.ctx, &types.QueryReporterReliabilityStateRequest{
		ReporterSupernodeAccount: reporter,
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), reporterResp.State.ReliabilityScore)
	require.Equal(t, uint64(0), reporterResp.State.LastUpdatedEpoch)
	require.Equal(t, types.ReporterTrustBand_REPORTER_TRUST_BAND_NORMAL, reporterResp.State.TrustBand)
	require.Equal(t, uint64(0), reporterResp.State.ContradictionCount)

	ticketResp, err := qs.TicketDeteriorationState(f.ctx, &types.QueryTicketDeteriorationStateRequest{
		TicketId: "ticket-query-score-1",
	})
	require.NoError(t, err)
	require.Equal(t, int64(12), ticketResp.State.DeteriorationScore)
	require.Equal(t, uint64(0), ticketResp.State.LastUpdatedEpoch)
	require.Equal(t, target, ticketResp.State.LastTargetSupernodeAccount)
	require.Equal(t, reporter, ticketResp.State.LastReporterSupernodeAccount)
	require.Equal(t, types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_HASH_MISMATCH, ticketResp.State.LastResultClass)
	require.Equal(t, uint64(0), ticketResp.State.LastResultEpoch)
}
