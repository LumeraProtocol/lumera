package keeper

import (
	"context"
	sdkerrors "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/pastelnetwork/pasteld/x/pastelid/types"
)

func (k msgServer) CreatePastelId(goCtx context.Context, msg *types.MsgCreatePastelId) (*types.MsgCreatePastelIdResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// Validate address
	addr, err := types.ValidateAddress(msg.Creator)
	if err != nil {
		return nil, err
	}

	// Check if PastelID exists
	if k.HasPastelidEntry(ctx, msg.Creator) {
		return nil, sdkerrors.Wrap(types.ErrPastelIDExists, msg.Creator)
	}

	// Get creation fee from params
	params := k.GetParams(ctx)

	//Check if the account has enough funds to cover the fee
	if !k.bankKeeper.SpendableCoins(ctx, addr).IsAllGTE(sdk.NewCoins(params.PastelIdCreateFee)) {
		return nil, sdkerrors.Wrap(types.ErrInsufficientFunds, "insufficient funds")
	}

	// Create PastelID entry
	var pastelidEntry = types.PastelidEntry{
		Address:   msg.Creator,
		IdType:    msg.IdType,
		PastelId:  msg.PastelId,
		PqKey:     msg.PqKey,
		Signature: msg.Signature,
		TimeStamp: msg.TimeStamp,
		Version:   msg.Version,
	}

	// Store PastelID
	k.SetPastelidEntry(ctx, pastelidEntry)

	// Burn the creation fee
	err = k.bankKeeper.SendCoinsFromAccountToModule(
		ctx,
		addr,
		types.ModuleName,
		sdk.NewCoins(params.PastelIdCreateFee),
	)
	if err != nil {
		return nil, err
	}

	// Emit event
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			types.EventTypePastelIDCreated,
			sdk.NewAttribute(types.AttributeKeyAddress, msg.Creator),
			sdk.NewAttribute(types.AttributeKeyPastelID, msg.PastelId),
			sdk.NewAttribute(types.AttributeKeyTimestamp, msg.TimeStamp),
		),
	)

	return &types.MsgCreatePastelIdResponse{}, nil
}
