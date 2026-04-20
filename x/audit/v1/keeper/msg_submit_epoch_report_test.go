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

func seedEpochAnchorForReportTest(t *testing.T, f *fixture, epochID uint64, active []string, targets []string) {
	t.Helper()

	err := f.keeper.SetEpochAnchor(f.ctx, types.EpochAnchor{
		EpochId:                 epochID,
		EpochStartHeight:        1,
		EpochEndHeight:          400,
		EpochLengthBlocks:       types.DefaultEpochLengthBlocks,
		Seed:                    make([]byte, 32),
		ActiveSupernodeAccounts: active,
		TargetSupernodeAccounts: targets,
		ParamsCommitment:        []byte{1},
		ActiveSetCommitment:     []byte{1},
		TargetsSetCommitment:    []byte{1},
	})
	require.NoError(t, err)
}

func TestSubmitEpochReport_ValidatesInboundPortStatesLength(t *testing.T) {
	f := initFixture(t)
	f.ctx = f.ctx.WithBlockHeight(1)

	ms := keeper.NewMsgServerImpl(f.keeper)

	reporter := sdk.AccAddress([]byte("reporter_address_20b")).String()
	active := sdk.AccAddress([]byte("active_address__20b")).String()

	// Reporter exists on-chain as a supernode, but is not necessarily ACTIVE at epoch start.
	f.supernodeKeeper.EXPECT().
		GetSuperNodeByAccount(gomock.Any(), reporter).
		Return(sntypes.SuperNode{}, true, nil).
		AnyTimes()

	// Seeded epoch anchor for epoch 0 (content not important for this test beyond existence).
	seedEpochAnchorForReportTest(t, f, 0, []string{active}, []string{active})

	requiredPortsLen := len(types.DefaultRequiredOpenPorts)
	require.Greater(t, requiredPortsLen, 0)

	// Empty inbound_port_states is allowed (unknown/unreported).
	_, err := ms.SubmitEpochReport(f.ctx, &types.MsgSubmitEpochReport{
		Creator:                      reporter,
		EpochId:                      0,
		HostReport:                   types.HostReport{},
		StorageChallengeObservations: nil,
	})
	require.NoError(t, err)

	// Partial inbound_port_states is rejected.
	_, err = ms.SubmitEpochReport(f.ctx, &types.MsgSubmitEpochReport{
		Creator: reporter,
		EpochId: 0,
		HostReport: types.HostReport{
			InboundPortStates: []types.PortState{types.PortState_PORT_STATE_OPEN},
		},
		StorageChallengeObservations: nil,
	})
	require.Error(t, err)

	// Oversized inbound_port_states is rejected.
	oversized := make([]types.PortState, requiredPortsLen+1)
	_, err = ms.SubmitEpochReport(f.ctx, &types.MsgSubmitEpochReport{
		Creator: reporter,
		EpochId: 0,
		HostReport: types.HostReport{
			InboundPortStates: oversized,
		},
		StorageChallengeObservations: nil,
	})
	require.Error(t, err)
}

func TestSubmitEpochReport_PersistsStorageProofResults(t *testing.T) {
	f := initFixture(t)
	f.ctx = f.ctx.WithBlockHeight(1)

	ms := keeper.NewMsgServerImpl(f.keeper)

	reporter := "sn-aaa-reporter"
	target := "sn-bbb-target"

	f.supernodeKeeper.EXPECT().
		GetSuperNodeByAccount(gomock.Any(), reporter).
		Return(sntypes.SuperNode{}, true, nil).
		AnyTimes()

	seedEpochAnchorForReportTest(t, f, 0, []string{reporter, target}, []string{reporter, target})

	portStates := make([]types.PortState, len(types.DefaultRequiredOpenPorts))
	for i := range portStates {
		portStates[i] = types.PortState_PORT_STATE_OPEN
	}

	result := &types.StorageProofResult{
		TargetSupernodeAccount:     target,
		ChallengerSupernodeAccount: reporter,
		TicketId:                   "ticket-1",
		BucketType:                 types.StorageProofBucketType_STORAGE_PROOF_BUCKET_TYPE_RECENT,
		ArtifactClass:              types.StorageProofArtifactClass_STORAGE_PROOF_ARTIFACT_CLASS_INDEX,
		ArtifactOrdinal:            0,
		ArtifactKey:                "artifact-key-1",
		ResultClass:                types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_PASS,
		TranscriptHash:             "transcript-hash-1",
	}

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
		StorageProofResults: []*types.StorageProofResult{result},
	})
	require.NoError(t, err)

	report, found := f.keeper.GetReport(f.ctx, 0, reporter)
	require.True(t, found)
	require.Len(t, report.StorageProofResults, 1)
	require.NotNil(t, report.StorageProofResults[0])
	require.Equal(t, *result, *report.StorageProofResults[0])
}

