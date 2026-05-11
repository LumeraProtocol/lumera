package evm_test

import (
	"testing"

	cosmosante "github.com/cosmos/evm/ante/cosmos"
	utiltx "github.com/cosmos/evm/testutil/tx"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"

	lcfg "github.com/LumeraProtocol/lumera/config"
)

// TestMinGasPriceDecoratorMatrix validates the Cosmos min-gas-price checks used
// by the EVM-enabled ante chain.
//
// Matrix:
// - Invalid tx type is rejected.
// - Zero min gas price allows empty fees.
// - Simulate bypasses strict fee checks.
// - Invalid fee denom is rejected.
// - Invalid multi-denom fee set is rejected.
// - Non-zero min gas price with nil fee is rejected.
// - Simulate bypasses invalid fee denom validation.
// - Fee below required threshold is rejected.
// - Fee equal to required threshold is accepted.
func TestMinGasPriceDecoratorMatrix(t *testing.T) {
	// Ensure x/vm global denom config is present for unit tests that don't boot an app.
	evmtypes.SetDefaultEvmCoinInfo(evmtypes.EvmCoinInfo{
		Denom:         lcfg.ChainDenom,
		ExtendedDenom: lcfg.ChainEVMExtendedDenom,
		DisplayDenom:  lcfg.ChainDisplayDenom,
		Decimals:      evmtypes.SixDecimals.Uint32(),
	})

	// Reuse one basic Cosmos msg; this test targets fee logic, not msg semantics.
	msg := banktypes.NewMsgSend(
		sdk.AccAddress("from_______________"),
		sdk.AccAddress("to_________________"),
		sdk.NewCoins(sdk.NewInt64Coin(lcfg.ChainDenom, 1)),
	)

	testCases := []struct {
		name            string            // Human-readable case title.
		tx              sdk.Tx            // Candidate tx passed to ante.
		minGasPrice     sdkmath.LegacyDec // feemarket MinGasPrice param for this case.
		simulate        bool              // Simulation mode toggle.
		expectErrIs     error             // Optional sentinel error expectation.
		expectErrSubstr string            // Optional error substring expectation.
	}{
		{
			name:            "invalid tx type",
			tx:              &utiltx.InvalidTx{},
			minGasPrice:     sdkmath.LegacyZeroDec(),
			expectErrIs:     sdkerrors.ErrInvalidType,
			expectErrSubstr: "expected sdk.FeeTx",
		},
		{
			name: "zero min gas price accepts empty fee",
			tx: mockFeeTx{
				fee:  nil,
				gas:  100,
				msgs: []sdk.Msg{msg},
			},
			minGasPrice: sdkmath.LegacyZeroDec(),
		},
		{
			name: "simulate bypasses min gas check",
			tx: mockFeeTx{
				fee:  sdk.NewCoins(sdk.NewCoin(lcfg.ChainDenom, sdkmath.NewInt(1))),
				gas:  100,
				msgs: []sdk.Msg{msg},
			},
			minGasPrice: sdkmath.LegacyNewDec(10), // required fee would be 1000
			simulate:    true,
		},
		{
			name: "rejects invalid fee denom",
			tx: mockFeeTx{
				fee:  sdk.NewCoins(sdk.NewCoin("invaliddenom", sdkmath.NewInt(1000))),
				gas:  100,
				msgs: []sdk.Msg{msg},
			},
			minGasPrice:     sdkmath.LegacyZeroDec(),
			expectErrSubstr: "expected only native token",
		},
		{
			name: "rejects invalid multi-denom fee set",
			tx: mockFeeTx{
				fee: sdk.NewCoins(
					sdk.NewCoin(lcfg.ChainDenom, sdkmath.NewInt(1000)),
					sdk.NewCoin("uatom", sdkmath.NewInt(1)),
				),
				gas:  100,
				msgs: []sdk.Msg{msg},
			},
			minGasPrice:     sdkmath.LegacyZeroDec(),
			expectErrSubstr: "expected only native token",
		},
		{
			name: "rejects nil fee when min gas price is non-zero",
			tx: mockFeeTx{
				fee:  nil,
				gas:  100,
				msgs: []sdk.Msg{msg},
			},
			minGasPrice:     sdkmath.LegacyNewDec(10),
			expectErrIs:     sdkerrors.ErrInsufficientFee,
			expectErrSubstr: "fee not provided",
		},
		{
			name: "simulate bypasses invalid fee denom validation",
			tx: mockFeeTx{
				fee:  sdk.NewCoins(sdk.NewCoin("invaliddenom", sdkmath.NewInt(1))),
				gas:  100,
				msgs: []sdk.Msg{msg},
			},
			minGasPrice: sdkmath.LegacyNewDec(10),
			simulate:    true,
		},
		{
			name: "rejects fee below required minimum",
			tx: mockFeeTx{
				fee:  sdk.NewCoins(sdk.NewCoin(lcfg.ChainDenom, sdkmath.NewInt(999))),
				gas:  100,
				msgs: []sdk.Msg{msg},
			},
			minGasPrice:     sdkmath.LegacyNewDec(10), // required fee = 1000
			expectErrIs:     sdkerrors.ErrInsufficientFee,
			expectErrSubstr: "provided fee < minimum global fee",
		},
		{
			name: "accepts fee equal required minimum",
			tx: mockFeeTx{
				fee:  sdk.NewCoins(sdk.NewCoin(lcfg.ChainDenom, sdkmath.NewInt(1000))),
				gas:  100,
				msgs: []sdk.Msg{msg},
			},
			minGasPrice: sdkmath.LegacyNewDec(10), // required fee = 1000
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			params := feemarkettypes.DefaultParams()
			params.MinGasPrice = tc.minGasPrice
			dec := cosmosante.NewMinGasPriceDecorator(&params)

			_, err := dec.AnteHandle(sdk.Context{}, tc.tx, tc.simulate, func(ctx sdk.Context, tx sdk.Tx, simulate bool) (sdk.Context, error) {
				return ctx, nil
			})

			if tc.expectErrIs == nil && tc.expectErrSubstr == "" {
				require.NoError(t, err)
				return
			}

			require.Error(t, err)
			if tc.expectErrIs != nil {
				require.ErrorIs(t, err, tc.expectErrIs)
			}
			if tc.expectErrSubstr != "" {
				require.Contains(t, err.Error(), tc.expectErrSubstr)
			}
		})
	}
}

type mockFeeTx struct {
	fee  sdk.Coins // Explicit fee coins returned by GetFee().
	gas  uint64    // Gas limit returned by GetGas().
	msgs []sdk.Msg // Messages exposed by GetMsgs().
}

func (m mockFeeTx) GetMsgs() []sdk.Msg { return m.msgs }

func (m mockFeeTx) GetMsgsV2() ([]proto.Message, error) { return nil, nil }

func (m mockFeeTx) ValidateBasic() error { return nil }

func (m mockFeeTx) GetGas() uint64 { return m.gas }

func (m mockFeeTx) GetFee() sdk.Coins { return m.fee }

func (m mockFeeTx) FeePayer() []byte { return nil }

func (m mockFeeTx) FeeGranter() []byte { return nil }
