package evm_test

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/tracing"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	evmantedecorators "github.com/cosmos/evm/ante/evm"
	"github.com/cosmos/evm/crypto/ethsecp256k1"
	evmencoding "github.com/cosmos/evm/encoding"
	utiltx "github.com/cosmos/evm/testutil/tx"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	"github.com/cosmos/evm/x/vm/statedb"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	vmtypes "github.com/cosmos/evm/x/vm/types/mocks"

	addresscodec "cosmossdk.io/core/address"
	"cosmossdk.io/log"
	sdkmath "cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"

	"github.com/cosmos/cosmos-sdk/client"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"

	lcfg "github.com/LumeraProtocol/lumera/config"
)

// TestEVMMonoDecoratorMatrix validates mono-decorator checks that are most
// relevant to Lumera's ante integration.
//
// Matrix:
// - Single signed EVM message in tx: accepted.
// - Multiple EVM messages packed into one tx: rejected.
func TestEVMMonoDecoratorMatrix(t *testing.T) {
	ensureChainConfigInitialized(t)
	// Use 18-decimal config for this unit test to match the assumptions in
	// testutil/tx.PrepareEthTx + CheckTxFee (denom == extended denom).
	evmtypes.SetDefaultEvmCoinInfo(evmtypes.EvmCoinInfo{
		Denom:         lcfg.ChainEVMExtendedDenom,
		ExtendedDenom: lcfg.ChainEVMExtendedDenom,
		DisplayDenom:  lcfg.ChainDisplayDenom,
		Decimals:      evmtypes.EighteenDecimals.Uint32(),
	})

	encodingCfg := evmencoding.MakeConfig(lcfg.EVMChainID)

	testCases := []struct {
		name      string
		buildMsgs func(t *testing.T, privKey *ethsecp256k1.PrivKey) []*evmtypes.MsgEthereumTx
		expectErr string
	}{
		{
			name: "single evm tx is accepted",
			buildMsgs: func(t *testing.T, privKey *ethsecp256k1.PrivKey) []*evmtypes.MsgEthereumTx {
				args := &evmtypes.EvmTxArgs{
					Nonce:    0,
					GasLimit: 100000,
					GasPrice: big.NewInt(1),
					Input:    []byte("test"),
				}
				return []*evmtypes.MsgEthereumTx{signMsgEthereumTx(t, privKey, args)}
			},
		},
		{
			name: "multiple evm tx messages in one cosmos tx are rejected",
			buildMsgs: func(t *testing.T, privKey *ethsecp256k1.PrivKey) []*evmtypes.MsgEthereumTx {
				args1 := &evmtypes.EvmTxArgs{
					Nonce:    0,
					GasLimit: 100000,
					GasPrice: big.NewInt(1),
					Input:    []byte("test"),
				}
				args2 := &evmtypes.EvmTxArgs{
					Nonce:    1,
					GasLimit: 100000,
					GasPrice: big.NewInt(1),
					Input:    []byte("test2"),
				}
				return []*evmtypes.MsgEthereumTx{
					signMsgEthereumTx(t, privKey, args1),
					signMsgEthereumTx(t, privKey, args2),
				}
			},
			expectErr: "expected 1 message, got 2",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			privKey, _ := ethsecp256k1.GenerateKey()
			keeper, cosmosAddr := setupFundedEVMKeeper(t, privKey)

			accountKeeper := monoMockAccountKeeper{fundedAddr: cosmosAddr}
			feeMarketKeeper := monoMockFeeMarketKeeper{}
			evmParams := keeper.GetParams(sdk.Context{})
			feeMarketParams := feeMarketKeeper.GetParams(sdk.Context{})

			monoDec := evmantedecorators.NewEVMMonoDecorator(
				accountKeeper,
				feeMarketKeeper,
				keeper,
				0,
				&evmParams,
				&feeMarketParams,
			)

			ctx := sdk.NewContext(nil, tmproto.Header{}, false, log.NewNopLogger())
			ctx = ctx.WithBlockGasMeter(storetypes.NewGasMeter(1e19))

			msgs := tc.buildMsgs(t, privKey)
			tx, err := utiltx.PrepareEthTx(encodingCfg.TxConfig, nil, toMsgSlice(msgs)...)
			require.NoError(t, err)

			_, err = monoDec.AnteHandle(ctx, tx, true, func(ctx sdk.Context, _ sdk.Tx, _ bool) (sdk.Context, error) {
				return ctx, nil
			})

			if tc.expectErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tc.expectErr)
		})
	}
}

