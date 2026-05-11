package keeper

import (
	"testing"

	sdkmath "cosmossdk.io/math"
	lcfg "github.com/LumeraProtocol/lumera/config"
	"github.com/stretchr/testify/require"

	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

func TestDistributePool_SkipsStaleAuditReports(t *testing.T) {
	k, ctx, bankKeeper, snKeeper, auditKeeper := setupTestKeeper(t)

	params := sntypes.DefaultParams()
	params.RewardDistribution.PaymentPeriodBlocks = 1
	params.RewardDistribution.MinCascadeBytesForPayment = 1_000
	params.MetricsFreshnessMaxBlocks = 5
	require.NoError(t, k.SetParams(ctx, params))

	// Add supernode at the initial ctx height so MetricsState.Height pins low,
	// then run distributePool at height 100 — staleness 99 > MetricsFreshnessMaxBlocks(5).
	val := makeValAddr(1)
	acc := makeAccAddr(1)
	addSupernode(snKeeper, auditKeeper, val, acc, sntypes.SuperNodeStateActive, 10_000)

	ctx = ctx.WithBlockHeight(100)
	snKeeper.ctx = ctx

	fundPool(bankKeeper, 1_000_000)
	err := k.distributePool(ctx)
	require.NoError(t, err)

	paid := sdkmath.ZeroInt()
	for _, s := range bankKeeper.sent {
		if s.to == acc.String() {
			paid = paid.Add(s.amount.AmountOf(lcfg.ChainDenom))
		}
	}
	require.True(t, paid.IsZero(), "stale-report SN must not be paid")
}
