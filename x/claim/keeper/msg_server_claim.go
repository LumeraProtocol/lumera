package keeper

import (
	"context"

	"strconv"
	"time"

	"github.com/LumeraProtocol/lumera/x/claim/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	crypto "github.com/LumeraProtocol/lumera/x/claim/keeper/crypto"
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
	claimsCount, err := k.GetBlockClaimCount(ctx)
	if err != nil {
		return nil, err
	}
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

	err = k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, destAddr, claimRecord.Balance)
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
