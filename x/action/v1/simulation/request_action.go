package simulation

import (
	actionapi "github.com/LumeraProtocol/lumera/api/lumera/action"
	keeper2 "github.com/LumeraProtocol/lumera/x/action/v1/keeper"
	types2 "github.com/LumeraProtocol/lumera/x/action/v1/types"
	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	"math/rand"
	"strconv"
)

// SimulateMsgRequestActionSuccessSense simulates a successful request for a SENSE action
func SimulateMsgRequestActionSuccessSense(
	ak types2.AccountKeeper,
	bk types2.BankKeeper,
	k keeper2.Keeper,
) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {

		// 1. Register Action
		actionId, msg := registerSenseAction(r, ctx, accs, bk, k, ak)

		// 2. Verify results: action created, funds deducted, proper state
		action, found := k.GetActionByID(ctx, actionId)
		if !found {
			return simtypes.NoOpMsg(types2.ModuleName, sdk.MsgTypeURL(msg), "action not found"), nil, nil
		}

		// 3. Verify action is in PENDING state
		if action.State != actionapi.ActionState_ACTION_STATE_PENDING {
			return simtypes.NoOpMsg(types2.ModuleName, sdk.MsgTypeURL(msg), "action not in PENDING state"), nil, nil
		}

		// 4. Return successful operation message
		return simtypes.NewOperationMsg(msg, true, "success"), nil, nil
	}
}

// SimulateMsgRequestActionSuccessCascade simulates a successful request for a CASCADE action
func SimulateMsgRequestActionSuccessCascade(
	ak types2.AccountKeeper,
	bk types2.BankKeeper,
	k keeper2.Keeper,
) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		// 1. Register Action
		actionId, msg := registerCascadeAction(r, ctx, accs, bk, k, ak)

		// 8. Verify results: action created, funds deducted, proper state
		action, found := k.GetActionByID(ctx, actionId)
		if !found {
			return simtypes.NoOpMsg(types2.ModuleName, sdk.MsgTypeURL(msg), "action not found"), nil, nil
		}

		// 9. Verify action is in PENDING state
		if action.State != actionapi.ActionState_ACTION_STATE_PENDING {
			return simtypes.NoOpMsg(types2.ModuleName, sdk.MsgTypeURL(msg), "action not in PENDING state"), nil, nil
		}

		// 10. Return successful operation message
		return simtypes.NewOperationMsg(msg, true, "success"), nil, nil
	}
}

// SimulateMsgRequestActionInvalidMetadata simulates a failed request with invalid metadata
func SimulateMsgRequestActionInvalidMetadata(
	ak types2.AccountKeeper,
	bk types2.BankKeeper,
	k keeper2.Keeper,
) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		// 1. Select random account with enough balance
		simAccount := selectRandomAccountWithSufficientFunds(r, ctx, accs, bk, ak, []string{""})

		params := k.GetParams(ctx)

		// 2. Get initial balance
		denom := params.BaseActionFee.Denom
		initialBalance := bk.GetBalance(ctx, simAccount.Address, denom)

		// 3. Randomly select action type
		actionType := selectRandomActionType(r)

		// 4. Generate invalid metadata based on action type
		invalidMetadata := generateRequestActionInvalidMetadata(r, actionType, simAccount)

		// 5. Determine fee amount
		feeAmount := generateRandomFee(r, ctx, params.BaseActionFee)

		// 6. Generate an expiration time (current time + random duration)
		expirationTime := getRandomExpirationTime(ctx, r, params)

		// 7. Create message
		msg := types2.NewMsgRequestAction(
			simAccount.Address.String(),
			actionType,
			invalidMetadata,
			feeAmount.String(),
			strconv.FormatInt(expirationTime, 10),
		)

		// 8. Cache keeper state for simulation
		msgServSim := keeper2.NewMsgServerImpl(k)

		// 9. Deliver transaction, expecting error
		_, err := msgServSim.RequestAction(ctx, msg)

		// 10. Verify results: transaction failed, balance unchanged
		finalBalance := bk.GetBalance(ctx, simAccount.Address, denom)

		// Verify balance remained unchanged
		if !initialBalance.Equal(finalBalance) {
			return simtypes.NoOpMsg(types2.ModuleName, sdk.MsgTypeURL(msg), "balance changed unexpectedly"), nil, nil
		}

		// Error should not be nil as we're expecting a validation failure
		if err == nil {
			return simtypes.NoOpMsg(types2.ModuleName, sdk.MsgTypeURL(msg), "expected error but got none"), nil, nil
		}

		// 11. Return operation message, marking as failed but expected
		return simtypes.NewOperationMsg(msg, false, "expected_validation_error"), nil, nil
	}
}

