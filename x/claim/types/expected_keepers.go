package types

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// AccountKeeper defines the expected interface for the Account module.
type AccountKeeper interface {
	NewAccount(context.Context, sdk.AccountI) sdk.AccountI
	GetAccount(context.Context, sdk.AccAddress) sdk.AccountI
	SetAccount(context.Context, sdk.AccountI)
	GetModuleAccount(ctx context.Context, moduleName string) sdk.ModuleAccountI
	SetModuleAccount(ctx context.Context, macc sdk.ModuleAccountI)
	NewAccountWithAddress(context.Context, sdk.AccAddress) sdk.AccountI
}

// BankKeeper defines expected bank keeper methods
type BankKeeper interface {
	MintCoins(ctx context.Context, moduleName string, amt sdk.Coins) error
	BurnCoins(ctx context.Context, moduleName string, amt sdk.Coins) error
	SendCoinsFromModuleToAccount(ctx context.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error
}

// ParamSubspace defines the expected Subspace interface for parameters.
type ParamSubspace interface {
	Get(context.Context, []byte, interface{})
	Set(context.Context, []byte, interface{})
}
