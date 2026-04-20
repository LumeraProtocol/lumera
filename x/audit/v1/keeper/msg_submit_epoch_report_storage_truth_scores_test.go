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
			// PASS + RECENT: node=-3, reporter=-4 (clamped to 0 from 0), ticket=-2 (clamped to 0)
			name:                "pass recent",
			class:               types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_PASS,
			bucket:              types.StorageProofBucketType_STORAGE_PROOF_BUCKET_TYPE_RECENT,
			expectedNodeScore:   int64Ptr(0), // clamped at 0
			expectedReporter:    0,           // clamped at 0 (positive-penalty model)
			expectedTicketScore: int64Ptr(0), // clamped at 0
			expectedTicketID:    "ticket-1",
		},
		{
			// HASH_MISMATCH + INDEX: node=+26, reporter=+1, ticket=+12
			name:                "hash mismatch index artifact",
			class:               types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_HASH_MISMATCH,
			bucket:              types.StorageProofBucketType_STORAGE_PROOF_BUCKET_TYPE_RECENT,
			expectedNodeScore:   int64Ptr(26),
			expectedReporter:    1,
			expectedTicketScore: int64Ptr(12),
			expectedTicketID:    "ticket-1",
		},
		{
			// TIMEOUT: node=+7, reporter=-1 clamped to 0, ticket=+3
			name:                "timeout",
			class:               types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_TIMEOUT_OR_NO_RESPONSE,
			bucket:              types.StorageProofBucketType_STORAGE_PROOF_BUCKET_TYPE_RECENT,
			expectedNodeScore:   int64Ptr(7),
			expectedReporter:    0, // clamped at 0 (was -1 before penalty model flip)
			expectedTicketScore: int64Ptr(3),
			expectedTicketID:    "ticket-1",
		},
		{
			// OBSERVER_QUORUM_FAIL: node=+4, reporter=-3 clamped to 0, ticket=+5
			name:                "observer quorum fail",
			class:               types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_OBSERVER_QUORUM_FAIL,
			bucket:              types.StorageProofBucketType_STORAGE_PROOF_BUCKET_TYPE_RECENT,
			expectedNodeScore:   int64Ptr(4),
			expectedReporter:    0, // clamped at 0
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
			// INVALID_TRANSCRIPT: reporter=-8 clamped to 0
			name:                "invalid transcript",
			class:               types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_INVALID_TRANSCRIPT,
			bucket:              types.StorageProofBucketType_STORAGE_PROOF_BUCKET_TYPE_RECENT,
			expectedNodeScore:   nil,
			expectedReporter:    0, // clamped at 0
			expectedTicketScore: nil,
		},
		{
			// RECHECK_CONFIRMED_FAIL: node=+15, reporter=+3, ticket=+8
			name:                "recheck confirmed fail",
			class:               types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_RECHECK_CONFIRMED_FAIL,
			bucket:              types.StorageProofBucketType_STORAGE_PROOF_BUCKET_TYPE_RECHECK,
			expectedNodeScore:   int64Ptr(15),
			expectedReporter:    3,
			expectedTicketScore: int64Ptr(8),
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
	// Use proper exponential decay factors (range 1..1000).
	// 920 means 0.920/epoch (LEP6.md §14 node decay).
	// 900 means 0.900/epoch (LEP6.md §15/16 reporter/ticket decay).
	params.StorageTruthNodeSuspicionDecayPerEpoch = 920
	params.StorageTruthReporterReliabilityDecayPerEpoch = 900
	params.StorageTruthTicketDeteriorationDecayPerEpoch = 900
	require.NoError(t, f.keeper.SetParams(f.ctx, params))

	reporter := "sn-aaa-reporter"
	target := "sn-bbb-target"
	ticketID := "ticket-1"

	// Node: suspicion=50, epoch=0, now epoch=3, decay=920 (0.92/epoch)
	// decayTowardZero(50, 920, 3): 50→46→42→38. Then +26 (HASH_MISMATCH INDEX) = 64.
	require.NoError(t, f.keeper.SetNodeSuspicionState(f.ctx, types.NodeSuspicionState{
		SupernodeAccount: target,
		SuspicionScore:   50,
		LastUpdatedEpoch: 0,
	}))
	// Reporter: score=10 (some existing penalty), decays over 3 epochs with factor=900.
	// decayTowardZero(10, 900, 3): 10→9→8→7. Then +1 (HASH_MISMATCH reporter delta) = 8.
	require.NoError(t, f.keeper.SetReporterReliabilityState(f.ctx, types.ReporterReliabilityState{
		ReporterSupernodeAccount: reporter,
		ReliabilityScore:         10,
		LastUpdatedEpoch:         0,
	}))
	// Ticket: score=20, epoch=0, now epoch=3, decay=900.
	// decayTowardZero(20, 900, 3): 20→18→16→14. Then +12 (HASH_MISMATCH INDEX ticket delta) = 26.
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
	// decayTowardZero(50, 920, 3): 50→46→42→38.
	// Reporter score decays to 7, so trust multiplier is 93%; floor(26*93/100)=24 → 62.
	require.Equal(t, int64(62), nodeState.SuspicionScore)
	require.Equal(t, uint64(3), nodeState.LastUpdatedEpoch)

	reporterState, found := f.keeper.GetReporterReliabilityState(f.ctx, reporter)
	require.True(t, found)
	// decayTowardZero(10, 900, 3): 10→9→8→7. +1 (HASH_MISMATCH reporter) = 8.
	require.Equal(t, int64(8), reporterState.ReliabilityScore)
	require.Equal(t, uint64(3), reporterState.LastUpdatedEpoch)
	require.Equal(t, types.ReporterTrustBand_REPORTER_TRUST_BAND_NORMAL, reporterState.TrustBand)

	ticketState, found := f.keeper.GetTicketDeteriorationState(f.ctx, ticketID)
	require.True(t, found)
	// Reporter trust multiplier also scales ticket delta: floor(12*93/100)=11, so 14+11=25.
	require.Equal(t, int64(25), ticketState.DeteriorationScore)
	require.Equal(t, uint64(3), ticketState.LastUpdatedEpoch)
	// Existing lifecycle metadata remains intact.
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
		// PASS RECENT: node=-3 clamped to 0, reporter=-4 clamped to 0, ticket=-2 clamped to 0
		require.Equal(t, "0", attrs[types.AttributeKeyNodeSuspicionScore])
		require.Equal(t, "0", attrs[types.AttributeKeyReporterReliabilityScore])
		require.Equal(t, "0", attrs[types.AttributeKeyTicketDeteriorationScore])
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
	// Positive-penalty model: low_trust=10, degraded=30, ineligible=50
	params.StorageTruthReporterReliabilityLowTrustThreshold = 10
	params.StorageTruthReporterReliabilityDegradedThreshold = 30
	params.StorageTruthReporterReliabilityIneligibleThreshold = 50
	require.NoError(t, f.keeper.SetParams(f.ctx, params))

	reporter := "sn-aaa-reporter"
	target := "sn-bbb-target"
	f.supernodeKeeper.EXPECT().
		GetSuperNodeByAccount(gomock.Any(), reporter).
		Return(sntypes.SuperNode{}, true, nil).
		AnyTimes()

	// Set reporter score to +20 = LOW_TRUST in positive-penalty model.
	require.NoError(t, f.keeper.SetReporterReliabilityState(f.ctx, types.ReporterReliabilityState{
		ReporterSupernodeAccount: reporter,
		ReliabilityScore:         20,
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

	// Reporter score 20 gives continuous trust multiplier max(50, 100-20)=80.
	// HASH_MISMATCH INDEX delta +26 node, +12 ticket: floor(26*80/100)=20, floor(12*80/100)=9.
	nodeState, found := f.keeper.GetNodeSuspicionState(f.ctx, target)
	require.True(t, found)
	require.Equal(t, int64(20), nodeState.SuspicionScore)

	ticketState, found := f.keeper.GetTicketDeteriorationState(f.ctx, "ticket-1")
	require.True(t, found)
	require.Equal(t, int64(9), ticketState.DeteriorationScore)

	reporterState, found := f.keeper.GetReporterReliabilityState(f.ctx, reporter)
	require.True(t, found)
	// Reporter: score=20, no decay (same epoch), HASH_MISMATCH reporter delta +1 = 21.
	require.Equal(t, int64(21), reporterState.ReliabilityScore)
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
	// Same-ticket repeat is no longer counted as a distinct-ticket node escalation.
	require.Equal(t, int64(26), nodeState.SuspicionScore)

	ticketState, found := f.keeper.GetTicketDeteriorationState(f.ctx, "ticket-1")
	require.True(t, found)
	// Ticket score=7, decays 1 epoch with default decay=920: floor(7*0.920)=6.
	// Then +12 (INDEX HASH_MISMATCH delta) + 6 (same-holder repeat §16) = 24.
	require.Equal(t, int64(24), ticketState.DeteriorationScore)
	require.Equal(t, uint32(2), ticketState.RecentFailureEpochCount)
	require.Equal(t, uint64(2), ticketState.LastFailureEpoch)
}

func TestSubmitEpochReport_DistinctTicketFailuresEscalateNodeSuspicion(t *testing.T) {
	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	reporter := "sn-aaa-reporter"
	target := "sn-bbb-target"
	f.supernodeKeeper.EXPECT().
		GetSuperNodeByAccount(gomock.Any(), reporter).
		Return(sntypes.SuperNode{}, true, nil).
		AnyTimes()

	f.ctx = f.ctx.WithBlockHeight(401).WithEventManager(sdk.NewEventManager()) // epoch_id = 1
	seedEpochAnchorForReportTest(t, f, 1, []string{reporter, target}, []string{reporter, target})
	first := baseStorageProofResult(types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_HASH_MISMATCH)
	first.TicketId = "ticket-distinct-1"
	first.TranscriptHash = "tx-distinct-1"
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
		StorageProofResults: []*types.StorageProofResult{first},
	})
	require.NoError(t, err)

	f.ctx = f.ctx.WithBlockHeight(801).WithEventManager(sdk.NewEventManager()) // epoch_id = 2
	seedEpochAnchorForReportTest(t, f, 2, []string{reporter, target}, []string{reporter, target})
	second := baseStorageProofResult(types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_HASH_MISMATCH)
	second.TicketId = "ticket-distinct-2"
	second.TranscriptHash = "tx-distinct-2"
	_, err = ms.SubmitEpochReport(f.ctx, &types.MsgSubmitEpochReport{
		Creator: reporter,
		EpochId: 2,
		HostReport: types.HostReport{
			InboundPortStates: fullOpenPortStates(),
		},
		StorageChallengeObservations: []*types.StorageChallengeObservation{{
			TargetSupernodeAccount: target,
			PortStates:             fullOpenPortStates(),
		}},
		StorageProofResults: []*types.StorageProofResult{second},
	})
	require.NoError(t, err)

	nodeState, found := f.keeper.GetNodeSuspicionState(f.ctx, target)
	require.True(t, found)
	// First fail score 26 decays one epoch at 0.92 to 23; second distinct ticket adds 26+10.
	require.Equal(t, int64(59), nodeState.SuspicionScore)
	require.Equal(t, uint32(2), nodeState.DistinctTicketFailWindow)
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
	// Same-ticket repeat is no longer counted as a distinct-ticket node escalation.
	require.Equal(t, int64(26), nodeState.SuspicionScore)

	ticketState, found := f.keeper.GetTicketDeteriorationState(f.ctx, "ticket-epoch-zero")
	require.True(t, found)
	require.Equal(t, uint32(2), ticketState.RecentFailureEpochCount)
	require.Equal(t, uint64(1), ticketState.LastFailureEpoch)
	// Ticket: 12 decays 1 epoch at 900: floor(12*0.900)=10. +12 (INDEX HASH) + 6 (same-holder repeat §16) = 28.
	require.Equal(t, int64(28), ticketState.DeteriorationScore)
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
	// Current reporter gets PASS (-4 recovery) + contradiction penalty (-4) = -8.
	// Starting from 0 and clamped to 0: max(0, 0 + (-4) + (-4)) = 0.
	require.Equal(t, int64(0), currentReporterState.ReliabilityScore)
	require.Equal(t, uint64(1), currentReporterState.ContradictionCount)
	require.Equal(t, types.ReporterTrustBand_REPORTER_TRUST_BAND_NORMAL, currentReporterState.TrustBand)

	previousReporterState, found := f.keeper.GetReporterReliabilityState(f.ctx, previousReporter)
	require.True(t, found)
	// Previous reporter: score=10, decays 1 epoch at 920: floor(10*0.920)=9.
	// Then +12 (contradiction penalty from LEP6.md §15.1) = 21.
	require.Equal(t, int64(21), previousReporterState.ReliabilityScore)
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
