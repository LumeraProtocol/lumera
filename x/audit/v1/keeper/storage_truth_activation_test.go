package keeper_test

import (
	"testing"

	"github.com/LumeraProtocol/lumera/testutil/crypto"
	"github.com/LumeraProtocol/lumera/x/audit/v1/keeper"
	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func makeActiveSupernode(t *testing.T) (sntypes.SuperNode, sdk.AccAddress, sdk.ValAddress) {
	t.Helper()
	_, accAddr, valAddr := cryptotestutils.SupernodeAddresses()
	sn := sntypes.SuperNode{
		SupernodeAccount: accAddr.String(),
		ValidatorAddress: sdk.ValAddress(valAddr).String(),
	}
	return sn, accAddr, valAddr
}

func setNodeSuspicion(t *testing.T, f *fixture, account string, score int64, epochID uint64) {
	t.Helper()
	err := f.keeper.SetNodeSuspicionState(f.ctx, types.NodeSuspicionState{
		SupernodeAccount: account,
		SuspicionScore:   score,
		LastUpdatedEpoch: epochID,
		// Preset predicate fields so postpone predicates are satisfied.
		ClassACountWindow: 1,
		ClassBCountWindow: 1,
	})
	require.NoError(t, err)
}

func setTicketDeterioration(t *testing.T, f *fixture, ticketID string, score int64, epochID uint64) {
	t.Helper()
	err := f.keeper.SetTicketDeteriorationState(f.ctx, types.TicketDeteriorationState{
		TicketId:           ticketID,
		DeteriorationScore: score,
		LastUpdatedEpoch:   epochID,
		// Preset eligibility predicates: 2 recent failures satisfies the heal eligibility check.
		RecentFailureEpochCount: 2,
	})
	require.NoError(t, err)
}

func submitSelfReport(t *testing.T, f *fixture, account string, epochID uint64) {
	t.Helper()
	err := f.keeper.SetReport(f.ctx, types.EpochReport{
		SupernodeAccount: account,
		EpochId:          epochID,
		ReportHeight:     f.ctx.BlockHeight(),
		HostReport:       types.HostReport{},
	})
	require.NoError(t, err)
}

// ---------------------------------------------------------------------------
// Enforcement mode gate
// ---------------------------------------------------------------------------

func TestStorageTruth_UnspecifiedModeSkipsSchedulingAndEnforcement(t *testing.T) {
	f := initFixture(t)
	sn, _, valAddr := makeActiveSupernode(t)

	params := types.DefaultParams()
	params.StorageTruthEnforcementMode = types.StorageTruthEnforcementMode_STORAGE_TRUTH_ENFORCEMENT_MODE_UNSPECIFIED
	params.StorageTruthMaxSelfHealOpsPerEpoch = 5
	params.StorageTruthTicketDeteriorationHealThreshold = 10

	// Set high suspicion — should NOT trigger postpone in UNSPECIFIED mode.
	setNodeSuspicion(t, f, sn.SupernodeAccount, 999, 0)
	// Set high deterioration — should NOT trigger heal-op scheduling.
	setTicketDeterioration(t, f, "ticket-1", 999, 0)
	submitSelfReport(t, f, sn.SupernodeAccount, 0)

	f.supernodeKeeper.EXPECT().
		GetAllSuperNodes(gomock.AssignableToTypeOf(f.ctx), sntypes.SuperNodeStateActive).
		Return([]sntypes.SuperNode{sn}, nil)
	f.supernodeKeeper.EXPECT().
		GetAllSuperNodes(gomock.AssignableToTypeOf(f.ctx), sntypes.SuperNodeStatePostponed).
		Return([]sntypes.SuperNode{}, nil)
	// SetSuperNodePostponed must NOT be called.
	f.supernodeKeeper.EXPECT().
		SetSuperNodePostponed(gomock.Any(), sdk.ValAddress(valAddr), gomock.Any()).
		Times(0)

	require.NoError(t, f.keeper.EnforceEpochEnd(f.ctx, 0, params))

	// No heal ops scheduled.
	require.NoError(t, f.keeper.ProcessStorageTruthHealOpsAtEpochEnd(f.ctx, 0, params))
	healOps, err := f.keeper.GetAllHealOps(f.ctx)
	require.NoError(t, err)
	require.Empty(t, healOps)
}

// ---------------------------------------------------------------------------
// Shadow mode: events only, no postpone
// ---------------------------------------------------------------------------

func TestStorageTruth_ShadowModeEmitsEventsButDoesNotPostpone(t *testing.T) {
	f := initFixture(t)
	sn, _, valAddr := makeActiveSupernode(t)

	params := types.DefaultParams()
	params.StorageTruthEnforcementMode = types.StorageTruthEnforcementMode_STORAGE_TRUTH_ENFORCEMENT_MODE_SHADOW
	params.StorageTruthNodeSuspicionThresholdPostpone = 50
	params.ConsecutiveEpochsToPostpone = 99 // disable legacy postpone

	setNodeSuspicion(t, f, sn.SupernodeAccount, 200, 0)
	submitSelfReport(t, f, sn.SupernodeAccount, 0)

	f.supernodeKeeper.EXPECT().
		GetAllSuperNodes(gomock.AssignableToTypeOf(f.ctx), sntypes.SuperNodeStateActive).
		Return([]sntypes.SuperNode{sn}, nil)
	f.supernodeKeeper.EXPECT().
		GetAllSuperNodes(gomock.AssignableToTypeOf(f.ctx), sntypes.SuperNodeStatePostponed).
		Return([]sntypes.SuperNode{}, nil)
	// SetSuperNodePostponed must NOT be called in shadow mode.
	f.supernodeKeeper.EXPECT().
		SetSuperNodePostponed(gomock.Any(), sdk.ValAddress(valAddr), gomock.Any()).
		Times(0)

	require.NoError(t, f.keeper.EnforceEpochEnd(f.ctx, 0, params))
}

// ---------------------------------------------------------------------------
// Soft/Full mode: postpone on suspicion threshold
// ---------------------------------------------------------------------------

func TestStorageTruth_SoftModePostponesOnSuspicionThreshold(t *testing.T) {
	f := initFixture(t)
	sn, _, valAddr := makeActiveSupernode(t)

	params := types.DefaultParams()
	params.StorageTruthEnforcementMode = types.StorageTruthEnforcementMode_STORAGE_TRUTH_ENFORCEMENT_MODE_SOFT
	params.StorageTruthNodeSuspicionThresholdPostpone = 50
	params.ConsecutiveEpochsToPostpone = 99

	setNodeSuspicion(t, f, sn.SupernodeAccount, 100, 0)
	submitSelfReport(t, f, sn.SupernodeAccount, 0)

	f.supernodeKeeper.EXPECT().
		GetAllSuperNodes(gomock.AssignableToTypeOf(f.ctx), sntypes.SuperNodeStateActive).
		Return([]sntypes.SuperNode{sn}, nil)
	f.supernodeKeeper.EXPECT().
		GetAllSuperNodes(gomock.AssignableToTypeOf(f.ctx), sntypes.SuperNodeStatePostponed).
		Return([]sntypes.SuperNode{}, nil)
	f.supernodeKeeper.EXPECT().
		SetSuperNodePostponed(gomock.AssignableToTypeOf(f.ctx), sdk.ValAddress(valAddr), "audit_storage_truth_suspicion").
		Return(nil).
		Times(1)

	require.NoError(t, f.keeper.EnforceEpochEnd(f.ctx, 0, params))
}

func TestStorageTruth_FullModePostponesOnSuspicionThreshold(t *testing.T) {
	f := initFixture(t)
	sn, _, valAddr := makeActiveSupernode(t)

	params := types.DefaultParams()
	params.StorageTruthEnforcementMode = types.StorageTruthEnforcementMode_STORAGE_TRUTH_ENFORCEMENT_MODE_FULL
	params.StorageTruthNodeSuspicionThresholdPostpone = 50
	params.ConsecutiveEpochsToPostpone = 99

	setNodeSuspicion(t, f, sn.SupernodeAccount, 75, 0)
	submitSelfReport(t, f, sn.SupernodeAccount, 0)

	f.supernodeKeeper.EXPECT().
		GetAllSuperNodes(gomock.AssignableToTypeOf(f.ctx), sntypes.SuperNodeStateActive).
		Return([]sntypes.SuperNode{sn}, nil)
	f.supernodeKeeper.EXPECT().
		GetAllSuperNodes(gomock.AssignableToTypeOf(f.ctx), sntypes.SuperNodeStatePostponed).
		Return([]sntypes.SuperNode{}, nil)
	f.supernodeKeeper.EXPECT().
		SetSuperNodePostponed(gomock.AssignableToTypeOf(f.ctx), sdk.ValAddress(valAddr), "audit_storage_truth_suspicion").
		Return(nil).
		Times(1)

	require.NoError(t, f.keeper.EnforceEpochEnd(f.ctx, 0, params))
}

func TestStorageTruth_BelowPostponeThresholdDoesNotPostpone(t *testing.T) {
	f := initFixture(t)
	sn, _, valAddr := makeActiveSupernode(t)

	params := types.DefaultParams()
	params.StorageTruthEnforcementMode = types.StorageTruthEnforcementMode_STORAGE_TRUTH_ENFORCEMENT_MODE_SOFT
	params.StorageTruthNodeSuspicionThresholdPostpone = 50
	params.ConsecutiveEpochsToPostpone = 99

	setNodeSuspicion(t, f, sn.SupernodeAccount, 30, 0)
	submitSelfReport(t, f, sn.SupernodeAccount, 0)

	f.supernodeKeeper.EXPECT().
		GetAllSuperNodes(gomock.AssignableToTypeOf(f.ctx), sntypes.SuperNodeStateActive).
		Return([]sntypes.SuperNode{sn}, nil)
	f.supernodeKeeper.EXPECT().
		GetAllSuperNodes(gomock.AssignableToTypeOf(f.ctx), sntypes.SuperNodeStatePostponed).
		Return([]sntypes.SuperNode{}, nil)
	f.supernodeKeeper.EXPECT().
		SetSuperNodePostponed(gomock.Any(), sdk.ValAddress(valAddr), gomock.Any()).
		Times(0)

	require.NoError(t, f.keeper.EnforceEpochEnd(f.ctx, 0, params))
}

// ---------------------------------------------------------------------------
// Recovery: storage-truth postponed node recovers when score decays below watch
// ---------------------------------------------------------------------------

func TestStorageTruth_RecoveryWhenScoreDecaysBelowWatchThreshold(t *testing.T) {
	f := initFixture(t)
	sn, _, valAddr := makeActiveSupernode(t)

	params := types.DefaultParams()
	params.StorageTruthEnforcementMode = types.StorageTruthEnforcementMode_STORAGE_TRUTH_ENFORCEMENT_MODE_SOFT
	params.StorageTruthNodeSuspicionThresholdWatch = 20
	params.StorageTruthNodeSuspicionThresholdPostpone = 50
	// Use exponential decay factor: 920 (0.92/epoch). score=200 decays over ~20 epochs below 20.
	// At factor=920: 200→184→169→155→143→131→120→110→101→93→85→78→71→65→59→54→49→45→41→37→34...
	// After ~24 epochs: below 20. Let's use 30 epochs to be safe.
	params.StorageTruthNodeSuspicionDecayPerEpoch = 920
	params.StorageTruthRecoveryCleanPassCount = 3
	// Per F121-F12 — strong-postpone recovery uses StrongRecoveryCleanPassCount.
	// Score=200 hits the strong band, so the strong-recovery threshold applies.
	params.StorageTruthStrongRecoveryCleanPassCount = 3
	params.ConsecutiveEpochsToPostpone = 99

	// Epoch 0: suspicion=200 hits StrongPostpone band (threshold=140).
	// StrongPostpone predicate requires ClassACountWindow >= 2 || LastIndexFailEpoch > 0.
	// Set ClassACountWindow=2 to satisfy the strong_postpone predicate.
	err := f.keeper.SetNodeSuspicionState(f.ctx, types.NodeSuspicionState{
		SupernodeAccount:  sn.SupernodeAccount,
		SuspicionScore:    200,
		LastUpdatedEpoch:  0,
		ClassACountWindow: 2,
		ClassBCountWindow: 1,
		CleanPassCount:    5, // sufficient clean passes for recovery
	})
	require.NoError(t, err)
	submitSelfReport(t, f, sn.SupernodeAccount, 0)

	f.supernodeKeeper.EXPECT().
		GetAllSuperNodes(gomock.AssignableToTypeOf(f.ctx), sntypes.SuperNodeStateActive).
		Return([]sntypes.SuperNode{sn}, nil)
	f.supernodeKeeper.EXPECT().
		GetAllSuperNodes(gomock.AssignableToTypeOf(f.ctx), sntypes.SuperNodeStatePostponed).
		Return([]sntypes.SuperNode{}, nil)
	f.supernodeKeeper.EXPECT().
		// Per F121-F12 — score=200 → STRONG (default strongThr=140).
		SetSuperNodePostponed(gomock.AssignableToTypeOf(f.ctx), sdk.ValAddress(valAddr), "audit_storage_truth_strong_suspicion").
		Return(nil).Times(1)

	require.NoError(t, f.keeper.EnforceEpochEnd(f.ctx, 0, params))

	// Per 121-F8: recovery uses delta = CleanPassCount - CleanPassCountAtPostpone >= required(3).
	// CleanPassCountAtPostpone was snapshotted to 5 at postpone; simulate clean passes accruing.
	{
		state, found := f.keeper.GetNodeSuspicionState(f.ctx, sn.SupernodeAccount)
		require.True(t, found)
		state.CleanPassCount = state.CleanPassCountAtPostpone + 3 // delta=3 >= required=3
		require.NoError(t, f.keeper.SetNodeSuspicionState(f.ctx, state))
	}

	// Epoch 30: after 30 epochs at 0.92/epoch: 200 * (0.92^30) ≈ 200 * 0.0816 ≈ 16 < watch(20) → recovery.
	// clean_pass_count delta=3 >= required=3 → recovery allowed.
	f.supernodeKeeper.EXPECT().
		GetAllSuperNodes(gomock.AssignableToTypeOf(f.ctx), sntypes.SuperNodeStateActive).
		Return([]sntypes.SuperNode{}, nil)
	f.supernodeKeeper.EXPECT().
		GetAllSuperNodes(gomock.AssignableToTypeOf(f.ctx), sntypes.SuperNodeStatePostponed).
		Return([]sntypes.SuperNode{sn}, nil)
	f.supernodeKeeper.EXPECT().
		RecoverSuperNodeFromPostponed(gomock.AssignableToTypeOf(f.ctx), sdk.ValAddress(valAddr)).
		Return(nil).Times(1)

	require.NoError(t, f.keeper.EnforceEpochEnd(f.ctx, 30, params))
}

func TestStorageTruth_NoRecoveryWhileScoreStillAboveWatch(t *testing.T) {
	f := initFixture(t)
	sn, _, valAddr := makeActiveSupernode(t)

	params := types.DefaultParams()
	params.StorageTruthEnforcementMode = types.StorageTruthEnforcementMode_STORAGE_TRUTH_ENFORCEMENT_MODE_SOFT
	params.StorageTruthNodeSuspicionThresholdWatch = 20
	params.StorageTruthNodeSuspicionThresholdPostpone = 50
	// Use exponential decay: 920 (0.92/epoch). 60 * 0.92^5 = 60 * 0.659 ≈ 39 > watch(20).
	params.StorageTruthNodeSuspicionDecayPerEpoch = 920
	params.StorageTruthRecoveryCleanPassCount = 3
	params.ConsecutiveEpochsToPostpone = 99

	// Postpone at epoch 0.
	setNodeSuspicion(t, f, sn.SupernodeAccount, 60, 0)
	submitSelfReport(t, f, sn.SupernodeAccount, 0)

	f.supernodeKeeper.EXPECT().GetAllSuperNodes(gomock.AssignableToTypeOf(f.ctx), sntypes.SuperNodeStateActive).Return([]sntypes.SuperNode{sn}, nil)
	f.supernodeKeeper.EXPECT().GetAllSuperNodes(gomock.AssignableToTypeOf(f.ctx), sntypes.SuperNodeStatePostponed).Return([]sntypes.SuperNode{}, nil)
	f.supernodeKeeper.EXPECT().SetSuperNodePostponed(gomock.AssignableToTypeOf(f.ctx), sdk.ValAddress(valAddr), "audit_storage_truth_suspicion").Return(nil)
	require.NoError(t, f.keeper.EnforceEpochEnd(f.ctx, 0, params))

	// Epoch 5: score = 60 * 0.92^5 ≈ 39 > watch(20) → no recovery.
	f.supernodeKeeper.EXPECT().GetAllSuperNodes(gomock.AssignableToTypeOf(f.ctx), sntypes.SuperNodeStateActive).Return([]sntypes.SuperNode{}, nil)
	f.supernodeKeeper.EXPECT().GetAllSuperNodes(gomock.AssignableToTypeOf(f.ctx), sntypes.SuperNodeStatePostponed).Return([]sntypes.SuperNode{sn}, nil)
	f.supernodeKeeper.EXPECT().RecoverSuperNodeFromPostponed(gomock.Any(), gomock.Any()).Times(0)

	require.NoError(t, f.keeper.EnforceEpochEnd(f.ctx, 5, params))
}

func TestStorageTruth_RecoveryBlockedByInsufficientCleanPasses(t *testing.T) {
	f := initFixture(t)
	sn, _, valAddr := makeActiveSupernode(t)

	params := types.DefaultParams()
	params.StorageTruthEnforcementMode = types.StorageTruthEnforcementMode_STORAGE_TRUTH_ENFORCEMENT_MODE_SOFT
	params.StorageTruthNodeSuspicionThresholdWatch = 20
	params.StorageTruthNodeSuspicionThresholdPostpone = 50
	params.StorageTruthNodeSuspicionThresholdStrongPostpone = 200 // won't trigger
	params.StorageTruthNodeSuspicionDecayPerEpoch = 920
	params.StorageTruthRecoveryCleanPassCount = 5 // requires 5 clean passes
	params.ConsecutiveEpochsToPostpone = 99

	// Postpone at epoch 0: score=100 > postpone(50), ClassA=1 + ClassB=1 → predicate met.
	err := f.keeper.SetNodeSuspicionState(f.ctx, types.NodeSuspicionState{
		SupernodeAccount:  sn.SupernodeAccount,
		SuspicionScore:    100,
		LastUpdatedEpoch:  0,
		ClassACountWindow: 1,
		ClassBCountWindow: 1,
		CleanPassCount:    2, // only 2 — insufficient for recovery
	})
	require.NoError(t, err)
	submitSelfReport(t, f, sn.SupernodeAccount, 0)

	f.supernodeKeeper.EXPECT().
		GetAllSuperNodes(gomock.AssignableToTypeOf(f.ctx), sntypes.SuperNodeStateActive).
		Return([]sntypes.SuperNode{sn}, nil)
	f.supernodeKeeper.EXPECT().
		GetAllSuperNodes(gomock.AssignableToTypeOf(f.ctx), sntypes.SuperNodeStatePostponed).
		Return([]sntypes.SuperNode{}, nil)
	f.supernodeKeeper.EXPECT().
		SetSuperNodePostponed(gomock.AssignableToTypeOf(f.ctx), sdk.ValAddress(valAddr), "audit_storage_truth_suspicion").
		Return(nil).Times(1)

	require.NoError(t, f.keeper.EnforceEpochEnd(f.ctx, 0, params))

	// Epoch 30: score=100 * 0.92^30 ≈ 8 < watch(20). But CleanPassCount=2 < required=5.
	// Recovery must be BLOCKED.
	f.supernodeKeeper.EXPECT().
		GetAllSuperNodes(gomock.AssignableToTypeOf(f.ctx), sntypes.SuperNodeStateActive).
		Return([]sntypes.SuperNode{}, nil)
	f.supernodeKeeper.EXPECT().
		GetAllSuperNodes(gomock.AssignableToTypeOf(f.ctx), sntypes.SuperNodeStatePostponed).
		Return([]sntypes.SuperNode{sn}, nil)
	f.supernodeKeeper.EXPECT().
		RecoverSuperNodeFromPostponed(gomock.Any(), gomock.Any()).
		Times(0)

	require.NoError(t, f.keeper.EnforceEpochEnd(f.ctx, 30, params))
}

// ---------------------------------------------------------------------------
// Heal-op scheduling gate
// ---------------------------------------------------------------------------

func TestStorageTruth_HealOpsScheduledInShadowMode(t *testing.T) {
	f := initFixture(t)

	params := types.DefaultParams()
	params.StorageTruthEnforcementMode = types.StorageTruthEnforcementMode_STORAGE_TRUTH_ENFORCEMENT_MODE_SHADOW
	params.StorageTruthMaxSelfHealOpsPerEpoch = 5
	params.StorageTruthTicketDeteriorationHealThreshold = 10

	setTicketDeterioration(t, f, "ticket-1", 50, 0)

	_, accAddr1, _ := cryptotestutils.SupernodeAddresses()
	_, accAddr2, _ := cryptotestutils.SupernodeAddresses()
	f.supernodeKeeper.EXPECT().
		GetAllSuperNodes(gomock.AssignableToTypeOf(f.ctx), sntypes.SuperNodeStateActive).
		Return([]sntypes.SuperNode{
			{SupernodeAccount: accAddr1.String()},
			{SupernodeAccount: accAddr2.String()},
		}, nil).AnyTimes()

	require.NoError(t, f.keeper.ProcessStorageTruthHealOpsAtEpochEnd(f.ctx, 0, params))

	healOps, err := f.keeper.GetAllHealOps(f.ctx)
	require.NoError(t, err)
	require.Len(t, healOps, 1)
	require.Equal(t, "ticket-1", healOps[0].TicketId)
}

func TestStorageTruth_HealOpsNotScheduledInUnspecifiedMode(t *testing.T) {
	f := initFixture(t)

	params := types.DefaultParams()
	params.StorageTruthEnforcementMode = types.StorageTruthEnforcementMode_STORAGE_TRUTH_ENFORCEMENT_MODE_UNSPECIFIED
	params.StorageTruthMaxSelfHealOpsPerEpoch = 5
	params.StorageTruthTicketDeteriorationHealThreshold = 10

	setTicketDeterioration(t, f, "ticket-1", 50, 0)

	require.NoError(t, f.keeper.ProcessStorageTruthHealOpsAtEpochEnd(f.ctx, 0, params))

	healOps, err := f.keeper.GetAllHealOps(f.ctx)
	require.NoError(t, err)
	require.Empty(t, healOps)
}

// ---------------------------------------------------------------------------
// Post-heal score reset: D = max(8, floor(D_old * 0.25))
// ---------------------------------------------------------------------------

func TestStorageTruth_VerifiedHealResetsTicketDeterioration(t *testing.T) {
	f := initFixture(t)

	_, accAddr, valAddr := cryptotestutils.SupernodeAddresses()
	healer := sntypes.SuperNode{
		SupernodeAccount: accAddr.String(),
		ValidatorAddress: sdk.ValAddress(valAddr).String(),
	}

	params := types.DefaultParams()
	params.StorageTruthProbationEpochs = 3

	setTicketDeterioration(t, f, "ticket-heal", 80, 0)

	healOp := types.HealOp{
		HealOpId:                  1,
		TicketId:                  "ticket-heal",
		HealerSupernodeAccount:    healer.SupernodeAccount,
		VerifierSupernodeAccounts: []string{"sn-verifier-heal"},
		Status:                    types.HealOpStatus_HEAL_OP_STATUS_SCHEDULED,
		CreatedHeight:             1,
		UpdatedHeight:             1,
		DeadlineEpochId:           10,
	}
	require.NoError(t, f.keeper.SetHealOp(f.ctx, healOp))
	f.keeper.SetNextHealOpID(f.ctx, 2)

	ticketState, found := f.keeper.GetTicketDeteriorationState(f.ctx, "ticket-heal")
	require.True(t, found)
	ticketState.ActiveHealOpId = 1
	require.NoError(t, f.keeper.SetTicketDeteriorationState(f.ctx, ticketState))

	f.supernodeKeeper.EXPECT().
		GetSuperNodeByAccount(gomock.AssignableToTypeOf(f.ctx), healer.SupernodeAccount).
		Return(healer, true, nil).AnyTimes()

	msgServer := keeper.NewMsgServerImpl(f.keeper)
	_, err := msgServer.ClaimHealComplete(f.ctx, &types.MsgClaimHealComplete{
		Creator:          healer.SupernodeAccount,
		HealOpId:         1,
		TicketId:         "ticket-heal",
		HealManifestHash: "abc123",
	})
	require.NoError(t, err)
	_, err = msgServer.SubmitHealVerification(f.ctx, &types.MsgSubmitHealVerification{
		Creator:          "sn-verifier-heal",
		HealOpId:         1,
		VerificationHash: "abc123", // Per 120-F6: must match ResultHash from ClaimHealComplete
		Verified:         true,
	})
	require.NoError(t, err)

	state, found := f.keeper.GetTicketDeteriorationState(f.ctx, "ticket-heal")
	require.True(t, found)

	// D_old=80 → floor(80*0.25)=20 >= 8 → resetScore=20.
	require.Equal(t, int64(20), state.DeteriorationScore)
	require.Greater(t, state.ProbationUntilEpoch, uint64(0))
}

func TestStorageTruth_VerifiedHealResetFloorIsEight(t *testing.T) {
	f := initFixture(t)

	_, accAddr, valAddr := cryptotestutils.SupernodeAddresses()
	healer := sntypes.SuperNode{
		SupernodeAccount: accAddr.String(),
		ValidatorAddress: sdk.ValAddress(valAddr).String(),
	}

	params := types.DefaultParams()
	params.StorageTruthProbationEpochs = 3
	require.NoError(t, f.keeper.SetParams(f.ctx, params))

	// D_old=20 → floor(20*0.25)=5 < 8 → resetScore=8.
	setTicketDeterioration(t, f, "ticket-floor", 20, 0)

	healOp := types.HealOp{
		HealOpId:                  1,
		TicketId:                  "ticket-floor",
		HealerSupernodeAccount:    healer.SupernodeAccount,
		VerifierSupernodeAccounts: []string{"sn-verifier-floor"},
		Status:                    types.HealOpStatus_HEAL_OP_STATUS_SCHEDULED,
		CreatedHeight:             1,
		UpdatedHeight:             1,
		DeadlineEpochId:           10,
	}
	require.NoError(t, f.keeper.SetHealOp(f.ctx, healOp))
	f.keeper.SetNextHealOpID(f.ctx, 2)

	ticketState, found := f.keeper.GetTicketDeteriorationState(f.ctx, "ticket-floor")
	require.True(t, found)
	ticketState.ActiveHealOpId = 1
	require.NoError(t, f.keeper.SetTicketDeteriorationState(f.ctx, ticketState))

	f.supernodeKeeper.EXPECT().
		GetSuperNodeByAccount(gomock.AssignableToTypeOf(f.ctx), healer.SupernodeAccount).
		Return(healer, true, nil).AnyTimes()

	msgServer := keeper.NewMsgServerImpl(f.keeper)
	_, err := msgServer.ClaimHealComplete(f.ctx, &types.MsgClaimHealComplete{
		Creator:          healer.SupernodeAccount,
		HealOpId:         1,
		TicketId:         "ticket-floor",
		HealManifestHash: "abc123",
	})
	require.NoError(t, err)
	_, err = msgServer.SubmitHealVerification(f.ctx, &types.MsgSubmitHealVerification{
		Creator:          "sn-verifier-floor",
		HealOpId:         1,
		VerificationHash: "abc123", // Per 120-F6: must match ResultHash from ClaimHealComplete
		Verified:         true,
	})
	require.NoError(t, err)

	state, found := f.keeper.GetTicketDeteriorationState(f.ctx, "ticket-floor")
	require.True(t, found)
	require.Equal(t, int64(8), state.DeteriorationScore)
}

// ---------------------------------------------------------------------------
// Failed heal: D += 15
// ---------------------------------------------------------------------------

func TestStorageTruth_FailedHealIncreasesDeterioration(t *testing.T) {
	f := initFixture(t)

	_, healerAddr, healerVal := cryptotestutils.SupernodeAddresses()
	_, verifierAddr, _ := cryptotestutils.SupernodeAddresses()
	healer := sntypes.SuperNode{
		SupernodeAccount: healerAddr.String(),
		ValidatorAddress: sdk.ValAddress(healerVal).String(),
	}
	verifier := sntypes.SuperNode{SupernodeAccount: verifierAddr.String()}

	setTicketDeterioration(t, f, "ticket-fail", 40, 0)

	healOp := types.HealOp{
		HealOpId:                  1,
		TicketId:                  "ticket-fail",
		HealerSupernodeAccount:    healer.SupernodeAccount,
		VerifierSupernodeAccounts: []string{verifier.SupernodeAccount},
		Status:                    types.HealOpStatus_HEAL_OP_STATUS_SCHEDULED,
		CreatedHeight:             1,
		UpdatedHeight:             1,
		DeadlineEpochId:           10,
	}
	require.NoError(t, f.keeper.SetHealOp(f.ctx, healOp))
	f.keeper.SetNextHealOpID(f.ctx, 2)

	ticketState, found := f.keeper.GetTicketDeteriorationState(f.ctx, "ticket-fail")
	require.True(t, found)
	ticketState.ActiveHealOpId = 1
	require.NoError(t, f.keeper.SetTicketDeteriorationState(f.ctx, ticketState))

	f.supernodeKeeper.EXPECT().
		GetSuperNodeByAccount(gomock.AssignableToTypeOf(f.ctx), healer.SupernodeAccount).
		Return(healer, true, nil).AnyTimes()
	f.supernodeKeeper.EXPECT().
		GetSuperNodeByAccount(gomock.AssignableToTypeOf(f.ctx), verifier.SupernodeAccount).
		Return(verifier, true, nil).AnyTimes()

	msgServer := keeper.NewMsgServerImpl(f.keeper)

	// Healer claims complete.
	_, err := msgServer.ClaimHealComplete(f.ctx, &types.MsgClaimHealComplete{
		Creator:          healer.SupernodeAccount,
		HealOpId:         1,
		TicketId:         "ticket-fail",
		HealManifestHash: "abc123",
	})
	require.NoError(t, err)

	// Verifier rejects.
	_, err = msgServer.SubmitHealVerification(f.ctx, &types.MsgSubmitHealVerification{
		Creator:          verifier.SupernodeAccount,
		HealOpId:         1,
		Verified:         false,
		VerificationHash: "rejected",
	})
	require.NoError(t, err)

	state, found := f.keeper.GetTicketDeteriorationState(f.ctx, "ticket-fail")
	require.True(t, found)
	require.Equal(t, int64(55), state.DeteriorationScore) // 40 + 15
}

// ---------------------------------------------------------------------------
// Recheck evidence
// ---------------------------------------------------------------------------

func TestStorageTruth_RecheckEvidenceUpdatesScores(t *testing.T) {
	f := initFixture(t)
	f.ctx = f.ctx.WithBlockHeight(1).WithEventManager(sdk.NewEventManager())

	_, reporterAddr, _ := cryptotestutils.SupernodeAddresses()
	_, targetAddr, _ := cryptotestutils.SupernodeAddresses()
	_, originalReporterAddr, _ := cryptotestutils.SupernodeAddresses()
	reporter := sntypes.SuperNode{SupernodeAccount: reporterAddr.String()}
	target := sntypes.SuperNode{SupernodeAccount: targetAddr.String()}
	originalReporter := originalReporterAddr.String()

	params := types.DefaultParams()
	params.StorageTruthEnforcementMode = types.StorageTruthEnforcementMode_STORAGE_TRUTH_ENFORCEMENT_MODE_SHADOW
	require.NoError(t, f.keeper.SetParams(f.ctx, params))

	seedEpochAnchorForReportTest(t, f, 0, []string{reporter.SupernodeAccount, target.SupernodeAccount}, []string{reporter.SupernodeAccount, target.SupernodeAccount})

	f.supernodeKeeper.EXPECT().
		GetSuperNodeByAccount(gomock.AssignableToTypeOf(f.ctx), reporter.SupernodeAccount).
		Return(reporter, true, nil).AnyTimes()
	f.supernodeKeeper.EXPECT().
		GetSuperNodeByAccount(gomock.AssignableToTypeOf(f.ctx), target.SupernodeAccount).
		Return(target, true, nil).AnyTimes()
	seedIndexedChallengeResult(t, f, originalReporter, target.SupernodeAccount, "ticket-recheck", "hash-orig")

	msgServer := keeper.NewMsgServerImpl(f.keeper)
	_, err := msgServer.SubmitStorageRecheckEvidence(f.ctx, &types.MsgSubmitStorageRecheckEvidence{
		Creator:                        reporter.SupernodeAccount,
		EpochId:                        0,
		ChallengedSupernodeAccount:     target.SupernodeAccount,
		TicketId:                       "ticket-recheck",
		ChallengedResultTranscriptHash: "hash-orig",
		RecheckTranscriptHash:          "hash-recheck",
		RecheckResultClass:             types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_RECHECK_CONFIRMED_FAIL,
	})
	require.NoError(t, err)

	// Target node suspicion should have increased.
	nodeState, found := f.keeper.GetNodeSuspicionState(f.ctx, target.SupernodeAccount)
	require.True(t, found)
	require.Greater(t, nodeState.SuspicionScore, int64(0))
}

// ---------------------------------------------------------------------------
// Band event granularity: watch / probation / below-watch
// ---------------------------------------------------------------------------

func TestStorageTruth_WatchBandEventEmitted(t *testing.T) {
	f := initFixture(t)
	f.ctx = f.ctx.WithBlockHeight(1).WithEventManager(sdk.NewEventManager())
	sn, _, _ := makeActiveSupernode(t)

	params := types.DefaultParams()
	params.StorageTruthEnforcementMode = types.StorageTruthEnforcementMode_STORAGE_TRUTH_ENFORCEMENT_MODE_SHADOW
	params.StorageTruthNodeSuspicionThresholdWatch = 20
	params.StorageTruthNodeSuspicionThresholdProbation = 40
	params.StorageTruthNodeSuspicionThresholdPostpone = 60
	params.StorageTruthNodeSuspicionDecayPerEpoch = 0
	params.ConsecutiveEpochsToPostpone = 99
	require.NoError(t, f.keeper.SetParams(f.ctx, params))

	// score=25: watch ≤ 25 < probation
	setNodeSuspicion(t, f, sn.SupernodeAccount, 25, 0)
	submitSelfReport(t, f, sn.SupernodeAccount, 0)

	f.supernodeKeeper.EXPECT().
		GetAllSuperNodes(gomock.AssignableToTypeOf(f.ctx), sntypes.SuperNodeStateActive).
		Return([]sntypes.SuperNode{sn}, nil)
	f.supernodeKeeper.EXPECT().
		GetAllSuperNodes(gomock.AssignableToTypeOf(f.ctx), sntypes.SuperNodeStatePostponed).
		Return([]sntypes.SuperNode{}, nil)

	require.NoError(t, f.keeper.EnforceEpochEnd(f.ctx, 0, params))

	events := f.ctx.EventManager().Events()
	found := false
	for _, e := range events {
		if e.Type == types.EventTypeStorageTruthBandWatch {
			found = true
			break
		}
	}
	require.True(t, found, "expected EventTypeStorageTruthBandWatch to be emitted for watch-band score")

	// Postpone event must NOT have been emitted (SHADOW mode).
	for _, e := range events {
		require.NotEqual(t, types.EventTypeStorageTruthEnforced, e.Type,
			"expected no postpone event in SHADOW mode")
	}
}

func TestStorageTruth_ProbationBandEventEmitted(t *testing.T) {
	f := initFixture(t)
	f.ctx = f.ctx.WithBlockHeight(1).WithEventManager(sdk.NewEventManager())
	sn, _, _ := makeActiveSupernode(t)

	params := types.DefaultParams()
	params.StorageTruthEnforcementMode = types.StorageTruthEnforcementMode_STORAGE_TRUTH_ENFORCEMENT_MODE_SHADOW
	params.StorageTruthNodeSuspicionThresholdWatch = 20
	params.StorageTruthNodeSuspicionThresholdProbation = 40
	params.StorageTruthNodeSuspicionThresholdPostpone = 60
	params.StorageTruthNodeSuspicionDecayPerEpoch = 0
	params.ConsecutiveEpochsToPostpone = 99
	require.NoError(t, f.keeper.SetParams(f.ctx, params))

	// score=45: probation ≤ 45 < postpone
	setNodeSuspicion(t, f, sn.SupernodeAccount, 45, 0)
	submitSelfReport(t, f, sn.SupernodeAccount, 0)

	f.supernodeKeeper.EXPECT().
		GetAllSuperNodes(gomock.AssignableToTypeOf(f.ctx), sntypes.SuperNodeStateActive).
		Return([]sntypes.SuperNode{sn}, nil)
	f.supernodeKeeper.EXPECT().
		GetAllSuperNodes(gomock.AssignableToTypeOf(f.ctx), sntypes.SuperNodeStatePostponed).
		Return([]sntypes.SuperNode{}, nil)

	require.NoError(t, f.keeper.EnforceEpochEnd(f.ctx, 0, params))

	events := f.ctx.EventManager().Events()
	found := false
	for _, e := range events {
		if e.Type == types.EventTypeStorageTruthBandProbation {
			found = true
			break
		}
	}
	require.True(t, found, "expected EventTypeStorageTruthBandProbation to be emitted for probation-band score")
}

func TestStorageTruth_BelowWatchThresholdNoEvents(t *testing.T) {
	f := initFixture(t)
	f.ctx = f.ctx.WithBlockHeight(1).WithEventManager(sdk.NewEventManager())
	sn, _, _ := makeActiveSupernode(t)

	params := types.DefaultParams()
	params.StorageTruthEnforcementMode = types.StorageTruthEnforcementMode_STORAGE_TRUTH_ENFORCEMENT_MODE_SHADOW
	params.StorageTruthNodeSuspicionThresholdWatch = 20
	params.StorageTruthNodeSuspicionThresholdProbation = 40
	params.StorageTruthNodeSuspicionThresholdPostpone = 60
	params.StorageTruthNodeSuspicionDecayPerEpoch = 0
	params.ConsecutiveEpochsToPostpone = 99
	require.NoError(t, f.keeper.SetParams(f.ctx, params))

	// score=10: below watch threshold → no band events
	setNodeSuspicion(t, f, sn.SupernodeAccount, 10, 0)
	submitSelfReport(t, f, sn.SupernodeAccount, 0)

	f.supernodeKeeper.EXPECT().
		GetAllSuperNodes(gomock.AssignableToTypeOf(f.ctx), sntypes.SuperNodeStateActive).
		Return([]sntypes.SuperNode{sn}, nil)
	f.supernodeKeeper.EXPECT().
		GetAllSuperNodes(gomock.AssignableToTypeOf(f.ctx), sntypes.SuperNodeStatePostponed).
		Return([]sntypes.SuperNode{}, nil)

	require.NoError(t, f.keeper.EnforceEpochEnd(f.ctx, 0, params))

	events := f.ctx.EventManager().Events()
	for _, e := range events {
		require.NotEqual(t, types.EventTypeStorageTruthBandWatch, e.Type, "unexpected watch event for score below watch threshold")
		require.NotEqual(t, types.EventTypeStorageTruthBandProbation, e.Type, "unexpected probation event for score below watch threshold")
		require.NotEqual(t, types.EventTypeStorageTruthBandPostpone, e.Type, "unexpected postpone event for score below watch threshold")
		require.NotEqual(t, types.EventTypeStorageTruthEnforced, e.Type, "unexpected enforced event for score below watch threshold")
	}
}

func TestStorageTruth_RecheckEvidenceReplayRejected(t *testing.T) {
	f := initFixture(t)
	f.ctx = f.ctx.WithBlockHeight(1).WithEventManager(sdk.NewEventManager())

	_, reporterAddr, _ := cryptotestutils.SupernodeAddresses()
	_, targetAddr, _ := cryptotestutils.SupernodeAddresses()
	_, originalReporterAddr, _ := cryptotestutils.SupernodeAddresses()
	reporter := sntypes.SuperNode{SupernodeAccount: reporterAddr.String()}
	target := sntypes.SuperNode{SupernodeAccount: targetAddr.String()}
	originalReporter := originalReporterAddr.String()

	params := types.DefaultParams()
	params.StorageTruthEnforcementMode = types.StorageTruthEnforcementMode_STORAGE_TRUTH_ENFORCEMENT_MODE_SHADOW
	require.NoError(t, f.keeper.SetParams(f.ctx, params))

	seedEpochAnchorForReportTest(t, f, 0, []string{reporter.SupernodeAccount, target.SupernodeAccount}, []string{reporter.SupernodeAccount, target.SupernodeAccount})

	f.supernodeKeeper.EXPECT().
		GetSuperNodeByAccount(gomock.AssignableToTypeOf(f.ctx), reporter.SupernodeAccount).
		Return(reporter, true, nil).AnyTimes()
	f.supernodeKeeper.EXPECT().
		GetSuperNodeByAccount(gomock.AssignableToTypeOf(f.ctx), target.SupernodeAccount).
		Return(target, true, nil).AnyTimes()
	seedIndexedChallengeResult(t, f, originalReporter, target.SupernodeAccount, "ticket-replay", "hash-orig")

	req := &types.MsgSubmitStorageRecheckEvidence{
		Creator:                        reporter.SupernodeAccount,
		EpochId:                        0,
		ChallengedSupernodeAccount:     target.SupernodeAccount,
		TicketId:                       "ticket-replay",
		ChallengedResultTranscriptHash: "hash-orig",
		RecheckTranscriptHash:          "hash-recheck",
		RecheckResultClass:             types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_PASS,
	}

	msgServer := keeper.NewMsgServerImpl(f.keeper)
	_, err := msgServer.SubmitStorageRecheckEvidence(f.ctx, req)
	require.NoError(t, err)

	// Second submission for same (epoch, ticket, creator) must fail.
	_, err = msgServer.SubmitStorageRecheckEvidence(f.ctx, req)
	require.Error(t, err)
	require.Contains(t, err.Error(), "already submitted")
}
