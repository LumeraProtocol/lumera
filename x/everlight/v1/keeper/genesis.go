package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/LumeraProtocol/lumera/x/everlight/v1/types"
)

// InitGenesis initializes the module's state from a provided genesis state.
func (k Keeper) InitGenesis(ctx sdk.Context, gs types.GenesisState) {
	if err := k.SetParams(ctx, gs.Params); err != nil {
		panic(err)
	}
	if gs.LastDistributionHeight > 0 {
		k.SetLastDistributionHeight(ctx, gs.LastDistributionHeight)
	}
}

// ExportGenesis returns the module's exported genesis state.
func (k Keeper) ExportGenesis(ctx sdk.Context) *types.GenesisState {
	return &types.GenesisState{
		Params:                 k.GetParams(ctx),
		LastDistributionHeight: k.GetLastDistributionHeight(ctx),
	}
}
