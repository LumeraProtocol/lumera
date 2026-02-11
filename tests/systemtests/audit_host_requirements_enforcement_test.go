//go:build system_test

package system

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/sjson"
)

func TestAuditHostRequirements_NoThresholdsDoNotPostponeActiveSupernode(t *testing.T) {
	const (
		// Keep epochs small so AwaitBlockHeight timeouts don't flake.
		epochLengthBlocks = uint64(10)
	)
	const originHeight = int64(1)

	sut.ModifyGenesisJSON(t,
		setSupernodeParamsForAuditTests(t),
		setAuditParamsForFastEpochs(t, epochLengthBlocks, 1, 1, 1, []uint32{4444}),
		func(genesis []byte) []byte {
			// Avoid missing-report postponement before the first tested epoch.
			state, err := sjson.SetRawBytes(genesis, "app_state.audit.params.consecutive_epochs_to_postpone", []byte("10"))
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
	epochID, epochStartHeight := nextEpochAfterHeight(originHeight, epochLengthBlocks, currentHeight)
	enforceHeight := epochStartHeight + int64(epochLengthBlocks)

	awaitAtLeastHeight(t, epochStartHeight)

	assigned0 := auditQueryAssignedTargets(t, epochID, true, n0.accAddr)
	require.Len(t, assigned0.TargetSupernodeAccounts, 1)

	assigned1 := auditQueryAssignedTargets(t, epochID, true, n1.accAddr)
	require.Len(t, assigned1.TargetSupernodeAccounts, 1)

	badSelfBz, err := json.Marshal(map[string]any{
		"cpu_usage_percent":    99.0,
		"mem_usage_percent":    1.0,
		"disk_usage_percent":   1.0,
		"inbound_port_states":  []string{"PORT_STATE_OPEN"},
		"failed_actions_count": 0,
	})
	require.NoError(t, err)
	badSelf := string(badSelfBz)

	okHost := auditHostReportJSON([]string{"PORT_STATE_OPEN"})

	// node0 reports high CPU usage. With threshold params left at defaults, this should not postpone.
	tx0 := submitEpochReport(t, cli, n0.nodeName, epochID, badSelf, []string{
		storageChallengeObservationJSON(assigned0.TargetSupernodeAccounts[0], []string{"PORT_STATE_OPEN"}),
	})
	RequireTxSuccess(t, tx0)

	// node1 also submits to avoid missing-report enforcement.
	tx1 := submitEpochReport(t, cli, n1.nodeName, epochID, okHost, []string{
		storageChallengeObservationJSON(assigned1.TargetSupernodeAccounts[0], []string{"PORT_STATE_OPEN"}),
	})
	RequireTxSuccess(t, tx1)

	awaitAtLeastHeight(t, enforceHeight)

	require.Equal(t, "SUPERNODE_STATE_ACTIVE", querySupernodeLatestState(t, cli, n0.valAddr))
	require.Equal(t, "SUPERNODE_STATE_ACTIVE", querySupernodeLatestState(t, cli, n1.valAddr))
}
