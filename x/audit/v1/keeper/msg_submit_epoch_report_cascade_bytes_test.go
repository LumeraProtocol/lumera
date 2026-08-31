package keeper_test

import (
	"math"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/LumeraProtocol/lumera/x/audit/v1/keeper"
	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

// TestSubmitEpochReport_CascadeBytes_HappyPersistedAndBridged covers I1 happy
// path (valid finite value accepted) and I2 bridge happy path (value lands in
// SupernodeMetricsState with the reporter's validator address and current block
// height).
func TestSubmitEpochReport_CascadeBytes_HappyPersistedAndBridged(t *testing.T) {
	f := initFixture(t)
	f.ctx = f.ctx.WithBlockHeight(123)

	ms := keeper.NewMsgServerImpl(f.keeper)

	reporter := sdk.AccAddress([]byte("reporter_address_20b")).String()
	valAddrStr := sdk.ValAddress([]byte("validator_addr__20b")).String()

	reporterSN := sntypes.SuperNode{
		SupernodeAccount: reporter,
		ValidatorAddress: valAddrStr,
	}
	f.supernodeKeeper.EXPECT().
		GetSuperNodeByAccount(gomock.Any(), reporter).
		Return(reporterSN, true, nil).
		AnyTimes()
	f.supernodeKeeper.EXPECT().
		GetMetricsState(gomock.Any(), gomock.Any()).
		Return(sntypes.SupernodeMetricsState{}, false).
		AnyTimes()

	const cascadeBytes float64 = 2_147_483_648 // 2 GB

	var captured sntypes.SupernodeMetricsState
	f.supernodeKeeper.EXPECT().
		SetMetricsState(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ sdk.Context, state sntypes.SupernodeMetricsState) error {
			captured = state
			return nil
		}).
		Times(1)

	seedEpochAnchorForReportTest(t, f, 0, []string{reporter}, []string{reporter})

	_, err := ms.SubmitEpochReport(f.ctx, &types.MsgSubmitEpochReport{
		Creator: reporter,
		EpochId: 0,
		HostReport: types.HostReport{
			CascadeKademliaDbBytes: cascadeBytes,
		},
	})
	require.NoError(t, err)

	require.Equal(t, valAddrStr, captured.ValidatorAddress)
	require.NotNil(t, captured.Metrics)
	require.Equal(t, cascadeBytes, captured.Metrics.CascadeKademliaDbBytes)
	require.EqualValues(t, 123, captured.Height)
	require.EqualValues(t, 1, captured.ReportCount)
}

// TestSubmitEpochReport_CascadeBytes_ZeroAccepted covers I1: zero is a valid
// value (empty Kademlia store) and the bridge persists it.
func TestSubmitEpochReport_CascadeBytes_ZeroAccepted(t *testing.T) {
	f := initFixture(t)
	f.ctx = f.ctx.WithBlockHeight(1)
	ms := keeper.NewMsgServerImpl(f.keeper)

	reporter := sdk.AccAddress([]byte("reporter_address_20b")).String()
	valAddrStr := sdk.ValAddress([]byte("validator_addr__20b")).String()

	f.supernodeKeeper.EXPECT().
		GetSuperNodeByAccount(gomock.Any(), reporter).
		Return(sntypes.SuperNode{SupernodeAccount: reporter, ValidatorAddress: valAddrStr}, true, nil).
		AnyTimes()
	f.supernodeKeeper.EXPECT().
		GetMetricsState(gomock.Any(), gomock.Any()).
		Return(sntypes.SupernodeMetricsState{}, false).
		AnyTimes()
	f.supernodeKeeper.EXPECT().
		SetMetricsState(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ sdk.Context, state sntypes.SupernodeMetricsState) error {
			require.NotNil(t, state.Metrics)
			require.Equal(t, float64(0), state.Metrics.CascadeKademliaDbBytes)
			return nil
		}).
		Times(1)

	seedEpochAnchorForReportTest(t, f, 0, []string{reporter}, []string{reporter})

	_, err := ms.SubmitEpochReport(f.ctx, &types.MsgSubmitEpochReport{
		Creator:    reporter,
		EpochId:    0,
		HostReport: types.HostReport{CascadeKademliaDbBytes: 0},
	})
	require.NoError(t, err)
}

