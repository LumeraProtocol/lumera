package keeper_test

import (
	"testing"

	"github.com/LumeraProtocol/lumera/x/audit/v1/keeper"
	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func fullOpenPortStates() []types.PortState {
	portStates := make([]types.PortState, len(types.DefaultRequiredOpenPorts))
	for i := range portStates {
		portStates[i] = types.PortState_PORT_STATE_OPEN
	}
	return portStates
}

func baseStorageProofResult(class types.StorageProofResultClass) *types.StorageProofResult {
	result := &types.StorageProofResult{
		TargetSupernodeAccount:     "sn-bbb-target",
		ChallengerSupernodeAccount: "sn-aaa-reporter",
		BucketType:                 types.StorageProofBucketType_STORAGE_PROOF_BUCKET_TYPE_RECENT,
		ResultClass:                class,
		TranscriptHash:             "tx-hash-1",
	}

	if class == types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_NO_ELIGIBLE_TICKET {
		result.ArtifactClass = types.StorageProofArtifactClass_STORAGE_PROOF_ARTIFACT_CLASS_UNSPECIFIED
		return result
	}

	result.TicketId = "ticket-1"
	result.ArtifactClass = types.StorageProofArtifactClass_STORAGE_PROOF_ARTIFACT_CLASS_INDEX
	result.ArtifactOrdinal = 1
	result.ArtifactKey = "artifact-key-1"
	return result
}

func TestSubmitEpochReport_StorageTruthScoresByResultClass(t *testing.T) {
	tests := []struct {
		name                string
		class               types.StorageProofResultClass
		bucket              types.StorageProofBucketType
		expectedNodeScore   *int64
		expectedReporter    int64
		expectedTicketScore *int64
		expectedTicketID    string
	}{
		{
			name:                "pass",
			class:               types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_PASS,
			bucket:              types.StorageProofBucketType_STORAGE_PROOF_BUCKET_TYPE_RECENT,
			expectedNodeScore:   int64Ptr(-2),
			expectedReporter:    2,
			expectedTicketScore: int64Ptr(-3),
			expectedTicketID:    "ticket-1",
		},
		{
			name:                "hash mismatch",
			class:               types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_HASH_MISMATCH,
			bucket:              types.StorageProofBucketType_STORAGE_PROOF_BUCKET_TYPE_RECENT,
			expectedNodeScore:   int64Ptr(12),
			expectedReporter:    1,
			expectedTicketScore: int64Ptr(12),
			expectedTicketID:    "ticket-1",
		},
		{
			name:                "timeout",
			class:               types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_TIMEOUT_OR_NO_RESPONSE,
			bucket:              types.StorageProofBucketType_STORAGE_PROOF_BUCKET_TYPE_RECENT,
			expectedNodeScore:   int64Ptr(4),
			expectedReporter:    -1,
			expectedTicketScore: int64Ptr(4),
			expectedTicketID:    "ticket-1",
		},
		{
			name:                "observer quorum fail",
			class:               types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_OBSERVER_QUORUM_FAIL,
			bucket:              types.StorageProofBucketType_STORAGE_PROOF_BUCKET_TYPE_RECENT,
			expectedNodeScore:   int64Ptr(3),
			expectedReporter:    -3,
			expectedTicketScore: int64Ptr(5),
			expectedTicketID:    "ticket-1",
		},
		{
			name:                "no eligible ticket",
			class:               types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_NO_ELIGIBLE_TICKET,
			bucket:              types.StorageProofBucketType_STORAGE_PROOF_BUCKET_TYPE_RECENT,
			expectedNodeScore:   nil,
			expectedReporter:    1,
			expectedTicketScore: nil,
		},
		{
			name:                "invalid transcript",
			class:               types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_INVALID_TRANSCRIPT,
			bucket:              types.StorageProofBucketType_STORAGE_PROOF_BUCKET_TYPE_RECENT,
			expectedNodeScore:   nil,
			expectedReporter:    -8,
			expectedTicketScore: nil,
		},
		{
			name:                "recheck confirmed fail",
			class:               types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_RECHECK_CONFIRMED_FAIL,
			bucket:              types.StorageProofBucketType_STORAGE_PROOF_BUCKET_TYPE_RECHECK,
			expectedNodeScore:   int64Ptr(20),
			expectedReporter:    3,
			expectedTicketScore: int64Ptr(20),
			expectedTicketID:    "ticket-1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := initFixture(t)
			f.ctx = f.ctx.WithBlockHeight(1).WithEventManager(sdk.NewEventManager())
			ms := keeper.NewMsgServerImpl(f.keeper)

			reporter := "sn-aaa-reporter"
			target := "sn-bbb-target"
			f.supernodeKeeper.EXPECT().
				GetSuperNodeByAccount(gomock.Any(), reporter).
				Return(sntypes.SuperNode{}, true, nil).
				AnyTimes()

			seedEpochAnchorForReportTest(t, f, 0, []string{reporter, target}, []string{reporter, target})

			result := baseStorageProofResult(tc.class)
			result.BucketType = tc.bucket

			_, err := ms.SubmitEpochReport(f.ctx, &types.MsgSubmitEpochReport{
				Creator: reporter,
				EpochId: 0,
				HostReport: types.HostReport{
					InboundPortStates: fullOpenPortStates(),
				},
				StorageChallengeObservations: []*types.StorageChallengeObservation{
					{
						TargetSupernodeAccount: target,
						PortStates:             fullOpenPortStates(),
					},
				},
				StorageProofResults: []*types.StorageProofResult{result},
			})
			require.NoError(t, err)

			nodeState, found := f.keeper.GetNodeSuspicionState(f.ctx, target)
			if tc.expectedNodeScore == nil {
				require.False(t, found)
			} else {
				require.True(t, found)
				require.Equal(t, *tc.expectedNodeScore, nodeState.SuspicionScore)
				require.Equal(t, uint64(0), nodeState.LastUpdatedEpoch)
			}

			reporterState, found := f.keeper.GetReporterReliabilityState(f.ctx, reporter)
			require.True(t, found)
			require.Equal(t, tc.expectedReporter, reporterState.ReliabilityScore)
			require.Equal(t, uint64(0), reporterState.LastUpdatedEpoch)
			require.Equal(t, types.ReporterTrustBand_REPORTER_TRUST_BAND_NORMAL, reporterState.TrustBand)

			ticketState, found := f.keeper.GetTicketDeteriorationState(f.ctx, tc.expectedTicketID)
			if tc.expectedTicketScore == nil {
				require.False(t, found)
			} else {
				require.True(t, found)
				require.Equal(t, *tc.expectedTicketScore, ticketState.DeteriorationScore)
				require.Equal(t, uint64(0), ticketState.LastUpdatedEpoch)
				require.Equal(t, target, ticketState.LastTargetSupernodeAccount)
				require.Equal(t, reporter, ticketState.LastReporterSupernodeAccount)
				require.Equal(t, tc.class, ticketState.LastResultClass)
				require.Equal(t, uint64(0), ticketState.LastResultEpoch)
			}
		})
	}
}