// TestEVMMonoDecoratorRejectsInvalidTxType verifies the mono decorator rejects
// tx values that do not satisfy the `anteinterfaces.ProtoTxProvider` contract.
func TestEVMMonoDecoratorRejectsInvalidTxType(t *testing.T) {
	ensureChainConfigInitialized(t)
	evmtypes.SetDefaultEvmCoinInfo(evmtypes.EvmCoinInfo{
		Denom:         lcfg.ChainEVMExtendedDenom,
		ExtendedDenom: lcfg.ChainEVMExtendedDenom,
		DisplayDenom:  lcfg.ChainDisplayDenom,
		Decimals:      evmtypes.EighteenDecimals.Uint32(),
	})

	accountKeeper := monoMockAccountKeeper{}
	feeMarketKeeper := monoMockFeeMarketKeeper{}
	evmKeeper := newExtendedEVMKeeper()
	evmParams := evmKeeper.GetParams(sdk.Context{})
	feeMarketParams := feeMarketKeeper.GetParams(sdk.Context{})

	monoDec := evmantedecorators.NewEVMMonoDecorator(
		accountKeeper,
		feeMarketKeeper,
		evmKeeper,
		0,
		&evmParams,
		&feeMarketParams,
	)

	_, err := monoDec.AnteHandle(sdk.Context{}, &utiltx.InvalidTx{}, true, func(ctx sdk.Context, _ sdk.Tx, _ bool) (sdk.Context, error) {
		return ctx, nil
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "didn't implement interface ProtoTxProvider")
}

// TestEVMMonoDecoratorRejectsNonEthereumMessage verifies that an EVM-extension
// tx containing a Cosmos message fails at Ethereum message unpacking.
func TestEVMMonoDecoratorRejectsNonEthereumMessage(t *testing.T) {
	ensureChainConfigInitialized(t)
	evmtypes.SetDefaultEvmCoinInfo(evmtypes.EvmCoinInfo{
		Denom:         lcfg.ChainEVMExtendedDenom,
		ExtendedDenom: lcfg.ChainEVMExtendedDenom,
		DisplayDenom:  lcfg.ChainDisplayDenom,
		Decimals:      evmtypes.EighteenDecimals.Uint32(),
	})

	encodingCfg := evmencoding.MakeConfig(lcfg.EVMChainID)
	accountKeeper := monoMockAccountKeeper{}
	feeMarketKeeper := monoMockFeeMarketKeeper{}
	evmKeeper := newExtendedEVMKeeper()
	evmParams := evmKeeper.GetParams(sdk.Context{})
	feeMarketParams := feeMarketKeeper.GetParams(sdk.Context{})

	monoDec := evmantedecorators.NewEVMMonoDecorator(
		accountKeeper,
		feeMarketKeeper,
		evmKeeper,
		0,
		&evmParams,
		&feeMarketParams,
	)

	msg := banktypes.NewMsgSend(
		sdk.AccAddress("from_______________"),
		sdk.AccAddress("to_________________"),
		sdk.NewCoins(sdk.NewInt64Coin(lcfg.ChainDenom, 1)),
	)
	tx := buildEthereumExtensionTx(t, encodingCfg.TxConfig, msg)

	_, err := monoDec.AnteHandle(sdk.Context{}, tx, true, func(ctx sdk.Context, _ sdk.Tx, _ bool) (sdk.Context, error) {
		return ctx, nil
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid message type")
}

// TestEVMMonoDecoratorRejectsSenderMismatch verifies the signature check fails
// if msg.From does not match the recovered signer from tx signature.
func TestEVMMonoDecoratorRejectsSenderMismatch(t *testing.T) {
	ensureChainConfigInitialized(t)
	evmtypes.SetDefaultEvmCoinInfo(evmtypes.EvmCoinInfo{
		Denom:         lcfg.ChainEVMExtendedDenom,
		ExtendedDenom: lcfg.ChainEVMExtendedDenom,
		DisplayDenom:  lcfg.ChainDisplayDenom,
		Decimals:      evmtypes.EighteenDecimals.Uint32(),
	})

	encodingCfg := evmencoding.MakeConfig(lcfg.EVMChainID)
	privKey, _ := ethsecp256k1.GenerateKey()
	keeper, cosmosAddr := setupFundedEVMKeeperWithBalance(t, privKey, "1000000000000000000000000000000")

	accountKeeper := monoMockAccountKeeper{fundedAddr: cosmosAddr}
	feeMarketKeeper := monoMockFeeMarketKeeper{}
	evmParams := keeper.GetParams(sdk.Context{})
	feeMarketParams := feeMarketKeeper.GetParams(sdk.Context{})
	monoDec := evmantedecorators.NewEVMMonoDecorator(
		accountKeeper,
		feeMarketKeeper,
		keeper,
		0,
		&evmParams,
		&feeMarketParams,
	)

	msg := signMsgEthereumTx(t, privKey, &evmtypes.EvmTxArgs{
		Nonce:    0,
		GasLimit: 100000,
		GasPrice: big.NewInt(1),
		Input:    []byte("test"),
	})
	// Tamper sender after signing so recovered signer != declared from.
	msg.From = common.HexToAddress("0x0000000000000000000000000000000000000001").Bytes()

	tx, err := utiltx.PrepareEthTx(encodingCfg.TxConfig, nil, msg)
	require.NoError(t, err)
	_, err = monoDec.AnteHandle(sdk.Context{}, tx, true, func(ctx sdk.Context, _ sdk.Tx, _ bool) (sdk.Context, error) {
		return ctx, nil
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "signature verification failed")
}

// TestEVMMonoDecoratorRejectsInsufficientBalance verifies sender balance checks
// fail when total tx cost exceeds account funds.
func TestEVMMonoDecoratorRejectsInsufficientBalance(t *testing.T) {
	ensureChainConfigInitialized(t)
	evmtypes.SetDefaultEvmCoinInfo(evmtypes.EvmCoinInfo{
		Denom:         lcfg.ChainEVMExtendedDenom,
		ExtendedDenom: lcfg.ChainEVMExtendedDenom,
		DisplayDenom:  lcfg.ChainDisplayDenom,
		Decimals:      evmtypes.EighteenDecimals.Uint32(),
	})

	encodingCfg := evmencoding.MakeConfig(lcfg.EVMChainID)
	privKey, _ := ethsecp256k1.GenerateKey()
	keeper, cosmosAddr := setupFundedEVMKeeperWithBalance(t, privKey, "1")

	accountKeeper := monoMockAccountKeeper{fundedAddr: cosmosAddr}
	feeMarketKeeper := monoMockFeeMarketKeeper{}
	evmParams := keeper.GetParams(sdk.Context{})
	feeMarketParams := feeMarketKeeper.GetParams(sdk.Context{})
	monoDec := evmantedecorators.NewEVMMonoDecorator(
		accountKeeper,
		feeMarketKeeper,
		keeper,
		0,
		&evmParams,
		&feeMarketParams,
	)

	msg := signMsgEthereumTx(t, privKey, &evmtypes.EvmTxArgs{
		Nonce:    0,
		Amount:   big.NewInt(100),
		GasLimit: 100000,
		GasPrice: big.NewInt(1),
		Input:    []byte("test"),
	})

	tx, err := utiltx.PrepareEthTx(encodingCfg.TxConfig, nil, msg)
	require.NoError(t, err)
	_, err = monoDec.AnteHandle(sdk.Context{}, tx, true, func(ctx sdk.Context, _ sdk.Tx, _ bool) (sdk.Context, error) {
		return ctx, nil
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to check sender balance")
}

// TestEVMMonoDecoratorRejectsNonEOASender verifies account verification rejects
// transactions when the sender account has non-delegated contract code.
func TestEVMMonoDecoratorRejectsNonEOASender(t *testing.T) {
	ensureChainConfigInitialized(t)
	evmtypes.SetDefaultEvmCoinInfo(evmtypes.EvmCoinInfo{
		Denom:         lcfg.ChainEVMExtendedDenom,
		ExtendedDenom: lcfg.ChainEVMExtendedDenom,
		DisplayDenom:  lcfg.ChainDisplayDenom,
		Decimals:      evmtypes.EighteenDecimals.Uint32(),
	})

	encodingCfg := evmencoding.MakeConfig(lcfg.EVMChainID)
	privKey, _ := ethsecp256k1.GenerateKey()
	keeper, cosmosAddr := setupFundedEVMKeeperWithBalance(t, privKey, "1000000000000000000000000000000")
	fromAddr := common.BytesToAddress(privKey.PubKey().Address().Bytes())

	// Mark sender as a contract account by attaching non-delegated code.
	account := keeper.GetAccount(sdk.Context{}, fromAddr)
	require.NotNil(t, account)
	code := []byte{0x60, 0x00}
	codeHash := ethcrypto.Keccak256Hash(code)
	account.CodeHash = codeHash.Bytes()
	require.NoError(t, keeper.SetAccount(sdk.Context{}, fromAddr, *account))
	keeper.SetCode(sdk.Context{}, codeHash.Bytes(), code)

	accountKeeper := monoMockAccountKeeper{fundedAddr: cosmosAddr}
	feeMarketKeeper := monoMockFeeMarketKeeper{}
	evmParams := keeper.GetParams(sdk.Context{})
	feeMarketParams := feeMarketKeeper.GetParams(sdk.Context{})
	monoDec := evmantedecorators.NewEVMMonoDecorator(
		accountKeeper,
		feeMarketKeeper,
		keeper,
		0,
		&evmParams,
		&feeMarketParams,
	)

	msg := signMsgEthereumTx(t, privKey, &evmtypes.EvmTxArgs{
		Nonce:    0,
		GasLimit: 100000,
		GasPrice: big.NewInt(1),
		Input:    []byte("test"),
	})
	tx, err := utiltx.PrepareEthTx(encodingCfg.TxConfig, nil, msg)
	require.NoError(t, err)

	_, err = monoDec.AnteHandle(sdk.Context{}, tx, true, func(ctx sdk.Context, _ sdk.Tx, _ bool) (sdk.Context, error) {
		return ctx, nil
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "sender is not EOA")
}

// TestEVMMonoDecoratorAllowsDelegatedCodeSender verifies that accounts with
// EIP-7702 delegation designator code are still treated as valid senders.
func TestEVMMonoDecoratorAllowsDelegatedCodeSender(t *testing.T) {
	ensureChainConfigInitialized(t)
	evmtypes.SetDefaultEvmCoinInfo(evmtypes.EvmCoinInfo{
		Denom:         lcfg.ChainEVMExtendedDenom,
		ExtendedDenom: lcfg.ChainEVMExtendedDenom,
		DisplayDenom:  lcfg.ChainDisplayDenom,
		Decimals:      evmtypes.EighteenDecimals.Uint32(),
	})

	encodingCfg := evmencoding.MakeConfig(lcfg.EVMChainID)
	privKey, _ := ethsecp256k1.GenerateKey()
	keeper, cosmosAddr := setupFundedEVMKeeperWithBalance(t, privKey, "1000000000000000000000000000000")
	fromAddr := common.BytesToAddress(privKey.PubKey().Address().Bytes())

	// Install delegation-designator code; this must not trigger non-EOA rejection.
	account := keeper.GetAccount(sdk.Context{}, fromAddr)
	require.NotNil(t, account)
	delegationCode := ethtypes.AddressToDelegation(common.HexToAddress("0x00000000000000000000000000000000000000aa"))
	codeHash := ethcrypto.Keccak256Hash(delegationCode)
	account.CodeHash = codeHash.Bytes()
	require.NoError(t, keeper.SetAccount(sdk.Context{}, fromAddr, *account))
	keeper.SetCode(sdk.Context{}, codeHash.Bytes(), delegationCode)

	accountKeeper := monoMockAccountKeeper{fundedAddr: cosmosAddr}
	feeMarketKeeper := monoMockFeeMarketKeeper{}
	evmParams := keeper.GetParams(sdk.Context{})
	feeMarketParams := feeMarketKeeper.GetParams(sdk.Context{})
	monoDec := evmantedecorators.NewEVMMonoDecorator(
		accountKeeper,
		feeMarketKeeper,
		keeper,
		0,
		&evmParams,
		&feeMarketParams,
	)

	msg := signMsgEthereumTx(t, privKey, &evmtypes.EvmTxArgs{
		Nonce:    0,
		GasLimit: 100000,
		GasPrice: big.NewInt(1),
		Input:    []byte("test"),
	})
	tx, err := utiltx.PrepareEthTx(encodingCfg.TxConfig, nil, msg)
	require.NoError(t, err)

	ctx := newGasWantedContext(1, 1_000_000).
		WithEventManager(sdk.NewEventManager())
	_, err = monoDec.AnteHandle(ctx, tx, true, func(ctx sdk.Context, _ sdk.Tx, _ bool) (sdk.Context, error) {
		return ctx, nil
	})
	require.NoError(t, err)
}

// TestEVMMonoDecoratorRejectsGasFeeCapBelowBaseFee verifies CanTransfer checks
// reject txs whose max fee per gas is lower than the London base fee.
func TestEVMMonoDecoratorRejectsGasFeeCapBelowBaseFee(t *testing.T) {
	ensureChainConfigInitialized(t)
	evmtypes.SetDefaultEvmCoinInfo(evmtypes.EvmCoinInfo{
		Denom:         lcfg.ChainEVMExtendedDenom,
		ExtendedDenom: lcfg.ChainEVMExtendedDenom,
		DisplayDenom:  lcfg.ChainDisplayDenom,
		Decimals:      evmtypes.EighteenDecimals.Uint32(),
	})

	encodingCfg := evmencoding.MakeConfig(lcfg.EVMChainID)
	privKey, _ := ethsecp256k1.GenerateKey()
	keeper, cosmosAddr := setupFundedEVMKeeperWithBalance(t, privKey, "1000000000000000000000000000000")

	accountKeeper := monoMockAccountKeeper{fundedAddr: cosmosAddr}
	feeMarketParams := feemarkettypes.DefaultParams()
	feeMarketParams.NoBaseFee = false
	feeMarketParams.BaseFee = sdkmath.LegacyNewDec(10)
	feeMarketParams.MinGasPrice = sdkmath.LegacyZeroDec()
	feeMarketKeeper := monoStaticFeeMarketKeeper{params: feeMarketParams}
	evmParams := keeper.GetParams(sdk.Context{})

	monoDec := evmantedecorators.NewEVMMonoDecorator(
		accountKeeper,
		feeMarketKeeper,
		keeper,
		0,
		&evmParams,
		&feeMarketParams,
	)

	msg := signMsgEthereumTx(t, privKey, &evmtypes.EvmTxArgs{
		Nonce:    0,
		GasLimit: 100000,
		GasPrice: big.NewInt(1),
		Input:    []byte("test"),
	})
	tx, err := utiltx.PrepareEthTx(encodingCfg.TxConfig, nil, msg)
	require.NoError(t, err)

	ctx := sdk.Context{}.WithBlockHeight(1)
	_, err = monoDec.AnteHandle(ctx, tx, true, func(ctx sdk.Context, _ sdk.Tx, _ bool) (sdk.Context, error) {
		return ctx, nil
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "max fee per gas less than block base fee")
}

func signMsgEthereumTx(t *testing.T, privKey *ethsecp256k1.PrivKey, args *evmtypes.EvmTxArgs) *evmtypes.MsgEthereumTx {
	t.Helper()

	msg := evmtypes.NewTx(args)
	fromAddr := common.BytesToAddress(privKey.PubKey().Address().Bytes())
	msg.From = fromAddr.Bytes()

	ethSigner := ethtypes.LatestSignerForChainID(evmtypes.GetEthChainConfig().ChainID)
	require.NoError(t, msg.Sign(ethSigner, utiltx.NewSigner(privKey)))
	return msg
}

func setupFundedEVMKeeper(t *testing.T, privKey *ethsecp256k1.PrivKey) (*extendedEVMKeeper, sdk.AccAddress) {
	return setupFundedEVMKeeperWithBalance(t, privKey, "1000000000000000000000000000000")
}

func setupFundedEVMKeeperWithBalance(t *testing.T, privKey *ethsecp256k1.PrivKey, balance string) (*extendedEVMKeeper, sdk.AccAddress) {
	t.Helper()

	fromAddr := common.BytesToAddress(privKey.PubKey().Address().Bytes())
	cosmosAddr := sdk.AccAddress(fromAddr.Bytes())

	keeper := newExtendedEVMKeeper()
	funded := statedb.NewEmptyAccount()
	funded.Balance = uint256.MustFromDecimal(balance)
	require.NoError(t, keeper.SetAccount(sdk.Context{}, fromAddr, *funded))

	return keeper, cosmosAddr
}

func buildEthereumExtensionTx(t *testing.T, txCfg client.TxConfig, msgs ...sdk.Msg) sdk.Tx {
	t.Helper()

	txBuilder := txCfg.NewTxBuilder().(authtx.ExtensionOptionsTxBuilder)
	option, err := codectypes.NewAnyWithValue(&evmtypes.ExtensionOptionsEthereumTx{})
	require.NoError(t, err)
	txBuilder.SetExtensionOptions(option)
	require.NoError(t, txBuilder.SetMsgs(msgs...))
	txBuilder.SetGasLimit(100000)
	txBuilder.SetFeeAmount(sdk.NewCoins(sdk.NewCoin(lcfg.ChainEVMExtendedDenom, sdkmath.NewInt(100000))))
	return txBuilder.GetTx()
}

func toMsgSlice(msgs []*evmtypes.MsgEthereumTx) []sdk.Msg {
	out := make([]sdk.Msg, len(msgs))
	for i, msg := range msgs {
		out[i] = msg
	}
	return out
}

// extendedEVMKeeper augments the embedded EVM keeper mock with extra methods
// required by the ante `interfaces.EVMKeeper` contract.
type extendedEVMKeeper struct {
	*vmtypes.EVMKeeper
}

func newExtendedEVMKeeper() *extendedEVMKeeper {
	return &extendedEVMKeeper{EVMKeeper: vmtypes.NewEVMKeeper()}
}

func (k *extendedEVMKeeper) NewEVM(_ sdk.Context, _ core.Message, _ *statedb.EVMConfig, _ *tracing.Hooks, _ vm.StateDB) *vm.EVM {
	return nil
}

func (k *extendedEVMKeeper) DeductTxCostsFromUserBalance(_ sdk.Context, _ sdk.Coins, _ common.Address) error {
	return nil
}

func (k *extendedEVMKeeper) SpendableCoin(ctx sdk.Context, addr common.Address) *uint256.Int {
	account := k.GetAccount(ctx, addr)
	if account != nil {
		return account.Balance
	}
	return uint256.NewInt(0)
}

func (k *extendedEVMKeeper) ResetTransientGasUsed(_ sdk.Context) {}

func (k *extendedEVMKeeper) GetParams(_ sdk.Context) evmtypes.Params {
	return evmtypes.DefaultParams()
}

func (k *extendedEVMKeeper) GetTxIndexTransient(_ sdk.Context) uint64 { return 0 }

type monoMockFeeMarketKeeper struct{}

func (monoMockFeeMarketKeeper) GetParams(_ sdk.Context) feemarkettypes.Params {
	params := feemarkettypes.DefaultParams()
	params.NoBaseFee = true
	params.BaseFee = sdkmath.LegacyZeroDec()
	params.MinGasPrice = sdkmath.LegacyZeroDec()
	return params
}

func (monoMockFeeMarketKeeper) AddTransientGasWanted(_ sdk.Context, _ uint64) (uint64, error) {
	return 0, nil
}

type monoStaticFeeMarketKeeper struct {
	params feemarkettypes.Params
}

func (m monoStaticFeeMarketKeeper) GetParams(_ sdk.Context) feemarkettypes.Params {
	return m.params
}

func (monoStaticFeeMarketKeeper) AddTransientGasWanted(_ sdk.Context, _ uint64) (uint64, error) {
	return 0, nil
}

type monoMockAccountKeeper struct {
	fundedAddr sdk.AccAddress
}

func (m monoMockAccountKeeper) GetAccount(_ context.Context, addr sdk.AccAddress) sdk.AccountI {
	if m.fundedAddr != nil && addr.Equals(m.fundedAddr) {
		return &authtypes.BaseAccount{Address: addr.String()}
	}
	return nil
}

func (monoMockAccountKeeper) SetAccount(_ context.Context, _ sdk.AccountI) {}

func (monoMockAccountKeeper) NewAccountWithAddress(_ context.Context, _ sdk.AccAddress) sdk.AccountI {
	return nil
}

func (monoMockAccountKeeper) RemoveAccount(_ context.Context, _ sdk.AccountI) {}

func (monoMockAccountKeeper) GetModuleAddress(_ string) sdk.AccAddress { return sdk.AccAddress{} }

func (monoMockAccountKeeper) GetParams(_ context.Context) authtypes.Params {
	return authtypes.DefaultParams()
}

func (monoMockAccountKeeper) GetSequence(_ context.Context, _ sdk.AccAddress) (uint64, error) {
	return 0, nil
}

func (monoMockAccountKeeper) RemoveExpiredUnorderedNonces(_ sdk.Context) error { return nil }

func (monoMockAccountKeeper) TryAddUnorderedNonce(_ sdk.Context, _ []byte, _ time.Time) error {
	return nil
}

func (monoMockAccountKeeper) UnorderedTransactionsEnabled() bool { return false }

func (monoMockAccountKeeper) AddressCodec() addresscodec.Codec { return nil }
