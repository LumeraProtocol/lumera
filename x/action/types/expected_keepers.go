package types

//go:generate mockgen -destination=../mocks/expected_keepers_mock.go -package=actionmocks -source=expected_keepers.go

import (
	"context"

	supernodetypes "github.com/LumeraProtocol/lumera/x/supernode/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// AccountKeeper defines the expected interface for the Account module.
type AccountKeeper interface {
	GetAccount(context.Context, sdk.AccAddress) sdk.AccountI // only used for simulation
	// Methods imported from account should be defined here
}

// BankKeeper defines the expected interface for the Bank module.
type BankKeeper interface {
	SpendableCoins(ctx context.Context, addr sdk.AccAddress) sdk.Coins
	GetBalance(ctx context.Context, addr sdk.AccAddress, denom string) sdk.Coin
	SendCoinsFromModuleToAccount(ctx context.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error
	SendCoinsFromAccountToModule(ctx context.Context, senderAddr sdk.AccAddress, recipientModule string, amt sdk.Coins) error
}

// StakingKeeper defines the expected staking keeper
type StakingKeeper interface {
	GetValidator(ctx context.Context, addr sdk.ValAddress) (validator stakingtypes.Validator, err error)
}

type SupernodeKeeper interface {
	GetTopSuperNodesForBlock(goCtx context.Context, req *supernodetypes.QueryGetTopSuperNodesForBlockRequest) (*supernodetypes.QueryGetTopSuperNodesForBlockResponse, error)
	IsSuperNodeActive(ctx sdk.Context, valAddr sdk.ValAddress) bool
	QuerySuperNode(ctx sdk.Context, valOperAddr sdk.ValAddress) (sn supernodetypes.SuperNode, exists bool)
}

type DistributionKeeper interface {
	FundCommunityPool(ctx context.Context, amount sdk.Coins, sender sdk.AccAddress) error
}

// ParamSubspace defines the expected Subspace interface for parameters.
type ParamSubspace interface {
	Get(context.Context, []byte, interface{})
	Set(context.Context, []byte, interface{})
}
