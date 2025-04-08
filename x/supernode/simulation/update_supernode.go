package simulation

import (
	"fmt"
	"math/rand"

	"github.com/LumeraProtocol/lumera/x/supernode/keeper"
	"github.com/LumeraProtocol/lumera/x/supernode/types"
	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
)

const (
	TypeMsgUpdateSupernode = "update_supernode"
)

func SimulateMsgUpdateSupernode(
	ak types.AccountKeeper,
	bk types.BankKeeper,
	k keeper.Keeper,
) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		// Find a validator with an existing supernode
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
			_, exists := k.QuerySuperNode(ctx, valAddr)
			if !exists {
				continue
			}

			validatorAddress = validator.GetOperator()
			found = true
			break
		}

		// If we couldn't find an eligible supernode, skip this operation
		if !found {
			return simtypes.NoOpMsg(types.ModuleName, TypeMsgUpdateSupernode, "no eligible supernode found"), nil, nil
		}

		// Generate random updates
		// Only update some fields randomly to simulate partial updates
		updateIP := r.Float64() < 0.5
		updateAccount := r.Float64() < 0.3
		updateVersion := r.Float64() < 0.7

		var ipAddress, supernodeAccount, version string

		if updateIP {
			ipAddress = fmt.Sprintf("%d.%d.%d.%d",
				r.Intn(256), r.Intn(256), r.Intn(256), r.Intn(256))
		}

		if updateAccount {
			newAcc, _ := simtypes.RandomAcc(r, accs)
			supernodeAccount = newAcc.Address.String()
		}

		if updateVersion {
			version = fmt.Sprintf("v%d.%d.%d", r.Intn(10), r.Intn(10), r.Intn(10))
		}

		msg := &types.MsgUpdateSupernode{
			Creator:          simAccount.Address.String(),
			ValidatorAddress: validatorAddress,
			IpAddress:        ipAddress,
			SupernodeAccount: supernodeAccount,
			Version:          version,
		}

		// Execute the message
		msgServer := keeper.NewMsgServerImpl(k)
		_, err := msgServer.UpdateSupernode(ctx, msg)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, TypeMsgUpdateSupernode, err.Error()), nil, err
		}

		return simtypes.NewOperationMsg(msg, true, "success"), nil, nil
	}
}
