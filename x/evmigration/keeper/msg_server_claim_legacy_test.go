package keeper_test

import (
	"fmt"
	"testing"
	"time"

	"cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	addresscodec "github.com/cosmos/cosmos-sdk/codec/address"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/cosmos/cosmos-sdk/runtime"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	moduletestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/cosmos/cosmos-sdk/x/authz"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	evmcryptotypes "github.com/cosmos/evm/crypto/ethsecp256k1"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/LumeraProtocol/lumera/x/evmigration/keeper"
	evmigrationmocks "github.com/LumeraProtocol/lumera/x/evmigration/mocks"
	module "github.com/LumeraProtocol/lumera/x/evmigration/module"
	"github.com/LumeraProtocol/lumera/x/evmigration/types"
	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

// msgServerFixture extends mockFixture with a message server for testing
// the full ClaimLegacyAccount and MigrateValidator message handlers.
type msgServerFixture struct {
	*mockFixture
	msgServer types.MsgServer
}

// newSingleKeyProofNew builds a valid new-side MigrationProof (eth_secp256k1,
// 65-byte EIP-191 signature) for use in msg-server test fixtures.
func newSingleKeyProofNew(pk []byte, sig []byte, format types.SigFormat) types.MigrationProof {
	return types.MigrationProof{Proof: &types.MigrationProof_Single{Single: &types.SingleKeyProof{
		PubKey: pk, Signature: sig, SigFormat: format,
	}}}
}

func newClaimMigrationMsg(
	t *testing.T,
	legacyPrivKey *secp256k1.PrivKey,
	legacyAddr sdk.AccAddress,
	newPrivKey *evmcryptotypes.PrivKey,
	newAddr sdk.AccAddress,
) *types.MsgClaimLegacyAccount {
	t.Helper()

	return &types.MsgClaimLegacyAccount{
		LegacyAddress: legacyAddr.String(),
		NewAddress:    newAddr.String(),
		LegacyProof: types.MigrationProof{Proof: &types.MigrationProof_Single{Single: &types.SingleKeyProof{
			PubKey:    legacyPrivKey.PubKey().(*secp256k1.PubKey).Key,
			Signature: signMigrationMessage(t, legacyPrivKey, legacyAddr, newAddr),
			SigFormat: types.SigFormat_SIG_FORMAT_CLI,
		}}},
		NewProof: newSingleKeyProofNew(
			newPrivKey.PubKey().Bytes(),
			signNewMigrationEIP191(t, keeperClaimKind, newPrivKey, legacyAddr, newAddr),
			types.SigFormat_SIG_FORMAT_EIP191,
		),
	}
}

func newValidatorMigrationMsg(
	t *testing.T,
	legacyPrivKey *secp256k1.PrivKey,
	legacyAddr sdk.AccAddress,
	newPrivKey *evmcryptotypes.PrivKey,
	newAddr sdk.AccAddress,
) *types.MsgMigrateValidator {
	t.Helper()

	return &types.MsgMigrateValidator{
		LegacyAddress: legacyAddr.String(),
		NewAddress:    newAddr.String(),
		LegacyProof: types.MigrationProof{Proof: &types.MigrationProof_Single{Single: &types.SingleKeyProof{
			PubKey:    legacyPrivKey.PubKey().(*secp256k1.PubKey).Key,
			Signature: signLegacyMigrationMessage(t, keeperValidatorKind, legacyPrivKey, legacyAddr, newAddr),
			SigFormat: types.SigFormat_SIG_FORMAT_CLI,
		}}},
		NewProof: newSingleKeyProofNew(
			newPrivKey.PubKey().Bytes(),
			signNewMigrationEIP191(t, keeperValidatorKind, newPrivKey, legacyAddr, newAddr),
			types.SigFormat_SIG_FORMAT_EIP191,
		),
	}
}

func initMsgServerFixture(t *testing.T) *msgServerFixture {
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
	claimKeeper := evmigrationmocks.NewMockClaimKeeper(ctrl)

	encCfg := moduletestutil.MakeTestEncodingConfig(module.AppModule{})
	addrCodec := addresscodec.NewBech32Codec(sdk.GetConfig().GetBech32AccountAddrPrefix())
	storeKey := storetypes.NewKVStoreKey(types.StoreKey)
	stakingStoreKey := storetypes.NewKVStoreKey(stakingtypes.StoreKey)
	storeService := runtime.NewKVStoreService(storeKey)
	stakingStoreService := runtime.NewKVStoreService(stakingStoreKey)
	ctx := testutil.DefaultContextWithKeys(
		map[string]*storetypes.KVStoreKey{
			types.StoreKey:        storeKey,
			stakingtypes.StoreKey: stakingStoreKey,
		},
		map[string]*storetypes.TransientStoreKey{"transient_test": storetypes.NewTransientStoreKey("transient_test")},
		nil,
	).WithChainID(testChainID).WithBlockTime(time.Now())

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
		claimKeeper,
	)

	// Wire an isolated staking store service so scoped redelegation iteration
	// and DeleteValidatorRecordNoHooks can run in unit tests. Production wiring
	// happens in app.go.
	k.SetStakingStoreService(stakingStoreService)

	// Initialize params with migration enabled.
	params := types.NewParams(true, 0, 50, 2000, 20)
	require.NoError(t, k.Params.Set(ctx, params))
	require.NoError(t, k.MigrationCounter.Set(ctx, 0))
	require.NoError(t, k.ValidatorMigrationCounter.Set(ctx, 0))

	mf := &mockFixture{
		ctx:                ctx,
		keeper:             k,
		cdc:                encCfg.Codec,
		stakingStore:       stakingStoreService,
		accountKeeper:      accountKeeper,
		bankKeeper:         bankKeeper,
		stakingKeeper:      stakingKeeper,
		distributionKeeper: distributionKeeper,
		authzKeeper:        authzKeeper,
		feegrantKeeper:     feegrantKeeper,
		supernodeKeeper:    supernodeKeeper,
		actionKeeper:       actionKeeper,
		claimKeeper:        claimKeeper,
	}

	return &msgServerFixture{
		mockFixture: mf,
		msgServer:   keeper.NewMsgServerImpl(k),
	}
}

// --- preChecks tests ---

// TestPreChecks_MigrationDisabled verifies that migration is rejected when
// the enable_migration param is false.
func TestPreChecks_MigrationDisabled(t *testing.T) {
	f := initMsgServerFixture(t)

	// Disable migration.
	params := types.NewParams(false, 0, 50, 2000, 20)
	require.NoError(t, f.keeper.Params.Set(f.ctx, params))

	privKey := secp256k1.GenPrivKey()
	legacyAddr := sdk.AccAddress(privKey.PubKey().Address())
	newPrivKey, newAddr := testNewMigrationAccount(t)

	msg := newClaimMigrationMsg(t, privKey, legacyAddr, newPrivKey, newAddr)

	_, err := f.msgServer.ClaimLegacyAccount(f.ctx, msg)
	require.ErrorIs(t, err, types.ErrMigrationDisabled)
}

// TestPreChecks_MigrationWindowClosed verifies that migration is rejected
// after the configured end time.
func TestPreChecks_MigrationWindowClosed(t *testing.T) {
	f := initMsgServerFixture(t)

	// Set migration end time in the past.
	pastTime := f.ctx.BlockTime().Add(-1 * time.Hour).Unix()
	params := types.NewParams(true, pastTime, 50, 2000, 20)
	require.NoError(t, f.keeper.Params.Set(f.ctx, params))

	privKey := secp256k1.GenPrivKey()
	legacyAddr := sdk.AccAddress(privKey.PubKey().Address())
	newPrivKey, newAddr := testNewMigrationAccount(t)

	msg := newClaimMigrationMsg(t, privKey, legacyAddr, newPrivKey, newAddr)

	_, err := f.msgServer.ClaimLegacyAccount(f.ctx, msg)
	require.ErrorIs(t, err, types.ErrMigrationWindowClosed)
}

// TestPreChecks_BlockRateLimitExceeded verifies that migration is rejected
// when the per-block migration count exceeds the configured limit.
func TestPreChecks_BlockRateLimitExceeded(t *testing.T) {
	f := initMsgServerFixture(t)

	// Set block counter to max.
	require.NoError(t, f.keeper.BlockMigrationCounter.Set(f.ctx, f.ctx.BlockHeight(), 50))

	privKey := secp256k1.GenPrivKey()
	legacyAddr := sdk.AccAddress(privKey.PubKey().Address())
	newPrivKey, newAddr := testNewMigrationAccount(t)

	msg := newClaimMigrationMsg(t, privKey, legacyAddr, newPrivKey, newAddr)

	_, err := f.msgServer.ClaimLegacyAccount(f.ctx, msg)
	require.ErrorIs(t, err, types.ErrBlockRateLimitExceeded)
}

// TestPreChecks_SameAddress verifies that migration is rejected when legacy
// and new addresses are identical.
func TestPreChecks_SameAddress(t *testing.T) {
	f := initMsgServerFixture(t)

	privKey := secp256k1.GenPrivKey()
	addr := sdk.AccAddress(privKey.PubKey().Address())
	newPrivKey, _ := testNewMigrationAccount(t)

	msg := newClaimMigrationMsg(t, privKey, addr, newPrivKey, addr)

	_, err := f.msgServer.ClaimLegacyAccount(f.ctx, msg)
	require.ErrorIs(t, err, types.ErrSameAddress)
}

