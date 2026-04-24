package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	kmultisig "github.com/cosmos/cosmos-sdk/crypto/keys/multisig"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	evmcryptotypes "github.com/cosmos/evm/crypto/ethsecp256k1"

	lcfg "github.com/LumeraProtocol/lumera/config"
	"github.com/LumeraProtocol/lumera/x/evmigration/types"
	"github.com/LumeraProtocol/lumera/x/evmigration/types/sigverify"
)

// partialProofVersion is the current on-disk format version for PartialProof.
const partialProofVersion = 2

const (
	flagLegacyAddr    = "legacy"
	flagNewAddr       = "new"
	flagKind          = "kind"
	flagEVMChainID    = "evm-chain-id"
	flagOut           = "out"
	flagLegacyKey     = "legacy-key"
	flagSigFormat     = "sig-format"
	flagNewKey        = "new-key"
	flagNewSubPubKeys = "new-sub-pub-keys"
	flagNewThreshold  = "new-threshold"
)

// PartialProof is a coordination artifact passed between co-signers during
// the multi-step offline signing flow. It is never stored on-chain.
//
// Version 2 schema: each side (legacy and new) has its own SideSpec describing
// whether it is single-key or multisig and what sig format to use. Partial
// signatures are separated into per-side slices.
type PartialProof struct {
	Version                 int                `json:"version"`
	Kind                    string             `json:"kind"` // migrationProofKindClaim | migrationProofKindValidator
	LegacyAddress           string             `json:"legacy_address"`
	NewAddress              string             `json:"new_address"`
	ChainID                 string             `json:"chain_id"`
	EVMChainID              uint64             `json:"evm_chain_id"`
	PayloadHex              string             `json:"payload_hex"`
	Legacy                  *SideSpec          `json:"legacy,omitempty"`
	New                     *SideSpec          `json:"new,omitempty"`
	PartialLegacySignatures []PartialSignature `json:"partial_legacy_signatures"`
	PartialNewSignatures    []PartialSignature `json:"partial_new_signatures"`
}

// SideSpec describes the pubkey configuration for one side of a migration proof.
// For single-key: PubKey is set (base64-encoded 33-byte compressed pubkey); Threshold/SubPubKeys are empty.
// For multisig:   Threshold and SubPubKeys are set; PubKey is empty.
type SideSpec struct {
	// For single-key: base64-encoded 33-byte compressed pubkey.
	// For multisig:   empty.
	PubKey string `json:"pub_key,omitempty"`
	// For multisig: minimum signers required.
	// For single-key: 0 (omitted).
	Threshold uint32 `json:"threshold,omitempty"`
	// For multisig: base64-encoded 33-byte compressed pubkeys, one per signer.
	// For single-key: nil (omitted).
	SubPubKeys []string `json:"sub_pub_keys,omitempty"`
	// Signing envelope. One of: SIG_FORMAT_CLI, SIG_FORMAT_ADR036, SIG_FORMAT_EIP191.
	// EIP-191 is only valid on the new side for single-key proofs.
	SigFormat string `json:"sig_format"`
}

// PartialSignature holds one signer's contribution to one side of the proof.
type PartialSignature struct {
	Index     uint32 `json:"index"`
	Signature string `json:"signature"` // base64-encoded
}

// MarshalIndent writes JSON with 2-space indent for human-readable review.
func (pp *PartialProof) MarshalIndent() ([]byte, error) {
	return json.MarshalIndent(pp, "", "  ")
}

// LoadPartialProof reads a PartialProof JSON file and validates its version
// and contents. Strict decode with DisallowUnknownFields catches both
// forward-drift within the v2 lineage and pre-v2 shapes (v1 files have
// unknown top-level fields like `single`, `multisig`, or `partial_sigs` and
// surface a generic "unknown field" error — v1 backcompat is intentionally
// not supported).
func LoadPartialProof(path string) (*PartialProof, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	var pp PartialProof
	if err := dec.Decode(&pp); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if err := validatePartialProof(&pp); err != nil {
		return nil, err
	}
	return &pp, nil
}

// SavePartialProof writes a PartialProof to disk with 0600 mode.
func SavePartialProof(path string, pp *PartialProof) error {
	b, err := pp.MarshalIndent()
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}

// ParseSigFormat converts the JSON string to a proto enum.
func ParseSigFormat(s string) (types.SigFormat, error) {
	switch s {
	case "SIG_FORMAT_CLI":
		return types.SigFormat_SIG_FORMAT_CLI, nil
	case "SIG_FORMAT_ADR036":
		return types.SigFormat_SIG_FORMAT_ADR036, nil
	case "SIG_FORMAT_EIP191":
		return types.SigFormat_SIG_FORMAT_EIP191, nil
	default:
		return types.SigFormat_SIG_FORMAT_UNSPECIFIED, fmt.Errorf("unknown sig_format %q", s)
	}
}

// SigFormatString is the inverse of ParseSigFormat.
func SigFormatString(f types.SigFormat) string {
	switch f {
	case types.SigFormat_SIG_FORMAT_CLI:
		return "SIG_FORMAT_CLI"
	case types.SigFormat_SIG_FORMAT_ADR036:
		return "SIG_FORMAT_ADR036"
	case types.SigFormat_SIG_FORMAT_EIP191:
		return "SIG_FORMAT_EIP191"
	default:
		return "SIG_FORMAT_UNSPECIFIED"
	}
}

// ComputePayload builds the canonical migration payload bytes. Exported for tests.
func ComputePayload(chainID string, evmChainID uint64, kind, legacyAddr, newAddr string) string {
	return fmt.Sprintf("lumera-evm-migration:%s:%d:%s:%s:%s", chainID, evmChainID, kind, legacyAddr, newAddr)
}

func canonicalPayloadBytes(pp *PartialProof) []byte {
	return []byte(ComputePayload(pp.ChainID, pp.EVMChainID, pp.Kind, pp.LegacyAddress, pp.NewAddress))
}

