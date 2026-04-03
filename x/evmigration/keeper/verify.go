package keeper

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"

	"github.com/LumeraProtocol/lumera/x/evmigration/types"
)

const (
	migrationPayloadKindClaim     = "claim"
	migrationPayloadKindValidator = "validator"
)

func migrationPayload(chainID string, evmChainID uint64, kind string, legacyAddr, newAddr sdk.AccAddress) []byte {
	return []byte(fmt.Sprintf("lumera-evm-migration:%s:%d:%s:%s:%s", chainID, evmChainID, kind, legacyAddr.String(), newAddr.String()))
}

// eip191PersonalSignPayload wraps msg in the EIP-191 "personal_sign" envelope.
// Result: "\x19Ethereum Signed Message:\n" || decimal(len(msg)) || msg
//
// Wallets (Keplr/Leap Ethereum provider) compute:
//
//	sign(Keccak256("\x19Ethereum Signed Message:\n" + len(msg) + msg))
//
// Since ethsecp256k1.VerifySignature internally does Keccak256(input),
// passing the prefixed payload matches what the wallet signed.
func eip191PersonalSignPayload(msg []byte) []byte {
	prefix := fmt.Appendf(nil, "\x19Ethereum Signed Message:\n%d", len(msg))
	return append(prefix, msg...)
}

// adr036SignDoc builds the canonical ADR-036 sign doc that Keplr/Leap produce
// when calling signArbitrary(). The JSON fields are alphabetically sorted
// (Amino canonical form) and must be byte-for-byte identical to what the wallet
// signs.
//
// Keplr's Sign(adr036_doc) internally computes SHA256(adr036_doc) before ECDSA
// signing. On the verification side, secp256k1.VerifySignature(msg, sig) also
// internally does SHA256(msg). So we pass the raw doc — not a pre-hash.
func adr036SignDoc(signer string, data []byte) []byte {
	return []byte(fmt.Sprintf(
		`{"account_number":"0","chain_id":"","fee":{"amount":[],"gas":"0"},`+
			`"memo":"","msgs":[{"type":"sign/MsgSignData","value":`+
			`{"data":"%s","signer":"%s"}}],"sequence":"0"}`,
		base64.StdEncoding.EncodeToString(data), signer,
	))
}

// VerifyLegacySignature verifies the legacy-account proof embedded in a
// migration message. Legacy keys use Cosmos secp256k1 signing over SHA-256.
//
// Two signature formats are accepted:
//   - Try 1 (CLI/keyring): Sign(SHA256(payload)) — the SDK's secp256k1.Sign
//     internally does SHA256, so the actual signature is over SHA256(SHA256(payload)).
//     VerifySignature also internally hashes, so we pass SHA256(payload).
//   - Try 2 (Keplr/Leap signArbitrary): Sign(adr036_doc) — Keplr wraps the
//     payload in ADR-036 canonical JSON before signing. We reconstruct the same
//     doc and pass it to VerifySignature (which internally hashes it).
func VerifyLegacySignature(chainID string, evmChainID uint64, kind string, legacyAddr, newAddr sdk.AccAddress, legacyPubKeyBytes, legacySignature []byte) error {
	// Step 1: decode the compressed secp256k1 public key.
	if len(legacyPubKeyBytes) != secp256k1.PubKeySize {
		return types.ErrInvalidLegacyPubKey.Wrapf("expected %d bytes, got %d", secp256k1.PubKeySize, len(legacyPubKeyBytes))
	}
	pubKey := &secp256k1.PubKey{Key: legacyPubKeyBytes}

	// Step 2: derive address and verify it matches legacy_address.
	derivedAddr := sdk.AccAddress(pubKey.Address())
	if !derivedAddr.Equals(legacyAddr) {
		return types.ErrPubKeyAddressMismatch.Wrapf(
			"pubkey derives to %s, expected %s", derivedAddr, legacyAddr,
		)
	}

	payload := migrationPayload(chainID, evmChainID, kind, legacyAddr, newAddr)

	// Try 1: raw SHA256(payload) — CLI / keyring signing path.
	hash := sha256.Sum256(payload)
	if pubKey.VerifySignature(hash[:], legacySignature) {
		return nil
	}

	// Try 2: ADR-036 signArbitrary — Keplr/Leap wallet signing.
	adr036Doc := adr036SignDoc(legacyAddr.String(), payload)
	if pubKey.VerifySignature(adr036Doc, legacySignature) {
		return nil
	}

	return types.ErrInvalidLegacySignature.Wrapf(
		"payload was signed for chain-id %q; verify the --chain-id flag matches the target chain", chainID,
	)
}

