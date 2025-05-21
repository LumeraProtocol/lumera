package keeper

import (
	"context"

	types2 "github.com/LumeraProtocol/lumera/x/action/v1/types"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// ApproveAction is the message handler for MsgApproveAction
// This handles the RPC call and delegates to the keeper method
func (k msgServer) ApproveAction(goCtx context.Context, msg *types2.MsgApproveAction) (*types2.MsgApproveActionResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// Note: Basic validation like valid creator address, action ID, and signature format
	// has already been performed in ValidateBasic method of the message

	// Call the keeper method directly to handle context-dependent validation and core logic
	err := k.Keeper.ApproveAction(ctx, msg.ActionId, msg.Creator)
	if err != nil {
		// Wrap with appropriate error type if not already wrapped
		if !errorsmod.IsOf(err, types2.ErrInvalidID, types2.ErrInvalidActionState,
			types2.ErrInvalidSignature) {
			err = errorsmod.Wrap(types2.ErrInvalidActionState, err.Error())
		}
		return nil, err
	}

	return &types2.MsgApproveActionResponse{}, nil
}
