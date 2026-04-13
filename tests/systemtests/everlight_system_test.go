//go:build system_test

package system

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	lcfg "github.com/LumeraProtocol/lumera/config"
)

func everlightHostReportJSON(diskUsagePercent float64) string {
	bz, _ := json.Marshal(map[string]any{
		"cpu_usage_percent":    10.0,
		"mem_usage_percent":    10.0,
		"disk_usage_percent":   diskUsagePercent,
		"failed_actions_count": 0,
		"inbound_port_states":  []string{},
	})
	return string(bz)
}

func TestEverlightSystem_AuditDrivesStorageFullState(t *testing.T) {
	const (
		epochLengthBlocks = uint64(10)
	)

	sut.ModifyGenesisJSON(t,
		setSupernodeParamsForAuditTests(t),
		setAuditParamsForFastEpochs(t, epochLengthBlocks, 1, 1, 1, []uint32{4444}),
		func(genesis []byte) []byte {
			state, err := sjson.SetRawBytes(genesis, "app_state.audit.params.consecutive_epochs_to_postpone", []byte("100"))
			require.NoError(t, err)
			return state
		},
	)
	sut.StartChain(t)

	cli := NewLumeradCLI(t, sut, true)
	n0 := getNodeIdentity(t, cli, "node0")
	registerSupernode(t, cli, n0, "192.168.1.1")

	paramsResp := cli.CustomQuery("q", "supernode", "params")
	t.Logf("supernode params: %s", paramsResp)
	require.Equal(t, int64(90), gjson.Get(paramsResp, "params.max_storage_usage_percent").Int())
	beforeSN := cli.CustomQuery("q", "supernode", "get-supernode", n0.valAddr)
	t.Logf("supernode before report: %s", beforeSN)

	currentHeight := sut.AwaitNextBlock(t)
	epochID1, epoch1Start := nextEpochAfterHeight(1, epochLengthBlocks, currentHeight)
	awaitAtLeastHeight(t, epoch1Start)

	// Report high disk usage through audit epoch report -> STORAGE_FULL transition.
	hostHighDisk := everlightHostReportJSON(95.0)
	tx := submitEpochReport(t, cli, n0.nodeName, epochID1, hostHighDisk, nil)
	t.Logf("submit tx: %s", tx)
	RequireTxSuccess(t, tx)
	sut.AwaitNextBlock(t)
	report := auditQueryReport(t, epochID1, n0.accAddr)
	t.Logf("host-report epoch=%d disk_usage=%f", epochID1, report.HostReport.DiskUsagePercent)
	require.Greater(t, report.HostReport.DiskUsagePercent, 90.0)
	afterSN := cli.CustomQuery("q", "supernode", "get-supernode", n0.valAddr)
	t.Logf("supernode after report: %s", afterSN)
	require.Equal(t, "SUPERNODE_STATE_STORAGE_FULL", querySupernodeLatestState(t, cli, n0.valAddr))

	// Submit again next epoch and verify node remains in STORAGE_FULL while reporting.
	epochID2 := epochID1 + 1
	epoch2Start := epoch1Start + int64(epochLengthBlocks)
	awaitAtLeastHeight(t, epoch2Start)
	RequireTxSuccess(t, submitEpochReport(t, cli, n0.nodeName, epochID2, hostHighDisk, nil))
	sut.AwaitNextBlock(t)
	require.Equal(t, "SUPERNODE_STATE_STORAGE_FULL", querySupernodeLatestState(t, cli, n0.valAddr))
}

