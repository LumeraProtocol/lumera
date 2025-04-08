package simulation

import (
	"fmt"
	"math/rand"
	"strconv"
	"time"

	cosmosmath "cosmossdk.io/math"
	actionapi "github.com/LumeraProtocol/lumera/api/lumera/action"
	"github.com/LumeraProtocol/lumera/x/action/keeper"
	"github.com/LumeraProtocol/lumera/x/action/types"
	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
)

// SimulateFeeDistribution_Success simulates the complete fee lifecycle:
// 1. Fee deduction from creator upon action request
// 2. Fee distribution to supernode(s) upon successful finalization
func SimulateFeeDistribution_Success(ak types.AccountKeeper, bk types.BankKeeper, k keeper.Keeper) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, chainID string) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		// 1. Select action type randomly (SENSE or CASCADE)
		actionType := selectRandomActionType(r)

		// 2. Select creator account with sufficient funds
		creator := selectRandomAccountWithSufficientFunds(r, ctx, accs, bk, ak)

		// 3. Record creator's initial balance
		feeDenom := k.GetParams(ctx).BaseActionFee.Denom
		creatorInitialBalance := bk.GetBalance(ctx, creator.Address, feeDenom)

		// 4. Generate fee amount (using the helper function but not directly needing the result,
		// as createPendingXxxAction will handle fee generation internally)
		_ = generateRandomFee(r, ctx, k.GetParams(ctx).BaseActionFee)

		// 5. Select and record initial balances of participating supernodes
		var supernodes []simtypes.Account
		supernodeInitialBalances := make(map[string]sdk.Coin)

		if actionType == actionapi.ActionType_ACTION_TYPE_CASCADE.String() {
			// CASCADE: single supernode
			supernodes = selectRandomSupernodes(r, ctx, accs, 1)
		} else {
			// SENSE: three supernodes
			supernodes = selectRandomSupernodes(r, ctx, accs, 3)
		}

		for _, sn := range supernodes {
			supernodeInitialBalances[sn.Address.String()] = bk.GetBalance(ctx, sn.Address, feeDenom)
		}

		// 6. Create the action
		var actionID string
		if actionType == actionapi.ActionType_ACTION_TYPE_CASCADE.String() {
			actionID = createPendingCascadeAction(r, ctx, accs, bk, k, ak)
		} else {
			actionID = createPendingSenseAction(r, ctx, accs, bk, k, ak)
		}

		// 7. Get the action details
		action, found := k.GetActionByID(ctx, actionID)
		if !found {
			return simtypes.NoOpMsg(types.ModuleName, "fee_distribution", "created action not found"), nil, nil
		}

		// 8. Verify creator's balance decreased by fee amount
		// Since we called createPendingCascadeAction or createPendingSenseAction, we need to get the actual creator
		actualCreator, found := FindAccount(accs, action.Creator)
		if !found {
			return simtypes.NoOpMsg(types.ModuleName, "fee_distribution", "creator not found"), nil, nil
		}

		creatorBalanceAfterRequest := bk.GetBalance(ctx, actualCreator.Address, feeDenom)
		if !creatorBalanceAfterRequest.Amount.LT(creatorInitialBalance.Amount) {
			return simtypes.NoOpMsg(types.ModuleName, "fee_distribution", "creator balance not decreased after request"), nil, nil
		}

		// Calculate the actual deducted amount
		deductedAmount := creatorInitialBalance.Amount.Sub(creatorBalanceAfterRequest.Amount)

		// 9. Finalize the action successfully
		if actionType == actionapi.ActionType_ACTION_TYPE_CASCADE.String() {
			// Finalize CASCADE action with single supernode
			finalizeCascadeAction(ctx, k, actionID, supernodes[0])
		} else {
			// Finalize SENSE action with three supernodes consensus
			finalizeSenseActionWithConsensus(ctx, k, actionID, supernodes)
		}

		// 10. Verify action is in DONE state
		finalizedAction, found := k.GetActionByID(ctx, actionID)
		if !found || finalizedAction.State != actionapi.ActionState_ACTION_STATE_DONE {
			return simtypes.NoOpMsg(types.ModuleName, "fee_distribution", "action not in DONE state after finalization"), nil, nil
		}

		// 11. Check supernode(s) balances and verify fee distribution
		totalDistributed := cosmosmath.ZeroInt()

		for _, sn := range supernodes {
			finalBalance := bk.GetBalance(ctx, sn.Address, feeDenom)
			initialBalance := supernodeInitialBalances[sn.Address.String()]

			feeReceived := finalBalance.Amount.Sub(initialBalance.Amount)
			if feeReceived.IsZero() || feeReceived.IsNegative() {
				return simtypes.NoOpMsg(types.ModuleName, "fee_distribution", "supernode did not receive fee"), nil, nil
			}

			totalDistributed = totalDistributed.Add(feeReceived)
		}

		// 12. Verify total distributed equals deducted amount
		if !totalDistributed.Equal(deductedAmount) {
			return simtypes.NoOpMsg(types.ModuleName, "fee_distribution", "total distributed fee does not match deducted amount"), nil, nil
		}

		return simtypes.NewOperationMsg(&types.MsgRequestAction{}, true, "fee_distribution_success"), nil, nil
	}
}

