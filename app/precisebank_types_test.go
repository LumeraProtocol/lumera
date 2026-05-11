package app

import (
	"math/big"
	"strings"
	"testing"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	precisebanktypes "github.com/cosmos/evm/x/precisebank/types"
	"github.com/stretchr/testify/require"
)

// TestPreciseBankTypesConversionFactorInvariants verifies conversion-factor
// immutability and expected 6-decimal chain value.
func TestPreciseBankTypesConversionFactorInvariants(t *testing.T) {
	_ = Setup(t)

	cf1 := precisebanktypes.ConversionFactor()
	original := cf1.Int64()

	// Mutate the returned big.Int pointer and ensure global conversion factor is unchanged.
	internal := cf1.BigIntMut()
	internal.Add(internal, big.NewInt(5))
	require.Equal(t, original+5, internal.Int64())

	cf2 := precisebanktypes.ConversionFactor()
	require.Equal(t, original, cf2.Int64())
	require.Equal(t, sdkmath.NewInt(1_000_000_000_000), cf2)

	// Independent calls should not share the same big.Int pointer.
	require.NotSame(t, precisebanktypes.ConversionFactor().BigIntMut(), precisebanktypes.ConversionFactor().BigIntMut())
}

// TestPreciseBankTypesNewFractionalBalance verifies constructor field wiring.
func TestPreciseBankTypesNewFractionalBalance(t *testing.T) {
	addr := sdk.AccAddress{9}.String()
	amount := sdkmath.NewInt(123)

	fb := precisebanktypes.NewFractionalBalance(addr, amount)
	require.Equal(t, addr, fb.Address)
	require.True(t, fb.Amount.Equal(amount))
}

// TestPreciseBankTypesFractionalBalanceValidateMatrix checks valid and invalid
// address/amount combinations for FractionalBalance validation.
func TestPreciseBankTypesFractionalBalanceValidateMatrix(t *testing.T) {
	_ = Setup(t)

	validAddr := sdk.AccAddress{1}.String()

	testCases := []struct {
		name        string
		address     string
		amount      sdkmath.Int
		errContains string
	}{
		{name: "valid", address: validAddr, amount: sdkmath.NewInt(100)},
		{name: "valid uppercase address", address: strings.ToUpper(validAddr), amount: sdkmath.NewInt(100)},
		{name: "valid min amount", address: validAddr, amount: sdkmath.NewInt(1)},
		{name: "valid max amount", address: validAddr, amount: precisebanktypes.ConversionFactor().SubRaw(1)},
		{name: "invalid zero amount", address: validAddr, amount: sdkmath.ZeroInt(), errContains: "non-positive amount 0"},
		{name: "invalid nil amount", address: validAddr, amount: sdkmath.Int{}, errContains: "nil amount"},
		{name: "invalid mixed case address", address: strings.ToLower(validAddr[:4]) + strings.ToUpper(validAddr[4:]), amount: sdkmath.NewInt(100), errContains: "string not all lowercase or all uppercase"},
		{name: "invalid non-bech32 address", address: "invalid", amount: sdkmath.NewInt(100), errContains: "invalid bech32"},
		{name: "invalid negative amount", address: validAddr, amount: sdkmath.NewInt(-100), errContains: "non-positive amount -100"},
		{name: "invalid amount above max", address: validAddr, amount: precisebanktypes.ConversionFactor(), errContains: "exceeds max"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := precisebanktypes.NewFractionalBalance(tc.address, tc.amount).Validate()
			if tc.errContains == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.ErrorContains(t, err, tc.errContains)
			}
		})
	}
}

// TestPreciseBankTypesFractionalBalancesValidateMatrix verifies aggregate slice
// validation and duplicate-address detection.
func TestPreciseBankTypesFractionalBalancesValidateMatrix(t *testing.T) {
	_ = Setup(t)

	addr1 := sdk.AccAddress{1}.String()
	addr2 := sdk.AccAddress{2}.String()
	addr3 := sdk.AccAddress{3}.String()

	testCases := []struct {
		name        string
		balances    precisebanktypes.FractionalBalances
		errContains string
	}{
		{name: "valid empty", balances: precisebanktypes.FractionalBalances{}},
		{name: "valid nil", balances: nil},
		{
			name: "valid multiple",
			balances: precisebanktypes.FractionalBalances{
				precisebanktypes.NewFractionalBalance(addr1, sdkmath.NewInt(100)),
				precisebanktypes.NewFractionalBalance(addr2, sdkmath.NewInt(100)),
				precisebanktypes.NewFractionalBalance(addr3, sdkmath.NewInt(100)),
			},
		},
		{
			name: "invalid single balance",
			balances: precisebanktypes.FractionalBalances{
				precisebanktypes.NewFractionalBalance(addr1, sdkmath.NewInt(100)),
				precisebanktypes.NewFractionalBalance(addr2, sdkmath.NewInt(-1)),
			},
			errContains: "invalid fractional balance",
		},
		{
			name: "invalid duplicate address",
			balances: precisebanktypes.FractionalBalances{
				precisebanktypes.NewFractionalBalance(addr1, sdkmath.NewInt(100)),
				precisebanktypes.NewFractionalBalance(addr1, sdkmath.NewInt(100)),
			},
			errContains: "duplicate address",
		},
		{
			name: "invalid duplicate uppercase/lowercase",
			balances: precisebanktypes.FractionalBalances{
				precisebanktypes.NewFractionalBalance(strings.ToLower(addr1), sdkmath.NewInt(100)),
				precisebanktypes.NewFractionalBalance(strings.ToUpper(addr1), sdkmath.NewInt(100)),
			},
			errContains: "duplicate address",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := tc.balances.Validate()
			if tc.errContains == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.ErrorContains(t, err, tc.errContains)
			}
		})
	}
}

