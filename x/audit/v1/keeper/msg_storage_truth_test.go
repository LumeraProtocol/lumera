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

func seedIndexedChallengeResult(t *testing.T, f *fixture, originalReporter string, challenged string, ticketID string, transcriptHash string) {
	t.Helper()
	result := &types.StorageProofResult{
		TargetSupernodeAccount:     challenged,
		ChallengerSupernodeAccount: originalReporter,
		BucketType:                 types.StorageProofBucketType_STORAGE_PROOF_BUCKET_TYPE_RECENT,
		ResultClass:                types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_HASH_MISMATCH,
		TicketId:                   ticketID,
		ArtifactClass:              types.StorageProofArtifactClass_STORAGE_PROOF_ARTIFACT_CLASS_INDEX,
		ArtifactKey:                "artifact-key-" + ticketID,
		ArtifactOrdinal:            1,
		ArtifactCount:              8,
		TranscriptHash:             transcriptHash,
		DerivationInputHash:        "derivation-hash-" + ticketID,
		ChallengerSignature:        "challenger-signature-" + ticketID,
	}
	require.NoError(t, f.keeper.IndexStorageProofTranscripts(f.ctx, 0, originalReporter, []*types.StorageProofResult{result}))
	require.NoError(t, f.keeper.SetTicketDeteriorationState(f.ctx, types.TicketDeteriorationState{
		TicketId:                     ticketID,
		DeteriorationScore:           20,
		LastUpdatedEpoch:             0,
		LastTargetSupernodeAccount:   challenged,
		LastReporterSupernodeAccount: originalReporter,
		LastResultClass:              types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_HASH_MISMATCH,
		LastResultEpoch:              0,
	}))
}

