package keeper_test

import (
	"errors"
	"fmt"
	"sort"
	"testing"

	corestore "cosmossdk.io/core/store"
	"cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	"cosmossdk.io/x/feegrant"
	"github.com/cosmos/cosmos-sdk/codec"
	addresscodec "github.com/cosmos/cosmos-sdk/codec/address"
	kmultisig "github.com/cosmos/cosmos-sdk/crypto/keys/multisig"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	moduletestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	vestingtypes "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
	"github.com/cosmos/cosmos-sdk/x/authz"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	ethsecp256k1 "github.com/cosmos/evm/crypto/ethsecp256k1"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	actiontypes "github.com/LumeraProtocol/lumera/x/action/v1/types"
	"github.com/LumeraProtocol/lumera/x/evmigration/keeper"
	evmigrationmocks "github.com/LumeraProtocol/lumera/x/evmigration/mocks"
	module "github.com/LumeraProtocol/lumera/x/evmigration/module"
	"github.com/LumeraProtocol/lumera/x/evmigration/types"
	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

// mockFixture is a test fixture with gomock-based keeper mocks.
type mockFixture struct {
	ctx                sdk.Context
	keeper             keeper.Keeper
	cdc                codec.Codec
	stakingStore       corestore.KVStoreService
	distributionStore  corestore.KVStoreService
	accountKeeper      *evmigrationmocks.MockAccountKeeper
	bankKeeper         *evmigrationmocks.MockBankKeeper
	stakingKeeper      *evmigrationmocks.MockStakingKeeper
	distributionKeeper *evmigrationmocks.MockDistributionKeeper
	authzKeeper        *evmigrationmocks.MockAuthzKeeper
	feegrantKeeper     *evmigrationmocks.MockFeegrantKeeper
	supernodeKeeper    *evmigrationmocks.MockSupernodeKeeper
	actionKeeper       *evmigrationmocks.MockActionKeeper
}

func initMockFixture(t *testing.T) *mockFixture {
	t.Helper()

	ctrl := gomock.NewController(t)

	accountKeeper := evmigrationmocks.NewMockAccountKeeper(ctrl)
	bankKeeper := evmigrationmocks.NewMockBankKeeper(ctrl)
	stakingKeeper := evmigrationmocks.NewMockStakingKeeper(ctrl)
	distributionKeeper := evmigrationmocks.NewMockDistributionKeeper(ctrl)
	authzKeeper := evmigrationmocks.NewMockAuthzKeeper(ctrl)
	feegrantKeeper := evmigrationmocks.NewMockFeegrantKeeper(ctrl)
	supernodeKeeper := evmigrationmocks.NewMockSupernodeKeeper(ctrl)
	actionKeeper := evmigrationmocks.NewMockActionKeeper(ctrl)

	encCfg := moduletestutil.MakeTestEncodingConfig(module.AppModule{})
	addrCodec := addresscodec.NewBech32Codec(sdk.GetConfig().GetBech32AccountAddrPrefix())
	storeKey := storetypes.NewKVStoreKey(types.StoreKey)
	stakingStoreKey := storetypes.NewKVStoreKey(stakingtypes.StoreKey)
	distributionStoreKey := storetypes.NewKVStoreKey(distrtypes.StoreKey)
	storeService := runtime.NewKVStoreService(storeKey)
	stakingStoreService := runtime.NewKVStoreService(stakingStoreKey)
	distributionStoreService := runtime.NewKVStoreService(distributionStoreKey)
	ctx := testutil.DefaultContextWithKeys(
		map[string]*storetypes.KVStoreKey{
			types.StoreKey:        storeKey,
			stakingtypes.StoreKey: stakingStoreKey,
			distrtypes.StoreKey:   distributionStoreKey,
		},
		map[string]*storetypes.TransientStoreKey{"transient_test": storetypes.NewTransientStoreKey("transient_test")},
		nil,
	)

	authority := authtypes.NewModuleAddress(types.GovModuleName)

	k := keeper.NewKeeper(
		storeService,
		encCfg.Codec,
		addrCodec,
		authority,
		accountKeeper,
		bankKeeper,
		stakingKeeper,
		distributionKeeper,
		authzKeeper,
		feegrantKeeper,
		supernodeKeeper,
		actionKeeper,
	)

	// Initialize params with migration enabled.
	params := types.NewParams(true, 0, 50, 2000, 20)
	require.NoError(t, k.Params.Set(ctx, params))
	require.NoError(t, k.MigrationCounter.Set(ctx, 0))
	require.NoError(t, k.ValidatorMigrationCounter.Set(ctx, 0))

	return &mockFixture{
		ctx:                ctx,
		keeper:             k,
		cdc:                encCfg.Codec,
		stakingStore:       stakingStoreService,
		distributionStore:  distributionStoreService,
		accountKeeper:      accountKeeper,
		bankKeeper:         bankKeeper,
		stakingKeeper:      stakingKeeper,
		distributionKeeper: distributionKeeper,
		authzKeeper:        authzKeeper,
		feegrantKeeper:     feegrantKeeper,
		supernodeKeeper:    supernodeKeeper,
		actionKeeper:       actionKeeper,
	}
}

func (f *mockFixture) wireScopedMigrationStores() {
	f.keeper.SetStakingStoreService(f.stakingStore)
	f.keeper.SetDistributionStoreService(f.distributionStore)
}

func (f *mockFixture) writeRedelegation(red stakingtypes.Redelegation) {
	delegator, err := sdk.AccAddressFromBech32(red.DelegatorAddress)
	if err != nil {
		panic(err)
	}
	src, err := sdk.ValAddressFromBech32(red.ValidatorSrcAddress)
	if err != nil {
		panic(err)
	}
	dst, err := sdk.ValAddressFromBech32(red.ValidatorDstAddress)
	if err != nil {
		panic(err)
	}

	store := f.stakingStore.OpenKVStore(f.ctx)
	bz := stakingtypes.MustMarshalRED(f.cdc, red)
	if err := store.Set(stakingtypes.GetREDKey(delegator, src, dst), bz); err != nil {
		panic(err)
	}
	if err := store.Set(stakingtypes.GetREDByValSrcIndexKey(delegator, src, dst), []byte{}); err != nil {
		panic(err)
	}
	if err := store.Set(stakingtypes.GetREDByValDstIndexKey(delegator, src, dst), []byte{}); err != nil {
		panic(err)
	}
}

func (f *mockFixture) writeRedelegationIndexes(delegator sdk.AccAddress, src, dst sdk.ValAddress) {
	store := f.stakingStore.OpenKVStore(f.ctx)
	if err := store.Set(stakingtypes.GetREDByValSrcIndexKey(delegator, src, dst), []byte{}); err != nil {
		panic(err)
	}
	if err := store.Set(stakingtypes.GetREDByValDstIndexKey(delegator, src, dst), []byte{}); err != nil {
		panic(err)
	}
}

func (f *mockFixture) writeValidatorHistoricalRewards(
	val sdk.ValAddress,
	period uint64,
	rewards distrtypes.ValidatorHistoricalRewards,
) {
	store := f.distributionStore.OpenKVStore(f.ctx)
	if err := store.Set(distrtypes.GetValidatorHistoricalRewardsKey(val, period), f.cdc.MustMarshal(&rewards)); err != nil {
		panic(err)
	}
}

func (f *mockFixture) writeValidatorSlashEvent(
	val sdk.ValAddress,
	height uint64,
	event distrtypes.ValidatorSlashEvent,
) {
	store := f.distributionStore.OpenKVStore(f.ctx)
	if err := store.Set(distrtypes.GetValidatorSlashEventKey(val, height, event.ValidatorPeriod), f.cdc.MustMarshal(&event)); err != nil {
		panic(err)
	}
}

func countKVPrefix(t *testing.T, storeService corestore.KVStoreService, ctx sdk.Context, prefix []byte) int {
	t.Helper()

	store := storeService.OpenKVStore(ctx)
	iterator, err := store.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	require.NoError(t, err)
	defer func() { _ = iterator.Close() }()

	var count int
	for ; iterator.Valid(); iterator.Next() {
		count++
	}
	return count
}

func testAccAddr() sdk.AccAddress {
	return sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
}

func expectHistoricalRewardsLookup(
	mock *evmigrationmocks.MockDistributionKeeper,
	val sdk.ValAddress,
	period uint64,
	refCount uint32,
) {
	mock.EXPECT().IterateValidatorHistoricalRewards(gomock.Any(), gomock.Any()).
		Do(func(_ any, cb func(sdk.ValAddress, uint64, distrtypes.ValidatorHistoricalRewards) bool) {
			cb(val, period, distrtypes.ValidatorHistoricalRewards{ReferenceCount: refCount})
		})
}

func expectHistoricalRewardsIncrement(
	mock *evmigrationmocks.MockDistributionKeeper,
	val sdk.ValAddress,
	period uint64,
	refCount uint32,
) {
	expectHistoricalRewardsLookup(mock, val, period, refCount)
	mock.EXPECT().SetValidatorHistoricalRewards(gomock.Any(), val, period, gomock.Any()).Return(nil)
}

// expectHistoricalRewardsSet sets up mock expectations for
// setHistoricalRewardsReferenceCount: look up the (val, period) row, then write
// its refcount back in a single set.
func expectHistoricalRewardsSet(
	mock *evmigrationmocks.MockDistributionKeeper,
	val sdk.ValAddress,
	period uint64,
	refCount uint32,
) {
	expectHistoricalRewardsLookup(mock, val, period, refCount)
	mock.EXPECT().SetValidatorHistoricalRewards(gomock.Any(), val, period, gomock.Any()).Return(nil)
}

// --- MigrateAuth tests ---

// TestMigrateAuth_BaseAccount verifies that a plain BaseAccount is removed
// from the legacy address and a new account is created at the new address.
func TestMigrateAuth_BaseAccount(t *testing.T) {
	f := initMockFixture(t)
	legacy := testAccAddr()
	newAddr := testAccAddr()
	baseAcc := authtypes.NewBaseAccountWithAddress(legacy)

	// Phase 1 probe: GetAccount(newAddr) is called first.
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), newAddr).Return(nil)
	// Phase 2: fetch + remove legacy, then create new account.
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), legacy).Return(baseAcc)
	f.accountKeeper.EXPECT().RemoveAccount(gomock.Any(), baseAcc)
	newAcc := authtypes.NewBaseAccountWithAddress(newAddr)
	f.accountKeeper.EXPECT().NewAccountWithAddress(gomock.Any(), newAddr).Return(newAcc)
	f.accountKeeper.EXPECT().SetAccount(gomock.Any(), newAcc)

	vi, err := f.keeper.MigrateAuth(f.ctx, legacy, newAddr, nil)
	require.NoError(t, err)
	require.Nil(t, vi)
}

// TestMigrateAuth_ContinuousVesting verifies that ContinuousVestingAccount
// parameters (start time, end time, original vesting) are captured in VestingInfo.
func TestMigrateAuth_ContinuousVesting(t *testing.T) {
	f := initMockFixture(t)
	legacy := testAccAddr()
	newAddr := testAccAddr()

	baseAcc := authtypes.NewBaseAccountWithAddress(legacy)
	origVesting := sdk.NewCoins(sdk.NewInt64Coin("ulume", 1000))
	bva, err := vestingtypes.NewBaseVestingAccount(baseAcc, origVesting, 1000000)
	require.NoError(t, err)
	cva := vestingtypes.NewContinuousVestingAccountRaw(bva, 500000)

	// Phase 1 probe: GetAccount(newAddr) is called first.
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), newAddr).Return(nil)
	// Phase 2: fetch + remove legacy, then create new account.
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), legacy).Return(cva)
	f.accountKeeper.EXPECT().RemoveAccount(gomock.Any(), cva)
	newAcc := authtypes.NewBaseAccountWithAddress(newAddr)
	f.accountKeeper.EXPECT().NewAccountWithAddress(gomock.Any(), newAddr).Return(newAcc)
	f.accountKeeper.EXPECT().SetAccount(gomock.Any(), newAcc)

	vi, err := f.keeper.MigrateAuth(f.ctx, legacy, newAddr, nil)
	require.NoError(t, err)
	require.NotNil(t, vi)
	require.Equal(t, origVesting, vi.OriginalVesting)
	require.Equal(t, int64(1000000), vi.EndTime)
	require.Equal(t, int64(500000), vi.StartTime)
}

// TestMigrateAuth_DelayedVesting verifies that DelayedVestingAccount parameters
// are captured in VestingInfo.
func TestMigrateAuth_DelayedVesting(t *testing.T) {
	f := initMockFixture(t)
	legacy := testAccAddr()
	newAddr := testAccAddr()

	baseAcc := authtypes.NewBaseAccountWithAddress(legacy)
	origVesting := sdk.NewCoins(sdk.NewInt64Coin("ulume", 500))
	bva, err := vestingtypes.NewBaseVestingAccount(baseAcc, origVesting, 2000000)
	require.NoError(t, err)
	dva := vestingtypes.NewDelayedVestingAccountRaw(bva)

	// Phase 1 probe: GetAccount(newAddr) is called first.
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), newAddr).Return(nil)
	// Phase 2: fetch + remove legacy, then create new account.
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), legacy).Return(dva)
	f.accountKeeper.EXPECT().RemoveAccount(gomock.Any(), dva)
	newAcc := authtypes.NewBaseAccountWithAddress(newAddr)
	f.accountKeeper.EXPECT().NewAccountWithAddress(gomock.Any(), newAddr).Return(newAcc)
	f.accountKeeper.EXPECT().SetAccount(gomock.Any(), newAcc)

	vi, err := f.keeper.MigrateAuth(f.ctx, legacy, newAddr, nil)
	require.NoError(t, err)
	require.NotNil(t, vi)
	require.Equal(t, int64(2000000), vi.EndTime)
}

// TestMigrateAuth_PeriodicVesting verifies that PeriodicVestingAccount parameters
// including vesting periods are captured in VestingInfo.
func TestMigrateAuth_PeriodicVesting(t *testing.T) {
	f := initMockFixture(t)
	legacy := testAccAddr()
	newAddr := testAccAddr()

	baseAcc := authtypes.NewBaseAccountWithAddress(legacy)
	origVesting := sdk.NewCoins(sdk.NewInt64Coin("ulume", 1000))
	bva, err := vestingtypes.NewBaseVestingAccount(baseAcc, origVesting, 3000000)
	require.NoError(t, err)
	periods := vestingtypes.Periods{
		{Length: 100000, Amount: sdk.NewCoins(sdk.NewInt64Coin("ulume", 500))},
		{Length: 200000, Amount: sdk.NewCoins(sdk.NewInt64Coin("ulume", 500))},
	}
	pva := vestingtypes.NewPeriodicVestingAccountRaw(bva, 1000000, periods)

	// Phase 1 probe: GetAccount(newAddr) is called first.
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), newAddr).Return(nil)
	// Phase 2: fetch + remove legacy, then create new account.
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), legacy).Return(pva)
	f.accountKeeper.EXPECT().RemoveAccount(gomock.Any(), pva)
	newAcc := authtypes.NewBaseAccountWithAddress(newAddr)
	f.accountKeeper.EXPECT().NewAccountWithAddress(gomock.Any(), newAddr).Return(newAcc)
	f.accountKeeper.EXPECT().SetAccount(gomock.Any(), newAcc)

	vi, err := f.keeper.MigrateAuth(f.ctx, legacy, newAddr, nil)
	require.NoError(t, err)
	require.NotNil(t, vi)
	require.Len(t, vi.Periods, 2)
}

// TestMigrateAuth_PermanentLocked verifies that PermanentLockedAccount parameters
// are captured in VestingInfo.
func TestMigrateAuth_PermanentLocked(t *testing.T) {
	f := initMockFixture(t)
	legacy := testAccAddr()
	newAddr := testAccAddr()

	baseAcc := authtypes.NewBaseAccountWithAddress(legacy)
	origVesting := sdk.NewCoins(sdk.NewInt64Coin("ulume", 1000))
	pla, err := vestingtypes.NewPermanentLockedAccount(baseAcc, origVesting)
	require.NoError(t, err)

	// Phase 1 probe: GetAccount(newAddr) is called first.
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), newAddr).Return(nil)
	// Phase 2: fetch + remove legacy, then create new account.
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), legacy).Return(pla)
	f.accountKeeper.EXPECT().RemoveAccount(gomock.Any(), pla)
	newAcc := authtypes.NewBaseAccountWithAddress(newAddr)
	f.accountKeeper.EXPECT().NewAccountWithAddress(gomock.Any(), newAddr).Return(newAcc)
	f.accountKeeper.EXPECT().SetAccount(gomock.Any(), newAcc)

	vi, err := f.keeper.MigrateAuth(f.ctx, legacy, newAddr, nil)
	require.NoError(t, err)
	require.NotNil(t, vi)
	require.Equal(t, origVesting, vi.OriginalVesting)
}

// TestMigrateAuth_ModuleAccount verifies that module accounts are rejected.
// The legacy module-account check is in Phase 2; Phase 1 probe of newAddr runs first.
func TestMigrateAuth_ModuleAccount(t *testing.T) {
	f := initMockFixture(t)
	legacy := testAccAddr()
	newAddr := testAccAddr()

	// Phase 1 probe: newAddr is fresh.
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), newAddr).Return(nil)
	// Phase 2: legacy fetch returns a module account → rejected.
	modAcc := authtypes.NewEmptyModuleAccount("bonded_tokens_pool")
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), legacy).Return(modAcc)

	vi, err := f.keeper.MigrateAuth(f.ctx, legacy, newAddr, nil)
	require.ErrorIs(t, err, types.ErrCannotMigrateModuleAccount)
	require.Nil(t, vi)
}

