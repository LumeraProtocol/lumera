package app

import (
	"testing"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	precisebanktypes "github.com/cosmos/evm/x/precisebank/types"
	"github.com/stretchr/testify/require"
)

// TestPreciseBankMintCoinsPermissionMatrix verifies mint permission handling:
// modules without minter permission are rejected, and valid minter modules pass.
func TestPreciseBankMintCoinsPermissionMatrix(t *testing.T) {
	testCases := []struct {
		name        string
		moduleName  string
		expectPanic string
	}{
		{
			name:        "rejects module without minter permission",
			moduleName:  feemarkettypes.ModuleName, // no module permissions
			expectPanic: "does not have permissions to mint tokens",
		},
		{
			name:       "allows module with minter permission",
			moduleName: minttypes.ModuleName,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			app := Setup(t)
			ctx := app.BaseApp.NewContext(false)

			mintCoins := sdk.NewCoins(sdk.NewCoin(precisebanktypes.IntegerCoinDenom(), sdkmath.NewInt(1)))
			if tc.expectPanic != "" {
				panicText := capturePanicString(func() {
					_ = app.PreciseBankKeeper.MintCoins(ctx, tc.moduleName, mintCoins)
				})
				require.Contains(t, panicText, tc.expectPanic)
				return
			}

			require.NoError(t, app.PreciseBankKeeper.MintCoins(ctx, tc.moduleName, mintCoins))
			moduleAddr := app.AuthKeeper.GetModuleAddress(tc.moduleName)
			require.True(t, app.BankKeeper.GetBalance(ctx, moduleAddr, precisebanktypes.IntegerCoinDenom()).Amount.Equal(sdkmath.OneInt()))
		})
	}
}

// TestPreciseBankBurnCoinsPermissionMatrix verifies burn permission handling:
// modules without burner permission are rejected, and valid burner modules pass.
func TestPreciseBankBurnCoinsPermissionMatrix(t *testing.T) {
	testCases := []struct {
		name        string
		moduleName  string
		expectPanic string
	}{
		{
			name:        "rejects module without burner permission",
			moduleName:  minttypes.ModuleName, // minter only
			expectPanic: "does not have permissions to burn tokens",
		},
		{
			name:       "allows module with burner permission",
			moduleName: govtypes.ModuleName,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			app := Setup(t)
			ctx := app.BaseApp.NewContext(false)

			coin := sdk.NewCoin(precisebanktypes.IntegerCoinDenom(), sdkmath.NewInt(1))
			coins := sdk.NewCoins(coin)

			// Fund target module from x/mint so burn tests have balance.
			require.NoError(t, app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, coins))
			require.NoError(t, app.BankKeeper.SendCoinsFromModuleToModule(ctx, minttypes.ModuleName, tc.moduleName, coins))

			if tc.expectPanic != "" {
				panicText := capturePanicString(func() {
					_ = app.PreciseBankKeeper.BurnCoins(ctx, tc.moduleName, coins)
				})
				require.Contains(t, panicText, tc.expectPanic)
				return
			}

			require.NoError(t, app.PreciseBankKeeper.BurnCoins(ctx, tc.moduleName, coins))
			moduleAddr := app.AuthKeeper.GetModuleAddress(tc.moduleName)
			require.True(t, app.BankKeeper.GetBalance(ctx, moduleAddr, precisebanktypes.IntegerCoinDenom()).Amount.IsZero())
		})
	}
}

