//go:build system_test

package system

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/sjson"
)

func TestAuditRecovery_PostponedBecomesActiveWithSelfAndPeerOpen_NoHostThresholds(t *testing.T) {
	const (
		epochLengthBlocks = uint64(10)
	)
	const originHeight = int64(1)

	sut.ModifyGenesisJSON(t,
		setSupernodeParamsForAuditTests(t),
		setAuditParamsForFastEpochs(t, epochLengthBlocks, 1, 1, 1, []uint32{4444}),
		func(genesis []byte) []byte {
			// Use 2 consecutive windows to avoid setup-time missing-report postponements.
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
	epochID1, epoch1Start := nextEpochAfterHeight(originHeight, epochLengthBlocks, currentHeight)
	epochID2 := epochID1 + 1
	epochID3 := epochID2 + 1
	epoch3Start := epoch1Start + 2*int64(epochLengthBlocks)
	epoch4Start := epoch3Start + int64(epochLengthBlocks)
	epoch2Start := epoch1Start + int64(epochLengthBlocks)

	hostOK := auditHostReportJSON([]string{"PORT_STATE_OPEN"})

	// Epoch 1: node0 reports node1 as CLOSED; node1 reports OPEN for node0.
	// Not enough streak yet (consecutive=2), so node1 remains ACTIVE after epoch1.
	awaitAtLeastHeight(t, epoch1Start)
	assigned0e1 := auditQueryAssignedTargets(t, epochID1, true, n0.accAddr)
	require.Len(t, assigned0e1.TargetSupernodeAccounts, 1)
	require.Equal(t, n1.accAddr, assigned0e1.TargetSupernodeAccounts[0])
	obs0e1 := make([]string, 0, len(assigned0e1.TargetSupernodeAccounts))
	for _, target := range assigned0e1.TargetSupernodeAccounts {
		obs0e1 = append(obs0e1, storageChallengeObservationJSON(target, []string{"PORT_STATE_CLOSED"}))
	}
	tx0e1 := submitEpochReport(t, cli, n0.nodeName, epochID1, hostOK, obs0e1)
	RequireTxSuccess(t, tx0e1)

	assigned1e1 := auditQueryAssignedTargets(t, epochID1, true, n1.accAddr)
	require.Len(t, assigned1e1.TargetSupernodeAccounts, 1)
	require.Equal(t, n0.accAddr, assigned1e1.TargetSupernodeAccounts[0])
	obs1e1 := make([]string, 0, len(assigned1e1.TargetSupernodeAccounts))
	for _, target := range assigned1e1.TargetSupernodeAccounts {
		obs1e1 = append(obs1e1, storageChallengeObservationJSON(target, []string{"PORT_STATE_OPEN"}))
	}
	tx1e1 := submitEpochReport(t, cli, n1.nodeName, epochID1, hostOK, obs1e1)
	RequireTxSuccess(t, tx1e1)

	awaitAtLeastHeight(t, epoch2Start)
	require.Equal(t, "SUPERNODE_STATE_ACTIVE", querySupernodeLatestState(t, cli, n1.valAddr))

	// Epoch 2: repeat CLOSED for node1 -> now node1 is POSTPONED at epoch2 end.
	assigned0e2 := auditQueryAssignedTargets(t, epochID2, true, n0.accAddr)
	require.Len(t, assigned0e2.TargetSupernodeAccounts, 1)
	require.Equal(t, n1.accAddr, assigned0e2.TargetSupernodeAccounts[0])
	obs0e2 := make([]string, 0, len(assigned0e2.TargetSupernodeAccounts))
	for _, target := range assigned0e2.TargetSupernodeAccounts {
		obs0e2 = append(obs0e2, storageChallengeObservationJSON(target, []string{"PORT_STATE_CLOSED"}))
	}
	tx0e2 := submitEpochReport(t, cli, n0.nodeName, epochID2, hostOK, obs0e2)
	RequireTxSuccess(t, tx0e2)

	assigned1e2 := auditQueryAssignedTargets(t, epochID2, true, n1.accAddr)
	require.Len(t, assigned1e2.TargetSupernodeAccounts, 1)
	require.Equal(t, n0.accAddr, assigned1e2.TargetSupernodeAccounts[0])
	obs1e2 := make([]string, 0, len(assigned1e2.TargetSupernodeAccounts))
	for _, target := range assigned1e2.TargetSupernodeAccounts {
		obs1e2 = append(obs1e2, storageChallengeObservationJSON(target, []string{"PORT_STATE_OPEN"}))
	}
	tx1e2 := submitEpochReport(t, cli, n1.nodeName, epochID2, hostOK, obs1e2)
	RequireTxSuccess(t, tx1e2)

	awaitAtLeastHeight(t, epoch3Start)
	require.Equal(t, "SUPERNODE_STATE_POSTPONED", querySupernodeLatestState(t, cli, n1.valAddr))

	// Epoch 3: node0 reports OPEN for node1; node1 (POSTPONED) submits host-only.
	// This satisfies recovery conditions at epoch3 end.
	assigned0e3 := auditQueryAssignedTargets(t, epochID3, true, n0.accAddr)
	require.Len(t, assigned0e3.TargetSupernodeAccounts, 1)
	require.Equal(t, n1.accAddr, assigned0e3.TargetSupernodeAccounts[0])
	obs0e3 := []string{
		storageChallengeObservationJSON(n1.accAddr, []string{"PORT_STATE_OPEN"}),
	}
	tx0e3 := submitEpochReport(t, cli, n0.nodeName, epochID3, hostOK, obs0e3)
	RequireTxSuccess(t, tx0e3)

	tx1e3 := submitEpochReport(t, cli, n1.nodeName, epochID3, hostOK, nil)
	RequireTxSuccess(t, tx1e3)

	awaitAtLeastHeight(t, epoch4Start)
	require.Equal(t, "SUPERNODE_STATE_ACTIVE", querySupernodeLatestState(t, cli, n1.valAddr))
}
