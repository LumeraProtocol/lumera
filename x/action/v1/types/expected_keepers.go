package types

//go:generate mockgen -copyright_file=../../../../testutil/mock_header.txt -destination=../mocks/expected_keepers_mock.go -package=actionmocks -source=expected_keepers.go

import (
	"context"

	"cosmossdk.io/core/address"
	audittypes "github.com/LumeraProtocol/lumera/x/audit/v1/types"
	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
)

// AuthKeeper defines the expected interface for the Auth module.
type AuthKeeper interface {
	AddressCodec() address.Codec
	GetAccount(context.Context, sdk.AccAddress) sdk.AccountI // only used for simulation
	GetModuleAccount(ctx context.Context, moduleName string) sdk.ModuleAccountI
	SetModuleAccount(ctx context.Context, macc sdk.ModuleAccountI)

	// for sim tests
	SetAccount(context.Context, sdk.AccountI)
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

	Validator(context.Context, sdk.ValAddress) (stakingtypes.ValidatorI, error) // get a particular validator by operator address
}

type SupernodeKeeper interface {
	IsSuperNodeActive(ctx sdk.Context, valAddr sdk.ValAddress) bool
	QuerySuperNode(ctx sdk.Context, valOperAddr sdk.ValAddress) (sn sntypes.SuperNode, exists bool)
	GetSuperNodeByAccount(ctx sdk.Context, supernodeAccount string) (sntypes.SuperNode, bool, error)

	SetSuperNode(ctx sdk.Context, supernode sntypes.SuperNode) error
}

type SupernodeQueryServer interface {
	GetTopSuperNodesForBlock(ctx context.Context, req *sntypes.QueryGetTopSuperNodesForBlockRequest) (*sntypes.QueryGetTopSuperNodesForBlockResponse, error)
}

type DistributionKeeper interface {
	FundCommunityPool(ctx context.Context, amount sdk.Coins, sender sdk.AccAddress) error
}

type AuditKeeper interface {
	CreateEvidence(
		ctx context.Context,
		reporterAddress string,
		subjectAddress string,
		actionID string,
		evidenceType audittypes.EvidenceType,
		metadataJSON string,
	) (uint64, error)
}

// ParamSubspace defines the expected Subspace interface for parameters.
type ParamSubspace interface {
	Get(context.Context, []byte, interface{})
	Set(context.Context, []byte, interface{})
}

// ChannelKeeper defines the expected IBC channel keeper.
type ChannelKeeper interface {
	GetChannel(ctx context.Context, portID, channelID string) (channeltypes.Channel, bool)
	GetNextSequenceSend(ctx context.Context, portID, channelID string) (uint64, bool)
	SendPacket(
		ctx context.Context,
		sourcePort string,
		sourceChannel string,
		timeoutHeight clienttypes.Height,
		timeoutTimestamp uint64,
		data []byte,
	) (uint64, error)
	ChanCloseInit(ctx context.Context, portID, channelID string) error
}
