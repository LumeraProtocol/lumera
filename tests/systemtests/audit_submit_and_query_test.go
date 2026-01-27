//go:build system_test

package system

// This test validates the "happy path" end-to-end:
// - supernode registration
// - audit report submission via CLI
// - querying stored report via gRPC
//
// Assertions:
// - the reporter's report is stored under (window_id, supernode_account)

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAuditSubmitReportAndQuery(t *testing.T) {
	const (
		// Keep windows long enough in real time to avoid end-blocker enforcement during the test.
		reportingWindowBlocks = uint64(20)
	)
	const originHeight = int64(1)

	sut.ModifyGenesisJSON(t,
		setSupernodeParamsForAuditTests(t),
		setAuditParamsForFastWindows(t, reportingWindowBlocks, 1, 1, 1, []uint32{4444}),
	)
	sut.StartChain(t)

	cli := NewLumeradCLI(t, sut, true)
	n0 := getNodeIdentity(t, cli, "node0")
	n1 := getNodeIdentity(t, cli, "node1")

	registerSupernode(t, cli, n0, "192.168.1.1")
	registerSupernode(t, cli, n1, "192.168.1.2")

	currentHeight := sut.AwaitNextBlock(t)
	// Always test against the next window boundary after registration so:
	// - the snapshot includes the registered supernodes,
	// - the window is guaranteed to be currently accepted at submission height.
	windowID, windowStartHeight := nextWindowAfterHeight(originHeight, reportingWindowBlocks, currentHeight)
	awaitAtLeastHeight(t, windowStartHeight)

	// Construct a minimal report.
	self := auditSelfReportJSON([]string{"PORT_STATE_OPEN"})
	var peerObs []string

	txResp := submitAuditReport(t, cli, n0.nodeName, windowID, self, peerObs)
	RequireTxSuccess(t, txResp)

	// Query via gRPC instead of CLI JSON because AuditReport contains float fields and
	// CLI JSON marshalling is currently broken ("unknown type float64") in this environment.
	report := auditQueryReport(t, windowID, n0.accAddr)
	require.Equal(t, n0.accAddr, report.SupernodeAccount)
	require.Equal(t, windowID, report.WindowId)
}