func validatePartialProof(pp *PartialProof) error {
	if pp.Version != partialProofVersion {
		return fmt.Errorf("unsupported partial_proof version %d (expected %d)", pp.Version, partialProofVersion)
	}
	if pp.Kind != migrationProofKindClaim && pp.Kind != migrationProofKindValidator {
		return fmt.Errorf("partial proof has invalid kind %q (expected %q or %q)",
			pp.Kind, migrationProofKindClaim, migrationProofKindValidator)
	}
	if pp.Legacy == nil {
		return fmt.Errorf("partial proof missing 'legacy' side spec")
	}
	if pp.New == nil {
		return fmt.Errorf("partial proof missing 'new' side spec")
	}
	if err := validateSideSpec("legacy", pp.Legacy); err != nil {
		return err
	}
	if err := validateSideSpec("new", pp.New); err != nil {
		return err
	}
	payloadBytes, err := hex.DecodeString(pp.PayloadHex)
	if err != nil {
		return fmt.Errorf("payload_hex: %w", err)
	}
	if !bytes.Equal(payloadBytes, canonicalPayloadBytes(pp)) {
		return fmt.Errorf("payload_hex does not match chain_id/kind/legacy_address/new_address fields")
	}
	return nil
}

// validateSideSpec enforces: single XOR multisig; threshold bounds; sig_format valid;
// EIP-191 scoping (legacy side rejects, multisig rejects).
func validateSideSpec(label string, s *SideSpec) error {
	isSingle := s.PubKey != ""
	isMulti := s.Threshold > 0 || len(s.SubPubKeys) > 0
	switch {
	case !isSingle && !isMulti:
		return fmt.Errorf("%s side: neither pub_key nor sub_pub_keys set", label)
	case isSingle && isMulti:
		return fmt.Errorf("%s side: both single-key (pub_key) and multisig (threshold/sub_pub_keys) fields are set", label)
	case isMulti && s.Threshold == 0:
		return fmt.Errorf("%s side: multisig has threshold=0", label)
	case isMulti && int(s.Threshold) > len(s.SubPubKeys):
		return fmt.Errorf("%s side: threshold=%d exceeds sub_pub_keys count=%d", label, s.Threshold, len(s.SubPubKeys))
	}
	if s.SigFormat == "" {
		return fmt.Errorf("%s side: sig_format empty", label)
	}
	parsed, err := ParseSigFormat(s.SigFormat)
	if err != nil {
		return fmt.Errorf("%s side: sig_format %q: %w", label, s.SigFormat, err)
	}
	if parsed == types.SigFormat_SIG_FORMAT_EIP191 {
		if label == "legacy" {
			return fmt.Errorf("%s side: SIG_FORMAT_EIP191 is not valid on the legacy side", label)
		}
		if isMulti {
			return fmt.Errorf("%s side: SIG_FORMAT_EIP191 is not valid for multisig proofs", label)
		}
	}
	return nil
}

// decodeBase64 wraps base64.StdEncoding.DecodeString with a field-aware error.
func decodeBase64(field, in string) ([]byte, error) {
	out, err := base64.StdEncoding.DecodeString(in)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", field, err)
	}
	return out, nil
}