// TestPreChecks_AlreadyMigrated verifies that a legacy address cannot be
// migrated twice.
func TestPreChecks_AlreadyMigrated(t *testing.T) {
	f := initMsgServerFixture(t)

	privKey := secp256k1.GenPrivKey()
	legacyAddr := sdk.AccAddress(privKey.PubKey().Address())
	newPrivKey, newAddr := testNewMigrationAccount(t)

	// Store a migration record for the legacy address.
	require.NoError(t, f.keeper.MigrationRecords.Set(f.ctx, legacyAddr.String(), types.MigrationRecord{
		LegacyAddress: legacyAddr.String(),
		NewAddress:    newAddr.String(),
	}))

	msg := newClaimMigrationMsg(t, privKey, legacyAddr, newPrivKey, newAddr)

	_, err := f.msgServer.ClaimLegacyAccount(f.ctx, msg)
	require.ErrorIs(t, err, types.ErrAlreadyMigrated)
}

// TestPreChecks_NewAddressWasMigrated verifies that a new address cannot be
// a previously-migrated legacy address.
func TestPreChecks_NewAddressWasMigrated(t *testing.T) {
	f := initMsgServerFixture(t)

	privKey := secp256k1.GenPrivKey()
	legacyAddr := sdk.AccAddress(privKey.PubKey().Address())
	newPrivKey, newAddr := testNewMigrationAccount(t)

	// Store a migration record where newAddr was a legacy address.
	require.NoError(t, f.keeper.MigrationRecords.Set(f.ctx, newAddr.String(), types.MigrationRecord{
		LegacyAddress: newAddr.String(),
		NewAddress:    testAccAddr().String(),
	}))

	msg := &types.MsgClaimLegacyAccount{
		LegacyAddress: legacyAddr.String(),
		NewAddress:    newAddr.String(),
		LegacyProof: types.MigrationProof{Proof: &types.MigrationProof_Single{Single: &types.SingleKeyProof{
			PubKey:    privKey.PubKey().(*secp256k1.PubKey).Key,
			Signature: signMigrationMessage(t, privKey, legacyAddr, newAddr),
			SigFormat: types.SigFormat_SIG_FORMAT_CLI,
		}}},
		NewProof: types.MigrationProof{Proof: &types.MigrationProof_Single{Single: &types.SingleKeyProof{
			Signature: signNewMigrationMessage(t, keeperClaimKind, newPrivKey, legacyAddr, newAddr),
			SigFormat: types.SigFormat_SIG_FORMAT_CLI,
		}}},
	}

	_, err := f.msgServer.ClaimLegacyAccount(f.ctx, msg)
	require.ErrorIs(t, err, types.ErrNewAddressWasMigrated)
}

// TestPreChecks_NewAddressAlreadyUsed verifies that a new address cannot be
// used if it was already the destination of a prior migration.
func TestPreChecks_NewAddressAlreadyUsed(t *testing.T) {
	f := initMsgServerFixture(t)

	privKey := secp256k1.GenPrivKey()
	legacyAddr := sdk.AccAddress(privKey.PubKey().Address())
	newPrivKey, newAddr := testNewMigrationAccount(t)

	// Store a reverse index entry: newAddr was already used as a destination.
	require.NoError(t, f.keeper.MigrationRecordByNewAddress.Set(f.ctx, newAddr.String(), testAccAddr().String()))

	msg := &types.MsgClaimLegacyAccount{
		LegacyAddress: legacyAddr.String(),
		NewAddress:    newAddr.String(),
		LegacyProof: types.MigrationProof{Proof: &types.MigrationProof_Single{Single: &types.SingleKeyProof{
			PubKey:    privKey.PubKey().(*secp256k1.PubKey).Key,
			Signature: signMigrationMessage(t, privKey, legacyAddr, newAddr),
			SigFormat: types.SigFormat_SIG_FORMAT_CLI,
		}}},
		NewProof: types.MigrationProof{Proof: &types.MigrationProof_Single{Single: &types.SingleKeyProof{
			Signature: signNewMigrationMessage(t, keeperClaimKind, newPrivKey, legacyAddr, newAddr),
			SigFormat: types.SigFormat_SIG_FORMAT_CLI,
		}}},
	}

	_, err := f.msgServer.ClaimLegacyAccount(f.ctx, msg)
	require.ErrorIs(t, err, types.ErrNewAddressAlreadyUsed)
}

// TestPreChecks_ModuleAccount verifies that module accounts cannot be migrated.
func TestPreChecks_ModuleAccount(t *testing.T) {
	f := initMsgServerFixture(t)

	privKey := secp256k1.GenPrivKey()
	legacyAddr := sdk.AccAddress(privKey.PubKey().Address())
	newPrivKey, newAddr := testNewMigrationAccount(t)

	modAcc := authtypes.NewEmptyModuleAccount("bonded_tokens_pool")
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), legacyAddr).Return(modAcc)

	msg := newClaimMigrationMsg(t, privKey, legacyAddr, newPrivKey, newAddr)

	_, err := f.msgServer.ClaimLegacyAccount(f.ctx, msg)
	require.ErrorIs(t, err, types.ErrCannotMigrateModuleAccount)
}

// TestPreChecks_LegacyAccountNotFound verifies error when legacy account
// does not exist in x/auth.
func TestPreChecks_LegacyAccountNotFound(t *testing.T) {
	f := initMsgServerFixture(t)

	privKey := secp256k1.GenPrivKey()
	legacyAddr := sdk.AccAddress(privKey.PubKey().Address())
	newPrivKey, newAddr := testNewMigrationAccount(t)

	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), legacyAddr).Return(nil)

	msg := newClaimMigrationMsg(t, privKey, legacyAddr, newPrivKey, newAddr)

	_, err := f.msgServer.ClaimLegacyAccount(f.ctx, msg)
	require.ErrorIs(t, err, types.ErrLegacyAccountNotFound)
}

// --- ClaimLegacyAccount tests ---

// TestClaimLegacyAccount_ValidatorMustUseMigrateValidator verifies that a validator
// operator is rejected by ClaimLegacyAccount and directed to MigrateValidator.
func TestClaimLegacyAccount_ValidatorMustUseMigrateValidator(t *testing.T) {
	f := initMsgServerFixture(t)

	privKey := secp256k1.GenPrivKey()
	legacyAddr := sdk.AccAddress(privKey.PubKey().Address())
	newPrivKey, newAddr := testNewMigrationAccount(t)

	baseAcc := authtypes.NewBaseAccountWithAddress(legacyAddr)
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), legacyAddr).Return(baseAcc)

	// Legacy address is a validator.
	valAddr := sdk.ValAddress(legacyAddr)
	f.stakingKeeper.EXPECT().GetValidator(gomock.Any(), valAddr).Return(
		stakingtypes.Validator{OperatorAddress: legacyAddr.String()}, nil,
	)

	msg := &types.MsgClaimLegacyAccount{
		LegacyAddress: legacyAddr.String(),
		NewAddress:    newAddr.String(),
		LegacyProof: types.MigrationProof{Proof: &types.MigrationProof_Single{Single: &types.SingleKeyProof{
			PubKey:    privKey.PubKey().(*secp256k1.PubKey).Key,
			Signature: signMigrationMessage(t, privKey, legacyAddr, newAddr),
			SigFormat: types.SigFormat_SIG_FORMAT_CLI,
		}}},
		NewProof: types.MigrationProof{Proof: &types.MigrationProof_Single{Single: &types.SingleKeyProof{
			Signature: signNewMigrationMessage(t, keeperClaimKind, newPrivKey, legacyAddr, newAddr),
			SigFormat: types.SigFormat_SIG_FORMAT_CLI,
		}}},
	}

	_, err := f.msgServer.ClaimLegacyAccount(f.ctx, msg)
	require.ErrorIs(t, err, types.ErrUseValidatorMigration)
}

// TestClaimLegacyAccount_InvalidSignature verifies that an invalid legacy
// signature is rejected.
func TestClaimLegacyAccount_InvalidSignature(t *testing.T) {
	f := initMsgServerFixture(t)

	privKey := secp256k1.GenPrivKey()
	pubKey := privKey.PubKey().(*secp256k1.PubKey)
	legacyAddr := sdk.AccAddress(pubKey.Address())
	newPrivKey, newAddr := testNewMigrationAccount(t)

	baseAcc := authtypes.NewBaseAccountWithAddress(legacyAddr)
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), legacyAddr).Return(baseAcc)

	// Not a validator.
	valAddr := sdk.ValAddress(legacyAddr)
	f.stakingKeeper.EXPECT().GetValidator(gomock.Any(), valAddr).Return(
		stakingtypes.Validator{}, stakingtypes.ErrNoValidatorFound,
	)

	msg := newClaimMigrationMsg(t, privKey, legacyAddr, newPrivKey, newAddr)
	msg.LegacyProof.Proof.(*types.MigrationProof_Single).Single.Signature = []byte("bad-signature")

	_, err := f.msgServer.ClaimLegacyAccount(f.ctx, msg)
	require.ErrorIs(t, err, types.ErrInvalidMigrationSignature)
}

