package types

import (
	"fmt"
	"sort"

	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"
)

var _ paramtypes.ParamSet = (*Params)(nil)

var (
	KeyReportingWindowBlocks        = []byte("ReportingWindowBlocks")
	KeyPeerQuorumReports            = []byte("PeerQuorumReports")
	KeyMinProbeTargetsPerWindow     = []byte("MinProbeTargetsPerWindow")
	KeyMaxProbeTargetsPerWindow     = []byte("MaxProbeTargetsPerWindow")
	KeyRequiredOpenPorts            = []byte("RequiredOpenPorts")
	KeyMinCpuFreePercent            = []byte("MinCpuFreePercent")
	KeyMinMemFreePercent            = []byte("MinMemFreePercent")
	KeyMinDiskFreePercent           = []byte("MinDiskFreePercent")
	KeyConsecutiveWindowsToPostpone = []byte("ConsecutiveWindowsToPostpone")
	KeyKeepLastWindowEntries        = []byte("KeepLastWindowEntries")
)

var (
	DefaultReportingWindowBlocks        = uint64(400)
	DefaultPeerQuorumReports            = uint32(3)
	DefaultMinProbeTargetsPerWindow     = uint32(3)
	DefaultMaxProbeTargetsPerWindow     = uint32(5)
	DefaultRequiredOpenPorts            = []uint32{4444, 4445, 8002}
	DefaultMinCpuFreePercent            = uint32(0)
	DefaultMinMemFreePercent            = uint32(0)
	DefaultMinDiskFreePercent           = uint32(0)
	DefaultConsecutiveWindowsToPostpone = uint32(1)
	DefaultKeepLastWindowEntries        = uint64(200)
)

// Params notes
//
// - reporting_window_blocks: defines the fixed-length reporting window size in blocks.
// - peer_quorum_reports: desired number of peer observations per receiver (drives k_window calculation).
// - min/max_probe_targets_per_window: clamps k_window to a safe range.
// - required_open_ports: ports every report must cover.
// - min_*_free_percent: minimum required free capacity from self report (0 disables).
// - consecutive_windows_to_postpone: consecutive windows of unanimous peer port CLOSED needed to postpone.
// - keep_last_window_entries: how many windows of window-scoped state to keep (pruning at window end).

func ParamKeyTable() paramtypes.KeyTable {
	return paramtypes.NewKeyTable().RegisterParamSet(&Params{})
}

func NewParams(
	reportingWindowBlocks uint64,
	peerQuorumReports uint32,
	minProbeTargetsPerWindow uint32,
	maxProbeTargetsPerWindow uint32,
	requiredOpenPorts []uint32,
	minCpuFreePercent uint32,
	minMemFreePercent uint32,
	minDiskFreePercent uint32,
	consecutiveWindowsToPostpone uint32,
	keepLastWindowEntries uint64,
) Params {
	return Params{
		ReportingWindowBlocks:        reportingWindowBlocks,
		PeerQuorumReports:            peerQuorumReports,
		MinProbeTargetsPerWindow:     minProbeTargetsPerWindow,
		MaxProbeTargetsPerWindow:     maxProbeTargetsPerWindow,
		RequiredOpenPorts:            requiredOpenPorts,
		MinCpuFreePercent:            minCpuFreePercent,
		MinMemFreePercent:            minMemFreePercent,
		MinDiskFreePercent:           minDiskFreePercent,
		ConsecutiveWindowsToPostpone: consecutiveWindowsToPostpone,
		KeepLastWindowEntries:        keepLastWindowEntries,
	}
}

func DefaultParams() Params {
	return NewParams(
		DefaultReportingWindowBlocks,
		DefaultPeerQuorumReports,
		DefaultMinProbeTargetsPerWindow,
		DefaultMaxProbeTargetsPerWindow,
		append([]uint32(nil), DefaultRequiredOpenPorts...),
		DefaultMinCpuFreePercent,
		DefaultMinMemFreePercent,
		DefaultMinDiskFreePercent,
		DefaultConsecutiveWindowsToPostpone,
		DefaultKeepLastWindowEntries,
	)
}

