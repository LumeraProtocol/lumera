package keeper

import (
	"fmt"
	"math"

	"github.com/Masterminds/semver/v3"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

// buildVersion reconstructs a semver from the individual
// version fields in SupernodeMetrics.
func buildVersion(m types.SupernodeMetrics) (*semver.Version, error) {
	versionStr := fmt.Sprintf("%d.%d.%d", m.VersionMajor, m.VersionMinor, m.VersionPatch)
	return semver.NewVersion(versionStr)
}

// evaluateCompliance validates the reported metrics against the configured
// parameter thresholds. It returns a list of human-readable issues; an empty
// list means the metrics are compliant. Freshness and staleness are handled
// separately in the end-block staleness handler.
func evaluateCompliance(ctx sdk.Context, params types.Params, m types.SupernodeMetrics) []string {
	_ = ctx // ctx reserved for future use (e.g. logging), currently unused.

	issues := make([]string, 0)

	checkFinite := func(name string, v float64) {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			issues = append(issues, fmt.Sprintf("invalid numeric value for %s", name))
		}
	}

	// 1) Version check: enforce minimum supernode binary version.
	if minVersion, err := semver.NewVersion(params.MinSupernodeVersion); err == nil {
		version, err := buildVersion(m)
		if err != nil {
			issues = append(issues, fmt.Sprintf("invalid reported version: %v", err))
		} else if version.LessThan(minVersion) {
			issues = append(issues, fmt.Sprintf("version %s below minimum %s", version, minVersion))
		}
	} else {
		issues = append(issues, fmt.Sprintf("invalid minimum version parameter: %v", err))
		ctx.Logger().Error(
			"invalid MinSupernodeVersion parameter; all reports will be marked non-compliant",
			"value", params.MinSupernodeVersion,
			"err", err,
		)
	}

	// 2) CPU checks: minimum cores and usage within configured bounds.
	checkFinite("cpu.cores_total", m.CpuCoresTotal)
	checkFinite("cpu.usage_percent", m.CpuUsagePercent)
	if m.CpuCoresTotal <= 0 {
		issues = append(issues, "cpu.cores_total must be > 0")
	}
	if m.CpuCoresTotal < params.MinCpuCores {
		issues = append(issues, fmt.Sprintf("cpu cores %.2f below minimum %.2f", m.CpuCoresTotal, params.MinCpuCores))
	}
	if m.CpuUsagePercent > params.MaxCpuUsagePercent {
		issues = append(issues, fmt.Sprintf("cpu usage %.2f above max %.2f", m.CpuUsagePercent, params.MaxCpuUsagePercent))
	}
	if m.CpuUsagePercent < 0 || m.CpuUsagePercent > 100 {
		issues = append(issues, "cpu.usage_percent outside 0-100 range")
	}

	// 3) Memory checks: minimum total GB and usage within configured bounds.
	checkFinite("mem.total_gb", m.MemTotalGb)
	checkFinite("mem.usage_percent", m.MemUsagePercent)
	checkFinite("mem.free_gb", m.MemFreeGb)
	if m.MemTotalGb <= 0 {
		issues = append(issues, "mem.total_gb must be > 0")
	}
	if m.MemTotalGb < params.MinMemGb {
		issues = append(issues, fmt.Sprintf("mem total %.2f below minimum %.2f", m.MemTotalGb, params.MinMemGb))
	}
	if m.MemUsagePercent > params.MaxMemUsagePercent {
		issues = append(issues, fmt.Sprintf("mem usage %.2f above max %.2f", m.MemUsagePercent, params.MaxMemUsagePercent))
	}
	if m.MemUsagePercent < 0 || m.MemUsagePercent > 100 {
		issues = append(issues, "mem.usage_percent outside 0-100 range")
	}

	// 4) Storage checks: minimum total GB and usage within configured bounds.
	checkFinite("disk.total_gb", m.DiskTotalGb)
	checkFinite("disk.usage_percent", m.DiskUsagePercent)
	checkFinite("disk.free_gb", m.DiskFreeGb)
	if m.DiskTotalGb <= 0 {
		issues = append(issues, "disk.total_gb must be > 0")
	}
	if m.DiskTotalGb < params.MinStorageGb {
		issues = append(issues, fmt.Sprintf("disk total %.2f below minimum %.2f", m.DiskTotalGb, params.MinStorageGb))
	}
	if m.DiskUsagePercent > params.MaxStorageUsagePercent {
		issues = append(issues, fmt.Sprintf("disk usage %.2f above max %.2f", m.DiskUsagePercent, params.MaxStorageUsagePercent))
	}
	if m.DiskUsagePercent < 0 || m.DiskUsagePercent > 100 {
		issues = append(issues, "disk.usage_percent outside 0-100 range")
	}

	// 5) Network checks: all required ports must be explicitly reported as open.
	openPorts := make(map[uint32]struct{}, len(m.OpenPorts))
	for _, port := range m.OpenPorts {
		openPorts[port] = struct{}{}
	}
	for _, port := range params.RequiredOpenPorts {
		if _, ok := openPorts[port]; !ok {
			issues = append(issues, fmt.Sprintf("required port %d not open", port))
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
