package integration_test

import (
	"bytes"
	"strings"
	"testing"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	evmigrationkeeper "github.com/LumeraProtocol/lumera/x/evmigration/keeper"
	evmigrationtypes "github.com/LumeraProtocol/lumera/x/evmigration/types"
	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

func validMigrationSupernode(valAddr sdk.ValAddress, account sdk.AccAddress) sntypes.SuperNode {
	return sntypes.SuperNode{
		ValidatorAddress: valAddr.String(),
		SupernodeAccount: account.String(),
		Note:             "1.0.0",
		PrevIpAddresses:  []*sntypes.IPAddressHistory{{Address: "127.0.0.1", Height: 1}},
		States:           []*sntypes.SuperNodeStateRecord{{State: sntypes.SuperNodeStateActive, Height: 1}},
		P2PPort:          "4445",
	}
}

func (s *MigrationIntegrationSuite) supernodeStoreSnapshot() map[string][]byte {
	store := s.ctx.KVStore(s.app.GetKey(sntypes.StoreKey))
	it := store.Iterator(nil, nil)
	s.Require().NoError(it.Error())
	defer it.Close()

	snapshot := make(map[string][]byte)
	for ; it.Valid(); it.Next() {
		snapshot[string(bytes.Clone(it.Key()))] = bytes.Clone(it.Value())
	}
	return snapshot
}

func (s *MigrationIntegrationSuite) putSupernodePrimary(valAddr sdk.ValAddress, sn sntypes.SuperNode) {
	store := s.ctx.KVStore(s.app.GetKey(sntypes.StoreKey))
	store.Set(sntypes.GetSupernodeKey(valAddr), s.app.AppCodec().MustMarshal(&sn))
}

func (s *MigrationIntegrationSuite) putSupernodeIndex(account sdk.AccAddress, valAddr sdk.ValAddress) {
	store := s.ctx.KVStore(s.app.GetKey(sntypes.StoreKey))
	key := append(bytes.Clone(sntypes.SuperNodeByAccountKey), []byte(account.String())...)
	store.Set(key, valAddr)
}

func (s *MigrationIntegrationSuite) putSupernodeIndexText(account string, valAddr sdk.ValAddress) {
	store := s.ctx.KVStore(s.app.GetKey(sntypes.StoreKey))
	key := append(bytes.Clone(sntypes.SuperNodeByAccountKey), []byte(account)...)
	store.Set(key, valAddr)
}

func (s *MigrationIntegrationSuite) assertClaimOwnershipCorruptionRejected(
	setup func(legacyAddr sdk.AccAddress),
	wantErr string,
) {
	s.enableMigration()
	coins := sdk.NewCoins(sdk.NewInt64Coin("ulume", 123_456))
	legacyPrivKey, legacyAddr := s.createFundedLegacyAccount(coins)
	newPrivKey, newAddr := createNewEVMAddress(s.T())
	setup(legacyAddr)

	beforeStore := s.supernodeStoreSnapshot()
	beforeLegacy := s.app.BankKeeper.GetAllBalances(s.ctx, legacyAddr)
	beforeNew := s.app.BankKeeper.GetAllBalances(s.ctx, newAddr)

	_, err := s.msgServer.ClaimLegacyAccount(s.ctx, newClaimMsg(s.T(), legacyPrivKey, legacyAddr, newPrivKey, newAddr))
	s.Require().Error(err)
	s.Require().Contains(err.Error(), wantErr)
	s.Require().Equal(beforeStore, s.supernodeStoreSnapshot(), "failed DeliverTx must not mutate supernode primary/index state")
	s.Require().Equal(beforeLegacy, s.app.BankKeeper.GetAllBalances(s.ctx, legacyAddr), "strict ownership failure must precede balance migration")
	s.Require().Equal(beforeNew, s.app.BankKeeper.GetAllBalances(s.ctx, newAddr), "strict ownership failure must precede destination writes")
	hasRecord, recordErr := s.keeper.MigrationRecords.Has(s.ctx, legacyAddr.String())
	s.Require().NoError(recordErr)
	s.Require().False(hasRecord)
}

func (s *MigrationIntegrationSuite) TestClaimLegacyAccount_RejectsMissingSupernodeAccountIndexBeforeWrites() {
	s.assertClaimOwnershipCorruptionRejected(func(legacyAddr sdk.AccAddress) {
		valAddr := sdk.ValAddress(testAddressBytes("missing-index-val"))
		s.putSupernodePrimary(valAddr, validMigrationSupernode(valAddr, legacyAddr))
	}, "missing account index")
}

func (s *MigrationIntegrationSuite) TestClaimLegacyAccount_RejectsStaleSupernodeAccountIndexBeforeWrites() {
	s.assertClaimOwnershipCorruptionRejected(func(legacyAddr sdk.AccAddress) {
		s.putSupernodeIndex(legacyAddr, sdk.ValAddress(testAddressBytes("stale-index-val")))
	}, "does not resolve to a primary record")
}

func (s *MigrationIntegrationSuite) TestClaimLegacyAccount_RejectsDuplicateSupernodeOwnershipBeforeWrites() {
	s.assertClaimOwnershipCorruptionRejected(func(legacyAddr sdk.AccAddress) {
		first := sdk.ValAddress(testAddressBytes("duplicate-owner-one"))
		second := sdk.ValAddress(testAddressBytes("duplicate-owner-two"))
		s.putSupernodePrimary(first, validMigrationSupernode(first, legacyAddr))
		s.putSupernodePrimary(second, validMigrationSupernode(second, legacyAddr))
		s.putSupernodeIndex(legacyAddr, first)
	}, "multiple primary records claim")
}

func (s *MigrationIntegrationSuite) TestClaimLegacyAccount_RejectsCanonicalDestinationSupernodeOwnerBeforeWrites() {
	s.enableMigration()
	coins := sdk.NewCoins(sdk.NewInt64Coin("ulume", 123_456))
	legacyPrivKey, legacyAddr := s.createFundedLegacyAccount(coins)
	newPrivKey, newAddr := createNewEVMAddress(s.T())

	sourceVal := sdk.ValAddress(testAddressBytes("claim-source-val"))
	s.Require().NoError(s.app.SupernodeKeeper.SetSuperNode(s.ctx, validMigrationSupernode(sourceVal, legacyAddr)))
	destinationVal := sdk.ValAddress(testAddressBytes("claim-dest-val"))
	destinationSN := validMigrationSupernode(destinationVal, newAddr)
	destinationSN.SupernodeAccount = strings.ToUpper(newAddr.String())
	s.putSupernodePrimary(destinationVal, destinationSN)
	s.putSupernodeIndexText(destinationSN.SupernodeAccount, destinationVal)

	beforeStore := s.supernodeStoreSnapshot()
	beforeLegacy := s.app.BankKeeper.GetAllBalances(s.ctx, legacyAddr)
	beforeNew := s.app.BankKeeper.GetAllBalances(s.ctx, newAddr)

	_, err := s.msgServer.ClaimLegacyAccount(s.ctx, newClaimMsg(s.T(), legacyPrivKey, legacyAddr, newPrivKey, newAddr))
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "destination supernode account")
	s.Require().Equal(beforeStore, s.supernodeStoreSnapshot())
	s.Require().Equal(beforeLegacy, s.app.BankKeeper.GetAllBalances(s.ctx, legacyAddr))
	s.Require().Equal(beforeNew, s.app.BankKeeper.GetAllBalances(s.ctx, newAddr))
}

