package securekeyx

import (
	"crypto/ecdh"
	"crypto/rand"
	"errors"
	"testing"

	"github.com/cosmos/cosmos-sdk/types"
	proto "github.com/cosmos/gogoproto/proto"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/LumeraProtocol/lumera/app"
	"github.com/LumeraProtocol/lumera/testutil/accounts"
	mocks "github.com/LumeraProtocol/lumera/testutil/mocks"
	lumeraidtypes "github.com/LumeraProtocol/lumera/x/lumeraid/types"
)

func TestCreateRequest(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	kr := accounts.CreateTestKeyring()
	
	// Create test accounts
	addresses := accounts.SetupTestAccounts(t, kr, []string{"test-client", "test-server"})
	localAddress := addresses[0]
	remoteAddress := addresses[1]

	peerType := Simplenode
	curve := ecdh.P256()

	ske, err := NewSecureKeyExchange(kr, localAddress, peerType, curve)
	require.NoError(t, err)
	
	handshakeBytes, signature, err := ske.CreateRequest(remoteAddress)
	assert.NoError(t, err)
	assert.NotEmpty(t, handshakeBytes)
	assert.NotEmpty(t, signature)
}

func TestCreateRequest_CurveNotSet(t *testing.T) {
	ke := &SecureKeyExchange{}
	_, _, err := ke.CreateRequest(accounts.TestAddress1)
	assert.Error(t, err)
}

func TestCreateRequest_SignWithKeyringFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockKeyring := mocks.NewMockKeyring(ctrl)
	mockKeyring.EXPECT().KeyByAddress(gomock.Any()).Return(nil, nil)
	
	ske, err := NewSecureKeyExchange(mockKeyring, accounts.TestAddress1, Simplenode, nil)
	require.NoError(t, err)

	// Simulate signing failure
	mockKeyring.EXPECT().SignByAddress(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil, errors.New("failed to sign handshake info"))

	_, _, err = ske.CreateRequest(accounts.TestAddress2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to sign handshake info")
}

func TestGetCurveName(t *testing.T) {
	tests := []struct {
		curve    ecdh.Curve
		expected string
	}{
		{ecdh.P256(), "P256"},
		{ecdh.P384(), "P384"},
		{ecdh.P521(), "P521"},
		{nil, "Unknown"},
	}

	for _, tt := range tests {
		ske := &SecureKeyExchange{curve: tt.curve}
		assert.Equal(t, tt.expected, ske.getCurveName())
	}
}

func TestNewSecureKeyExchange(t *testing.T) {
	kr := accounts.CreateTestKeyring()
	accountNames := []string{"test-client"}
	addresses := accounts.SetupTestAccounts(t, kr, accountNames)
	require.Len(t, addresses, 1)
	localAddress := addresses[0]
	curve := ecdh.P256()

	ske, err := NewSecureKeyExchange(kr, localAddress, Simplenode, curve)
	require.NoError(t, err)
	assert.Equal(t, localAddress, ske.LocalAddress())
	assert.Equal(t, Simplenode, ske.PeerType())
	assert.Equal(t, "P256", ske.getCurveName())
}

func TestNewSecureKeyExchange_InvalidAddress(t *testing.T) {
	kr := accounts.CreateTestKeyring()
	invalidAddress := "invalid-address"
	curve := ecdh.P256()

	ske, err := NewSecureKeyExchange(kr, invalidAddress, Simplenode, curve)
	require.Error(t, err)
	assert.Nil(t, ske)
	assert.Contains(t, err.Error(), "invalid address")
}

func TestNewSecureKeyExchange_MissingKeyInKeyring(t *testing.T) {
	kr := accounts.CreateTestKeyring()
	// Generate a valid Bech32 formatted address that isn't in the keyring
	validAddress, err := types.AccAddressFromBech32(accounts.TestAddress1)
	require.NoError(t, err)
	curve := ecdh.P256()

	ske, err := NewSecureKeyExchange(kr, validAddress.String(), Simplenode, curve)
	require.Error(t, err)
	assert.Nil(t, ske)
	assert.Contains(t, err.Error(), "address not found in keyring")
}

func TestNewSecureKeyExchange_DefaultCurve(t *testing.T) {
	kr := accounts.CreateTestKeyring()
	accountNames := []string{"test-client"}
	addresses := accounts.SetupTestAccounts(t, kr, accountNames)
	require.Len(t, addresses, 1)
	localAddress := addresses[0]

	ske, err := NewSecureKeyExchange(kr, localAddress, Simplenode, nil)
	require.NoError(t, err)
	assert.Equal(t, "P256", ske.getCurveName())
}

func TestValidateSignature(t *testing.T) {
	kr := accounts.CreateTestKeyring()
	accountNames := []string{"test-client"}
	addresses := accounts.SetupTestAccounts(t, kr, accountNames)
	require.Len(t, addresses, 1)
	localAddress := addresses[0]
	curve := ecdh.P256()
	ske, err := NewSecureKeyExchange(kr, localAddress, Simplenode, curve)
	require.NoError(t, err)

	validData := []byte("valid-data")
	validSignature, _ := ske.signWithKeyring(validData)

	isValid, err := ske.validateSignature(localAddress, validData, validSignature)
	assert.NoError(t, err)
	assert.True(t, isValid)

	invalidSignature := []byte("invalid-signature")
	isValid, err = ske.validateSignature(localAddress, validData, invalidSignature)
	assert.Error(t, err)
	assert.False(t, isValid)
}

