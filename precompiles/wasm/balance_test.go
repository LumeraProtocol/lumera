package wasm_test

import (
	"testing"

	sdkmath "cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	sdktestutil "github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"

	cmn "github.com/cosmos/evm/precompiles/common"
	cmnmocks "github.com/cosmos/evm/precompiles/common/mocks"
	"github.com/cosmos/evm/x/vm/statedb"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	evmmocks "github.com/cosmos/evm/x/vm/types/mocks"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	wasmprecompile "github.com/LumeraProtocol/lumera/precompiles/wasm"
)

// newMockBankKeeper returns a mock cmn.BankKeeper whose blocked set is empty.
// Only BlockedAddr is exercised by the BalanceHandler; any other call fails
// the test, so the harness cannot silently grow dependencies on real bank
// state.
func newMockBankKeeper(t *testing.T) *cmnmocks.BankKeeper {
	t.Helper()
	bk := cmnmocks.NewBankKeeper(t)
	bk.Mock.On("BlockedAddr", mock.AnythingOfType("types.AccAddress")).Return(false).Maybe()
	return bk
}

// wasmContractAddr is a realistic 32-byte CosmWasm contract address.
// evmRecipientAddr is a normal 20-byte EVM-projectable account.
// Lengths are asserted in init so a typo in a literal cannot silently change
// which branch of the replay the test exercises.
var (
	wasmContractAddr = sdk.AccAddress([]byte("wasm-contract-address-32-bytes!!")) // 32 bytes
	evmRecipientAddr = sdk.AccAddress([]byte("evm-recipient-20byte"))             // 20 bytes
)

func init() {
	if len(wasmContractAddr) != 32 {
		panic("wasmContractAddr must be 32 bytes")
	}
	if len(evmRecipientAddr) != 20 {
		panic("evmRecipientAddr must be 20 bytes")
	}

	// sdk.Config bech32 prefix setup mirrors app config; use lumera prefix.
	cfg := sdk.GetConfig()
	cfg.SetBech32PrefixForAccount("lumera", "lumerapub")
	// Bank balance events carry the display (6-decimal) denom; the balance
	// handler parses them with the EVM coin denom. Match the app wiring:
	// EVM coin denom is ulume (6 decimals) with alume as the extended denom.
	evmtypes.SetDefaultEvmCoinInfo(evmtypes.EvmCoinInfo{
		Denom:         "ulume",
		ExtendedDenom: "alume",
		DisplayDenom:  "lume",
		Decimals:      evmtypes.SixDecimals.Uint32(),
	})
}

// replayBankEvents drives the exact replay loop used by
// cmn.Precompile.runNativeAction for a precompile whose balance handler
// factory is `factory`: BeforeBalanceChange, emit events, AfterBalanceChange.
// It returns the resulting StateDB so tests can assert balances.
func replayBankEvents(t *testing.T, factory *cmn.BalanceHandlerFactory, events sdk.Events) *statedb.StateDB {
	t.Helper()
	storeKey := storetypes.NewKVStoreKey("test")
	tKey := storetypes.NewTransientStoreKey("test_t")
	ctx := sdktestutil.DefaultContext(storeKey, tKey)
	stateDB := statedb.New(ctx, evmmocks.NewEVMKeeper(), statedb.NewEmptyTxConfig())

	var bh *cmn.BalanceHandler
	if factory != nil {
		bh = factory.NewBalanceHandler()
	}
	if bh != nil {
		bh.BeforeBalanceChange(ctx)
	}
	ctx.EventManager().EmitEvents(events)
	if bh != nil {
		require.NoError(t, bh.AfterBalanceChange(ctx, stateDB))
	}
	return stateDB
}

func wasmPayoutEvents() sdk.Events {
	// Real production event pair emitted by x/bank when a wasm contract with a
	// 32-byte address sends funds to a 20-byte EVM recipient.
	amount := sdk.NewCoins(sdk.NewCoin("ulume", sdkmath.NewInt(111)))
	return sdk.Events{
		banktypes.NewCoinSpentEvent(wasmContractAddr, amount),
		banktypes.NewCoinReceivedEvent(evmRecipientAddr, amount),
	}
}

