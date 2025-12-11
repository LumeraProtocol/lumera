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
	metrics.OpenPorts = append([]uint32(nil), params.RequiredOpenPorts...)

	issues := evaluateCompliance(ctx, params, metrics)

	require.Empty(t, issues)
}

func TestEvaluateComplianceDetectsStaleMetrics(t *testing.T) {
	params := types.DefaultParams()
	ctx := sdk.NewContext(nil, tmproto.Header{Height: 10}, false, log.NewNopLogger())

	metrics := types.SupernodeMetrics{
		VersionMajor:    1,
		VersionMinor:    0,
		VersionPatch:    0,
		CpuCoresTotal:   params.MinCpuCores,
		CpuUsagePercent: params.MaxCpuUsagePercent - 10,
		MemTotalGb:      params.MinMemGb,
		MemUsagePercent: params.MaxMemUsagePercent - 10,
		DiskTotalGb:     params.MinStorageGb,
		DiskUsagePercent: params.MaxStorageUsagePercent - 10,
		UptimeSeconds:   100,
		PeersCount:      5,
	}
	metrics.OpenPorts = append([]uint32(nil), params.RequiredOpenPorts...)

	issues := evaluateCompliance(ctx, params, metrics)

	require.True(t, containsSubstring(issues, "version"))
}

func TestEvaluateComplianceRequiresOpenPorts(t *testing.T) {
	ctx := sdk.NewContext(nil, tmproto.Header{Height: 10}, false, log.NewNopLogger())
	params := types.DefaultParams()

	metrics := types.SupernodeMetrics{
		VersionMajor:    2,
		VersionMinor:    0,
		VersionPatch:    0,
		CpuCoresTotal:   params.MinCpuCores,
		CpuUsagePercent: params.MaxCpuUsagePercent - 10,
		MemTotalGb:      params.MinMemGb,
		MemUsagePercent: params.MaxMemUsagePercent - 10,
		DiskTotalGb:     params.MinStorageGb,
		DiskUsagePercent: params.MaxStorageUsagePercent - 10,
		UptimeSeconds:   100,
		PeersCount:      5,
	}
	// Deliberately omit one required port
	if len(params.RequiredOpenPorts) > 1 {
		metrics.OpenPorts = append([]uint32(nil), params.RequiredOpenPorts[1:]...)
	}

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
