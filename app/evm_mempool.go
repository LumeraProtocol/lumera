package app

import (
	"cosmossdk.io/log"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"
	evmconfig "github.com/cosmos/evm/config"
	evmmempool "github.com/cosmos/evm/mempool"
	"github.com/prometheus/client_golang/prometheus"
)

// configureEVMMempool wires the Cosmos EVM mempool into BaseApp after ante is set.
func (app *App) configureEVMMempool(appOpts servertypes.AppOptions, logger log.Logger) error {
	if app.EVMKeeper == nil {
		logger.Debug("EVM keeper is nil, skipping EVM mempool configuration")
		return nil
	}

	// SDK semantics for mempool max tx:
	//   - < 0: app-side mempool disabled
	//   - = 0: unlimited
	//   - > 0: bounded
	cosmosPoolMaxTx := evmconfig.GetCosmosPoolMaxTx(appOpts, logger)
	if cosmosPoolMaxTx < 0 {
		logger.Debug("app-side mempool is disabled, skipping EVM mempool configuration")
		return nil
	}

	broadcastLogger := logger.With(log.ModuleKey, evmBroadcastLogModule)
	app.configureEVMBroadcastOptions(appOpts, broadcastLogger)
	app.startEVMBroadcastWorker(broadcastLogger)

	// Use cosmos/evm config readers so app.toml/flags values map 1:1
	// with upstream EVM behavior.
	// BroadCastTxFn is overridden to use app.clientCtx at runtime (after
	// server startup) rather than a static context captured during app.New().
	mempoolConfig := &evmmempool.EVMMempoolConfig{
		AnteHandler:      app.AnteHandler(),
		LegacyPoolConfig: evmconfig.GetLegacyPoolConfig(appOpts, logger),
		BlockGasLimit:    evmconfig.GetBlockGasLimit(appOpts, logger),
		MinTip:           evmconfig.GetMinTip(appOpts, logger),
		BroadCastTxFn:    app.broadcastEVMTransactions,
	}

	// The constructor requires a client context; we pass a minimal context with
	// TxConfig because broadcasting is handled by BroadCastTxFn above.
	evmMempool := evmmempool.NewExperimentalEVMMempool(
		app.CreateQueryContext,
		logger,
		app.EVMKeeper,
		app.FeeMarketKeeper,
		app.txConfig,
		client.Context{}.WithTxConfig(app.txConfig),
		mempoolConfig,
		cosmosPoolMaxTx,
	)

	app.evmMempool = evmMempool
	app.SetMempool(evmMempool)

	// Wrap the upstream CheckTxHandler so that rejected transactions
	// (non-zero response code or error) increment the labeled Prometheus
	// rejection counter with source="checktx".
	upstreamCheckTx := evmmempool.NewCheckTxHandler(evmMempool)
	app.SetCheckTxHandler(func(runTx sdk.RunTx, req *abci.RequestCheckTx) (*abci.ResponseCheckTx, error) {
		resp, err := upstreamCheckTx(runTx, req)
		if app.evmMempoolMetrics != nil && (err != nil || (resp != nil && resp.Code != 0)) {
			app.evmMempoolMetrics.IncRejection(rejSourceCheckTx, rejReasonAnte)
			if app.evmBroadcastDebug {
				code := int64(-1)
				rawLog := ""
				if resp != nil {
					code = int64(resp.Code)
					rawLog = resp.Log
				}
				app.evmBroadcastLog().Debug(
					"checktx rejection counted",
					"code", code,
					"log", rawLog,
					"err", err,
				)
			}
		}
		return resp, err
	})

	// PrepareProposal must use EVM-aware signer extraction so Ethereum txs are
	// ordered by (sender, nonce) correctly in proposal selection.
	abciProposalHandler := baseapp.NewDefaultProposalHandler(evmMempool, app)
	abciProposalHandler.SetSignerExtractionAdapter(
		evmmempool.NewEthSignerExtractionAdapter(
			sdkmempool.NewDefaultSignerExtractionAdapter(),
		),
	)
	app.SetPrepareProposal(abciProposalHandler.PrepareProposalHandler())

	// Register Prometheus metrics for the EVM mempool. Gauges are read from
	// live mempool state on each scrape; the rejection counter is incremented
	// by broadcastEVMTransactions and CheckTx paths.
	var broadcastQueueLenFn func() int
	if app.evmTxBroadcaster != nil {
		broadcastQueueLenFn = app.evmTxBroadcaster.queueLen
	}
	app.evmMempoolMetrics = newEVMMempoolMetrics(evmMempool, broadcastQueueLenFn)
	if err := prometheus.Register(app.evmMempoolMetrics); err != nil {
		logger.Warn("failed to register EVM mempool Prometheus metrics (may already be registered)", "err", err)
	}

	return nil
}
