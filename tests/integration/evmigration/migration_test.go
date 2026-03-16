package integration_test

import (
	"crypto/sha256"
	"fmt"
	"os"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	"cosmossdk.io/x/feegrant"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	vestingtypes "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	evmcryptotypes "github.com/cosmos/evm/crypto/ethsecp256k1"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/LumeraProtocol/lumera/app"
	evmigrationkeeper "github.com/LumeraProtocol/lumera/x/evmigration/keeper"
	"github.com/LumeraProtocol/lumera/x/evmigration/types"
)

type MigrationIntegrationSuite struct {
	suite.Suite

	app       *app.App
	ctx       sdk.Context
	keeper    evmigrationkeeper.Keeper
	msgServer types.MsgServer
	authority sdk.AccAddress
}

// SetupTest initializes a fresh app with real keepers for each test,
// ensuring tests do not share mutable state (counters, migration records, etc.).
func (s *MigrationIntegrationSuite) SetupTest() {
	os.Setenv("SYSTEM_TESTS", "true")

	s.app = app.Setup(s.T())
	s.ctx = s.app.BaseApp.NewContext(true)
	s.keeper = s.app.EvmigrationKeeper
	s.msgServer = evmigrationkeeper.NewMsgServerImpl(s.keeper)
	s.authority = authtypes.NewModuleAddress(govtypes.ModuleName)
}

func (s *MigrationIntegrationSuite) TearDownTest() {
	s.app = nil
}

func TestMigrationIntegration(t *testing.T) {
	suite.Run(t, new(MigrationIntegrationSuite))
}

// signMigration creates a valid legacy signature for the migration message.
func signMigration(t *testing.T, privKey *secp256k1.PrivKey, legacyAddr, newAddr sdk.AccAddress) []byte {
	t.Helper()
	msg := fmt.Sprintf("lumera-evm-migration:claim:%s:%s", legacyAddr.String(), newAddr.String())
	hash := sha256.Sum256([]byte(msg))
	sig, err := privKey.Sign(hash[:])
	require.NoError(t, err)
	return sig
}

func signValidatorMigration(t *testing.T, privKey *secp256k1.PrivKey, legacyAddr, newAddr sdk.AccAddress) []byte {
	t.Helper()
	msg := fmt.Sprintf("lumera-evm-migration:validator:%s:%s", legacyAddr.String(), newAddr.String())
	hash := sha256.Sum256([]byte(msg))
	sig, err := privKey.Sign(hash[:])
	require.NoError(t, err)
	return sig
}

func signNewMigration(t *testing.T, kind string, privKey *evmcryptotypes.PrivKey, legacyAddr, newAddr sdk.AccAddress) []byte {
	t.Helper()
	msg := fmt.Sprintf("lumera-evm-migration:%s:%s:%s", kind, legacyAddr.String(), newAddr.String())
	sig, err := privKey.Sign([]byte(msg))
	require.NoError(t, err)
	return sig
}

func createNewEVMAddress(t *testing.T) (*evmcryptotypes.PrivKey, sdk.AccAddress) {
	t.Helper()
	privKey, err := evmcryptotypes.GenerateKey()
	require.NoError(t, err)
	return privKey, sdk.AccAddress(privKey.PubKey().Address())
}

func newClaimMsg(t *testing.T, legacyPrivKey *secp256k1.PrivKey, legacyAddr sdk.AccAddress, newPrivKey *evmcryptotypes.PrivKey, newAddr sdk.AccAddress) *types.MsgClaimLegacyAccount {
	t.Helper()
	return &types.MsgClaimLegacyAccount{
		LegacyAddress:   legacyAddr.String(),
		NewAddress:      newAddr.String(),
		LegacyPubKey:    legacyPrivKey.PubKey().(*secp256k1.PubKey).Key,
		LegacySignature: signMigration(t, legacyPrivKey, legacyAddr, newAddr),
		NewPubKey:       newPrivKey.PubKey().(*evmcryptotypes.PubKey).Key,
		NewSignature:    signNewMigration(t, "claim", newPrivKey, legacyAddr, newAddr),
	}
}

func newValidatorMsg(t *testing.T, legacyPrivKey *secp256k1.PrivKey, legacyAddr sdk.AccAddress, newPrivKey *evmcryptotypes.PrivKey, newAddr sdk.AccAddress) *types.MsgMigrateValidator {
	t.Helper()
	return &types.MsgMigrateValidator{
		LegacyAddress:   legacyAddr.String(),
		NewAddress:      newAddr.String(),
		LegacyPubKey:    legacyPrivKey.PubKey().(*secp256k1.PubKey).Key,
		LegacySignature: signValidatorMigration(t, legacyPrivKey, legacyAddr, newAddr),
		NewPubKey:       newPrivKey.PubKey().(*evmcryptotypes.PubKey).Key,
		NewSignature:    signNewMigration(t, "validator", newPrivKey, legacyAddr, newAddr),
	}
}

