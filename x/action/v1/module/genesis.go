package action

import (
	"context"
	"fmt"
	"github.com/LumeraProtocol/lumera/x/action/v1/keeper"
	types2 "github.com/LumeraProtocol/lumera/x/action/v1/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
)

// InitGenesis initializes the module's state from a provided genesis state.
func InitGenesis(ctx sdk.Context, k keeper.Keeper, genState types2.GenesisState) {
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
	acc := accountKeeper.GetModuleAccount(ctx, types2.ModuleName)
	if acc != nil {
		return nil // Module account already exists
	}

	moduleAcc := authtypes.NewEmptyModuleAccount(
		types2.ModuleName,
		authtypes.Minter,
		authtypes.Burner,
	)

	accountKeeper.SetModuleAccount(ctx, moduleAcc)
	return nil
}

// ExportGenesis returns the module's exported genesis.
func ExportGenesis(ctx sdk.Context, k keeper.Keeper) *types2.GenesisState {
	genesis := types2.DefaultGenesis()
	genesis.Params = k.GetParams(ctx)

	// this line is used by starport scaffolding # genesis/module/export

	return genesis
}
