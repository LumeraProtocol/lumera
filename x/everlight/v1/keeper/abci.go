package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// BeginBlocker contains logic that runs at the beginning of each block.
func (k Keeper) BeginBlocker(_ context.Context) error {
	return nil
}

// EndBlocker contains logic that runs at the end of each block.
// It checks whether payment_period_blocks have elapsed since the last
// distribution and, if so, triggers the distribution of the pool balance
// proportionally to eligible supernodes based on their smoothed cascade bytes.
func (k Keeper) EndBlocker(goCtx context.Context) error {
	ctx := sdk.UnwrapSDKContext(goCtx)

	params := k.GetParams(ctx)
	if params.PaymentPeriodBlocks == 0 {
		return nil
	}

	currentHeight := ctx.BlockHeight()
	lastDistHeight := k.GetLastDistributionHeight(ctx)

	// Check if enough blocks have elapsed since the last distribution.
	if lastDistHeight > 0 && uint64(currentHeight-lastDistHeight) < params.PaymentPeriodBlocks {
		return nil
	}

	// If lastDistHeight is 0 (first run) and current height is 0, skip.
	// This avoids distributing on genesis block.
	if lastDistHeight == 0 && currentHeight == 0 {
		return nil
	}

	return k.distributePool(ctx)
}
