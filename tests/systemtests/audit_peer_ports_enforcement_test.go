//go:build system_test

package system

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/sjson"
)

func TestAuditPeerPortsUnanimousClosedPostponesAfterConsecutiveWindows(t *testing.T) {
	const (
		epochLengthBlocks = uint64(10)
	)
	const originHeight = int64(1)

	sut.ModifyGenesisJSON(t,
		setSupernodeParamsForAuditTests(t),
		setAuditParamsForFastEpochs(t, epochLengthBlocks, 1, 1, 1, []uint32{4444}),
		func(genesis []byte) []byte {
			state, err := sjson.SetRawBytes(genesis, "app_state.audit.params.consecutive_epochs_to_postpone", []byte("3"))
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
	epochID1, epoch1Start := nextEpochAfterHeight(originHeight, epochLengthBlocks, currentHeight)
	epochID2 := epochID1 + 1
	epochID3 := epochID2 + 1
	epoch2Start := epoch1Start + int64(epochLengthBlocks)
	epoch3Start := epoch2Start + int64(epochLengthBlocks)
	enforce3 := epoch3Start + int64(epochLengthBlocks)

	hostOpen := auditHostReportJSON([]string{"PORT_STATE_OPEN"})

	submitWindow := func(epochID uint64, epochStart int64) {
		awaitAtLeastHeight(t, epochStart)
		a0 := auditQueryAssignedTargets(t, epochID, true, n0.accAddr)
		a1 := auditQueryAssignedTargets(t, epochID, true, n1.accAddr)
		require.Len(t, a0.TargetSupernodeAccounts, 1)
		require.Len(t, a1.TargetSupernodeAccounts, 1)

		tx0 := submitEpochReport(t, cli, n0.nodeName, epochID, hostOpen, []string{
			storageChallengeObservationJSON(a0.TargetSupernodeAccounts[0], []string{"PORT_STATE_CLOSED"}),
		})
		RequireTxSuccess(t, tx0)
		tx1 := submitEpochReport(t, cli, n1.nodeName, epochID, hostOpen, []string{
			storageChallengeObservationJSON(a1.TargetSupernodeAccounts[0], []string{"PORT_STATE_OPEN"}),
		})
		RequireTxSuccess(t, tx1)
	}

	// 3 consecutive windows: node0 reports target as CLOSED, node1 reports OPEN.
	submitWindow(epochID1, epoch1Start)
	submitWindow(epochID2, epoch2Start)
	submitWindow(epochID3, epoch3Start)

	awaitAtLeastHeight(t, enforce3)

	require.Equal(t, "SUPERNODE_STATE_ACTIVE", querySupernodeLatestState(t, cli, n0.valAddr))
	require.Equal(t, "SUPERNODE_STATE_POSTPONED", querySupernodeLatestState(t, cli, n1.valAddr))
}
