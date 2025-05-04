package types

import (
	"context"

    stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
    sdk "github.com/cosmos/cosmos-sdk/types"
    "cosmossdk.io/core/address"
)

// StakingKeeper defines the expected staking keeper (only the methods needed).
type StakingKeeper interface {
    GetHistoricalInfo(ctx context.Context, height int64) (stakingtypes.HistoricalInfo, error)
    GetValidatorByConsAddr(ctx context.Context, consAddr sdk.ConsAddress) (stakingtypes.Validator, error)
    ValidatorAddressCodec() address.Codec
	BondDenom(ctx context.Context) (string, error)
}