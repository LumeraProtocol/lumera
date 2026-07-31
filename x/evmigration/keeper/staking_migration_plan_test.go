package keeper_test

import (
	"testing"
	"time"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestStakingMigrationPlanUsesCanonicalRawUBDQueueKey(t *testing.T) {
	f := initMockFixture(t)
	legacy, replacement := testAccAddr(), testAccAddr()
	validator := sdk.ValAddress(testAccAddr())
	completion := f.ctx.BlockTime().Add(48 * time.Hour)
	ubd := stakingtypes.UnbondingDelegation{
		DelegatorAddress: legacy.String(),
		ValidatorAddress: validator.String(),
		Entries: []stakingtypes.UnbondingDelegationEntry{{
			CreationHeight: 4,
			CompletionTime: completion,
			InitialBalance: math.NewInt(25),
			Balance:        math.NewInt(25),
			UnbondingId:    17,
		}},
	}
	f.writeUnbondingDelegation(ubd)

	f.stakingKeeper.EXPECT().GetDelegatorDelegations(gomock.Any(), legacy, ^uint16(0)).Return(nil, nil)
	f.stakingKeeper.EXPECT().GetUnbondingDelegations(gomock.Any(), legacy, ^uint16(0)).Return([]stakingtypes.UnbondingDelegation{ubd}, nil)
	f.stakingKeeper.EXPECT().GetRedelegations(gomock.Any(), legacy, ^uint16(0)).Return(nil, nil)
	f.stakingKeeper.EXPECT().RemoveUnbondingDelegation(gomock.Any(), ubd).Return(nil)
	f.stakingKeeper.EXPECT().SetUnbondingDelegation(gomock.Any(), gomock.Any()).Return(nil)
	f.stakingKeeper.EXPECT().SetUnbondingDelegationByUnbondingID(gomock.Any(), gomock.Any(), uint64(17)).Return(nil)
	f.distributionKeeper.EXPECT().SetDelegatorWithdrawAddr(gomock.Any(), replacement, replacement).Return(nil)
	require.NoError(t, f.keeper.MigrateStaking(f.ctx, legacy, replacement, nil))

	// The migration must update the SDK's exact raw maturity-key row rather than
	// appending through InsertUBDQueue (which can leave the old tuple behind).
	key := stakingtypes.GetUnbondingDelegationTimeKey(completion)
	raw, err := f.stakingStore.OpenKVStore(f.ctx).Get(key)
	require.NoError(t, err)
	var pairs stakingtypes.DVPairs
	require.NoError(t, f.cdc.Unmarshal(raw, &pairs))
	require.Equal(t, []stakingtypes.DVPair{{
		DelegatorAddress: replacement.String(),
		ValidatorAddress: validator.String(),
	}}, pairs.Pairs)
}

func TestStakingMigrationPlanApplyFailureRollsBackRawQueueWrite(t *testing.T) {
	f := initMockFixture(t)
	legacy, replacement := testAccAddr(), testAccAddr()
	validator := sdk.ValAddress(testAccAddr())
	completion := f.ctx.BlockTime().Add(72 * time.Hour)
	ubd := stakingtypes.UnbondingDelegation{
		DelegatorAddress: legacy.String(),
		ValidatorAddress: validator.String(),
		Entries:          []stakingtypes.UnbondingDelegationEntry{{CompletionTime: completion}},
	}
	f.writeUnbondingDelegation(ubd)
	key := stakingtypes.GetUnbondingDelegationTimeKey(completion)
	before, err := f.stakingStore.OpenKVStore(f.ctx).Get(key)
	require.NoError(t, err)

	f.stakingKeeper.EXPECT().GetDelegatorDelegations(gomock.Any(), legacy, ^uint16(0)).Return(nil, nil)
	f.stakingKeeper.EXPECT().GetUnbondingDelegations(gomock.Any(), legacy, ^uint16(0)).Return([]stakingtypes.UnbondingDelegation{ubd}, nil)
	f.stakingKeeper.EXPECT().GetRedelegations(gomock.Any(), legacy, ^uint16(0)).Return(nil, nil)
	f.stakingKeeper.EXPECT().RemoveUnbondingDelegation(gomock.Any(), ubd).Return(assertionError("injected apply failure"))

	err = f.keeper.MigrateStaking(f.ctx, legacy, replacement, nil)
	require.ErrorContains(t, err, "injected apply failure")
	after, getErr := f.stakingStore.OpenKVStore(f.ctx).Get(key)
	require.NoError(t, getErr)
	require.Equal(t, before, after)
}

func TestStakingMigrationPlanRejectsExistingDestinationPrimary(t *testing.T) {
	f := initMockFixture(t)
	legacy, replacement := testAccAddr(), testAccAddr()
	validator := sdk.ValAddress(testAccAddr())
	old := stakingtypes.UnbondingDelegation{DelegatorAddress: legacy.String(), ValidatorAddress: validator.String()}
	destination := stakingtypes.UnbondingDelegation{DelegatorAddress: replacement.String(), ValidatorAddress: validator.String()}
	f.writeUnbondingDelegation(old)
	f.writeUnbondingDelegation(destination)

	f.stakingKeeper.EXPECT().GetDelegatorDelegations(gomock.Any(), legacy, ^uint16(0)).Return(nil, nil)
	f.stakingKeeper.EXPECT().GetUnbondingDelegations(gomock.Any(), legacy, ^uint16(0)).Return([]stakingtypes.UnbondingDelegation{old}, nil)
	f.stakingKeeper.EXPECT().GetRedelegations(gomock.Any(), legacy, ^uint16(0)).Return(nil, nil)

	err := f.keeper.MigrateStaking(f.ctx, legacy, replacement, nil)
	require.ErrorContains(t, err, "destination unbonding delegation primary already exists")
}

func TestStakingMigrationPlanRejectsMissingDelegationSourcePrimary(t *testing.T) {
	f := initMockFixture(t)
	legacy, replacement := testAccAddr(), testAccAddr()
	validator := sdk.ValAddress(testAccAddr())
	delegation := stakingtypes.NewDelegation(legacy.String(), validator.String(), math.LegacyNewDec(25))

	f.stakingKeeper.EXPECT().GetDelegatorDelegations(gomock.Any(), legacy, ^uint16(0)).Return([]stakingtypes.Delegation{delegation}, nil)
	f.stakingKeeper.EXPECT().GetUnbondingDelegations(gomock.Any(), legacy, ^uint16(0)).Return(nil, nil)
	f.stakingKeeper.EXPECT().GetRedelegations(gomock.Any(), legacy, ^uint16(0)).Return(nil, nil)

	err := f.keeper.MigrateStaking(f.ctx, legacy, replacement, nil)
	require.ErrorContains(t, err, "source delegation primary missing")
}

func TestStakingMigrationPlanRejectsExistingDelegationDestinationPrimary(t *testing.T) {
	f := initMockFixture(t)
	legacy, replacement := testAccAddr(), testAccAddr()
	validator := sdk.ValAddress(testAccAddr())
	source := stakingtypes.NewDelegation(legacy.String(), validator.String(), math.LegacyNewDec(25))
	seedDelegationPrimary(t, f, source)
	destination := stakingtypes.NewDelegation(replacement.String(), validator.String(), source.Shares)
	seedDelegationPrimary(t, f, destination)

	f.stakingKeeper.EXPECT().GetDelegatorDelegations(gomock.Any(), legacy, ^uint16(0)).Return([]stakingtypes.Delegation{source}, nil)
	f.stakingKeeper.EXPECT().GetUnbondingDelegations(gomock.Any(), legacy, ^uint16(0)).Return(nil, nil)
	f.stakingKeeper.EXPECT().GetRedelegations(gomock.Any(), legacy, ^uint16(0)).Return(nil, nil)

	err := f.keeper.MigrateStaking(f.ctx, legacy, replacement, nil)
	require.ErrorContains(t, err, "destination delegation primary already exists")
}

type assertionError string

func (e assertionError) Error() string { return string(e) }
