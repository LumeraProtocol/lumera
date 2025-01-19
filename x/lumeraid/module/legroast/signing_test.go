package legroast_test

import (
	"testing"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/types"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/go-bip39"
	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	lumeraidmocks "github.com/LumeraProtocol/lumera/x/lumeraid/mocks"
	. "github.com/LumeraProtocol/lumera/x/lumeraid/module/legroast"
)

func generateMnemonic() (string, error) {
	entropy, err := bip39.NewEntropy(128) // 128 bits for a 12-word mnemonic
	if err != nil {
		return "", err
	}
	mnemonic, err := bip39.NewMnemonic(entropy)
	if err != nil {
		return "", err
	}
	return mnemonic, nil
}

func createTestKeyring() keyring.Keyring {
	// Create a codec using the modern protobuf-based codec
	interfaceRegistry := types.NewInterfaceRegistry()
	protoCodec := codec.NewProtoCodec(interfaceRegistry)
	// Register public and private key implementations
	cryptocodec.RegisterInterfaces(interfaceRegistry)

	// Create an in-memory keyring
	kr := keyring.NewInMemory(protoCodec)

	return kr
}

func addTestAccountToKeyring(kr keyring.Keyring, accountName string) error {
	mnemonic, err := generateMnemonic()
	if err != nil {
		return err
	}
	algoList, _ := kr.SupportedAlgorithms()
	signingAlgo, err := keyring.NewSigningAlgoFromString("secp256k1", algoList)
	if err != nil {
		return err
	}
	hdPath := "m/44'/118'/0'/0/0" // Standard Cosmos HD path

	_, err = kr.NewAccount(accountName, mnemonic, "", hdPath, signingAlgo)
	if err != nil {
		return err
	}

	return nil
}

// TestGenerateLegRoastKeySeed tests the GenerateLegRoastKeySeed function
func TestGenerateLegRoastKeySeed(t *testing.T) {
	// Step 1: Create test keyring
	kr := createTestKeyring()

	// Step 2: Add a test account to the keyring
	accountName := "test-account"
	err := addTestAccountToKeyring(kr, accountName)
	require.NoError(t, err)

	// Step 3: Get the address of the test account
	keyInfo, err := kr.Key(accountName)
	require.NoError(t, err)
	address, err := keyInfo.GetAddress()
	require.NoError(t, err, "keyInfo.GetAddress should not return an error")

	// Step 4: Generate a seed for the LegRoast key generation from address
	seed, err := GenerateLegRoastKeySeed(address.String(), kr)
	require.NoError(t, err, "GenerateLegRoastKeySeed should not return an error")
	require.NotNil(t, seed, "Seed should not be nil")
	require.Equal(t, 16, len(seed), "Seed length should be 16")
}

func TestGenerateLegRoastKeySeed_InvalidAddress(t *testing.T) {
	// Step 1: Create test keyring
	kr := createTestKeyring()

	// Step 2: Call the function with an invalid address
	invalidAddress := "cosmos1invalidaddress"
	_, err := GenerateLegRoastKeySeed(invalidAddress, kr)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid Cosmos address")
}

// test Sign/Verify for different algorithms
func TestSignAndVerify(t *testing.T) {
	tests := []struct {
		name      string
		algorithm LegRoastAlgorithm
		message   string
	}{
		{name: "Sign and Verify - Legendre Fast", algorithm: LegendreFast, message: "test message for Legendre Fast"},
		{name: "Sign and Verify - Legendre Middle", algorithm: LegendreMiddle, message: "test message for  Legendre Middle"},
		{name: "Sign and Verify - Legendre Compact", algorithm: LegendreCompact, message: "test message for Legendre Compact"},
		{name: "Sign and Verify - Power Fast", algorithm: PowerFast, message: "test message for Power Fast"},
		{name: "Sign and Verify - Power Middle", algorithm: PowerMiddle, message: "test message for Power Middle"},
		{name: "Sign and Verify - Power Compact", algorithm: PowerCompact, message: "test message for Power Compact"},
	}

	// Step 1: Create test keyring
	kr := createTestKeyring()

	// Step 2: Add a test account to the keyring
	accountName := "test-account"
	err := addTestAccountToKeyring(kr, accountName)
	require.NoError(t, err)

	// Step 3: Get the address of the test account
	keyInfo, err := kr.Key(accountName)
	require.NoError(t, err)
	address, err := keyInfo.GetAddress()
	require.NoError(t, err, "keyInfo.GetAddress should not return an error")

	// Step 4: Run the tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Sign and Verify
			signature, pubkey, err := Sign(address.String(), kr, []byte(tt.message), tt.algorithm)
			require.NoError(t, err, "Sign should not return an error")
			require.NotNil(t, signature, "Signature should not be nil")
			require.NotNil(t, pubkey, "Public key should not be nil")

			err = Verify([]byte(tt.message), pubkey, signature)
			require.NoError(t, err, "Verify should not return an error")
		})
	}
}

func TestSignInvalidAddress(t *testing.T) {
	kr := createTestKeyring()

	// Call the function with an invalid address
	invalidAddress := "cosmos1invalidaddress"
	_, _, err := Sign(invalidAddress, kr, []byte("test message"), LegendreMiddle)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid Cosmos address")
}

