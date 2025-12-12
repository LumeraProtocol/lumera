//go:build system_test

package system

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LumeraProtocol/supernode/v2/pkg/keyring"
	"github.com/LumeraProtocol/supernode/v2/pkg/lumera"
	"github.com/LumeraProtocol/supernode/v2/supernode/config"
	sdktx "github.com/cosmos/cosmos-sdk/types/tx"
	gogoproto "github.com/cosmos/gogoproto/proto"
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
				txResp, err := fx.client1.SuperNodeMsg().ReportMetrics(fx.ctx, fx.sn1.account, metricsSN1)
				require.NoError(t, err, "failed to broadcast metrics tx for SN1")
				require.NotNil(t, txResp)
				require.NotNil(t, txResp.TxResponse)
				require.Zero(t, txResp.TxResponse.Code, "metrics tx failed: %v", txResp.TxResponse.RawLog)
				t.Logf("SN1 metrics tx hash: %s", txResp.TxResponse.TxHash)

				// SN3 compliant metrics (with its own ports).
				metricsSN3 := fx.baseMetrics
				metricsSN3.OpenPorts = []uint32{4444, 4445, 4448, 4449, 8002}
				txResp3, err := fx.client3.SuperNodeMsg().ReportMetrics(fx.ctx, fx.sn3.account, metricsSN3)
				require.NoError(t, err, "failed to broadcast metrics tx for SN3")
				require.NotNil(t, txResp3)
				require.NotNil(t, txResp3.TxResponse)
				require.Zero(t, txResp3.TxResponse.Code, "metrics tx failed: %v", txResp3.TxResponse.RawLog)

				// SN2 non-compliant: bad ports and insufficient CPU cores -> POSTPONED.
				metricsSN2Bad := fx.baseMetrics
				metricsSN2Bad.OpenPorts = []uint32{1, 2, 3}
				metricsSN2Bad.CpuCoresTotal = 0
				txResp2, err := fx.client2.SuperNodeMsg().ReportMetrics(fx.ctx, fx.sn2.account, metricsSN2Bad)
				require.NoError(t, err, "failed to broadcast metrics tx for SN2")
				require.NotNil(t, txResp2)
				require.NotNil(t, txResp2.TxResponse)
				require.Zero(t, txResp2.TxResponse.Code, "metrics tx failed: %v", txResp2.TxResponse.RawLog)

				sut.AwaitNextBlock(t)
				return txResp.TxResponse.TxHash
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
				metricsSN3.OpenPorts = []uint32{4444, 4445, 4448, 4449, 8002}
				txResp1, err := fx.client1.SuperNodeMsg().ReportMetrics(fx.ctx, fx.sn1.account, metricsSN1)
				require.NoError(t, err)
				_, err = fx.client3.SuperNodeMsg().ReportMetrics(fx.ctx, fx.sn3.account, metricsSN3)
				require.NoError(t, err)

				// SN2 sends bad metrics twice and should remain POSTPONED.
				metricsBad := fx.baseMetrics
				metricsBad.OpenPorts = []uint32{1, 2, 3}
				metricsBad.CpuCoresTotal = 0
				txBad1, err := fx.client2.SuperNodeMsg().ReportMetrics(fx.ctx, fx.sn2.account, metricsBad)
				require.NoError(t, err)
				fx.waitForTx(txBad1.TxResponse.TxHash)
				sut.AwaitNextBlock(t)
				txBad2, err := fx.client2.SuperNodeMsg().ReportMetrics(fx.ctx, fx.sn2.account, metricsBad)
				require.NoError(t, err)
				fx.waitForTx(txBad2.TxResponse.TxHash)
				sut.AwaitNextBlock(t)
				return txResp1.TxResponse.TxHash
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
				recovered.OpenPorts = []uint32{4444, 4445, 4446, 4447, 8002}
				recovered.CpuCoresTotal = 4
				txResp2Recover, err := fx.client2.SuperNodeMsg().ReportMetrics(fx.ctx, fx.sn2.account, recovered)
				require.NoError(t, err, "failed to broadcast recovery metrics tx for SN2")
				require.NotNil(t, txResp2Recover)
				require.NotNil(t, txResp2Recover.TxResponse)
				require.Zero(t, txResp2Recover.TxResponse.Code, "recovery metrics tx failed: %v", txResp2Recover.TxResponse.RawLog)
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
				// Query SN1's transaction once to verify payload and events.
				queriedTx := fx.waitForTx(txHashSN1)
				require.NotNil(t, queriedTx.Tx)
				t.Logf("Queried SN1 metrics tx log: %s", queriedTx.TxResponse.RawLog)
				for _, ev := range queriedTx.TxResponse.Events {
					if ev.Type == "supernode_metrics_reported" || ev.Type == "supernode_postponed" {
						t.Logf("SN1 event %s attrs: %+v", ev.Type, ev.Attributes)
					}
				}

				msgs := queriedTx.Tx.GetBody().GetMessages()
				require.NotEmpty(t, msgs, "metrics tx contained no messages")

				var metricsMsg sntypes.MsgReportSupernodeMetrics
				require.NoError(t, gogoproto.Unmarshal(msgs[0].GetValue(), &metricsMsg))
				require.Equal(t, "/lumera.supernode.v1.MsgReportSupernodeMetrics", msgs[0].GetTypeUrl())
				require.Equal(t, fx.sn1.account, metricsMsg.SupernodeAccount)
				require.Equal(t, fx.sn1.valAddr, metricsMsg.ValidatorAddress)
				require.True(t, metricsMsg.Metrics.VersionMajor > 0, "version major should be set")
				require.True(t, metricsMsg.Metrics.UptimeSeconds >= 0, "uptime should be present")
				require.ElementsMatch(t, []uint32{4444, 4445, 8002}, metricsMsg.Metrics.OpenPorts, "open ports should include required ports")
			}

			for acc, expectedState := range expected {
				fx.assertState(acc, expectedState, fx.validatorsByAccount[acc])
			}
		})
	}
}

