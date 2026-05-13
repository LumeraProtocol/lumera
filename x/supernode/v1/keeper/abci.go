package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// BeginBlocker contains logic that runs at the beginning of each block.
// Currently the supernode module has no begin-block behavior.
func (k Keeper) BeginBlocker(ctx context.Context) error {
	return nil
}

// EndBlocker contains logic that runs at the end of each block.
// It delegates to HandleMetricsStaleness, which may transition ACTIVE
// supernodes into POSTPONED when they fail to report metrics on time.
func (k Keeper) EndBlocker(ctx context.Context) error {
	// Metrics staleness enforcement is handled by the audit module.
	return k.distributeSuperNodeRewards(ctx)
}

func (k Keeper) distributeSuperNodeRewards(goCtx context.Context) error {
	ctx := sdk.UnwrapSDKContext(goCtx)

	params := k.GetParams(ctx)
	if params.RewardDistribution == nil || params.RewardDistribution.PaymentPeriodBlocks == 0 {
		return nil
	}

	currentHeight := ctx.BlockHeight()
	lastDistHeight := k.GetLastDistributionHeight(ctx)

	// Check if enough blocks have elapsed since the last distribution.
	if lastDistHeight > 0 && uint64(currentHeight-lastDistHeight) < params.RewardDistribution.PaymentPeriodBlocks {
		return nil
	}

	// If lastDistHeight is 0 (first run) and current height is 0, skip.
	// This avoids distributing on genesis block.
	if lastDistHeight == 0 && currentHeight == 0 {
		return nil
	}

	return k.distributePool(ctx)
}
