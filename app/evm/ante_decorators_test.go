package evm_test

import (
	"context"
	"testing"
	"time"

	cosmosante "github.com/cosmos/evm/ante/cosmos"
	evmencoding "github.com/cosmos/evm/encoding"
	evmtestutil "github.com/cosmos/evm/testutil"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/stretchr/testify/require"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	sdkvesting "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
	"github.com/cosmos/cosmos-sdk/x/authz"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"

	lcfg "github.com/LumeraProtocol/lumera/config"
)

// TestRejectMessagesDecorator verifies Cosmos-path rejection rules for MsgEthereumTx.
//
// Matrix:
// - MsgEthereumTx inside a regular Cosmos tx should be rejected.
// - A normal Cosmos message should pass through unchanged.
func TestRejectMessagesDecorator(t *testing.T) {
	// Build encoding + signer material once, then drive decorator behavior directly.
	encodingCfg := evmencoding.MakeConfig(lcfg.EVMChainID)
	testPrivKeys, testAddresses, err := evmtestutil.GeneratePrivKeyAddressPairs(2)
	require.NoError(t, err)

	dec := cosmosante.NewRejectMessagesDecorator()
	ctx := sdk.Context{}

	t.Run("rejects MsgEthereumTx outside extension tx", func(t *testing.T) {
		tx, err := evmtestutil.CreateTx(
			context.Background(),
			encodingCfg.TxConfig,
			testPrivKeys[0],
			&evmtypes.MsgEthereumTx{},
		)
		require.NoError(t, err)

		_, err = dec.AnteHandle(ctx, tx, false, evmtestutil.NoOpNextFn)
		require.Error(t, err)
		require.ErrorIs(t, err, sdkerrors.ErrInvalidType)
		require.Contains(t, err.Error(), "ExtensionOptionsEthereumTx")
	})

	t.Run("allows standard cosmos messages", func(t *testing.T) {
		msg := banktypes.NewMsgSend(
			testAddresses[0],
			testAddresses[1],
			sdk.NewCoins(sdk.NewInt64Coin(lcfg.ChainDenom, 1)),
		)
		tx, err := evmtestutil.CreateTx(
			context.Background(),
			encodingCfg.TxConfig,
			testPrivKeys[0],
			msg,
		)
		require.NoError(t, err)

		_, err = dec.AnteHandle(ctx, tx, false, evmtestutil.NoOpNextFn)
		require.NoError(t, err)
	})
}