// TestPreciseBankMintExtendedCoinStateTransitions verifies representative
// extended-denom mint transitions for carry/remainder/reserve accounting.
func TestPreciseBankMintExtendedCoinStateTransitions(t *testing.T) {
	_ = Setup(t)
	cf := precisebanktypes.ConversionFactor()

	testCases := []struct {
		name                     string
		startFractional          sdkmath.Int
		startRemainder           sdkmath.Int
		mintAmount               sdkmath.Int
		expectedModuleIntDelta   sdkmath.Int
		expectedModuleFractional sdkmath.Int
		expectedReserveIntDelta  sdkmath.Int
		expectedRemainder        sdkmath.Int
	}{
		{
			name:                     "no carry, reserve mint needed",
			startFractional:          sdkmath.ZeroInt(),
			startRemainder:           sdkmath.ZeroInt(),
			mintAmount:               sdkmath.NewInt(1000),
			expectedModuleIntDelta:   sdkmath.ZeroInt(),
			expectedModuleFractional: sdkmath.NewInt(1000),
			expectedReserveIntDelta:  sdkmath.OneInt(),
			expectedRemainder:        cf.Sub(sdkmath.NewInt(1000)),
		},
		{
			name:                     "carry with insufficient remainder uses optimized direct integer mint",
			startFractional:          cf.SubRaw(1),
			startRemainder:           sdkmath.ZeroInt(),
			mintAmount:               sdkmath.OneInt(),
			expectedModuleIntDelta:   sdkmath.OneInt(),
			expectedModuleFractional: sdkmath.ZeroInt(),
			expectedReserveIntDelta:  sdkmath.ZeroInt(),
			expectedRemainder:        cf.SubRaw(1),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			app := Setup(t)
			ctx := app.BaseApp.NewContext(false)

			moduleAddr := app.AuthKeeper.GetModuleAddress(minttypes.ModuleName)
			reserveAddr := app.AuthKeeper.GetModuleAddress(precisebanktypes.ModuleName)

			if tc.startFractional.IsPositive() {
				app.PreciseBankKeeper.SetFractionalBalance(ctx, moduleAddr, tc.startFractional)
			}
			app.PreciseBankKeeper.SetRemainderAmount(ctx, tc.startRemainder)

			moduleIntBefore := app.BankKeeper.GetBalance(ctx, moduleAddr, precisebanktypes.IntegerCoinDenom()).Amount
			reserveIntBefore := app.BankKeeper.GetBalance(ctx, reserveAddr, precisebanktypes.IntegerCoinDenom()).Amount

			mintCoins := sdk.NewCoins(sdk.NewCoin(precisebanktypes.ExtendedCoinDenom(), tc.mintAmount))
			require.NoError(t, app.PreciseBankKeeper.MintCoins(ctx, minttypes.ModuleName, mintCoins))

			moduleIntAfter := app.BankKeeper.GetBalance(ctx, moduleAddr, precisebanktypes.IntegerCoinDenom()).Amount
			moduleFracAfter := app.PreciseBankKeeper.GetFractionalBalance(ctx, moduleAddr)
			reserveIntAfter := app.BankKeeper.GetBalance(ctx, reserveAddr, precisebanktypes.IntegerCoinDenom()).Amount
			remainderAfter := app.PreciseBankKeeper.GetRemainderAmount(ctx)

			require.True(t, moduleIntAfter.Sub(moduleIntBefore).Equal(tc.expectedModuleIntDelta))
			require.True(t, moduleFracAfter.Equal(tc.expectedModuleFractional))
			require.True(t, reserveIntAfter.Sub(reserveIntBefore).Equal(tc.expectedReserveIntDelta))
			require.True(t, remainderAfter.Equal(tc.expectedRemainder))
		})
	}
}

