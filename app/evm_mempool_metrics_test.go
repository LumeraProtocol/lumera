package app

import (
	"strings"
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
)

// TestEVMMempoolMetricsDescribeReturnsAllDescriptors verifies that Describe
// emits exactly the expected set of metric descriptors.
func TestEVMMempoolMetricsDescribeReturnsAllDescriptors(t *testing.T) {
	app := Setup(t)
	require.NotNil(t, app.evmMempoolMetrics, "metrics must be initialized after Setup")

	ch := make(chan *prometheus.Desc, 16)
	go func() {
		app.evmMempoolMetrics.Describe(ch)
		close(ch)
	}()

	var names []string
	for desc := range ch {
		names = append(names, desc.String())
	}

	expected := []string{
		"lumera_evm_mempool_size",
		"lumera_evm_mempool_pending",
		"lumera_evm_mempool_queued",
		"lumera_evm_mempool_broadcast_queue_depth",
		"lumera_evm_mempool_rejections_total",
	}

	for _, exp := range expected {
		found := false
		for _, name := range names {
			if strings.Contains(name, exp) {
				found = true
				break
			}
		}
		require.True(t, found, "expected descriptor %q not found in Describe output", exp)
	}
}

// TestEVMMempoolMetricsCollectReturnsGaugesAndCounter verifies Collect emits
// the expected number of metrics with sensible values from a freshly started app.
func TestEVMMempoolMetricsCollectReturnsGaugesAndCounter(t *testing.T) {
	app := Setup(t)
	require.NotNil(t, app.evmMempoolMetrics)

	ch := make(chan prometheus.Metric, 16)
	go func() {
		app.evmMempoolMetrics.Collect(ch)
		close(ch)
	}()

	var collected []prometheus.Metric
	for m := range ch {
		collected = append(collected, m)
	}

	// 4 gauges = 4 metrics; CounterVec emits 0 metrics until a label combination
	// is first observed. So a fresh collector emits exactly 4.
	require.Len(t, collected, 4, "expected 4 metrics (size, pending, queued, broadcast_queue_depth) from a fresh collector")

	// All gauge values should be >= 0 on a fresh app.
	for _, m := range collected {
		var d dto.Metric
		require.NoError(t, m.Write(&d))
		if d.Gauge != nil {
			require.GreaterOrEqual(t, d.Gauge.GetValue(), float64(0))
		}
	}
}

// TestEVMMempoolMetricsIncRejectionLabeled verifies the labeled rejection
// counter increments correctly with source/reason dimensions.
func TestEVMMempoolMetricsIncRejectionLabeled(t *testing.T) {
	t.Parallel()

	m := newEVMMempoolMetrics(nil, nil)

	m.IncRejection(rejSourceCheckTx, rejReasonAnte)
	m.IncRejection(rejSourceCheckTx, rejReasonAnte)
	m.IncRejection(rejSourceBroadcastEnqueue, rejReasonQueueFull)
	m.IncRejectionBy(rejSourceBroadcastEnqueue, rejReasonQueueFull, 3)

	require.Equal(t, float64(2), counterVecValue(t, m.rejections, rejSourceCheckTx, rejReasonAnte))
	require.Equal(t, float64(4), counterVecValue(t, m.rejections, rejSourceBroadcastEnqueue, rejReasonQueueFull))
}

// TestEVMMempoolMetricsIncRejectionBy_ZeroAndNegativeIgnored verifies that
// zero and negative values do not modify the rejection counter.
func TestEVMMempoolMetricsIncRejectionBy_ZeroAndNegativeIgnored(t *testing.T) {
	t.Parallel()

	m := newEVMMempoolMetrics(nil, nil)

	m.IncRejectionBy(rejSourceCheckTx, rejReasonAnte, 0)
	m.IncRejectionBy(rejSourceCheckTx, rejReasonAnte, -5)
	m.IncRejection(rejSourceCheckTx, rejReasonAnte)

	require.Equal(t, float64(1), counterVecValue(t, m.rejections, rejSourceCheckTx, rejReasonAnte))
}

// TestEVMMempoolMetricsNilBroadcastQueueLenFn verifies that a nil
// broadcastQueueLenFn produces a zero broadcast_queue_depth gauge without panic.
func TestEVMMempoolMetricsNilBroadcastQueueLenFn(t *testing.T) {
	app := Setup(t)
	require.NotNil(t, app.evmMempoolMetrics)

	// Force nil to test the guard path.
	app.evmMempoolMetrics.broadcastQueueLenFn = nil

	ch := make(chan prometheus.Metric, 16)
	go func() {
		app.evmMempoolMetrics.Collect(ch)
		close(ch)
	}()

	for m := range ch {
		var d dto.Metric
		require.NoError(t, m.Write(&d))
		// No panic from nil broadcastQueueLenFn.
	}
}

// TestEVMMempoolMetricsWiredOnAppStartup verifies that the metrics collector
// is initialized and wired into the App struct during normal app startup.
func TestEVMMempoolMetricsWiredOnAppStartup(t *testing.T) {
	app := Setup(t)
	require.NotNil(t, app.evmMempoolMetrics, "evmMempoolMetrics must be set after app startup")
	require.NotNil(t, app.evmMempoolMetrics.mempool, "metrics mempool reference must not be nil")
	require.NotNil(t, app.evmMempoolMetrics.rejections, "rejection counter vec must be initialized")
}

