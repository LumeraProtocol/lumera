package evm_test

import (
	"context"
	"math/big"
	"testing"

	"cosmossdk.io/core/store"
	sdkmath "cosmossdk.io/math"
	circuitkeeper "cosmossdk.io/x/circuit/keeper"
	wasmkeeper "github.com/CosmWasm/wasmd/x/wasm/keeper"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/auth/ante"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	evmante "github.com/cosmos/evm/ante/types"
	"github.com/cosmos/evm/crypto/ethsecp256k1"
	evmencoding "github.com/cosmos/evm/encoding"
	utiltx "github.com/cosmos/evm/testutil/tx"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	ibckeeper "github.com/cosmos/ibc-go/v10/modules/core/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	appevm "github.com/LumeraProtocol/lumera/app/evm"
	lcfg "github.com/LumeraProtocol/lumera/config"
)

// TestNewAnteHandlerRequiredDependencies verifies constructor guardrails for
// all mandatory dependencies in app/evm.NewAnteHandler.
func TestNewAnteHandlerRequiredDependencies(t *testing.T) {
	testCases := []struct {
		name        string
		mutate      func(*appevm.HandlerOptions)
		expectError string
	}{
		{
			name: "missing account keeper",
			mutate: func(opts *appevm.HandlerOptions) {
				opts.AccountKeeper = nil
			},
			expectError: "auth keeper is required for ante builder",
		},
		{
			name: "missing bank keeper",
			mutate: func(opts *appevm.HandlerOptions) {
				opts.BankKeeper = nil
			},
			expectError: "bank keeper is required for ante builder",
		},
		{
			name: "missing sign mode handler",
			mutate: func(opts *appevm.HandlerOptions) {
				opts.SignModeHandler = nil
			},
			expectError: "sign mode handler is required for ante builder",
		},
		{
			name: "missing wasm config",
			mutate: func(opts *appevm.HandlerOptions) {
				opts.WasmConfig = nil
			},
			expectError: "wasm config is required for ante builder",
		},
		{
			name: "missing wasm store service",
			mutate: func(opts *appevm.HandlerOptions) {
				opts.TXCounterStoreService = nil
			},
			expectError: "wasm store service is required for ante builder",
		},
		{
			name: "missing circuit keeper",
			mutate: func(opts *appevm.HandlerOptions) {
				opts.CircuitKeeper = nil
			},
			expectError: "circuit keeper is required for ante builder",
		},
		{
			name: "missing feemarket keeper",
			mutate: func(opts *appevm.HandlerOptions) {
				opts.FeeMarketKeeper = nil
			},
			expectError: "fee market keeper is required for ante builder",
		},
		{
			name: "missing evm keeper",
			mutate: func(opts *appevm.HandlerOptions) {
				opts.EvmKeeper = nil
			},
			expectError: "evm keeper is required for ante builder",
		},
		{
			name: "missing evm account keeper",
			mutate: func(opts *appevm.HandlerOptions) {
				opts.EVMAccountKeeper = nil
			},
			expectError: "evm account keeper is required for ante builder",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			opts := newValidAnteHandlerOptions(t)
			tc.mutate(&opts)

			anteHandler, err := appevm.NewAnteHandler(opts)
			require.Error(t, err)
			require.Nil(t, anteHandler)
			require.Contains(t, err.Error(), tc.expectError)
		})
	}
}

// TestNewAnteHandlerRoutesEthereumExtension verifies extension-option based
// routing reaches the EVM ante path for Ethereum txs.
func TestNewAnteHandlerRoutesEthereumExtension(t *testing.T) {
	ensureChainConfigInitialized(t)
	evmtypes.SetDefaultEvmCoinInfo(evmtypes.EvmCoinInfo{
		Denom:         lcfg.ChainEVMExtendedDenom,
		ExtendedDenom: lcfg.ChainEVMExtendedDenom,
		DisplayDenom:  lcfg.ChainDisplayDenom,
		Decimals:      evmtypes.EighteenDecimals.Uint32(),
	})

	opts := newValidAnteHandlerOptions(t)
	anteHandler, err := appevm.NewAnteHandler(opts)
	require.NoError(t, err)

	tx := newEthereumExtensionTxWithoutMsgs(t)
	_, err = anteHandler(sdk.Context{}, tx, true)
	require.Error(t, err)
	require.Contains(t, err.Error(), "expected 1 message, got 0")
}

