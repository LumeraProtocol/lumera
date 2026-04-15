package keeper_test

import (
	"testing"

	"github.com/LumeraProtocol/lumera/testutil/cryptotestutils"
	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"go.uber.org/mock/gomock"
)

// TestEnforceEpochEnd_EmptyActiveSet_PostponedCannotRecover verifies that when
// the active set is empty (all supernodes POSTPONED), submitting compliant
// host-only epoch reports is insufficient for recovery because no peer
// observations exist. This is the "empty active set deadlock".
func TestEnforceEpochEnd_EmptyActiveSet_PostponedCannotRecover(t *testing.T) {
	f := initFixture(t)

	_, sn0Acc, sn0Val := cryptotestutils.SupernodeAddresses()
	_, sn1Acc, sn1Val := cryptotestutils.SupernodeAddresses()

	sn0 := sntypes.SuperNode{
		SupernodeAccount: sn0Acc.String(),
		ValidatorAddress: sdk.ValAddress(sn0Val).String(),
	}
	sn1 := sntypes.SuperNode{
		SupernodeAccount: sn1Acc.String(),
		ValidatorAddress: sdk.ValAddress(sn1Val).String(),
	}

	params := types.DefaultParams()
	params.RequiredOpenPorts = []uint32{4444}
	params.ConsecutiveEpochsToPostpone = 1

	epochID := uint64(1)

	// Both POSTPONED supernodes submit compliant host-only reports.
	for _, sn := range []sntypes.SuperNode{sn0, sn1} {
		err := f.keeper.SetReport(f.ctx, types.EpochReport{
			SupernodeAccount: sn.SupernodeAccount,
			EpochId:          epochID,
			ReportHeight:     f.ctx.BlockHeight(),
			HostReport:       types.HostReport{},
		})
		if err != nil {
			t.Fatalf("failed to set report for %s: %v", sn.SupernodeAccount, err)
		}
	}

	// No StorageChallengeReportIndex entries — no one probed anyone
	// (empty active set means no probers were assigned).

	// Mock: no ACTIVE supernodes, two POSTPONED.
	f.supernodeKeeper.EXPECT().
		GetAllSuperNodes(gomock.AssignableToTypeOf(f.ctx), sntypes.SuperNodeStateActive).
		Return([]sntypes.SuperNode{}, nil).
		Times(1)
	f.supernodeKeeper.EXPECT().
		GetAllSuperNodes(gomock.AssignableToTypeOf(f.ctx), sntypes.SuperNodeStatePostponed).
		Return([]sntypes.SuperNode{sn0, sn1}, nil).
		Times(1)

	// Recovery should NOT be called — no peer observations exist.
	f.supernodeKeeper.EXPECT().
		RecoverSuperNodeFromPostponed(gomock.Any(), gomock.Any()).
		Times(0)

	err := f.keeper.EnforceEpochEnd(f.ctx, epochID, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestEnforceEpochEnd_LegacyRecoveredSN_SurvivesWithReport verifies that a
// supernode which was recovered to ACTIVE mid-epoch (e.g., by legacy
// MsgReportSupernodeMetrics) and also submitted an audit epoch report
// is NOT re-postponed at epoch end, even when no peer observations exist.
//
// This confirms the fix: legacy metrics recovery + audit epoch report =
// the SN survives enforcement and can appear in the next epoch's anchor.
func TestEnforceEpochEnd_LegacyRecoveredSN_SurvivesWithReport(t *testing.T) {
	f := initFixture(t)

	_, sn0Acc, sn0Val := cryptotestutils.SupernodeAddresses()
	_, sn1Acc, sn1Val := cryptotestutils.SupernodeAddresses()

	sn0 := sntypes.SuperNode{
		SupernodeAccount: sn0Acc.String(),
		ValidatorAddress: sdk.ValAddress(sn0Val).String(),
	}
	sn1 := sntypes.SuperNode{
		SupernodeAccount: sn1Acc.String(),
		ValidatorAddress: sdk.ValAddress(sn1Val).String(),
	}

	params := types.DefaultParams()
	params.RequiredOpenPorts = []uint32{4444}
	params.ConsecutiveEpochsToPostpone = 1

	epochID := uint64(1)

	// Both supernodes submitted epoch reports (host-only, as they were
	// POSTPONED when submitting — no storage challenge observations).
	for _, sn := range []sntypes.SuperNode{sn0, sn1} {
		err := f.keeper.SetReport(f.ctx, types.EpochReport{
			SupernodeAccount: sn.SupernodeAccount,
			EpochId:          epochID,
			ReportHeight:     f.ctx.BlockHeight(),
			HostReport:       types.HostReport{},
		})
		if err != nil {
			t.Fatalf("failed to set report for %s: %v", sn.SupernodeAccount, err)
		}
	}

	// Simulate: both were recovered to ACTIVE mid-epoch via legacy metrics.
	// At epoch end, the audit enforcement sees them as ACTIVE.
	f.supernodeKeeper.EXPECT().
		GetAllSuperNodes(gomock.AssignableToTypeOf(f.ctx), sntypes.SuperNodeStateActive).
		Return([]sntypes.SuperNode{sn0, sn1}, nil).
		Times(1)
	f.supernodeKeeper.EXPECT().
		GetAllSuperNodes(gomock.AssignableToTypeOf(f.ctx), sntypes.SuperNodeStatePostponed).
		Return([]sntypes.SuperNode{}, nil).
		Times(1)

	// They have reports → no missing-report postponement.
	// Host minimums are all 0 → no violation.
	// No peer observations → peersPortStateMeetsThreshold returns false → no streak → no postponement.
	// Expect: SetSuperNodePostponed is NEVER called.
	f.supernodeKeeper.EXPECT().
		SetSuperNodePostponed(gomock.Any(), gomock.Any(), gomock.Any()).
		Times(0)

	err := f.keeper.EnforceEpochEnd(f.ctx, epochID, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