// TestPreciseBankTypesFractionalBalancesSumAndOverflow verifies sum behavior
// and overflow safety for large integer accumulation.
func TestPreciseBankTypesFractionalBalancesSumAndOverflow(t *testing.T) {
	_ = Setup(t)

	addr1 := sdk.AccAddress{1}.String()
	addr2 := sdk.AccAddress{2}.String()

	require.True(t, precisebanktypes.FractionalBalances{}.SumAmount().IsZero())

	single := precisebanktypes.FractionalBalances{
		precisebanktypes.NewFractionalBalance(addr1, sdkmath.NewInt(100)),
	}
	require.True(t, single.SumAmount().Equal(sdkmath.NewInt(100)))

	multi := precisebanktypes.FractionalBalances{
		precisebanktypes.NewFractionalBalance(addr1, sdkmath.NewInt(100)),
		precisebanktypes.NewFractionalBalance(addr2, sdkmath.NewInt(200)),
	}
	require.True(t, multi.SumAmount().Equal(sdkmath.NewInt(300)))

	maxInt := new(big.Int).Sub(new(big.Int).Exp(big.NewInt(2), big.NewInt(256), nil), big.NewInt(1))
	overflow := precisebanktypes.FractionalBalances{
		precisebanktypes.NewFractionalBalance(addr1, sdkmath.NewInt(100)),
		precisebanktypes.NewFractionalBalance(addr2, sdkmath.NewIntFromBigInt(maxInt)),
	}
	require.PanicsWithError(t, sdkmath.ErrIntOverflow.Error(), func() {
		_ = overflow.SumAmount()
	})
}

// TestPreciseBankTypesGenesisValidateMatrix verifies genesis validation for
// balances, remainder bounds, and divisibility rules.
func TestPreciseBankTypesGenesisValidateMatrix(t *testing.T) {
	_ = Setup(t)

	addr1 := sdk.AccAddress{1}.String()
	addr2 := sdk.AccAddress{2}.String()

	testCases := []struct {
		name        string
		genesis     *precisebanktypes.GenesisState
		errContains string
	}{
		{name: "default valid", genesis: precisebanktypes.DefaultGenesisState()},
		{name: "empty balances zero remainder", genesis: &precisebanktypes.GenesisState{Remainder: sdkmath.ZeroInt()}},
		{name: "nil balances constructor", genesis: precisebanktypes.NewGenesisState(nil, sdkmath.ZeroInt())},
		{
			name: "max remainder valid with one balance",
			genesis: precisebanktypes.NewGenesisState(
				precisebanktypes.FractionalBalances{
					precisebanktypes.NewFractionalBalance(addr1, sdkmath.NewInt(1)),
				},
				precisebanktypes.ConversionFactor().SubRaw(1),
			),
		},
		{name: "invalid nil remainder", genesis: &precisebanktypes.GenesisState{}, errContains: "nil remainder amount"},
		{
			name: "invalid duplicate balances",
			genesis: precisebanktypes.NewGenesisState(
				precisebanktypes.FractionalBalances{
					precisebanktypes.NewFractionalBalance(addr1, sdkmath.NewInt(1)),
					precisebanktypes.NewFractionalBalance(addr1, sdkmath.NewInt(1)),
				},
				sdkmath.ZeroInt(),
			),
			errContains: "invalid balances: duplicate address",
		},
		{
			name: "invalid negative remainder",
			genesis: precisebanktypes.NewGenesisState(
				precisebanktypes.FractionalBalances{
					precisebanktypes.NewFractionalBalance(addr1, sdkmath.NewInt(1)),
					precisebanktypes.NewFractionalBalance(addr2, sdkmath.NewInt(1)),
				},
				sdkmath.NewInt(-1),
			),
			errContains: "negative remainder amount -1",
		},
		{
			name: "invalid remainder over max",
			genesis: precisebanktypes.NewGenesisState(
				precisebanktypes.FractionalBalances{
					precisebanktypes.NewFractionalBalance(addr1, sdkmath.NewInt(1)),
					precisebanktypes.NewFractionalBalance(addr2, sdkmath.NewInt(1)),
				},
				precisebanktypes.ConversionFactor(),
			),
			errContains: "exceeds max",
		},
		{
			name:        "invalid non-divisible total",
			genesis:     precisebanktypes.NewGenesisState(precisebanktypes.FractionalBalances{}, sdkmath.NewInt(1)),
			errContains: "is not a multiple",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := tc.genesis.Validate()
			if tc.errContains == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.ErrorContains(t, err, tc.errContains)
			}
		})
	}
}

