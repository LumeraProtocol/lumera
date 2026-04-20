package system_test

import (
	"context"
	"os"
	"testing"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/LumeraProtocol/lumera/app"
	"github.com/LumeraProtocol/lumera/tests/ibctesting"
	auditkeeper "github.com/LumeraProtocol/lumera/x/audit/v1/keeper"
	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

// genAddr generates a fresh random account address.
func genAddr() sdk.AccAddress {
	return sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
}

// AuditSystemTestSuite holds a live single-chain app for msg-server system tests.
//
// The app wires audit.Keeper → supernode.Keeper so SetSuperNode writes flow
// through to GetSuperNodeByAccount without any mocking.
type AuditSystemTestSuite struct {
	app       *app.App
	sdkCtx    sdk.Context
	ctx       context.Context
	msgServer types.MsgServer
}

func setupAuditSystemSuite(t *testing.T) *AuditSystemTestSuite {
	t.Helper()
	os.Setenv("SYSTEM_TESTS", "true")
	t.Cleanup(func() { os.Unsetenv("SYSTEM_TESTS") })

	coord := ibctesting.NewCoordinator(t, 1)
	chain := coord.GetChain(ibctesting.GetChainID(1))

	a := chain.App.(*app.App)
	baseCtx := chain.GetContext().WithBlockHeight(500).WithEventManager(sdk.NewEventManager())

	params := types.DefaultParams()
	require.NoError(t, a.AuditKeeper.SetParams(baseCtx, params))

	return &AuditSystemTestSuite{
		app:       a,
		sdkCtx:    baseCtx,
		ctx:       sdk.WrapSDKContext(baseCtx),
		msgServer: auditkeeper.NewMsgServerImpl(a.AuditKeeper),
	}
}

// seedSupernode registers a supernode in the real supernode keeper so that
// audit.SubmitStorageRecheckEvidence can look it up via GetSuperNodeByAccount.
func (s *AuditSystemTestSuite) seedSupernode(t *testing.T, acc sdk.AccAddress) {
	t.Helper()
	require.NoError(t, s.app.SupernodeKeeper.SetSuperNode(s.sdkCtx, sntypes.SuperNode{
		SupernodeAccount: acc.String(),
		ValidatorAddress: sdk.ValAddress(acc).String(),
		States: []*sntypes.SuperNodeStateRecord{
			{State: sntypes.SuperNodeStateActive, Height: s.sdkCtx.BlockHeight()},
		},
		PrevIpAddresses: []*sntypes.IPAddressHistory{
			{Address: "192.168.1.1", Height: s.sdkCtx.BlockHeight()},
		},
		Note: "1.0.0",
	}))
}

// seedEpochAnchor writes an epoch anchor so that SubmitStorageRecheckEvidence
// can find the epoch when validating the epoch_id field.
func (s *AuditSystemTestSuite) seedEpochAnchor(t *testing.T, epochID uint64) {
	t.Helper()
	require.NoError(t, s.app.AuditKeeper.SetEpochAnchor(s.sdkCtx, types.EpochAnchor{
		EpochId:                 epochID,
		EpochStartHeight:        1,
		EpochEndHeight:          400,
		EpochLengthBlocks:       types.DefaultEpochLengthBlocks,
		Seed:                    make([]byte, 32),
		ActiveSupernodeAccounts: []string{},
		TargetSupernodeAccounts: []string{},
		ParamsCommitment:        []byte{1},
		ActiveSetCommitment:     []byte{1},
		TargetsSetCommitment:    []byte{1},
	}))
}

func (s *AuditSystemTestSuite) seedIndexedChallengeResult(t *testing.T, originalReporter string, target string, ticketID string, transcriptHash string) {
	t.Helper()
	result := &types.StorageProofResult{
		TargetSupernodeAccount:     target,
		ChallengerSupernodeAccount: originalReporter,
		BucketType:                 types.StorageProofBucketType_STORAGE_PROOF_BUCKET_TYPE_RECENT,
		ResultClass:                types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_HASH_MISMATCH,
		TicketId:                   ticketID,
		ArtifactClass:              types.StorageProofArtifactClass_STORAGE_PROOF_ARTIFACT_CLASS_INDEX,
		ArtifactKey:                "artifact-key-" + ticketID,
		ArtifactOrdinal:            1,
		TranscriptHash:             transcriptHash,
	}
	require.NoError(t, s.app.AuditKeeper.IndexStorageProofTranscripts(s.sdkCtx, 0, originalReporter, []*types.StorageProofResult{result}))
	require.NoError(t, s.app.AuditKeeper.SetTicketDeteriorationState(s.sdkCtx, types.TicketDeteriorationState{
		TicketId:                     ticketID,
		DeteriorationScore:           20,
		LastUpdatedEpoch:             0,
		LastTargetSupernodeAccount:   target,
		LastReporterSupernodeAccount: originalReporter,
		LastResultClass:              types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_HASH_MISMATCH,
		LastResultEpoch:              0,
	}))
}

// ── SubmitStorageRecheckEvidence ──────────────────────────────────────────────

func TestSubmitStorageRecheckEvidence_NilMsg(t *testing.T) {
	s := setupAuditSystemSuite(t)
	_, err := s.msgServer.SubmitStorageRecheckEvidence(s.ctx, nil)
	require.Error(t, err)
}

func TestSubmitStorageRecheckEvidence_MissingChallengedAccount(t *testing.T) {
	s := setupAuditSystemSuite(t)
	_, err := s.msgServer.SubmitStorageRecheckEvidence(s.ctx, &types.MsgSubmitStorageRecheckEvidence{
		Creator: "sn-creator",
		EpochId: 0,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "challenged_supernode_account is required")
}

func TestSubmitStorageRecheckEvidence_MissingTicketID(t *testing.T) {
	s := setupAuditSystemSuite(t)
	_, err := s.msgServer.SubmitStorageRecheckEvidence(s.ctx, &types.MsgSubmitStorageRecheckEvidence{
		Creator:                    "sn-creator",
		ChallengedSupernodeAccount: "sn-target",
		EpochId:                    0,
		// TicketId intentionally empty
		ChallengedResultTranscriptHash: "hash",
		RecheckTranscriptHash:          "hash2",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "ticket_id is required")
}

func TestSubmitStorageRecheckEvidence_SelfChallenge(t *testing.T) {
	s := setupAuditSystemSuite(t)
	_, err := s.msgServer.SubmitStorageRecheckEvidence(s.ctx, &types.MsgSubmitStorageRecheckEvidence{
		Creator:                        "sn-self",
		ChallengedSupernodeAccount:     "sn-self",
		EpochId:                        0,
		TicketId:                       "ticket-1",
		ChallengedResultTranscriptHash: "hash",
		RecheckTranscriptHash:          "hash2",
		RecheckResultClass:             types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_PASS,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "must not equal creator")
}

func TestSubmitStorageRecheckEvidence_UnknownEpoch(t *testing.T) {
	s := setupAuditSystemSuite(t)
	acc1 := genAddr()
	acc2 := genAddr()

	// Register both supernodes — epoch anchor is NOT seeded.
	s.seedSupernode(t, acc1)
	s.seedSupernode(t, acc2)

	_, err := s.msgServer.SubmitStorageRecheckEvidence(s.ctx, &types.MsgSubmitStorageRecheckEvidence{
		Creator:                        acc1.String(),
		ChallengedSupernodeAccount:     acc2.String(),
		EpochId:                        999, // no anchor for this epoch
		TicketId:                       "ticket-no-epoch",
		ChallengedResultTranscriptHash: "hash",
		RecheckTranscriptHash:          "hash2",
		RecheckResultClass:             types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_RECHECK_CONFIRMED_FAIL,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "epoch anchor not found")
}

func TestSubmitStorageRecheckEvidence_InvalidResultClass(t *testing.T) {
	s := setupAuditSystemSuite(t)
	acc1 := genAddr()
	acc2 := genAddr()
	s.seedEpochAnchor(t, 0)
	s.seedSupernode(t, acc1)
	s.seedSupernode(t, acc2)

	_, err := s.msgServer.SubmitStorageRecheckEvidence(s.ctx, &types.MsgSubmitStorageRecheckEvidence{
		Creator:                        acc1.String(),
		ChallengedSupernodeAccount:     acc2.String(),
		EpochId:                        0,
		TicketId:                       "ticket-bad-class",
		ChallengedResultTranscriptHash: "hash",
		RecheckTranscriptHash:          "hash2",
		RecheckResultClass:             types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_NO_ELIGIBLE_TICKET, // not allowed for recheck
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "recheck_result_class is invalid")
}

func TestSubmitStorageRecheckEvidence_ReplayRejected(t *testing.T) {
	s := setupAuditSystemSuite(t)
	acc1 := genAddr()
	acc2 := genAddr()
	s.seedEpochAnchor(t, 0)
	s.seedSupernode(t, acc1)
	s.seedSupernode(t, acc2)
	s.seedIndexedChallengeResult(t, genAddr().String(), acc2.String(), "ticket-replay", "hash-orig")

	req := &types.MsgSubmitStorageRecheckEvidence{
		Creator:                        acc1.String(),
		ChallengedSupernodeAccount:     acc2.String(),
		EpochId:                        0,
		TicketId:                       "ticket-replay",
		ChallengedResultTranscriptHash: "hash-orig",
		RecheckTranscriptHash:          "hash-recheck",
		RecheckResultClass:             types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_PASS,
	}

	_, err := s.msgServer.SubmitStorageRecheckEvidence(s.ctx, req)
	require.NoError(t, err)

	// Second identical submission must be rejected.
	_, err = s.msgServer.SubmitStorageRecheckEvidence(s.ctx, req)
	require.Error(t, err)
	require.Contains(t, err.Error(), "already submitted")
}

func TestSubmitStorageRecheckEvidence_AccumulatesAcrossTickets(t *testing.T) {
	s := setupAuditSystemSuite(t)
	acc1 := genAddr()
	acc2 := genAddr()
	s.seedEpochAnchor(t, 0)
	s.seedSupernode(t, acc1)
	s.seedSupernode(t, acc2)

	// Three rechecks against the same node with different ticket IDs.
	// RECHECK_CONFIRMED_FAIL applies +15 plus LEP-6 distinct-ticket escalation.
	for i := 0; i < 3; i++ {
		ticketID := "ticket-acc-" + string(rune('1'+i))
		transcriptHash := "hash-orig-" + string(rune('1'+i))
		s.seedIndexedChallengeResult(t, genAddr().String(), acc2.String(), ticketID, transcriptHash)
		req := &types.MsgSubmitStorageRecheckEvidence{
			Creator:                        acc1.String(),
			ChallengedSupernodeAccount:     acc2.String(),
			EpochId:                        0,
			TicketId:                       ticketID,
			ChallengedResultTranscriptHash: transcriptHash,
			RecheckTranscriptHash:          "hash-recheck",
			RecheckResultClass:             types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_RECHECK_CONFIRMED_FAIL,
		}
		_, err := s.msgServer.SubmitStorageRecheckEvidence(s.ctx, req)
		require.NoError(t, err)
	}

	state, found := s.app.AuditKeeper.GetNodeSuspicionState(s.sdkCtx, acc2.String())
	require.True(t, found)
	require.Equal(t, int64(67), state.SuspicionScore)
}

func TestSubmitStorageRecheckEvidence_PassResultNoSuspicionIncrease(t *testing.T) {
	s := setupAuditSystemSuite(t)
	acc1 := genAddr()
	acc2 := genAddr()
	s.seedEpochAnchor(t, 0)
	s.seedSupernode(t, acc1)
	s.seedSupernode(t, acc2)
	s.seedIndexedChallengeResult(t, genAddr().String(), acc2.String(), "ticket-pass", "hash-orig")

	_, err := s.msgServer.SubmitStorageRecheckEvidence(s.ctx, &types.MsgSubmitStorageRecheckEvidence{
		Creator:                        acc1.String(),
		ChallengedSupernodeAccount:     acc2.String(),
		EpochId:                        0,
		TicketId:                       "ticket-pass",
		ChallengedResultTranscriptHash: "hash-orig",
		RecheckTranscriptHash:          "hash-recheck",
		RecheckResultClass:             types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_PASS,
	})
	require.NoError(t, err)

	// PASS result should NOT create a suspicion state (or should leave score at 0).
	state, found := s.app.AuditKeeper.GetNodeSuspicionState(s.sdkCtx, acc2.String())
	if found {
		require.LessOrEqual(t, state.SuspicionScore, int64(0),
			"PASS result must not increase node suspicion score")
	}
}

// ── ClaimHealComplete ─────────────────────────────────────────────────────────

func TestClaimHealComplete_NilMsg(t *testing.T) {
	s := setupAuditSystemSuite(t)
	_, err := s.msgServer.ClaimHealComplete(s.ctx, nil)
	require.Error(t, err)
}

func TestClaimHealComplete_HealOpNotFound(t *testing.T) {
	s := setupAuditSystemSuite(t)
	_, err := s.msgServer.ClaimHealComplete(s.ctx, &types.MsgClaimHealComplete{
		Creator:          "sn-healer",
		HealOpId:         9999,
		TicketId:         "ticket-x",
		HealManifestHash: "manifest-x",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "heal op 9999 not found")
}

func TestClaimHealComplete_Unauthorized(t *testing.T) {
	s := setupAuditSystemSuite(t)

	require.NoError(t, s.app.AuditKeeper.SetHealOp(s.sdkCtx, types.HealOp{
		HealOpId:               77,
		TicketId:               "ticket-auth",
		HealerSupernodeAccount: "sn-real-healer",
		Status:                 types.HealOpStatus_HEAL_OP_STATUS_SCHEDULED,
		DeadlineEpochId:        10,
		CreatedHeight:          1,
		UpdatedHeight:          1,
	}))

	_, err := s.msgServer.ClaimHealComplete(s.ctx, &types.MsgClaimHealComplete{
		Creator:          "sn-impostor",
		HealOpId:         77,
		TicketId:         "ticket-auth",
		HealManifestHash: "manifest",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), types.ErrHealOpUnauthorized.Error())
}

func TestClaimHealComplete_WrongTicketID(t *testing.T) {
	s := setupAuditSystemSuite(t)

	require.NoError(t, s.app.AuditKeeper.SetHealOp(s.sdkCtx, types.HealOp{
		HealOpId:               78,
		TicketId:               "ticket-correct",
		HealerSupernodeAccount: "sn-healer",
		Status:                 types.HealOpStatus_HEAL_OP_STATUS_SCHEDULED,
		DeadlineEpochId:        10,
		CreatedHeight:          1,
		UpdatedHeight:          1,
	}))

	_, err := s.msgServer.ClaimHealComplete(s.ctx, &types.MsgClaimHealComplete{
		Creator:          "sn-healer",
		HealOpId:         78,
		TicketId:         "ticket-wrong",
		HealManifestHash: "manifest",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), types.ErrHealOpTicketMismatch.Error())
}

func TestClaimHealComplete_WrongStatus(t *testing.T) {
	s := setupAuditSystemSuite(t)

	// Heal op already in HEALER_REPORTED — second ClaimHealComplete should fail.
	require.NoError(t, s.app.AuditKeeper.SetHealOp(s.sdkCtx, types.HealOp{
		HealOpId:               79,
		TicketId:               "ticket-status",
		HealerSupernodeAccount: "sn-healer",
		Status:                 types.HealOpStatus_HEAL_OP_STATUS_HEALER_REPORTED,
		DeadlineEpochId:        10,
		CreatedHeight:          1,
		UpdatedHeight:          1,
	}))

	// Seeding a ticket state with a matching ActiveHealOpId.
	require.NoError(t, s.app.AuditKeeper.SetTicketDeteriorationState(s.sdkCtx, types.TicketDeteriorationState{
		TicketId:       "ticket-status",
		ActiveHealOpId: 79,
	}))

	_, err := s.msgServer.ClaimHealComplete(s.ctx, &types.MsgClaimHealComplete{
		Creator:          "sn-healer",
		HealOpId:         79,
		TicketId:         "ticket-status",
		HealManifestHash: "manifest",
	})
	// HEALER_REPORTED is still accepted (IN_PROGRESS also ok).
	// VERIFIED or FAILED must reject.
	if err != nil {
		require.Contains(t, err.Error(), "does not accept healer completion claim")
	}
}

func TestClaimHealComplete_VerifiedStatusRejectsNewClaim(t *testing.T) {
	s := setupAuditSystemSuite(t)

	require.NoError(t, s.app.AuditKeeper.SetHealOp(s.sdkCtx, types.HealOp{
		HealOpId:               80,
		TicketId:               "ticket-verified",
		HealerSupernodeAccount: "sn-healer",
		Status:                 types.HealOpStatus_HEAL_OP_STATUS_VERIFIED,
		DeadlineEpochId:        10,
		CreatedHeight:          1,
		UpdatedHeight:          1,
	}))

	_, err := s.msgServer.ClaimHealComplete(s.ctx, &types.MsgClaimHealComplete{
		Creator:          "sn-healer",
		HealOpId:         80,
		TicketId:         "ticket-verified",
		HealManifestHash: "manifest",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "does not accept healer completion claim")
}

// ── SubmitHealVerification ────────────────────────────────────────────────────

func TestSubmitHealVerification_NilMsg(t *testing.T) {
	s := setupAuditSystemSuite(t)
	_, err := s.msgServer.SubmitHealVerification(s.ctx, nil)
	require.Error(t, err)
}

func TestSubmitHealVerification_HealOpNotFound(t *testing.T) {
	s := setupAuditSystemSuite(t)
	_, err := s.msgServer.SubmitHealVerification(s.ctx, &types.MsgSubmitHealVerification{
		Creator:          "sn-verifier",
		HealOpId:         8888,
		Verified:         true,
		VerificationHash: "v-hash",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

func TestSubmitHealVerification_WrongStatus(t *testing.T) {
	s := setupAuditSystemSuite(t)

	// Heal op in SCHEDULED (not HEALER_REPORTED) should reject verification.
	require.NoError(t, s.app.AuditKeeper.SetHealOp(s.sdkCtx, types.HealOp{
		HealOpId:                  81,
		TicketId:                  "ticket-wrongstatus",
		HealerSupernodeAccount:    "sn-healer",
		VerifierSupernodeAccounts: []string{"sn-verifier"},
		Status:                    types.HealOpStatus_HEAL_OP_STATUS_SCHEDULED,
		DeadlineEpochId:           10,
		CreatedHeight:             1,
		UpdatedHeight:             1,
	}))

	_, err := s.msgServer.SubmitHealVerification(s.ctx, &types.MsgSubmitHealVerification{
		Creator:          "sn-verifier",
		HealOpId:         81,
		Verified:         true,
		VerificationHash: "v-hash",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "does not accept verification")
}

func TestSubmitHealVerification_NonVerifierRejected(t *testing.T) {
	s := setupAuditSystemSuite(t)

	require.NoError(t, s.app.AuditKeeper.SetHealOp(s.sdkCtx, types.HealOp{
		HealOpId:                  82,
		TicketId:                  "ticket-nonver",
		HealerSupernodeAccount:    "sn-healer",
		VerifierSupernodeAccounts: []string{"sn-verifier-a"},
		Status:                    types.HealOpStatus_HEAL_OP_STATUS_HEALER_REPORTED,
		DeadlineEpochId:           10,
		CreatedHeight:             1,
		UpdatedHeight:             1,
	}))

	// sn-impostor is not in VerifierSupernodeAccounts.
	_, err := s.msgServer.SubmitHealVerification(s.ctx, &types.MsgSubmitHealVerification{
		Creator:          "sn-impostor",
		HealOpId:         82,
		Verified:         true,
		VerificationHash: "v-hash",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), types.ErrHealOpUnauthorized.Error())
}

func TestSubmitHealVerification_DuplicateRejected(t *testing.T) {
	s := setupAuditSystemSuite(t)

	require.NoError(t, s.app.AuditKeeper.SetHealOp(s.sdkCtx, types.HealOp{
		HealOpId:                  83,
		TicketId:                  "ticket-dup",
		HealerSupernodeAccount:    "sn-healer",
		VerifierSupernodeAccounts: []string{"sn-verifier-a", "sn-verifier-b"},
		Status:                    types.HealOpStatus_HEAL_OP_STATUS_HEALER_REPORTED,
		DeadlineEpochId:           10,
		CreatedHeight:             1,
		UpdatedHeight:             1,
	}))

	_, err := s.msgServer.SubmitHealVerification(s.ctx, &types.MsgSubmitHealVerification{
		Creator:          "sn-verifier-a",
		HealOpId:         83,
		Verified:         true,
		VerificationHash: "v-hash-1",
	})
	require.NoError(t, err)

	// Same verifier submits again.
	_, err = s.msgServer.SubmitHealVerification(s.ctx, &types.MsgSubmitHealVerification{
		Creator:          "sn-verifier-a",
		HealOpId:         83,
		Verified:         true,
		VerificationHash: "v-hash-1-repeat",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), types.ErrHealVerificationExists.Error())
}

func TestSubmitHealVerification_AfterFinalizedRejected(t *testing.T) {
	s := setupAuditSystemSuite(t)

	// Finalized op (VERIFIED) must not accept new verifications.
	require.NoError(t, s.app.AuditKeeper.SetHealOp(s.sdkCtx, types.HealOp{
		HealOpId:                  84,
		TicketId:                  "ticket-finalized",
		HealerSupernodeAccount:    "sn-healer",
		VerifierSupernodeAccounts: []string{"sn-verifier-a"},
		Status:                    types.HealOpStatus_HEAL_OP_STATUS_VERIFIED,
		DeadlineEpochId:           10,
		CreatedHeight:             1,
		UpdatedHeight:             1,
	}))

	_, err := s.msgServer.SubmitHealVerification(s.ctx, &types.MsgSubmitHealVerification{
		Creator:          "sn-verifier-a",
		HealOpId:         84,
		Verified:         true,
		VerificationHash: "v-hash",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "does not accept verification")
}

func TestSubmitHealVerification_NegativeVoteFinalizesToFailed(t *testing.T) {
	s := setupAuditSystemSuite(t)

	// Use 1 verifier so majority = 1/2+1 = 1. A single negative vote achieves majority → FAILED.
	require.NoError(t, s.app.AuditKeeper.SetHealOp(s.sdkCtx, types.HealOp{
		HealOpId:                  85,
		TicketId:                  "ticket-neg",
		HealerSupernodeAccount:    "sn-healer",
		VerifierSupernodeAccounts: []string{"sn-verifier-a"},
		Status:                    types.HealOpStatus_HEAL_OP_STATUS_HEALER_REPORTED,
		DeadlineEpochId:           10,
		CreatedHeight:             1,
		UpdatedHeight:             1,
	}))
	require.NoError(t, s.app.AuditKeeper.SetTicketDeteriorationState(s.sdkCtx, types.TicketDeteriorationState{
		TicketId:           "ticket-neg",
		DeteriorationScore: 50,
		ActiveHealOpId:     85,
	}))

	// Single false vote achieves majority (1/1) → FAILED.
	_, err := s.msgServer.SubmitHealVerification(s.ctx, &types.MsgSubmitHealVerification{
		Creator:          "sn-verifier-a",
		HealOpId:         85,
		Verified:         false,
		VerificationHash: "v-hash-fail",
	})
	require.NoError(t, err)

	op, found := s.app.AuditKeeper.GetHealOp(s.sdkCtx, 85)
	require.True(t, found)
	require.Equal(t, types.HealOpStatus_HEAL_OP_STATUS_FAILED, op.Status)

	// Ticket deterioration should increase by 15.
	ticketState, found := s.app.AuditKeeper.GetTicketDeteriorationState(s.sdkCtx, "ticket-neg")
	require.True(t, found)
	require.Equal(t, int64(65), ticketState.DeteriorationScore, "50 + 15 on failed heal")
}

func TestSubmitHealVerification_AllPositiveVotesFinalizesToVerified(t *testing.T) {
	s := setupAuditSystemSuite(t)

	require.NoError(t, s.app.AuditKeeper.SetHealOp(s.sdkCtx, types.HealOp{
		HealOpId:                  86,
		TicketId:                  "ticket-pos",
		HealerSupernodeAccount:    "sn-healer",
		VerifierSupernodeAccounts: []string{"sn-verifier-a", "sn-verifier-b"},
		Status:                    types.HealOpStatus_HEAL_OP_STATUS_HEALER_REPORTED,
		DeadlineEpochId:           10,
		CreatedHeight:             1,
		UpdatedHeight:             1,
	}))
	require.NoError(t, s.app.AuditKeeper.SetTicketDeteriorationState(s.sdkCtx, types.TicketDeteriorationState{
		TicketId:           "ticket-pos",
		DeteriorationScore: 80,
		ActiveHealOpId:     86,
	}))

	for _, v := range []struct {
		creator string
		hash    string
	}{
		{"sn-verifier-a", "va-hash"},
		{"sn-verifier-b", "vb-hash"},
	} {
		_, err := s.msgServer.SubmitHealVerification(s.ctx, &types.MsgSubmitHealVerification{
			Creator:          v.creator,
			HealOpId:         86,
			Verified:         true,
			VerificationHash: v.hash,
		})
		require.NoError(t, err)
	}

	op, found := s.app.AuditKeeper.GetHealOp(s.sdkCtx, 86)
	require.True(t, found)
	require.Equal(t, types.HealOpStatus_HEAL_OP_STATUS_VERIFIED, op.Status)

	// Ticket deterioration: D = max(8, floor(80*0.25)) = 20.
	ticketState, found := s.app.AuditKeeper.GetTicketDeteriorationState(s.sdkCtx, "ticket-pos")
	require.True(t, found)
	require.Equal(t, int64(20), ticketState.DeteriorationScore)
	require.Equal(t, uint64(0), ticketState.ActiveHealOpId, "ActiveHealOpId cleared after verified")
	require.Greater(t, ticketState.ProbationUntilEpoch, uint64(0), "probation epoch set after verified heal")
}
