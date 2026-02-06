package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (k Keeper) BeginBlocker(ctx context.Context) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	// BeginBlocker is responsible for epoch-start duties only. It derives the current epoch
	// from immutable epoch-cadence params (length + zero height), and persists an EpochAnchor
	// exactly at epoch start. The anchor freezes the epoch seed and eligible sets used by
	// deterministic off-chain selection.
	params := k.GetParams(ctx).WithDefaults()

	epoch, err := deriveEpochAtHeight(sdkCtx.BlockHeight(), params)
	if err != nil {
		return err
	}

	// Only create the anchor exactly at the epoch start height.
	if sdkCtx.BlockHeight() != epoch.StartHeight {
		return nil
	}

	if err := k.CreateEpochAnchorIfNeeded(sdkCtx, epoch.EpochID, epoch.StartHeight, epoch.EndHeight, params); err != nil {
		return err
	}
	return nil
}

func (k Keeper) EndBlocker(ctx context.Context) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// EndBlocker does two things:
	// 1) epoch-end audit enforcement + pruning (only at epoch end height).

	params := k.GetParams(ctx).WithDefaults()

	epoch, err := deriveEpochAtHeight(sdkCtx.BlockHeight(), params)
	if err != nil {
		return err
	}

	// Only enforce and prune exactly at the epoch end height.
	if sdkCtx.BlockHeight() != epoch.EndHeight {
		return nil
	}

	if err := k.EnforceEpochEnd(sdkCtx, epoch.EpochID, params); err != nil {
		return err
	}

	return k.PruneOldEpochs(sdkCtx, epoch.EpochID, params)
}