func (s *MigrationIntegrationSuite) TestClaimLegacyAccount_RejectsMalformedSupernodePrimaryBeforeWrites() {
	s.assertClaimOwnershipCorruptionRejected(func(_ sdk.AccAddress) {
		valAddr := sdk.ValAddress(testAddressBytes("malformed-primary"))
		store := s.ctx.KVStore(s.app.GetKey(sntypes.StoreKey))
		store.Set(sntypes.GetSupernodeKey(valAddr), []byte{0xff, 0xff, 0xff})
	}, "unmarshal supernode")
}

func (s *MigrationIntegrationSuite) TestClaimLegacyAccount_RejectsMalformedEmbeddedSupernodeAccountBeforeWrites() {
	s.assertClaimOwnershipCorruptionRejected(func(_ sdk.AccAddress) {
		valAddr := sdk.ValAddress(testAddressBytes("malformed-account"))
		sn := validMigrationSupernode(valAddr, sdk.AccAddress(testAddressBytes("valid-account")))
		sn.SupernodeAccount = "not-a-lumera-address"
		s.putSupernodePrimary(valAddr, sn)
	}, "invalid embedded supernode account")
}

func (s *MigrationIntegrationSuite) TestMigrateValidator_RejectsSourceOwnershipCorruptionBeforeValidatorMutation() {
	s.enableMigration()
	operatorCoins := sdk.NewCoins(sdk.NewInt64Coin("ulume", 2_000_000))
	legacyPrivKey, legacyAddr := s.createFundedLegacyAccount(operatorCoins)
	oldValAddr, _ := s.createTestValidator(legacyAddr, sdkmath.NewInt(1_000_000))
	newPrivKey, newAddr := createNewEVMAddress(s.T())

	// A valid primary claiming the source account without its mandatory account
	// index must abort before V1 reward withdrawal or V2 validator re-keying.
	s.putSupernodePrimary(oldValAddr, validMigrationSupernode(oldValAddr, legacyAddr))
	beforeStore := s.supernodeStoreSnapshot()
	beforeValidator, err := s.app.StakingKeeper.GetValidator(s.ctx, oldValAddr)
	s.Require().NoError(err)

	_, err = s.msgServer.MigrateValidator(s.ctx, newValidatorMsg(s.T(), legacyPrivKey, legacyAddr, newPrivKey, newAddr))
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "missing account index")
	s.Require().Equal(beforeStore, s.supernodeStoreSnapshot())
	afterValidator, getErr := s.app.StakingKeeper.GetValidator(s.ctx, oldValAddr)
	s.Require().NoError(getErr)
	s.Require().Equal(beforeValidator, afterValidator, "ownership corruption must be rejected before validator mutation")
	_, newValidatorErr := s.app.StakingKeeper.GetValidator(s.ctx, sdk.ValAddress(newAddr))
	s.Require().Error(newValidatorErr)
}

