package ante

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/auth/ante"
	"github.com/cosmos/cosmos-sdk/x/auth/types"
	claimtypes "github.com/pastelnetwork/pastel/x/claim/types"
)

type CustomDeductFeeDecorator struct {
	accountKeeper        ante.AccountKeeper
	bankKeeper           types.BankKeeper
	feegrantKeeper       ante.FeegrantKeeper
	txFeeChecker         ante.TxFeeChecker
	standardFeeDecorator ante.DeductFeeDecorator
}

func NewCustomDeductFeeDecorator(ak ante.AccountKeeper, bk types.BankKeeper, fk ante.FeegrantKeeper, tfc ante.TxFeeChecker) CustomDeductFeeDecorator {
	return CustomDeductFeeDecorator{
		accountKeeper:        ak,
		bankKeeper:           bk,
		feegrantKeeper:       fk,
		txFeeChecker:         tfc,
		standardFeeDecorator: ante.NewDeductFeeDecorator(ak, bk, fk, tfc),
	}
}

func (cfd CustomDeductFeeDecorator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (sdk.Context, error) {
	if len(tx.GetMsgs()) == 1 {
		msg := tx.GetMsgs()[0]
		if _, ok := msg.(*claimtypes.MsgClaim); ok {
			return next(ctx, tx, simulate)
		}
	}
	return cfd.standardFeeDecorator.AnteHandle(ctx, tx, simulate, next)
}
