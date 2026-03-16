package evm

import (
	"errors"

	corestoretypes "cosmossdk.io/core/store"
	circuitante "cosmossdk.io/x/circuit/ante"
	circuitkeeper "cosmossdk.io/x/circuit/keeper"
	wasmkeeper "github.com/CosmWasm/wasmd/x/wasm/keeper"
	wasmTypes "github.com/CosmWasm/wasmd/x/wasm/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/auth/ante"
	sdkvesting "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
	evmante "github.com/cosmos/evm/ante"
	cosmosante "github.com/cosmos/evm/ante/cosmos"
	evmantedecorators "github.com/cosmos/evm/ante/evm"
	anteinterfaces "github.com/cosmos/evm/ante/interfaces"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	ibcante "github.com/cosmos/ibc-go/v10/modules/core/ante"
	ibckeeper "github.com/cosmos/ibc-go/v10/modules/core/keeper"
	"github.com/ethereum/go-ethereum/common"

	lumante "github.com/LumeraProtocol/lumera/ante"
)

// genesisSkipDecorator wraps an inner AnteDecorator and skips it at genesis
// height (BlockHeight == 0). This matches how the SDK itself skips fee, gas,
// and signature checks during InitGenesis so that gentxs don't need fees.
type genesisSkipDecorator struct {
	inner sdk.AnteDecorator
}

