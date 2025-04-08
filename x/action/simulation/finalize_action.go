package simulation

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"

	actionapi "github.com/LumeraProtocol/lumera/api/lumera/action"
	"github.com/LumeraProtocol/lumera/x/action/keeper"
	"github.com/LumeraProtocol/lumera/x/action/types"
	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
)

// SimulateMsgFinalizeAction_Success_Sense simulates a successful finalization of a SENSE action
func SimulateMsgFinalizeAction_Success_Sense(
	ak types.AccountKeeper,
	bk types.BankKeeper,
	k keeper.Keeper,
) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		// 1. Create a PENDING SENSE action
		actionID := createPendingSenseAction(r, ctx, accs, bk, k, ak)

		// 2. Select three random supernode accounts
		supernodes := selectRandomSupernodes(r, ctx, accs, 3)

		// 3. Generate consistent finalization results for all supernodes
		ddIds := generateRandomKademliaIDs(r, 3)
		fingerprintResults := generateConsistentFingerprintResults(r)

		// 4. Create finalization metadata with supernode1's signature
		metadata1 := generateFinalizeMetadataForSense(r, ctx, k, actionID, fingerprintResults, ddIds)
		signature1 := signMetadata(supernodes[0], metadata1)
		metadata1WithSig := addSignatureToMetadata(metadata1, signature1)

		// 5. Create first finalization message
		msg1 := types.NewMsgFinalizeAction(
			supernodes[0].Address.String(),
			actionID,
			actionapi.ActionType_ACTION_TYPE_SENSE.String(),
			metadata1WithSig,
		)

		// 6. Deliver first transaction
		msgServSim := keeper.NewMsgServerImpl(k)
		_, err1 := msgServSim.FinalizeAction(ctx, msg1)
		if err1 != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg1), err1.Error()), nil, err1
		}

		// 7. Verify action moved to PROCESSING state
		updatedAction, found := k.GetActionByID(ctx, actionID)
		if !found {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg1), "action not found after first finalization"), nil, nil
		}

		if updatedAction.State != actionapi.ActionState_ACTION_STATE_PROCESSING {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg1), "action not in PROCESSING state after first finalization"), nil, nil
		}

		// 8. Create second finalization message with supernode2's signature
		metadata2 := generateFinalizeMetadataForSense(r, ctx, k, actionID, fingerprintResults, ddIds)
		signature2 := signMetadata(supernodes[1], metadata2)
		metadata2WithSig := addSignatureToMetadata(metadata2, signature2)

		msg2 := types.NewMsgFinalizeAction(
			supernodes[1].Address.String(),
			actionID,
			actionapi.ActionType_ACTION_TYPE_SENSE.String(),
			metadata2WithSig,
		)

		// 9. Deliver second transaction
		_, err2 := msgServSim.FinalizeAction(ctx, msg2)
		if err2 != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg2), err2.Error()), nil, err2
		}

		// 10. Verify action is still in PROCESSING state
		updatedAction2, found := k.GetActionByID(ctx, actionID)
		if !found {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg2), "action not found after second finalization"), nil, nil
		}

		if updatedAction2.State != actionapi.ActionState_ACTION_STATE_PROCESSING {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg2), "action not in PROCESSING state after second finalization"), nil, nil
		}

		// 11. Create third finalization message with supernode3's signature
		metadata3 := generateFinalizeMetadataForSense(r, ctx, k, actionID, fingerprintResults, ddIds)
		signature3 := signMetadata(supernodes[2], metadata3)
		metadata3WithSig := addSignatureToMetadata(metadata3, signature3)

		msg3 := types.NewMsgFinalizeAction(
			supernodes[2].Address.String(),
			actionID,
			actionapi.ActionType_ACTION_TYPE_SENSE.String(),
			metadata3WithSig,
		)

		// 12. Store initial balances of supernodes to check fee distribution later
		feeDenom := k.GetParams(ctx).BaseActionFee.Denom
		initialBalance1 := bk.GetBalance(ctx, supernodes[0].Address, feeDenom)
		initialBalance2 := bk.GetBalance(ctx, supernodes[1].Address, feeDenom)
		initialBalance3 := bk.GetBalance(ctx, supernodes[2].Address, feeDenom)

		// 13. Deliver third transaction
		_, err3 := msgServSim.FinalizeAction(ctx, msg3)
		if err3 != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg3), err3.Error()), nil, err3
		}

		// 14. Verify action is now in DONE state
		finalAction, found := k.GetActionByID(ctx, actionID)
		if !found {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg3), "action not found after third finalization"), nil, nil
		}

		if finalAction.State != actionapi.ActionState_ACTION_STATE_DONE {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg3), "action not in DONE state after third finalization"), nil, nil
		}

		// 15. Check supernode balances to confirm fee distribution
		finalBalance1 := bk.GetBalance(ctx, supernodes[0].Address, feeDenom)
		finalBalance2 := bk.GetBalance(ctx, supernodes[1].Address, feeDenom)
		finalBalance3 := bk.GetBalance(ctx, supernodes[2].Address, feeDenom)

		// All three supernodes should have received fees
		if !finalBalance1.Amount.GT(initialBalance1.Amount) ||
			!finalBalance2.Amount.GT(initialBalance2.Amount) ||
			!finalBalance3.Amount.GT(initialBalance3.Amount) {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg3), "fee distribution not as expected"), nil, nil
		}

		// 16. Return successful operation message
		return simtypes.NewOperationMsg(msg3, true, "success"), nil, nil
	}
}

