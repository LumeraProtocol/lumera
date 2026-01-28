package simulation

import (
	"fmt"
	"math/rand"

	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"

	"github.com/LumeraProtocol/lumera/x/supernode/v1/keeper"
	"github.com/LumeraProtocol/lumera/x/supernode/v1/types"
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

		stakingkeeper := k.GetStakingKeeper()
		// Try up to 10 times to find an eligible validator
		for i := 0; i < 10; i++ {
			simAccount, _ = simtypes.RandomAcc(r, accs)
			valAddr := sdk.ValAddress(simAccount.Address)

			validator, err := stakingkeeper.Validator(ctx, valAddr)
			if err != nil {
				continue
			}

			if validator.IsJailed() {
				continue
			}

			// Ensure the account isn't already associated with a different validator.
			if snByAccount, foundByAccount, err := k.GetSuperNodeByAccount(ctx, simAccount.Address.String()); err == nil && foundByAccount {
				if snByAccount.ValidatorAddress != valAddr.String() {
					continue
				}
			} else if err != nil {
				continue
			}

			// Check if supernode already exists and is not disabled
			supernode, superNodeExists := k.QuerySuperNode(ctx, valAddr)
			if superNodeExists {
				// Allow re-registration if the supernode is disabled
				if len(supernode.States) > 0 && supernode.States[len(supernode.States)-1].State == types.SuperNodeStateDisabled {
					found = true
					break
				}
				// Skip if supernode exists and is not disabled
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

		p2pPort := fmt.Sprintf("%d", r.Intn(65535))

		msg := &types.MsgRegisterSupernode{
			Creator:          simAccount.Address.String(),
			ValidatorAddress: validatorAddress,
			SupernodeAccount: simAccount.Address.String(),
			IpAddress:        ipAddress,
			P2PPort:          p2pPort,
		}

		// Execute the message
		msgServer := keeper.NewMsgServerImpl(k)
		_, err := msgServer.RegisterSupernode(ctx, msg)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, TypeMsgRegisterSupernode, err.Error()), nil, err
		}

		return simtypes.NewOperationMsg(msg, true, "success"), nil, nil
	}
}
