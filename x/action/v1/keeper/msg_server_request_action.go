package keeper

import (
	"context"
	"strconv"

	"github.com/LumeraProtocol/lumera/x/action/v1/common"

	errorsmod "cosmossdk.io/errors"
	"github.com/LumeraProtocol/lumera/x/action/v1/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (k msgServer) RequestAction(goCtx context.Context, msg *types.MsgRequestAction) (*types.MsgRequestActionResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// Get module parameters to access expiration_duration
	params := k.GetParams(goCtx)

	// Get current block time
	currentTime := ctx.BlockTime().Unix()

	// Calculate minimum valid expiration time (current time + expiration_duration)
	minExpTime := currentTime + int64(params.ExpirationDuration.Seconds())

	var expTime int64
	if msg.ExpirationTime != "" {
		// Parse expiration time from the message - Already validated in ValidateBasic
		expTime, _ = strconv.ParseInt(msg.ExpirationTime, 10, 64)

		// Context-dependent validation: Validate that expiration time is in the future and further than block time + expiration_duration
		if expTime <= currentTime {
			return nil, errorsmod.Wrap(types.ErrActionExpired, "expiration time must be in the future")
		}
		if expTime < minExpTime {
			return nil, errorsmod.Wrapf(types.ErrActionExpired,
				"expiration time must be at least %f seconds from current block time",
				params.ExpirationDuration.Seconds(),
			)
		}
	} else {
		// If no expiration time provided, set it to current time + expiration_duration
		expTime = minExpTime
	}

	// Parse and validate action type - Already validated in ValidateBasic
	actionType, _ := types.ParseActionType(msg.ActionType)

	// Validate the metadata
	actionHandler, err := k.actionRegistry.GetHandler(actionType)
	if err != nil {
		return nil, errorsmod.Wrap(types.ErrInvalidActionType, err.Error())
	}

	// Process the metadata with the handler:
	// msg.Metadata is a JSON string
	// processedData is the protobuf binary format as []byte
	processedData, err := actionHandler.Process([]byte(msg.Metadata), common.MsgRequestAction, &params)
	if err != nil {
		return nil, errorsmod.Wrap(types.ErrInvalidMetadata, err.Error())
	}

	price, err := sdk.ParseCoinNormalized(msg.Price)
	if err != nil {
		return nil, errorsmod.Wrapf(types.ErrInvalidPrice, "invalid price format: %s", err)
	}

	// Create a new action with metadata embedded directly
	action := &types.Action{
		Creator:        msg.Creator,
		ActionType:     actionType,
		Metadata:       processedData,
		Price:          &price,
		ExpirationTime: expTime,
		State:          types.ActionStatePending,
	}

	// Save the action (this generates the action ID)
	actionID, err := k.RegisterAction(ctx, action)
	if err != nil {
		return nil, err
	}

	actionNew, ok := k.GetActionByID(ctx, actionID)
	if !ok {
		// This should not happen as we just registered the action
		return &types.MsgRequestActionResponse{}, errorsmod.Wrap(types.ErrActionNotFound,
			"failed to retrieve action by ID after registration")
	}

	return &types.MsgRequestActionResponse{
		ActionId: actionID,
		Status:   actionNew.State.String(),
	}, nil
}