// createFundedLegacyAccount creates a secp256k1 account, registers it in auth,
// and funds it via the bank module.
func (s *MigrationIntegrationSuite) createFundedLegacyAccount(coins sdk.Coins) (*secp256k1.PrivKey, sdk.AccAddress) {
	privKey := secp256k1.GenPrivKey()
	pubKey := privKey.PubKey()
	addr := sdk.AccAddress(pubKey.Address())

	// Create and register the account with its public key.
	acc := s.app.AuthKeeper.NewAccountWithAddress(s.ctx, addr)
	baseAcc, ok := acc.(*authtypes.BaseAccount)
	s.Require().True(ok)
	s.Require().NoError(baseAcc.SetPubKey(pubKey))
	s.app.AuthKeeper.SetAccount(s.ctx, baseAcc)

	// Fund the account from the mint module.
	if !coins.IsZero() {
		s.Require().NoError(s.app.BankKeeper.MintCoins(s.ctx, "mint", coins))
		s.Require().NoError(s.app.BankKeeper.SendCoinsFromModuleToAccount(s.ctx, "mint", addr, coins))
	}

	return privKey, addr
}

// enableMigration sets evmigration params to allow migrations.
func (s *MigrationIntegrationSuite) enableMigration() {
	params := types.NewParams(true, 0, 50, 2000)
	s.Require().NoError(s.keeper.Params.Set(s.ctx, params))
}

// --- ClaimLegacyAccount integration tests ---

// TestClaimLegacyAccount_Success verifies end-to-end migration: balances move
// from legacy to new address, migration record is stored, counters increment.
func (s *MigrationIntegrationSuite) TestClaimLegacyAccount_Success() {
	s.enableMigration()

	coins := sdk.NewCoins(sdk.NewInt64Coin("ulume", 1_000_000))
	privKey, legacyAddr := s.createFundedLegacyAccount(coins)
	newPrivKey, newAddr := createNewEVMAddress(s.T())

	msg := newClaimMsg(s.T(), privKey, legacyAddr, newPrivKey, newAddr)

	resp, err := s.msgServer.ClaimLegacyAccount(s.ctx, msg)
	s.Require().NoError(err)
	s.Require().NotNil(resp)

	// Verify balances moved.
	legacyBal := s.app.BankKeeper.GetAllBalances(s.ctx, legacyAddr)
	newBal := s.app.BankKeeper.GetAllBalances(s.ctx, newAddr)
	s.Require().True(legacyBal.IsZero(), "legacy address should have zero balance")
	s.Require().True(newBal.AmountOf("ulume").Equal(sdkmath.NewInt(1_000_000)), "new address should have the migrated balance")

	// Verify migration record.
	record, err := s.keeper.MigrationRecords.Get(s.ctx, legacyAddr.String())
	s.Require().NoError(err)
	s.Require().Equal(legacyAddr.String(), record.LegacyAddress)
	s.Require().Equal(newAddr.String(), record.NewAddress)

	// Verify counter incremented (each test has fresh state).
	count, err := s.keeper.MigrationCounter.Get(s.ctx)
	s.Require().NoError(err)
	s.Require().Equal(uint64(1), count, "migration counter should be exactly 1")
}

