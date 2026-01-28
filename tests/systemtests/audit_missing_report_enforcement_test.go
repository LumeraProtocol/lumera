//go:build system_test

package system

// This test validates missing-report enforcement in EndBlocker:
// - two ACTIVE supernodes exist during a window
// - only one submits a report for that window
// - after `window_end + 1`, the missing sender is POSTPONED

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAuditMissingReportPostponesSender(t *testing.T) {
	const (
		reportingWindowBlocks = uint64(10)
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
	blocks := int64(reportingWindowBlocks)

	var windowID uint64
	var windowStartHeight int64
	var windowEndHeight int64
	for {
		windowID = uint64((currentHeight - originHeight) / blocks)
		windowStartHeight = originHeight + int64(windowID)*blocks
		windowEndHeight = windowStartHeight + blocks - 1
		// Ensure there's enough room in the window so the tx is committed before the next window starts.
		if windowEndHeight-currentHeight >= 3 {
			break
		}
		currentHeight = sut.AwaitNextBlock(t)
	}
	enforceHeight := windowEndHeight + 1

	self := auditSelfReportJSON([]string{"PORT_STATE_OPEN"})
	// node0 submits a report; node1 submits nothing in this window.
	txResp := submitAuditReport(t, cli, n0.nodeName, windowID, self, nil)
	RequireTxSuccess(t, txResp)

	// node1 does not submit any report for this window -> should be postponed after the window ends.
	awaitAtLeastHeight(t, enforceHeight)

	require.Equal(t, "SUPERNODE_STATE_POSTPONED", querySupernodeLatestState(t, cli, n1.valAddr))
}