// TestClaimLegacyAccount_Success verifies the full happy-path claim flow:
// preChecks pass, signature verified, account migrated, record stored, counters incremented.
func TestClaimLegacyAccount_Success(t *testing.T) {
	f := initMsgServerFixture(t)

	privKey := secp256k1.GenPrivKey()
	legacyAddr := sdk.AccAddress(privKey.PubKey().Address())
	newPrivKey, newAddr := testNewMigrationAccount(t)

	baseAcc := authtypes.NewBaseAccountWithAddress(legacyAddr)
	valAddr := sdk.ValAddress(legacyAddr)

	// preChecks: account exists and is not a module account.
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), legacyAddr).Return(baseAcc)

	// Not a validator.
	f.stakingKeeper.EXPECT().GetValidator(gomock.Any(), valAddr).Return(
		stakingtypes.Validator{}, stakingtypes.ErrNoValidatorFound,
	)

	// migrateAccount steps:
	// Step 1: MigrateDistribution — no delegations.
	f.stakingKeeper.EXPECT().GetDelegatorDelegations(gomock.Any(), legacyAddr, ^uint16(0)).Return(nil, nil)

	// Step 2: MigrateStaking — no delegations, unbondings, or redelegations.
	f.stakingKeeper.EXPECT().GetDelegatorDelegations(gomock.Any(), legacyAddr, ^uint16(0)).Return(nil, nil)
	f.stakingKeeper.EXPECT().GetUnbondingDelegations(gomock.Any(), legacyAddr, ^uint16(0)).Return(nil, nil)
	f.stakingKeeper.EXPECT().GetRedelegations(gomock.Any(), legacyAddr, ^uint16(0)).Return(nil, nil)
	f.distributionKeeper.EXPECT().GetDelegatorWithdrawAddr(gomock.Any(), legacyAddr).Return(legacyAddr, nil).Times(2)
	f.distributionKeeper.EXPECT().SetDelegatorWithdrawAddr(gomock.Any(), newAddr, newAddr).Return(nil)

	// Step 3a: MigrateAuth — base account.
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), legacyAddr).Return(baseAcc)
	f.accountKeeper.EXPECT().RemoveAccount(gomock.Any(), baseAcc)
	newAcc := authtypes.NewBaseAccountWithAddress(newAddr)
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), newAddr).Return(nil)
	f.accountKeeper.EXPECT().NewAccountWithAddress(gomock.Any(), newAddr).Return(newAcc)
	f.accountKeeper.EXPECT().SetAccount(gomock.Any(), newAcc)

	// Step 3b: MigrateBank — some balance.
	balances := sdk.NewCoins(sdk.NewInt64Coin("ulume", 1000))
	f.bankKeeper.EXPECT().GetAllBalances(gomock.Any(), legacyAddr).Return(balances)
	f.bankKeeper.EXPECT().SendCoins(gomock.Any(), legacyAddr, newAddr, balances).Return(nil)

	// Step 4: MigrateAuthz — no grants.
	f.authzKeeper.EXPECT().IterateGrants(gomock.Any(), gomock.Any())

	// Step 5: MigrateFeegrant — no allowances.
	f.feegrantKeeper.EXPECT().IterateAllFeeAllowances(gomock.Any(), gomock.Any()).Return(nil)

	// Step 6: MigrateSupernode — not a supernode.
	f.supernodeKeeper.EXPECT().GetSuperNodeByAccount(gomock.Any(), legacyAddr.String()).Return(
		sntypes.SuperNode{}, false, nil,
	)

	// Step 7: MigrateActions — no matching actions.
	f.actionKeeper.EXPECT().GetActionsByCreator(gomock.Any(), gomock.Any()).Return(nil, nil)
	f.actionKeeper.EXPECT().GetActionsBySuperNode(gomock.Any(), gomock.Any()).Return(nil, nil)

	// Step 8: MigrateClaim — no claim records targeting this address.
	f.claimKeeper.EXPECT().IterateClaimRecords(gomock.Any(), gomock.Any()).Return(nil)

	msg := newClaimMigrationMsg(t, privKey, legacyAddr, newPrivKey, newAddr)

	resp, err := f.msgServer.ClaimLegacyAccount(f.ctx, msg)
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

	blockCount, err := f.keeper.BlockMigrationCounter.Get(f.ctx, f.ctx.BlockHeight())
	require.NoError(t, err)
	require.Equal(t, uint64(1), blockCount)

	// Validator counter should NOT be incremented for a regular claim.
	valCount, err := f.keeper.ValidatorMigrationCounter.Get(f.ctx)
	require.NoError(t, err)
	require.Equal(t, uint64(0), valCount)
}

// TestClaimLegacyAccount_MigratedThirdPartyWithdrawAddress verifies the full
// ClaimLegacyAccount flow when the legacy account's withdraw address points to
// a previously-migrated third-party address. This is the end-to-end regression
// test for bug #16: the snapshot of origWithdrawAddr in migrateAccount (before
// MigrateDistribution redirects it to self) must be passed through to
// migrateWithdrawAddress so it resolves via MigrationRecords.
func TestClaimLegacyAccount_MigratedThirdPartyWithdrawAddress(t *testing.T) {
	f := initMsgServerFixture(t)

	privKey := secp256k1.GenPrivKey()
	pubKey := privKey.PubKey().(*secp256k1.PubKey)
	legacyAddr := sdk.AccAddress(pubKey.Address())
	newPrivKey, newAddr := testNewMigrationAccount(t)

	// Third-party withdraw address that was already migrated.
	thirdPartyLegacy := testAccAddr()
	thirdPartyNew := testAccAddr()
	require.NoError(t, f.keeper.MigrationRecords.Set(f.ctx, thirdPartyLegacy.String(), types.MigrationRecord{
		LegacyAddress: thirdPartyLegacy.String(),
		NewAddress:    thirdPartyNew.String(),
	}))

	baseAcc := authtypes.NewBaseAccountWithAddress(legacyAddr)

	// preChecks: account exists and is not a module account.
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), legacyAddr).Return(baseAcc)
	// Not a validator.
	f.stakingKeeper.EXPECT().GetValidator(gomock.Any(), sdk.ValAddress(legacyAddr)).Return(
		stakingtypes.Validator{}, stakingtypes.ErrNoValidatorFound,
	)

	// origWithdrawAddr snapshot: returns thirdPartyLegacy (the pre-redirect value).
	// redirectWithdrawAddrIfMigrated: also reads it, sees it's migrated, resets to self.
	f.distributionKeeper.EXPECT().GetDelegatorWithdrawAddr(gomock.Any(), legacyAddr).Return(thirdPartyLegacy, nil).Times(2)
	// redirectWithdrawAddrIfMigrated resets to self for safe reward withdrawal.
	f.distributionKeeper.EXPECT().SetDelegatorWithdrawAddr(gomock.Any(), legacyAddr, legacyAddr).Return(nil)

	// Step 1: MigrateDistribution — no delegations.
	f.stakingKeeper.EXPECT().GetDelegatorDelegations(gomock.Any(), legacyAddr, ^uint16(0)).Return(nil, nil)

	// Step 2: MigrateStaking — no delegations, unbondings, or redelegations.
	f.stakingKeeper.EXPECT().GetDelegatorDelegations(gomock.Any(), legacyAddr, ^uint16(0)).Return(nil, nil)
	f.stakingKeeper.EXPECT().GetUnbondingDelegations(gomock.Any(), legacyAddr, ^uint16(0)).Return(nil, nil)
	f.stakingKeeper.EXPECT().GetRedelegations(gomock.Any(), legacyAddr, ^uint16(0)).Return(nil, nil)
	// migrateWithdrawAddress: must resolve thirdPartyLegacy → thirdPartyNew via MigrationRecords.
	f.distributionKeeper.EXPECT().SetDelegatorWithdrawAddr(gomock.Any(), newAddr, thirdPartyNew).Return(nil)

	// Step 3a: MigrateAuth — base account.
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), legacyAddr).Return(baseAcc)
	f.accountKeeper.EXPECT().RemoveAccount(gomock.Any(), baseAcc)
	newAcc := authtypes.NewBaseAccountWithAddress(newAddr)
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), newAddr).Return(nil)
	f.accountKeeper.EXPECT().NewAccountWithAddress(gomock.Any(), newAddr).Return(newAcc)
	f.accountKeeper.EXPECT().SetAccount(gomock.Any(), newAcc)

	// Step 3b: MigrateBank — no balance.
	f.bankKeeper.EXPECT().GetAllBalances(gomock.Any(), legacyAddr).Return(sdk.Coins{})

	// Steps 4-8: no authz/feegrant/supernode/action/claim to migrate.
	f.authzKeeper.EXPECT().IterateGrants(gomock.Any(), gomock.Any())
	f.feegrantKeeper.EXPECT().IterateAllFeeAllowances(gomock.Any(), gomock.Any()).Return(nil)
	f.supernodeKeeper.EXPECT().GetSuperNodeByAccount(gomock.Any(), legacyAddr.String()).Return(
		sntypes.SuperNode{}, false, nil,
	)
	f.actionKeeper.EXPECT().GetActionsByCreator(gomock.Any(), gomock.Any()).Return(nil, nil)
	f.actionKeeper.EXPECT().GetActionsBySuperNode(gomock.Any(), gomock.Any()).Return(nil, nil)
	f.claimKeeper.EXPECT().IterateClaimRecords(gomock.Any(), gomock.Any()).Return(nil)

	msg := &types.MsgClaimLegacyAccount{
		LegacyAddress: legacyAddr.String(),
		NewAddress:    newAddr.String(),
		LegacyProof: types.MigrationProof{Proof: &types.MigrationProof_Single{Single: &types.SingleKeyProof{
			PubKey:    pubKey.Key,
			Signature: signMigrationMessage(t, privKey, legacyAddr, newAddr),
			SigFormat: types.SigFormat_SIG_FORMAT_CLI,
		}}},
		NewProof: newSingleKeyProofNew(
			newPrivKey.PubKey().Bytes(),
			signNewMigrationEIP191(t, keeperClaimKind, newPrivKey, legacyAddr, newAddr),
			types.SigFormat_SIG_FORMAT_EIP191,
		),
	}

	resp, err := f.msgServer.ClaimLegacyAccount(f.ctx, msg)
	require.NoError(t, err)
	require.NotNil(t, resp)
}