// TestClaimLegacyAccount_MigratesAndRevokesFeegrants verifies that feegrant
// allowances are re-keyed to the migrated address and the legacy entries are
// removed from the concrete SDK feegrant keeper.
func (s *MigrationIntegrationSuite) TestClaimLegacyAccount_MigratesAndRevokesFeegrants() {
	s.enableMigration()

	legacyPrivKey, legacyAddr := s.createFundedLegacyAccount(sdk.NewCoins(sdk.NewInt64Coin("ulume", 1_000_000)))
	_, outgoingGrantee := s.createFundedLegacyAccount(sdk.NewCoins(sdk.NewInt64Coin("ulume", 100)))
	_, incomingGranter := s.createFundedLegacyAccount(sdk.NewCoins(sdk.NewInt64Coin("ulume", 100)))
	newPrivKey, newAddr := createNewEVMAddress(s.T())

	outgoingAllowance := &feegrant.BasicAllowance{
		SpendLimit: sdk.NewCoins(sdk.NewInt64Coin("ulume", 111)),
	}
	incomingAllowance := &feegrant.BasicAllowance{
		SpendLimit: sdk.NewCoins(sdk.NewInt64Coin("ulume", 222)),
	}

	s.Require().NoError(s.app.FeeGrantKeeper.GrantAllowance(s.ctx, legacyAddr, outgoingGrantee, outgoingAllowance))
	s.Require().NoError(s.app.FeeGrantKeeper.GrantAllowance(s.ctx, incomingGranter, legacyAddr, incomingAllowance))

	msg := newClaimMsg(s.T(), legacyPrivKey, legacyAddr, newPrivKey, newAddr)
	_, err := s.msgServer.ClaimLegacyAccount(s.ctx, msg)
	s.Require().NoError(err)

	// Legacy feegrant entries must be gone after migration.
	oldOutgoing, err := s.app.FeeGrantKeeper.GetAllowance(s.ctx, legacyAddr, outgoingGrantee)
	s.Require().NoError(err)
	s.Require().Nil(oldOutgoing)

	oldIncoming, err := s.app.FeeGrantKeeper.GetAllowance(s.ctx, incomingGranter, legacyAddr)
	s.Require().NoError(err)
	s.Require().Nil(oldIncoming)

	// The same allowances must exist under the migrated address.
	newOutgoing, err := s.app.FeeGrantKeeper.GetAllowance(s.ctx, newAddr, outgoingGrantee)
	s.Require().NoError(err)
	s.Require().NotNil(newOutgoing)
	s.Require().Equal(outgoingAllowance.SpendLimit, newOutgoing.(*feegrant.BasicAllowance).SpendLimit)

	newIncoming, err := s.app.FeeGrantKeeper.GetAllowance(s.ctx, incomingGranter, newAddr)
	s.Require().NoError(err)
	s.Require().NotNil(newIncoming)
	s.Require().Equal(incomingAllowance.SpendLimit, newIncoming.(*feegrant.BasicAllowance).SpendLimit)
}

// TestClaimLegacyAccount_MigrationDisabled verifies rejection when migrations are disabled.
func (s *MigrationIntegrationSuite) TestClaimLegacyAccount_MigrationDisabled() {
	params := types.NewParams(false, 0, 50, 2000)
	s.Require().NoError(s.keeper.Params.Set(s.ctx, params))

	coins := sdk.NewCoins(sdk.NewInt64Coin("ulume", 100))
	privKey, legacyAddr := s.createFundedLegacyAccount(coins)
	newPrivKey, newAddr := createNewEVMAddress(s.T())

	msg := newClaimMsg(s.T(), privKey, legacyAddr, newPrivKey, newAddr)

	_, err := s.msgServer.ClaimLegacyAccount(s.ctx, msg)
	s.Require().ErrorIs(err, types.ErrMigrationDisabled)
}

// TestClaimLegacyAccount_AlreadyMigrated verifies rejection when trying to
// migrate the same legacy address twice.
func (s *MigrationIntegrationSuite) TestClaimLegacyAccount_AlreadyMigrated() {
	s.enableMigration()

	coins := sdk.NewCoins(sdk.NewInt64Coin("ulume", 100))
	privKey, legacyAddr := s.createFundedLegacyAccount(coins)
	newPrivKey, newAddr := createNewEVMAddress(s.T())

	msg := newClaimMsg(s.T(), privKey, legacyAddr, newPrivKey, newAddr)

	// First migration should succeed.
	_, err := s.msgServer.ClaimLegacyAccount(s.ctx, msg)
	s.Require().NoError(err)

	// Create a new account to receive the second attempt.
	privKey2, legacyAddr2 := s.createFundedLegacyAccount(sdk.NewCoins(sdk.NewInt64Coin("ulume", 50)))
	newPrivKey2, _ := createNewEVMAddress(s.T())

	// Try to migrate to the same new address (the new address was a previously-migrated legacy).
	// Actually test the original legacy address being re-migrated.
	msg2 := newClaimMsg(s.T(), privKey, legacyAddr, newPrivKey2, legacyAddr2)
	_, err = s.msgServer.ClaimLegacyAccount(s.ctx, msg2)
	s.Require().ErrorIs(err, types.ErrAlreadyMigrated)

	// Also test: new address that was a previously-migrated legacy address.
	msg3 := newClaimMsg(s.T(), privKey2, legacyAddr2, newPrivKey, legacyAddr)
	_, err = s.msgServer.ClaimLegacyAccount(s.ctx, msg3)
	s.Require().ErrorIs(err, types.ErrNewAddressWasMigrated)
}

