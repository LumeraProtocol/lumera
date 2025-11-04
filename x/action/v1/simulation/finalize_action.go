package simulation

import (
	"fmt"
	"math/rand"
	"strings"

	"github.com/LumeraProtocol/lumera/x/action/v1/keeper"
	"github.com/LumeraProtocol/lumera/x/action/v1/types"
	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"

	gogoproto "github.com/cosmos/gogoproto/proto"
)

// SimulateMsgFinalizeActionSuccessSense simulates a successful finalization of a SENSE action
func SimulateMsgFinalizeActionSuccessSense(
	ak types.AuthKeeper,
	bk types.BankKeeper,
	k keeper.Keeper,
) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		// 1. Register Action
		actionID, _ := registerSenseAction(r, ctx, accs, bk, k, ak)

		// 2. Select three random supernode accounts
		supernodes, err := getRandomActiveSupernodes(r, ctx, 3, ak, k, accs)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgFinalizeAction{}), "failed to get random supernodes"), nil, nil
		}

		// 3. Store initial balances of supernodes to check fee distribution later
		feeDenom := k.GetParams(ctx).BaseActionFee.Denom
		initialBalance1 := bk.GetBalance(ctx, supernodes[0].Address, feeDenom)
		initialBalance2 := bk.GetBalance(ctx, supernodes[1].Address, feeDenom)
		initialBalance3 := bk.GetBalance(ctx, supernodes[2].Address, feeDenom)

		// 4. Finalize action by all 3 supernodes
		msg := finalizeSenseAction(ctx, k, bk, actionID, supernodes)

		// 5. Check supernode balances to confirm fee distribution
		finalBalance1 := bk.GetBalance(ctx, supernodes[0].Address, feeDenom)
		finalBalance2 := bk.GetBalance(ctx, supernodes[1].Address, feeDenom)
		finalBalance3 := bk.GetBalance(ctx, supernodes[2].Address, feeDenom)

		// All three supernodes should have received fees
		if !finalBalance1.Amount.GT(initialBalance1.Amount) ||
			!finalBalance2.Amount.GT(initialBalance2.Amount) ||
			!finalBalance3.Amount.GT(initialBalance3.Amount) {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), "fee distribution not as expected"), nil, nil
		}

		// 16. Return successful operation message
		return simtypes.NewOperationMsg(msg, true, "success"), nil, nil
	}
}

// SimulateMsgFinalizeActionSuccessCascade simulates a successful finalization of a CASCADE action
func SimulateMsgFinalizeActionSuccessCascade(
	ak types.AuthKeeper,
	bk types.BankKeeper,
	k keeper.Keeper,
) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		// 1. Create a PENDING CASCADE action
		actionID, _ := registerCascadeAction(r, ctx, accs, bk, k, ak)

		// 2. Select three random supernode accounts
		supernodes, err := getRandomActiveSupernodes(r, ctx, 1, ak, k, accs)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgFinalizeAction{}), "failed to get random supernodes"), nil, nil
		}

		// 3. Store initial balance of supernode to check fee distribution later
		feeDenom := k.GetParams(ctx).BaseActionFee.Denom
		initialBalance := bk.GetBalance(ctx, supernodes[0].Address, feeDenom)

		// 4. Finalize action by supernode
		msg := finalizeCascadeAction(ctx, k, actionID, supernodes)

		// 5. Check supernode balance to confirm fee distribution
		finalBalance := bk.GetBalance(ctx, supernodes[0].Address, feeDenom)
		if !finalBalance.Amount.GT(initialBalance.Amount) {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), "fee distribution not as expected"), nil, nil
		}

		// 6. Return successful operation message
		return simtypes.NewOperationMsg(msg, true, "success"), nil, nil
	}
}

// SimulateMsgFinalizeActionInvalidID simulates attempting to finalize an action with a non-existent ID
func SimulateMsgFinalizeActionInvalidID(
	ak types.AuthKeeper,
	bk types.BankKeeper,
	k keeper.Keeper,
) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		// Randomly choose between SENSE and CASCADE metadata
		actionType := selectRandomActionType(r)

		// 1. Select random supernode account
		supernodes, err := getRandomActiveSupernodes(r, ctx, 1, ak, k, accs)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgFinalizeAction{}), "failed to get random supernodes"), nil, nil
		}

		// 2. Generate non-existent action ID
		invalidActionID := generateNonExistentActionID(r, ctx, k)

		// 3. Get initial supernode balance
		feeDenom := k.GetParams(ctx).BaseActionFee.Denom
		initialBalance := bk.GetBalance(ctx, supernodes[0].Address, feeDenom)

		// 4. Create valid metadata (doesn't matter since ID is invalid)
		metadata := generateValidFinalizeMetadata(1, 50, actionType, supernodes, "")

		// 5. Create finalization message
		msg := types.NewMsgFinalizeAction(
			supernodes[0].Address.String(),
			invalidActionID,
			actionType,
			metadata,
		)

		// 6. Deliver transaction, expecting error
		msgServSim := keeper.NewMsgServerImpl(k)
		_, err = msgServSim.FinalizeAction(ctx, msg)

		// 7. Check that an error occurred (should be about invalid action ID)
		if err == nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), "expected error when finalizing with invalid action ID"), nil, nil
		}

		// 8. Verify supernode balance remains unchanged
		finalBalance := bk.GetBalance(ctx, supernodes[0].Address, feeDenom)
		if !finalBalance.Equal(initialBalance) {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), "balance changed despite error"), nil, nil
		}

		// 9. Return operation message, marking as failed but expected
		return simtypes.NewOperationMsg(msg, false, "invalid action ID"), nil, nil
	}
}

