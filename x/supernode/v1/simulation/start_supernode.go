package simulation

import (
	"math/rand"

	"github.com/LumeraProtocol/lumera/x/supernode/v1/keeper"
	"github.com/LumeraProtocol/lumera/x/supernode/v1/types"
	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
)

const (
	TypeMsgStartSupernode = "start_supernode"
)

func SimulateMsgStartSupernode(
	ak types.AccountKeeper,
	bk types.BankKeeper,
	k keeper.Keeper,
) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		// Find a validator with a non-active supernode
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
			if !exists {
				continue
			}

			// Skip if supernode is already active
			if len(supernode.States) > 0 && supernode.States[len(supernode.States)-1].State == types.SuperNodeStateActive {
				continue
			}

			validatorAddress = validator.GetOperator()
			found = true
			break
		}

		// If we couldn't find an eligible supernode, skip this operation
		if !found {
			return simtypes.NoOpMsg(types.ModuleName, TypeMsgStartSupernode, "no eligible supernode found"), nil, nil
		}

		msg := &types.MsgStartSupernode{
			Creator:          simAccount.Address.String(),
			ValidatorAddress: validatorAddress,
		}

		// Execute the message
		msgServer := keeper.NewMsgServerImpl(k)
		_, err := msgServer.StartSupernode(ctx, msg)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, TypeMsgStartSupernode, err.Error()), nil, err
		}

		return simtypes.NewOperationMsg(msg, true, "success"), nil, nil
	}
}
