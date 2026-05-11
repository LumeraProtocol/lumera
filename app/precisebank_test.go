package app

import (
	"fmt"
	"testing"

	sdkmath "cosmossdk.io/math"
	"github.com/stretchr/testify/require"

	testaccounts "github.com/LumeraProtocol/lumera/testutil/accounts"
	sdk "github.com/cosmos/cosmos-sdk/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	precisebanktypes "github.com/cosmos/evm/x/precisebank/types"
)

// TestPreciseBankSplitAndRecomposeBalance verifies that extended-denom balances
// are correctly split across integer bank balance + fractional precisebank state
// and recomposed by GetBalance.
func TestPreciseBankSplitAndRecomposeBalance(t *testing.T) {
	app := Setup(t)
	ctx := app.BaseApp.NewContext(false)

	addr := sdk.MustAccAddressFromBech32(testaccounts.TestAddress1)

	conversionFactor := precisebanktypes.ConversionFactor()
	fractional := sdkmath.NewInt(890_123_456_789)
	extendedAmount := conversionFactor.MulRaw(1_234_567).Add(fractional)

	fundAccountWithExtendedCoin(t, app, ctx, addr, extendedAmount)

	assertSplitBalance(t, app, ctx, addr, extendedAmount)

	extendedBalance := app.PreciseBankKeeper.GetBalance(ctx, addr, precisebanktypes.ExtendedCoinDenom())
	require.True(t, extendedBalance.Amount.Equal(extendedAmount))
}

// TestPreciseBankSendExtendedCoinBorrowCarry verifies borrow/carry behavior
// when sender/recipient fractional parts cross conversion boundaries.
func TestPreciseBankSendExtendedCoinBorrowCarry(t *testing.T) {
	app := Setup(t)
	ctx := app.BaseApp.NewContext(false)

	sender := sdk.MustAccAddressFromBech32(testaccounts.TestAddress1)
	recipient := sdk.MustAccAddressFromBech32(testaccounts.TestAddress2)

	conversionFactor := precisebanktypes.ConversionFactor()
	// Sender: 2 integer units + 100 fractional units.
	senderStart := conversionFactor.MulRaw(2).AddRaw(100)
	// Recipient: (conversionFactor - 50) fractional units.
	recipientStart := conversionFactor.SubRaw(50)

	fundAccountWithExtendedCoin(t, app, ctx, sender, senderStart)
	fundAccountWithExtendedCoin(t, app, ctx, recipient, recipientStart)

	reserveAddr := app.AuthKeeper.GetModuleAddress(precisebanktypes.ModuleName)
	reserveBefore := app.BankKeeper.GetBalance(ctx, reserveAddr, precisebanktypes.IntegerCoinDenom()).Amount
	remainderBefore := app.PreciseBankKeeper.GetRemainderAmount(ctx)

	sendAmount := sdkmath.NewInt(200)
	sendCoin := sdk.NewCoin(precisebanktypes.ExtendedCoinDenom(), sendAmount)
	err := app.PreciseBankKeeper.SendCoins(ctx, sender, recipient, sdk.NewCoins(sendCoin))
	require.NoError(t, err)

	senderExpected := senderStart.Sub(sendAmount)
	recipientExpected := recipientStart.Add(sendAmount)
	assertSplitBalance(t, app, ctx, sender, senderExpected)
	assertSplitBalance(t, app, ctx, recipient, recipientExpected)

	// In sender-borrow + recipient-carry case, reserve/remainder stay unchanged.
	reserveAfter := app.BankKeeper.GetBalance(ctx, reserveAddr, precisebanktypes.IntegerCoinDenom()).Amount
	remainderAfter := app.PreciseBankKeeper.GetRemainderAmount(ctx)
	require.True(t, reserveAfter.Equal(reserveBefore))
	require.True(t, remainderAfter.Equal(remainderBefore))
}

