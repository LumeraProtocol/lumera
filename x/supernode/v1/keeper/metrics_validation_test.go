package keeper

import (
	"math"
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

	result := evaluateCompliance(ctx, params, metrics)

	require.True(t, result.IsCompliant())
	require.Empty(t, result.Issues)
	require.False(t, result.StorageFull)
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

	result := evaluateCompliance(ctx, params, metrics)

	require.False(t, containsSubstring(result.Issues, "cpu usage"), "cpu usage should be ignored when 0, issues=%v", result.Issues)
	require.False(t, containsSubstring(result.Issues, "mem usage"), "mem usage should be ignored when 0, issues=%v", result.Issues)
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

	result := evaluateCompliance(ctx, params, metrics)

	require.NotEmpty(t, result.Issues)
	require.True(t, containsSubstring(result.Issues, "version"), "expected version-related issue, got: %v", result.Issues)
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

	result := evaluateCompliance(ctx, params, metrics)

	require.True(t, containsSubstring(result.Issues, "required port"))
}

func TestEvaluateComplianceStorageFullOnly(t *testing.T) {
	ctx := sdk.NewContext(nil, tmproto.Header{Height: 10}, false, log.NewNopLogger())
	params := types.DefaultParams()
	params.CascadeKademliaDbMaxBytes = 1_000_000_000 // 1 GB threshold

	metrics := types.SupernodeMetrics{
		VersionMajor:           2,
		VersionMinor:           0,
		VersionPatch:           0,
		CpuCoresTotal:          float64(params.MinCpuCores),
		CpuUsagePercent:        float64(params.MaxCpuUsagePercent - 10),
		MemTotalGb:             float64(params.MinMemGb),
		MemUsagePercent:        float64(params.MaxMemUsagePercent - 10),
		MemFreeGb:              float64(params.MinMemGb) / 2,
		DiskTotalGb:            float64(params.MinStorageGb),
		DiskUsagePercent:       float64(params.MaxStorageUsagePercent - 10),
		DiskFreeGb:             float64(params.MinStorageGb) / 2,
		UptimeSeconds:          100,
		PeersCount:             10,
		CascadeKademliaDbBytes: 1_500_000_000, // exceeds threshold
	}
	for _, port := range params.RequiredOpenPorts {
		metrics.OpenPorts = append(metrics.OpenPorts, types.PortStatus{
			Port:  port,
			State: types.PortState_PORT_STATE_OPEN,
		})
	}

	result := evaluateCompliance(ctx, params, metrics)

	// Storage full but no other issues.
	require.Empty(t, result.Issues, "expected no non-storage issues, got: %v", result.Issues)
	require.True(t, result.StorageFull, "expected StorageFull=true")
	require.False(t, result.IsCompliant(), "should not be fully compliant when storage full")
}

func TestEvaluateComplianceStorageFullPlusOtherIssue(t *testing.T) {
	ctx := sdk.NewContext(nil, tmproto.Header{Height: 10}, false, log.NewNopLogger())
	params := types.DefaultParams()
	params.CascadeKademliaDbMaxBytes = 1_000_000_000

	metrics := types.SupernodeMetrics{
		VersionMajor:           2,
		VersionMinor:           0,
		VersionPatch:           0,
		CpuCoresTotal:          float64(params.MinCpuCores),
		CpuUsagePercent:        float64(params.MaxCpuUsagePercent - 10),
		MemTotalGb:             float64(params.MinMemGb),
		MemUsagePercent:        float64(params.MaxMemUsagePercent - 10),
		MemFreeGb:              float64(params.MinMemGb) / 2,
		DiskTotalGb:            float64(params.MinStorageGb),
		DiskUsagePercent:       float64(params.MaxStorageUsagePercent - 10),
		DiskFreeGb:             float64(params.MinStorageGb) / 2,
		UptimeSeconds:          100,
		PeersCount:             0, // fails peers check
		CascadeKademliaDbBytes: 1_500_000_000,
	}
	for _, port := range params.RequiredOpenPorts {
		metrics.OpenPorts = append(metrics.OpenPorts, types.PortStatus{
			Port:  port,
			State: types.PortState_PORT_STATE_OPEN,
		})
	}

	result := evaluateCompliance(ctx, params, metrics)

	require.NotEmpty(t, result.Issues, "expected non-storage issues")
	require.True(t, result.StorageFull, "expected StorageFull=true")
	require.True(t, containsSubstring(result.Issues, "peers_count"))
}

func TestEvaluateComplianceStorageFullDisabledWhenZeroThreshold(t *testing.T) {
	ctx := sdk.NewContext(nil, tmproto.Header{Height: 10}, false, log.NewNopLogger())
	params := types.DefaultParams()
	// Default: CascadeKademliaDbMaxBytes == 0 (disabled)

	metrics := types.SupernodeMetrics{
		VersionMajor:           2,
		VersionMinor:           0,
		VersionPatch:           0,
		CpuCoresTotal:          float64(params.MinCpuCores),
		CpuUsagePercent:        float64(params.MaxCpuUsagePercent - 10),
		MemTotalGb:             float64(params.MinMemGb),
		MemUsagePercent:        float64(params.MaxMemUsagePercent - 10),
		MemFreeGb:              float64(params.MinMemGb) / 2,
		DiskTotalGb:            float64(params.MinStorageGb),
		DiskUsagePercent:       float64(params.MaxStorageUsagePercent - 10),
		DiskFreeGb:             float64(params.MinStorageGb) / 2,
		UptimeSeconds:          100,
		PeersCount:             10,
		CascadeKademliaDbBytes: 999_999_999_999, // huge, but threshold disabled
	}
	for _, port := range params.RequiredOpenPorts {
		metrics.OpenPorts = append(metrics.OpenPorts, types.PortStatus{
			Port:  port,
			State: types.PortState_PORT_STATE_OPEN,
		})
	}

	result := evaluateCompliance(ctx, params, metrics)

	require.True(t, result.IsCompliant(), "should be compliant when threshold is disabled")
	require.False(t, result.StorageFull)
}

func TestEvaluateComplianceRejectsInvalidCascadeKademliaBytes(t *testing.T) {
	ctx := sdk.NewContext(nil, tmproto.Header{Height: 10}, false, log.NewNopLogger())
	params := types.DefaultParams()
	params.CascadeKademliaDbMaxBytes = 1_000_000_000

	baseMetrics := types.SupernodeMetrics{
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
		baseMetrics.OpenPorts = append(baseMetrics.OpenPorts, types.PortStatus{
			Port:  port,
			State: types.PortState_PORT_STATE_OPEN,
		})
	}

	t.Run("nan", func(t *testing.T) {
		metrics := baseMetrics
		metrics.CascadeKademliaDbBytes = math.NaN()

		result := evaluateCompliance(ctx, params, metrics)

		require.True(t, containsSubstring(result.Issues, "invalid numeric value for cascade_kademlia_db_bytes"))
		require.False(t, result.IsCompliant())
	})

	t.Run("negative", func(t *testing.T) {
		metrics := baseMetrics
		metrics.CascadeKademliaDbBytes = -1

		result := evaluateCompliance(ctx, params, metrics)

		require.True(t, containsSubstring(result.Issues, "cascade_kademlia_db_bytes must be >= 0"))
		require.False(t, result.IsCompliant())
		require.False(t, result.StorageFull)
	})
}

func containsSubstring(items []string, substr string) bool {
	for _, item := range items {
		if strings.Contains(item, substr) {
			return true
		}
	}
	return false
}
