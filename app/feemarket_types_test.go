package app

import (
	"testing"

	sdkmath "cosmossdk.io/math"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	"github.com/stretchr/testify/require"
)

// TestFeeMarketTypesParamsValidateMatrix verifies feemarket params validation
// behavior with valid and invalid parameter sets.
func TestFeeMarketTypesParamsValidateMatrix(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		params    feemarkettypes.Params
		expectErr bool
	}{
		{name: "default", params: feemarkettypes.DefaultParams()},
		{
			name: "valid custom",
			params: feemarkettypes.NewParams(
				true,
				7,
				3,
				sdkmath.LegacyNewDec(2_000_000_000),
				int64(544435345345435345),
				sdkmath.LegacyNewDecWithPrec(20, 4),
				feemarkettypes.DefaultMinGasMultiplier,
			),
		},
		{name: "empty invalid", params: feemarkettypes.Params{}, expectErr: true},
		{
			name: "invalid base fee change denom zero",
			params: feemarkettypes.NewParams(
				true, 0, 3, sdkmath.LegacyNewDec(2_000_000_000), 100,
				feemarkettypes.DefaultMinGasPrice, feemarkettypes.DefaultMinGasMultiplier,
			),
			expectErr: true,
		},
		{
			name: "invalid elasticity multiplier zero",
			params: feemarkettypes.NewParams(
				true, 7, 0, sdkmath.LegacyNewDec(2_000_000_000), 100,
				feemarkettypes.DefaultMinGasPrice, feemarkettypes.DefaultMinGasMultiplier,
			),
			expectErr: true,
		},
		{
			name: "invalid enable height negative",
			params: feemarkettypes.NewParams(
				true, 7, 3, sdkmath.LegacyNewDec(2_000_000_000), -10,
				feemarkettypes.DefaultMinGasPrice, feemarkettypes.DefaultMinGasMultiplier,
			),
			expectErr: true,
		},
		{
			name: "invalid base fee negative",
			params: feemarkettypes.NewParams(
				true, 7, 3, sdkmath.LegacyNewDec(-2_000_000_000), 100,
				feemarkettypes.DefaultMinGasPrice, feemarkettypes.DefaultMinGasMultiplier,
			),
			expectErr: true,
		},
		{
			name: "invalid min gas price negative",
			params: feemarkettypes.NewParams(
				true, 7, 3, sdkmath.LegacyNewDec(2_000_000_000), 100,
				sdkmath.LegacyNewDecFromInt(sdkmath.NewInt(-1)), feemarkettypes.DefaultMinGasMultiplier,
			),
			expectErr: true,
		},
		{
			name: "valid min gas multiplier zero",
			params: feemarkettypes.NewParams(
				true, 7, 3, sdkmath.LegacyNewDec(2_000_000_000), 100,
				feemarkettypes.DefaultMinGasPrice, sdkmath.LegacyZeroDec(),
			),
		},
		{
			name: "invalid min gas multiplier negative",
			params: feemarkettypes.NewParams(
				true, 7, 3, sdkmath.LegacyNewDec(2_000_000_000), 100,
				feemarkettypes.DefaultMinGasPrice, sdkmath.LegacyNewDecWithPrec(-5, 1),
			),
			expectErr: true,
		},
		{
			name: "invalid min gas multiplier greater than one",
			params: feemarkettypes.NewParams(
				true, 7, 3, sdkmath.LegacyNewDec(2_000_000_000), 100,
				feemarkettypes.DefaultMinGasPrice, sdkmath.LegacyNewDec(2),
			),
			expectErr: true,
		},
		{
			name: "invalid min gas price nil",
			params: feemarkettypes.NewParams(
				true, 7, 3, sdkmath.LegacyNewDec(2_000_000_000), 100,
				sdkmath.LegacyDec{}, feemarkettypes.DefaultMinGasMultiplier,
			),
			expectErr: true,
		},
		{
			name: "invalid min gas multiplier nil",
			params: feemarkettypes.NewParams(
				true, 7, 3, sdkmath.LegacyNewDec(2_000_000_000), 100,
				feemarkettypes.DefaultMinGasPrice, sdkmath.LegacyDec{},
			),
			expectErr: true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.params.Validate()
			if tc.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestFeeMarketTypesMsgUpdateParamsValidateBasic verifies authority and params
// validation checks for MsgUpdateParams.
func TestFeeMarketTypesMsgUpdateParamsValidateBasic(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		msg       *feemarkettypes.MsgUpdateParams
		expectErr bool
	}{
		{
			name: "invalid authority",
			msg: &feemarkettypes.MsgUpdateParams{
				Authority: "invalid",
				Params:    feemarkettypes.DefaultParams(),
			},
			expectErr: true,
		},
		{
			name: "invalid params",
			msg: &feemarkettypes.MsgUpdateParams{
				Authority: authtypes.NewModuleAddress(govtypes.ModuleName).String(),
				Params: feemarkettypes.NewParams(
					true, 0, 3, sdkmath.LegacyNewDec(2_000_000_000), 100,
					feemarkettypes.DefaultMinGasPrice, feemarkettypes.DefaultMinGasMultiplier,
				),
			},
			expectErr: true,
		},
		{
			name: "valid message",
			msg: &feemarkettypes.MsgUpdateParams{
				Authority: authtypes.NewModuleAddress(govtypes.ModuleName).String(),
				Params:    feemarkettypes.DefaultParams(),
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.msg.ValidateBasic()
			if tc.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestFeeMarketTypesGenesisValidateMatrix verifies genesis-state validation
// checks.
func TestFeeMarketTypesGenesisValidateMatrix(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		genesis   *feemarkettypes.GenesisState
		expectErr bool
	}{
		{name: "default", genesis: feemarkettypes.DefaultGenesisState()},
		{
			name: "valid explicit",
			genesis: &feemarkettypes.GenesisState{
				Params:   feemarkettypes.DefaultParams(),
				BlockGas: 1,
			},
		},
		{
			name: "valid constructor",
			genesis: feemarkettypes.NewGenesisState(
				feemarkettypes.DefaultParams(),
				1,
			),
		},
		{
			name: "empty invalid",
			genesis: &feemarkettypes.GenesisState{
				Params:   feemarkettypes.Params{},
				BlockGas: 0,
			},
			expectErr: true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.genesis.Validate()
			if tc.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
