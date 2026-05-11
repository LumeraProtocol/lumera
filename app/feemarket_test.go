package app

import (
	"context"
	"testing"

	sdkmath "cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	"github.com/stretchr/testify/require"
)

// TestFeeMarketCalculateBaseFee validates EIP-1559 base-fee calculation rules.
//
// Matrix:
// - no-base-fee mode returns nil
// - first enabled block returns configured base fee
// - above/at/below target gas moves base fee as expected
// - min gas price can floor downward movement
func TestFeeMarketCalculateBaseFee(t *testing.T) {
	testCases := []struct {
		name           string                                                       // Case name.
		noBaseFee      bool                                                         // Feemarket NoBaseFee toggle.
		minGasPrice    func(base sdkmath.LegacyDec) sdkmath.LegacyDec               // Optional min gas price override.
		blockHeight    int64                                                        // Context block height.
		blockMaxGas    int64                                                        // Consensus max gas for target computation.
		parentGasUsage uint64                                                       // Previous block gas used input.
		assertFn       func(t *testing.T, got, base, minGasPrice sdkmath.LegacyDec) // Case-specific assertion.
	}{
		{
			name:        "disabled returns nil",
			noBaseFee:   true,
			blockHeight: 1,
			blockMaxGas: 10_000_000,
			assertFn: func(t *testing.T, got, _, _ sdkmath.LegacyDec) {
				t.Helper()
				require.True(t, got.IsNil())
			},
		},
		{
			name:        "first eip1559 block returns configured base fee",
			blockHeight: 0,
			blockMaxGas: 10_000_000,
			assertFn: func(t *testing.T, got, base, _ sdkmath.LegacyDec) {
				t.Helper()
				require.True(t, got.Equal(base))
			},
		},
		{
			name:           "gas target match keeps base fee unchanged",
			blockHeight:    1,
			blockMaxGas:    10_000_000,
			parentGasUsage: 5_000_000, // max_gas / elasticity_multiplier(2)
			assertFn: func(t *testing.T, got, base, _ sdkmath.LegacyDec) {
				t.Helper()
				require.True(t, got.Equal(base))
			},
		},
		{
			name:           "gas above target increases base fee",
			blockHeight:    1,
			blockMaxGas:    10_000_000,
			parentGasUsage: 7_500_000,
			assertFn: func(t *testing.T, got, base, _ sdkmath.LegacyDec) {
				t.Helper()
				require.True(t, got.GT(base), "expected base fee increase: got=%s base=%s", got, base)
			},
		},
		{
			name:           "gas below target decreases base fee",
			blockHeight:    1,
			blockMaxGas:    10_000_000,
			parentGasUsage: 2_500_000,
			assertFn: func(t *testing.T, got, base, _ sdkmath.LegacyDec) {
				t.Helper()
				require.True(t, got.LT(base), "expected base fee decrease: got=%s base=%s", got, base)
			},
		},
		{
			name:        "min gas price floors base fee decrease",
			blockHeight: 1,
			blockMaxGas: 10_000_000,
			minGasPrice: func(base sdkmath.LegacyDec) sdkmath.LegacyDec {
				return base
			},
			parentGasUsage: 2_500_000,
			assertFn: func(t *testing.T, got, _, minGasPrice sdkmath.LegacyDec) {
				t.Helper()
				require.True(t, got.Equal(minGasPrice), "expected floor at min gas price: got=%s min=%s", got, minGasPrice)
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			app := Setup(t)
			ctx := app.BaseApp.NewContext(false)

			// Configure params and synthetic block context, then verify computed base fee.
			params := app.FeeMarketKeeper.GetParams(ctx)
			params.NoBaseFee = tc.noBaseFee
			params.EnableHeight = 0
			params.MinGasPrice = sdkmath.LegacyZeroDec()
			if tc.minGasPrice != nil {
				params.MinGasPrice = tc.minGasPrice(params.BaseFee)
			}
			require.NoError(t, app.FeeMarketKeeper.SetParams(ctx, params))

			ctx = ctx.WithBlockHeight(tc.blockHeight).WithConsensusParams(tmproto.ConsensusParams{
				Block: &tmproto.BlockParams{
					MaxGas:   tc.blockMaxGas,
					MaxBytes: 22020096,
				},
			})
			app.FeeMarketKeeper.SetBlockGasWanted(ctx, tc.parentGasUsage)

			got := app.FeeMarketKeeper.CalculateBaseFee(ctx)
			tc.assertFn(t, got, params.BaseFee, params.MinGasPrice)
		})
	}
}

