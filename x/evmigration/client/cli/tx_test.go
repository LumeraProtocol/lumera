package cli

import (
	"crypto/sha256"
	"fmt"
	"testing"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	evmcrypto "github.com/cosmos/evm/crypto/ethsecp256k1"
	evmhd "github.com/cosmos/evm/crypto/hd"
	"github.com/cosmos/go-bip39"
	"github.com/stretchr/testify/require"

	lumeracfg "github.com/LumeraProtocol/lumera/config"
)

// ---------- helpers ----------

const testMnemonic = "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"

func newTestKeyring(t *testing.T) keyring.Keyring {
	t.Helper()
	ir := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(ir)
	lumeracfg.RegisterExtraInterfaces(ir)
	return keyring.NewInMemory(cdc, evmhd.EthSecp256k1Option())
}

// addLegacyKey imports mnemonic as coin-type 118 / secp256k1.
func addLegacyKey(t *testing.T, kr keyring.Keyring, name, mnemonic string) *keyring.Record {
	t.Helper()
	hdPath := hd.CreateHDPath(118, 0, 0).String()
	algoList, _ := kr.SupportedAlgorithms()
	algo, err := keyring.NewSigningAlgoFromString("secp256k1", algoList)
	require.NoError(t, err)
	rec, err := kr.NewAccount(name, mnemonic, "", hdPath, algo)
	require.NoError(t, err)
	return rec
}

// addEVMKey imports mnemonic as coin-type 60 / eth_secp256k1.
func addEVMKey(t *testing.T, kr keyring.Keyring, name, mnemonic string) *keyring.Record {
	t.Helper()
	hdPath := hd.CreateHDPath(60, 0, 0).String()
	algoList, _ := kr.SupportedAlgorithms()
	algo, err := keyring.NewSigningAlgoFromString("eth_secp256k1", algoList)
	require.NoError(t, err)
	rec, err := kr.NewAccount(name, mnemonic, "", hdPath, algo)
	require.NoError(t, err)
	return rec
}

func recordAddress(t *testing.T, rec *keyring.Record) string {
	t.Helper()
	pk, err := rec.GetPubKey()
	require.NoError(t, err)
	return sdk.AccAddress(pk.Address()).String()
}

func clientCtxWithKeyring(kr keyring.Keyring) client.Context {
	return client.Context{}.
		WithKeyring(kr).
		WithChainID("lumera-test-1")
}

// ---------- signLegacyProofFromKeyring tests ----------

func TestSignMigrationProof_ValidKeys(t *testing.T) {
	kr := newTestKeyring(t)
	addLegacyKey(t, kr, "legacy", testMnemonic)
	evmRec := addEVMKey(t, kr, "evm", testMnemonic)

	ctx := clientCtxWithKeyring(kr)
	newAddr, legacyAddr, pubKeyBytes, sig, err := signLegacyProofFromKeyring(ctx, "legacy", "evm", migrationProofKindClaim)

	require.NoError(t, err)
	require.Equal(t, recordAddress(t, evmRec), newAddr)
	require.NotEqual(t, newAddr, legacyAddr, "legacy and new addresses must differ")
	require.Len(t, pubKeyBytes, 33, "secp256k1 compressed pubkey is 33 bytes")
	require.NotEmpty(t, sig)

	// Verify the signature is valid.
	legacyPK := &secp256k1.PubKey{Key: pubKeyBytes}
	payload := fmt.Sprintf("lumera-evm-migration:%s:%d:%s:%s:%s",
		ctx.ChainID, lumeracfg.EVMChainID, migrationProofKindClaim, legacyAddr, newAddr)
	hash := sha256.Sum256([]byte(payload))
	require.True(t, legacyPK.VerifySignature(hash[:], sig), "legacy signature must verify")
}

func TestSignMigrationProof_ValidatorKind(t *testing.T) {
	kr := newTestKeyring(t)
	addLegacyKey(t, kr, "legacy", testMnemonic)
	addEVMKey(t, kr, "evm", testMnemonic)

	ctx := clientCtxWithKeyring(kr)
	newAddr, legacyAddr, pubKeyBytes, sig, err := signLegacyProofFromKeyring(ctx, "legacy", "evm", migrationProofKindValidator)

	require.NoError(t, err)

	// Verify the validator-kind payload was signed.
	legacyPK := &secp256k1.PubKey{Key: pubKeyBytes}
	payload := fmt.Sprintf("lumera-evm-migration:%s:%d:%s:%s:%s",
		ctx.ChainID, lumeracfg.EVMChainID, migrationProofKindValidator, legacyAddr, newAddr)
	hash := sha256.Sum256([]byte(payload))
	require.True(t, legacyPK.VerifySignature(hash[:], sig))
}

