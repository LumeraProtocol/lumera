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
	if m.CpuCoresTotal < float64(params.MinCpuCores) {
		issues = append(issues, fmt.Sprintf("cpu cores %.2f below minimum %d", m.CpuCoresTotal, params.MinCpuCores))
	}
	// A usage value of 0 is treated as "unknown/not reported" to avoid false
	// non-compliance due to proto3 defaults. Only non-zero values are enforced.
	if m.CpuUsagePercent < 0 || m.CpuUsagePercent > 100 {
		issues = append(issues, "cpu.usage_percent outside 0-100 range")
	} else if m.CpuUsagePercent > 0 {
		if m.CpuUsagePercent > float64(params.MaxCpuUsagePercent) {
			issues = append(issues, fmt.Sprintf("cpu usage %.2f above max %d", m.CpuUsagePercent, params.MaxCpuUsagePercent))
		}
	}

	// 3) Memory checks: minimum total GB and usage within configured bounds.
	checkFinite("mem.total_gb", m.MemTotalGb)
	checkFinite("mem.usage_percent", m.MemUsagePercent)
	checkFinite("mem.free_gb", m.MemFreeGb)
	if m.MemTotalGb <= 0 {
		issues = append(issues, "mem.total_gb must be > 0")
	}
	if m.MemFreeGb < 0 {
		issues = append(issues, "mem.free_gb must be >= 0")
	}
	if m.MemFreeGb > m.MemTotalGb {
		issues = append(issues, "mem.free_gb cannot exceed mem.total_gb")
	}
	if m.MemTotalGb < float64(params.MinMemGb) {
		issues = append(issues, fmt.Sprintf("mem total %.2f below minimum %d", m.MemTotalGb, params.MinMemGb))
	}
	// A usage value of 0 is treated as "unknown/not reported" to avoid false
	// non-compliance due to proto3 defaults. Only non-zero values are enforced.
	if m.MemUsagePercent < 0 || m.MemUsagePercent > 100 {
		issues = append(issues, "mem.usage_percent outside 0-100 range")
	} else if m.MemUsagePercent > 0 {
		if m.MemUsagePercent > float64(params.MaxMemUsagePercent) {
			issues = append(issues, fmt.Sprintf("mem usage %.2f above max %d", m.MemUsagePercent, params.MaxMemUsagePercent))
		}
	}

	// 4) Storage checks: minimum total GB and usage within configured bounds.
	checkFinite("disk.total_gb", m.DiskTotalGb)
	checkFinite("disk.usage_percent", m.DiskUsagePercent)
	checkFinite("disk.free_gb", m.DiskFreeGb)
	if m.DiskTotalGb <= 0 {
		issues = append(issues, "disk.total_gb must be > 0")
	}
	if m.DiskFreeGb < 0 {
		issues = append(issues, "disk.free_gb must be >= 0")
	}
	if m.DiskFreeGb > m.DiskTotalGb {
		issues = append(issues, "disk.free_gb cannot exceed disk.total_gb")
	}
	if m.DiskTotalGb < float64(params.MinStorageGb) {
		issues = append(issues, fmt.Sprintf("disk total %.2f below minimum %d", m.DiskTotalGb, params.MinStorageGb))
	}
	if m.DiskUsagePercent > float64(params.MaxStorageUsagePercent) {
		issues = append(issues, fmt.Sprintf("disk usage %.2f above max %d", m.DiskUsagePercent, params.MaxStorageUsagePercent))
	}
	if m.DiskUsagePercent < 0 || m.DiskUsagePercent > 100 {
		issues = append(issues, "disk.usage_percent outside 0-100 range")
	}

	// 5) Network checks: explicit CLOSED required ports cause immediate non-compliance.
	//
	// Port state defaults to UNKNOWN in proto3; UNKNOWN is ignored to avoid false
	// non-compliance from omitted/unmeasured port data.
	portStates := make(map[uint32]types.PortState, len(m.OpenPorts))
	for _, st := range m.OpenPorts {
		port := st.Port
		if port == 0 || port > 65535 {
			issues = append(issues, fmt.Sprintf("invalid port value %d", port))
			continue
		}
		// Resolve duplicates by taking the most specific/strict state.
		prev, ok := portStates[port]
		if !ok {
			portStates[port] = st.State
			continue
		}
		if prev == types.PortState_PORT_STATE_CLOSED {
			continue
		}
		if st.State == types.PortState_PORT_STATE_CLOSED {
			portStates[port] = st.State
			continue
		}
		if prev == types.PortState_PORT_STATE_OPEN {
			continue
		}
		portStates[port] = st.State
	}

	for _, required := range params.RequiredOpenPorts {
		if required == 0 || required > 65535 {
			issues = append(issues, fmt.Sprintf("invalid required port value %d", required))
			continue
		}
		if state, ok := portStates[required]; ok && state == types.PortState_PORT_STATE_CLOSED {
			issues = append(issues, fmt.Sprintf("required port %d closed", required))
		}
	}

	// 6) Liveness/connectivity sanity checks.
	checkFinite("uptime_seconds", m.UptimeSeconds)
	if m.UptimeSeconds < 0 {
		issues = append(issues, "uptime_seconds must be >= 0")
	}
	if m.PeersCount == 0 {
		issues = append(issues, "peers_count must be > 0")
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