func TestSubmitEpochReport_StorageTruthScoresApplyDecay(t *testing.T) {
	f := initFixture(t)
	f.ctx = f.ctx.WithBlockHeight(1201).WithEventManager(sdk.NewEventManager()) // epoch_id = 3
	ms := keeper.NewMsgServerImpl(f.keeper)

	params := f.keeper.GetParams(f.ctx).WithDefaults()
	params.StorageTruthNodeSuspicionDecayPerEpoch = 2
	params.StorageTruthReporterReliabilityDecayPerEpoch = 3
	params.StorageTruthTicketDeteriorationDecayPerEpoch = 4
	require.NoError(t, f.keeper.SetParams(f.ctx, params))

	reporter := "sn-aaa-reporter"
	target := "sn-bbb-target"
	ticketID := "ticket-1"

	require.NoError(t, f.keeper.SetNodeSuspicionState(f.ctx, types.NodeSuspicionState{
		SupernodeAccount: target,
		SuspicionScore:   10,
		LastUpdatedEpoch: 0,
	}))
	require.NoError(t, f.keeper.SetReporterReliabilityState(f.ctx, types.ReporterReliabilityState{
		ReporterSupernodeAccount: reporter,
		ReliabilityScore:         -9,
		LastUpdatedEpoch:         0,
	}))
	require.NoError(t, f.keeper.SetTicketDeteriorationState(f.ctx, types.TicketDeteriorationState{
		TicketId:            ticketID,
		DeteriorationScore:  20,
		LastUpdatedEpoch:    0,
		ActiveHealOpId:      9,
		ProbationUntilEpoch: 11,
		LastHealEpoch:       1,
	}))

	f.supernodeKeeper.EXPECT().
		GetSuperNodeByAccount(gomock.Any(), reporter).
		Return(sntypes.SuperNode{}, true, nil).
		AnyTimes()

	seedEpochAnchorForReportTest(t, f, 3, []string{reporter, target}, []string{reporter, target})

	result := baseStorageProofResult(types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_HASH_MISMATCH)
	result.TicketId = ticketID

	_, err := ms.SubmitEpochReport(f.ctx, &types.MsgSubmitEpochReport{
		Creator: reporter,
		EpochId: 3,
		HostReport: types.HostReport{
			InboundPortStates: fullOpenPortStates(),
		},
		StorageChallengeObservations: []*types.StorageChallengeObservation{
			{
				TargetSupernodeAccount: target,
				PortStates:             fullOpenPortStates(),
			},
		},
		StorageProofResults: []*types.StorageProofResult{result},
	})
	require.NoError(t, err)

	nodeState, found := f.keeper.GetNodeSuspicionState(f.ctx, target)
	require.True(t, found)
	require.Equal(t, int64(16), nodeState.SuspicionScore) // (10 - 2*3) + 12
	require.Equal(t, uint64(3), nodeState.LastUpdatedEpoch)

	reporterState, found := f.keeper.GetReporterReliabilityState(f.ctx, reporter)
	require.True(t, found)
	require.Equal(t, int64(1), reporterState.ReliabilityScore) // (-9 + 3*3) + 1 => 1
	require.Equal(t, uint64(3), reporterState.LastUpdatedEpoch)
	require.Equal(t, types.ReporterTrustBand_REPORTER_TRUST_BAND_NORMAL, reporterState.TrustBand)

	ticketState, found := f.keeper.GetTicketDeteriorationState(f.ctx, ticketID)
	require.True(t, found)
	require.Equal(t, int64(20), ticketState.DeteriorationScore) // (20 - 4*3) + 12 => 20
	require.Equal(t, uint64(3), ticketState.LastUpdatedEpoch)
	// Existing lifecycle metadata remains intact in PR3.
	require.Equal(t, uint64(9), ticketState.ActiveHealOpId)
	require.Equal(t, uint64(11), ticketState.ProbationUntilEpoch)
	require.Equal(t, uint64(1), ticketState.LastHealEpoch)
}

