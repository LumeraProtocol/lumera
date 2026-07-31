package keeper_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/LumeraProtocol/lumera/x/audit/v1/keeper"
	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func auditStoreSnapshot(f *fixture) map[string][]byte {
	store := f.ctx.KVStore(f.storeKey)
	it := store.Iterator(nil, nil)
	defer func() { _ = it.Close() }()
	out := make(map[string][]byte)
	for ; it.Valid(); it.Next() {
		out[string(it.Key())] = append([]byte(nil), it.Value()...)
	}
	return out
}

func TestAccountTransitionPlanStaleAndDuplicateApplyHaveZeroWrites(t *testing.T) {
	t.Run("same-count raw transition corruption", func(t *testing.T) {
		f := initFixture(t)
		a := testAddress(t, f, []byte{1, 1, 1, 10})
		b := testAddress(t, f, []byte{2, 2, 2, 20})
		c := testAddress(t, f, []byte{3, 3, 3, 30})
		d := testAddress(t, f, []byte{4, 4, 4, 40})
		require.NoError(t, recordAccountTransition(t, f, types.AccountTransition{SourceAccount: a, DestinationAccount: b, EffectiveEpoch: 1}))
		f.ctx.KVStore(f.storeKey).Set(types.ActionFinalizationPostponementKey(c), []byte{0, 0, 0, 0, 0, 0, 0, 7})
		plan, err := buildAccountTransitionPlan(t, f, types.AccountTransition{SourceAccount: c, DestinationAccount: d, EffectiveEpoch: 2})
		require.NoError(t, err)

		store := f.ctx.KVStore(f.storeKey)
		store.Set(types.AccountTransitionForwardKey(a), []byte{0xff})
		before := auditStoreSnapshot(f)
		err = f.keeper.ApplyAccountTransitionPlan(f.ctx, plan)
		require.ErrorContains(t, err, "index precondition")
		require.Equal(t, before, auditStoreSnapshot(f))
		require.True(t, store.Has(types.ActionFinalizationPostponementKey(c)))
		require.False(t, store.Has(types.ActionFinalizationPostponementKey(d)))
		require.False(t, store.Has(types.AccountTransitionForwardKey(c)))
	})

	t.Run("duplicate", func(t *testing.T) {
		f := initFixture(t)
		a := testAddress(t, f, []byte{11, 11, 11, 11})
		b := testAddress(t, f, []byte{12, 12, 12, 12})
		plan, err := buildAccountTransitionPlan(t, f, types.AccountTransition{SourceAccount: a, DestinationAccount: b, EffectiveEpoch: 1})
		require.NoError(t, err)
		require.NoError(t, f.keeper.ApplyAccountTransitionPlan(f.ctx, plan))
		before := auditStoreSnapshot(f)
		err = f.keeper.ApplyAccountTransitionPlan(f.ctx, plan)
		require.ErrorContains(t, err, "stale or already applied")
		require.Equal(t, before, auditStoreSnapshot(f))
	})
}

func TestAccountTransitionRejectsForwardAndReverseKeyValueMismatch(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		f := initFixture(t)
		a := testAddress(t, f, []byte{21, 21, 21, 21})
		b := testAddress(t, f, []byte{22, 22, 22, 22})
		other := testAddress(t, f, []byte{23, 23, 23, 23})
		require.NoError(t, recordAccountTransition(t, f, types.AccountTransition{SourceAccount: a, DestinationAccount: b, EffectiveEpoch: 1}))
		store := f.ctx.KVStore(f.storeKey)
		if reverse {
			store.Set(types.AccountTransitionReverseKey(other), store.Get(types.AccountTransitionReverseKey(b)))
			_, err := f.keeper.CurrentAccount(f.ctx, other)
			require.ErrorContains(t, err, "reverse index key does not match")
		} else {
			store.Set(types.AccountTransitionForwardKey(other), store.Get(types.AccountTransitionForwardKey(a)))
			_, err := f.keeper.CurrentAccount(f.ctx, other)
			require.ErrorContains(t, err, "forward index key does not match")
		}
	}
}

