//go:build system_test

package system

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/sjson"
)

func awaitAtLeastHeightWithSlackPeerPorts(t *testing.T, height int64) {
	t.Helper()
	if sut.currentHeight >= height {
		return
	}
	sut.AwaitBlockHeight(t, height, 45*time.Second)
}

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
	epoch2Start := epoch1Start + int64(epochLengthBlocks)
	enforce2 := epoch2Start + int64(epochLengthBlocks)
	epochID3 := epochID2 + 1
	epoch3Start := epoch2Start + int64(epochLengthBlocks)
	enforce3 := epoch3Start + int64(epochLengthBlocks)

	hostOpen := auditHostReportJSON([]string{"PORT_STATE_OPEN"})

	buildObs := func(targets []string, closeFor string) []string {
		obs := make([]string, 0, len(targets))
		for _, target := range targets {
			state := []string{"PORT_STATE_OPEN"}
			if target == closeFor {
				state = []string{"PORT_STATE_CLOSED"}
			}
			obs = append(obs, storageChallengeObservationJSON(target, state))
		}
		return obs
	}

	// Window 1: report using keeper-assigned targets for this epoch.
	awaitAtLeastHeightWithSlackPeerPorts(t, epoch1Start)
	assigned0e1 := auditQueryAssignedTargets(t, epochID1, true, n0.accAddr)
	assigned1e1 := auditQueryAssignedTargets(t, epochID1, true, n1.accAddr)

	tx0e1 := submitEpochReport(t, cli, n0.nodeName, epochID1, hostOpen, buildObs(assigned0e1.TargetSupernodeAccounts, n1.accAddr))
	RequireTxSuccess(t, tx0e1)
	tx1e1 := submitEpochReport(t, cli, n1.nodeName, epochID1, hostOpen, buildObs(assigned1e1.TargetSupernodeAccounts, ""))
	RequireTxSuccess(t, tx1e1)

	// Window 2: repeat CLOSED observation, still below the 3-epoch postponement threshold.
	awaitAtLeastHeightWithSlackPeerPorts(t, epoch2Start)
	assigned0e2 := auditQueryAssignedTargets(t, epochID2, true, n0.accAddr)
	assigned1e2 := auditQueryAssignedTargets(t, epochID2, true, n1.accAddr)

	tx0e2 := submitEpochReport(t, cli, n0.nodeName, epochID2, hostOpen, buildObs(assigned0e2.TargetSupernodeAccounts, n1.accAddr))
	RequireTxSuccess(t, tx0e2)
	tx1e2 := submitEpochReport(t, cli, n1.nodeName, epochID2, hostOpen, buildObs(assigned1e2.TargetSupernodeAccounts, ""))
	RequireTxSuccess(t, tx1e2)

	awaitAtLeastHeightWithSlackPeerPorts(t, enforce2)
	require.Equal(t, "SUPERNODE_STATE_ACTIVE", querySupernodeLatestState(t, cli, n0.valAddr))
	require.Equal(t, "SUPERNODE_STATE_ACTIVE", querySupernodeLatestState(t, cli, n1.valAddr))

	// Window 3: third consecutive unanimous CLOSED should postpone node1.
	awaitAtLeastHeightWithSlackPeerPorts(t, epoch3Start)
	assigned0e3 := auditQueryAssignedTargets(t, epochID3, true, n0.accAddr)
	assigned1e3 := auditQueryAssignedTargets(t, epochID3, true, n1.accAddr)

	tx0e3 := submitEpochReport(t, cli, n0.nodeName, epochID3, hostOpen, buildObs(assigned0e3.TargetSupernodeAccounts, n1.accAddr))
	RequireTxSuccess(t, tx0e3)
	tx1e3 := submitEpochReport(t, cli, n1.nodeName, epochID3, hostOpen, buildObs(assigned1e3.TargetSupernodeAccounts, ""))
	RequireTxSuccess(t, tx1e3)

	awaitAtLeastHeightWithSlackPeerPorts(t, enforce3)

	require.Equal(t, "SUPERNODE_STATE_ACTIVE", querySupernodeLatestState(t, cli, n0.valAddr))
	require.Equal(t, "SUPERNODE_STATE_POSTPONED", querySupernodeLatestState(t, cli, n1.valAddr))
}
