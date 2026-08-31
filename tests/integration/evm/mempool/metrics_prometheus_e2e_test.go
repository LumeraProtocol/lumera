//go:build integration
// +build integration

package mempool_test

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	evmtest "github.com/LumeraProtocol/lumera/tests/integration/evmtest"
	testaccounts "github.com/LumeraProtocol/lumera/testutil/accounts"
	"github.com/stretchr/testify/require"
)

// TestPrometheusMetricsExposeMempoolGauges verifies that the Cosmos SDK
// /metrics?format=prometheus endpoint exposes the lumera_evm_mempool_* gauge
// metrics end-to-end: a real node reads live mempool state on each HTTP GET.
func TestPrometheusMetricsExposeMempoolGauges(t *testing.T) {
	node := evmtest.NewEVMNode(t, "lumera-mempool-prom", 600)
	node.AppendStartArgs("--json-rpc.api", "eth,txpool,net,web3")
	evmtest.EnablePrometheusMetrics(t, node.HomeDir(), node.APIListenAddress())
	node.StartAndWaitRPC()
	defer node.Stop()

	metricsURL := node.APIURL() + "/metrics?format=prometheus"

	// Wait for the API/telemetry server to be ready.
	waitForHTTP(t, metricsURL, 30*time.Second)

	// --- Baseline scrape: gauge metrics should exist ---
	// Note: rejections_total uses CounterVec and emits no series until the first
	// label combination is observed, so it is NOT checked at baseline.
	baseline := scrapeMetrics(t, metricsURL)
	requireMetricExists(t, baseline, "lumera_evm_mempool_size")
	requireMetricExists(t, baseline, "lumera_evm_mempool_pending")
	requireMetricExists(t, baseline, "lumera_evm_mempool_queued")
	requireMetricExists(t, baseline, "lumera_evm_mempool_broadcast_queue_depth")

	baselinePending := baseline["lumera_evm_mempool_pending"]
	baselineQueued := baseline["lumera_evm_mempool_queued"]
	t.Logf("baseline: pending=%.0f, queued=%.0f, size=%.0f, depth=%.0f",
		baselinePending, baselineQueued,
		baseline["lumera_evm_mempool_size"],
		baseline["lumera_evm_mempool_broadcast_queue_depth"])

	// --- Submit 3 sequential txs so pending increases ---
	fromAddr := testaccounts.MustAccountAddressFromTestKeyInfo(t, node.KeyInfo())
	privateKey := evmtest.MustDerivePrivateKey(t, node.KeyInfo().Mnemonic)
	gasPrice := node.MustGetGasPriceWithRetry(t, 20*time.Second)
	baseNonce := node.MustGetPendingNonceWithRetry(t, fromAddr.Hex(), 20*time.Second)
	toAddr := fromAddr

	sendTx := func(nonce uint64, price *big.Int) (string, error) {
		return evmtest.SendLegacyTxWithParamsResult(node.RPCURL(), evmtest.LegacyTxParams{
			PrivateKey: privateKey,
			Nonce:      nonce,
			To:         &toAddr,
			Value:      big.NewInt(1),
			Gas:        21_000,
			GasPrice:   price,
		})
	}

	for i := uint64(0); i < 3; i++ {
		_, err := sendTx(baseNonce+i, gasPrice)
		require.NoError(t, err, "tx nonce=%d should be accepted", baseNonce+i)
	}

	// --- Poll /metrics until pending increases ---
	require.Eventually(t, func() bool {
		m := scrapeMetrics(t, metricsURL)
		return m["lumera_evm_mempool_pending"] > baselinePending
	}, 15*time.Second, 500*time.Millisecond,
		"lumera_evm_mempool_pending must increase after submitting txs")

	afterPending := scrapeMetrics(t, metricsURL)
	t.Logf("after 3 txs: pending=%.0f, size=%.0f",
		afterPending["lumera_evm_mempool_pending"], afterPending["lumera_evm_mempool_size"])

	require.Greater(t, afterPending["lumera_evm_mempool_pending"], baselinePending,
		"pending gauge must increase after submitting sequential txs")
	require.GreaterOrEqual(t, afterPending["lumera_evm_mempool_size"], afterPending["lumera_evm_mempool_pending"],
		"size must be >= pending (size includes cosmos pool txs)")

	// --- Submit a nonce-gap tx so queued increases ---
	gapNonce := baseNonce + 100
	_, err := sendTx(gapNonce, gasPrice)
	require.NoError(t, err, "nonce-gap tx should be accepted into queued pool")

	require.Eventually(t, func() bool {
		m := scrapeMetrics(t, metricsURL)
		return m["lumera_evm_mempool_queued"] > baselineQueued
	}, 15*time.Second, 500*time.Millisecond,
		"lumera_evm_mempool_queued must increase after submitting nonce-gap tx")

	afterQueued := scrapeMetrics(t, metricsURL)
	t.Logf("after nonce-gap tx: queued=%.0f (was %.0f)",
		afterQueued["lumera_evm_mempool_queued"], baselineQueued)
	require.Greater(t, afterQueued["lumera_evm_mempool_queued"], baselineQueued,
		"queued gauge must increase after submitting nonce-gap tx")

	// Note: broadcast_queue_depth is validated for existence only because the
	// async broadcast worker drains the queue within milliseconds, making a
	// non-zero observation impractical in an integration test.
}

