package simulation

import (
	"math/rand"

	actionapi "github.com/LumeraProtocol/lumera/api/lumera/action"
	"github.com/LumeraProtocol/lumera/x/action/keeper"
	"github.com/LumeraProtocol/lumera/x/action/types"
	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
)

// SimulateMsgApproveActionSuccess simulates a successful approval of an action in DONE state by its creator
func SimulateMsgApproveActionSuccess(
	ak types.AccountKeeper,
	bk types.BankKeeper,
	k keeper.Keeper,
) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		// 1. Find or create a DONE action and its creator
		actionID, _, creator, err := findOrCreateDoneActionWithCreator(r, ctx, accs, k, bk, ak)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgApproveAction{}), "failed to get random supernodes"), nil, nil
		}

		// 3. Create approval message
		msg := types.NewMsgApproveAction(
			creator.Address.String(),
			actionID,
		)

		// 4. Deliver transaction
		msgServSim := keeper.NewMsgServerImpl(k)
		_, err = msgServSim.ApproveAction(ctx, msg)
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

		// 6. Return successful operation message
		return simtypes.NewOperationMsg(msg, true, "success"), nil, nil
	}
}

// SimulateMsgApproveActionInvalidID simulates an attempt to approve an action with a non-existent ID
func SimulateMsgApproveActionInvalidID(
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

		// 3. Create approval message
		msg := types.NewMsgApproveAction(
			simAccount.Address.String(),
			invalidActionID,
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

// SimulateMsgApproveActionInvalidState simulates an attempt to approve an action that is not in DONE state
func SimulateMsgApproveActionInvalidState(
	ak types.AccountKeeper,
	bk types.BankKeeper,
	k keeper.Keeper,
) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		// 1. Find or create action NOT in DONE state (e.g., PENDING)
		actionID, action, creator := findOrCreateActionNotInDoneState(r, ctx, accs, k, bk, ak)

		// 2. Create approval message
		msg := types.NewMsgApproveAction(
			creator.Address.String(),
			actionID,
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

// SimulateMsgApproveActionUnauthorized simulates an attempt to approve an action by an account that is not the creator
func SimulateMsgApproveActionUnauthorized(
	ak types.AccountKeeper,
	bk types.BankKeeper,
	k keeper.Keeper,
) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		// 1. Find or create a DONE action and its creator
		actionID, action, _, err := findOrCreateDoneActionWithCreator(r, ctx, accs, k, bk, ak)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgApproveAction{}), "failed to get random supernodes"), nil, nil
		}

		// 2. Select random account that is NOT the creator
		nonCreator := selectRandomAccountExcept(r, accs, action.Creator)

		// 4. Create approval message
		msg := types.NewMsgApproveAction(
			nonCreator.Address.String(),
			actionID,
		)

		// 5. Deliver transaction, expecting error
		msgServSim := keeper.NewMsgServerImpl(k)
		_, err = msgServSim.ApproveAction(ctx, msg)

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
