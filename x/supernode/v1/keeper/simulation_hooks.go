package keeper

import sdk "github.com/cosmos/cosmos-sdk/types"

// RunEverlightDistributionForSimulation exposes one distribution step for simulation operations.
func (k Keeper) RunEverlightDistributionForSimulation(ctx sdk.Context) error {
	return k.distributePool(ctx)
}
