package supernode

import (
	"math"

	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

// SuperNodeInfo is the ABI-compatible struct returned by query methods.
// Field names and types must match the ABI tuple definition exactly.
type SuperNodeInfo struct {
	ValidatorAddress string `abi:"validatorAddress"`
	SupernodeAccount string `abi:"supernodeAccount"`
	CurrentState     uint8  `abi:"currentState"`
	StateHeight      int64  `abi:"stateHeight"`
	IpAddress        string `abi:"ipAddress"`
	P2PPort          string `abi:"p2pPort"`
	Note             string `abi:"note"`
	EvidenceCount    uint64 `abi:"evidenceCount"`
}

// MetricsReport is the ABI-compatible struct for supernode metrics.
// Floats from protobuf are truncated to integers for Solidity compatibility.
type MetricsReport struct {
	VersionMajor    uint32 `abi:"versionMajor"`
	VersionMinor    uint32 `abi:"versionMinor"`
	VersionPatch    uint32 `abi:"versionPatch"`
	CpuCoresTotal   uint32 `abi:"cpuCoresTotal"`
	CpuUsagePercent uint64 `abi:"cpuUsagePercent"`
	MemTotalGb      uint64 `abi:"memTotalGb"`
	MemUsagePercent uint64 `abi:"memUsagePercent"`
	MemFreeGb       uint64 `abi:"memFreeGb"`
	DiskTotalGb     uint64 `abi:"diskTotalGb"`
	DiskUsagePercent uint64 `abi:"diskUsagePercent"`
	DiskFreeGb      uint64 `abi:"diskFreeGb"`
	UptimeSeconds   uint64 `abi:"uptimeSeconds"`
	PeersCount      uint32 `abi:"peersCount"`
}

// supernodeToABIInfo converts a keeper SuperNode to the ABI-compatible SuperNodeInfo struct.
func supernodeToABIInfo(sn *sntypes.SuperNode) SuperNodeInfo {
	var currentState uint8
	var stateHeight int64
	if len(sn.States) > 0 {
		last := sn.States[len(sn.States)-1]
		currentState = uint8(last.State)
		stateHeight = last.Height
	}

	ipAddress := ""
	if len(sn.PrevIpAddresses) > 0 {
		ipAddress = sn.PrevIpAddresses[len(sn.PrevIpAddresses)-1].Address
	}

	return SuperNodeInfo{
		ValidatorAddress: sn.ValidatorAddress,
		SupernodeAccount: sn.SupernodeAccount,
		CurrentState:     currentState,
		StateHeight:      stateHeight,
		IpAddress:        ipAddress,
		P2PPort:          sn.P2PPort,
		Note:             sn.Note,
		EvidenceCount:    uint64(len(sn.Evidence)),
	}
}

// metricsToABI converts protobuf SupernodeMetrics to the ABI-compatible MetricsReport.
// Floats are truncated to integers (these are whole-number metrics in practice).
func metricsToABI(m *sntypes.SupernodeMetrics) MetricsReport {
	if m == nil {
		return MetricsReport{}
	}
	return MetricsReport{
		VersionMajor:     m.VersionMajor,
		VersionMinor:     m.VersionMinor,
		VersionPatch:     m.VersionPatch,
		CpuCoresTotal:    uint32(math.Round(m.CpuCoresTotal)),
		CpuUsagePercent:  uint64(math.Round(m.CpuUsagePercent)),
		MemTotalGb:       uint64(math.Round(m.MemTotalGb)),
		MemUsagePercent:  uint64(math.Round(m.MemUsagePercent)),
		MemFreeGb:        uint64(math.Round(m.MemFreeGb)),
		DiskTotalGb:      uint64(math.Round(m.DiskTotalGb)),
		DiskUsagePercent: uint64(math.Round(m.DiskUsagePercent)),
		DiskFreeGb:       uint64(math.Round(m.DiskFreeGb)),
		UptimeSeconds:    uint64(math.Round(m.UptimeSeconds)),
		PeersCount:       m.PeersCount,
	}
}

// abiToMetrics converts an ABI MetricsReport back to protobuf SupernodeMetrics.
func abiToMetrics(r MetricsReport) sntypes.SupernodeMetrics {
	return sntypes.SupernodeMetrics{
		VersionMajor:     r.VersionMajor,
		VersionMinor:     r.VersionMinor,
		VersionPatch:     r.VersionPatch,
		CpuCoresTotal:    float64(r.CpuCoresTotal),
		CpuUsagePercent:  float64(r.CpuUsagePercent),
		MemTotalGb:       float64(r.MemTotalGb),
		MemUsagePercent:  float64(r.MemUsagePercent),
		MemFreeGb:        float64(r.MemFreeGb),
		DiskTotalGb:      float64(r.DiskTotalGb),
		DiskUsagePercent: float64(r.DiskUsagePercent),
		DiskFreeGb:       float64(r.DiskFreeGb),
		UptimeSeconds:    float64(r.UptimeSeconds),
		PeersCount:       r.PeersCount,
	}
}