// TestPreciseBankMintTransferBurnRestoresReserveAndRemainder verifies reserve
// and remainder bookkeeping round-trips after mint -> transfer -> burn.
func TestPreciseBankMintTransferBurnRestoresReserveAndRemainder(t *testing.T) {
	app := Setup(t)
	ctx := app.BaseApp.NewContext(false)

	conversionFactor := precisebanktypes.ConversionFactor()
	fractionalMint := sdkmath.NewInt(123_456_789_012) // strictly < conversion factor
	mintCoin := sdk.NewCoin(precisebanktypes.ExtendedCoinDenom(), fractionalMint)
	mintCoins := sdk.NewCoins(mintCoin)

	reserveAddr := app.AuthKeeper.GetModuleAddress(precisebanktypes.ModuleName)
	mintModuleAddr := app.AuthKeeper.GetModuleAddress(minttypes.ModuleName)
	govModuleAddr := app.AuthKeeper.GetModuleAddress(govtypes.ModuleName)

	reserveBefore := app.BankKeeper.GetBalance(ctx, reserveAddr, precisebanktypes.IntegerCoinDenom()).Amount
	remainderBefore := app.PreciseBankKeeper.GetRemainderAmount(ctx)

	// 1) Mint fractional-only extended coin into x/mint module.
	err := app.PreciseBankKeeper.MintCoins(ctx, minttypes.ModuleName, mintCoins)
	require.NoError(t, err)

	// Minting fractional-only amount should increase reserve by 1 integer unit.
	reserveAfterMint := app.BankKeeper.GetBalance(ctx, reserveAddr, precisebanktypes.IntegerCoinDenom()).Amount
	require.True(t, reserveAfterMint.Equal(reserveBefore.AddRaw(1)))
	remainderAfterMint := app.PreciseBankKeeper.GetRemainderAmount(ctx)
	require.True(t, remainderAfterMint.Equal(conversionFactor.Sub(fractionalMint)))

	// 2) Move minted extended amount to x/gov (burner module).
	err = app.PreciseBankKeeper.SendCoinsFromModuleToModule(ctx, minttypes.ModuleName, govtypes.ModuleName, mintCoins)
	require.NoError(t, err)

	// 3) Burn the same extended amount from x/gov.
	err = app.PreciseBankKeeper.BurnCoins(ctx, govtypes.ModuleName, mintCoins)
	require.NoError(t, err)

	// End state: reserve and remainder should be back to initial values.
	reserveAfterBurn := app.BankKeeper.GetBalance(ctx, reserveAddr, precisebanktypes.IntegerCoinDenom()).Amount
	remainderAfterBurn := app.PreciseBankKeeper.GetRemainderAmount(ctx)
	require.True(t, reserveAfterBurn.Equal(reserveBefore))
	require.True(t, remainderAfterBurn.Equal(remainderBefore))

	// And there should be no fractional residue left on x/mint or x/gov.
	require.True(t, app.PreciseBankKeeper.GetFractionalBalance(ctx, mintModuleAddr).IsZero())
	require.True(t, app.PreciseBankKeeper.GetFractionalBalance(ctx, govModuleAddr).IsZero())
}

// TestPreciseBankSendCoinsErrorParityWithBank verifies precisebank mirrors bank
// errors for invalid/insufficient SendCoins cases.
func TestPreciseBankSendCoinsErrorParityWithBank(t *testing.T) {
	app := Setup(t)
	ctx := app.BaseApp.NewContext(false)

	from := sdk.MustAccAddressFromBech32(testaccounts.TestAddress1)
	to := sdk.MustAccAddressFromBech32(testaccounts.TestAddress2)

	testCases := []struct {
		name  string    // Case name.
		coins sdk.Coins // Coins passed to SendCoins.
	}{
		{
			name: "invalid coins",
			coins: sdk.Coins{
				sdk.Coin{Denom: precisebanktypes.IntegerCoinDenom(), Amount: sdkmath.NewInt(-1)},
			},
		},
		{
			name: "insufficient funds",
			coins: sdk.NewCoins(
				sdk.NewCoin(precisebanktypes.IntegerCoinDenom(), sdkmath.NewInt(1_000)),
			),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			bankErr := app.BankKeeper.SendCoins(ctx, from, to, tc.coins)
			require.Error(t, bankErr)

			preciseErr := app.PreciseBankKeeper.SendCoins(ctx, from, to, tc.coins)
			require.Error(t, preciseErr)

			require.Equal(t, bankErr.Error(), preciseErr.Error())
		})
	}
}