func TestSignKeyRingNotSet(t *testing.T) {
	kr := createTestKeyring()

	// Add a test account to the keyring
	accountName := "test-account"
	err := addTestAccountToKeyring(kr, accountName)
	require.NoError(t, err)

	// Step 3: Get the address of the test account
	keyInfo, err := kr.Key(accountName)
	require.NoError(t, err)
	address, err := keyInfo.GetAddress()
	require.NoError(t, err, "keyInfo.GetAddress should not return an error")

	// Call the function with a nil keyring
	_, _, err = Sign(address.String(), nil, []byte("test message"), LegendreMiddle)
	require.Error(t, err)
	require.Contains(t, err.Error(), "keyring is not set")

	// remove address from keyring
	err = kr.DeleteByAddress(address)
	require.NoError(t, err)

	// Call the function with a keyring that does not contain the address
	_, _, err = Sign(address.String(), kr, []byte("test message"), LegendreMiddle)
	require.Error(t, err)
}

func TestSignKeygenFailed(t *testing.T) {
	ctl := gomock.NewController(t)
	defer ctl.Finish()

	lrMock := lumeraidmocks.NewMockLegRoast(ctl)
	lrMock.EXPECT().Keygen(gomock.Any()).Return(errors.New("keygen failed"))

	// replace NewLegRoast with the mock
	savedNewLegRoast := NewLegRoast
	NewLegRoast = func(algorithm LegRoastAlgorithm) LegRoastInterface {
		_ = algorithm
		return lrMock
	}
	savedGenerateLegRoastKeySeed := GenerateLegRoastKeySeed
	GenerateLegRoastKeySeed = func(address string, kr keyring.Keyring) ([]byte, error) {
		return []byte("seed"), nil
	}
	defer func() {
		NewLegRoast = savedNewLegRoast
		GenerateLegRoastKeySeed = savedGenerateLegRoastKeySeed
	}()

	// Call the function with a mock that returns an error
	_, _, err := Sign("cosmos1address", nil, []byte("test message"), LegendreMiddle)
	require.Error(t, err)
}

func TestSignSignFailed(t *testing.T) {
	ctl := gomock.NewController(t)
	defer ctl.Finish()

	lrMock := lumeraidmocks.NewMockLegRoast(ctl)
	lrMock.EXPECT().Keygen(gomock.Any()).Return(nil)
	lrMock.EXPECT().Sign(gomock.Any()).Return(nil, errors.New("sign failed"))

	// replace NewLegRoast with the mock
	savedNewLegRoast := NewLegRoast
	NewLegRoast = func(algorithm LegRoastAlgorithm) LegRoastInterface {
		_ = algorithm
		return lrMock
	}
	savedGenerateLegRoastKeySeed := GenerateLegRoastKeySeed
	GenerateLegRoastKeySeed = func(address string, kr keyring.Keyring) ([]byte, error) {
		return []byte("seed"), nil
	}
	defer func() {
		NewLegRoast = savedNewLegRoast
		GenerateLegRoastKeySeed = savedGenerateLegRoastKeySeed
	}()

	// Call the function with a mock that returns an error
	_, _, err := Sign("cosmos1address", nil, []byte("test message"), LegendreMiddle)
	require.Error(t, err)
}

func TestVerifyInvalidSigSize(t *testing.T) {
	savedGetAlgorithmBySigSize := GetAlgorithmBySigSize
	GetAlgorithmBySigSize = func(sigSize int) (LegRoastAlgorithm, error) {
		return AlgorithmCount, errors.New("incorrect signature size")
	}
	defer func() {
		GetAlgorithmBySigSize = savedGetAlgorithmBySigSize
	}()

	// Call the function with an invalid signature size
	err := Verify([]byte("test message"), []byte("signature"), []byte("public key"))
	require.Error(t, err)
}

func TestVerifyInvalidPublicKey(t *testing.T) {
	ctl := gomock.NewController(t)
	defer ctl.Finish()

	lrMock := lumeraidmocks.NewMockLegRoast(ctl)
	lrMock.EXPECT().SetPublicKey(gomock.Any()).Return(errors.New("set public key failed"))

	// replace NewLegRoast with the mock
	savedNewLegRoast := NewLegRoast
	NewLegRoast = func(algorithm LegRoastAlgorithm) LegRoastInterface {
		_ = algorithm
		return lrMock
	}

	savedGetAlgorithmBySigSize := GetAlgorithmBySigSize
	GetAlgorithmBySigSize = func(sigSize int) (LegRoastAlgorithm, error) {
		return LegendreFast, nil
	}
	defer func() {
		NewLegRoast = savedNewLegRoast
		GetAlgorithmBySigSize = savedGetAlgorithmBySigSize
	}()

	// Call the function with an invalid public key
	err := Verify([]byte("test message"), []byte("signature"), []byte(""))
	require.Error(t, err)
}

func TestVerifyInvalidSignature(t *testing.T) {
	ctl := gomock.NewController(t)
	defer ctl.Finish()

	lrMock := lumeraidmocks.NewMockLegRoast(ctl)
	lrMock.EXPECT().SetPublicKey(gomock.Any()).Return(nil)
	lrMock.EXPECT().Verify(gomock.Any(), gomock.Any()).Return(errors.New("verify failed"))

	// replace NewLegRoast with the mock
	savedNewLegRoast := NewLegRoast
	NewLegRoast = func(algorithm LegRoastAlgorithm) LegRoastInterface {
		_ = algorithm
		return lrMock
	}

	savedGetAlgorithmBySigSize := GetAlgorithmBySigSize
	GetAlgorithmBySigSize = func(sigSize int) (LegRoastAlgorithm, error) {
		return LegendreFast, nil
	}
	defer func() {
		NewLegRoast = savedNewLegRoast
		GetAlgorithmBySigSize = savedGetAlgorithmBySigSize
	}()

	err := Verify([]byte("test message"), []byte("signature"), []byte("public key"))
	require.Error(t, err)
}
