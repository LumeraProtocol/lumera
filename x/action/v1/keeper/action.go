package keeper

import (
	"context"
	"encoding/binary"
	"fmt"
	"slices"
	"strings"

	"cosmossdk.io/errors"
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

	actiontypes "github.com/LumeraProtocol/lumera/x/action/v1/types"
	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

// Key prefixes for store
const (
	ActionKeyPrefix       = "Action/value/"
	ActionCountKey        = "Action/count/"
	ActionByStatePrefix   = "Action/state/"
	ActionByCreatorPrefix = "Action/creator/"
)

// RegisterAction creates and configures a new action with default parameters
// This is the recommended method for creating new actions
func (k *Keeper) RegisterAction(ctx sdk.Context, action *actiontypes.Action) (string, error) {
	// Validate that the action is for a new registration
	if action.ActionID != "" {
		return "", errors.Wrapf(
			sdkerrors.ErrInvalidRequest,
			"cannot register action with existing ID %s",
			action.ActionID,
		)
	}

	// Parse price from stored string and validate against params
	parsedPrice, err := sdk.ParseCoinNormalized(action.Price)
	if err != nil {
		return "", errors.Wrapf(sdkerrors.ErrInvalidRequest, "invalid price %s: %s", action.Price, err)
	}
	if err := k.validatePrice(ctx, &parsedPrice); err != nil {
		return "", err
	}

	creator, err := k.addressCodec.StringToBytes(action.Creator)
	if err != nil {
		return "", errors.Wrapf(actiontypes.ErrInvalidSignature,
			"invalid account address: %s", err)
	}
	coins := k.bankKeeper.SpendableCoins(ctx, creator)
	if coins == nil || coins.IsZero() {
		return "", errors.Wrapf(
			sdkerrors.ErrInvalidRequest,
			"creator %s does not have a valid account or any spendable coins",
			action.Creator,
		)
	}

	if !coins.IsAllGTE(sdk.Coins{parsedPrice}) {
		return "", errors.Wrapf(
			sdkerrors.ErrInsufficientFunds,
			"creator %s needs at least %s but only has %s",
			action.Creator,
			parsedPrice.String(),
			coins.String(),
		)
	}

	// Generate a new action ID
	count, err := k.getLastActionID(ctx)
	if err != nil {
		return "", err
	}

	// Increment counter and save it
	newID := count + 1
	bz := make([]byte, 8)
	binary.BigEndian.PutUint64(bz, newID)

	store := k.storeService.OpenKVStore(ctx)
	err = store.Set([]byte(ActionCountKey), bz)
	if err != nil {
		return "", err
	}

	// Set action ID as string
	action.ActionID = fmt.Sprintf("%d", newID)

	// Set current block height
	action.BlockHeight = ctx.BlockHeight()

	// Ensure action state is set to PENDING for new actions
	if action.State == actiontypes.ActionStateUnspecified {
		action.State = actiontypes.ActionStatePending
	}
	if action.State != actiontypes.ActionStatePending {
		return "", errors.Wrapf(
			actiontypes.ErrInvalidActionState,
			"new action cannot have state other than pending, but got %s",
			action.State.String(),
		)
	}

	// Get the appropriate handler for this action type to validate and process metadata
	handler, err := k.actionRegistry.GetHandler(action.ActionType)
	if err != nil {
		return "", errors.Wrap(actiontypes.ErrInvalidActionType, err.Error())
	}

	// Call the handler's RegisterAction method with the metadata
	err = handler.RegisterAction(ctx, action)
	if err != nil {
		return "", errors.Wrap(actiontypes.ErrInvalidMetadata, err.Error())
	}

	// Store the action using the low-level SetAction method
	err = k.SetAction(ctx, action)
	if err != nil {
		return "", err
	}

	// Transfer Fee from Creator account to Action Module Account
	err = k.bankKeeper.SendCoinsFromAccountToModule(
		ctx,
		creator,                   // sender - creator
		actiontypes.ModuleName,    // Recipient
		sdk.NewCoins(parsedPrice), // Amount
	)
	if err != nil {
		return "", errors.Wrap(actiontypes.ErrInternalError, err.Error())
	}

	// Emit event for pending state
	if action.State == actiontypes.ActionStatePending {
		ctx.EventManager().EmitEvent(
			sdk.NewEvent(
				actiontypes.EventTypeActionRegistered,
				sdk.NewAttribute(actiontypes.AttributeKeyActionID, action.ActionID),
				sdk.NewAttribute(actiontypes.AttributeKeyCreator, action.Creator),
				sdk.NewAttribute(actiontypes.AttributeKeyActionType, action.ActionType.String()),
				sdk.NewAttribute(actiontypes.AttributeKeyFee, parsedPrice.String()),
			),
		)
	}

	return action.ActionID, nil
}

// FinalizeAction updates an action state after processing by supernodes
// This method implements the ActionFinalizer interface by:
// 1. Validating the action exists and can be finalized
// 2. Verifying supernode authorization
// 3. Processing and validating the provided metadata
// 4. Handling action-specific finalization logic for Cascade and Sense
// 5. Emitting events and potentially distributing fees
func (k *Keeper) FinalizeAction(ctx sdk.Context, actionID string, superNodeAccount /*creator!*/ string, newMetadata []byte) error {
	// Ensure action exists
	existingAction, found := k.GetActionByID(ctx, actionID)
	if !found {
		return errors.Wrapf(sdkerrors.ErrNotFound, "action %s not found", actionID)
	}

	// Check if action is in a valid state to be finalized
	if existingAction.State != actiontypes.ActionStatePending && existingAction.State != actiontypes.ActionStateProcessing {
		return errors.Wrapf(
			actiontypes.ErrInvalidActionState,
			"action %s cannot be finalized: current state %s is not one of pending or processing",
			actionID,
			existingAction.State.String(),
		)
	}

	// Verify reporting superNode -
	// it must be in the top-10 supernodes for the (existing) action's block height
	// and not already in the (existing) action's SuperNodes list
	if err := k.validateSupernode(ctx, existingAction, superNodeAccount); err != nil {
		return err
	}

	// Get the appropriate handler for this action type
	handler, err := k.actionRegistry.GetHandler(existingAction.ActionType)
	if err != nil {
		return errors.Wrap(actiontypes.ErrInvalidActionType, err.Error())
	}

	// Validate and determine state changes
	newState, err := handler.FinalizeAction(ctx, existingAction, superNodeAccount, newMetadata)
	if err != nil {
		return err
	}

	// Apply state changes if a new state is recommended
	if newState != actiontypes.ActionStateUnspecified {
		existingAction.State = newState
		existingAction.Metadata, err = handler.GetUpdatedMetadata(ctx, existingAction.Metadata, newMetadata)
		if err != nil {
			return err
		}

		// Add supernode to the list
		existingAction.SuperNodes = append(existingAction.SuperNodes, superNodeAccount)
	}

	// Save the updated action
	err = k.SetAction(ctx, existingAction)
	if err != nil {
		return errors.Wrap(actiontypes.ErrInternalError, fmt.Sprintf("failed to update action: %v", err))
	}

	if existingAction.State == actiontypes.ActionStateFailed {
		// If the action failed, we should emit an event and return early
		ctx.EventManager().EmitEvent(
			sdk.NewEvent(
				actiontypes.EventTypeActionFailed,
				sdk.NewAttribute(actiontypes.AttributeKeyActionID, existingAction.ActionID),
				sdk.NewAttribute(actiontypes.AttributeKeyCreator, existingAction.Creator),
				sdk.NewAttribute(actiontypes.AttributeKeyActionType, existingAction.ActionType.String()),
				sdk.NewAttribute(actiontypes.AttributeKeyError, "finalization failed"),
				sdk.NewAttribute(actiontypes.AttributeKeySuperNodes, strings.Join(existingAction.SuperNodes, ",")),
			),
		)
		return errors.Wrapf(actiontypes.ErrFinalizationError, "action %s failed", actionID)
	}

	// If the action is now in DONE state, emit an event and distribute fees
	if existingAction.State == actiontypes.ActionStateDone {
		ctx.EventManager().EmitEvent(
			sdk.NewEvent(
				actiontypes.EventTypeActionFinalized,
				sdk.NewAttribute(actiontypes.AttributeKeyActionID, existingAction.ActionID),
				sdk.NewAttribute(actiontypes.AttributeKeyCreator, existingAction.Creator),
				sdk.NewAttribute(actiontypes.AttributeKeyActionType, existingAction.ActionType.String()),
				sdk.NewAttribute(actiontypes.AttributeKeySuperNodes, strings.Join(existingAction.SuperNodes, ",")),
			),
		)

		// Distribute fees to supernodes
		return k.DistributeFees(ctx, actionID)
	}

	return nil
}

// ApproveAction updates an action to APPROVED state after creator approves it
// This method implements the ActionApprover interface by:
// 1. Validating the action exists and can be approved (must be in DONE state)
// 2. Verifying the creator matches the action's original creator
// 3. Updating the action state to APPROVED and storing the signature
// 4. Emitting events
func (k *Keeper) ApproveAction(ctx sdk.Context, actionID string, creator string) error {
	// Ensure action exists
	existingAction, found := k.GetActionByID(ctx, actionID)
	if !found {
		return errors.Wrapf(sdkerrors.ErrNotFound, "action %s not found", actionID)
	}

	// Check if action is in a valid state to be approved
	if existingAction.State != actiontypes.ActionStateDone {
		return errors.Wrapf(
			actiontypes.ErrInvalidActionState,
			"action %s cannot be approved: current state %s",
			actionID,
			existingAction.State.String(),
		)
	}

	// Verify creator
	if existingAction.Creator != creator {
		return errors.Wrapf(
			actiontypes.ErrUnauthorizedSN,
			"only the creator %s can approve action %s",
			existingAction.Creator,
			actionID,
		)
	}

	// Get the appropriate handler for this action type
	handler, err := k.actionRegistry.GetHandler(existingAction.ActionType)
	if err != nil {
		return errors.Wrap(actiontypes.ErrInvalidActionType, err.Error())
	}

	// Call the handler's ValidateApproval method for action-specific approval logic
	err = handler.ValidateApproval(ctx, existingAction)
	if err != nil {
		return err
	}

	// Update action state and store signature
	existingAction.State = actiontypes.ActionStateApproved

	// Save updated action
	err = k.SetAction(ctx, existingAction)
	if err != nil {
		return err
	}

	// Emit event
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			actiontypes.EventTypeActionApproved,
			sdk.NewAttribute(actiontypes.AttributeKeyActionID, existingAction.ActionID),
			sdk.NewAttribute(actiontypes.AttributeKeyCreator, existingAction.Creator),
			sdk.NewAttribute(actiontypes.AttributeKeyActionType, existingAction.ActionType.String()),
		),
	)

	return nil
}