func TestMigratedTicketTargetAndReporterRemainSameIdentities(t *testing.T) {
	f := initFixture(t)
	targetOld := testAddress(t, f, []byte{31, 31, 31, 31})
	targetNew := testAddress(t, f, []byte{32, 32, 32, 32})
	reporterOld := testAddress(t, f, []byte{33, 33, 33, 33})
	reporterNew := testAddress(t, f, []byte{34, 34, 34, 34})
	require.NoError(t, f.keeper.SetTicketDeteriorationState(f.ctx, types.TicketDeteriorationState{
		TicketId: "same-lineage", DeteriorationScore: 5, LastUpdatedEpoch: 1,
		LastFailureEpoch: 1, RecentFailureEpochCount: 1, DistinctHolderFailureCount: 1,
		LastTargetSupernodeAccount: targetOld, LastReporterSupernodeAccount: reporterOld,
		LastResultClass: types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_HASH_MISMATCH, LastResultEpoch: 1,
	}))
	require.NoError(t, recordAccountTransition(t, f, types.AccountTransition{SourceAccount: targetOld, DestinationAccount: targetNew, EffectiveEpoch: 2}))
	require.NoError(t, recordAccountTransition(t, f, types.AccountTransition{SourceAccount: reporterOld, DestinationAccount: reporterNew, EffectiveEpoch: 2}))

	state, updated, err := keeper.ApplyTicketDeteriorationDeltaForTest(f.keeper, f.ctx, 3, reporterNew, &types.StorageProofResult{
		TicketId: "same-lineage", TargetSupernodeAccount: targetNew,
		ResultClass: types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_HASH_MISMATCH,
	}, "same-lineage", 5, 0, false)
	require.NoError(t, err)
	require.True(t, updated)
	require.Equal(t, uint32(1), state.DistinctHolderFailureCount, "migration is not a distinct holder")
	require.Equal(t, targetNew, state.LastTargetSupernodeAccount)
	require.Equal(t, reporterNew, state.LastReporterSupernodeAccount)
}

func TestCurrentReporterReliabilityScalesMigratedReporterResult(t *testing.T) {
	f := initFixture(t)
	reporterOld := testAddress(t, f, []byte{41, 41, 41, 41})
	reporterNew := testAddress(t, f, []byte{42, 42, 42, 42})
	target := testAddress(t, f, []byte{43, 43, 43, 43})
	require.NoError(t, f.keeper.SetReporterReliabilityState(f.ctx, types.ReporterReliabilityState{
		ReporterSupernodeAccount: reporterOld, ReliabilityScore: 50, LastUpdatedEpoch: 2,
	}))
	require.NoError(t, recordAccountTransition(t, f, types.AccountTransition{SourceAccount: reporterOld, DestinationAccount: reporterNew, EffectiveEpoch: 2}))
	require.NoError(t, keeper.ApplyStorageTruthScoresForTest(f.keeper, f.ctx.WithEventManager(sdk.NewEventManager()), 2, reporterNew, []*types.StorageProofResult{{
		TicketId: "scaled", TargetSupernodeAccount: target,
		ResultClass:   types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_HASH_MISMATCH,
		ArtifactClass: types.StorageProofArtifactClass_STORAGE_PROOF_ARTIFACT_CLASS_SYMBOL,
		BucketType:    types.StorageProofBucketType_STORAGE_PROOF_BUCKET_TYPE_RECENT,
	}}))
	node, found := f.keeper.GetNodeSuspicionState(f.ctx, target)
	require.True(t, found)
	require.Equal(t, int64(9), node.SuspicionScore, "current singleton reliability 50 scales +18 to +9")
	ticket, found := f.keeper.GetTicketDeteriorationState(f.ctx, "scaled")
	require.True(t, found)
	require.Equal(t, int64(2), ticket.DeteriorationScore, "current singleton reliability 50 scales +5 toward zero")
}

func TestHistoricalPeerPortObservationUsesEpochTarget(t *testing.T) {
	f := initFixture(t)
	targetOld := testAddress(t, f, []byte{51, 51, 51, 51})
	targetNew := testAddress(t, f, []byte{52, 52, 52, 52})
	reporter := testAddress(t, f, []byte{53, 53, 53, 53})
	require.NoError(t, f.keeper.SetReportRaw(f.ctx, types.EpochReport{
		EpochId: 1, SupernodeAccount: reporter,
		StorageChallengeObservations: []*types.StorageChallengeObservation{{TargetSupernodeAccount: targetOld, PortStates: []types.PortState{types.PortState_PORT_STATE_OPEN}}},
	}))
	f.keeper.SetStorageChallengeReportIndex(f.ctx, targetOld, 1, reporter)
	require.NoError(t, recordAccountTransition(t, f, types.AccountTransition{SourceAccount: targetOld, DestinationAccount: targetNew, EffectiveEpoch: 2}))
	met, err := keeper.PeersPortStateMeetsThresholdForTest(f.keeper, f.ctx, targetNew, 1, 0, types.PortState_PORT_STATE_OPEN, 100)
	require.NoError(t, err)
	require.True(t, met)
}