// TestAuthzLimiterDecorator verifies authz guardrails configured in the Cosmos ante path.
//
// Matrix:
// - Blocked msg type nested in MsgExec -> rejected.
// - Blocked authorization in MsgGrant -> rejected.
// - Blocked msg type at top-level (non-authz) -> allowed.
// - Non-blocked authorization in MsgGrant -> allowed.
// - Nested MsgGrant containing blocked type -> rejected.
// - Over-nested MsgExec tree -> rejected.
// - Two nested MsgExec trees over cumulative depth limit -> rejected.
// - Valid non-blocked authz flow -> allowed.
func TestAuthzLimiterDecorator(t *testing.T) {
	encodingCfg := evmencoding.MakeConfig(lcfg.EVMChainID)
	testPrivKeys, testAddresses, err := evmtestutil.GeneratePrivKeyAddressPairs(4)
	require.NoError(t, err)

	dec := cosmosante.NewAuthzLimiterDecorator(
		sdk.MsgTypeURL(&evmtypes.MsgEthereumTx{}),
		sdk.MsgTypeURL(&sdkvesting.MsgCreateVestingAccount{}),
	)

	// MsgGrant requires an expiration when created through helper constructors.
	distantFuture := time.Date(9000, 1, 1, 0, 0, 0, 0, time.UTC)
	ctx := sdk.Context{}

	t.Run("rejects blocked message nested in MsgExec", func(t *testing.T) {
		tx, err := evmtestutil.CreateTx(
			context.Background(),
			encodingCfg.TxConfig,
			testPrivKeys[0],
			evmtestutil.NewMsgExec(
				testAddresses[0],
				[]sdk.Msg{&evmtypes.MsgEthereumTx{}},
			),
		)
		require.NoError(t, err)

		_, err = dec.AnteHandle(ctx, tx, false, evmtestutil.NoOpNextFn)
		require.Error(t, err)
		require.ErrorIs(t, err, sdkerrors.ErrUnauthorized)
		require.Contains(t, err.Error(), "disabled msg type")
	})

	t.Run("rejects blocked authorization in MsgGrant", func(t *testing.T) {
		tx, err := evmtestutil.CreateTx(
			context.Background(),
			encodingCfg.TxConfig,
			testPrivKeys[0],
			evmtestutil.NewMsgGrant(
				testAddresses[0],
				testAddresses[1],
				authz.NewGenericAuthorization(sdk.MsgTypeURL(&evmtypes.MsgEthereumTx{})),
				&distantFuture,
			),
		)
		require.NoError(t, err)

		_, err = dec.AnteHandle(ctx, tx, false, evmtestutil.NoOpNextFn)
		require.Error(t, err)
		require.ErrorIs(t, err, sdkerrors.ErrUnauthorized)
		require.Contains(t, err.Error(), "disabled msg type")
	})

	t.Run("allows blocked type when not wrapped in authz", func(t *testing.T) {
		tx, err := evmtestutil.CreateTx(
			context.Background(),
			encodingCfg.TxConfig,
			testPrivKeys[0],
			&evmtypes.MsgEthereumTx{},
		)
		require.NoError(t, err)

		_, err = dec.AnteHandle(ctx, tx, false, evmtestutil.NoOpNextFn)
		require.NoError(t, err)
	})

	t.Run("allows non-blocked authorization in MsgGrant", func(t *testing.T) {
		tx, err := evmtestutil.CreateTx(
			context.Background(),
			encodingCfg.TxConfig,
			testPrivKeys[0],
			evmtestutil.NewMsgGrant(
				testAddresses[0],
				testAddresses[1],
				authz.NewGenericAuthorization(sdk.MsgTypeURL(&banktypes.MsgSend{})),
				&distantFuture,
			),
		)
		require.NoError(t, err)

		_, err = dec.AnteHandle(ctx, tx, false, evmtestutil.NoOpNextFn)
		require.NoError(t, err)
	})

	t.Run("rejects nested MsgGrant containing blocked authorization", func(t *testing.T) {
		tx, err := evmtestutil.CreateTx(
			context.Background(),
			encodingCfg.TxConfig,
			testPrivKeys[0],
			evmtestutil.NewMsgExec(
				testAddresses[1],
				[]sdk.Msg{
					evmtestutil.NewMsgGrant(
						testAddresses[0],
						testAddresses[1],
						authz.NewGenericAuthorization(sdk.MsgTypeURL(&evmtypes.MsgEthereumTx{})),
						&distantFuture,
					),
				},
			),
		)
		require.NoError(t, err)

		_, err = dec.AnteHandle(ctx, tx, false, evmtestutil.NoOpNextFn)
		require.Error(t, err)
		require.ErrorIs(t, err, sdkerrors.ErrUnauthorized)
		require.Contains(t, err.Error(), "disabled msg type")
	})

	t.Run("rejects excessive nested MsgExec depth", func(t *testing.T) {
		tx, err := evmtestutil.CreateTx(
			context.Background(),
			encodingCfg.TxConfig,
			testPrivKeys[0],
			evmtestutil.CreateNestedMsgExec(
				testAddresses[0],
				6, // max allowed depth is < 7 in cosmos/evm ante/cosmos/authz.go
				[]sdk.Msg{
					banktypes.NewMsgSend(
						testAddresses[0],
						testAddresses[2],
						sdk.NewCoins(sdk.NewInt64Coin(lcfg.ChainDenom, 1)),
					),
				},
			),
		)
		require.NoError(t, err)

		_, err = dec.AnteHandle(ctx, tx, false, evmtestutil.NoOpNextFn)
		require.Error(t, err)
		require.ErrorIs(t, err, sdkerrors.ErrUnauthorized)
		require.Contains(t, err.Error(), "more nested msgs than permitted")
	})

	t.Run("rejects cumulative nested MsgExec depth across tx messages", func(t *testing.T) {
		tx, err := evmtestutil.CreateTx(
			context.Background(),
			encodingCfg.TxConfig,
			testPrivKeys[0],
			evmtestutil.CreateNestedMsgExec(
				testAddresses[0],
				5,
				[]sdk.Msg{
					banktypes.NewMsgSend(
						testAddresses[0],
						testAddresses[2],
						sdk.NewCoins(sdk.NewInt64Coin(lcfg.ChainDenom, 1)),
					),
				},
			),
			evmtestutil.CreateNestedMsgExec(
				testAddresses[0],
				5,
				[]sdk.Msg{
					banktypes.NewMsgSend(
						testAddresses[0],
						testAddresses[2],
						sdk.NewCoins(sdk.NewInt64Coin(lcfg.ChainDenom, 1)),
					),
				},
			),
		)
		require.NoError(t, err)

		_, err = dec.AnteHandle(ctx, tx, false, evmtestutil.NoOpNextFn)
		require.Error(t, err)
		require.ErrorIs(t, err, sdkerrors.ErrUnauthorized)
		require.Contains(t, err.Error(), "more nested msgs than permitted")
	})

	t.Run("allows valid non-blocked authz flow", func(t *testing.T) {
		msgExec := evmtestutil.NewMsgExec(
			testAddresses[0],
			[]sdk.Msg{
				banktypes.NewMsgSend(
					testAddresses[1],
					testAddresses[3],
					sdk.NewCoins(sdk.NewInt64Coin(lcfg.ChainDenom, 1)),
				),
			},
		)
		tx, err := evmtestutil.CreateTx(
			context.Background(),
			encodingCfg.TxConfig,
			testPrivKeys[1],
			msgExec,
		)
		require.NoError(t, err)

		_, err = dec.AnteHandle(ctx, tx, false, evmtestutil.NoOpNextFn)
		require.NoError(t, err)
	})
}
