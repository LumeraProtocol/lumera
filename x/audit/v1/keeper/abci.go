package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (k Keeper) BeginBlocker(ctx context.Context) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params := k.GetParams(ctx).WithDefaults()

	ws, err := k.getCurrentWindowState(sdkCtx, params)
	if err != nil {
		return err
	}
	currentWindowID := ws.WindowID
	windowStart := ws.StartHeight

	// Only create the snapshot exactly at the window start height.
	if sdkCtx.BlockHeight() != windowStart {
		return nil
	}

	return k.CreateWindowSnapshotIfNeeded(sdkCtx, currentWindowID, params)
}

func (k Keeper) EndBlocker(ctx context.Context) error {
	// Windowing/snapshotting only: no EndBlock side effects here.
	return nil
}