// SetAction handles the low-level storing/updating of an action in the KVStore
// This is an internal method that should be used by other Keeper methods
func (k *Keeper) SetAction(ctx sdk.Context, action *actiontypes.Action) error {
	// First check if the action already exists to handle state changes correctly
	existingAction, found := k.GetActionByID(ctx, action.ActionID)

	store := k.storeService.OpenKVStore(ctx)

	//fmt.Printf("Action: %+v\n", action)

	// Marshal action to store it
	bz, err := k.cdc.Marshal(action)
	if err != nil {
		return err
	}

	// Store action by primary key
	key := []byte(ActionKeyPrefix + action.ActionID)
	err = store.Set(key, bz)
	if err != nil {
		return err
	}

	// Handle state indexing
	// If the action already existed and its state has changed, we need to remove it from the old state index
	if found && existingAction.State != action.State {
		oldStateKey := []byte(ActionByStatePrefix + existingAction.State.String() + "/" + action.ActionID)
		err = store.Delete(oldStateKey)
		if err != nil {
			return err
		}
		k.Logger().Debug("Removed action from previous state index",
			"action_id", action.ActionID,
			"old_state", existingAction.State.String(),
			"new_state", action.State.String())
	}

	// Add to current state index
	stateKey := []byte(ActionByStatePrefix + action.State.String() + "/" + action.ActionID)
	err = store.Set(stateKey, []byte{1}) // Just a marker
	if err != nil {
		return err
	}

	// Index by creator
	creatorKey := []byte(ActionByCreatorPrefix + action.Creator + "/" + action.ActionID)
	err = store.Set(creatorKey, []byte{1}) // Just a marker
	if err != nil {
		return err
	}
	return nil
}

