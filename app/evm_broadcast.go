package app

import (
	"errors"
	"fmt"
	"math/big"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"cosmossdk.io/log"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	ethtypes "github.com/ethereum/go-ethereum/core/types"

	lcfg "github.com/LumeraProtocol/lumera/config"
	textutil "github.com/LumeraProtocol/lumera/pkg/text"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/ethereum/go-ethereum/common"
)

const (
	evmMempoolBroadcastDebugAppOpt = "lumera.evm-mempool.broadcast-debug"
	evmBroadcastLogModule          = "evm-broadcast"
	evmBroadcastQueueSize          = 1024
	evmBroadcastStopTimeout        = 2 * time.Second
)

// evmBroadcastBatch is the unit sent from the mempool callback into the
// asynchronous broadcaster worker.
type evmBroadcastBatch struct {
	txs []*ethtypes.Transaction
}

type evmBroadcastWorkerExit struct {
	panicked   bool
	panicValue interface{}
	panicStack string
}

// evmTxBroadcastDispatcher decouples txpool promotion from Comet CheckTx
// submission so we do not re-enter app mempool Insert() in the same call stack.
type evmTxBroadcastDispatcher struct {
	logger  log.Logger
	process func([]*ethtypes.Transaction) error

	// queue holds pending broadcast batches produced by BroadcastTxFn.
	queue chan evmBroadcastBatch
	// stopCh requests worker termination; doneCh signals worker has exited.
	stopCh chan struct{}
	doneCh chan evmBroadcastWorkerExit
	// processing indicates whether the worker is currently executing process().
	processing atomic.Bool
	stopOnce   sync.Once

	// pending tracks tx hashes currently queued or being processed to dedupe
	// repeated promotion notifications.
	mtx     sync.Mutex
	pending map[common.Hash]struct{}
}

// configureEVMBroadcastOptions reads app-level broadcast debug settings once on
// startup and wires the logger module key used by log-level filters.
func (app *App) configureEVMBroadcastOptions(appOpts servertypes.AppOptions, logger log.Logger) {
	app.evmBroadcastLogger = logger
	app.evmBroadcastDebug = textutil.ParseAppOptionBool(appOpts.Get(evmMempoolBroadcastDebugAppOpt))

	if app.evmBroadcastDebug {
		app.evmBroadcastLogger.Info(
			"evm mempool broadcast debug logs enabled",
			"app_option", evmMempoolBroadcastDebugAppOpt,
		)
	}
}

func (app *App) evmBroadcastLog() log.Logger {
	if app.evmBroadcastLogger != nil {
		return app.evmBroadcastLogger
	}
	if app.App != nil {
		return app.Logger().With(log.ModuleKey, evmBroadcastLogModule)
	}
	return log.NewNopLogger().With(log.ModuleKey, evmBroadcastLogModule)
}

// newEVMTxBroadcastDispatcher starts a single worker that processes broadcast
// batches sequentially.
func newEVMTxBroadcastDispatcher(
	logger log.Logger,
	queueSize int,
	process func([]*ethtypes.Transaction) error,
) *evmTxBroadcastDispatcher {
	dispatcher := &evmTxBroadcastDispatcher{
		logger:  logger,
		process: process,
		queue:   make(chan evmBroadcastBatch, queueSize),
		stopCh:  make(chan struct{}),
		doneCh:  make(chan evmBroadcastWorkerExit, 1),
		pending: make(map[common.Hash]struct{}),
	}

	go dispatcher.run()
	return dispatcher
}

// stop requests worker shutdown and waits up to timeout for clean exit.
func (d *evmTxBroadcastDispatcher) stop(timeout time.Duration) {
	d.stopOnce.Do(func() {
		close(d.stopCh)
	})

	select {
	case exit := <-d.doneCh:
		if exit.panicked {
			d.logger.Error(
				"evm mempool broadcast worker exited due to panic",
				"panic", fmt.Sprint(exit.panicValue),
				"stack", exit.panicStack,
			)
		}
	case <-time.After(timeout):
		d.logger.Error(
			"timed out waiting for evm mempool broadcast worker to stop (likely slow or blocked processing)",
			"processing", d.processing.Load(),
			"queue_len", len(d.queue),
		)
	}
}