func TestEverlightSystem_PayoutAndHistoryWhileStorageFull(t *testing.T) {
	const (
		epochLengthBlocks = uint64(10)
	)

	sut.ModifyGenesisJSON(t,
		SetGovVotingPeriod(t, 10*time.Second),
		setSupernodeParamsForAuditTests(t),
		setAuditParamsForFastEpochs(t, epochLengthBlocks, 1, 1, 1, []uint32{4444}),
		func(genesis []byte) []byte {
			state, err := sjson.SetRawBytes(genesis, "app_state.audit.params.consecutive_epochs_to_postpone", []byte("100"))
			require.NoError(t, err)
			return state
		},
	)
	sut.StartChain(t)

	cli := NewLumeradCLI(t, sut, true)
	n0 := getNodeIdentity(t, cli, "node0")
	n1 := getNodeIdentity(t, cli, "node1")
	registerSupernode(t, cli, n0, "192.168.1.1")

	govAcctResp := cli.CustomQuery("q", "auth", "module-account", "gov")
	govAddr := gjson.Get(govAcctResp, "account.value.address").String()
	if govAddr == "" {
		govAddr = gjson.Get(govAcctResp, "account.base_account.address").String()
	}
	require.NotEmpty(t, govAddr)

	proposal := fmt.Sprintf(`{
		"messages": [{
			"@type": "/lumera.supernode.v1.MsgUpdateParams",
			"authority": %q,
			"params": {
				"reward_distribution": {
					"payment_period_blocks": "5",
					"registration_fee_share_bps": "1000",
					"min_cascade_bytes_for_payment": "1",
					"new_sn_ramp_up_periods": "1",
					"measurement_smoothing_periods": "1",
					"usage_growth_cap_bps_per_period": "10000"
				}
			}
		}],
		"deposit": "100000000%s",
		"metadata": "ipfs://CID",
		"title": "Set everlight fast payout params",
		"summary": "Set payment_period_blocks for system tests"
	}`, govAddr, lcfg.ChainDenom)

	proposalID := cli.SubmitAndVoteGovProposal(proposal)
	require.NotEmpty(t, proposalID)
	deadline := time.Now().Add(35 * time.Second)
	for time.Now().Before(deadline) {
		sut.AwaitNextBlock(t)
		status := cli.CustomQuery("q", "gov", "proposal", proposalID)
		if gjson.Get(status, "proposal.status").String() == "PROPOSAL_STATUS_PASSED" {
			break
		}
	}

	_ = sut.AwaitNextBlock(t)
	currentEpoch := cli.CustomQuery("q", "audit", "current-epoch")
	epochID1 := uint64(gjson.Get(currentEpoch, "epoch_id").Uint())
	require.Greater(t, epochID1, uint64(0))

	hostReport := `{"cpu_usage_percent":10,"disk_usage_percent":95,"failed_actions_count":0,"inbound_port_states":[],"mem_usage_percent":10,"cascade_kademlia_db_bytes":2000000}`
	submitted := false
	for try := 0; try < 3; try++ {
		tx := submitEpochReport(t, cli, n0.nodeName, epochID1+uint64(try), hostReport, nil)
		if gjson.Get(tx, "code").Int() == 0 {
			submitted = true
			epochID1 = epochID1 + uint64(try)
			break
		}
		sut.AwaitNextBlock(t)
	}
	require.True(t, submitted, "failed to submit epoch report in current/next epochs")
	sut.AwaitNextBlock(t)
	require.Equal(t, "SUPERNODE_STATE_STORAGE_FULL", querySupernodeLatestState(t, cli, n0.valAddr))
	report := auditQueryReport(t, epochID1, n0.accAddr)
	require.Greater(t, report.HostReport.CascadeKademliaDbBytes, 1.0)
	eligBefore := cli.CustomQuery("q", "supernode", "sn-eligibility", n0.valAddr)
	t.Logf("eligibility after report: %s", eligBefore)

	snMod := cli.CustomQuery("q", "auth", "module-account", "supernode")
	snAddr := gjson.Get(snMod, "account.value.address").String()
	if snAddr == "" {
		snAddr = gjson.Get(snMod, "account.base_account.address").String()
	}
	require.NotEmpty(t, snAddr)
	before := gjson.Get(cli.CustomQuery("q", "bank", "balance", n0.accAddr, "ulume"), "balance.amount").Int()
	RequireTxSuccess(t, cli.CustomCommand("tx", "bank", "send", n1.accAddr, snAddr, "1000000ulume", "--from", n1.nodeName))

	awaitAtLeastHeight(t, int64(30))
	sut.AwaitNextBlock(t)
	eligAtPay := cli.CustomQuery("q", "supernode", "sn-eligibility", n0.valAddr)
	t.Logf("eligibility at payout: %s", eligAtPay)
	after := gjson.Get(cli.CustomQuery("q", "bank", "balance", n0.accAddr, "ulume"), "balance.amount").Int()
	require.Greater(t, after, before, "expected payout to storage_full supernode")

	history := cli.CustomQuery("q", "supernode", "payout-history", n0.valAddr)
	require.GreaterOrEqual(t, len(gjson.Get(history, "entries").Array()), 1)

	elig := cli.CustomQuery("q", "supernode", "sn-eligibility", n0.valAddr)
	require.True(t, gjson.Get(elig, "eligible").Bool())
}
