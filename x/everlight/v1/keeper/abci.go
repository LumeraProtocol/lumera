package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"

	lcfg "github.com/LumeraProtocol/lumera/config"
	"github.com/LumeraProtocol/lumera/x/everlight/v1/types"
)

// BeginBlocker contains logic that runs at the beginning of each block.
// It skims a portion of the fee collector balance (gas fees + minted inflation)
// and routes it to the Everlight module account before x/distribution processes it.
func (k Keeper) BeginBlocker(goCtx context.Context) error {
	ctx := sdk.UnwrapSDKContext(goCtx)
	params := k.GetParams(ctx)
	if params.ValidatorRewardShareBps == 0 {
		return nil
	}

	// Read fee collector balance (contains gas fees + minted inflation)
	feeCollectorAddr := k.accountKeeper.GetModuleAddress(authtypes.FeeCollectorName)
	balance := k.bankKeeper.GetBalance(ctx, feeCollectorAddr, lcfg.ChainDenom)
	if balance.IsZero() {
		return nil
	}

	// Calculate Everlight share
	shareAmount := balance.Amount.MulRaw(int64(params.ValidatorRewardShareBps)).QuoRaw(10000)
	if !shareAmount.IsPositive() {
		return nil
	}

	shareCoin := sdk.NewCoin(lcfg.ChainDenom, shareAmount)
	// Transfer from fee collector to everlight module account
	err := k.bankKeeper.SendCoinsFromModuleToModule(ctx, authtypes.FeeCollectorName, types.ModuleName, sdk.NewCoins(shareCoin))
	if err != nil {
		k.Logger().Error("failed to route block reward share to everlight", "amount", shareCoin, "err", err)
		return nil // Don't halt the chain on fee routing failure
	}

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