// SimulateMsgFinalizeAction_Success_Cascade simulates a successful finalization of a CASCADE action
func SimulateMsgFinalizeAction_Success_Cascade(
	ak types.AccountKeeper,
	bk types.BankKeeper,
	k keeper.Keeper,
) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		// 1. Create a PENDING CASCADE action
		actionID := createPendingCascadeAction(r, ctx, accs, bk, k, ak)

		// 2. Select single random supernode account
		supernode := selectRandomSupernode(r, ctx, accs)

		// 3. Generate random RQ IDs and OTI values for storage results
		rqIds := generateRandomRqIds(r, 5)
		otiValues := generateRandomOtiValues(r, 5)

		// 4. Store initial balance of supernode to check fee distribution later
		feeDenom := k.GetParams(ctx).BaseActionFee.Denom
		initialBalance := bk.GetBalance(ctx, supernode.Address, feeDenom)

		// 5. Get the action to create finalization metadata
		action, found := k.GetActionByID(ctx, actionID)
		if !found {
			return simtypes.NoOpMsg(types.ModuleName, "FinalizeAction", "action not found"), nil, nil
		}

		// 6. Create finalization metadata with signature
		metadata := generateFinalizeMetadataForCascade(action, rqIds, otiValues)
		signature := signMetadata(supernode, metadata)
		metadataWithSig := addSignatureToMetadata(metadata, signature)

		// 7. Create finalization message
		msg := types.NewMsgFinalizeAction(
			supernode.Address.String(),
			actionID,
			actionapi.ActionType_ACTION_TYPE_CASCADE.String(),
			metadataWithSig,
		)

		// 8. Deliver transaction
		msgServSim := keeper.NewMsgServerImpl(k)
		_, err := msgServSim.FinalizeAction(ctx, msg)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), err.Error()), nil, err
		}

		// 9. Verify action moved to DONE state
		finalizedAction, found := k.GetActionByID(ctx, actionID)
		if !found {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), "action not found after finalization"), nil, nil
		}

		if finalizedAction.State != actionapi.ActionState_ACTION_STATE_DONE {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), "action not in DONE state after finalization"), nil, nil
		}

		// 10. Check supernode balance to confirm fee distribution
		finalBalance := bk.GetBalance(ctx, supernode.Address, feeDenom)
		if !finalBalance.Amount.GT(initialBalance.Amount) {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), "fee distribution not as expected"), nil, nil
		}

		// 11. Return successful operation message
		return simtypes.NewOperationMsg(msg, true, "success"), nil, nil
	}
}