// TestNewAnteHandlerRoutesDynamicFeeExtensionToCosmosPath verifies txs with
// dynamic-fee extension use the Cosmos ante path, where MsgEthereumTx is
// explicitly rejected by RejectMessagesDecorator.
func TestNewAnteHandlerRoutesDynamicFeeExtensionToCosmosPath(t *testing.T) {
	opts := newValidAnteHandlerOptions(t)
	anteHandler, err := appevm.NewAnteHandler(opts)
	require.NoError(t, err)

	tx := newDynamicFeeExtensionTxWithEthereumMsg(t)
	_, err = anteHandler(sdk.Context{}, tx, true)
	require.Error(t, err)
	require.Contains(t, err.Error(), "ExtensionOptionsEthereumTx")
}

// TestNewAnteHandlerDefaultRouteWithoutExtension verifies txs without
// extension options go to the default Cosmos ante path.
func TestNewAnteHandlerDefaultRouteWithoutExtension(t *testing.T) {
	opts := newValidAnteHandlerOptions(t)
	anteHandler, err := appevm.NewAnteHandler(opts)
	require.NoError(t, err)

	tx := newTxWithoutExtensionWithEthereumMsg(t)
	_, err = anteHandler(sdk.Context{}, tx, true)
	require.Error(t, err)
	require.Contains(t, err.Error(), "ExtensionOptionsEthereumTx")
}

// TestNewAnteHandlerUsesFirstExtensionOption_EthereumBeforeDynamic verifies
// routing is determined by the first extension option when multiple are present.
func TestNewAnteHandlerUsesFirstExtensionOption_EthereumBeforeDynamic(t *testing.T) {
	ensureChainConfigInitialized(t)
	evmtypes.SetDefaultEvmCoinInfo(evmtypes.EvmCoinInfo{
		Denom:         lcfg.ChainEVMExtendedDenom,
		ExtendedDenom: lcfg.ChainEVMExtendedDenom,
		DisplayDenom:  lcfg.ChainDisplayDenom,
		Decimals:      evmtypes.EighteenDecimals.Uint32(),
	})

	opts := newValidAnteHandlerOptions(t)
	anteHandler, err := appevm.NewAnteHandler(opts)
	require.NoError(t, err)

	ethOption, err := codectypes.NewAnyWithValue(&evmtypes.ExtensionOptionsEthereumTx{})
	require.NoError(t, err)
	dynamicFeeOption, err := codectypes.NewAnyWithValue(&evmante.ExtensionOptionDynamicFeeTx{})
	require.NoError(t, err)

	tx := newExtensionTxWithoutMsgs(t, ethOption, dynamicFeeOption)
	_, err = anteHandler(sdk.Context{}, tx, true)
	require.Error(t, err)
	require.Contains(t, err.Error(), "length of ExtensionOptions should be 1")
}

// TestNewAnteHandlerUsesFirstExtensionOption_DynamicBeforeEthereum verifies
// the second extension option is ignored for routing.
func TestNewAnteHandlerUsesFirstExtensionOption_DynamicBeforeEthereum(t *testing.T) {
	opts := newValidAnteHandlerOptions(t)
	anteHandler, err := appevm.NewAnteHandler(opts)
	require.NoError(t, err)

	dynamicFeeOption, err := codectypes.NewAnyWithValue(&evmante.ExtensionOptionDynamicFeeTx{})
	require.NoError(t, err)
	ethOption, err := codectypes.NewAnyWithValue(&evmtypes.ExtensionOptionsEthereumTx{})
	require.NoError(t, err)

	tx := newExtensionTxWithEthereumMsg(t, dynamicFeeOption, ethOption)
	_, err = anteHandler(sdk.Context{}, tx, true)
	require.Error(t, err)
	require.Contains(t, err.Error(), "ExtensionOptionsEthereumTx")
}

