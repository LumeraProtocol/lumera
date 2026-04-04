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
			state, err := sjson.SetRawBytes(genesis, "app_state.audit.params.consecutive_epochs_to_postpone", []byte("2"))
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

	// Submit filler reports for the current epoch to prevent missing-report enforcement
	// from postponing supernodes before the test epochs start (consecutive_epochs_to_postpone=2).
	submitFillerReports(t, cli, originHeight, epochLengthBlocks, currentHeight, []testNodeIdentity{n0, n1})

	currentHeight = sut.AwaitNextBlock(t)
	epochID1, epoch1Start := nextEpochAfterHeight(originHeight, epochLengthBlocks, currentHeight)
	epochID2 := epochID1 + 1
	epoch2Start := epoch1Start + int64(epochLengthBlocks)
	enforce2 := epoch2Start + int64(epochLengthBlocks)

	hostOpen := auditHostReportJSON([]string{"PORT_STATE_OPEN"})

	// Window 1: node0 reports node1 as CLOSED, node1 reports node0 as OPEN.
	awaitAtLeastHeight(t, epoch1Start)

	assigned0e1 := auditQueryAssignedTargets(t, epochID1, true, n0.accAddr)
	require.Len(t, assigned0e1.TargetSupernodeAccounts, 1)
	assigned1e1 := auditQueryAssignedTargets(t, epochID1, true, n1.accAddr)
	require.Len(t, assigned1e1.TargetSupernodeAccounts, 1)

	tx0e1 := submitEpochReport(t, cli, n0.nodeName, epochID1, hostOpen, []string{
		storageChallengeObservationJSON(assigned0e1.TargetSupernodeAccounts[0], []string{"PORT_STATE_CLOSED"}),
	})
	RequireTxSuccess(t, tx0e1)
	tx1e1 := submitEpochReport(t, cli, n1.nodeName, epochID1, hostOpen, []string{
		storageChallengeObservationJSON(assigned1e1.TargetSupernodeAccounts[0], []string{"PORT_STATE_OPEN"}),
	})
	RequireTxSuccess(t, tx1e1)

	// Window 2: repeat -> node1 should be POSTPONED at window end due to consecutive unanimous CLOSED.
	awaitAtLeastHeight(t, epoch2Start)

	assigned0e2 := auditQueryAssignedTargets(t, epochID2, true, n0.accAddr)
	require.Len(t, assigned0e2.TargetSupernodeAccounts, 1)
	assigned1e2 := auditQueryAssignedTargets(t, epochID2, true, n1.accAddr)
	require.Len(t, assigned1e2.TargetSupernodeAccounts, 1)

	tx0e2 := submitEpochReport(t, cli, n0.nodeName, epochID2, hostOpen, []string{
		storageChallengeObservationJSON(assigned0e2.TargetSupernodeAccounts[0], []string{"PORT_STATE_CLOSED"}),
	})
	RequireTxSuccess(t, tx0e2)
	tx1e2 := submitEpochReport(t, cli, n1.nodeName, epochID2, hostOpen, []string{
		storageChallengeObservationJSON(assigned1e2.TargetSupernodeAccounts[0], []string{"PORT_STATE_OPEN"}),
	})
	RequireTxSuccess(t, tx1e2)

	awaitAtLeastHeight(t, enforce2)

	require.Equal(t, "SUPERNODE_STATE_ACTIVE", querySupernodeLatestState(t, cli, n0.valAddr))
	require.Equal(t, "SUPERNODE_STATE_POSTPONED", querySupernodeLatestState(t, cli, n1.valAddr))
}
