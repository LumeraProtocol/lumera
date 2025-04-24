package securekeyx

import (
	"bytes"
	"crypto/ecdh"
	"crypto/rand"
	"errors"
	"testing"

	//	"github.com/cometbft/cometbft/abci/server"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
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
	testAccounts := accounts.SetupTestAccounts(t, kr, []string{"test-client", "test-server"})
	localAddress := testAccounts[0].Address
	remoteAddress := testAccounts[1].Address

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

	// Use real keyring to generate a valid test account
	kr := accounts.CreateTestKeyring()
	testAccounts := accounts.SetupTestAccounts(t, kr, []string{"test-client"})
	clientAccount := testAccounts[0]

	keyInfo, err := kr.Key(clientAccount.Name)
	require.NoError(t, err)

	accAddr, err := sdk.AccAddressFromBech32(clientAccount.Address)
	require.NoError(t, err)

	mockKeyring := mocks.NewMockKeyring(ctrl)
	mockKeyring.EXPECT().
		KeyByAddress(accAddr).
		Return(keyInfo, nil).
		Times(2)
	// Simulate signing failure
	mockKeyring.EXPECT().
		SignByAddress(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, nil, errors.New("simulated sign failure"))

	ske, err := NewSecureKeyExchange(mockKeyring, clientAccount.Address, Simplenode, nil)
	require.NoError(t, err)

	_, _, err = ske.CreateRequest(accounts.TestAddress2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "simulated sign failure")
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
	testAccounts := accounts.SetupTestAccounts(t, kr, []string{"test-client"})
	localAddress := testAccounts[0].Address
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
	validAddress, err := sdk.AccAddressFromBech32(accounts.TestAddress1)
	require.NoError(t, err)
	curve := ecdh.P256()

	ske, err := NewSecureKeyExchange(kr, validAddress.String(), Simplenode, curve)
	require.Error(t, err)
	assert.Nil(t, ske)
	assert.Contains(t, err.Error(), "address not found in keyring")
}

func TestNewSecureKeyExchange_DefaultCurve(t *testing.T) {
	kr := accounts.CreateTestKeyring()
	testAccounts := accounts.SetupTestAccounts(t, kr, []string{"test-client"})
	localAddress := testAccounts[0].Address

	ske, err := NewSecureKeyExchange(kr, localAddress, Simplenode, nil)
	require.NoError(t, err)
	assert.Equal(t, "P256", ske.getCurveName())
}

func TestValidateSignature(t *testing.T) {
	kr := accounts.CreateTestKeyring()
	testAccounts := accounts.SetupTestAccounts(t, kr, []string{"test-client"})
	localAddress := testAccounts[0].Address
	curve := ecdh.P256()

	ske, err := NewSecureKeyExchange(kr, localAddress, Simplenode, curve)
	require.NoError(t, err)

	validData := []byte("valid-data")
	validSignature, _ := ske.signWithKeyring(validData)

	isValid, err := ske.validateSignature(testAccounts[0].PubKey, validData, validSignature)
	assert.NoError(t, err)
	assert.True(t, isValid)

	invalidSignature := []byte("invalid-signature")
	isValid, err = ske.validateSignature(testAccounts[0].PubKey, validData, invalidSignature)
	assert.Error(t, err)
	assert.False(t, isValid)
}

