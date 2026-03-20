package evm_test

import (
	"testing"

	"cosmossdk.io/math"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/x/auth/ante"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/stretchr/testify/require"

	lumeraapp "github.com/LumeraProtocol/lumera/app"
	appevm "github.com/LumeraProtocol/lumera/app/evm"
	lcfg "github.com/LumeraProtocol/lumera/config"
	evmigrationtypes "github.com/LumeraProtocol/lumera/x/evmigration/types"
)

// TestNewAnteHandlerMigrationOnlyCosmosTxUsesReducedAntePath verifies the
// Cosmos ante builder branches once for migration-only txs and skips the
// standard fee/signature/sequence subchain.
func TestNewAnteHandlerMigrationOnlyCosmosTxUsesReducedAntePath(t *testing.T) {
	app := lumeraapp.Setup(t)

	evmtypes.SetDefaultEvmCoinInfo(evmtypes.EvmCoinInfo{
		Denom:         lcfg.ChainDenom,
		ExtendedDenom: lcfg.ChainEVMExtendedDenom,
		DisplayDenom:  lcfg.ChainDisplayDenom,
		Decimals:      evmtypes.SixDecimals.Uint32(),
	})

	anteHandler, err := appevm.NewAnteHandler(appevm.HandlerOptions{
		HandlerOptions: ante.HandlerOptions{
			AccountKeeper:          app.AuthKeeper,
			BankKeeper:             app.BankKeeper,
			FeegrantKeeper:         app.FeeGrantKeeper,
			SignModeHandler:        app.TxConfig().SignModeHandler(),
			ExtensionOptionChecker: func(*codectypes.Any) bool { return true },
		},
		IBCKeeper:             app.IBCKeeper,
		WasmConfig:            &wasmtypes.NodeConfig{},
		WasmKeeper:            app.WasmKeeper,
		TXCounterStoreService: runtime.NewKVStoreService(app.GetKey(wasmtypes.StoreKey)),
		CircuitKeeper:         &app.CircuitBreakerKeeper,
		EVMAccountKeeper:      app.AuthKeeper,
		FeeMarketKeeper:       app.FeeMarketKeeper,
		EvmKeeper:             app.EVMKeeper,
		DynamicFeeChecker:     true,
	})
	require.NoError(t, err)

	ctx := app.BaseApp.NewContext(false).
		WithIsCheckTx(true).
		WithMinGasPrices(sdk.NewDecCoins(sdk.NewDecCoin(lcfg.ChainDenom, math.NewInt(10))))

	t.Run("migration-only unsigned zero-fee tx is accepted", func(t *testing.T) {
		tx := newUnsignedMigrationTx(t, app, validMigrationMsg(t))

		_, err := anteHandler(ctx, tx, false)
		require.NoError(t, err)
	})

	t.Run("mixed tx still uses standard cosmos ante path", func(t *testing.T) {
		migrationMsg := validMigrationMsg(t)
		bankFrom := sdk.MustAccAddressFromBech32(migrationMsg.LegacyAddress)
		bankTo := sdk.MustAccAddressFromBech32(migrationMsg.NewAddress)
		tx := newUnsignedMigrationTx(
			t,
			app,
			migrationMsg,
			banktypes.NewMsgSend(bankFrom, bankTo, sdk.NewCoins(sdk.NewCoin(lcfg.ChainDenom, math.NewInt(1)))),
		)

		_, err := anteHandler(ctx, tx, false)
		require.ErrorIs(t, err, sdkerrors.ErrNoSignatures)
	})
}

func newUnsignedMigrationTx(t *testing.T, app *lumeraapp.App, msgs ...sdk.Msg) sdk.Tx {
	t.Helper()

	txBuilder := app.TxConfig().NewTxBuilder()
	require.NoError(t, txBuilder.SetMsgs(msgs...))
	txBuilder.SetGasLimit(100_000)

	return txBuilder.GetTx()
}

func validMigrationMsg(t *testing.T) *evmigrationtypes.MsgClaimLegacyAccount {
	t.Helper()

	legacy := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address().Bytes())
	newAddr := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address().Bytes())

	require.False(t, legacy.Equals(newAddr))

	return &evmigrationtypes.MsgClaimLegacyAccount{
		LegacyAddress:   legacy.String(),
		NewAddress:      newAddr.String(),
		LegacyPubKey:    make([]byte, 33),
		LegacySignature: []byte{1},
		NewPubKey:       make([]byte, 33),
		NewSignature:    []byte{1},
	}
}
