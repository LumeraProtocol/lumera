//go:build system_test

package system

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/sjson"
)

// This test validates that ACTIVE probers must submit peer observations for all assigned targets.
func TestAuditSubmitReport_ProberRequiresAllPeerObservations(t *testing.T) {
	const (
		// Keep windows long enough in real time to avoid end-blocker enforcement during the test.
		reportingWindowBlocks = uint64(20)
	)
	const originHeight = int64(1)

	sut.ModifyGenesisJSON(t,
		setSupernodeParamsForAuditTests(t),
		setAuditParamsForFastWindows(t, reportingWindowBlocks, 1, 1, 1, []uint32{4444}),
		func(genesis []byte) []byte {
			// Avoid missing-report postponement before the window under test.
			state, err := sjson.SetRawBytes(genesis, "app_state.audit.params.consecutive_windows_to_postpone", []byte(strconv.FormatUint(2, 10)))
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
	awaitAtLeastHeight(t, windowStartHeight)

	self := auditSelfReportJSON([]string{"PORT_STATE_OPEN"})
	txResp := submitAuditReport(t, cli, n0.nodeName, windowID, self, nil)
	RequireTxFailure(t, txResp, "expected peer observations")
}
