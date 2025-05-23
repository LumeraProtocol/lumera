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
	TypeMsgDeregisterSupernode = "deregister_supernode"
)

func SimulateMsgDeregisterSupernode(
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

			// Check if supernode exists and is active
			supernode, exists := k.QuerySuperNode(ctx, valAddr)
			if !exists {
				continue
			}

			// Check if supernode is not already disabled
			if len(supernode.States) > 0 && supernode.States[len(supernode.States)-1].State != types2.SuperNodeStateDisabled {
				validatorAddress = validator.GetOperator()
				found = true
				break
			}
		}

		// If we couldn't find an eligible supernode, skip this operation
		if !found {
			return simtypes.NoOpMsg(types2.ModuleName, TypeMsgDeregisterSupernode, "no eligible supernode found"), nil, nil
		}

		msg := &types2.MsgDeregisterSupernode{
			Creator:          simAccount.Address.String(),
			ValidatorAddress: validatorAddress,
		}

		// Execute the message
		msgServer := keeper2.NewMsgServerImpl(k)
		_, err := msgServer.DeregisterSupernode(ctx, msg)
		if err != nil {
			return simtypes.NoOpMsg(types2.ModuleName, TypeMsgDeregisterSupernode, err.Error()), nil, err
		}

		return simtypes.NewOperationMsg(msg, true, "success"), nil, nil
	}
}