// buildProofFromPartial assembles a MigrationProof from a partial-file side spec
// and the merged partial signatures for that side. Verifies every merged partial
// cryptographically (drops invalid with warnings), selects K valid partials with
// lowest ascending indices, assembles the proof.
//
// For single-key sides (side.PubKey != ""): expects exactly one partial at index 0.
// For multisig sides (side.Threshold > 0): requires at least K valid partials.
func buildProofFromPartial(
	side *SideSpec, sigs []PartialSignature, payload []byte,
	keyType sigverify.SubKeyType, sideLabel string, stderr io.Writer,
) (types.MigrationProof, error) {
	format, err := ParseSigFormat(side.SigFormat)
	if err != nil {
		return types.MigrationProof{}, fmt.Errorf("%s side sig_format %q: %w", sideLabel, side.SigFormat, err)
	}

	// Single-key side: strict exactly-one-at-index-0.
	if side.PubKey != "" {
		switch {
		case len(sigs) == 0:
			return types.MigrationProof{}, fmt.Errorf("%s side has no partial signature (single-key side expects exactly one at index 0)", sideLabel)
		case len(sigs) > 1:
			return types.MigrationProof{}, fmt.Errorf("%s side has %d partial signatures; single-key side expects exactly one at index 0", sideLabel, len(sigs))
		case sigs[0].Index != 0:
			return types.MigrationProof{}, fmt.Errorf("%s side partial signature has index=%d; single-key side expects index=0", sideLabel, sigs[0].Index)
		}
		pkBytes, err := decodeBase64(sideLabel+".pub_key", side.PubKey)
		if err != nil {
			return types.MigrationProof{}, err
		}
		if len(pkBytes) != secp256k1.PubKeySize {
			return types.MigrationProof{}, fmt.Errorf("%s side single-key pub_key: expected %d bytes, got %d", sideLabel, secp256k1.PubKeySize, len(pkBytes))
		}
		sigBytes, err := decodeBase64(fmt.Sprintf("%s.partial_signatures[0].signature", sideLabel), sigs[0].Signature)
		if err != nil {
			return types.MigrationProof{}, err
		}
		if err := verifyOne(keyType, pkBytes, payload, sigBytes, format); err != nil {
			return types.MigrationProof{}, fmt.Errorf("%s side single-key partial signature invalid: %w", sideLabel, err)
		}
		return types.MigrationProof{Proof: &types.MigrationProof_Single{Single: &types.SingleKeyProof{
			PubKey: pkBytes, Signature: sigBytes, SigFormat: format,
		}}}, nil
	}

	// Multisig: verify every merged partial, drop invalid, select K valid ascending.
	subPubs := make([][]byte, len(side.SubPubKeys))
	for i, k := range side.SubPubKeys {
		raw, err := decodeBase64(fmt.Sprintf("%s.sub_pub_keys[%d]", sideLabel, i), k)
		if err != nil {
			return types.MigrationProof{}, err
		}
		if len(raw) != secp256k1.PubKeySize {
			return types.MigrationProof{}, fmt.Errorf("%s side sub_pub_keys[%d]: expected %d bytes, got %d", sideLabel, i, secp256k1.PubKeySize, len(raw))
		}
		subPubs[i] = raw
	}

	// Sort merged sigs by index for canonical ordering.
	sort.Slice(sigs, func(i, j int) bool { return sigs[i].Index < sigs[j].Index })

	validIdxs := make([]uint32, 0, len(sigs))
	validSigs := make([][]byte, 0, len(sigs))
	for _, ps := range sigs {
		// Dedupe: strictly ascending unique indices are required by
		// MultisigProof.validateBasic. If a partial file happens to contain
		// two entries at the same index (e.g. a co-signer re-ran sign-proof
		// within one file), keep the first valid one and warn on subsequent.
		if len(validIdxs) > 0 && validIdxs[len(validIdxs)-1] == ps.Index {
			_, _ = fmt.Fprintf(stderr, "WARN %s side: dropping duplicate partial at index %d (already accepted)\n", sideLabel, ps.Index)
			continue
		}
		if int(ps.Index) >= len(subPubs) {
			_, _ = fmt.Fprintf(stderr, "WARN %s side: dropping partial at index %d (out of range for N=%d)\n", sideLabel, ps.Index, len(subPubs))
			continue
		}
		sigBytes, err := decodeBase64(fmt.Sprintf("%s.partial_signatures[index=%d].signature", sideLabel, ps.Index), ps.Signature)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "WARN %s side: dropping partial at index %d (base64 decode error): %s\n", sideLabel, ps.Index, err)
			continue
		}
		if err := verifyOne(keyType, subPubs[ps.Index], payload, sigBytes, format); err != nil {
			_, _ = fmt.Fprintf(stderr, "WARN %s side: dropping partial at index %d: %s\n", sideLabel, ps.Index, err)
			continue
		}
		validIdxs = append(validIdxs, ps.Index)
		validSigs = append(validSigs, sigBytes)
	}

	if uint32(len(validIdxs)) < side.Threshold {
		return types.MigrationProof{}, fmt.Errorf("need %d valid partial signatures on %s side, have %d",
			side.Threshold, sideLabel, len(validIdxs))
	}

	// Select the first K valid ones (lowest indices, already ascending).
	validIdxs = validIdxs[:int(side.Threshold)]
	validSigs = validSigs[:int(side.Threshold)]

	return types.MigrationProof{Proof: &types.MigrationProof_Multisig{Multisig: &types.MultisigProof{
		Threshold: side.Threshold, SubPubKeys: subPubs,
		SignerIndices: validIdxs, SubSignatures: validSigs, SigFormat: format,
	}}}, nil
}

// verifyOne dispatches to the right sigverify helper based on keyType.
func verifyOne(keyType sigverify.SubKeyType, pubKeyBytes, payload, sig []byte, format types.SigFormat) error {
	switch keyType {
	case sigverify.SubKeyTypeCosmosSecp256k1:
		pk := &secp256k1.PubKey{Key: pubKeyBytes}
		return sigverify.VerifyCosmosSecp256k1(pk, sdk.AccAddress(pk.Address()), payload, sig, format)
	case sigverify.SubKeyTypeEthSecp256k1:
		pk := &evmcryptotypes.PubKey{Key: pubKeyBytes}
		return sigverify.VerifyEthSecp256k1(pk, sdk.AccAddress(pk.Address()), payload, sig, format)
	default:
		return fmt.Errorf("unknown sub-key type")
	}
}

// AssertPartialProofsConsistent verifies two PartialProof files agree on
// every field that would change the assembled tx identity. Exported for testing.
func AssertPartialProofsConsistent(a, b *PartialProof) error {
	if a.Version != b.Version {
		return fmt.Errorf("version differs: %d vs %d", a.Version, b.Version)
	}
	if a.Kind != b.Kind {
		return fmt.Errorf("kind differs: %q vs %q", a.Kind, b.Kind)
	}
	if a.ChainID != b.ChainID {
		return fmt.Errorf("chain_id differs: %q vs %q", a.ChainID, b.ChainID)
	}
	if a.EVMChainID != b.EVMChainID {
		return fmt.Errorf("evm_chain_id differs: %d vs %d", a.EVMChainID, b.EVMChainID)
	}
	if a.LegacyAddress != b.LegacyAddress {
		return fmt.Errorf("legacy_address differs: %q vs %q", a.LegacyAddress, b.LegacyAddress)
	}
	if a.NewAddress != b.NewAddress {
		return fmt.Errorf("new_address differs: %q vs %q", a.NewAddress, b.NewAddress)
	}
	if a.PayloadHex != b.PayloadHex {
		return fmt.Errorf("payload_hex differs (chain_id/kind/legacy_address/new_address mismatch between files)")
	}
	if err := assertSideSpecsEqual("legacy", a.Legacy, b.Legacy); err != nil {
		return err
	}
	if err := assertSideSpecsEqual("new", a.New, b.New); err != nil {
		return err
	}
	return nil
}

func assertSideSpecsEqual(label string, a, b *SideSpec) error {
	if (a == nil) != (b == nil) {
		return fmt.Errorf("%s side spec presence differs between partial files", label)
	}
	if a == nil {
		return nil
	}
	if a.PubKey != b.PubKey {
		return fmt.Errorf("%s side pub_key differs", label)
	}
	if a.Threshold != b.Threshold {
		return fmt.Errorf("%s side threshold differs: %d vs %d", label, a.Threshold, b.Threshold)
	}
	if !slicesEqualString(a.SubPubKeys, b.SubPubKeys) {
		return fmt.Errorf("%s side sub_pub_keys differ", label)
	}
	if a.SigFormat != b.SigFormat {
		return fmt.Errorf("%s side sig_format differs: %q vs %q", label, a.SigFormat, b.SigFormat)
	}
	return nil
}