// GetActionByID retrieves an action from the store by actionId
// Note: This is different from the GRPC query handler
func (k *Keeper) GetActionByID(ctx sdk.Context, actionID string) (*actiontypes.Action, bool) {
	store := k.storeService.OpenKVStore(ctx)

	key := []byte(ActionKeyPrefix + actionID)

	bz, err := store.Get(key)
	if err != nil {
		k.Logger().Error("failed to get action", "error", err)
		return nil, false
	}

	if bz == nil {
		return nil, false
	}

	var actionData actiontypes.Action
	err = k.cdc.Unmarshal(bz, &actionData)
	if err != nil {
		k.Logger().Error("failed to unmarshal action", "error", err)
		return nil, false
	}

	return &actionData, true
}

// IterateActions iterates over all actions and calls the provided handler function
func (k *Keeper) IterateActions(ctx sdk.Context, handler func(*actiontypes.Action) bool) error {
	store := k.storeService.OpenKVStore(ctx)

	// Use prefix iterator to get all actions with the ActionKeyPrefix
	iter, err := store.Iterator([]byte(ActionKeyPrefix), nil)
	if err != nil {
		return errors.Wrap(err, "failed to create iterator for actions")
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		// Extract the action bytes
		bz := iter.Value()

		// Unmarshal the action
		var action actiontypes.Action
		err = k.cdc.Unmarshal(bz, &action)
		if err != nil {
			k.Logger().Error("failed to unmarshal action", "error", err)
			continue
		}

		// Call the handler function and check if we should stop iterating
		if handler(&action) {
			break
		}
	}
	return nil
}

