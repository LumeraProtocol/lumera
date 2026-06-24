package evm

import (
	"errors"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

type testAnteDecorator struct {
	called bool
	err    error
}

func (d *testAnteDecorator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (sdk.Context, error) {
	d.called = true
	if d.err != nil {
		return ctx, d.err
	}
	return next(ctx, tx, simulate)
}

// TestGenesisSkipDecorator_GenesisHeight verifies the wrapped decorator is
// bypassed at height 0 so genesis/gentx processing can continue.
func TestGenesisSkipDecorator_GenesisHeight(t *testing.T) {
	inner := &testAnteDecorator{err: errors.New("inner should be skipped")}
	dec := genesisSkipDecorator{inner: inner}
	nextCalled := false

	_, err := dec.AnteHandle(
		sdk.Context{}.WithBlockHeight(0),
		nil,
		false,
		func(ctx sdk.Context, tx sdk.Tx, simulate bool) (sdk.Context, error) {
			nextCalled = true
			return ctx, nil
		},
	)
	require.NoError(t, err)
	require.False(t, inner.called, "inner decorator must be skipped at genesis height")
	require.True(t, nextCalled, "next handler should be called when skipping inner decorator")
}

// TestGenesisSkipDecorator_NonGenesisHeight verifies normal execution delegates
// to the wrapped decorator for non-genesis blocks.
func TestGenesisSkipDecorator_NonGenesisHeight(t *testing.T) {
	innerErr := errors.New("inner called")
	inner := &testAnteDecorator{err: innerErr}
	dec := genesisSkipDecorator{inner: inner}

	_, err := dec.AnteHandle(
		sdk.Context{}.WithBlockHeight(1),
		nil,
		false,
		func(ctx sdk.Context, tx sdk.Tx, simulate bool) (sdk.Context, error) {
			return ctx, nil
		},
	)
	require.ErrorIs(t, err, innerErr)
	require.True(t, inner.called, "inner decorator must run on non-genesis heights")
}
