package keeper

import (
	"context"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/pastelnetwork/pastel/x/claim/types"
)

func (k Keeper) BeginBlocker(ctx context.Context) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	k.ResetBlockClaimCount(sdkCtx)
	return nil
}

func (k Keeper) EndBlocker(ctx context.Context) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	params := k.GetParams(ctx)
	if params.EnableClaims {
		if sdkCtx.BlockTime().After(time.Unix(params.ClaimEndTime, 1)) {
			// Emit claim period end event
			sdkCtx.EventManager().EmitEvent(
				sdk.NewEvent(
					types.EventTypeClaimPeriodEnd,
					sdk.NewAttribute(types.AttributeKeyEndTime, sdkCtx.BlockTime().String()),
				),
			)

			params.EnableClaims = false
			if err := k.SetParams(ctx, params); err != nil {
				return err
			}
		}
	}

	return nil
}
