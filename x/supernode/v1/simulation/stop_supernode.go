package simulation

import (
	"math/rand"

	keeper2 "github.com/LumeraProtocol/lumera/x/supernode/v1/keeper"
	types2 "github.com/LumeraProtocol/lumera/x/supernode/v1/types"

	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
)

const (
	TypeMsgStopSupernode = "stop_supernode"
)

func SimulateMsgStopSupernode(
	ak types2.AccountKeeper,
	bk types2.BankKeeper,
	k keeper2.Keeper,
) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		// Find a validator with an active supernode
		var simAccount simtypes.Account
		var found bool
		var validatorAddress string

		stakingkeepr := k.GetStakingKeeper()
		// Try up to 10 times to find an eligible validator
		for i := 0; i < 10; i++ {
			simAccount, _ = simtypes.RandomAcc(r, accs)
			valAddr := sdk.ValAddress(simAccount.Address)

			validator, err := stakingkeepr.Validator(ctx, valAddr)
			if err != nil {
				continue
			}

			// Check if supernode exists
			supernode, exists := k.QuerySuperNode(ctx, valAddr)
			if !exists || len(supernode.States) == 0 {
				continue
			}

			// Check current state
			currentState := supernode.States[len(supernode.States)-1].State
			if currentState == types2.SuperNodeStateStopped || currentState == types2.SuperNodeStateDisabled {
				continue
			}

			validatorAddress = validator.GetOperator()
			found = true
			break
		}

		// If we couldn't find an eligible supernode, skip this operation
		if !found {
			return simtypes.NoOpMsg(types2.ModuleName, TypeMsgStopSupernode, "no eligible supernode found"), nil, nil
		}

		// Generate random reason for stopping
		reasons := []string{
			"Maintenance",
			"System upgrade",
			"Network issues",
			"Performance optimization",
			"Scheduled downtime",
		}
		randomReason := reasons[r.Intn(len(reasons))]

		msg := &types2.MsgStopSupernode{
			Creator:          simAccount.Address.String(),
			ValidatorAddress: validatorAddress,
			Reason:           randomReason,
		}

		// Execute the message
		msgServer := keeper2.NewMsgServerImpl(k)
		_, err := msgServer.StopSupernode(ctx, msg)
		if err != nil {
			return simtypes.NoOpMsg(types2.ModuleName, TypeMsgStopSupernode, err.Error()), nil, err
		}

		return simtypes.NewOperationMsg(msg, true, "success"), nil, nil
	}
}