func TestSubmitEpochReport_StorageTruthScoreEventsAreEmitted(t *testing.T) {
	f := initFixture(t)
	f.ctx = f.ctx.WithBlockHeight(1).WithEventManager(sdk.NewEventManager())
	ms := keeper.NewMsgServerImpl(f.keeper)

	reporter := "sn-aaa-reporter"
	target := "sn-bbb-target"

	f.supernodeKeeper.EXPECT().
		GetSuperNodeByAccount(gomock.Any(), reporter).
		Return(sntypes.SuperNode{}, true, nil).
		AnyTimes()

	seedEpochAnchorForReportTest(t, f, 0, []string{reporter, target}, []string{reporter, target})

	_, err := ms.SubmitEpochReport(f.ctx, &types.MsgSubmitEpochReport{
		Creator: reporter,
		EpochId: 0,
		HostReport: types.HostReport{
			InboundPortStates: fullOpenPortStates(),
		},
		StorageChallengeObservations: []*types.StorageChallengeObservation{
			{
				TargetSupernodeAccount: target,
				PortStates:             fullOpenPortStates(),
			},
		},
		StorageProofResults: []*types.StorageProofResult{
			baseStorageProofResult(types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_PASS),
		},
	})
	require.NoError(t, err)

	events := f.ctx.EventManager().Events()
	require.NotEmpty(t, events)

	var found bool
	for _, event := range events {
		if event.Type != types.EventTypeStorageTruthScoreUpdated {
			continue
		}
		found = true

		attrs := make(map[string]string, len(event.Attributes))
		for _, attr := range event.Attributes {
			attrs[string(attr.Key)] = string(attr.Value)
		}

		require.Equal(t, types.ModuleName, attrs[sdk.AttributeKeyModule])
		require.Equal(t, "0", attrs[types.AttributeKeyEpochID])
		require.Equal(t, reporter, attrs[types.AttributeKeyReporterSupernodeAccount])
		require.Equal(t, target, attrs[types.AttributeKeyTargetSupernodeAccount])
		require.Equal(t, "ticket-1", attrs[types.AttributeKeyTicketID])
		require.Equal(t, types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_PASS.String(), attrs[types.AttributeKeyResultClass])
		require.Equal(t, types.ReporterTrustBand_REPORTER_TRUST_BAND_NORMAL.String(), attrs[types.AttributeKeyReporterTrustBand])
		require.Equal(t, "0", attrs[types.AttributeKeyRepeatedFailureCount])
		require.Equal(t, "false", attrs[types.AttributeKeyContradictionDetected])
		require.Equal(t, "-2", attrs[types.AttributeKeyNodeSuspicionScore])
		require.Equal(t, "2", attrs[types.AttributeKeyReporterReliabilityScore])
		require.Equal(t, "-3", attrs[types.AttributeKeyTicketDeteriorationScore])
	}
	require.True(t, found, "expected storage truth score update event")
}