// TestSubmitEpochReport_CascadeBytes_InvalidValuesRejected covers I1 violation
// tests: NaN, +Inf, -Inf, and negative values are all rejected, with no bridge
// write attempted. One violation cell per row of the invariant table.
func TestSubmitEpochReport_CascadeBytes_InvalidValuesRejected(t *testing.T) {
	cases := []struct {
		name  string
		value float64
	}{
		{"NaN", math.NaN()},
		{"+Inf", math.Inf(1)},
		{"-Inf", math.Inf(-1)},
		{"negative", -1.0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := initFixture(t)
			f.ctx = f.ctx.WithBlockHeight(1)
			ms := keeper.NewMsgServerImpl(f.keeper)

			reporter := sdk.AccAddress([]byte("reporter_address_20b")).String()
			valAddrStr := sdk.ValAddress([]byte("validator_addr__20b")).String()

			f.supernodeKeeper.EXPECT().
				GetSuperNodeByAccount(gomock.Any(), reporter).
				Return(sntypes.SuperNode{SupernodeAccount: reporter, ValidatorAddress: valAddrStr}, true, nil).
				AnyTimes()
			// Bridge must NOT be invoked for rejected inputs.
			f.supernodeKeeper.EXPECT().
				SetMetricsState(gomock.Any(), gomock.Any()).
				Times(0)

			seedEpochAnchorForReportTest(t, f, 0, []string{reporter}, []string{reporter})

			_, err := ms.SubmitEpochReport(f.ctx, &types.MsgSubmitEpochReport{
				Creator:    reporter,
				EpochId:    0,
				HostReport: types.HostReport{CascadeKademliaDbBytes: tc.value},
			})
			require.Error(t, err)
			require.ErrorIs(t, err, types.ErrInvalidHostMetric)
		})
	}
}

func TestSubmitEpochReport_HostUsagePercent_InvalidValuesRejected(t *testing.T) {
	cases := []struct {
		name   string
		report types.HostReport
	}{
		{"cpu_nan", types.HostReport{CpuUsagePercent: math.NaN()}},
		{"cpu_negative", types.HostReport{CpuUsagePercent: -1}},
		{"cpu_over_100", types.HostReport{CpuUsagePercent: 100.1}},
		{"mem_inf", types.HostReport{MemUsagePercent: math.Inf(1)}},
		{"mem_negative", types.HostReport{MemUsagePercent: -1}},
		{"mem_over_100", types.HostReport{MemUsagePercent: 100.1}},
		{"disk_inf", types.HostReport{DiskUsagePercent: math.Inf(-1)}},
		{"disk_negative", types.HostReport{DiskUsagePercent: -1}},
		{"disk_over_100", types.HostReport{DiskUsagePercent: 100.1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := initFixture(t)
			f.ctx = f.ctx.WithBlockHeight(1)
			ms := keeper.NewMsgServerImpl(f.keeper)

			reporter := sdk.AccAddress([]byte("reporter_address_20b")).String()
			valAddrStr := sdk.ValAddress([]byte("validator_addr__20b")).String()

			f.supernodeKeeper.EXPECT().
				GetSuperNodeByAccount(gomock.Any(), reporter).
				Return(sntypes.SuperNode{SupernodeAccount: reporter, ValidatorAddress: valAddrStr}, true, nil).
				AnyTimes()
			f.supernodeKeeper.EXPECT().
				SetMetricsState(gomock.Any(), gomock.Any()).
				Times(0)

			_, err := ms.SubmitEpochReport(f.ctx, &types.MsgSubmitEpochReport{
				Creator:    reporter,
				EpochId:    0,
				HostReport: tc.report,
			})
			require.Error(t, err)
			require.ErrorIs(t, err, types.ErrInvalidHostMetric)
		})
	}
}

