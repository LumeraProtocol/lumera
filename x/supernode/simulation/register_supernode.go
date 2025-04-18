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
	TypeMsgRegisterSupernode = "register_supernode"
)

func SimulateMsgRegisterSupernode(
	ak types.AccountKeeper,
	bk types.BankKeeper,
	k keeper.Keeper,
) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		// Find a non-jailed validator without an existing supernode
		var simAccount simtypes.Account
		var found bool

		stakingkeepr := k.GetStakingKeeper()
		// Try up to 10 times to find an eligible validator
		for i := 0; i < 10; i++ {
			simAccount, _ = simtypes.RandomAcc(r, accs)
			valAddr := sdk.ValAddress(simAccount.Address)

			validator, err := stakingkeepr.Validator(ctx, valAddr)
			if err != nil {
				continue
			}

			if validator.IsJailed() {
				continue
			}

			// Check if supernode already exists
			_, superNodeExists := k.QuerySuperNode(ctx, valAddr)
			if superNodeExists {
				continue
			}

			found = true
			break
		}

		// If we couldn't find an eligible validator, skip this operation
		if !found {
			return simtypes.NoOpMsg(types.ModuleName, TypeMsgRegisterSupernode, "no eligible validator found"), nil, nil
		}

		valAddr := sdk.ValAddress(simAccount.Address)
		validatorAddress := valAddr.String()

		// Generate a random IP address
		ipAddress := fmt.Sprintf("%d.%d.%d.%d",
			r.Intn(256), r.Intn(256), r.Intn(256), r.Intn(256))

		// Generate a random version
		version := fmt.Sprintf("v%d.%d.%d", r.Intn(10), r.Intn(10), r.Intn(10))

		msg := &types.MsgRegisterSupernode{
			Creator:          simAccount.Address.String(),
			ValidatorAddress: validatorAddress,
			SupernodeAccount: simAccount.Address.String(),
			IpAddress:        ipAddress,
			Version:          version,
		}

		// Execute the message
		msgServer := keeper.NewMsgServerImpl(k)
		_, err := msgServer.RegisterSupernode(sdk.WrapSDKContext(ctx), msg)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, TypeMsgRegisterSupernode, err.Error()), nil, err
		}

		return simtypes.NewOperationMsg(msg, true, "success"), nil, nil
	}
}