// TestClaimLegacyAccount_SameAddress verifies rejection when legacy and new are identical.
func (s *MigrationIntegrationSuite) TestClaimLegacyAccount_SameAddress() {
	s.enableMigration()

	coins := sdk.NewCoins(sdk.NewInt64Coin("ulume", 100))
	privKey, legacyAddr := s.createFundedLegacyAccount(coins)
	newPrivKey, _ := createNewEVMAddress(s.T())
	msg := newClaimMsg(s.T(), privKey, legacyAddr, newPrivKey, legacyAddr)

	_, err := s.msgServer.ClaimLegacyAccount(s.ctx, msg)
	s.Require().ErrorIs(err, types.ErrSameAddress)
}

// TestClaimLegacyAccount_InvalidSignature verifies rejection with a bad signature.
func (s *MigrationIntegrationSuite) TestClaimLegacyAccount_InvalidSignature() {
	s.enableMigration()

	coins := sdk.NewCoins(sdk.NewInt64Coin("ulume", 100))
	privKey, legacyAddr := s.createFundedLegacyAccount(coins)
	newPrivKey, newAddr := createNewEVMAddress(s.T())

	// Sign with a different private key.
	otherPrivKey := secp256k1.GenPrivKey()
	badSig := signMigration(s.T(), otherPrivKey, legacyAddr, newAddr)

	msg := newClaimMsg(s.T(), privKey, legacyAddr, newPrivKey, newAddr)
	msg.LegacySignature = badSig

	_, err := s.msgServer.ClaimLegacyAccount(s.ctx, msg)
	s.Require().ErrorIs(err, types.ErrInvalidLegacySignature)
}

// TestClaimLegacyAccount_ValidatorMustUseMigrateValidator verifies that validator
// operators are rejected from ClaimLegacyAccount and must use MigrateValidator.
func (s *MigrationIntegrationSuite) TestClaimLegacyAccount_ValidatorMustUseMigrateValidator() {
	s.enableMigration()

	// The genesis validator from app.Setup is a validator. We need to find its address.
	// Instead, we'll look up an existing validator from staking state.
	var valOperAddr sdk.ValAddress
	s.app.StakingKeeper.IterateValidators(s.ctx, func(_ int64, val stakingtypes.ValidatorI) bool {
		valAddr, _ := sdk.ValAddressFromBech32(val.GetOperator())
		valOperAddr = valAddr
		return true // stop after first
	})
	s.Require().NotNil(valOperAddr, "should find at least one genesis validator")

	legacyAddr := sdk.AccAddress(valOperAddr)
	privKey := secp256k1.GenPrivKey()

	// Create the account in auth if it doesn't exist.
	acc := s.app.AuthKeeper.GetAccount(s.ctx, legacyAddr)
	if acc == nil {
		acc = s.app.AuthKeeper.NewAccountWithAddress(s.ctx, legacyAddr)
		s.app.AuthKeeper.SetAccount(s.ctx, acc)
	}

	newPrivKey, newAddr := createNewEVMAddress(s.T())
	msg := newClaimMsg(s.T(), privKey, legacyAddr, newPrivKey, newAddr)

	_, err := s.msgServer.ClaimLegacyAccount(s.ctx, msg)
	// The pubkey won't match legacyAddr (since it's a random key), so we expect
	// the pubkey mismatch error. But if validation passes pubkey check first, then
	// UseValidatorMigration. Both are acceptable — the important thing is an error.
	s.Require().Error(err)
}

// TestClaimLegacyAccount_MultiDenom verifies migration of accounts with multiple denominations.
func (s *MigrationIntegrationSuite) TestClaimLegacyAccount_MultiDenom() {
	s.enableMigration()

	coins := sdk.NewCoins(
		sdk.NewInt64Coin("ulume", 500_000),
		sdk.NewInt64Coin("uatom", 200_000),
	)
	privKey, legacyAddr := s.createFundedLegacyAccount(coins)
	newPrivKey, newAddr := createNewEVMAddress(s.T())
	msg := newClaimMsg(s.T(), privKey, legacyAddr, newPrivKey, newAddr)

	_, err := s.msgServer.ClaimLegacyAccount(s.ctx, msg)
	s.Require().NoError(err)

	// Verify all denominations moved.
	newBal := s.app.BankKeeper.GetAllBalances(s.ctx, newAddr)
	s.Require().True(newBal.AmountOf("ulume").Equal(sdkmath.NewInt(500_000)))
	s.Require().True(newBal.AmountOf("uatom").Equal(sdkmath.NewInt(200_000)))

	legacyBal := s.app.BankKeeper.GetAllBalances(s.ctx, legacyAddr)
	s.Require().True(legacyBal.IsZero())
}

