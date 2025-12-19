package simulation

import (
	"fmt"
	"math/rand"

	errorsmod "cosmossdk.io/errors"
	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"

	"github.com/LumeraProtocol/lumera/x/supernode/v1/keeper"
	"github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

const (
	TypeMsgUpdateSupernodeInvalidAccount = "update_supernode_invalid_account"
)

func SimulateMsgUpdateSupernodeInvalidAccount(
	ak types.AccountKeeper,
	bk types.BankKeeper,
	k keeper.Keeper,
) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		// Find a validator with an existing supernode.
		var simAccount simtypes.Account
		var validatorAddress string
		var targetSupernodeAccount string
		var found bool

		stakingkeeper := k.GetStakingKeeper()
		for i := 0; i < 10; i++ {
			simAccount, _ = simtypes.RandomAcc(r, accs)
			valAddr := sdk.ValAddress(simAccount.Address)

			validator, err := stakingkeeper.Validator(ctx, valAddr)
			if err != nil {
				continue
			}

			supernode, exists := k.QuerySuperNode(ctx, valAddr)
			if !exists || supernode.SupernodeAccount == "" {
				continue
			}

			validatorAddress = validator.GetOperator()
			targetSupernodeAccount = supernode.SupernodeAccount
			found = true
			break
		}

		if !found {
			return simtypes.NoOpMsg(types.ModuleName, TypeMsgUpdateSupernodeInvalidAccount, "no eligible supernode found"), nil, nil
		}

		// Find another supernode account that belongs to a different validator.
		var conflictAccount string
		for i := 0; i < len(accs); i++ {
			otherAcc, _ := simtypes.RandomAcc(r, accs)
			if otherAcc.Address.Equals(simAccount.Address) {
				continue
			}

			otherValAddr := sdk.ValAddress(otherAcc.Address)
			otherSupernode, exists := k.QuerySuperNode(ctx, otherValAddr)
			if !exists || otherSupernode.SupernodeAccount == "" {
				continue
			}

			if otherSupernode.SupernodeAccount != targetSupernodeAccount {
				conflictAccount = otherSupernode.SupernodeAccount
				break
			}
		}

		if conflictAccount == "" {
			return simtypes.NoOpMsg(types.ModuleName, TypeMsgUpdateSupernodeInvalidAccount, "no conflicting supernode account found"), nil, nil
		}

		msg := &types.MsgUpdateSupernode{
			Creator:          simAccount.Address.String(),
			ValidatorAddress: validatorAddress,
			SupernodeAccount: conflictAccount,
		}

		msgServer := keeper.NewMsgServerImpl(k)
		_, err := msgServer.UpdateSupernode(ctx, msg)
		if err == nil {
			return simtypes.NoOpMsg(types.ModuleName, TypeMsgUpdateSupernodeInvalidAccount, "expected invalid supernode account update to fail"), nil, fmt.Errorf("expected invalid supernode account update to fail")
		}
		if errorsmod.IsOf(err, sdkerrors.ErrInvalidRequest) {
			return simtypes.NewOperationMsg(msg, true, "expected invalid supernode account update rejected"), nil, nil
		}

		return simtypes.NoOpMsg(types.ModuleName, TypeMsgUpdateSupernodeInvalidAccount, err.Error()), nil, err
	}
}
