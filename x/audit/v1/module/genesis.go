package audit

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/LumeraProtocol/lumera/x/audit/v1/keeper"
	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
)

func InitGenesis(ctx sdk.Context, k keeper.Keeper, genState types.GenesisState) {
	if err := genState.Validate(); err != nil {
		panic(fmt.Sprintf("failed to validate genesis state: %s", err))
	}

	if err := k.InitGenesis(ctx, genState); err != nil {
		panic(err)
	}
}

func ExportGenesis(ctx sdk.Context, k keeper.Keeper) *types.GenesisState {
	genesis, err := k.ExportGenesis(ctx)
	if err != nil {
		panic(err)
	}
	return genesis
}
