package keeper

import (
	"context"
	"strconv"
	"time"

	errorsmod "cosmossdk.io/errors"

	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/pastelnetwork/pastel/x/claim/types"

	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	crypto "github.com/pastelnetwork/pastel/x/claim/keeper/crypto"
)

func (k msgServer) Claim(goCtx context.Context, msg *types.MsgClaim) (*types.MsgClaimResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	params := k.GetParams(ctx)

	if !params.EnableClaims {
		return nil, types.ErrClaimDisabled
	}

	// check if claim period expired
	claimEndTime := time.Unix(params.ClaimEndTime, 0)
	if ctx.BlockTime().After(claimEndTime) {
		return nil, types.ErrClaimPeriodExpired
	}

	// Check claims per block limit
	claimsCount := k.GetBlockClaimCount(ctx)
	if claimsCount >= params.MaxClaimsPerBlock {
		return nil, types.ErrTooManyClaims
	}

	// Retrieve the claim record from the store
	claimRecord, found, err := k.GetClaimRecord(ctx, msg.OldAddress)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, types.ErrClaimNotFound
	}

	// Check if already claimed
	if claimRecord.Claimed {
		return nil, types.ErrClaimAlreadyClaimed
	}

	// Verify address reconstruction and signature
	reconstructedAddress, err := crypto.GetAddressFromPubKey(msg.PubKey)
	if err != nil {
		return nil, types.ErrInvalidPubKey
	}

	if reconstructedAddress != msg.OldAddress {
		return nil, types.ErrMismatchReconstructedAddr
	}

	// Construct message for signature verification
	verificationMessage := msg.OldAddress + "." + msg.PubKey + "." + msg.NewAddress

	valid, err := crypto.VerifySignature(msg.PubKey, verificationMessage, msg.Signature)
	if err != nil {
		return nil, types.ErrInvalidSignature
	}
	if !valid {
		return nil, types.ErrInvalidSignature
	}

	// Increment block claims counter before processing
	k.IncrementBlockClaimCount(ctx)

	destAddr, err := sdk.AccAddressFromBech32(msg.NewAddress)
	if err != nil {
		return nil, err
	}

	// Check if account exists, create if it doesn't
	acc := k.accountKeeper.GetAccount(ctx, destAddr)
	if acc == nil {
		acc = k.accountKeeper.NewAccountWithAddress(ctx, destAddr)
		k.accountKeeper.SetAccount(ctx, acc)
	}

	// Get fee from context
	feeValue := ctx.Value(types.ClaimTxFee)
	fee, ok := feeValue.(sdk.Coins)
	if !ok {
		return nil, sdkerrors.ErrInvalidCoins
	}

	// fee should be smaller than the claim balance
	if fee.IsAllGTE(claimRecord.Balance) {
		return nil, sdkerrors.ErrInsufficientFee
	}

	err = k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, destAddr, claimRecord.Balance)
	if err != nil {
		return nil, err
	}

	// deduct the fee from the new account
	err = deductClaimFee(k.bankKeeper, ctx, acc, fee)
	if err != nil {
		return nil, err
	}

	// Mark the claim as processed
	claimRecord.Claimed = true
	claimRecord.ClaimTime = ctx.BlockTime().Unix()
	ClaimTimeString := strconv.FormatInt(claimRecord.ClaimTime, 10)
	err = k.SetClaimRecord(ctx, claimRecord)
	if err != nil {
		return nil, err
	}
	// Emit claim processed event
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			types.EventTypeClaimProcessed,
			sdk.NewAttribute(sdk.AttributeKeyModule, types.ModuleName),
			sdk.NewAttribute(sdk.AttributeKeyAmount, claimRecord.Balance.String()),
			sdk.NewAttribute(types.AttributeKeyOldAddress, msg.OldAddress),
			sdk.NewAttribute(types.AttributeKeyNewAddress, msg.NewAddress),
			sdk.NewAttribute(types.AttributeKeyClaimTime, ClaimTimeString),
		),
	)

	return &types.MsgClaimResponse{}, nil
}

func deductClaimFee(bankKeeper types.BankKeeper, ctx sdk.Context, acc sdk.AccountI, fees sdk.Coins) error {
	if !fees.IsValid() {
		return errorsmod.Wrapf(sdkerrors.ErrInsufficientFee, "invalid fee amount: %s", fees)
	}

	err := bankKeeper.SendCoinsFromAccountToModule(ctx, acc.GetAddress(), authtypes.FeeCollectorName, fees)
	if err != nil {
		return errorsmod.Wrapf(sdkerrors.ErrInsufficientFunds, err.Error())
	}

	events := sdk.Events{
		sdk.NewEvent(
			sdk.EventTypeTx,
			sdk.NewAttribute(sdk.AttributeKeyFee, fees.String()),
			sdk.NewAttribute(sdk.AttributeKeyFeePayer, acc.String()),
		),
	}
	ctx.EventManager().EmitEvents(events)
	return nil
}
