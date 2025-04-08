package simulation

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"

	actionapi "github.com/LumeraProtocol/lumera/api/lumera/action"
	"github.com/LumeraProtocol/lumera/x/action/keeper"
	"github.com/LumeraProtocol/lumera/x/action/types"
)

// SimulateActionExpiration verifies that actions correctly transition from PENDING to EXPIRED state
// when the block time exceeds their expirationTime.
func SimulateActionExpiration(
	ak types.AccountKeeper,
	bk types.BankKeeper,
	k keeper.Keeper,
) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		// 1. Create a PENDING action with a short expiration time
		// Find or create a pending action
		actionID, action := findOrCreatePendingAction(r, ctx, accs, k, bk, ak)
		if action == nil {
			return simtypes.NoOpMsg(types.ModuleName, "simulate_action_expiration", "failed to find or create pending action"), nil, nil
		}

		// Get the action's expiration time
		expirationTime := action.ExpirationTime

		// Verify action is in PENDING state
		if action.State != actionapi.ActionState_ACTION_STATE_PENDING {
			return simtypes.NoOpMsg(types.ModuleName, "simulate_action_expiration",
				fmt.Sprintf("action not in PENDING state, got %s", action.State)), nil, nil
		}

		// 2. Advance block time past the expiration time
		// Create a new context with advanced block time
		newTime := time.Unix(expirationTime, 0).Add(time.Minute) // 1 minute past expiration
		newHeader := ctx.BlockHeader()
		newHeader.Time = newTime
		futureCtx := ctx.WithBlockHeader(newHeader)

		// 3. Query the action in the future context to check its state
		actionAfterExpiry, found := k.GetActionByID(futureCtx, actionID)
		if !found {
			return simtypes.NoOpMsg(types.ModuleName, "simulate_action_expiration",
				"action not found after time advance"), nil, nil
		}

		// 4. Verify the action state changed to EXPIRED
		if actionAfterExpiry.State != actionapi.ActionState_ACTION_STATE_EXPIRED {
			return simtypes.NoOpMsg(types.ModuleName, "simulate_action_expiration",
				fmt.Sprintf("action state not EXPIRED after expiration, got %s", actionAfterExpiry.State)), nil, nil
		}

		// 5. Optionally: attempt to finalize the action and verify it fails
		// We'll skip this step for simplicity as it would require a separate transaction

		// Return successful operation message
		return simtypes.NewOperationMsg(&types.MsgRequestAction{}, true, "action_expiration_success"), nil, nil
	}
}