// TestFeeMarketBeginBlockUpdatesBaseFee verifies BeginBlock updates stored base
// fee when parent gas usage is above target.
func TestFeeMarketBeginBlockUpdatesBaseFee(t *testing.T) {
	app := Setup(t)
	ctx := app.BaseApp.NewContext(false)

	params := app.FeeMarketKeeper.GetParams(ctx)
	params.NoBaseFee = false
	params.EnableHeight = 0
	params.MinGasPrice = sdkmath.LegacyZeroDec()
	require.NoError(t, app.FeeMarketKeeper.SetParams(ctx, params))

	ctx = ctx.WithBlockHeight(1).WithConsensusParams(tmproto.ConsensusParams{
		Block: &tmproto.BlockParams{
			MaxGas:   10_000_000,
			MaxBytes: 22020096,
		},
	})

	// Force parent gas usage above target to trigger base fee increase.
	app.FeeMarketKeeper.SetBlockGasWanted(ctx, 8_000_000)
	baseBefore := app.FeeMarketKeeper.GetParams(ctx).BaseFee

	require.NoError(t, app.FeeMarketKeeper.BeginBlock(ctx))

	baseAfter := app.FeeMarketKeeper.GetParams(ctx).BaseFee
	require.True(t, baseAfter.GT(baseBefore), "expected BeginBlock to increase base fee")
}

// TestFeeMarketEndBlockGasWantedClamp verifies EndBlock clamping logic that
// combines transient gas wanted and min-gas-multiplier floor.
func TestFeeMarketEndBlockGasWantedClamp(t *testing.T) {
	testCases := []struct {
		name              string            // Case name.
		transientGas      uint64            // Transient gas wanted accumulated in ante.
		blockGasConsumed  uint64            // Block gas meter consumption.
		minGasMultiplier  sdkmath.LegacyDec // Feemarket min gas multiplier.
		expectedGasWanted uint64            // Expected persisted block gas wanted.
	}{
		{
			name:              "min gas multiplier path",
			transientGas:      1_000,
			blockGasConsumed:  400,
			minGasMultiplier:  sdkmath.LegacyNewDecWithPrec(50, 2), // 0.50
			expectedGasWanted: 500,
		},
		{
			name:              "block gas used dominates",
			transientGas:      1_000,
			blockGasConsumed:  900,
			minGasMultiplier:  sdkmath.LegacyNewDecWithPrec(50, 2), // 0.50
			expectedGasWanted: 900,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			app := Setup(t)
			ctx := app.BaseApp.NewContext(false)

			params := app.FeeMarketKeeper.GetParams(ctx)
			params.MinGasMultiplier = tc.minGasMultiplier
			require.NoError(t, app.FeeMarketKeeper.SetParams(ctx, params))

			meter := storetypes.NewGasMeter(10_000_000)
			meter.ConsumeGas(tc.blockGasConsumed, "test")
			ctx = ctx.WithBlockGasMeter(meter)

			app.FeeMarketKeeper.SetTransientBlockGasWanted(ctx, tc.transientGas)
			require.NoError(t, app.FeeMarketKeeper.EndBlock(ctx))

			require.Equal(t, tc.expectedGasWanted, app.FeeMarketKeeper.GetBlockGasWanted(ctx))
		})
	}
}