// --- Failure-path / atomicity tests ---
// These tests verify that when a mid-migration step fails, the error propagates
// to the caller (so CacheMultiStore rolls back) and no migration record or
// counter increment is committed.

// setupPassingPreChecks configures mocks so that preChecks and signature
// verification pass, returning the legacy/new addresses and the ready message.
func setupPassingPreChecks(t *testing.T, f *msgServerFixture) (
	*secp256k1.PrivKey, sdk.AccAddress, sdk.AccAddress, *types.MsgClaimLegacyAccount,
) {
	t.Helper()
	privKey := secp256k1.GenPrivKey()
	legacyAddr := sdk.AccAddress(privKey.PubKey().Address())
	newPrivKey, newAddr := testNewMigrationAccount(t)

	baseAcc := authtypes.NewBaseAccountWithAddress(legacyAddr)
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), legacyAddr).Return(baseAcc)
	f.stakingKeeper.EXPECT().GetValidator(gomock.Any(), sdk.ValAddress(legacyAddr)).Return(
		stakingtypes.Validator{}, stakingtypes.ErrNoValidatorFound,
	)

	msg := newClaimMigrationMsg(t, privKey, legacyAddr, newPrivKey, newAddr)

	return privKey, legacyAddr, newAddr, msg
}

// assertNoFinalization verifies that no migration record or counter was stored.
func assertNoFinalization(t *testing.T, f *msgServerFixture, legacyAddr sdk.AccAddress) {
	t.Helper()
	has, err := f.keeper.MigrationRecords.Has(f.ctx, legacyAddr.String())
	require.NoError(t, err)
	require.False(t, has, "migration record should not exist after failed migration")

	count, err := f.keeper.MigrationCounter.Get(f.ctx)
	require.NoError(t, err)
	require.Equal(t, uint64(0), count, "migration counter should remain 0")
}

// TestClaimLegacyAccount_FailAtDistribution verifies that a failure in
// MigrateDistribution (step 1) propagates and no record is stored.
func TestClaimLegacyAccount_FailAtDistribution(t *testing.T) {
	f := initMsgServerFixture(t)
	_, legacyAddr, _, msg := setupPassingPreChecks(t, f)

	// Snapshot + redirectWithdrawAddrIfMigrated both call GetDelegatorWithdrawAddr.
	f.distributionKeeper.EXPECT().GetDelegatorWithdrawAddr(gomock.Any(), legacyAddr).Return(legacyAddr, nil).Times(2)
	// Step 1: MigrateDistribution fails — GetDelegatorDelegations returns error.
	f.stakingKeeper.EXPECT().GetDelegatorDelegations(gomock.Any(), legacyAddr, ^uint16(0)).Return(
		nil, fmt.Errorf("staking store corrupted"),
	)

	_, err := f.msgServer.ClaimLegacyAccount(f.ctx, msg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "migrate distribution")
	assertNoFinalization(t, f, legacyAddr)
}

// TestClaimLegacyAccount_FailAtStaking verifies that a failure in
// MigrateStaking (step 2) propagates and no record is stored.
func TestClaimLegacyAccount_FailAtStaking(t *testing.T) {
	f := initMsgServerFixture(t)
	_, legacyAddr, _, msg := setupPassingPreChecks(t, f)

	// Snapshot + redirectWithdrawAddrIfMigrated both call GetDelegatorWithdrawAddr.
	f.distributionKeeper.EXPECT().GetDelegatorWithdrawAddr(gomock.Any(), legacyAddr).Return(legacyAddr, nil).Times(2)
	// Step 1: MigrateDistribution succeeds (no delegations).
	f.stakingKeeper.EXPECT().GetDelegatorDelegations(gomock.Any(), legacyAddr, ^uint16(0)).Return(nil, nil)

	// Step 2: MigrateStaking — migrateActiveDelegations fails.
	f.stakingKeeper.EXPECT().GetDelegatorDelegations(gomock.Any(), legacyAddr, ^uint16(0)).Return(
		nil, fmt.Errorf("staking index corrupted"),
	)

	_, err := f.msgServer.ClaimLegacyAccount(f.ctx, msg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "migrate staking")
	assertNoFinalization(t, f, legacyAddr)
}

// TestClaimLegacyAccount_FailAtBank verifies that a failure in MigrateBank
// (step 3b) propagates after auth was already removed, and no record is stored.
// This is the most critical atomicity test: auth account is removed in step 3a,
// then bank fails in 3b. The SDK's CacheMultiStore ensures both are rolled back.
func TestClaimLegacyAccount_FailAtBank(t *testing.T) {
	f := initMsgServerFixture(t)
	_, legacyAddr, newAddr, msg := setupPassingPreChecks(t, f)

	// Step 1: MigrateDistribution — no delegations.
	f.stakingKeeper.EXPECT().GetDelegatorDelegations(gomock.Any(), legacyAddr, ^uint16(0)).Return(nil, nil)

	// Step 2: MigrateStaking — no delegations, unbondings, or redelegations.
	f.stakingKeeper.EXPECT().GetDelegatorDelegations(gomock.Any(), legacyAddr, ^uint16(0)).Return(nil, nil)
	f.stakingKeeper.EXPECT().GetUnbondingDelegations(gomock.Any(), legacyAddr, ^uint16(0)).Return(nil, nil)
	f.stakingKeeper.EXPECT().GetRedelegations(gomock.Any(), legacyAddr, ^uint16(0)).Return(nil, nil)
	f.distributionKeeper.EXPECT().GetDelegatorWithdrawAddr(gomock.Any(), legacyAddr).Return(legacyAddr, nil).Times(2)
	f.distributionKeeper.EXPECT().SetDelegatorWithdrawAddr(gomock.Any(), newAddr, newAddr).Return(nil)

	// Step 3a: MigrateAuth succeeds — removes legacy account.
	baseAcc := authtypes.NewBaseAccountWithAddress(legacyAddr)
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), legacyAddr).Return(baseAcc)
	f.accountKeeper.EXPECT().RemoveAccount(gomock.Any(), baseAcc)
	newAcc := authtypes.NewBaseAccountWithAddress(newAddr)
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), newAddr).Return(nil)
	f.accountKeeper.EXPECT().NewAccountWithAddress(gomock.Any(), newAddr).Return(newAcc)
	f.accountKeeper.EXPECT().SetAccount(gomock.Any(), newAcc)

	// Step 3b: MigrateBank FAILS.
	balances := sdk.NewCoins(sdk.NewInt64Coin("ulume", 1000))
	f.bankKeeper.EXPECT().GetAllBalances(gomock.Any(), legacyAddr).Return(balances)
	f.bankKeeper.EXPECT().SendCoins(gomock.Any(), legacyAddr, newAddr, balances).Return(
		fmt.Errorf("insufficient funds in module account"),
	)

	_, err := f.msgServer.ClaimLegacyAccount(f.ctx, msg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "migrate bank")
	assertNoFinalization(t, f, legacyAddr)
}

