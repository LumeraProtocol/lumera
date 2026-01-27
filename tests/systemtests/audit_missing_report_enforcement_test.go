//go:build system_test

package system

// This test validates missing-report enforcement in EndBlocker:
// - two ACTIVE supernodes are snapshotted as senders at window start
// - only one submits a report for the window
// - after `window_end + 1`, the missing sender is POSTPONED

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAuditMissingReportPostponesSender(t *testing.T) {
	const (
		reportingWindowBlocks = uint64(5)
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
	// Use the next window after registration so both supernodes are in the sender snapshot for that window.
	windowID, windowStartHeight := nextWindowAfterHeight(originHeight, reportingWindowBlocks, currentHeight)
	enforceHeight := windowStartHeight + int64(reportingWindowBlocks)

	awaitAtLeastHeight(t, windowStartHeight)

	seed := headerHashAtHeight(t, sut.rpcAddr, windowStartHeight)
	senders := sortedStrings(n0.accAddr, n1.accAddr)
	receivers := sortedStrings(n0.accAddr, n1.accAddr)
	kWindow := computeKWindow(1, 1, 1, len(senders), len(receivers))
	require.Equal(t, uint32(1), kWindow)

	targets0, ok := assignedTargets(seed, senders, receivers, kWindow, n0.accAddr)
	require.True(t, ok)
	require.Len(t, targets0, 1)

	self := auditSelfReportJSON([]string{"PORT_STATE_OPEN"})
	txResp := submitAuditReport(t, cli, n0.nodeName, windowID, self, []string{
		auditPeerObservationJSON(targets0[0], []string{"PORT_STATE_OPEN"}),
	})
	RequireTxSuccess(t, txResp)

	// node1 does not submit any report for this window -> should be postponed at enforceHeight.
	awaitAtLeastHeight(t, enforceHeight)

	require.Equal(t, "SUPERNODE_STATE_POSTPONED", querySupernodeLatestState(t, cli, n1.valAddr))
}
