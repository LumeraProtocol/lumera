//go:generate mockgen -copyright_file=../../../testutil/mock_header.txt -destination=../mocks/expected_keepers_mock.go -package=evmigrationmocks -source=expected_keepers.go

package types

import (
	"context"
	"time"

	"cosmossdk.io/core/address"
	"cosmossdk.io/x/feegrant"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/authz"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	actiontypes "github.com/LumeraProtocol/lumera/x/action/v1/types"
	claimtypes "github.com/LumeraProtocol/lumera/x/claim/types"
	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

// AccountKeeper defines the expected interface for the x/auth module.
type AccountKeeper interface {
	AddressCodec() address.Codec
	GetAccount(ctx context.Context, addr sdk.AccAddress) sdk.AccountI
	SetAccount(ctx context.Context, acc sdk.AccountI)
	RemoveAccount(ctx context.Context, acc sdk.AccountI)
	NewAccountWithAddress(ctx context.Context, addr sdk.AccAddress) sdk.AccountI
	IterateAccounts(ctx context.Context, cb func(acc sdk.AccountI) (stop bool))
}

// BankKeeper defines the expected interface for the x/bank module.
type BankKeeper interface {
	GetAllBalances(ctx context.Context, addr sdk.AccAddress) sdk.Coins
	SendCoins(ctx context.Context, fromAddr, toAddr sdk.AccAddress, amt sdk.Coins) error
	BlockedAddr(addr sdk.AccAddress) bool
}

// StakingKeeper defines the expected interface for the x/staking module.
type StakingKeeper interface {
	GetValidator(ctx context.Context, addr sdk.ValAddress) (stakingtypes.Validator, error)
	SetValidator(ctx context.Context, validator stakingtypes.Validator) error
	ValidatorByConsAddr(ctx context.Context, consAddr sdk.ConsAddress) (stakingtypes.ValidatorI, error)

	GetDelegatorDelegations(ctx context.Context, delegator sdk.AccAddress, maxRetrieve uint16) ([]stakingtypes.Delegation, error)
	GetUnbondingDelegations(ctx context.Context, delegator sdk.AccAddress, maxRetrieve uint16) ([]stakingtypes.UnbondingDelegation, error)
	GetRedelegations(ctx context.Context, delegator sdk.AccAddress, maxRetrieve uint16) ([]stakingtypes.Redelegation, error)
	GetValidatorDelegations(ctx context.Context, valAddr sdk.ValAddress) ([]stakingtypes.Delegation, error)
	SetDelegation(ctx context.Context, delegation stakingtypes.Delegation) error
	RemoveDelegation(ctx context.Context, delegation stakingtypes.Delegation) error

	GetUnbondingDelegationsFromValidator(ctx context.Context, valAddr sdk.ValAddress) ([]stakingtypes.UnbondingDelegation, error)
	GetUnbondingDelegation(ctx context.Context, delAddr sdk.AccAddress, valAddr sdk.ValAddress) (stakingtypes.UnbondingDelegation, error)
	SetUnbondingDelegation(ctx context.Context, ubd stakingtypes.UnbondingDelegation) error
	RemoveUnbondingDelegation(ctx context.Context, ubd stakingtypes.UnbondingDelegation) error
	SetUnbondingDelegationByUnbondingID(ctx context.Context, ubd stakingtypes.UnbondingDelegation, id uint64) error
	InsertUBDQueue(ctx context.Context, ubd stakingtypes.UnbondingDelegation, completionTime time.Time) error

	GetRedelegationsFromSrcValidator(ctx context.Context, valAddr sdk.ValAddress) ([]stakingtypes.Redelegation, error)
	SetRedelegation(ctx context.Context, red stakingtypes.Redelegation) error
	RemoveRedelegation(ctx context.Context, red stakingtypes.Redelegation) error
	SetRedelegationByUnbondingID(ctx context.Context, red stakingtypes.Redelegation, id uint64) error
	InsertRedelegationQueue(ctx context.Context, red stakingtypes.Redelegation, completionTime time.Time) error

	GetLastValidatorPower(ctx context.Context, operator sdk.ValAddress) (int64, error)
	SetLastValidatorPower(ctx context.Context, operator sdk.ValAddress, power int64) error
	DeleteLastValidatorPower(ctx context.Context, operator sdk.ValAddress) error
	SetValidatorByPowerIndex(ctx context.Context, validator stakingtypes.Validator) error
	DeleteValidatorByPowerIndex(ctx context.Context, validator stakingtypes.Validator) error
	SetValidatorByConsAddr(ctx context.Context, validator stakingtypes.Validator) error

	BondDenom(ctx context.Context) (string, error)
}

// DistributionKeeper defines the expected interface for the x/distribution module.
type DistributionKeeper interface {
	WithdrawDelegationRewards(ctx context.Context, delAddr sdk.AccAddress, valAddr sdk.ValAddress) (sdk.Coins, error)
	WithdrawValidatorCommission(ctx context.Context, valAddr sdk.ValAddress) (sdk.Coins, error)

	GetDelegatorWithdrawAddr(ctx context.Context, delAddr sdk.AccAddress) (sdk.AccAddress, error)
	SetDelegatorWithdrawAddr(ctx context.Context, delAddr, withdrawAddr sdk.AccAddress) error

	GetDelegatorStartingInfo(ctx context.Context, val sdk.ValAddress, del sdk.AccAddress) (distrtypes.DelegatorStartingInfo, error)
	SetDelegatorStartingInfo(ctx context.Context, val sdk.ValAddress, del sdk.AccAddress, period distrtypes.DelegatorStartingInfo) error
	DeleteDelegatorStartingInfo(ctx context.Context, val sdk.ValAddress, del sdk.AccAddress) error

	GetValidatorCurrentRewards(ctx context.Context, val sdk.ValAddress) (distrtypes.ValidatorCurrentRewards, error)
	SetValidatorCurrentRewards(ctx context.Context, val sdk.ValAddress, rewards distrtypes.ValidatorCurrentRewards) error
	DeleteValidatorCurrentRewards(ctx context.Context, val sdk.ValAddress) error

	GetValidatorAccumulatedCommission(ctx context.Context, val sdk.ValAddress) (distrtypes.ValidatorAccumulatedCommission, error)
	SetValidatorAccumulatedCommission(ctx context.Context, val sdk.ValAddress, commission distrtypes.ValidatorAccumulatedCommission) error
	DeleteValidatorAccumulatedCommission(ctx context.Context, val sdk.ValAddress) error

	GetValidatorOutstandingRewards(ctx context.Context, val sdk.ValAddress) (distrtypes.ValidatorOutstandingRewards, error)
	SetValidatorOutstandingRewards(ctx context.Context, val sdk.ValAddress, rewards distrtypes.ValidatorOutstandingRewards) error
	DeleteValidatorOutstandingRewards(ctx context.Context, val sdk.ValAddress) error

	SetValidatorHistoricalRewards(ctx context.Context, val sdk.ValAddress, period uint64, rewards distrtypes.ValidatorHistoricalRewards) error
	DeleteValidatorHistoricalRewards(ctx context.Context, val sdk.ValAddress)
	IterateValidatorHistoricalRewards(ctx context.Context, handler func(val sdk.ValAddress, period uint64, rewards distrtypes.ValidatorHistoricalRewards) (stop bool))

	SetValidatorSlashEvent(ctx context.Context, val sdk.ValAddress, height, period uint64, event distrtypes.ValidatorSlashEvent) error
	DeleteValidatorSlashEvents(ctx context.Context, val sdk.ValAddress)
	IterateValidatorSlashEvents(ctx context.Context, handler func(val sdk.ValAddress, height uint64, event distrtypes.ValidatorSlashEvent) (stop bool))
}

// AuthzKeeper defines the expected interface for the x/authz module.
type AuthzKeeper interface {
	GetAuthorizations(ctx context.Context, grantee, granter sdk.AccAddress) ([]authz.Authorization, error)
	SaveGrant(ctx context.Context, grantee, granter sdk.AccAddress, authorization authz.Authorization, expiration *time.Time) error
	DeleteGrant(ctx context.Context, grantee, granter sdk.AccAddress, msgType string) error
	IterateGrants(ctx context.Context, handler func(granterAddr, granteeAddr sdk.AccAddress, grant authz.Grant) bool)
}

// FeegrantKeeper defines the expected interface for the x/feegrant module.
type FeegrantKeeper interface {
	IterateAllFeeAllowances(ctx context.Context, cb func(grant feegrant.Grant) bool) error
	GrantAllowance(ctx context.Context, granter, grantee sdk.AccAddress, feeAllowance feegrant.FeeAllowanceI) error
}

// SupernodeKeeper defines the expected interface for the x/supernode module.
type SupernodeKeeper interface {
	GetSuperNodeByAccount(ctx sdk.Context, supernodeAccount string) (sntypes.SuperNode, bool, error)
	QuerySuperNode(ctx sdk.Context, valOperAddr sdk.ValAddress) (sn sntypes.SuperNode, exists bool)
	SetSuperNode(ctx sdk.Context, supernode sntypes.SuperNode) error
	GetMetricsState(ctx sdk.Context, valAddr sdk.ValAddress) (sntypes.SupernodeMetricsState, bool)
	SetMetricsState(ctx sdk.Context, state sntypes.SupernodeMetricsState) error
	DeleteMetricsState(ctx sdk.Context, valAddr sdk.ValAddress)
}

// ActionKeeper defines the expected interface for the x/action module.
type ActionKeeper interface {
	IterateActions(ctx sdk.Context, handler func(*actiontypes.Action) bool) error
	SetAction(ctx sdk.Context, action *actiontypes.Action) error
	GetActionByID(ctx sdk.Context, actionID string) (*actiontypes.Action, bool)
}

// ClaimKeeper defines the expected interface for the x/claim module.
type ClaimKeeper interface {
	GetClaimRecord(ctx sdk.Context, address string) (val claimtypes.ClaimRecord, found bool, err error)
	SetClaimRecord(ctx sdk.Context, claimRecord claimtypes.ClaimRecord) error
	IterateClaimRecords(ctx sdk.Context, cb func(claimtypes.ClaimRecord) (stop bool, err error)) error
}
