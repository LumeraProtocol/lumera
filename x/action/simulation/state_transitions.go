package simulation

import (
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
		actionID, action := registerSenseOrCascadeAction(r, ctx, accs, k, bk, ak)
		if action == nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgRequestAction{}), "failed to find or create pending action"), nil, nil
		}

		// 2. Get the action's expiration time
		expirationTime := action.ExpirationTime

		// 3. Verify action is in PENDING state
		if action.State != actionapi.ActionState_ACTION_STATE_PENDING {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgRequestAction{}),
				fmt.Sprintf("action not in PENDING state, got %s", action.State)), nil, nil
		}

		// 4. Advance block time past the expiration time
		newTime := time.Unix(expirationTime, 0).Add(time.Minute) // 1 minute past expiration
		newHeader := ctx.BlockHeader()
		newHeader.Time = newTime
		futureCtx := ctx.WithBlockHeader(newHeader)

		// 5. Query the action in the future context to check its state
		actionAfterExpiry, found := k.GetActionByID(futureCtx, actionID)
		if !found {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgRequestAction{}),
				"action not found after time advance"), nil, nil
		}

		// 6. Verify the action state changed to EXPIRED
		if actionAfterExpiry.State != actionapi.ActionState_ACTION_STATE_EXPIRED {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgRequestAction{}),
				fmt.Sprintf("action state not EXPIRED after expiration, got %s", actionAfterExpiry.State)), nil, nil
		}

		// 7. Return successful operation message
		return simtypes.NewOperationMsg(&types.MsgRequestAction{}, true, "action_expiration_success"), nil, nil
	}
}
