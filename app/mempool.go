package app

import (
	"fmt"

	"cosmossdk.io/log"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"
	evmconfig "github.com/cosmos/evm/config"
	evmmempool "github.com/cosmos/evm/mempool"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
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

// broadcastEVMTransactions broadcasts promoted EVM txs using the server-set client context.
func (app *App) broadcastEVMTransactions(ethTxs []*ethtypes.Transaction) error {
	clientCtx := app.clientCtx
	if clientCtx.TxConfig == nil {
		// Keep tx encoding available even if SetClientCtx has not run yet.
		clientCtx = clientCtx.WithTxConfig(app.txConfig)
	}

	for _, ethTx := range ethTxs {
		// Wrap Ethereum tx as MsgEthereumTx and submit via Comet CheckTx path.
		msg := &evmtypes.MsgEthereumTx{}
		msg.FromEthereumTx(ethTx)

		txBuilder := app.txConfig.NewTxBuilder()
		if err := txBuilder.SetMsgs(msg); err != nil {
			return fmt.Errorf("failed to set msg in tx builder: %w", err)
		}

		txBytes, err := app.txConfig.TxEncoder()(txBuilder.GetTx())
		if err != nil {
			return fmt.Errorf("failed to encode transaction: %w", err)
		}

		res, err := clientCtx.BroadcastTxSync(txBytes)
		if err != nil {
			return fmt.Errorf("failed to broadcast transaction %s: %w", ethTx.Hash().Hex(), err)
		}
		if res.Code != 0 {
			return fmt.Errorf("transaction %s rejected by mempool: code=%d, log=%s", ethTx.Hash().Hex(), res.Code, res.RawLog)
		}
	}

	return nil
}