func slicesEqualString(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// hexEncode encodes payload bytes to hex for PartialProof.PayloadHex.
func hexEncode(b []byte) string { return hex.EncodeToString(b) }

// fetchAccount queries the on-chain account and returns its pubkey (may be nil).
func fetchAccount(clientCtx client.Context, addr sdk.AccAddress) (cryptotypes.PubKey, error) {
	accRetriever := authtypes.AccountRetriever{}
	acc, err := accRetriever.GetAccount(clientCtx, addr)
	if err != nil {
		return nil, fmt.Errorf("query account %s: %w", addr, err)
	}
	return acc.GetPubKey(), nil
}

// keep unused-import guard
var _ = signingtypes.SignMode_SIGN_MODE_UNSPECIFIED

// resolveEthSubKey accepts either a keyring key-name or a base64-encoded
// 33-byte compressed eth_secp256k1 pubkey and returns the raw pubkey bytes.
// Errors if the spec resolves to a non-ethsecp256k1 key.
func resolveEthSubKey(clientCtx client.Context, spec string) ([]byte, error) {
	if rec, err := clientCtx.Keyring.Key(spec); err == nil {
		pk, err := rec.GetPubKey()
		if err != nil {
			return nil, fmt.Errorf("cannot get pubkey for key %q: %w", spec, err)
		}
		ethPK, ok := pk.(*evmcryptotypes.PubKey)
		if !ok {
			return nil, fmt.Errorf("key %q is %T, expected eth_secp256k1", spec, pk)
		}
		return ethPK.Key, nil
	}
	raw, err := base64.StdEncoding.DecodeString(spec)
	if err != nil {
		return nil, fmt.Errorf("%q is neither a keyring key nor a base64-encoded pubkey: %w", spec, err)
	}
	if len(raw) != 33 {
		return nil, fmt.Errorf("base64 pubkey %q decodes to %d bytes, expected 33", spec, len(raw))
	}
	return raw, nil
}

// buildLegacySideSpec builds the legacy SideSpec from the on-chain pubkey, handling
// four cases: on-chain secp256k1 / on-chain multisig / nil+--legacy-key / nil without.
func buildLegacySideSpec(clientCtx client.Context, accPubKey cryptotypes.PubKey, legacyKeyName, sigFmt string, legacyAddr sdk.AccAddress) (*SideSpec, error) {
	switch pk := accPubKey.(type) {
	case *secp256k1.PubKey:
		if legacyKeyName != "" {
			rec, err := clientCtx.Keyring.Key(legacyKeyName)
			if err != nil {
				return nil, fmt.Errorf("--legacy-key %q not found: %w", legacyKeyName, err)
			}
			kp, err := rec.GetPubKey()
			if err != nil {
				return nil, err
			}
			if !bytes.Equal(kp.Bytes(), pk.Bytes()) {
				return nil, fmt.Errorf("--legacy-key pubkey does not match on-chain pubkey for %s", legacyAddr)
			}
		}
		return &SideSpec{
			PubKey:    base64.StdEncoding.EncodeToString(pk.Bytes()),
			SigFormat: sigFmt,
		}, nil

	case *kmultisig.LegacyAminoPubKey:
		if legacyKeyName != "" {
			return nil, fmt.Errorf("--legacy-key is not applicable to multisig accounts; co-signers sign via sign-proof")
		}
		subs := pk.GetPubKeys()
		subBytes := make([]string, len(subs))
		for i, sub := range subs {
			cpk, ok := sub.(*secp256k1.PubKey)
			if !ok {
				return nil, fmt.Errorf("legacy multisig sub-key %d is %T, expected Cosmos secp256k1", i, sub)
			}
			subBytes[i] = base64.StdEncoding.EncodeToString(cpk.Bytes())
		}
		return &SideSpec{
			Threshold:  uint32(pk.Threshold),
			SubPubKeys: subBytes,
			SigFormat:  sigFmt,
		}, nil

	case nil:
		if legacyKeyName == "" {
			return nil, fmt.Errorf(
				"account at %s has no on-chain pubkey record; pass --legacy-key to seed the pubkey from your keyring (single-sig only), or for a multisig address submit a 1-ulume self-send first",
				legacyAddr,
			)
		}
		rec, err := clientCtx.Keyring.Key(legacyKeyName)
		if err != nil {
			return nil, fmt.Errorf("--legacy-key %q not found: %w", legacyKeyName, err)
		}
		kp, err := rec.GetPubKey()
		if err != nil {
			return nil, err
		}
		cpk, ok := kp.(*secp256k1.PubKey)
		if !ok {
			return nil, fmt.Errorf("--legacy-key is %T, expected Cosmos secp256k1 (eth keys belong on --new-key)", kp)
		}
		derivedAddr := sdk.AccAddress(cpk.Address())
		if !derivedAddr.Equals(legacyAddr) {
			return nil, fmt.Errorf("--legacy-key derives to %s, not the requested --legacy %s", derivedAddr, legacyAddr)
		}
		return &SideSpec{
			PubKey:    base64.StdEncoding.EncodeToString(cpk.Bytes()),
			SigFormat: sigFmt,
		}, nil

	default:
		return nil, fmt.Errorf("legacy account has unsupported pubkey type %T (expected Cosmos secp256k1 or LegacyAminoPubKey)", pk)
	}
}

func cmdGenerateProofPayload() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate-proof-payload",
		Short: "Generate a PartialProof template for offline multi-party signing",
		Long: `Generate an unsigned PartialProof JSON file for offline multi-party
coordination. The legacy side is resolved from the on-chain account record;
the new (destination) side is specified via --new-key (single-key) or
--new-sub-pub-keys + --new-threshold (multisig).

For nil-pubkey single-key legacy accounts, pass --legacy-key to seed the
pubkey from your local keyring. For multisig legacy accounts, the pubkey
is read from the on-chain account record (submit a 1-ulume self-send first
if the account has no on-chain pubkey).`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}
			legacyStr, _ := cmd.Flags().GetString(flagLegacyAddr)
			newStr, _ := cmd.Flags().GetString(flagNewAddr)
			kind, _ := cmd.Flags().GetString(flagKind)
			evmChainID, _ := cmd.Flags().GetUint64(flagEVMChainID)
			out, _ := cmd.Flags().GetString(flagOut)
			legacyKey, _ := cmd.Flags().GetString(flagLegacyKey)
			sigFmtStr, _ := cmd.Flags().GetString(flagSigFormat)
			newKey, _ := cmd.Flags().GetString(flagNewKey)
			newSubPubKeys, _ := cmd.Flags().GetStringSlice(flagNewSubPubKeys)
			newThreshold, _ := cmd.Flags().GetUint32(flagNewThreshold)

			if kind != migrationProofKindClaim && kind != migrationProofKindValidator {
				return fmt.Errorf("--kind must be %q or %q",
					migrationProofKindClaim, migrationProofKindValidator)
			}
			if _, err := ParseSigFormat(sigFmtStr); err != nil {
				return err
			}
			if evmChainID == 0 {
				evmChainID = lcfg.EVMChainID
			}

			// Validate mutual exclusivity of new-side flags.
			hasNewKey := newKey != ""
			hasNewSubs := len(newSubPubKeys) > 0
			switch {
			case !hasNewKey && !hasNewSubs:
				return fmt.Errorf("one of --new-key or --new-sub-pub-keys is required to specify the destination side")
			case hasNewKey && hasNewSubs:
				return fmt.Errorf("--new-key and --new-sub-pub-keys are mutually exclusive")
			case hasNewSubs && newThreshold == 0:
				return fmt.Errorf("--new-threshold is required with --new-sub-pub-keys")
			}

			legacyAddr, err := sdk.AccAddressFromBech32(legacyStr)
			if err != nil {
				return fmt.Errorf("--legacy: %w", err)
			}

			// Derive the authoritative new address from key material.
			var derivedNewAddr string
			var newSide *SideSpec
			if hasNewKey {
				// Single-key new side: resolve from keyring.
				ethPubKeyBytes, err := resolveEthSubKey(clientCtx, newKey)
				if err != nil {
					return fmt.Errorf("--new-key: %w", err)
				}
				ethPK := &evmcryptotypes.PubKey{Key: ethPubKeyBytes}
				derivedNewAddr = sdk.AccAddress(ethPK.Address()).String()
				newSide = &SideSpec{
					PubKey:    base64.StdEncoding.EncodeToString(ethPubKeyBytes),
					SigFormat: "SIG_FORMAT_EIP191", // default for eth single-key new side
				}
			} else {
				// Multisig new side: resolve each sub-key.
				if int(newThreshold) > len(newSubPubKeys) {
					return fmt.Errorf("--new-threshold=%d exceeds --new-sub-pub-keys count=%d", newThreshold, len(newSubPubKeys))
				}
				ethSubKeys := make([]cryptotypes.PubKey, len(newSubPubKeys))
				subPubKeyB64s := make([]string, len(newSubPubKeys))
				for i, spec := range newSubPubKeys {
					raw, err := resolveEthSubKey(clientCtx, spec)
					if err != nil {
						return fmt.Errorf("--new-sub-pub-keys[%d]: %w", i, err)
					}
					ethSubKeys[i] = &evmcryptotypes.PubKey{Key: raw}
					subPubKeyB64s[i] = base64.StdEncoding.EncodeToString(raw)
				}
				multiPK := kmultisig.NewLegacyAminoPubKey(int(newThreshold), ethSubKeys)
				derivedNewAddr = sdk.AccAddress(multiPK.Address()).String()
				newSide = &SideSpec{
					Threshold:  newThreshold,
					SubPubKeys: subPubKeyB64s,
					SigFormat:  sigFmtStr, // multisig new side uses the caller-specified format
				}
			}

			// If --new was supplied, cross-check against derived address.
			if newStr != "" {
				if newStr != derivedNewAddr {
					return fmt.Errorf("--new %s does not match the address derived from key material (%s)", newStr, derivedNewAddr)
				}
			}

			accPubKey, err := fetchAccount(clientCtx, legacyAddr)
			if err != nil {
				return err
			}

			legacySide, err := buildLegacySideSpec(clientCtx, accPubKey, legacyKey, sigFmtStr, legacyAddr)
			if err != nil {
				return err
			}

			// Mirror-source rule — enforced at consensus via
			// types.ValidateProofPair; surfaced here for early feedback.
			legacyIsSingle := legacySide.PubKey != ""
			newIsSingle := newSide.PubKey != ""
			if legacyIsSingle != newIsSingle {
				return fmt.Errorf("legacy and new sides must have the same shape: legacy is %s but new is %s",
					sideShapeLabel(legacyIsSingle), sideShapeLabel(newIsSingle))
			}
			if !legacyIsSingle {
				if legacySide.Threshold != newSide.Threshold {
					return fmt.Errorf("mirror-source rule: legacy threshold K=%d does not match new threshold K=%d",
						legacySide.Threshold, newSide.Threshold)
				}
				if len(legacySide.SubPubKeys) != len(newSide.SubPubKeys) {
					return fmt.Errorf("mirror-source rule: legacy sub-keys N=%d does not match new sub-keys N=%d",
						len(legacySide.SubPubKeys), len(newSide.SubPubKeys))
				}
			}

			// Key-reuse guard: the same 33-byte compressed secp256k1 pubkey must not
			// appear on both sides. Cosmos secp256k1 and eth_secp256k1 share the curve
			// and the compressed-SEC1 encoding, so a user who accidentally reuses the
			// SAME private key for both sides would have identical base64 pubkey
			// strings. Catch it here before a migration ceremony is built around a
			// self-collision that defeats the point of key rotation.
			if newIsSingle {
				if legacySide.PubKey == newSide.PubKey {
					return fmt.Errorf("destination pub_key %s matches the legacy pub_key; generate a fresh eth key for the new side", newSide.PubKey)
				}
			} else {
				legacySubs := legacySide.SubPubKeys
				for _, ns := range newSide.SubPubKeys {
					for _, ls := range legacySubs {
						if ns == ls {
							return fmt.Errorf("destination sub-key %s matches a legacy sub-key; generate fresh eth keys for the new side", ns)
						}
					}
				}
			}

			pp := &PartialProof{
				Version:                 partialProofVersion,
				Kind:                    kind,
				LegacyAddress:           legacyStr,
				NewAddress:              derivedNewAddr,
				ChainID:                 clientCtx.ChainID,
				EVMChainID:              evmChainID,
				PayloadHex:              hexEncode([]byte(ComputePayload(clientCtx.ChainID, evmChainID, kind, legacyStr, derivedNewAddr))),
				Legacy:                  legacySide,
				New:                     newSide,
				PartialLegacySignatures: []PartialSignature{},
				PartialNewSignatures:    []PartialSignature{},
			}

			if err := validatePartialProof(pp); err != nil {
				return fmt.Errorf("BUG: generated proof fails validation: %w", err)
			}

			if out == "" {
				b, err := pp.MarshalIndent()
				if err != nil {
					return err
				}
				_, err = fmt.Fprintln(cmd.OutOrStdout(), string(b))
				return err
			}
			return SavePartialProof(out, pp)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	cmd.Flags().String(flagLegacyAddr, "", "Legacy (coin-type 118) bech32 address to migrate from")
	cmd.Flags().String(flagNewAddr, "", "New (coin-type 60) bech32 destination address (optional; cross-checked when supplied)")
	cmd.Flags().String(flagKind, migrationProofKindClaim,
		fmt.Sprintf("%q for account migration or %q for operator migration",
			migrationProofKindClaim, migrationProofKindValidator))
	cmd.Flags().Uint64(flagEVMChainID, 0, "EVM chain ID (defaults to lcfg.EVMChainID)")
	cmd.Flags().String(flagOut, "", "Output file path; if empty, writes JSON to stdout")
	cmd.Flags().String(flagLegacyKey, "", "Local keyring key name to seed pubkey for nil-pubkey single-sig accounts")
	cmd.Flags().String(flagSigFormat, "SIG_FORMAT_CLI", "Signing envelope for legacy side: SIG_FORMAT_CLI or SIG_FORMAT_ADR036")
	cmd.Flags().String(flagNewKey, "", "Keyring name of the destination-side single-key (must be eth_secp256k1). Mutually exclusive with --new-sub-pub-keys.")
	cmd.Flags().StringSlice(flagNewSubPubKeys, nil, "Comma-separated list of destination-side sub-keys. Each entry is either a keyring key name or a base64-encoded 33-byte eth_secp256k1 pubkey.")
	cmd.Flags().Uint32(flagNewThreshold, 0, "Threshold K for the destination-side multisig. Required with --new-sub-pub-keys.")
	_ = cmd.MarkFlagRequired(flagLegacyAddr)
	return cmd
}