func TestSubmitEpochReport_NoStorageProofResults_DoesNotCreateStorageTruthStates(t *testing.T) {
	f := initFixture(t)
	f.ctx = f.ctx.WithBlockHeight(1).WithEventManager(sdk.NewEventManager())
	ms := keeper.NewMsgServerImpl(f.keeper)

	reporter := "sn-aaa-reporter"
	target := "sn-bbb-target"

	f.supernodeKeeper.EXPECT().
		GetSuperNodeByAccount(gomock.Any(), reporter).
		Return(sntypes.SuperNode{}, true, nil).
		AnyTimes()

	seedEpochAnchorForReportTest(t, f, 0, []string{reporter, target}, []string{reporter, target})

	_, err := ms.SubmitEpochReport(f.ctx, &types.MsgSubmitEpochReport{
		Creator: reporter,
		EpochId: 0,
		HostReport: types.HostReport{
			InboundPortStates: fullOpenPortStates(),
		},
		StorageChallengeObservations: []*types.StorageChallengeObservation{
			{
				TargetSupernodeAccount: target,
				PortStates:             fullOpenPortStates(),
			},
		},
	})
	require.NoError(t, err)

	_, found := f.keeper.GetNodeSuspicionState(f.ctx, target)
	require.False(t, found)
	_, found = f.keeper.GetReporterReliabilityState(f.ctx, reporter)
	require.False(t, found)
	_, found = f.keeper.GetTicketDeteriorationState(f.ctx, "ticket-1")
	require.False(t, found)
}

