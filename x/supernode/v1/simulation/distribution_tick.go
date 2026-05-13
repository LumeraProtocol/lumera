package simulation

import (
	"math/rand"

	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"

	"github.com/LumeraProtocol/lumera/x/supernode/v1/keeper"
	"github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

const TypeEverlightDistributionTick = "everlight_distribution_tick"

func SimulateEverlightDistributionTick(k keeper.Keeper) simtypes.Operation {
	return func(_ *rand.Rand, _ *baseapp.BaseApp, ctx sdk.Context, _ []simtypes.Account, _ string) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		if err := k.RunEverlightDistributionForSimulation(ctx); err != nil {
			return simtypes.NoOpMsg(types.ModuleName, TypeEverlightDistributionTick, err.Error()), nil, nil
		}
		return simtypes.NoOpMsg(types.ModuleName, TypeEverlightDistributionTick, "success"), nil, nil
	}
}
