package keeper

import (
	"strings"
	"testing"

	"cosmossdk.io/log"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/stretchr/testify/require"

	"github.com/LumeraProtocol/lumera/x/supernode/v1/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func TestEvaluateCompliancePassesWithValidMetrics(t *testing.T) {
	ctx := sdk.NewContext(nil, tmproto.Header{Height: 10}, false, log.NewNopLogger())
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

	issues := evaluateCompliance(ctx, params, metrics)

	require.Empty(t, issues)
}

func TestEvaluateComplianceIgnoresZeroCpuAndMemUsage(t *testing.T) {
	ctx := sdk.NewContext(nil, tmproto.Header{Height: 10}, false, log.NewNopLogger())
	params := types.DefaultParams()

	metrics := types.SupernodeMetrics{
		VersionMajor:     2,
		VersionMinor:     0,
		VersionPatch:     0,
		CpuCoresTotal:    float64(params.MinCpuCores),
		CpuUsagePercent:  0, // treated as unknown
		MemTotalGb:       float64(params.MinMemGb),
		MemUsagePercent:  0, // treated as unknown
		MemFreeGb:        float64(params.MinMemGb) / 2,
		DiskTotalGb:      float64(params.MinStorageGb),
		DiskUsagePercent: float64(params.MaxStorageUsagePercent - 10),
		DiskFreeGb:       float64(params.MinStorageGb) / 2,
		UptimeSeconds:    100,
		PeersCount:       10,
	}

	issues := evaluateCompliance(ctx, params, metrics)

	require.False(t, containsSubstring(issues, "cpu usage"), "cpu usage should be ignored when 0, issues=%v", issues)
	require.False(t, containsSubstring(issues, "mem usage"), "mem usage should be ignored when 0, issues=%v", issues)
}

func TestEvaluateComplianceDetectsStaleMetrics(t *testing.T) {
	params := types.DefaultParams()
	ctx := sdk.NewContext(nil, tmproto.Header{Height: 10}, false, log.NewNopLogger())
	t.Logf("min supernode version: %s", params.MinSupernodeVersion)

	metrics := types.SupernodeMetrics{
		VersionMajor:     1,
		VersionMinor:     0,
		VersionPatch:     0,
		CpuCoresTotal:    float64(params.MinCpuCores),
		CpuUsagePercent:  float64(params.MaxCpuUsagePercent - 10),
		MemTotalGb:       float64(params.MinMemGb),
		MemUsagePercent:  float64(params.MaxMemUsagePercent - 10),
		DiskTotalGb:      float64(params.MinStorageGb),
		DiskUsagePercent: float64(params.MaxStorageUsagePercent - 10),
		UptimeSeconds:    100,
		PeersCount:       5,
	}
	for _, port := range params.RequiredOpenPorts {
		metrics.OpenPorts = append(metrics.OpenPorts, types.PortStatus{
			Port:  port,
			State: types.PortState_PORT_STATE_OPEN,
		})
	}

	issues := evaluateCompliance(ctx, params, metrics)

	require.NotEmpty(t, issues)
	require.True(t, containsSubstring(issues, "version"), "expected version-related issue, got: %v", issues)
}

func TestEvaluateComplianceRequiresOpenPorts(t *testing.T) {
	ctx := sdk.NewContext(nil, tmproto.Header{Height: 10}, false, log.NewNopLogger())
	params := types.DefaultParams()

	metrics := types.SupernodeMetrics{
		VersionMajor:     2,
		VersionMinor:     0,
		VersionPatch:     0,
		CpuCoresTotal:    float64(params.MinCpuCores),
		CpuUsagePercent:  float64(params.MaxCpuUsagePercent - 10),
		MemTotalGb:       float64(params.MinMemGb),
		MemUsagePercent:  float64(params.MaxMemUsagePercent - 10),
		DiskTotalGb:      float64(params.MinStorageGb),
		DiskUsagePercent: float64(params.MaxStorageUsagePercent - 10),
		UptimeSeconds:    100,
		PeersCount:       5,
	}
	// Explicitly report a required port as CLOSED. This should cause non-compliance.
	require.NotEmpty(t, params.RequiredOpenPorts)
	metrics.OpenPorts = append(metrics.OpenPorts, types.PortStatus{
		Port:  params.RequiredOpenPorts[0],
		State: types.PortState_PORT_STATE_CLOSED,
	})

	issues := evaluateCompliance(ctx, params, metrics)

	require.True(t, containsSubstring(issues, "required port"))
}

func containsSubstring(items []string, substr string) bool {
	for _, item := range items {
		if strings.Contains(item, substr) {
			return true
		}
	}
	return false
}