// TestPreciseBankBurnExtendedCoinStateTransitions verifies representative
// extended-denom burn transitions for borrow and remainder-overflow paths.
func TestPreciseBankBurnExtendedCoinStateTransitions(t *testing.T) {
	_ = Setup(t)
	cf := precisebanktypes.ConversionFactor()

	testCases := []struct {
		name                     string
		startModuleInt           sdkmath.Int
		startFractional          sdkmath.Int
		startRemainder           sdkmath.Int
		startReserveInt          sdkmath.Int
		burnAmount               sdkmath.Int
		expectedModuleInt        sdkmath.Int
		expectedModuleFractional sdkmath.Int
		expectedReserveIntDelta  sdkmath.Int
		expectedRemainder        sdkmath.Int
	}{
		{
			name:                     "borrow from integer to cover fractional burn",
			startModuleInt:           sdkmath.OneInt(),
			startFractional:          sdkmath.NewInt(100),
			startRemainder:           sdkmath.ZeroInt(),
			startReserveInt:          sdkmath.ZeroInt(),
			burnAmount:               sdkmath.NewInt(200),
			expectedModuleInt:        sdkmath.ZeroInt(),
			expectedModuleFractional: cf.Sub(sdkmath.NewInt(100)),
			expectedReserveIntDelta:  sdkmath.OneInt(),
			expectedRemainder:        sdkmath.NewInt(200),
		},
		{
			name:                     "borrow plus remainder overflow burns directly (optimized path)",
			startModuleInt:           sdkmath.OneInt(),
			startFractional:          sdkmath.NewInt(100),
			startRemainder:           cf.Sub(sdkmath.NewInt(100)),
			startReserveInt:          sdkmath.ZeroInt(),
			burnAmount:               sdkmath.NewInt(200),
			expectedModuleInt:        sdkmath.ZeroInt(),
			expectedModuleFractional: cf.Sub(sdkmath.NewInt(100)),
			expectedReserveIntDelta:  sdkmath.ZeroInt(),
			expectedRemainder:        sdkmath.NewInt(100),
		},
		{
			name:                     "no borrow with overflowing remainder burns one reserve integer",
			startModuleInt:           sdkmath.OneInt(),
			startFractional:          sdkmath.NewInt(500),
			startRemainder:           cf.Sub(sdkmath.NewInt(100)),
			startReserveInt:          sdkmath.OneInt(),
			burnAmount:               sdkmath.NewInt(50),
			expectedModuleInt:        sdkmath.OneInt(),
			expectedModuleFractional: sdkmath.NewInt(450),
			expectedReserveIntDelta:  sdkmath.NewInt(-1),
			expectedRemainder:        sdkmath.NewInt(50),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			app := Setup(t)
			ctx := app.BaseApp.NewContext(false)

			moduleAddr := app.AuthKeeper.GetModuleAddress(minttypes.ModuleName)
			reserveAddr := app.AuthKeeper.GetModuleAddress(precisebanktypes.ModuleName)

			if tc.startModuleInt.IsPositive() {
				require.NoError(t, app.BankKeeper.MintCoins(
					ctx,
					minttypes.ModuleName,
					sdk.NewCoins(sdk.NewCoin(precisebanktypes.IntegerCoinDenom(), tc.startModuleInt)),
				))
			}
			if tc.startReserveInt.IsPositive() {
				require.NoError(t, app.BankKeeper.MintCoins(
					ctx,
					precisebanktypes.ModuleName,
					sdk.NewCoins(sdk.NewCoin(precisebanktypes.IntegerCoinDenom(), tc.startReserveInt)),
				))
			}
			if tc.startFractional.IsPositive() {
				app.PreciseBankKeeper.SetFractionalBalance(ctx, moduleAddr, tc.startFractional)
			}
			app.PreciseBankKeeper.SetRemainderAmount(ctx, tc.startRemainder)

			reserveIntBefore := app.BankKeeper.GetBalance(ctx, reserveAddr, precisebanktypes.IntegerCoinDenom()).Amount

			burnCoins := sdk.NewCoins(sdk.NewCoin(precisebanktypes.ExtendedCoinDenom(), tc.burnAmount))
			require.NoError(t, app.PreciseBankKeeper.BurnCoins(ctx, minttypes.ModuleName, burnCoins))

			moduleIntAfter := app.BankKeeper.GetBalance(ctx, moduleAddr, precisebanktypes.IntegerCoinDenom()).Amount
			moduleFracAfter := app.PreciseBankKeeper.GetFractionalBalance(ctx, moduleAddr)
			reserveIntAfter := app.BankKeeper.GetBalance(ctx, reserveAddr, precisebanktypes.IntegerCoinDenom()).Amount
			remainderAfter := app.PreciseBankKeeper.GetRemainderAmount(ctx)

			require.True(t, moduleIntAfter.Equal(tc.expectedModuleInt))
			require.True(t, moduleFracAfter.Equal(tc.expectedModuleFractional))
			require.True(t, reserveIntAfter.Sub(reserveIntBefore).Equal(tc.expectedReserveIntDelta))
			require.True(t, remainderAfter.Equal(tc.expectedRemainder))
		})
	}
}

