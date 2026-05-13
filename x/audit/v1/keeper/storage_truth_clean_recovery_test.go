package keeper_test

import (
	"fmt"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/LumeraProtocol/lumera/x/audit/v1/keeper"
	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
)

// Per NEW-A-18 — verifies ApplyReporterCleanEpochRecoveryAtEpochEnd grants
// the spec §15.3 single −4 reliability delta on ≥5 PASS results in the
// closing epoch with no overturned-fail.

// seedReporterPassResultsForEpoch writes count PASS records for the reporter at epochID.
func seedReporterPassResultsForEpoch(t *testing.T, f *fixture, reporterAccount string, epochID uint64, count int) {
	t.Helper()
	for i := 0; i < count; i++ {
		result := &types.StorageProofResult{
			TicketId:               fmt.Sprintf("%s-clean-%d", reporterAccount, i),
			TargetSupernodeAccount: fmt.Sprintf("target-clean-%s-%d", reporterAccount, i),
			ResultClass:            types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_PASS,
			BucketType:             types.StorageProofBucketType_STORAGE_PROOF_BUCKET_TYPE_RECENT,
		}
		require.NoError(t, keeper.SetStorageTruthReporterResultForTest(f.keeper, f.ctx, epochID, reporterAccount, result))
	}
}

func TestApplyReporterCleanEpochRecovery_AppliesMinusFourOnFivePasses(t *testing.T) {
	f := initFixture(t)
	f.ctx = f.ctx.WithBlockHeight(1).WithEventManager(sdk.NewEventManager())

	const epochID = uint64(7)
	const reporter = "reporter-clean-five"

	// Seed prior reliability score 10 so the −4 delta is observable above the 0 floor.
	require.NoError(t, f.keeper.SetReporterReliabilityState(f.ctx, types.ReporterReliabilityState{
		ReporterSupernodeAccount: reporter,
		ReliabilityScore:         10,
		LastUpdatedEpoch:         epochID, // No decay step — isolate the -4 delta
	}))

	// 5 PASS in epoch 7, no overturned-fails.
	seedReporterPassResultsForEpoch(t, f, reporter, epochID, 5)

	params := f.keeper.GetParams(f.ctx).WithDefaults()
	require.NoError(t, f.keeper.ApplyReporterCleanEpochRecoveryAtEpochEnd(f.ctx, epochID, params))

	final, found := f.keeper.GetReporterReliabilityState(f.ctx, reporter)
	require.True(t, found)
	require.Equal(t, int64(6), final.ReliabilityScore, "10 + (-4) = 6")
}

func TestApplyReporterCleanEpochRecovery_NoEffectOnFourPasses(t *testing.T) {
	f := initFixture(t)
	f.ctx = f.ctx.WithBlockHeight(1).WithEventManager(sdk.NewEventManager())

	const epochID = uint64(7)
	const reporter = "reporter-clean-four"

	require.NoError(t, f.keeper.SetReporterReliabilityState(f.ctx, types.ReporterReliabilityState{
		ReporterSupernodeAccount: reporter,
		ReliabilityScore:         10,
		LastUpdatedEpoch:         epochID, // No decay step — isolate the -4 delta
	}))

	seedReporterPassResultsForEpoch(t, f, reporter, epochID, 4) // below threshold of 5

	params := f.keeper.GetParams(f.ctx).WithDefaults()
	require.NoError(t, f.keeper.ApplyReporterCleanEpochRecoveryAtEpochEnd(f.ctx, epochID, params))

	final, found := f.keeper.GetReporterReliabilityState(f.ctx, reporter)
	require.True(t, found)
	require.Equal(t, int64(10), final.ReliabilityScore, "below 5-PASS minimum: no delta")
}

func TestApplyReporterCleanEpochRecovery_ScoresFloorAtZero(t *testing.T) {
	f := initFixture(t)
	f.ctx = f.ctx.WithBlockHeight(1).WithEventManager(sdk.NewEventManager())

	const epochID = uint64(7)
	const reporter = "reporter-clean-floor"

	// Score already at 0 — recovery should not push it negative.
	require.NoError(t, f.keeper.SetReporterReliabilityState(f.ctx, types.ReporterReliabilityState{
		ReporterSupernodeAccount: reporter,
		ReliabilityScore:         0,
		LastUpdatedEpoch:         epochID, // No decay step — isolate the -4 delta
	}))

	seedReporterPassResultsForEpoch(t, f, reporter, epochID, 5)

	params := f.keeper.GetParams(f.ctx).WithDefaults()
	require.NoError(t, f.keeper.ApplyReporterCleanEpochRecoveryAtEpochEnd(f.ctx, epochID, params))

	final, found := f.keeper.GetReporterReliabilityState(f.ctx, reporter)
	require.True(t, found)
	require.GreaterOrEqual(t, final.ReliabilityScore, int64(0), "score must not go negative")
}

func TestApplyReporterCleanEpochRecovery_CreatesStateForFreshReporter(t *testing.T) {
	f := initFixture(t)
	f.ctx = f.ctx.WithBlockHeight(1).WithEventManager(sdk.NewEventManager())

	const epochID = uint64(7)
	const reporter = "reporter-clean-fresh"

	_, found := f.keeper.GetReporterReliabilityState(f.ctx, reporter)
	require.False(t, found)
	seedReporterPassResultsForEpoch(t, f, reporter, epochID, 5)

	params := f.keeper.GetParams(f.ctx).WithDefaults()
	require.NoError(t, f.keeper.ApplyReporterCleanEpochRecoveryAtEpochEnd(f.ctx, epochID, params))

	final, found := f.keeper.GetReporterReliabilityState(f.ctx, reporter)
	require.True(t, found, "fresh reporter with clean epoch should get an explicit dashboard state row")
	require.Equal(t, int64(0), final.ReliabilityScore)
	require.Equal(t, epochID, final.LastUpdatedEpoch)
	require.Equal(t, uint32(1), final.WindowPositiveCount)
}