func TestSubmitEpochReport_RejectsStorageProofResultsForNonProber(t *testing.T) {
	f := initFixture(t)
	f.ctx = f.ctx.WithBlockHeight(1)

	ms := keeper.NewMsgServerImpl(f.keeper)

	reporter := "sn-zzz-reporter"
	active := "sn-aaa-active"
	target := "sn-bbb-target"

	f.supernodeKeeper.EXPECT().
		GetSuperNodeByAccount(gomock.Any(), reporter).
		Return(sntypes.SuperNode{}, true, nil).
		AnyTimes()

	seedEpochAnchorForReportTest(t, f, 0, []string{active}, []string{active, target})

	_, err := ms.SubmitEpochReport(f.ctx, &types.MsgSubmitEpochReport{
		Creator:    reporter,
		EpochId:    0,
		HostReport: types.HostReport{},
		StorageProofResults: []*types.StorageProofResult{
			{
				TargetSupernodeAccount:     target,
				ChallengerSupernodeAccount: reporter,
				BucketType:                 types.StorageProofBucketType_STORAGE_PROOF_BUCKET_TYPE_RECENT,
				ResultClass:                types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_NO_ELIGIBLE_TICKET,
				TranscriptHash:             "transcript-hash-1",
			},
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), types.ErrInvalidReporterState.Error())
}

func TestSubmitEpochReport_RejectsMalformedStorageProofResults(t *testing.T) {
	baseResult := func() *types.StorageProofResult {
		return &types.StorageProofResult{
			TargetSupernodeAccount:     "sn-bbb-target",
			ChallengerSupernodeAccount: "sn-aaa-reporter",
			TicketId:                   "ticket-1",
			BucketType:                 types.StorageProofBucketType_STORAGE_PROOF_BUCKET_TYPE_RECENT,
			ArtifactClass:              types.StorageProofArtifactClass_STORAGE_PROOF_ARTIFACT_CLASS_INDEX,
			ArtifactOrdinal:            1,
			ArtifactKey:                "artifact-key-1",
			ResultClass:                types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_PASS,
			TranscriptHash:             "transcript-hash-1",
		}
	}

	testCases := []struct {
		name          string
		buildResults  func() []*types.StorageProofResult
		wantSubstring string
	}{
		{
			name: "challenger mismatch",
			buildResults: func() []*types.StorageProofResult {
				result := baseResult()
				result.ChallengerSupernodeAccount = "sn-ccc-other"
				return []*types.StorageProofResult{result}
			},
			wantSubstring: "challenger_supernode_account must match report creator",
		},
		{
			name: "missing ticket for non no eligible result",
			buildResults: func() []*types.StorageProofResult {
				result := baseResult()
				result.TicketId = ""
				return []*types.StorageProofResult{result}
			},
			wantSubstring: "ticket_id is required",
		},
		{
			name: "recheck confirmed fail requires recheck bucket",
			buildResults: func() []*types.StorageProofResult {
				result := baseResult()
				result.ResultClass = types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_RECHECK_CONFIRMED_FAIL
				return []*types.StorageProofResult{result}
			},
			wantSubstring: "RECHECK_CONFIRMED_FAIL requires RECHECK bucket",
		},
		{
			name: "duplicate descriptors",
			buildResults: func() []*types.StorageProofResult {
				resultA := baseResult()
				resultB := baseResult()
				resultB.ResultClass = types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_HASH_MISMATCH
				return []*types.StorageProofResult{resultA, resultB}
			},
			wantSubstring: "duplicates another storage proof result descriptor",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			f := initFixture(t)
			f.ctx = f.ctx.WithBlockHeight(1)

			ms := keeper.NewMsgServerImpl(f.keeper)

			reporter := "sn-aaa-reporter"
			target := "sn-bbb-target"

			f.supernodeKeeper.EXPECT().
				GetSuperNodeByAccount(gomock.Any(), reporter).
				Return(sntypes.SuperNode{}, true, nil).
				AnyTimes()

			seedEpochAnchorForReportTest(t, f, 0, []string{reporter, target}, []string{reporter, target})

			portStates := make([]types.PortState, len(types.DefaultRequiredOpenPorts))
			for i := range portStates {
				portStates[i] = types.PortState_PORT_STATE_OPEN
			}

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
				StorageProofResults: tc.buildResults(),
			})
			require.Error(t, err)
			require.Contains(t, err.Error(), types.ErrInvalidStorageProofs.Error())
			require.Contains(t, err.Error(), tc.wantSubstring)
		})
	}
}