// TestPreciseBankSendCoinsFromModuleToAccountBlockedRecipientParity verifies
// blocked-recipient errors remain parity-compatible with bank keeper.
func TestPreciseBankSendCoinsFromModuleToAccountBlockedRecipientParity(t *testing.T) {
	app := Setup(t)
	ctx := app.BaseApp.NewContext(false)

	senderModule := minttypes.ModuleName
	blockedRecipient := mustFindBlockedModuleAddress(t, app, ctx, senderModule, precisebanktypes.ModuleName)
	sendCoins := sdk.NewCoins(sdk.NewCoin(precisebanktypes.IntegerCoinDenom(), sdkmath.NewInt(1)))

	bankErr := app.BankKeeper.SendCoinsFromModuleToAccount(ctx, senderModule, blockedRecipient, sendCoins)
	require.Error(t, bankErr)

	preciseErr := app.PreciseBankKeeper.SendCoinsFromModuleToAccount(ctx, senderModule, blockedRecipient, sendCoins)
	require.Error(t, preciseErr)

	require.Equal(t, bankErr.Error(), preciseErr.Error())
}

// TestPreciseBankSendCoinsFromModuleToAccountMissingModulePanicParity ensures
// missing module-account panics match bank keeper behavior.
func TestPreciseBankSendCoinsFromModuleToAccountMissingModulePanicParity(t *testing.T) {
	app := Setup(t)
	ctx := app.BaseApp.NewContext(false)

	recipient := sdk.MustAccAddressFromBech32(testaccounts.TestAddress1)
	sendCoins := sdk.NewCoins(sdk.NewCoin(precisebanktypes.IntegerCoinDenom(), sdkmath.NewInt(1)))

	bankPanic := capturePanicString(func() {
		_ = app.BankKeeper.SendCoinsFromModuleToAccount(ctx, "missing-module", recipient, sendCoins)
	})
	precisePanic := capturePanicString(func() {
		_ = app.PreciseBankKeeper.SendCoinsFromModuleToAccount(ctx, "missing-module", recipient, sendCoins)
	})

	require.NotEmpty(t, bankPanic)
	require.Equal(t, bankPanic, precisePanic)
	require.Contains(t, bankPanic, "module account missing-module does not exist")
}

// TestPreciseBankSendCoinsFromAccountToModuleMissingModulePanicParity ensures
// missing recipient module panics match bank keeper behavior.
func TestPreciseBankSendCoinsFromAccountToModuleMissingModulePanicParity(t *testing.T) {
	app := Setup(t)
	ctx := app.BaseApp.NewContext(false)

	sender := sdk.MustAccAddressFromBech32(testaccounts.TestAddress1)
	sendCoins := sdk.NewCoins(sdk.NewCoin(precisebanktypes.IntegerCoinDenom(), sdkmath.NewInt(1)))

	bankPanic := capturePanicString(func() {
		_ = app.BankKeeper.SendCoinsFromAccountToModule(ctx, sender, "missing-module", sendCoins)
	})
	precisePanic := capturePanicString(func() {
		_ = app.PreciseBankKeeper.SendCoinsFromAccountToModule(ctx, sender, "missing-module", sendCoins)
	})

	require.NotEmpty(t, bankPanic)
	require.Equal(t, bankPanic, precisePanic)
	require.Contains(t, bankPanic, "module account missing-module does not exist")
}