// TestPreciseBankMintCoinsStateMatrix verifies mint transitions across
// passthrough, carry, and reserve/remainder accounting scenarios.
func TestPreciseBankMintCoinsStateMatrix(t *testing.T) {
	_ = Setup(t)
	cf := precisebanktypes.ConversionFactor()

	testCases := []struct {
		name                     string
		startFractional          sdkmath.Int
		startRemainder           sdkmath.Int
		mintCoins                sdk.Coins
		expectedModuleIntDelta   sdkmath.Int
		expectedModuleFractional sdkmath.Int
		expectedReserveIntDelta  sdkmath.Int
		expectedRemainder        sdkmath.Int
		expectedMeowBalance      sdkmath.Int
	}{
		{
			name:                     "passthrough integer denom",
			startFractional:          sdkmath.ZeroInt(),
			startRemainder:           sdkmath.ZeroInt(),
			mintCoins:                sdk.NewCoins(sdk.NewCoin(precisebanktypes.IntegerCoinDenom(), sdkmath.NewInt(1000))),
			expectedModuleIntDelta:   sdkmath.NewInt(1000),
			expectedModuleFractional: sdkmath.ZeroInt(),
			expectedReserveIntDelta:  sdkmath.ZeroInt(),
			expectedRemainder:        sdkmath.ZeroInt(),
			expectedMeowBalance:      sdkmath.ZeroInt(),
		},
		{
			name:                     "passthrough unrelated denom",
			startFractional:          sdkmath.ZeroInt(),
			startRemainder:           sdkmath.ZeroInt(),
			mintCoins:                sdk.NewCoins(sdk.NewCoin("meow", sdkmath.NewInt(1000))),
			expectedModuleIntDelta:   sdkmath.ZeroInt(),
			expectedModuleFractional: sdkmath.ZeroInt(),
			expectedReserveIntDelta:  sdkmath.ZeroInt(),
			expectedRemainder:        sdkmath.ZeroInt(),
			expectedMeowBalance:      sdkmath.NewInt(1000),
		},
		{
			name:                     "no carry with zero starting fractional",
			startFractional:          sdkmath.ZeroInt(),
			startRemainder:           sdkmath.ZeroInt(),
			mintCoins:                sdk.NewCoins(sdk.NewCoin(precisebanktypes.ExtendedCoinDenom(), sdkmath.NewInt(1000))),
			expectedModuleIntDelta:   sdkmath.ZeroInt(),
			expectedModuleFractional: sdkmath.NewInt(1000),
			expectedReserveIntDelta:  sdkmath.OneInt(),
			expectedRemainder:        cf.Sub(sdkmath.NewInt(1000)),
			expectedMeowBalance:      sdkmath.ZeroInt(),
		},
		{
			name:                     "no carry with non-zero starting fractional",
			startFractional:          sdkmath.NewInt(1_000_000),
			startRemainder:           sdkmath.ZeroInt(),
			mintCoins:                sdk.NewCoins(sdk.NewCoin(precisebanktypes.ExtendedCoinDenom(), sdkmath.NewInt(1000))),
			expectedModuleIntDelta:   sdkmath.ZeroInt(),
			expectedModuleFractional: sdkmath.NewInt(1_001_000),
			expectedReserveIntDelta:  sdkmath.OneInt(),
			expectedRemainder:        cf.Sub(sdkmath.NewInt(1000)),
			expectedMeowBalance:      sdkmath.ZeroInt(),
		},
		{
			name:                     "fractional carry",
			startFractional:          cf.SubRaw(1),
			startRemainder:           sdkmath.ZeroInt(),
			mintCoins:                sdk.NewCoins(sdk.NewCoin(precisebanktypes.ExtendedCoinDenom(), sdkmath.OneInt())),
			expectedModuleIntDelta:   sdkmath.OneInt(),
			expectedModuleFractional: sdkmath.ZeroInt(),
			expectedReserveIntDelta:  sdkmath.ZeroInt(),
			expectedRemainder:        cf.SubRaw(1),
			expectedMeowBalance:      sdkmath.ZeroInt(),
		},
		{
			name:                     "fractional carry max",
			startFractional:          cf.SubRaw(1),
			startRemainder:           sdkmath.ZeroInt(),
			mintCoins:                sdk.NewCoins(sdk.NewCoin(precisebanktypes.ExtendedCoinDenom(), cf.SubRaw(1))),
			expectedModuleIntDelta:   sdkmath.OneInt(),
			expectedModuleFractional: cf.SubRaw(2),
			expectedReserveIntDelta:  sdkmath.ZeroInt(),
			expectedRemainder:        sdkmath.OneInt(),
			expectedMeowBalance:      sdkmath.ZeroInt(),
		},
		{
			name:                     "integer with fractional no carry",
			startFractional:          sdkmath.NewInt(1234),
			startRemainder:           sdkmath.ZeroInt(),
			mintCoins:                sdk.NewCoins(sdk.NewCoin(precisebanktypes.ExtendedCoinDenom(), sdkmath.NewInt(100))),
			expectedModuleIntDelta:   sdkmath.ZeroInt(),
			expectedModuleFractional: sdkmath.NewInt(1334),
			expectedReserveIntDelta:  sdkmath.OneInt(),
			expectedRemainder:        cf.Sub(sdkmath.NewInt(100)),
			expectedMeowBalance:      sdkmath.ZeroInt(),
		},
		{
			name:                     "integer with fractional carry",
			startFractional:          cf.Sub(sdkmath.NewInt(100)),
			startRemainder:           sdkmath.ZeroInt(),
			mintCoins:                sdk.NewCoins(sdk.NewCoin(precisebanktypes.ExtendedCoinDenom(), sdkmath.NewInt(105))),
			expectedModuleIntDelta:   sdkmath.OneInt(),
			expectedModuleFractional: sdkmath.NewInt(5),
			expectedReserveIntDelta:  sdkmath.ZeroInt(),
			expectedRemainder:        cf.Sub(sdkmath.NewInt(105)),
			expectedMeowBalance:      sdkmath.ZeroInt(),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			app := Setup(t)
			ctx := app.BaseApp.NewContext(false)

			moduleAddr := app.AuthKeeper.GetModuleAddress(minttypes.ModuleName)
			reserveAddr := app.AuthKeeper.GetModuleAddress(precisebanktypes.ModuleName)

			if tc.startFractional.IsPositive() {
				app.PreciseBankKeeper.SetFractionalBalance(ctx, moduleAddr, tc.startFractional)
			}
			app.PreciseBankKeeper.SetRemainderAmount(ctx, tc.startRemainder)

			moduleIntBefore := app.BankKeeper.GetBalance(ctx, moduleAddr, precisebanktypes.IntegerCoinDenom()).Amount
			reserveIntBefore := app.BankKeeper.GetBalance(ctx, reserveAddr, precisebanktypes.IntegerCoinDenom()).Amount

			require.NoError(t, app.PreciseBankKeeper.MintCoins(ctx, minttypes.ModuleName, tc.mintCoins))

			moduleIntAfter := app.BankKeeper.GetBalance(ctx, moduleAddr, precisebanktypes.IntegerCoinDenom()).Amount
			moduleFracAfter := app.PreciseBankKeeper.GetFractionalBalance(ctx, moduleAddr)
			reserveIntAfter := app.BankKeeper.GetBalance(ctx, reserveAddr, precisebanktypes.IntegerCoinDenom()).Amount
			remainderAfter := app.PreciseBankKeeper.GetRemainderAmount(ctx)
			meowAfter := app.BankKeeper.GetBalance(ctx, moduleAddr, "meow").Amount

			require.True(t, moduleIntAfter.Sub(moduleIntBefore).Equal(tc.expectedModuleIntDelta))
			require.True(t, moduleFracAfter.Equal(tc.expectedModuleFractional))
			require.True(t, reserveIntAfter.Sub(reserveIntBefore).Equal(tc.expectedReserveIntDelta))
			require.True(t, remainderAfter.Equal(tc.expectedRemainder))
			require.True(t, meowAfter.Equal(tc.expectedMeowBalance))
		})
	}
}
