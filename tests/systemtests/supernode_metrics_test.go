//go:build system_test && supernode_metrics_tests

package system

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/sjson"

	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

// TestSupernodeMetricsE2E validates supernode metrics reporting end-to-end without creating actions.
func TestSupernodeMetricsE2E(t *testing.T) {
	type scenario struct {
		name       string
		run        func(t *testing.T, fx *metricsFixture) (txHashSN1 string)
		expected   func(fx *metricsFixture) map[string]sntypes.SuperNodeState
		checkTxSN1 bool
	}

	scenarios := []scenario{
		{
			name: "initial_metrics_and_postponed_sn2",
			run: func(t *testing.T, fx *metricsFixture) string {
				// SN1 compliant metrics.
				metricsSN1 := fx.baseMetrics
				txHash := reportSupernodeMetrics(t, fx.cli, "node0", fx.sn1.valAddr, fx.sn1.account, metricsSN1)
				txResp := fx.waitForTx(txHash)
				require.Zero(t, txResp.Code, "metrics tx failed: %v", txResp.RawLog)
				t.Logf("SN1 metrics tx hash: %s", txHash)

				// SN3 compliant metrics (with its own ports).
				metricsSN3 := fx.baseMetrics
				metricsSN3.OpenPorts = []sntypes.PortStatus{
					{Port: 4444, State: sntypes.PortState_PORT_STATE_OPEN},
					{Port: 4445, State: sntypes.PortState_PORT_STATE_OPEN},
					{Port: 4448, State: sntypes.PortState_PORT_STATE_OPEN},
					{Port: 4449, State: sntypes.PortState_PORT_STATE_OPEN},
					{Port: 8002, State: sntypes.PortState_PORT_STATE_OPEN},
				}
				txHash3 := reportSupernodeMetrics(t, fx.cli, "node2", fx.sn3.valAddr, fx.sn3.account, metricsSN3)
				txResp3 := fx.waitForTx(txHash3)
				require.Zero(t, txResp3.Code, "metrics tx failed: %v", txResp3.RawLog)

				// SN2 non-compliant: bad ports and insufficient CPU cores -> POSTPONED.
				metricsSN2Bad := fx.baseMetrics
				metricsSN2Bad.OpenPorts = []sntypes.PortStatus{
					{Port: 4444, State: sntypes.PortState_PORT_STATE_CLOSED},
				}
				metricsSN2Bad.CpuCoresTotal = 0
				txHash2 := reportSupernodeMetrics(t, fx.cli, "node1", fx.sn2.valAddr, fx.sn2.account, metricsSN2Bad)
				txResp2 := fx.waitForTx(txHash2)
				require.Zero(t, txResp2.Code, "metrics tx failed: %v", txResp2.RawLog)

				sut.AwaitNextBlock(t)
				return txHash
			},
			expected: func(fx *metricsFixture) map[string]sntypes.SuperNodeState {
				return map[string]sntypes.SuperNodeState{
					fx.sn1.account: sntypes.SuperNodeStateActive,
					fx.sn2.account: sntypes.SuperNodeStatePostponed,
					fx.sn3.account: sntypes.SuperNodeStateActive,
				}
			},
			checkTxSN1: true,
		},
		{
			name: "sn2_keeps_sending_bad_metrics_stays_postponed",
			run: func(t *testing.T, fx *metricsFixture) string {
				// SN1 and SN3 compliant first.
				metricsSN1 := fx.baseMetrics
				metricsSN3 := fx.baseMetrics
				metricsSN3.OpenPorts = []sntypes.PortStatus{
					{Port: 4444, State: sntypes.PortState_PORT_STATE_OPEN},
					{Port: 4445, State: sntypes.PortState_PORT_STATE_OPEN},
					{Port: 4448, State: sntypes.PortState_PORT_STATE_OPEN},
					{Port: 4449, State: sntypes.PortState_PORT_STATE_OPEN},
					{Port: 8002, State: sntypes.PortState_PORT_STATE_OPEN},
				}
				txHash1 := reportSupernodeMetrics(t, fx.cli, "node0", fx.sn1.valAddr, fx.sn1.account, metricsSN1)
				reportSupernodeMetrics(t, fx.cli, "node2", fx.sn3.valAddr, fx.sn3.account, metricsSN3)

				// SN2 sends bad metrics twice and should remain POSTPONED.
				metricsBad := fx.baseMetrics
				metricsBad.OpenPorts = []sntypes.PortStatus{
					{Port: 4444, State: sntypes.PortState_PORT_STATE_CLOSED},
				}
				metricsBad.CpuCoresTotal = 0
				txBad1 := reportSupernodeMetrics(t, fx.cli, "node1", fx.sn2.valAddr, fx.sn2.account, metricsBad)
				fx.waitForTx(txBad1)
				sut.AwaitNextBlock(t)
				txBad2 := reportSupernodeMetrics(t, fx.cli, "node1", fx.sn2.valAddr, fx.sn2.account, metricsBad)
				fx.waitForTx(txBad2)
				sut.AwaitNextBlock(t)
				return txHash1
			},
			expected: func(fx *metricsFixture) map[string]sntypes.SuperNodeState {
				return map[string]sntypes.SuperNodeState{
					fx.sn1.account: sntypes.SuperNodeStateActive,
					fx.sn2.account: sntypes.SuperNodeStatePostponed,
					fx.sn3.account: sntypes.SuperNodeStateActive,
				}
			},
		},
		{
			name: "sn2_recovers_with_good_metrics",
			run: func(t *testing.T, fx *metricsFixture) string {
				// SN2 now reports compliant metrics and should return to ACTIVE.
				recovered := fx.baseMetrics
				recovered.OpenPorts = []sntypes.PortStatus{
					{Port: 4444, State: sntypes.PortState_PORT_STATE_OPEN},
					{Port: 4445, State: sntypes.PortState_PORT_STATE_OPEN},
					{Port: 4446, State: sntypes.PortState_PORT_STATE_OPEN},
					{Port: 4447, State: sntypes.PortState_PORT_STATE_OPEN},
					{Port: 8002, State: sntypes.PortState_PORT_STATE_OPEN},
				}
				recovered.CpuCoresTotal = 4
				txHash := reportSupernodeMetrics(t, fx.cli, "node1", fx.sn2.valAddr, fx.sn2.account, recovered)
				txResp := fx.waitForTx(txHash)
				require.Zero(t, txResp.Code, "recovery metrics tx failed: %v", txResp.RawLog)
				sut.AwaitNextBlock(t)
				return ""
			},
			expected: func(fx *metricsFixture) map[string]sntypes.SuperNodeState {
				return map[string]sntypes.SuperNodeState{
					fx.sn1.account: sntypes.SuperNodeStateActive,
					fx.sn2.account: sntypes.SuperNodeStateActive,
					fx.sn3.account: sntypes.SuperNodeStateActive,
				}
			},
		},
	}

	for _, sc := range scenarios {
		sc := sc
		t.Run(sc.name, func(t *testing.T) {
			fx := newMetricsFixture(t)
			expected := sc.expected(fx)

			txHashSN1 := sc.run(t, fx)

			if sc.checkTxSN1 && txHashSN1 != "" {
				txJSON := waitForTx(t, fx.cli, txHashSN1)
				txResp := decodeTxResponse(t, txJSON)
				require.Zero(t, txResp.Code, "metrics tx failed: %v", txResp.RawLog)
				t.Logf("Queried SN1 metrics tx log: %s", txResp.RawLog)
				msg := decodeMetricsMsg(t, txJSON)
				require.Equal(t, fx.sn1.account, msg.SupernodeAccount)
				require.Equal(t, fx.sn1.valAddr, msg.ValidatorAddress)
				require.True(t, msg.Metrics.VersionMajor > 0, "version major should be set")
				require.True(t, msg.Metrics.UptimeSeconds >= 0, "uptime should be present")
				require.NotEmpty(t, msg.Metrics.OpenPorts)
			}

			for acc, expectedState := range expected {
				fx.assertState(acc, expectedState, fx.validatorsByAccount[acc])
			}
		})
	}
}

