package lumeraidmocks

import (
	"errors"
	"testing"
	"crypto/ecdh"

	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	_ "github.com/LumeraProtocol/lumera/app"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"	
	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
	. "github.com/LumeraProtocol/lumera/x/lumeraid/securekeyx"
	"github.com/LumeraProtocol/lumera/testutil/accounts"
	mocks "github.com/LumeraProtocol/lumera/testutil/mocks"
)

type SecureKeyExchangeTestSuite struct {
	suite.Suite

	testAccounts []accounts.TestAccount
	kr           keyring.Keyring

	ctrl *gomock.Controller
	mockKeyring *mocks.MockKeyring
	mockValidator *MockKeyExchangerValidator
}

func TestSecureKeyExchangeTestSuite(t *testing.T) {
	suite.Run(t, new(SecureKeyExchangeTestSuite))
}

func (suite *SecureKeyExchangeTestSuite) SetupSuite() {
	// Use real keyring to generate a valid test account
	suite.kr = accounts.CreateTestKeyring()
	suite.testAccounts = accounts.SetupTestAccounts(suite.T(), suite.kr, []string{"test-client", "test-server"})
}

func (suite *SecureKeyExchangeTestSuite) TearDownSuite() {
}

func (suite *SecureKeyExchangeTestSuite) GetClientAddress() string {
	return suite.testAccounts[0].Address
}

func (suite *SecureKeyExchangeTestSuite) GetServerAddress() string {
	return suite.testAccounts[1].Address
}

func (suite *SecureKeyExchangeTestSuite) GetClientAccount() accounts.TestAccount {
	return suite.testAccounts[0]
}

func (suite *SecureKeyExchangeTestSuite) GetServerAccount() accounts.TestAccount {
	return suite.testAccounts[1]
}

func (suite *SecureKeyExchangeTestSuite) mockAccountCheck(peerType PeerType, accNo int, times int) {
	require.Less(suite.T(), accNo, len(suite.testAccounts), "Not enough test accounts")
	var accAddr string
	if accNo == 0 {
		accAddr = suite.GetClientAddress()
	} else {
		accAddr = suite.GetServerAddress()
	}
	if peerType == Supernode {
		suite.mockValidator.EXPECT().
			GetSupernodeBySupernodeAddress(gomock.Any(), accAddr).
			Return(&sntypes.SuperNode{
				SupernodeAccount: accAddr,
			}, nil).
			Times(times)
	} else {
		suite.mockValidator.EXPECT().
			AccountInfoByAddress(gomock.Any(), accAddr).
			Return(&authtypes.QueryAccountInfoResponse{
				Info: &authtypes.BaseAccount{Address: accAddr},
			}, nil).
			Times(times)
	}
}

func (suite *SecureKeyExchangeTestSuite) mockAccountClientCheck(peerType PeerType, times int) {
	suite.mockAccountCheck(peerType, 0, times)
}

func (suite *SecureKeyExchangeTestSuite) mockAccountServerCheck(peerType PeerType, times int) {
	suite.mockAccountCheck(peerType, 1, times)
}

func (suite *SecureKeyExchangeTestSuite) mockKeyByAddress(addrNo int, times int) {
	require.Less(suite.T(), addrNo, len(suite.testAccounts), "Not enough test accounts")
	var account accounts.TestAccount
	if addrNo == 0 {
		account = suite.GetClientAccount()
	} else {
		account = suite.GetServerAccount()
	}
	accAddress, err := sdk.AccAddressFromBech32(account.Address)
	require.NoError(suite.T(), err)

	keyInfo, err := suite.kr.Key(account.Name)
	require.NoError(suite.T(), err)

	suite.mockKeyring.EXPECT().
		KeyByAddress(accAddress).
		Return(keyInfo, nil).
		Times(times)
}

func (suite *SecureKeyExchangeTestSuite) mockClientKeyByAddress(times int) {
	suite.mockKeyByAddress(0, times)
}

func (suite *SecureKeyExchangeTestSuite) mockServerKeyByAddress(times int) {
	suite.mockKeyByAddress(1, times)
}

func (suite *SecureKeyExchangeTestSuite) SetupTest() {
	suite.ctrl = gomock.NewController(suite.T())
	suite.mockKeyring = mocks.NewMockKeyring(suite.ctrl)
	suite.mockValidator = NewMockKeyExchangerValidator(suite.ctrl)
}

func (suite *SecureKeyExchangeTestSuite) TearDownTest() {
	suite.ctrl.Finish()
	suite.mockKeyring = nil
	suite.mockValidator = nil
}

func (suite *SecureKeyExchangeTestSuite) TestCreateRequest() {

	peerType := Simplenode
	curve := ecdh.P256()

	suite.mockAccountClientCheck(peerType, 1)
	ske, err := NewSecureKeyExchange(suite.kr, suite.GetClientAddress(), peerType, curve, suite.mockValidator)
	require.NoError(suite.T(), err)

	handshakeBytes, signature, err := ske.CreateRequest(suite.GetServerAddress())
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), handshakeBytes)
	assert.NotEmpty(suite.T(), signature)
}

