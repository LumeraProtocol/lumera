package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"

	"github.com/LumeraProtocol/lumera/x/evmigration/types"
)

// --- MigrateValidator pre-check tests ---

// TestMigrateValidator_NotValidator verifies rejection when the legacy address
// is not a validator operator.
func TestMigrateValidator_NotValidator(t *testing.T) {
	f := initMsgServerFixture(t)

	privKey := secp256k1.GenPrivKey()
	legacyAddr := sdk.AccAddress(privKey.PubKey().Address())
	newPrivKey, newAddr := testNewMigrationAccount(t)

	baseAcc := authtypes.NewBaseAccountWithAddress(legacyAddr)
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), legacyAddr).Return(baseAcc)

	// Not a validator.
	oldValAddr := sdk.ValAddress(legacyAddr)
	f.stakingKeeper.EXPECT().GetValidator(gomock.Any(), oldValAddr).Return(
		stakingtypes.Validator{}, stakingtypes.ErrNoValidatorFound,
	)

	msg := newValidatorMigrationMsg(t, privKey, legacyAddr, newPrivKey, newAddr)

	_, err := f.msgServer.MigrateValidator(f.ctx, msg)
	require.ErrorIs(t, err, types.ErrNotValidator)
}

// TestMigrateValidator_UnbondingValidator verifies rejection when the validator
// is in unbonding or unbonded status.
func TestMigrateValidator_UnbondingValidator(t *testing.T) {
	f := initMsgServerFixture(t)

	privKey := secp256k1.GenPrivKey()
	legacyAddr := sdk.AccAddress(privKey.PubKey().Address())
	newPrivKey, newAddr := testNewMigrationAccount(t)

	baseAcc := authtypes.NewBaseAccountWithAddress(legacyAddr)
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), legacyAddr).Return(baseAcc)

	oldValAddr := sdk.ValAddress(legacyAddr)
	f.stakingKeeper.EXPECT().GetValidator(gomock.Any(), oldValAddr).Return(
		stakingtypes.Validator{
			OperatorAddress: legacyAddr.String(),
			Status:          stakingtypes.Unbonding,
		}, nil,
	)

	msg := newValidatorMigrationMsg(t, privKey, legacyAddr, newPrivKey, newAddr)

	_, err := f.msgServer.MigrateValidator(f.ctx, msg)
	require.ErrorIs(t, err, types.ErrValidatorUnbonding)
}

// TestMigrateValidator_TooManyDelegators verifies rejection when total delegation
// records exceed MaxValidatorDelegations.
func TestMigrateValidator_TooManyDelegators(t *testing.T) {
	f := initMsgServerFixture(t)

	// Set max to 1 for easy testing.
	params := types.NewParams(true, 0, 50, 1)
	require.NoError(t, f.keeper.Params.Set(f.ctx, params))

	privKey := secp256k1.GenPrivKey()
	legacyAddr := sdk.AccAddress(privKey.PubKey().Address())
	newPrivKey, newAddr := testNewMigrationAccount(t)

	baseAcc := authtypes.NewBaseAccountWithAddress(legacyAddr)
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), legacyAddr).Return(baseAcc)

	oldValAddr := sdk.ValAddress(legacyAddr)
	f.stakingKeeper.EXPECT().GetValidator(gomock.Any(), oldValAddr).Return(
		stakingtypes.Validator{
			OperatorAddress: legacyAddr.String(),
			Status:          stakingtypes.Bonded,
		}, nil,
	)

	// 2 delegations > max of 1.
	f.stakingKeeper.EXPECT().GetValidatorDelegations(gomock.Any(), oldValAddr).Return(
		[]stakingtypes.Delegation{
			stakingtypes.NewDelegation(testAccAddr().String(), oldValAddr.String(), math.LegacyNewDec(50)),
			stakingtypes.NewDelegation(testAccAddr().String(), oldValAddr.String(), math.LegacyNewDec(50)),
		}, nil,
	)
	f.stakingKeeper.EXPECT().GetUnbondingDelegationsFromValidator(gomock.Any(), oldValAddr).Return(nil, nil)
	f.stakingKeeper.EXPECT().GetRedelegationsFromSrcValidator(gomock.Any(), oldValAddr).Return(nil, nil)

	msg := newValidatorMigrationMsg(t, privKey, legacyAddr, newPrivKey, newAddr)

	_, err := f.msgServer.MigrateValidator(f.ctx, msg)
	require.ErrorIs(t, err, types.ErrTooManyDelegators)
}