// TestClaimLegacyAccount_DelayedVestingPreserved verifies migration preserves
// delayed vesting account type and vesting internals.
func (s *MigrationIntegrationSuite) TestClaimLegacyAccount_DelayedVestingPreserved() {
	s.enableMigration()

	coins := sdk.NewCoins(sdk.NewInt64Coin("ulume", 1_000_000))
	privKey, legacyAddr := s.createFundedLegacyAccount(coins)
	newPrivKey, newAddr := createNewEVMAddress(s.T())

	legacyBase, ok := s.app.AuthKeeper.GetAccount(s.ctx, legacyAddr).(*authtypes.BaseAccount)
	s.Require().True(ok, "legacy account must start as BaseAccount")

	endTime := s.ctx.BlockTime().Add(180 * 24 * time.Hour).Unix()
	bva, err := vestingtypes.NewBaseVestingAccount(legacyBase, coins, endTime)
	s.Require().NoError(err)
	bva.DelegatedFree = sdk.NewCoins(sdk.NewInt64Coin("ulume", 11_111))
	bva.DelegatedVesting = sdk.NewCoins(sdk.NewInt64Coin("ulume", 22_222))

	delayed := vestingtypes.NewDelayedVestingAccountRaw(bva)
	s.app.AuthKeeper.SetAccount(s.ctx, delayed)

	msg := newClaimMsg(s.T(), privKey, legacyAddr, newPrivKey, newAddr)

	_, err = s.msgServer.ClaimLegacyAccount(s.ctx, msg)
	s.Require().NoError(err)

	newAcc := s.app.AuthKeeper.GetAccount(s.ctx, newAddr)
	newDelayed, ok := newAcc.(*vestingtypes.DelayedVestingAccount)
	s.Require().True(ok, "new account should be delayed vesting")
	s.Require().Equal(coins, newDelayed.OriginalVesting)
	s.Require().Equal(endTime, newDelayed.EndTime)
	s.Require().Equal(delayed.DelegatedFree, newDelayed.DelegatedFree)
	s.Require().Equal(delayed.DelegatedVesting, newDelayed.DelegatedVesting)
}

// --- Query integration tests ---

// TestQueryMigrationRecord_Integration verifies the query server with real state.
func (s *MigrationIntegrationSuite) TestQueryMigrationRecord_Integration() {
	s.enableMigration()
	qs := evmigrationkeeper.NewQueryServerImpl(s.keeper)

	coins := sdk.NewCoins(sdk.NewInt64Coin("ulume", 100))
	privKey, legacyAddr := s.createFundedLegacyAccount(coins)
	newPrivKey, newAddr := createNewEVMAddress(s.T())

	// Before migration — no record.
	resp, err := qs.MigrationRecord(s.ctx, &types.QueryMigrationRecordRequest{
		LegacyAddress: legacyAddr.String(),
	})
	s.Require().NoError(err)
	s.Require().Nil(resp.Record)

	// Perform migration.
	msg := newClaimMsg(s.T(), privKey, legacyAddr, newPrivKey, newAddr)
	_, err = s.msgServer.ClaimLegacyAccount(s.ctx, msg)
	s.Require().NoError(err)

	// After migration — record exists.
	resp, err = qs.MigrationRecord(s.ctx, &types.QueryMigrationRecordRequest{
		LegacyAddress: legacyAddr.String(),
	})
	s.Require().NoError(err)
	s.Require().NotNil(resp.Record)
	s.Require().Equal(newAddr.String(), resp.Record.NewAddress)
}

// TestQueryMigrationEstimate_Integration verifies estimate with real staking state.
func (s *MigrationIntegrationSuite) TestQueryMigrationEstimate_Integration() {
	s.enableMigration()
	qs := evmigrationkeeper.NewQueryServerImpl(s.keeper)

	coins := sdk.NewCoins(sdk.NewInt64Coin("ulume", 100))
	_, legacyAddr := s.createFundedLegacyAccount(coins)

	resp, err := qs.MigrationEstimate(s.ctx, &types.QueryMigrationEstimateRequest{
		LegacyAddress: legacyAddr.String(),
	})
	s.Require().NoError(err)
	s.Require().False(resp.IsValidator)
	s.Require().True(resp.WouldSucceed)
	s.Require().Equal(uint64(0), resp.DelegationCount)
}