// TestMigrateAuth_AccountNotFound verifies error when legacy account does not exist.
func TestMigrateAuth_AccountNotFound(t *testing.T) {
	f := initMockFixture(t)
	legacy := testAccAddr()
	newAddr := testAccAddr()

	// Phase 1 probe: newAddr is fresh.
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), newAddr).Return(nil)
	// Phase 2: legacy fetch returns nil → ErrLegacyAccountNotFound.
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), legacy).Return(nil)

	vi, err := f.keeper.MigrateAuth(f.ctx, legacy, newAddr, nil)
	require.ErrorIs(t, err, types.ErrLegacyAccountNotFound)
	require.Nil(t, vi)
}

// TestMigrateAuth_NewAddressAlreadyExists verifies that if the new address already
// has a plain BaseAccount, it is reused (not recreated) and SetAccount is still called.
func TestMigrateAuth_NewAddressAlreadyExists(t *testing.T) {
	f := initMockFixture(t)
	legacy := testAccAddr()
	newAddr := testAccAddr()
	baseAcc := authtypes.NewBaseAccountWithAddress(legacy)

	// Phase 1 probe: newAddr already has a BaseAccount.
	existingAcc := authtypes.NewBaseAccountWithAddress(newAddr)
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), newAddr).Return(existingAcc)
	// Phase 2: fetch + remove legacy, reuse cached existingAcc, call SetAccount.
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), legacy).Return(baseAcc)
	f.accountKeeper.EXPECT().RemoveAccount(gomock.Any(), baseAcc)
	f.accountKeeper.EXPECT().SetAccount(gomock.Any(), existingAcc)

	vi, err := f.keeper.MigrateAuth(f.ctx, legacy, newAddr, nil)
	require.NoError(t, err)
	require.Nil(t, vi)
}

// --- MigrateAuth Phase 1/2 new tests ---

// makeMultisigProof builds a well-formed new-side MultisigProof from freshly
// generated eth_secp256k1 keys and returns both the proof and the slice of
// public keys so callers can derive the expected multisig address.
func makeMultisigProof(t *testing.T, n int, threshold uint32) (*types.MigrationProof, []cryptotypes.PubKey) {
	t.Helper()
	subKeys := make([]cryptotypes.PubKey, n)
	rawKeys := make([][]byte, n)
	sigs := make([][]byte, int(threshold))
	idxs := make([]uint32, int(threshold))

	for i := 0; i < n; i++ {
		priv, err := ethsecp256k1.GenerateKey()
		require.NoError(t, err)
		subKeys[i] = priv.PubKey()
		rawKeys[i] = priv.PubKey().Bytes()
	}
	for i := uint32(0); i < threshold; i++ {
		sigs[i] = make([]byte, 65) // 65-byte new-side sub-sig placeholder
		idxs[i] = i
	}

	proof := &types.MigrationProof{
		Proof: &types.MigrationProof_Multisig{
			Multisig: &types.MultisigProof{
				Threshold:     threshold,
				SubPubKeys:    rawKeys,
				SignerIndices: idxs,
				SubSignatures: sigs,
				SigFormat:     types.SigFormat_SIG_FORMAT_CLI,
			},
		},
	}
	return proof, subKeys
}

// deriveMultisigAddr reconstructs the LegacyAminoPubKey address from a slice
// of eth_secp256k1 public keys and a threshold, matching MigrateAuth's logic.
func deriveMultisigAddr(subKeys []cryptotypes.PubKey, threshold int) sdk.AccAddress {
	multiPK := kmultisig.NewLegacyAminoPubKey(threshold, subKeys)
	return sdk.AccAddress(multiPK.Address())
}

// TestMigrateAuth_MultisigDestination_SetsPubKey verifies that a multisig destProof
// causes the reconstructed pubkey to be persisted on the new account.
func TestMigrateAuth_MultisigDestination_SetsPubKey(t *testing.T) {
	f := initMockFixture(t)
	legacy := testAccAddr()

	// threshold in proof and in deriveMultisigAddr must match for address check to pass.
	proof, subKeys := makeMultisigProof(t, 3, 2)
	newAddr := deriveMultisigAddr(subKeys, 2)

	legacyAcc := authtypes.NewBaseAccountWithAddress(legacy)

	// Phase 1 probe: newAddr is fresh.
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), newAddr).Return(nil)
	// Phase 2.
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), legacy).Return(legacyAcc)
	f.accountKeeper.EXPECT().RemoveAccount(gomock.Any(), legacyAcc)
	newAcc := authtypes.NewBaseAccountWithAddress(newAddr)
	f.accountKeeper.EXPECT().NewAccountWithAddress(gomock.Any(), newAddr).Return(newAcc)
	f.accountKeeper.EXPECT().SetAccount(gomock.Any(), gomock.Any()).Do(func(_ any, acc sdk.AccountI) {
		require.NotNil(t, acc.GetPubKey(), "expected multisig pubkey to be set on new account")
		// threshold=2 matches what makeMultisigProof encoded.
		expected := kmultisig.NewLegacyAminoPubKey(2, subKeys)
		require.Equal(t, expected.Bytes(), acc.GetPubKey().Bytes())
	})

	vi, err := f.keeper.MigrateAuth(f.ctx, legacy, newAddr, proof)
	require.NoError(t, err)
	require.Nil(t, vi)
}

// TestMigrateAuth_SingleKeyDestination_NilPubKey verifies that a single-key destProof
// leaves the new BaseAccount with a nil pubkey (consistent with pre-refactor behaviour).
func TestMigrateAuth_SingleKeyDestination_NilPubKey(t *testing.T) {
	f := initMockFixture(t)
	legacy := testAccAddr()
	newAddr := testAccAddr()
	legacyAcc := authtypes.NewBaseAccountWithAddress(legacy)

	priv, err := ethsecp256k1.GenerateKey()
	require.NoError(t, err)
	singleProof := &types.MigrationProof{
		Proof: &types.MigrationProof_Single{
			Single: &types.SingleKeyProof{
				PubKey:    priv.PubKey().Bytes(),
				Signature: make([]byte, 65),
				SigFormat: types.SigFormat_SIG_FORMAT_CLI,
			},
		},
	}

	// Phase 1 probe: newAddr is fresh.
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), newAddr).Return(nil)
	// Phase 2.
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), legacy).Return(legacyAcc)
	f.accountKeeper.EXPECT().RemoveAccount(gomock.Any(), legacyAcc)
	newAcc := authtypes.NewBaseAccountWithAddress(newAddr)
	f.accountKeeper.EXPECT().NewAccountWithAddress(gomock.Any(), newAddr).Return(newAcc)
	f.accountKeeper.EXPECT().SetAccount(gomock.Any(), gomock.Any()).Do(func(_ any, acc sdk.AccountI) {
		require.Nil(t, acc.GetPubKey(), "expected nil pubkey for single-key proof")
	})

	vi, err := f.keeper.MigrateAuth(f.ctx, legacy, newAddr, singleProof)
	require.NoError(t, err)
	require.Nil(t, vi)
}

// TestMigrateAuth_MultisigDestination_AddressMismatch verifies that a multisig
// destProof whose sub-keys derive to a different address than newAddr is rejected
// in Phase 1 with no state mutation.
func TestMigrateAuth_MultisigDestination_AddressMismatch(t *testing.T) {
	f := initMockFixture(t)
	// newAddr is a random address that will NOT match the multisig derivation.
	newAddr := testAccAddr()

	proof, _ := makeMultisigProof(t, 2, 1)

	// Phase 1 probe: newAddr is fresh (only call allowed before rejection).
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), newAddr).Return(nil)
	// No further keeper calls — rejection must be pre-mutation.

	_, err := f.keeper.MigrateAuth(f.ctx, testAccAddr(), newAddr, proof)
	require.ErrorIs(t, err, types.ErrPubKeyAddressMismatch)
}

// TestMigrateAuth_PreExistingVestingDestination_Rejected verifies that a pre-existing
// ContinuousVestingAccount at newAddr is rejected in Phase 1 with no state mutation.
func TestMigrateAuth_PreExistingVestingDestination_Rejected(t *testing.T) {
	f := initMockFixture(t)
	legacy := testAccAddr()
	newAddr := testAccAddr()

	baseAcc := authtypes.NewBaseAccountWithAddress(newAddr)
	origVesting := sdk.NewCoins(sdk.NewInt64Coin("ulume", 500))
	bva, err := vestingtypes.NewBaseVestingAccount(baseAcc, origVesting, 1000000)
	require.NoError(t, err)
	cva := vestingtypes.NewContinuousVestingAccountRaw(bva, 500000)

	// Phase 1 probe: returns a vesting account → rejected immediately.
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), newAddr).Return(cva)
	// No GetAccount(legacy), no RemoveAccount, no SetAccount.

	_, err = f.keeper.MigrateAuth(f.ctx, legacy, newAddr, nil)
	require.ErrorIs(t, err, types.ErrInvalidMigrationDestination)
	require.Contains(t, err.Error(), "non-BaseAccount")
}

// TestMigrateAuth_PreExistingModuleDestination_Rejected verifies that a pre-existing
// ModuleAccount at newAddr is rejected in Phase 1 with ErrCannotMigrateModuleAccount.
func TestMigrateAuth_PreExistingModuleDestination_Rejected(t *testing.T) {
	f := initMockFixture(t)
	legacy := testAccAddr()
	// Use the module account's own address as newAddr.
	modAcc := authtypes.NewEmptyModuleAccount("test_pool")
	newAddr := modAcc.GetAddress()

	// Phase 1 probe: returns a module account → rejected immediately.
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), newAddr).Return(modAcc)
	// No GetAccount(legacy), no RemoveAccount, no SetAccount.

	_, err := f.keeper.MigrateAuth(f.ctx, legacy, newAddr, nil)
	require.ErrorIs(t, err, types.ErrCannotMigrateModuleAccount)
}

// TestMigrateAuth_MultisigDestination_PreExistingMismatchedPubKey_Rejected verifies
// that a pre-existing BaseAccount at newAddr with a DIFFERENT pubkey already set
// causes a pre-mutation rejection (refusing to overwrite).
func TestMigrateAuth_MultisigDestination_PreExistingMismatchedPubKey_Rejected(t *testing.T) {
	f := initMockFixture(t)

	// threshold must equal the value passed to deriveMultisigAddr so the address
	// derivation inside MigrateAuth matches newAddr.
	proof, subKeys := makeMultisigProof(t, 2, 2)
	newAddr := deriveMultisigAddr(subKeys, 2)

	// Pre-existing account with a DIFFERENT pubkey.
	otherPriv, err := ethsecp256k1.GenerateKey()
	require.NoError(t, err)
	existingAcc := authtypes.NewBaseAccountWithAddress(newAddr)
	require.NoError(t, existingAcc.SetPubKey(otherPriv.PubKey()))

	// Phase 1 probe: returns existing account with mismatched pubkey → rejected.
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), newAddr).Return(existingAcc)
	// No GetAccount(legacy), no RemoveAccount, no SetAccount.

	_, err = f.keeper.MigrateAuth(f.ctx, testAccAddr(), newAddr, proof)
	require.ErrorIs(t, err, types.ErrPubKeyAddressMismatch)
	require.Contains(t, err.Error(), "refusing to overwrite")
}

// TestMigrateAuth_MultisigDestination_PreExistingMatchingPubKey_Idempotent verifies
// that a pre-existing BaseAccount at newAddr with the CORRECT multisig pubkey already
// set succeeds without calling SetPubKey again (idempotent re-run).
// GetAccount(newAddr) is called exactly ONCE (Phase 1 probe reused in Phase 2).
func TestMigrateAuth_MultisigDestination_PreExistingMatchingPubKey_Idempotent(t *testing.T) {
	f := initMockFixture(t)
	legacy := testAccAddr()

	// threshold=2 must match deriveMultisigAddr so MigrateAuth's address check passes.
	proof, subKeys := makeMultisigProof(t, 2, 2)
	newAddr := deriveMultisigAddr(subKeys, 2)

	// Pre-existing account with the MATCHING multisig pubkey.
	multiPK := kmultisig.NewLegacyAminoPubKey(2, subKeys)
	existingAcc := authtypes.NewBaseAccountWithAddress(newAddr)
	require.NoError(t, existingAcc.SetPubKey(multiPK))

	legacyAcc := authtypes.NewBaseAccountWithAddress(legacy)

	// Phase 1 probe: returns existing account with matching pubkey.
	// This is the ONE AND ONLY GetAccount(newAddr) call.
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), newAddr).Return(existingAcc)
	// Phase 2: legacy removal, then SetAccount (pubkey already set → skipped internally).
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), legacy).Return(legacyAcc)
	f.accountKeeper.EXPECT().RemoveAccount(gomock.Any(), legacyAcc)
	f.accountKeeper.EXPECT().SetAccount(gomock.Any(), existingAcc).Do(func(_ any, acc sdk.AccountI) {
		// Pubkey must still be the original matching key — not nil, not overwritten.
		require.NotNil(t, acc.GetPubKey())
		require.Equal(t, multiPK.Bytes(), acc.GetPubKey().Bytes())
	})

	vi, err := f.keeper.MigrateAuth(f.ctx, legacy, newAddr, proof)
	require.NoError(t, err)
	require.Nil(t, vi)
}

// TestMigrateAuth_MultisigDestination_PreExistingFundedButNeverSigned_SetsPubKey
// verifies the third positive-case branch: a pre-existing BaseAccount at newAddr
// with nil pubkey (funded-but-never-signed — someone sent coins to the address
// pre-migration but no tx has been signed yet). MigrateAuth must reuse the
// existing account (no NewAccountWithAddress call), set the multisig pubkey on
// it, and persist via SetAccount. Distinct from:
//   - SetsPubKey (fresh account case, NewAccountWithAddress IS called)
//   - PreExistingMatchingPubKey_Idempotent (pubkey already matches, SetPubKey skipped)
func TestMigrateAuth_MultisigDestination_PreExistingFundedButNeverSigned_SetsPubKey(t *testing.T) {
	f := initMockFixture(t)
	legacy := testAccAddr()

	// threshold=2 must match deriveMultisigAddr so MigrateAuth's address check passes.
	proof, subKeys := makeMultisigProof(t, 2, 2)
	newAddr := deriveMultisigAddr(subKeys, 2)

	// Pre-existing account with NIL pubkey (funded but never signed).
	existingAcc := authtypes.NewBaseAccountWithAddress(newAddr)
	require.Nil(t, existingAcc.GetPubKey(), "fixture precondition: pubkey must start nil")

	legacyAcc := authtypes.NewBaseAccountWithAddress(legacy)
	multiPK := kmultisig.NewLegacyAminoPubKey(2, subKeys)

	// Phase 1 probe: returns existing nil-pubkey account.
	// This is the ONE AND ONLY GetAccount(newAddr) call.
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), newAddr).Return(existingAcc)
	// Phase 2: legacy fetch + removal. NO NewAccountWithAddress — we reuse existingAcc.
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), legacy).Return(legacyAcc)
	f.accountKeeper.EXPECT().RemoveAccount(gomock.Any(), legacyAcc)
	// SetAccount must receive the existing account with the multisig pubkey set on it.
	f.accountKeeper.EXPECT().SetAccount(gomock.Any(), existingAcc).Do(func(_ any, acc sdk.AccountI) {
		require.NotNil(t, acc.GetPubKey(), "SetPubKey must have fired for the nil-pubkey funded-but-never-signed case")
		require.Equal(t, multiPK.Bytes(), acc.GetPubKey().Bytes(),
			"persisted pubkey must match the reconstructed multisig")
	})

	vi, err := f.keeper.MigrateAuth(f.ctx, legacy, newAddr, proof)
	require.NoError(t, err)
	require.Nil(t, vi)
}