func TestSubmitEpochReport_LowTrustReporterScalesNodeAndTicketDeltas(t *testing.T) {
	f := initFixture(t)
	f.ctx = f.ctx.WithBlockHeight(1).WithEventManager(sdk.NewEventManager())
	ms := keeper.NewMsgServerImpl(f.keeper)

	params := f.keeper.GetParams(f.ctx).WithDefaults()
	params.StorageTruthReporterReliabilityLowTrustThreshold = -10
	params.StorageTruthReporterReliabilityIneligibleThreshold = -50
	require.NoError(t, f.keeper.SetParams(f.ctx, params))

	reporter := "sn-aaa-reporter"
	target := "sn-bbb-target"
	f.supernodeKeeper.EXPECT().
		GetSuperNodeByAccount(gomock.Any(), reporter).
		Return(sntypes.SuperNode{}, true, nil).
		AnyTimes()

	require.NoError(t, f.keeper.SetReporterReliabilityState(f.ctx, types.ReporterReliabilityState{
		ReporterSupernodeAccount: reporter,
		ReliabilityScore:         -20,
		LastUpdatedEpoch:         0,
		TrustBand:                types.ReporterTrustBand_REPORTER_TRUST_BAND_LOW_TRUST,
	}))
	seedEpochAnchorForReportTest(t, f, 0, []string{reporter, target}, []string{reporter, target})

	_, err := ms.SubmitEpochReport(f.ctx, &types.MsgSubmitEpochReport{
		Creator: reporter,
		EpochId: 0,
		HostReport: types.HostReport{
			InboundPortStates: fullOpenPortStates(),
		},
		StorageChallengeObservations: []*types.StorageChallengeObservation{{
			TargetSupernodeAccount: target,
			PortStates:             fullOpenPortStates(),
		}},
		StorageProofResults: []*types.StorageProofResult{
			baseStorageProofResult(types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_HASH_MISMATCH),
		},
	})
	require.NoError(t, err)

	nodeState, found := f.keeper.GetNodeSuspicionState(f.ctx, target)
	require.True(t, found)
	require.Equal(t, int64(6), nodeState.SuspicionScore)

	ticketState, found := f.keeper.GetTicketDeteriorationState(f.ctx, "ticket-1")
	require.True(t, found)
	require.Equal(t, int64(6), ticketState.DeteriorationScore)

	reporterState, found := f.keeper.GetReporterReliabilityState(f.ctx, reporter)
	require.True(t, found)
	require.Equal(t, int64(-19), reporterState.ReliabilityScore)
	require.Equal(t, types.ReporterTrustBand_REPORTER_TRUST_BAND_LOW_TRUST, reporterState.TrustBand)
}