// sideShapeLabel returns "single-key" or "multisig" for error messages.
func sideShapeLabel(isSingle bool) string {
	if isSingle {
		return "single-key"
	}
	return "multisig"
}

// legacySigningInput returns the bytes to pass to Keyring.Sign for a
// Cosmos-secp256k1-side partial, matching what sigverify.VerifyCosmosSecp256k1
// expects at verification time.
//   - SIG_FORMAT_CLI: sha256(payload) — keyring.Sign hashes again internally;
//     verifier calls pk.VerifySignature(sha256(payload), sig) which also
//     hashes internally, so both sides end up at sha256(sha256(payload)).
//   - SIG_FORMAT_ADR036: canonical ADR-036 sign doc.
//   - SIG_FORMAT_EIP191: error — EIP-191 is not valid on the legacy side.
func legacySigningInput(payload []byte, format string, signerAddr string) ([]byte, error) {
	switch format {
	case types.SigFormat_SIG_FORMAT_CLI.String():
		h := sha256.Sum256(payload)
		return h[:], nil
	case types.SigFormat_SIG_FORMAT_ADR036.String():
		return sigverify.ADR036SignDoc(signerAddr, payload), nil
	case types.SigFormat_SIG_FORMAT_EIP191.String():
		return nil, fmt.Errorf("SIG_FORMAT_EIP191 is not valid on the legacy side")
	default:
		return nil, fmt.Errorf("unsupported legacy sig_format %q", format)
	}
}

