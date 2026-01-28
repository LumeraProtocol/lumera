//go:build system_test

package system

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/sjson"
)

func TestAuditHostRequirementsPostponesActiveSupernode(t *testing.T) {
	const (
		// Keep windows small so AwaitBlockHeight timeouts don't flake.
		reportingWindowBlocks = uint64(10)
	)
	const originHeight = int64(1)

	sut.ModifyGenesisJSON(t,
		setSupernodeParamsForAuditTests(t),
		setAuditParamsForFastWindows(t, reportingWindowBlocks, 1, 1, 1, []uint32{4444}),
		func(genesis []byte) []byte {
			// Avoid missing-report postponement before the first tested window.
			state, err := sjson.SetRawBytes(genesis, "app_state.audit.params.consecutive_windows_to_postpone", []byte("10"))
			require.NoError(t, err)

			// Enforce host requirements.
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
	targets1, ok := assignedTargets(seed, senders, receivers, kWindow, n1.accAddr)
	require.True(t, ok)
	require.Len(t, targets1, 1)

	badSelfBz, err := json.Marshal(map[string]any{
		"cpu_usage_percent":    99.0,
		"mem_usage_percent":    1.0,
		"disk_usage_percent":   1.0,
		"inbound_port_states":  []string{"PORT_STATE_OPEN"},
		"failed_actions_count": 0,
	})
	require.NoError(t, err)
	badSelf := string(badSelfBz)

	okSelf := auditSelfReportJSON([]string{"PORT_STATE_OPEN"})

	// node0 violates host requirements.
	tx0 := submitAuditReport(t, cli, n0.nodeName, windowID, badSelf, []string{
		auditPeerObservationJSON(targets0[0], []string{"PORT_STATE_OPEN"}),
	})
	RequireTxSuccess(t, tx0)

	// node1 stays compliant (and also submits to avoid missing-report enforcement).
	tx1 := submitAuditReport(t, cli, n1.nodeName, windowID, okSelf, []string{
		auditPeerObservationJSON(targets1[0], []string{"PORT_STATE_OPEN"}),
	})
	RequireTxSuccess(t, tx1)

	awaitAtLeastHeight(t, enforceHeight)

	require.Equal(t, "SUPERNODE_STATE_POSTPONED", querySupernodeLatestState(t, cli, n0.valAddr))
	require.Equal(t, "SUPERNODE_STATE_ACTIVE", querySupernodeLatestState(t, cli, n1.valAddr))
}