// IterateActionsByState iterates over actions with a specific state
func (k *Keeper) IterateActionsByState(ctx sdk.Context, state actiontypes.ActionState, handler func(*actiontypes.Action) bool) error {
	store := k.storeService.OpenKVStore(ctx)

	// Create the state-specific prefix for iteration
	// The key format is ActionByStatePrefix + state + "/" + actionID
	prefixStr := ActionByStatePrefix + state.String() + "/"
	prefixLen := len(prefixStr)
	statePrefix := []byte(prefixStr)

	// Use prefix iterator to get all actions with this state
	iter, err := store.Iterator(statePrefix, nil)
	if err != nil {
		return errors.Wrap(err, "failed to create iterator for actions by state")
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		key := iter.Key()
		keyStr := string(key)

		// Validate the key has the correct prefix to prevent panics
		if len(keyStr) <= prefixLen || !strings.HasPrefix(keyStr, prefixStr) {
			continue
		}

		// Extract the action ID from the key
		actionID := keyStr[prefixLen:]

		// Get the full action using the actionID
		action, found := k.GetActionByID(ctx, actionID)
		if !found {
			k.Logger().Error("action referenced in state index not found", "action_id", actionID, "state", state.String())
			continue
		}

		// Call the handler function and check if we should stop iterating
		if handler(action) {
			break
		}
	}
	return nil
}

// validateSupernode checks if a supernode is authorized to finalize an action
// This method validates that the supernode:
// 1. Is not already in the action's SuperNodes list
// 2. Is in the top-10 supernodes for the action's block height
func (k *Keeper) validateSupernode(ctx sdk.Context, action *actiontypes.Action, superNodeAccount string) error {

	// If SuperNode already in the list, return an error
	if len(action.SuperNodes) > 0 {
		if slices.Contains(action.SuperNodes, superNodeAccount) {
			return errors.Wrapf(
				actiontypes.ErrUnauthorizedSN,
				"supernode %s is already in the SuperNodes list for action %s",
				superNodeAccount,
				action.ActionID,
			)
		}
	}

	// Query top-10 ACTIVE SuperNodes for action's block height
	topSuperNodesReq := &sntypes.QueryGetTopSuperNodesForBlockRequest{
		BlockHeight: int32(action.BlockHeight),
		Limit:       10,
		State:       sntypes.SuperNodeStateActive.String(),
	}
	topSuperNodesResp, err := k.supernodeQueryServer.GetTopSuperNodesForBlock(ctx, topSuperNodesReq)
	if err != nil {
		return errors.Wrapf(sdkerrors.ErrInvalidRequest, "failed to query top supernodes: %s", err)
	}

	// Check if superNode is in the top-10 ACTIVE list
	isInTop10 := false

	k.Logger().Info("Checking if supernode is in top-10 ACTIVE list",
		"supernode", superNodeAccount,
		"block_height", action.BlockHeight,
		"top_supernodes_count", len(topSuperNodesResp.Supernodes))

	for _, sn := range topSuperNodesResp.Supernodes {
		k.Logger().Debug("Comparing supernodes",
			"validator_address", sn.ValidatorAddress,
			"current_supernode", superNodeAccount)

		if sn.SupernodeAccount == superNodeAccount {
			isInTop10 = true
			break
		}
	}

	if !isInTop10 {
		return errors.Wrapf(
			actiontypes.ErrUnauthorizedSN,
			"supernode %s is not in the top-10 ACTIVE supernodes for block height %d",
			superNodeAccount,
			action.BlockHeight,
		)
	}

	return nil
}