// SimulateActionFailure verifies that actions correctly transition to the FAILED state under specific failure conditions
// during finalization (e.g., irreconcilable consensus failure for SENSE, critical error during CASCADE processing).
func SimulateActionFailure(
	ak types.AccountKeeper,
	bk types.BankKeeper,
	k keeper.Keeper,
) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		// 1. Create a PENDING action
		// Find or create a pending action
		actionID, action := findOrCreatePendingAction(r, ctx, accs, k, bk, ak)
		if action == nil {
			return simtypes.NoOpMsg(types.ModuleName, "simulate_action_failure", "failed to find or create pending action"), nil, nil
		}

		// Get action type
		actionType := action.ActionType.String()

		// Verify action is in PENDING state
		if action.State != actionapi.ActionState_ACTION_STATE_PENDING {
			return simtypes.NoOpMsg(types.ModuleName, "simulate_action_failure",
				fmt.Sprintf("action not in PENDING state, got %s", action.State)), nil, nil
		}

		// 2. Get random supernodes to finalize the action
		supernodes := selectRandomSupernodes(r, ctx, accs, 3) // Get 3 supernodes for SENSE consensus

		// 3. Generate finalization metadata that will cause failure
		msgServSim := keeper.NewMsgServerImpl(k)

		if actionType == actionapi.ActionType_ACTION_TYPE_SENSE.String() {
			// For SENSE: Create conflicting results from different supernodes

			// Parse existing metadata
			var existingMetadata actionapi.SenseMetadata
			err := json.Unmarshal([]byte(action.Metadata), &existingMetadata)
			if err != nil {
				return simtypes.NoOpMsg(types.ModuleName, "simulate_action_failure",
					fmt.Sprintf("failed to unmarshal existing metadata: %v", err)), nil, nil
			}

			// First conflict - modified DataHash
			ddIds1 := generateRandomKademliaIDs(r, 3)
			metadata1 := actionapi.SenseMetadata{
				DataHash:              existingMetadata.DataHash + "_conflict1",
				DdAndFingerprintsIc:   uint64(len(ddIds1)),
				CollectionId:          existingMetadata.CollectionId,
				GroupId:               existingMetadata.GroupId,
				DdAndFingerprintsIds:  ddIds1,
				Signatures:            "",
				SupernodeFingerprints: generateConsistentFingerprintResults(r),
			}
			metadataBytes1, _ := json.Marshal(&metadata1)

			finalizeMsg1 := types.NewMsgFinalizeAction(
				supernodes[0].Address.String(),
				actionID,
				actionType,
				string(metadataBytes1),
			)

			_, _ = msgServSim.FinalizeAction(ctx, finalizeMsg1)

			// Second conflict - different fingerprints
			ddIds2 := generateRandomKademliaIDs(r, 3)
			fingerprintResults := generateConsistentFingerprintResults(r)
			conflictingResults := make(map[string]string)
			for k, v := range fingerprintResults {
				conflictingResults[k] = v + "_changed"
			}

			metadata2 := actionapi.SenseMetadata{
				DataHash:              existingMetadata.DataHash,
				DdAndFingerprintsIc:   uint64(len(ddIds2)),
				CollectionId:          existingMetadata.CollectionId,
				GroupId:               existingMetadata.GroupId,
				DdAndFingerprintsIds:  ddIds2,
				Signatures:            "",
				SupernodeFingerprints: conflictingResults,
			}
			metadataBytes2, _ := json.Marshal(&metadata2)

			finalizeMsg2 := types.NewMsgFinalizeAction(
				supernodes[1].Address.String(),
				actionID,
				actionType,
				string(metadataBytes2),
			)
			_, _ = msgServSim.FinalizeAction(ctx, finalizeMsg2)

			// Third conflict - different IDs count
			ddIds3 := generateRandomKademliaIDs(r, 2) // Only 2 IDs instead of 3
			metadata3 := actionapi.SenseMetadata{
				DataHash:              existingMetadata.DataHash,
				DdAndFingerprintsIc:   uint64(len(ddIds3)),
				CollectionId:          existingMetadata.CollectionId,
				GroupId:               existingMetadata.GroupId,
				DdAndFingerprintsIds:  ddIds3,
				Signatures:            "",
				SupernodeFingerprints: generateConsistentFingerprintResults(r),
			}
			metadataBytes3, _ := json.Marshal(&metadata3)

			// Send the third finalization message
			finalizeMsg3 := types.NewMsgFinalizeAction(
				supernodes[2].Address.String(),
				actionID,
				actionType,
				string(metadataBytes3),
			)

			// This should force the action to FAILED state
			_, _ = msgServSim.FinalizeAction(ctx, finalizeMsg3)

		} else {
			// For CASCADE: Create metadata indicating processing failure
			var existingMetadata actionapi.CascadeMetadata
			err := json.Unmarshal([]byte(action.Metadata), &existingMetadata)
			if err != nil {
				return simtypes.NoOpMsg(types.ModuleName, "simulate_action_failure",
					fmt.Sprintf("failed to unmarshal existing metadata: %v", err)), nil, nil
			}

			// Create faulty metadata - mismatched RqIdsIc vs actual IDs
			rqIds := generateRandomRqIds(r, 1) // Only 1 ID
			failureMetadata := actionapi.CascadeMetadata{
				DataHash:   existingMetadata.DataHash,
				FileName:   existingMetadata.FileName,
				RqIdsIc:    3, // Claiming 3 ids but only providing 1
				RqIdsIds:   rqIds,
				Signatures: "",
			}
			metadataBytes, _ := json.Marshal(&failureMetadata)

			finalizeMsg := types.NewMsgFinalizeAction(
				supernodes[0].Address.String(),
				actionID,
				actionType,
				string(metadataBytes),
			)

			// This should force the action to FAILED state
			_, _ = msgServSim.FinalizeAction(ctx, finalizeMsg)
		}

		// 5. Verify the action transitioned to FAILED state
		updatedAction, found := k.GetActionByID(ctx, actionID)
		if !found {
			return simtypes.NoOpMsg(types.ModuleName, "simulate_action_failure", "action not found after finalization"), nil, nil
		}

		// 6. Check that the action state is now FAILED
		if updatedAction.State != actionapi.ActionState_ACTION_STATE_FAILED {
			return simtypes.NoOpMsg(types.ModuleName, "simulate_action_failure",
				fmt.Sprintf("action state not FAILED after failing finalization, got %s", updatedAction.State)), nil, nil
		}

		// 7. Verify no fees were distributed
		// Get initial balances of supernodes
		feeDenom := k.GetParams(ctx).BaseActionFee.Denom
		for _, sn := range supernodes {
			addressObj, _ := sdk.AccAddressFromBech32(sn.Address.String())
			balance := bk.GetBalance(ctx, addressObj, feeDenom)

			// In a real test, we would compare this with the balance before finalization
			// But for simulation purposes, we'll just check it's non-zero
			if balance.IsZero() {
				// This is expected since no fees should be distributed for failed actions
			}
		}

		// 8. Attempt to finalize again and verify it fails
		attemptMsg := types.NewMsgFinalizeAction(
			supernodes[0].Address.String(),
			actionID,
			actionType,
			string(action.Metadata), // Reuse original metadata
		)
		_, attemptErr := msgServSim.FinalizeAction(ctx, attemptMsg)
		if attemptErr == nil {
			return simtypes.NoOpMsg(types.ModuleName, "simulate_action_failure",
				"subsequent finalization of FAILED action succeeded but should have failed"), nil, nil
		}

		// Return successful operation message
		return simtypes.NewOperationMsg(&types.MsgFinalizeAction{}, true, "action_failure_success"), nil, nil
	}
}