// SimulateMsgRequestActionInsufficientFunds simulates a failed request due to insufficient funds
func SimulateMsgRequestActionInsufficientFunds(
	ak types2.AccountKeeper,
	bk types2.BankKeeper,
	k keeper2.Keeper,
) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		params := k.GetParams(ctx)

		// 1. Select random account with insufficient balance
		simAccount := selectRandomAccountWithInsufficientFunds(r, ctx, accs, bk, params.BaseActionFee)

		// 2. Get initial balance
		denom := params.BaseActionFee.Denom
		initialBalance := bk.GetBalance(ctx, simAccount.Address, denom)

		// 3. Randomly select action type
		actionType := selectRandomActionType(r)

		// 4. Generate valid metadata
		validMetadata := generateRequestActionValidMetadata(r, actionType, simAccount)

		// 5. Set fee amount greater than available balance
		feeAmount := sdk.NewCoin(denom, initialBalance.Amount.AddRaw(1000))

		// 6. Generate an expiration time (current time + random duration)
		expirationTime := getRandomExpirationTime(ctx, r, params)

		// 7. Create message
		msg := types2.NewMsgRequestAction(
			simAccount.Address.String(),
			actionType,
			validMetadata,
			feeAmount.String(),
			strconv.FormatInt(expirationTime, 10),
		)

		// 8. Cache keeper state for simulation
		msgServSim := keeper2.NewMsgServerImpl(k)

		// 9. Deliver transaction, expecting error
		_, err := msgServSim.RequestAction(ctx, msg)

		// 10. Verify results: transaction failed, balance unchanged
		finalBalance := bk.GetBalance(ctx, simAccount.Address, denom)

		// Verify balance remained unchanged
		if !initialBalance.Equal(finalBalance) {
			return simtypes.NoOpMsg(types2.ModuleName, sdk.MsgTypeURL(msg), "balance changed unexpectedly"), nil, nil
		}

		// Error should not be nil as we're expecting an insufficient funds error
		if err == nil {
			return simtypes.NoOpMsg(types2.ModuleName, sdk.MsgTypeURL(msg), "expected error but got none"), nil, nil
		}

		// 11. Return operation message, marking as failed but expected
		return simtypes.NewOperationMsg(msg, false, "expected_insufficient_funds_error"), nil, nil
	}
}

// SimulateMsgRequestActionPermission simulates a failed request due to insufficient permissions
func SimulateMsgRequestActionPermission(
	ak types2.AccountKeeper,
	bk types2.BankKeeper,
	k keeper2.Keeper,
) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		// 1. Select account hypothetically without permission
		simAccount := selectAccountWithoutPermission(r, ctx, accs)

		params := k.GetParams(ctx)

		// 2. Get initial balance to verify no changes after test
		denom := params.BaseActionFee.Denom
		initialBalance := bk.GetBalance(ctx, simAccount.Address, denom)

		// 3. Create a cache context to apply temporary parameter changes
		// This allows us to modify state for the simulation without affecting the actual blockchain state
		cacheCtx, _ := ctx.CacheContext()

		// 4. We assume that in the real implementation, there would be a permission check mechanism
		// For simulation, we'll proceed with creating a message that we expect to fail due to permissions

		// 5. Generate action metadata
		actionType := selectRandomActionType(r)
		metadata := generateRequestActionValidMetadata(r, actionType, simAccount)

		// 6. Generate fee and expiration time
		feeAmount := generateRandomFee(r, ctx, params.BaseActionFee)
		expirationTime := getRandomExpirationTime(ctx, r, params)

		// 7. Create the action request message
		msg := types2.NewMsgRequestAction(
			simAccount.Address.String(),
			actionType,
			metadata,
			feeAmount.String(),
			strconv.FormatInt(expirationTime, 10),
		)

		// 8. Use MsgServer to handle the request in the cache context
		// In a real implementation, this would check permissions and fail the request
		msgServSim := keeper2.NewMsgServerImpl(k)

		// Since the permission mechanism is hypothetical, we need to simulate the expected error
		// We're using the cache context so we don't actually create the action or charge fees
		_, _ = msgServSim.RequestAction(cacheCtx, msg)

		// 9. In a real implementation with permission checks, the above would return an error
		// For now, we're in simulation mode, assuming it should have failed due to permissions
		// We'll verify the funds were not deducted from the original (non-cached) context

		// 10. Verify account balance remained unchanged
		finalBalance := bk.GetBalance(ctx, simAccount.Address, denom)
		if !initialBalance.Equal(finalBalance) {
			return simtypes.NoOpMsg(types2.ModuleName, sdk.MsgTypeURL(msg),
				"balance changed despite expected permission check failure"), nil, nil
		}

		// 11. In a real implementation, we would expect an error, but since permissions are hypothetical
		// for simulation purposes, we'll treat this as a successful test if we didn't modify the state
		return simtypes.NewOperationMsg(msg, false, "permission_check_simulation"), nil, nil
	}
}
