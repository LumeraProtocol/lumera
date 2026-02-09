//go:build system_test

package system

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/sjson"
)

func TestAuditRecovery_PostponedBecomesActiveWithSelfAndPeerOpen(t *testing.T) {
	const (
		epochLengthBlocks = uint64(10)
	)
	const originHeight = int64(1)

	sut.ModifyGenesisJSON(t,
		setSupernodeParamsForAuditTests(t),
		setAuditParamsForFastEpochs(t, epochLengthBlocks, 1, 1, 1, []uint32{4444}),
		func(genesis []byte) []byte {
			// Keep missing-report / peer-port streaks from interfering with this recovery test.
			state, err := sjson.SetRawBytes(genesis, "app_state.audit.params.consecutive_epochs_to_postpone", []byte("10"))
			require.NoError(t, err)

			// Use host requirements to get into POSTPONED state deterministically.
			state, err = sjson.SetRawBytes(state, "app_state.audit.params.min_cpu_free_percent", []byte("90"))
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

	senders := sortedStrings(n0.accAddr, n1.accAddr)
	receivers := sortedStrings(n0.accAddr, n1.accAddr)
	kEpoch := computeKEpoch(1, 1, 1, len(senders), len(receivers))
	require.Equal(t, uint32(1), kEpoch)

	hostOK := auditHostReportJSON([]string{"PORT_STATE_OPEN"})

	badSelfBz, err := json.Marshal(map[string]any{
		"cpu_usage_percent":    99.0,
		"mem_usage_percent":    1.0,
		"disk_usage_percent":   1.0,
		"inbound_port_states":  []string{"PORT_STATE_OPEN"},
		"failed_actions_count": 0,
	})
	require.NoError(t, err)
	hostBad := string(badSelfBz)

	// Epoch 1: node1 violates host requirements -> becomes POSTPONED.
	awaitAtLeastHeight(t, epoch1Start)
	seed1 := headerHashAtHeight(t, sut.rpcAddr, epoch1Start)
	targets0e1, ok := assignedTargets(seed1, senders, receivers, kEpoch, n0.accAddr)
	require.True(t, ok)
	require.Len(t, targets0e1, 1)
	targets1e1, ok := assignedTargets(seed1, senders, receivers, kEpoch, n1.accAddr)
	require.True(t, ok)
	require.Len(t, targets1e1, 1)

	tx0e1 := submitEpochReport(t, cli, n0.nodeName, epochID1, hostOK, []string{
		storageChallengeObservationJSON(targets0e1[0], []string{"PORT_STATE_OPEN"}),
	})
	RequireTxSuccess(t, tx0e1)
	tx1e1 := submitEpochReport(t, cli, n1.nodeName, epochID1, hostBad, []string{
		storageChallengeObservationJSON(targets1e1[0], []string{"PORT_STATE_OPEN"}),
	})
	RequireTxSuccess(t, tx1e1)

	awaitAtLeastHeight(t, epoch2Start)
	require.Equal(t, "SUPERNODE_STATE_POSTPONED", querySupernodeLatestState(t, cli, n1.valAddr))

	// Epoch 2: node1 submits compliant host report (no storage challenge observations),
	// and node0 submits an OPEN storage challenge observation about node1 in the same epoch -> recovery at epoch end.
	seed2 := headerHashAtHeight(t, sut.rpcAddr, epoch2Start)
	targets0e2, ok := assignedTargets(seed2, senders, receivers, kEpoch, n0.accAddr)
	require.True(t, ok)
	require.Len(t, targets0e2, 1)

	tx0e2 := submitEpochReport(t, cli, n0.nodeName, epochID2, hostOK, []string{
		storageChallengeObservationJSON(targets0e2[0], []string{"PORT_STATE_OPEN"}),
	})
	RequireTxSuccess(t, tx0e2)

	tx1e2 := submitEpochReport(t, cli, n1.nodeName, epochID2, hostOK, nil)
	RequireTxSuccess(t, tx1e2)

	awaitAtLeastHeight(t, enforce2)
	require.Equal(t, "SUPERNODE_STATE_ACTIVE", querySupernodeLatestState(t, cli, n1.valAddr))
}
