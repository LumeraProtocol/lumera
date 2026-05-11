package keeper_test

import (
	"testing"

	"github.com/LumeraProtocol/lumera/testutil/crypto"
	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"go.uber.org/mock/gomock"
)

// These tests cover the audit bootstrap-recovery exception in
// shouldRecoverAtEpochEnd. When the epoch's anchored active set is empty
// (all supernodes POSTPONED), the peer-port recovery rule is unsatisfiable
// by construction (no probers exist), so the chain must accept a compliant
// self host-report alone as sufficient to recover. Without this exception,
// the chain cannot self-heal from the "empty active set deadlock" and
// requires every validator key holder to perform a manual deregister/
// re-register cycle out-of-band — a distributed coordination problem on
// mainnet.
//
// Self-compliance is still mandatory: a misbehaving SN (e.g. disk usage
// over threshold) cannot self-recover via this branch.

// helper: writes an empty-active-set EpochAnchor at the given epochID so
// shouldRecoverAtEpochEnd can read it.
func writeEmptyActiveSetAnchor(t *testing.T, f *fixture, epochID uint64) {
	t.Helper()
	if err := f.keeper.SetEpochAnchor(f.ctx, types.EpochAnchor{
		EpochId:                 epochID,
		EpochStartHeight:        f.ctx.BlockHeight(),
		EpochEndHeight:          f.ctx.BlockHeight() + 1,
		EpochLengthBlocks:       1,
		ActiveSupernodeAccounts: nil, // empty active set — the deadlock condition
	}); err != nil {
		t.Fatalf("failed to set epoch anchor: %v", err)
	}
}

// helper: writes a non-empty active-set EpochAnchor so the bootstrap branch
// does NOT fire and the legacy peer-port path is exercised.
func writeNonEmptyActiveSetAnchor(t *testing.T, f *fixture, epochID uint64, accounts []string) {
	t.Helper()
	if err := f.keeper.SetEpochAnchor(f.ctx, types.EpochAnchor{
		EpochId:                 epochID,
		EpochStartHeight:        f.ctx.BlockHeight(),
		EpochEndHeight:          f.ctx.BlockHeight() + 1,
		EpochLengthBlocks:       1,
		ActiveSupernodeAccounts: accounts,
	}); err != nil {
		t.Fatalf("failed to set epoch anchor: %v", err)
	}
}

