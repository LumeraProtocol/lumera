package keeper

import (
	"context"
	"github.com/LumeraProtocol/lumera/x/action/v1/common"
	types2 "github.com/LumeraProtocol/lumera/x/action/v1/types"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// FinalizeAction is the message handler for MsgFinalizeAction
// This handles the RPC call and delegates to the keeper method
// Metadata is now embedded directly in the Action object rather than stored separately
func (k msgServer) FinalizeAction(goCtx context.Context, msg *types2.MsgFinalizeAction) (*types2.MsgFinalizeActionResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// Parse and validate action type - Already validated in ValidateBasic
	actionType, _ := types2.ParseActionType(msg.ActionType)

	// Get the appropriate handler for this action type
	actionHandler, err := k.actionRegistry.GetHandler(actionType)
	if err != nil {
		return nil, errorsmod.Wrap(types2.ErrInvalidActionType, err.Error())
	}

	// Process the metadata
	processedData, err := actionHandler.Process([]byte(msg.Metadata), common.MsgFinalizeAction, nil)
	if err != nil {
		return nil, errorsmod.Wrap(types2.ErrInvalidMetadata, err.Error())
	}

	// Call the keeper method directly to handle context-dependent validation and core logic
	// The metadata will be embedded directly in the Action object via the Metadata field
	err = k.Keeper.FinalizeAction(ctx, msg.ActionId, msg.Creator, processedData)
	if err != nil {
		// Wrap with appropriate error type if not already wrapped
		if !errorsmod.IsOf(err, types2.ErrInvalidMetadata, types2.ErrInvalidActionState,
			types2.ErrUnauthorizedSN, types2.ErrInvalidID, types2.ErrInvalidActionType) {
			err = errorsmod.Wrap(types2.ErrInvalidActionState, err.Error())
		}
		return nil, err
	}

	return &types2.MsgFinalizeActionResponse{}, nil
}
