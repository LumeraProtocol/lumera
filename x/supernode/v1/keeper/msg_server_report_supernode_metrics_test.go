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
		Metrics: &types.MetricsAggregate{
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
		VersionMajor:     2,
		VersionMinor:     0,
		VersionPatch:     0,
		CpuCoresTotal:    float64(params.MinCpuCores),
		CpuUsagePercent:  float64(params.MaxCpuUsagePercent - 10),
		MemTotalGb:       float64(params.MinMemGb),
		MemUsagePercent:  float64(params.MaxMemUsagePercent - 10),
		MemFreeGb:        float64(params.MinMemGb) / 2,
		DiskTotalGb:      float64(params.MinStorageGb),
		DiskUsagePercent: float64(params.MaxStorageUsagePercent - 10),
		DiskFreeGb:       float64(params.MinStorageGb) / 2,
		UptimeSeconds:    100,
		PeersCount:       10,
	}
	for _, port := range params.RequiredOpenPorts {
		metrics.OpenPorts = append(metrics.OpenPorts, types.PortStatus{
			Port:  port,
			State: types.PortState_PORT_STATE_OPEN,
		})
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

func TestReportSupernodeMetrics_ClosedRequiredPortPostpones(t *testing.T) {
	k, ctx := keepertest.SupernodeKeeper(t)
	ctx = ctx.WithBlockHeight(100)

	valAddr := sdk.ValAddress("validator1_______________")

	supernode := types.SuperNode{
		ValidatorAddress: valAddr.String(),
		SupernodeAccount: sdk.AccAddress([]byte("supernode1")).String(),
		States: []*types.SuperNodeStateRecord{
			{
				State:  types.SuperNodeStateActive,
				Height: 10,
			},
		},
		PrevIpAddresses: []*types.IPAddressHistory{
			{Address: "127.0.0.1", Height: 10},
		},
		P2PPort: "26657",
	}

	require.NoError(t, k.SetSuperNode(ctx, supernode))
	require.NoError(t, k.SetParams(ctx, types.DefaultParams()))

	params := types.DefaultParams()
	require.NotEmpty(t, params.RequiredOpenPorts)
	metrics := types.SupernodeMetrics{
		VersionMajor:     2,
		VersionMinor:     0,
		VersionPatch:     0,
		CpuCoresTotal:    float64(params.MinCpuCores),
		CpuUsagePercent:  float64(params.MaxCpuUsagePercent - 10),
		MemTotalGb:       float64(params.MinMemGb),
		MemUsagePercent:  float64(params.MaxMemUsagePercent - 10),
		MemFreeGb:        float64(params.MinMemGb) / 2,
		DiskTotalGb:      float64(params.MinStorageGb),
		DiskUsagePercent: float64(params.MaxStorageUsagePercent - 10),
		DiskFreeGb:       float64(params.MinStorageGb) / 2,
		UptimeSeconds:    100,
		PeersCount:       10,
	}

	for i, port := range params.RequiredOpenPorts {
		state := types.PortState_PORT_STATE_OPEN
		if i == 0 {
			state = types.PortState_PORT_STATE_CLOSED
		}
		metrics.OpenPorts = append(metrics.OpenPorts, types.PortStatus{
			Port:  port,
			State: state,
		})
	}

	ms := keeper.NewMsgServerImpl(k)
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
	require.False(t, resp.Compliant)
	require.NotEmpty(t, resp.Issues)

	stored, found := k.QuerySuperNode(ctx, valAddr)
	require.True(t, found)
	require.NotEmpty(t, stored.States)
	require.Equal(t, types.SuperNodeStatePostponed, stored.States[len(stored.States)-1].State)
}

func TestReportSupernodeMetrics_EmptyPortsStillPersistsAndRecovers(t *testing.T) {
	k, ctx := keepertest.SupernodeKeeper(t)
	ctx = ctx.WithBlockHeight(100)

	valAddr := sdk.ValAddress("validator1_______________")

	supernode := types.SuperNode{
		ValidatorAddress: valAddr.String(),
		SupernodeAccount: sdk.AccAddress([]byte("supernode1")).String(),
		States: []*types.SuperNodeStateRecord{
			{State: types.SuperNodeStateActive, Height: 10},
			{State: types.SuperNodeStatePostponed, Height: 50},
		},
		PrevIpAddresses: []*types.IPAddressHistory{
			{Address: "127.0.0.1", Height: 10},
		},
		P2PPort: "26657",
	}
	require.NoError(t, k.SetSuperNode(ctx, supernode))
	require.NoError(t, k.SetParams(ctx, types.DefaultParams()))

	params := types.DefaultParams()
	metrics := types.SupernodeMetrics{
		VersionMajor:     2,
		VersionMinor:     0,
		VersionPatch:     0,
		CpuCoresTotal:    float64(params.MinCpuCores),
		CpuUsagePercent:  0, // unknown
		MemTotalGb:       float64(params.MinMemGb),
		MemUsagePercent:  0, // unknown
		MemFreeGb:        float64(params.MinMemGb) / 2,
		DiskTotalGb:      float64(params.MinStorageGb),
		DiskUsagePercent: float64(params.MaxStorageUsagePercent - 10),
		DiskFreeGb:       float64(params.MinStorageGb) / 2,
		UptimeSeconds:    100,
		PeersCount:       10,
		// Empty open_ports is allowed; should still persist and recover.
	}

	ms := keeper.NewMsgServerImpl(k)
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

	stored, found := k.QuerySuperNode(ctx, valAddr)
	require.True(t, found)
	require.Equal(t, types.SuperNodeStateActive, stored.States[len(stored.States)-1].State)

	state, ok := k.GetMetricsState(ctx, valAddr)
	require.True(t, ok, "report should persist metrics state")
	require.Equal(t, ctx.BlockHeight(), state.Height)
}
