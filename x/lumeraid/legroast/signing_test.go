package legroast_test

import (
	"testing"

	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	lumeraidmocks "github.com/LumeraProtocol/lumera/x/lumeraid/mocks"
	"github.com/LumeraProtocol/lumera/testutil/accounts"
	. "github.com/LumeraProtocol/lumera/x/lumeraid/legroast"
)

// TestGenerateLegRoastKeySeed tests the GenerateLegRoastKeySeed function
func TestGenerateLegRoastKeySeed(t *testing.T) {
	kr := accounts.CreateTestKeyring()
	testAccounts := accounts.SetupTestAccounts(t, kr, []string{"test-account"})

	// Generate a seed for the LegRoast key generation from address
	seed, err := GenerateLegRoastKeySeed(testAccounts[0].Address, kr)
	require.NoError(t, err, "GenerateLegRoastKeySeed should not return an error")
	require.NotNil(t, seed, "Seed should not be nil")
	require.Equal(t, 16, len(seed), "Seed length should be 16")
}

func TestGenerateLegRoastKeySeed_InvalidAddress(t *testing.T) {
	kr := accounts.CreateTestKeyring()

	// Call the function with an invalid address
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

	kr := accounts.CreateTestKeyring()
	testAccounts := accounts.SetupTestAccounts(t, kr, []string{"test-account"})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Sign and Verify
			signature, pubkey, err := Sign(testAccounts[0].Address, kr, []byte(tt.message), tt.algorithm)
			require.NoError(t, err, "Sign should not return an error")
			require.NotNil(t, signature, "Signature should not be nil")
			require.NotNil(t, pubkey, "Public key should not be nil")

			err = Verify([]byte(tt.message), pubkey, signature)
			require.NoError(t, err, "Verify should not return an error")
		})
	}
}

func TestSignInvalidAddress(t *testing.T) {
	kr := accounts.CreateTestKeyring()

	// Call the function with an invalid address
	invalidAddress := "cosmos1invalidaddress"
	_, _, err := Sign(invalidAddress, kr, []byte("test message"), LegendreMiddle)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid Cosmos address")
}

func TestSignKeyRingNotSet(t *testing.T) {
	kr := accounts.CreateTestKeyring()
	testAccounts := accounts.SetupTestAccounts(t, kr, []string{"test-account"})

	// Call the function with a nil keyring
	_, _, err := Sign(testAccounts[0].Address, nil, []byte("test message"), LegendreMiddle)
	require.Error(t, err)
	require.Contains(t, err.Error(), "keyring is not set")

	// remove address from keyring
	err = kr.Delete(testAccounts[0].Name)
	require.NoError(t, err)

	// Call the function with a keyring that does not contain the address
	_, _, err = Sign(testAccounts[0].Address, kr, []byte("test message"), LegendreMiddle)
	require.Error(t, err)
}

func TestSignKeygenFailed(t *testing.T) {
	ctl := gomock.NewController(t)
	defer ctl.Finish()

	lrMock := lumeraidmocks.NewMockLegRoastInterface(ctl)
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

	lrMock := lumeraidmocks.NewMockLegRoastInterface(ctl)
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

	lrMock := lumeraidmocks.NewMockLegRoastInterface(ctl)
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

	lrMock := lumeraidmocks.NewMockLegRoastInterface(ctl)
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