// TestMigrateAuth_MultisigDestination_MalformedDestProof is a table-driven test
// covering 4 malformed destProof cases. Each must be rejected in Phase 1 with
// NO accountKeeper calls at all (strict gomock catches any leak).
func TestMigrateAuth_MultisigDestination_MalformedDestProof(t *testing.T) {
	priv1, _ := ethsecp256k1.GenerateKey()
	priv2, _ := ethsecp256k1.GenerateKey()
	goodKey1 := priv1.PubKey().Bytes()
	goodKey2 := priv2.PubKey().Bytes()

	cases := []struct {
		name  string
		proof *types.MigrationProof
	}{
		{
			name: "threshold_zero",
			proof: &types.MigrationProof{
				Proof: &types.MigrationProof_Multisig{
					Multisig: &types.MultisigProof{
						Threshold:     0,
						SubPubKeys:    [][]byte{goodKey1, goodKey2},
						SignerIndices: []uint32{},
						SubSignatures: [][]byte{},
						SigFormat:     types.SigFormat_SIG_FORMAT_CLI,
					},
				},
			},
		},
		{
			name: "threshold_exceeds_n",
			proof: &types.MigrationProof{
				Proof: &types.MigrationProof_Multisig{
					Multisig: &types.MultisigProof{
						Threshold:     3,
						SubPubKeys:    [][]byte{goodKey1, goodKey2},
						SignerIndices: []uint32{0, 1, 0}, // wrong count but threshold=3 > N=2
						SubSignatures: [][]byte{make([]byte, 65), make([]byte, 65), make([]byte, 65)},
						SigFormat:     types.SigFormat_SIG_FORMAT_CLI,
					},
				},
			},
		},
		{
			name: "sub_pub_key_wrong_length",
			proof: &types.MigrationProof{
				Proof: &types.MigrationProof_Multisig{
					Multisig: &types.MultisigProof{
						Threshold:     1,
						SubPubKeys:    [][]byte{{0x01, 0x02}}, // wrong length
						SignerIndices: []uint32{0},
						SubSignatures: [][]byte{make([]byte, 65)},
						SigFormat:     types.SigFormat_SIG_FORMAT_CLI,
					},
				},
			},
		},
		{
			name: "multisig_with_eip191_format",
			proof: &types.MigrationProof{
				Proof: &types.MigrationProof_Multisig{
					Multisig: &types.MultisigProof{
						Threshold:     1,
						SubPubKeys:    [][]byte{goodKey1},
						SignerIndices: []uint32{0},
						SubSignatures: [][]byte{make([]byte, 65)},
						SigFormat:     types.SigFormat_SIG_FORMAT_EIP191,
					},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := initMockFixture(t)
			// strict gomock: any unexpected call fails the test.
			// No EXPECT() calls registered — proof rejection must happen
			// before any keeper interaction.

			_, err := f.keeper.MigrateAuth(f.ctx, testAccAddr(), testAccAddr(), tc.proof)
			require.Error(t, err, "expected rejection for case %s", tc.name)
		})
	}
}

// --- FinalizeVestingAccount tests ---

// TestFinalizeVestingAccount_Continuous verifies that a ContinuousVestingAccount
// is correctly recreated at the new address from VestingInfo.
func TestFinalizeVestingAccount_Continuous(t *testing.T) {
	f := initMockFixture(t)
	newAddr := testAccAddr()
	baseAcc := authtypes.NewBaseAccountWithAddress(newAddr)

	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), newAddr).Return(baseAcc)
	f.accountKeeper.EXPECT().SetAccount(gomock.Any(), gomock.Any())

	vi := &keeper.VestingInfo{
		Type:            keeper.VestingTypeContinuous,
		OriginalVesting: sdk.NewCoins(sdk.NewInt64Coin("ulume", 1000)),
		EndTime:         1000000,
		StartTime:       500000,
	}

	err := f.keeper.FinalizeVestingAccount(f.ctx, newAddr, vi)
	require.NoError(t, err)
}

// TestFinalizeVestingAccount_PreservesDelegatedBalances verifies delegated
// vesting/free balances are preserved for all vesting account types.
func TestFinalizeVestingAccount_PreservesDelegatedBalances(t *testing.T) {
	testCases := []struct {
		name string
		vi   *keeper.VestingInfo
	}{
		{
			name: "continuous",
			vi: &keeper.VestingInfo{
				Type:             keeper.VestingTypeContinuous,
				OriginalVesting:  sdk.NewCoins(sdk.NewInt64Coin("ulume", 1000)),
				DelegatedFree:    sdk.NewCoins(sdk.NewInt64Coin("ulume", 11)),
				DelegatedVesting: sdk.NewCoins(sdk.NewInt64Coin("ulume", 22)),
				EndTime:          1000000,
				StartTime:        500000,
			},
		},
		{
			name: "delayed",
			vi: &keeper.VestingInfo{
				Type:             keeper.VestingTypeDelayed,
				OriginalVesting:  sdk.NewCoins(sdk.NewInt64Coin("ulume", 1000)),
				DelegatedFree:    sdk.NewCoins(sdk.NewInt64Coin("ulume", 33)),
				DelegatedVesting: sdk.NewCoins(sdk.NewInt64Coin("ulume", 44)),
				EndTime:          1000000,
			},
		},
		{
			name: "periodic",
			vi: &keeper.VestingInfo{
				Type:             keeper.VestingTypePeriodic,
				OriginalVesting:  sdk.NewCoins(sdk.NewInt64Coin("ulume", 1000)),
				DelegatedFree:    sdk.NewCoins(sdk.NewInt64Coin("ulume", 55)),
				DelegatedVesting: sdk.NewCoins(sdk.NewInt64Coin("ulume", 66)),
				EndTime:          3000000,
				StartTime:        1000000,
				Periods: vestingtypes.Periods{
					{Length: 100000, Amount: sdk.NewCoins(sdk.NewInt64Coin("ulume", 500))},
					{Length: 200000, Amount: sdk.NewCoins(sdk.NewInt64Coin("ulume", 500))},
				},
			},
		},
		{
			name: "permanent_locked",
			vi: &keeper.VestingInfo{
				Type:             keeper.VestingTypePermanentLocked,
				OriginalVesting:  sdk.NewCoins(sdk.NewInt64Coin("ulume", 1000)),
				DelegatedFree:    sdk.NewCoins(sdk.NewInt64Coin("ulume", 77)),
				DelegatedVesting: sdk.NewCoins(sdk.NewInt64Coin("ulume", 88)),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			f := initMockFixture(t)
			newAddr := testAccAddr()
			baseAcc := authtypes.NewBaseAccountWithAddress(newAddr)

			f.accountKeeper.EXPECT().GetAccount(gomock.Any(), newAddr).Return(baseAcc)
			f.accountKeeper.EXPECT().SetAccount(gomock.Any(), gomock.Any()).Do(func(_ any, acc sdk.AccountI) {
				switch va := acc.(type) {
				case *vestingtypes.ContinuousVestingAccount:
					require.Equal(t, tc.vi.DelegatedFree, va.DelegatedFree)
					require.Equal(t, tc.vi.DelegatedVesting, va.DelegatedVesting)
				case *vestingtypes.DelayedVestingAccount:
					require.Equal(t, tc.vi.DelegatedFree, va.DelegatedFree)
					require.Equal(t, tc.vi.DelegatedVesting, va.DelegatedVesting)
				case *vestingtypes.PeriodicVestingAccount:
					require.Equal(t, tc.vi.DelegatedFree, va.DelegatedFree)
					require.Equal(t, tc.vi.DelegatedVesting, va.DelegatedVesting)
				case *vestingtypes.PermanentLockedAccount:
					require.Equal(t, tc.vi.DelegatedFree, va.DelegatedFree)
					require.Equal(t, tc.vi.DelegatedVesting, va.DelegatedVesting)
				default:
					t.Fatalf("unexpected vesting account type: %T", acc)
				}
			})

			err := f.keeper.FinalizeVestingAccount(f.ctx, newAddr, tc.vi)
			require.NoError(t, err)
		})
	}
}

// TestFinalizeVestingAccount_AccountNotFound verifies error when the new account
// does not exist at finalization time.
func TestFinalizeVestingAccount_AccountNotFound(t *testing.T) {
	f := initMockFixture(t)
	newAddr := testAccAddr()

	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), newAddr).Return(nil)

	vi := &keeper.VestingInfo{
		Type:            keeper.VestingTypeContinuous,
		OriginalVesting: sdk.NewCoins(sdk.NewInt64Coin("ulume", 1000)),
		EndTime:         1000000,
		StartTime:       500000,
	}

	err := f.keeper.FinalizeVestingAccount(f.ctx, newAddr, vi)
	require.Error(t, err)
}

// --- MigrateBank tests ---

// TestMigrateBank_WithBalance verifies that all balances are transferred via SendCoins.
func TestMigrateBank_WithBalance(t *testing.T) {
	f := initMockFixture(t)
	legacy := testAccAddr()
	newAddr := testAccAddr()
	balances := sdk.NewCoins(sdk.NewInt64Coin("ulume", 1000))

	f.bankKeeper.EXPECT().GetAllBalances(gomock.Any(), legacy).Return(balances)
	f.bankKeeper.EXPECT().SendCoins(gomock.Any(), legacy, newAddr, balances).Return(nil)

	err := f.keeper.MigrateBank(f.ctx, legacy, newAddr)
	require.NoError(t, err)
}

// TestMigrateBank_ZeroBalance verifies that SendCoins is not called when balance is zero.
func TestMigrateBank_ZeroBalance(t *testing.T) {
	f := initMockFixture(t)
	legacy := testAccAddr()
	newAddr := testAccAddr()

	f.bankKeeper.EXPECT().GetAllBalances(gomock.Any(), legacy).Return(sdk.Coins{})

	// SendCoins should NOT be called when balance is zero.
	err := f.keeper.MigrateBank(f.ctx, legacy, newAddr)
	require.NoError(t, err)
}

// TestMigrateBank_MultiDenom verifies that multi-denom balances are transferred correctly.
func TestMigrateBank_MultiDenom(t *testing.T) {
	f := initMockFixture(t)
	legacy := testAccAddr()
	newAddr := testAccAddr()
	balances := sdk.NewCoins(
		sdk.NewInt64Coin("ulume", 500),
		sdk.NewInt64Coin("uatom", 200),
	)

	f.bankKeeper.EXPECT().GetAllBalances(gomock.Any(), legacy).Return(balances)
	f.bankKeeper.EXPECT().SendCoins(gomock.Any(), legacy, newAddr, balances).Return(nil)

	err := f.keeper.MigrateBank(f.ctx, legacy, newAddr)
	require.NoError(t, err)
}

// --- MigrateDistribution tests ---

// TestMigrateDistribution_WithDelegations verifies that pending rewards are
// withdrawn for all delegations.
func TestMigrateDistribution_WithDelegations(t *testing.T) {
	f := initMockFixture(t)
	legacy := testAccAddr()
	valAddr := sdk.ValAddress(testAccAddr())

	delegations := []stakingtypes.Delegation{
		stakingtypes.NewDelegation(legacy.String(), valAddr.String(), math.LegacyNewDec(100)),
	}

	// redirectWithdrawAddrIfMigrated: withdraw addr returns self — no redirect needed.
	f.distributionKeeper.EXPECT().GetDelegatorWithdrawAddr(gomock.Any(), legacy).Return(legacy, nil)

	f.stakingKeeper.EXPECT().GetDelegatorDelegations(gomock.Any(), legacy, ^uint16(0)).Return(delegations, nil)
	f.distributionKeeper.EXPECT().GetDelegatorStartingInfo(gomock.Any(), valAddr, legacy).Return(
		distrtypes.DelegatorStartingInfo{PreviousPeriod: 4}, nil,
	)
	expectHistoricalRewardsLookup(f.distributionKeeper, valAddr, 4, 1)
	f.distributionKeeper.EXPECT().WithdrawDelegationRewards(gomock.Any(), legacy, valAddr).Return(sdk.Coins{}, nil)

	err := f.keeper.MigrateDistribution(f.ctx, legacy)
	require.NoError(t, err)
}

// TestMigrateDistribution_NoDelegations verifies no-op when there are no delegations.
func TestMigrateDistribution_NoDelegations(t *testing.T) {
	f := initMockFixture(t)
	legacy := testAccAddr()

	// redirectWithdrawAddrIfMigrated: withdraw addr returns self — no redirect needed.
	f.distributionKeeper.EXPECT().GetDelegatorWithdrawAddr(gomock.Any(), legacy).Return(legacy, nil)

	f.stakingKeeper.EXPECT().GetDelegatorDelegations(gomock.Any(), legacy, ^uint16(0)).Return(nil, nil)

	err := f.keeper.MigrateDistribution(f.ctx, legacy)
	require.NoError(t, err)
}

// --- MigrateAuthz tests ---

// TestMigrateAuthz_AsGranter verifies that grants where legacy is the granter
// are re-keyed to the new address.
func TestMigrateAuthz_AsGranter(t *testing.T) {
	f := initMockFixture(t)
	legacy := testAccAddr()
	newAddr := testAccAddr()
	grantee := testAccAddr()

	genericAuth := authz.NewGenericAuthorization("/cosmos.bank.v1beta1.MsgSend")
	grant, err := authz.NewGrant(f.ctx.BlockTime(), genericAuth, nil)
	require.NoError(t, err)

	f.authzKeeper.EXPECT().IterateGrants(gomock.Any(), gomock.Any()).
		Do(func(_ any, cb func(sdk.AccAddress, sdk.AccAddress, authz.Grant) bool) {
			cb(legacy, grantee, grant)
		})
	f.authzKeeper.EXPECT().DeleteGrant(gomock.Any(), grantee, legacy, "/cosmos.bank.v1beta1.MsgSend").Return(nil)
	f.authzKeeper.EXPECT().SaveGrant(gomock.Any(), grantee, newAddr, genericAuth, grant.Expiration).Return(nil)

	err = f.keeper.MigrateAuthz(f.ctx, legacy, newAddr)
	require.NoError(t, err)
}

// TestMigrateAuthz_AsGrantee verifies that grants where legacy is the grantee
// are re-keyed to the new address.
func TestMigrateAuthz_AsGrantee(t *testing.T) {
	f := initMockFixture(t)
	legacy := testAccAddr()
	newAddr := testAccAddr()
	granter := testAccAddr()

	genericAuth := authz.NewGenericAuthorization("/cosmos.bank.v1beta1.MsgSend")
	grant, err := authz.NewGrant(f.ctx.BlockTime(), genericAuth, nil)
	require.NoError(t, err)

	f.authzKeeper.EXPECT().IterateGrants(gomock.Any(), gomock.Any()).
		Do(func(_ any, cb func(sdk.AccAddress, sdk.AccAddress, authz.Grant) bool) {
			cb(granter, legacy, grant)
		})
	f.authzKeeper.EXPECT().DeleteGrant(gomock.Any(), legacy, granter, "/cosmos.bank.v1beta1.MsgSend").Return(nil)
	f.authzKeeper.EXPECT().SaveGrant(gomock.Any(), newAddr, granter, genericAuth, grant.Expiration).Return(nil)

	err = f.keeper.MigrateAuthz(f.ctx, legacy, newAddr)
	require.NoError(t, err)
}

// TestMigrateAuthz_NoGrants verifies no-op when there are no authz grants.
func TestMigrateAuthz_NoGrants(t *testing.T) {
	f := initMockFixture(t)
	legacy := testAccAddr()
	newAddr := testAccAddr()

	f.authzKeeper.EXPECT().IterateGrants(gomock.Any(), gomock.Any())

	err := f.keeper.MigrateAuthz(f.ctx, legacy, newAddr)
	require.NoError(t, err)
}

// --- MigrateFeegrant tests ---

// TestMigrateFeegrant_AsGranter verifies that fee allowances where legacy is the
// granter are re-created at the new address.
func TestMigrateFeegrant_AsGranter(t *testing.T) {
	f := initMockFixture(t)
	legacy := testAccAddr()
	newAddr := testAccAddr()
	grantee := testAccAddr()

	allowance := &feegrant.BasicAllowance{SpendLimit: sdk.NewCoins(sdk.NewInt64Coin("ulume", 100))}
	grant, err := feegrant.NewGrant(legacy, grantee, allowance)
	require.NoError(t, err)

	f.feegrantKeeper.EXPECT().IterateAllFeeAllowances(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, cb func(feegrant.Grant) bool) error {
			cb(grant)
			return nil
		})
	f.feegrantKeeper.EXPECT().GrantAllowance(gomock.Any(), newAddr, grantee, allowance).Return(nil)

	err = f.keeper.MigrateFeegrant(f.ctx, legacy, newAddr)
	require.NoError(t, err)
}

// TestMigrateFeegrant_NoAllowances verifies no-op when there are no fee allowances.
func TestMigrateFeegrant_NoAllowances(t *testing.T) {
	f := initMockFixture(t)
	legacy := testAccAddr()
	newAddr := testAccAddr()

	f.feegrantKeeper.EXPECT().IterateAllFeeAllowances(gomock.Any(), gomock.Any()).Return(nil)

	err := f.keeper.MigrateFeegrant(f.ctx, legacy, newAddr)
	require.NoError(t, err)
}

// --- MigrateSupernode tests ---

// TestMigrateSupernode_Found verifies that the supernode account field is updated
// from legacy to new address and PrevSupernodeAccounts history is maintained.
func TestMigrateSupernode_Found(t *testing.T) {
	f := initMockFixture(t)
	legacy := testAccAddr()
	newAddr := testAccAddr()

	sn := sntypes.SuperNode{
		SupernodeAccount: legacy.String(),
		ValidatorAddress: sdk.ValAddress(legacy).String(),
		PrevSupernodeAccounts: []*sntypes.SupernodeAccountHistory{
			{Account: legacy.String(), Height: 1},
		},
	}

	f.supernodeKeeper.EXPECT().GetSuperNodeByAccount(gomock.Any(), legacy.String()).Return(sn, true, nil)
	f.supernodeKeeper.EXPECT().SetSuperNode(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, updated sntypes.SuperNode) error {
			require.Equal(t, newAddr.String(), updated.SupernodeAccount)
			// Existing legacy entry should be rewritten to new address.
			require.Len(t, updated.PrevSupernodeAccounts, 2)
			require.Equal(t, newAddr.String(), updated.PrevSupernodeAccounts[0].Account)
			require.Equal(t, int64(1), updated.PrevSupernodeAccounts[0].Height)
			// New migration entry appended.
			require.Equal(t, newAddr.String(), updated.PrevSupernodeAccounts[1].Account)
			return nil
		})

	err := f.keeper.MigrateSupernode(f.ctx, legacy, newAddr)
	require.NoError(t, err)
}

// TestMigrateSupernode_NotFound verifies no-op when legacy is not a supernode.
func TestMigrateSupernode_NotFound(t *testing.T) {
	f := initMockFixture(t)
	legacy := testAccAddr()
	newAddr := testAccAddr()

	f.supernodeKeeper.EXPECT().GetSuperNodeByAccount(gomock.Any(), legacy.String()).Return(sntypes.SuperNode{}, false, nil)

	err := f.keeper.MigrateSupernode(f.ctx, legacy, newAddr)
	require.NoError(t, err)
}