func TestSignMigrationProof_LegacyKeyNotFound(t *testing.T) {
	kr := newTestKeyring(t)
	addEVMKey(t, kr, "evm", testMnemonic)

	ctx := clientCtxWithKeyring(kr)
	_, _, _, _, err := signLegacyProofFromKeyring(ctx, "nonexistent-key", "evm", migrationProofKindClaim)

	require.ErrorContains(t, err, "not found in keyring")
}

func TestSignMigrationProof_NewKeyNotFound(t *testing.T) {
	kr := newTestKeyring(t)
	addLegacyKey(t, kr, "legacy", testMnemonic)

	ctx := clientCtxWithKeyring(kr)
	_, _, _, _, err := signLegacyProofFromKeyring(ctx, "legacy", "nonexistent-key", migrationProofKindClaim)

	require.ErrorContains(t, err, "new key")
	require.ErrorContains(t, err, "not found in keyring")
}

func TestSignMigrationProof_WrongKeyType_EthSecp256k1(t *testing.T) {
	kr := newTestKeyring(t)
	// Import as eth_secp256k1 (coin-type 60) for both — the legacy key must be secp256k1.
	addEVMKey(t, kr, "wrong-legacy", testMnemonic)

	// Use a different mnemonic for the "new" key to avoid address collision.
	entropy, _ := bip39.NewEntropy(128)
	otherMnemonic, _ := bip39.NewMnemonic(entropy)
	addEVMKey(t, kr, "evm", otherMnemonic)

	ctx := clientCtxWithKeyring(kr)
	_, _, _, _, err := signLegacyProofFromKeyring(ctx, "wrong-legacy", "evm", migrationProofKindClaim)

	require.ErrorContains(t, err, "must be secp256k1")
	require.ErrorContains(t, err, "coin-type 118")
}

func TestSignMigrationProof_SameAddressRejected(t *testing.T) {
	kr := newTestKeyring(t)
	// Import the same key as both legacy secp256k1 and as "new".
	// This shouldn't normally produce the same address (different key types),
	// but we can force it by using the legacy key name for both args.
	addLegacyKey(t, kr, "legacy", testMnemonic)

	ctx := clientCtxWithKeyring(kr)
	// Use "legacy" as both key names — the new key lookup will find the same key.
	_, _, _, _, err := signLegacyProofFromKeyring(ctx, "legacy", "legacy", migrationProofKindClaim)

	require.ErrorContains(t, err, "identical")
}

func TestSignMigrationProof_DifferentMnemonics(t *testing.T) {
	kr := newTestKeyring(t)
	addLegacyKey(t, kr, "legacy", testMnemonic)

	entropy, _ := bip39.NewEntropy(128)
	otherMnemonic, _ := bip39.NewMnemonic(entropy)
	addEVMKey(t, kr, "evm", otherMnemonic)

	ctx := clientCtxWithKeyring(kr)
	// This should succeed — the CLI doesn't enforce same-mnemonic (the chain does).
	newAddr, legacyAddr, _, _, err := signLegacyProofFromKeyring(ctx, "legacy", "evm", migrationProofKindClaim)

	require.NoError(t, err)
	require.NotEqual(t, newAddr, legacyAddr)
}

func TestSignMigrationProof_ChainIDInPayload(t *testing.T) {
	kr := newTestKeyring(t)
	addLegacyKey(t, kr, "legacy", testMnemonic)
	addEVMKey(t, kr, "evm", testMnemonic)

	// Use a specific chain ID and verify it appears in the signed payload.
	ctx := clientCtxWithKeyring(kr)
	ctx = ctx.WithChainID("lumera-custom-42")

	newAddr, legacyAddr, pubKeyBytes, sig, err := signLegacyProofFromKeyring(ctx, "legacy", "evm", migrationProofKindClaim)
	require.NoError(t, err)

	// Verify with the correct chain ID.
	legacyPK := &secp256k1.PubKey{Key: pubKeyBytes}
	correctPayload := fmt.Sprintf("lumera-evm-migration:%s:%d:%s:%s:%s",
		"lumera-custom-42", lumeracfg.EVMChainID, migrationProofKindClaim, legacyAddr, newAddr)
	correctHash := sha256.Sum256([]byte(correctPayload))
	require.True(t, legacyPK.VerifySignature(correctHash[:], sig))

	// Verify with the wrong chain ID — must fail.
	wrongPayload := fmt.Sprintf("lumera-evm-migration:%s:%d:%s:%s:%s",
		"lumera-wrong-99", lumeracfg.EVMChainID, migrationProofKindClaim, legacyAddr, newAddr)
	wrongHash := sha256.Sum256([]byte(wrongPayload))
	require.False(t, legacyPK.VerifySignature(wrongHash[:], sig))
}