// TestMigrateValidator_Success verifies the full happy-path validator migration:
// commission withdrawal, validator record re-keying, delegation re-keying,
// distribution state re-keying, supernode re-keying, account migration, finalization.
func TestMigrateValidator_Success(t *testing.T) {
	f := initMsgServerFixture(t)

	privKey := secp256k1.GenPrivKey()
	pubKey := privKey.PubKey().(*secp256k1.PubKey)
	legacyAddr := sdk.AccAddress(pubKey.Address())
	newPrivKey, newAddr := testNewMigrationAccount(t)
	oldValAddr := sdk.ValAddress(legacyAddr)
	newValAddr := sdk.ValAddress(newAddr)

	baseAcc := authtypes.NewBaseAccountWithAddress(legacyAddr)

	// preChecks: account exists and is not a module account.
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), legacyAddr).Return(baseAcc)

	// Validator exists and is bonded.
	val := stakingtypes.Validator{
		OperatorAddress: legacyAddr.String(),
		Status:          stakingtypes.Bonded,
	}
	f.stakingKeeper.EXPECT().GetValidator(gomock.Any(), oldValAddr).Return(val, nil)

	// Delegation count check — 1 delegation, no unbonding/redelegations.
	del := stakingtypes.NewDelegation(legacyAddr.String(), oldValAddr.String(), math.LegacyNewDec(100))
	f.stakingKeeper.EXPECT().GetValidatorDelegations(gomock.Any(), oldValAddr).Return(
		[]stakingtypes.Delegation{del}, nil,
	)
	f.stakingKeeper.EXPECT().GetUnbondingDelegationsFromValidator(gomock.Any(), oldValAddr).Return(nil, nil)
	f.stakingKeeper.EXPECT().GetRedelegationsFromSrcValidator(gomock.Any(), oldValAddr).Return(nil, nil)

	// Step V1: Withdraw commission and delegation rewards.
	f.distributionKeeper.EXPECT().WithdrawValidatorCommission(gomock.Any(), oldValAddr).Return(sdk.Coins{}, nil)
	f.distributionKeeper.EXPECT().WithdrawDelegationRewards(gomock.Any(), legacyAddr, oldValAddr).Return(sdk.Coins{}, nil)

	// Step V2: MigrateValidatorRecord.
	f.stakingKeeper.EXPECT().GetValidator(gomock.Any(), oldValAddr).Return(val, nil)
	f.stakingKeeper.EXPECT().DeleteValidatorByPowerIndex(gomock.Any(), val).Return(nil)
	f.stakingKeeper.EXPECT().SetValidator(gomock.Any(), gomock.Any()).Return(nil)
	f.stakingKeeper.EXPECT().SetValidatorByPowerIndex(gomock.Any(), gomock.Any()).Return(nil)
	f.stakingKeeper.EXPECT().GetLastValidatorPower(gomock.Any(), oldValAddr).Return(int64(100), nil)
	f.stakingKeeper.EXPECT().DeleteLastValidatorPower(gomock.Any(), oldValAddr).Return(nil)
	f.stakingKeeper.EXPECT().SetLastValidatorPower(gomock.Any(), newValAddr, int64(100)).Return(nil)
	f.stakingKeeper.EXPECT().SetValidatorByConsAddr(gomock.Any(), gomock.Any()).Return(nil)

	// Step V3: MigrateValidatorDistribution — re-key all distribution state.
	// Must happen before delegation re-keying.
	f.distributionKeeper.EXPECT().GetValidatorCurrentRewards(gomock.Any(), oldValAddr).Return(
		distrtypes.ValidatorCurrentRewards{Period: 3}, nil,
	)
	f.distributionKeeper.EXPECT().DeleteValidatorCurrentRewards(gomock.Any(), oldValAddr).Return(nil)
	f.distributionKeeper.EXPECT().SetValidatorCurrentRewards(gomock.Any(), newValAddr, gomock.Any()).Return(nil)

	f.distributionKeeper.EXPECT().GetValidatorAccumulatedCommission(gomock.Any(), oldValAddr).Return(
		distrtypes.ValidatorAccumulatedCommission{}, nil,
	)
	f.distributionKeeper.EXPECT().DeleteValidatorAccumulatedCommission(gomock.Any(), oldValAddr).Return(nil)
	f.distributionKeeper.EXPECT().SetValidatorAccumulatedCommission(gomock.Any(), newValAddr, gomock.Any()).Return(nil)

	f.distributionKeeper.EXPECT().GetValidatorOutstandingRewards(gomock.Any(), oldValAddr).Return(
		distrtypes.ValidatorOutstandingRewards{}, nil,
	)
	f.distributionKeeper.EXPECT().DeleteValidatorOutstandingRewards(gomock.Any(), oldValAddr).Return(nil)
	f.distributionKeeper.EXPECT().SetValidatorOutstandingRewards(gomock.Any(), newValAddr, gomock.Any()).Return(nil)

	// HistoricalRewards — one entry carried over to the new validator.
	f.distributionKeeper.EXPECT().IterateValidatorHistoricalRewards(gomock.Any(), gomock.Any()).
		Do(func(_ any, cb func(sdk.ValAddress, uint64, distrtypes.ValidatorHistoricalRewards) bool) {
			cb(oldValAddr, 2, distrtypes.ValidatorHistoricalRewards{ReferenceCount: 1})
		})
	f.distributionKeeper.EXPECT().DeleteValidatorHistoricalRewards(gomock.Any(), oldValAddr)
	f.distributionKeeper.EXPECT().SetValidatorHistoricalRewards(gomock.Any(), newValAddr, uint64(2), gomock.Any()).Return(nil)

	// SlashEvents — none.
	f.distributionKeeper.EXPECT().IterateValidatorSlashEvents(gomock.Any(), gomock.Any())
	f.distributionKeeper.EXPECT().DeleteValidatorSlashEvents(gomock.Any(), oldValAddr)

	// Step V4: MigrateValidatorDelegations — re-key the one delegation.
	f.stakingKeeper.EXPECT().GetValidatorDelegations(gomock.Any(), oldValAddr).Return(
		[]stakingtypes.Delegation{del}, nil,
	)
	// Reset target period refcount before delegation loop.
	f.distributionKeeper.EXPECT().GetValidatorCurrentRewards(gomock.Any(), newValAddr).Return(
		distrtypes.ValidatorCurrentRewards{Period: 3}, nil,
	)
	expectHistoricalRewardsReset(f.distributionKeeper, newValAddr, 2, 2)
	// Per-delegation re-keying.
	f.distributionKeeper.EXPECT().DeleteDelegatorStartingInfo(gomock.Any(), oldValAddr, legacyAddr).Return(nil)
	f.stakingKeeper.EXPECT().RemoveDelegation(gomock.Any(), del).Return(nil)
	f.stakingKeeper.EXPECT().SetDelegation(gomock.Any(), gomock.Any()).Return(nil)
	f.distributionKeeper.EXPECT().GetDelegatorStartingInfo(gomock.Any(), oldValAddr, legacyAddr).Return(
		distrtypes.DelegatorStartingInfo{}, nil,
	)
	expectHistoricalRewardsIncrement(f.distributionKeeper, newValAddr, 2, 1)
	f.distributionKeeper.EXPECT().SetDelegatorStartingInfo(gomock.Any(), newValAddr, legacyAddr, gomock.Any()).Return(nil)
	// No unbonding delegations or redelegations.
	f.stakingKeeper.EXPECT().GetUnbondingDelegationsFromValidator(gomock.Any(), oldValAddr).Return(nil, nil)
	f.stakingKeeper.EXPECT().IterateRedelegations(gomock.Any(), gomock.Any()).Return(nil)

	// Step V5: MigrateValidatorSupernode — not a supernode.
	f.supernodeKeeper.EXPECT().QuerySuperNode(gomock.Any(), oldValAddr).Return(sntypes.SuperNode{}, false)

	// Step V6: MigrateValidatorActions — no matching actions.
	f.actionKeeper.EXPECT().IterateActions(gomock.Any(), gomock.Any()).Return(nil)

	// Step V7: Account-level migration.
	// MigrateAuth
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), legacyAddr).Return(baseAcc)
	f.accountKeeper.EXPECT().RemoveAccount(gomock.Any(), baseAcc)
	newAcc := authtypes.NewBaseAccountWithAddress(newAddr)
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), newAddr).Return(nil)
	f.accountKeeper.EXPECT().NewAccountWithAddress(gomock.Any(), newAddr).Return(newAcc)
	f.accountKeeper.EXPECT().SetAccount(gomock.Any(), newAcc)

	// MigrateBank
	balances := sdk.NewCoins(sdk.NewInt64Coin("ulume", 500))
	f.bankKeeper.EXPECT().GetAllBalances(gomock.Any(), legacyAddr).Return(balances)
	f.bankKeeper.EXPECT().SendCoins(gomock.Any(), legacyAddr, newAddr, balances).Return(nil)

	// MigrateAuthz — no grants.
	f.authzKeeper.EXPECT().IterateGrants(gomock.Any(), gomock.Any())

	// MigrateFeegrant — no allowances.
	f.feegrantKeeper.EXPECT().IterateAllFeeAllowances(gomock.Any(), gomock.Any()).Return(nil)

	// MigrateClaim — no claim records targeting this address.
	f.claimKeeper.EXPECT().IterateClaimRecords(gomock.Any(), gomock.Any()).Return(nil)

	msg := newValidatorMigrationMsg(t, privKey, legacyAddr, newPrivKey, newAddr)

	resp, err := f.msgServer.MigrateValidator(f.ctx, msg)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Verify migration record was stored.
	record, err := f.keeper.MigrationRecords.Get(f.ctx, legacyAddr.String())
	require.NoError(t, err)
	require.Equal(t, legacyAddr.String(), record.LegacyAddress)
	require.Equal(t, newAddr.String(), record.NewAddress)

	// Verify counters were incremented.
	count, err := f.keeper.MigrationCounter.Get(f.ctx)
	require.NoError(t, err)
	require.Equal(t, uint64(1), count)

	// Validator counter SHOULD be incremented.
	valCount, err := f.keeper.ValidatorMigrationCounter.Get(f.ctx)
	require.NoError(t, err)
	require.Equal(t, uint64(1), valCount)
}
