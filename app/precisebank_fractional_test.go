package app

import (
	"testing"

	"cosmossdk.io/math"
	"cosmossdk.io/store/prefix"
	sdk "github.com/cosmos/cosmos-sdk/types"
	precisebanktypes "github.com/cosmos/evm/x/precisebank/types"
	"github.com/stretchr/testify/require"
)

// TestPreciseBankSetGetFractionalBalanceMatrix validates fractional-balance
// state transitions and validation checks.
//
// Matrix:
// - valid positive amounts (min/regular/max) are persisted and retrievable
// - zero amount deletes the store entry
// - invalid amounts (negative / conversion-factor overflow) panic
func TestPreciseBankSetGetFractionalBalanceMatrix(t *testing.T) {
	app := Setup(t)
	ctx := app.BaseApp.NewContext(false)
	store := prefix.NewStore(ctx.KVStore(app.GetKey(precisebanktypes.StoreKey)), precisebanktypes.FractionalBalancePrefix)

	addr := sdk.AccAddress([]byte("fractional-test-address"))
	maxFractional := precisebanktypes.ConversionFactor().SubRaw(1)

	testCases := []struct {
		name        string
		amount      math.Int
		setPanicMsg string
	}{
		{name: "valid min amount", amount: math.NewInt(1)},
		{name: "valid positive amount", amount: math.NewInt(100)},
		{name: "valid max amount", amount: maxFractional},
		{name: "valid zero amount deletes", amount: math.ZeroInt()},
		{name: "invalid negative amount", amount: math.NewInt(-1), setPanicMsg: "amount is invalid: non-positive amount -1"},
		{
			name:        "invalid overflow amount",
			amount:      precisebanktypes.ConversionFactor(),
			setPanicMsg: "amount is invalid: amount 1000000000000 exceeds max of 999999999999",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if tc.setPanicMsg != "" {
				require.PanicsWithError(t, tc.setPanicMsg, func() {
					app.PreciseBankKeeper.SetFractionalBalance(ctx, addr, tc.amount)
				})
				return
			}

			require.NotPanics(t, func() {
				app.PreciseBankKeeper.SetFractionalBalance(ctx, addr, tc.amount)
			})

			if tc.amount.IsZero() {
				require.Nil(t, store.Get(precisebanktypes.FractionalBalanceKey(addr)))
				return
			}

			require.True(t, app.PreciseBankKeeper.GetFractionalBalance(ctx, addr).Equal(tc.amount))

			app.PreciseBankKeeper.DeleteFractionalBalance(ctx, addr)
			require.Nil(t, store.Get(precisebanktypes.FractionalBalanceKey(addr)))
		})
	}
}

// TestPreciseBankSetFractionalBalanceEmptyAddrPanics verifies empty addresses
// are rejected by precisebank keeper.
func TestPreciseBankSetFractionalBalanceEmptyAddrPanics(t *testing.T) {
	app := Setup(t)
	ctx := app.BaseApp.NewContext(false)

	require.PanicsWithError(t, "address cannot be empty", func() {
		app.PreciseBankKeeper.SetFractionalBalance(ctx, sdk.AccAddress{}, math.NewInt(100))
	})
}

// TestPreciseBankSetFractionalBalanceZeroDeletes verifies explicit zeroing
// clears existing state and remains idempotent when repeated.
func TestPreciseBankSetFractionalBalanceZeroDeletes(t *testing.T) {
	app := Setup(t)
	ctx := app.BaseApp.NewContext(false)
	store := prefix.NewStore(ctx.KVStore(app.GetKey(precisebanktypes.StoreKey)), precisebanktypes.FractionalBalancePrefix)

	addr := sdk.AccAddress([]byte("fractional-zero-delete"))
	app.PreciseBankKeeper.SetFractionalBalance(ctx, addr, math.NewInt(100))
	require.True(t, app.PreciseBankKeeper.GetFractionalBalance(ctx, addr).Equal(math.NewInt(100)))

	app.PreciseBankKeeper.SetFractionalBalance(ctx, addr, math.ZeroInt())
	require.Nil(t, store.Get(precisebanktypes.FractionalBalanceKey(addr)))

	require.NotPanics(t, func() {
		app.PreciseBankKeeper.SetFractionalBalance(ctx, addr, math.ZeroInt())
	})
}

// TestPreciseBankIterateFractionalBalancesAndAggregateSum verifies iterator and
// aggregate-sum behavior across stored fractional balances.
func TestPreciseBankIterateFractionalBalancesAndAggregateSum(t *testing.T) {
	app := Setup(t)
	ctx := app.BaseApp.NewContext(false)

	var (
		addrs []sdk.AccAddress
		sum   = math.ZeroInt()
	)

	for i := 1; i < 10; i++ {
		addr := sdk.AccAddress([]byte{byte(i)})
		amt := math.NewInt(int64(i))
		addrs = append(addrs, addr)
		sum = sum.Add(amt)
		app.PreciseBankKeeper.SetFractionalBalance(ctx, addr, amt)
	}

	var seen []sdk.AccAddress
	app.PreciseBankKeeper.IterateFractionalBalances(ctx, func(addr sdk.AccAddress, bal math.Int) bool {
		seen = append(seen, addr)
		require.Equal(t, int64(addr.Bytes()[0]), bal.Int64())
		return false
	})
	require.ElementsMatch(t, addrs, seen)

	require.True(t, app.PreciseBankKeeper.GetTotalSumFractionalBalances(ctx).Equal(sum))
}
