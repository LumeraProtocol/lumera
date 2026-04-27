package keeper

import (
	"testing"

	sdkmath "cosmossdk.io/math"
	lcfg "github.com/LumeraProtocol/lumera/config"
	sdk "github.com/cosmos/cosmos-sdk/types"
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

	ctx = ctx.WithBlockHeight(100)
	snKeeper.ctx = ctx

	val := makeValAddr(1)
	acc := makeAccAddr(1)
	addSupernode(snKeeper, auditKeeper, val, acc, sntypes.SuperNodeStateActive, 10_000)

	accBech32, err := sdk.Bech32ifyAddressBytes(lcfg.AccountAddressPrefix, acc)
	require.NoError(t, err)
	auditKeeper.setReport(auditKeeper.currentEpochID, accBech32, 90, 10_000) // stale by 10 blocks

	fundPool(bankKeeper, 1_000_000)
	err = k.distributePool(ctx)
	require.NoError(t, err)

	paid := sdkmath.ZeroInt()
	for _, s := range bankKeeper.sent {
		if s.to == acc.String() {
			paid = paid.Add(s.amount.AmountOf(lcfg.ChainDenom))
		}
	}
	require.True(t, paid.IsZero(), "stale-report SN must not be paid")
}