func (d *evmTxBroadcastDispatcher) queueLen() int {
	return len(d.queue)
}

// enqueue dedupes by tx hash against the in-flight set and pushes accepted txs
// to the worker queue. Duplicates are intentionally dropped, because a tx hash
// already queued or broadcasting will either succeed or be retried by later
// promotion events if it gets re-promoted.
func (d *evmTxBroadcastDispatcher) enqueue(txs []*ethtypes.Transaction) (accepted, deduped int, err error) {
	if len(txs) == 0 {
		return 0, 0, nil
	}

	var filtered []*ethtypes.Transaction

	d.mtx.Lock()
	for _, tx := range txs {
		if tx == nil {
			deduped++
			continue
		}

		hash := tx.Hash()
		if _, exists := d.pending[hash]; exists {
			deduped++
			continue
		}

		d.pending[hash] = struct{}{}
		if filtered == nil {
			filtered = make([]*ethtypes.Transaction, 0, len(txs))
		}
		filtered = append(filtered, tx)
	}
	d.mtx.Unlock()

	if len(filtered) == 0 {
		return 0, deduped, nil
	}

	batch := evmBroadcastBatch{txs: filtered}
	select {
	case d.queue <- batch:
		return len(filtered), deduped, nil
	default:
		d.releasePending(filtered)
		return 0, deduped, fmt.Errorf("evm mempool broadcast queue is full (capacity=%d)", cap(d.queue))
	}
}

// run processes batches on a single goroutine to keep broadcast order stable
// and simplify dedupe bookkeeping.
func (d *evmTxBroadcastDispatcher) run() {
	defer func() {
		exit := evmBroadcastWorkerExit{}
		if r := recover(); r != nil {
			exit.panicked = true
			exit.panicValue = r
			exit.panicStack = string(debug.Stack())
		}
		d.doneCh <- exit
		close(d.doneCh)
	}()

	for {
		select {
		case <-d.stopCh:
			return
		case batch := <-d.queue:
			if len(batch.txs) == 0 {
				continue
			}

			d.processing.Store(true)
			func() {
				defer d.processing.Store(false)
				defer d.releasePending(batch.txs)

				if err := d.process(batch.txs); err != nil {
					d.logger.Error(
						"failed to broadcast promoted evm transactions",
						"count", len(batch.txs),
						"err", err,
					)
				}
			}()
		}
	}
}

// releasePending removes hashes from the in-flight set after processing or when
// queueing fails.
func (d *evmTxBroadcastDispatcher) releasePending(txs []*ethtypes.Transaction) {
	d.mtx.Lock()
	defer d.mtx.Unlock()

	for _, tx := range txs {
		if tx == nil {
			continue
		}
		delete(d.pending, tx.Hash())
	}
}

// startEVMBroadcastWorker initializes the async broadcaster once during app
// startup after mempool config is known.
func (app *App) startEVMBroadcastWorker(logger log.Logger) {
	if app.evmTxBroadcaster != nil {
		return
	}

	app.evmTxBroadcaster = newEVMTxBroadcastDispatcher(
		logger,
		evmBroadcastQueueSize,
		app.broadcastEVMTransactionsSync,
	)
	logger.Info("started evm mempool broadcast worker", "queue_size", evmBroadcastQueueSize)
}

// stopEVMBroadcastWorker terminates the worker on app shutdown.
func (app *App) stopEVMBroadcastWorker() {
	if app.evmTxBroadcaster == nil {
		return
	}

	app.evmTxBroadcaster.stop(evmBroadcastStopTimeout)
	app.evmTxBroadcaster = nil
}