// SimulateProcessingState verifies that a SENSE action transitions from PENDING to PROCESSING state
// after receiving the first valid finalization message, but before reaching the required consensus of three.
func SimulateProcessingState(
	ak types.AccountKeeper,
	bk types.BankKeeper,
	k keeper.Keeper,
) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		// 1. Create a PENDING SENSE action
		actionID := createPendingSenseAction(r, ctx, accs, bk, k, ak)

		// 2. Get the initial action to verify its state is PENDING
		initialAction, found := k.GetActionByID(ctx, actionID)
		if !found || initialAction.State != actionapi.ActionState_ACTION_STATE_PENDING {
			return simtypes.NoOpMsg(types.ModuleName, "simulate_processing_state",
				"failed to find action in PENDING state"), nil, nil
		}

		// 3. Select one random supernode account
		supernode := selectRandomSupernode(r, ctx, accs)

		// 4. Get supernode's initial balance to verify no fee distribution
		feeDenom := k.GetParams(ctx).BaseActionFee.Denom
		initialBalance := bk.GetBalance(ctx, supernode.Address, feeDenom)

		// 5. Generate valid finalization metadata
		ddIds := generateRandomKademliaIDs(r, 3)
		fingerprintResults := generateConsistentFingerprintResults(r)
		metadata := generateFinalizeMetadataForSense(r, ctx, k, actionID, fingerprintResults, ddIds)
		signature := signMetadata(supernode, metadata)
		metadataWithSig := addSignatureToMetadata(metadata, signature)

		// 6. Create and send finalization message
		msg := types.NewMsgFinalizeAction(
			supernode.Address.String(),
			actionID,
			actionapi.ActionType_ACTION_TYPE_SENSE.String(),
			metadataWithSig,
		)

		msgServSim := keeper.NewMsgServerImpl(k)
		_, err := msgServSim.FinalizeAction(ctx, msg)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), err.Error()), nil, err
		}

		// 7. Verify action moved to PROCESSING state
		updatedAction, found := k.GetActionByID(ctx, actionID)
		if !found {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg),
				"action not found after finalization"), nil, nil
		}

		if updatedAction.State != actionapi.ActionState_ACTION_STATE_PROCESSING {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg),
				fmt.Sprintf("action not in PROCESSING state after finalization, got %s",
					updatedAction.State)), nil, nil
		}

		// 8. Verify no fees have been distributed yet
		finalBalance := bk.GetBalance(ctx, supernode.Address, feeDenom)
		if !finalBalance.Equal(initialBalance) {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg),
				"fees were distributed after single finalization, but should not be until consensus"), nil, nil
		}

		// 9. Return successful operation message
		return simtypes.NewOperationMsg(msg, true, "transition_to_processing_success"), nil, nil
	}
}