// TestClaimLegacyAccount_FailAtAuthz verifies that a failure in MigrateAuthz
// (step 4) propagates and no record is stored.
func TestClaimLegacyAccount_FailAtAuthz(t *testing.T) {
	f := initMsgServerFixture(t)
	_, legacyAddr, newAddr, msg := setupPassingPreChecks(t, f)

	// Steps 1-3 succeed (no delegations, base account, zero balance).
	f.stakingKeeper.EXPECT().GetDelegatorDelegations(gomock.Any(), legacyAddr, ^uint16(0)).Return(nil, nil).Times(2)
	f.stakingKeeper.EXPECT().GetUnbondingDelegations(gomock.Any(), legacyAddr, ^uint16(0)).Return(nil, nil)
	f.stakingKeeper.EXPECT().GetRedelegations(gomock.Any(), legacyAddr, ^uint16(0)).Return(nil, nil)
	f.distributionKeeper.EXPECT().GetDelegatorWithdrawAddr(gomock.Any(), legacyAddr).Return(legacyAddr, nil).Times(2)
	f.distributionKeeper.EXPECT().SetDelegatorWithdrawAddr(gomock.Any(), newAddr, newAddr).Return(nil)

	baseAcc := authtypes.NewBaseAccountWithAddress(legacyAddr)
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), legacyAddr).Return(baseAcc)
	f.accountKeeper.EXPECT().RemoveAccount(gomock.Any(), baseAcc)
	newAcc := authtypes.NewBaseAccountWithAddress(newAddr)
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), newAddr).Return(nil)
	f.accountKeeper.EXPECT().NewAccountWithAddress(gomock.Any(), newAddr).Return(newAcc)
	f.accountKeeper.EXPECT().SetAccount(gomock.Any(), newAcc)

	f.bankKeeper.EXPECT().GetAllBalances(gomock.Any(), legacyAddr).Return(sdk.Coins{})

	// Step 4: MigrateAuthz — DeleteGrant fails.
	genericAuth := authz.NewGenericAuthorization("/cosmos.bank.v1beta1.MsgSend")
	grant, err := authz.NewGrant(f.ctx.BlockTime(), genericAuth, nil)
	require.NoError(t, err)
	f.authzKeeper.EXPECT().IterateGrants(gomock.Any(), gomock.Any()).
		Do(func(_ any, cb func(sdk.AccAddress, sdk.AccAddress, authz.Grant) bool) {
			cb(legacyAddr, testAccAddr(), grant)
		})
	f.authzKeeper.EXPECT().DeleteGrant(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(
		fmt.Errorf("authz store corrupted"),
	)

	_, err = f.msgServer.ClaimLegacyAccount(f.ctx, msg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "migrate authz")
	assertNoFinalization(t, f, legacyAddr)
}

// TestClaimLegacyAccount_FailAtFeegrant verifies that a failure in MigrateFeegrant
// (step 5) propagates and no record is stored.
func TestClaimLegacyAccount_FailAtFeegrant(t *testing.T) {
	f := initMsgServerFixture(t)
	_, legacyAddr, newAddr, msg := setupPassingPreChecks(t, f)

	// Steps 1-4 succeed.
	f.stakingKeeper.EXPECT().GetDelegatorDelegations(gomock.Any(), legacyAddr, ^uint16(0)).Return(nil, nil).Times(2)
	f.stakingKeeper.EXPECT().GetUnbondingDelegations(gomock.Any(), legacyAddr, ^uint16(0)).Return(nil, nil)
	f.stakingKeeper.EXPECT().GetRedelegations(gomock.Any(), legacyAddr, ^uint16(0)).Return(nil, nil)
	f.distributionKeeper.EXPECT().GetDelegatorWithdrawAddr(gomock.Any(), legacyAddr).Return(legacyAddr, nil).Times(2)
	f.distributionKeeper.EXPECT().SetDelegatorWithdrawAddr(gomock.Any(), newAddr, newAddr).Return(nil)

	baseAcc := authtypes.NewBaseAccountWithAddress(legacyAddr)
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), legacyAddr).Return(baseAcc)
	f.accountKeeper.EXPECT().RemoveAccount(gomock.Any(), baseAcc)
	newAcc := authtypes.NewBaseAccountWithAddress(newAddr)
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), newAddr).Return(nil)
	f.accountKeeper.EXPECT().NewAccountWithAddress(gomock.Any(), newAddr).Return(newAcc)
	f.accountKeeper.EXPECT().SetAccount(gomock.Any(), newAcc)

	f.bankKeeper.EXPECT().GetAllBalances(gomock.Any(), legacyAddr).Return(sdk.Coins{})
	f.authzKeeper.EXPECT().IterateGrants(gomock.Any(), gomock.Any())

	// Step 5: MigrateFeegrant fails.
	f.feegrantKeeper.EXPECT().IterateAllFeeAllowances(gomock.Any(), gomock.Any()).Return(
		fmt.Errorf("feegrant store corrupted"),
	)

	_, err := f.msgServer.ClaimLegacyAccount(f.ctx, msg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "migrate feegrant")
	assertNoFinalization(t, f, legacyAddr)
}

// TestClaimLegacyAccount_FailAtSupernode verifies that a failure in MigrateSupernode
// (step 6) propagates and no record is stored.
func TestClaimLegacyAccount_FailAtSupernode(t *testing.T) {
	f := initMsgServerFixture(t)
	_, legacyAddr, newAddr, msg := setupPassingPreChecks(t, f)

	// Steps 1-5 succeed.
	f.stakingKeeper.EXPECT().GetDelegatorDelegations(gomock.Any(), legacyAddr, ^uint16(0)).Return(nil, nil).Times(2)
	f.stakingKeeper.EXPECT().GetUnbondingDelegations(gomock.Any(), legacyAddr, ^uint16(0)).Return(nil, nil)
	f.stakingKeeper.EXPECT().GetRedelegations(gomock.Any(), legacyAddr, ^uint16(0)).Return(nil, nil)
	f.distributionKeeper.EXPECT().GetDelegatorWithdrawAddr(gomock.Any(), legacyAddr).Return(legacyAddr, nil).Times(2)
	f.distributionKeeper.EXPECT().SetDelegatorWithdrawAddr(gomock.Any(), newAddr, newAddr).Return(nil)

	baseAcc := authtypes.NewBaseAccountWithAddress(legacyAddr)
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), legacyAddr).Return(baseAcc)
	f.accountKeeper.EXPECT().RemoveAccount(gomock.Any(), baseAcc)
	newAcc := authtypes.NewBaseAccountWithAddress(newAddr)
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), newAddr).Return(nil)
	f.accountKeeper.EXPECT().NewAccountWithAddress(gomock.Any(), newAddr).Return(newAcc)
	f.accountKeeper.EXPECT().SetAccount(gomock.Any(), newAcc)

	f.bankKeeper.EXPECT().GetAllBalances(gomock.Any(), legacyAddr).Return(sdk.Coins{})
	f.authzKeeper.EXPECT().IterateGrants(gomock.Any(), gomock.Any())
	f.feegrantKeeper.EXPECT().IterateAllFeeAllowances(gomock.Any(), gomock.Any()).Return(nil)

	// Step 6: MigrateSupernode fails.
	f.supernodeKeeper.EXPECT().GetSuperNodeByAccount(gomock.Any(), legacyAddr.String()).Return(
		sntypes.SuperNode{}, false, fmt.Errorf("supernode store corrupted"),
	)

	_, err := f.msgServer.ClaimLegacyAccount(f.ctx, msg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "migrate supernode")
	assertNoFinalization(t, f, legacyAddr)
}

// TestClaimLegacyAccount_FailAtActions verifies that a failure in MigrateActions
// (step 7) propagates and no record is stored.
func TestClaimLegacyAccount_FailAtActions(t *testing.T) {
	f := initMsgServerFixture(t)
	_, legacyAddr, newAddr, msg := setupPassingPreChecks(t, f)

	// Steps 1-6 succeed.
	f.stakingKeeper.EXPECT().GetDelegatorDelegations(gomock.Any(), legacyAddr, ^uint16(0)).Return(nil, nil).Times(2)
	f.stakingKeeper.EXPECT().GetUnbondingDelegations(gomock.Any(), legacyAddr, ^uint16(0)).Return(nil, nil)
	f.stakingKeeper.EXPECT().GetRedelegations(gomock.Any(), legacyAddr, ^uint16(0)).Return(nil, nil)
	f.distributionKeeper.EXPECT().GetDelegatorWithdrawAddr(gomock.Any(), legacyAddr).Return(legacyAddr, nil).Times(2)
	f.distributionKeeper.EXPECT().SetDelegatorWithdrawAddr(gomock.Any(), newAddr, newAddr).Return(nil)

	baseAcc := authtypes.NewBaseAccountWithAddress(legacyAddr)
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), legacyAddr).Return(baseAcc)
	f.accountKeeper.EXPECT().RemoveAccount(gomock.Any(), baseAcc)
	newAcc := authtypes.NewBaseAccountWithAddress(newAddr)
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), newAddr).Return(nil)
	f.accountKeeper.EXPECT().NewAccountWithAddress(gomock.Any(), newAddr).Return(newAcc)
	f.accountKeeper.EXPECT().SetAccount(gomock.Any(), newAcc)

	f.bankKeeper.EXPECT().GetAllBalances(gomock.Any(), legacyAddr).Return(sdk.Coins{})
	f.authzKeeper.EXPECT().IterateGrants(gomock.Any(), gomock.Any())
	f.feegrantKeeper.EXPECT().IterateAllFeeAllowances(gomock.Any(), gomock.Any()).Return(nil)
	f.supernodeKeeper.EXPECT().GetSuperNodeByAccount(gomock.Any(), legacyAddr.String()).Return(
		sntypes.SuperNode{}, false, nil,
	)

	// Step 7: MigrateActions fails.
	f.actionKeeper.EXPECT().GetActionsByCreator(gomock.Any(), gomock.Any()).Return(
		nil, fmt.Errorf("action store corrupted"),
	)

	_, err := f.msgServer.ClaimLegacyAccount(f.ctx, msg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "migrate actions")
	assertNoFinalization(t, f, legacyAddr)
}