// broadcastEVMTransactions enqueues promoted txs so Insert() is never blocked by
// Comet CheckTx execution in the same call stack.
func (app *App) broadcastEVMTransactions(ethTxs []*ethtypes.Transaction) error {
	if len(ethTxs) == 0 {
		return nil
	}

	if app.clientCtx.Client == nil {
		// Keep explicit offline behavior for tests/startup diagnostics.
		return fmt.Errorf("failed to broadcast transaction: no RPC client is defined in offline mode")
	}

	// Defensive fallback (worker should always be initialized during app setup).
	// Keeping this path avoids panics if lifecycle wiring changes in the future.
	if app.evmTxBroadcaster == nil {
		return app.broadcastEVMTransactionsSync(ethTxs)
	}

	accepted, deduped, err := app.evmTxBroadcaster.enqueue(ethTxs)
	if err != nil {
		return err
	}

	if app.evmBroadcastDebug {
		app.evmBroadcastLog().Debug(
			"evm mempool broadcast batch enqueued",
			"count", len(ethTxs),
			"accepted", accepted,
			"deduped", deduped,
			"queue_len", app.evmTxBroadcaster.queueLen(),
		)
	}

	return nil
}

// broadcastEVMTransactionsSync performs actual CheckTx submission and is called
// by the worker (and only as a defensive fallback directly).
func (app *App) broadcastEVMTransactionsSync(ethTxs []*ethtypes.Transaction) error {
	clientCtx := app.clientCtx
	if clientCtx.TxConfig == nil {
		// Keep tx encoding available even if SetClientCtx has not run yet.
		clientCtx = clientCtx.WithTxConfig(app.txConfig)
	}
	if app.evmBroadcastDebug {
		app.evmBroadcastLog().Debug(
			"evm mempool broadcast batch start",
			"count", len(ethTxs),
			"has_client", clientCtx.Client != nil,
			"client_type", fmt.Sprintf("%T", clientCtx.Client),
		)
	}

	var errs []error
	for _, ethTx := range ethTxs {
		startedAt := time.Now()
		if app.evmBroadcastDebug {
			app.evmBroadcastLog().Debug(
				"evm mempool broadcast tx start",
				"hash", ethTx.Hash().Hex(),
				"nonce", ethTx.Nonce(),
			)
		}

		// Wrap Ethereum tx as MsgEthereumTx and submit via Comet CheckTx path.
		// FromSignedEthereumTx recovers the sender address from the signature,
		// which is required by MsgEthereumTx.ValidateBasic / GetSigners.
		msg := &evmtypes.MsgEthereumTx{}
		ethSigner := ethtypes.LatestSignerForChainID(new(big.Int).SetUint64(lcfg.EVMChainID))
		if err := msg.FromSignedEthereumTx(ethTx, ethSigner); err != nil {
			errs = append(errs, fmt.Errorf("failed to recover sender for tx %s: %w", ethTx.Hash().Hex(), err))
			continue
		}

		txBuilder := app.txConfig.NewTxBuilder()
		if err := txBuilder.SetMsgs(msg); err != nil {
			errs = append(errs, fmt.Errorf("failed to set msg in tx builder: %w", err))
			continue
		}

		txBytes, err := app.txConfig.TxEncoder()(txBuilder.GetTx())
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to encode transaction: %w", err))
			continue
		}

		res, err := clientCtx.BroadcastTxSync(txBytes)
		if app.evmBroadcastDebug {
			broadcastCode := int64(-1)
			broadcastLog := ""
			if res != nil {
				broadcastCode = int64(res.Code)
				broadcastLog = res.RawLog
			}
			app.evmBroadcastLog().Debug(
				"evm mempool broadcast tx end",
				"hash", ethTx.Hash().Hex(),
				"nonce", ethTx.Nonce(),
				"elapsed_ms", time.Since(startedAt).Milliseconds(),
				"err", err,
				"code", broadcastCode,
				"log", broadcastLog,
			)
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to broadcast transaction %s: %w", ethTx.Hash().Hex(), err))
			continue
		}
		if res.Code != 0 {
			errs = append(errs, fmt.Errorf("transaction %s rejected by mempool: code=%d, log=%s", ethTx.Hash().Hex(), res.Code, res.RawLog))
			continue
		}
	}
	if app.evmBroadcastDebug {
		app.evmBroadcastLog().Debug("evm mempool broadcast batch end", "count", len(ethTxs), "errors", len(errs))
	}

	return errors.Join(errs...)
}
