package keeper

import (
	"context"
	types2 "github.com/LumeraProtocol/lumera/x/supernode/v1/types"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

func (k msgServer) StartSupernode(goCtx context.Context, msg *types2.MsgStartSupernode) (*types2.MsgStartSupernodeResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	valOperAddr, err := sdk.ValAddressFromBech32(msg.ValidatorAddress)
	if err != nil {
		return nil, errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid validator address: %s", err)
	}

	supernode, found := k.QuerySuperNode(ctx, valOperAddr)
	if !found {
		return nil, errorsmod.Wrapf(sdkerrors.ErrNotFound, "no supernode found for validator %s", msg.ValidatorAddress)
	}

	if err := VerifyValidatorOperator(valOperAddr, msg.Creator); err != nil {
		return nil, err
	}

	if len(supernode.States) == 0 || supernode.States[len(supernode.States)-1].State != types2.SuperNodeStateActive {
		supernode.States = append(supernode.States, &types2.SuperNodeStateRecord{
			State:  types2.SuperNodeStateActive,
			Height: ctx.BlockHeight(),
		})
	} else {
		return nil, errorsmod.Wrapf(sdkerrors.ErrInvalidRequest, "supernode is already started")
	}

	if err := k.SetSuperNode(ctx, supernode); err != nil {
		return nil, err
	}

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			types2.EventTypeSupernodeStarted,
			sdk.NewAttribute(types2.AttributeKeyValidatorAddress, msg.ValidatorAddress),
		),
	)

	return &types2.MsgStartSupernodeResponse{}, nil
}