// newSigningInput returns the bytes to pass to Keyring.Sign for an
// eth-secp256k1-side partial. The eth keyring applies Keccak256 internally,
// so we pass the payload as-is for SIG_FORMAT_CLI (no pre-hash). For EIP-191
// we wrap in the personal-sign envelope.
func newSigningInput(payload []byte, format string, signerAddr string) ([]byte, error) {
	switch format {
	case types.SigFormat_SIG_FORMAT_CLI.String():
		return payload, nil
	case types.SigFormat_SIG_FORMAT_EIP191.String():
		return sigverify.EIP191PersonalSignPayload(payload), nil
	case types.SigFormat_SIG_FORMAT_ADR036.String():
		return sigverify.ADR036SignDoc(signerAddr, payload), nil
	default:
		return nil, fmt.Errorf("unsupported new sig_format %q", format)
	}
}

// findSubKeyIndex looks up keyName in the keyring, matches its pubkey against
// spec.SubPubKeys (for multisig) or spec.PubKey (for single-key), and returns
// the sub-key index. Errors on key not found, type mismatch, or pubkey mismatch.
func findSubKeyIndex(clientCtx client.Context, keyName string, spec *SideSpec, expected sigverify.SubKeyType) (uint32, error) {
	rec, err := clientCtx.Keyring.Key(keyName)
	if err != nil {
		return 0, fmt.Errorf("key %q not found in keyring: %w", keyName, err)
	}
	pk, err := rec.GetPubKey()
	if err != nil {
		return 0, err
	}
	var keyBytes []byte
	switch expected {
	case sigverify.SubKeyTypeCosmosSecp256k1:
		cpk, ok := pk.(*secp256k1.PubKey)
		if !ok {
			return 0, fmt.Errorf("key %q is %T, expected Cosmos secp256k1", keyName, pk)
		}
		keyBytes = cpk.Bytes()
	case sigverify.SubKeyTypeEthSecp256k1:
		epk, ok := pk.(*evmcryptotypes.PubKey)
		if !ok {
			return 0, fmt.Errorf("key %q is %T, expected eth_secp256k1", keyName, pk)
		}
		keyBytes = epk.Key
	default:
		return 0, fmt.Errorf("unknown expected sub-key type")
	}
	target := base64.StdEncoding.EncodeToString(keyBytes)
	// Single-key side:
	if spec.PubKey != "" {
		if spec.PubKey != target {
			return 0, fmt.Errorf("key %q pubkey does not match partial.PubKey", keyName)
		}
		return 0, nil
	}
	// Multisig side:
	for i, k := range spec.SubPubKeys {
		if k == target {
			return uint32(i), nil
		}
	}
	return 0, fmt.Errorf("key %q pubkey is not a member of partial.SubPubKeys", keyName)
}