func TestValidateSignature_InvalidAddress(t *testing.T) {
	ske := &SecureKeyExchange{}

	validData := []byte("valid-data")
	validSignature := []byte("valid-signature")

	// Test with nil pubkey
	isValid, err := ske.validateSignature(nil, validData, validSignature)
	assert.Error(t, err)
	assert.False(t, isValid)
	assert.Contains(t, err.Error(), "public key is nil")

	// Test with an invalid pubkey that doesn't verify the signature
	var wrongPubKey secp256k1.PubKey
	copy(wrongPubKey.Key[:], bytes.Repeat([]byte{0x01}, 32)) // invalid or wrong key

	isValid, err = ske.validateSignature(&wrongPubKey, validData, validSignature)
	assert.Error(t, err)
	assert.False(t, isValid)
	assert.Contains(t, err.Error(), "invalid signature")
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

type createHandshakeType int

// createHandshake enum
const (
	hsValid createHandshakeType = iota
	hsMissingEphemeralKey
	hsNilAccountPublicKey
	hsInvalidAccountPublicKey
	hsOtherAccountPublicKey
)

func createTestHandshake(t *testing.T, ske *SecureKeyExchange, remoteAddress string, remoteAccount accounts.TestAccount, hsType createHandshakeType) []byte {
	privKey, err := ske.curve.GenerateKey(rand.Reader)
	require.NoError(t, err)
	if hsType != hsMissingEphemeralKey {
		ske.ephemeralKeys[remoteAddress] = privKey
	}

	var accountPubKey []byte
	switch hsType {
	case hsValid, hsMissingEphemeralKey, hsOtherAccountPublicKey:
		accountPubKey, err = ske.codec.MarshalInterface(remoteAccount.PubKey)
		require.NoError(t, err)
	case hsNilAccountPublicKey:
		accountPubKey = nil
	case hsInvalidAccountPublicKey:
		accountPubKey = []byte("invalid-account-public-key")
	}

	handshakeInfo := &lumeraidtypes.HandshakeInfo{
		Address:          remoteAddress,
		PeerType:         int32(Supernode),
		PublicKey:        privKey.PublicKey().Bytes(),
		AccountPublicKey: accountPubKey,
		Curve:            ske.getCurveName(),
	}
	handshakeBytes, err := proto.Marshal(handshakeInfo)
	require.NoError(t, err)
	return handshakeBytes
}

func TestComputeSharedSecret_Failure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	testCases := []struct {
		name    string
		hsType  createHandshakeType
		wantErr string
	}{
		{"MissingEphemeralKey", hsMissingEphemeralKey, "ephemeral private key not found for address"},
		{"NilAccountPubKey", hsNilAccountPublicKey, "account public key is nil"},
		{"InvalidAccountPubKey", hsInvalidAccountPublicKey, "failed to unmarshal remote account's public key"},
		{"OtherAccountPubKey", hsOtherAccountPublicKey, "address mismatch"},
		{"InvalidSignature", hsValid, "signature validation failed"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			clientKr := accounts.CreateTestKeyring()
			testClientAccounts := accounts.SetupTestAccounts(t, clientKr, []string{"test-client"})

			serverKr := accounts.CreateTestKeyring()
			testServerAccounts := accounts.SetupTestAccounts(t, serverKr, []string{"test-server", "test-other-server"})

			ske, err := NewSecureKeyExchange(clientKr, testClientAccounts[0].Address, Simplenode, nil)
			require.NoError(t, err)

			remoteAddress := testServerAccounts[0].Address
			var remoteAccount accounts.TestAccount
			if tc.hsType == hsOtherAccountPublicKey {
				remoteAccount = testServerAccounts[1]
			} else {
				remoteAccount = testServerAccounts[0]
			}

			handshakeBytes := createTestHandshake(t, ske, remoteAddress, remoteAccount, tc.hsType)
			_, err = ske.ComputeSharedSecret(handshakeBytes, []byte("signature"))
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

func TestComputeSharedSecret(t *testing.T) {
	kr := accounts.CreateTestKeyring()
	testAccounts := accounts.SetupTestAccounts(t, kr, []string{"test-client", "test-server"})

	clientAddress := testAccounts[0].Address
	serverAddress := testAccounts[1].Address

	skeLocal, err := NewSecureKeyExchange(kr, clientAddress, Simplenode, nil)
	require.NoError(t, err)

	skeRemote, err := NewSecureKeyExchange(kr, serverAddress, Supernode, nil)
	require.NoError(t, err)

	_, _, err = skeLocal.CreateRequest(serverAddress)
	require.NoError(t, err)

	handshakeRemoteBytes, signatureRemote, err := skeRemote.CreateRequest(clientAddress)
	require.NoError(t, err)

	sharedSecret, err := skeLocal.ComputeSharedSecret(handshakeRemoteBytes, signatureRemote)
	assert.NoError(t, err)
	assert.NotEmpty(t, sharedSecret)
	assert.Len(t, sharedSecret, 32)
}
