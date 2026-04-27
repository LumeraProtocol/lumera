package supernode

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/LumeraProtocol/lumera/x/supernode/v1/keeper"
	"github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

// InitGenesis initializes the module's state from a provided genesis state.
func InitGenesis(ctx sdk.Context, k keeper.Keeper, genState types.GenesisState) {
	// this line is used by starport scaffolding # genesis/module/init
	if err := k.SetParams(ctx, genState.Params); err != nil {
		panic(err)
	}
	if genState.LastDistributionHeight < 0 {
		panic("invalid supernode genesis: last_distribution_height must be >= 0")
	}
	k.SetLastDistributionHeight(ctx, genState.LastDistributionHeight)

	// Ensure the supernode module account is persisted in the account store.
	// Without this, the address exists in maccPerms but no ModuleAccount object
	// is stored — the first bank send to the address would create a BaseAccount
	// instead, permanently corrupting the module account.
	k.EnsureModuleAccount(ctx)
}

// ExportGenesis returns the module's exported genesis.
func ExportGenesis(ctx sdk.Context, k keeper.Keeper) *types.GenesisState {
	genesis := types.DefaultGenesis()
	genesis.Params = k.GetParams(ctx)
	genesis.LastDistributionHeight = k.GetLastDistributionHeight(ctx)

	// this line is used by starport scaffolding # genesis/module/export

	return genesis
}