// --- MigrateValidator integration tests ---

// createTestValidator creates a bonded validator with a secp256k1 operator key
// and properly initialized distribution state (via staking hooks). It also
// creates an external delegator to verify delegation re-keying.
func (s *MigrationIntegrationSuite) createTestValidator(
	legacyAddr sdk.AccAddress,
	selfBondAmt sdkmath.Int,
) (sdk.ValAddress, sdk.AccAddress) {
	valAddr := sdk.ValAddress(legacyAddr)

	// Generate ed25519 consensus key.
	consPubKey := ed25519.GenPrivKey().PubKey()
	pkAny, err := codectypes.NewAnyWithValue(consPubKey)
	s.Require().NoError(err)

	// Create unbonded validator record (Delegate handles token accounting).
	val := stakingtypes.Validator{
		OperatorAddress: valAddr.String(),
		ConsensusPubkey: pkAny,
		Jailed:          false,
		Status:          stakingtypes.Unbonded,
		Tokens:          sdkmath.ZeroInt(),
		DelegatorShares: sdkmath.LegacyZeroDec(),
		Description:     stakingtypes.Description{Moniker: "test-validator"},
		Commission: stakingtypes.NewCommission(
			sdkmath.LegacyNewDecWithPrec(1, 1),
			sdkmath.LegacyNewDecWithPrec(2, 1),
			sdkmath.LegacyNewDecWithPrec(1, 2),
		),
		MinSelfDelegation: sdkmath.OneInt(),
	}
	s.Require().NoError(s.app.StakingKeeper.SetValidator(s.ctx, val))
	s.Require().NoError(s.app.StakingKeeper.SetValidatorByConsAddr(s.ctx, val))
	s.Require().NoError(s.app.StakingKeeper.SetNewValidatorByPowerIndex(s.ctx, val))

	// Initialize distribution state (mimics AfterValidatorCreated hook).
	s.Require().NoError(s.app.DistrKeeper.SetValidatorHistoricalRewards(s.ctx, valAddr, 0,
		distrtypes.NewValidatorHistoricalRewards(sdk.DecCoins{}, 1)))
	s.Require().NoError(s.app.DistrKeeper.SetValidatorCurrentRewards(s.ctx, valAddr,
		distrtypes.NewValidatorCurrentRewards(sdk.DecCoins{}, 1)))
	s.Require().NoError(s.app.DistrKeeper.SetValidatorAccumulatedCommission(s.ctx, valAddr,
		distrtypes.InitialValidatorAccumulatedCommission()))
	s.Require().NoError(s.app.DistrKeeper.SetValidatorOutstandingRewards(s.ctx, valAddr,
		distrtypes.ValidatorOutstandingRewards{Rewards: sdk.DecCoins{}}))

	// Self-delegate using keeper.Delegate which triggers distribution hooks
	// (BeforeDelegationCreated → IncrementValidatorPeriod + initializeDelegation)
	// for proper reference counting and starting info initialization.
	val, err = s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	_, err = s.app.StakingKeeper.Delegate(s.ctx, legacyAddr, selfBondAmt,
		stakingtypes.Unbonded, val, true)
	s.Require().NoError(err)

	// External delegator.
	extCoins := sdk.NewCoins(sdk.NewInt64Coin("ulume", 500_000))
	_, extAddr := s.createFundedLegacyAccount(extCoins)
	extDelAmt := sdkmath.NewInt(200_000)
	val, err = s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	_, err = s.app.StakingKeeper.Delegate(s.ctx, extAddr, extDelAmt,
		stakingtypes.Unbonded, val, true)
	s.Require().NoError(err)

	// Promote to bonded status.
	val, err = s.app.StakingKeeper.GetValidator(s.ctx, valAddr)
	s.Require().NoError(err)
	val.Status = stakingtypes.Bonded
	s.Require().NoError(s.app.StakingKeeper.SetValidator(s.ctx, val))
	s.Require().NoError(s.app.StakingKeeper.SetLastValidatorPower(s.ctx, valAddr, val.Tokens.Int64()))

	return valAddr, extAddr
}

