package evm_test

import (
	"errors"
	"testing"

	evmantedecorators "github.com/cosmos/evm/ante/evm"
	utiltx "github.com/cosmos/evm/testutil/tx"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/stretchr/testify/require"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

// TestGasWantedDecoratorMatrix verifies gas-wanted accounting and block-gas
// guardrails for London-era blocks.
//
// Matrix:
// - Adds gas cumulatively to transient feemarket state.
// - Skips accumulation when base fee is disabled.
// - Rejects txs above block gas limit.
// - Ignores non-FeeTx inputs.
// - Propagates feemarket transient-store errors.
func TestGasWantedDecoratorMatrix(t *testing.T) {
	// The decorator consults global EVM chain config to determine London activation.
	ensureChainConfigInitialized(t)

	t.Run("tracks cumulative transient gas wanted", func(t *testing.T) {
		params := feemarkettypes.DefaultParams()
		params.NoBaseFee = false
		params.EnableHeight = 0
		keeper := &mockAnteFeeMarketKeeper{params: params}

		dec := evmantedecorators.NewGasWantedDecorator(nil, keeper, &params)
		ctx := newGasWantedContext(1, 1_000_000)

		_, err := dec.AnteHandle(ctx, mockFeeTx{gas: 21_000}, false, noopAnteHandler)
		require.NoError(t, err)
		_, err = dec.AnteHandle(ctx, mockFeeTx{gas: 33_000}, false, noopAnteHandler)
		require.NoError(t, err)

		require.EqualValues(t, 54_000, keeper.gasWanted)
	})

	t.Run("skips accumulation when base fee disabled", func(t *testing.T) {
		params := feemarkettypes.DefaultParams()
		params.NoBaseFee = true
		params.EnableHeight = 0
		keeper := &mockAnteFeeMarketKeeper{params: params, gasWanted: 7}

		dec := evmantedecorators.NewGasWantedDecorator(nil, keeper, &params)
		ctx := newGasWantedContext(1, 1_000_000)

		_, err := dec.AnteHandle(ctx, mockFeeTx{gas: 21_000}, false, noopAnteHandler)
		require.NoError(t, err)
		require.EqualValues(t, 7, keeper.gasWanted)
	})

	t.Run("rejects tx gas above block gas limit", func(t *testing.T) {
		params := feemarkettypes.DefaultParams()
		params.NoBaseFee = false
		params.EnableHeight = 0
		keeper := &mockAnteFeeMarketKeeper{params: params}

		dec := evmantedecorators.NewGasWantedDecorator(nil, keeper, &params)
		ctx := newGasWantedContext(1, 100)

		_, err := dec.AnteHandle(ctx, mockFeeTx{gas: 101}, false, noopAnteHandler)
		require.Error(t, err)
		require.ErrorIs(t, err, sdkerrors.ErrOutOfGas)
		require.Contains(t, err.Error(), "exceeds block gas limit")
		require.EqualValues(t, 0, keeper.gasWanted)
	})

	t.Run("ignores non fee tx", func(t *testing.T) {
		params := feemarkettypes.DefaultParams()
		params.NoBaseFee = false
		params.EnableHeight = 0
		keeper := &mockAnteFeeMarketKeeper{params: params}

		dec := evmantedecorators.NewGasWantedDecorator(nil, keeper, &params)
		ctx := newGasWantedContext(1, 1_000_000)

		_, err := dec.AnteHandle(ctx, &utiltx.InvalidTx{}, false, noopAnteHandler)
		require.NoError(t, err)
		require.EqualValues(t, 0, keeper.gasWanted)
	})

	t.Run("surfaces transient gas accumulation errors", func(t *testing.T) {
		params := feemarkettypes.DefaultParams()
		params.NoBaseFee = false
		params.EnableHeight = 0
		keeper := &mockAnteFeeMarketKeeper{
			params: params,
			addErr: errors.New("boom"),
		}

		dec := evmantedecorators.NewGasWantedDecorator(nil, keeper, &params)
		ctx := newGasWantedContext(1, 1_000_000)

		_, err := dec.AnteHandle(ctx, mockFeeTx{gas: 21_000}, false, noopAnteHandler)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to add gas wanted to transient store")
	})
}

// ensureChainConfigInitialized sets a default chain config when tests run
// outside a full app/bootstrap flow.
func ensureChainConfigInitialized(t *testing.T) {
	t.Helper()

	if evmtypes.GetChainConfig() != nil {
		return
	}
	require.NoError(t, evmtypes.SetChainConfig(nil))
}

// newGasWantedContext creates a minimal SDK context with consensus max gas so
// BlockGasLimit(ctx) has deterministic behavior.
func newGasWantedContext(height int64, maxGas int64) sdk.Context {
	return sdk.Context{}.
		WithBlockHeight(height).
		WithConsensusParams(tmproto.ConsensusParams{
			Block: &tmproto.BlockParams{
				MaxGas:   maxGas,
				MaxBytes: 22020096,
			},
		})
}

func noopAnteHandler(ctx sdk.Context, tx sdk.Tx, simulate bool) (sdk.Context, error) {
	return ctx, nil
}

type mockAnteFeeMarketKeeper struct {
	params    feemarkettypes.Params // Params returned by GetParams().
	gasWanted uint64                // In-memory transient gas counter.
	addErr    error                 // Optional injected error for AddTransientGasWanted().
}

func (m *mockAnteFeeMarketKeeper) GetParams(ctx sdk.Context) feemarkettypes.Params {
	return m.params
}

func (m *mockAnteFeeMarketKeeper) AddTransientGasWanted(ctx sdk.Context, gasWanted uint64) (uint64, error) {
	if m.addErr != nil {
		return 0, m.addErr
	}
	m.gasWanted += gasWanted
	return m.gasWanted, nil
}