func TestSubmitEpochReport_FullModeRequiresRecentAndOldStorageProofs(t *testing.T) {
	f := initFixture(t)
	f.ctx = f.ctx.WithBlockHeight(1)

	params := types.DefaultParams().WithDefaults()
	params.StorageTruthEnforcementMode = types.StorageTruthEnforcementMode_STORAGE_TRUTH_ENFORCEMENT_MODE_FULL
	require.NoError(t, f.keeper.SetParams(f.ctx, params))

	ms := keeper.NewMsgServerImpl(f.keeper)

	reporter := "sn-aaa-reporter"
	target := "sn-bbb-target"

	f.supernodeKeeper.EXPECT().
		GetSuperNodeByAccount(gomock.Any(), reporter).
		Return(sntypes.SuperNode{}, true, nil).
		AnyTimes()

	seedEpochAnchorForReportTest(t, f, 0, []string{reporter, target}, []string{reporter, target})

	portStates := make([]types.PortState, len(types.DefaultRequiredOpenPorts))
	for i := range portStates {
		portStates[i] = types.PortState_PORT_STATE_OPEN
	}

	recent := &types.StorageProofResult{
		TargetSupernodeAccount:     target,
		ChallengerSupernodeAccount: reporter,
		TicketId:                   "ticket-recent",
		BucketType:                 types.StorageProofBucketType_STORAGE_PROOF_BUCKET_TYPE_RECENT,
		ArtifactClass:              types.StorageProofArtifactClass_STORAGE_PROOF_ARTIFACT_CLASS_INDEX,
		ArtifactOrdinal:            1,
		ArtifactKey:                "artifact-recent",
		ResultClass:                types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_PASS,
		TranscriptHash:             "transcript-recent",
	}

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
		StorageProofResults: []*types.StorageProofResult{recent},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "must have exactly one RECENT and one OLD")

	old := &types.StorageProofResult{
		TargetSupernodeAccount:     target,
		ChallengerSupernodeAccount: reporter,
		TicketId:                   "ticket-old",
		BucketType:                 types.StorageProofBucketType_STORAGE_PROOF_BUCKET_TYPE_OLD,
		ArtifactClass:              types.StorageProofArtifactClass_STORAGE_PROOF_ARTIFACT_CLASS_SYMBOL,
		ArtifactOrdinal:            2,
		ArtifactKey:                "artifact-old",
		ResultClass:                types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_PASS,
		TranscriptHash:             "transcript-old",
	}

	_, err = ms.SubmitEpochReport(f.ctx, &types.MsgSubmitEpochReport{
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
		StorageProofResults: []*types.StorageProofResult{recent, old},
	})
	require.NoError(t, err)
}