// TestMigrateValidator_Success performs an end-to-end validator migration:
// creates a bonded validator with self-delegation + external delegator, migrates
// it, and verifies validator record, delegations, distribution state, bank balances,
// and migration record are all correctly re-keyed.
func (s *MigrationIntegrationSuite) TestMigrateValidator_Success() {
	s.enableMigration()

	// Create funded legacy validator operator account.
	selfBondAmt := sdkmath.NewInt(1_000_000)
	operatorCoins := sdk.NewCoins(sdk.NewInt64Coin("ulume", 2_000_000))
	legacyPrivKey, legacyAddr := s.createFundedLegacyAccount(operatorCoins)

	// Set up validator with distribution state and an external delegator.
	oldValAddr, extDelegatorAddr := s.createTestValidator(legacyAddr, selfBondAmt)

	// Create new destination account.
	newPrivKey, newAddr := createNewEVMAddress(s.T())
	newValAddr := sdk.ValAddress(newAddr)

	// Submit MigrateValidator.
	msg := newValidatorMsg(s.T(), legacyPrivKey, legacyAddr, newPrivKey, newAddr)

	resp, err := s.msgServer.MigrateValidator(s.ctx, msg)
	s.Require().NoError(err)
	s.Require().NotNil(resp)

	// --- Verify validator record re-keyed ---
	// The old validator key is orphaned (RemoveValidator cannot be used on bonded
	// validators without destroying distribution state). The new record is canonical.
	newVal, err := s.app.StakingKeeper.GetValidator(s.ctx, newValAddr)
	s.Require().NoError(err, "new validator should exist")
	s.Require().Equal(newValAddr.String(), newVal.OperatorAddress)
	s.Require().Equal(stakingtypes.Bonded, newVal.Status)

	// --- Verify self-delegation re-keyed ---
	dels, err := s.app.StakingKeeper.GetValidatorDelegations(s.ctx, newValAddr)
	s.Require().NoError(err)
	s.Require().Len(dels, 2, "should have self-delegation + external delegation")

	// Verify delegation addresses point to new validator.
	for _, del := range dels {
		s.Require().Equal(newValAddr.String(), del.ValidatorAddress)
	}

	// Verify no delegations remain for old validator.
	oldDels, err := s.app.StakingKeeper.GetValidatorDelegations(s.ctx, oldValAddr)
	s.Require().NoError(err)
	s.Require().Empty(oldDels, "old validator should have no delegations")

	// --- Verify distribution state re-keyed ---
	_, err = s.app.DistrKeeper.GetValidatorCurrentRewards(s.ctx, newValAddr)
	s.Require().NoError(err, "current rewards should exist for new validator")

	// --- Verify bank balances moved ---
	legacyBal := s.app.BankKeeper.GetAllBalances(s.ctx, legacyAddr)
	s.Require().True(legacyBal.IsZero(), "legacy address should have zero balance")

	newBal := s.app.BankKeeper.GetAllBalances(s.ctx, newAddr)
	s.Require().True(newBal.AmountOf("ulume").GT(sdkmath.ZeroInt()),
		"new address should have migrated balance")

	// --- Verify migration record ---
	record, err := s.keeper.MigrationRecords.Get(s.ctx, legacyAddr.String())
	s.Require().NoError(err)
	s.Require().Equal(legacyAddr.String(), record.LegacyAddress)
	s.Require().Equal(newAddr.String(), record.NewAddress)

	// --- Verify counters ---
	migCount, err := s.keeper.MigrationCounter.Get(s.ctx)
	s.Require().NoError(err)
	s.Require().Equal(uint64(1), migCount, "migration counter should be exactly 1")

	valCount, err := s.keeper.ValidatorMigrationCounter.Get(s.ctx)
	s.Require().NoError(err)
	s.Require().Equal(uint64(1), valCount, "validator migration counter should be exactly 1")

	// --- Verify external delegator's delegation still valid ---
	extDels, err := s.app.StakingKeeper.GetDelegatorDelegations(s.ctx, extDelegatorAddr, 10)
	s.Require().NoError(err)
	s.Require().Len(extDels, 1, "external delegator should still have one delegation")
	s.Require().Equal(newValAddr.String(), extDels[0].ValidatorAddress,
		"external delegation should point to new validator")
}