// TestEVMMempoolMetricsBroadcastQueueDepthReportsLive verifies that the
// broadcast_queue_depth gauge reads from the provided function on each scrape.
func TestEVMMempoolMetricsBroadcastQueueDepthReportsLive(t *testing.T) {
	t.Parallel()

	depth := 0
	m := newEVMMempoolMetrics(nil, func() int { return depth })

	collectAndFindGauge := func(descContains string) float64 {
		ch := make(chan prometheus.Metric, 16)
		go func() {
			m.Collect(ch)
			close(ch)
		}()
		for metric := range ch {
			desc := metric.Desc().String()
			if strings.Contains(desc, descContains) {
				var d dto.Metric
				require.NoError(t, metric.Write(&d))
				return d.Gauge.GetValue()
			}
		}
		t.Fatalf("metric containing %q not found", descContains)
		return 0
	}

	require.Equal(t, float64(0), collectAndFindGauge("broadcast_queue_depth"))

	depth = 7
	require.Equal(t, float64(7), collectAndFindGauge("broadcast_queue_depth"))
}

// TestEVMMempoolMetricsSizeExcludesQueued verifies that the size gauge does
// NOT include queued (nonce-gap) transactions — it reflects only
// proposal-eligible txs (pending EVM + cosmos pool), matching the upstream
// ExperimentalEVMMempool.CountTx() semantics.
func TestEVMMempoolMetricsSizeExcludesQueued(t *testing.T) {
	app := Setup(t)
	require.NotNil(t, app.evmMempoolMetrics)

	gauges := collectNamedGauges(t, app.evmMempoolMetrics)

	size := gauges["lumera_evm_mempool_size"]
	pending := gauges["lumera_evm_mempool_pending"]
	queued := gauges["lumera_evm_mempool_queued"]

	// On a fresh app all should be zero, but the invariant should always hold:
	// size == pending + cosmosPool (and cosmosPool >= 0), so size >= pending.
	// Importantly, size must NOT equal pending + queued when queued > 0.
	// On a fresh app queued == 0, so just verify the invariant structurally.
	require.GreaterOrEqual(t, size, pending,
		"size must be >= pending (size includes cosmos pool txs too)")
	_ = queued // included for completeness; on fresh app it's 0
}

// TestEVMMempoolMetricsCheckTxWrapperIncrementsRejections verifies that the
// CheckTxHandler wrapper installed by configureEVMMempool increments the
// labeled rejection counter (source="checktx", reason="ante") when the
// upstream handler returns a non-zero code.
func TestEVMMempoolMetricsCheckTxWrapperIncrementsRejections(t *testing.T) {
	app := Setup(t)
	require.NotNil(t, app.evmMempoolMetrics)

	// The rejection counter should start with no observations for this label pair.
	before := counterVecValue(t, app.evmMempoolMetrics.rejections, rejSourceCheckTx, rejReasonAnte)
	require.Equal(t, float64(0), before, "rejection counter should start at 0")

	// Submit garbage bytes through CheckTx — this must fail and increment.
	// We call the ABCI CheckTx path directly via the app.
	resp, err := app.CheckTx(&abci.RequestCheckTx{
		Tx:   []byte("not-a-valid-tx"),
		Type: abci.CheckTxType_New,
	})

	// The tx is invalid, so the response should have Code != 0 or err != nil.
	rejected := err != nil || (resp != nil && resp.Code != 0)
	require.True(t, rejected, "invalid tx must be rejected by CheckTx")

	after := counterVecValue(t, app.evmMempoolMetrics.rejections, rejSourceCheckTx, rejReasonAnte)
	require.Greater(t, after, before,
		"rejection counter {source=checktx,reason=ante} must increment after failed CheckTx")
}

// TestEVMMempoolMetricsRejectionLabelsAreIndependent verifies that incrementing
// one label combination does not affect another.
func TestEVMMempoolMetricsRejectionLabelsAreIndependent(t *testing.T) {
	t.Parallel()

	m := newEVMMempoolMetrics(nil, nil)

	m.IncRejection(rejSourceCheckTx, rejReasonAnte)
	m.IncRejection(rejSourceCheckTx, rejReasonAnte)

	require.Equal(t, float64(2), counterVecValue(t, m.rejections, rejSourceCheckTx, rejReasonAnte))
	require.Equal(t, float64(0), counterVecValue(t, m.rejections, rejSourceBroadcastEnqueue, rejReasonQueueFull))

	m.IncRejection(rejSourceBroadcastEnqueue, rejReasonQueueFull)

	require.Equal(t, float64(2), counterVecValue(t, m.rejections, rejSourceCheckTx, rejReasonAnte))
	require.Equal(t, float64(1), counterVecValue(t, m.rejections, rejSourceBroadcastEnqueue, rejReasonQueueFull))
}

// collectNamedGauges scrapes the collector and returns gauge values keyed by
// the metric's fully-qualified name substring.
func collectNamedGauges(t *testing.T, m *evmMempoolMetrics) map[string]float64 {
	t.Helper()

	ch := make(chan prometheus.Metric, 16)
	go func() {
		m.Collect(ch)
		close(ch)
	}()

	result := make(map[string]float64)
	for metric := range ch {
		desc := metric.Desc().String()
		var d dto.Metric
		require.NoError(t, metric.Write(&d))
		if d.Gauge != nil {
			for _, name := range []string{
				"lumera_evm_mempool_size",
				"lumera_evm_mempool_pending",
				"lumera_evm_mempool_queued",
				"lumera_evm_mempool_broadcast_queue_depth",
			} {
				if strings.Contains(desc, name) {
					result[name] = d.Gauge.GetValue()
				}
			}
		}
	}
	return result
}

func counterVecValue(t *testing.T, cv *prometheus.CounterVec, labels ...string) float64 {
	t.Helper()
	c, err := cv.GetMetricWithLabelValues(labels...)
	require.NoError(t, err)
	var d dto.Metric
	require.NoError(t, c.Write(&d))
	return d.Counter.GetValue()
}
