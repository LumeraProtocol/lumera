package ante

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	evmigrationtypes "github.com/LumeraProtocol/lumera/x/evmigration/types"
)

// EVMigrationFeeDecorator must be placed BEFORE MinGasPriceDecorator
// in the AnteHandler chain. If every message inside the tx is a migration
// message (MsgClaimLegacyAccount or MsgMigrateValidator) we clear
// min-gas-prices, allowing zero-fee txs. This solves the chicken-and-egg
// problem where the new address has zero balance before migration.
type EVMigrationFeeDecorator struct{}

var _ sdk.AnteDecorator = EVMigrationFeeDecorator{}

func (d EVMigrationFeeDecorator) AnteHandle(
	ctx sdk.Context,
	tx sdk.Tx,
	simulate bool,
	next sdk.AnteHandler,
) (sdk.Context, error) {
	for _, msg := range tx.GetMsgs() {
		switch msg.(type) {
		case *evmigrationtypes.MsgClaimLegacyAccount,
			*evmigrationtypes.MsgMigrateValidator:
			continue
		default:
			// Non-migration message in tx — run normal fee checks.
			return next(ctx, tx, simulate)
		}
	}

	// All messages are migration messages — waive the fee.
	ctx = ctx.WithMinGasPrices(nil)

	return next(ctx, tx, simulate)
}