func TestValidateSignature_InvalidAddress(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockKeyring := mocks.NewMockKeyring(ctrl)
	ske := &SecureKeyExchange{keyring: mockKeyring}

	invalidAddress := "invalid-address"
	validData := []byte("valid-data")
	validSignature := []byte("valid-signature")

	isValid, err := ske.validateSignature(invalidAddress, validData, validSignature)
	assert.Error(t, err)
	assert.False(t, isValid)
	assert.Contains(t, err.Error(), "invalid address")
}

func TestValidateSignature_KeyByAddressFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockKeyring := mocks.NewMockKeyring(ctrl)
	validData := []byte("valid-data")
	validSignature := []byte("valid-signature")

	mockKeyring.EXPECT().KeyByAddress(gomock.Any()).Return(nil, errors.New("key not found"))

	ske := &SecureKeyExchange{keyring: mockKeyring}
	isValid, err := ske.validateSignature(accounts.TestAddress1, validData, validSignature)
	assert.Error(t, err)
	assert.False(t, isValid)
	assert.Contains(t, err.Error(), "address not found in keyring")
}

func TestComputeSharedSecret_CurveNotSet(t *testing.T) {
	ke := &SecureKeyExchange{}
	_, err := ke.ComputeSharedSecret([]byte("handshake"), []byte("signature"))
	assert.Error(t, err)
}

func TestComputeSharedSecret_InvalidHandshakeBytes(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockKeyring := mocks.NewMockKeyring(ctrl)
	mockKeyring.EXPECT().KeyByAddress(gomock.Any()).Return(nil, nil)

	ske, err := NewSecureKeyExchange(mockKeyring, accounts.TestAddress1, Simplenode, nil)
	require.NoError(t, err)

	invalidHandshake := []byte("invalid-data")
	_, err = ske.ComputeSharedSecret(invalidHandshake, []byte("signature"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to deserialize handshake info")
}

func createValidHandshake(t *testing.T, ske *SecureKeyExchange) []byte {
	privKey, err := ske.curve.GenerateKey(rand.Reader)
	require.NoError(t, err)
	ske.ephemeralKeys[ske.LocalAddress()] = privKey

	handshakeInfo := &lumeraidtypes.HandshakeInfo{
		Address:   ske.LocalAddress(),
		PeerType:  int32(ske.PeerType()),
		PublicKey: privKey.PublicKey().Bytes(),
		Curve:     ske.getCurveName(),
	}
	handshakeBytes, err := proto.Marshal(handshakeInfo)
	require.NoError(t, err)
	return handshakeBytes
}

func TestComputeSharedSecret_EphemeralKeyNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockKeyring := mocks.NewMockKeyring(ctrl)
	mockKeyring.EXPECT().KeyByAddress(gomock.Any()).Return(nil, nil)

	ske, err := NewSecureKeyExchange(mockKeyring, accounts.TestAddress1, Simplenode, nil)
	require.NoError(t, err)

	handshakeBytes := createValidHandshake(t, ske)
	delete(ske.ephemeralKeys, ske.LocalAddress())

	_, err = ske.ComputeSharedSecret(handshakeBytes, []byte("signature"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ephemeral private key not found for address")
}

func TestComputeSharedSecret_ValidateSignatureFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockKeyring := mocks.NewMockKeyring(ctrl)
	mockKeyring.EXPECT().KeyByAddress(gomock.Any()).Return(nil, nil)

	ske, err := NewSecureKeyExchange(mockKeyring, accounts.TestAddress1, Simplenode, nil)
	require.NoError(t, err)

	mockKeyring.EXPECT().KeyByAddress(gomock.Any()).Return(nil, errors.New("failed to validate signature"))

	handshakeBytes := createValidHandshake(t, ske)
	_, err = ske.ComputeSharedSecret(handshakeBytes, []byte("signature"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "signature validation failed")
}

func TestComputeSharedSecret(t *testing.T) {
	kr := accounts.CreateTestKeyring()
	accountNames := []string{"test-client", "test-server"}
	addresses := accounts.SetupTestAccounts(t, kr, accountNames)

	skeLocal, err := NewSecureKeyExchange(kr, addresses[0], Simplenode, nil)
	require.NoError(t, err)

	skeRemote, err := NewSecureKeyExchange(kr, addresses[1], Supernode, nil)
	require.NoError(t, err)

	_, _, err = skeLocal.CreateRequest(addresses[1])
	require.NoError(t, err)

	handshakeRemoteBytes, signatureRemote, err := skeRemote.CreateRequest(addresses[0])
	require.NoError(t, err)

	sharedSecret, err := skeLocal.ComputeSharedSecret(handshakeRemoteBytes, signatureRemote)
	assert.NoError(t, err)
	assert.NotEmpty(t, sharedSecret)
	assert.Len(t, sharedSecret, 32)	
}