// deriveSubKeyAddr returns the bech32 address of keyName from the keyring.
func deriveSubKeyAddr(clientCtx client.Context, keyName string) (string, error) {
	rec, err := clientCtx.Keyring.Key(keyName)
	if err != nil {
		return "", fmt.Errorf("cannot look up key %q for signer-address derivation: %w", keyName, err)
	}
	addr, err := rec.GetAddress()
	if err != nil {
		return "", fmt.Errorf("cannot derive address for key %q: %w", keyName, err)
	}
	return addr.String(), nil
}

// upsertSig replaces any entry at the same index, otherwise appends — idempotent.
// Returns a NEW slice — never aliases the caller's backing array — so a caller
// that retains a reference to the input slice is not surprised by in-place
// mutation. All current call sites reassign the result to the same field, so
// this is a defensive choice against future callers.
func upsertSig(existing []PartialSignature, fresh PartialSignature) []PartialSignature {
	filtered := make([]PartialSignature, 0, len(existing)+1)
	for _, p := range existing {
		if p.Index != fresh.Index {
			filtered = append(filtered, p)
		}
	}
	return append(filtered, fresh)
}

func cmdSignProof() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sign-proof <partial-proof.json>",
		Short: "Add a legacy sub-signature and/or a new sub-signature to a partial proof file",
		Long: `Each co-signer runs sign-proof on their own machine against their own
keyring. Supply --from <legacy-sub-key> to sign the legacy half, and/or
--new-key <new-eth-sub-key> to sign the new half. At least one must be set.

Signatures are idempotent: re-running with the same key replaces that
index's entry in place rather than duplicating it.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			fromKey, _ := cmd.Flags().GetString(flags.FlagFrom)
			newKey, _ := cmd.Flags().GetString(flagNewKey)
			outPath, _ := cmd.Flags().GetString(flagOut)
			if outPath == "" {
				outPath = args[0]
			}
			if fromKey == "" && newKey == "" {
				return fmt.Errorf("at least one of --from (legacy sub-key) or --new-key (new sub-key) must be supplied")
			}

			pp, err := LoadPartialProof(args[0])
			if err != nil {
				return err
			}

			payload, err := hex.DecodeString(pp.PayloadHex)
			if err != nil {
				return fmt.Errorf("invalid payload_hex in partial file: %w", err)
			}

			if fromKey != "" {
				idx, err := findSubKeyIndex(clientCtx, fromKey, pp.Legacy, sigverify.SubKeyTypeCosmosSecp256k1)
				if err != nil {
					return fmt.Errorf("--from: %w", err)
				}
				signerAddr, err := deriveSubKeyAddr(clientCtx, fromKey)
				if err != nil {
					return fmt.Errorf("--from: %w", err)
				}
				signInput, err := legacySigningInput(payload, pp.Legacy.SigFormat, signerAddr)
				if err != nil {
					return fmt.Errorf("--from: %w", err)
				}
				sig, _, err := clientCtx.Keyring.Sign(fromKey, signInput, signingtypes.SignMode_SIGN_MODE_UNSPECIFIED)
				if err != nil {
					return fmt.Errorf("legacy sign: %w", err)
				}
				pp.PartialLegacySignatures = upsertSig(pp.PartialLegacySignatures, PartialSignature{
					Index:     idx,
					Signature: base64.StdEncoding.EncodeToString(sig),
				})
			}

			if newKey != "" {
				idx, err := findSubKeyIndex(clientCtx, newKey, pp.New, sigverify.SubKeyTypeEthSecp256k1)
				if err != nil {
					return fmt.Errorf("--new-key: %w", err)
				}
				signerAddr, err := deriveSubKeyAddr(clientCtx, newKey)
				if err != nil {
					return fmt.Errorf("--new-key: %w", err)
				}
				signInput, err := newSigningInput(payload, pp.New.SigFormat, signerAddr)
				if err != nil {
					return fmt.Errorf("--new-key: %w", err)
				}
				// Eth keyring uses LEGACY_AMINO_JSON sign mode for the interface;
				// internally it applies Keccak256 to whatever bytes we hand it.
				sig, _, err := clientCtx.Keyring.Sign(newKey, signInput, signingtypes.SignMode_SIGN_MODE_LEGACY_AMINO_JSON)
				if err != nil {
					return fmt.Errorf("new sign: %w", err)
				}
				pp.PartialNewSignatures = upsertSig(pp.PartialNewSignatures, PartialSignature{
					Index:     idx,
					Signature: base64.StdEncoding.EncodeToString(sig),
				})
			}

			return SavePartialProof(outPath, pp)
		},
	}
	flags.AddTxFlagsToCmd(cmd)
	cmd.Flags().String(flagOut, "", "Write to this path instead of overwriting the input file")
	return cmd
}

func cmdCombineProof() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "combine-proof <partial1.json> [<partial2.json> ...] --out <tx.json>",
		Short: "Merge partial proofs into a fully-signed migration tx",
		Long: `Merge partials produced by sign-proof (possibly from multiple machines)
