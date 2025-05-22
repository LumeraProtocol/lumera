package ante

import (
	claimtypes "github.com/LumeraProtocol/lumera/x/claim/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
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

/* THIS CODE IS DISABLED, BECAUSE IT IS BETTER TO REQUIRE A USER TO ASK FOR A CLAIMING WALLET

// EnsureDelayedClaimAccountDecorator makes sure the `NewAddress` contained in
// a MsgDelayedClaim exists as a BaseAccount so the standard ante-decorators
// don’t fail with “account … not found”.
//
// It must be placed BEFORE:
//   - ante.NewValidateMemoDecorator
//   - ante.NewDeductFeeDecorator
//   - ante.NewSetPubKeyDecorator
//
// …basically before the first decorator that touches signer accounts.
type EnsureDelayedClaimAccountDecorator struct {
	AccountKeeper ante.AccountKeeper // <- use the SDK ante interface directly
}

var _ sdk.AnteDecorator = EnsureDelayedClaimAccountDecorator{}

func (d EnsureDelayedClaimAccountDecorator) AnteHandle(
	ctx sdk.Context,
	tx sdk.Tx,
	simulate bool,
	next sdk.AnteHandler,
) (sdk.Context, error) {
	for _, msg := range tx.GetMsgs() {
		if dc, ok := msg.(*claimtypes.MsgDelayedClaim); ok {
			newAddr, err := sdk.AccAddressFromBech32(dc.NewAddress)
			if err != nil {
				return ctx, err
			}

			if acc := d.AccountKeeper.GetAccount(ctx, newAddr); acc == nil {
				// create a stub BaseAccount so later decorators can work
				var emptyCtx context.Context = ctx // sdk.Context implements context.Context
				acc = authtypes.NewBaseAccountWithAddress(newAddr)
				d.AccountKeeper.SetAccount(emptyCtx, acc) // AccountKeeper expects context.Context
			}
		}
	}

	return next(ctx, tx, simulate)
}
*/