// SimulateMsgFinalizeAction_Invalid_ID simulates attempting to finalize an action with a non-existent ID
func SimulateMsgFinalizeAction_Invalid_ID(
	ak types.AccountKeeper,
	bk types.BankKeeper,
	k keeper.Keeper,
) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		// 1. Select random supernode account
		supernode := selectRandomSupernode(r, ctx, accs)

		// 2. Generate non-existent action ID
		invalidActionID := generateNonExistentActionID(r, ctx, k)

		// 3. Get initial supernode balance
		feeDenom := k.GetParams(ctx).BaseActionFee.Denom
		initialBalance := bk.GetBalance(ctx, supernode.Address, feeDenom)

		// 4. Create valid metadata (doesn't matter since ID is invalid)
		metadata := generateValidFinalizeMetadata(r)
		signature := signMetadata(supernode, metadata)
		metadataWithSig := addSignatureToMetadata(metadata, signature)

		// 5. Create finalization message
		// We need to randomly select an action type for the message
		actionType := selectRandomActionType(r)
		msg := types.NewMsgFinalizeAction(
			supernode.Address.String(),
			invalidActionID,
			actionType,
			metadataWithSig,
		)

		// 6. Deliver transaction, expecting error
		msgServSim := keeper.NewMsgServerImpl(k)
		_, err := msgServSim.FinalizeAction(ctx, msg)

		// 7. Check that an error occurred (should be about invalid action ID)
		if err == nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), "expected error when finalizing with invalid action ID"), nil, nil
		}

		// 8. Verify supernode balance remains unchanged
		finalBalance := bk.GetBalance(ctx, supernode.Address, feeDenom)
		if !finalBalance.Equal(initialBalance) {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), "balance changed despite error"), nil, nil
		}

		// 9. Return operation message, marking as failed but expected
		return simtypes.NewOperationMsg(msg, false, "invalid action ID"), nil, nil
	}
}

// SimulateMsgFinalizeAction_InvalidState simulates attempting to finalize an action that is not in PENDING state
func SimulateMsgFinalizeAction_InvalidState(
	ak types.AccountKeeper,
	bk types.BankKeeper,
	k keeper.Keeper,
) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		// 1. Find or create an action in DONE state
		actionID, action := createDoneAction(r, ctx, accs, k, bk, ak)

		// 2. Select random supernode account
		supernode := selectRandomSupernode(r, ctx, accs)

		// 3. Get initial supernode balance
		feeDenom := k.GetParams(ctx).BaseActionFee.Denom
		initialBalance := bk.GetBalance(ctx, supernode.Address, feeDenom)

		// 4. Create valid metadata
		metadata := generateValidFinalizeMetadata(r)
		signature := signMetadata(supernode, metadata)
		metadataWithSig := addSignatureToMetadata(metadata, signature)

		// 5. Create finalization message
		msg := types.NewMsgFinalizeAction(
			supernode.Address.String(),
			actionID,
			action.ActionType.String(),
			metadataWithSig,
		)

		// 6. Deliver transaction, expecting error
		msgServSim := keeper.NewMsgServerImpl(k)
		_, err := msgServSim.FinalizeAction(ctx, msg)

		// 7. Check that an error occurred (should be about invalid action state)
		if err == nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), "expected error when finalizing action in DONE state"), nil, nil
		}

		// Check if the error is about invalid action state
		if !strings.Contains(err.Error(), "invalid state") && !strings.Contains(err.Error(), "not in PENDING state") {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), fmt.Sprintf("unexpected error: %v", err)), nil, nil
		}

		// 8. Verify supernode balance remains unchanged
		finalBalance := bk.GetBalance(ctx, supernode.Address, feeDenom)
		if !finalBalance.Equal(initialBalance) {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), "balance changed despite error"), nil, nil
		}

		// 9. Verify action state remains unchanged - this will be checked against the actual stored action
		unchangedAction, found := k.GetActionByID(ctx, actionID)
		if !found {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), "action not found after finalization attempt"), nil, nil
		}

		// The state should still be PENDING in the store (not DONE which is only in our simulated version)
		if unchangedAction.State != actionapi.ActionState_ACTION_STATE_PENDING {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), "action state changed despite error"), nil, nil
		}

		// 10. Return operation message, marking as failed but expected
		return simtypes.NewOperationMsg(msg, false, "invalid action state"), nil, nil
	}
}

