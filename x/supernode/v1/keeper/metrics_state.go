package keeper

import (
	"strings"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

	"github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

func (k Keeper) markPostponed(ctx sdk.Context, sn *types.SuperNode, reason string) error {
	if len(sn.States) == 0 {
		return errorsmod.Wrap(sdkerrors.ErrInvalidRequest, "supernode state missing")
	}
	last := sn.States[len(sn.States)-1]
	if last.State == types.SuperNodeStatePostponed {
		return nil
	}
	sn.States = append(sn.States, &types.SuperNodeStateRecord{State: types.SuperNodeStatePostponed, Height: ctx.BlockHeight()})
	if err := k.SetSuperNode(ctx, *sn); err != nil {
		return err
	}
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			types.EventTypeSupernodePostponed,
			sdk.NewAttribute(types.AttributeKeyValidatorAddress, sn.ValidatorAddress),
			sdk.NewAttribute(types.AttributeKeyOldState, last.State.String()),
			sdk.NewAttribute(types.AttributeKeyReason, reason),
			sdk.NewAttribute(types.AttributeKeyHeight, stringHeight(ctx.BlockHeight())),
		),
	)
	return nil
}

func (k Keeper) recoverFromPostponed(ctx sdk.Context, sn *types.SuperNode, target types.SuperNodeState) error {
	if target == types.SuperNodeStateUnspecified {
		target = types.SuperNodeStateActive
	}
	sn.States = append(sn.States, &types.SuperNodeStateRecord{State: target, Height: ctx.BlockHeight()})
	if err := k.SetSuperNode(ctx, *sn); err != nil {
		return err
	}
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			types.EventTypeSupernodeRecovered,
			sdk.NewAttribute(types.AttributeKeyValidatorAddress, sn.ValidatorAddress),
			sdk.NewAttribute(types.AttributeKeyOldState, types.SuperNodeStatePostponed.String()),
			sdk.NewAttribute(types.AttributeKeyHeight, stringHeight(ctx.BlockHeight())),
		),
	)
	return nil
}

func stringHeight(height int64) string {
	return strings.TrimSpace(sdk.NewInt(height).String())
}
