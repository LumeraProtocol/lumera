package ante

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	claimtypes "github.com/LumeraProtocol/lumera/x/claim/types"
)

// DelayedClaimFeeDecorator must be placed BEFORE authante.NewMempoolFeeDecorator
// in the AnteHandler chain. If every message inside the tx is a
// claimtypes.MsgDelayedClaim we clear min-gas-prices, allowing zero-fee txs.
type DelayedClaimFeeDecorator struct{}

var _ sdk.AnteDecorator = DelayedClaimFeeDecorator{}

func (d DelayedClaimFeeDecorator) AnteHandle(
	ctx sdk.Context,
	tx sdk.Tx,
	simulate bool,
	next sdk.AnteHandler,
) (sdk.Context, error) {
	for _, msg := range tx.GetMsgs() {
		if _, ok := msg.(*claimtypes.MsgDelayedClaim); !ok {
			// Some other message exists – run normal fee checks.
			return next(ctx, tx, simulate)
		}
	}

	// All messages are MsgDelayedClaim – waive the fee.
	ctx = ctx.WithMinGasPrices(nil)

	return next(ctx, tx, simulate)
}