func (p Params) WithDefaults() Params {
	if p.ReportingWindowBlocks == 0 {
		p.ReportingWindowBlocks = DefaultReportingWindowBlocks
	}
	if p.PeerQuorumReports == 0 {
		p.PeerQuorumReports = DefaultPeerQuorumReports
	}
	if p.MinProbeTargetsPerWindow == 0 {
		p.MinProbeTargetsPerWindow = DefaultMinProbeTargetsPerWindow
	}
	if p.MaxProbeTargetsPerWindow == 0 {
		p.MaxProbeTargetsPerWindow = DefaultMaxProbeTargetsPerWindow
	}
	if len(p.RequiredOpenPorts) == 0 {
		p.RequiredOpenPorts = append([]uint32(nil), DefaultRequiredOpenPorts...)
	}
	if p.ConsecutiveWindowsToPostpone == 0 {
		p.ConsecutiveWindowsToPostpone = DefaultConsecutiveWindowsToPostpone
	}
	if p.KeepLastWindowEntries == 0 {
		p.KeepLastWindowEntries = DefaultKeepLastWindowEntries
	}
	return p
}

func (p *Params) ParamSetPairs() paramtypes.ParamSetPairs {
	return paramtypes.ParamSetPairs{
		paramtypes.NewParamSetPair(KeyReportingWindowBlocks, &p.ReportingWindowBlocks, validateUint64),
		paramtypes.NewParamSetPair(KeyPeerQuorumReports, &p.PeerQuorumReports, validateUint32),
		paramtypes.NewParamSetPair(KeyMinProbeTargetsPerWindow, &p.MinProbeTargetsPerWindow, validateUint32),
		paramtypes.NewParamSetPair(KeyMaxProbeTargetsPerWindow, &p.MaxProbeTargetsPerWindow, validateUint32),
		paramtypes.NewParamSetPair(KeyRequiredOpenPorts, &p.RequiredOpenPorts, validateUint32Slice),
		paramtypes.NewParamSetPair(KeyMinCpuFreePercent, &p.MinCpuFreePercent, validateUint32),
		paramtypes.NewParamSetPair(KeyMinMemFreePercent, &p.MinMemFreePercent, validateUint32),
		paramtypes.NewParamSetPair(KeyMinDiskFreePercent, &p.MinDiskFreePercent, validateUint32),
		paramtypes.NewParamSetPair(KeyConsecutiveWindowsToPostpone, &p.ConsecutiveWindowsToPostpone, validateUint32),
		paramtypes.NewParamSetPair(KeyKeepLastWindowEntries, &p.KeepLastWindowEntries, validateUint64),
	}
}

func (p Params) Validate() error {
	p = p.WithDefaults()

	if p.ReportingWindowBlocks == 0 {
		return fmt.Errorf("reporting_window_blocks must be > 0")
	}
	if p.PeerQuorumReports == 0 {
		return fmt.Errorf("peer_quorum_reports must be > 0")
	}
	if p.MinProbeTargetsPerWindow > p.MaxProbeTargetsPerWindow {
		return fmt.Errorf("min_probe_targets_per_window must be <= max_probe_targets_per_window")
	}
	if len(p.RequiredOpenPorts) == 0 {
		return fmt.Errorf("required_open_ports must not be empty")
	}
	if p.MinCpuFreePercent > 100 {
		return fmt.Errorf("min_cpu_free_percent must be <= 100")
	}
	if p.MinMemFreePercent > 100 {
		return fmt.Errorf("min_mem_free_percent must be <= 100")
	}
	if p.MinDiskFreePercent > 100 {
		return fmt.Errorf("min_disk_free_percent must be <= 100")
	}
	if p.ConsecutiveWindowsToPostpone == 0 {
		return fmt.Errorf("consecutive_windows_to_postpone must be > 0")
	}
	if p.KeepLastWindowEntries == 0 {
		return fmt.Errorf("keep_last_window_entries must be > 0")
	}

	ports := append([]uint32(nil), p.RequiredOpenPorts...)
	sort.Slice(ports, func(i, j int) bool { return ports[i] < ports[j] })
	for i := 1; i < len(ports); i++ {
		if ports[i] == ports[i-1] {
			return fmt.Errorf("required_open_ports must be unique")
		}
	}

	return nil
}

func validateUint64(v interface{}) error {
	_, ok := v.(uint64)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", v)
	}
	return nil
}

func validateUint32(v interface{}) error {
	_, ok := v.(uint32)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", v)
	}
	return nil
}

func validateUint32Slice(v interface{}) error {
	_, ok := v.([]uint32)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", v)
	}
	return nil
}