// TestFeeMarketQueryMethods verifies direct keeper query methods return values
// consistent with keeper state.
func TestFeeMarketQueryMethods(t *testing.T) {
	app := Setup(t)
	ctx := app.BaseApp.NewContext(false)
	goCtx := sdk.WrapSDKContext(ctx)

	paramsRes, err := app.FeeMarketKeeper.Params(goCtx, &feemarkettypes.QueryParamsRequest{})
	require.NoError(t, err)
	require.Equal(t, app.FeeMarketKeeper.GetParams(ctx), paramsRes.Params)

	baseFeeRes, err := app.FeeMarketKeeper.BaseFee(goCtx, &feemarkettypes.QueryBaseFeeRequest{})
	require.NoError(t, err)
	require.NotNil(t, baseFeeRes.BaseFee)
	require.True(t, baseFeeRes.BaseFee.Equal(app.FeeMarketKeeper.GetBaseFee(ctx)))

	app.FeeMarketKeeper.SetBlockGasWanted(ctx, 12345)
	blockGasRes, err := app.FeeMarketKeeper.BlockGas(goCtx, &feemarkettypes.QueryBlockGasRequest{})
	require.NoError(t, err)
	require.EqualValues(t, 12345, blockGasRes.Gas)
}

// TestFeeMarketUpdateParamsAuthority verifies MsgUpdateParams authority checks.
func TestFeeMarketUpdateParamsAuthority(t *testing.T) {
	app := Setup(t)
	ctx := app.BaseApp.NewContext(false)
	goCtx := sdk.WrapSDKContext(ctx)

	current := app.FeeMarketKeeper.GetParams(ctx)
	updated := current
	updated.MinGasPrice = current.MinGasPrice.Add(sdkmath.LegacyNewDec(1))

	_, err := app.FeeMarketKeeper.UpdateParams(goCtx, &feemarkettypes.MsgUpdateParams{
		Authority: "not-gov-authority",
		Params:    updated,
	})
	require.Error(t, err)

	govAuthority := authtypes.NewModuleAddress(govtypes.ModuleName).String()
	_, err = app.FeeMarketKeeper.UpdateParams(goCtx, &feemarkettypes.MsgUpdateParams{
		Authority: govAuthority,
		Params:    updated,
	})
	require.NoError(t, err)
	require.Equal(t, updated, app.FeeMarketKeeper.GetParams(ctx))
}

// TestFeeMarketGRPCQueryClient validates gRPC query client wiring for params,
// base fee, and block gas endpoints.
func TestFeeMarketGRPCQueryClient(t *testing.T) {
	app := Setup(t)
	ctx := app.BaseApp.NewContext(false)

	// Set a deterministic block gas value so query assertions are stable.
	app.FeeMarketKeeper.SetBlockGasWanted(ctx, 424242)

	queryHelper := baseapp.NewQueryServerTestHelper(ctx, app.InterfaceRegistry())
	feemarkettypes.RegisterQueryServer(queryHelper, app.FeeMarketKeeper)
	queryClient := feemarkettypes.NewQueryClient(queryHelper)

	paramsRes, err := queryClient.Params(context.Background(), &feemarkettypes.QueryParamsRequest{})
	require.NoError(t, err)
	require.Equal(t, app.FeeMarketKeeper.GetParams(ctx), paramsRes.Params)

	baseFeeRes, err := queryClient.BaseFee(context.Background(), &feemarkettypes.QueryBaseFeeRequest{})
	require.NoError(t, err)
	require.NotNil(t, baseFeeRes.BaseFee)
	require.True(t, baseFeeRes.BaseFee.Equal(app.FeeMarketKeeper.GetBaseFee(ctx)))

	blockGasRes, err := queryClient.BlockGas(context.Background(), &feemarkettypes.QueryBlockGasRequest{})
	require.NoError(t, err)
	require.EqualValues(t, 424242, blockGasRes.Gas)
}