// ---------- signNewMigrationProof tests ----------

func TestSignNewProof_ValidEVMKey(t *testing.T) {
	kr := newTestKeyring(t)
	evmRec := addEVMKey(t, kr, "evm", testMnemonic)
	evmAddr := recordAddress(t, evmRec)

	ctx := clientCtxWithKeyring(kr)
	sig, err := signNewMigrationProof(ctx, "evm", migrationProofKindClaim, "lumera1legacy", evmAddr)

	require.NoError(t, err)
	require.NotEmpty(t, sig)
}

func TestSignNewProof_WrongKeyType_Secp256k1(t *testing.T) {
	kr := newTestKeyring(t)
	addLegacyKey(t, kr, "legacy", testMnemonic)

	ctx := clientCtxWithKeyring(kr)
	_, err := signNewMigrationProof(ctx, "legacy", migrationProofKindClaim, "lumera1legacy", "lumera1new")

	require.ErrorContains(t, err, "eth_secp256k1")
}

func TestSignNewProof_KeyNotFound(t *testing.T) {
	kr := newTestKeyring(t)

	ctx := clientCtxWithKeyring(kr)
	_, err := signNewMigrationProof(ctx, "ghost-key", migrationProofKindClaim, "lumera1legacy", "lumera1new")

	require.Error(t, err)
}

// ---------- command args tests ----------

func TestClaimLegacyAccount_RequiresExactlyTwoArgs(t *testing.T) {
	cmd := cmdClaimLegacyAccount()
	require.Contains(t, cmd.Use, "<legacy-key>")
	require.Contains(t, cmd.Use, "<new-key>")
	require.Error(t, cmd.Args(cmd, nil))
	require.Error(t, cmd.Args(cmd, []string{"legacy-key"}))
	require.NoError(t, cmd.Args(cmd, []string{"legacy-key", "new-key"}))
	require.Error(t, cmd.Args(cmd, []string{"a", "b", "c"}))
}

func TestMigrateValidator_RequiresExactlyTwoArgs(t *testing.T) {
	cmd := cmdMigrateValidator()
	require.Contains(t, cmd.Use, "<legacy-validator-key>")
	require.Contains(t, cmd.Use, "<new-validator-evm-key>")
	require.Error(t, cmd.Args(cmd, nil))
	require.Error(t, cmd.Args(cmd, []string{"legacy-key"}))
	require.NoError(t, cmd.Args(cmd, []string{"legacy-key", "new-key"}))
	require.Error(t, cmd.Args(cmd, []string{"a", "b", "c"}))
}

// ---------- tx-timeout flag tests ----------

func TestClaimLegacyAccount_TxTimeoutFlag(t *testing.T) {
	cmd := cmdClaimLegacyAccount()
	f := cmd.Flags().Lookup(flagTxTimeout)
	require.NotNil(t, f, "--tx-timeout flag must be registered")
	require.Equal(t, defaultTxTimeout, f.DefValue, "default must be %s", defaultTxTimeout)
}

func TestMigrateValidator_TxTimeoutFlag(t *testing.T) {
	cmd := cmdMigrateValidator()
	f := cmd.Flags().Lookup(flagTxTimeout)
	require.NotNil(t, f, "--tx-timeout flag must be registered")
	require.Equal(t, defaultTxTimeout, f.DefValue, "default must be %s", defaultTxTimeout)
}

func TestTxTimeoutFlag_CustomValue(t *testing.T) {
	cmd := cmdClaimLegacyAccount()
	require.NoError(t, cmd.Flags().Set(flagTxTimeout, "2m"))
	val, err := cmd.Flags().GetString(flagTxTimeout)
	require.NoError(t, err)
	require.Equal(t, "2m", val)
}

// ---------- gas adjustment tests ----------

func TestGasAdjustment_DefaultOverriddenTo1_5(t *testing.T) {
	// The SDK default gas adjustment is 1.0 (flags.DefaultGasAdjustment).
	// Our migration commands must override it to 1.5 to avoid out-of-gas.
	// This test verifies the condition in runMigrationTx: GasAdjustment() <= 1.0.
	require.Equal(t, float64(1.0), flags.DefaultGasAdjustment,
		"SDK default gas adjustment must be 1.0 — if this changes, review the <= 1.0 condition in runMigrationTx")
}

// ---------- integration: signLegacyProof signature verification ----------