func normalizeRecoverySignatures(signature []byte) ([][]byte, error) {
	switch len(signature) {
	case 64:
		candidates := make([][]byte, 0, 4)
		for recoveryID := byte(0); recoveryID < 4; recoveryID++ {
			candidate := append(append([]byte(nil), signature...), recoveryID)
			candidates = append(candidates, candidate)
		}
		return candidates, nil
	case 65:
		candidate := append([]byte(nil), signature...)
		if candidate[64] >= 27 {
			candidate[64] -= 27
		}
		if candidate[64] > 3 {
			return nil, types.ErrInvalidNewSignature.Wrapf("unsupported recovery id %d", signature[64])
		}
		return [][]byte{candidate}, nil
	default:
		return nil, types.ErrInvalidNewSignature.Wrapf("expected 64 or 65 bytes, got %d", len(signature))
	}
}

func recoverDerivedNewAddresses(hash []byte, signature []byte) ([]sdk.AccAddress, error) {
	candidates, err := normalizeRecoverySignatures(signature)
	if err != nil {
		return nil, err
	}

	recovered := make([]sdk.AccAddress, 0, len(candidates))
	var lastErr error
	for _, candidate := range candidates {
		pubKey, recoverErr := ethcrypto.SigToPub(hash, candidate)
		if recoverErr == nil {
			recovered = append(recovered, sdk.AccAddress(ethcrypto.PubkeyToAddress(*pubKey).Bytes()))
			continue
		}
		lastErr = recoverErr
	}

	if len(recovered) > 0 {
		return recovered, nil
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, types.ErrInvalidNewSignature
}

func findMatchingRecoveredAddress(hash []byte, signature []byte, expected sdk.AccAddress) (sdk.AccAddress, bool, error) {
	recovered, err := recoverDerivedNewAddresses(hash, signature)
	if err != nil {
		return nil, false, err
	}

	for _, addr := range recovered {
		if addr.Equals(expected) {
			return addr, true, nil
		}
	}

	return recovered[0], false, nil
}

// VerifyNewSignature verifies the destination-account proof embedded in a
// migration message. The destination address is now authenticated by recovering
// the signer directly from the ECDSA signature instead of requiring a separate
// new_pub_key field in the message.
//
// Two signature formats are accepted:
//   - Try 1 (CLI/keyring): Sign(payload) — the eth key path signs Keccak256(payload).
//   - Try 2 (Keplr/Leap personal_sign): sign(Keccak256("\x19Ethereum Signed Message:\n" + len(payload) + payload)).
func VerifyNewSignature(chainID string, evmChainID uint64, kind string, legacyAddr, newAddr sdk.AccAddress, newSignature []byte) error {
	payload := migrationPayload(chainID, evmChainID, kind, legacyAddr, newAddr)

	chainIDHint := fmt.Sprintf("; if the signing chain-id differs from %q the recovered address will not match", chainID)

	// Try 1: raw payload — CLI / keyring signing path.
	if derivedAddr, ok, err := findMatchingRecoveredAddress(ethcrypto.Keccak256(payload), newSignature, newAddr); err == nil {
		if ok {
			return nil
		}
		if eip191DerivedAddr, eip191OK, eip191Err := findMatchingRecoveredAddress(ethcrypto.Keccak256(eip191PersonalSignPayload(payload)), newSignature, newAddr); eip191Err == nil {
			if eip191OK {
				return nil
			}
			return types.ErrNewPubKeyAddressMismatch.Wrapf(
				"recovered signer derives to %s, expected %s%s", eip191DerivedAddr, newAddr, chainIDHint,
			)
		}
		return types.ErrNewPubKeyAddressMismatch.Wrapf(
			"recovered signer derives to %s, expected %s%s", derivedAddr, newAddr, chainIDHint,
		)
	}

	// Try 2: EIP-191 personal_sign — Keplr/Leap wallet signing.
	if derivedAddr, ok, err := findMatchingRecoveredAddress(ethcrypto.Keccak256(eip191PersonalSignPayload(payload)), newSignature, newAddr); err == nil {
		if ok {
			return nil
		}
		return types.ErrNewPubKeyAddressMismatch.Wrapf(
			"recovered signer derives to %s, expected %s%s", derivedAddr, newAddr, chainIDHint,
		)
	}

	return types.ErrInvalidNewSignature.Wrapf(
		"payload was signed for chain-id %q; verify the --chain-id flag matches the target chain", chainID,
	)
}
