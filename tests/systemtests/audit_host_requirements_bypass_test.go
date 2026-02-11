//go:build system_test

package system

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/sjson"
)

func TestAuditHostRequirements_UsageZeroBypassesMinimums(t *testing.T) {
	const (
		epochLengthBlocks = uint64(10)
	)
	const originHeight = int64(1)

	sut.ModifyGenesisJSON(t,
		setSupernodeParamsForAuditTests(t),
		setAuditParamsForFastEpochs(t, epochLengthBlocks, 1, 1, 1, []uint32{4444}),
		func(genesis []byte) []byte {
			// Avoid missing-report postponement before/around the tested epoch.
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

	// Use the on-chain assignment query so tests track current assignment logic.
	assigned0 := auditQueryAssignedTargets(t, epochID, true, n0.accAddr)
	require.Equal(t, epochID, assigned0.EpochId)
	require.Len(t, assigned0.TargetSupernodeAccounts, 1)
	require.Equal(t, []uint32{4444}, assigned0.RequiredOpenPorts)

	assigned1 := auditQueryAssignedTargets(t, epochID, true, n1.accAddr)
	require.Equal(t, epochID, assigned1.EpochId)
	require.Len(t, assigned1.TargetSupernodeAccounts, 1)
	require.Equal(t, []uint32{4444}, assigned1.RequiredOpenPorts)

	unknownSelfBz, err := json.Marshal(map[string]any{
		"cpu_usage_percent":    0.0,
		"mem_usage_percent":    0.0,
		"disk_usage_percent":   0.0,
		"inbound_port_states":  []string{"PORT_STATE_OPEN"},
		"failed_actions_count": 0,
	})
	require.NoError(t, err)
	unknownSelf := string(unknownSelfBz)

	okHost := auditHostReportJSON([]string{"PORT_STATE_OPEN"})

	// node0 reports "unknown" cpu usage (0), which must not trigger host-requirements postponement.
	tx0 := submitEpochReport(t, cli, n0.nodeName, epochID, unknownSelf, []string{
		storageChallengeObservationJSON(assigned0.TargetSupernodeAccounts[0], []string{"PORT_STATE_OPEN"}),
	})
	RequireTxSuccess(t, tx0)

	tx1 := submitEpochReport(t, cli, n1.nodeName, epochID, okHost, []string{
		storageChallengeObservationJSON(assigned1.TargetSupernodeAccounts[0], []string{"PORT_STATE_OPEN"}),
	})
	RequireTxSuccess(t, tx1)

	// Verify the report was persisted with unknown host usage values.
	r0 := auditQueryReport(t, epochID, n0.accAddr)
	require.Equal(t, n0.accAddr, r0.SupernodeAccount)
	require.EqualValues(t, 0, r0.HostReport.CpuUsagePercent)
	require.EqualValues(t, 0, r0.HostReport.MemUsagePercent)
	require.EqualValues(t, 0, r0.HostReport.DiskUsagePercent)
	require.Len(t, r0.StorageChallengeObservations, 1)
	require.Equal(t, assigned0.TargetSupernodeAccounts[0], r0.StorageChallengeObservations[0].TargetSupernodeAccount)

	awaitAtLeastHeight(t, enforceHeight)

	require.Equal(t, "SUPERNODE_STATE_ACTIVE", querySupernodeLatestState(t, cli, n0.valAddr))
	require.Equal(t, "SUPERNODE_STATE_ACTIVE", querySupernodeLatestState(t, cli, n1.valAddr))
}
