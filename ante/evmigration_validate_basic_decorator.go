package ante

import (
	"errors"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

// EVMigrationValidateBasicDecorator preserves the SDK's transaction-level basic
// validation while allowing migration-only txs to omit Cosmos signatures.
// Those txs authenticate inside the message payload, so ErrNoSignatures is
// expected and should not block execution.
type EVMigrationValidateBasicDecorator struct{}

var _ sdk.AnteDecorator = EVMigrationValidateBasicDecorator{}

func (d EVMigrationValidateBasicDecorator) AnteHandle(
	ctx sdk.Context,
	tx sdk.Tx,
	simulate bool,
	next sdk.AnteHandler,
) (sdk.Context, error) {
	if ctx.IsReCheckTx() {
		return next(ctx, tx, simulate)
	}

	validateBasic, ok := tx.(sdk.HasValidateBasic)
	if !ok {
		return ctx, errorsmod.Wrap(sdkerrors.ErrTxDecode, "invalid transaction type")
	}

	if err := validateBasic.ValidateBasic(); err != nil {
		if !IsEVMigrationOnlyTx(tx) || !errors.Is(err, sdkerrors.ErrNoSignatures) {
			return ctx, err
		}
	}

	return next(ctx, tx, simulate)
}
