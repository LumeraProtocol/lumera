//go:build system_test

package system

import (
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/sjson"

	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

// TestSupernodeMetricsStalenessAndRecovery exercises metrics staleness and recovery flows with short intervals.
func TestSupernodeMetricsStalenessAndRecovery(t *testing.T) {
	os.Setenv("INTEGRATION_TEST", "true")
	defer os.Unsetenv("INTEGRATION_TEST")

	sut.ModifyGenesisJSON(t, SetStakingBondDenomUlume(t), SetSupernodeStalenessParams(t))
	sut.StartChain(t)
	defer sut.StopChain()

	cli := NewLumeradCLI(t, sut, true)

	type registeredSN struct {
		account string
		valAddr string
		port    string
		p2pPort string
	}

	registerSupernode := func(nodeKey string, port string, p2pPort string) registeredSN {
		accountAddr := cli.GetKeyAddr(nodeKey)
		valAddrOutput := cli.Keys("keys", "show", nodeKey, "--bech", "val", "-a")
		valAddr := strings.TrimSpace(valAddrOutput)

		registerCmd := []string{
			"tx", "supernode", "register-supernode",
			valAddr,
			"localhost:" + port,
			accountAddr,
			"--p2p-port", p2pPort,
			"--from", nodeKey,
		}

		resp := cli.CustomCommand(registerCmd...)
		RequireTxSuccess(t, resp)
		sut.AwaitNextBlock(t)

		return registeredSN{account: accountAddr, valAddr: valAddr, port: port, p2pPort: p2pPort}
	}

	sn1 := registerSupernode("node0", "4444", "4445")
	sn2 := registerSupernode("node1", "4446", "4447")

	cli.FundAddress(sn1.account, "100000ulume")
	cli.FundAddressWithNode(sn2.account, "100000ulume", "node1")
	sut.AwaitNextBlock(t)

	baseMetrics := sntypes.SupernodeMetrics{
		VersionMajor:     2,
		VersionMinor:     0,
		VersionPatch:     0,
		CpuCoresTotal:    4,
		CpuUsagePercent:  10,
		MemTotalGb:       8,
		MemUsagePercent:  50,
		MemFreeGb:        4,
		DiskTotalGb:      50,
		DiskUsagePercent: 20,
		DiskFreeGb:       40,
		UptimeSeconds:    120,
		PeersCount:       1,
		OpenPorts: []sntypes.PortStatus{
			{Port: 4444, State: sntypes.PortState_PORT_STATE_OPEN},
			{Port: 4445, State: sntypes.PortState_PORT_STATE_OPEN},
			{Port: 8002, State: sntypes.PortState_PORT_STATE_OPEN},
		},
	}

	getState := func(accountAddr string) sntypes.SuperNodeState {
		sn := querySupernodeByAddress(t, cli, accountAddr)
		require.NotNil(t, sn)
		require.NotNil(t, sn.Supernode)
		require.NotEmpty(t, sn.Supernode.States)
		return sn.Supernode.States[len(sn.Supernode.States)-1].State
	}

	waitForState := func(accountAddr string, expected sntypes.SuperNodeState, maxBlocks int) {
		for i := 0; i < maxBlocks; i++ {
			if state := getState(accountAddr); state == expected {
				return
			}
			sut.AwaitNextBlock(t)
		}
		require.Equal(t, expected, getState(accountAddr), "state did not converge within %d blocks", maxBlocks)
	}

	// SN1 reports compliant metrics once.
	txHash := reportSupernodeMetrics(t, cli, "node0", sn1.valAddr, sn1.account, baseMetrics)
	txResp := decodeTxResponse(t, waitForTx(t, cli, txHash))
	require.Zero(t, txResp.Code, "metrics tx failed: %v", txResp.RawLog)
	metricsHeight := sut.AwaitNextBlock(t)

	// SN1 should become ACTIVE after reporting; SN2 eventually gets POSTPONED due to missing metrics.
	waitForState(sn1.account, sntypes.SuperNodeStateActive, 3)
	waitForState(sn2.account, sntypes.SuperNodeStatePostponed, 8)

	// Advance well past update+grace intervals to make SN1 stale.
	targetHeight := metricsHeight + 12
	currentHeight := atomic.LoadInt64(&sut.currentHeight)
	if targetHeight <= currentHeight {
		targetHeight = currentHeight + 1
	}
	sut.AwaitBlockHeight(t, targetHeight)
	waitForState(sn1.account, sntypes.SuperNodeStatePostponed, 3)

	// SN1 recovers with fresh metrics; SN2 stays POSTPONED because it never reports.
	txHashFresh := reportSupernodeMetrics(t, cli, "node0", sn1.valAddr, sn1.account, baseMetrics)
	txRespFresh := decodeTxResponse(t, waitForTx(t, cli, txHashFresh))
	require.Zero(t, txRespFresh.Code, "recovery metrics tx failed: %v", txRespFresh.RawLog)
	sut.AwaitNextBlock(t)

	waitForState(sn1.account, sntypes.SuperNodeStateActive, 4)
	require.Equal(t, sntypes.SuperNodeStatePostponed, getState(sn2.account))
}

// TestSupernodeMetricsNoReportsAllPostponed verifies that without any reports all supernodes end up postponed.
func TestSupernodeMetricsNoReportsAllPostponed(t *testing.T) {
	os.Setenv("INTEGRATION_TEST", "true")
	defer os.Unsetenv("INTEGRATION_TEST")

	sut.ModifyGenesisJSON(t, SetStakingBondDenomUlume(t), SetSupernodeStalenessParams(t))
	sut.StartChain(t)
	defer sut.StopChain()

	cli := NewLumeradCLI(t, sut, true)

	register := func(nodeKey, port, p2pPort string) registeredSN {
		accountAddr := cli.GetKeyAddr(nodeKey)
		valAddrOutput := cli.Keys("keys", "show", nodeKey, "--bech", "val", "-a")
		valAddr := strings.TrimSpace(valAddrOutput)

		registerCmd := []string{
			"tx", "supernode", "register-supernode",
			valAddr,
			"localhost:" + port,
			accountAddr,
			"--p2p-port", p2pPort,
			"--from", nodeKey,
		}
		resp := cli.CustomCommand(registerCmd...)
		RequireTxSuccess(t, resp)
		sut.AwaitNextBlock(t)
		return registeredSN{account: accountAddr, valAddr: valAddr, port: port, p2pPort: p2pPort}
	}

	sn1 := register("node0", "4444", "4445")
	sn2 := register("node1", "4446", "4447")

	cli.FundAddress(sn1.account, "100000ulume")
	cli.FundAddressWithNode(sn2.account, "100000ulume", "node1")
	sut.AwaitNextBlock(t)

	getState := func(accountAddr string) sntypes.SuperNodeState {
		sn := querySupernodeByAddress(t, cli, accountAddr)
		require.NotNil(t, sn)
		require.NotNil(t, sn.Supernode)
		require.NotEmpty(t, sn.Supernode.States)
		return sn.Supernode.States[len(sn.Supernode.States)-1].State
	}

	waitForState := func(accountAddr string, expected sntypes.SuperNodeState, maxBlocks int) {
		for i := 0; i < maxBlocks; i++ {
			if state := getState(accountAddr); state == expected {
				return
			}
			sut.AwaitNextBlock(t)
		}
		require.Equal(t, expected, getState(accountAddr), "state did not converge within %d blocks", maxBlocks)
	}

	// Do not send any metrics. Wait past update+grace to ensure both get postponed.
	targetHeight := sut.AwaitNextBlock(t) + 12
	sut.AwaitBlockHeight(t, targetHeight)

	waitForState(sn1.account, sntypes.SuperNodeStatePostponed, 4)
	waitForState(sn2.account, sntypes.SuperNodeStatePostponed, 4)
}

// SetSupernodeStalenessParams configures short metrics intervals to test staleness.
func SetSupernodeStalenessParams(t *testing.T) GenesisMutator {
	return func(genesis []byte) []byte {
		t.Helper()

		state, err := sjson.SetRawBytes(genesis, "app_state.supernode.params.minimum_stake_for_sn", []byte(`{"denom":"ulume","amount":"100000000"}`))
		require.NoError(t, err)

		state, err = sjson.SetRawBytes(state, "app_state.supernode.params.reporting_threshold", []byte(`"1"`))
		require.NoError(t, err)

		state, err = sjson.SetRawBytes(state, "app_state.supernode.params.slashing_threshold", []byte(`"1"`))
		require.NoError(t, err)

		state, err = sjson.SetRawBytes(state, "app_state.supernode.params.metrics_thresholds", []byte(`""`))
		require.NoError(t, err)

		state, err = sjson.SetRawBytes(state, "app_state.supernode.params.evidence_retention_period", []byte(`"180days"`))
		require.NoError(t, err)

		state, err = sjson.SetRawBytes(state, "app_state.supernode.params.slashing_fraction", []byte(`"0.010000000000000000"`))
		require.NoError(t, err)

		state, err = sjson.SetRawBytes(state, "app_state.supernode.params.inactivity_penalty_period", []byte(`"86400s"`))
		require.NoError(t, err)

		state, err = sjson.SetRawBytes(state, "app_state.supernode.params.metrics_update_interval_blocks", []byte(`"5"`))
		require.NoError(t, err)

		state, err = sjson.SetRawBytes(state, "app_state.supernode.params.metrics_grace_period_blocks", []byte(`"3"`))
		require.NoError(t, err)

		state, err = sjson.SetRawBytes(state, "app_state.supernode.params.metrics_freshness_max_blocks", []byte(`"20"`))
		require.NoError(t, err)

		state, err = sjson.SetRawBytes(state, "app_state.supernode.params.min_supernode_version", []byte(`"0.0.0"`))
		require.NoError(t, err)

		state, err = sjson.SetRawBytes(state, "app_state.supernode.params.min_cpu_cores", []byte(`1`))
		require.NoError(t, err)

		state, err = sjson.SetRawBytes(state, "app_state.supernode.params.max_cpu_usage_percent", []byte(`100`))
		require.NoError(t, err)

		state, err = sjson.SetRawBytes(state, "app_state.supernode.params.min_mem_gb", []byte(`1`))
		require.NoError(t, err)

		state, err = sjson.SetRawBytes(state, "app_state.supernode.params.max_mem_usage_percent", []byte(`100`))
		require.NoError(t, err)

		state, err = sjson.SetRawBytes(state, "app_state.supernode.params.min_storage_gb", []byte(`1`))
		require.NoError(t, err)

		state, err = sjson.SetRawBytes(state, "app_state.supernode.params.max_storage_usage_percent", []byte(`100`))
		require.NoError(t, err)

		state, err = sjson.SetRawBytes(state, "app_state.supernode.params.required_open_ports", []byte(`[]`))
		require.NoError(t, err)

		return state
	}
}
