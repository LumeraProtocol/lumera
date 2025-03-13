package keeper_test

import (
	"fmt"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	"testing"

	"github.com/LumeraProtocol/lumera/testutil/sample"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	sdk "github.com/cosmos/cosmos-sdk/types"

	actionapi "github.com/LumeraProtocol/lumera/api/lumera/action"
	keepertest "github.com/LumeraProtocol/lumera/testutil/keeper"
	"github.com/LumeraProtocol/lumera/x/action/keeper"
	supernodetypes "github.com/LumeraProtocol/lumera/x/supernode/types"
)

// KeeperTestSuite is a test suite to test keeper functions
type KeeperTestSuiteConfig struct {
	ctx        sdk.Context
	keeper     keeper.Keeper
	mockKeeper *keepertest.ActionMockSupernodeKeeper

	supernodes   []*supernodetypes.SuperNode
	badSupernode supernodetypes.SuperNode

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
func (config *KeeperTestSuiteConfig) SetupTestSuite(suite *suite.Suite) {
	var err error

	// Will add four accounts into Mock Keeper
	// 5 SNs and one Creator
	pairs := make([]keepertest.AccountPair, 6)

	// Create test supernodes
	snKeys := make([]ed25519.PrivKey, 3)
	config.supernodes = make([]*supernodetypes.SuperNode, 5)
	for i := 0; i < 5; i++ {
		key, address, valAddress := sample.SupernodeAddresses()
		config.supernodes[i] = &supernodetypes.SuperNode{
			ValidatorAddress: valAddress.String(),
			SupernodeAccount: address.String(),
		}
		if i < 3 { // First 3 are active supernodes
			snKeys[i] = key
		}
		pairs[i] = keepertest.AccountPair{Address: address, PubKey: key.PubKey()}
	}
	config.signatureSense, err = sample.CreateSignatureString(snKeys, 50)
	suite.Require().NoError(err)
	config.signatureSenseBad1, err = sample.CreateSignatureString(snKeys, 50)
	suite.Require().NoError(err)
	config.signatureSenseBad2, err = sample.CreateSignatureString(snKeys, 50)
	suite.Require().NoError(err)

	_, badSNAddress, badSNValAddress := sample.SupernodeAddresses()
	config.badSupernode = supernodetypes.SuperNode{
		ValidatorAddress: badSNValAddress.String(),
		SupernodeAccount: badSNAddress.String(),
	}

	key, address := sample.KeyAndAddress()
	pubKey := key.PubKey()
	pairs[3] = keepertest.AccountPair{Address: address, PubKey: pubKey}
	config.signatureCascade, err = sample.CreateSignatureString([]ed25519.PrivKey{key}, 50)
	suite.Require().NoError(err)
	config.creatorAddress = address

	config.keeper, config.ctx = keepertest.ActionKeeperWithAddress(suite.T(), pairs)

	config.ic = 20
	config.max = 50

	config.imposterAddress = sample.AccAddressAcc()

	// Set context with block height for consistent testing
	config.ctx = config.ctx.WithBlockHeight(1)

	// Get the mock supernode keeper to configure it
	config.mockKeeper = getMockSupernodeKeeper(config.keeper)

	// Configure the mock supernode keeper
	config.configureMockKeeper(suite)
}

// getMockSupernodeKeeper extracts the mock supernode keeper from the action keeper
func getMockSupernodeKeeper(k keeper.Keeper) *keepertest.ActionMockSupernodeKeeper {
	// Use type assertion to access the underlying mock
	supernodeKeeper, ok := k.GetSupernodeKeeper().(*keepertest.ActionMockSupernodeKeeper)
	if !ok {
		panic("Failed to get ActionMockSupernodeKeeper from Keeper")
	}

	// Reset existing mocks
	supernodeKeeper.ExpectedCalls = nil

	return supernodeKeeper
}

// configureMockKeeper sets up mock expectations for the supernode keeper
func (config *KeeperTestSuiteConfig) configureMockKeeper(suite *suite.Suite) {

	// Mock GetTopSuperNodesForBlock - return our test validators
	config.mockKeeper.On("GetTopSuperNodesForBlock", mock.Anything, mock.Anything).Return(
		&supernodetypes.QueryGetTopSuperNodesForBlockResponse{
			Supernodes: config.supernodes,
		}, nil)

	// Mock IsSuperNodeActive - return true for our test validators
	for _, sn := range config.supernodes {
		config.mockKeeper.On("IsSuperNodeActive", mock.Anything, sn.SupernodeAccount).Return(true)
	}

	// Default fallback for any other value
	config.mockKeeper.On("IsSuperNodeActive", mock.Anything, mock.Anything).Return(false)

	// Mock QuerySuperNode - return node for test validators
	for _, sn := range config.supernodes {
		config.mockKeeper.On("QuerySuperNode", mock.Anything, sn.SupernodeAccount).Return(sn, true)
	}

	// Default fallback for any other value
	config.mockKeeper.On("QuerySuperNode", mock.Anything, mock.Anything).Return(
		supernodetypes.SuperNode{}, false)
}

type KeeperTestSuite struct {
	suite.Suite
	KeeperTestSuiteConfig
}

func (suite *KeeperTestSuite) SetupTest() {
	suite.SetupTestSuite(&suite.Suite)
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

// prepareCascadeAction creates a test Cascade action
func (suite *KeeperTestSuite) prepareCascadeActionForRegistration(creator string, missing MetadataFieldToMiss) *actionapi.Action {
	// Create cascade metadata with all required fields
	cascadeMetadata := &actionapi.CascadeMetadata{
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

	// Create action with embedded metadata
	action := &actionapi.Action{
		Creator:    creator,
		ActionType: actionapi.ActionType_ACTION_TYPE_CASCADE,
		Price:      "200ulume",
		Metadata:   metadataBytes,
	}

	return action
}

// prepareSenseAction creates a test Sense action
func (suite *KeeperTestSuite) prepareSenseActionForRegistration(creator string, missing MetadataFieldToMiss) *actionapi.Action {
	// Create sense metadata with all required fields for validation
	senseMetadata := &actionapi.SenseMetadata{
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

	// Create action with embedded metadata
	action := &actionapi.Action{
		Creator:    creator,
		ActionType: actionapi.ActionType_ACTION_TYPE_SENSE,
		Price:      "100000ulume",
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
			id, err := keeper.CreateKademliaID(suite.signatureCascade, i)
			suite.Require().NoError(err)
			validIDs = append(validIDs, id)
		}
	}

	var rqOti []byte
	if missing != MetadataFieldToMissRqOti {
		rqOti = make([]byte, 12)
	}

	senseMetadata := &actionapi.CascadeMetadata{
		RqIdsIds: validIDs,
		RqIdsOti: rqOti,
	}

	// Marshal metadata to bytes
	metadataBytes, err := suite.keeper.GetCodec().Marshal(senseMetadata)
	if err != nil {
		panic(fmt.Sprintf("Failed to marshal sense metadata: %v", err))
	}

	return metadataBytes
}

// generateSenseFinalizationMetadata generates test finalization metadata for Sense actions
func (suite *KeeperTestSuite) generateSenseFinalizationMetadata(signatures string, missing MetadataFieldToMiss) []byte {
	// Generate more complete metadata with required fields

	var validIDs []string
	for i := suite.ic; i < suite.ic+50; i++ { // 50 is default value for MaxDdAndFingerprints
		id, err := keeper.CreateKademliaID(signatures, i)
		suite.Require().NoError(err)
		validIDs = append(validIDs, id)
	}

	senseMetadata := &actionapi.SenseMetadata{
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
