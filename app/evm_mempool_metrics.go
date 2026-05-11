package app

import (
	"github.com/prometheus/client_golang/prometheus"

	evmmempool "github.com/cosmos/evm/mempool"
)

const evmMempoolMetricsNamespace = "lumera"
const evmMempoolMetricsSubsystem = "evm_mempool"

// Rejection label values for source.
const (
	rejSourceCheckTx          = "checktx"
	rejSourceBroadcastEnqueue = "broadcast_enqueue"
)

// Rejection label values for reason.
const (
	rejReasonQueueFull = "queue_full"
	rejReasonAnte      = "ante"
)

// evmMempoolMetrics exposes Prometheus metrics for the app-side EVM mempool.
//
// Gauges (size, pending, queued, broadcast_queue_depth) are read from live
// mempool state on each Prometheus scrape via the Collector interface — no
// stale values, no background goroutine.
//
// The rejection counter (rejections_total) tracks app-side mempool rejections
// observed through two instrumented paths:
//   - source="checktx": the wrapped CheckTxHandler saw a non-zero response code
//     or error from the upstream handler (ante failure, decode failure, etc.)
//   - source="broadcast_enqueue": the async broadcast dispatcher queue was full
//     and the batch could not be enqueued
//
// JSON-RPC-level rejections (e.g. replacement-underpriced from txpool.Add) are
// NOT counted here — they do not pass through CheckTx.
type evmMempoolMetrics struct {
	mempool *evmmempool.ExperimentalEVMMempool

	// broadcastQueueLenFn returns the current broadcast dispatcher queue length.
	// nil when the dispatcher is not running (e.g. mempool disabled).
	broadcastQueueLenFn func() int

	// Descriptors (registered once, reported on every scrape).
	sizeDesc              *prometheus.Desc
	pendingDesc           *prometheus.Desc
	queuedDesc            *prometheus.Desc
	broadcastQueueLenDesc *prometheus.Desc

	// rejections is a labeled push counter: labels are "source" and "reason".
	rejections *prometheus.CounterVec
}

// newEVMMempoolMetrics creates a new metrics collector. It does NOT register
// with Prometheus — the caller must call prometheus.Register or
// prometheus.MustRegister.
func newEVMMempoolMetrics(
	mempool *evmmempool.ExperimentalEVMMempool,
	broadcastQueueLenFn func() int,
) *evmMempoolMetrics {
	return &evmMempoolMetrics{
		mempool:             mempool,
		broadcastQueueLenFn: broadcastQueueLenFn,

		sizeDesc: prometheus.NewDesc(
			prometheus.BuildFQName(evmMempoolMetricsNamespace, evmMempoolMetricsSubsystem, "size"),
			"Total number of proposal-eligible transactions in the EVM mempool (pending EVM + cosmos pool). Does not include queued (nonce-gap) transactions.",
			nil, nil,
		),
		pendingDesc: prometheus.NewDesc(
			prometheus.BuildFQName(evmMempoolMetricsNamespace, evmMempoolMetricsSubsystem, "pending"),
			"Number of executable (pending) transactions in the EVM tx pool.",
			nil, nil,
		),
		queuedDesc: prometheus.NewDesc(
			prometheus.BuildFQName(evmMempoolMetricsNamespace, evmMempoolMetricsSubsystem, "queued"),
			"Number of non-executable (queued) transactions in the EVM tx pool.",
			nil, nil,
		),
		broadcastQueueLenDesc: prometheus.NewDesc(
			prometheus.BuildFQName(evmMempoolMetricsNamespace, evmMempoolMetricsSubsystem, "broadcast_queue_depth"),
			"Number of batches waiting in the async broadcast dispatcher queue.",
			nil, nil,
		),

		rejections: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: evmMempoolMetricsNamespace,
			Subsystem: evmMempoolMetricsSubsystem,
			Name:      "rejections_total",
			Help:      "App-side EVM mempool rejections by source and reason.",
		}, []string{"source", "reason"}),
	}
}

// Describe implements prometheus.Collector.
func (m *evmMempoolMetrics) Describe(ch chan<- *prometheus.Desc) {
	ch <- m.sizeDesc
	ch <- m.pendingDesc
	ch <- m.queuedDesc
	ch <- m.broadcastQueueLenDesc
	m.rejections.Describe(ch)
}

// Collect implements prometheus.Collector.
// Gauge values are read from the live mempool on every scrape.
func (m *evmMempoolMetrics) Collect(ch chan<- prometheus.Metric) {
	var pending, queued, size int
	if m.mempool != nil {
		if pool := m.mempool.GetTxPool(); pool != nil {
			pending, queued = pool.Stats()
		}
		size = m.mempool.CountTx()
	}

	ch <- prometheus.MustNewConstMetric(m.sizeDesc, prometheus.GaugeValue, float64(size))
	ch <- prometheus.MustNewConstMetric(m.pendingDesc, prometheus.GaugeValue, float64(pending))
	ch <- prometheus.MustNewConstMetric(m.queuedDesc, prometheus.GaugeValue, float64(queued))

	broadcastDepth := 0
	if m.broadcastQueueLenFn != nil {
		broadcastDepth = m.broadcastQueueLenFn()
	}
	ch <- prometheus.MustNewConstMetric(m.broadcastQueueLenDesc, prometheus.GaugeValue, float64(broadcastDepth))

	m.rejections.Collect(ch)
}

// IncRejection increments the labeled rejection counter.
func (m *evmMempoolMetrics) IncRejection(source, reason string) {
	m.rejections.WithLabelValues(source, reason).Inc()
}

// IncRejectionBy increments the labeled rejection counter by n.
func (m *evmMempoolMetrics) IncRejectionBy(source, reason string, n int) {
	if n > 0 {
		m.rejections.WithLabelValues(source, reason).Add(float64(n))
	}
}

// RejectionsCounterVec returns the underlying CounterVec for test assertions.
func (m *evmMempoolMetrics) RejectionsCounterVec() *prometheus.CounterVec {
	return m.rejections
}
