package integration_test

import (
	"crypto/sha256"
	"fmt"
	"os"
	"testing"
	"time"

	errorsmod "cosmossdk.io/errors"
	sdkmath "cosmossdk.io/math"
	"cosmossdk.io/x/feegrant"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	vestingtypes "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	evmcryptotypes "github.com/cosmos/evm/crypto/ethsecp256k1"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/LumeraProtocol/lumera/app"
	lcfg "github.com/LumeraProtocol/lumera/config"
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
	// NewContext(true) doesn't carry chain ID or block time from InitChain
	// in SDK v0.53. We set both explicitly:
	//   - ChainID must match app.Setup's "testing" for signature verification.
	//   - BlockTime must be realistic (not zero-value year 0001) because vesting
	//     accounts compute EndTime relative to it, and feegrant expiry checks
	//     compare against it.
	s.ctx = s.app.BaseApp.NewContext(true).
		WithChainID(integrationTestChainID).
		WithBlockTime(time.Now().UTC())
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

const integrationTestChainID = "testing" // must match app.Setup's chainID

// signMigration creates a valid legacy signature for the migration message.
func signMigration(t *testing.T, privKey *secp256k1.PrivKey, legacyAddr, newAddr sdk.AccAddress) []byte {
	t.Helper()
	msg := fmt.Sprintf("lumera-evm-migration:%s:%d:claim:%s:%s", integrationTestChainID, lcfg.EVMChainID, legacyAddr.String(), newAddr.String())
	hash := sha256.Sum256([]byte(msg))
	sig, err := privKey.Sign(hash[:])
	require.NoError(t, err)
	return sig
}

func signValidatorMigration(t *testing.T, privKey *secp256k1.PrivKey, legacyAddr, newAddr sdk.AccAddress) []byte {
	t.Helper()
	msg := fmt.Sprintf("lumera-evm-migration:%s:%d:validator:%s:%s", integrationTestChainID, lcfg.EVMChainID, legacyAddr.String(), newAddr.String())
	hash := sha256.Sum256([]byte(msg))
	sig, err := privKey.Sign(hash[:])
	require.NoError(t, err)
	return sig
}

func signNewMigration(t *testing.T, kind string, privKey *evmcryptotypes.PrivKey, legacyAddr, newAddr sdk.AccAddress) []byte {
	t.Helper()
	msg := fmt.Sprintf("lumera-evm-migration:%s:%d:%s:%s:%s", integrationTestChainID, lcfg.EVMChainID, kind, legacyAddr.String(), newAddr.String())
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
		LegacyAddress: legacyAddr.String(),
		NewAddress:    newAddr.String(),
		LegacyProof: types.MigrationProof{Proof: &types.MigrationProof_Single{Single: &types.SingleKeyProof{
			PubKey:    legacyPrivKey.PubKey().(*secp256k1.PubKey).Key,
			Signature: signMigration(t, legacyPrivKey, legacyAddr, newAddr),
			SigFormat: types.SigFormat_SIG_FORMAT_CLI,
		}}},
		NewProof: types.MigrationProof{Proof: &types.MigrationProof_Single{Single: &types.SingleKeyProof{
			PubKey:    newPrivKey.PubKey().(*evmcryptotypes.PubKey).Key,
			Signature: signNewMigration(t, "claim", newPrivKey, legacyAddr, newAddr),
			SigFormat: types.SigFormat_SIG_FORMAT_CLI,
		}}},
	}
}

func newValidatorMsg(t *testing.T, legacyPrivKey *secp256k1.PrivKey, legacyAddr sdk.AccAddress, newPrivKey *evmcryptotypes.PrivKey, newAddr sdk.AccAddress) *types.MsgMigrateValidator {
	t.Helper()
	return &types.MsgMigrateValidator{
		LegacyAddress: legacyAddr.String(),
		NewAddress:    newAddr.String(),
		LegacyProof: types.MigrationProof{Proof: &types.MigrationProof_Single{Single: &types.SingleKeyProof{
			PubKey:    legacyPrivKey.PubKey().(*secp256k1.PubKey).Key,
			Signature: signValidatorMigration(t, legacyPrivKey, legacyAddr, newAddr),
			SigFormat: types.SigFormat_SIG_FORMAT_CLI,
		}}},
		NewProof: types.MigrationProof{Proof: &types.MigrationProof_Single{Single: &types.SingleKeyProof{
			PubKey:    newPrivKey.PubKey().(*evmcryptotypes.PubKey).Key,
			Signature: signNewMigration(t, "validator", newPrivKey, legacyAddr, newAddr),
			SigFormat: types.SigFormat_SIG_FORMAT_CLI,
		}}},
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
	params := types.NewParams(true, 0, 50, 2000, 20)
	s.Require().NoError(s.keeper.Params.Set(s.ctx, params))
}

// --- ClaimLegacyAccount integration tests ---

// TestClaimLegacyAccount_Success verifies end-to-end migration: balances move
// from legacy to new address, migration record is stored, counters increment.
//
// Regression-lock: this is the canonical single-key-legacy → single-key-new
// migration test. The multisig refactor plan (Task 21) calls for an explicit
// `TestClaimLegacyAccount_SingleKeyToSingleKey_Regression`; that role is filled
// by this test — its pass signal is exactly what locks down prior single→single
// behavior as the module grows multisig support.
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
	// GetAllowance returns an error (not nil) when the grant doesn't exist.
	_, err = s.app.FeeGrantKeeper.GetAllowance(s.ctx, legacyAddr, outgoingGrantee)
	s.Require().Error(err, "legacy outgoing feegrant should be removed")

	_, err = s.app.FeeGrantKeeper.GetAllowance(s.ctx, incomingGranter, legacyAddr)
	s.Require().Error(err, "legacy incoming feegrant should be removed")

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
	params := types.NewParams(false, 0, 50, 2000, 20)
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
	msg.LegacyProof.Proof.(*types.MigrationProof_Single).Single.Signature = badSig

	_, err := s.msgServer.ClaimLegacyAccount(s.ctx, msg)
	s.Require().ErrorIs(err, types.ErrInvalidMigrationSignature)
}

