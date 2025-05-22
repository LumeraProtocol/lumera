package securekeyx

import (
	"bytes"
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"testing"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	proto "github.com/cosmos/gogoproto/proto"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/LumeraProtocol/lumera/app"
	"github.com/LumeraProtocol/lumera/testutil/accounts"
	lumeraidtypes "github.com/LumeraProtocol/lumera/x/lumeraid/types"
	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
)


func TestCreateRequest_CurveNotSet(t *testing.T) {
	ke := &SecureKeyExchange{}
	_, _, err := ke.CreateRequest(accounts.TestAddress1)
	assert.Error(t, err)
}

type TestKeyExchangerValidator struct {
}

func (v *TestKeyExchangerValidator) AccountInfoByAddress(ctx context.Context, addr string) (*authtypes.QueryAccountInfoResponse, error) {
	return &authtypes.QueryAccountInfoResponse{
		Info: &authtypes.BaseAccount{
			Address: addr,
		},
	}, nil
}

func (v *TestKeyExchangerValidator) GetSupernodeBySupernodeAddress(ctx context.Context, address string) (*sntypes.SuperNode, error) {
	return &sntypes.SuperNode{
		SupernodeAccount: address,
	}, nil
}

func TestValidateSignature(t *testing.T) {
	kr := accounts.CreateTestKeyring()
	testAccounts := accounts.SetupTestAccounts(t, kr, []string{"test-client", "test-server"})
	clientAccount := testAccounts[0]
	curve := ecdh.P256()
	validator := &TestKeyExchangerValidator{}

	ske, err := NewSecureKeyExchange(kr, clientAccount.Address, Simplenode, curve, validator)
	require.NoError(t, err)

	validData := []byte("valid-data")
	validSignature, _ := ske.signWithKeyring(validData)

	isValid, err := ske.validateSignature(clientAccount.PubKey, validData, validSignature)
	assert.NoError(t, err)
	assert.True(t, isValid)

	invalidSignature := []byte("invalid-signature")
	isValid, err = ske.validateSignature(clientAccount.PubKey, validData, invalidSignature)
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
		Curve:            ske.GetCurveName(),
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

			ske, err := NewSecureKeyExchange(clientKr, testClientAccounts[0].Address, Simplenode, nil, &TestKeyExchangerValidator{})
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
