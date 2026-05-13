package keeper_test

import (
	"testing"

	keeper "github.com/LumeraProtocol/lumera/x/audit/v1/keeper"
	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestGenesisParamsRoundTrip(t *testing.T) {
	f := initFixture(t)

	genesisState := types.GenesisState{
		Params: types.DefaultParams(),
	}

	err := f.keeper.InitGenesis(f.ctx, genesisState)
	require.NoError(t, err)

	got, err := f.keeper.ExportGenesis(f.ctx)
	require.NoError(t, err)
	require.NotNil(t, got)

	require.EqualExportedValues(t, genesisState.Params, got.Params)
	require.Equal(t, uint64(1), got.NextEvidenceId)
	require.Equal(t, uint64(1), got.NextHealOpId)
	require.Empty(t, got.Evidence)
	require.Empty(t, got.NodeSuspicionStates)
	require.Empty(t, got.ReporterReliabilityStates)
	require.Empty(t, got.TicketDeteriorationStates)
	require.Empty(t, got.TicketArtifactCountStates)
	require.Empty(t, got.HealOps)
}

func TestGenesisEvidenceRoundTripSetsNextID(t *testing.T) {
	f := initFixture(t)

	ev := types.Evidence{
		EvidenceId:      7,
		SubjectAddress:  "lumera1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqg7l7x8",
		ReporterAddress: "lumera1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqg7l7x8",
		ActionId:        "action-1",
		EvidenceType:    types.EvidenceType_EVIDENCE_TYPE_ACTION_EXPIRED,
		Metadata:        []byte{1, 2, 3},
		ReportedHeight:  10,
	}

	genesisState := types.GenesisState{
		Params:   types.DefaultParams(),
		Evidence: []types.Evidence{ev},
	}

	err := f.keeper.InitGenesis(f.ctx, genesisState)
	require.NoError(t, err)

	got, err := f.keeper.ExportGenesis(f.ctx)
	require.NoError(t, err)

	require.Len(t, got.Evidence, 1)
	require.Equal(t, ev.EvidenceId, got.Evidence[0].EvidenceId)
	require.Equal(t, uint64(8), got.NextEvidenceId)
	require.Equal(t, uint64(1), got.NextHealOpId)
}

func TestGenesis_RoundTripsAllStPrefixes(t *testing.T) {
	f := initFixture(t)

	// Seed every NEW-C-1 prefix family with at least one record.
	f.keeper.SetRecheckEvidence(f.ctx, 5, "ticket-rce", "lumera1aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa5xm4ep")

	// Storage proof transcript via public IndexStorageProofTranscripts.
	failureResult := &types.StorageProofResult{
		TranscriptHash:         "h-abcd",
		TargetSupernodeAccount: "lumera1cccccccccccccccccccccccccccccccccc7gqs5y",
		TicketId:               "ticket-spt",
		ResultClass:            types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_HASH_MISMATCH,
		BucketType:             types.StorageProofBucketType_STORAGE_PROOF_BUCKET_TYPE_RECENT,
		ArtifactClass:          types.StorageProofArtifactClass_STORAGE_PROOF_ARTIFACT_CLASS_SYMBOL,
	}
	require.NoError(t, f.keeper.IndexStorageProofTranscripts(f.ctx, 5, "lumera1bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbadc7mh", []*types.StorageProofResult{failureResult}))
	require.NoError(t, keeper.SetStorageTruthNodeFailureForTest(f.keeper, f.ctx, 5, "lumera1bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbadc7mh", failureResult))
	require.NoError(t, keeper.SetStorageTruthReporterResultForTest(f.keeper, f.ctx, 5, "lumera1bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbadc7mh", failureResult))

	// Failed-heal marker.
	require.NoError(t, f.keeper.SetParams(f.ctx, types.DefaultParams()))
	// Use ProcessStorageTruthHealOpsAtEpochEnd to seed a failed-heal marker via expire? Use direct keeper helper exposed via genesis import.
	// Simpler: import a marker through InitGenesis test seam — write via the genesis import directly using a constructed GenesisState below.

	// Report indices via public setters.
	f.keeper.SetReportIndex(f.ctx, 7, "lumera1ddddddddddddddddddddddddddddddddddx2nrmt")
	f.keeper.SetHostReportIndex(f.ctx, 7, "lumera1ddddddddddddddddddddddddddddddddddx2nrmt")
	f.keeper.SetStorageChallengeReportIndex(f.ctx, "lumera1eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeennf6kk", 7, "lumera1ddddddddddddddddddddddddddddddddddx2nrmt")

	// EpochReport raw record.
	require.NoError(t, f.keeper.SetReportRaw(f.ctx, types.EpochReport{
		EpochId:          9,
		SupernodeAccount: "lumera1ddddddddddddddddddddddddddddddddddx2nrmt",
	}))

	exported, err := f.keeper.ExportGenesis(f.ctx)
	require.NoError(t, err)
	require.NotNil(t, exported)

	// Sanity-check exports are non-empty for the seeded families.
	require.NotEmpty(t, exported.RecheckEvidence)
	require.NotEmpty(t, exported.StorageProofTranscripts)
	require.NotEmpty(t, exported.ReportIndices)
	require.NotEmpty(t, exported.HostReportIndices)
	require.NotEmpty(t, exported.StorageChallengeIndices)
	require.NotEmpty(t, exported.EpochReports)

	// Add a FailedHealMarker into exported state to verify InitGenesis re-emits it.
	exported.FailedHealMarkers = append(exported.FailedHealMarkers, types.GenesisFailedHealMarker{
		SupernodeAccount: "lumera1cccccccccccccccccccccccccccccccccc7gqs5y",
		EpochId:          5,
		TicketId:         "ticket-fh",
	})

	// Round-trip into a fresh fixture.
	f2 := initFixture(t)
	require.NoError(t, f2.keeper.InitGenesis(f2.ctx, *exported))

	got, err := f2.keeper.ExportGenesis(f2.ctx)
	require.NoError(t, err)

	require.ElementsMatch(t, exported.RecheckEvidence, got.RecheckEvidence)
	require.ElementsMatch(t, exported.StorageProofTranscripts, got.StorageProofTranscripts)
	require.ElementsMatch(t, exported.NodeFailureFacts, got.NodeFailureFacts)
	require.ElementsMatch(t, exported.ReporterResultFacts, got.ReporterResultFacts)
	require.ElementsMatch(t, exported.FailedHealMarkers, got.FailedHealMarkers)
	require.ElementsMatch(t, exported.ReportIndices, got.ReportIndices)
	require.ElementsMatch(t, exported.HostReportIndices, got.HostReportIndices)
	require.ElementsMatch(t, exported.StorageChallengeIndices, got.StorageChallengeIndices)
	require.Equal(t, len(exported.EpochReports), len(got.EpochReports))
}

func TestGenesisStorageTruthPostponementRoundTrip(t *testing.T) {
	f := initFixture(t)

	snA := sntypes.SuperNode{
		SupernodeAccount: "lumera1aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa5xm4ep",
		States:           []*sntypes.SuperNodeStateRecord{{State: sntypes.SuperNodeStatePostponed, Height: 5}},
	}
	snB := sntypes.SuperNode{
		SupernodeAccount: "lumera1bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbadc7mh",
		States:           []*sntypes.SuperNodeStateRecord{{State: sntypes.SuperNodeStatePostponed, Height: 7}},
	}
	// Per NEW-B-6/B-9 — InitGenesis cross-validates audit postponements against
	// supernode state. Match expectations to whichever account the keeper looks up first.
	f.supernodeKeeper.EXPECT().GetSuperNodeByAccount(gomock.Any(), snA.SupernodeAccount).Return(snA, true, nil).Times(1)
	f.supernodeKeeper.EXPECT().GetSuperNodeByAccount(gomock.Any(), snB.SupernodeAccount).Return(snB, true, nil).Times(1)

	genesisState := types.GenesisState{
		Params: types.DefaultParams(),
		StorageTruthPostponements: []types.StorageTruthPostponement{
			{SupernodeAccount: snA.SupernodeAccount, PostponedAtEpochId: 5},
			{SupernodeAccount: snB.SupernodeAccount, PostponedAtEpochId: 7, StrongPostpone: true},
		},
	}

	require.NoError(t, f.keeper.InitGenesis(f.ctx, genesisState))

	got, err := f.keeper.ExportGenesis(f.ctx)
	require.NoError(t, err)
	require.Len(t, got.StorageTruthPostponements, 2)

	// Validate round-trip: all entries are recovered (order may vary).
	byAccount := make(map[string]types.StorageTruthPostponement, len(got.StorageTruthPostponements))
	for _, p := range got.StorageTruthPostponements {
		byAccount[p.SupernodeAccount] = p
	}
	require.Equal(t, uint64(5), byAccount["lumera1aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa5xm4ep"].PostponedAtEpochId)
	require.False(t, byAccount["lumera1aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa5xm4ep"].StrongPostpone)
	require.Equal(t, uint64(7), byAccount["lumera1bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbadc7mh"].PostponedAtEpochId)
	require.True(t, byAccount["lumera1bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbadc7mh"].StrongPostpone)
}

func TestGenesisStorageProofTranscriptRawImportCompatibility(t *testing.T) {
	const (
		target = "lumera1cccccccccccccccccccccccccccccccccc7gqs5y"
		ticket = "ticket-fb10"
	)

	t.Run("preserves unknown fields while rebuilding secondary index", func(t *testing.T) {
		f := initFixture(t)
		recordJSON := []byte(`{
			"epoch_id": 11,
			"reporter_account": "lumera1bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbadc7mh",
			"target_account": "lumera1cccccccccccccccccccccccccccccccccc7gqs5y",
			"ticket_id": "ticket-fb10",
			"result_class": 1,
			"bucket_type": 4,
			"artifact_class": 1,
			"recheck_eligible": false,
			"unexpected_future_field": {"preserve": true}
		}`)

		genesisState := types.GenesisState{
			Params: types.DefaultParams(),
			StorageProofTranscripts: []types.GenesisStorageProofTranscript{
				{TranscriptHash: "h-fb10-unknown", RecordJson: recordJSON},
			},
		}

		require.NoError(t, f.keeper.InitGenesis(f.ctx, genesisState))

		exported, err := f.keeper.ExportGenesis(f.ctx)
		require.NoError(t, err)
		require.Len(t, exported.StorageProofTranscripts, 1)
		require.Equal(t, "h-fb10-unknown", exported.StorageProofTranscripts[0].TranscriptHash)
		require.Equal(t, recordJSON, exported.StorageProofTranscripts[0].RecordJson)

		found, err := keeper.HasCleanRecheckInWindowForTest(f.keeper, f.ctx, ticket, target, 10, 11)
		require.NoError(t, err)
		require.True(t, found, "InitGenesis must rebuild st/spt-tbe/ secondary index from decoded index fields")
	})

	t.Run("invalid json rejected", func(t *testing.T) {
		f := initFixture(t)
		genesisState := types.GenesisState{
			Params: types.DefaultParams(),
			StorageProofTranscripts: []types.GenesisStorageProofTranscript{
				{TranscriptHash: "h-fb10-invalid", RecordJson: []byte(`{"epoch_id":`)},
			},
		}

		require.Error(t, f.keeper.InitGenesis(f.ctx, genesisState))
	})

	t.Run("trailing json rejected", func(t *testing.T) {
		f := initFixture(t)
		genesisState := types.GenesisState{
			Params: types.DefaultParams(),
			StorageProofTranscripts: []types.GenesisStorageProofTranscript{
				{TranscriptHash: "h-fb10-trailing", RecordJson: []byte(`{"epoch_id":11,"target_account":"` + target + `","ticket_id":"` + ticket + `","result_class":1,"bucket_type":4} {}`)},
			},
		}

		err := f.keeper.InitGenesis(f.ctx, genesisState)
		require.Error(t, err)
		require.Contains(t, err.Error(), "trailing JSON data")
	})

	t.Run("empty target preserves primary without rebuilding target secondary index", func(t *testing.T) {
		f := initFixture(t)
		recordJSON := []byte(`{
			"epoch_id": 11,
			"reporter_account": "lumera1bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbadc7mh",
			"target_account": "",
			"ticket_id": "ticket-fb10",
			"result_class": 1,
			"bucket_type": 4,
			"artifact_class": 1,
			"recheck_eligible": false,
			"unexpected_future_field": "preserved"
		}`)
		genesisState := types.GenesisState{
			Params: types.DefaultParams(),
			StorageProofTranscripts: []types.GenesisStorageProofTranscript{
				{TranscriptHash: "h-fb10-primary-only", RecordJson: recordJSON},
			},
		}

		require.NoError(t, f.keeper.InitGenesis(f.ctx, genesisState))
		exported, err := f.keeper.ExportGenesis(f.ctx)
		require.NoError(t, err)
		require.Len(t, exported.StorageProofTranscripts, 1)
		require.Equal(t, recordJSON, exported.StorageProofTranscripts[0].RecordJson)

		found, err := keeper.HasCleanRecheckInWindowForTest(f.keeper, f.ctx, ticket, target, 10, 11)
		require.NoError(t, err)
		require.False(t, found, "empty target_account must not create a malformed target secondary-index entry")
	})
}

func TestGenesisFinalGateStateRoundTrip(t *testing.T) {
	f := initFixture(t)

	genesisState := types.GenesisState{
		Params: types.DefaultParams(),
		HealOps: []types.HealOp{
			{
				HealOpId:                  11,
				TicketId:                  "ticket-heal-11",
				ScheduledEpochId:          3,
				HealerSupernodeAccount:    "lumera1aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa5xm4ep",
				VerifierSupernodeAccounts: []string{"lumera1bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbadc7mh", "lumera1cccccccccccccccccccccccccccccccccc7gqs5y"},
				Status:                    types.HealOpStatus_HEAL_OP_STATUS_HEALER_REPORTED,
				CreatedHeight:             10,
				UpdatedHeight:             12,
				DeadlineEpochId:           6,
				ResultHash:                "heal-result-hash",
			},
		},
		ActionFinalizationPostponements: []types.GenesisActionFinalizationPostponement{
			{SupernodeAccount: "lumera1ddddddddddddddddddddddddddddddddddx2nrmt", PostponedAtEpochId: 13},
		},
		EvidenceEpochCounts: []types.GenesisEvidenceEpochCount{
			{
				EpochId:        13,
				SubjectAddress: "lumera1eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeennf6kk",
				EvidenceType:   types.EvidenceType_EVIDENCE_TYPE_ACTION_EXPIRED,
				Count:          2,
			},
		},
		HealOpVerifications: []types.GenesisHealOpVerification{
			{HealOpId: 11, VerifierSupernodeAccount: "lumera1bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbadc7mh", Verified: true},
			{HealOpId: 11, VerifierSupernodeAccount: "lumera1cccccccccccccccccccccccccccccccccc7gqs5y", Verified: false},
		},
	}

	require.NoError(t, f.keeper.InitGenesis(f.ctx, genesisState))

	got, err := f.keeper.ExportGenesis(f.ctx)
	require.NoError(t, err)
	require.ElementsMatch(t, genesisState.ActionFinalizationPostponements, got.ActionFinalizationPostponements)
	require.ElementsMatch(t, genesisState.EvidenceEpochCounts, got.EvidenceEpochCounts)
	require.ElementsMatch(t, genesisState.HealOpVerifications, got.HealOpVerifications)

	f2 := initFixture(t)
	require.NoError(t, f2.keeper.InitGenesis(f2.ctx, *got))
	roundTripped, err := f2.keeper.ExportGenesis(f2.ctx)
	require.NoError(t, err)
	require.ElementsMatch(t, got.ActionFinalizationPostponements, roundTripped.ActionFinalizationPostponements)
	require.ElementsMatch(t, got.EvidenceEpochCounts, roundTripped.EvidenceEpochCounts)
	require.ElementsMatch(t, got.HealOpVerifications, roundTripped.HealOpVerifications)
}

func TestGenesisRoundTripWithTicketArtifactCountStates(t *testing.T) {
	f := initFixture(t)

	genesisState := types.GenesisState{
		Params: types.DefaultParams(),
		TicketArtifactCountStates: []types.TicketArtifactCountState{
			{
				TicketId:            "ticket-1",
				IndexArtifactCount:  32,
				SymbolArtifactCount: 128,
			},
		},
	}

	require.NoError(t, f.keeper.InitGenesis(f.ctx, genesisState))

	got, err := f.keeper.ExportGenesis(f.ctx)
	require.NoError(t, err)
	require.Len(t, got.TicketArtifactCountStates, 1)
	require.Equal(t, genesisState.TicketArtifactCountStates[0], got.TicketArtifactCountStates[0])
}