type metricsFixture struct {
	cli                 *LumeradCli
	sn1                 registeredSN
	sn2                 registeredSN
	sn3                 registeredSN
	baseMetrics         sntypes.SupernodeMetrics
	validatorsByAccount map[string]string
	waitForTx           func(hash string) *txResponse
	assertState         func(accountAddr string, expected sntypes.SuperNodeState, expectedVal string)
}

type registeredSN struct {
	account string
	valAddr string
	port    string
	p2pPort string
}

func newMetricsFixture(t *testing.T) *metricsFixture {
	t.Helper()

	os.Setenv("INTEGRATION_TEST", "true")
	t.Cleanup(func() { os.Unsetenv("INTEGRATION_TEST") })

	sut.ModifyGenesisJSON(t, SetStakingBondDenomUlume(t), SetSupernodeMetricsParams(t))
	sut.StartChain(t)
	t.Cleanup(func() { sut.StopChain() })

	cli := NewLumeradCLI(t, sut, true)

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

	fx := &metricsFixture{cli: cli}

	fx.sn1 = registerSupernode("node0", "4444", "4445")
	fx.sn2 = registerSupernode("node1", "4446", "4447")
	fx.sn3 = registerSupernode("node2", "4448", "4449")

	cli.FundAddress(fx.sn1.account, "100000ulume")
	cli.FundAddressWithNode(fx.sn2.account, "100000ulume", "node1")
	cli.FundAddressWithNode(fx.sn3.account, "100000ulume", "node2")
	sut.AwaitNextBlock(t)

	paramsResp := querySupernodeParams(t, cli)
	t.Logf("Params required_open_ports: %v, min_cpu_cores: %d", paramsResp.Params.RequiredOpenPorts, paramsResp.Params.MinCpuCores)

	fx.baseMetrics = sntypes.SupernodeMetrics{
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

	fx.validatorsByAccount = map[string]string{
		fx.sn1.account: fx.sn1.valAddr,
		fx.sn2.account: fx.sn2.valAddr,
		fx.sn3.account: fx.sn3.valAddr,
	}

	fx.waitForTx = func(hash string) *txResponse {
		return decodeTxResponse(t, waitForTx(t, cli, hash))
	}

	fx.assertState = func(accountAddr string, expected sntypes.SuperNodeState, expectedVal string) {
		snResp := querySupernodeByAddress(t, cli, accountAddr)
		require.NotNil(t, snResp)
		require.NotNil(t, snResp.Supernode)
		require.NotEmpty(t, snResp.Supernode.States)
		latest := snResp.Supernode.States[len(snResp.Supernode.States)-1]
		if latest.State != expected {
			t.Logf("states for %s: %+v", accountAddr, snResp.Supernode.States)
		}
		require.Equal(t, expected, latest.State, "unexpected state for %s", accountAddr)
		require.Equal(t, expectedVal, snResp.Supernode.ValidatorAddress)
	}

	return fx
}

// SetSupernodeMetricsParams configures supernode metrics-related params for faster testing.
func SetSupernodeMetricsParams(t *testing.T) GenesisMutator {
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

		// Allow plenty of time for supernodes to start and report metrics in tests.
		state, err = sjson.SetRawBytes(state, "app_state.supernode.params.metrics_update_interval_blocks", []byte(`"120"`))
		require.NoError(t, err)

		state, err = sjson.SetRawBytes(state, "app_state.supernode.params.metrics_grace_period_blocks", []byte(`"120"`))
		require.NoError(t, err)

		state, err = sjson.SetRawBytes(state, "app_state.supernode.params.metrics_freshness_max_blocks", []byte(`"500"`))
		require.NoError(t, err)

		// Permit any version in tests; binaries are often built with "dev".
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

		// Allow any open ports for multi-supernode tests.
		state, err = sjson.SetRawBytes(state, "app_state.supernode.params.required_open_ports", []byte(`[]`))
		require.NoError(t, err)

		return state
	}
}

// SetStakingBondDenomUlume sets the staking module bond denom to "ulume" in genesis.
func SetStakingBondDenomUlume(t *testing.T) GenesisMutator {
	return func(genesis []byte) []byte {
		t.Helper()
		state, err := sjson.SetBytes(genesis, "app_state.staking.params.bond_denom", "ulume")
		require.NoError(t, err)
		return state
	}
}
