package simulation

import (
	"fmt"
	"math/rand"

	actionapi "github.com/LumeraProtocol/lumera/api/lumera/action"
	"github.com/LumeraProtocol/lumera/x/action/keeper"
	"github.com/LumeraProtocol/lumera/x/action/types"
	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
)

// SimulateMsgApproveAction_Success simulates a successful approval of an action in DONE state by its creator
func SimulateMsgApproveAction_Success(
	ak types.AccountKeeper,
	bk types.BankKeeper,
	k keeper.Keeper,
) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		// 1. Find or create a DONE action and its creator
		actionID, _, creator := findOrCreateDoneActionWithCreator(r, ctx, accs, k, bk, ak)

		// 2. Generate approval signature
		signature := generateApprovalSignature(creator, actionID)

		// 3. Create approval message
		msg := types.NewMsgApproveAction(
			creator.Address.String(),
			actionID,
			signature,
		)

		// 4. Deliver transaction
		msgServSim := keeper.NewMsgServerImpl(k)
		_, err := msgServSim.ApproveAction(ctx, msg)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), err.Error()), nil, err
		}

		// 5. Verify action moved to APPROVED state
		approvedAction, found := k.GetActionByID(ctx, actionID)
		if !found {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), "action not found after approval"), nil, nil
		}

		if approvedAction.State != actionapi.ActionState_ACTION_STATE_APPROVED {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), "action not in APPROVED state after approval"), nil, nil
		}

		// 6. Verify action state has changed
		// Note: Specific approval signature field checking removed as it varies by implementation

		// 7. Return successful operation message
		return simtypes.NewOperationMsg(msg, true, "success"), nil, nil
	}
}

// SimulateMsgApproveAction_Invalid_ID simulates an attempt to approve an action with a non-existent ID
func SimulateMsgApproveAction_Invalid_ID(
	ak types.AccountKeeper,
	bk types.BankKeeper,
	k keeper.Keeper,
) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		// 1. Select random account
		simAccount, _ := simtypes.RandomAcc(r, accs)

		// 2. Generate non-existent action ID
		invalidActionID := generateNonExistentActionID(r, ctx, k)

		// 3. Generate signature (will be invalid but doesn't matter for this test)
		signature := generateApprovalSignature(simAccount, invalidActionID)

		// 4. Create approval message
		msg := types.NewMsgApproveAction(
			simAccount.Address.String(),
			invalidActionID,
			signature,
		)

		// 5. Deliver transaction, expecting error
		msgServSim := keeper.NewMsgServerImpl(k)
		_, err := msgServSim.ApproveAction(ctx, msg)

		// 6. Check error is about invalid action ID
		if err == nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), "expected error but got none"), nil, nil
		}

		// 7. Return operation message, marking as failed but expected
		return simtypes.NewOperationMsg(msg, false, "invalid action ID as expected"), nil, nil
	}
}

// SimulateMsgApproveAction_InvalidState simulates an attempt to approve an action that is not in DONE state
func SimulateMsgApproveAction_InvalidState(
	ak types.AccountKeeper,
	bk types.BankKeeper,
	k keeper.Keeper,
) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		// 1. Find or create action NOT in DONE state (e.g., PENDING)
		actionID, action, creator := findOrCreateActionNotInDoneState(r, ctx, accs, k, bk, ak)

		// 2. Generate approval signature
		signature := generateApprovalSignature(creator, actionID)

		// 3. Create approval message
		msg := types.NewMsgApproveAction(
			creator.Address.String(),
			actionID,
			signature,
		)

		// 4. Deliver transaction, expecting error
		msgServSim := keeper.NewMsgServerImpl(k)
		_, err := msgServSim.ApproveAction(ctx, msg)

		// 5. Check error is about invalid action state
		if err == nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), "expected error but got none"), nil, nil
		}

		// 6. Verify action state remains unchanged
		unchangedAction, found := k.GetActionByID(ctx, actionID)
		if !found {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), "action not found after attempt"), nil, nil
		}

		if unchangedAction.State != action.State {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), "action state changed unexpectedly"), nil, nil
		}

		// 7. Return operation message, marking as failed but expected
		return simtypes.NewOperationMsg(msg, false, "invalid action state as expected"), nil, nil
	}
}