// --- MigrateActions tests ---

// TestMigrateActions_CreatorAndSuperNodes verifies that when the legacy address
// is both the creator and a supernode of the same action, the action is updated
// exactly once (deduped across the creator and supernode indexes) with both the
// Creator field and the matching SuperNodes entry re-keyed to the new address.
func TestMigrateActions_CreatorAndSuperNodes(t *testing.T) {
	f := initMockFixture(t)
	legacy := testAccAddr()
	newAddr := testAccAddr()
	otherAddr := testAccAddr()

	// Each index lookup resolves an independent copy of the same action, mirroring
	// the real keeper which unmarshals a fresh Action on every GetActionByID.
	byCreator := &actiontypes.Action{
		ActionID:   "action-1",
		Creator:    legacy.String(),
		SuperNodes: []string{legacy.String(), otherAddr.String()},
	}
	bySuperNode := &actiontypes.Action{
		ActionID:   "action-1",
		Creator:    legacy.String(),
		SuperNodes: []string{legacy.String(), otherAddr.String()},
	}

	f.actionKeeper.EXPECT().GetActionsByCreator(gomock.Any(), legacy.String()).
		Return([]*actiontypes.Action{byCreator}, nil)
	f.actionKeeper.EXPECT().GetActionsBySuperNode(gomock.Any(), legacy.String()).
		Return([]*actiontypes.Action{bySuperNode}, nil)
	f.actionKeeper.EXPECT().SetAction(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, updated *actiontypes.Action) error {
			require.Equal(t, "action-1", updated.ActionID)
			require.Equal(t, newAddr.String(), updated.Creator)
			require.Equal(t, newAddr.String(), updated.SuperNodes[0])
			require.Equal(t, otherAddr.String(), updated.SuperNodes[1])
			return nil
		}).Times(1)

	err := f.keeper.MigrateActions(f.ctx, legacy, newAddr)
	require.NoError(t, err)
}

// TestMigrateActions_SuperNodeOnly verifies that an action where the legacy
// address appears only as a supernode (with a different creator) has just its
// SuperNodes entry re-keyed, leaving the Creator field untouched.
func TestMigrateActions_SuperNodeOnly(t *testing.T) {
	f := initMockFixture(t)
	legacy := testAccAddr()
	newAddr := testAccAddr()
	creator := testAccAddr()

	action := &actiontypes.Action{
		ActionID:   "action-2",
		Creator:    creator.String(),
		SuperNodes: []string{legacy.String()},
	}

	f.actionKeeper.EXPECT().GetActionsByCreator(gomock.Any(), legacy.String()).Return(nil, nil)
	f.actionKeeper.EXPECT().GetActionsBySuperNode(gomock.Any(), legacy.String()).
		Return([]*actiontypes.Action{action}, nil)
	f.actionKeeper.EXPECT().SetAction(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, updated *actiontypes.Action) error {
			require.Equal(t, creator.String(), updated.Creator)
			require.Equal(t, newAddr.String(), updated.SuperNodes[0])
			return nil
		}).Times(1)

	err := f.keeper.MigrateActions(f.ctx, legacy, newAddr)
	require.NoError(t, err)
}

// TestMigrateActions_NoMatch verifies no-op when no actions reference legacy address.
func TestMigrateActions_NoMatch(t *testing.T) {
	f := initMockFixture(t)
	legacy := testAccAddr()
	newAddr := testAccAddr()

	f.actionKeeper.EXPECT().GetActionsByCreator(gomock.Any(), legacy.String()).Return(nil, nil)
	f.actionKeeper.EXPECT().GetActionsBySuperNode(gomock.Any(), legacy.String()).Return(nil, nil)
	// No SetAction expected: nothing references the legacy address.

	err := f.keeper.MigrateActions(f.ctx, legacy, newAddr)
	require.NoError(t, err)
}

// --- MigrateStaking tests ---

// TestMigrateStaking_ActiveDelegations verifies the full staking migration flow:
// active delegation re-keying, distribution starting info, and withdraw address.
func TestMigrateStaking_ActiveDelegations(t *testing.T) {
	f := initMockFixture(t)
	legacy := testAccAddr()
	newAddr := testAccAddr()
	valAddr := sdk.ValAddress(testAccAddr())

	del := stakingtypes.NewDelegation(legacy.String(), valAddr.String(), math.LegacyNewDec(100))

	// migrateActiveDelegations
	f.stakingKeeper.EXPECT().GetDelegatorDelegations(gomock.Any(), legacy, ^uint16(0)).Return([]stakingtypes.Delegation{del}, nil)
	f.distributionKeeper.EXPECT().DeleteDelegatorStartingInfo(gomock.Any(), valAddr, legacy).Return(nil)
	f.stakingKeeper.EXPECT().RemoveDelegation(gomock.Any(), del).Return(nil)
	f.stakingKeeper.EXPECT().SetDelegation(gomock.Any(), gomock.Any()).Return(nil)
	f.distributionKeeper.EXPECT().GetValidatorCurrentRewards(gomock.Any(), valAddr).Return(distrtypes.ValidatorCurrentRewards{Period: 5}, nil)
	expectHistoricalRewardsIncrement(f.distributionKeeper, valAddr, 4, 1)
	// migrateActiveDelegations fetches the validator to convert shares → tokens (rate 1.0).
	f.stakingKeeper.EXPECT().GetValidator(gomock.Any(), valAddr).Return(
		stakingtypes.Validator{OperatorAddress: valAddr.String(), Tokens: math.NewInt(100), DelegatorShares: math.LegacyNewDec(100)}, nil,
	)
	f.distributionKeeper.EXPECT().SetDelegatorStartingInfo(gomock.Any(), valAddr, newAddr, gomock.Any()).Return(nil)

	// migrateUnbondingDelegations
	f.stakingKeeper.EXPECT().GetUnbondingDelegations(gomock.Any(), legacy, ^uint16(0)).Return(nil, nil)

	// migrateRedelegations
	f.stakingKeeper.EXPECT().GetRedelegations(gomock.Any(), legacy, ^uint16(0)).Return(nil, nil)

	// migrateWithdrawAddress — origWithdrawAddr is legacy (self).
	f.distributionKeeper.EXPECT().SetDelegatorWithdrawAddr(gomock.Any(), newAddr, newAddr).Return(nil)

	err := f.keeper.MigrateStaking(f.ctx, legacy, newAddr, legacy)
	require.NoError(t, err)
}

// TestMigrateStaking_SlashedValidatorStoresTokensNotShares verifies the account
// migration path stores DelegatorStartingInfo.Stake as tokens
// (val.TokensFromSharesTruncated(shares)), not raw shares — the same SDK
// invariant as the validator path. For a delegation to a slashed validator
// (exchange rate < 1) storing shares would panic the reward math on the
// delegator's next withdraw/undelegate/redelegate.
func TestMigrateStaking_SlashedValidatorStoresTokensNotShares(t *testing.T) {
	f := initMockFixture(t)
	legacy := testAccAddr()
	newAddr := testAccAddr()
	valAddr := sdk.ValAddress(testAccAddr())

	del := stakingtypes.NewDelegation(legacy.String(), valAddr.String(), math.LegacyNewDec(100))

	// Slashed validator: 90 tokens / 100 shares → TokensFromSharesTruncated(100) = 90.
	slashedVal := stakingtypes.Validator{
		OperatorAddress: valAddr.String(),
		Tokens:          math.NewInt(90),
		DelegatorShares: math.LegacyNewDec(100),
	}
	expectedStake := slashedVal.TokensFromSharesTruncated(del.Shares)

	// migrateActiveDelegations
	f.stakingKeeper.EXPECT().GetDelegatorDelegations(gomock.Any(), legacy, ^uint16(0)).Return([]stakingtypes.Delegation{del}, nil)
	f.distributionKeeper.EXPECT().DeleteDelegatorStartingInfo(gomock.Any(), valAddr, legacy).Return(nil)
	f.stakingKeeper.EXPECT().RemoveDelegation(gomock.Any(), del).Return(nil)
	f.stakingKeeper.EXPECT().SetDelegation(gomock.Any(), gomock.Any()).Return(nil)
	f.distributionKeeper.EXPECT().GetValidatorCurrentRewards(gomock.Any(), valAddr).Return(distrtypes.ValidatorCurrentRewards{Period: 5}, nil)
	expectHistoricalRewardsIncrement(f.distributionKeeper, valAddr, 4, 1)
	// The validator must be fetched to convert shares → tokens.
	f.stakingKeeper.EXPECT().GetValidator(gomock.Any(), valAddr).Return(slashedVal, nil)

	var capturedStake math.LegacyDec
	f.distributionKeeper.EXPECT().SetDelegatorStartingInfo(gomock.Any(), valAddr, newAddr, gomock.Any()).DoAndReturn(
		func(_ sdk.Context, _ sdk.ValAddress, _ sdk.AccAddress, info distrtypes.DelegatorStartingInfo) error {
			capturedStake = info.Stake
			return nil
		},
	)

	// migrateUnbondingDelegations / migrateRedelegations — none.
	f.stakingKeeper.EXPECT().GetUnbondingDelegations(gomock.Any(), legacy, ^uint16(0)).Return(nil, nil)
	f.stakingKeeper.EXPECT().GetRedelegations(gomock.Any(), legacy, ^uint16(0)).Return(nil, nil)

	// migrateWithdrawAddress — origWithdrawAddr is legacy (self).
	f.distributionKeeper.EXPECT().SetDelegatorWithdrawAddr(gomock.Any(), newAddr, newAddr).Return(nil)

	err := f.keeper.MigrateStaking(f.ctx, legacy, newAddr, legacy)
	require.NoError(t, err)
	require.Equal(t, expectedStake, capturedStake)
	require.True(t, capturedStake.LT(del.Shares), "stake must be tokens (< shares) for a slashed validator")
}

// TestMigrateStaking_NoDelegations verifies no-op when delegator has no delegations.
func TestMigrateStaking_NoDelegations(t *testing.T) {
	f := initMockFixture(t)
	legacy := testAccAddr()
	newAddr := testAccAddr()

	// migrateActiveDelegations — no delegations.
	f.stakingKeeper.EXPECT().GetDelegatorDelegations(gomock.Any(), legacy, ^uint16(0)).Return(nil, nil)
	f.stakingKeeper.EXPECT().GetUnbondingDelegations(gomock.Any(), legacy, ^uint16(0)).Return(nil, nil)
	f.stakingKeeper.EXPECT().GetRedelegations(gomock.Any(), legacy, ^uint16(0)).Return(nil, nil)
	// migrateWithdrawAddress — origWithdrawAddr is nil (not set).
	f.distributionKeeper.EXPECT().SetDelegatorWithdrawAddr(gomock.Any(), newAddr, newAddr).Return(nil)

	err := f.keeper.MigrateStaking(f.ctx, legacy, newAddr, nil)
	require.NoError(t, err)
}

// TestMigrateStaking_ThirdPartyWithdrawAddress verifies that a third-party
// withdraw address is preserved (not replaced with newAddr).
func TestMigrateStaking_ThirdPartyWithdrawAddress(t *testing.T) {
	f := initMockFixture(t)
	legacy := testAccAddr()
	newAddr := testAccAddr()
	thirdParty := testAccAddr()

	f.stakingKeeper.EXPECT().GetDelegatorDelegations(gomock.Any(), legacy, ^uint16(0)).Return(nil, nil)
	f.stakingKeeper.EXPECT().GetUnbondingDelegations(gomock.Any(), legacy, ^uint16(0)).Return(nil, nil)
	f.stakingKeeper.EXPECT().GetRedelegations(gomock.Any(), legacy, ^uint16(0)).Return(nil, nil)
	f.distributionKeeper.EXPECT().SetDelegatorWithdrawAddr(gomock.Any(), newAddr, thirdParty).Return(nil)

	err := f.keeper.MigrateStaking(f.ctx, legacy, newAddr, thirdParty)
	require.NoError(t, err)
}

// TestMigrateStaking_MigratedThirdPartyWithdrawAddress verifies that when the
// third-party withdraw address has already been migrated, the withdraw address
// is resolved to that party's new (migrated) address via MigrationRecords.
// This is the bug-16 regression test.
func TestMigrateStaking_MigratedThirdPartyWithdrawAddress(t *testing.T) {
	f := initMockFixture(t)
	legacy := testAccAddr()
	newAddr := testAccAddr()
	thirdPartyLegacy := testAccAddr()
	thirdPartyNew := testAccAddr()

	// Seed a migration record for the third-party address — it was migrated earlier.
	require.NoError(t, f.keeper.MigrationRecords.Set(f.ctx, thirdPartyLegacy.String(), types.MigrationRecord{
		LegacyAddress: thirdPartyLegacy.String(),
		NewAddress:    thirdPartyNew.String(),
	}))

	f.stakingKeeper.EXPECT().GetDelegatorDelegations(gomock.Any(), legacy, ^uint16(0)).Return(nil, nil)
	f.stakingKeeper.EXPECT().GetUnbondingDelegations(gomock.Any(), legacy, ^uint16(0)).Return(nil, nil)
	f.stakingKeeper.EXPECT().GetRedelegations(gomock.Any(), legacy, ^uint16(0)).Return(nil, nil)
	// The withdraw address must be resolved to thirdPartyNew, not thirdPartyLegacy.
	f.distributionKeeper.EXPECT().SetDelegatorWithdrawAddr(gomock.Any(), newAddr, thirdPartyNew).Return(nil)

	err := f.keeper.MigrateStaking(f.ctx, legacy, newAddr, thirdPartyLegacy)
	require.NoError(t, err)
}

// --- Unbonding delegation re-key tests ---

// TestMigrateStaking_WithUnbondingDelegation verifies that unbonding delegations
// are re-keyed from legacy to new address, including queue and UnbondingId indexes.
func TestMigrateStaking_WithUnbondingDelegation(t *testing.T) {
	f := initMockFixture(t)
	legacy := testAccAddr()
	newAddr := testAccAddr()
	valAddr := sdk.ValAddress(testAccAddr())

	del := stakingtypes.NewDelegation(legacy.String(), valAddr.String(), math.LegacyNewDec(100))
	completionTime := f.ctx.BlockTime().Add(21 * 24 * 3600 * 1e9) // 21 days
	ubd := stakingtypes.UnbondingDelegation{
		DelegatorAddress: legacy.String(),
		ValidatorAddress: valAddr.String(),
		Entries: []stakingtypes.UnbondingDelegationEntry{
			{
				CreationHeight: 10,
				CompletionTime: completionTime,
				InitialBalance: math.NewInt(50),
				Balance:        math.NewInt(50),
				UnbondingId:    42,
			},
		},
	}

	// migrateActiveDelegations
	f.stakingKeeper.EXPECT().GetDelegatorDelegations(gomock.Any(), legacy, ^uint16(0)).Return([]stakingtypes.Delegation{del}, nil)
	f.distributionKeeper.EXPECT().DeleteDelegatorStartingInfo(gomock.Any(), valAddr, legacy).Return(nil)
	f.stakingKeeper.EXPECT().RemoveDelegation(gomock.Any(), del).Return(nil)
	f.stakingKeeper.EXPECT().SetDelegation(gomock.Any(), gomock.Any()).Return(nil)
	f.distributionKeeper.EXPECT().GetValidatorCurrentRewards(gomock.Any(), valAddr).Return(distrtypes.ValidatorCurrentRewards{Period: 5}, nil)
	expectHistoricalRewardsIncrement(f.distributionKeeper, valAddr, 4, 1)
	// migrateActiveDelegations fetches the validator to convert shares → tokens (rate 1.0).
	f.stakingKeeper.EXPECT().GetValidator(gomock.Any(), valAddr).Return(
		stakingtypes.Validator{OperatorAddress: valAddr.String(), Tokens: math.NewInt(100), DelegatorShares: math.LegacyNewDec(100)}, nil,
	)
	f.distributionKeeper.EXPECT().SetDelegatorStartingInfo(gomock.Any(), valAddr, newAddr, gomock.Any()).Return(nil)

	// migrateUnbondingDelegations
	f.stakingKeeper.EXPECT().GetUnbondingDelegations(gomock.Any(), legacy, ^uint16(0)).Return([]stakingtypes.UnbondingDelegation{ubd}, nil)
	f.stakingKeeper.EXPECT().RemoveUnbondingDelegation(gomock.Any(), ubd).Return(nil)
	f.stakingKeeper.EXPECT().SetUnbondingDelegation(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ any, newUbd stakingtypes.UnbondingDelegation) error {
			require.Equal(t, newAddr.String(), newUbd.DelegatorAddress)
			require.Equal(t, valAddr.String(), newUbd.ValidatorAddress)
			require.Len(t, newUbd.Entries, 1)
			return nil
		})
	f.stakingKeeper.EXPECT().InsertUBDQueue(gomock.Any(), gomock.Any(), completionTime).Return(nil)
	f.stakingKeeper.EXPECT().SetUnbondingDelegationByUnbondingID(gomock.Any(), gomock.Any(), uint64(42)).Return(nil)

	// migrateRedelegations
	f.stakingKeeper.EXPECT().GetRedelegations(gomock.Any(), legacy, ^uint16(0)).Return(nil, nil)

	// migrateWithdrawAddress — origWithdrawAddr is nil (not set).
	f.distributionKeeper.EXPECT().SetDelegatorWithdrawAddr(gomock.Any(), newAddr, newAddr).Return(nil)

	err := f.keeper.MigrateStaking(f.ctx, legacy, newAddr, nil)
	require.NoError(t, err)
}

