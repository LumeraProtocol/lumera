package evm_test

import (
	"testing"

	"cosmossdk.io/math"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
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

	// SetUpContextDecorator (the SDK gas-meter setup) compares tx gas against
	// consensusParams.Block.MaxGas; an empty/zero value rejects the tx with
	// "tx gas exceeds block gas limit" before EVMigrationValidateBasicDecorator
	// can run. Set a nonzero block gas limit so the migration-only path is
	// what's actually under test.
	ctx := app.BaseApp.NewContext(false).
		WithIsCheckTx(true).
		WithMinGasPrices(sdk.NewDecCoins(sdk.NewDecCoin(lcfg.ChainDenom, math.NewInt(10)))).
		WithConsensusParams(tmproto.ConsensusParams{
			Block: &tmproto.BlockParams{
				MaxGas:   100_000_000,
				MaxBytes: 22020096,
			},
		})

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

// validMigrationMsg builds a MsgClaimLegacyAccount whose proofs pass per-side
// MigrationProof.validateBasic and the cross-side ValidateProofPair (matching
// single-key shape on both sides). Bytes are placeholders — the ante chain's
// EVMigrationValidateBasicDecorator only runs ValidateBasic; signature
// cryptography is verified later in the msg server, which this test does not
// reach. The destination pubkey is filled with a non-zero pattern so the
// SingleKeyProof "expected 33 bytes" length check is satisfied; using zero
// bytes there would still pass length but is less obvious as a deliberate
// stub.
func validMigrationMsg(t *testing.T) *evmigrationtypes.MsgClaimLegacyAccount {
	t.Helper()

	legacy := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address().Bytes())
	newAddr := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address().Bytes())

	require.False(t, legacy.Equals(newAddr))

	// Legacy side: 33-byte secp256k1 compressed pubkey + 64-byte raw R||S sig.
	legacyPubKey := make([]byte, 33)
	legacyPubKey[0] = 0x02
	legacySig := make([]byte, 64)

	// New side: 33-byte secp256k1 compressed pubkey + 65-byte R||S||V sig.
	newPubKey := make([]byte, 33)
	newPubKey[0] = 0x03
	newSig := make([]byte, 65)

	return &evmigrationtypes.MsgClaimLegacyAccount{
		LegacyAddress: legacy.String(),
		NewAddress:    newAddr.String(),
		LegacyProof: evmigrationtypes.MigrationProof{Proof: &evmigrationtypes.MigrationProof_Single{Single: &evmigrationtypes.SingleKeyProof{
			PubKey:    legacyPubKey,
			Signature: legacySig,
			SigFormat: evmigrationtypes.SigFormat_SIG_FORMAT_CLI,
		}}},
		NewProof: evmigrationtypes.MigrationProof{Proof: &evmigrationtypes.MigrationProof_Single{Single: &evmigrationtypes.SingleKeyProof{
			PubKey:    newPubKey,
			Signature: newSig,
			SigFormat: evmigrationtypes.SigFormat_SIG_FORMAT_CLI,
		}}},
	}
}