// TestClaimLegacyAccount_ValidatorMustUseMigrateValidator verifies that validator
// operators are rejected from ClaimLegacyAccount and must use MigrateValidator.
func (s *MigrationIntegrationSuite) TestClaimLegacyAccount_ValidatorMustUseMigrateValidator() {
	s.enableMigration()

	// The genesis validator from app.Setup is a validator. We need to find its address.
	// Instead, we'll look up an existing validator from staking state.
	var valOperAddr sdk.ValAddress
	err := s.app.StakingKeeper.IterateValidators(s.ctx, func(_ int64, val stakingtypes.ValidatorI) bool {
		valAddr, _ := sdk.ValAddressFromBech32(val.GetOperator())
		valOperAddr = valAddr
		return true // stop after first
	})
	s.Require().NoError(err)
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

	_, err = s.msgServer.ClaimLegacyAccount(s.ctx, msg)
	// The validator check (GetValidator) runs before signature verification,
	// so this must fail with ErrUseValidatorMigration specifically.
	s.Require().ErrorIs(err, types.ErrUseValidatorMigration)
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

// TestQueryMigrationEstimate_Multisig_Success verifies that a valid K-of-N
// Cosmos secp256k1 multisig account is detected by the preflight: IsMultisig
// is set, threshold/num_signers match the pubkey, and WouldSucceed remains
// true because no rejection branch fires (all sub-keys secp256k1, N within
// MaxMultisigSubKeys). Covers the happy path of the multisig preflight at
// x/evmigration/keeper/query.go:278-303.
func (s *MigrationIntegrationSuite) TestQueryMigrationEstimate_Multisig_Success() {
	s.enableMigration()
	qs := evmigrationkeeper.NewQueryServerImpl(s.keeper)

	coins := sdk.NewCoins(sdk.NewInt64Coin("ulume", 500_000))
	_, _, legacyAddr := s.createFundedMultisigAccount(2, 3, coins)

	resp, err := qs.MigrationEstimate(s.ctx, &types.QueryMigrationEstimateRequest{
		LegacyAddress: legacyAddr.String(),
	})
	s.Require().NoError(err)
	s.Require().True(resp.IsMultisig, "multisig account should be detected")
	s.Require().Equal(uint32(2), resp.Threshold)
	s.Require().Equal(uint32(3), resp.NumSigners)
	s.Require().False(resp.IsValidator)
	s.Require().True(resp.WouldSucceed, "2-of-3 secp256k1 multisig should pass preflight")
	s.Require().Empty(resp.RejectionReason)
}

// TestQueryMigrationEstimate_Multisig_SizeCapped verifies that a multisig with
// N sub-keys exceeding params.MaxMultisigSubKeys (default 20) is rejected by
// the preflight with the exact "multisig has X sub-keys; max is Y" format from
// query.go:295.
func (s *MigrationIntegrationSuite) TestQueryMigrationEstimate_Multisig_SizeCapped() {
	s.enableMigration()
	qs := evmigrationkeeper.NewQueryServerImpl(s.keeper)

	// Default MaxMultisigSubKeys is 20 (see types/params.go). 21-of-25 exceeds it.
	multiPK := buildLargeCosmosMultisig(s.T(), 21, 25)
	legacyAddr := s.registerAndFundMultisigPubKey(multiPK,
		sdk.NewCoins(sdk.NewInt64Coin("ulume", 1_000)))

	resp, err := qs.MigrationEstimate(s.ctx, &types.QueryMigrationEstimateRequest{
		LegacyAddress: legacyAddr.String(),
	})
	s.Require().NoError(err)
	s.Require().True(resp.IsMultisig)
	s.Require().Equal(uint32(21), resp.Threshold)
	s.Require().Equal(uint32(25), resp.NumSigners)
	s.Require().False(resp.WouldSucceed, "N>MaxMultisigSubKeys must reject")
	s.Require().Contains(resp.RejectionReason, "25 sub-keys")
	s.Require().Contains(resp.RejectionReason, "max is 20")
}

// TestQueryMigrationEstimate_Multisig_NonSecp256k1SubKey verifies that a
// multisig containing an eth_secp256k1 sub-key on the legacy side is rejected
// by the preflight with the "non-secp256k1 sub-key" reason from query.go:288.
// The legacy side must be plain Cosmos secp256k1 only.
func (s *MigrationIntegrationSuite) TestQueryMigrationEstimate_Multisig_NonSecp256k1SubKey() {
	s.enableMigration()
	qs := evmigrationkeeper.NewQueryServerImpl(s.keeper)

	// 2-of-3 where one sub-key is eth_secp256k1 (unsupported on legacy side).
	multiPK := buildMultisigWithEthSubKey(s.T(), 2)
	legacyAddr := s.registerAndFundMultisigPubKey(multiPK,
		sdk.NewCoins(sdk.NewInt64Coin("ulume", 1_000)))

	resp, err := qs.MigrationEstimate(s.ctx, &types.QueryMigrationEstimateRequest{
		LegacyAddress: legacyAddr.String(),
	})
	s.Require().NoError(err)
	s.Require().True(resp.IsMultisig)
	s.Require().Equal(uint32(2), resp.Threshold)
	s.Require().Equal(uint32(3), resp.NumSigners)
	s.Require().False(resp.WouldSucceed, "non-secp256k1 sub-key must reject")
	s.Require().Contains(resp.RejectionReason, "non-secp256k1 sub-key")
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
	// tokenSrc must be Unbonded for a fresh delegation from the user's balance.
	_, err = s.app.StakingKeeper.Delegate(s.ctx, delegatorLegacyAddr, delegatorStake, stakingtypes.Unbonded, oldVal, true)
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

// --- Multisig integration tests ---

// TestClaimLegacyAccount_Multisig_Success migrates a 2-of-3 multisig legacy
// account to a single eth EOA.
func (s *MigrationIntegrationSuite) TestClaimLegacyAccount_Multisig_Success() {
	s.enableMigration()

	coins := sdk.NewCoins(sdk.NewInt64Coin("ulume", 1_000_000))
	multiPK, privs, legacyAddr := s.createFundedMultisigAccount(2, 3, coins)
	newPrivKey, newAddr := createNewEVMAddress(s.T())

	proof := SignMultisigProof(s.T(), integrationTestChainID, "claim", multiPK, privs, []int{0, 2}, legacyAddr, newAddr, types.SigFormat_SIG_FORMAT_CLI)

	msg := &types.MsgClaimLegacyAccount{
		NewAddress:    newAddr.String(),
		LegacyAddress: legacyAddr.String(),
		LegacyProof:   *proof,
		NewProof: types.MigrationProof{Proof: &types.MigrationProof_Single{Single: &types.SingleKeyProof{
			PubKey:    newPrivKey.PubKey().(*evmcryptotypes.PubKey).Key,
			Signature: signNewMigration(s.T(), "claim", newPrivKey, legacyAddr, newAddr),
			SigFormat: types.SigFormat_SIG_FORMAT_CLI,
		}}},
	}
	_, err := s.msgServer.ClaimLegacyAccount(s.ctx, msg)
	s.Require().NoError(err)

	legacyBal := s.app.BankKeeper.GetAllBalances(s.ctx, legacyAddr)
	newBal := s.app.BankKeeper.GetAllBalances(s.ctx, newAddr)
	s.Require().True(legacyBal.IsZero())
	s.Require().True(newBal.AmountOf("ulume").Equal(sdkmath.NewInt(1_000_000)))

	rec, err := s.keeper.MigrationRecords.Get(s.ctx, legacyAddr.String())
	s.Require().NoError(err)
	s.Require().Equal(newAddr.String(), rec.NewAddress)
}

// TestClaimLegacyAccount_Multisig_ADR036 verifies ADR-036 sub-signature format.
func (s *MigrationIntegrationSuite) TestClaimLegacyAccount_Multisig_ADR036() {
	s.enableMigration()

	coins := sdk.NewCoins(sdk.NewInt64Coin("ulume", 500_000))
	multiPK, privs, legacyAddr := s.createFundedMultisigAccount(2, 3, coins)
	newPrivKey, newAddr := createNewEVMAddress(s.T())

	proof := SignMultisigProof(s.T(), integrationTestChainID, "claim", multiPK, privs, []int{1, 2}, legacyAddr, newAddr, types.SigFormat_SIG_FORMAT_ADR036)

	msg := &types.MsgClaimLegacyAccount{
		NewAddress:    newAddr.String(),
		LegacyAddress: legacyAddr.String(),
		LegacyProof:   *proof,
		NewProof: types.MigrationProof{Proof: &types.MigrationProof_Single{Single: &types.SingleKeyProof{
			PubKey:    newPrivKey.PubKey().(*evmcryptotypes.PubKey).Key,
			Signature: signNewMigration(s.T(), "claim", newPrivKey, legacyAddr, newAddr),
			SigFormat: types.SigFormat_SIG_FORMAT_CLI,
		}}},
	}
	_, err := s.msgServer.ClaimLegacyAccount(s.ctx, msg)
	s.Require().NoError(err)
}

// TestClaimLegacyAccount_Multisig_Replay verifies the replay guard on a migrated multisig.
func (s *MigrationIntegrationSuite) TestClaimLegacyAccount_Multisig_Replay() {
	s.enableMigration()

	coins := sdk.NewCoins(sdk.NewInt64Coin("ulume", 1_000_000))
	multiPK, privs, legacyAddr := s.createFundedMultisigAccount(2, 3, coins)
	newPrivKey, newAddr := createNewEVMAddress(s.T())

	proof := SignMultisigProof(s.T(), integrationTestChainID, "claim", multiPK, privs, []int{0, 1}, legacyAddr, newAddr, types.SigFormat_SIG_FORMAT_CLI)
	msg := &types.MsgClaimLegacyAccount{
		NewAddress:    newAddr.String(),
		LegacyAddress: legacyAddr.String(),
		LegacyProof:   *proof,
		NewProof: types.MigrationProof{Proof: &types.MigrationProof_Single{Single: &types.SingleKeyProof{
			PubKey:    newPrivKey.PubKey().(*evmcryptotypes.PubKey).Key,
			Signature: signNewMigration(s.T(), "claim", newPrivKey, legacyAddr, newAddr),
			SigFormat: types.SigFormat_SIG_FORMAT_CLI,
		}}},
	}
	_, err := s.msgServer.ClaimLegacyAccount(s.ctx, msg)
	s.Require().NoError(err)

	// Replay must fail.
	_, err = s.msgServer.ClaimLegacyAccount(s.ctx, msg)
	s.Require().Error(err)
	s.Require().ErrorContains(err, "has already been migrated")
}

// TestClaimLegacyAccount_Multisig_CorruptedSubSig verifies sub-sig tampering is rejected.
func (s *MigrationIntegrationSuite) TestClaimLegacyAccount_Multisig_CorruptedSubSig() {
	s.enableMigration()

	coins := sdk.NewCoins(sdk.NewInt64Coin("ulume", 1_000_000))
	multiPK, privs, legacyAddr := s.createFundedMultisigAccount(2, 3, coins)
	newPrivKey, newAddr := createNewEVMAddress(s.T())

	proof := SignMultisigProof(s.T(), integrationTestChainID, "claim", multiPK, privs, []int{0, 2}, legacyAddr, newAddr, types.SigFormat_SIG_FORMAT_CLI)
	// Corrupt the first sub-signature.
	proof.GetMultisig().SubSignatures[0][0] ^= 0xFF

	msg := &types.MsgClaimLegacyAccount{
		NewAddress:    newAddr.String(),
		LegacyAddress: legacyAddr.String(),
		LegacyProof:   *proof,
		NewProof: types.MigrationProof{Proof: &types.MigrationProof_Single{Single: &types.SingleKeyProof{
			Signature: signNewMigration(s.T(), "claim", newPrivKey, legacyAddr, newAddr),
			SigFormat: types.SigFormat_SIG_FORMAT_EIP191,
		}}},
	}
	_, err := s.msgServer.ClaimLegacyAccount(s.ctx, msg)
	s.Require().Error(err)
	s.Require().ErrorContains(err, "sub-sig 0")
}

// TestClaimLegacyAccount_MultisigToMultisig migrates a 2-of-3 Cosmos multisig
// legacy account to a 2-of-3 eth_secp256k1 multisig destination. Asserts:
//   - Migration record created with the correct new bech32.
//   - Destination BaseAccount has PubKey set to the reconstructed eth multisig pubkey.
//   - Full balance migrated.
//   - Legacy account balance is zero.
func (s *MigrationIntegrationSuite) TestClaimLegacyAccount_MultisigToMultisig() {
	s.enableMigration()

	coins := sdk.NewCoins(sdk.NewInt64Coin("ulume", 1_000_000_000))
	legacyMultiPK, legacyPrivs, legacyAddr := s.createFundedMultisigAccount(2, 3, coins)
	newMultiPK, newPrivs, newAddr := BuildMultisigNewAccount(s.T(), 2, 3)

	legacyProof := SignMultisigProof(s.T(), integrationTestChainID, "claim",
		legacyMultiPK, legacyPrivs, []int{0, 2}, legacyAddr, newAddr,
		types.SigFormat_SIG_FORMAT_CLI)
	newProof := SignNewMultisigProof(s.T(), integrationTestChainID, "claim",
		newMultiPK, newPrivs, []int{0, 2}, legacyAddr, newAddr,
		types.SigFormat_SIG_FORMAT_CLI)

	msg := &types.MsgClaimLegacyAccount{
		LegacyAddress: legacyAddr.String(),
		NewAddress:    newAddr.String(),
		LegacyProof:   *legacyProof,
		NewProof:      *newProof,
	}

	_, err := s.msgServer.ClaimLegacyAccount(s.ctx, msg)
	s.Require().NoError(err)

	// Migration record
	rec, err := s.keeper.MigrationRecords.Get(s.ctx, legacyAddr.String())
	s.Require().NoError(err)
	s.Require().Equal(newAddr.String(), rec.NewAddress)

	// Destination BaseAccount has the reconstructed eth multisig pubkey.
	newAcc := s.app.AuthKeeper.GetAccount(s.ctx, newAddr)
	s.Require().NotNil(newAcc)
	s.Require().NotNil(newAcc.GetPubKey())
	// The pubkey on the account should derive to newAddr (the multisig address).
	s.Require().Equal(newAddr.Bytes(), newAcc.GetPubKey().Address().Bytes())
	// And the actual pubkey bytes should match the reconstructed multiPK bytes.
	s.Require().Equal(newMultiPK.Bytes(), newAcc.GetPubKey().Bytes())

	// Balances moved.
	legacyBal := s.app.BankKeeper.GetAllBalances(s.ctx, legacyAddr)
	s.Require().True(legacyBal.IsZero())
	newBal := s.app.BankKeeper.GetBalance(s.ctx, newAddr, "ulume")
	s.Require().Equal(int64(1_000_000_000), newBal.Amount.Int64())
}

// TestMigrateValidator_MultisigToMultisig migrates a validator whose operator
// is a 2-of-3 Cosmos multisig to a 2-of-3 eth_secp256k1 multisig destination.
// It asserts structural re-keying (validator record, delegations, distribution,
// balance, migration record) and then invokes MsgEditValidator with the new
// multisig-eth ValAddress to prove the staking module accepts the re-keyed
// operator post-migration.
//
// NOTE: This integration layer calls msgServer.X(ctx, msg) directly and does
// NOT exercise the ante handler where multisig-eth signature verification
// happens. Full ante-handler-driven signature verification for multisig-eth
// signing is covered by devnet integration tests (Tasks 23-24) using a real
// lumerad binary.
func (s *MigrationIntegrationSuite) TestMigrateValidator_MultisigToMultisig() {
	s.enableMigration()

	// === Setup: legacy Cosmos multisig operator, bonded validator, and
	// destination eth multisig. ===
	operatorCoins := sdk.NewCoins(sdk.NewInt64Coin("ulume", 2_000_000))
	selfBondAmt := sdkmath.NewInt(1_000_000)
	legacyMultiPK, legacyPrivs, legacyAddr := s.createFundedMultisigAccount(2, 3, operatorCoins)
	oldValAddr, extDelegatorAddr := s.createTestValidator(legacyAddr, selfBondAmt)

	newMultiPK, newPrivs, newAddr := BuildMultisigNewAccount(s.T(), 2, 3)
	newValAddr := sdk.ValAddress(newAddr)

	// === Migrate multisig → multisig. ===
	legacyProof := SignMultisigProof(s.T(), integrationTestChainID, "validator",
		legacyMultiPK, legacyPrivs, []int{0, 2}, legacyAddr, newAddr,
		types.SigFormat_SIG_FORMAT_CLI)
	newProof := SignNewMultisigProof(s.T(), integrationTestChainID, "validator",
		newMultiPK, newPrivs, []int{0, 2}, legacyAddr, newAddr,
		types.SigFormat_SIG_FORMAT_CLI)

	msg := &types.MsgMigrateValidator{
		LegacyAddress: legacyAddr.String(),
		NewAddress:    newAddr.String(),
		LegacyProof:   *legacyProof,
		NewProof:      *newProof,
	}
	_, err := s.msgServer.MigrateValidator(s.ctx, msg)
	s.Require().NoError(err)

	// === Assert structural re-keying. ===
	// Validator record re-keyed to the new multisig-eth valoper.
	newVal, err := s.app.StakingKeeper.GetValidator(s.ctx, newValAddr)
	s.Require().NoError(err)
	s.Require().Equal(newValAddr.String(), newVal.OperatorAddress)
	s.Require().Equal(stakingtypes.Bonded, newVal.Status)

	// Old operator's validator record is orphaned (no delegations remain);
	// removing a bonded validator record outright would destroy distribution
	// state, so the migration leaves it dangling. The new record is canonical.
	oldDels, err := s.app.StakingKeeper.GetValidatorDelegations(s.ctx, oldValAddr)
	s.Require().NoError(err)
	s.Require().Empty(oldDels)

	// Delegations re-keyed (self + external).
	dels, err := s.app.StakingKeeper.GetValidatorDelegations(s.ctx, newValAddr)
	s.Require().NoError(err)
	s.Require().Len(dels, 2)
	for _, del := range dels {
		s.Require().Equal(newValAddr.String(), del.ValidatorAddress)
	}

	// External delegator's delegation now points at new valoper.
	extDels, err := s.app.StakingKeeper.GetDelegatorDelegations(s.ctx, extDelegatorAddr, 10)
	s.Require().NoError(err)
	s.Require().Len(extDels, 1)
	s.Require().Equal(newValAddr.String(), extDels[0].ValidatorAddress)

	// Distribution state re-keyed.
	_, err = s.app.DistrKeeper.GetValidatorCurrentRewards(s.ctx, newValAddr)
	s.Require().NoError(err)

	// Destination BaseAccount carries the reconstructed eth multisig pubkey.
	newAcc := s.app.AuthKeeper.GetAccount(s.ctx, newAddr)
	s.Require().NotNil(newAcc)
	s.Require().NotNil(newAcc.GetPubKey())
	s.Require().Equal(newAddr.Bytes(), newAcc.GetPubKey().Address().Bytes())
	s.Require().Equal(newMultiPK.Bytes(), newAcc.GetPubKey().Bytes())

	// Legacy balance fully moved.
	legacyBal := s.app.BankKeeper.GetAllBalances(s.ctx, legacyAddr)
	s.Require().True(legacyBal.IsZero())

	// Migration record created.
	rec, err := s.keeper.MigrationRecords.Get(s.ctx, legacyAddr.String())
	s.Require().NoError(err)
	s.Require().Equal(newAddr.String(), rec.NewAddress)

	// === Post-migration MsgEditValidator updates the moniker. ===
	// This proves the new multisig-eth operator is accepted by the staking
	// module. Signature verification is NOT exercised at this layer (ante
	// bypassed); devnet tests exercise the full signed-tx path.
	editMsg := &stakingtypes.MsgEditValidator{
		ValidatorAddress: newValAddr.String(),
		Description: stakingtypes.Description{
			Moniker:         "edited-by-multisig-eth",
			Identity:        stakingtypes.DoNotModifyDesc,
			Website:         stakingtypes.DoNotModifyDesc,
			SecurityContact: stakingtypes.DoNotModifyDesc,
			Details:         stakingtypes.DoNotModifyDesc,
		},
	}
	stakingMsgServer := stakingkeeper.NewMsgServerImpl(s.app.StakingKeeper)
	_, err = stakingMsgServer.EditValidator(s.ctx, editMsg)
	s.Require().NoError(err)

	updatedVal, err := s.app.StakingKeeper.GetValidator(s.ctx, newValAddr)
	s.Require().NoError(err)
	s.Require().Equal("edited-by-multisig-eth", updatedVal.Description.Moniker)
}

// TestClaimLegacyAccount_MultisigVesting_ToMultisig migrates a 2-of-3 Cosmos
// multisig legacy account that is wrapped in a ContinuousVestingAccount to a
// 2-of-3 eth_secp256k1 multisig destination. Asserts that:
//   - Destination account is a ContinuousVestingAccount.
//   - StartTime / EndTime / OriginalVesting are preserved.
//   - Destination BaseAccount.PubKey is the reconstructed eth multisig pubkey.
//   - Balance moved; migration record written.
func (s *MigrationIntegrationSuite) TestClaimLegacyAccount_MultisigVesting_ToMultisig() {
	s.enableMigration()

	coins := sdk.NewCoins(sdk.NewInt64Coin("ulume", 3_000_000))
	legacyMultiPK, legacyPrivs, legacyAddr := s.createFundedMultisigAccount(2, 3, coins)

	// Wrap the existing BaseAccount into a ContinuousVestingAccount. The helper
	// just stored a BaseAccount with the multisig pubkey; we replace it with a
	// continuous-vesting wrapper preserving the same PubKey / AccountNumber /
	// Sequence so downstream migration sees a real vesting account.
	legacyBase, ok := s.app.AuthKeeper.GetAccount(s.ctx, legacyAddr).(*authtypes.BaseAccount)
	s.Require().True(ok, "legacy multisig account must start as BaseAccount")

	startTime := s.ctx.BlockTime().Unix()
	endTime := s.ctx.BlockTime().Add(365 * 24 * time.Hour).Unix()
	bva, err := vestingtypes.NewBaseVestingAccount(legacyBase, coins, endTime)
	s.Require().NoError(err)
	cva := vestingtypes.NewContinuousVestingAccountRaw(bva, startTime)
	s.app.AuthKeeper.SetAccount(s.ctx, cva)

	newMultiPK, newPrivs, newAddr := BuildMultisigNewAccount(s.T(), 2, 3)

	legacyProof := SignMultisigProof(s.T(), integrationTestChainID, "claim",
		legacyMultiPK, legacyPrivs, []int{0, 2}, legacyAddr, newAddr,
		types.SigFormat_SIG_FORMAT_CLI)
	newProof := SignNewMultisigProof(s.T(), integrationTestChainID, "claim",
		newMultiPK, newPrivs, []int{0, 2}, legacyAddr, newAddr,
		types.SigFormat_SIG_FORMAT_CLI)

	msg := &types.MsgClaimLegacyAccount{
		LegacyAddress: legacyAddr.String(),
		NewAddress:    newAddr.String(),
		LegacyProof:   *legacyProof,
		NewProof:      *newProof,
	}
	_, err = s.msgServer.ClaimLegacyAccount(s.ctx, msg)
	s.Require().NoError(err)

	// Destination account must be a ContinuousVestingAccount with preserved fields.
	newAcc := s.app.AuthKeeper.GetAccount(s.ctx, newAddr)
	newCVA, ok := newAcc.(*vestingtypes.ContinuousVestingAccount)
	s.Require().True(ok, "new account should be ContinuousVestingAccount, got %T", newAcc)
	s.Require().Equal(startTime, newCVA.StartTime, "StartTime preserved")
	s.Require().Equal(endTime, newCVA.EndTime, "EndTime preserved")
	s.Require().Equal(coins, newCVA.OriginalVesting, "OriginalVesting preserved")

	// BaseAccount PubKey equals the reconstructed eth multisig pubkey.
	s.Require().NotNil(newCVA.GetPubKey())
	s.Require().Equal(newAddr.Bytes(), newCVA.GetPubKey().Address().Bytes())
	s.Require().Equal(newMultiPK.Bytes(), newCVA.GetPubKey().Bytes())

	// Balance moved.
	legacyBal := s.app.BankKeeper.GetAllBalances(s.ctx, legacyAddr)
	s.Require().True(legacyBal.IsZero())
	newBal := s.app.BankKeeper.GetAllBalances(s.ctx, newAddr)
	s.Require().True(newBal.AmountOf("ulume").Equal(sdkmath.NewInt(3_000_000)))

	// Migration record present.
	rec, err := s.keeper.MigrationRecords.Get(s.ctx, legacyAddr.String())
	s.Require().NoError(err)
	s.Require().Equal(newAddr.String(), rec.NewAddress)
}

// TestClaimLegacyAccount_Multisig_WrongThreshold_LegacySide asserts that a
// MultisigProof with fewer than K legacy sub-signatures is rejected by
// ValidateBasic (threshold/signer_indices mismatch).
func (s *MigrationIntegrationSuite) TestClaimLegacyAccount_Multisig_WrongThreshold_LegacySide() {
	s.enableMigration()

	coins := sdk.NewCoins(sdk.NewInt64Coin("ulume", 500_000))
	legacyMultiPK, legacyPrivs, legacyAddr := s.createFundedMultisigAccount(2, 3, coins)
	newPrivKey, newAddr := createNewEVMAddress(s.T())

	// Build a 2-of-3 proof, then truncate to just one sub-sig / signer index
	// so the proof is under threshold. The MultisigProof's Threshold field is
	// still 2 (matching legacyMultiPK), but only 1 signer is supplied.
	proof := SignMultisigProof(s.T(), integrationTestChainID, "claim",
		legacyMultiPK, legacyPrivs, []int{0, 2}, legacyAddr, newAddr,
		types.SigFormat_SIG_FORMAT_CLI)
	proof.GetMultisig().SignerIndices = proof.GetMultisig().SignerIndices[:1]
	proof.GetMultisig().SubSignatures = proof.GetMultisig().SubSignatures[:1]

	msg := &types.MsgClaimLegacyAccount{
		LegacyAddress: legacyAddr.String(),
		NewAddress:    newAddr.String(),
		LegacyProof:   *proof,
		NewProof: types.MigrationProof{Proof: &types.MigrationProof_Single{Single: &types.SingleKeyProof{
			PubKey:    newPrivKey.PubKey().(*evmcryptotypes.PubKey).Key,
			Signature: signNewMigration(s.T(), "claim", newPrivKey, legacyAddr, newAddr),
			SigFormat: types.SigFormat_SIG_FORMAT_CLI,
		}}},
	}
	_, err := s.msgServer.ClaimLegacyAccount(s.ctx, msg)
	s.Require().Error(err)
	// ValidateBasic rejects with "expected exactly K=... signer_indices, got ...".
	s.Require().ErrorContains(err, "signer_indices")
}

// TestClaimLegacyAccount_Multisig_WrongThreshold_NewSide asserts that a new-side
// MultisigProof with fewer than K sub-signatures is rejected.
func (s *MigrationIntegrationSuite) TestClaimLegacyAccount_Multisig_WrongThreshold_NewSide() {
	s.enableMigration()

	coins := sdk.NewCoins(sdk.NewInt64Coin("ulume", 500_000))
	legacyMultiPK, legacyPrivs, legacyAddr := s.createFundedMultisigAccount(2, 3, coins)
	newMultiPK, newPrivs, newAddr := BuildMultisigNewAccount(s.T(), 2, 3)

	legacyProof := SignMultisigProof(s.T(), integrationTestChainID, "claim",
		legacyMultiPK, legacyPrivs, []int{0, 2}, legacyAddr, newAddr,
		types.SigFormat_SIG_FORMAT_CLI)

	newProof := SignNewMultisigProof(s.T(), integrationTestChainID, "claim",
		newMultiPK, newPrivs, []int{0, 2}, legacyAddr, newAddr,
		types.SigFormat_SIG_FORMAT_CLI)
	// Drop one sub-sig / signer index on the new side to drop below threshold.
	newProof.GetMultisig().SignerIndices = newProof.GetMultisig().SignerIndices[:1]
	newProof.GetMultisig().SubSignatures = newProof.GetMultisig().SubSignatures[:1]

	msg := &types.MsgClaimLegacyAccount{
		LegacyAddress: legacyAddr.String(),
		NewAddress:    newAddr.String(),
		LegacyProof:   *legacyProof,
		NewProof:      *newProof,
	}
	_, err := s.msgServer.ClaimLegacyAccount(s.ctx, msg)
	s.Require().Error(err)
	s.Require().ErrorContains(err, "signer_indices")
}

// TestClaimLegacyAccount_Multisig_ADR036_BothSides migrates a 2-of-3 Cosmos
// multisig to a 2-of-3 eth_secp256k1 multisig with SIG_FORMAT_ADR036 on BOTH
// sides. This exercises the path where neither side uses the default CLI hash
// format and covers the ADR-036 new-side verifier introduced in Task 19.
func (s *MigrationIntegrationSuite) TestClaimLegacyAccount_Multisig_ADR036_BothSides() {
	s.enableMigration()

	coins := sdk.NewCoins(sdk.NewInt64Coin("ulume", 750_000))
	legacyMultiPK, legacyPrivs, legacyAddr := s.createFundedMultisigAccount(2, 3, coins)
	newMultiPK, newPrivs, newAddr := BuildMultisigNewAccount(s.T(), 2, 3)

	legacyProof := SignMultisigProof(s.T(), integrationTestChainID, "claim",
		legacyMultiPK, legacyPrivs, []int{0, 1}, legacyAddr, newAddr,
		types.SigFormat_SIG_FORMAT_ADR036)
	newProof := SignNewMultisigProof(s.T(), integrationTestChainID, "claim",
		newMultiPK, newPrivs, []int{1, 2}, legacyAddr, newAddr,
		types.SigFormat_SIG_FORMAT_ADR036)

	msg := &types.MsgClaimLegacyAccount{
		LegacyAddress: legacyAddr.String(),
		NewAddress:    newAddr.String(),
		LegacyProof:   *legacyProof,
		NewProof:      *newProof,
	}
	_, err := s.msgServer.ClaimLegacyAccount(s.ctx, msg)
	s.Require().NoError(err)

	// Balances moved; migration record present; destination pubkey is the
	// reconstructed eth multisig.
	legacyBal := s.app.BankKeeper.GetAllBalances(s.ctx, legacyAddr)
	s.Require().True(legacyBal.IsZero())
	newBal := s.app.BankKeeper.GetBalance(s.ctx, newAddr, "ulume")
	s.Require().Equal(int64(750_000), newBal.Amount.Int64())

	rec, err := s.keeper.MigrationRecords.Get(s.ctx, legacyAddr.String())
	s.Require().NoError(err)
	s.Require().Equal(newAddr.String(), rec.NewAddress)

	newAcc := s.app.AuthKeeper.GetAccount(s.ctx, newAddr)
	s.Require().NotNil(newAcc)
	s.Require().NotNil(newAcc.GetPubKey())
	s.Require().Equal(newMultiPK.Bytes(), newAcc.GetPubKey().Bytes())
}

// --- Mirror-source rule at full Msg*.ValidateBasic path ---
//
// Each of the four tests below exercises a consensus invariant (mirror-source
// shape, cross-side K/N, matching signer_indices, sub-key uniqueness) via a
// hand-crafted MsgClaimLegacyAccount. The cross-side pair check
// (ValidateProofPair) lives in Msg*.ValidateBasic, which in production is
// auto-invoked by baseapp's msg_service_router before dispatch. These tests
// call s.msgServer.ClaimLegacyAccount directly (bypassing the router), so
// they explicitly invoke msg.ValidateBasic() first to mirror production —
// otherwise the cross-side check wouldn't fire and we'd only exercise the
// per-proof validation path inside VerifyMigrationProof.

// TestClaimLegacyAccount_Multisig_MirrorSourceMismatch_Shape pairs a multisig
// legacy proof with a single-key new proof. ValidateProofPair rejects shape
// mismatches with ErrMirrorSourceMismatch before any crypto work.
func (s *MigrationIntegrationSuite) TestClaimLegacyAccount_Multisig_MirrorSourceMismatch_Shape() {
	s.enableMigration()

	coins := sdk.NewCoins(sdk.NewInt64Coin("ulume", 500_000))
	legacyMultiPK, legacyPrivs, legacyAddr := s.createFundedMultisigAccount(2, 3, coins)
	newPrivKey, newAddr := createNewEVMAddress(s.T())

	legacyProof := SignMultisigProof(s.T(), integrationTestChainID, "claim",
		legacyMultiPK, legacyPrivs, []int{0, 2}, legacyAddr, newAddr,
		types.SigFormat_SIG_FORMAT_CLI)
	// Pair it with a SINGLE-KEY new proof — shape mismatch.
	msg := &types.MsgClaimLegacyAccount{
		LegacyAddress: legacyAddr.String(),
		NewAddress:    newAddr.String(),
		LegacyProof:   *legacyProof,
		NewProof: types.MigrationProof{Proof: &types.MigrationProof_Single{Single: &types.SingleKeyProof{
			PubKey:    newPrivKey.PubKey().(*evmcryptotypes.PubKey).Key,
			Signature: signNewMigration(s.T(), "claim", newPrivKey, legacyAddr, newAddr),
			SigFormat: types.SigFormat_SIG_FORMAT_CLI,
		}}},
	}
	// Mirror production: msg_service_router calls msg.ValidateBasic() before
	// dispatch. That's where ValidateProofPair fires.
	err := msg.ValidateBasic()
	if err == nil {
		_, err = s.msgServer.ClaimLegacyAccount(s.ctx, msg)
	}
	s.Require().Error(err)
	s.Require().ErrorIs(err, types.ErrMirrorSourceMismatch)
	s.Require().ErrorContains(err, "shape")
}

// TestClaimLegacyAccount_Multisig_MirrorSourceMismatch_KN pairs a 2-of-3
// legacy with a 3-of-5 new — same shape, different K and N. ValidateProofPair
// rejects with ErrMirrorSourceMismatch before verifying either proof.
func (s *MigrationIntegrationSuite) TestClaimLegacyAccount_Multisig_MirrorSourceMismatch_KN() {
	s.enableMigration()

	coins := sdk.NewCoins(sdk.NewInt64Coin("ulume", 500_000))
	legacyMultiPK, legacyPrivs, legacyAddr := s.createFundedMultisigAccount(2, 3, coins)
	newMultiPK, newPrivs, newAddr := BuildMultisigNewAccount(s.T(), 3, 5) // K=3, N=5

	legacyProof := SignMultisigProof(s.T(), integrationTestChainID, "claim",
		legacyMultiPK, legacyPrivs, []int{0, 2}, legacyAddr, newAddr,
		types.SigFormat_SIG_FORMAT_CLI)
	newProof := SignNewMultisigProof(s.T(), integrationTestChainID, "claim",
		newMultiPK, newPrivs, []int{0, 2, 4}, legacyAddr, newAddr,
		types.SigFormat_SIG_FORMAT_CLI)

	msg := &types.MsgClaimLegacyAccount{
		LegacyAddress: legacyAddr.String(),
		NewAddress:    newAddr.String(),
		LegacyProof:   *legacyProof,
		NewProof:      *newProof,
	}
	err := msg.ValidateBasic()
	if err == nil {
		_, err = s.msgServer.ClaimLegacyAccount(s.ctx, msg)
	}
	s.Require().Error(err)
	s.Require().ErrorIs(err, types.ErrMirrorSourceMismatch)
	// Either threshold or sub_pub_keys count will be the rejection reason;
	// both qualify — just pin the rule.
}

// TestClaimLegacyAccount_Multisig_SignerIndicesMismatch pairs legacy signed
// at [0,1] with new signed at [0,2] — same shape and K/N, but the two
// K-subsets are disjoint. ValidateProofPair requires legacy_proof.
// signer_indices == new_proof.signer_indices.
func (s *MigrationIntegrationSuite) TestClaimLegacyAccount_Multisig_SignerIndicesMismatch() {
	s.enableMigration()

	coins := sdk.NewCoins(sdk.NewInt64Coin("ulume", 500_000))
	legacyMultiPK, legacyPrivs, legacyAddr := s.createFundedMultisigAccount(2, 3, coins)
	newMultiPK, newPrivs, newAddr := BuildMultisigNewAccount(s.T(), 2, 3)

	// Legacy signs at [0,1]; new signs at [0,2]. Cross-side K-subsets disjoint.
	legacyProof := SignMultisigProof(s.T(), integrationTestChainID, "claim",
		legacyMultiPK, legacyPrivs, []int{0, 1}, legacyAddr, newAddr,
		types.SigFormat_SIG_FORMAT_CLI)
	newProof := SignNewMultisigProof(s.T(), integrationTestChainID, "claim",
		newMultiPK, newPrivs, []int{0, 2}, legacyAddr, newAddr,
		types.SigFormat_SIG_FORMAT_CLI)

	msg := &types.MsgClaimLegacyAccount{
		LegacyAddress: legacyAddr.String(),
		NewAddress:    newAddr.String(),
		LegacyProof:   *legacyProof,
		NewProof:      *newProof,
	}
	err := msg.ValidateBasic()
	if err == nil {
		_, err = s.msgServer.ClaimLegacyAccount(s.ctx, msg)
	}
	s.Require().Error(err)
	s.Require().ErrorIs(err, types.ErrMirrorSourceMismatch)
	s.Require().ErrorContains(err, "signer_indices")
}

// TestClaimLegacyAccount_Multisig_DuplicateSubKey_Submit builds an otherwise-
// valid multisig→multisig pair, then mutates the legacy-side sub_pub_keys to
// duplicate position 0 into position 2. MultisigProof.validateBasic rejects
// this at Msg*.ValidateBasic with ErrInvalidMigrationPubKey. Complements the
// preflight coverage in TestMigrationEstimate_Multisig_DuplicateSubKey.
func (s *MigrationIntegrationSuite) TestClaimLegacyAccount_Multisig_DuplicateSubKey_Submit() {
	s.enableMigration()

	coins := sdk.NewCoins(sdk.NewInt64Coin("ulume", 500_000))
	legacyMultiPK, legacyPrivs, legacyAddr := s.createFundedMultisigAccount(2, 3, coins)
	newMultiPK, newPrivs, newAddr := BuildMultisigNewAccount(s.T(), 2, 3)

	legacyProof := SignMultisigProof(s.T(), integrationTestChainID, "claim",
		legacyMultiPK, legacyPrivs, []int{0, 2}, legacyAddr, newAddr,
		types.SigFormat_SIG_FORMAT_CLI)
	// Duplicate legacy sub-key at position 0 into position 2 after signing —
	// ValidateBasic runs before VerifyMigrationProof, so we can stay with the
	// original sub-sigs; the test checks the structural rejection only.
	lm := legacyProof.GetMultisig()
	lm.SubPubKeys[2] = append([]byte(nil), lm.SubPubKeys[0]...)
	newProof := SignNewMultisigProof(s.T(), integrationTestChainID, "claim",
		newMultiPK, newPrivs, []int{0, 2}, legacyAddr, newAddr,
		types.SigFormat_SIG_FORMAT_CLI)

	msg := &types.MsgClaimLegacyAccount{
		LegacyAddress: legacyAddr.String(),
		NewAddress:    newAddr.String(),
		LegacyProof:   *legacyProof,
		NewProof:      *newProof,
	}
	_, err := s.msgServer.ClaimLegacyAccount(s.ctx, msg)
	s.Require().Error(err)
	s.Require().ErrorIs(err, types.ErrInvalidMigrationPubKey)
	s.Require().ErrorContains(err, "duplicates sub_pub_keys[0]")
}

// TestMigrateValidator_RejectsDestinationAlreadyValidator verifies that
// MigrateValidator rejects a destination address whose ValAddress is already a
// validator operator. Without the guard, MigrateValidatorRecord.SetValidator
// would silently overwrite the pre-existing destination validator record.
func (s *MigrationIntegrationSuite) TestMigrateValidator_RejectsDestinationAlreadyValidator() {
	s.enableMigration()

	// Legacy validator (source of migration).
	selfBondAmt := sdkmath.NewInt(1_000_000)
	operatorCoins := sdk.NewCoins(sdk.NewInt64Coin("ulume", 2_000_000))
	legacyPrivKey, legacyAddr := s.createFundedLegacyAccount(operatorCoins)
	s.createTestValidator(legacyAddr, selfBondAmt)

	// Pre-seed the destination address as an existing validator.
	// createTestValidator treats its arg as a funded legacy account whose
	// ValAddress becomes the validator operator, so we create a funded legacy
	// account at newAddr and use it as the destination. Using a secp256k1
	// destination here is fine: the check under test runs before signature
	// verification, so an EVM destination signature is not required.
	_, newAddr := s.createFundedLegacyAccount(sdk.NewCoins(sdk.NewInt64Coin("ulume", 2_000_000)))
	s.createTestValidator(newAddr, sdkmath.NewInt(500_000))

	// Build a MigrateValidator message. Destination proof is not reachable in
	// this test (the guard fires first), so an EVM key is unnecessary — we use
	// a dummy evmcryptotypes key just to populate the message structurally.
	newPrivKey, err := evmcryptotypes.GenerateKey()
	s.Require().NoError(err)
	msg := &types.MsgMigrateValidator{
		LegacyAddress: legacyAddr.String(),
		NewAddress:    newAddr.String(),
		LegacyProof: types.MigrationProof{Proof: &types.MigrationProof_Single{Single: &types.SingleKeyProof{
			PubKey:    legacyPrivKey.PubKey().(*secp256k1.PubKey).Key,
			Signature: signValidatorMigration(s.T(), legacyPrivKey, legacyAddr, newAddr),
			SigFormat: types.SigFormat_SIG_FORMAT_CLI,
		}}},
		NewProof: types.MigrationProof{Proof: &types.MigrationProof_Single{Single: &types.SingleKeyProof{
			PubKey:    newPrivKey.PubKey().(*evmcryptotypes.PubKey).Key,
			Signature: signNewMigration(s.T(), "validator", newPrivKey, legacyAddr, newAddr),
			SigFormat: types.SigFormat_SIG_FORMAT_CLI,
		}}},
	}

	_, err = s.msgServer.MigrateValidator(s.ctx, msg)
	s.Require().Error(err)
	s.Require().ErrorIs(err, types.ErrNewAddressIsValidator)
	s.Require().ErrorContains(err, "already a validator operator")
}

// TestQueryLegacyAccounts_Pagination_KeyRoundtrip verifies that
// LegacyAccounts honors Pagination.Key as a big-endian-encoded offset,
// allowing clients to round-trip pages via NextKey from the prior response.
//
// The test tolerates pre-existing funded accounts from the test genesis
// (e.g. the default validator operator): what we verify is that
//
//   - each page emits NextKey of length 8 while more results remain;
//   - feeding NextKey back as Pagination.Key advances past the prior page
//     (no duplicates across consecutive pages);
//   - the final page (covering the Total) emits NextKey == nil.
func (s *MigrationIntegrationSuite) TestQueryLegacyAccounts_Pagination_KeyRoundtrip() {
	s.enableMigration()
	qs := evmigrationkeeper.NewQueryServerImpl(s.keeper)

	// Five additional funded legacy accounts on top of whatever the genesis
	// fixture already has.
	coins := sdk.NewCoins(sdk.NewInt64Coin("ulume", 1_000))
	for i := 0; i < 5; i++ {
		s.createFundedLegacyAccount(coins)
	}

	// Page 1: Limit=2, no Key/Offset.
	resp1, err := qs.LegacyAccounts(s.ctx, &types.QueryLegacyAccountsRequest{
		Pagination: &query.PageRequest{Limit: 2},
	})
	s.Require().NoError(err)
	s.Require().Len(resp1.Accounts, 2)
	s.Require().NotNil(resp1.Pagination)
	total := resp1.Pagination.Total
	s.Require().GreaterOrEqual(total, uint64(5), "at least the 5 created accounts should be enumerated")
	s.Require().Len(resp1.Pagination.NextKey, 8, "NextKey must be big-endian uint64 offset")

	// Page 2: Limit=2, Key=resp1.NextKey. Must be disjoint from page 1.
	resp2, err := qs.LegacyAccounts(s.ctx, &types.QueryLegacyAccountsRequest{
		Pagination: &query.PageRequest{Key: resp1.Pagination.NextKey, Limit: 2},
	})
	s.Require().NoError(err)
	s.Require().NotEmpty(resp2.Accounts)
	s.Require().LessOrEqual(len(resp2.Accounts), 2)
	s.Require().NotNil(resp2.Pagination)

	page1Addrs := map[string]bool{resp1.Accounts[0].Address: true, resp1.Accounts[1].Address: true}
	for _, a := range resp2.Accounts {
		s.Require().False(page1Addrs[a.Address], "page 2 account %s duplicates page 1", a.Address)
	}

	// Walk remaining pages using each response's NextKey until it is nil. The
	// final page must have NextKey == nil; intermediate pages must have
	// NextKey of length 8.
	lastNextKey := resp2.Pagination.NextKey
	seen := 0
	for i := 0; lastNextKey != nil; i++ {
		s.Require().Len(lastNextKey, 8, "intermediate NextKey must be 8-byte big-endian offset")
		resp, err := qs.LegacyAccounts(s.ctx, &types.QueryLegacyAccountsRequest{
			Pagination: &query.PageRequest{Key: lastNextKey, Limit: 2},
		})
		s.Require().NoError(err)
		s.Require().NotEmpty(resp.Accounts)
		lastNextKey = resp.Pagination.NextKey
		seen += len(resp.Accounts)
		s.Require().Less(i, 100, "pagination loop runaway")
	}
	s.Require().Nil(lastNextKey, "final page must not emit NextKey")
	_ = seen
}

// TestMigrateValidator_NoOrphanedValidatorRecord asserts that after a successful
// validator migration, the staking keeper's main validator store has exactly
// ONE record for this operator (at newValAddr, not oldValAddr). Locks in the
// fix for Finding #1: DeleteValidatorRecordNoHooks removes the dead old row.
func (s *MigrationIntegrationSuite) TestMigrateValidator_NoOrphanedValidatorRecord() {
	s.enableMigration()

	// Use the existing single-key validator setup.
	selfBondAmt := sdkmath.NewInt(1_000_000)
	operatorCoins := sdk.NewCoins(sdk.NewInt64Coin("ulume", 2_000_000))
	legacyPrivKey, legacyAddr := s.createFundedLegacyAccount(operatorCoins)
	oldValAddr, _ := s.createTestValidator(legacyAddr, selfBondAmt)

	newPrivKey, newAddr := createNewEVMAddress(s.T())
	newValAddr := sdk.ValAddress(newAddr)

	msg := newValidatorMsg(s.T(), legacyPrivKey, legacyAddr, newPrivKey, newAddr)
	_, err := s.msgServer.MigrateValidator(s.ctx, msg)
	s.Require().NoError(err)

	// New record exists.
	_, err = s.app.StakingKeeper.GetValidator(s.ctx, newValAddr)
	s.Require().NoError(err)

	// Old record GONE (the fix).
	_, err = s.app.StakingKeeper.GetValidator(s.ctx, oldValAddr)
	s.Require().Error(err, "old validator row must be deleted after migration")
	s.Require().True(errorsmod.IsOf(err, stakingtypes.ErrNoValidatorFound),
		"unexpected error from GetValidator on deleted old addr: %v", err)

	// And iterating all validators must NOT surface both rows.
	allVals, err := s.app.StakingKeeper.GetAllValidators(s.ctx)
	s.Require().NoError(err)
	for _, v := range allVals {
		s.Require().NotEqual(oldValAddr.String(), v.OperatorAddress,
			"GetAllValidators must not surface the orphaned old operator")
	}
}
