package keeper_test

import (
	"fmt"
	"testing"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"

	"github.com/LumeraProtocol/lumera/testutil/cryptotestutils"

	"go.uber.org/mock/gomock"
	"github.com/stretchr/testify/suite"

	sdk "github.com/cosmos/cosmos-sdk/types"

	keepertest "github.com/LumeraProtocol/lumera/testutil/keeper"
	actionkeeper "github.com/LumeraProtocol/lumera/x/action/v1/keeper"
	actiontypes "github.com/LumeraProtocol/lumera/x/action/v1/types"
	supernodemocks "github.com/LumeraProtocol/lumera/x/supernode/v1/mocks"
	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

type KeeperTestSuite struct {
	suite.Suite
	KeeperTestSuiteConfig

	// fields that are reinitialized in SetupTest
	ctx             sdk.Context
	ctrl            *gomock.Controller
	keeper          actionkeeper.Keeper
	mockKeeper      *supernodemocks.MockSupernodeKeeper
	mockQueryServer *supernodemocks.MockQueryServer
}

// KeeperTestSuite is a test suite to test keeper functions
type KeeperTestSuiteConfig struct {
	accountPairs []keepertest.AccountPair
	supernodes   []*sntypes.SuperNode
	badSupernode sntypes.SuperNode

	creatorAddress  sdk.AccAddress
	imposterAddress sdk.AccAddress

	ic                 uint64
	max                uint64
	signatureCascade   string
	signatureSense     string
	signatureSenseBad1 string
	signatureSenseBad2 string
}

// SetupTest sets up the test suite
// Creates test data common for all tests
func (suite *KeeperTestSuite) SetupSuite() {
	var err error

	// Will add four accounts into Mock Keeper
	// 5 SNs and one Creator
	suite.accountPairs = make([]keepertest.AccountPair, 6)

	// Create test supernodes
	snKeys := make([]secp256k1.PrivKey, 3)
	suite.supernodes = make([]*sntypes.SuperNode, 5)
	for i := 0; i < 5; i++ {
		key, address, valAddress := cryptotestutils.SupernodeAddresses()
		suite.supernodes[i] = &sntypes.SuperNode{
			ValidatorAddress: valAddress.String(),
			SupernodeAccount: address.String(),
		}
		if i < 3 { // First 3 are active supernodes
			snKeys[i] = key
		}
		suite.accountPairs[i] = keepertest.AccountPair{Address: address, PubKey: key.PubKey()}
	}
	suite.signatureSense, err = cryptotestutils.CreateSignatureString(snKeys, 50)
	suite.Require().NoError(err)
	suite.signatureSenseBad1, err = cryptotestutils.CreateSignatureString(snKeys, 50)
	suite.Require().NoError(err)
	suite.signatureSenseBad2, err = cryptotestutils.CreateSignatureString(snKeys, 50)
	suite.Require().NoError(err)

	_, badSNAddress, badSNValAddress := cryptotestutils.SupernodeAddresses()
	suite.badSupernode = sntypes.SuperNode{
		ValidatorAddress: badSNValAddress.String(),
		SupernodeAccount: badSNAddress.String(),
	}

	key, address := cryptotestutils.KeyAndAddress()
	pubKey := key.PubKey()
	suite.accountPairs[3] = keepertest.AccountPair{Address: address, PubKey: pubKey}
	suite.signatureCascade, err = cryptotestutils.CreateSignatureString([]secp256k1.PrivKey{key}, 50)
	suite.Require().NoError(err)
	suite.creatorAddress = address

	suite.ic = 20
	suite.max = 50

	suite.imposterAddress = cryptotestutils.AccAddressAcc()
}

func (suite *KeeperTestSuite) SetupTest() {
	suite.ctrl = gomock.NewController(suite.T())
	suite.keeper, suite.ctx = keepertest.ActionKeeperWithAddress(suite.T(), suite.ctrl, suite.accountPairs)

	// Set up the mock supernode keeper and query server
	var ok bool
	suite.mockKeeper, ok = suite.keeper.GetSupernodeKeeper().(*supernodemocks.MockSupernodeKeeper)
	suite.Require().True(ok, "Failed to get MockSupernodeKeeper from Keeper")

	suite.mockQueryServer, ok = suite.keeper.GetSupernodeQueryServer().(*supernodemocks.MockQueryServer)
	suite.Require().True(ok, "Failed to get MockQueryServer from Keeper")

	// Set context with block height for consistent testing
	suite.ctx = suite.ctx.WithBlockHeight(1)
}

// TearDownTest cleans up after each test
func (suite *KeeperTestSuite) TearDownTest() {
	suite.ctrl.Finish()
	suite.ctrl = nil
}

// Helper functions
// Enum to specify metadta filed to miss
type MetadataFieldToMiss int

const (
	MetadataFieldToMissNone MetadataFieldToMiss = iota
	MetadataFieldToMissDataHash
	MetadataFieldToMissFileName
	MetadataFieldToMissIdsIc
	MetadataFieldToMissSignatures
	MetadataFieldToMissIds
	MetadataFieldToMissRqOti
)

func (suite *KeeperTestSuite) setupExpectationsGetAllTopSNs(count int) {
	// Mock GetAllTopSuperNodes - return our test validators
	suite.mockQueryServer.EXPECT().
		GetTopSuperNodesForBlock(
			gomock.AssignableToTypeOf(sdk.Context{}), gomock.AssignableToTypeOf(&sntypes.QueryGetTopSuperNodesForBlockRequest{})).
		Return(
			&sntypes.QueryGetTopSuperNodesForBlockResponse{
				Supernodes: suite.supernodes,
			}, nil).
		Times(count)
}

// prepareCascadeAction creates a test Cascade action
func (suite *KeeperTestSuite) prepareCascadeActionForRegistration(creator string, missing MetadataFieldToMiss) *actiontypes.Action {
	// Create cascade metadata with all required fields
	cascadeMetadata := &actiontypes.CascadeMetadata{
		DataHash:   "hash456",
		FileName:   "test.file",
		RqIdsIc:    suite.ic,
		RqIdsMax:   suite.max,
		Signatures: suite.signatureCascade,
	}
	// Set missing fields based on the provided enum value
	switch missing {
	case MetadataFieldToMissDataHash:
		cascadeMetadata.DataHash = ""
	case MetadataFieldToMissFileName:
		cascadeMetadata.FileName = ""
	case MetadataFieldToMissIdsIc:
		cascadeMetadata.RqIdsIc = 0
	case MetadataFieldToMissSignatures:
		cascadeMetadata.Signatures = ""
	}

	// Marshal metadata to bytes
	metadataBytes, err := suite.keeper.GetCodec().Marshal(cascadeMetadata)
	if err != nil {
		panic(fmt.Sprintf("Failed to marshal cascade metadata: %v", err))
	}

	testPrice := sdk.NewInt64Coin("ulume", 100_000)

	// Create action with embedded metadata
	action := &actiontypes.Action{
		Creator:    creator,
		ActionType: actiontypes.ActionTypeCascade,
		Price:      testPrice.String(),
		Metadata:   metadataBytes,
	}

	return action
}

// prepareSenseAction creates a test Sense action
func (suite *KeeperTestSuite) prepareSenseActionForRegistration(creator string, missing MetadataFieldToMiss) *actiontypes.Action {
	// Create sense metadata with all required fields for validation
	senseMetadata := &actiontypes.SenseMetadata{
		DataHash:             "hash123",
		DdAndFingerprintsIc:  suite.ic,
		DdAndFingerprintsMax: suite.max,
	}

	// Set missing fields based on the provided enum value
	switch missing {
	case MetadataFieldToMissDataHash:
		senseMetadata.DataHash = ""
	case MetadataFieldToMissIdsIc:
		senseMetadata.DdAndFingerprintsIc = 0
	}

	// Marshal metadata to bytes
	metadataBytes, err := suite.keeper.GetCodec().Marshal(senseMetadata)
	if err != nil {
		panic(fmt.Sprintf("Failed to marshal sense metadata: %v", err))
	}

	testPrice := sdk.NewInt64Coin("ulume", 100_000)

	// Create action with embedded metadata
	action := &actiontypes.Action{
		Creator:    creator,
		ActionType: actiontypes.ActionTypeSense,
		Price:      testPrice.String(),
		Metadata:   metadataBytes,
	}

	return action
}

// generateCascadeFinalizationMetadata generates test finalization metadata for Cascade actions
func (suite *KeeperTestSuite) generateCascadeFinalizationMetadata(missing MetadataFieldToMiss) []byte {
	// Generate more complete metadata with required fields

	var validIDs []string
	if missing != MetadataFieldToMissIds {
		for i := suite.ic; i < suite.ic+50; i++ { // 50 is default value for MaxDdAndFingerprints
			id, err := actionkeeper.CreateKademliaID(suite.signatureCascade, i)
			suite.Require().NoError(err)
			validIDs = append(validIDs, id)
		}
	}

	senseMetadata := &actiontypes.CascadeMetadata{
		RqIdsIds: validIDs,
	}

	// Marshal metadata to bytes
	metadataBytes, err := suite.keeper.GetCodec().Marshal(senseMetadata)
	if err != nil {
		panic(fmt.Sprintf("Failed to marshal sense metadata: %v", err))
	}

	return metadataBytes
}

// generateSenseFinalizationMetadata generates test finalization metadata for Sense actions
func (suite *KeeperTestSuite) generateSenseFinalizationMetadata(signatures string, _ MetadataFieldToMiss) []byte {
	// Generate more complete metadata with required fields

	var validIDs []string
	for i := suite.ic; i < suite.ic+50; i++ { // 50 is default value for MaxDdAndFingerprints
		id, err := actionkeeper.CreateKademliaID(signatures, i)
		suite.Require().NoError(err)
		validIDs = append(validIDs, id)
	}

	senseMetadata := &actiontypes.SenseMetadata{
		DdAndFingerprintsIds: validIDs,
		Signatures:           signatures,
	}

	// Marshal metadata to bytes
	metadataBytes, err := suite.keeper.GetCodec().Marshal(senseMetadata)
	if err != nil {
		panic(fmt.Sprintf("Failed to marshal sense metadata: %v", err))
	}

	return metadataBytes
}

func TestKeeperTestSuite(t *testing.T) {
	suite.Run(t, new(KeeperTestSuite))
}