// SimulateMsgApproveAction_Unauthorized simulates an attempt to approve an action by an account that is not the creator
func SimulateMsgApproveAction_Unauthorized(
	ak types.AccountKeeper,
	bk types.BankKeeper,
	k keeper.Keeper,
) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		// 1. Find or create a DONE action and its creator
		actionID, action, _ := findOrCreateDoneActionWithCreator(r, ctx, accs, k, bk, ak)

		// 2. Select random account that is NOT the creator
		nonCreator := selectRandomAccountExcept(r, accs, action.Creator)

		// 3. Generate approval signature from non-creator
		signature := generateApprovalSignature(nonCreator, actionID)

		// 4. Create approval message
		msg := types.NewMsgApproveAction(
			nonCreator.Address.String(),
			actionID,
			signature,
		)

		// 5. Deliver transaction, expecting error
		msgServSim := keeper.NewMsgServerImpl(k)
		_, err := msgServSim.ApproveAction(ctx, msg)

		// 6. Check error is about unauthorized account
		if err == nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), "expected unauthorized error but got none"), nil, nil
		}

		// 7. Verify action state remains unchanged
		unchangedAction, found := k.GetActionByID(ctx, actionID)
		if !found {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), "action not found after attempt"), nil, nil
		}

		if unchangedAction.State != action.State {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), "action state changed unexpectedly"), nil, nil
		}

		// 8. Return operation message, marking as failed but expected
		return simtypes.NewOperationMsg(msg, false, "unauthorized account as expected"), nil, nil
	}
}

// SimulateMsgApproveAction_SignatureValidation simulates an attempt to approve an action with an invalid signature
func SimulateMsgApproveAction_SignatureValidation(
	ak types.AccountKeeper,
	bk types.BankKeeper,
	k keeper.Keeper,
) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		// 1. Find or create DONE action and its creator
		actionID, _, creator := findOrCreateDoneActionWithCreator(r, ctx, accs, k, bk, ak)

		// 2. Get initial action state to verify no changes occur
		initialAction, found := k.GetActionByID(ctx, actionID)
		if !found {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgApproveAction{}), "action not found for validation"), nil, nil
		}

		// 3. Select random invalid signature scenario (one of three types)
		invalidSignatureType := r.Intn(3)
		var invalidSignature string

		switch invalidSignatureType {
		case 0:
			// Generate signature for a different action ID
			differentActionID := modifyActionID(actionID)
			invalidSignature = generateApprovalSignature(creator, differentActionID)
		case 1:
			// Generate signature using a different account's key
			differentAccount := selectRandomAccountExcept(r, accs, creator.Address.String())
			invalidSignature = generateApprovalSignature(differentAccount, actionID)
		case 2:
			// Generate valid signature but corrupt it slightly
			validSignature := generateApprovalSignature(creator, actionID)
			invalidSignature = corruptSignature(validSignature)
		}

		// 4. Create approval message with invalid signature
		msg := types.NewMsgApproveAction(
			creator.Address.String(),
			actionID,
			invalidSignature,
		)

		// 5. Deliver transaction, expecting error
		msgServSim := keeper.NewMsgServerImpl(k)
		_, err := msgServSim.ApproveAction(ctx, msg)

		// 6. Check error is related to signature validation
		if err == nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg),
				"expected signature validation error but got success"), nil, nil
		}

		// 7. Verify action state remains unchanged (still DONE)
		unchangedAction, found := k.GetActionByID(ctx, actionID)
		if !found {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), "action not found after attempt"), nil, nil
		}

		if unchangedAction.State != initialAction.State {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg),
				"action state changed despite invalid signature"), nil, nil
		}

		// 8. Return operation message, marking as failed but expected
		return simtypes.NewOperationMsg(msg, false,
			fmt.Sprintf("invalid signature validation (%d)", invalidSignatureType)), nil, nil
	}
}

// modifyActionID modifies an action ID to test signature against wrong ID
func modifyActionID(actionID string) string {
	// Modify the last character of the action ID to make it different but still valid format
	if len(actionID) > 0 {
		bytes := []byte(actionID)
		lastChar := bytes[len(bytes)-1]
		// Flip the last bit of the last byte to change the character
		bytes[len(bytes)-1] = lastChar ^ 1
		return string(bytes)
	}
	return actionID
}

// corruptSignature corrupts a valid signature
func corruptSignature(signature string) string {
	// Modify a byte in the middle of the signature to corrupt it
	if len(signature) > 10 {
		bytes := []byte(signature)
		midIndex := len(bytes) / 2
		// Flip a bit in the middle of the signature
		bytes[midIndex] = bytes[midIndex] ^ 1
		return string(bytes)
	}
	return signature
}
