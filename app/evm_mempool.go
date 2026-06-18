package app

import (
	"context"

	"cosmossdk.io/log"
	sdkmath "cosmossdk.io/math"

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

	// Build the Cosmos-side mempool config explicitly so we can install a
	// migration-aware SignerExtractionAdapter. Without this override, the
	// upstream PriorityNonceMempool falls back to
	// DefaultSignerExtractionAdapter, which calls tx.GetSignaturesV2() and
	// refuses zero-signer migration txs with "tx must have at least one
	// signer" *before* the migration-aware ante chain
	// (app/evm/ante.go: migrationCosmosAnte) ever runs.
	//
	// Priority / Compare / MinValue mirror upstream defaults from
	// evmmempool.NewExperimentalEVMMempool (mempool.go ~line 152) so this
	// override changes only signer extraction, nothing else.
	cosmosPoolConfig := defaultCosmosPoolConfig(app)
	cosmosPoolConfig.SignerExtractor = newEVMigrationSignerExtractionAdapter(
		sdkmempool.NewDefaultSignerExtractionAdapter(),
	)

	// Use cosmos/evm config readers so app.toml/flags values map 1:1
	// with upstream EVM behavior.
	// BroadCastTxFn is overridden to use app.clientCtx at runtime (after
	// server startup) rather than a static context captured during app.New().
	mempoolConfig := &evmmempool.EVMMempoolConfig{
		AnteHandler:      app.AnteHandler(),
		LegacyPoolConfig: evmconfig.GetLegacyPoolConfig(appOpts, logger),
		CosmosPoolConfig: cosmosPoolConfig,
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
	// ordered by (sender, nonce) correctly in proposal selection. The
	// evmigration-aware adapter is layered underneath so migration-only txs
	// — which have zero envelope signers and would otherwise be skipped
	// during proposal building — get a synthetic signer derived from
	// legacy_address.
	abciProposalHandler := baseapp.NewDefaultProposalHandler(evmMempool, app)
	abciProposalHandler.SetSignerExtractionAdapter(
		evmmempool.NewEthSignerExtractionAdapter(
			newEVMigrationSignerExtractionAdapter(
				sdkmempool.NewDefaultSignerExtractionAdapter(),
			),
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

// defaultCosmosPoolConfig replicates the upstream default Cosmos-side mempool
// config that evmmempool.NewExperimentalEVMMempool builds when
// EVMMempoolConfig.CosmosPoolConfig is nil (cosmos/evm mempool.go ~line 152).
//
// We reproduce it here so we can inject our own SignerExtractionAdapter
// (newEVMigrationSignerExtractionAdapter) without changing the priority,
// compare, or min-value semantics. Keep this function aligned with upstream
// when bumping the cosmos/evm dependency.
func defaultCosmosPoolConfig(app *App) *sdkmempool.PriorityNonceMempoolConfig[sdkmath.Int] {
	return &sdkmempool.PriorityNonceMempoolConfig[sdkmath.Int]{
		TxPriority: sdkmempool.TxPriority[sdkmath.Int]{
			GetTxPriority: func(goCtx context.Context, tx sdk.Tx) sdkmath.Int {
				ctx := sdk.UnwrapSDKContext(goCtx)
				cosmosTxFee, ok := tx.(sdk.FeeTx)
				if !ok {
					return sdkmath.ZeroInt()
				}
				// Short-circuit zero-fee / zero-gas txs without touching
				// EVM keeper state. This matters for two reasons:
				//   1. Migration-only txs (MsgClaimLegacyAccount) carry no
				//      fee — their priority is unambiguously zero and we
				//      avoid an unnecessary KVStore read.
				//   2. The SDK PriorityNonceMempool may invoke this with
				//      a ctx that has no KVStore attached (e.g. some test
				//      paths), in which case a state read panics.
				fee := cosmosTxFee.GetFee()
				gas := cosmosTxFee.GetGas()
				if gas == 0 || fee.IsZero() {
					return sdkmath.ZeroInt()
				}
				if app.EVMKeeper == nil {
					return sdkmath.ZeroInt()
				}
				found, coin := fee.Find(app.EVMKeeper.GetEvmCoinInfo(ctx).Denom)
				if !found {
					return sdkmath.ZeroInt()
				}
				return coin.Amount.Quo(sdkmath.NewIntFromUint64(gas))
			},
			Compare: func(a, b sdkmath.Int) int {
				return a.BigInt().Cmp(b.BigInt())
			},
			MinValue: sdkmath.ZeroInt(),
		},
	}
}
