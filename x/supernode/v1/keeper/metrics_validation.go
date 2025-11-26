package keeper

import (
	"fmt"
	"math"
	"strings"

	"github.com/Masterminds/semver/v3"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

var canonicalMetricKeys = map[string]struct{}{
	"version.major":      {},
	"version.minor":      {},
	"version.patch":      {},
	"cpu.cores_total":    {},
	"cpu.usage_percent":  {},
	"mem.total_gb":       {},
	"mem.free_gb":        {},
	"mem.usage_percent":  {},
	"disk.total_gb":      {},
	"disk.free_gb":       {},
	"disk.usage_percent": {},
	"uptime_seconds":     {},
	"peers.count":        {},
	"port.4444_open":     {},
	"port.4445_open":     {},
	"port.8002_open":     {},
}

func validateMetricKeys(metrics map[string]float64) []string {
	issues := make([]string, 0)
	for key, value := range metrics {
		if _, ok := canonicalMetricKeys[key]; !ok {
			if !strings.HasPrefix(key, "port.") || !strings.HasSuffix(key, "_open") {
				issues = append(issues, fmt.Sprintf("unknown metric key: %s", key))
			}
		}
		if math.IsNaN(value) || math.IsInf(value, 0) {
			issues = append(issues, fmt.Sprintf("invalid numeric value for %s", key))
		}
	}
	return issues
}

func metricValue(metrics map[string]float64, key string) (float64, bool) {
	v, ok := metrics[key]
	return v, ok
}

func buildVersion(metrics map[string]float64) (*semver.Version, error) {
	major, ok := metricValue(metrics, "version.major")
	if !ok {
		return nil, fmt.Errorf("missing version.major")
	}
	minor, ok := metricValue(metrics, "version.minor")
	if !ok {
		return nil, fmt.Errorf("missing version.minor")
	}
	patch, ok := metricValue(metrics, "version.patch")
	if !ok {
		return nil, fmt.Errorf("missing version.patch")
	}
	versionStr := fmt.Sprintf("%.0f.%.0f.%.0f", major, minor, patch)
	return semver.NewVersion(versionStr)
}

func evaluateCompliance(ctx sdk.Context, params types.Params, sn types.SuperNode, metrics map[string]float64) []string {
	issues := validateMetricKeys(metrics)

	// Version check
	if minVersion, err := semver.NewVersion(params.MinSupernodeVersion); err == nil {
		version, err := buildVersion(metrics)
		if err != nil {
			issues = append(issues, err.Error())
		} else if version.LessThan(minVersion) {
			issues = append(issues, fmt.Sprintf("version %s below minimum %s", version, minVersion))
		}
	} else {
		issues = append(issues, fmt.Sprintf("invalid minimum version parameter: %v", err))
	}

	// CPU
	if cores, ok := metricValue(metrics, "cpu.cores_total"); ok {
		if cores < params.MinCpuCores {
			issues = append(issues, fmt.Sprintf("cpu cores %.2f below minimum %.2f", cores, params.MinCpuCores))
		}
	} else {
		issues = append(issues, "cpu.cores_total missing")
	}
	if usage, ok := metricValue(metrics, "cpu.usage_percent"); ok {
		if usage > params.MaxCpuUsagePercent {
			issues = append(issues, fmt.Sprintf("cpu usage %.2f above max %.2f", usage, params.MaxCpuUsagePercent))
		}
		if usage < 0 || usage > 100 {
			issues = append(issues, "cpu.usage_percent outside 0-100 range")
		}
	} else {
		issues = append(issues, "cpu.usage_percent missing")
	}

	// Memory
	if total, ok := metricValue(metrics, "mem.total_gb"); ok {
		if total < params.MinMemGb {
			issues = append(issues, fmt.Sprintf("mem total %.2f below minimum %.2f", total, params.MinMemGb))
		}
	} else {
		issues = append(issues, "mem.total_gb missing")
	}
	if usage, ok := metricValue(metrics, "mem.usage_percent"); ok {
		if usage > params.MaxMemUsagePercent {
			issues = append(issues, fmt.Sprintf("mem usage %.2f above max %.2f", usage, params.MaxMemUsagePercent))
		}
		if usage < 0 || usage > 100 {
			issues = append(issues, "mem.usage_percent outside 0-100 range")
		}
	} else {
		issues = append(issues, "mem.usage_percent missing")
	}

	// Storage
	if total, ok := metricValue(metrics, "disk.total_gb"); ok {
		if total < params.MinStorageGb {
			issues = append(issues, fmt.Sprintf("disk total %.2f below minimum %.2f", total, params.MinStorageGb))
		}
	} else {
		issues = append(issues, "disk.total_gb missing")
	}
	if usage, ok := metricValue(metrics, "disk.usage_percent"); ok {
		if usage > params.MaxStorageUsagePercent {
			issues = append(issues, fmt.Sprintf("disk usage %.2f above max %.2f", usage, params.MaxStorageUsagePercent))
		}
		if usage < 0 || usage > 100 {
			issues = append(issues, "disk.usage_percent outside 0-100 range")
		}
	} else {
		issues = append(issues, "disk.usage_percent missing")
	}

	for _, port := range params.RequiredOpenPorts {
		key := fmt.Sprintf("port.%d_open", port)
		if val, ok := metricValue(metrics, key); !ok || val != 1 {
			issues = append(issues, fmt.Sprintf("required port %d not open", port))
		}
	}

	if sn.Metrics != nil && sn.Metrics.Height > 0 {
		if ctx.BlockHeight()-sn.Metrics.Height > int64(params.MetricsFreshnessMaxBlocks) {
			issues = append(issues, "metrics report is stale")
		}
	}

	return issues
}

func lastNonPostponedState(states []*types.SuperNodeStateRecord) types.SuperNodeState {
	for i := len(states) - 1; i >= 0; i-- {
		if states[i] == nil {
			continue
		}
		if states[i].State != types.SuperNodeStatePostponed {
			return states[i].State
		}
	}
	return types.SuperNodeStateUnspecified
}