func TestMsgSubmitStorageRecheckEvidence(t *testing.T) {
	f := initFixture(t)
	f.ctx = f.ctx.WithBlockHeight(1).WithEventManager(sdk.NewEventManager())
	ms := keeper.NewMsgServerImpl(f.keeper)

	creator := "sn-aaa-rechecker"
	challenged := "sn-bbb-target"
	originalReporter := "sn-ccc-original"

	f.supernodeKeeper.EXPECT().
		GetSuperNodeByAccount(gomock.Any(), creator).
		Return(sntypes.SuperNode{}, true, nil).
		AnyTimes()
	f.supernodeKeeper.EXPECT().
		GetSuperNodeByAccount(gomock.Any(), challenged).
		Return(sntypes.SuperNode{}, true, nil).
		AnyTimes()

	seedEpochAnchorForReportTest(t, f, 0, []string{creator, challenged}, []string{creator, challenged})
	seedIndexedChallengeResult(t, f, originalReporter, challenged, "ticket-1", "old-hash")

	_, err := ms.SubmitStorageRecheckEvidence(f.ctx, nil)
	require.Error(t, err)

	_, err = ms.SubmitStorageRecheckEvidence(f.ctx, &types.MsgSubmitStorageRecheckEvidence{
		Creator: creator,
		EpochId: 0,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "challenged_supernode_account is required")

	_, err = ms.SubmitStorageRecheckEvidence(f.ctx, &types.MsgSubmitStorageRecheckEvidence{
		Creator:                        creator,
		EpochId:                        0,
		ChallengedSupernodeAccount:     creator,
		TicketId:                       "ticket-1",
		ChallengedResultTranscriptHash: "old-hash",
		RecheckTranscriptHash:          "new-hash",
		RecheckResultClass:             types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_PASS,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "must not equal creator")

	// Valid request: recheck is now implemented and should succeed.
	_, err = ms.SubmitStorageRecheckEvidence(f.ctx, &types.MsgSubmitStorageRecheckEvidence{
		Creator:                        creator,
		EpochId:                        0,
		ChallengedSupernodeAccount:     challenged,
		TicketId:                       "ticket-1",
		ChallengedResultTranscriptHash: "old-hash",
		RecheckTranscriptHash:          "new-hash",
		RecheckResultClass:             types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_RECHECK_CONFIRMED_FAIL,
	})
	require.NoError(t, err)

	// Scores should now be updated for RECHECK_CONFIRMED_FAIL.
	nodeState, found := f.keeper.GetNodeSuspicionState(f.ctx, challenged)
	require.True(t, found)
	require.Greater(t, nodeState.SuspicionScore, int64(0))

	// A second rechecker cannot link the same challenged transcript to a different
	// recheck transcript hash.
	secondRechecker := "sn-ddd-rechecker-2"
	f.supernodeKeeper.EXPECT().
		GetSuperNodeByAccount(gomock.Any(), secondRechecker).
		Return(sntypes.SuperNode{}, true, nil).
		AnyTimes()

	_, err = ms.SubmitStorageRecheckEvidence(f.ctx, &types.MsgSubmitStorageRecheckEvidence{
		Creator:                        secondRechecker,
		EpochId:                        0,
		ChallengedSupernodeAccount:     challenged,
		TicketId:                       "ticket-1",
		ChallengedResultTranscriptHash: "old-hash",
		RecheckTranscriptHash:          "new-hash-2",
		RecheckResultClass:             types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_PASS,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "already linked")

	// Replay must fail.
	_, err = ms.SubmitStorageRecheckEvidence(f.ctx, &types.MsgSubmitStorageRecheckEvidence{
		Creator:                        creator,
		EpochId:                        0,
		ChallengedSupernodeAccount:     challenged,
		TicketId:                       "ticket-1",
		ChallengedResultTranscriptHash: "old-hash",
		RecheckTranscriptHash:          "new-hash",
		RecheckResultClass:             types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_RECHECK_CONFIRMED_FAIL,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "already submitted")
}

func TestMsgClaimHealCompleteAndSubmitVerification(t *testing.T) {
	f := initFixture(t)
	f.ctx = f.ctx.WithBlockHeight(1).WithEventManager(sdk.NewEventManager())
	ms := keeper.NewMsgServerImpl(f.keeper)

	healOp := types.HealOp{
		HealOpId:                  11,
		TicketId:                  "ticket-11",
		ScheduledEpochId:          0,
		HealerSupernodeAccount:    "sn-healer",
		VerifierSupernodeAccounts: []string{"sn-verifier-a", "sn-verifier-b"},
		Status:                    types.HealOpStatus_HEAL_OP_STATUS_SCHEDULED,
		CreatedHeight:             1,
		UpdatedHeight:             1,
		DeadlineEpochId:           1,
	}
	require.NoError(t, f.keeper.SetHealOp(f.ctx, healOp))
	require.NoError(t, f.keeper.SetTicketDeteriorationState(f.ctx, types.TicketDeteriorationState{
		TicketId:           healOp.TicketId,
		DeteriorationScore: 110,
		ActiveHealOpId:     healOp.HealOpId,
	}))

	_, err := ms.ClaimHealComplete(f.ctx, &types.MsgClaimHealComplete{
		Creator:          "sn-not-healer",
		HealOpId:         healOp.HealOpId,
		TicketId:         healOp.TicketId,
		HealManifestHash: "manifest-1",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), types.ErrHealOpUnauthorized.Error())

	_, err = ms.ClaimHealComplete(f.ctx, &types.MsgClaimHealComplete{
		Creator:          "sn-healer",
		HealOpId:         healOp.HealOpId,
		TicketId:         healOp.TicketId,
		HealManifestHash: "manifest-1",
		Details:          "healer completed",
	})
	require.NoError(t, err)

	claimed, found := f.keeper.GetHealOp(f.ctx, healOp.HealOpId)
	require.True(t, found)
	require.Equal(t, types.HealOpStatus_HEAL_OP_STATUS_HEALER_REPORTED, claimed.Status)
	require.Equal(t, "manifest-1", claimed.ResultHash)
	require.Contains(t, claimed.Notes, "healer completed")

	// Per 120-F6 — positive attestation hash must match the heal-op ResultHash.
	_, err = ms.SubmitHealVerification(f.ctx, &types.MsgSubmitHealVerification{
		Creator:          "sn-verifier-a",
		HealOpId:         healOp.HealOpId,
		Verified:         true,
		VerificationHash: "manifest-1",
	})
	require.NoError(t, err)

	inFlight, found := f.keeper.GetHealOp(f.ctx, healOp.HealOpId)
	require.True(t, found)
	require.Equal(t, types.HealOpStatus_HEAL_OP_STATUS_HEALER_REPORTED, inFlight.Status)

	_, err = ms.SubmitHealVerification(f.ctx, &types.MsgSubmitHealVerification{
		Creator:          "sn-verifier-a",
		HealOpId:         healOp.HealOpId,
		Verified:         true,
		VerificationHash: "manifest-1-repeat",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), types.ErrHealVerificationExists.Error())

	_, err = ms.SubmitHealVerification(f.ctx, &types.MsgSubmitHealVerification{
		Creator:          "sn-verifier-b",
		HealOpId:         healOp.HealOpId,
		Verified:         true,
		VerificationHash: "manifest-1",
	})
	require.NoError(t, err)

	finalized, found := f.keeper.GetHealOp(f.ctx, healOp.HealOpId)
	require.True(t, found)
	require.Equal(t, types.HealOpStatus_HEAL_OP_STATUS_VERIFIED, finalized.Status)

	ticketState, found := f.keeper.GetTicketDeteriorationState(f.ctx, healOp.TicketId)
	require.True(t, found)
	require.Equal(t, uint64(0), ticketState.ActiveHealOpId)
	require.Equal(t, uint64(0), ticketState.LastHealEpoch)
	require.Equal(t, uint64(types.DefaultStorageTruthProbationEpochs), ticketState.ProbationUntilEpoch)
}

func TestMsgSubmitHealVerification_FailedPath(t *testing.T) {
	f := initFixture(t)
	f.ctx = f.ctx.WithBlockHeight(1).WithEventManager(sdk.NewEventManager())
	ms := keeper.NewMsgServerImpl(f.keeper)

	// Use 3 verifiers: majority quorum = 2. Two fails → FAILED.
	healOp := types.HealOp{
		HealOpId:                  12,
		TicketId:                  "ticket-12",
		ScheduledEpochId:          0,
		HealerSupernodeAccount:    "sn-healer",
		VerifierSupernodeAccounts: []string{"sn-verifier-a", "sn-verifier-b", "sn-verifier-c"},
		Status:                    types.HealOpStatus_HEAL_OP_STATUS_HEALER_REPORTED,
		CreatedHeight:             1,
		UpdatedHeight:             1,
		DeadlineEpochId:           1,
	}
	require.NoError(t, f.keeper.SetHealOp(f.ctx, healOp))
	require.NoError(t, f.keeper.SetTicketDeteriorationState(f.ctx, types.TicketDeteriorationState{
		TicketId:            healOp.TicketId,
		DeteriorationScore:  120,
		ActiveHealOpId:      healOp.HealOpId,
		ProbationUntilEpoch: 17,
	}))

	// First negative vote — not yet majority (1/3).
	_, err := ms.SubmitHealVerification(f.ctx, &types.MsgSubmitHealVerification{
		Creator:          "sn-verifier-a",
		HealOpId:         healOp.HealOpId,
		Verified:         false,
		VerificationHash: "verify-fail-1",
	})
	require.NoError(t, err)

	// Should still be in progress.
	inFlight, found := f.keeper.GetHealOp(f.ctx, healOp.HealOpId)
	require.True(t, found)
	require.Equal(t, types.HealOpStatus_HEAL_OP_STATUS_HEALER_REPORTED, inFlight.Status)

	// Second negative vote — now majority (2/3) → FAILED.
	_, err = ms.SubmitHealVerification(f.ctx, &types.MsgSubmitHealVerification{
		Creator:          "sn-verifier-b",
		HealOpId:         healOp.HealOpId,
		Verified:         false,
		VerificationHash: "verify-fail-2",
	})
	require.NoError(t, err)

	finalized, found := f.keeper.GetHealOp(f.ctx, healOp.HealOpId)
	require.True(t, found)
	require.Equal(t, types.HealOpStatus_HEAL_OP_STATUS_FAILED, finalized.Status)

	ticketState, found := f.keeper.GetTicketDeteriorationState(f.ctx, healOp.TicketId)
	require.True(t, found)
	require.Equal(t, uint64(0), ticketState.ActiveHealOpId)
	// Failed verification does not move probation/last-heal markers.
	require.Equal(t, uint64(17), ticketState.ProbationUntilEpoch)
	require.Equal(t, uint64(0), ticketState.LastHealEpoch)
}

// ---------------------------------------------------------------------------
// SubmitStorageRecheckEvidence: additional validation / coverage gaps
// ---------------------------------------------------------------------------

func TestMsgSubmitStorageRecheckEvidence_UnregisteredCreatorRejected(t *testing.T) {
	f := initFixture(t)
	f.ctx = f.ctx.WithBlockHeight(1).WithEventManager(sdk.NewEventManager())
	ms := keeper.NewMsgServerImpl(f.keeper)

	creator := "sn-unknown-creator"
	challenged := "sn-known-target"

	seedEpochAnchorForReportTest(t, f, 0, []string{creator, challenged}, []string{creator, challenged})

	// Creator is NOT a registered supernode.
	f.supernodeKeeper.EXPECT().
		GetSuperNodeByAccount(gomock.Any(), creator).
		Return(sntypes.SuperNode{}, false, nil).AnyTimes()

	_, err := ms.SubmitStorageRecheckEvidence(f.ctx, &types.MsgSubmitStorageRecheckEvidence{
		Creator:                        creator,
		EpochId:                        0,
		ChallengedSupernodeAccount:     challenged,
		TicketId:                       "ticket-x",
		ChallengedResultTranscriptHash: "orig-hash",
		RecheckTranscriptHash:          "recheck-hash",
		RecheckResultClass:             types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_RECHECK_CONFIRMED_FAIL,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "creator is not a registered supernode")
}

func TestMsgSubmitStorageRecheckEvidence_UnregisteredChallengedRejected(t *testing.T) {
	f := initFixture(t)
	f.ctx = f.ctx.WithBlockHeight(1).WithEventManager(sdk.NewEventManager())
	ms := keeper.NewMsgServerImpl(f.keeper)

	creator := "sn-valid-creator"
	challenged := "sn-unknown-challenged"

	seedEpochAnchorForReportTest(t, f, 0, []string{creator, challenged}, []string{creator, challenged})

	// Creator is found; challenged is NOT registered.
	f.supernodeKeeper.EXPECT().
		GetSuperNodeByAccount(gomock.Any(), creator).
		Return(sntypes.SuperNode{SupernodeAccount: creator}, true, nil).AnyTimes()
	f.supernodeKeeper.EXPECT().
		GetSuperNodeByAccount(gomock.Any(), challenged).
		Return(sntypes.SuperNode{}, false, nil).AnyTimes()

	_, err := ms.SubmitStorageRecheckEvidence(f.ctx, &types.MsgSubmitStorageRecheckEvidence{
		Creator:                        creator,
		EpochId:                        0,
		ChallengedSupernodeAccount:     challenged,
		TicketId:                       "ticket-y",
		ChallengedResultTranscriptHash: "orig-hash",
		RecheckTranscriptHash:          "recheck-hash",
		RecheckResultClass:             types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_RECHECK_CONFIRMED_FAIL,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "challenged_supernode_account is not a registered supernode")
}

func TestMsgSubmitStorageRecheckEvidence_EmptyTranscriptHashRejected(t *testing.T) {
	f := initFixture(t)
	f.ctx = f.ctx.WithBlockHeight(1).WithEventManager(sdk.NewEventManager())
	ms := keeper.NewMsgServerImpl(f.keeper)

	creator := "sn-creator"
	challenged := "sn-target"

	seedEpochAnchorForReportTest(t, f, 0, []string{creator, challenged}, []string{creator, challenged})

	// Empty ChallengedResultTranscriptHash rejected before any keeper call.
	_, err := ms.SubmitStorageRecheckEvidence(f.ctx, &types.MsgSubmitStorageRecheckEvidence{
		Creator:                        creator,
		EpochId:                        0,
		ChallengedSupernodeAccount:     challenged,
		TicketId:                       "ticket-z",
		ChallengedResultTranscriptHash: "",
		RecheckTranscriptHash:          "recheck-hash",
		RecheckResultClass:             types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_RECHECK_CONFIRMED_FAIL,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "challenged_result_transcript_hash is required")

	// Empty RecheckTranscriptHash rejected.
	_, err = ms.SubmitStorageRecheckEvidence(f.ctx, &types.MsgSubmitStorageRecheckEvidence{
		Creator:                        creator,
		EpochId:                        0,
		ChallengedSupernodeAccount:     challenged,
		TicketId:                       "ticket-z2",
		ChallengedResultTranscriptHash: "orig-hash",
		RecheckTranscriptHash:          "",
		RecheckResultClass:             types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_RECHECK_CONFIRMED_FAIL,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "recheck_transcript_hash is required")
}

func TestMsgSubmitStorageRecheckEvidence_UpdatesReporterReliability(t *testing.T) {
	f := initFixture(t)
	f.ctx = f.ctx.WithBlockHeight(1).WithEventManager(sdk.NewEventManager())
	ms := keeper.NewMsgServerImpl(f.keeper)

	creator := "sn-reporter"
	challenged := "sn-challenged-node"
	originalReporter := "sn-original-reporter-rel"

	params := types.DefaultParams()
	params.StorageTruthEnforcementMode = types.StorageTruthEnforcementMode_STORAGE_TRUTH_ENFORCEMENT_MODE_SHADOW
	require.NoError(t, f.keeper.SetParams(f.ctx, params))

	seedEpochAnchorForReportTest(t, f, 0, []string{creator, challenged}, []string{creator, challenged})
	seedIndexedChallengeResult(t, f, originalReporter, challenged, "ticket-rel", "orig-hash")

	f.supernodeKeeper.EXPECT().
		GetSuperNodeByAccount(gomock.Any(), creator).
		Return(sntypes.SuperNode{SupernodeAccount: creator}, true, nil).AnyTimes()
	f.supernodeKeeper.EXPECT().
		GetSuperNodeByAccount(gomock.Any(), challenged).
		Return(sntypes.SuperNode{SupernodeAccount: challenged}, true, nil).AnyTimes()

	// Before recheck: reporter should have no reliability state.
	_, beforeFound := f.keeper.GetReporterReliabilityState(f.ctx, creator)
	require.False(t, beforeFound, "reporter reliability state should not exist before any recheck")

	_, err := ms.SubmitStorageRecheckEvidence(f.ctx, &types.MsgSubmitStorageRecheckEvidence{
		Creator:                        creator,
		EpochId:                        0,
		ChallengedSupernodeAccount:     challenged,
		TicketId:                       "ticket-rel",
		ChallengedResultTranscriptHash: "orig-hash",
		RecheckTranscriptHash:          "recheck-hash",
		RecheckResultClass:             types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_RECHECK_CONFIRMED_FAIL,
	})
	require.NoError(t, err)

	// After recheck: reporter reliability state should exist (created by applyStorageTruthScores).
	_, afterFound := f.keeper.GetReporterReliabilityState(f.ctx, creator)
	require.True(t, afterFound, "reporter reliability state should be created after recheck evidence submission")
}

// TestMsgSubmitHealVerification_MajorityQuorum verifies that verification requires a
// majority (n/2+1) of verifiers to agree for finalization in either direction.
func TestMsgSubmitHealVerification_MajorityQuorum(t *testing.T) {
	f := initFixture(t)
	f.ctx = f.ctx.WithBlockHeight(1).WithEventManager(sdk.NewEventManager())
	ms := keeper.NewMsgServerImpl(f.keeper)

	// 4 verifiers: majority = 3. Test that 2 positives do NOT finalize.
	healOp := types.HealOp{
		HealOpId:                  30,
		TicketId:                  "ticket-quorum",
		ScheduledEpochId:          0,
		HealerSupernodeAccount:    "sn-healer-q",
		VerifierSupernodeAccounts: []string{"sn-v1", "sn-v2", "sn-v3", "sn-v4"},
		Status:                    types.HealOpStatus_HEAL_OP_STATUS_HEALER_REPORTED,
		CreatedHeight:             1,
		UpdatedHeight:             1,
		DeadlineEpochId:           5,
	}
	require.NoError(t, f.keeper.SetHealOp(f.ctx, healOp))
	require.NoError(t, f.keeper.SetTicketDeteriorationState(f.ctx, types.TicketDeteriorationState{
		TicketId:       healOp.TicketId,
		ActiveHealOpId: healOp.HealOpId,
	}))

	// First positive vote — not yet majority (1/4).
	_, err := ms.SubmitHealVerification(f.ctx, &types.MsgSubmitHealVerification{
		Creator:          "sn-v1",
		HealOpId:         healOp.HealOpId,
		Verified:         true,
		VerificationHash: "v1-hash",
	})
	require.NoError(t, err)

	inFlight, found := f.keeper.GetHealOp(f.ctx, healOp.HealOpId)
	require.True(t, found)
	require.Equal(t, types.HealOpStatus_HEAL_OP_STATUS_HEALER_REPORTED, inFlight.Status)

	// Second positive vote — not yet majority (2/4).
	_, err = ms.SubmitHealVerification(f.ctx, &types.MsgSubmitHealVerification{
		Creator:          "sn-v2",
		HealOpId:         healOp.HealOpId,
		Verified:         true,
		VerificationHash: "v2-hash",
	})
	require.NoError(t, err)

	inFlight, found = f.keeper.GetHealOp(f.ctx, healOp.HealOpId)
	require.True(t, found)
	require.Equal(t, types.HealOpStatus_HEAL_OP_STATUS_HEALER_REPORTED, inFlight.Status)

	// Third positive vote — majority (3/4) → VERIFIED.
	_, err = ms.SubmitHealVerification(f.ctx, &types.MsgSubmitHealVerification{
		Creator:          "sn-v3",
		HealOpId:         healOp.HealOpId,
		Verified:         true,
		VerificationHash: "v3-hash",
	})
	require.NoError(t, err)

	finalized, found := f.keeper.GetHealOp(f.ctx, healOp.HealOpId)
	require.True(t, found)
	require.Equal(t, types.HealOpStatus_HEAL_OP_STATUS_VERIFIED, finalized.Status)
}

// TestMsgSubmitStorageRecheckEvidence_OverturnPenalizesOriginalReporter verifies that
// when a recheck finds PASS (overturn of a previous fail), the original reporter gets
// a +25 penalty (LEP6.md §16.1 recheck-overturn penalty).
func TestMsgSubmitStorageRecheckEvidence_OverturnPenalizesOriginalReporter(t *testing.T) {
	f := initFixture(t)
	f.ctx = f.ctx.WithBlockHeight(1).WithEventManager(sdk.NewEventManager())
	ms := keeper.NewMsgServerImpl(f.keeper)

	recheckerAccount := "sn-rechecker"
	challengedAccount := "sn-challenged"
	originalReporter := "sn-original-reporter"

	params := types.DefaultParams()
	params.StorageTruthEnforcementMode = types.StorageTruthEnforcementMode_STORAGE_TRUTH_ENFORCEMENT_MODE_SHADOW
	require.NoError(t, f.keeper.SetParams(f.ctx, params))

	seedEpochAnchorForReportTest(t, f, 0, []string{recheckerAccount, challengedAccount}, []string{recheckerAccount, challengedAccount})

	f.supernodeKeeper.EXPECT().
		GetSuperNodeByAccount(gomock.Any(), recheckerAccount).
		Return(sntypes.SuperNode{SupernodeAccount: recheckerAccount}, true, nil).AnyTimes()
	f.supernodeKeeper.EXPECT().
		GetSuperNodeByAccount(gomock.Any(), challengedAccount).
		Return(sntypes.SuperNode{SupernodeAccount: challengedAccount}, true, nil).AnyTimes()

	seedIndexedChallengeResult(t, f, originalReporter, challengedAccount, "ticket-overturn", "orig-hash")

	// Original reporter has non-zero score.
	require.NoError(t, f.keeper.SetReporterReliabilityState(f.ctx, types.ReporterReliabilityState{
		ReporterSupernodeAccount: originalReporter,
		ReliabilityScore:         5,
		LastUpdatedEpoch:         0,
	}))

	// Submit recheck with PASS result — this is an overturn of the prior fail.
	_, err := ms.SubmitStorageRecheckEvidence(f.ctx, &types.MsgSubmitStorageRecheckEvidence{
		Creator:                        recheckerAccount,
		EpochId:                        0,
		ChallengedSupernodeAccount:     challengedAccount,
		TicketId:                       "ticket-overturn",
		ChallengedResultTranscriptHash: "orig-hash",
		RecheckTranscriptHash:          "recheck-hash",
		RecheckResultClass:             types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_PASS,
	})
	require.NoError(t, err)

	// The original reporter should be penalized +25 for the overturn.
	originalState, found := f.keeper.GetReporterReliabilityState(f.ctx, originalReporter)
	require.True(t, found)
	// Score=5, no decay (same epoch), +25 overturn penalty = 30.
	require.Equal(t, int64(30), originalState.ReliabilityScore)
}

// TestMsgSubmitStorageRecheckEvidence_ConfirmFailRewardsOriginalReporter verifies that
// when a recheck confirms a fail (RECHECK_CONFIRMED_FAIL), the original reporter who
// first submitted the fail receives a -3 recovery credit (LEP6.md §15.3).
func TestMsgSubmitStorageRecheckEvidence_ConfirmFailRewardsOriginalReporter(t *testing.T) {
	f := initFixture(t)
	f.ctx = f.ctx.WithBlockHeight(1).WithEventManager(sdk.NewEventManager())
	ms := keeper.NewMsgServerImpl(f.keeper)

	recheckerAccount := "sn-rechecker-confirm"
	challengedAccount := "sn-challenged-confirm"
	originalReporter := "sn-original-reporter-confirm"

	params := types.DefaultParams()
	params.StorageTruthEnforcementMode = types.StorageTruthEnforcementMode_STORAGE_TRUTH_ENFORCEMENT_MODE_SHADOW
	require.NoError(t, f.keeper.SetParams(f.ctx, params))

	seedEpochAnchorForReportTest(t, f, 0, []string{recheckerAccount, challengedAccount}, []string{recheckerAccount, challengedAccount})

	f.supernodeKeeper.EXPECT().
		GetSuperNodeByAccount(gomock.Any(), recheckerAccount).
		Return(sntypes.SuperNode{SupernodeAccount: recheckerAccount}, true, nil).AnyTimes()
	f.supernodeKeeper.EXPECT().
		GetSuperNodeByAccount(gomock.Any(), challengedAccount).
		Return(sntypes.SuperNode{SupernodeAccount: challengedAccount}, true, nil).AnyTimes()

	// Original reporter starts with reliability score of 10 (some prior issues).
	require.NoError(t, f.keeper.SetReporterReliabilityState(f.ctx, types.ReporterReliabilityState{
		ReporterSupernodeAccount: originalReporter,
		ReliabilityScore:         10,
		LastUpdatedEpoch:         0,
	}))

	seedIndexedChallengeResult(t, f, originalReporter, challengedAccount, "ticket-confirm", "orig-hash")

	// Recheck confirms the fail.
	_, err := ms.SubmitStorageRecheckEvidence(f.ctx, &types.MsgSubmitStorageRecheckEvidence{
		Creator:                        recheckerAccount,
		EpochId:                        0,
		ChallengedSupernodeAccount:     challengedAccount,
		TicketId:                       "ticket-confirm",
		ChallengedResultTranscriptHash: "orig-hash",
		RecheckTranscriptHash:          "recheck-hash",
		RecheckResultClass:             types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_RECHECK_CONFIRMED_FAIL,
	})
	require.NoError(t, err)

	// Original reporter should receive -3 recovery credit: 10 - 3 = 7.
	originalState, found := f.keeper.GetReporterReliabilityState(f.ctx, originalReporter)
	require.True(t, found)
	require.Equal(t, int64(7), originalState.ReliabilityScore)
}

func TestMsgClaimHealComplete_RequiresIndependentVerifierAssignments(t *testing.T) {
	f := initFixture(t)
	f.ctx = f.ctx.WithBlockHeight(1).WithEventManager(sdk.NewEventManager())
	ms := keeper.NewMsgServerImpl(f.keeper)

	healOp := types.HealOp{
		HealOpId:               21,
		TicketId:               "ticket-single-node",
		ScheduledEpochId:       0,
		HealerSupernodeAccount: "sn-healer",
		Status:                 types.HealOpStatus_HEAL_OP_STATUS_SCHEDULED,
		CreatedHeight:          1,
		UpdatedHeight:          1,
		DeadlineEpochId:        1,
	}
	require.NoError(t, f.keeper.SetHealOp(f.ctx, healOp))
	require.NoError(t, f.keeper.SetTicketDeteriorationState(f.ctx, types.TicketDeteriorationState{
		TicketId:       healOp.TicketId,
		ActiveHealOpId: healOp.HealOpId,
	}))

	_, err := ms.ClaimHealComplete(f.ctx, &types.MsgClaimHealComplete{
		Creator:          "sn-healer",
		HealOpId:         healOp.HealOpId,
		TicketId:         healOp.TicketId,
		HealManifestHash: "manifest-single",
		Details:          "single node finalized",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "no independent verifier")

	current, found := f.keeper.GetHealOp(f.ctx, healOp.HealOpId)
	require.True(t, found)
	require.Equal(t, types.HealOpStatus_HEAL_OP_STATUS_SCHEDULED, current.Status)

	ticketState, found := f.keeper.GetTicketDeteriorationState(f.ctx, healOp.TicketId)
	require.True(t, found)
	require.Equal(t, healOp.HealOpId, ticketState.ActiveHealOpId)
}
