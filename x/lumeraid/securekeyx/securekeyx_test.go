package securekeyx

import (
	"crypto/ecdh"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/cosmos/cosmos-sdk/types"

	"github.com/LumeraProtocol/lumera/testutil/accounts"
)

func TestCreateRequest(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	kr := accounts.CreateTestKeyring()
	// Create test accounts
	accountNames := []string{"test-client", "test-server"}
	addresses := accounts.SetupTestAccounts(t, kr, accountNames)
	require.Len(t, addresses, 2)
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
	validAddress, err := types.AccAddressFromBech32("cosmos1qg65a9q6k2sqq7l3ycp428sqqpmqcucgzze299")
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