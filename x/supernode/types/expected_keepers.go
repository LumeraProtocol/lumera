package types

import (
	"context"
	sdkmath "cosmossdk.io/math"
	"time"

	"cosmossdk.io/core/address"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// StakingKeeper defines the expected interface for the Staking module.
type StakingKeeper interface {
	ConsensusAddressCodec() address.Codec
	ValidatorByConsAddr(context.Context, sdk.ConsAddress) (stakingtypes.ValidatorI, error)
}

// SlashingKeeper defines the expected interface for the Slashing module.
type SlashingKeeper interface {
	IsTombstoned(context.Context, sdk.ConsAddress) bool
	Jail(context.Context, sdk.ConsAddress) error
	JailUntil(context.Context, sdk.ConsAddress, time.Time) error
	Slash(ctx context.Context, consAddr sdk.ConsAddress, fraction sdkmath.LegacyDec, power, distributionHeight int64) error
}

// AccountKeeper defines the expected interface for the Account module.
type AccountKeeper interface {
	GetAccount(context.Context, sdk.AccAddress) sdk.AccountI // only used for simulation
}

// BankKeeper defines the expected interface for the Bank module.
type BankKeeper interface {
	SpendableCoins(context.Context, sdk.AccAddress) sdk.Coins
	GetBalance(ctx context.Context, addr sdk.AccAddress, denom string) sdk.Coin
}

// ParamSubspace defines the expected Subspace interface for parameters.
type ParamSubspace interface {
	Get(context.Context, []byte, interface{})
	Set(context.Context, []byte, interface{})
}
