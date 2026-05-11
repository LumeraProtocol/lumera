package keeper_test

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"testing"

	kmultisig "github.com/cosmos/cosmos-sdk/crypto/keys/multisig"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	evmcryptotypes "github.com/cosmos/evm/crypto/ethsecp256k1"
	"github.com/stretchr/testify/require"

	lcfg "github.com/LumeraProtocol/lumera/config"
	"github.com/LumeraProtocol/lumera/x/evmigration/keeper"
	"github.com/LumeraProtocol/lumera/x/evmigration/types"
	"github.com/LumeraProtocol/lumera/x/evmigration/types/sigverify"
)

// testMigrationPayload reconstructs the canonical payload for test signing.
func testMigrationPayload(kind string, legacyAddr, newAddr sdk.AccAddress) []byte {
	return []byte(fmt.Sprintf("lumera-evm-migration:%s:%d:%s:%s:%s", testChainID, lcfg.EVMChainID, kind, legacyAddr.String(), newAddr.String()))
}

// TestVerifyMigrationProof_NewSide_Multisig_Valid2of3 exercises the happy path
// for a new-side multisig: three eth_secp256k1 sub-keys, 2-of-3 threshold,
// sub-signers 0 and 2 sign the canonical payload. VerifyMigrationProof
// called with SubKeyTypeEthSecp256k1 and boundAddr=newAddr must accept.
func TestVerifyMigrationProof_NewSide_Multisig_Valid2of3(t *testing.T) {
	privs := make([]*evmcryptotypes.PrivKey, 3)
	pubs := make([]cryptotypes.PubKey, 3)
	rawPubs := make([][]byte, 3)
	for i := range privs {
		p, err := evmcryptotypes.GenerateKey()
		require.NoError(t, err)
		privs[i] = p
		pubs[i] = p.PubKey()
		rawPubs[i] = pubs[i].Bytes()
	}
	multiPK := kmultisig.NewLegacyAminoPubKey(2, pubs)
	newAddr := sdk.AccAddress(multiPK.Address())
	legacyAddr := sdk.AccAddress(bytes.Repeat([]byte{1}, 20))

	payload := testMigrationPayload(keeperClaimKind, legacyAddr, newAddr)

	// Sub-signers 0 and 2 sign the raw payload (CLI format: eth Sign does Keccak256 internally).
	sig0, err := privs[0].Sign(payload)
	require.NoError(t, err)
	require.Equal(t, 65, len(sig0), "eth sig contract is 65 bytes")
	sig2, err := privs[2].Sign(payload)
	require.NoError(t, err)
	require.Equal(t, 65, len(sig2))

	proof := &types.MigrationProof{Proof: &types.MigrationProof_Multisig{Multisig: &types.MultisigProof{
		Threshold:     2,
		SubPubKeys:    rawPubs,
		SignerIndices: []uint32{0, 2},
		SubSignatures: [][]byte{sig0, sig2},
		SigFormat:     types.SigFormat_SIG_FORMAT_CLI,
	}}}

	err = keeper.VerifyMigrationProof(
		testChainID, lcfg.EVMChainID, keeperClaimKind,
		legacyAddr, newAddr, newAddr,
		proof, sigverify.SubKeyTypeEthSecp256k1,
	)
	require.NoError(t, err)
}