func (s *MigrationIntegrationSuite) TestMigrateValidator_RejectsCanonicalDestinationSupernodeOwnerBeforeValidatorMutation() {
	s.enableMigration()
	operatorCoins := sdk.NewCoins(sdk.NewInt64Coin("ulume", 2_000_000))
	legacyPrivKey, legacyAddr := s.createFundedLegacyAccount(operatorCoins)
	oldValAddr, _ := s.createTestValidator(legacyAddr, sdkmath.NewInt(1_000_000))
	newPrivKey, newAddr := createNewEVMAddress(s.T())

	s.Require().NoError(s.app.SupernodeKeeper.SetSuperNode(s.ctx, validMigrationSupernode(oldValAddr, legacyAddr)))
	destinationVal := sdk.ValAddress(testAddressBytes("validator-dest-val"))
	destinationSN := validMigrationSupernode(destinationVal, newAddr)
	destinationSN.SupernodeAccount = strings.ToUpper(newAddr.String())
	s.putSupernodePrimary(destinationVal, destinationSN)
	s.putSupernodeIndexText(destinationSN.SupernodeAccount, destinationVal)

	beforeStore := s.supernodeStoreSnapshot()
	beforeValidator, err := s.app.StakingKeeper.GetValidator(s.ctx, oldValAddr)
	s.Require().NoError(err)

	_, err = s.msgServer.MigrateValidator(s.ctx, newValidatorMsg(s.T(), legacyPrivKey, legacyAddr, newPrivKey, newAddr))
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "destination supernode account")
	s.Require().Equal(beforeStore, s.supernodeStoreSnapshot())
	afterValidator, getErr := s.app.StakingKeeper.GetValidator(s.ctx, oldValAddr)
	s.Require().NoError(getErr)
	s.Require().Equal(beforeValidator, afterValidator)
}

