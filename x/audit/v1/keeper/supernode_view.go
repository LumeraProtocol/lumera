package keeper

import (
	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (k Keeper) GetAllSuperNodes(ctx sdk.Context) ([]sntypes.SuperNode, error) {
	return k.supernodeKeeper.GetAllSuperNodes(ctx)
}