into a migration transaction. Validates all inputs agree on legacy_address,
new_address, chain_id, evm_chain_id, kind, sig_format (per side), threshold
(per side), and sub_pub_keys (per side). Verifies each partial signature
cryptographically; drops invalid ones with a stderr warning and selects the K
valid partials with lowest ascending indices. Writes an unsigned migration tx
to --out, with both legacy_proof and new_proof populated.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}
			outPath, _ := cmd.Flags().GetString(flagOut)
			if outPath == "" {
				return fmt.Errorf("--out is required")
			}

			// Load and cross-validate all partials.
			merged, err := LoadPartialProof(args[0])
			if err != nil {
				return fmt.Errorf("%s: %w", args[0], err)
			}
			for _, path := range args[1:] {
				other, err := LoadPartialProof(path)
				if err != nil {
					return fmt.Errorf("%s: %w", path, err)
				}
				if err := AssertPartialProofsConsistent(merged, other); err != nil {
					return fmt.Errorf("%s vs %s: %w", args[0], path, err)
				}
				// Merge partial signatures (idempotent by index; later files win for duplicate indices).
				for _, p := range other.PartialLegacySignatures {
					merged.PartialLegacySignatures = upsertSig(merged.PartialLegacySignatures, p)
				}
				for _, p := range other.PartialNewSignatures {
					merged.PartialNewSignatures = upsertSig(merged.PartialNewSignatures, p)
				}
			}

			payload, err := hex.DecodeString(merged.PayloadHex)
			if err != nil {
				return fmt.Errorf("invalid payload_hex in partial file: %w", err)
			}

			// Verify both sides and assemble proofs.
			legacyProof, err := buildProofFromPartial(
				merged.Legacy, merged.PartialLegacySignatures, payload,
				sigverify.SubKeyTypeCosmosSecp256k1, "legacy", cmd.ErrOrStderr(),
			)
			if err != nil {
				return err
			}
			newProof, err := buildProofFromPartial(
				merged.New, merged.PartialNewSignatures, payload,
				sigverify.SubKeyTypeEthSecp256k1, "new", cmd.ErrOrStderr(),
			)
			if err != nil {
				return err
			}

			// Assemble the final Msg based on kind.
			var msg sdk.Msg
			switch merged.Kind {
			case migrationProofKindClaim:
				msg = &types.MsgClaimLegacyAccount{
					LegacyAddress: merged.LegacyAddress,
					NewAddress:    merged.NewAddress,
					LegacyProof:   legacyProof,
					NewProof:      newProof,
				}
			case migrationProofKindValidator:
				msg = &types.MsgMigrateValidator{
					LegacyAddress: merged.LegacyAddress,
					NewAddress:    merged.NewAddress,
					LegacyProof:   legacyProof,
					NewProof:      newProof,
				}
			default:
				return fmt.Errorf("unsupported kind %q", merged.Kind)
			}

			// Final sanity check: the assembled msg must satisfy ValidateBasic.
			// Belt-and-suspenders — duplicate-index dedupe and per-side selection
			// are already enforced by buildProofFromPartial, but this guarantees
			// we never write a malformed tx to disk. Fail-closed: if a future
			// migration msg type is added to the switch above without a
			// ValidateBasic method, trip here rather than silently bypass.
			vb, ok := msg.(sdk.HasValidateBasic)
			if !ok {
				return fmt.Errorf("BUG: assembled msg type %T does not implement ValidateBasic", msg)
			}
			if err := vb.ValidateBasic(); err != nil {
				return fmt.Errorf("assembled message failed ValidateBasic: %w", err)
			}

			// Build the unsigned tx JSON.
			txb := clientCtx.TxConfig.NewTxBuilder()
			if err := txb.SetMsgs(msg); err != nil {
				return fmt.Errorf("set msgs: %w", err)
			}
			txBytes, err := clientCtx.TxConfig.TxJSONEncoder()(txb.GetTx())
			if err != nil {
				return fmt.Errorf("encode tx: %w", err)
			}
			if err := os.WriteFile(outPath, txBytes, 0o600); err != nil {
				return fmt.Errorf("write %s: %w", outPath, err)
			}
			return nil
		},
	}
	flags.AddTxFlagsToCmd(cmd)
	cmd.Flags().String(flagOut, "", "Output unsigned tx JSON path (required)")
	return cmd
}

func cmdSubmitProof() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "submit-proof <tx.json>",
		Short: "Broadcast a pre-assembled migration tx from a JSON file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}
			b, err := os.ReadFile(args[0])
			if err != nil {
				return err
			}
			tx, err := clientCtx.TxConfig.TxJSONDecoder()(b)
			if err != nil {
				return err
			}
			msgs := tx.GetMsgs()
			if len(msgs) != 1 {
				return fmt.Errorf("expected exactly 1 msg, got %d", len(msgs))
			}
			mpm, ok := msgs[0].(migrationProofMsg)
			if !ok {
				return fmt.Errorf("unexpected msg type %T", msgs[0])
			}

			return runMigrationTx(cmd, mpm)
		},
	}
	flags.AddTxFlagsToCmd(cmd)
	cmd.Flags().String(flagTxTimeout, defaultTxTimeout, "How long to wait for the transaction to be included in a block")
	return cmd
}
