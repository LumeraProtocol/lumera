package types

//go:generate mockgen -copyright_file=../../../../testutil/mock_header.txt -destination=../mocks/expected_keepers_mock.go -package=supernodemocks -source=expected_keepers.go
//go:generate mockgen -copyright_file=../../../../testutil/mock_header.txt -destination=../mocks/queryserver_mock.go -package=supernodemocks -source=query.pb.go

import (
	"context"
	"time"

	math "cosmossdk.io/math"
	sdkmath "cosmossdk.io/math"
	"cosmossdk.io/core/address"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"

	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// SupernodeKeeper defines the expected interface for the Supernode module.
// For Generating mocks only not used in depinject
type SupernodeKeeper interface {
	SetSuperNode(ctx sdk.Context, supernode SuperNode) error
	EnableSuperNode(ctx sdk.Context, valAddr sdk.ValAddress) error
	DisableSuperNode(ctx sdk.Context, valAddr sdk.ValAddress) error
	SetParams(ctx sdk.Context, params Params) error
	CheckValidatorSupernodeEligibility(ctx sdk.Context, validator stakingtypes.ValidatorI, valAddr string) error

	GetAuthority() string
	GetStakingKeeper() StakingKeeper
	GetParams(ctx sdk.Context) (params Params)
	GetAllSuperNodes(ctx sdk.Context, stateFilters ...SuperNodeState) ([]SuperNode, error)
	GetBlockHashForHeight(ctx sdk.Context, height int64) ([]byte, error)
	RankSuperNodesByDistance(blockHash []byte, supernodes []SuperNode, topN int) []SuperNode
	QuerySuperNode(ctx sdk.Context, valOperAddr sdk.ValAddress) (sn SuperNode, exists bool)
	GetSuperNodesPaginated(ctx sdk.Context, pagination *query.PageRequest, stateFilters ...SuperNodeState) ([]*SuperNode, *query.PageResponse, error)
	IsSuperNodeActive(ctx sdk.Context, valAddr sdk.ValAddress) bool
	IsEligibleAndNotJailedValidator(ctx sdk.Context, valAddr sdk.ValAddress) bool
}

// StakingKeeper defines the expected interface for the Staking module.
type StakingKeeper interface {
	ConsensusAddressCodec() address.Codec
	Validator(context.Context, sdk.ValAddress) (stakingtypes.ValidatorI, error)            // get a particular validator by operator address
	ValidatorByConsAddr(context.Context, sdk.ConsAddress) (stakingtypes.ValidatorI, error) // get a particular validator by consensus address
	Delegation(ctx context.Context, delAddr sdk.AccAddress, valAddr sdk.ValAddress) (stakingtypes.DelegationI, error)
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

// StakingHooks event hooks for staking validator object (noalias)
type StakingHooks interface {
	AfterValidatorCreated(ctx context.Context, valAddr sdk.ValAddress) error                           // Must be called when a validator is created
	BeforeValidatorModified(ctx context.Context, valAddr sdk.ValAddress) error                         // Must be called when a validator's state changes
	AfterValidatorRemoved(ctx context.Context, consAddr sdk.ConsAddress, valAddr sdk.ValAddress) error // Must be called when a validator is deleted

	AfterValidatorBonded(ctx context.Context, consAddr sdk.ConsAddress, valAddr sdk.ValAddress) error         // Must be called when a validator is bonded
	AfterValidatorBeginUnbonding(ctx context.Context, consAddr sdk.ConsAddress, valAddr sdk.ValAddress) error // Must be called when a validator begins unbonding

	BeforeDelegationCreated(ctx context.Context, delAddr sdk.AccAddress, valAddr sdk.ValAddress) error        // Must be called when a delegation is created
	BeforeDelegationSharesModified(ctx context.Context, delAddr sdk.AccAddress, valAddr sdk.ValAddress) error // Must be called when a delegation's shares are modified
	BeforeDelegationRemoved(ctx context.Context, delAddr sdk.AccAddress, valAddr sdk.ValAddress) error        // Must be called when a delegation is removed
	AfterDelegationModified(ctx context.Context, delAddr sdk.AccAddress, valAddr sdk.ValAddress) error
	BeforeValidatorSlashed(ctx context.Context, valAddr sdk.ValAddress, fraction math.LegacyDec) error
}