// TestMigrateStaking_WithRedelegation verifies that redelegations are re-keyed
// from legacy to new address, including queue and UnbondingId indexes.
func TestMigrateStaking_WithRedelegation(t *testing.T) {
	f := initMockFixture(t)
	legacy := testAccAddr()
	newAddr := testAccAddr()
	srcValAddr := sdk.ValAddress(testAccAddr())
	dstValAddr := sdk.ValAddress(testAccAddr())

	del := stakingtypes.NewDelegation(legacy.String(), srcValAddr.String(), math.LegacyNewDec(100))
	completionTime := f.ctx.BlockTime().Add(21 * 24 * 3600 * 1e9)
	red := stakingtypes.Redelegation{
		DelegatorAddress:    legacy.String(),
		ValidatorSrcAddress: srcValAddr.String(),
		ValidatorDstAddress: dstValAddr.String(),
		Entries: []stakingtypes.RedelegationEntry{
			{
				CreationHeight: 10,
				CompletionTime: completionTime,
				InitialBalance: math.NewInt(30),
				SharesDst:      math.LegacyNewDec(30),
				UnbondingId:    99,
			},
		},
	}

	// migrateActiveDelegations
	f.stakingKeeper.EXPECT().GetDelegatorDelegations(gomock.Any(), legacy, ^uint16(0)).Return([]stakingtypes.Delegation{del}, nil)
	f.distributionKeeper.EXPECT().DeleteDelegatorStartingInfo(gomock.Any(), srcValAddr, legacy).Return(nil)
	f.stakingKeeper.EXPECT().RemoveDelegation(gomock.Any(), del).Return(nil)
	f.stakingKeeper.EXPECT().SetDelegation(gomock.Any(), gomock.Any()).Return(nil)
	f.distributionKeeper.EXPECT().GetValidatorCurrentRewards(gomock.Any(), srcValAddr).Return(distrtypes.ValidatorCurrentRewards{Period: 3}, nil)
	expectHistoricalRewardsIncrement(f.distributionKeeper, srcValAddr, 2, 1)
	// migrateActiveDelegations fetches the validator to convert shares → tokens (rate 1.0).
	f.stakingKeeper.EXPECT().GetValidator(gomock.Any(), srcValAddr).Return(
		stakingtypes.Validator{OperatorAddress: srcValAddr.String(), Tokens: math.NewInt(100), DelegatorShares: math.LegacyNewDec(100)}, nil,
	)
	f.distributionKeeper.EXPECT().SetDelegatorStartingInfo(gomock.Any(), srcValAddr, newAddr, gomock.Any()).Return(nil)

	// migrateUnbondingDelegations
	f.stakingKeeper.EXPECT().GetUnbondingDelegations(gomock.Any(), legacy, ^uint16(0)).Return(nil, nil)

	// migrateRedelegations
	f.stakingKeeper.EXPECT().GetRedelegations(gomock.Any(), legacy, ^uint16(0)).Return([]stakingtypes.Redelegation{red}, nil)
	f.stakingKeeper.EXPECT().RemoveRedelegation(gomock.Any(), red).Return(nil)
	f.stakingKeeper.EXPECT().SetRedelegation(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ any, newRed stakingtypes.Redelegation) error {
			require.Equal(t, newAddr.String(), newRed.DelegatorAddress)
			require.Equal(t, srcValAddr.String(), newRed.ValidatorSrcAddress)
			require.Equal(t, dstValAddr.String(), newRed.ValidatorDstAddress)
			require.Len(t, newRed.Entries, 1)
			return nil
		})
	f.stakingKeeper.EXPECT().InsertRedelegationQueue(gomock.Any(), gomock.Any(), completionTime).Return(nil)
	f.stakingKeeper.EXPECT().SetRedelegationByUnbondingID(gomock.Any(), gomock.Any(), uint64(99)).Return(nil)

	// migrateWithdrawAddress — origWithdrawAddr is nil (not set).
	f.distributionKeeper.EXPECT().SetDelegatorWithdrawAddr(gomock.Any(), newAddr, newAddr).Return(nil)

	err := f.keeper.MigrateStaking(f.ctx, legacy, newAddr, nil)
	require.NoError(t, err)
}

// TestMigrateStaking_UnbondingWithoutActiveDelegation verifies that unbonding
// delegations are still migrated when the delegator no longer has an active
// delegation to the validator.
func TestMigrateStaking_UnbondingWithoutActiveDelegation(t *testing.T) {
	f := initMockFixture(t)
	legacy := testAccAddr()
	newAddr := testAccAddr()
	valAddr := sdk.ValAddress(testAccAddr())

	completionTime := f.ctx.BlockTime().Add(21 * 24 * 3600 * 1e9)
	ubd := stakingtypes.UnbondingDelegation{
		DelegatorAddress: legacy.String(),
		ValidatorAddress: valAddr.String(),
		Entries: []stakingtypes.UnbondingDelegationEntry{
			{
				CreationHeight: 11,
				CompletionTime: completionTime,
				InitialBalance: math.NewInt(40),
				Balance:        math.NewInt(40),
				UnbondingId:    77,
			},
		},
	}

	// migrateActiveDelegations
	f.stakingKeeper.EXPECT().GetDelegatorDelegations(gomock.Any(), legacy, ^uint16(0)).Return(nil, nil)

	// migrateUnbondingDelegations
	f.stakingKeeper.EXPECT().GetUnbondingDelegations(gomock.Any(), legacy, ^uint16(0)).Return([]stakingtypes.UnbondingDelegation{ubd}, nil)
	f.stakingKeeper.EXPECT().RemoveUnbondingDelegation(gomock.Any(), ubd).Return(nil)
	f.stakingKeeper.EXPECT().SetUnbondingDelegation(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ any, newUbd stakingtypes.UnbondingDelegation) error {
			require.Equal(t, newAddr.String(), newUbd.DelegatorAddress)
			require.Equal(t, valAddr.String(), newUbd.ValidatorAddress)
			require.Len(t, newUbd.Entries, 1)
			return nil
		})
	f.stakingKeeper.EXPECT().InsertUBDQueue(gomock.Any(), gomock.Any(), completionTime).Return(nil)
	f.stakingKeeper.EXPECT().SetUnbondingDelegationByUnbondingID(gomock.Any(), gomock.Any(), uint64(77)).Return(nil)

	// migrateRedelegations
	f.stakingKeeper.EXPECT().GetRedelegations(gomock.Any(), legacy, ^uint16(0)).Return(nil, nil)

	// migrateWithdrawAddress — origWithdrawAddr is nil (not set).
	f.distributionKeeper.EXPECT().SetDelegatorWithdrawAddr(gomock.Any(), newAddr, newAddr).Return(nil)

	err := f.keeper.MigrateStaking(f.ctx, legacy, newAddr, nil)
	require.NoError(t, err)
}

// --- Validator-specific delegation re-key tests ---

// TestMigrateValidatorDelegations_WithUnbondingAndRedelegation verifies
// MigrateValidatorDelegations re-keys unbonding delegations and redelegations
// with UnbondingId indexes.
func TestMigrateValidatorDelegations_WithUnbondingAndRedelegation(t *testing.T) {
	f := initMockFixture(t)
	oldValAddr := sdk.ValAddress(testAccAddr())
	newValAddr := sdk.ValAddress(testAccAddr())
	delegator := testAccAddr()

	completionTime := f.ctx.BlockTime().Add(21 * 24 * 3600 * 1e9)

	// No active delegations (passed in directly, not fetched).

	// One unbonding delegation with an UnbondingId.
	ubd := stakingtypes.UnbondingDelegation{
		DelegatorAddress: delegator.String(),
		ValidatorAddress: oldValAddr.String(),
		Entries: []stakingtypes.UnbondingDelegationEntry{
			{
				CreationHeight: 5,
				CompletionTime: completionTime,
				InitialBalance: math.NewInt(100),
				Balance:        math.NewInt(100),
				UnbondingId:    77,
			},
		},
	}
	f.stakingKeeper.EXPECT().RemoveUnbondingDelegation(gomock.Any(), ubd).Return(nil)
	f.stakingKeeper.EXPECT().SetUnbondingDelegation(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ any, newUbd stakingtypes.UnbondingDelegation) error {
			require.Equal(t, newValAddr.String(), newUbd.ValidatorAddress)
			require.Equal(t, delegator.String(), newUbd.DelegatorAddress)
			return nil
		})
	f.stakingKeeper.EXPECT().InsertUBDQueue(gomock.Any(), gomock.Any(), completionTime).Return(nil)
	f.stakingKeeper.EXPECT().SetUnbondingDelegationByUnbondingID(gomock.Any(), gomock.Any(), uint64(77)).Return(nil)

	// Two redelegations with an UnbondingId: one where the migrated validator is
	// the source, and one where it is the destination.
	dstVal := sdk.ValAddress(testAccAddr())
	srcRed := stakingtypes.Redelegation{
		DelegatorAddress:    delegator.String(),
		ValidatorSrcAddress: oldValAddr.String(),
		ValidatorDstAddress: dstVal.String(),
		Entries: []stakingtypes.RedelegationEntry{
			{
				CreationHeight: 8,
				CompletionTime: completionTime,
				InitialBalance: math.NewInt(50),
				SharesDst:      math.LegacyNewDec(50),
				UnbondingId:    88,
			},
		},
	}
	srcVal := sdk.ValAddress(testAccAddr())
	dstRed := stakingtypes.Redelegation{
		DelegatorAddress:    delegator.String(),
		ValidatorSrcAddress: srcVal.String(),
		ValidatorDstAddress: oldValAddr.String(),
		Entries: []stakingtypes.RedelegationEntry{
			{
				CreationHeight: 9,
				CompletionTime: completionTime,
				InitialBalance: math.NewInt(75),
				SharesDst:      math.LegacyNewDec(75),
				UnbondingId:    89,
			},
		},
	}
	// Redelegations are discovered by an internal scan; this fixture leaves the
	// scoped store unwired, so the scan falls back to IterateRedelegations.
	f.stakingKeeper.EXPECT().IterateRedelegations(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ any, fn func(int64, stakingtypes.Redelegation) bool) error {
			require.False(t, fn(0, srcRed))
			require.False(t, fn(1, dstRed))
			return nil
		},
	)
	f.stakingKeeper.EXPECT().RemoveRedelegation(gomock.Any(), srcRed).Return(nil)
	f.stakingKeeper.EXPECT().SetRedelegation(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ any, newRed stakingtypes.Redelegation) error {
			require.Equal(t, newValAddr.String(), newRed.ValidatorSrcAddress)
			require.Equal(t, dstVal.String(), newRed.ValidatorDstAddress)
			return nil
		},
	)
	f.stakingKeeper.EXPECT().InsertRedelegationQueue(gomock.Any(), gomock.Any(), completionTime).Return(nil)
	f.stakingKeeper.EXPECT().SetRedelegationByUnbondingID(gomock.Any(), gomock.Any(), uint64(88)).Return(nil)
	f.stakingKeeper.EXPECT().RemoveRedelegation(gomock.Any(), dstRed).Return(nil)
	f.stakingKeeper.EXPECT().SetRedelegation(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ any, newRed stakingtypes.Redelegation) error {
			require.Equal(t, srcVal.String(), newRed.ValidatorSrcAddress)
			require.Equal(t, newValAddr.String(), newRed.ValidatorDstAddress)
			return nil
		},
	)
	f.stakingKeeper.EXPECT().InsertRedelegationQueue(gomock.Any(), gomock.Any(), completionTime).Return(nil)
	f.stakingKeeper.EXPECT().SetRedelegationByUnbondingID(gomock.Any(), gomock.Any(), uint64(89)).Return(nil)

	err := f.keeper.MigrateValidatorDelegations(
		f.ctx, oldValAddr, newValAddr,
		nil,
		[]stakingtypes.UnbondingDelegation{ubd},
		nil,
	)
	require.NoError(t, err)
}

func TestMigrateValidatorDelegations_UsesScopedRedelegationIndexes(t *testing.T) {
	f := initMockFixture(t)
	f.wireScopedMigrationStores()

	oldValAddr := sdk.ValAddress(testAccAddr())
	newValAddr := sdk.ValAddress(testAccAddr())
	delegator := testAccAddr()
	completionTime := f.ctx.BlockTime().Add(21 * 24 * 3600 * 1e9)

	dstVal := sdk.ValAddress(testAccAddr())
	srcRed := stakingtypes.Redelegation{
		DelegatorAddress:    delegator.String(),
		ValidatorSrcAddress: oldValAddr.String(),
		ValidatorDstAddress: dstVal.String(),
		Entries: []stakingtypes.RedelegationEntry{{
			CreationHeight: 8,
			CompletionTime: completionTime,
			InitialBalance: math.NewInt(50),
			SharesDst:      math.LegacyNewDec(50),
			UnbondingId:    88,
		}},
	}
	srcVal := sdk.ValAddress(testAccAddr())
	dstRed := stakingtypes.Redelegation{
		DelegatorAddress:    delegator.String(),
		ValidatorSrcAddress: srcVal.String(),
		ValidatorDstAddress: oldValAddr.String(),
		Entries: []stakingtypes.RedelegationEntry{{
			CreationHeight: 9,
			CompletionTime: completionTime,
			InitialBalance: math.NewInt(75),
			SharesDst:      math.LegacyNewDec(75),
			UnbondingId:    89,
		}},
	}
	unrelatedRed := stakingtypes.Redelegation{
		DelegatorAddress:    testAccAddr().String(),
		ValidatorSrcAddress: sdk.ValAddress(testAccAddr()).String(),
		ValidatorDstAddress: sdk.ValAddress(testAccAddr()).String(),
		Entries: []stakingtypes.RedelegationEntry{{
			CreationHeight: 10,
			CompletionTime: completionTime,
			InitialBalance: math.NewInt(100),
			SharesDst:      math.LegacyNewDec(100),
			UnbondingId:    90,
		}},
	}
	f.writeRedelegation(srcRed)
	f.writeRedelegation(dstRed)
	f.writeRedelegation(unrelatedRed)

	rewritten := map[uint64]stakingtypes.Redelegation{}
	f.stakingKeeper.EXPECT().RemoveRedelegation(gomock.Any(), gomock.Any()).Return(nil).Times(2)
	f.stakingKeeper.EXPECT().SetRedelegation(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ any, red stakingtypes.Redelegation) error {
			require.True(t, red.ValidatorSrcAddress == newValAddr.String() || red.ValidatorDstAddress == newValAddr.String())
			require.False(t, red.ValidatorSrcAddress == oldValAddr.String())
			require.False(t, red.ValidatorDstAddress == oldValAddr.String())
			rewritten[red.Entries[0].UnbondingId] = red
			return nil
		},
	).Times(2)
	f.stakingKeeper.EXPECT().InsertRedelegationQueue(gomock.Any(), gomock.Any(), completionTime).Return(nil).Times(2)
	f.stakingKeeper.EXPECT().SetRedelegationByUnbondingID(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(2)

	// V4's internal scoped scan discovers the two related redelegations
	// (dropping the unrelated one) and re-keys exactly those.
	err := f.keeper.MigrateValidatorDelegations(f.ctx, oldValAddr, newValAddr, nil, nil, nil)
	require.NoError(t, err)
	require.Len(t, rewritten, 2)
	require.Equal(t, newValAddr.String(), rewritten[88].ValidatorSrcAddress)
	require.Equal(t, dstVal.String(), rewritten[88].ValidatorDstAddress)
	require.Equal(t, srcVal.String(), rewritten[89].ValidatorSrcAddress)
	require.Equal(t, newValAddr.String(), rewritten[89].ValidatorDstAddress)
}

func TestMigrateValidatorDelegations_DeduplicatesSourceAndDestinationIndexes(t *testing.T) {
	f := initMockFixture(t)
	f.wireScopedMigrationStores()

	oldValAddr := sdk.ValAddress(testAccAddr())
	newValAddr := sdk.ValAddress(testAccAddr())
	delegator := testAccAddr()
	completionTime := f.ctx.BlockTime().Add(21 * 24 * 3600 * 1e9)

	red := stakingtypes.Redelegation{
		DelegatorAddress:    delegator.String(),
		ValidatorSrcAddress: oldValAddr.String(),
		ValidatorDstAddress: oldValAddr.String(),
		Entries: []stakingtypes.RedelegationEntry{{
			CreationHeight: 10,
			CompletionTime: completionTime,
			InitialBalance: math.NewInt(100),
			SharesDst:      math.LegacyNewDec(100),
			UnbondingId:    101,
		}},
	}
	f.writeRedelegation(red)

	f.stakingKeeper.EXPECT().RemoveRedelegation(gomock.Any(), red).Return(nil).Times(1)
	f.stakingKeeper.EXPECT().SetRedelegation(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ any, migrated stakingtypes.Redelegation) error {
			require.Equal(t, newValAddr.String(), migrated.ValidatorSrcAddress)
			require.Equal(t, newValAddr.String(), migrated.ValidatorDstAddress)
			return nil
		},
	).Times(1)
	f.stakingKeeper.EXPECT().InsertRedelegationQueue(gomock.Any(), gomock.Any(), completionTime).Return(nil).Times(1)
	f.stakingKeeper.EXPECT().SetRedelegationByUnbondingID(gomock.Any(), gomock.Any(), uint64(101)).Return(nil).Times(1)

	// V4's internal scan collects the doubly-indexed redelegation exactly once
	// (src == dst), so it is re-keyed a single time (the .Times(1) mocks above).
	err := f.keeper.MigrateValidatorDelegations(f.ctx, oldValAddr, newValAddr, nil, nil, nil)
	require.NoError(t, err)
}

