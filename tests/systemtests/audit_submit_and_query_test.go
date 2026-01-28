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

	// These nodes register during window 0, after the window-0 snapshot was created at height 1.
	// If they submit *no* report in window 0, EndBlock(0) will postpone them for "missing report" at height 20.
	// Submit self-only reports in the current window to keep them ACTIVE for the next window.
	ws0 := auditQueryAssignedTargets(t, 0, false, n0.accAddr)
	self := auditSelfReportJSON([]string{"PORT_STATE_OPEN"})
	RequireTxSuccess(t, submitAuditReport(t, cli, n0.nodeName, ws0.WindowId, self, nil))
	RequireTxSuccess(t, submitAuditReport(t, cli, n1.nodeName, ws0.WindowId, self, nil))

	// Now wait for the next window boundary so the snapshot includes the registered supernodes.
	awaitAtLeastHeight(t, ws0.WindowStartHeight+int64(reportingWindowBlocks))
	ws1 := auditQueryAssignedTargets(t, 0, false, n0.accAddr)
	require.Greater(t, len(ws1.TargetSupernodeAccounts), 0, "expected n0 to be assigned targets in window %d", ws1.WindowId)

	// Construct a minimal report. Since n0 is ACTIVE, it is a prober and must submit peer observations
	// for all assigned targets for the window.
	peerObs := make([]string, 0, len(ws1.TargetSupernodeAccounts))
	for _, target := range ws1.TargetSupernodeAccounts {
		peerObs = append(peerObs, auditPeerObservationJSON(target, []string{"PORT_STATE_OPEN"}))
	}

	txResp := submitAuditReport(t, cli, n0.nodeName, ws1.WindowId, self, peerObs)
	RequireTxSuccess(t, txResp)

	// Query via gRPC instead of CLI JSON because AuditReport contains float fields and
	// CLI JSON marshalling is currently broken ("unknown type float64") in this environment.
	report := auditQueryReport(t, ws1.WindowId, n0.accAddr)
	require.Equal(t, n0.accAddr, report.SupernodeAccount)
	require.Equal(t, ws1.WindowId, report.WindowId)
}