// TestPreciseBankSendCoinsFromModuleToModuleMissingModulePanicParity verifies
// panic parity for missing sender/recipient module accounts.
func TestPreciseBankSendCoinsFromModuleToModuleMissingModulePanicParity(t *testing.T) {
	app := Setup(t)
	ctx := app.BaseApp.NewContext(false)
	sendCoins := sdk.NewCoins(sdk.NewCoin(precisebanktypes.IntegerCoinDenom(), sdkmath.NewInt(1)))

	testCases := []struct {
		name      string // Case name.
		sender    string // Sender module name.
		recipient string // Recipient module name.
	}{
		{
			name:      "missing sender module",
			sender:    "missing-sender-module",
			recipient: minttypes.ModuleName,
		},
		{
			name:      "missing recipient module",
			sender:    minttypes.ModuleName,
			recipient: "missing-recipient-module",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			bankPanic := capturePanicString(func() {
				_ = app.BankKeeper.SendCoinsFromModuleToModule(ctx, tc.sender, tc.recipient, sendCoins)
			})
			precisePanic := capturePanicString(func() {
				_ = app.PreciseBankKeeper.SendCoinsFromModuleToModule(ctx, tc.sender, tc.recipient, sendCoins)
			})

			require.NotEmpty(t, bankPanic)
			require.Equal(t, bankPanic, precisePanic)
		})
	}
}

// TestPreciseBankSendCoinsFromModuleToModuleErrorParityWithBank verifies error
// parity (non-panic paths) for module-to-module sends.
func TestPreciseBankSendCoinsFromModuleToModuleErrorParityWithBank(t *testing.T) {
	app := Setup(t)
	ctx := app.BaseApp.NewContext(false)

	testCases := []struct {
		name  string    // Case name.
		coins sdk.Coins // Coins passed to SendCoinsFromModuleToModule.
	}{
		{
			name: "invalid coins",
			coins: sdk.Coins{
				sdk.Coin{Denom: precisebanktypes.IntegerCoinDenom(), Amount: sdkmath.NewInt(-1)},
			},
		},
		{
			name: "insufficient funds",
			coins: sdk.NewCoins(
				sdk.NewCoin(precisebanktypes.IntegerCoinDenom(), sdkmath.NewInt(1_000)),
			),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			bankErr := app.BankKeeper.SendCoinsFromModuleToModule(ctx, minttypes.ModuleName, govtypes.ModuleName, tc.coins)
			require.Error(t, bankErr)

			preciseErr := app.PreciseBankKeeper.SendCoinsFromModuleToModule(ctx, minttypes.ModuleName, govtypes.ModuleName, tc.coins)
			require.Error(t, preciseErr)

			require.Equal(t, bankErr.Error(), preciseErr.Error())
		})
	}
}

// TestPreciseBankSendCoinsFromAccountToPrecisebankModuleBlocked verifies
// precisebank module account cannot receive funds from accounts.
func TestPreciseBankSendCoinsFromAccountToPrecisebankModuleBlocked(t *testing.T) {
	app := Setup(t)
	ctx := app.BaseApp.NewContext(false)

	sender := sdk.MustAccAddressFromBech32(testaccounts.TestAddress1)
	funding := sdk.NewCoins(sdk.NewCoin(precisebanktypes.IntegerCoinDenom(), sdkmath.NewInt(10)))
	require.NoError(t, app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, funding))
	require.NoError(t, app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sender, funding))

	sendCoins := sdk.NewCoins(sdk.NewCoin(precisebanktypes.IntegerCoinDenom(), sdkmath.NewInt(1)))
	err := app.PreciseBankKeeper.SendCoinsFromAccountToModule(ctx, sender, precisebanktypes.ModuleName, sendCoins)
	require.Error(t, err)
	require.ErrorContains(t, err, "module account precisebank is not allowed to receive funds")
}

// TestPreciseBankSendCoinsFromPrecisebankModuleToAccountBlocked verifies
// precisebank module account cannot send funds to accounts.
func TestPreciseBankSendCoinsFromPrecisebankModuleToAccountBlocked(t *testing.T) {
	app := Setup(t)
	ctx := app.BaseApp.NewContext(false)

	recipient := sdk.MustAccAddressFromBech32(testaccounts.TestAddress1)
	sendCoins := sdk.NewCoins(sdk.NewCoin(precisebanktypes.IntegerCoinDenom(), sdkmath.NewInt(1)))
	err := app.PreciseBankKeeper.SendCoinsFromModuleToAccount(ctx, precisebanktypes.ModuleName, recipient, sendCoins)
	require.Error(t, err)
	require.ErrorContains(t, err, "module account precisebank is not allowed to send funds")
}

