package keeper_test

import (
	"testing"

	sdkmath "cosmossdk.io/math"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/stretchr/testify/require"

	keepertest "github.com/LumeraProtocol/lumera/testutil/keeper"
	"github.com/LumeraProtocol/lumera/x/supernode/v1/keeper"
	"github.com/LumeraProtocol/lumera/x/supernode/v1/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// TestReportSupernodeMetrics_SingleReportRecoversPostponed verifies that a single
// compliant metrics report is sufficient to recover a POSTPONED supernode.
func TestReportSupernodeMetrics_SingleReportRecoversPostponed(t *testing.T) {
	k, ctx := keepertest.SupernodeKeeper(t)

	// Set a non-zero block height to simulate an existing chain state.
	ctx = ctx.WithBlockHeight(100)

	// Create a validator and store it so the keeper can query it if needed.
	valAddr := sdk.ValAddress("validator1_______________")
	validator := stakingtypes.Validator{
		OperatorAddress: valAddr.String(),
		Tokens:          sdkmath.NewInt(1_000_000),
		Status:          stakingtypes.Bonded,
	}
	// Staking keeper is nil in the unit-test keeper setup, so we skip any
	// eligibility checks that would require it and instead focus on the
	// state transition behavior of ReportSupernodeMetrics.
	_ = validator

	supernode := types.SuperNode{
		ValidatorAddress: valAddr.String(),
		SupernodeAccount: sdk.AccAddress([]byte("supernode1")).String(),
		Note:             "1.0.0",
		States: []*types.SuperNodeStateRecord{
			{
				State:  types.SuperNodeStateActive,
				Height: 10,
			},
			{
				State:  types.SuperNodeStatePostponed,
				Height: 50,
			},
		},
		LegacyMetrics: &types.MetricsAggregate{
			Metrics:     make(map[string]float64),
			ReportCount: 1,
			Height:      1, // Very old height to simulate staleness
		},
		PrevIpAddresses: []*types.IPAddressHistory{
			{
				Address: "127.0.0.1",
				Height:  5,
			},
		},
		P2PPort: "26657",
	}

	require.NoError(t, k.SetSuperNode(ctx, supernode))

	// Use default params to define the compliance thresholds.
	require.NoError(t, k.SetParams(ctx, types.DefaultParams()))

	// Build a fully compliant metrics payload.
	params := types.DefaultParams()
	metrics := types.SupernodeMetrics{
		VersionMajor:    2,
		VersionMinor:    0,
		VersionPatch:    0,
		CpuCoresTotal:   params.MinCpuCores,
		CpuUsagePercent: params.MaxCpuUsagePercent - 10,
		MemTotalGb:      params.MinMemGb,
		MemUsagePercent: params.MaxMemUsagePercent - 10,
		MemFreeGb:       params.MinMemGb / 2,
		DiskTotalGb:     params.MinStorageGb,
		DiskUsagePercent: params.MaxStorageUsagePercent - 10,
		DiskFreeGb:      params.MinStorageGb / 2,
		UptimeSeconds:   100,
		PeersCount:      10,
	}
	for _, port := range params.RequiredOpenPorts {
		metrics.OpenPorts = append(metrics.OpenPorts, port)
	}

	ms := keeper.NewMsgServerImpl(k)

	// Ensure the context has a header (some SDK logic expects it).
	header := tmproto.Header{Height: ctx.BlockHeight()}
	ctx = ctx.WithBlockHeader(header)

	resp, err := ms.ReportSupernodeMetrics(
		sdk.WrapSDKContext(ctx),
		&types.MsgReportSupernodeMetrics{
			ValidatorAddress: supernode.ValidatorAddress,
			SupernodeAccount: supernode.SupernodeAccount,
			Metrics:          metrics,
		},
	)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.True(t, resp.Compliant)

	// The supernode should now have recovered from POSTPONED to its last
	// non-postponed state (ACTIVE) in a single report.
	stored, found := k.QuerySuperNode(ctx, valAddr)
	require.True(t, found)
	require.NotEmpty(t, stored.States)
	require.Equal(t, types.SuperNodeStateActive, stored.States[len(stored.States)-1].State)
}