// SimulateMsgFinalizeActionInvalidState simulates attempting to finalize an action that is not in PENDING state
func SimulateMsgFinalizeActionInvalidState(
	ak types.AuthKeeper,
	bk types.BankKeeper,
	k keeper.Keeper,
) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		// 1. Create a DONE action
		actionID, action := registerSenseOrCascadeAction(r, ctx, accs, k, bk, ak)
		supernodes, err := finalizeAction(r, ctx, k, ak, bk, actionID, action.ActionType, accs)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgFinalizeAction{}), "failed to get random supernodes"), nil, nil
		}

		// 2. Get initial supernode balance
		feeDenom := k.GetParams(ctx).BaseActionFee.Denom
		initialBalance := bk.GetBalance(ctx, supernodes[0].Address, feeDenom)

		// 3. Create valid metadata
		metadata := generateValidFinalizeMetadata(1, 50, action.ActionType.String(), supernodes, "")

		// 4. Create finalization message
		msg := types.NewMsgFinalizeAction(
			supernodes[0].Address.String(),
			actionID,
			action.ActionType.String(),
			metadata,
		)

		// 6. Deliver transaction, expecting error
		msgServSim := keeper.NewMsgServerImpl(k)
		_, err = msgServSim.FinalizeAction(ctx, msg)

		// 7. Check that an error occurred (should be about invalid action state)
		if err == nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), "expected error when finalizing action in DONE state"), nil, nil
		}

		// Check if the error is about invalid action state
		if !strings.Contains(err.Error(), "invalid state") && !strings.Contains(err.Error(), "not in PENDING state") {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), fmt.Sprintf("unexpected error: %v", err)), nil, nil
		}

		// 8. Verify supernode balance remains unchanged
		finalBalance := bk.GetBalance(ctx, supernodes[0].Address, feeDenom)
		if !finalBalance.Equal(initialBalance) {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), "balance changed despite error"), nil, nil
		}

		// 9. Verify action state remains unchanged - this will be checked against the actual stored action
		unchangedAction, found := k.GetActionByID(ctx, actionID)
		if !found {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), "action not found after finalization attempt"), nil, nil
		}

		// The state should still be PENDING in the store (not DONE which is only in our simulated version)
		if unchangedAction.State != types.ActionStatePending {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), "action state changed despite error"), nil, nil
		}

		// 10. Return operation message, marking as failed but expected
		return simtypes.NewOperationMsg(msg, false, "invalid action state"), nil, nil
	}
}

// SimulateMsgFinalizeActionUnauthorized simulates attempting to finalize an action by a non-supernode account
func SimulateMsgFinalizeActionUnauthorized(
	ak types.AuthKeeper,
	bk types.BankKeeper,
	k keeper.Keeper,
) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		// 1. Create a PENDING action
		actionID, action := registerSenseOrCascadeAction(r, ctx, accs, k, bk, ak)

		// 2. Create valid metadata using regular NON supernode accounts
		metadata := generateValidFinalizeMetadata(1, 50, action.ActionType.String(), accs, "")

		// 4. Create finalization message
		msg := types.NewMsgFinalizeAction(
			accs[0].Address.String(),
			actionID,
			action.ActionType.String(),
			metadata,
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

		if unchangedAction.State != types.ActionStatePending {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), "action state changed despite error"), nil, nil
		}

		// 8. Return operation message, marking as failed but expected
		return simtypes.NewOperationMsg(msg, false, "unauthorized account"), nil, nil
	}
}