// SimulateMsgFinalizeAction_Unauthorized simulates attempting to finalize an action by a non-supernode account
func SimulateMsgFinalizeAction_Unauthorized(
	ak types.AccountKeeper,
	bk types.BankKeeper,
	k keeper.Keeper,
) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		// 1. Find or create a PENDING action
		actionID, action := findOrCreatePendingAction(r, ctx, accs, k, bk, ak)

		// 2. Select random non-supernode account
		regularAccount := selectRandomNonSupernode(r, ctx, accs)

		// 3. Create valid metadata
		metadata := generateValidFinalizeMetadata(r)
		signature := signMetadata(regularAccount, metadata)
		metadataWithSig := addSignatureToMetadata(metadata, signature)

		// 4. Create finalization message
		msg := types.NewMsgFinalizeAction(
			regularAccount.Address.String(),
			actionID,
			action.ActionType.String(),
			metadataWithSig,
		)

		// 5. Deliver transaction, expecting error
		msgServSim := keeper.NewMsgServerImpl(k)
		_, err := msgServSim.FinalizeAction(ctx, msg)

		// 6. Check that an error occurred (should be about unauthorized account)
		if err == nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), "expected error when finalizing with unauthorized account"), nil, nil
		}

		// Check if the error is about unauthorized account
		if !strings.Contains(err.Error(), "unauthorized") && !strings.Contains(err.Error(), "authority") {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), fmt.Sprintf("unexpected error: %v", err)), nil, nil
		}

		// 7. Verify action state remains unchanged
		unchangedAction, found := k.GetActionByID(ctx, actionID)
		if !found {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), "action not found after finalization attempt"), nil, nil
		}

		if unchangedAction.State != actionapi.ActionState_ACTION_STATE_PENDING {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), "action state changed despite error"), nil, nil
		}

		// 8. Return operation message, marking as failed but expected
		return simtypes.NewOperationMsg(msg, false, "unauthorized account"), nil, nil
	}
}