func TestMigratedRecheckerDedupSelfAttestationAndTranscriptIdentity(t *testing.T) {
	f := initFixture(t)
	f.ctx = f.ctx.WithBlockHeight(1).WithEventManager(sdk.NewEventManager())
	recheckerOld := testAddress(t, f, []byte{61, 61, 61, 61})
	recheckerNew := testAddress(t, f, []byte{62, 62, 62, 62})
	targetOld := testAddress(t, f, []byte{63, 63, 63, 63})
	targetNew := testAddress(t, f, []byte{64, 64, 64, 64})
	originalReporter := testAddress(t, f, []byte{65, 65, 65, 65})
	seedEpochAnchorForReportTest(t, f, 0, []string{recheckerOld, targetOld}, []string{recheckerOld, targetOld})
	seedIndexedChallengeResult(t, f, originalReporter, targetOld, "migrated-ticket", "challenged-hash")
	require.NoError(t, recordAccountTransition(t, f, types.AccountTransition{SourceAccount: recheckerOld, DestinationAccount: recheckerNew, EffectiveEpoch: 1}))
	require.NoError(t, recordAccountTransition(t, f, types.AccountTransition{SourceAccount: targetOld, DestinationAccount: targetNew, EffectiveEpoch: 1}))
	f.supernodeKeeper.EXPECT().GetSuperNodeByAccount(gomock.Any(), recheckerNew).Return(sntypes.SuperNode{}, true, nil).AnyTimes()
	f.supernodeKeeper.EXPECT().GetSuperNodeByAccount(gomock.Any(), targetNew).Return(sntypes.SuperNode{}, true, nil).AnyTimes()
	ms := keeper.NewMsgServerImpl(f.keeper)

	self := &types.MsgSubmitStorageRecheckEvidence{Creator: targetNew, EpochId: 0, ChallengedSupernodeAccount: targetNew, TicketId: "migrated-ticket", ChallengedResultTranscriptHash: "challenged-hash", RecheckTranscriptHash: "self-hash", RecheckResultClass: types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_PASS}
	_, err := ms.SubmitStorageRecheckEvidence(f.ctx, self)
	require.ErrorContains(t, err, "target lineage")

	req := &types.MsgSubmitStorageRecheckEvidence{Creator: recheckerNew, EpochId: 0, ChallengedSupernodeAccount: targetNew, TicketId: "migrated-ticket", ChallengedResultTranscriptHash: "challenged-hash", RecheckTranscriptHash: "recheck-hash", RecheckResultClass: types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_PASS}
	_, err = ms.SubmitStorageRecheckEvidence(f.ctx, req)
	require.NoError(t, err)
	_, err = ms.SubmitStorageRecheckEvidence(f.ctx, req)
	require.ErrorContains(t, err, "already submitted")

	exported, err := f.keeper.ExportGenesis(f.ctx)
	require.NoError(t, err)
	require.Contains(t, exported.RecheckEvidence, types.GenesisRecheckEvidence{EpochId: 0, TicketId: "migrated-ticket", CreatorAccount: recheckerOld})
	var recheckJSON []byte
	for _, transcript := range exported.StorageProofTranscripts {
		if transcript.TranscriptHash == "recheck-hash" {
			recheckJSON = transcript.RecordJson
		}
	}
	require.NotEmpty(t, recheckJSON)
	var record map[string]any
	require.NoError(t, json.Unmarshal(recheckJSON, &record))
	require.Equal(t, recheckerOld, record["reporter_account"])
	require.Equal(t, "challenged-hash", record["challenged_transcript_hash"])
	require.False(t, bytes.Contains(recheckJSON, []byte(recheckerNew)), "historical transcript preserves epoch identity")
}

func TestRecheckScoresHistoricalTargetUnderCurrentEpochIdentity(t *testing.T) {
	f := initFixture(t)
	f.ctx = f.ctx.WithBlockHeight(401).WithEventManager(sdk.NewEventManager())
	creator := testAddress(t, f, []byte{71, 71, 71, 71})
	targetOld := testAddress(t, f, []byte{72, 72, 72, 72})
	targetNew := testAddress(t, f, []byte{73, 73, 73, 73})
	originalReporter := testAddress(t, f, []byte{74, 74, 74, 74})
	seedEpochAnchorForReportTest(t, f, 0, []string{creator, targetOld}, []string{creator, targetOld})
	seedIndexedChallengeResult(t, f, originalReporter, targetOld, "recheck-current-target", "historical-target-hash")
	require.NoError(t, recordAccountTransition(t, f, types.AccountTransition{SourceAccount: targetOld, DestinationAccount: targetNew, EffectiveEpoch: 1}))
	f.supernodeKeeper.EXPECT().GetSuperNodeByAccount(gomock.Any(), creator).Return(sntypes.SuperNode{}, true, nil).AnyTimes()
	f.supernodeKeeper.EXPECT().GetSuperNodeByAccount(gomock.Any(), targetNew).Return(sntypes.SuperNode{}, true, nil).AnyTimes()

	_, err := keeper.NewMsgServerImpl(f.keeper).SubmitStorageRecheckEvidence(f.ctx, &types.MsgSubmitStorageRecheckEvidence{
		Creator:                        creator,
		EpochId:                        0,
		ChallengedSupernodeAccount:     targetOld,
		TicketId:                       "recheck-current-target",
		ChallengedResultTranscriptHash: "historical-target-hash",
		RecheckTranscriptHash:          "current-epoch-recheck-hash",
		RecheckResultClass:             types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_RECHECK_CONFIRMED_FAIL,
	})
	require.NoError(t, err)
	_, found := f.keeper.GetNodeSuspicionState(f.ctx, targetNew)
	require.True(t, found, "current-epoch scoring must write the current logical target")
	_, found = f.keeper.GetNodeSuspicionState(f.ctx, targetOld)
	require.False(t, found, "historical alias must not receive current-epoch singleton state")
}
