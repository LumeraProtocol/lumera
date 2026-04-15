//go:build system_test

package system

import (
	"testing"
	"time"

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

	currentHeight := sut.AwaitNextBlock(t, 12*time.Second)
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
	if sut.currentHeight < epoch1Start {
		sut.AwaitBlockHeight(t, epoch1Start, 20*time.Second)
	}
	assigned0e1 := auditQueryAssignedTargets(t, epochID1, true, n0.accAddr)
	assigned1e1 := auditQueryAssignedTargets(t, epochID1, true, n1.accAddr)
	tx0e1 := submitEpochReport(t, cli, n0.nodeName, epochID1, hostOK, buildObs(assigned0e1.TargetSupernodeAccounts, n1.accAddr))
	RequireTxSuccess(t, tx0e1)
	tx1e1 := submitEpochReport(t, cli, n1.nodeName, epochID1, hostOK, buildObs(assigned1e1.TargetSupernodeAccounts, ""))
	RequireTxSuccess(t, tx1e1)

	if sut.currentHeight < epoch2Start {
		sut.AwaitBlockHeight(t, epoch2Start, 20*time.Second)
	}

	// Epoch 2: repeat CLOSED-for-node1 observations on assigned targets.
	assigned0e2 := auditQueryAssignedTargets(t, epochID2, true, n0.accAddr)
	assigned1e2 := auditQueryAssignedTargets(t, epochID2, true, n1.accAddr)
	tx0e2 := submitEpochReport(t, cli, n0.nodeName, epochID2, hostOK, buildObs(assigned0e2.TargetSupernodeAccounts, n1.accAddr))
	RequireTxSuccess(t, tx0e2)
	tx1e2 := submitEpochReport(t, cli, n1.nodeName, epochID2, hostOK, buildObs(assigned1e2.TargetSupernodeAccounts, ""))
	RequireTxSuccess(t, tx1e2)

	if sut.currentHeight < epoch3Start {
		sut.AwaitBlockHeight(t, epoch3Start, 20*time.Second)
	}
	require.Equal(t, "SUPERNODE_STATE_POSTPONED", querySupernodeLatestState(t, cli, n1.valAddr))

	// Recovery can only happen on epochs where an eligible reporter submits OPEN
	// observations for node1. Assignment can vary by epoch, so retry a few epochs.
	recovered := false
	for i := int64(0); i < 10; i++ {
		epochID := epochID3 + uint64(i)
		epochStart := epoch3Start + i*int64(epochLengthBlocks)
		nextEpochStart := epochStart + int64(epochLengthBlocks)

		if sut.currentHeight < epochStart {
			sut.AwaitBlockHeight(t, epochStart, 20*time.Second)
		}
		assigned0 := auditQueryAssignedTargets(t, epochID, true, n0.accAddr)
		tx0 := submitEpochReport(t, cli, n0.nodeName, epochID, hostOK, buildObs(assigned0.TargetSupernodeAccounts, ""))
		RequireTxSuccess(t, tx0)

		tx1 := submitEpochReport(t, cli, n1.nodeName, epochID, hostOK, nil)
		RequireTxSuccess(t, tx1)

		if sut.currentHeight < nextEpochStart {
			sut.AwaitBlockHeight(t, nextEpochStart, 20*time.Second)
		}
		if querySupernodeLatestState(t, cli, n1.valAddr) == "SUPERNODE_STATE_ACTIVE" {
			recovered = true
			break
		}
	}
	require.True(t, recovered, "expected node1 to recover to ACTIVE within retry window")
}
