package keeper

import (
    "context"

    addresscodec "cosmossdk.io/core/address"
    stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
    stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
    lumevmtypes "github.com/LumeraProtocol/lumera/x/evm/types"
)

// Keeper wraps the Cosmos SDK staking keeper and adds custom logic.
type Keeper struct {
    stakingKeeper lumevmtypes.StakingKeeper
}

// NewKeeper creates a new wrapped staking Keeper instance.
func NewKeeper(sdkKeeper *stakingkeeper.Keeper) *Keeper {
    return &Keeper{
        stakingKeeper: sdkKeeper,
    }
}

// GetHistoricalInfo implements the StakingKeeper interface for the EVM module.
func (k Keeper) GetHistoricalInfo(ctx context.Context, height int64) (stakingtypes.HistoricalInfo, error) {
    sdkCtx := sdk.UnwrapSDKContext(ctx)
    return k.stakingKeeper.GetHistoricalInfo(sdkCtx, height)
}

// GetValidatorByConsAddr implements the StakingKeeper interface for the EVM module.
func (k Keeper) GetValidatorByConsAddr(ctx context.Context, consAddr sdk.ConsAddress) (stakingtypes.Validator, error) {
    sdkCtx := sdk.UnwrapSDKContext(ctx)
    return k.stakingKeeper.GetValidatorByConsAddr(sdkCtx, consAddr)
}

// ValidatorAddressCodec implements the StakingKeeper interface for the EVM module.
func (k Keeper) ValidatorAddressCodec() addresscodec.Codec {
    return k.stakingKeeper.ValidatorAddressCodec()
}

// BondDenom implements the StakingKeeper interface for the EVM module.
func (k Keeper) BondDenom(ctx context.Context) (string, error) {
	return k.stakingKeeper.BondDenom(ctx)
}