// TestPreciseBankMintCoinsToPrecisebankModulePanic verifies minting directly
// to precisebank module account panics.
func TestPreciseBankMintCoinsToPrecisebankModulePanic(t *testing.T) {
	app := Setup(t)
	ctx := app.BaseApp.NewContext(false)

	mintCoins := sdk.NewCoins(sdk.NewCoin(precisebanktypes.IntegerCoinDenom(), sdkmath.NewInt(1)))
	panicText := capturePanicString(func() {
		_ = app.PreciseBankKeeper.MintCoins(ctx, precisebanktypes.ModuleName, mintCoins)
	})

	require.NotEmpty(t, panicText)
	require.Contains(t, panicText, "module account precisebank cannot be minted to")
}

// TestPreciseBankBurnCoinsFromPrecisebankModulePanic verifies burning directly
// from precisebank module account panics.
func TestPreciseBankBurnCoinsFromPrecisebankModulePanic(t *testing.T) {
	app := Setup(t)
	ctx := app.BaseApp.NewContext(false)

	burnCoins := sdk.NewCoins(sdk.NewCoin(precisebanktypes.IntegerCoinDenom(), sdkmath.NewInt(1)))
	panicText := capturePanicString(func() {
		_ = app.PreciseBankKeeper.BurnCoins(ctx, precisebanktypes.ModuleName, burnCoins)
	})

	require.NotEmpty(t, panicText)
	require.Contains(t, panicText, "module account precisebank cannot be burned from")
}

// TestPreciseBankRemainderAmountLifecycle verifies set/get/delete lifecycle for
// remainder storage key and zero-value behavior.
func TestPreciseBankRemainderAmountLifecycle(t *testing.T) {
	app := Setup(t)
	ctx := app.BaseApp.NewContext(false)

	require.True(t, app.PreciseBankKeeper.GetRemainderAmount(ctx).IsZero())

	app.PreciseBankKeeper.SetRemainderAmount(ctx, sdkmath.NewInt(100))
	require.True(t, app.PreciseBankKeeper.GetRemainderAmount(ctx).Equal(sdkmath.NewInt(100)))

	app.PreciseBankKeeper.SetRemainderAmount(ctx, sdkmath.ZeroInt())
	require.True(t, app.PreciseBankKeeper.GetRemainderAmount(ctx).IsZero())

	store := ctx.KVStore(app.GetKey(precisebanktypes.StoreKey))
	require.Nil(t, store.Get(precisebanktypes.RemainderBalanceKey))

	app.PreciseBankKeeper.SetRemainderAmount(ctx, sdkmath.NewInt(321))
	require.True(t, app.PreciseBankKeeper.GetRemainderAmount(ctx).Equal(sdkmath.NewInt(321)))
	app.PreciseBankKeeper.DeleteRemainderAmount(ctx)
	require.True(t, app.PreciseBankKeeper.GetRemainderAmount(ctx).IsZero())
	require.Nil(t, store.Get(precisebanktypes.RemainderBalanceKey))
}

// TestPreciseBankInvalidRemainderAmountPanics validates remainder invariants:
// non-negative and strictly less than conversion factor.
func TestPreciseBankInvalidRemainderAmountPanics(t *testing.T) {
	app := Setup(t)
	ctx := app.BaseApp.NewContext(false)

	panicNegative := capturePanicString(func() {
		app.PreciseBankKeeper.SetRemainderAmount(ctx, sdkmath.NewInt(-1))
	})
	require.Contains(t, panicNegative, "remainder amount is invalid")

	panicOverflow := capturePanicString(func() {
		app.PreciseBankKeeper.SetRemainderAmount(ctx, precisebanktypes.ConversionFactor())
	})
	require.Contains(t, panicOverflow, "remainder amount is invalid")
}