type metricsFixture struct {
	ctx                 context.Context
	cli                 *LumeradCli
	queryClient         lumera.Client
	client1             lumera.Client
	client2             lumera.Client
	client3             lumera.Client
	sn1                 registeredSN
	sn2                 registeredSN
	sn3                 registeredSN
	baseMetrics         sntypes.SupernodeMetrics
	validatorsByAccount map[string]string
	waitForTx           func(hash string) *sdktx.GetTxResponse
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

		t.Logf("Registering supernode for %s (validator: %s, account: %s)", nodeKey, valAddr, accountAddr)

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

	fx := &metricsFixture{
		ctx: context.Background(),
		cli: cli,
	}

	fx.sn1 = registerSupernode("node0", "4444", "4445")
	fx.sn2 = registerSupernode("node1", "4446", "4447")
	fx.sn3 = registerSupernode("node2", "4448", "4449")

	cli.FundAddress(fx.sn1.account, "100000ulume")
	cli.FundAddressWithNode(fx.sn2.account, "100000ulume", "node1")
	cli.FundAddressWithNode(fx.sn3.account, "100000ulume", "node2")
	sut.AwaitNextBlock(t)

	clientFor := func(keyName string, home string) lumera.Client {
		kr, err := keyring.InitKeyring(config.KeyringConfig{
			Backend: "test",
			Dir:     home,
		})
		require.NoError(t, err)

		cfg, cfgErr := lumera.NewConfig("localhost:9090", sut.chainID, keyName, kr)
		require.NoError(t, cfgErr)

		client, cliErr := lumera.NewClient(fx.ctx, cfg)
		require.NoError(t, cliErr)
		return client
	}

	fx.queryClient = clientFor("node0", filepath.Join(WorkDir, sut.nodePath(0)))
	fx.client1 = clientFor("node0", filepath.Join(WorkDir, sut.nodePath(0)))
	fx.client2 = clientFor("node1", filepath.Join(WorkDir, sut.nodePath(1)))
	fx.client3 = clientFor("node2", filepath.Join(WorkDir, sut.nodePath(2)))

	t.Cleanup(func() {
		fx.client1.Close()
		fx.client2.Close()
		fx.client3.Close()
		fx.queryClient.Close()
	})

	paramsResp, err := fx.queryClient.SuperNode().GetParams(fx.ctx)
	require.NoError(t, err)
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
		OpenPorts:        []uint32{4444, 4445, 8002},
	}

	fx.validatorsByAccount = map[string]string{
		fx.sn1.account: fx.sn1.valAddr,
		fx.sn2.account: fx.sn2.valAddr,
		fx.sn3.account: fx.sn3.valAddr,
	}

	fx.waitForTx = func(hash string) *sdktx.GetTxResponse {
		var (
			resp *sdktx.GetTxResponse
			err  error
		)
		for i := 0; i < 10; i++ {
			resp, err = fx.queryClient.Tx().GetTransaction(fx.ctx, hash)
			if err == nil && resp != nil {
				return resp
			}
			time.Sleep(300 * time.Millisecond)
		}
		require.NoError(t, err, "failed to fetch metrics tx by hash")
		require.NotNil(t, resp)
		return resp
	}

	fx.assertState = func(accountAddr string, expected sntypes.SuperNodeState, expectedVal string) {
		sn, err := fx.queryClient.SuperNode().GetSupernodeBySupernodeAddress(fx.ctx, accountAddr)
		require.NoError(t, err)
		require.NotNil(t, sn)
		require.NotEmpty(t, sn.States)
		latest := sn.States[len(sn.States)-1]
		require.NotNil(t, latest)
		if latest.State != expected {
			t.Logf("states for %s: %+v", accountAddr, sn.States)
		}
		require.Equal(t, expected, latest.State, "unexpected state for %s", accountAddr)
		require.Equal(t, expectedVal, sn.ValidatorAddress)
	}

	return fx
}

// SetSupernodeMetricsParams configures supernode metrics-related params for faster testing.
func SetSupernodeMetricsParams(t *testing.T) GenesisMutator {
	return func(genesis []byte) []byte {
		t.Helper()

		// Align minimum stake with the default testnet validators' stake so that
		// they are eligible to register as supernodes.
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
