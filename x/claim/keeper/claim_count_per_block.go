package keeper

import (
	"fmt"

	"github.com/LumeraProtocol/lumera/x/claim/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// GetBlockClaimCount gets the claim count for the current block
func (k Keeper) GetBlockClaimCount(ctx sdk.Context) (uint64, error) {
	store := k.tstoreService.OpenTransientStore(ctx)
	bz, err := store.Get([]byte(types.BlockClaimsKey))
	if err != nil {
		return 0, fmt.Errorf("failed to get block claim count: %w", err)
	}
	if bz == nil {
		return 0, nil
	}
	return sdk.BigEndianToUint64(bz), nil
}

// IncrementBlockClaimCount increments the claim count for the current block
func (k Keeper) IncrementBlockClaimCount(ctx sdk.Context) error {
	store := k.tstoreService.OpenTransientStore(ctx)
	count, err := k.GetBlockClaimCount(ctx)
	if err != nil {
		return fmt.Errorf("unable to read block claim count: %w", err)
	}
	count++
	if err := store.Set([]byte(types.BlockClaimsKey), sdk.Uint64ToBigEndian(count)); err != nil {
		return fmt.Errorf("failed to set block claim count: %w", err)
	}
	return nil
}