func (s *MigrationIntegrationSuite) TestMigrateValidatorSupernode_TwoDistinctRecordsRealStore() {
	legacyAddr := sdk.AccAddress(testAddressBytes("legacy-owner"))
	newAddr := sdk.AccAddress(testAddressBytes("new-owner"))
	oldValAddr := sdk.ValAddress(legacyAddr)
	newValAddr := sdk.ValAddress(newAddr)
	accountOwnedVal := sdk.ValAddress(testAddressBytes("account-owned-val"))
	independentAccount := sdk.AccAddress(testAddressBytes("independent-owner"))

	accountOwned := validMigrationSupernode(accountOwnedVal, legacyAddr)
	accountOwned.PrevSupernodeAccounts = []*sntypes.SupernodeAccountHistory{{Account: legacyAddr.String(), Height: 7}}
	validatorAssociated := validMigrationSupernode(oldValAddr, independentAccount)
	validatorAssociated.PrevSupernodeAccounts = []*sntypes.SupernodeAccountHistory{{Account: independentAccount.String(), Height: 9}}

	s.Require().NoError(s.app.SupernodeKeeper.SetSuperNode(s.ctx, accountOwned))
	s.Require().NoError(s.app.SupernodeKeeper.SetSuperNode(s.ctx, validatorAssociated))
	s.Require().NoError(s.keeper.MigrateValidatorSupernode(s.ctx, oldValAddr, newValAddr, legacyAddr, newAddr))

	migratedOwned, found := s.app.SupernodeKeeper.QuerySuperNode(s.ctx, accountOwnedVal)
	s.Require().True(found)
	s.Require().Equal(newAddr.String(), migratedOwned.SupernodeAccount)
	s.Require().Len(migratedOwned.PrevSupernodeAccounts, 2)
	s.Require().Equal(legacyAddr.String(), migratedOwned.PrevSupernodeAccounts[0].Account)
	s.Require().Equal(newAddr.String(), migratedOwned.PrevSupernodeAccounts[1].Account)

	_, found = s.app.SupernodeKeeper.QuerySuperNode(s.ctx, oldValAddr)
	s.Require().False(found)
	migratedValidator, found := s.app.SupernodeKeeper.QuerySuperNode(s.ctx, newValAddr)
	s.Require().True(found)
	s.Require().Equal(independentAccount.String(), migratedValidator.SupernodeAccount)
	s.Require().Equal(validatorAssociated.PrevSupernodeAccounts, migratedValidator.PrevSupernodeAccounts)

	byOldOwner, found, err := s.app.SupernodeKeeper.GetSuperNodeByAccount(s.ctx, legacyAddr.String())
	s.Require().NoError(err)
	s.Require().False(found)
	s.Require().Empty(byOldOwner.ValidatorAddress)
	byNewOwner, found, err := s.app.SupernodeKeeper.GetSuperNodeByAccount(s.ctx, newAddr.String())
	s.Require().NoError(err)
	s.Require().True(found)
	s.Require().Equal(accountOwnedVal.String(), byNewOwner.ValidatorAddress)
	byIndependentOwner, found, err := s.app.SupernodeKeeper.GetSuperNodeByAccount(s.ctx, independentAccount.String())
	s.Require().NoError(err)
	s.Require().True(found)
	s.Require().Equal(newValAddr.String(), byIndependentOwner.ValidatorAddress)
}