// TestNewAnteHandlerUsesFirstExtensionOption_UnknownBeforeEthereum verifies
// unknown first extension options fall back to Cosmos path even if Ethereum
// extension appears later.
func TestNewAnteHandlerUsesFirstExtensionOption_UnknownBeforeEthereum(t *testing.T) {
	opts := newValidAnteHandlerOptions(t)
	anteHandler, err := appevm.NewAnteHandler(opts)
	require.NoError(t, err)

	unknownOption := &codectypes.Any{TypeUrl: "/lumera.test.UnknownExtensionOption"}
	ethOption, err := codectypes.NewAnyWithValue(&evmtypes.ExtensionOptionsEthereumTx{})
	require.NoError(t, err)

	tx := newExtensionTxWithEthereumMsg(t, unknownOption, ethOption)
	_, err = anteHandler(sdk.Context{}, tx, true)
	require.Error(t, err)
	require.Contains(t, err.Error(), "ExtensionOptionsEthereumTx")
}

// TestNewAnteHandlerPendingTxListenerTriggeredForEVMCheckTx verifies the
// pending tx listener is invoked for accepted EVM txs during CheckTx.
func TestNewAnteHandlerPendingTxListenerTriggeredForEVMCheckTx(t *testing.T) {
	ensureChainConfigInitialized(t)
	evmtypes.SetDefaultEvmCoinInfo(evmtypes.EvmCoinInfo{
		Denom:         lcfg.ChainEVMExtendedDenom,
		ExtendedDenom: lcfg.ChainEVMExtendedDenom,
		DisplayDenom:  lcfg.ChainDisplayDenom,
		Decimals:      evmtypes.EighteenDecimals.Uint32(),
	})

	privKey, _ := ethsecp256k1.GenerateKey()
	keeper, cosmosAddr := setupFundedEVMKeeperWithBalance(t, privKey, "1000000000000000000000000000000")
	accountKeeper := monoMockAccountKeeper{fundedAddr: cosmosAddr}

	var heard []common.Hash
	opts := newValidAnteHandlerOptions(t)
	opts.EvmKeeper = keeper
	opts.EVMAccountKeeper = accountKeeper
	opts.AccountKeeper = accountKeeper
	opts.PendingTxListener = func(hash common.Hash) {
		heard = append(heard, hash)
	}

	anteHandler, err := appevm.NewAnteHandler(opts)
	require.NoError(t, err)

	msg := signMsgEthereumTx(t, privKey, &evmtypes.EvmTxArgs{
		Nonce:    0,
		GasLimit: 100000,
		GasPrice: big.NewInt(1),
		Input:    []byte("listener"),
	})
	tx, err := utiltx.PrepareEthTx(evmencoding.MakeConfig(lcfg.EVMChainID).TxConfig, nil, msg)
	require.NoError(t, err)

	ctx := sdk.Context{}.
		WithConsensusParams(newGasWantedContext(1, 1_000_000).ConsensusParams()).
		WithBlockHeight(1).
		WithIsCheckTx(true).
		WithEventManager(sdk.NewEventManager())

	_, err = anteHandler(ctx, tx, false)
	require.NoError(t, err)
	require.Len(t, heard, 1)
	require.Equal(t, msg.Hash(), heard[0])
}

// TestNewAnteHandlerPendingTxListenerNotTriggeredOnCosmosPath verifies the
// pending listener is not called when tx routing stays on Cosmos ante path.
func TestNewAnteHandlerPendingTxListenerNotTriggeredOnCosmosPath(t *testing.T) {
	var heard []common.Hash
	opts := newValidAnteHandlerOptions(t)
	opts.PendingTxListener = func(hash common.Hash) {
		heard = append(heard, hash)
	}

	anteHandler, err := appevm.NewAnteHandler(opts)
	require.NoError(t, err)

	tx := newTxWithoutExtensionWithEthereumMsg(t)
	ctx := sdk.Context{}.
		WithConsensusParams(newGasWantedContext(1, 1_000_000).ConsensusParams()).
		WithBlockHeight(1).
		WithIsCheckTx(true).
		WithEventManager(sdk.NewEventManager())

	_, err = anteHandler(ctx, tx, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "ExtensionOptionsEthereumTx")
	require.Empty(t, heard)
}