// TestPrometheusRejectionsCountedViaCometCheckTx verifies that the labeled
// rejection counter lumera_evm_mempool_rejections_total{source="checktx",reason="ante"}
// increases when malformed tx bytes are submitted via CometBFT broadcast_tx_sync.
// This exercises the exact path instrumented in the wrapped CheckTxHandler.
func TestPrometheusRejectionsCountedViaCometCheckTx(t *testing.T) {
	node := evmtest.NewEVMNode(t, "lumera-mempool-rej-prom", 600)
	node.AppendStartArgs("--json-rpc.api", "eth,txpool,net,web3")
	evmtest.EnablePrometheusMetrics(t, node.HomeDir(), node.APIListenAddress())
	node.StartAndWaitRPC()
	defer node.Stop()

	metricsURL := node.APIURL() + "/metrics?format=prometheus"

	waitForHTTP(t, metricsURL, 30*time.Second)

	// At baseline, the counter label pair should not exist yet (CounterVec
	// emits nothing until first observation).
	rejKey := `lumera_evm_mempool_rejections_total{reason="ante",source="checktx"}`
	baseline := scrapeMetrics(t, metricsURL)
	baselineRejections := baseline[rejKey] // 0 if absent

	// Submit malformed bytes directly through CometBFT broadcast_tx_sync.
	// This bypasses the EVM JSON-RPC layer and hits the app's CheckTx directly,
	// exercising the wrapped CheckTxHandler that increments the counter.
	garbageTx := base64.StdEncoding.EncodeToString([]byte("not-a-valid-tx"))
	cometRPCURL := node.CometRPCURL()
	cometHTTP := strings.Replace(cometRPCURL, "tcp://", "http://", 1)

	for i := 0; i < 3; i++ {
		resp := broadcastTxSyncComet(t, cometHTTP, garbageTx)
		// CometBFT returns the CheckTx result inline; code != 0 means rejected.
		t.Logf("broadcast_tx_sync #%d: %s", i, string(resp))
	}

	// Poll /metrics until the rejection counter appears and increases.
	require.Eventually(t, func() bool {
		m := scrapeMetrics(t, metricsURL)
		return m[rejKey] > baselineRejections
	}, 15*time.Second, 500*time.Millisecond,
		"lumera_evm_mempool_rejections_total{source=checktx,reason=ante} must increase after malformed tx submission")

	after := scrapeMetrics(t, metricsURL)
	t.Logf("after malformed txs: rejections{checktx,ante}=%.0f (was %.0f)",
		after[rejKey], baselineRejections)
	require.Greater(t, after[rejKey], baselineRejections,
		"rejection counter must increase after malformed CheckTx submissions")
}

// waitForHTTP polls the URL until a 200 response or timeout.
func waitForHTTP(t *testing.T, url string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 2 * time.Second}
	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("HTTP endpoint %s did not become ready within %s", url, timeout)
}

// scrapeMetrics fetches /metrics and parses Prometheus text exposition format
// into a map of metric-line key -> value. For unlabeled metrics the key is
// just the metric name (e.g. "lumera_evm_mempool_size"). For labeled metrics
// the key includes the label set (e.g. `lumera_evm_mempool_rejections_total{source="checktx",reason="ante"}`).
func scrapeMetrics(t *testing.T, url string) map[string]float64 {
	t.Helper()

	resp, err := http.Get(url)
	if err != nil {
		t.Logf("scrape failed (non-fatal): %v", err)
		return nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	result := make(map[string]float64)
	scanner := bufio.NewScanner(strings.NewReader(string(body)))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		// Only parse lines matching "lumera_evm_mempool_*" to avoid noise.
		if !strings.HasPrefix(line, "lumera_evm_mempool_") {
			continue
		}

		// Split on the last space to handle labeled metrics:
		//   lumera_evm_mempool_rejections_total{source="checktx",reason="ante"} 5
		// The key is everything before the last space, the value is after it.
		lastSpace := strings.LastIndex(line, " ")
		if lastSpace < 0 {
			continue
		}
		key := line[:lastSpace]
		valStr := line[lastSpace+1:]

		val, err := strconv.ParseFloat(valStr, 64)
		if err != nil {
			continue
		}
		result[key] = val
	}
	return result
}

func requireMetricExists(t *testing.T, metrics map[string]float64, name string) {
	t.Helper()
	for key := range metrics {
		if strings.HasPrefix(key, name) {
			return
		}
	}
	require.Fail(t, fmt.Sprintf("expected metric %q not found in /metrics scrape", name))
}

// broadcastTxSyncComet sends a base64-encoded tx to CometBFT's
// broadcast_tx_sync JSON-RPC endpoint and returns the raw response body.
func broadcastTxSyncComet(t *testing.T, cometHTTPURL, txBase64 string) []byte {
	t.Helper()

	reqBody, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "broadcast_tx_sync",
		"params":  map[string]any{"tx": txBase64},
	})
	require.NoError(t, err)

	resp, err := http.Post(cometHTTPURL, "application/json", bytes.NewReader(reqBody))
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return body
}
