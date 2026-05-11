package keeper

import (
	"testing"

	lcfg "github.com/LumeraProtocol/lumera/config"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

func TestQuerySNEligibility_UsesAuditReportAndStateGate(t *testing.T) {
	k, ctx, _, snKeeper, auditKeeper := setupTestKeeper(t)
	q := NewQueryServerImpl(k)

	params := k.GetParams(ctx)
	params.MetricsFreshnessMaxBlocks = 100
	params.RewardDistribution.MinCascadeBytesForPayment = 1_000
	require.NoError(t, k.SetParams(ctx, params))

	val := makeValAddr(1)
	acc := makeAccAddr(1)
	addSupernode(snKeeper, auditKeeper, val, acc, sntypes.SuperNodeStateActive, 2_000)

	valBech32, err := sdk.Bech32ifyAddressBytes(lcfg.Bech32ValidatorAddressPrefix, val)
	require.NoError(t, err)

	resp, err := q.SNEligibility(ctx, &sntypes.QuerySNEligibilityRequest{ValidatorAddress: valBech32})
	require.NoError(t, err)
	require.True(t, resp.Eligible)
	require.Equal(t, "eligible", resp.Reason)
}

func TestQuerySNEligibility_RejectsStaleAuditReport(t *testing.T) {
	k, ctx, _, snKeeper, auditKeeper := setupTestKeeper(t)
	q := NewQueryServerImpl(k)

	params := k.GetParams(ctx)
	params.MetricsFreshnessMaxBlocks = 5
	params.RewardDistribution.MinCascadeBytesForPayment = 1_000
	require.NoError(t, k.SetParams(ctx, params))

	// Add supernode at the initial ctx height so MetricsState.Height pins low,
	// then query at height 100 — staleness 99 > MetricsFreshnessMaxBlocks(5) → stale.
	val := makeValAddr(2)
	acc := makeAccAddr(2)
	addSupernode(snKeeper, auditKeeper, val, acc, sntypes.SuperNodeStateActive, 5_000)

	ctx = ctx.WithBlockHeight(100)
	snKeeper.ctx = ctx

	valBech32, err := sdk.Bech32ifyAddressBytes(lcfg.Bech32ValidatorAddressPrefix, val)
	require.NoError(t, err)

	resp, err := q.SNEligibility(ctx, &sntypes.QuerySNEligibilityRequest{ValidatorAddress: valBech32})
	require.NoError(t, err)
	require.False(t, resp.Eligible)
	require.Equal(t, "audit report is stale", resp.Reason)
}

func TestQuerySNEligibility_RejectsPostponedState(t *testing.T) {
	k, ctx, _, snKeeper, auditKeeper := setupTestKeeper(t)
	q := NewQueryServerImpl(k)

	params := k.GetParams(ctx)
	params.MetricsFreshnessMaxBlocks = 100
	params.RewardDistribution.MinCascadeBytesForPayment = 1_000
	require.NoError(t, k.SetParams(ctx, params))

	val := makeValAddr(3)
	acc := makeAccAddr(3)
	addSupernode(snKeeper, auditKeeper, val, acc, sntypes.SuperNodeStatePostponed, 9_000)

	valBech32, err := sdk.Bech32ifyAddressBytes(lcfg.Bech32ValidatorAddressPrefix, val)
	require.NoError(t, err)

	resp, err := q.SNEligibility(ctx, &sntypes.QuerySNEligibilityRequest{ValidatorAddress: valBech32})
	require.NoError(t, err)
	require.False(t, resp.Eligible)
	require.Equal(t, "supernode state is not eligible", resp.Reason)
}