// DistributeFees splits fees among SuperNodes and optionally a foundation address
func (k *Keeper) DistributeFees(ctx sdk.Context, actionID string) error {
	actionData, found := k.GetActionByID(ctx, actionID)
	if !found {
		return errors.Wrapf(sdkerrors.ErrNotFound, "action %s not found", actionID)
	}

	// Check if the action is in a valid state for fee distribution
	if actionData.State != actiontypes.ActionStateDone {
		return errors.Wrapf(
			actiontypes.ErrInvalidActionState,
			"cannot distribute fees for action %s: invalid state %s",
			actionID,
			actionData.State.String(),
		)
	}

	// Parse the fee amount from stored string
	fee, err := sdk.ParseCoinNormalized(actionData.Price)
	if err != nil || fee.IsZero() || len(actionData.SuperNodes) == 0 {
		return nil
	}

	// Count unique supernodes
	numSupernodes := 0
	uniqueSupernodes := make(map[string]bool)
	for _, sn := range actionData.SuperNodes {
		if !uniqueSupernodes[sn] && !strings.Contains(sn, "bad") {
			uniqueSupernodes[sn] = true
			numSupernodes++
		}
	}

	if numSupernodes == 0 {
		return nil // No supernodes to pay
	}

	params := k.GetParams(ctx)
	if params.FoundationFeeShare != "" {
		foundationFeeShareDec, err := math.LegacyNewDecFromStr(params.FoundationFeeShare)
		if err != nil {
			return errors.Wrapf(sdkerrors.ErrInvalidRequest, "invalid foundation fee share: %s", err)
		}
		if !foundationFeeShareDec.IsZero() {
			// Calculate foundation fee - convert fee amount to Dec and calculate share
			k.Logger().Info("Foundation fee calculation starting values",
				"fee_amount", fee.Amount.String(),
				"fee_denom", fee.Denom,
				"foundation_share_percentage", foundationFeeShareDec.String())

			// Convert fee amount to Dec
			feeDec := math.LegacyNewDecFromInt(fee.Amount)
			k.Logger().Info("Fee converted to Dec", "fee_dec", feeDec.String())

			// Calculate foundation's portion
			foundationShare := feeDec.Mul(foundationFeeShareDec)
			k.Logger().Info("Foundation share calculated", "foundation_share", foundationShare.String())

			foundationCoin := sdk.NewCoin(fee.Denom, foundationShare.TruncateInt())
			k.Logger().Info("Foundation coin created", "foundation_coin", foundationCoin.String())
			err = k.distributionKeeper.FundCommunityPool(
				ctx,
				sdk.NewCoins(foundationCoin),
				actiontypes.ModuleAccountAddress,
			)
			if err != nil {
				return errors.Wrapf(sdkerrors.ErrInsufficientFunds, "failed to send foundation fee: %s", err)
			}
			fee.Amount = fee.Amount.Sub(foundationShare.TruncateInt())
		}
	}

	// Distribute the fee to each unique supernode
	for sn := range uniqueSupernodes {
		// Calculate fee for this supernode - divide the amount evenly
		feeAmountInt := fee.Amount.Int64() / int64(numSupernodes)

		// Ensure minimum of 1 if fee is positive but too small to divide evenly
		if feeAmountInt == 0 && fee.Amount.Int64() > 0 {
			feeAmountInt = 1
		}

		// Format the fee as a string and parse it back to create a Coin
		feeStr := fmt.Sprintf("%d%s", feeAmountInt, fee.Denom)
		feePerSN, err := sdk.ParseCoinNormalized(feeStr)
		if err != nil {
			k.Logger().Error("Failed to parse fee coin",
				"amount", feeAmountInt,
				"denom", fee.Denom,
				"error", err.Error(),
			)
			continue
		}

		// Get the supernode's account address
		snAddr, err := k.addressCodec.StringToBytes(sn)
		if err != nil {
			return errors.Wrapf(actiontypes.ErrInvalidSignature,
				"invalid account address: %s", err)
		}

		// Actual payment
		err = k.bankKeeper.SendCoinsFromModuleToAccount(
			ctx,
			actiontypes.ModuleName, // Module account name
			snAddr,                 // Recipient
			sdk.NewCoins(feePerSN), // Amount
		)
		if err != nil {
			k.Logger().Error("Failed to distribute fee to supernode",
				"supernode", sn,
				"fee", feePerSN.String(),
				"error", err.Error(),
			)
			continue
		}

		k.Logger().Info("Distributed fee to supernode",
			"supernode", sn,
			"fee", feePerSN.String(),
		)
	}

	return nil
}

