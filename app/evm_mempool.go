package app

import (
	"cosmossdk.io/log"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"
	evmconfig "github.com/cosmos/evm/config"
	evmmempool "github.com/cosmos/evm/mempool"
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
	app.SetCheckTxHandler(evmmempool.NewCheckTxHandler(evmMempool))

	// PrepareProposal must use EVM-aware signer extraction so Ethereum txs are
	// ordered by (sender, nonce) correctly in proposal selection.
	abciProposalHandler := baseapp.NewDefaultProposalHandler(evmMempool, app)
	abciProposalHandler.SetSignerExtractionAdapter(
		evmmempool.NewEthSignerExtractionAdapter(
			sdkmempool.NewDefaultSignerExtractionAdapter(),
		),
	)
	app.SetPrepareProposal(abciProposalHandler.PrepareProposalHandler())

	return nil
}