// SimulateMsgFinalizeAction_SenseConsensus simulates the consensus requirement for SENSE actions
// Testing different scenarios of matching and non-matching results from multiple supernodes
func SimulateMsgFinalizeAction_SenseConsensus(
	ak types.AccountKeeper,
	bk types.BankKeeper,
	k keeper.Keeper,
) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		// Random scenario selection - we'll test one scenario per simulation run
		scenario := r.Intn(4) + 1

		// 1. Create a PENDING SENSE action
		actionID := createPendingSenseAction(r, ctx, accs, bk, k, ak)

		// 2. Select required number of supernode accounts (at least 5 for all scenarios)
		supernodes := selectRandomSupernodes(r, ctx, accs, 5)

		// Store initial balances to verify fee distribution
		feeDenom := k.GetParams(ctx).BaseActionFee.Denom
		initialBalances := make(map[string]sdk.Coin)
		for i := 0; i < len(supernodes); i++ {
			initialBalances[supernodes[i].Address.String()] = bk.GetBalance(ctx, supernodes[i].Address, feeDenom)
		}

		msgServSim := keeper.NewMsgServerImpl(k)

		switch scenario {
		case 1:
			// Scenario 1: 3 supernodes submit matching results (should succeed)
			// Generate consistent data for all three supernodes
			ddIds := generateRandomKademliaIDs(r, 3)
			fingerprintResults := generateConsistentFingerprintResults(r)

			// Process 3 matching submissions
			for i := 0; i < 3; i++ {
				metadata := generateFinalizeMetadataForSense(r, ctx, k, actionID, fingerprintResults, ddIds)
				signature := signMetadata(supernodes[i], metadata)
				metadataWithSig := addSignatureToMetadata(metadata, signature)

				msg := types.NewMsgFinalizeAction(
					supernodes[i].Address.String(),
					actionID,
					actionapi.ActionType_ACTION_TYPE_SENSE.String(),
					metadataWithSig,
				)

				_, err := msgServSim.FinalizeAction(ctx, msg)
				if err != nil {
					return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), err.Error()), nil, err
				}

				// Check state after each submission
				action, found := k.GetActionByID(ctx, actionID)
				if !found {
					return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), "action not found"), nil, nil
				}

				expectedState := actionapi.ActionState_ACTION_STATE_PROCESSING
				if i == 2 {
					// After 3rd matching submission, state should be DONE
					expectedState = actionapi.ActionState_ACTION_STATE_DONE
				}

				if action.State != expectedState {
					return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg),
						fmt.Sprintf("unexpected state after submission %d: got %s, expected %s",
							i+1, action.State, expectedState)), nil, nil
				}
			}

			// Verify fees were distributed to all three supernodes
			for i := 0; i < 3; i++ {
				finalBalance := bk.GetBalance(ctx, supernodes[i].Address, feeDenom)
				if !finalBalance.Amount.GT(initialBalances[supernodes[i].Address.String()].Amount) {
					return simtypes.NoOpMsg(types.ModuleName, "FinalizeAction",
						"fee not distributed to supernode"), nil, nil
				}
			}

			return simtypes.NewOperationMsg(&types.MsgFinalizeAction{}, true, "success - 3 matching results"), nil, nil

		case 2:
			// Scenario 2: 2 supernodes submit matching results, 1 submits different results (should fail/remain in PROCESSING)
			// Generate consistent data for first two supernodes
			ddIds1 := generateRandomKademliaIDs(r, 3)
			fingerprintResults1 := generateConsistentFingerprintResults(r)

			// Generate different data for third supernode
			ddIds2 := generateRandomKademliaIDs(r, 3)
			fingerprintResults2 := generateNonMatchingFingerprintResults(r, fingerprintResults1)

			// First two supernodes submit matching results
			for i := 0; i < 2; i++ {
				metadata := generateFinalizeMetadataForSense(r, ctx, k, actionID, fingerprintResults1, ddIds1)
				signature := signMetadata(supernodes[i], metadata)
				metadataWithSig := addSignatureToMetadata(metadata, signature)

				msg := types.NewMsgFinalizeAction(
					supernodes[i].Address.String(),
					actionID,
					actionapi.ActionType_ACTION_TYPE_SENSE.String(),
					metadataWithSig,
				)

				_, err := msgServSim.FinalizeAction(ctx, msg)
				if err != nil {
					return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), err.Error()), nil, err
				}

				// After 1st and 2nd submission, state should be PROCESSING
				action, _ := k.GetActionByID(ctx, actionID)
				if action.State != actionapi.ActionState_ACTION_STATE_PROCESSING {
					return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg),
						"unexpected state after submission"), nil, nil
				}
			}

			// Third supernode submits different results
			metadata3 := generateFinalizeMetadataForSense(r, ctx, k, actionID, fingerprintResults2, ddIds2)
			signature3 := signMetadata(supernodes[2], metadata3)
			metadataWithSig3 := addSignatureToMetadata(metadata3, signature3)

			msg3 := types.NewMsgFinalizeAction(
				supernodes[2].Address.String(),
				actionID,
				actionapi.ActionType_ACTION_TYPE_SENSE.String(),
				metadataWithSig3,
			)

			_, err := msgServSim.FinalizeAction(ctx, msg3)
			if err != nil {
				return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg3), err.Error()), nil, err
			}

			// After 3rd non-matching submission, state should still be PROCESSING
			action, _ := k.GetActionByID(ctx, actionID)
			if action.State != actionapi.ActionState_ACTION_STATE_PROCESSING {
				return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg3),
					"unexpected state after non-matching submission"), nil, nil
			}

			// Verify no fees were distributed yet
			for i := 0; i < 3; i++ {
				finalBalance := bk.GetBalance(ctx, supernodes[i].Address, feeDenom)
				if !finalBalance.Equal(initialBalances[supernodes[i].Address.String()]) {
					return simtypes.NoOpMsg(types.ModuleName, "FinalizeAction",
						"fee distributed before consensus reached"), nil, nil
				}
			}

			return simtypes.NewOperationMsg(&types.MsgFinalizeAction{}, true, "success - no consensus with conflicting results"), nil, nil

		case 3:
			// Scenario 3: Only 2 supernodes submit matching results (should remain in PROCESSING)
			// Generate consistent data for both supernodes
			ddIds := generateRandomKademliaIDs(r, 3)
			fingerprintResults := generateConsistentFingerprintResults(r)

			// Only two supernodes submit results
			for i := 0; i < 2; i++ {
				metadata := generateFinalizeMetadataForSense(r, ctx, k, actionID, fingerprintResults, ddIds)
				signature := signMetadata(supernodes[i], metadata)
				metadataWithSig := addSignatureToMetadata(metadata, signature)

				msg := types.NewMsgFinalizeAction(
					supernodes[i].Address.String(),
					actionID,
					actionapi.ActionType_ACTION_TYPE_SENSE.String(),
					metadataWithSig,
				)

				_, err := msgServSim.FinalizeAction(ctx, msg)
				if err != nil {
					return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), err.Error()), nil, err
				}
			}

			// After only 2 submissions, state should still be PROCESSING
			action, _ := k.GetActionByID(ctx, actionID)
			if action.State != actionapi.ActionState_ACTION_STATE_PROCESSING {
				return simtypes.NoOpMsg(types.ModuleName, "FinalizeAction",
					fmt.Sprintf("unexpected state after 2 submissions: got %s, expected PROCESSING",
						action.State)), nil, nil
			}

			// Verify no fees were distributed
			for i := 0; i < 2; i++ {
				finalBalance := bk.GetBalance(ctx, supernodes[i].Address, feeDenom)
				if !finalBalance.Equal(initialBalances[supernodes[i].Address.String()]) {
					return simtypes.NoOpMsg(types.ModuleName, "FinalizeAction",
						"fee distributed before consensus reached"), nil, nil
				}
			}

			return simtypes.NewOperationMsg(&types.MsgFinalizeAction{}, true, "success - insufficient submissions"), nil, nil

		case 4:
			// Scenario 4: 3 supernodes submit completely different results (should fail/remain in PROCESSING)
			// Three different sets of data
			for i := 0; i < 3; i++ {
				ddIds := generateRandomKademliaIDs(r, 3)
				// For first supernode, generate a base set of fingerprints
				// For subsequent supernodes, ensure each set is unique
				var fingerprintResults map[string]string
				if i == 0 {
					fingerprintResults = generateConsistentFingerprintResults(r)
				} else {
					// Get previous supernode's fingerprint results and modify them
					prevAction, _ := k.GetActionByID(ctx, actionID)
					var prevMetadata actionapi.SenseMetadata
					err := json.Unmarshal([]byte(prevAction.Metadata), &prevMetadata)
					if err != nil {
						return simtypes.NoOpMsg(types.ModuleName, "FinalizeAction", "failed to unmarshal metadata"), nil, err
					}
					fingerprintResults = generateNonMatchingFingerprintResults(r, prevMetadata.SupernodeFingerprints)
				}

				metadata := generateFinalizeMetadataForSense(r, ctx, k, actionID, fingerprintResults, ddIds)
				signature := signMetadata(supernodes[i], metadata)
				metadataWithSig := addSignatureToMetadata(metadata, signature)

				msg := types.NewMsgFinalizeAction(
					supernodes[i].Address.String(),
					actionID,
					actionapi.ActionType_ACTION_TYPE_SENSE.String(),
					metadataWithSig,
				)

				_, err := msgServSim.FinalizeAction(ctx, msg)
				if err != nil {
					return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), err.Error()), nil, err
				}
			}

			// After 3 different submissions, state should still be PROCESSING
			action, _ := k.GetActionByID(ctx, actionID)
			if action.State != actionapi.ActionState_ACTION_STATE_PROCESSING {
				return simtypes.NoOpMsg(types.ModuleName, "FinalizeAction",
					fmt.Sprintf("unexpected state after 3 different submissions: got %s, expected PROCESSING",
						action.State)), nil, nil
			}

			// Verify no fees were distributed
			for i := 0; i < 3; i++ {
				finalBalance := bk.GetBalance(ctx, supernodes[i].Address, feeDenom)
				if !finalBalance.Equal(initialBalances[supernodes[i].Address.String()]) {
					return simtypes.NoOpMsg(types.ModuleName, "FinalizeAction",
						"fee distributed before consensus reached"), nil, nil
				}
			}

			return simtypes.NewOperationMsg(&types.MsgFinalizeAction{}, true, "success - no consensus with different results"), nil, nil
		}

		// Should never reach here, but return a generic message if somehow we do
		return simtypes.NoOpMsg(types.ModuleName, "FinalizeAction", "unknown scenario"), nil, nil
	}
}

