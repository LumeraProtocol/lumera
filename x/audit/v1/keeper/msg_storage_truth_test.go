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

func TestMsgSubmitStorageRecheckEvidence(t *testing.T) {
	f := initFixture(t)
	f.ctx = f.ctx.WithBlockHeight(1).WithEventManager(sdk.NewEventManager())
	ms := keeper.NewMsgServerImpl(f.keeper)

	creator := "sn-aaa-rechecker"
	challenged := "sn-bbb-target"

	f.supernodeKeeper.EXPECT().
		GetSuperNodeByAccount(gomock.Any(), creator).
		Return(sntypes.SuperNode{}, true, nil).
		AnyTimes()
	f.supernodeKeeper.EXPECT().
		GetSuperNodeByAccount(gomock.Any(), challenged).
		Return(sntypes.SuperNode{}, true, nil).
		AnyTimes()

	seedEpochAnchorForReportTest(t, f, 0, []string{creator, challenged}, []string{creator, challenged})

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
	require.Contains(t, err.Error(), types.ErrNotImplemented.Error())
	require.Contains(t, err.Error(), "not active")

	nodeState, found := f.keeper.GetNodeSuspicionState(f.ctx, challenged)
	require.False(t, found)
	require.Equal(t, types.NodeSuspicionState{}, nodeState)

	reporterState, found := f.keeper.GetReporterReliabilityState(f.ctx, creator)
	require.False(t, found)
	require.Equal(t, types.ReporterReliabilityState{}, reporterState)

	ticketState, found := f.keeper.GetTicketDeteriorationState(f.ctx, "ticket-1")
	require.False(t, found)
	require.Equal(t, types.TicketDeteriorationState{}, ticketState)
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

	_, err = ms.SubmitHealVerification(f.ctx, &types.MsgSubmitHealVerification{
		Creator:          "sn-verifier-a",
		HealOpId:         healOp.HealOpId,
		Verified:         true,
		VerificationHash: "verify-1",
	})
	require.NoError(t, err)

	inFlight, found := f.keeper.GetHealOp(f.ctx, healOp.HealOpId)
	require.True(t, found)
	require.Equal(t, types.HealOpStatus_HEAL_OP_STATUS_HEALER_REPORTED, inFlight.Status)

	_, err = ms.SubmitHealVerification(f.ctx, &types.MsgSubmitHealVerification{
		Creator:          "sn-verifier-a",
		HealOpId:         healOp.HealOpId,
		Verified:         true,
		VerificationHash: "verify-1-repeat",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), types.ErrHealVerificationExists.Error())

	_, err = ms.SubmitHealVerification(f.ctx, &types.MsgSubmitHealVerification{
		Creator:          "sn-verifier-b",
		HealOpId:         healOp.HealOpId,
		Verified:         true,
		VerificationHash: "verify-2",
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

	healOp := types.HealOp{
		HealOpId:                  12,
		TicketId:                  "ticket-12",
		ScheduledEpochId:          0,
		HealerSupernodeAccount:    "sn-healer",
		VerifierSupernodeAccounts: []string{"sn-verifier-a", "sn-verifier-b"},
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

	_, err := ms.SubmitHealVerification(f.ctx, &types.MsgSubmitHealVerification{
		Creator:          "sn-verifier-a",
		HealOpId:         healOp.HealOpId,
		Verified:         false,
		VerificationHash: "verify-fail",
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

func TestMsgClaimHealComplete_SingleNodeFinalizesImmediately(t *testing.T) {
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
	require.NoError(t, err)

	finalized, found := f.keeper.GetHealOp(f.ctx, healOp.HealOpId)
	require.True(t, found)
	require.Equal(t, types.HealOpStatus_HEAL_OP_STATUS_VERIFIED, finalized.Status)
	require.Equal(t, "manifest-single", finalized.ResultHash)
	require.Contains(t, finalized.Notes, "single node finalized")

	ticketState, found := f.keeper.GetTicketDeteriorationState(f.ctx, healOp.TicketId)
	require.True(t, found)
	require.Equal(t, uint64(0), ticketState.ActiveHealOpId)
	require.Equal(t, uint64(0), ticketState.LastHealEpoch)
	require.Equal(t, uint64(types.DefaultStorageTruthProbationEpochs), ticketState.ProbationUntilEpoch)
}