// TestEnforceEpochEnd_EmptyActiveSet_PostponedRecoversViaBootstrapException
// verifies that when the active set is empty (all SNs POSTPONED) AND every
// POSTPONED SN submitted a compliant self host-report, the bootstrap
// exception allows each SN to recover at epoch end, breaking the deadlock.
//
// This inverts the pre-fix behavior documented in commit history:
// previously this scenario asserted Times(0) recovery (deadlock confirmed).
func TestEnforceEpochEnd_EmptyActiveSet_PostponedRecoversViaBootstrapException(t *testing.T) {
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

	// Anchor the empty-active-set condition for this epoch.
	writeEmptyActiveSetAnchor(t, f, epochID)

	// Both POSTPONED supernodes submit compliant host-only reports.
	for _, sn := range []sntypes.SuperNode{sn0, sn1} {
		if err := f.keeper.SetReport(f.ctx, types.EpochReport{
			SupernodeAccount: sn.SupernodeAccount,
			EpochId:          epochID,
			ReportHeight:     f.ctx.BlockHeight(),
			HostReport:       types.HostReport{}, // defaults are compliant (mins are 0)
		}); err != nil {
			t.Fatalf("failed to set report for %s: %v", sn.SupernodeAccount, err)
		}
	}

	// No StorageChallengeReportIndex entries — no one probed anyone.
	// With the bootstrap exception, recovery still succeeds.

	// Mock: no ACTIVE supernodes, two POSTPONED.
	f.supernodeKeeper.EXPECT().
		GetAllSuperNodes(gomock.AssignableToTypeOf(f.ctx), sntypes.SuperNodeStateActive).
		Return([]sntypes.SuperNode{}, nil).
		Times(1)
	f.supernodeKeeper.EXPECT().
		GetAllSuperNodes(gomock.AssignableToTypeOf(f.ctx), sntypes.SuperNodeStatePostponed).
		Return([]sntypes.SuperNode{sn0, sn1}, nil).
		Times(1)

	// Recovery MUST be called exactly once per SN — the bootstrap exception fired.
	f.supernodeKeeper.EXPECT().
		RecoverSuperNodeFromPostponed(gomock.Any(), gomock.Any()).
		Return(nil).
		Times(2)

	err := f.keeper.EnforceEpochEnd(f.ctx, epochID, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestEnforceEpochEnd_EmptyActiveSet_NoSelfReport_NoRecover verifies the
// bootstrap exception does NOT bypass the self-compliance requirement.
// A POSTPONED SN that did not submit a report stays POSTPONED even when
// the active set is empty.
func TestEnforceEpochEnd_EmptyActiveSet_NoSelfReport_NoRecover(t *testing.T) {
	f := initFixture(t)

	_, sn0Acc, sn0Val := cryptotestutils.SupernodeAddresses()

	sn0 := sntypes.SuperNode{
		SupernodeAccount: sn0Acc.String(),
		ValidatorAddress: sdk.ValAddress(sn0Val).String(),
	}

	params := types.DefaultParams()
	params.RequiredOpenPorts = []uint32{4444}
	params.ConsecutiveEpochsToPostpone = 1

	epochID := uint64(1)

	writeEmptyActiveSetAnchor(t, f, epochID)

	// sn0 did NOT submit any report this epoch — selfHostCompliant returns false.

	f.supernodeKeeper.EXPECT().
		GetAllSuperNodes(gomock.AssignableToTypeOf(f.ctx), sntypes.SuperNodeStateActive).
		Return([]sntypes.SuperNode{}, nil).
		Times(1)
	f.supernodeKeeper.EXPECT().
		GetAllSuperNodes(gomock.AssignableToTypeOf(f.ctx), sntypes.SuperNodeStatePostponed).
		Return([]sntypes.SuperNode{sn0}, nil).
		Times(1)

	// Recovery MUST NOT be called — self-compliance gate blocked the bootstrap branch.
	f.supernodeKeeper.EXPECT().
		RecoverSuperNodeFromPostponed(gomock.Any(), gomock.Any()).
		Times(0)

	if err := f.keeper.EnforceEpochEnd(f.ctx, epochID, params); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestEnforceEpochEnd_EmptyActiveSet_NonCompliantSelf_NoRecover verifies the
// bootstrap exception does NOT bypass the self-compliance health checks.
// A POSTPONED SN that submits a report violating the disk-usage minimum
// stays POSTPONED even when the active set is empty.
func TestEnforceEpochEnd_EmptyActiveSet_NonCompliantSelf_NoRecover(t *testing.T) {
	f := initFixture(t)

	_, sn0Acc, sn0Val := cryptotestutils.SupernodeAddresses()

	sn0 := sntypes.SuperNode{
		SupernodeAccount: sn0Acc.String(),
		ValidatorAddress: sdk.ValAddress(sn0Val).String(),
	}

	params := types.DefaultParams()
	params.RequiredOpenPorts = []uint32{4444}
	params.ConsecutiveEpochsToPostpone = 1
	// Require at least 20% disk free; sn0 reports 95% usage → 5% free → not compliant.
	params.MinDiskFreePercent = 20

	epochID := uint64(1)

	writeEmptyActiveSetAnchor(t, f, epochID)

	// SetReport with non-zero DiskUsagePercent invokes the STORAGE_FULL
	// transition source path, which queries supernodeKeeper. Stub these
	// dependencies so the call lands cleanly without triggering a
	// transition (we return "not found" → SetReport short-circuits).
	f.supernodeKeeper.EXPECT().
		GetSuperNodeByAccount(gomock.AssignableToTypeOf(f.ctx), sn0.SupernodeAccount).
		Return(sntypes.SuperNode{}, false, nil).
		Times(1)

	if err := f.keeper.SetReport(f.ctx, types.EpochReport{
		SupernodeAccount: sn0.SupernodeAccount,
		EpochId:          epochID,
		ReportHeight:     f.ctx.BlockHeight(),
		HostReport: types.HostReport{
			DiskUsagePercent: 95.0, // 5% free, below the 20% minimum
		},
	}); err != nil {
		t.Fatalf("failed to set report: %v", err)
	}

	f.supernodeKeeper.EXPECT().
		GetAllSuperNodes(gomock.AssignableToTypeOf(f.ctx), sntypes.SuperNodeStateActive).
		Return([]sntypes.SuperNode{}, nil).
		Times(1)
	f.supernodeKeeper.EXPECT().
		GetAllSuperNodes(gomock.AssignableToTypeOf(f.ctx), sntypes.SuperNodeStatePostponed).
		Return([]sntypes.SuperNode{sn0}, nil).
		Times(1)

	// Recovery MUST NOT be called — self-compliance gate blocked the bootstrap branch.
	f.supernodeKeeper.EXPECT().
		RecoverSuperNodeFromPostponed(gomock.Any(), gomock.Any()).
		Times(0)

	if err := f.keeper.EnforceEpochEnd(f.ctx, epochID, params); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestEnforceEpochEnd_NonEmptyActiveSet_NoPeerObs_NoRecover verifies the
// legacy peer-port recovery rule is preserved when the active set is
// non-empty. A POSTPONED SN with a compliant self-report but NO peer
// observations stays POSTPONED — the bootstrap branch does not fire when
// probers exist (or could exist).
func TestEnforceEpochEnd_NonEmptyActiveSet_NoPeerObs_NoRecover(t *testing.T) {
	f := initFixture(t)

	_, sn0Acc, sn0Val := cryptotestutils.SupernodeAddresses()
	_, sn1Acc, sn1Val := cryptotestutils.SupernodeAddresses()

	sn0 := sntypes.SuperNode{
		SupernodeAccount: sn0Acc.String(),
		ValidatorAddress: sdk.ValAddress(sn0Val).String(),
	}
	sn1Active := sntypes.SuperNode{
		SupernodeAccount: sn1Acc.String(),
		ValidatorAddress: sdk.ValAddress(sn1Val).String(),
	}

	params := types.DefaultParams()
	params.RequiredOpenPorts = []uint32{4444}
	params.ConsecutiveEpochsToPostpone = 1

	epochID := uint64(1)

	// Active set is non-empty (sn1 is active) — bootstrap branch must NOT fire.
	writeNonEmptyActiveSetAnchor(t, f, epochID, []string{sn1Active.SupernodeAccount})

	if err := f.keeper.SetReport(f.ctx, types.EpochReport{
		SupernodeAccount: sn0.SupernodeAccount,
		EpochId:          epochID,
		ReportHeight:     f.ctx.BlockHeight(),
		HostReport:       types.HostReport{}, // compliant
	}); err != nil {
		t.Fatalf("failed to set report: %v", err)
	}

	// sn1 (active) submits no report → no peer observations about sn0.

	f.supernodeKeeper.EXPECT().
		GetAllSuperNodes(gomock.AssignableToTypeOf(f.ctx), sntypes.SuperNodeStateActive).
		Return([]sntypes.SuperNode{sn1Active}, nil).
		Times(1)
	f.supernodeKeeper.EXPECT().
		GetAllSuperNodes(gomock.AssignableToTypeOf(f.ctx), sntypes.SuperNodeStatePostponed).
		Return([]sntypes.SuperNode{sn0}, nil).
		Times(1)

	// Legacy behavior: no peer all-open observation → no recovery.
	f.supernodeKeeper.EXPECT().
		RecoverSuperNodeFromPostponed(gomock.Any(), gomock.Any()).
		Times(0)
	// sn1 (active) has no report → may be postponed for missing report,
	// but consecutive_epochs_to_postpone=1 at epochID=1 means the streak
	// check sees not-enough-history; mock both directions defensively.
	f.supernodeKeeper.EXPECT().
		SetSuperNodePostponed(gomock.Any(), gomock.Any(), gomock.Any()).
		AnyTimes()

	if err := f.keeper.EnforceEpochEnd(f.ctx, epochID, params); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestEnforceEpochEnd_NoEpochAnchor_FallsThroughToLegacyPath verifies that
// when the epoch anchor is missing (e.g. a node that started mid-epoch
// before the anchor was persisted, or a test fixture that doesn't write
// one), the bootstrap branch does NOT fire and the legacy peer-port path
// runs unchanged. This is the safe default.
func TestEnforceEpochEnd_NoEpochAnchor_FallsThroughToLegacyPath(t *testing.T) {
	f := initFixture(t)

	_, sn0Acc, sn0Val := cryptotestutils.SupernodeAddresses()

	sn0 := sntypes.SuperNode{
		SupernodeAccount: sn0Acc.String(),
		ValidatorAddress: sdk.ValAddress(sn0Val).String(),
	}

	params := types.DefaultParams()
	params.RequiredOpenPorts = []uint32{4444}
	params.ConsecutiveEpochsToPostpone = 1

	epochID := uint64(1)

	// NO anchor written for this epoch.

	if err := f.keeper.SetReport(f.ctx, types.EpochReport{
		SupernodeAccount: sn0.SupernodeAccount,
		EpochId:          epochID,
		ReportHeight:     f.ctx.BlockHeight(),
		HostReport:       types.HostReport{},
	}); err != nil {
		t.Fatalf("failed to set report: %v", err)
	}

	f.supernodeKeeper.EXPECT().
		GetAllSuperNodes(gomock.AssignableToTypeOf(f.ctx), sntypes.SuperNodeStateActive).
		Return([]sntypes.SuperNode{}, nil).
		Times(1)
	f.supernodeKeeper.EXPECT().
		GetAllSuperNodes(gomock.AssignableToTypeOf(f.ctx), sntypes.SuperNodeStatePostponed).
		Return([]sntypes.SuperNode{sn0}, nil).
		Times(1)

	// No anchor → bootstrap branch not taken → legacy peer-port path returns
	// false (no peers, no observations) → recovery not invoked.
	f.supernodeKeeper.EXPECT().
		RecoverSuperNodeFromPostponed(gomock.Any(), gomock.Any()).
		Times(0)

	if err := f.keeper.EnforceEpochEnd(f.ctx, epochID, params); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