// SimulateMsgFinalizeAction_MetadataValidation simulates attempting to finalize actions with invalid metadata
// This tests that invalid or incomplete metadata submitted during MsgFinalizeAction is correctly rejected
func SimulateMsgFinalizeAction_MetadataValidation(
	ak types.AccountKeeper,
	bk types.BankKeeper,
	k keeper.Keeper,
) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		// Randomly select metadata validation scenario to test
		scenarioType := r.Intn(2)     // 0 for SENSE, 1 for CASCADE
		invalidationType := r.Intn(5) // Different types of invalid metadata

		var actionID string
		var action *actionapi.Action

		// 1. Create the appropriate type of PENDING action
		if scenarioType == 0 {
			// SENSE action
			actionID = createPendingSenseAction(r, ctx, accs, bk, k, ak)
		} else {
			// CASCADE action
			actionID = createPendingCascadeAction(r, ctx, accs, bk, k, ak)
		}

		// 2. Get the created action
		action, found := k.GetActionByID(ctx, actionID)
		if !found {
			return simtypes.NoOpMsg(types.ModuleName, "FinalizeAction", "action not found"), nil, nil
		}

		// 3. Select a random supernode account
		supernode := selectRandomSupernode(r, ctx, accs)

		// 4. Get initial supernode balance to verify no fees are distributed
		feeDenom := k.GetParams(ctx).BaseActionFee.Denom
		initialBalance := bk.GetBalance(ctx, supernode.Address, feeDenom)

		// 5. Generate invalid metadata based on action type and invalidation type
		var invalidMetadata string

		if scenarioType == 0 {
			// SENSE action invalid metadata
			switch invalidationType {
			case 0:
				// Missing DdAndFingerprintsIds
				invalidMetadata = generateSenseMetadataMissingDdIds(action)
			case 1:
				// Empty DdAndFingerprintsIds
				invalidMetadata = generateSenseMetadataEmptyDdIds(action)
			case 2:
				// Invalid DdAndFingerprintsIc count
				invalidMetadata = generateSenseMetadataInvalidDdIc(action)
			case 3:
				// Missing SupernodeFingerprints
				invalidMetadata = generateSenseMetadataMissingFingerprints(action)
			case 4:
				// DataHash mismatch
				invalidMetadata = generateSenseMetadataDataHashMismatch(action)
			}
		} else {
			// CASCADE action invalid metadata
			switch invalidationType {
			case 0:
				// Missing RqIdsIds
				invalidMetadata = generateCascadeMetadataMissingRqIds(action)
			case 1:
				// Empty RqIdsIds
				invalidMetadata = generateCascadeMetadataEmptyRqIds(action)
			case 2:
				// Invalid RqIdsIc count
				invalidMetadata = generateCascadeMetadataInvalidRqIc(action)
			case 3:
				// DataHash mismatch
				invalidMetadata = generateCascadeMetadataDataHashMismatch(action)
			case 4:
				// FileName mismatch
				invalidMetadata = generateCascadeMetadataFileNameMismatch(action)
			}
		}

		// 6. Add supernode signature
		signature := signMetadata(supernode, invalidMetadata)
		metadataWithSig := addSignatureToMetadata(invalidMetadata, signature)

		// 7. Create finalization message
		msg := types.NewMsgFinalizeAction(
			supernode.Address.String(),
			actionID,
			action.ActionType.String(),
			metadataWithSig,
		)

		// 8. Deliver transaction, expecting error
		msgServSim := keeper.NewMsgServerImpl(k)
		_, err := msgServSim.FinalizeAction(ctx, msg)

		// 9. Check that an error occurred
		if err == nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg),
				"expected error when finalizing with invalid metadata"), nil, nil
		}

		// 10. Verify the action state remains unchanged
		updatedAction, found := k.GetActionByID(ctx, actionID)
		if !found {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg),
				"action not found after finalization attempt"), nil, nil
		}

		expectedState := actionapi.ActionState_ACTION_STATE_PENDING
		if action.ActionType == actionapi.ActionType_ACTION_TYPE_SENSE &&
			updatedAction.State == actionapi.ActionState_ACTION_STATE_PROCESSING {
			// For SENSE actions, it's also valid if the state is PROCESSING
			// (if a valid message was sent prior to our test)
			expectedState = actionapi.ActionState_ACTION_STATE_PROCESSING
		}

		if updatedAction.State != expectedState {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg),
				fmt.Sprintf("action state changed despite error: expected %s, got %s",
					expectedState, updatedAction.State)), nil, nil
		}

		// 11. Verify supernode balance remains unchanged (no fees distributed)
		finalBalance := bk.GetBalance(ctx, supernode.Address, feeDenom)
		if !finalBalance.Equal(initialBalance) {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg),
				"balance changed despite error"), nil, nil
		}

		// 12. Return operation message, marking as failed but expected
		return simtypes.NewOperationMsg(msg, false, fmt.Sprintf("invalid metadata validation for %s (%d)",
			action.ActionType, invalidationType)), nil, nil
	}
}
