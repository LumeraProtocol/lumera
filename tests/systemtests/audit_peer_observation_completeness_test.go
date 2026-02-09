//go:build system_test

package system

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/sjson"
)

// This test validates that ACTIVE probers must submit storage challenge observations for all assigned targets.
func TestAuditSubmitReport_ProberRequiresAllPeerObservations(t *testing.T) {
	const (
		// Keep epochs long enough in real time to avoid end-blocker enforcement during the test.
		epochLengthBlocks = uint64(20)
	)
	const originHeight = int64(1)

	sut.ModifyGenesisJSON(t,
		setSupernodeParamsForAuditTests(t),
		setAuditParamsForFastEpochs(t, epochLengthBlocks, 1, 1, 1, []uint32{4444}),
		func(genesis []byte) []byte {
			// Avoid missing-report postponement before the epoch under test.
			state, err := sjson.SetRawBytes(genesis, "app_state.audit.params.consecutive_epochs_to_postpone", []byte(strconv.FormatUint(2, 10)))
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
	epochID, epochStartHeight := nextEpochAfterHeight(originHeight, epochLengthBlocks, currentHeight)
	awaitAtLeastHeight(t, epochStartHeight)

	host := auditHostReportJSON([]string{"PORT_STATE_OPEN"})
	txResp := submitEpochReport(t, cli, n0.nodeName, epochID, host, nil)
	RequireTxFailure(t, txResp, "expected storage challenge observations")
}
