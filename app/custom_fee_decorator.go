package app

import (
	errorsmod "cosmossdk.io/errors"
	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
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
	// Loop through all the messages in the transaction
	// Although we assume this transaction will only have one message, we still loop through all the messages
	//TODO: Research if there is a better way to check for the message type and edge cases
	for _, msg := range tx.GetMsgs() {
		if _, ok := msg.(*claimtypes.MsgClaim); ok {
			fee, err := CalculateRequiredFee(ctx, tx)
			if err != nil {
				return ctx, err
			}

			ctx = ctx.WithValue(claimtypes.ClaimTxFee, fee)

			// Fee handling is done in the claims handler

			newCtx := ctx.WithPriority(1)

			return next(newCtx, tx, simulate)
		}
	}

	return cfd.standardFeeDecorator.AnteHandle(ctx, tx, simulate, next)
}

func CalculateRequiredFee(ctx sdk.Context, tx sdk.Tx) (sdk.Coins, error) {
	feeTx, ok := tx.(sdk.FeeTx)
	if !ok {
		return nil, errorsmod.Wrap(sdkerrors.ErrTxDecode, "Tx must be a FeeTx")
	}

	gas := feeTx.GetGas()
	if gas == 0 {
		return nil, errorsmod.Wrap(sdkerrors.ErrInvalidGasLimit, "must provide positive gas")
	}

	minGasPrices := ctx.MinGasPrices()

	if minGasPrices.IsZero() {
		return sdk.NewCoins(), nil
	}

	requiredFees := make(sdk.Coins, len(minGasPrices))
	gasLimitDec := sdkmath.LegacyNewDec(int64(gas))

	for i, gp := range minGasPrices {
		fee := gp.Amount.Mul(gasLimitDec)
		requiredFees[i] = sdk.NewCoin(gp.Denom, fee.Ceil().RoundInt())
	}
	return requiredFees, nil
}