func TestMigrateValidatorDelegations_UsesPreloadedRedelegations(t *testing.T) {
	f := initMockFixture(t)
	f.wireScopedMigrationStores()

	oldValAddr := sdk.ValAddress(testAccAddr())
	newValAddr := sdk.ValAddress(testAccAddr())
	dstValAddr := sdk.ValAddress(testAccAddr())
	completionTime := f.ctx.BlockTime().Add(21 * 24 * 3600 * 1e9)

	red := stakingtypes.Redelegation{
		DelegatorAddress:    testAccAddr().String(),
		ValidatorSrcAddress: oldValAddr.String(),
		ValidatorDstAddress: dstValAddr.String(),
		Entries: []stakingtypes.RedelegationEntry{{
			CreationHeight: 10,
			CompletionTime: completionTime,
			InitialBalance: math.NewInt(100),
			SharesDst:      math.LegacyNewDec(100),
			UnbondingId:    111,
		}},
	}

	f.stakingKeeper.EXPECT().RemoveRedelegation(gomock.Any(), red).Return(nil).Times(1)
	f.stakingKeeper.EXPECT().SetRedelegation(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ sdk.Context, migrated stakingtypes.Redelegation) error {
			require.Equal(t, newValAddr.String(), migrated.ValidatorSrcAddress)
			require.Equal(t, dstValAddr.String(), migrated.ValidatorDstAddress)
			return nil
		},
	).Times(1)
	f.stakingKeeper.EXPECT().InsertRedelegationQueue(gomock.Any(), gomock.Any(), completionTime).Return(nil).Times(1)
	f.stakingKeeper.EXPECT().SetRedelegationByUnbondingID(gomock.Any(), gomock.Any(), uint64(111)).Return(nil).Times(1)

	// The staking store intentionally has no redelegation rows. Passing a
	// non-nil preloaded slice proves V4 does not rescan the scoped indexes.
	err := f.keeper.MigrateValidatorDelegations(f.ctx, oldValAddr, newValAddr, nil, nil, []stakingtypes.Redelegation{red})
	require.NoError(t, err)
}

func TestMigrateValidatorDelegations_ReturnsErrorForStaleRedelegationIndex(t *testing.T) {
	f := initMockFixture(t)
	f.wireScopedMigrationStores()

	oldValAddr := sdk.ValAddress(testAccAddr())
	newValAddr := sdk.ValAddress(testAccAddr())
	delegator := testAccAddr()
	dstValAddr := sdk.ValAddress(testAccAddr())

	f.writeRedelegationIndexes(delegator, oldValAddr, dstValAddr)

	// A stale redelegation index (no backing record) surfaces from V4's internal
	// scoped scan, which aborts before re-keying anything. delegations/ubds are
	// nil, so the scan runs first and its error propagates unchanged.
	err := f.keeper.MigrateValidatorDelegations(f.ctx, oldValAddr, newValAddr, nil, nil, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "points to missing record")
}

func TestMigrateValidatorDistribution_UsesScopedDistributionPrefixes(t *testing.T) {
	f := initMockFixture(t)
	f.wireScopedMigrationStores()

	oldValAddr := sdk.ValAddress(testAccAddr())
	newValAddr := sdk.ValAddress(testAccAddr())
	otherValAddr := sdk.ValAddress(testAccAddr())

	oldRewards := distrtypes.ValidatorHistoricalRewards{ReferenceCount: 7}
	otherRewards := distrtypes.ValidatorHistoricalRewards{ReferenceCount: 99}
	oldSlash := distrtypes.ValidatorSlashEvent{ValidatorPeriod: 11, Fraction: math.LegacyMustNewDecFromStr("0.010000000000000000")}
	otherSlash := distrtypes.ValidatorSlashEvent{ValidatorPeriod: 22, Fraction: math.LegacyMustNewDecFromStr("0.020000000000000000")}

	f.writeValidatorHistoricalRewards(oldValAddr, 11, oldRewards)
	f.writeValidatorHistoricalRewards(otherValAddr, 22, otherRewards)
	f.writeValidatorSlashEvent(oldValAddr, 100, oldSlash)
	f.writeValidatorSlashEvent(otherValAddr, 200, otherSlash)

	notFound := errors.New("not found")
	f.distributionKeeper.EXPECT().GetValidatorCurrentRewards(gomock.Any(), oldValAddr).Return(distrtypes.ValidatorCurrentRewards{}, notFound)
	f.distributionKeeper.EXPECT().GetValidatorAccumulatedCommission(gomock.Any(), oldValAddr).Return(distrtypes.ValidatorAccumulatedCommission{}, notFound)
	f.distributionKeeper.EXPECT().GetValidatorOutstandingRewards(gomock.Any(), oldValAddr).Return(distrtypes.ValidatorOutstandingRewards{}, notFound)

	f.distributionKeeper.EXPECT().DeleteValidatorHistoricalRewards(gomock.Any(), oldValAddr)
	f.distributionKeeper.EXPECT().SetValidatorHistoricalRewards(gomock.Any(), newValAddr, uint64(11), oldRewards).Return(nil)
	f.distributionKeeper.EXPECT().DeleteValidatorSlashEvents(gomock.Any(), oldValAddr)
	f.distributionKeeper.EXPECT().SetValidatorSlashEvent(gomock.Any(), newValAddr, uint64(100), oldSlash.ValidatorPeriod, oldSlash).Return(nil)

	err := f.keeper.MigrateValidatorDistribution(f.ctx, oldValAddr, newValAddr)
	require.NoError(t, err)
}

func TestMigrateValidatorDelegations_RekeysMultipleSourceRedelegations(t *testing.T) {
	f := initMockFixture(t)
	f.wireScopedMigrationStores()

	oldValAddr := sdk.ValAddress(testAccAddr())
	newValAddr := sdk.ValAddress(testAccAddr())
	completionTime := f.ctx.BlockTime().Add(21 * 24 * 3600 * 1e9)

	// Two redelegations that both have oldValAddr as SOURCE, from distinct
	// delegators to distinct destinations. Both live under the same val-src
	// index prefix, so the scan must advance the iterator past the first key.
	dstA := sdk.ValAddress(testAccAddr())
	dstB := sdk.ValAddress(testAccAddr())
	redA := stakingtypes.Redelegation{
		DelegatorAddress:    testAccAddr().String(),
		ValidatorSrcAddress: oldValAddr.String(),
		ValidatorDstAddress: dstA.String(),
		Entries: []stakingtypes.RedelegationEntry{{
			CreationHeight: 8,
			CompletionTime: completionTime,
			InitialBalance: math.NewInt(50),
			SharesDst:      math.LegacyNewDec(50),
			UnbondingId:    201,
		}},
	}
	redB := stakingtypes.Redelegation{
		DelegatorAddress:    testAccAddr().String(),
		ValidatorSrcAddress: oldValAddr.String(),
		ValidatorDstAddress: dstB.String(),
		Entries: []stakingtypes.RedelegationEntry{{
			CreationHeight: 9,
			CompletionTime: completionTime,
			InitialBalance: math.NewInt(75),
			SharesDst:      math.LegacyNewDec(75),
			UnbondingId:    202,
		}},
	}
	f.writeRedelegation(redA)
	f.writeRedelegation(redB)

	rewritten := map[uint64]stakingtypes.Redelegation{}
	f.stakingKeeper.EXPECT().RemoveRedelegation(gomock.Any(), gomock.Any()).Return(nil).Times(2)
	f.stakingKeeper.EXPECT().SetRedelegation(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ any, red stakingtypes.Redelegation) error {
			require.Equal(t, newValAddr.String(), red.ValidatorSrcAddress)
			require.NotEqual(t, oldValAddr.String(), red.ValidatorDstAddress)
			rewritten[red.Entries[0].UnbondingId] = red
			return nil
		},
	).Times(2)
	f.stakingKeeper.EXPECT().InsertRedelegationQueue(gomock.Any(), gomock.Any(), completionTime).Return(nil).Times(2)
	f.stakingKeeper.EXPECT().SetRedelegationByUnbondingID(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(2)

	// Both redelegations share the val-src index prefix; V4's internal scan must
	// advance the iterator past the first key to collect (and re-key) both.
	err := f.keeper.MigrateValidatorDelegations(f.ctx, oldValAddr, newValAddr, nil, nil, nil)
	require.NoError(t, err)
	require.Len(t, rewritten, 2)
	require.Equal(t, dstA.String(), rewritten[201].ValidatorDstAddress)
	require.Equal(t, dstB.String(), rewritten[202].ValidatorDstAddress)
}

// TestMigrateValidatorDelegations_SetsHistoricalRewardsRefCountOnce pins the V4
// optimization: rather than resetting the target period's historical-rewards
// reference count and then incrementing it once per delegation (N+1 full-chain
// scans), the count is written exactly once as base(1) + N. The scoped
// distribution store is wired, so the lookup is the production O(1) store.Get
// path; the single write is captured to assert the exact final count. A wrong
// count (off-by-one) or a per-delegation-increment regression (Times(1) → N)
// both fail here.
func TestMigrateValidatorDelegations_SetsHistoricalRewardsRefCountOnce(t *testing.T) {
	f := initMockFixture(t)
	f.wireScopedMigrationStores()

	oldValAddr := sdk.ValAddress(testAccAddr())
	newValAddr := sdk.ValAddress(testAccAddr())

	// Three active delegations from distinct delegators.
	dels := make([]stakingtypes.Delegation, 3)
	for i := range dels {
		dels[i] = stakingtypes.NewDelegation(testAccAddr().String(), oldValAddr.String(), math.LegacyNewDec(int64(10*(i+1))))
	}

	// Current rewards period 5 → target (previous) period 4.
	const targetPeriod = uint64(4)
	f.distributionKeeper.EXPECT().GetValidatorCurrentRewards(gomock.Any(), newValAddr).Return(
		distrtypes.ValidatorCurrentRewards{Period: 5}, nil,
	)
	// Seed a stale base row so the scoped O(1) lookup succeeds; its count must be
	// overwritten in a single write, not incremented from.
	f.writeValidatorHistoricalRewards(newValAddr, targetPeriod, distrtypes.ValidatorHistoricalRewards{ReferenceCount: 1})

	// The refcount write must happen EXACTLY once; a per-delegation increment
	// regression would call it N times and fail the Times(1) bound.
	var capturedRefCount uint32
	f.distributionKeeper.EXPECT().SetValidatorHistoricalRewards(gomock.Any(), newValAddr, targetPeriod, gomock.Any()).DoAndReturn(
		func(_ sdk.Context, _ sdk.ValAddress, _ uint64, h distrtypes.ValidatorHistoricalRewards) error {
			capturedRefCount = h.ReferenceCount
			return nil
		},
	).Times(1)

	// V4 fetches the re-keyed validator once to convert shares → tokens (rate 1.0).
	f.stakingKeeper.EXPECT().GetValidator(gomock.Any(), newValAddr).Return(
		stakingtypes.Validator{OperatorAddress: newValAddr.String(), Tokens: math.NewInt(100), DelegatorShares: math.LegacyNewDec(100)}, nil,
	)

	// Per-delegation re-keying: no refcount call inside the loop. Each delegation's
	// fresh starting info must reference the target period.
	f.distributionKeeper.EXPECT().DeleteDelegatorStartingInfo(gomock.Any(), oldValAddr, gomock.Any()).Return(nil).Times(3)
	f.stakingKeeper.EXPECT().RemoveDelegation(gomock.Any(), gomock.Any()).Return(nil).Times(3)
	f.stakingKeeper.EXPECT().SetDelegation(gomock.Any(), gomock.Any()).Return(nil).Times(3)
	f.distributionKeeper.EXPECT().SetDelegatorStartingInfo(gomock.Any(), newValAddr, gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ sdk.Context, _ sdk.ValAddress, _ sdk.AccAddress, info distrtypes.DelegatorStartingInfo) error {
			require.Equal(t, targetPeriod, info.PreviousPeriod)
			return nil
		},
	).Times(3)

	// Delegations passed in directly; no unbondings/redelegations.
	err := f.keeper.MigrateValidatorDelegations(f.ctx, oldValAddr, newValAddr, dels, nil, nil)
	require.NoError(t, err)
	// base(1) + 3 delegations.
	require.Equal(t, uint32(4), capturedRefCount)
}

// TestMigrateValidatorDelegations_SlashedValidatorStoresTokensNotShares verifies
// the re-keyed DelegatorStartingInfo.Stake holds tokens
// (val.TokensFromSharesTruncated(shares)), not raw shares. The SDK stores stake
// as tokens (x/distribution initializeDelegation); for a validator whose exchange
// rate dropped below 1 — any validator ever slashed, which passes the
// only-currently-jailed pre-check — shares overstate the real stake, so storing
// shares makes CalculateDelegationRewards panic ("calculated final stake greater
// than current stake") on every delegator's next withdraw/undelegate/redelegate.
func TestMigrateValidatorDelegations_SlashedValidatorStoresTokensNotShares(t *testing.T) {
	f := initMockFixture(t)
	f.wireScopedMigrationStores()

	oldValAddr := sdk.ValAddress(testAccAddr())
	newValAddr := sdk.ValAddress(testAccAddr())

	del := stakingtypes.NewDelegation(testAccAddr().String(), oldValAddr.String(), math.LegacyNewDec(100))

	// Slashed validator: 90 tokens back 100 shares (exchange rate 0.9), so
	// TokensFromSharesTruncated(100) = 90, strictly less than the 100 shares.
	slashedVal := stakingtypes.Validator{
		OperatorAddress: newValAddr.String(),
		Tokens:          math.NewInt(90),
		DelegatorShares: math.LegacyNewDec(100),
	}
	expectedStake := slashedVal.TokensFromSharesTruncated(del.Shares)

	const targetPeriod = uint64(4)
	f.distributionKeeper.EXPECT().GetValidatorCurrentRewards(gomock.Any(), newValAddr).Return(
		distrtypes.ValidatorCurrentRewards{Period: 5}, nil,
	)
	f.writeValidatorHistoricalRewards(newValAddr, targetPeriod, distrtypes.ValidatorHistoricalRewards{ReferenceCount: 1})
	f.distributionKeeper.EXPECT().SetValidatorHistoricalRewards(gomock.Any(), newValAddr, targetPeriod, gomock.Any()).Return(nil).Times(1)

	// The re-keyed validator must be fetched to convert shares → tokens.
	f.stakingKeeper.EXPECT().GetValidator(gomock.Any(), newValAddr).Return(slashedVal, nil)

	f.distributionKeeper.EXPECT().DeleteDelegatorStartingInfo(gomock.Any(), oldValAddr, gomock.Any()).Return(nil)
	f.stakingKeeper.EXPECT().RemoveDelegation(gomock.Any(), gomock.Any()).Return(nil)
	f.stakingKeeper.EXPECT().SetDelegation(gomock.Any(), gomock.Any()).Return(nil)

	var capturedStake math.LegacyDec
	f.distributionKeeper.EXPECT().SetDelegatorStartingInfo(gomock.Any(), newValAddr, gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ sdk.Context, _ sdk.ValAddress, _ sdk.AccAddress, info distrtypes.DelegatorStartingInfo) error {
			capturedStake = info.Stake
			return nil
		},
	)

	err := f.keeper.MigrateValidatorDelegations(f.ctx, oldValAddr, newValAddr, []stakingtypes.Delegation{del}, nil, nil)
	require.NoError(t, err)
	require.Equal(t, expectedStake, capturedStake)
	require.True(t, capturedStake.LT(del.Shares), "stake must be tokens (< shares) for a slashed validator")
}

// TestRepairLegacyRawShareStartingInfo_ReconstructsPreSlashTokenStake pins the
// repair for rows already written by v1.20.0. The first slash predates the
// starting row and is therefore reflected only in the validator exchange rate;
// the second slash follows the row and must remain part of period replay.
func TestRepairLegacyRawShareStartingInfo_ReconstructsPreSlashTokenStake(t *testing.T) {
	f := initMockFixture(t)
	f.wireScopedMigrationStores()
	f.ctx = f.ctx.WithBlockHeight(300)

	valAddr := sdk.ValAddress(testAccAddr())
	delAddr := testAccAddr()
	del := stakingtypes.NewDelegation(delAddr.String(), valAddr.String(), math.LegacyNewDec(500_000))

	// Two 0.1% slashes leave the validator at a 0.998001 token/share rate.
	// The correct stake at the row's height (after only the first slash) was
	// 499,500; v1.20.0 incorrectly stored the raw 500,000 shares instead.
	val := stakingtypes.Validator{
		OperatorAddress: valAddr.String(),
		Tokens:          math.NewInt(998_001),
		DelegatorShares: math.LegacyNewDec(1_000_000),
	}
	startingInfo := distrtypes.DelegatorStartingInfo{
		PreviousPeriod: 180,
		Stake:          del.Shares,
		Height:         100,
	}
	f.distributionKeeper.EXPECT().GetDelegatorStartingInfo(gomock.Any(), valAddr, delAddr).Return(startingInfo, nil)

	// The pre-start event must not be replayed; the post-start event must be.
	f.writeValidatorSlashEvent(valAddr, 50, distrtypes.ValidatorSlashEvent{
		ValidatorPeriod: 31,
		Fraction:        math.LegacyMustNewDecFromStr("0.001"),
	})
	f.writeValidatorSlashEvent(valAddr, 200, distrtypes.ValidatorSlashEvent{
		ValidatorPeriod: 182,
		Fraction:        math.LegacyMustNewDecFromStr("0.001"),
	})

	var repaired distrtypes.DelegatorStartingInfo
	f.distributionKeeper.EXPECT().SetDelegatorStartingInfo(gomock.Any(), valAddr, delAddr, gomock.Any()).
		DoAndReturn(func(_ sdk.Context, _ sdk.ValAddress, _ sdk.AccAddress, info distrtypes.DelegatorStartingInfo) error {
			repaired = info
			return nil
		})

	err := f.keeper.RepairLegacyRawShareStartingInfoForTest(f.ctx, val, del, delAddr)
	require.NoError(t, err)
	require.Equal(t, math.LegacyMustNewDecFromStr("499500"), repaired.Stake)

	// Replaying the post-start slash now lands exactly at current token stake,
	// satisfying the x/distribution invariant that previously panicked.
	finalStake := repaired.Stake.MulTruncate(math.LegacyMustNewDecFromStr("0.999"))
	require.Equal(t, val.TokensFromShares(del.Shares), finalStake)
}

