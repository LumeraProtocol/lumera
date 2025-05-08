package keeper

import (
	"context"
	"time"

	"github.com/LumeraProtocol/lumera/x/claim/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (k Keeper) BeginBlocker(ctx context.Context) error {
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

			// Get module account balance
			moduleAddr := k.accountKeeper.GetModuleAccount(sdkCtx, types.ModuleName).GetAddress()
			balance := k.bankKeeper.GetBalance(sdkCtx, moduleAddr, types.DefaultClaimsDenom)

			// Burn all coins if there's a balance
			if !balance.IsZero() {
				if err := k.bankKeeper.BurnCoins(sdkCtx, types.ModuleName, sdk.NewCoins(balance)); err != nil {
					k.Logger().Error("failed to burn unclaimed tokens", "error", err)
					return err
				}

				// Emit event for burning unclaimed tokens
				sdkCtx.EventManager().EmitEvent(
					sdk.NewEvent(
						types.EventTypeBurnUnclaimedTokens,
						sdk.NewAttribute(sdk.AttributeKeyAmount, balance.String()),
						sdk.NewAttribute(types.AttributeKeyBurnTime, sdkCtx.BlockTime().String()),
					),
				)

				k.Logger().Info("burned unclaimed tokens", "amount", balance.String())
			}

			params.EnableClaims = false
			if err := k.SetParams(ctx, params); err != nil {
				return err
			}
		}
	}

	return nil
}