// TestPreciseBankTypesGenesisTotalAmountWithRemainder verifies total amount
// aggregation from balances plus remainder.
func TestPreciseBankTypesGenesisTotalAmountWithRemainder(t *testing.T) {
	_ = Setup(t)

	addr1 := sdk.AccAddress{1}.String()
	addr2 := sdk.AccAddress{2}.String()
	cf := precisebanktypes.ConversionFactor()

	testCases := []struct {
		name        string
		balances    precisebanktypes.FractionalBalances
		remainder   sdkmath.Int
		expectedSum sdkmath.Int
	}{
		{
			name:        "empty balances zero remainder",
			balances:    precisebanktypes.FractionalBalances{},
			remainder:   sdkmath.ZeroInt(),
			expectedSum: sdkmath.ZeroInt(),
		},
		{
			name: "non-empty zero remainder",
			balances: precisebanktypes.FractionalBalances{
				precisebanktypes.NewFractionalBalance(addr1, cf.QuoRaw(2)),
				precisebanktypes.NewFractionalBalance(addr2, cf.QuoRaw(2)),
			},
			remainder:   sdkmath.ZeroInt(),
			expectedSum: cf,
		},
		{
			name: "non-empty with one remainder",
			balances: precisebanktypes.FractionalBalances{
				precisebanktypes.NewFractionalBalance(addr1, cf.QuoRaw(2)),
				precisebanktypes.NewFractionalBalance(addr2, cf.QuoRaw(2).SubRaw(1)),
			},
			remainder:   sdkmath.OneInt(),
			expectedSum: cf,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			genesis := precisebanktypes.NewGenesisState(tc.balances, tc.remainder)
			require.NoError(t, genesis.Validate())
			require.True(t, genesis.TotalAmountWithRemainder().Equal(tc.expectedSum))
		})
	}
}

// TestPreciseBankTypesFractionalBalanceKey verifies key encoding is the raw
// account-address bytes.
func TestPreciseBankTypesFractionalBalanceKey(t *testing.T) {
	addr := sdk.AccAddress([]byte("test-address"))
	key := precisebanktypes.FractionalBalanceKey(addr)
	require.Equal(t, addr.Bytes(), key)
	require.Equal(t, addr, sdk.AccAddress(key))
}

// TestPreciseBankTypesSumExtendedCoin verifies integer and extended denoms are
// combined into one extended-denom total.
func TestPreciseBankTypesSumExtendedCoin(t *testing.T) {
	_ = Setup(t)

	require.False(t, precisebanktypes.IsExtendedDenomSameAsIntegerDenom())

	integerDenom := precisebanktypes.IntegerCoinDenom()
	extendedDenom := precisebanktypes.ExtendedCoinDenom()
	cf := precisebanktypes.ConversionFactor()

	testCases := []struct {
		name string
		amt  sdk.Coins
		want sdk.Coin
	}{
		{
			name: "empty",
			amt:  sdk.NewCoins(),
			want: sdk.NewCoin(extendedDenom, sdkmath.ZeroInt()),
		},
		{
			name: "only integer",
			amt:  sdk.NewCoins(sdk.NewInt64Coin(integerDenom, 100)),
			want: sdk.NewCoin(extendedDenom, cf.MulRaw(100)),
		},
		{
			name: "only extended",
			amt:  sdk.NewCoins(sdk.NewInt64Coin(extendedDenom, 100)),
			want: sdk.NewCoin(extendedDenom, sdkmath.NewInt(100)),
		},
		{
			name: "integer and extended",
			amt: sdk.NewCoins(
				sdk.NewInt64Coin(integerDenom, 100),
				sdk.NewInt64Coin(extendedDenom, 100),
			),
			want: sdk.NewCoin(extendedDenom, cf.MulRaw(100).AddRaw(100)),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, precisebanktypes.SumExtendedCoin(tc.amt))
		})
	}
}
