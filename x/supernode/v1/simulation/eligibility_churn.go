package simulation

import (
	"math/rand"

	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"

	"github.com/LumeraProtocol/lumera/x/supernode/v1/keeper"
	"github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

const TypeEverlightEligibilityChurn = "everlight_eligibility_churn"

func SimulateEverlightEligibilityChurn(k keeper.Keeper) simtypes.Operation {
	return func(r *rand.Rand, _ *baseapp.BaseApp, ctx sdk.Context, _ []simtypes.Account, _ string) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		sns, err := k.GetAllSuperNodes(ctx)
		if err != nil || len(sns) == 0 {
			return simtypes.NoOpMsg(types.ModuleName, TypeEverlightEligibilityChurn, "no supernodes"), nil, nil
		}
		sn := sns[r.Intn(len(sns))]
		if len(sn.States) == 0 {
			return simtypes.NoOpMsg(types.ModuleName, TypeEverlightEligibilityChurn, "no states"), nil, nil
		}

		valAddr, addrErr := sdk.ValAddressFromBech32(sn.ValidatorAddress)
		if addrErr != nil {
			return simtypes.NoOpMsg(types.ModuleName, TypeEverlightEligibilityChurn, addrErr.Error()), nil, nil
		}
		choice := r.Intn(3)
		switch choice {
		case 0:
			err = k.SetSuperNodePostponed(ctx, valAddr, "simulation churn")
		case 1:
			err = k.SetSuperNodeActive(ctx, valAddr, "simulation churn")
		default:
			sn.States = append(sn.States, &types.SuperNodeStateRecord{State: types.SuperNodeStateStorageFull, Height: ctx.BlockHeight()})
			err = k.SetSuperNode(ctx, sn)
		}
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, TypeEverlightEligibilityChurn, err.Error()), nil, nil
		}
		return simtypes.NoOpMsg(types.ModuleName, TypeEverlightEligibilityChurn, "success"), nil, nil
	}
}
