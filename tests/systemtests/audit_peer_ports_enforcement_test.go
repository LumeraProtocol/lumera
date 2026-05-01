//go:build system_test

package system

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/sjson"
)

func TestAuditPeerPortsUnanimousClosedPostponesAfterConsecutiveWindows(t *testing.T) {
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
		setStorageTruthEnforcementModeUnspecified(t),
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

	hostOpen := auditHostReportJSON([]string{"PORT_STATE_OPEN"})

	// Window 1: wait for the real persisted anchor to include both registered reporters,
	// then use the on-chain assignment query instead of locally guessing epoch membership.
	anchor1 := awaitCurrentEpochAnchorWithActiveSupernodes(t, 0, n0.accAddr, n1.accAddr)
	assigned0e1 := auditQueryAssignedTargets(t, anchor1.EpochId, true, n0.accAddr)
	require.Len(t, assigned0e1.TargetSupernodeAccounts, 1)
	assigned1e1 := auditQueryAssignedTargets(t, anchor1.EpochId, true, n1.accAddr)
	require.Len(t, assigned1e1.TargetSupernodeAccounts, 1)

	tx0e1 := submitEpochReport(t, cli, n0.nodeName, anchor1.EpochId, hostOpen, []string{
		storageChallengeObservationJSON(assigned0e1.TargetSupernodeAccounts[0], []string{"PORT_STATE_CLOSED"}),
	})
	RequireTxSuccess(t, tx0e1)
	tx1e1 := submitEpochReport(t, cli, n1.nodeName, anchor1.EpochId, hostOpen, []string{
		storageChallengeObservationJSON(assigned1e1.TargetSupernodeAccounts[0], []string{"PORT_STATE_OPEN"}),
	})
	RequireTxSuccess(t, tx1e1)

	// Window 2: repeat in the next anchored epoch. The target should be POSTPONED
	// at epoch end due to consecutive unanimous CLOSED observations.
	anchor2 := awaitCurrentEpochAnchorWithActiveSupernodes(t, anchor1.EpochId+1, n0.accAddr, n1.accAddr)
	assigned0e2 := auditQueryAssignedTargets(t, anchor2.EpochId, true, n0.accAddr)
	require.Len(t, assigned0e2.TargetSupernodeAccounts, 1)
	assigned1e2 := auditQueryAssignedTargets(t, anchor2.EpochId, true, n1.accAddr)
	require.Len(t, assigned1e2.TargetSupernodeAccounts, 1)

	tx0e2 := submitEpochReport(t, cli, n0.nodeName, anchor2.EpochId, hostOpen, []string{
		storageChallengeObservationJSON(assigned0e2.TargetSupernodeAccounts[0], []string{"PORT_STATE_CLOSED"}),
	})
	RequireTxSuccess(t, tx0e2)
	tx1e2 := submitEpochReport(t, cli, n1.nodeName, anchor2.EpochId, hostOpen, []string{
		storageChallengeObservationJSON(assigned1e2.TargetSupernodeAccounts[0], []string{"PORT_STATE_OPEN"}),
	})
	RequireTxSuccess(t, tx1e2)

	awaitAtLeastHeight(t, anchor2.EpochEndHeight+1)

	require.Equal(t, "SUPERNODE_STATE_ACTIVE", querySupernodeLatestState(t, cli, n0.valAddr))
	require.Equal(t, "SUPERNODE_STATE_POSTPONED", querySupernodeLatestState(t, cli, n1.valAddr))
}
