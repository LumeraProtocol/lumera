package keeper_test

import (
	"fmt"
	"testing"

	"github.com/LumeraProtocol/lumera/x/audit/v1/keeper"
	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

// addDivergenceRecords stores (negCount) HASH_MISMATCH + (posCount) PASS records for
// reporterAccount in epoch 1 so that storageTruthReporterDivergenceStats returns real counts.
func addDivergenceRecords(t *testing.T, f *fixture, reporterAccount string, negCount, posCount int) {
	t.Helper()
	for i := 0; i < negCount; i++ {
		result := &types.StorageProofResult{
			TicketId:               fmt.Sprintf("%s-fail-%d", reporterAccount, i),
			TargetSupernodeAccount: fmt.Sprintf("target-%s-%d", reporterAccount, i),
			ResultClass:            types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_HASH_MISMATCH,
			BucketType:             types.StorageProofBucketType_STORAGE_PROOF_BUCKET_TYPE_RECENT,
		}
		require.NoError(t, keeper.SetStorageTruthReporterResultForTest(f.keeper, f.ctx, 1, reporterAccount, result))
	}
	for i := 0; i < posCount; i++ {
		result := &types.StorageProofResult{
			TicketId:               fmt.Sprintf("%s-pass-%d", reporterAccount, i),
			TargetSupernodeAccount: fmt.Sprintf("target-%s-%d", reporterAccount, i),
			ResultClass:            types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_PASS,
			BucketType:             types.StorageProofBucketType_STORAGE_PROOF_BUCKET_TYPE_RECENT,
		}
		require.NoError(t, keeper.SetStorageTruthReporterResultForTest(f.keeper, f.ctx, 1, reporterAccount, result))
	}
}

func TestApplyReporterDivergenceAtEpochEnd_PenalizesChronic(t *testing.T) {
	f := initFixture(t)
	f.ctx = f.ctx.WithBlockHeight(1).WithEventManager(sdk.NewEventManager())

	params := f.keeper.GetParams(f.ctx).WithDefaults()
	params.StorageTruthReporterMinReportsForDivergence = 5
	require.NoError(t, f.keeper.SetParams(f.ctx, params))

	// 3 reporters with sufficient volume.
	// Reporter A: 2 negative out of 10 = 20% neg rate (low, will be "normal")
	// Reporter B: 2 negative out of 10 = 20% neg rate (same)
	// Reporter C: 9 negative out of 10 = 90% neg rate (outlier: > 2x median of 20%)

	// Seed ReporterReliabilityState so GetAllReporterReliabilityStates returns all three.
	require.NoError(t, f.keeper.SetReporterReliabilityState(f.ctx, types.ReporterReliabilityState{
		ReporterSupernodeAccount: "reporter-a",
	}))
	require.NoError(t, f.keeper.SetReporterReliabilityState(f.ctx, types.ReporterReliabilityState{
		ReporterSupernodeAccount: "reporter-b",
	}))
	require.NoError(t, f.keeper.SetReporterReliabilityState(f.ctx, types.ReporterReliabilityState{
		ReporterSupernodeAccount: "reporter-c",
	}))

	// Populate real per-record stats (required after 121-F15 removed the window fallback).
	addDivergenceRecords(t, f, "reporter-a", 2, 8) // 2 neg / 10 total = 20%
	addDivergenceRecords(t, f, "reporter-b", 2, 8) // 2 neg / 10 total = 20%
	addDivergenceRecords(t, f, "reporter-c", 9, 1) // 9 neg / 10 total = 90%

	require.NoError(t, f.keeper.ApplyReporterDivergenceAtEpochEnd(f.ctx, 1, params))

	// reporter-a and reporter-b: not penalized (neg_rate 0.2 <= 2 * median_neg_rate 0.2).
	stateA, found := f.keeper.GetReporterReliabilityState(f.ctx, "reporter-a")
	require.True(t, found)
	require.Equal(t, int64(0), stateA.ReliabilityScore, "reporter-a should not be penalized")

	stateB, found := f.keeper.GetReporterReliabilityState(f.ctx, "reporter-b")
	require.True(t, found)
	require.Equal(t, int64(0), stateB.ReliabilityScore, "reporter-b should not be penalized")

	// reporter-c: penalized +8 for divergence (neg_rate 0.9 > 2 * median 0.2).
	stateC, found := f.keeper.GetReporterReliabilityState(f.ctx, "reporter-c")
	require.True(t, found)
	require.Equal(t, int64(8), stateC.ReliabilityScore, "reporter-c should be penalized +8 for divergence")
}

func TestApplyReporterDivergenceAtEpochEnd_SkipsInsufficientVolume(t *testing.T) {
	f := initFixture(t)
	f.ctx = f.ctx.WithBlockHeight(1).WithEventManager(sdk.NewEventManager())

	params := f.keeper.GetParams(f.ctx).WithDefaults()
	params.StorageTruthReporterMinReportsForDivergence = 5
	require.NoError(t, f.keeper.SetParams(f.ctx, params))

	// Reporter with only 4 reports — below the minimum volume threshold.
	require.NoError(t, f.keeper.SetReporterReliabilityState(f.ctx, types.ReporterReliabilityState{
		ReporterSupernodeAccount: "reporter-low-volume",
	}))
	addDivergenceRecords(t, f, "reporter-low-volume", 3, 1) // 4 records total < minReports=5

	require.NoError(t, f.keeper.ApplyReporterDivergenceAtEpochEnd(f.ctx, 1, params))

	state, found := f.keeper.GetReporterReliabilityState(f.ctx, "reporter-low-volume")
	require.True(t, found)
	require.Equal(t, int64(0), state.ReliabilityScore, "low-volume reporter should not be penalized")
}