// TestClaimLegacyAccount_FailAtClaim verifies that a failure in MigrateClaim
// (step 8, the last step before finalization) propagates and no record is stored.
func TestClaimLegacyAccount_FailAtClaim(t *testing.T) {
	f := initMsgServerFixture(t)
	_, legacyAddr, newAddr, msg := setupPassingPreChecks(t, f)

	// Steps 1-7 succeed.
	f.stakingKeeper.EXPECT().GetDelegatorDelegations(gomock.Any(), legacyAddr, ^uint16(0)).Return(nil, nil).Times(2)
	f.stakingKeeper.EXPECT().GetUnbondingDelegations(gomock.Any(), legacyAddr, ^uint16(0)).Return(nil, nil)
	f.stakingKeeper.EXPECT().GetRedelegations(gomock.Any(), legacyAddr, ^uint16(0)).Return(nil, nil)
	f.distributionKeeper.EXPECT().GetDelegatorWithdrawAddr(gomock.Any(), legacyAddr).Return(legacyAddr, nil).Times(2)
	f.distributionKeeper.EXPECT().SetDelegatorWithdrawAddr(gomock.Any(), newAddr, newAddr).Return(nil)

	baseAcc := authtypes.NewBaseAccountWithAddress(legacyAddr)
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), legacyAddr).Return(baseAcc)
	f.accountKeeper.EXPECT().RemoveAccount(gomock.Any(), baseAcc)
	newAcc := authtypes.NewBaseAccountWithAddress(newAddr)
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), newAddr).Return(nil)
	f.accountKeeper.EXPECT().NewAccountWithAddress(gomock.Any(), newAddr).Return(newAcc)
	f.accountKeeper.EXPECT().SetAccount(gomock.Any(), newAcc)

	f.bankKeeper.EXPECT().GetAllBalances(gomock.Any(), legacyAddr).Return(sdk.Coins{})
	f.authzKeeper.EXPECT().IterateGrants(gomock.Any(), gomock.Any())
	f.feegrantKeeper.EXPECT().IterateAllFeeAllowances(gomock.Any(), gomock.Any()).Return(nil)
	f.supernodeKeeper.EXPECT().GetSuperNodeByAccount(gomock.Any(), legacyAddr.String()).Return(
		sntypes.SuperNode{}, false, nil,
	)
	f.actionKeeper.EXPECT().GetActionsByCreator(gomock.Any(), gomock.Any()).Return(nil, nil)
	f.actionKeeper.EXPECT().GetActionsBySuperNode(gomock.Any(), gomock.Any()).Return(nil, nil)

	// Step 8: MigrateClaim fails.
	f.claimKeeper.EXPECT().IterateClaimRecords(gomock.Any(), gomock.Any()).Return(
		fmt.Errorf("claim store corrupted"),
	)

	_, err := f.msgServer.ClaimLegacyAccount(f.ctx, msg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "migrate claim")
	assertNoFinalization(t, f, legacyAddr)
}

// --- MigrateValidator failure-path / atomicity tests ---

// setupPassingValPreChecks configures mocks so that preChecks, validator-specific
// checks, and signature verification pass for MigrateValidator, returning the
// addresses, validator addresses, and the ready message.
func setupPassingValPreChecks(t *testing.T, f *msgServerFixture, ubds ...stakingtypes.UnbondingDelegation) (
	sdk.AccAddress, sdk.AccAddress, sdk.ValAddress, sdk.ValAddress, *types.MsgMigrateValidator,
) {
	t.Helper()
	privKey := secp256k1.GenPrivKey()
	legacyAddr := sdk.AccAddress(privKey.PubKey().Address())
	newPrivKey, newAddr := testNewMigrationAccount(t)
	oldValAddr := sdk.ValAddress(legacyAddr)
	newValAddr := sdk.ValAddress(newAddr)

	baseAcc := authtypes.NewBaseAccountWithAddress(legacyAddr)
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), legacyAddr).Return(baseAcc)

	// Validator exists and is bonded.
	f.stakingKeeper.EXPECT().GetValidator(gomock.Any(), oldValAddr).Return(
		stakingtypes.Validator{OperatorAddress: oldValAddr.String(), Status: stakingtypes.Bonded}, nil,
	)

	// Destination is NOT already a validator (post-unbonding-check guard).
	f.stakingKeeper.EXPECT().GetValidator(gomock.Any(), newValAddr).Return(
		stakingtypes.Validator{}, stakingtypes.ErrNoValidatorFound,
	)

	// Pre-check count read: no delegations; ubds default to empty unless a caller
	// supplies records to drive a later V4 re-key step. The reds scan hits the
	// wired (empty) store.
	f.stakingKeeper.EXPECT().GetValidatorDelegations(gomock.Any(), oldValAddr).Return(nil, nil)
	f.stakingKeeper.EXPECT().GetUnbondingDelegationsFromValidator(gomock.Any(), oldValAddr).Return(ubds, nil)

	msg := newValidatorMigrationMsg(t, privKey, legacyAddr, newPrivKey, newAddr)

	_ = newValAddr // used by callers
	return legacyAddr, newAddr, oldValAddr, newValAddr, msg
}

// assertNoValFinalization verifies that no migration record or counters were stored.
func assertNoValFinalization(t *testing.T, f *msgServerFixture, legacyAddr sdk.AccAddress) {
	t.Helper()
	has, err := f.keeper.MigrationRecords.Has(f.ctx, legacyAddr.String())
	require.NoError(t, err)
	require.False(t, has, "migration record should not exist after failed migration")

	count, err := f.keeper.MigrationCounter.Get(f.ctx)
	require.NoError(t, err)
	require.Equal(t, uint64(0), count, "migration counter should remain 0")

	valCount, err := f.keeper.ValidatorMigrationCounter.Get(f.ctx)
	require.NoError(t, err)
	require.Equal(t, uint64(0), valCount, "validator migration counter should remain 0")
}

// setupV1toV4 sets up mock expectations for steps V1 through V4 of MigrateValidator
// with no delegations, no commission, and minimal distribution state.
func setupV1toV4(f *mockFixture, oldValAddr, newValAddr sdk.ValAddress) {
	// V1: commission withdrawal (ignored error).
	f.distributionKeeper.EXPECT().WithdrawValidatorCommission(gomock.Any(), oldValAddr).Return(sdk.Coins{}, nil)

	// V2: record re-key.
	val := stakingtypes.Validator{OperatorAddress: oldValAddr.String(), Status: stakingtypes.Bonded}
	f.stakingKeeper.EXPECT().GetValidator(gomock.Any(), oldValAddr).Return(val, nil)
	f.stakingKeeper.EXPECT().DeleteValidatorByPowerIndex(gomock.Any(), val).Return(nil)
	f.stakingKeeper.EXPECT().SetValidator(gomock.Any(), gomock.Any()).Return(nil)
	f.stakingKeeper.EXPECT().SetValidatorByPowerIndex(gomock.Any(), gomock.Any()).Return(nil)
	f.stakingKeeper.EXPECT().GetLastValidatorPower(gomock.Any(), oldValAddr).Return(int64(0), fmt.Errorf("not found"))
	f.stakingKeeper.EXPECT().SetValidatorByConsAddr(gomock.Any(), gomock.Any()).Return(nil)

	// V3: distribution re-key.
	f.distributionKeeper.EXPECT().GetValidatorCurrentRewards(gomock.Any(), oldValAddr).Return(
		distrtypes.ValidatorCurrentRewards{Period: 1}, nil,
	)
	f.distributionKeeper.EXPECT().DeleteValidatorCurrentRewards(gomock.Any(), oldValAddr).Return(nil)
	f.distributionKeeper.EXPECT().SetValidatorCurrentRewards(gomock.Any(), newValAddr, gomock.Any()).Return(nil)
	f.distributionKeeper.EXPECT().GetValidatorAccumulatedCommission(gomock.Any(), oldValAddr).Return(
		distrtypes.ValidatorAccumulatedCommission{}, fmt.Errorf("not found"),
	)
	f.distributionKeeper.EXPECT().GetValidatorOutstandingRewards(gomock.Any(), oldValAddr).Return(
		distrtypes.ValidatorOutstandingRewards{}, fmt.Errorf("not found"),
	)
	f.distributionKeeper.EXPECT().IterateValidatorHistoricalRewards(gomock.Any(), gomock.Any())
	f.distributionKeeper.EXPECT().DeleteValidatorHistoricalRewards(gomock.Any(), oldValAddr)
	f.distributionKeeper.EXPECT().IterateValidatorSlashEvents(gomock.Any(), gomock.Any())
	f.distributionKeeper.EXPECT().DeleteValidatorSlashEvents(gomock.Any(), oldValAddr)

	// V4: no delegations. The pre-check already read delegations/ubds and V4
	// reuses those slices instead of re-fetching, so no staking Get mocks here.
	// With both slices empty and no redelegations in the wired store, V4 makes
	// no staking calls.
}