func TestSubmitEpochReport_RepeatedDistinctTicketFailuresEscalate(t *testing.T) {
	f := initFixture(t)
	f.ctx = f.ctx.WithBlockHeight(801).WithEventManager(sdk.NewEventManager()) // epoch_id = 2
	ms := keeper.NewMsgServerImpl(f.keeper)

	params := f.keeper.GetParams(f.ctx).WithDefaults()
	params.StorageTruthProbationEpochs = 3
	require.NoError(t, f.keeper.SetParams(f.ctx, params))

	reporter := "sn-aaa-reporter"
	target := "sn-bbb-target"
	f.supernodeKeeper.EXPECT().
		GetSuperNodeByAccount(gomock.Any(), reporter).
		Return(sntypes.SuperNode{}, true, nil).
		AnyTimes()

	require.NoError(t, f.keeper.SetTicketDeteriorationState(f.ctx, types.TicketDeteriorationState{
		TicketId:                     "ticket-1",
		DeteriorationScore:           7,
		LastUpdatedEpoch:             1,
		LastFailureEpoch:             1,
		RecentFailureEpochCount:      1,
		LastTargetSupernodeAccount:   target,
		LastReporterSupernodeAccount: reporter,
		LastResultClass:              types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_HASH_MISMATCH,
		LastResultEpoch:              1,
	}))
	seedEpochAnchorForReportTest(t, f, 2, []string{reporter, target}, []string{reporter, target})

	_, err := ms.SubmitEpochReport(f.ctx, &types.MsgSubmitEpochReport{
		Creator: reporter,
		EpochId: 2,
		HostReport: types.HostReport{
			InboundPortStates: fullOpenPortStates(),
		},
		StorageChallengeObservations: []*types.StorageChallengeObservation{{
			TargetSupernodeAccount: target,
			PortStates:             fullOpenPortStates(),
		}},
		StorageProofResults: []*types.StorageProofResult{
			baseStorageProofResult(types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_HASH_MISMATCH),
		},
	})
	require.NoError(t, err)

	nodeState, found := f.keeper.GetNodeSuspicionState(f.ctx, target)
	require.True(t, found)
	require.Equal(t, int64(14), nodeState.SuspicionScore) // 12 + escalation bonus 2

	ticketState, found := f.keeper.GetTicketDeteriorationState(f.ctx, "ticket-1")
	require.True(t, found)
	require.Equal(t, int64(20), ticketState.DeteriorationScore) // 7 decays to 6, then +12 +2
	require.Equal(t, uint32(2), ticketState.RecentFailureEpochCount)
	require.Equal(t, uint64(2), ticketState.LastFailureEpoch)
}

func TestSubmitEpochReport_EpochZeroFailureWindowCarriesIntoNextEpoch(t *testing.T) {
	f := initFixture(t)
	f.ctx = f.ctx.WithBlockHeight(401).WithEventManager(sdk.NewEventManager()) // epoch_id = 1
	ms := keeper.NewMsgServerImpl(f.keeper)

	params := f.keeper.GetParams(f.ctx).WithDefaults()
	params.StorageTruthProbationEpochs = 3
	require.NoError(t, f.keeper.SetParams(f.ctx, params))

	reporter := "sn-aaa-reporter"
	target := "sn-bbb-target"
	f.supernodeKeeper.EXPECT().
		GetSuperNodeByAccount(gomock.Any(), reporter).
		Return(sntypes.SuperNode{}, true, nil).
		AnyTimes()

	require.NoError(t, f.keeper.SetTicketDeteriorationState(f.ctx, types.TicketDeteriorationState{
		TicketId:                     "ticket-epoch-zero",
		DeteriorationScore:           12,
		LastUpdatedEpoch:             0,
		LastFailureEpoch:             0,
		RecentFailureEpochCount:      1,
		LastTargetSupernodeAccount:   target,
		LastReporterSupernodeAccount: reporter,
		LastResultClass:              types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_HASH_MISMATCH,
		LastResultEpoch:              0,
	}))
	seedEpochAnchorForReportTest(t, f, 1, []string{reporter, target}, []string{reporter, target})

	result := baseStorageProofResult(types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_HASH_MISMATCH)
	result.TicketId = "ticket-epoch-zero"

	_, err := ms.SubmitEpochReport(f.ctx, &types.MsgSubmitEpochReport{
		Creator: reporter,
		EpochId: 1,
		HostReport: types.HostReport{
			InboundPortStates: fullOpenPortStates(),
		},
		StorageChallengeObservations: []*types.StorageChallengeObservation{{
			TargetSupernodeAccount: target,
			PortStates:             fullOpenPortStates(),
		}},
		StorageProofResults: []*types.StorageProofResult{result},
	})
	require.NoError(t, err)

	nodeState, found := f.keeper.GetNodeSuspicionState(f.ctx, target)
	require.True(t, found)
	require.Equal(t, int64(14), nodeState.SuspicionScore) // 12 base + 2 escalation bonus from epoch-0 carryover

	ticketState, found := f.keeper.GetTicketDeteriorationState(f.ctx, "ticket-epoch-zero")
	require.True(t, found)
	require.Equal(t, uint32(2), ticketState.RecentFailureEpochCount)
	require.Equal(t, uint64(1), ticketState.LastFailureEpoch)
	require.Equal(t, int64(25), ticketState.DeteriorationScore) // 11 after decay, then +12 +2
}

