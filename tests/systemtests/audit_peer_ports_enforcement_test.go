//go:build system_test

package system

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/sjson"
)

func TestAuditPeerPortsUnanimousClosedPostponesAfterConsecutiveWindows(t *testing.T) {
	const (
		reportingWindowBlocks = uint64(10)
	)
	const originHeight = int64(1)

	sut.ModifyGenesisJSON(t,
		setSupernodeParamsForAuditTests(t),
		setAuditParamsForFastWindows(t, reportingWindowBlocks, 1, 1, 1, []uint32{4444}),
		func(genesis []byte) []byte {
			state, err := sjson.SetRawBytes(genesis, "app_state.audit.params.consecutive_windows_to_postpone", []byte("2"))
			require.NoError(t, err)
			return state
		},
	)
	sut.StartChain(t)

	cli := NewLumeradCLI(t, sut, true)
	n0 := getNodeIdentity(t, cli, "node0")
	n1 := getNodeIdentity(t, cli, "node1")

	registerSupernode(t, cli, n0, "192.168.1.1")
	registerSupernode(t, cli, n1, "192.168.1.2")

	currentHeight := sut.AwaitNextBlock(t)
	windowID1, window1Start := nextWindowAfterHeight(originHeight, reportingWindowBlocks, currentHeight)
	windowID2 := windowID1 + 1
	window2Start := window1Start + int64(reportingWindowBlocks)
	enforce2 := window2Start + int64(reportingWindowBlocks)

	senders := sortedStrings(n0.accAddr, n1.accAddr)
	receivers := sortedStrings(n0.accAddr, n1.accAddr)
	kWindow := computeKWindow(1, 1, 1, len(senders), len(receivers))
	require.Equal(t, uint32(1), kWindow)

	selfOpen := auditSelfReportJSON([]string{"PORT_STATE_OPEN"})

	// Window 1: node0 reports node1 as CLOSED, node1 reports node0 as OPEN.
	awaitAtLeastHeight(t, window1Start)
	seed1 := headerHashAtHeight(t, sut.rpcAddr, window1Start)
	targets0w1, ok := assignedTargets(seed1, senders, receivers, kWindow, n0.accAddr)
	require.True(t, ok)
	require.Len(t, targets0w1, 1)
	targets1w1, ok := assignedTargets(seed1, senders, receivers, kWindow, n1.accAddr)
	require.True(t, ok)
	require.Len(t, targets1w1, 1)

	tx0w1 := submitAuditReport(t, cli, n0.nodeName, windowID1, selfOpen, []string{
		auditPeerObservationJSON(targets0w1[0], []string{"PORT_STATE_CLOSED"}),
	})
	RequireTxSuccess(t, tx0w1)
	tx1w1 := submitAuditReport(t, cli, n1.nodeName, windowID1, selfOpen, []string{
		auditPeerObservationJSON(targets1w1[0], []string{"PORT_STATE_OPEN"}),
	})
	RequireTxSuccess(t, tx1w1)

	// Window 2: repeat -> node1 should be POSTPONED at window end due to consecutive unanimous CLOSED.
	awaitAtLeastHeight(t, window2Start)
	seed2 := headerHashAtHeight(t, sut.rpcAddr, window2Start)
	targets0w2, ok := assignedTargets(seed2, senders, receivers, kWindow, n0.accAddr)
	require.True(t, ok)
	require.Len(t, targets0w2, 1)
	targets1w2, ok := assignedTargets(seed2, senders, receivers, kWindow, n1.accAddr)
	require.True(t, ok)
	require.Len(t, targets1w2, 1)

	tx0w2 := submitAuditReport(t, cli, n0.nodeName, windowID2, selfOpen, []string{
		auditPeerObservationJSON(targets0w2[0], []string{"PORT_STATE_CLOSED"}),
	})
	RequireTxSuccess(t, tx0w2)
	tx1w2 := submitAuditReport(t, cli, n1.nodeName, windowID2, selfOpen, []string{
		auditPeerObservationJSON(targets1w2[0], []string{"PORT_STATE_OPEN"}),
	})
	RequireTxSuccess(t, tx1w2)

	awaitAtLeastHeight(t, enforce2)

	require.Equal(t, "SUPERNODE_STATE_ACTIVE", querySupernodeLatestState(t, cli, n0.valAddr))
	require.Equal(t, "SUPERNODE_STATE_POSTPONED", querySupernodeLatestState(t, cli, n1.valAddr))
}
