package keeper

import (
	"fmt"
	"math"
	"testing"

	"cosmossdk.io/log"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/stretchr/testify/require"

	"github.com/LumeraProtocol/lumera/x/supernode/v1/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func TestValidateMetricKeysDeterministicOrder(t *testing.T) {
	metrics := map[string]float64{
		"unknownB": 1,
		"unknownA": math.Inf(1),
	}

	issues := validateMetricKeys(metrics)

	require.Equal(t, []string{
		"unknown metric key: unknownA",
		"invalid numeric value for unknownA",
		"unknown metric key: unknownB",
	}, issues)
}

func TestEvaluateCompliancePassesWithValidMetrics(t *testing.T) {
	ctx := sdk.NewContext(nil, tmproto.Header{Height: 10}, false, log.NewNopLogger())
	params := types.DefaultParams()

	metrics := map[string]float64{
		"version.major":      2,
		"version.minor":      0,
		"version.patch":      0,
		"cpu.cores_total":    params.MinCpuCores,
		"cpu.usage_percent":  params.MaxCpuUsagePercent - 10,
		"mem.total_gb":       params.MinMemGb,
		"mem.usage_percent":  params.MaxMemUsagePercent - 10,
		"disk.total_gb":      params.MinStorageGb,
		"disk.usage_percent": params.MaxStorageUsagePercent - 10,
		"disk.free_gb":       params.MinStorageGb / 2,
		"mem.free_gb":        params.MinMemGb / 2,
		"uptime_seconds":     100,
		"peers.count":        10,
	}
	for _, port := range params.RequiredOpenPorts {
		metrics[portKey(port)] = 1
	}

	sn := types.SuperNode{Metrics: &types.MetricsAggregate{Height: ctx.BlockHeight()}}
	issues := evaluateCompliance(ctx, params, sn, metrics)

	require.Empty(t, issues)
}

func TestEvaluateComplianceDetectsStaleMetrics(t *testing.T) {
	ctx := sdk.NewContext(nil, tmproto.Header{Height: 10}, false, log.NewNopLogger())
	params := types.DefaultParams()

	metrics := map[string]float64{
		"version.major":      2,
		"version.minor":      0,
		"version.patch":      0,
		"cpu.cores_total":    params.MinCpuCores,
		"cpu.usage_percent":  params.MaxCpuUsagePercent - 10,
		"mem.total_gb":       params.MinMemGb,
		"mem.usage_percent":  params.MaxMemUsagePercent - 10,
		"disk.total_gb":      params.MinStorageGb,
		"disk.usage_percent": params.MaxStorageUsagePercent - 10,
		"uptime_seconds":     100,
		"peers.count":        5,
	}
	for _, port := range params.RequiredOpenPorts {
		metrics[portKey(port)] = 1
	}

	staleHeight := ctx.BlockHeight() - int64(params.MetricsFreshnessMaxBlocks) - 1
	sn := types.SuperNode{Metrics: &types.MetricsAggregate{Height: staleHeight}}
	issues := evaluateCompliance(ctx, params, sn, metrics)

	require.Contains(t, issues, "metrics report is stale")
}

func TestEvaluateComplianceRequiresOpenPorts(t *testing.T) {
	ctx := sdk.NewContext(nil, tmproto.Header{Height: 10}, false, log.NewNopLogger())
	params := types.DefaultParams()

	metrics := map[string]float64{
		"version.major":      2,
		"version.minor":      0,
		"version.patch":      0,
		"cpu.cores_total":    params.MinCpuCores,
		"cpu.usage_percent":  params.MaxCpuUsagePercent - 10,
		"mem.total_gb":       params.MinMemGb,
		"mem.usage_percent":  params.MaxMemUsagePercent - 10,
		"disk.total_gb":      params.MinStorageGb,
		"disk.usage_percent": params.MaxStorageUsagePercent - 10,
		"uptime_seconds":     100,
		"peers.count":        5,
	}
	// Deliberately omit one required port
	for i, port := range params.RequiredOpenPorts {
		if i == 0 {
			continue
		}
		metrics[portKey(port)] = 1
	}

	sn := types.SuperNode{Metrics: &types.MetricsAggregate{Height: ctx.BlockHeight()}}
	issues := evaluateCompliance(ctx, params, sn, metrics)

	require.Contains(t, issues, "required port")
}

func portKey(port uint32) string {
	return fmt.Sprintf("port.%d_open", port)
}