func (suite *SecureKeyExchangeTestSuite) TestCreateRequest_SignWithKeyringFails() {
	suite.mockClientKeyByAddress(2)

	// Simulate signing failure
	suite.mockKeyring.EXPECT().
		SignByAddress(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, nil, errors.New("simulated sign failure"))
	suite.mockAccountClientCheck(Supernode, 1)

	ske, err := NewSecureKeyExchange(suite.mockKeyring, suite.GetClientAddress(), Supernode, nil, suite.mockValidator)
	require.NoError(suite.T(), err)

	_, _, err = ske.CreateRequest(accounts.TestAddress2)
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "simulated sign failure")
}

func (suite *SecureKeyExchangeTestSuite) TestGetCurveName() {
	tests := []struct {
		curve    ecdh.Curve
		expected string
	}{
		{ecdh.P256(), "P256"},
		{ecdh.P384(), "P384"},
		{ecdh.P521(), "P521"},
		{nil, "P256"}, // Default curve
	}
	suite.mockClientKeyByAddress(len(tests))
	suite.mockAccountClientCheck(Supernode, len(tests))

	for _, tt := range tests {
		ske, err := NewSecureKeyExchange(suite.mockKeyring, suite.GetClientAddress(), Supernode, tt.curve, suite.mockValidator)
		require.NoError(suite.T(), err)
		assert.Equal(suite.T(), tt.expected, ske.GetCurveName())
	}
}

func (suite *SecureKeyExchangeTestSuite) TestNewSecureKeyExchange() {
	curve := ecdh.P256()
	suite.mockAccountClientCheck(Simplenode, 1)

	ske, err := NewSecureKeyExchange(suite.kr, suite.GetClientAddress(), Simplenode, curve, suite.mockValidator)
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), suite.GetClientAddress(), ske.LocalAddress())
	assert.Equal(suite.T(), Simplenode, ske.PeerType())
	assert.Equal(suite.T(), "P256", ske.GetCurveName())
}

func (suite *SecureKeyExchangeTestSuite) TestNewSecureKeyExchange_InvalidAddress() {
	invalidAddress := "invalid-address"
	curve := ecdh.P256()

	ske, err := NewSecureKeyExchange(suite.kr, invalidAddress, Simplenode, curve, suite.mockValidator)
	require.Error(suite.T(), err)
	assert.Nil(suite.T(), ske)
	assert.Contains(suite.T(), err.Error(), "invalid address")
}

func (suite *SecureKeyExchangeTestSuite) TestComputeSharedSecret_InvalidHandshakeBytes() {
	suite.mockKeyring.EXPECT().
		KeyByAddress(gomock.Any()).
		Return(nil, nil)
	suite.mockValidator.EXPECT().
		AccountInfoByAddress(gomock.Any(), gomock.Any()).
		Return(&authtypes.QueryAccountInfoResponse{
			Info: &authtypes.BaseAccount{Address: accounts.TestAddress1},
		}, nil)

	ske, err := NewSecureKeyExchange(suite.mockKeyring, accounts.TestAddress1, Simplenode, nil, suite.mockValidator)
	require.NoError(suite.T(), err)

	invalidHandshake := []byte("invalid-data")
	_, err = ske.ComputeSharedSecret(invalidHandshake, []byte("signature"))
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "failed to deserialize handshake info")
}

func (suite *SecureKeyExchangeTestSuite) TestNewSecureKeyExchange_MissingKeyInKeyring() {
	// Generate a valid Bech32 formatted address that isn't in the keyring
	validAddress, err := sdk.AccAddressFromBech32(accounts.TestAddress1)
	require.NoError(suite.T(), err)
	curve := ecdh.P256()

	ske, err := NewSecureKeyExchange(suite.kr, validAddress.String(), Simplenode, curve, suite.mockValidator)
	require.Error(suite.T(), err)
	assert.Nil(suite.T(), ske)
	assert.Contains(suite.T(), err.Error(), "address not found in keyring")
}

func (suite *SecureKeyExchangeTestSuite) TestNewSecureKeyExchange_DefaultCurve() {
	suite.mockAccountClientCheck(Simplenode, 1)
	ske, err := NewSecureKeyExchange(suite.kr, suite.GetClientAddress(), Simplenode, nil, suite.mockValidator)
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), "P256", ske.GetCurveName())
}

func (suite *SecureKeyExchangeTestSuite) TestComputeSharedSecret() {
	suite.mockAccountClientCheck(Simplenode, 1)
	suite.mockAccountServerCheck(Supernode, 2)
	clientAddress := suite.GetClientAddress()
	serverAddress := suite.GetServerAddress()

	skeLocal, err := NewSecureKeyExchange(suite.kr, clientAddress, Simplenode, nil, suite.mockValidator)
	require.NoError(suite.T(), err)

	skeRemote, err := NewSecureKeyExchange(suite.kr, serverAddress, Supernode, nil, suite.mockValidator)
	require.NoError(suite.T(), err)

	_, _, err = skeLocal.CreateRequest(serverAddress)
	require.NoError(suite.T(), err)

	handshakeRemoteBytes, signatureRemote, err := skeRemote.CreateRequest(clientAddress)
	require.NoError(suite.T(), err)

	sharedSecret, err := skeLocal.ComputeSharedSecret(handshakeRemoteBytes, signatureRemote)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), sharedSecret)
	assert.Len(suite.T(), sharedSecret, 32)
}