// TestMigrateValidator_FailAtValidatorRecord verifies that a failure in
// MigrateValidatorRecord (step V2) propagates and no record is stored.
func TestMigrateValidator_FailAtValidatorRecord(t *testing.T) {
	f := initMsgServerFixture(t)
	legacyAddr, _, oldValAddr, _, msg := setupPassingValPreChecks(t, f)

	// Step V1: WithdrawValidatorCommission (no commission — sentinel error is swallowed).
	f.distributionKeeper.EXPECT().WithdrawValidatorCommission(gomock.Any(), oldValAddr).Return(
		sdk.Coins{}, distrtypes.ErrNoValidatorCommission,
	)

	// Step V2: MigrateValidatorRecord fails at GetValidator.
	f.stakingKeeper.EXPECT().GetValidator(gomock.Any(), oldValAddr).Return(
		stakingtypes.Validator{}, fmt.Errorf("validator store corrupted"),
	)

	_, err := f.msgServer.MigrateValidator(f.ctx, msg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "migrate validator record")
	assertNoValFinalization(t, f, legacyAddr)
}

// TestMigrateValidator_FailAtValidatorDistribution verifies that a failure in
// MigrateValidatorDistribution (step V3) propagates and no record is stored.
func TestMigrateValidator_FailAtValidatorDistribution(t *testing.T) {
	f := initMsgServerFixture(t)
	legacyAddr, _, oldValAddr, newValAddr, msg := setupPassingValPreChecks(t, f)

	// Step V1: commission withdrawal.
	f.distributionKeeper.EXPECT().WithdrawValidatorCommission(gomock.Any(), oldValAddr).Return(sdk.Coins{}, nil)

	// Step V2: record re-key succeeds.
	val := stakingtypes.Validator{OperatorAddress: oldValAddr.String(), Status: stakingtypes.Bonded}
	f.stakingKeeper.EXPECT().GetValidator(gomock.Any(), oldValAddr).Return(val, nil)
	f.stakingKeeper.EXPECT().DeleteValidatorByPowerIndex(gomock.Any(), val).Return(nil)
	f.stakingKeeper.EXPECT().SetValidator(gomock.Any(), gomock.Any()).Return(nil)
	f.stakingKeeper.EXPECT().SetValidatorByPowerIndex(gomock.Any(), gomock.Any()).Return(nil)
	f.stakingKeeper.EXPECT().GetLastValidatorPower(gomock.Any(), oldValAddr).Return(int64(0), fmt.Errorf("not found"))
	f.stakingKeeper.EXPECT().SetValidatorByConsAddr(gomock.Any(), gomock.Any()).Return(nil)

	// Step V3: MigrateValidatorDistribution fails at DeleteValidatorCurrentRewards.
	f.distributionKeeper.EXPECT().GetValidatorCurrentRewards(gomock.Any(), oldValAddr).Return(
		distrtypes.ValidatorCurrentRewards{Period: 1}, nil,
	)
	f.distributionKeeper.EXPECT().DeleteValidatorCurrentRewards(gomock.Any(), oldValAddr).Return(
		fmt.Errorf("distribution store corrupted"),
	)

	_ = newValAddr
	_, err := f.msgServer.MigrateValidator(f.ctx, msg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "migrate validator distribution")
	assertNoValFinalization(t, f, legacyAddr)
}

// TestMigrateValidator_FailAtValidatorDelegations verifies that a failure in
// MigrateValidatorDelegations (step V4) propagates and no record is stored.
func TestMigrateValidator_FailAtValidatorDelegations(t *testing.T) {
	f := initMsgServerFixture(t)
	// The delegation/ubd read moved into the pre-check, so V4 now fails while
	// *re-keying* a record, not while fetching one. Feed one unbonding delegation
	// through the pre-check count read; V4's RemoveUnbondingDelegation then errors.
	// An unbonding delegation avoids the V1 per-delegator reward withdrawal that a
	// regular delegation would trigger, keeping this focused on the V4 re-key.
	ubd := stakingtypes.UnbondingDelegation{
		DelegatorAddress: testAccAddr().String(),
		ValidatorAddress: sdk.ValAddress(testAccAddr()).String(),
	}
	legacyAddr, _, oldValAddr, newValAddr, msg := setupPassingValPreChecks(t, f, ubd)

	// Steps V1-V3 succeed.
	f.distributionKeeper.EXPECT().WithdrawValidatorCommission(gomock.Any(), oldValAddr).Return(sdk.Coins{}, nil)

	val := stakingtypes.Validator{OperatorAddress: oldValAddr.String(), Status: stakingtypes.Bonded}
	f.stakingKeeper.EXPECT().GetValidator(gomock.Any(), oldValAddr).Return(val, nil)
	f.stakingKeeper.EXPECT().DeleteValidatorByPowerIndex(gomock.Any(), val).Return(nil)
	f.stakingKeeper.EXPECT().SetValidator(gomock.Any(), gomock.Any()).Return(nil)
	f.stakingKeeper.EXPECT().SetValidatorByPowerIndex(gomock.Any(), gomock.Any()).Return(nil)
	f.stakingKeeper.EXPECT().GetLastValidatorPower(gomock.Any(), oldValAddr).Return(int64(0), fmt.Errorf("not found"))
	f.stakingKeeper.EXPECT().SetValidatorByConsAddr(gomock.Any(), gomock.Any()).Return(nil)

	f.distributionKeeper.EXPECT().GetValidatorCurrentRewards(gomock.Any(), oldValAddr).Return(
		distrtypes.ValidatorCurrentRewards{Period: 1}, nil,
	)
	f.distributionKeeper.EXPECT().DeleteValidatorCurrentRewards(gomock.Any(), oldValAddr).Return(nil)
	f.distributionKeeper.EXPECT().SetValidatorCurrentRewards(gomock.Any(), newValAddr, gomock.Any()).Return(nil)
	f.distributionKeeper.EXPECT().GetValidatorAccumulatedCommission(gomock.Any(), oldValAddr).Return(
		distrtypes.ValidatorAccumulatedCommission{}, fmt.Errorf("not found"),
	)
	f.distributionKeeper.EXPECT().GetValidatorOutstandingRewards(gomock.Any(), oldValAddr).Return(
		distrtypes.ValidatorOutstandingRewards{}, fmt.Errorf("not found"),
	)
	f.distributionKeeper.EXPECT().IterateValidatorHistoricalRewards(gomock.Any(), gomock.Any())
	f.distributionKeeper.EXPECT().DeleteValidatorHistoricalRewards(gomock.Any(), oldValAddr)
	f.distributionKeeper.EXPECT().IterateValidatorSlashEvents(gomock.Any(), gomock.Any())
	f.distributionKeeper.EXPECT().DeleteValidatorSlashEvents(gomock.Any(), oldValAddr)

	// Step V4: unbonding-delegation re-key fails on its first store op.
	f.stakingKeeper.EXPECT().RemoveUnbondingDelegation(gomock.Any(), ubd).Return(
		fmt.Errorf("unbonding index corrupted"),
	)

	_, err := f.msgServer.MigrateValidator(f.ctx, msg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "migrate validator delegations")
	assertNoValFinalization(t, f, legacyAddr)
}

// TestMigrateValidator_FailAtValidatorSupernode verifies that a failure in
// MigrateValidatorSupernode (step V5) propagates and no record is stored.
func TestMigrateValidator_FailAtValidatorSupernode(t *testing.T) {
	f := initMsgServerFixture(t)
	legacyAddr, _, oldValAddr, newValAddr, msg := setupPassingValPreChecks(t, f)

	// Steps V1-V4 succeed.
	setupV1toV4(f.mockFixture, oldValAddr, newValAddr)

	// Step V5: supernode re-key fails.
	f.supernodeKeeper.EXPECT().QuerySuperNode(gomock.Any(), oldValAddr).Return(
		sntypes.SuperNode{ValidatorAddress: oldValAddr.String()}, true,
	)
	f.supernodeKeeper.EXPECT().DeleteSuperNode(gomock.Any(), oldValAddr)
	f.supernodeKeeper.EXPECT().GetMetricsState(gomock.Any(), oldValAddr).Return(
		sntypes.SupernodeMetricsState{}, false,
	)
	f.supernodeKeeper.EXPECT().SetSuperNode(gomock.Any(), gomock.Any()).Return(
		fmt.Errorf("supernode store write failed"),
	)

	_, err := f.msgServer.MigrateValidator(f.ctx, msg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "migrate validator supernode")
	assertNoValFinalization(t, f, legacyAddr)
}

// TestMigrateValidator_FailAtValidatorActions verifies that a failure in
// MigrateValidatorActions (step V6) propagates and no record is stored.
func TestMigrateValidator_FailAtValidatorActions(t *testing.T) {
	f := initMsgServerFixture(t)
	legacyAddr, _, oldValAddr, newValAddr, msg := setupPassingValPreChecks(t, f)

	// Steps V1-V4 succeed.
	setupV1toV4(f.mockFixture, oldValAddr, newValAddr)

	// V5: no supernode.
	f.supernodeKeeper.EXPECT().QuerySuperNode(gomock.Any(), oldValAddr).Return(sntypes.SuperNode{}, false)

	// Step V6: action re-key fails.
	f.actionKeeper.EXPECT().GetActionsByCreator(gomock.Any(), gomock.Any()).Return(
		nil, fmt.Errorf("action store corrupted"),
	)

	_, err := f.msgServer.MigrateValidator(f.ctx, msg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "migrate validator actions")
	assertNoValFinalization(t, f, legacyAddr)
}

