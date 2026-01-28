//go:build system_test

package system

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/sjson"
)

func TestAuditRecovery_PostponedBecomesActiveWithSelfAndPeerOpen(t *testing.T) {
	const (
		reportingWindowBlocks = uint64(10)
	)
	const originHeight = int64(1)

	sut.ModifyGenesisJSON(t,
		setSupernodeParamsForAuditTests(t),
		setAuditParamsForFastWindows(t, reportingWindowBlocks, 1, 1, 1, []uint32{4444}),
		func(genesis []byte) []byte {
			// Keep missing-report / peer-port streaks from interfering with this recovery test.
			state, err := sjson.SetRawBytes(genesis, "app_state.audit.params.consecutive_windows_to_postpone", []byte("10"))
			require.NoError(t, err)

			// Use host requirements to get into POSTPONED state deterministically.
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
	enforce2 := window2Start + int64(reportingWindowBlocks)

	senders := sortedStrings(n0.accAddr, n1.accAddr)
	receivers := sortedStrings(n0.accAddr, n1.accAddr)
	kWindow := computeKWindow(1, 1, 1, len(senders), len(receivers))
	require.Equal(t, uint32(1), kWindow)

	selfOK := auditSelfReportJSON([]string{"PORT_STATE_OPEN"})

	badSelfBz, err := json.Marshal(map[string]any{
		"cpu_usage_percent":    99.0,
		"mem_usage_percent":    1.0,
		"disk_usage_percent":   1.0,
		"inbound_port_states":  []string{"PORT_STATE_OPEN"},
		"failed_actions_count": 0,
	})
	require.NoError(t, err)
	selfBad := string(badSelfBz)

	// Window 1: node1 violates host requirements -> becomes POSTPONED.
	awaitAtLeastHeight(t, window1Start)
	seed1 := headerHashAtHeight(t, sut.rpcAddr, window1Start)
	targets0w1, ok := assignedTargets(seed1, senders, receivers, kWindow, n0.accAddr)
	require.True(t, ok)
	require.Len(t, targets0w1, 1)
	targets1w1, ok := assignedTargets(seed1, senders, receivers, kWindow, n1.accAddr)
	require.True(t, ok)
	require.Len(t, targets1w1, 1)

	tx0w1 := submitAuditReport(t, cli, n0.nodeName, windowID1, selfOK, []string{
		auditPeerObservationJSON(targets0w1[0], []string{"PORT_STATE_OPEN"}),
	})
	RequireTxSuccess(t, tx0w1)
	tx1w1 := submitAuditReport(t, cli, n1.nodeName, windowID1, selfBad, []string{
		auditPeerObservationJSON(targets1w1[0], []string{"PORT_STATE_OPEN"}),
	})
	RequireTxSuccess(t, tx1w1)

	awaitAtLeastHeight(t, window2Start)
	require.Equal(t, "SUPERNODE_STATE_POSTPONED", querySupernodeLatestState(t, cli, n1.valAddr))

	// Window 2: node1 submits compliant self report (no peer observations),
	// and node0 submits an OPEN peer observation about node1 in the same window -> recovery at window end.
	seed2 := headerHashAtHeight(t, sut.rpcAddr, window2Start)
	targets0w2, ok := assignedTargets(seed2, senders, receivers, kWindow, n0.accAddr)
	require.True(t, ok)
	require.Len(t, targets0w2, 1)

	tx0w2 := submitAuditReport(t, cli, n0.nodeName, windowID2, selfOK, []string{
		auditPeerObservationJSON(targets0w2[0], []string{"PORT_STATE_OPEN"}),
	})
	RequireTxSuccess(t, tx0w2)

	tx1w2 := submitAuditReport(t, cli, n1.nodeName, windowID2, selfOK, nil)
	RequireTxSuccess(t, tx1w2)

	awaitAtLeastHeight(t, enforce2)
	require.Equal(t, "SUPERNODE_STATE_ACTIVE", querySupernodeLatestState(t, cli, n1.valAddr))
}