func TestMigrateValidatorDistribution_RekeysAllPeriodsAndSlashEvents(t *testing.T) {
	f := initMockFixture(t)
	f.wireScopedMigrationStores()

	oldValAddr := sdk.ValAddress(testAccAddr())
	newValAddr := sdk.ValAddress(testAccAddr())
	otherValAddr := sdk.ValAddress(testAccAddr())

	// Two historical-reward periods for old, one for an unrelated validator.
	// Distinct ReferenceCounts make the two re-key calls distinguishable.
	rewards11 := distrtypes.ValidatorHistoricalRewards{ReferenceCount: 7}
	rewards42 := distrtypes.ValidatorHistoricalRewards{ReferenceCount: 13}
	otherRewards := distrtypes.ValidatorHistoricalRewards{ReferenceCount: 99}
	f.writeValidatorHistoricalRewards(oldValAddr, 11, rewards11)
	f.writeValidatorHistoricalRewards(oldValAddr, 42, rewards42)
	f.writeValidatorHistoricalRewards(otherValAddr, 11, otherRewards)

	// Two slash events for old with DISTINCT heights AND periods (height!=period
	// per event), plus one for an unrelated validator. Height comes from the
	// store key, period from the unmarshaled value; making them differ pins down
	// that the re-key does not swap the two.
	slash100 := distrtypes.ValidatorSlashEvent{ValidatorPeriod: 5, Fraction: math.LegacyMustNewDecFromStr("0.010000000000000000")}
	slash250 := distrtypes.ValidatorSlashEvent{ValidatorPeriod: 9, Fraction: math.LegacyMustNewDecFromStr("0.030000000000000000")}
	otherSlash := distrtypes.ValidatorSlashEvent{ValidatorPeriod: 22, Fraction: math.LegacyMustNewDecFromStr("0.020000000000000000")}
	f.writeValidatorSlashEvent(oldValAddr, 100, slash100)
	f.writeValidatorSlashEvent(oldValAddr, 250, slash250)
	f.writeValidatorSlashEvent(otherValAddr, 300, otherSlash)

	notFound := errors.New("not found")
	f.distributionKeeper.EXPECT().GetValidatorCurrentRewards(gomock.Any(), oldValAddr).Return(distrtypes.ValidatorCurrentRewards{}, notFound)
	f.distributionKeeper.EXPECT().GetValidatorAccumulatedCommission(gomock.Any(), oldValAddr).Return(distrtypes.ValidatorAccumulatedCommission{}, notFound)
	f.distributionKeeper.EXPECT().GetValidatorOutstandingRewards(gomock.Any(), oldValAddr).Return(distrtypes.ValidatorOutstandingRewards{}, notFound)

	f.distributionKeeper.EXPECT().DeleteValidatorHistoricalRewards(gomock.Any(), oldValAddr)
	f.distributionKeeper.EXPECT().SetValidatorHistoricalRewards(gomock.Any(), newValAddr, uint64(11), rewards11).Return(nil)
	f.distributionKeeper.EXPECT().SetValidatorHistoricalRewards(gomock.Any(), newValAddr, uint64(42), rewards42).Return(nil)

	f.distributionKeeper.EXPECT().DeleteValidatorSlashEvents(gomock.Any(), oldValAddr)
	f.distributionKeeper.EXPECT().SetValidatorSlashEvent(gomock.Any(), newValAddr, uint64(100), uint64(5), slash100).Return(nil)
	f.distributionKeeper.EXPECT().SetValidatorSlashEvent(gomock.Any(), newValAddr, uint64(250), uint64(9), slash250).Return(nil)

	err := f.keeper.MigrateValidatorDistribution(f.ctx, oldValAddr, newValAddr)
	require.NoError(t, err)
}

func TestMigrateValidatorScopedIteration_SimulatesGlobalStateImprovement(t *testing.T) {
	f := initMockFixture(t)
	f.wireScopedMigrationStores()

	oldValAddr := sdk.ValAddress(testAccAddr())
	newValAddr := sdk.ValAddress(testAccAddr())
	completionTime := f.ctx.BlockTime().Add(21 * 24 * 3600 * 1e9)

	const (
		unrelatedValidators = 25
		recordsPerValidator = 20
	)
	for i := 0; i < unrelatedValidators; i++ {
		val := sdk.ValAddress(testAccAddr())
		for j := 0; j < recordsPerValidator; j++ {
			period := uint64(10_000 + i*recordsPerValidator + j)
			height := uint64(20_000 + i*recordsPerValidator + j)
			f.writeValidatorHistoricalRewards(val, period, distrtypes.ValidatorHistoricalRewards{ReferenceCount: uint32(1 + j%3)})
			f.writeValidatorSlashEvent(
				val,
				height,
				distrtypes.ValidatorSlashEvent{
					ValidatorPeriod: period,
					Fraction:        math.LegacyMustNewDecFromStr("0.010000000000000000"),
				},
			)
			f.writeRedelegation(stakingtypes.Redelegation{
				DelegatorAddress:    testAccAddr().String(),
				ValidatorSrcAddress: sdk.ValAddress(testAccAddr()).String(),
				ValidatorDstAddress: sdk.ValAddress(testAccAddr()).String(),
				Entries: []stakingtypes.RedelegationEntry{{
					CreationHeight: int64(height),
					CompletionTime: completionTime,
					InitialBalance: math.NewInt(100),
					SharesDst:      math.LegacyNewDec(100),
					UnbondingId:    uint64(30_000 + i*recordsPerValidator + j),
				}},
			})
		}
	}

	rewards11 := distrtypes.ValidatorHistoricalRewards{ReferenceCount: 7}
	rewards12 := distrtypes.ValidatorHistoricalRewards{ReferenceCount: 8}
	rewards13 := distrtypes.ValidatorHistoricalRewards{ReferenceCount: 9}
	f.writeValidatorHistoricalRewards(oldValAddr, 11, rewards11)
	f.writeValidatorHistoricalRewards(oldValAddr, 12, rewards12)
	f.writeValidatorHistoricalRewards(oldValAddr, 13, rewards13)

	slash100 := distrtypes.ValidatorSlashEvent{ValidatorPeriod: 21, Fraction: math.LegacyMustNewDecFromStr("0.020000000000000000")}
	slash101 := distrtypes.ValidatorSlashEvent{ValidatorPeriod: 22, Fraction: math.LegacyMustNewDecFromStr("0.030000000000000000")}
	f.writeValidatorSlashEvent(oldValAddr, 100, slash100)
	f.writeValidatorSlashEvent(oldValAddr, 101, slash101)

	dstValAddr := sdk.ValAddress(testAccAddr())
	srcValAddr := sdk.ValAddress(testAccAddr())
	srcRed := stakingtypes.Redelegation{
		DelegatorAddress:    testAccAddr().String(),
		ValidatorSrcAddress: oldValAddr.String(),
		ValidatorDstAddress: dstValAddr.String(),
		Entries: []stakingtypes.RedelegationEntry{{
			CreationHeight: 101,
			CompletionTime: completionTime,
			InitialBalance: math.NewInt(50),
			SharesDst:      math.LegacyNewDec(50),
			UnbondingId:    40_001,
		}},
	}
	dstRed := stakingtypes.Redelegation{
		DelegatorAddress:    testAccAddr().String(),
		ValidatorSrcAddress: srcValAddr.String(),
		ValidatorDstAddress: oldValAddr.String(),
		Entries: []stakingtypes.RedelegationEntry{{
			CreationHeight: 102,
			CompletionTime: completionTime,
			InitialBalance: math.NewInt(75),
			SharesDst:      math.LegacyNewDec(75),
			UnbondingId:    40_002,
		}},
	}
	f.writeRedelegation(srcRed)
	f.writeRedelegation(dstRed)

	broadScanKeys := countKVPrefix(t, f.distributionStore, f.ctx, distrtypes.ValidatorHistoricalRewardsPrefix) +
		countKVPrefix(t, f.distributionStore, f.ctx, distrtypes.ValidatorSlashEventPrefix) +
		countKVPrefix(t, f.stakingStore, f.ctx, stakingtypes.RedelegationKey)
	scopedDistributionKeys := countKVPrefix(t, f.distributionStore, f.ctx, distrtypes.GetValidatorHistoricalRewardsPrefix(oldValAddr)) +
		countKVPrefix(t, f.distributionStore, f.ctx, distrtypes.GetValidatorSlashEventPrefix(oldValAddr))
	scopedRedelegationKeys := countKVPrefix(t, f.stakingStore, f.ctx, stakingtypes.GetREDsFromValSrcIndexKey(oldValAddr)) +
		countKVPrefix(t, f.stakingStore, f.ctx, stakingtypes.GetREDsToValDstIndexKey(oldValAddr))
	scopedKeysBeforeRedelegationReuse := scopedDistributionKeys + scopedRedelegationKeys + scopedRedelegationKeys
	scopedKeysAfterRedelegationReuse := scopedDistributionKeys + scopedRedelegationKeys

	require.Equal(t, 1507, broadScanKeys)
	require.Equal(t, 9, scopedKeysBeforeRedelegationReuse)
	require.Equal(t, 7, scopedKeysAfterRedelegationReuse)
	require.GreaterOrEqual(t, broadScanKeys/scopedKeysAfterRedelegationReuse, 200)
	t.Logf(
		"simulated testnet shape: old broad scan keys=%d, scoped before red reuse=%d, scoped after red reuse=%d, main reduction=%dx, incremental scoped reduction=%d%%",
		broadScanKeys,
		scopedKeysBeforeRedelegationReuse,
		scopedKeysAfterRedelegationReuse,
		broadScanKeys/scopedKeysAfterRedelegationReuse,
		100*(scopedKeysBeforeRedelegationReuse-scopedKeysAfterRedelegationReuse)/scopedKeysBeforeRedelegationReuse,
	)

	notFound := errors.New("not found")
	f.distributionKeeper.EXPECT().GetValidatorCurrentRewards(gomock.Any(), oldValAddr).Return(distrtypes.ValidatorCurrentRewards{}, notFound)
	f.distributionKeeper.EXPECT().GetValidatorAccumulatedCommission(gomock.Any(), oldValAddr).Return(distrtypes.ValidatorAccumulatedCommission{}, notFound)
	f.distributionKeeper.EXPECT().GetValidatorOutstandingRewards(gomock.Any(), oldValAddr).Return(distrtypes.ValidatorOutstandingRewards{}, notFound)
	f.distributionKeeper.EXPECT().DeleteValidatorHistoricalRewards(gomock.Any(), oldValAddr)
	f.distributionKeeper.EXPECT().SetValidatorHistoricalRewards(gomock.Any(), newValAddr, uint64(11), rewards11).Return(nil)
	f.distributionKeeper.EXPECT().SetValidatorHistoricalRewards(gomock.Any(), newValAddr, uint64(12), rewards12).Return(nil)
	f.distributionKeeper.EXPECT().SetValidatorHistoricalRewards(gomock.Any(), newValAddr, uint64(13), rewards13).Return(nil)
	f.distributionKeeper.EXPECT().DeleteValidatorSlashEvents(gomock.Any(), oldValAddr)
	f.distributionKeeper.EXPECT().SetValidatorSlashEvent(gomock.Any(), newValAddr, uint64(100), slash100.ValidatorPeriod, slash100).Return(nil)
	f.distributionKeeper.EXPECT().SetValidatorSlashEvent(gomock.Any(), newValAddr, uint64(101), slash101.ValidatorPeriod, slash101).Return(nil)

	err := f.keeper.MigrateValidatorDistribution(f.ctx, oldValAddr, newValAddr)
	require.NoError(t, err)

	rewritten := map[uint64]stakingtypes.Redelegation{}
	f.stakingKeeper.EXPECT().RemoveRedelegation(gomock.Any(), gomock.Any()).Return(nil).Times(2)
	f.stakingKeeper.EXPECT().SetRedelegation(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ sdk.Context, red stakingtypes.Redelegation) error {
			rewritten[red.Entries[0].UnbondingId] = red
			return nil
		},
	).Times(2)
	f.stakingKeeper.EXPECT().InsertRedelegationQueue(gomock.Any(), gomock.Any(), completionTime).Return(nil).Times(2)
	f.stakingKeeper.EXPECT().SetRedelegationByUnbondingID(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(2)

	err = f.keeper.MigrateValidatorDelegations(f.ctx, oldValAddr, newValAddr, nil, nil, []stakingtypes.Redelegation{srcRed, dstRed})
	require.NoError(t, err)
	require.Len(t, rewritten, 2)
	require.Equal(t, newValAddr.String(), rewritten[40_001].ValidatorSrcAddress)
	require.Equal(t, dstValAddr.String(), rewritten[40_001].ValidatorDstAddress)
	require.Equal(t, srcValAddr.String(), rewritten[40_002].ValidatorSrcAddress)
	require.Equal(t, newValAddr.String(), rewritten[40_002].ValidatorDstAddress)
}

// TestMigrateValidatorDelegations_RedelegationReplayIsDeterministic locks in the
// fix for a consensus-halt bug: redelegationsForValidator collects records into a
// Go map, and iterating that map directly to replay them (RemoveRedelegation ->
// SetRedelegation -> InsertRedelegationQueue) leaked Go's randomized map order into
// state writes. InsertRedelegationQueue appends to shared queue timeslices, so a
// nondeterministic replay order diverges the queue bytes (and the app hash) across
// nodes. The fix emits records in redelegation-store-key order; this test asserts
// the replay order matches that canonical order regardless of insertion order.
//
// With numReds records there are numReds! possible map orderings, so if the sort is
// ever reverted to raw map iteration this test fails essentially every run rather
// than flaking intermittently.
func TestMigrateValidatorDelegations_RedelegationReplayIsDeterministic(t *testing.T) {
	f := initMockFixture(t)
	f.wireScopedMigrationStores()

	oldValAddr := sdk.ValAddress(testAccAddr())
	newValAddr := sdk.ValAddress(testAccAddr())
	completionTime := f.ctx.BlockTime().Add(21 * 24 * 3600 * 1e9)

	const numReds = 12

	type placedRed struct {
		key         string
		unbondingID uint64
	}
	placed := make([]placedRed, 0, numReds)
	for i := 0; i < numReds; i++ {
		del := testAccAddr()
		dst := sdk.ValAddress(testAccAddr())
		unbondingID := uint64(70_000 + i)
		f.writeRedelegation(stakingtypes.Redelegation{
			DelegatorAddress:    del.String(),
			ValidatorSrcAddress: oldValAddr.String(),
			ValidatorDstAddress: dst.String(),
			Entries: []stakingtypes.RedelegationEntry{{
				CreationHeight: int64(i),
				CompletionTime: completionTime,
				InitialBalance: math.NewInt(100),
				SharesDst:      math.LegacyNewDec(100),
				UnbondingId:    unbondingID,
			}},
		})
		placed = append(placed, placedRed{
			key:         string(stakingtypes.GetREDKey(del, oldValAddr, dst)),
			unbondingID: unbondingID,
		})
	}

	// Expected replay order == redelegations sorted by their store key, matching
	// the canonical order redelegationsForValidator now emits.
	sorted := make([]placedRed, len(placed))
	copy(sorted, placed)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].key < sorted[j].key })
	expectedOrder := make([]uint64, len(sorted))
	for i, p := range sorted {
		expectedOrder[i] = p.unbondingID
	}

	var replayOrder []uint64
	f.stakingKeeper.EXPECT().RemoveRedelegation(gomock.Any(), gomock.Any()).Return(nil).Times(numReds)
	f.stakingKeeper.EXPECT().SetRedelegation(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ sdk.Context, red stakingtypes.Redelegation) error {
			replayOrder = append(replayOrder, red.Entries[0].UnbondingId)
			return nil
		},
	).Times(numReds)
	f.stakingKeeper.EXPECT().InsertRedelegationQueue(gomock.Any(), gomock.Any(), completionTime).Return(nil).Times(numReds)
	f.stakingKeeper.EXPECT().SetRedelegationByUnbondingID(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(numReds)

	// Passing nil redelegations forces the internal scoped scan (the map path).
	err := f.keeper.MigrateValidatorDelegations(f.ctx, oldValAddr, newValAddr, nil, nil, nil)
	require.NoError(t, err)

	require.Equal(t, expectedOrder, replayOrder, "redelegations must replay in deterministic store-key order")
}