// CheckExpiration checks for expired actions in PENDING and PROCESSING states
func (k *Keeper) CheckExpiration(ctx sdk.Context) {
	currentTime := ctx.BlockTime().Unix()
	expiredCount := 0

	pendingErr := k.processExpiredActionsInState(ctx, actiontypes.ActionStatePending, currentTime, &expiredCount)
	if pendingErr != nil {
		k.Logger().Error("Error checking pending actions for expiration", "error", pendingErr.Error())
	}

	processingErr := k.processExpiredActionsInState(ctx, actiontypes.ActionStateProcessing, currentTime, &expiredCount)
	if processingErr != nil {
		k.Logger().Error("Error checking processing actions for expiration", "error", processingErr.Error())
	}

	if expiredCount > 0 {
		k.Logger().Info("Expired actions checked",
			"expired_count", expiredCount,
			"block_height", ctx.BlockHeight(),
			"block_time", ctx.BlockTime(),
		)
	}
}

// processExpiredActionsInState iterates through actions in a specific state and marks expired ones
func (k *Keeper) processExpiredActionsInState(ctx sdk.Context, state actiontypes.ActionState, now int64, expiredCount *int) error {
	return k.IterateActionsByState(ctx, state, func(action *actiontypes.Action) bool {
		// Check if action is expired
		if action.ExpirationTime != 0 && action.ExpirationTime <= now {
			// refund action fee to creator before updating state
			if fee, err := sdk.ParseCoinNormalized(action.Price); err == nil && !fee.IsZero() {
				creatorAddr, err := k.addressCodec.StringToBytes(action.Creator)
				if err != nil {
					k.Logger().Error("Failed to decode action creator address for refund",
						"action_id", action.ActionID,
						"creator", action.Creator,
						"error", err.Error(),
					)
					return false // continue iteration, retry next block
				}

				// Refund the action fee to the creator
				if err := k.bankKeeper.SendCoinsFromModuleToAccount(
					ctx,
					actiontypes.ModuleName,
					creatorAddr,
					sdk.NewCoins(fee),
				); err != nil {
					k.Logger().Error("Failed to refund action fee",
						"action_id", action.ActionID,
						"creator", action.Creator,
						"fee", fee.String(),
						"error", err.Error(),
					)
					return false // continue iteration, retry next block
				}
			}

			// Update action state to EXPIRED
			action.State = actiontypes.ActionStateExpired

			// Save updated action
			err := k.SetAction(ctx, action)
			if err != nil {
				k.Logger().Error("Failed to update expired action",
					"action_id", action.ActionID,
					"error", err.Error(),
				)
				return false // Continue iteration
			}

			// Increment counter
			*expiredCount++

			// Emit event
			ctx.EventManager().EmitEvent(
				sdk.NewEvent(
					actiontypes.EventTypeActionExpired,
					sdk.NewAttribute(actiontypes.AttributeKeyActionID, action.ActionID),
					sdk.NewAttribute(actiontypes.AttributeKeyCreator, action.Creator),
					sdk.NewAttribute(actiontypes.AttributeKeyActionType, action.ActionType.String()),
				),
			)
		}

		return false // Continue iteration
	})
}

// getLastActionID retrieves the last used action ID counter
func (k *Keeper) getLastActionID(ctx sdk.Context) (uint64, error) {
	store := k.storeService.OpenKVStore(ctx)

	bz, err := store.Get([]byte(ActionCountKey))
	if err != nil {
		return 0, err
	}

	if bz == nil {
		return 0, nil
	}

	return binary.BigEndian.Uint64(bz), nil
}

func (k *Keeper) validatePrice(ctx context.Context, price *sdk.Coin) error {
	params := k.GetParams(ctx)

	minFeeAmount := params.BaseActionFee.Amount

	if price == nil {
		return errors.Wrapf(
			sdkerrors.ErrInvalidRequest,
			"price is not specified: must be at least %s (base fee)",
			minFeeAmount.String(),
		)
	}

	// Validate denom
	if price.Denom != params.BaseActionFee.Denom {
		return errors.Wrapf(
			sdkerrors.ErrInvalidRequest,
			"invalid price denom %s: must be %s",
			price.Denom,
			params.BaseActionFee.Denom,
		)
	}

	// Validate amount is at least the base fee
	if price.Amount.LT(minFeeAmount) {
		return errors.Wrapf(
			sdkerrors.ErrInvalidRequest,
			"invalid price amount %s: must be at least %s (base fee)",
			price.Amount.String(),
			minFeeAmount.String(),
		)
	}

	return nil
}
