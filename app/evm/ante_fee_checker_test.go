package evm_test

import (
	"math/big"
	"testing"

	evmante "github.com/cosmos/evm/ante/evm"
	cosmosante "github.com/cosmos/evm/ante/types"
	evmencoding "github.com/cosmos/evm/encoding"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/stretchr/testify/require"

	sdkmath "cosmossdk.io/math"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"

	lcfg "github.com/LumeraProtocol/lumera/config"
)

// TestDynamicFeeCheckerMatrix validates dynamic-fee TxFeeChecker behavior for
// Lumera's denom/config setup.
//
// Matrix:
// - Genesis path falls back to validator min-gas-prices checks.
// - CheckTx without sufficient fees is rejected.
// - CheckTx with sufficient fees is accepted.
// - DeliverTx fallback path does not enforce validator min-gas-prices.
// - Dynamic fee path enforces base fee when London is enabled.
// - Dynamic fee path accepts exact-base-fee txs.
// - Dynamic fee path without extension option computes priority from fee cap.
// - Dynamic fee extension option changes effective priority.
// - Dynamic fee extension with empty tip cap falls back to base fee priority 0.
// - Negative tip cap in extension option is rejected.
func TestDynamicFeeCheckerMatrix(t *testing.T) {
	ensureChainConfigInitialized(t)
	evmtypes.SetDefaultEvmCoinInfo(evmtypes.EvmCoinInfo{
		Denom:         lcfg.ChainDenom,
		ExtendedDenom: lcfg.ChainEVMExtendedDenom,
		DisplayDenom:  lcfg.ChainDisplayDenom,
		Decimals:      evmtypes.SixDecimals.Uint32(),
	})

	encodingCfg := evmencoding.MakeConfig(lcfg.EVMChainID)
	denom := lcfg.ChainDenom

	genesisCtx := sdk.Context{}.
		WithBlockHeight(0)
	genesisCheckTxCtx := sdk.Context{}.
		WithBlockHeight(0).
		WithIsCheckTx(true).
		WithMinGasPrices(sdk.NewDecCoins(sdk.NewDecCoin(denom, sdkmath.NewInt(10))))
	genesisDeliverWithMinGasCtx := sdk.Context{}.
		WithBlockHeight(0).
		WithIsCheckTx(false).
		WithMinGasPrices(sdk.NewDecCoins(sdk.NewDecCoin(denom, sdkmath.NewInt(10))))
	checkTxCtx := sdk.Context{}.
		WithBlockHeight(1).
		WithIsCheckTx(true).
		WithMinGasPrices(sdk.NewDecCoins(sdk.NewDecCoin(denom, sdkmath.NewInt(10))))
	deliverTxCtx := sdk.Context{}.
		WithBlockHeight(1).
		WithIsCheckTx(false)

	priorityReduction := evmtypes.DefaultPriorityReduction

	testCases := []struct {
		name          string
		ctx           sdk.Context
		londonEnabled bool
		params        feemarkettypes.Params
		buildTx       func() sdk.Tx
		expectFees    string
		expectPrio    int64
		expectErr     bool
	}{
		{
			name:          "genesis tx uses fallback fee logic",
			ctx:           genesisCtx,
			londonEnabled: false,
			params:        feemarkettypes.DefaultParams(),
			buildTx: func() sdk.Tx {
				txBuilder := encodingCfg.TxConfig.NewTxBuilder()
				txBuilder.SetGasLimit(1)
				return txBuilder.GetTx()
			},
			expectFees: "",
			expectPrio: 0,
		},
		{
			name:          "checktx enforces validator min gas prices",
			ctx:           checkTxCtx,
			londonEnabled: false,
			params:        feemarkettypes.DefaultParams(),
			buildTx: func() sdk.Tx {
				txBuilder := encodingCfg.TxConfig.NewTxBuilder()
				txBuilder.SetGasLimit(1)
				return txBuilder.GetTx()
			},
			expectErr: true,
		},
		{
			name:          "genesis checktx fallback accepts fees meeting validator min gas prices",
			ctx:           genesisCheckTxCtx,
			londonEnabled: false,
			params:        feemarkettypes.DefaultParams(),
			buildTx: func() sdk.Tx {
				txBuilder := encodingCfg.TxConfig.NewTxBuilder()
				txBuilder.SetGasLimit(1)
				txBuilder.SetFeeAmount(sdk.NewCoins(sdk.NewCoin(denom, sdkmath.NewInt(10))))
				return txBuilder.GetTx()
			},
			expectFees: "10" + denom,
			expectPrio: 0,
		},
		{
			name:          "genesis deliver fallback ignores validator min gas prices",
			ctx:           genesisDeliverWithMinGasCtx,
			londonEnabled: false,
			params:        feemarkettypes.DefaultParams(),
			buildTx: func() sdk.Tx {
				txBuilder := encodingCfg.TxConfig.NewTxBuilder()
				txBuilder.SetGasLimit(1)
				return txBuilder.GetTx()
			},
			expectFees: "",
			expectPrio: 0,
		},
		{
			name:          "rejects fee cap below base fee when london enabled",
			ctx:           deliverTxCtx,
			londonEnabled: true,
			params: feemarkettypes.Params{
				BaseFee: sdkmath.LegacyNewDec(10),
			},
			buildTx: func() sdk.Tx {
				txBuilder := encodingCfg.TxConfig.NewTxBuilder()
				txBuilder.SetGasLimit(1)
				txBuilder.SetFeeAmount(sdk.NewCoins(sdk.NewCoin(denom, sdkmath.NewInt(9))))
				return txBuilder.GetTx()
			},
			expectErr: true,
		},
		{
			name:          "accepts fee equal to base fee when london enabled",
			ctx:           deliverTxCtx,
			londonEnabled: true,
			params: feemarkettypes.Params{
				BaseFee: sdkmath.LegacyNewDec(10),
			},
			buildTx: func() sdk.Tx {
				txBuilder := encodingCfg.TxConfig.NewTxBuilder()
				txBuilder.SetGasLimit(1)
				txBuilder.SetFeeAmount(sdk.NewCoins(sdk.NewCoin(denom, sdkmath.NewInt(10))))
				return txBuilder.GetTx()
			},
			expectFees: "10" + denom,
			expectPrio: 0,
		},
		{
			name:          "dynamic fee without extension computes priority from fee cap",
			ctx:           deliverTxCtx,
			londonEnabled: true,
			params: feemarkettypes.Params{
				BaseFee: sdkmath.LegacyNewDec(10),
			},
			buildTx: func() sdk.Tx {
				txBuilder := encodingCfg.TxConfig.NewTxBuilder()
				txBuilder.SetGasLimit(1)
				txBuilder.SetFeeAmount(sdk.NewCoins(sdk.NewCoin(
					denom,
					sdkmath.NewInt(10).Mul(priorityReduction).Add(sdkmath.NewInt(10)),
				)))
				return txBuilder.GetTx()
			},
			expectFees: "10000010" + denom,
			expectPrio: 10,
		},
		{
			name:          "dynamic fee extension option applies tip cap to priority",
			ctx:           deliverTxCtx,
			londonEnabled: true,
			params: feemarkettypes.Params{
				BaseFee: sdkmath.LegacyNewDec(10),
			},
			buildTx: func() sdk.Tx {
				txBuilder := encodingCfg.TxConfig.NewTxBuilder().(authtx.ExtensionOptionsTxBuilder)
				txBuilder.SetGasLimit(1)
				txBuilder.SetFeeAmount(sdk.NewCoins(sdk.NewCoin(
					denom,
					sdkmath.NewInt(10).Mul(priorityReduction).Add(sdkmath.NewInt(10)),
				)))

				option, err := codectypes.NewAnyWithValue(&cosmosante.ExtensionOptionDynamicFeeTx{
					MaxPriorityPrice: sdkmath.LegacyNewDec(5).MulInt(priorityReduction),
				})
				require.NoError(t, err)
				txBuilder.SetExtensionOptions(option)
				return txBuilder.GetTx()
			},
			expectFees: "5000010" + denom,
			expectPrio: 5,
		},
		{
			name:          "dynamic fee extension with empty tip cap uses base fee only",
			ctx:           deliverTxCtx,
			londonEnabled: true,
			params: feemarkettypes.Params{
				BaseFee: sdkmath.LegacyNewDec(10),
			},
			buildTx: func() sdk.Tx {
				txBuilder := encodingCfg.TxConfig.NewTxBuilder().(authtx.ExtensionOptionsTxBuilder)
				txBuilder.SetGasLimit(1)
				txBuilder.SetFeeAmount(sdk.NewCoins(sdk.NewCoin(
					denom,
					sdkmath.NewInt(10).Mul(priorityReduction),
				)))

				option, err := codectypes.NewAnyWithValue(&cosmosante.ExtensionOptionDynamicFeeTx{})
				require.NoError(t, err)
				txBuilder.SetExtensionOptions(option)
				return txBuilder.GetTx()
			},
			expectFees: "10" + denom,
			expectPrio: 0,
		},
		{
			name:          "rejects negative tip cap in extension option",
			ctx:           deliverTxCtx,
			londonEnabled: true,
			params: feemarkettypes.Params{
				BaseFee: sdkmath.LegacyNewDec(10),
			},
			buildTx: func() sdk.Tx {
				txBuilder := encodingCfg.TxConfig.NewTxBuilder().(authtx.ExtensionOptionsTxBuilder)
				txBuilder.SetGasLimit(1)
				txBuilder.SetFeeAmount(sdk.NewCoins(sdk.NewCoin(
					denom,
					sdkmath.NewInt(10).Mul(priorityReduction).Add(sdkmath.NewInt(10)),
				)))

				option, err := codectypes.NewAnyWithValue(&cosmosante.ExtensionOptionDynamicFeeTx{
					MaxPriorityPrice: sdkmath.LegacyNewDec(-5).MulInt(priorityReduction),
				})
				require.NoError(t, err)
				txBuilder.SetExtensionOptions(option)
				return txBuilder.GetTx()
			},
			expectErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ethCfg := evmtypes.GetEthChainConfig()
			originalLondon := ethCfg.LondonBlock
			t.Cleanup(func() { ethCfg.LondonBlock = originalLondon })
			if tc.londonEnabled {
				ethCfg.LondonBlock = big.NewInt(0)
			} else {
				ethCfg.LondonBlock = big.NewInt(10_000)
			}

			fees, priority, err := evmante.NewDynamicFeeChecker(&tc.params)(tc.ctx, tc.buildTx())
			if tc.expectErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.expectFees, fees.String())
			require.Equal(t, tc.expectPrio, priority)
		})
	}
}
