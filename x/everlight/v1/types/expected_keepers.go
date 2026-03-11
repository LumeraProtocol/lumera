package types

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

// BankKeeper defines the expected bank keeper interface.
type BankKeeper interface {
	GetBalance(ctx context.Context, addr sdk.AccAddress, denom string) sdk.Coin
	GetAllBalances(ctx context.Context, addr sdk.AccAddress) sdk.Coins
	SendCoins(ctx context.Context, fromAddr, toAddr sdk.AccAddress, amt sdk.Coins) error
	SendCoinsFromModuleToAccount(ctx context.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error
	SendCoinsFromAccountToModule(ctx context.Context, senderAddr sdk.AccAddress, recipientModule string, amt sdk.Coins) error
	SendCoinsFromModuleToModule(ctx context.Context, senderModule, recipientModule string, amt sdk.Coins) error
}

// AccountKeeper defines the expected account keeper interface.
type AccountKeeper interface {
	GetModuleAddress(moduleName string) sdk.AccAddress
	GetModuleAccount(ctx context.Context, moduleName string) sdk.ModuleAccountI
}

// SupernodeKeeper defines the expected interface for querying supernode data.
type SupernodeKeeper interface {
	GetAllSuperNodes(ctx sdk.Context, stateFilters ...sntypes.SuperNodeState) ([]sntypes.SuperNode, error)
	GetMetricsState(ctx sdk.Context, valAddr sdk.ValAddress) (sntypes.SupernodeMetricsState, bool)
}
