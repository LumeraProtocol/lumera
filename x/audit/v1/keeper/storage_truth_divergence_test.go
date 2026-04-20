package keeper_test

import (
	"testing"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

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

	require.NoError(t, f.keeper.SetReporterReliabilityState(f.ctx, types.ReporterReliabilityState{
		ReporterSupernodeAccount: "reporter-a",
		ReliabilityScore:         0,
		LastUpdatedEpoch:         0,
		WindowPositiveCount:      8,
		WindowNegativeCount:      2,
	}))
	require.NoError(t, f.keeper.SetReporterReliabilityState(f.ctx, types.ReporterReliabilityState{
		ReporterSupernodeAccount: "reporter-b",
		ReliabilityScore:         0,
		LastUpdatedEpoch:         0,
		WindowPositiveCount:      8,
		WindowNegativeCount:      2,
	}))
	require.NoError(t, f.keeper.SetReporterReliabilityState(f.ctx, types.ReporterReliabilityState{
		ReporterSupernodeAccount: "reporter-c",
		ReliabilityScore:         0,
		LastUpdatedEpoch:         0,
		WindowPositiveCount:      1,
		WindowNegativeCount:      9,
	}))

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
		ReliabilityScore:         0,
		LastUpdatedEpoch:         0,
		WindowPositiveCount:      1,
		WindowNegativeCount:      3, // total=4 < minReports=5
	}))

	require.NoError(t, f.keeper.ApplyReporterDivergenceAtEpochEnd(f.ctx, 1, params))

	state, found := f.keeper.GetReporterReliabilityState(f.ctx, "reporter-low-volume")
	require.True(t, found)
	require.Equal(t, int64(0), state.ReliabilityScore, "low-volume reporter should not be penalized")
}
