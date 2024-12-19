package keeper

import (
	"context"
	"fmt"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/pastelnetwork/pastel/x/supernode/types"
)

func (k msgServer) StopSupernode(goCtx context.Context, msg *types.MsgStopSupernode) (*types.MsgStopSupernodeResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	valOperAddr, err := sdk.ValAddressFromBech32(msg.ValidatorAddress)
	if err != nil {
		return nil, errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid validator address: %s", err)
	}

	supernode, found := k.QuerySuperNode(ctx, valOperAddr)
	if !found {
		return nil, errorsmod.Wrapf(sdkerrors.ErrNotFound, "no supernode found for validator %s", msg.ValidatorAddress)
	}

	if err := k.verifyValidatorOperator(ctx, valOperAddr, msg.Creator); err != nil {
		return nil, err
	}

	supernode.State = types.SuperNodeStateStopped
	supernode.LastTimeDisabled = ctx.BlockTime()
	fmt.Println("supernode.LastTimeDisabled", supernode.LastTimeDisabled)

	if err := k.SetSuperNode(ctx, supernode); err != nil {
		return nil, err
	}

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			types.EventTypeSupernodeStopped,
			sdk.NewAttribute(types.AttributeKeyValidatorAddress, msg.ValidatorAddress),
			sdk.NewAttribute(types.AttributeKeyReason, msg.Reason),
		),
	)

	return &types.MsgStopSupernodeResponse{}, nil
}
