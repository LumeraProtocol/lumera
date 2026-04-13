//go:build system_test

package system

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/sjson"
)

func TestAuditRecovery_PostponedBecomesActiveWithSelfAndPeerOpen_NoHostThresholds(t *testing.T) {
	const (
		// Use 20-block epochs so that chain setup (StartChain + CLI init + 2 registrations,
		// ~10-12 blocks) always completes within epoch 0. This prevents missing-report
		// enforcement from postponing supernodes before the test's target epochs start.
		epochLengthBlocks = uint64(20)
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
	epochID1, epoch1Start := nextEpochAfterHeight(originHeight, epochLengthBlocks, currentHeight)
	epochID2 := epochID1 + 1
	epochID3 := epochID2 + 1
	epoch3Start := epoch1Start + 2*int64(epochLengthBlocks)
	epoch2Start := epoch1Start + int64(epochLengthBlocks)

	hostOK := auditHostReportJSON([]string{"PORT_STATE_OPEN"})

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

	// Epoch 1: whichever reporter is assigned node1 reports CLOSED for node1.
	// Not enough streak yet (consecutive=2), so node1 should remain ACTIVE after epoch1.
	awaitAtLeastHeight(t, epoch1Start)
	assigned0e1 := auditQueryAssignedTargets(t, epochID1, true, n0.accAddr)
	assigned1e1 := auditQueryAssignedTargets(t, epochID1, true, n1.accAddr)
	tx0e1 := submitEpochReport(t, cli, n0.nodeName, epochID1, hostOK, buildObs(assigned0e1.TargetSupernodeAccounts, n1.accAddr))
	RequireTxSuccess(t, tx0e1)
	tx1e1 := submitEpochReport(t, cli, n1.nodeName, epochID1, hostOK, buildObs(assigned1e1.TargetSupernodeAccounts, ""))
	RequireTxSuccess(t, tx1e1)

	awaitAtLeastHeight(t, epoch2Start)

	// Epoch 2: repeat CLOSED-for-node1 observations on assigned targets.
	assigned0e2 := auditQueryAssignedTargets(t, epochID2, true, n0.accAddr)
	assigned1e2 := auditQueryAssignedTargets(t, epochID2, true, n1.accAddr)
	tx0e2 := submitEpochReport(t, cli, n0.nodeName, epochID2, hostOK, buildObs(assigned0e2.TargetSupernodeAccounts, n1.accAddr))
	RequireTxSuccess(t, tx0e2)
	tx1e2 := submitEpochReport(t, cli, n1.nodeName, epochID2, hostOK, buildObs(assigned1e2.TargetSupernodeAccounts, ""))
	RequireTxSuccess(t, tx1e2)

	awaitAtLeastHeight(t, epoch3Start)
	require.Equal(t, "SUPERNODE_STATE_POSTPONED", querySupernodeLatestState(t, cli, n1.valAddr))

	// Recovery can only happen on epochs where an eligible reporter submits OPEN
	// observations for node1. Assignment can vary by epoch, so retry a few epochs.
	recovered := false
	for i := int64(0); i < 4; i++ {
		epochID := epochID3 + uint64(i)
		epochStart := epoch3Start + i*int64(epochLengthBlocks)
		nextEpochStart := epochStart + int64(epochLengthBlocks)

		awaitAtLeastHeight(t, epochStart)
		assigned0 := auditQueryAssignedTargets(t, epochID, true, n0.accAddr)
		tx0 := submitEpochReport(t, cli, n0.nodeName, epochID, hostOK, buildObs(assigned0.TargetSupernodeAccounts, ""))
		RequireTxSuccess(t, tx0)

		tx1 := submitEpochReport(t, cli, n1.nodeName, epochID, hostOK, nil)
		RequireTxSuccess(t, tx1)

		awaitAtLeastHeight(t, nextEpochStart)
		if querySupernodeLatestState(t, cli, n1.valAddr) == "SUPERNODE_STATE_ACTIVE" {
			recovered = true
			break
		}
	}
	require.True(t, recovered, "expected node1 to recover to ACTIVE within retry window")
}