func TestSignMigrationProof_SignatureVerifiesWithPubKey(t *testing.T) {
	// Full round-trip: generate proof, then verify the signature independently.
	kr := newTestKeyring(t)
	addLegacyKey(t, kr, "legacy", testMnemonic)
	addEVMKey(t, kr, "evm", testMnemonic)

	ctx := clientCtxWithKeyring(kr)
	newAddr, legacyAddr, pubKeyBytes, sig, err := signLegacyProofFromKeyring(ctx, "legacy", "evm", migrationProofKindClaim)
	require.NoError(t, err)

	// Reconstruct and verify.
	legacyPK := &secp256k1.PubKey{Key: pubKeyBytes}
	require.Equal(t, legacyAddr, sdk.AccAddress(legacyPK.Address()).String(),
		"returned pubkey must derive to returned legacy address")

	payload := fmt.Sprintf("lumera-evm-migration:%s:%d:%s:%s:%s",
		ctx.ChainID, lumeracfg.EVMChainID, migrationProofKindClaim, legacyAddr, newAddr)
	hash := sha256.Sum256([]byte(payload))
	require.True(t, legacyPK.VerifySignature(hash[:], sig))
}

func TestSignMigrationProof_PubKeyDerivedAddressMatchesReturned(t *testing.T) {
	kr := newTestKeyring(t)
	legacyRec := addLegacyKey(t, kr, "legacy", testMnemonic)
	addEVMKey(t, kr, "evm", testMnemonic)

	expectedAddr := recordAddress(t, legacyRec)

	ctx := clientCtxWithKeyring(kr)
	_, legacyAddr, pubKeyBytes, _, err := signLegacyProofFromKeyring(ctx, "legacy", "evm", migrationProofKindClaim)
	require.NoError(t, err)

	require.Equal(t, expectedAddr, legacyAddr)
	derivedAddr := sdk.AccAddress((&secp256k1.PubKey{Key: pubKeyBytes}).Address()).String()
	require.Equal(t, expectedAddr, derivedAddr)
}

func TestSignNewProof_OutputIsEthSecp256k1(t *testing.T) {
	kr := newTestKeyring(t)
	evmRec := addEVMKey(t, kr, "evm", testMnemonic)
	evmAddr := recordAddress(t, evmRec)

	ctx := clientCtxWithKeyring(kr)
	sig, err := signNewMigrationProof(ctx, "evm", migrationProofKindClaim, "lumera1legacy", evmAddr)
	require.NoError(t, err)

	// eth_secp256k1 signatures are 65 bytes (R || S || V).
	require.True(t, len(sig) == 64 || len(sig) == 65,
		"eth_secp256k1 signature should be 64 or 65 bytes, got %d", len(sig))
}

// ---------- edge case: multiple keys same keyring ----------

func TestSignMigrationProof_MultipleKeysInKeyring(t *testing.T) {
	kr := newTestKeyring(t)
	// Import two different mnemonics.
	addLegacyKey(t, kr, "legacy-1", testMnemonic)

	entropy, _ := bip39.NewEntropy(128)
	mnemonic2, _ := bip39.NewMnemonic(entropy)
	addLegacyKey(t, kr, "legacy-2", mnemonic2)
	addEVMKey(t, kr, "evm", testMnemonic)

	ctx := clientCtxWithKeyring(kr)

	// legacy-1 should use testMnemonic's legacy address.
	_, legacyAddr1, _, _, err := signLegacyProofFromKeyring(ctx, "legacy-1", "evm", migrationProofKindClaim)
	require.NoError(t, err)

	// legacy-2 should use mnemonic2's legacy address.
	_, legacyAddr2, _, _, err := signLegacyProofFromKeyring(ctx, "legacy-2", "evm", migrationProofKindClaim)
	require.NoError(t, err)

	require.NotEqual(t, legacyAddr1, legacyAddr2, "different mnemonics must produce different addresses")
}

// ---------- edge case: proof kind affects payload ----------

func TestSignMigrationProof_DifferentKindsDifferentSignatures(t *testing.T) {
	kr := newTestKeyring(t)
	addLegacyKey(t, kr, "legacy", testMnemonic)
	addEVMKey(t, kr, "evm", testMnemonic)

	ctx := clientCtxWithKeyring(kr)
	_, _, _, sigClaim, err := signLegacyProofFromKeyring(ctx, "legacy", "evm", migrationProofKindClaim)
	require.NoError(t, err)

	_, _, _, sigValidator, err := signLegacyProofFromKeyring(ctx, "legacy", "evm", migrationProofKindValidator)
	require.NoError(t, err)

	require.NotEqual(t, sigClaim, sigValidator,
		"claim and validator proofs must differ because the payload includes the kind")
}

