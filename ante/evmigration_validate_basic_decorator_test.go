package ante

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	evmigrationtypes "github.com/LumeraProtocol/lumera/x/evmigration/types"
)

type mockValidateBasicTx struct {
	msgs []sdk.Msg
	err  error
}

func (m mockValidateBasicTx) GetMsgs() []sdk.Msg { return m.msgs }

func (m mockValidateBasicTx) GetMsgsV2() ([]proto.Message, error) { return nil, nil }

func (m mockValidateBasicTx) ValidateBasic() error { return m.err }

func noopAnteHandler(ctx sdk.Context, tx sdk.Tx, simulate bool) (sdk.Context, error) {
	return ctx, nil
}

// TestEVMigrationValidateBasicDecorator_AllowsMissingTxSignatures verifies that
// migration-only txs can omit Cosmos tx signatures while still using tx-level
// basic validation for all other errors.
func TestEVMigrationValidateBasicDecorator_AllowsMissingTxSignatures(t *testing.T) {
	t.Parallel()

	dec := EVMigrationValidateBasicDecorator{}
	tx := mockValidateBasicTx{
		msgs: []sdk.Msg{&evmigrationtypes.MsgClaimLegacyAccount{}},
		err:  sdkerrors.ErrNoSignatures,
	}

	called := false
	_, err := dec.AnteHandle(sdk.Context{}, tx, false, func(ctx sdk.Context, tx sdk.Tx, simulate bool) (sdk.Context, error) {
		called = true
		return ctx, nil
	})
	require.NoError(t, err)
	require.True(t, called)
}

// TestEVMigrationValidateBasicDecorator_RejectsOtherErrors verifies that the
// decorator only suppresses ErrNoSignatures for migration-only txs.
func TestEVMigrationValidateBasicDecorator_RejectsOtherErrors(t *testing.T) {
	t.Parallel()

	dec := EVMigrationValidateBasicDecorator{}
	tx := mockValidateBasicTx{
		msgs: []sdk.Msg{&evmigrationtypes.MsgClaimLegacyAccount{}},
		err:  sdkerrors.ErrInvalidAddress,
	}

	_, err := dec.AnteHandle(sdk.Context{}, tx, false, noopAnteHandler)
	require.ErrorIs(t, err, sdkerrors.ErrInvalidAddress)
}

// TestEVMigrationValidateBasicDecorator_NonMigrationStillRequiresSigs verifies
// that regular txs keep the SDK's no-signature rejection.
func TestEVMigrationValidateBasicDecorator_NonMigrationStillRequiresSigs(t *testing.T) {
	t.Parallel()

	dec := EVMigrationValidateBasicDecorator{}
	tx := mockValidateBasicTx{
		msgs: []sdk.Msg{&banktypes.MsgSend{}},
		err:  sdkerrors.ErrNoSignatures,
	}

	_, err := dec.AnteHandle(sdk.Context{}, tx, false, noopAnteHandler)
	require.ErrorIs(t, err, sdkerrors.ErrNoSignatures)
}
