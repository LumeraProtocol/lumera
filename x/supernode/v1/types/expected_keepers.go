package types

//go:generate mockgen -copyright_file=../../../../testutil/mock_header.txt -destination=../mocks/expected_keepers_mock.go -package=supernodemocks -source=expected_keepers.go
//go:generate mockgen -copyright_file=../../../../testutil/mock_header.txt -destination=../mocks/queryserver_mock.go -package=supernodemocks -source=query.pb.go

import (
	"context"
	"time"

	"cosmossdk.io/core/address"
	"cosmossdk.io/log"
	sdkmath "cosmossdk.io/math"
	audittypes "github.com/LumeraProtocol/lumera/x/audit/v1/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"

	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// SupernodeKeeper defines the expected interface for the Supernode module.
// For Generating mocks only not used in depinject
type SupernodeKeeper interface {
	SetSuperNode(ctx sdk.Context, supernode SuperNode) error
	SetParams(ctx sdk.Context, params Params) error
	CheckValidatorSupernodeEligibility(ctx sdk.Context, validator stakingtypes.ValidatorI, valAddr string, supernodeAccount string) error
	SetSuperNodeStopped(ctx sdk.Context, valAddr sdk.ValAddress, reason string) error
	SetSuperNodeActive(ctx sdk.Context, valAddr sdk.ValAddress, reason string) error
	SetSuperNodePostponed(ctx sdk.Context, valAddr sdk.ValAddress, reason string) error
	RecoverSuperNodeFromPostponed(ctx sdk.Context, valAddr sdk.ValAddress) error
	SetMetricsState(ctx sdk.Context, state SupernodeMetricsState) error
	GetMetricsState(ctx sdk.Context, valAddr sdk.ValAddress) (SupernodeMetricsState, bool)
	Logger() log.Logger
	GetAuthority() string
	GetStakingKeeper() StakingKeeper
	GetParams(ctx sdk.Context) (params Params)
	GetAllSuperNodes(ctx sdk.Context, stateFilters ...SuperNodeState) ([]SuperNode, error)
	GetSuperNodeByAccount(ctx sdk.Context, supernodeAccount string) (SuperNode, bool, error)
	GetBlockHashForHeight(ctx sdk.Context, height int64) ([]byte, error)
	RankSuperNodesByDistance(blockHash []byte, supernodes []SuperNode, topN int) []SuperNode
	QuerySuperNode(ctx sdk.Context, valOperAddr sdk.ValAddress) (sn SuperNode, exists bool)
	GetSuperNodesPaginated(ctx sdk.Context, pagination *query.PageRequest, stateFilters ...SuperNodeState) ([]*SuperNode, *query.PageResponse, error)
	IsSuperNodeActive(ctx sdk.Context, valAddr sdk.ValAddress) bool
	IsEligibleAndNotJailedValidator(ctx sdk.Context, valAddr sdk.ValAddress) bool
	GetLastDistributionHeight(ctx sdk.Context) int64
	SetLastDistributionHeight(ctx sdk.Context, height int64)
	GetPoolBalance(ctx sdk.Context) sdk.Coins
	GetTotalDistributed(ctx sdk.Context) sdk.Coins
	GetSNDistState(ctx sdk.Context, valAddr string) (SNDistState, bool)
	GetRegistrationFeeShareBps(ctx sdk.Context) uint64
	CountEligibleSNs(ctx sdk.Context) uint64
	GetLatestCascadeBytesForPayout(ctx sdk.Context, supernodeAccount string) (float64, int64, bool)
}

// StakingKeeper defines the expected interface for the Staking module.
type StakingKeeper interface {
	ConsensusAddressCodec() address.Codec
	Validator(context.Context, sdk.ValAddress) (stakingtypes.ValidatorI, error)            // get a particular validator by operator address
	ValidatorByConsAddr(context.Context, sdk.ConsAddress) (stakingtypes.ValidatorI, error) // get a particular validator by consensus address
	Delegation(ctx context.Context, delAddr sdk.AccAddress, valAddr sdk.ValAddress) (stakingtypes.DelegationI, error)
}

// AuditKeeper defines the audit-source methods used by Everlight payout logic.
type AuditKeeper interface {
	GetCurrentEpochInfo(ctx sdk.Context) (epochID uint64, startHeight int64, endHeight int64, err error)
	GetReport(ctx sdk.Context, epochID uint64, reporterSupernodeAccount string) (audittypes.EpochReport, bool)
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
	GetModuleAddress(moduleName string) sdk.AccAddress
	GetModuleAccount(ctx context.Context, moduleName string) sdk.ModuleAccountI
}

// BankKeeper defines the expected interface for the Bank module.
type BankKeeper interface {
	SpendableCoins(context.Context, sdk.AccAddress) sdk.Coins
	GetBalance(ctx context.Context, addr sdk.AccAddress, denom string) sdk.Coin
	GetAllBalances(ctx context.Context, addr sdk.AccAddress) sdk.Coins
	SendCoinsFromModuleToAccount(ctx context.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error
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
	BeforeValidatorSlashed(ctx context.Context, valAddr sdk.ValAddress, fraction sdkmath.LegacyDec) error
}