// --- Validator-supernode metrics tests ---

// TestMigrateValidatorSupernode_WithMetrics verifies that metrics state is
// re-keyed when the supernode has metrics.
func TestMigrateValidatorSupernode_WithMetrics(t *testing.T) {
	f := initMockFixture(t)
	oldValAddr := sdk.ValAddress(testAccAddr())
	newValAddr := sdk.ValAddress(testAccAddr())
	newAddr := sdk.AccAddress(newValAddr)

	sn := sntypes.SuperNode{
		ValidatorAddress: oldValAddr.String(),
		SupernodeAccount: sdk.AccAddress(oldValAddr).String(),
	}
	metrics := sntypes.SupernodeMetricsState{
		ValidatorAddress: oldValAddr.String(),
	}

	f.supernodeKeeper.EXPECT().QuerySuperNode(gomock.Any(), oldValAddr).Return(sn, true)
	f.supernodeKeeper.EXPECT().DeleteSuperNode(gomock.Any(), oldValAddr)
	f.supernodeKeeper.EXPECT().GetMetricsState(gomock.Any(), oldValAddr).Return(metrics, true)
	f.supernodeKeeper.EXPECT().SetMetricsState(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ any, updated sntypes.SupernodeMetricsState) error {
			require.Equal(t, newValAddr.String(), updated.ValidatorAddress)
			return nil
		})
	f.supernodeKeeper.EXPECT().DeleteMetricsState(gomock.Any(), oldValAddr)
	f.supernodeKeeper.EXPECT().SetSuperNode(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ any, updated sntypes.SuperNode) error {
			require.Equal(t, newAddr.String(), updated.SupernodeAccount)
			return nil
		})

	err := f.keeper.MigrateValidatorSupernode(f.ctx, oldValAddr, newValAddr, sdk.AccAddress(oldValAddr), newAddr)
	require.NoError(t, err)
}

// TestMigrateValidatorSupernode_MetricsWriteFails verifies that a failure
// writing metrics state propagates as an error.
func TestMigrateValidatorSupernode_MetricsWriteFails(t *testing.T) {
	f := initMockFixture(t)
	oldValAddr := sdk.ValAddress(testAccAddr())
	newValAddr := sdk.ValAddress(testAccAddr())
	newAddr := sdk.AccAddress(newValAddr)

	sn := sntypes.SuperNode{
		ValidatorAddress: oldValAddr.String(),
		SupernodeAccount: sdk.AccAddress(oldValAddr).String(),
	}
	metrics := sntypes.SupernodeMetricsState{
		ValidatorAddress: oldValAddr.String(),
	}

	f.supernodeKeeper.EXPECT().QuerySuperNode(gomock.Any(), oldValAddr).Return(sn, true)
	f.supernodeKeeper.EXPECT().DeleteSuperNode(gomock.Any(), oldValAddr)
	f.supernodeKeeper.EXPECT().GetMetricsState(gomock.Any(), oldValAddr).Return(metrics, true)
	f.supernodeKeeper.EXPECT().SetMetricsState(gomock.Any(), gomock.Any()).Return(
		fmt.Errorf("metrics store write failed"),
	)

	err := f.keeper.MigrateValidatorSupernode(f.ctx, oldValAddr, newValAddr, sdk.AccAddress(oldValAddr), newAddr)
	require.Error(t, err)
	require.Contains(t, err.Error(), "metrics store write failed")
}

// TestMigrateValidatorSupernode_NotFound verifies no-op when not a supernode.
func TestMigrateValidatorSupernode_NotFound(t *testing.T) {
	f := initMockFixture(t)
	oldValAddr := sdk.ValAddress(testAccAddr())
	newValAddr := sdk.ValAddress(testAccAddr())
	newAddr := sdk.AccAddress(newValAddr)

	f.supernodeKeeper.EXPECT().QuerySuperNode(gomock.Any(), oldValAddr).Return(sntypes.SuperNode{}, false)

	err := f.keeper.MigrateValidatorSupernode(f.ctx, oldValAddr, newValAddr, sdk.AccAddress(oldValAddr), newAddr)
	require.NoError(t, err)
}

// TestMigrateValidatorSupernode_EvidenceAddressMigrated verifies that
// Evidence.ValidatorAddress entries matching the old valoper are updated.
func TestMigrateValidatorSupernode_EvidenceAddressMigrated(t *testing.T) {
	f := initMockFixture(t)
	oldValAddr := sdk.ValAddress(testAccAddr())
	newValAddr := sdk.ValAddress(testAccAddr())
	newAddr := sdk.AccAddress(newValAddr)
	otherValAddr := sdk.ValAddress(testAccAddr()).String()

	sn := sntypes.SuperNode{
		ValidatorAddress: oldValAddr.String(),
		SupernodeAccount: sdk.AccAddress(oldValAddr).String(),
		Evidence: []*sntypes.Evidence{
			{ValidatorAddress: oldValAddr.String(), ReporterAddress: testAccAddr().String(), ActionId: "1"},
			{ValidatorAddress: otherValAddr, ReporterAddress: testAccAddr().String(), ActionId: "2"},
		},
	}

	f.supernodeKeeper.EXPECT().QuerySuperNode(gomock.Any(), oldValAddr).Return(sn, true)
	f.supernodeKeeper.EXPECT().DeleteSuperNode(gomock.Any(), oldValAddr)
	f.supernodeKeeper.EXPECT().GetMetricsState(gomock.Any(), oldValAddr).Return(sntypes.SupernodeMetricsState{}, false)
	f.supernodeKeeper.EXPECT().SetSuperNode(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ any, updated sntypes.SuperNode) error {
			require.Len(t, updated.Evidence, 2)
			// Evidence pointing to the migrated validator should be updated.
			require.Equal(t, newValAddr.String(), updated.Evidence[0].ValidatorAddress)
			// Evidence pointing to a different validator should be unchanged.
			require.Equal(t, otherValAddr, updated.Evidence[1].ValidatorAddress)
			return nil
		})

	err := f.keeper.MigrateValidatorSupernode(f.ctx, oldValAddr, newValAddr, sdk.AccAddress(oldValAddr), newAddr)
	require.NoError(t, err)
}

// TestMigrateValidatorSupernode_AccountHistoryMigrated verifies that
// PrevSupernodeAccounts entries matching the old account are updated.
func TestMigrateValidatorSupernode_AccountHistoryMigrated(t *testing.T) {
	f := initMockFixture(t)
	oldValAddr := sdk.ValAddress(testAccAddr())
	newValAddr := sdk.ValAddress(testAccAddr())
	newAddr := sdk.AccAddress(newValAddr)
	oldAccountStr := sdk.AccAddress(oldValAddr).String()
	otherAccount := testAccAddr().String()

	sn := sntypes.SuperNode{
		ValidatorAddress: oldValAddr.String(),
		SupernodeAccount: oldAccountStr,
		PrevSupernodeAccounts: []*sntypes.SupernodeAccountHistory{
			{Account: oldAccountStr, Height: 100},
			{Account: otherAccount, Height: 50},
		},
	}

	f.supernodeKeeper.EXPECT().QuerySuperNode(gomock.Any(), oldValAddr).Return(sn, true)
	f.supernodeKeeper.EXPECT().DeleteSuperNode(gomock.Any(), oldValAddr)
	f.supernodeKeeper.EXPECT().GetMetricsState(gomock.Any(), oldValAddr).Return(sntypes.SupernodeMetricsState{}, false)
	f.supernodeKeeper.EXPECT().SetSuperNode(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ any, updated sntypes.SuperNode) error {
			require.Len(t, updated.PrevSupernodeAccounts, 3)
			// Entry matching old account should be updated.
			require.Equal(t, newAddr.String(), updated.PrevSupernodeAccounts[0].Account)
			// Entry for a different account should be unchanged.
			require.Equal(t, otherAccount, updated.PrevSupernodeAccounts[1].Account)
			// New migration entry appended with new address and current height.
			require.Equal(t, newAddr.String(), updated.PrevSupernodeAccounts[2].Account)
			require.Equal(t, f.ctx.BlockHeight(), updated.PrevSupernodeAccounts[2].Height)
			return nil
		})

	err := f.keeper.MigrateValidatorSupernode(f.ctx, oldValAddr, newValAddr, sdk.AccAddress(oldValAddr), newAddr)
	require.NoError(t, err)
}

// TestMigrateValidatorSupernode_IndependentAccountPreserved verifies that when
// the supernode account is a different entity from the validator (already migrated
// independently or set to a separate EVM address), it is NOT overwritten with
// the validator's new address.
func TestMigrateValidatorSupernode_IndependentAccountPreserved(t *testing.T) {
	f := initMockFixture(t)
	oldValAddr := sdk.ValAddress(testAccAddr())
	newValAddr := sdk.ValAddress(testAccAddr())
	newAddr := sdk.AccAddress(newValAddr)
	// Supernode account is a separate address (e.g. already migrated to EVM key).
	independentSNAccount := testAccAddr().String()

	sn := sntypes.SuperNode{
		ValidatorAddress: oldValAddr.String(),
		SupernodeAccount: independentSNAccount,
		PrevSupernodeAccounts: []*sntypes.SupernodeAccountHistory{
			{Account: independentSNAccount, Height: 100},
		},
	}

	f.supernodeKeeper.EXPECT().QuerySuperNode(gomock.Any(), oldValAddr).Return(sn, true)
	f.supernodeKeeper.EXPECT().DeleteSuperNode(gomock.Any(), oldValAddr)
	f.supernodeKeeper.EXPECT().GetMetricsState(gomock.Any(), oldValAddr).Return(sntypes.SupernodeMetricsState{}, false)
	f.supernodeKeeper.EXPECT().SetSuperNode(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ any, updated sntypes.SuperNode) error {
			// Validator address should be re-keyed.
			require.Equal(t, sdk.ValAddress(newAddr).String(), updated.ValidatorAddress)
			// Supernode account should be preserved (not overwritten).
			require.Equal(t, independentSNAccount, updated.SupernodeAccount)
			// History should be unchanged — no entries added or modified since the
			// supernode account is independent from the validator.
			require.Len(t, updated.PrevSupernodeAccounts, 1)
			require.Equal(t, independentSNAccount, updated.PrevSupernodeAccounts[0].Account)
			return nil
		})

	err := f.keeper.MigrateValidatorSupernode(f.ctx, oldValAddr, newValAddr, sdk.AccAddress(oldValAddr), newAddr)
	require.NoError(t, err)
}

// --- FinalizeVestingAccount tests for all vesting types ---

// TestFinalizeVestingAccount_Delayed verifies that a DelayedVestingAccount
// is correctly recreated at the new address.
func TestFinalizeVestingAccount_Delayed(t *testing.T) {
	f := initMockFixture(t)
	newAddr := testAccAddr()
	baseAcc := authtypes.NewBaseAccountWithAddress(newAddr)

	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), newAddr).Return(baseAcc)
	f.accountKeeper.EXPECT().SetAccount(gomock.Any(), gomock.Any()).Do(func(_ any, acc sdk.AccountI) {
		_, ok := acc.(*vestingtypes.DelayedVestingAccount)
		require.True(t, ok, "should create a DelayedVestingAccount")
	})

	vi := &keeper.VestingInfo{
		Type:            keeper.VestingTypeDelayed,
		OriginalVesting: sdk.NewCoins(sdk.NewInt64Coin("ulume", 500)),
		EndTime:         2000000,
	}

	err := f.keeper.FinalizeVestingAccount(f.ctx, newAddr, vi)
	require.NoError(t, err)
}

// TestFinalizeVestingAccount_Periodic verifies that a PeriodicVestingAccount
// is correctly recreated with the original periods.
func TestFinalizeVestingAccount_Periodic(t *testing.T) {
	f := initMockFixture(t)
	newAddr := testAccAddr()
	baseAcc := authtypes.NewBaseAccountWithAddress(newAddr)

	periods := vestingtypes.Periods{
		{Length: 100000, Amount: sdk.NewCoins(sdk.NewInt64Coin("ulume", 500))},
		{Length: 200000, Amount: sdk.NewCoins(sdk.NewInt64Coin("ulume", 500))},
	}

	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), newAddr).Return(baseAcc)
	f.accountKeeper.EXPECT().SetAccount(gomock.Any(), gomock.Any()).Do(func(_ any, acc sdk.AccountI) {
		pva, ok := acc.(*vestingtypes.PeriodicVestingAccount)
		require.True(t, ok, "should create a PeriodicVestingAccount")
		require.Len(t, pva.VestingPeriods, 2)
	})

	vi := &keeper.VestingInfo{
		Type:            keeper.VestingTypePeriodic,
		OriginalVesting: sdk.NewCoins(sdk.NewInt64Coin("ulume", 1000)),
		EndTime:         3000000,
		StartTime:       1000000,
		Periods:         periods,
	}

	err := f.keeper.FinalizeVestingAccount(f.ctx, newAddr, vi)
	require.NoError(t, err)
}

// TestFinalizeVestingAccount_PermanentLocked verifies that a PermanentLockedAccount
// is correctly recreated at the new address.
func TestFinalizeVestingAccount_PermanentLocked(t *testing.T) {
	f := initMockFixture(t)
	newAddr := testAccAddr()
	baseAcc := authtypes.NewBaseAccountWithAddress(newAddr)

	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), newAddr).Return(baseAcc)
	f.accountKeeper.EXPECT().SetAccount(gomock.Any(), gomock.Any()).Do(func(_ any, acc sdk.AccountI) {
		_, ok := acc.(*vestingtypes.PermanentLockedAccount)
		require.True(t, ok, "should create a PermanentLockedAccount")
	})

	vi := &keeper.VestingInfo{
		Type:            keeper.VestingTypePermanentLocked,
		OriginalVesting: sdk.NewCoins(sdk.NewInt64Coin("ulume", 1000)),
	}

	err := f.keeper.FinalizeVestingAccount(f.ctx, newAddr, vi)
	require.NoError(t, err)
}

// TestFinalizeVestingAccount_NonBaseAccountFallback verifies that when the new
// account is not a *BaseAccount, a BaseAccount is extracted and used.
func TestFinalizeVestingAccount_NonBaseAccountFallback(t *testing.T) {
	f := initMockFixture(t)
	newAddr := testAccAddr()

	// Return a ContinuousVestingAccount (not a *BaseAccount) as the existing account.
	baseAcc := authtypes.NewBaseAccountWithAddress(newAddr)
	origVesting := sdk.NewCoins(sdk.NewInt64Coin("ulume", 500))
	bva, err := vestingtypes.NewBaseVestingAccount(baseAcc, origVesting, 999999)
	require.NoError(t, err)
	existingCVA := vestingtypes.NewContinuousVestingAccountRaw(bva, 100000)

	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), newAddr).Return(existingCVA)
	f.accountKeeper.EXPECT().SetAccount(gomock.Any(), gomock.Any()).Do(func(_ any, acc sdk.AccountI) {
		_, ok := acc.(*vestingtypes.DelayedVestingAccount)
		require.True(t, ok, "should create a DelayedVestingAccount even from non-base account")
	})

	vi := &keeper.VestingInfo{
		Type:            keeper.VestingTypeDelayed,
		OriginalVesting: sdk.NewCoins(sdk.NewInt64Coin("ulume", 1000)),
		EndTime:         5000000,
	}

	err = f.keeper.FinalizeVestingAccount(f.ctx, newAddr, vi)
	require.NoError(t, err)
}

// --- Params endpoint validation tests ---

// TestQueryParams_NilRequest verifies that a nil request returns an error.
func TestQueryParams_NilRequest(t *testing.T) {
	f := initMockFixture(t)
	qs := keeper.NewQueryServerImpl(f.keeper)

	resp, err := qs.Params(f.ctx, nil)
	require.Error(t, err)
	require.Nil(t, resp)
	require.Contains(t, err.Error(), "invalid request")
}

// TestQueryParams_Valid verifies that a valid request returns params.
func TestQueryParams_Valid(t *testing.T) {
	f := initMockFixture(t)
	qs := keeper.NewQueryServerImpl(f.keeper)

	resp, err := qs.Params(f.ctx, &types.QueryParamsRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.True(t, resp.Params.EnableMigration)
}

// TestUpdateParams_InvalidAuthority verifies that UpdateParams rejects
// requests from non-authority addresses.
func TestUpdateParams_InvalidAuthority(t *testing.T) {
	f := initMockFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	badAuthority := testAccAddr()
	req := &types.MsgUpdateParams{
		Authority: badAuthority.String(),
		Params:    types.DefaultParams(),
	}

	_, err := ms.UpdateParams(f.ctx, req)
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrInvalidSigner)
}

// TestUpdateParams_ValidAuthority verifies that UpdateParams succeeds with
// the correct authority and valid params.
func TestUpdateParams_ValidAuthority(t *testing.T) {
	f := initMockFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	authority := authtypes.NewModuleAddress(types.GovModuleName)
	newParams := types.NewParams(false, 100, 25, 1000, 20)
	req := &types.MsgUpdateParams{
		Authority: authority.String(),
		Params:    newParams,
	}

	resp, err := ms.UpdateParams(f.ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Verify params were updated.
	got, err := f.keeper.Params.Get(f.ctx)
	require.NoError(t, err)
	require.Equal(t, false, got.EnableMigration)
	require.Equal(t, int64(100), got.MigrationEndTime)
	require.Equal(t, uint64(25), got.MaxMigrationsPerBlock)
}