func (d genesisSkipDecorator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (sdk.Context, error) {
	if ctx.BlockHeight() == 0 {
		return next(ctx, tx, simulate)
	}
	return d.inner.AnteHandle(ctx, tx, simulate, next)
}

// HandlerOptions extend the SDK's AnteHandler options by requiring the IBC
// channel keeper, wasm keepers, and EVM keepers for dual-routing.
type HandlerOptions struct {
	ante.HandlerOptions

	IBCKeeper             *ibckeeper.Keeper
	WasmConfig            *wasmTypes.NodeConfig
	WasmKeeper            *wasmkeeper.Keeper
	TXCounterStoreService corestoretypes.KVStoreService
	CircuitKeeper         *circuitkeeper.Keeper

	// EVM keepers for dual-routing ante handler.
	// EVMAccountKeeper satisfies the cosmos/evm AccountKeeper interface
	// (superset of the SDK ante.AccountKeeper).
	EVMAccountKeeper  anteinterfaces.AccountKeeper
	FeeMarketKeeper   anteinterfaces.FeeMarketKeeper
	EvmKeeper         anteinterfaces.EVMKeeper
	PendingTxListener func(common.Hash)
	MaxTxGasWanted    uint64
	DynamicFeeChecker bool
}

// NewAnteHandler returns an ante handler that routes EVM transactions to the
// EVM mono decorator and Cosmos transactions to the Lumera-custom Cosmos chain.
func NewAnteHandler(options HandlerOptions) (sdk.AnteHandler, error) {
	if options.AccountKeeper == nil {
		return nil, errors.New("auth keeper is required for ante builder")
	}
	if options.BankKeeper == nil {
		return nil, errors.New("bank keeper is required for ante builder")
	}
	if options.SignModeHandler == nil {
		return nil, errors.New("sign mode handler is required for ante builder")
	}
	if options.WasmConfig == nil {
		return nil, errors.New("wasm config is required for ante builder")
	}
	if options.TXCounterStoreService == nil {
		return nil, errors.New("wasm store service is required for ante builder")
	}
	if options.CircuitKeeper == nil {
		return nil, errors.New("circuit keeper is required for ante builder")
	}
	if options.FeeMarketKeeper == nil {
		return nil, errors.New("fee market keeper is required for ante builder")
	}
	if options.EvmKeeper == nil {
		return nil, errors.New("evm keeper is required for ante builder")
	}
	if options.EVMAccountKeeper == nil {
		return nil, errors.New("evm account keeper is required for ante builder")
	}

	return func(ctx sdk.Context, tx sdk.Tx, sim bool) (sdk.Context, error) {
		// Check for EVM extension options
		txWithExtensions, ok := tx.(ante.HasExtensionOptionsTx)
		if ok {
			opts := txWithExtensions.GetExtensionOptions()
			if len(opts) > 0 {
				typeURL := opts[0].GetTypeUrl()
				switch typeURL {
				case "/cosmos.evm.vm.v1.ExtensionOptionsEthereumTx":
					return newEVMAnteHandler(ctx, options)(ctx, tx, sim)
				case "/cosmos.evm.ante.v1.ExtensionOptionDynamicFeeTx":
					return newLumeraCosmosAnteHandler(ctx, options)(ctx, tx, sim)
				}
			}
		}

		// Default: standard Cosmos tx
		return newLumeraCosmosAnteHandler(ctx, options)(ctx, tx, sim)
	}, nil
}

// newEVMAnteHandler builds the ante handler chain for EVM transactions.
func newEVMAnteHandler(ctx sdk.Context, options HandlerOptions) sdk.AnteHandler {
	evmParams := options.EvmKeeper.GetParams(ctx)
	feemarketParams := options.FeeMarketKeeper.GetParams(ctx)
	pendingTxListener := options.PendingTxListener
	if pendingTxListener == nil {
		pendingTxListener = func(common.Hash) {}
	}

	return sdk.ChainAnteDecorators(
		// NewEVMMonoDecorator is the canonical Cosmos-EVM precheck pipeline for
		// Ethereum transactions (validation, signature, balance/fee checks, nonce,
		// gas accounting). Keep it first so EVM tx semantics stay aligned upstream.
		evmantedecorators.NewEVMMonoDecorator(
			options.EVMAccountKeeper,
			options.FeeMarketKeeper,
			options.EvmKeeper,
			options.MaxTxGasWanted,
			&evmParams,
			&feemarketParams,
		),
		evmante.NewTxListenerDecorator(pendingTxListener),
	)
}

// newLumeraCosmosAnteHandler builds the ante handler chain for Cosmos transactions,
// merging Lumera-specific decorators with cosmos/evm additions.
func newLumeraCosmosAnteHandler(ctx sdk.Context, options HandlerOptions) sdk.AnteHandler {
	feemarketParams := options.FeeMarketKeeper.GetParams(ctx)

	var txFeeChecker ante.TxFeeChecker
	if options.DynamicFeeChecker {
		txFeeChecker = evmantedecorators.NewDynamicFeeChecker(&feemarketParams)
	}

	minGasDecorator := genesisSkipDecorator{cosmosante.NewMinGasPriceDecorator(&feemarketParams)}
	deductFeeDecorator := ante.NewDeductFeeDecorator(options.AccountKeeper, options.BankKeeper, options.FeegrantKeeper, txFeeChecker)

	standardCosmosAnte := sdk.ChainAnteDecorators(
		// Lumera: waive fees for delayed claim txs
		lumante.DelayedClaimFeeDecorator{},
		// cosmos/evm: reject MsgEthereumTx in Cosmos path
		cosmosante.NewRejectMessagesDecorator(),
		// cosmos/evm: block EVM msgs in authz
		cosmosante.NewAuthzLimiterDecorator(
			sdk.MsgTypeURL(&evmtypes.MsgEthereumTx{}),
			sdk.MsgTypeURL(&sdkvesting.MsgCreateVestingAccount{}),
		),
		ante.NewSetUpContextDecorator(),
		// Lumera: wasm decorators
		wasmkeeper.NewLimitSimulationGasDecorator(options.WasmConfig.SimulationGasLimit),
		wasmkeeper.NewCountTXDecorator(options.TXCounterStoreService),
		wasmkeeper.NewGasRegisterDecorator(options.WasmKeeper.GetGasRegister()),
		// Lumera: circuit breaker
		circuitante.NewCircuitBreakerDecorator(options.CircuitKeeper),
		ante.NewExtensionOptionsDecorator(options.ExtensionOptionChecker),
		lumante.EVMigrationValidateBasicDecorator{},
		ante.NewTxTimeoutHeightDecorator(),
		ante.NewValidateMemoDecorator(options.AccountKeeper),
		// cosmos/evm: min gas price from feemarket params
		// Wrapped to skip at genesis height (BlockHeight==0) so gentxs don't
		// need fees, matching how the SDK skips fee/gas/sig checks at genesis.
		minGasDecorator,
		ante.NewConsumeGasForTxSizeDecorator(options.AccountKeeper),
		deductFeeDecorator,
		ante.NewSetPubKeyDecorator(options.AccountKeeper),
		ante.NewValidateSigCountDecorator(options.AccountKeeper),
		ante.NewSigGasConsumeDecorator(options.AccountKeeper, options.SigGasConsumer),
		ante.NewSigVerificationDecorator(options.AccountKeeper, options.SignModeHandler),
		ante.NewIncrementSequenceDecorator(options.AccountKeeper),
		ibcante.NewRedundantRelayDecorator(options.IBCKeeper),
		// cosmos/evm: track gas wanted for feemarket
		evmantedecorators.NewGasWantedDecorator(options.EvmKeeper, options.FeeMarketKeeper, &feemarketParams),
	)

	migrationCosmosAnte := sdk.ChainAnteDecorators(
		// cosmos/evm: reject MsgEthereumTx in Cosmos path
		cosmosante.NewRejectMessagesDecorator(),
		// cosmos/evm: block EVM msgs in authz
		cosmosante.NewAuthzLimiterDecorator(
			sdk.MsgTypeURL(&evmtypes.MsgEthereumTx{}),
			sdk.MsgTypeURL(&sdkvesting.MsgCreateVestingAccount{}),
		),
		ante.NewSetUpContextDecorator(),
		// Lumera: wasm decorators
		wasmkeeper.NewLimitSimulationGasDecorator(options.WasmConfig.SimulationGasLimit),
		wasmkeeper.NewCountTXDecorator(options.TXCounterStoreService),
		wasmkeeper.NewGasRegisterDecorator(options.WasmKeeper.GetGasRegister()),
		// Lumera: circuit breaker
		circuitante.NewCircuitBreakerDecorator(options.CircuitKeeper),
		ante.NewExtensionOptionsDecorator(options.ExtensionOptionChecker),
		// Migration txs authenticate via message payload proofs and intentionally
		// skip the standard fee/signature/sequence subchain.
		lumante.EVMigrationValidateBasicDecorator{},
		ante.NewTxTimeoutHeightDecorator(),
		ante.NewValidateMemoDecorator(options.AccountKeeper),
		ante.NewConsumeGasForTxSizeDecorator(options.AccountKeeper),
		ibcante.NewRedundantRelayDecorator(options.IBCKeeper),
		// cosmos/evm: track gas wanted for feemarket
		evmantedecorators.NewGasWantedDecorator(options.EvmKeeper, options.FeeMarketKeeper, &feemarketParams),
	)

	return func(ctx sdk.Context, tx sdk.Tx, simulate bool) (sdk.Context, error) {
		if lumante.IsEVMigrationOnlyTx(tx) {
			return migrationCosmosAnte(ctx, tx, simulate)
		}
		return standardCosmosAnte(ctx, tx, simulate)
	}
}