func newValidAnteHandlerOptions(t *testing.T) appevm.HandlerOptions {
	t.Helper()

	encodingCfg := evmencoding.MakeConfig(lcfg.EVMChainID)

	accountKeeper := monoMockAccountKeeper{}

	return appevm.HandlerOptions{
		HandlerOptions: ante.HandlerOptions{
			AccountKeeper:          accountKeeper,
			BankKeeper:             noopBankKeeper{},
			SignModeHandler:        encodingCfg.TxConfig.SignModeHandler(),
			ExtensionOptionChecker: func(*codectypes.Any) bool { return true },
		},
		IBCKeeper:             &ibckeeper.Keeper{},
		WasmConfig:            &wasmtypes.NodeConfig{},
		WasmKeeper:            &wasmkeeper.Keeper{},
		TXCounterStoreService: noopKVStoreService{},
		CircuitKeeper:         &circuitkeeper.Keeper{},
		EVMAccountKeeper:      accountKeeper,
		FeeMarketKeeper:       monoMockFeeMarketKeeper{},
		EvmKeeper:             newExtendedEVMKeeper(),
		MaxTxGasWanted:        0,
		DynamicFeeChecker:     true,
	}
}

func newEthereumExtensionTxWithoutMsgs(t *testing.T) sdk.Tx {
	t.Helper()

	option, err := codectypes.NewAnyWithValue(&evmtypes.ExtensionOptionsEthereumTx{})
	require.NoError(t, err)

	return newExtensionTxWithoutMsgs(t, option)
}

func newDynamicFeeExtensionTxWithEthereumMsg(t *testing.T) sdk.Tx {
	t.Helper()

	option, err := codectypes.NewAnyWithValue(&evmante.ExtensionOptionDynamicFeeTx{})
	require.NoError(t, err)

	return newExtensionTxWithEthereumMsg(t, option)
}

func newExtensionTxWithoutMsgs(t *testing.T, options ...*codectypes.Any) sdk.Tx {
	t.Helper()

	txCfg := evmencoding.MakeConfig(lcfg.EVMChainID).TxConfig
	txBuilder := txCfg.NewTxBuilder().(authtx.ExtensionOptionsTxBuilder)
	txBuilder.SetExtensionOptions(options...)
	txBuilder.SetGasLimit(1)
	txBuilder.SetFeeAmount(sdk.NewCoins(sdk.NewCoin(lcfg.ChainEVMExtendedDenom, sdkmath.NewInt(1))))

	return txBuilder.GetTx()
}

func newExtensionTxWithEthereumMsg(t *testing.T, options ...*codectypes.Any) sdk.Tx {
	t.Helper()

	txCfg := evmencoding.MakeConfig(lcfg.EVMChainID).TxConfig
	txBuilder := txCfg.NewTxBuilder().(authtx.ExtensionOptionsTxBuilder)
	txBuilder.SetExtensionOptions(options...)

	msg := evmtypes.NewTx(&evmtypes.EvmTxArgs{
		Nonce:    0,
		GasLimit: 21_000,
		GasPrice: big.NewInt(1),
		Input:    nil,
	})
	require.NoError(t, txBuilder.SetMsgs(msg))
	txBuilder.SetGasLimit(21_000)
	txBuilder.SetFeeAmount(sdk.NewCoins(sdk.NewCoin(lcfg.ChainEVMExtendedDenom, sdkmath.NewInt(21_000))))

	return txBuilder.GetTx()
}

func newTxWithoutExtensionWithEthereumMsg(t *testing.T) sdk.Tx {
	t.Helper()

	txCfg := evmencoding.MakeConfig(lcfg.EVMChainID).TxConfig
	txBuilder := txCfg.NewTxBuilder()
	msg := evmtypes.NewTx(&evmtypes.EvmTxArgs{
		Nonce:    0,
		GasLimit: 21_000,
		GasPrice: big.NewInt(1),
		Input:    nil,
	})
	require.NoError(t, txBuilder.SetMsgs(msg))
	txBuilder.SetGasLimit(21_000)
	txBuilder.SetFeeAmount(sdk.NewCoins(sdk.NewCoin(lcfg.ChainEVMExtendedDenom, sdkmath.NewInt(21_000))))

	return txBuilder.GetTx()
}

type noopBankKeeper struct{}

func (noopBankKeeper) IsSendEnabledCoins(_ context.Context, _ ...sdk.Coin) error { return nil }
func (noopBankKeeper) SendCoins(_ context.Context, _, _ sdk.AccAddress, _ sdk.Coins) error {
	return nil
}
func (noopBankKeeper) SendCoinsFromAccountToModule(_ context.Context, _ sdk.AccAddress, _ string, _ sdk.Coins) error {
	return nil
}

type noopKVStoreService struct{}

func (noopKVStoreService) OpenKVStore(context.Context) store.KVStore { return nil }
