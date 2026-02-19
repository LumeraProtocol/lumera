package app

import (
	"testing"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	precisebanktypes "github.com/cosmos/evm/x/precisebank/types"
	"github.com/stretchr/testify/require"
)

// TestPreciseBankMintCoinsMissingModulePanicParity verifies missing module
// panics are parity-compatible between precisebank and bank keeper.
func TestPreciseBankMintCoinsMissingModulePanicParity(t *testing.T) {
	app := Setup(t)
	ctx := app.BaseApp.NewContext(false)

	mintCoins := sdk.NewCoins(sdk.NewCoin(precisebanktypes.IntegerCoinDenom(), sdkmath.NewInt(1)))

	bankPanic := capturePanicString(func() {
		_ = app.BankKeeper.MintCoins(ctx, "missing-module", mintCoins)
	})
	precisePanic := capturePanicString(func() {
		_ = app.PreciseBankKeeper.MintCoins(ctx, "missing-module", mintCoins)
	})

	require.NotEmpty(t, bankPanic)
	require.Equal(t, bankPanic, precisePanic)
	require.Contains(t, bankPanic, "module account missing-module does not exist")
}

// TestPreciseBankBurnCoinsMissingModulePanicParity verifies missing module
// panics are parity-compatible between precisebank and bank keeper.
func TestPreciseBankBurnCoinsMissingModulePanicParity(t *testing.T) {
	app := Setup(t)
	ctx := app.BaseApp.NewContext(false)

	burnCoins := sdk.NewCoins(sdk.NewCoin(precisebanktypes.IntegerCoinDenom(), sdkmath.NewInt(1)))

	bankPanic := capturePanicString(func() {
		_ = app.BankKeeper.BurnCoins(ctx, "missing-module", burnCoins)
	})
	precisePanic := capturePanicString(func() {
		_ = app.PreciseBankKeeper.BurnCoins(ctx, "missing-module", burnCoins)
	})

	require.NotEmpty(t, bankPanic)
	require.Equal(t, bankPanic, precisePanic)
	require.Contains(t, bankPanic, "module account missing-module does not exist")
}

// TestPreciseBankMintCoinsInvalidCoinsErrorParity verifies invalid-coin
// validation errors are parity-compatible for mint paths.
func TestPreciseBankMintCoinsInvalidCoinsErrorParity(t *testing.T) {
	app := Setup(t)
	ctx := app.BaseApp.NewContext(false)

	invalidCoins := sdk.Coins{
		sdk.Coin{Denom: precisebanktypes.IntegerCoinDenom(), Amount: sdkmath.NewInt(-1000)},
	}

	bankErr := app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, invalidCoins)
	require.Error(t, bankErr)

	preciseErr := app.PreciseBankKeeper.MintCoins(ctx, minttypes.ModuleName, invalidCoins)
	require.Error(t, preciseErr)

	require.Equal(t, bankErr.Error(), preciseErr.Error())
}

// TestPreciseBankBurnCoinsInvalidCoinsErrorParity verifies invalid-coin
// validation errors are parity-compatible for burn paths.
func TestPreciseBankBurnCoinsInvalidCoinsErrorParity(t *testing.T) {
	app := Setup(t)
	ctx := app.BaseApp.NewContext(false)

	invalidCoins := sdk.Coins{
		sdk.Coin{Denom: precisebanktypes.IntegerCoinDenom(), Amount: sdkmath.NewInt(-1000)},
	}

	// x/gov has burner permission in app config.
	bankErr := app.BankKeeper.BurnCoins(ctx, govtypes.ModuleName, invalidCoins)
	require.Error(t, bankErr)

	preciseErr := app.PreciseBankKeeper.BurnCoins(ctx, govtypes.ModuleName, invalidCoins)
	require.Error(t, preciseErr)

	require.Equal(t, bankErr.Error(), preciseErr.Error())
}
