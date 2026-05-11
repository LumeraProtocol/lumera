//go:build system_test

package system

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestAuditCLIQueriesE2E(t *testing.T) {
	const epochLengthBlocks = uint64(20)

	sut.ModifyGenesisJSON(t,
		setSupernodeParamsForAuditTests(t),
		setAuditParamsForFastEpochs(t, epochLengthBlocks, 1, 1, 1, []uint32{4444}),
		// Per CP3 — k-based peer-assignment requires UNSPECIFIED enforcement mode (SHADOW activates one-third formula).
		setStorageTruthEnforcementModeUnspecified(t),
	)
	sut.StartChain(t)

	cli := NewLumeradCLI(t, sut, true)
	n0 := getNodeIdentity(t, cli, "node0")
	n1 := getNodeIdentity(t, cli, "node1")

	registerSupernode(t, cli, n0, "192.168.1.1")
	registerSupernode(t, cli, n1, "192.168.1.2")

	ws0 := auditQueryAssignedTargets(t, 0, false, n0.accAddr)
	host := auditHostReportJSON([]string{"PORT_STATE_OPEN"})
	RequireTxSuccess(t, submitEpochReport(t, cli, n0.nodeName, ws0.EpochId, host, nil))
	RequireTxSuccess(t, submitEpochReport(t, cli, n1.nodeName, ws0.EpochId, host, nil))

	// awaitAtLeastHeightWithSlackPeerPorts unified into awaitAtLeastHeight during PR #122 rebase.
	awaitAtLeastHeight(t, ws0.EpochStartHeight+int64(epochLengthBlocks), 45*time.Second)

	assignedRaw := cli.CustomQuery("q", "audit", "assigned-targets", n0.accAddr, "--epoch-id", strconv.FormatUint(ws0.EpochId+1, 10), "--filter-by-epoch-id")
	assignedEpochID := gjsonUint64(gjson.Get(assignedRaw, "epoch_id"))
	require.Equal(t, ws0.EpochId+1, assignedEpochID, "assigned-targets should honor the requested epoch")
	require.NotEmpty(t, gjson.Get(assignedRaw, "target_supernode_accounts").Array(), "assigned-targets should return target accounts")

	currentAnchorRaw := cli.CustomQuery("q", "audit", "current-epoch-anchor")
	currentAnchorEpochID := gjsonUint64(gjson.Get(currentAnchorRaw, "anchor.epoch_id"))
	require.Equal(t, ws0.EpochId+1, currentAnchorEpochID, "current-epoch-anchor should return the current epoch anchor")
	require.NotEmpty(t, gjson.Get(currentAnchorRaw, "anchor.seed").String(), "current-epoch-anchor should expose the epoch seed")

	epochAnchorRaw := cli.CustomQuery("q", "audit", "epoch-anchor", strconv.FormatUint(currentAnchorEpochID, 10))
	require.Equal(t, currentAnchorEpochID, gjsonUint64(gjson.Get(epochAnchorRaw, "anchor.epoch_id")))
	require.Equal(t, gjson.Get(currentAnchorRaw, "anchor.seed").String(), gjson.Get(epochAnchorRaw, "anchor.seed").String())
}