func (s *MigrationIntegrationSuite) TestMigrateValidatorSupernode_AlternateEncodingSelfOwnedRealStore() {
	legacyAddr := sdk.AccAddress(testAddressBytes("alternate-owner"))
	newAddr := sdk.AccAddress(testAddressBytes("alternate-new"))
	oldValAddr := sdk.ValAddress(legacyAddr)
	newValAddr := sdk.ValAddress(newAddr)
	sn := validMigrationSupernode(oldValAddr, legacyAddr)
	sn.ValidatorAddress = strings.ToUpper(sn.ValidatorAddress)
	sn.SupernodeAccount = strings.ToUpper(sn.SupernodeAccount)
	sn.PrevSupernodeAccounts = []*sntypes.SupernodeAccountHistory{{Account: sn.SupernodeAccount, Height: 7}}
	s.putSupernodePrimary(oldValAddr, sn)
	store := s.ctx.KVStore(s.app.GetKey(sntypes.StoreKey))
	store.Set(append(bytes.Clone(sntypes.SuperNodeByAccountKey), []byte(sn.SupernodeAccount)...), oldValAddr)

	s.Require().NoError(s.keeper.MigrateValidatorSupernode(s.ctx, oldValAddr, newValAddr, legacyAddr, newAddr))

	_, found := s.app.SupernodeKeeper.QuerySuperNode(s.ctx, oldValAddr)
	s.Require().False(found)
	migrated, found := s.app.SupernodeKeeper.QuerySuperNode(s.ctx, newValAddr)
	s.Require().True(found)
	s.Require().Equal(newAddr.String(), migrated.SupernodeAccount)
	s.Require().Len(migrated.PrevSupernodeAccounts, 2)
	s.Require().Equal(sn.SupernodeAccount, migrated.PrevSupernodeAccounts[0].Account)
	s.Require().Equal(newAddr.String(), migrated.PrevSupernodeAccounts[1].Account)

	_, found, err := s.app.SupernodeKeeper.GetSuperNodeByAccount(s.ctx, legacyAddr.String())
	s.Require().NoError(err)
	s.Require().False(found, "legacy owner must not be restored under canonical encoding")
	byNewOwner, found, err := s.app.SupernodeKeeper.GetSuperNodeByAccount(s.ctx, newAddr.String())
	s.Require().NoError(err)
	s.Require().True(found)
	s.Require().Equal(newValAddr.String(), byNewOwner.ValidatorAddress)
}

func (s *MigrationIntegrationSuite) TestMigrationEstimate_ValidatorPrimaryOnlyHasSupernodeParity() {
	s.enableMigration()
	_, legacyAddr := s.createFundedLegacyAccount(sdk.NewCoins(sdk.NewInt64Coin("ulume", 2_000_000)))
	oldValAddr, _ := s.createTestValidator(legacyAddr, sdkmath.NewInt(1_000_000))
	independentAccount := sdk.AccAddress(testAddressBytes("estimate-independent"))
	s.Require().NoError(s.app.SupernodeKeeper.SetSuperNode(s.ctx, validMigrationSupernode(oldValAddr, independentAccount)))

	queryServer := evmigrationkeeper.NewQueryServerImpl(s.keeper)
	estimate, err := queryServer.MigrationEstimate(s.ctx, &evmigrationtypes.QueryMigrationEstimateRequest{
		LegacyAddress: legacyAddr.String(),
	})
	s.Require().NoError(err)
	s.Require().True(estimate.IsValidator)
	s.Require().True(estimate.HasSupernode, "B-only validator primary must be visible to estimate just as it is to execution")

	_, newAddr := createNewEVMAddress(s.T())
	s.Require().NoError(s.keeper.MigrateValidatorSupernode(s.ctx, oldValAddr, sdk.ValAddress(newAddr), legacyAddr, newAddr))
	_, found := s.app.SupernodeKeeper.QuerySuperNode(s.ctx, oldValAddr)
	s.Require().False(found)
	migrated, found := s.app.SupernodeKeeper.QuerySuperNode(s.ctx, sdk.ValAddress(newAddr))
	s.Require().True(found)
	s.Require().Equal(independentAccount.String(), migrated.SupernodeAccount)
}

func testAddressBytes(seed string) []byte {
	out := make([]byte, 20)
	copy(out, []byte(seed))
	return out
}

func TestOwnershipIntegrityHelpersUseTwentyByteAddresses(t *testing.T) {
	require.Len(t, testAddressBytes("short"), 20)
}