func TestSubmitEpochReport_ContradictionsPenalizeBothReportersAndTrackState(t *testing.T) {
	f := initFixture(t)
	f.ctx = f.ctx.WithBlockHeight(401).WithEventManager(sdk.NewEventManager()) // epoch_id = 1
	ms := keeper.NewMsgServerImpl(f.keeper)

	reporter := "sn-aaa-reporter"
	previousReporter := "sn-ccc-previous"
	target := "sn-bbb-target"
	f.supernodeKeeper.EXPECT().
		GetSuperNodeByAccount(gomock.Any(), reporter).
		Return(sntypes.SuperNode{}, true, nil).
		AnyTimes()

	require.NoError(t, f.keeper.SetReporterReliabilityState(f.ctx, types.ReporterReliabilityState{
		ReporterSupernodeAccount: previousReporter,
		ReliabilityScore:         10,
		LastUpdatedEpoch:         0,
		TrustBand:                types.ReporterTrustBand_REPORTER_TRUST_BAND_NORMAL,
	}))
	require.NoError(t, f.keeper.SetTicketDeteriorationState(f.ctx, types.TicketDeteriorationState{
		TicketId:                     "ticket-1",
		DeteriorationScore:           12,
		LastUpdatedEpoch:             0,
		LastTargetSupernodeAccount:   target,
		LastReporterSupernodeAccount: previousReporter,
		LastResultClass:              types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_HASH_MISMATCH,
		LastResultEpoch:              0,
		LastFailureEpoch:             0,
		RecentFailureEpochCount:      1,
	}))
	seedEpochAnchorForReportTest(t, f, 1, []string{reporter, target}, []string{reporter, target})

	_, err := ms.SubmitEpochReport(f.ctx, &types.MsgSubmitEpochReport{
		Creator: reporter,
		EpochId: 1,
		HostReport: types.HostReport{
			InboundPortStates: fullOpenPortStates(),
		},
		StorageChallengeObservations: []*types.StorageChallengeObservation{{
			TargetSupernodeAccount: target,
			PortStates:             fullOpenPortStates(),
		}},
		StorageProofResults: []*types.StorageProofResult{
			baseStorageProofResult(types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_PASS),
		},
	})
	require.NoError(t, err)

	currentReporterState, found := f.keeper.GetReporterReliabilityState(f.ctx, reporter)
	require.True(t, found)
	require.Equal(t, int64(-4), currentReporterState.ReliabilityScore) // +2 pass delta, -6 contradiction penalty
	require.Equal(t, uint64(1), currentReporterState.ContradictionCount)
	require.Equal(t, types.ReporterTrustBand_REPORTER_TRUST_BAND_NORMAL, currentReporterState.TrustBand)

	previousReporterState, found := f.keeper.GetReporterReliabilityState(f.ctx, previousReporter)
	require.True(t, found)
	require.Equal(t, int64(3), previousReporterState.ReliabilityScore) // 10 decays to 9, then -6
	require.Equal(t, uint64(1), previousReporterState.ContradictionCount)

	ticketState, found := f.keeper.GetTicketDeteriorationState(f.ctx, "ticket-1")
	require.True(t, found)
	require.Equal(t, uint64(1), ticketState.ContradictionCount)
	require.Equal(t, reporter, ticketState.LastReporterSupernodeAccount)
	require.Equal(t, types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_PASS, ticketState.LastResultClass)
	require.Equal(t, uint64(1), ticketState.LastResultEpoch)
}

func int64Ptr(v int64) *int64 {
	return &v
}
