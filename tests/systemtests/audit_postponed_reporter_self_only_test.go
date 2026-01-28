//go:build system_test

package system

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/sjson"
)

func TestAuditSubmitReport_PostponedReporterSelfOnly(t *testing.T) {
	const (
		// Keep windows long enough in real time to avoid missing-report postponement before the first tested window.
		reportingWindowBlocks = uint64(20)
	)
	const originHeight = int64(1)

	sut.ModifyGenesisJSON(t,
		setSupernodeParamsForAuditTests(t),
		setAuditParamsForFastWindows(t, reportingWindowBlocks, 1, 1, 1, []uint32{4444}),
		func(genesis []byte) []byte {
			// Avoid missing-report postponement before the first tested window.
			state, err := sjson.SetRawBytes(genesis, "app_state.audit.params.consecutive_windows_to_postpone", []byte("2"))
			require.NoError(t, err)

			// Make it easy to postpone node1 via self host requirements (independent of consecutive windows).
			state, err = sjson.SetRawBytes(state, "app_state.audit.params.min_cpu_free_percent", []byte("90"))
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

	awaitAtLeastHeight(t, window1Start)

	seed1 := headerHashAtHeight(t, sut.rpcAddr, window1Start)
	senders := sortedStrings(n0.accAddr, n1.accAddr)
	receivers := sortedStrings(n0.accAddr, n1.accAddr)
	kWindow := computeKWindow(1, 1, 1, len(senders), len(receivers))
	require.Equal(t, uint32(1), kWindow)

	targets0, ok := assignedTargets(seed1, senders, receivers, kWindow, n0.accAddr)
	require.True(t, ok)
	require.Len(t, targets0, 1)

	self := auditSelfReportJSON([]string{"PORT_STATE_OPEN"})

	node1BadSelfBz, err := json.Marshal(map[string]any{
		"cpu_usage_percent":    99.0,
		"mem_usage_percent":    1.0,
		"disk_usage_percent":   1.0,
		"inbound_port_states":  []string{"PORT_STATE_OPEN"},
		"failed_actions_count": 0,
	})
	require.NoError(t, err)
	node1BadSelf := string(node1BadSelfBz)

	// Both submit in window1 so missing-report enforcement doesn't interfere.
	// node1 violates host minimums and should become POSTPONED at window end.
	txResp0 := submitAuditReport(t, cli, n0.nodeName, windowID1, self, []string{
		auditPeerObservationJSON(targets0[0], []string{"PORT_STATE_OPEN"}),
	})
	RequireTxSuccess(t, txResp0)

	txResp1 := submitAuditReport(t, cli, n1.nodeName, windowID1, node1BadSelf, []string{
		auditPeerObservationJSON(n0.accAddr, []string{"PORT_STATE_OPEN"}),
	})
	RequireTxSuccess(t, txResp1)

	awaitAtLeastHeight(t, window2Start)
	require.Equal(t, "SUPERNODE_STATE_POSTPONED", querySupernodeLatestState(t, cli, n1.valAddr))

	// POSTPONED reporter cannot submit peer observations.
	txBad := submitAuditReport(t, cli, n1.nodeName, windowID2, self, []string{
		auditPeerObservationJSON(n0.accAddr, []string{"PORT_STATE_OPEN"}),
	})
	RequireTxFailure(t, txBad, "reporter not eligible for peer observations in this window")

	// POSTPONED reporter can submit a self report only.
	txOK := submitAuditReport(t, cli, n1.nodeName, windowID2, self, nil)
	RequireTxSuccess(t, txOK)
}