// SimulateFeeDistribution_MultipleSuperNodes simulates fee distribution among 3 supernodes
// for a successfully finalized SENSE action, verifying that each supernode receives
// an equal share (1/3) of the total fee.
func SimulateFeeDistribution_MultipleSuperNodes(ak types.AccountKeeper, bk types.BankKeeper, k keeper.Keeper) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, chainID string) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		// 1. Select creator account with sufficient funds
		creator := selectRandomAccountWithSufficientFunds(r, ctx, accs, bk, ak)

		// 2. Record creator's initial balance
		feeDenom := k.GetParams(ctx).BaseActionFee.Denom
		creatorInitialBalance := bk.GetBalance(ctx, creator.Address, feeDenom)

		// 3. Generate fee amount (ensure it's divisible by 3 for clean division)
		baseFee := k.GetParams(ctx).BaseActionFee.Amount
		multiplier := simtypes.RandIntBetween(r, 3, 10)                         // Ensure it's at least 3 for clean division
		feeAmount := sdk.NewCoin(feeDenom, baseFee.MulRaw(int64(multiplier*3))) // Make divisible by 3

		// 4. Select exactly 3 supernodes and record their initial balances
		supernodes := selectRandomSupernodes(r, ctx, accs, 3)
		if len(supernodes) != 3 {
			return simtypes.NoOpMsg(types.ModuleName, "fee_distribution_multiple",
				"couldn't find 3 supernodes"), nil, nil
		}

		supernodeInitialBalances := make(map[string]sdk.Coin)
		for _, sn := range supernodes {
			supernodeInitialBalances[sn.Address.String()] = bk.GetBalance(ctx, sn.Address, feeDenom)
		}

		// 5. Create a SENSE action with the specified fee
		dataHash := generateRandomHash(r)
		senseMetadata := generateRequestActionSenseMetadata(dataHash)
		expirationDuration := time.Duration(r.Int63n(int64(k.GetParams(ctx).ExpirationDuration)))
		expirationTime := ctx.BlockTime().Add(expirationDuration).Unix()

		msg := types.NewMsgRequestAction(
			creator.Address.String(),
			actionapi.ActionType_ACTION_TYPE_SENSE.String(),
			senseMetadata,
			feeAmount.String(),
			strconv.FormatInt(expirationTime, 10),
		)

		msgServSim := keeper.NewMsgServerImpl(k)
		result, err := msgServSim.RequestAction(ctx, msg)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, "fee_distribution_multiple",
				fmt.Sprintf("failed to create SENSE action: %v", err)), nil, nil
		}

		actionID := result.ActionId

		// 6. Verify creator's balance decreased by fee amount
		creatorBalanceAfterRequest := bk.GetBalance(ctx, creator.Address, feeDenom)
		expectedCreatorBalance := creatorInitialBalance.Sub(feeAmount)
		if !creatorBalanceAfterRequest.Equal(expectedCreatorBalance) {
			return simtypes.NoOpMsg(types.ModuleName, "fee_distribution_multiple",
				"creator balance not decreased correctly"), nil, nil
		}

		// 7. Finalize SENSE action with the three supernodes achieving consensus
		finalizeSenseActionWithConsensus(ctx, k, actionID, supernodes)

		// 8. Verify action is in DONE state
		finalizedAction, found := k.GetActionByID(ctx, actionID)
		if !found || finalizedAction.State != actionapi.ActionState_ACTION_STATE_DONE {
			return simtypes.NoOpMsg(types.ModuleName, "fee_distribution_multiple",
				"action not in DONE state after finalization"), nil, nil
		}

		// 9. Check each supernode's balance and verify equal fee distribution
		expectedFeePerNode := feeAmount.Amount.QuoRaw(3) // Divide by 3 for equal distribution
		totalDistributed := cosmosmath.ZeroInt()

		for _, sn := range supernodes {
			finalBalance := bk.GetBalance(ctx, sn.Address, feeDenom)
			initialBalance := supernodeInitialBalances[sn.Address.String()]

			feeReceived := finalBalance.Amount.Sub(initialBalance.Amount)
			if feeReceived.IsZero() || feeReceived.IsNegative() {
				return simtypes.NoOpMsg(types.ModuleName, "fee_distribution_multiple",
					"supernode did not receive fee"), nil, nil
			}

			// Verify this supernode received exactly 1/3 of the fee
			if !feeReceived.Equal(expectedFeePerNode) {
				return simtypes.NoOpMsg(types.ModuleName, "fee_distribution_multiple",
					"supernode did not receive equal share (1/3) of fee"), nil, nil
			}

			totalDistributed = totalDistributed.Add(feeReceived)
		}

		// 10. Verify total distributed equals fee amount
		if !totalDistributed.Equal(feeAmount.Amount) {
			return simtypes.NoOpMsg(types.ModuleName, "fee_distribution_multiple",
				"total distributed fee does not match expected amount"), nil, nil
		}

		return simtypes.NewOperationMsg(&types.MsgRequestAction{}, true,
			"fee_distribution_multiple_supernodes_success"), nil, nil
	}
}
