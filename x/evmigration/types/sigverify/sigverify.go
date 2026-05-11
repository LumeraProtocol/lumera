package sigverify

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	ethsecp256k1 "github.com/cosmos/evm/crypto/ethsecp256k1"

	"github.com/LumeraProtocol/lumera/x/evmigration/types"
)

// SubKeyType identifies which curve/hash-convention the verifier should use
// for a single-key or multisig sub-key verification. Exported here (rather
// than in x/evmigration/keeper) so both the keeper's VerifyMigrationProof
// and the CLI's combine-proof can share a single definition.
type SubKeyType int

const (
	SubKeyTypeCosmosSecp256k1 SubKeyType = iota + 1 // legacy-side sub-keys
	SubKeyTypeEthSecp256k1                          // new-side sub-keys
)

// EIP191PersonalSignPayload wraps msg in the EIP-191 "personal_sign" envelope:
//
//	"\x19Ethereum Signed Message:\n" || decimal(len(msg)) || msg
//
// Wallets (Keplr/Leap Ethereum provider) compute:
//
//	sign(Keccak256("\x19Ethereum Signed Message:\n" + len(msg) + msg))
//
// Since ethsecp256k1.VerifySignature internally does Keccak256(input),
// passing the prefixed payload matches what the wallet signed.
func EIP191PersonalSignPayload(msg []byte) []byte {
	prefix := fmt.Appendf(nil, "\x19Ethereum Signed Message:\n%d", len(msg))
	return append(prefix, msg...)
}

// ADR036SignDoc builds the canonical ADR-036 sign doc for MsgSignData.
// The JSON is alphabetically sorted (Amino canonical form) and must be
// byte-for-byte identical to what Keplr's signArbitrary() produces, because
// secp256k1.VerifySignature hashes the doc and compares to the sig's hash.
func ADR036SignDoc(signer string, data []byte) []byte {
	return []byte(fmt.Sprintf(
		`{"account_number":"0","chain_id":"","fee":{"amount":[],"gas":"0"},`+
			`"memo":"","msgs":[{"type":"sign/MsgSignData","value":`+
			`{"data":"%s","signer":"%s"}}],"sequence":"0"}`,
		base64.StdEncoding.EncodeToString(data), signer,
	))
}

// VerifyCosmosSecp256k1 checks a single Cosmos secp256k1 signature over the
// migration payload. Accepts CLI (raw SHA256) or ADR-036 (canonical JSON)
// envelopes. signerAddr is the bech32 derived from pk — for single-key proofs
// this is bound_addr, for multisig it is the sub-signer's own bech32.
func VerifyCosmosSecp256k1(pk *secp256k1.PubKey, signerAddr sdk.AccAddress, payload, sig []byte, format types.SigFormat) error {
	switch format {
	case types.SigFormat_SIG_FORMAT_CLI:
		hash := sha256.Sum256(payload)
		if pk.VerifySignature(hash[:], sig) {
			return nil
		}
	case types.SigFormat_SIG_FORMAT_ADR036:
		doc := ADR036SignDoc(signerAddr.String(), payload)
		if pk.VerifySignature(doc, sig) {
			return nil
		}
	case types.SigFormat_SIG_FORMAT_EIP191:
		return types.ErrInvalidMigrationProof.Wrap("EIP191 is not valid for Cosmos secp256k1 signatures")
	default:
		return types.ErrInvalidMigrationProof.Wrap("sig_format unspecified")
	}
	return types.ErrInvalidMigrationSignature
}

// VerifyEthSecp256k1 checks a single eth_secp256k1 signature.
//
// Wire contract (design §4.1): eth signatures are strictly 65 bytes (R||S||V).
// This function REJECTS any other length — including the 64-byte form — so a
// caller that skipped ValidateBasic doesn't sneak a malformed sig through.
//
// Verification semantics (design §4.2): direct-verify, NOT ecrecover-and-compare.
//   - Build the format-specific message bytes (CLI = raw payload, ADR-036 =
//     canonical sign-doc, EIP-191 = personal-sign-wrapped payload).
//   - Slice off the V byte (sig[:64]); V is recovery metadata ignored by the
//     verifier and kept on the wire only for Ethereum-native tooling
//     consistency.
//   - Call pk.VerifySignature(msg, sig[:64]) — VerifySignature internally
//     applies Keccak256 and performs ECDSA verify under the supplied pubkey.
//   - The caller (verifySingleKeyProof or verifyMultisigProof) independently
//     asserts that sdk.AccAddress(pk.Address()) == bound_addr, which binds
//     the pubkey to the declared new_address.
func VerifyEthSecp256k1(pk *ethsecp256k1.PubKey, signerAddr sdk.AccAddress, payload, sig []byte, format types.SigFormat) error {
	// Strict wire format: eth signatures are always 65 bytes (R||S||V) per
	// design §4.1. ValidateBasic should have rejected non-65-byte input
	// upstream; this length check is a defense-in-depth belt-and-braces
	// guard for any direct callers that skip ValidateBasic.
	if len(sig) != 65 {
		return types.ErrInvalidMigrationSignature.Wrapf("eth signature must be 65 bytes (R||S||V), got %d", len(sig))
	}
	var msg []byte
	switch format {
	case types.SigFormat_SIG_FORMAT_CLI:
		msg = payload
	case types.SigFormat_SIG_FORMAT_EIP191:
		msg = EIP191PersonalSignPayload(payload)
	case types.SigFormat_SIG_FORMAT_ADR036:
		msg = ADR036SignDoc(signerAddr.String(), payload)
	default:
		return types.ErrInvalidMigrationProof.Wrap("sig_format unspecified")
	}
	// Slice off the V recovery byte — pk.VerifySignature needs R||S only.
	if pk.VerifySignature(msg, sig[:64]) {
		return nil
	}
	return types.ErrInvalidMigrationSignature
}
