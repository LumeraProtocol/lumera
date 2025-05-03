package action

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"

	"github.com/LumeraProtocol/lumera/x/action/keeper"
	"github.com/LumeraProtocol/lumera/x/action/types"
)

// InitGenesis initializes the module's state from a provided genesis state.
func InitGenesis(ctx sdk.Context, k keeper.Keeper, genState types.GenesisState) {
	err := genState.Validate()
	if err != nil {
		panic(fmt.Sprintf("failed to validate genesis state: %s", err))
	}

	// this line is used by starport scaffolding # genesis/module/init
	if err := k.SetParams(ctx, genState.Params); err != nil {
		panic(err)
	}

	if err := initModuleAccount(ctx, k); err != nil {
		panic(fmt.Sprintf("failed to initialize module account: %s", err))
	}
}

func initModuleAccount(ctx context.Context, k keeper.Keeper) error {
	accountKeeper := k.GetAccountKeeper()
	acc := accountKeeper.GetModuleAccount(ctx, types.ModuleName)
	if acc != nil {
		return nil // Module account already exists
	}

	moduleAcc := authtypes.NewEmptyModuleAccount(
		types.ModuleName,
		authtypes.Minter,
		authtypes.Burner,
	)

	accountKeeper.SetModuleAccount(ctx, moduleAcc)
	return nil
}

// ExportGenesis returns the module's exported genesis.
func ExportGenesis(ctx sdk.Context, k keeper.Keeper) *types.GenesisState {
	genesis := types.DefaultGenesis()
	genesis.Params = k.GetParams(ctx)

	// this line is used by starport scaffolding # genesis/module/export

	return genesis
}