// TestPreciseBankReserveAddressHiddenForExtendedDenom verifies reserve module
// address reports zero for extended denom while preserving integer balances.
func TestPreciseBankReserveAddressHiddenForExtendedDenom(t *testing.T) {
	app := Setup(t)
	ctx := app.BaseApp.NewContext(false)

	reserveAddr := app.AuthKeeper.GetModuleAddress(precisebanktypes.ModuleName)
	require.NotNil(t, reserveAddr)

	// Populate reserve balances so we can assert only ExtendedCoinDenom is hidden.
	require.NoError(t, app.BankKeeper.MintCoins(
		ctx,
		precisebanktypes.ModuleName,
		sdk.NewCoins(sdk.NewCoin(precisebanktypes.IntegerCoinDenom(), sdkmath.NewInt(2))),
	))
	app.PreciseBankKeeper.SetFractionalBalance(ctx, reserveAddr, sdkmath.NewInt(123))

	extended := app.PreciseBankKeeper.GetBalance(ctx, reserveAddr, precisebanktypes.ExtendedCoinDenom())
	require.Equal(t, precisebanktypes.ExtendedCoinDenom(), extended.Denom)
	require.True(t, extended.Amount.IsZero())

	spendableExtended := app.PreciseBankKeeper.SpendableCoin(ctx, reserveAddr, precisebanktypes.ExtendedCoinDenom())
	require.Equal(t, precisebanktypes.ExtendedCoinDenom(), spendableExtended.Denom)
	require.True(t, spendableExtended.Amount.IsZero())

	integerBal := app.PreciseBankKeeper.GetBalance(ctx, reserveAddr, precisebanktypes.IntegerCoinDenom())
	require.True(t, integerBal.Amount.Equal(sdkmath.NewInt(2)))
}

// TestPreciseBankGetBalanceAndSpendableCoin verifies denom-specific balance
// behavior for extended/integer/other denoms with fractional state.
func TestPreciseBankGetBalanceAndSpendableCoin(t *testing.T) {
	testCases := []struct {
		name           string      // Case name.
		denomKind      string      // Which denom is queried: extended/integer/other.
		integerBalance sdkmath.Int // Initial integer bank balance.
		fractional     sdkmath.Int // Initial precisebank fractional balance.
		otherDenom     string      // Optional unrelated denom.
		otherDenomBal  sdkmath.Int // Balance for unrelated denom.
		expectedKind   string      // Expected resolution mode in assertion switch.
		expectedValue  sdkmath.Int // Optional direct expected value (used by some cases).
	}{
		{
			name:           "extended denom with integer and fractional",
			denomKind:      "extended",
			integerBalance: sdkmath.NewInt(5),
			fractional:     sdkmath.NewInt(321),
			expectedKind:   "extended",
			expectedValue:  sdkmath.NewInt(0), // computed after Setup
		},
		{
			name:           "extended denom only fractional",
			denomKind:      "extended",
			integerBalance: sdkmath.ZeroInt(),
			fractional:     sdkmath.NewInt(777),
			expectedKind:   "fractional-only",
			expectedValue:  sdkmath.NewInt(777),
		},
		{
			name:           "integer denom passthrough",
			denomKind:      "integer",
			integerBalance: sdkmath.NewInt(42),
			fractional:     sdkmath.NewInt(999),
			expectedKind:   "integer",
			expectedValue:  sdkmath.NewInt(42),
		},
		{
			name:           "unrelated denom passthrough",
			denomKind:      "other",
			integerBalance: sdkmath.NewInt(7),
			fractional:     sdkmath.NewInt(555),
			otherDenom:     "utest",
			otherDenomBal:  sdkmath.NewInt(1234),
			expectedKind:   "other",
			expectedValue:  sdkmath.NewInt(1234),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			app := Setup(t)
			ctx := app.BaseApp.NewContext(false)
			addr := sdk.MustAccAddressFromBech32(testaccounts.TestAddress1)
			extendedDenom := precisebanktypes.ExtendedCoinDenom()
			integerDenom := precisebanktypes.IntegerCoinDenom()
			conversionFactor := precisebanktypes.ConversionFactor()

			denom := tc.otherDenom
			switch tc.denomKind {
			case "extended":
				denom = extendedDenom
			case "integer":
				denom = integerDenom
			}

			if tc.integerBalance.IsPositive() {
				intCoins := sdk.NewCoins(sdk.NewCoin(integerDenom, tc.integerBalance))
				require.NoError(t, app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, intCoins))
				require.NoError(t, app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, addr, intCoins))
			}
			if tc.otherDenom != "" && tc.otherDenomBal.IsPositive() {
				otherCoins := sdk.NewCoins(sdk.NewCoin(tc.otherDenom, tc.otherDenomBal))
				require.NoError(t, app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, otherCoins))
				require.NoError(t, app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, addr, otherCoins))
			}
			if tc.fractional.IsPositive() {
				app.PreciseBankKeeper.SetFractionalBalance(ctx, addr, tc.fractional)
			}

			expectedBalance := tc.expectedValue
			switch tc.expectedKind {
			case "extended":
				expectedBalance = conversionFactor.Mul(tc.integerBalance).Add(tc.fractional)
			case "fractional-only":
				expectedBalance = tc.fractional
			case "integer":
				expectedBalance = tc.integerBalance
			case "other":
				expectedBalance = tc.otherDenomBal
			}

			getBal := app.PreciseBankKeeper.GetBalance(ctx, addr, denom)
			require.Equal(t, denom, getBal.Denom)
			require.True(t, getBal.Amount.Equal(expectedBalance))

			spendable := app.PreciseBankKeeper.SpendableCoin(ctx, addr, denom)
			require.Equal(t, denom, spendable.Denom)
			require.True(t, spendable.Amount.Equal(expectedBalance))
		})
	}
}

