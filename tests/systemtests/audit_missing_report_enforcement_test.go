//go:build system_test

package system

// This test validates missing-report enforcement in EndBlocker:
// - two ACTIVE supernodes exist during an epoch
// - only one submits a report for that epoch
// - after `epoch_end + 1`, the missing sender is POSTPONED

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAuditMissingReportPostponesSender(t *testing.T) {
	const (
		epochLengthBlocks = uint64(10)
	)
	const originHeight = int64(1)

	sut.ModifyGenesisJSON(t,
		setSupernodeParamsForAuditTests(t),
		setAuditParamsForFastEpochs(t, epochLengthBlocks, 1, 1, 1, []uint32{4444}),
	)
	sut.StartChain(t)

	cli := NewLumeradCLI(t, sut, true)
	n0 := getNodeIdentity(t, cli, "node0")
	n1 := getNodeIdentity(t, cli, "node1")

	registerSupernode(t, cli, n0, "192.168.1.1")
	registerSupernode(t, cli, n1, "192.168.1.2")

	currentHeight := sut.AwaitNextBlock(t)
	blocks := int64(epochLengthBlocks)

	var epochID uint64
	var epochStartHeight int64
	var epochEndHeight int64
	for {
		epochID = uint64((currentHeight - originHeight) / blocks)
		epochStartHeight = originHeight + int64(epochID)*blocks
		epochEndHeight = epochStartHeight + blocks - 1
		// Ensure there's enough room in the epoch so the tx is committed before the next epoch starts.
		if epochEndHeight-currentHeight >= 3 {
			break
		}
		currentHeight = sut.AwaitNextBlock(t)
	}
	enforceHeight := epochEndHeight + 1

	host := auditHostReportJSON([]string{"PORT_STATE_OPEN"})
	// node0 submits a report; node1 submits nothing in this epoch.
	txResp := submitEpochReport(t, cli, n0.nodeName, epochID, host, nil)
	RequireTxSuccess(t, txResp)

	// node1 does not submit any report for this epoch -> should be postponed after the epoch ends.
	awaitAtLeastHeight(t, enforceHeight)

	require.Equal(t, "SUPERNODE_STATE_POSTPONED", querySupernodeLatestState(t, cli, n1.valAddr))
}