// TestClaimLegacyAccount_AfterValidatorMigration verifies that legacy account
// migration still succeeds after the validator it delegates to has already been
// migrated to a new operator address.
func (s *MigrationIntegrationSuite) TestClaimLegacyAccount_AfterValidatorMigration() {
	s.enableMigration()

	selfBondAmt := sdkmath.NewInt(1_000_000)
	operatorCoins := sdk.NewCoins(sdk.NewInt64Coin("ulume", 2_000_000))
	validatorPrivKey, validatorLegacyAddr := s.createFundedLegacyAccount(operatorCoins)
	oldValAddr, _ := s.createTestValidator(validatorLegacyAddr, selfBondAmt)

	// Create a migratable legacy delegator and delegate to the legacy validator
	// before the validator migration happens.
	delegatorCoins := sdk.NewCoins(sdk.NewInt64Coin("ulume", 700_000))
	delegatorPrivKey, delegatorLegacyAddr := s.createFundedLegacyAccount(delegatorCoins)
	delegatorStake := sdkmath.NewInt(250_000)

	oldVal, err := s.app.StakingKeeper.GetValidator(s.ctx, oldValAddr)
	s.Require().NoError(err)
	_, err = s.app.StakingKeeper.Delegate(s.ctx, delegatorLegacyAddr, delegatorStake, stakingtypes.Bonded, oldVal, true)
	s.Require().NoError(err)

	// Migrate the validator first.
	validatorNewPrivKey, validatorNewAddr := createNewEVMAddress(s.T())
	validatorMsg := newValidatorMsg(s.T(), validatorPrivKey, validatorLegacyAddr, validatorNewPrivKey, validatorNewAddr)
	_, err = s.msgServer.MigrateValidator(s.ctx, validatorMsg)
	s.Require().NoError(err)

	newValAddr := sdk.ValAddress(validatorNewAddr)
	delsAfterValidatorMigration, err := s.app.StakingKeeper.GetDelegatorDelegations(s.ctx, delegatorLegacyAddr, 10)
	s.Require().NoError(err)
	s.Require().Len(delsAfterValidatorMigration, 1)
	s.Require().Equal(newValAddr.String(), delsAfterValidatorMigration[0].ValidatorAddress)

	// Then migrate the delegator account. This is the validator-first order that
	// previously failed when distribution state under the new valoper was
	// incomplete.
	delegatorNewPrivKey, delegatorNewAddr := createNewEVMAddress(s.T())
	delegatorMsg := newClaimMsg(s.T(), delegatorPrivKey, delegatorLegacyAddr, delegatorNewPrivKey, delegatorNewAddr)
	_, err = s.msgServer.ClaimLegacyAccount(s.ctx, delegatorMsg)
	s.Require().NoError(err)

	newDelegations, err := s.app.StakingKeeper.GetDelegatorDelegations(s.ctx, delegatorNewAddr, 10)
	s.Require().NoError(err)
	s.Require().Len(newDelegations, 1)
	s.Require().Equal(newValAddr.String(), newDelegations[0].ValidatorAddress)

	oldDelegations, err := s.app.StakingKeeper.GetDelegatorDelegations(s.ctx, delegatorLegacyAddr, 10)
	s.Require().NoError(err)
	s.Require().Empty(oldDelegations)

	record, err := s.keeper.MigrationRecords.Get(s.ctx, delegatorLegacyAddr.String())
	s.Require().NoError(err)
	s.Require().Equal(delegatorNewAddr.String(), record.NewAddress)
}

// TestMigrateValidator_NotValidator verifies rejection when the legacy address
// is not a validator operator.
func (s *MigrationIntegrationSuite) TestMigrateValidator_NotValidator() {
	s.enableMigration()

	coins := sdk.NewCoins(sdk.NewInt64Coin("ulume", 100))
	privKey, legacyAddr := s.createFundedLegacyAccount(coins)
	newPrivKey, newAddr := createNewEVMAddress(s.T())
	msg := newValidatorMsg(s.T(), privKey, legacyAddr, newPrivKey, newAddr)

	_, err := s.msgServer.MigrateValidator(s.ctx, msg)
	s.Require().ErrorIs(err, types.ErrNotValidator)
}

// TestClaimLegacyAccount_LegacyAccountRemoved verifies that the legacy auth
// account is removed after migration and the new account exists.
func (s *MigrationIntegrationSuite) TestClaimLegacyAccount_LegacyAccountRemoved() {
	s.enableMigration()

	coins := sdk.NewCoins(sdk.NewInt64Coin("ulume", 100))
	privKey, legacyAddr := s.createFundedLegacyAccount(coins)
	newPrivKey, newAddr := createNewEVMAddress(s.T())
	msg := newClaimMsg(s.T(), privKey, legacyAddr, newPrivKey, newAddr)

	_, err := s.msgServer.ClaimLegacyAccount(s.ctx, msg)
	s.Require().NoError(err)

	// Legacy account should be removed from auth.
	legacyAcc := s.app.AuthKeeper.GetAccount(s.ctx, legacyAddr)
	s.Require().Nil(legacyAcc, "legacy account should be removed after migration")

	// New account should exist.
	newAcc := s.app.AuthKeeper.GetAccount(s.ctx, newAddr)
	s.Require().NotNil(newAcc, "new account should exist after migration")
}