// TestWasmBalanceReplayDoesNotMintOrDoubleApply is the regression test for the
// EVM→Wasm accounting defect: a wasm contract (32-byte address) sends N to a
// 20-byte EVM account. The native bank transfer is the single source of
// truth; the precompile must install NO balance handler, so the StateDB must
// not:
//   - mint into the truncated EVM alias of the wasm contract, nor
//   - double-apply the transfer by adding N to the recipient's EVM balance
//     (the bank keeper already moved the funds, and SetBalance reconciles the
//     journal target against a bank view that does not include the in-flight
//     native transfer, minting the replayed amount a second time).
//
// The assertions build the replay through the PRODUCTION wiring helper
// (wasmprecompile.NewBalanceHandlerFactory), so restoring a balance handler
// in production wiring fails these tests.
func TestWasmBalanceReplayDoesNotMintOrDoubleApply(t *testing.T) {
	events := wasmPayoutEvents()

	t.Run("production wiring installs no balance handler", func(t *testing.T) {
		require.Nil(t, wasmprecompile.NewBalanceHandlerFactory(),
			"wasm precompile must not mirror native bank events into the EVM StateDB")
	})

	t.Run("does not mutate the truncated alias of a 32-byte wasm address", func(t *testing.T) {
		stateDB := replayBankEvents(t, wasmprecompile.NewBalanceHandlerFactory(), events)
		truncated := ethcommon.BytesToAddress(wasmContractAddr.Bytes())
		require.Equal(t, "0", stateDB.GetBalance(truncated).String(),
			"replay must not mutate the truncated alias of a 32-byte wasm contract address")
	})

	t.Run("does not double-apply the recipient credit", func(t *testing.T) {
		stateDB := replayBankEvents(t, wasmprecompile.NewBalanceHandlerFactory(), events)
		require.Equal(t, "0", stateDB.GetBalance(ethcommon.BytesToAddress(evmRecipientAddr.Bytes())).String(),
			"replay must not double-apply the native bank transfer to the recipient")
	})

	t.Run("unfiltered upstream replay wraps the truncated alias and double-mints (defect demonstration)", func(t *testing.T) {
		// Pins the vulnerable behavior of the exact pre-fix production wiring:
		// upstream BalanceHandlerFactory over an unfiltered bank keeper. This
		// keeps the RED evidence in-repo and guards the assertions above from
		// passing vacuously (e.g. if events stopped being parsed).
		stateDB := replayBankEvents(t, cmn.NewBalanceHandlerFactory(newMockBankKeeper(t)), events)

		truncated := ethcommon.BytesToAddress(wasmContractAddr.Bytes())
		aliasBal := stateDB.GetBalance(truncated).ToBig()
		require.True(t, aliasBal.Sign() > 0,
			"unfiltered replay is expected to mint into the truncated alias (this is the defect)")
		require.True(t, aliasBal.BitLen() > 200,
			"wrapped alias balance must be astronomically large, got %s", aliasBal)

		recipientBal := stateDB.GetBalance(ethcommon.BytesToAddress(evmRecipientAddr.Bytes())).ToBig()
		require.Equal(t, "111000000000000", recipientBal.String(),
			"unfiltered replay adds the amount on top of the native bank transfer (second mint at reconciliation)")
	})
}

// TestWasmBalanceReplayKeepsEVMAccountBehavior documents that legitimate
// EVM-native precompiles (staking, distribution, ...) keep their upstream
// balance handler; this test builds the upstream handler directly, matching
// those precompiles' wiring (they are unchanged by this fix).
func TestWasmBalanceReplayKeepsEVMAccountBehavior(t *testing.T) {
	spender := sdk.AccAddress([]byte("evm-spender-20-byte!"))
	receiver := sdk.AccAddress([]byte("evm-receiver-20-byte"))
	if len(spender) != 20 || len(receiver) != 20 {
		t.Fatalf("test literals must be exactly 20 bytes, got %d and %d", len(spender), len(receiver))
	}

	storeKey := storetypes.NewKVStoreKey("test")
	tKey := storetypes.NewTransientStoreKey("test_t")
	ctx := sdktestutil.DefaultContext(storeKey, tKey)
	stateDB := statedb.New(ctx, evmmocks.NewEVMKeeper(), statedb.NewEmptyTxConfig())
	bh := cmn.NewBalanceHandlerFactory(newMockBankKeeper(t)).NewBalanceHandler()
	bh.BeforeBalanceChange(ctx)

	amount := sdk.NewCoins(sdk.NewCoin("ulume", sdkmath.NewInt(5)))
	// 5 ulume == 5e12 at 18 decimals; seed the spender above that so the
	// legitimate path does not depend on unsigned wrap behavior.
	seed := new(uint256.Int).Mul(uint256.NewInt(10), uint256.NewInt(1_000_000_000_000))
	stateDB.AddBalance(ethcommon.BytesToAddress(spender.Bytes()), seed, tracing.BalanceChangeUnspecified)

	ctx.EventManager().EmitEvents(sdk.Events{
		banktypes.NewCoinSpentEvent(spender, amount),
		banktypes.NewCoinReceivedEvent(receiver, amount),
	})

	require.NoError(t, bh.AfterBalanceChange(ctx, stateDB))
	require.Equal(t, "5000000000000", stateDB.GetBalance(ethcommon.BytesToAddress(spender.Bytes())).String())
	require.Equal(t, "5000000000000", stateDB.GetBalance(ethcommon.BytesToAddress(receiver.Bytes())).String())
}