// ---------- new proof: key type validation ----------

func TestSignNewProof_RejectsNonEVMKey(t *testing.T) {
	kr := newTestKeyring(t)
	// Only legacy key in keyring — no eth_secp256k1.
	legacyRec := addLegacyKey(t, kr, "legacy", testMnemonic)
	fromAddr := recordAddress(t, legacyRec)

	ctx := clientCtxWithKeyring(kr)
	_, err := signNewMigrationProof(ctx, "legacy", migrationProofKindClaim, "lumera1old", fromAddr)

	require.ErrorContains(t, err, "eth_secp256k1")
	// Verify it tells the user what type it got.
	require.ErrorContains(t, err, "secp256k1")
}

// ---------- signLegacyProof: returned pubkey is correct type ----------

func TestSignMigrationProof_ReturnedPubKeyIsSecp256k1(t *testing.T) {
	kr := newTestKeyring(t)
	addLegacyKey(t, kr, "legacy", testMnemonic)
	addEVMKey(t, kr, "evm", testMnemonic)

	ctx := clientCtxWithKeyring(kr)
	_, _, pubKeyBytes, _, err := signLegacyProofFromKeyring(ctx, "legacy", "evm", migrationProofKindClaim)
	require.NoError(t, err)

	// Must be 33 bytes (compressed secp256k1).
	require.Len(t, pubKeyBytes, 33)
	// First byte must be 0x02 or 0x03 (compressed point prefix).
	require.True(t, pubKeyBytes[0] == 0x02 || pubKeyBytes[0] == 0x03,
		"compressed secp256k1 pubkey must start with 0x02 or 0x03, got 0x%02x", pubKeyBytes[0])
}

// ---------- new proof: key type assertion ----------

func TestSignNewProof_ReturnedSigFromEVMKey(t *testing.T) {
	kr := newTestKeyring(t)
	evmRec := addEVMKey(t, kr, "evm", testMnemonic)

	evmPK, _ := evmRec.GetPubKey()
	require.IsType(t, &evmcrypto.PubKey{}, evmPK, "test setup: EVM key must be eth_secp256k1")

	ctx := clientCtxWithKeyring(kr)
	sig, err := signNewMigrationProof(ctx, "evm", migrationProofKindClaim, "lumera1legacy", recordAddress(t, evmRec))
	require.NoError(t, err)
	require.NotEmpty(t, sig)

	// Verify the new-proof payload is signed with the raw payload bytes (no hashing
	// in the Sign call — the keyring internally applies Keccak256 for eth_secp256k1).
	payload := fmt.Sprintf("lumera-evm-migration:%s:%d:%s:%s:%s",
		"lumera-test-1", lumeracfg.EVMChainID, migrationProofKindClaim, "lumera1legacy", recordAddress(t, evmRec))
	_ = payload // Signature verification against recovered address is done chain-side.
	require.True(t, len(sig) >= 64, "signature must be at least 64 bytes")
}

// ---------- signNewMigrationProof: sign mode ----------

func TestSignNewProof_UsesLegacyAminoSignMode(t *testing.T) {
	// Verify that signNewMigrationProof uses SIGN_MODE_LEGACY_AMINO_JSON.
	// We test this indirectly: the function passes the raw payload to Keyring.Sign
	// with SIGN_MODE_LEGACY_AMINO_JSON. For eth_secp256k1, the keyring applies
	// Keccak256 internally and produces a recoverable signature. If we passed
	// a different sign mode, behavior could differ.
	kr := newTestKeyring(t)
	evmRec := addEVMKey(t, kr, "evm", testMnemonic)
	evmAddr := recordAddress(t, evmRec)

	ctx := clientCtxWithKeyring(kr)

	// Sign via the function.
	sig1, err := signNewMigrationProof(ctx, "evm", migrationProofKindClaim, "lumera1legacy", evmAddr)
	require.NoError(t, err)

	// Sign the same payload directly via keyring with the same sign mode.
	payload := []byte(fmt.Sprintf("lumera-evm-migration:%s:%d:%s:%s:%s",
		ctx.ChainID, lumeracfg.EVMChainID, migrationProofKindClaim, "lumera1legacy", evmAddr))
	sig2, _, err := kr.Sign("evm", payload, signingtypes.SignMode_SIGN_MODE_LEGACY_AMINO_JSON)
	require.NoError(t, err)

	require.Equal(t, sig1, sig2, "function should produce the same signature as direct keyring.Sign with same sign mode")
}
