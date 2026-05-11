package keeper_test

import (
	"fmt"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/LumeraProtocol/lumera/x/audit/v1/keeper"
	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
)

// Per CP-R3 B-F1 — the "no overturned fails" gate must look at failure-class
// records (where OverturnedByRecheck is actually written), not at PASS-class
// records. The previous implementation skipped non-PASS in the same iterator
// that checked the overturn flag, making the gate structurally always false:
// a fraudulent reporter with 5 PASS + 1 overturned FAIL still received -4.

func TestApplyReporterCleanEpochRecovery_OverturnedFailSuppressesMinusFour(t *testing.T) {
	f := initFixture(t)
	f.ctx = f.ctx.WithBlockHeight(1).WithEventManager(sdk.NewEventManager())

	const epochID = uint64(11)
	const reporter = "reporter-overturn-fail"

	// Prior reliability score 10 — observable above the floor.
	require.NoError(t, f.keeper.SetReporterReliabilityState(f.ctx, types.ReporterReliabilityState{
		ReporterSupernodeAccount: reporter,
		ReliabilityScore:         10,
		LastUpdatedEpoch:         epochID,
	}))

	// 5 PASS results in the epoch.
	for i := 0; i < 5; i++ {
		passResult := &types.StorageProofResult{
			TicketId:               fmt.Sprintf("ticket-pass-%d", i),
			TargetSupernodeAccount: fmt.Sprintf("target-pass-%d", i),
			ResultClass:            types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_PASS,
			BucketType:             types.StorageProofBucketType_STORAGE_PROOF_BUCKET_TYPE_RECENT,
		}
		require.NoError(t, keeper.SetStorageTruthReporterResultForTest(f.keeper, f.ctx, epochID, reporter, passResult))
	}

	// 1 HASH_MISMATCH (Class A failure) flagged as overturned by recheck.
	failResult := &types.StorageProofResult{
		TicketId:               "ticket-overturned-fail",
		TargetSupernodeAccount: "target-overturned",
		ResultClass:            types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_HASH_MISMATCH,
		BucketType:             types.StorageProofBucketType_STORAGE_PROOF_BUCKET_TYPE_RECENT,
	}
	require.NoError(t, keeper.SetReporterResultOverturnFlagForTest(f.keeper, f.ctx, epochID, reporter, failResult, true))

	params := f.keeper.GetParams(f.ctx).WithDefaults()
	require.NoError(t, f.keeper.ApplyReporterCleanEpochRecoveryAtEpochEnd(f.ctx, epochID, params))

	final, found := f.keeper.GetReporterReliabilityState(f.ctx, reporter)
	require.True(t, found)
	require.Equal(t, int64(10), final.ReliabilityScore,
		"reporter with overturned FAIL must NOT receive -4 §15.3 reward (B-F1)")
}

func TestApplyReporterCleanEpochRecovery_NonOverturnedFailDoesNotBlockReward(t *testing.T) {
	// Sibling case to confirm the gate is not over-restrictive: a confirmed
	// failure (or any non-overturned fail) does NOT block the reward — only
	// an overturned-by-recheck fail does. Spec §15.3 wording: "no overturned
	// fails" means specifically those overturned via recheck.
	f := initFixture(t)
	f.ctx = f.ctx.WithBlockHeight(1).WithEventManager(sdk.NewEventManager())

	const epochID = uint64(12)
	const reporter = "reporter-confirmed-fail"

	require.NoError(t, f.keeper.SetReporterReliabilityState(f.ctx, types.ReporterReliabilityState{
		ReporterSupernodeAccount: reporter,
		ReliabilityScore:         10,
		LastUpdatedEpoch:         epochID,
	}))

	for i := 0; i < 5; i++ {
		passResult := &types.StorageProofResult{
			TicketId:               fmt.Sprintf("ticket-pass-cf-%d", i),
			TargetSupernodeAccount: fmt.Sprintf("target-pass-cf-%d", i),
			ResultClass:            types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_PASS,
			BucketType:             types.StorageProofBucketType_STORAGE_PROOF_BUCKET_TYPE_RECENT,
		}
		require.NoError(t, keeper.SetStorageTruthReporterResultForTest(f.keeper, f.ctx, epochID, reporter, passResult))
	}

	// Confirmed failure (overturned=false) — should NOT block the -4 reward.
	failResult := &types.StorageProofResult{
		TicketId:               "ticket-confirmed-fail",
		TargetSupernodeAccount: "target-confirmed",
		ResultClass:            types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_HASH_MISMATCH,
		BucketType:             types.StorageProofBucketType_STORAGE_PROOF_BUCKET_TYPE_RECENT,
	}
	require.NoError(t, keeper.SetReporterResultOverturnFlagForTest(f.keeper, f.ctx, epochID, reporter, failResult, false))

	params := f.keeper.GetParams(f.ctx).WithDefaults()
	require.NoError(t, f.keeper.ApplyReporterCleanEpochRecoveryAtEpochEnd(f.ctx, epochID, params))

	final, found := f.keeper.GetReporterReliabilityState(f.ctx, reporter)
	require.True(t, found)
	require.Equal(t, int64(6), final.ReliabilityScore,
		"reporter with non-overturned FAIL should still earn -4 reward (B-F1 sibling check)")
}