// TestMigrateValidator_FailAtAuth verifies that a failure in MigrateAuth
// (step V7) propagates and no record is stored.
func TestMigrateValidator_FailAtAuth(t *testing.T) {
	f := initMsgServerFixture(t)
	legacyAddr, newAddr, oldValAddr, newValAddr, msg := setupPassingValPreChecks(t, f)

	// Steps V1-V4 succeed.
	setupV1toV4(f.mockFixture, oldValAddr, newValAddr)

	// V5-V6: no supernode, no actions.
	f.supernodeKeeper.EXPECT().QuerySuperNode(gomock.Any(), oldValAddr).Return(sntypes.SuperNode{}, false)
	f.actionKeeper.EXPECT().GetActionsByCreator(gomock.Any(), gomock.Any()).Return(nil, nil)
	f.actionKeeper.EXPECT().GetActionsBySuperNode(gomock.Any(), gomock.Any()).Return(nil, nil)

	// Step V7: MigrateDistribution + MigrateStaking succeed before MigrateAuth fails.
	// Snapshot withdraw addr.
	f.distributionKeeper.EXPECT().GetDelegatorWithdrawAddr(gomock.Any(), legacyAddr).Return(legacyAddr, nil)
	// MigrateDistribution: redirect check (self → no-op), no delegations to other validators.
	f.distributionKeeper.EXPECT().GetDelegatorWithdrawAddr(gomock.Any(), legacyAddr).Return(legacyAddr, nil)
	f.stakingKeeper.EXPECT().GetDelegatorDelegations(gomock.Any(), legacyAddr, ^uint16(0)).Return(nil, nil)
	// MigrateStaking: no delegations/unbonding/redelegations to other validators.
	f.stakingKeeper.EXPECT().GetDelegatorDelegations(gomock.Any(), legacyAddr, ^uint16(0)).Return(nil, nil)
	f.stakingKeeper.EXPECT().GetUnbondingDelegations(gomock.Any(), legacyAddr, ^uint16(0)).Return(nil, nil)
	f.stakingKeeper.EXPECT().GetRedelegations(gomock.Any(), legacyAddr, ^uint16(0)).Return(nil, nil)
	f.distributionKeeper.EXPECT().SetDelegatorWithdrawAddr(gomock.Any(), newAddr, newAddr).Return(nil)

	// MigrateAuth fails — Phase 1 probe of newAddr succeeds (fresh), then legacy not found.
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), newAddr).Return(nil)
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), legacyAddr).Return(nil)

	_, err := f.msgServer.MigrateValidator(f.ctx, msg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "migrate auth")
	assertNoValFinalization(t, f, legacyAddr)
}

// TestClaimLegacyAccount_WithDelegations verifies that pending rewards are
// withdrawn and delegations are re-keyed during claim.
func TestClaimLegacyAccount_WithDelegations(t *testing.T) {
	f := initMsgServerFixture(t)

	privKey := secp256k1.GenPrivKey()
	pubKey := privKey.PubKey().(*secp256k1.PubKey)
	legacyAddr := sdk.AccAddress(pubKey.Address())
	newPrivKey, newAddr := testNewMigrationAccount(t)
	valAddr := sdk.ValAddress(testAccAddr())

	baseAcc := authtypes.NewBaseAccountWithAddress(legacyAddr)
	del := stakingtypes.NewDelegation(legacyAddr.String(), valAddr.String(), math.LegacyNewDec(100))

	// preChecks: account exists and is not a module account.
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), legacyAddr).Return(baseAcc)
	// Not a validator.
	f.stakingKeeper.EXPECT().GetValidator(gomock.Any(), sdk.ValAddress(legacyAddr)).Return(
		stakingtypes.Validator{}, stakingtypes.ErrNoValidatorFound,
	)

	// Step 1: MigrateDistribution — withdraw rewards for one delegation.
	f.stakingKeeper.EXPECT().GetDelegatorDelegations(gomock.Any(), legacyAddr, ^uint16(0)).Return(
		[]stakingtypes.Delegation{del}, nil,
	)
	f.distributionKeeper.EXPECT().GetDelegatorStartingInfo(gomock.Any(), valAddr, legacyAddr).Return(
		distrtypes.DelegatorStartingInfo{PreviousPeriod: 4}, nil,
	)
	expectHistoricalRewardsLookup(f.distributionKeeper, valAddr, 4, 1)
	f.distributionKeeper.EXPECT().WithdrawDelegationRewards(gomock.Any(), legacyAddr, valAddr).Return(sdk.Coins{}, nil)

	// Step 2: MigrateStaking — re-key delegation.
	// migrateActiveDelegations
	f.stakingKeeper.EXPECT().GetDelegatorDelegations(gomock.Any(), legacyAddr, ^uint16(0)).Return(
		[]stakingtypes.Delegation{del}, nil,
	)
	f.distributionKeeper.EXPECT().DeleteDelegatorStartingInfo(gomock.Any(), valAddr, legacyAddr).Return(nil)
	f.stakingKeeper.EXPECT().RemoveDelegation(gomock.Any(), del).Return(nil)
	f.stakingKeeper.EXPECT().SetDelegation(gomock.Any(), gomock.Any()).Return(nil)
	f.distributionKeeper.EXPECT().GetValidatorCurrentRewards(gomock.Any(), valAddr).Return(
		distrtypes.ValidatorCurrentRewards{Period: 5}, nil,
	)
	expectHistoricalRewardsIncrement(f.distributionKeeper, valAddr, 4, 1)
	// migrateActiveDelegations fetches the validator to convert shares → tokens (rate 1.0).
	f.stakingKeeper.EXPECT().GetValidator(gomock.Any(), valAddr).Return(
		stakingtypes.Validator{OperatorAddress: valAddr.String(), Tokens: math.NewInt(100), DelegatorShares: math.LegacyNewDec(100)}, nil,
	)
	f.distributionKeeper.EXPECT().SetDelegatorStartingInfo(gomock.Any(), valAddr, newAddr, gomock.Any()).Return(nil)

	// migrateUnbondingDelegations
	f.stakingKeeper.EXPECT().GetUnbondingDelegations(gomock.Any(), legacyAddr, ^uint16(0)).Return(nil, nil)

	// migrateRedelegations
	f.stakingKeeper.EXPECT().GetRedelegations(gomock.Any(), legacyAddr, ^uint16(0)).Return(nil, nil)

	// migrateWithdrawAddress (called twice: once in redirectWithdrawAddrIfMigrated, once in migrateWithdrawAddress)
	f.distributionKeeper.EXPECT().GetDelegatorWithdrawAddr(gomock.Any(), legacyAddr).Return(legacyAddr, nil).Times(2)
	f.distributionKeeper.EXPECT().SetDelegatorWithdrawAddr(gomock.Any(), newAddr, newAddr).Return(nil)

	// Step 3a: MigrateAuth
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), legacyAddr).Return(baseAcc)
	f.accountKeeper.EXPECT().RemoveAccount(gomock.Any(), baseAcc)
	newAcc := authtypes.NewBaseAccountWithAddress(newAddr)
	f.accountKeeper.EXPECT().GetAccount(gomock.Any(), newAddr).Return(nil)
	f.accountKeeper.EXPECT().NewAccountWithAddress(gomock.Any(), newAddr).Return(newAcc)
	f.accountKeeper.EXPECT().SetAccount(gomock.Any(), newAcc)

	// Step 3b: MigrateBank
	f.bankKeeper.EXPECT().GetAllBalances(gomock.Any(), legacyAddr).Return(sdk.Coins{})

	// Steps 4-8: no authz/feegrant/supernode/action/claim to migrate.
	f.authzKeeper.EXPECT().IterateGrants(gomock.Any(), gomock.Any())
	f.feegrantKeeper.EXPECT().IterateAllFeeAllowances(gomock.Any(), gomock.Any()).Return(nil)
	f.supernodeKeeper.EXPECT().GetSuperNodeByAccount(gomock.Any(), legacyAddr.String()).Return(
		sntypes.SuperNode{}, false, nil,
	)
	f.actionKeeper.EXPECT().GetActionsByCreator(gomock.Any(), gomock.Any()).Return(nil, nil)
	f.actionKeeper.EXPECT().GetActionsBySuperNode(gomock.Any(), gomock.Any()).Return(nil, nil)
	f.claimKeeper.EXPECT().IterateClaimRecords(gomock.Any(), gomock.Any()).Return(nil)

	msg := &types.MsgClaimLegacyAccount{
		LegacyAddress: legacyAddr.String(),
		NewAddress:    newAddr.String(),
		LegacyProof: types.MigrationProof{Proof: &types.MigrationProof_Single{Single: &types.SingleKeyProof{
			PubKey:    pubKey.Key,
			Signature: signMigrationMessage(t, privKey, legacyAddr, newAddr),
			SigFormat: types.SigFormat_SIG_FORMAT_CLI,
		}}},
		NewProof: newSingleKeyProofNew(
			newPrivKey.PubKey().Bytes(),
			signNewMigrationEIP191(t, keeperClaimKind, newPrivKey, legacyAddr, newAddr),
			types.SigFormat_SIG_FORMAT_EIP191,
		),
	}

	resp, err := f.msgServer.ClaimLegacyAccount(f.ctx, msg)
	require.NoError(t, err)
	require.NotNil(t, resp)
}
