package evm_test

import (
	"fmt"
	"strings"
	"testing"

	evmante "github.com/cosmos/evm/ante"
	"github.com/cosmos/evm/crypto/ethsecp256k1"
	evmencoding "github.com/cosmos/evm/encoding"
	"github.com/stretchr/testify/require"

	storetypes "cosmossdk.io/store/types"

	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	kmultisig "github.com/cosmos/cosmos-sdk/crypto/keys/multisig"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256r1"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/crypto/types/multisig"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	"github.com/cosmos/cosmos-sdk/x/auth/migrations/legacytx"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"

	lcfg "github.com/LumeraProtocol/lumera/config"
)

// TestSigVerificationGasConsumerMatrix validates signature-gas consumer checks
// used by Lumera's ante chain.
//
// Matrix:
// - Ed25519: rejected (unsupported for tx verification in this path).
// - EthSecp256k1: accepted, charged secp256k1 verify cost.
// - Secp256k1: accepted with SDK configured cost.
// - Secp256r1: rejected in this path.
// - Multisig over eth_secp256k1 keys: accepted, summed costs.
// - Unknown/nil pubkey: rejected.
func TestSigVerificationGasConsumerMatrix(t *testing.T) {
	params := authtypes.DefaultParams()
	msg := []byte{1, 2, 3, 4}

	encodingCfg := evmencoding.MakeConfig(lcfg.EVMChainID)
	cdc := encodingCfg.Amino

	pkSet, sigSet := generateEthPubKeysAndSignatures(5, msg)
	multisigKey := kmultisig.NewLegacyAminoPubKey(2, pkSet)
	multisignature := multisig.NewMultisig(len(pkSet))
	expectedMultisigCost := expectedGasCostByKeys(pkSet)

	// Build a multisignature object from plain signatures so we can exercise
	// recursive gas accounting for nested signature data.
	for i := 0; i < len(pkSet); i++ {
		legacySig := legacytx.StdSignature{PubKey: pkSet[i], Signature: sigSet[i]}
		sigV2, err := legacytx.StdSignatureToSignatureV2(cdc, legacySig)
		require.NoError(t, err)
		require.NoError(t, multisig.AddSignatureV2(multisignature, sigV2, pkSet))
	}

	ethSecpPriv, _ := ethsecp256k1.GenerateKey()
	secpR1Priv, _ := secp256r1.GenPrivKey()

	testCases := []struct {
		name        string
		sigData     signing.SignatureData
		pubKey      cryptotypes.PubKey
		gasConsumed uint64
		expectErr   bool
	}{
		{
			name:        "ed25519 rejected",
			pubKey:      ed25519.GenPrivKey().PubKey(),
			gasConsumed: params.SigVerifyCostED25519,
			expectErr:   true,
		},
		{
			name:        "eth_secp256k1 accepted",
			pubKey:      ethSecpPriv.PubKey(),
			gasConsumed: evmante.Secp256k1VerifyCost,
		},
		{
			name:        "sdk secp256k1 accepted",
			pubKey:      secp256k1.GenPrivKey().PubKey(),
			gasConsumed: params.SigVerifyCostSecp256k1,
		},
		{
			name:        "secp256r1 rejected",
			pubKey:      secpR1Priv.PubKey(),
			gasConsumed: params.SigVerifyCostSecp256r1(),
			expectErr:   true,
		},
		{
			name:        "multisig accepted",
			sigData:     multisignature,
			pubKey:      multisigKey,
			gasConsumed: expectedMultisigCost,
		},
		{
			name:      "unknown key rejected",
			expectErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			meter := storetypes.NewInfiniteGasMeter()
			sig := signing.SignatureV2{
				PubKey:   tc.pubKey,
				Data:     tc.sigData,
				Sequence: 0,
			}

			err := evmante.SigVerificationGasConsumer(meter, sig, params)
			if tc.expectErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.gasConsumed, meter.GasConsumed())
		})
	}
}

func generateEthPubKeysAndSignatures(n int, msg []byte) ([]cryptotypes.PubKey, [][]byte) {
	pubKeys := make([]cryptotypes.PubKey, n)
	signatures := make([][]byte, n)

	for i := 0; i < n; i++ {
		privKey, _ := ethsecp256k1.GenerateKey()
		pubKeys[i] = privKey.PubKey()
		signatures[i], _ = privKey.Sign(msg)
	}

	return pubKeys, signatures
}

func expectedGasCostByKeys(pubKeys []cryptotypes.PubKey) uint64 {
	var cost uint64
	for _, pubKey := range pubKeys {
		pubKeyType := strings.ToLower(fmt.Sprintf("%T", pubKey))
		switch {
		case strings.Contains(pubKeyType, "ed25519"):
			cost += authtypes.DefaultSigVerifyCostED25519
		case strings.Contains(pubKeyType, "ethsecp256k1"):
			cost += evmante.Secp256k1VerifyCost
		default:
			panic("unexpected key type in expectedGasCostByKeys")
		}
	}
	return cost
}