// TestVerifyMigrationProof_NewSide_Multisig_AminoAddressMismatch_OnKeyTypeSwap
// confirms that the amino-encoded LegacyAminoPubKey address embeds the sub-key
// type-URL: a bag of Cosmos secp256k1 sub-keys bound to the address derived
// from a COSMOS multisig cannot masquerade as an eth multisig — because the
// amino bytes (and therefore Address()) differ between the two sub-key types.
// VerifyMigrationProof(SubKeyTypeEthSecp256k1) must reject with
// ErrPubKeyAddressMismatch at the outer multisig-address check, before
// per-sub-sig verification even runs.
func TestVerifyMigrationProof_NewSide_Multisig_AminoAddressMismatch_OnKeyTypeSwap(t *testing.T) {
	priv := secp256k1.GenPrivKey()
	pk := priv.PubKey().(*secp256k1.PubKey)
	// Build the multisig address under the COSMOS interpretation.
	boundAddr := sdk.AccAddress(kmultisig.NewLegacyAminoPubKey(1, []cryptotypes.PubKey{pk}).Address())

	proof := &types.MigrationProof{Proof: &types.MigrationProof_Multisig{Multisig: &types.MultisigProof{
		Threshold:     1,
		SubPubKeys:    [][]byte{pk.Bytes()}, // Cosmos-compressed secp256k1 bag
		SignerIndices: []uint32{0},
		SubSignatures: [][]byte{make([]byte, 65)}, // placeholder — won't be reached
		SigFormat:     types.SigFormat_SIG_FORMAT_CLI,
	}}}

	err := keeper.VerifyMigrationProof(
		testChainID, lcfg.EVMChainID, keeperClaimKind,
		boundAddr, boundAddr, boundAddr,
		proof, sigverify.SubKeyTypeEthSecp256k1, // verifier wraps bytes as eth
	)
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrPubKeyAddressMismatch,
		"expected address-derivation mismatch (amino bytes diverge on sub-key-type-URL), got: %v", err)
}

// TestVerifyMigrationProof_NewSide_Multisig_SubSigInvalid_UnderCosmosKeyBytes
// covers the orthogonal failure mode where the OUTER multisig address DOES
// match (caller deliberately builds the address under the eth interpretation
// to hit the sig-check path), but the sub-signature was produced with a
// Cosmos secp256k1 key and therefore fails under eth_secp256k1 Keccak256
// verification. Precise expectation: ErrInvalidMigrationSignature.
func TestVerifyMigrationProof_NewSide_Multisig_SubSigInvalid_UnderCosmosKeyBytes(t *testing.T) {
	priv := secp256k1.GenPrivKey()
	cosmosPK := priv.PubKey().(*secp256k1.PubKey)

	// Build the bound address under the ETH interpretation (same bytes, eth-typed
	// amino wrapper) so the outer address comparison inside verifyMultisigProofSide
	// matches. This isolates the sub-sig verification failure.
	ethPK := &evmcryptotypes.PubKey{Key: cosmosPK.Bytes()}
	boundAddr := sdk.AccAddress(kmultisig.NewLegacyAminoPubKey(1, []cryptotypes.PubKey{ethPK}).Address())

	// Produce a SHA256-convention Cosmos signature over the canonical payload.
	// Strict ValidateBasic on SideNew requires 65 bytes, but priv.Sign(hash) of a
	// Cosmos secp256k1 key returns 64 bytes. Pad with a single 0x00 V byte so
	// ValidateBasic(SideNew) passes the length check — VerifyEthSecp256k1's
	// direct-verify will still reject the R||S because the signature is over
	// the SHA256 hash, not the Keccak256 of the payload.
	payload := testMigrationPayload(keeperClaimKind, boundAddr, boundAddr)
	hash := sha256.Sum256(payload)
	rawSig, err := priv.Sign(hash[:])
	require.NoError(t, err)
	require.Equal(t, 64, len(rawSig), "Cosmos secp256k1 sig is 64 bytes")
	paddedSig := append(append([]byte(nil), rawSig...), 0x00)
	require.Equal(t, 65, len(paddedSig))

	proof := &types.MigrationProof{Proof: &types.MigrationProof_Multisig{Multisig: &types.MultisigProof{
		Threshold:     1,
		SubPubKeys:    [][]byte{cosmosPK.Bytes()},
		SignerIndices: []uint32{0},
		SubSignatures: [][]byte{paddedSig},
		SigFormat:     types.SigFormat_SIG_FORMAT_CLI,
	}}}

	err = keeper.VerifyMigrationProof(
		testChainID, lcfg.EVMChainID, keeperClaimKind,
		boundAddr, boundAddr, boundAddr,
		proof, sigverify.SubKeyTypeEthSecp256k1,
	)
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrInvalidMigrationSignature)
}
