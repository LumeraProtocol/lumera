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
		// This test submits multiple txs per epoch against a real chain. Keep enough
		// block slack that tx commit latency cannot push a report into the next epoch.
		epochLengthBlocks = uint64(30)
	)
	const originHeight = int64(1)

	sut.ModifyGenesisJSON(t,
		setSupernodeParamsForAuditTests(t),
		setAuditParamsForFastEpochs(t, epochLengthBlocks, 1, 1, 1, []uint32{4444}),
		setStorageTruthEnforcementModeUnspecified(t),
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
	//
	// In a 2-node UNSPECIFIED-mode network each node is always assigned the other:
	// n0→n1 and n1→n0.  We hardcode this rather than querying the chain to avoid
	// a race between anchor creation and the gRPC endpoint becoming consistent.
	awaitAtLeastHeight(t, epoch1Start, 120*time.Second)
	sut.AwaitNextBlock(t) // let anchor propagate to gRPC before submitting

	tx0e1 := submitEpochReport(t, cli, n0.nodeName, epochID1, hostOK, []string{
		storageChallengeObservationJSON(n1.accAddr, []string{"PORT_STATE_CLOSED"}),
	})
	RequireTxSuccess(t, tx0e1)

	tx1e1 := submitEpochReport(t, cli, n1.nodeName, epochID1, hostOK, []string{
		storageChallengeObservationJSON(n0.accAddr, []string{"PORT_STATE_OPEN"}),
	})
	RequireTxSuccess(t, tx1e1)

	awaitAtLeastHeight(t, epoch2Start, 120*time.Second)
	require.Equal(t, "SUPERNODE_STATE_ACTIVE", querySupernodeLatestState(t, cli, n1.valAddr))

	// Epoch 2: repeat CLOSED for node1 -> now node1 is POSTPONED at epoch2 end.
	awaitAtLeastHeight(t, epoch2Start, 120*time.Second)
	sut.AwaitNextBlock(t)

	tx0e2 := submitEpochReport(t, cli, n0.nodeName, epochID2, hostOK, []string{
		storageChallengeObservationJSON(n1.accAddr, []string{"PORT_STATE_CLOSED"}),
	})
	RequireTxSuccess(t, tx0e2)

	tx1e2 := submitEpochReport(t, cli, n1.nodeName, epochID2, hostOK, []string{
		storageChallengeObservationJSON(n0.accAddr, []string{"PORT_STATE_OPEN"}),
	})
	RequireTxSuccess(t, tx1e2)

	awaitAtLeastHeight(t, epoch3Start, 120*time.Second)
	require.Equal(t, "SUPERNODE_STATE_POSTPONED", querySupernodeLatestState(t, cli, n1.valAddr))

	// Epoch 3: node0 reports OPEN for node1; node1 (POSTPONED) submits host-only.
	// In UNSPECIFIED mode, POSTPONED nodes are still included as targets, so n0
	// is assigned n1 even when n1 is not ACTIVE.
	awaitAtLeastHeight(t, epoch3Start, 120*time.Second)
	sut.AwaitNextBlock(t)

	tx0e3 := submitEpochReport(t, cli, n0.nodeName, epochID3, hostOK, []string{
		storageChallengeObservationJSON(n1.accAddr, []string{"PORT_STATE_OPEN"}),
	})
	RequireTxSuccess(t, tx0e3)

	tx1e3 := submitEpochReport(t, cli, n1.nodeName, epochID3, hostOK, nil)
	RequireTxSuccess(t, tx1e3)

	awaitAtLeastHeight(t, epoch4Start, 120*time.Second)
	require.Equal(t, "SUPERNODE_STATE_ACTIVE", querySupernodeLatestState(t, cli, n1.valAddr))
}