// fundAccountWithExtendedCoin mints extended-denom coins to x/mint and
// transfers them to recipient through precisebank keeper logic.
func fundAccountWithExtendedCoin(t *testing.T, app *App, ctx sdk.Context, recipient sdk.AccAddress, amount sdkmath.Int) {
	t.Helper()

	coin := sdk.NewCoin(precisebanktypes.ExtendedCoinDenom(), amount)
	coins := sdk.NewCoins(coin)

	err := app.PreciseBankKeeper.MintCoins(ctx, minttypes.ModuleName, coins)
	require.NoError(t, err)

	err = app.PreciseBankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, recipient, coins)
	require.NoError(t, err)
}

// assertSplitBalance verifies integer/fractional decomposition and recomposition
// for an expected extended-denom amount.
func assertSplitBalance(t *testing.T, app *App, ctx sdk.Context, addr sdk.AccAddress, extendedAmount sdkmath.Int) {
	t.Helper()

	conversionFactor := precisebanktypes.ConversionFactor()
	expectedInteger := extendedAmount.Quo(conversionFactor)
	expectedFractional := extendedAmount.Mod(conversionFactor)

	bankBalance := app.BankKeeper.GetBalance(ctx, addr, precisebanktypes.IntegerCoinDenom())
	require.True(t, bankBalance.Amount.Equal(expectedInteger))

	fractionalBalance := app.PreciseBankKeeper.GetFractionalBalance(ctx, addr)
	require.True(t, fractionalBalance.Equal(expectedFractional))

	recomposed := bankBalance.Amount.Mul(conversionFactor).Add(fractionalBalance)
	require.True(t, recomposed.Equal(extendedAmount))
}

// mustFindBlockedModuleAddress returns any blocked module account address while
// excluding explicitly provided module names.
func mustFindBlockedModuleAddress(t *testing.T, app *App, ctx sdk.Context, excludedModules ...string) sdk.AccAddress {
	t.Helper()

	excluded := map[string]struct{}{}
	for _, module := range excludedModules {
		excluded[module] = struct{}{}
	}

	for module := range GetMaccPerms() {
		if _, skip := excluded[module]; skip {
			continue
		}
		addr := app.AuthKeeper.GetModuleAddress(module)
		if addr == nil {
			continue
		}
		if app.BankKeeper.BlockedAddr(addr) {
			return addr
		}
	}

	t.Fatal("failed to find blocked module address for parity test")
	return nil
}

// capturePanicString executes fn and returns recovered panic text (if any).
func capturePanicString(fn func()) (panicText string) {
	defer func() {
		if r := recover(); r != nil {
			panicText = fmt.Sprint(r)
		}
	}()
	fn()
	return panicText
}
