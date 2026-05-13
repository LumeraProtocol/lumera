package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// GetCurrentEpochInfo derives current epoch data at the current block height.
func (k Keeper) GetCurrentEpochInfo(ctx sdk.Context) (epochID uint64, startHeight int64, endHeight int64, err error) {
	params := k.GetParams(ctx).WithDefaults()
	epoch, err := deriveEpochAtHeight(ctx.BlockHeight(), params)
	if err != nil {
		return 0, 0, 0, err
	}
	return epoch.EpochID, epoch.StartHeight, epoch.EndHeight, nil
}