// SimulateMsgFinalizeActionSenseConsensus simulates the consensus requirement for SENSE actions
// Testing different scenarios of matching and non-matching results from multiple supernodes
func SimulateMsgFinalizeActionSenseConsensus(
	ak types.AuthKeeper,
	bk types.BankKeeper,
	k keeper.Keeper,
) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		// Random scenario selection - we'll test one scenario per simulation run
		scenario := r.Intn(4) + 1

		// 1. Create a PENDING SENSE action
		actionID, msg := registerSenseAction(r, ctx, accs, bk, k, ak)

		// 2. Select three random supernode accounts
		supernodes, err := getRandomActiveSupernodes(r, ctx, 3, ak, k, accs)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgFinalizeAction{}), "failed to get random supernodes"), nil, nil
		}

		// Store initial balances to verify fee distribution
		feeDenom := k.GetParams(ctx).BaseActionFee.Denom
		initialBalances := make(map[string]sdk.Coin)
		for i := 0; i < len(supernodes); i++ {
			initialBalances[supernodes[i].Address.String()] = bk.GetBalance(ctx, supernodes[i].Address, feeDenom)
		}

		var existingMetadata types.SenseMetadata
		err = gogoproto.Unmarshal([]byte(msg.Metadata), &existingMetadata)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgFinalizeAction{}),
				fmt.Sprintf("failed to unmarshal existing metadata: %v", err)), nil, nil
		}

		msgServSim := keeper.NewMsgServerImpl(k)

		switch scenario {
		case 1:
			// Scenario 1: 2 supernodes submit matching results, 1 submits different results (should fail/remain in PROCESSING)
			// Generate consistent data for first two supernodes
			metadata1 := generateValidFinalizeMetadata(
				existingMetadata.DdAndFingerprintsIc,
				existingMetadata.DdAndFingerprintsMax,
				types.ActionTypeSense.String(),
				supernodes,
				"")

			// First two supernodes submit matching results
			for i := 0; i < 2; i++ {
				msg := types.NewMsgFinalizeAction(
					supernodes[i].Address.String(),
					actionID,
					types.ActionTypeSense.String(),
					metadata1,
				)

				_, err := msgServSim.FinalizeAction(ctx, msg)
				if err != nil {
					return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), err.Error()), nil, err
				}

				// After 1st and 2nd submission, state should be PROCESSING
				action, _ := k.GetActionByID(ctx, actionID)
				if action.State != types.ActionStateProcessing {
					return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg),
						"unexpected state after submission"), nil, nil
				}
			}
			// Verify no fees were distributed
			for i := 0; i < 2; i++ {
				finalBalance := bk.GetBalance(ctx, supernodes[i].Address, feeDenom)
				if !finalBalance.Equal(initialBalances[supernodes[i].Address.String()]) {
					return simtypes.NoOpMsg(types.ModuleName, "FinalizeAction",
						"fee distributed before consensus reached"), nil, nil
				}
			}

			// Third supernode submits different results
			// Each call to generateValidFinalizeMetadata/generateSenseSignature/cryptotestutils.CreateSignatureString generate random Signature String
			metadata3 := generateValidFinalizeMetadata(
				existingMetadata.DdAndFingerprintsIc,
				existingMetadata.DdAndFingerprintsMax,
				types.ActionTypeSense.String(),
				supernodes,
				"")

			msg3 := types.NewMsgFinalizeAction(
				supernodes[2].Address.String(),
				actionID,
				types.ActionTypeSense.String(),
				metadata3,
			)

			_, err := msgServSim.FinalizeAction(ctx, msg3)
			if err != nil {
				return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg3), err.Error()), nil, err
			}

			// After 3rd non-matching submission, state should be FAILED
			action, _ := k.GetActionByID(ctx, actionID)
			if action.State != types.ActionStateFailed {
				return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg3),
					"unexpected state after non-matching submission"), nil, nil
			}

			// Verify no fees were distributed
			for i := 0; i < 3; i++ {
				finalBalance := bk.GetBalance(ctx, supernodes[i].Address, feeDenom)
				if !finalBalance.Equal(initialBalances[supernodes[i].Address.String()]) {
					return simtypes.NoOpMsg(types.ModuleName, "FinalizeAction",
						"fee distributed before consensus reached"), nil, nil
				}
			}

			return simtypes.NewOperationMsg(&types.MsgFinalizeAction{}, true, "success - no consensus with conflicting results"), nil, nil

		case 2:
			// Scenario 4: 3 supernodes submit completely different results (should fail/remain in PROCESSING)
			// Three different sets of data
			for i := 0; i < 3; i++ {
				// Generate different data for third supernode
				// Each call to generateValidFinalizeMetadata/generateSenseSignature/cryptotestutils.CreateSignatureString generate random Signature String
				metadata := generateValidFinalizeMetadata(
					existingMetadata.DdAndFingerprintsIc,
					existingMetadata.DdAndFingerprintsMax,
					types.ActionTypeSense.String(),
					supernodes,
					"")

				msg := types.NewMsgFinalizeAction(
					supernodes[i].Address.String(),
					actionID,
					types.ActionTypeSense.String(),
					metadata,
				)

				_, err := msgServSim.FinalizeAction(ctx, msg)
				if err != nil {
					return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), err.Error()), nil, err
				}
			}

			// After 3 different submissions, state should still be FAILED
			action, _ := k.GetActionByID(ctx, actionID)
			if action.State != types.ActionStateFailed {
				return simtypes.NoOpMsg(types.ModuleName, "FinalizeAction",
					fmt.Sprintf("unexpected state after 3 different submissions: got %s, expected FAILED",
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

// SimulateMsgFinalizeActionMetadataValidation simulates attempting to finalize actions with invalid metadata
// This tests that invalid or incomplete metadata submitted during MsgFinalizeAction is correctly rejected
func SimulateMsgFinalizeActionMetadataValidation(
	ak types.AuthKeeper,
	bk types.BankKeeper,
	k keeper.Keeper,
) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		// Randomly select metadata validation scenario to test
		scenarioType := r.Intn(2)     // 0 for SENSE, 1 for CASCADE
		invalidationType := r.Intn(5) // Different types of invalid metadata

		var actionID string
		var action *types.Action

		// 1. Create the appropriate type of PENDING action
		if scenarioType == 0 {
			// SENSE action
			actionID, _ = registerSenseAction(r, ctx, accs, bk, k, ak)
		} else {
			// CASCADE action
			actionID, _ = registerCascadeAction(r, ctx, accs, bk, k, ak)
		}

		// 2. Get the created action
		action, found := k.GetActionByID(ctx, actionID)
		if !found {
			return simtypes.NoOpMsg(types.ModuleName, "FinalizeAction", "action not found"), nil, nil
		}

		// 3. Select a random supernode account
		supernodes, err := getRandomActiveSupernodes(r, ctx, 3, ak, k, accs)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgFinalizeAction{}), "failed to get random supernodes"), nil, nil
		}

		// 4. Get initial supernode balance to verify no fees are distributed
		feeDenom := k.GetParams(ctx).BaseActionFee.Denom
		initialBalance := bk.GetBalance(ctx, supernodes[0].Address, feeDenom)

		// 5. Generate invalid metadata based on action type and invalidation type
		var invalidMetadata string

		if scenarioType == 0 {
			// SENSE action invalid metadata
			switch invalidationType {
			case 0:
				// Missing DdAndFingerprintsIds
				invalidMetadata = generateFinalizeSenseMetadataMissingDdIds(action, supernodes)
			case 1:
				// Empty DdAndFingerprintsIds
				invalidMetadata = generateSenseMetadataEmptyDdIds(action, supernodes)
			case 2:
				// Invalid DdAndFingerprintsIc count
				invalidMetadata = generateSenseMetadataInvalidDdIc(action, supernodes)
			case 3:
				// Missing SupernodeFingerprints
				invalidMetadata = generateSenseMetadataMissingIds(action, supernodes)
			case 4:
				// DataHash mismatch
				invalidMetadata = generateSenseMetadataSignatureMismatch(action, supernodes)
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
				invalidMetadata = generateCascadeMetadataMissingIds(action)
			case 4:
				// FileName mismatch
				invalidMetadata = generateCascadeMetadataSignatureMismatch(action)
			}
		}

		// 6. Create finalization message
		msg := types.NewMsgFinalizeAction(
			supernodes[0].Address.String(),
			actionID,
			action.ActionType.String(),
			invalidMetadata,
		)

		// 8. Deliver transaction, expecting error
		msgServSim := keeper.NewMsgServerImpl(k)
		_, err = msgServSim.FinalizeAction(ctx, msg)

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

		expectedState := types.ActionStatePending
		if action.ActionType == types.ActionTypeSense &&
			updatedAction.State == types.ActionStateProcessing {
			// For SENSE actions, it's also valid if the state is PROCESSING
			// (if a valid message was sent prior to our test)
			expectedState = types.ActionStateProcessing
		}

		if updatedAction.State != expectedState {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg),
				fmt.Sprintf("action state changed despite error: expected %s, got %s",
					expectedState, updatedAction.State)), nil, nil
		}

		// 11. Verify supernode balance remains unchanged (no fees distributed)
		finalBalance := bk.GetBalance(ctx, supernodes[0].Address, feeDenom)
		if !finalBalance.Equal(initialBalance) {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg),
				"balance changed despite error"), nil, nil
		}

		// 12. Return operation message, marking as failed but expected
		return simtypes.NewOperationMsg(msg, false, fmt.Sprintf("invalid metadata validation for %s (%d)",
			action.ActionType, invalidationType)), nil, nil
	}
}