// TestSubmitEpochReport_Bridge_PreservesPriorNonCascadeMetrics covers I3: the
// bridge is read-modify-write; pre-existing metrics fields owned by other
// writers must not be clobbered to zero by the cascade-bytes write.
func TestSubmitEpochReport_Bridge_PreservesPriorNonCascadeMetrics(t *testing.T) {
	f := initFixture(t)
	f.ctx = f.ctx.WithBlockHeight(7)
	ms := keeper.NewMsgServerImpl(f.keeper)

	reporter := sdk.AccAddress([]byte("reporter_address_20b")).String()
	valAddrStr := sdk.ValAddress([]byte("validator_addr__20b")).String()

	priorState := sntypes.SupernodeMetricsState{
		ValidatorAddress: valAddrStr,
		Metrics: &sntypes.SupernodeMetrics{
			CpuUsagePercent:        42.0,
			DiskUsagePercent:       50.0,
			UptimeSeconds:          9999,
			CascadeKademliaDbBytes: 100, // will be overwritten
		},
		ReportCount: 5,
		Height:      6,
	}

	f.supernodeKeeper.EXPECT().
		GetSuperNodeByAccount(gomock.Any(), reporter).
		Return(sntypes.SuperNode{SupernodeAccount: reporter, ValidatorAddress: valAddrStr}, true, nil).
		AnyTimes()
	f.supernodeKeeper.EXPECT().
		GetMetricsState(gomock.Any(), gomock.Any()).
		Return(priorState, true).
		AnyTimes()

	var captured sntypes.SupernodeMetricsState
	f.supernodeKeeper.EXPECT().
		SetMetricsState(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ sdk.Context, state sntypes.SupernodeMetricsState) error {
			captured = state
			return nil
		}).
		Times(1)

	seedEpochAnchorForReportTest(t, f, 0, []string{reporter}, []string{reporter})

	const newBytes float64 = 4_294_967_296 // 4 GB
	_, err := ms.SubmitEpochReport(f.ctx, &types.MsgSubmitEpochReport{
		Creator:    reporter,
		EpochId:    0,
		HostReport: types.HostReport{CascadeKademliaDbBytes: newBytes},
	})
	require.NoError(t, err)

	// CascadeKademliaDbBytes overwritten with the new value.
	require.Equal(t, newBytes, captured.Metrics.CascadeKademliaDbBytes)
	// Non-cascade fields preserved (I3).
	require.Equal(t, 42.0, captured.Metrics.CpuUsagePercent)
	require.Equal(t, 50.0, captured.Metrics.DiskUsagePercent)
	require.EqualValues(t, 9999, captured.Metrics.UptimeSeconds)
	// Height bumped to current block; ReportCount incremented from prior 5 to 6.
	require.EqualValues(t, 7, captured.Height)
	require.EqualValues(t, 6, captured.ReportCount)
}

// TestSubmitEpochReport_Bridge_EmptyValidatorAddressNoOp covers the defensive
// no-op path: a SuperNode record with an empty ValidatorAddress (a x/supernode
// data invariant violation) must NOT fail the epoch report; the bridge skips
// with an event but the report is still accepted.
func TestSubmitEpochReport_Bridge_EmptyValidatorAddressNoOp(t *testing.T) {
	f := initFixture(t)
	f.ctx = f.ctx.WithBlockHeight(1).WithEventManager(sdk.NewEventManager())
	ms := keeper.NewMsgServerImpl(f.keeper)

	reporter := sdk.AccAddress([]byte("reporter_address_20b")).String()

	f.supernodeKeeper.EXPECT().
		GetSuperNodeByAccount(gomock.Any(), reporter).
		Return(sntypes.SuperNode{SupernodeAccount: reporter, ValidatorAddress: ""}, true, nil).
		AnyTimes()
	// Bridge MUST NOT call GetMetricsState / SetMetricsState when val addr is empty.
	f.supernodeKeeper.EXPECT().
		GetMetricsState(gomock.Any(), gomock.Any()).
		Times(0)
	f.supernodeKeeper.EXPECT().
		SetMetricsState(gomock.Any(), gomock.Any()).
		Times(0)

	seedEpochAnchorForReportTest(t, f, 0, []string{reporter}, []string{reporter})

	_, err := ms.SubmitEpochReport(f.ctx, &types.MsgSubmitEpochReport{
		Creator:    reporter,
		EpochId:    0,
		HostReport: types.HostReport{CascadeKademliaDbBytes: 1024},
	})
	require.NoError(t, err)

	// Skip event emitted for observability.
	foundEvent := false
	for _, e := range f.ctx.EventManager().Events() {
		if e.Type == "audit_cascade_bytes_bridge_skipped" {
			foundEvent = true
			break
		}
	}
	require.True(t, foundEvent, "expected audit_cascade_bytes_bridge_skipped event")
}
