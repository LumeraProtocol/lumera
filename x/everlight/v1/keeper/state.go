package keeper

import (
	"encoding/binary"

	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/LumeraProtocol/lumera/x/everlight/v1/types"
)

// GetLastDistributionHeight returns the block height of the last distribution.
func (k Keeper) GetLastDistributionHeight(ctx sdk.Context) int64 {
	store := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	bz := store.Get(types.LastDistributionHeightKey)
	if bz == nil {
		return 0
	}
	return int64(binary.BigEndian.Uint64(bz))
}

// SetLastDistributionHeight stores the block height of the last distribution.
func (k Keeper) SetLastDistributionHeight(ctx sdk.Context, height int64) {
	store := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	bz := make([]byte, 8)
	binary.BigEndian.PutUint64(bz, uint64(height))
	store.Set(types.LastDistributionHeightKey, bz)
}

// GetPoolBalance returns the current balance of the everlight module account.
func (k Keeper) GetPoolBalance(ctx sdk.Context) sdk.Coins {
	moduleAddr := k.accountKeeper.GetModuleAddress(types.ModuleName)
	return k.bankKeeper.GetAllBalances(ctx, moduleAddr)
}
