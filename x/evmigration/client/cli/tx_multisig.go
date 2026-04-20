package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
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

	lcfg "github.com/LumeraProtocol/lumera/config"
	"github.com/LumeraProtocol/lumera/x/evmigration/types"
)

// partialProofVersion is the current on-disk format version for PartialProof.
const partialProofVersion = 1

const (
	flagLegacyAddr = "legacy"
	flagNewAddr    = "new"
	flagKind       = "kind"
	flagEVMChainID = "evm-chain-id"
	flagOut        = "out"
	flagLegacyKey  = "legacy-key"
	flagSigFormat  = "sig-format"
)

// PartialProof is a coordination artifact passed between co-signers during
// the multi-step offline signing flow. It is never stored on-chain.
type PartialProof struct {
	Version       int                   `json:"version"`
	Kind          string                `json:"kind"` // "claim" | "validator"
	LegacyAddress string                `json:"legacy_address"`
	NewAddress    string                `json:"new_address"`
	ChainID       string                `json:"chain_id"`
	EVMChainID    uint64                `json:"evm_chain_id"`
	PayloadHex    string                `json:"payload_hex"`
	Single        *PartialSingle        `json:"single,omitempty"`
	Multisig      *PartialMultisig      `json:"multisig,omitempty"`
	PartialSigs   []PartialSubSignature `json:"partial_signatures"`
}

// PartialSingle holds pubkey material for single-key offline proofs.
type PartialSingle struct {
	PubKeyB64 string `json:"pub_key_b64"`
	SigFormat string `json:"sig_format"`
}

// PartialMultisig holds pubkey material for multisig offline proofs.
type PartialMultisig struct {
	Threshold     uint32   `json:"threshold"`
	SubPubKeysB64 []string `json:"sub_pub_keys_b64"`
	SigFormat     string   `json:"sig_format"`
}

// PartialSubSignature holds one signer's contribution to the proof.
type PartialSubSignature struct {
	Index        uint32 `json:"index"`
	SignatureB64 string `json:"signature_b64"`
}

// MarshalIndent writes JSON with 2-space indent for human-readable review.
func (pp *PartialProof) MarshalIndent() ([]byte, error) {
	return json.MarshalIndent(pp, "", "  ")
}

// LoadPartialProof reads a PartialProof JSON file and validates its version.
func LoadPartialProof(path string) (*PartialProof, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var pp PartialProof
	if err := json.Unmarshal(b, &pp); err != nil {
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
	default:
		return "SIG_FORMAT_UNSPECIFIED"
	}
}

// ComputePayload builds the canonical migration payload bytes. Exported for tests.
func ComputePayload(chainID string, evmChainID uint64, kind, legacyAddr, newAddr string) string {
	return fmt.Sprintf("lumera-evm-migration:%s:%d:%s:%s:%s", chainID, evmChainID, kind, legacyAddr, newAddr)
}

// decodeSubPubKeys decodes the base64 sub-pubkeys from a PartialMultisig.
func decodeSubPubKeys(ms *PartialMultisig) ([][]byte, error) {
	out := make([][]byte, len(ms.SubPubKeysB64))
	for i, s := range ms.SubPubKeysB64 {
		b, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			return nil, fmt.Errorf("sub_pub_keys_b64[%d]: %w", i, err)
		}
		if len(b) != secp256k1.PubKeySize {
			return nil, fmt.Errorf("sub_pub_keys_b64[%d]: expected %d bytes, got %d",
				i, secp256k1.PubKeySize, len(b))
		}
		out[i] = b
	}
	return out, nil
}

func canonicalPayloadBytes(pp *PartialProof) []byte {
	return []byte(ComputePayload(pp.ChainID, pp.EVMChainID, pp.Kind, pp.LegacyAddress, pp.NewAddress))
}

func validatePartialProof(pp *PartialProof) error {
	if pp.Version != partialProofVersion {
		return fmt.Errorf("unsupported partial_proof version %d (expected %d)", pp.Version, partialProofVersion)
	}
	if pp.Kind != "claim" && pp.Kind != "validator" {
		return fmt.Errorf("partial proof has invalid kind %q", pp.Kind)
	}
	if pp.Single == nil && pp.Multisig == nil {
		return fmt.Errorf("partial proof has neither 'single' nor 'multisig' section")
	}
	if pp.Single != nil && pp.Multisig != nil {
		return fmt.Errorf("partial proof has both 'single' and 'multisig' sections")
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

func verifyPartialSignature(pkBytes, payload, sig []byte, sigFmt types.SigFormat) bool {
	pk := &secp256k1.PubKey{Key: pkBytes}
	switch sigFmt {
	case types.SigFormat_SIG_FORMAT_CLI:
		hash := sha256.Sum256(payload)
		return pk.VerifySignature(hash[:], sig)
	case types.SigFormat_SIG_FORMAT_ADR036:
		signerAddr := sdk.AccAddress(pk.Address()).String()
		doc := []byte(fmt.Sprintf(
			`{"account_number":"0","chain_id":"","fee":{"amount":[],"gas":"0"},`+
				`"memo":"","msgs":[{"type":"sign/MsgSignData","value":`+
				`{"data":"%s","signer":"%s"}}],"sequence":"0"}`,
			base64.StdEncoding.EncodeToString(payload), signerAddr,
		))
		return pk.VerifySignature(doc, sig)
	default:
		return false
	}
}

// assembleMultisigProof merges partial sub-signatures into a MultisigProof.
// Signatures are deduplicated by index (last write wins). If more than K
// signatures are present, the first K valid signatures in signer-index order
// are selected so stale/corrupted extras do not poison the assembled proof.
func assembleMultisigProof(ms *PartialMultisig, payload []byte, partials []PartialSubSignature) (*types.MultisigProof, error) {
	sigFmt, err := ParseSigFormat(ms.SigFormat)
	if err != nil {
		return nil, err
	}
	subs, err := decodeSubPubKeys(ms)
	if err != nil {
		return nil, err
	}
	byIdx := map[uint32][]byte{}
	for _, p := range partials {
		if int(p.Index) >= len(subs) {
			return nil, fmt.Errorf("partial signature index %d out of range (N=%d)", p.Index, len(subs))
		}
		sig, err := base64.StdEncoding.DecodeString(p.SignatureB64)
		if err != nil {
			return nil, fmt.Errorf("partial signature %d: %w", p.Index, err)
		}
		byIdx[p.Index] = sig
	}
	if uint32(len(byIdx)) < ms.Threshold {
		return nil, fmt.Errorf("need %d partial signatures, have %d", ms.Threshold, len(byIdx))
	}
	indices := make([]uint32, 0, len(byIdx))
	for idx := range byIdx {
		indices = append(indices, idx)
	}
	sort.Slice(indices, func(i, j int) bool { return indices[i] < indices[j] })
	validIndices := make([]uint32, 0, len(indices))
	sigs := make([][]byte, 0, len(indices))
	for _, idx := range indices {
		sig := byIdx[idx]
		if !verifyPartialSignature(subs[idx], payload, sig, sigFmt) {
			continue
		}
		validIndices = append(validIndices, idx)
		sigs = append(sigs, sig)
		if uint32(len(validIndices)) == ms.Threshold {
			break
		}
	}
	if uint32(len(validIndices)) < ms.Threshold {
		return nil, fmt.Errorf("need %d valid partial signatures, have %d", ms.Threshold, len(validIndices))
	}
	return &types.MultisigProof{
		Threshold:     ms.Threshold,
		SubPubKeys:    subs,
		SignerIndices: validIndices,
		SubSignatures: sigs,
		SigFormat:     sigFmt,
	}, nil
}

// assembleSingleProof builds a SingleKeyProof from a single-entry partial list.
func assembleSingleProof(ss *PartialSingle, partials []PartialSubSignature) (*types.SingleKeyProof, error) {
	sigFmt, err := ParseSigFormat(ss.SigFormat)
	if err != nil {
		return nil, err
	}
	pub, err := base64.StdEncoding.DecodeString(ss.PubKeyB64)
	if err != nil {
		return nil, fmt.Errorf("pub_key_b64: %w", err)
	}
	if len(partials) < 1 {
		return nil, fmt.Errorf("need 1 partial signature for single-key proof")
	}
	var sigB64 string
	for _, p := range partials {
		if p.Index != 0 {
			return nil, fmt.Errorf("single-key proof must have index=0, got %d", p.Index)
		}
		sigB64 = p.SignatureB64
	}
	sig, err := base64.StdEncoding.DecodeString(sigB64)
	if err != nil {
		return nil, fmt.Errorf("signature_b64: %w", err)
	}
	return &types.SingleKeyProof{PubKey: pub, Signature: sig, SigFormat: sigFmt}, nil
}

// AssertPartialProofsConsistent verifies two PartialProof files agree on
// every field that would change the assembled tx identity. Exported for testing.
func AssertPartialProofsConsistent(a, b *PartialProof) error {
	if a.Kind != b.Kind {
		return fmt.Errorf("kind mismatch: %q vs %q", a.Kind, b.Kind)
	}
	if a.LegacyAddress != b.LegacyAddress {
		return fmt.Errorf("legacy_address mismatch: %s vs %s", a.LegacyAddress, b.LegacyAddress)
	}
	if a.NewAddress != b.NewAddress {
		return fmt.Errorf("new_address mismatch: %s vs %s", a.NewAddress, b.NewAddress)
	}
	if a.ChainID != b.ChainID {
		return fmt.Errorf("chain_id mismatch: %s vs %s", a.ChainID, b.ChainID)
	}
	if a.EVMChainID != b.EVMChainID {
		return fmt.Errorf("evm_chain_id mismatch: %d vs %d", a.EVMChainID, b.EVMChainID)
	}
	if a.PayloadHex != b.PayloadHex {
		return fmt.Errorf("payload_hex mismatch")
	}
	if (a.Single == nil) != (b.Single == nil) {
		return fmt.Errorf("proof-kind mismatch: one has 'single', the other does not")
	}
	if (a.Multisig == nil) != (b.Multisig == nil) {
		return fmt.Errorf("proof-kind mismatch: one has 'multisig', the other does not")
	}
	if a.Single != nil {
		if a.Single.PubKeyB64 != b.Single.PubKeyB64 {
			return fmt.Errorf("single.pub_key_b64 mismatch")
		}
		if a.Single.SigFormat != b.Single.SigFormat {
			return fmt.Errorf("sig_format mismatch: %s vs %s", a.Single.SigFormat, b.Single.SigFormat)
		}
	}
	if a.Multisig != nil {
		if a.Multisig.Threshold != b.Multisig.Threshold {
			return fmt.Errorf("threshold mismatch: %d vs %d", a.Multisig.Threshold, b.Multisig.Threshold)
		}
		if a.Multisig.SigFormat != b.Multisig.SigFormat {
			return fmt.Errorf("sig_format mismatch: %s vs %s", a.Multisig.SigFormat, b.Multisig.SigFormat)
		}
		if len(a.Multisig.SubPubKeysB64) != len(b.Multisig.SubPubKeysB64) {
			return fmt.Errorf("num sub_pub_keys mismatch: %d vs %d", len(a.Multisig.SubPubKeysB64), len(b.Multisig.SubPubKeysB64))
		}
		for i := range a.Multisig.SubPubKeysB64 {
			if a.Multisig.SubPubKeysB64[i] != b.Multisig.SubPubKeysB64[i] {
				return fmt.Errorf("sub_pub_keys_b64[%d] mismatch", i)
			}
		}
	}
	return nil
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

func cmdGenerateProofPayload() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate-proof-payload",
		Short: "Generate a PartialProof template for offline multi-party signing",
		Long: `Generate an unsigned PartialProof JSON file for offline multi-party
coordination. For multisig accounts the sub-pubkeys and threshold are
read from the on-chain account record. For nil-pubkey single-key
accounts, pass --legacy-key to seed the pubkey from your local keyring.`,
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

			if kind != "claim" && kind != "validator" {
				return fmt.Errorf("--kind must be 'claim' or 'validator'")
			}
			if _, err := ParseSigFormat(sigFmtStr); err != nil {
				return err
			}
			if evmChainID == 0 {
				evmChainID = lcfg.EVMChainID
			}

			legacyAddr, err := sdk.AccAddressFromBech32(legacyStr)
			if err != nil {
				return fmt.Errorf("--legacy: %w", err)
			}
			if _, err := sdk.AccAddressFromBech32(newStr); err != nil {
				return fmt.Errorf("--new: %w", err)
			}

			accPubKey, err := fetchAccount(clientCtx, legacyAddr)
			if err != nil {
				return err
			}

			pp := &PartialProof{
				Version:       partialProofVersion,
				Kind:          kind,
				LegacyAddress: legacyStr,
				NewAddress:    newStr,
				ChainID:       clientCtx.ChainID,
				EVMChainID:    evmChainID,
				PayloadHex:    hexEncode([]byte(ComputePayload(clientCtx.ChainID, evmChainID, kind, legacyStr, newStr))),
				PartialSigs:   []PartialSubSignature{},
			}

			switch pk := accPubKey.(type) {
			case *secp256k1.PubKey:
				if legacyKey != "" {
					rec, err := clientCtx.Keyring.Key(legacyKey)
					if err != nil {
						return fmt.Errorf("--legacy-key %q not found: %w", legacyKey, err)
					}
					kp, err := rec.GetPubKey()
					if err != nil {
						return err
					}
					if !bytes.Equal(kp.Bytes(), pk.Bytes()) {
						return fmt.Errorf("--legacy-key pubkey does not match on-chain pubkey")
					}
				}
				pp.Single = &PartialSingle{
					PubKeyB64: base64.StdEncoding.EncodeToString(pk.Bytes()),
					SigFormat: sigFmtStr,
				}
			case *kmultisig.LegacyAminoPubKey:
				if legacyKey != "" {
					return fmt.Errorf("--legacy-key is not applicable for multisig accounts")
				}
				subs := make([]string, len(pk.GetPubKeys()))
				for i, k := range pk.GetPubKeys() {
					subs[i] = base64.StdEncoding.EncodeToString(k.Bytes())
				}
				pp.Multisig = &PartialMultisig{
					Threshold:     uint32(pk.Threshold),
					SubPubKeysB64: subs,
					SigFormat:     sigFmtStr,
				}
			case nil:
				if legacyKey == "" {
					return fmt.Errorf("account at %s has no on-chain pubkey record; pass --legacy-key to seed the pubkey from your keyring (single-sig only), or for a multisig address submit a 1-ulume self-send first", legacyAddr)
				}
				rec, err := clientCtx.Keyring.Key(legacyKey)
				if err != nil {
					return fmt.Errorf("--legacy-key %q not found: %w", legacyKey, err)
				}
				kp, err := rec.GetPubKey()
				if err != nil {
					return err
				}
				secp, ok := kp.(*secp256k1.PubKey)
				if !ok {
					return fmt.Errorf("--legacy-key %q is not secp256k1 (got %T)", legacyKey, kp)
				}
				if !sdk.AccAddress(secp.Address()).Equals(legacyAddr) {
					return fmt.Errorf("--legacy-key derives to %s, expected %s",
						sdk.AccAddress(secp.Address()), legacyAddr)
				}
				pp.Single = &PartialSingle{
					PubKeyB64: base64.StdEncoding.EncodeToString(secp.Bytes()),
					SigFormat: sigFmtStr,
				}
			default:
				return fmt.Errorf("unsupported pubkey type %T", pk)
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
	cmd.Flags().String(flagNewAddr, "", "New (coin-type 60) bech32 destination address")
	cmd.Flags().String(flagKind, "claim", "'claim' for account migration or 'validator' for operator migration")
	cmd.Flags().Uint64(flagEVMChainID, 0, "EVM chain ID (defaults to lcfg.EVMChainID)")
	cmd.Flags().String(flagOut, "", "Output file path; if empty, writes JSON to stdout")
	cmd.Flags().String(flagLegacyKey, "", "Local keyring key name to seed pubkey for nil-pubkey single-sig accounts")
	cmd.Flags().String(flagSigFormat, "SIG_FORMAT_CLI", "Signing envelope: SIG_FORMAT_CLI or SIG_FORMAT_ADR036")
	_ = cmd.MarkFlagRequired(flagLegacyAddr)
	_ = cmd.MarkFlagRequired(flagNewAddr)
	return cmd
}

func cmdSignProof() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sign-proof <partial-proof.json>",
		Short: "Append your sub-signature to a PartialProof file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}
			fromKey := clientCtx.FromName
			out, _ := cmd.Flags().GetString(flagOut)
			if out == "" {
				out = args[0]
			}

			pp, err := LoadPartialProof(args[0])
			if err != nil {
				return err
			}

			rec, err := clientCtx.Keyring.Key(fromKey)
			if err != nil {
				return fmt.Errorf("--from key %q not found: %w", fromKey, err)
			}
			kp, err := rec.GetPubKey()
			if err != nil {
				return err
			}
			secp, ok := kp.(*secp256k1.PubKey)
			if !ok {
				return fmt.Errorf("--from key %q is not secp256k1 (got %T)", fromKey, kp)
			}

			payloadBytes := canonicalPayloadBytes(pp)

			var idx uint32
			var found bool
			switch {
			case pp.Single != nil:
				pub, err := base64.StdEncoding.DecodeString(pp.Single.PubKeyB64)
				if err != nil {
					return err
				}
				if !bytes.Equal(pub, secp.Bytes()) {
					return fmt.Errorf("--from key does not match single proof's pubkey")
				}
				idx = 0
				found = true
			case pp.Multisig != nil:
				for i, s := range pp.Multisig.SubPubKeysB64 {
					b, err := base64.StdEncoding.DecodeString(s)
					if err != nil {
						return err
					}
					if bytes.Equal(b, secp.Bytes()) {
						idx = uint32(i)
						found = true
						break
					}
				}
				if !found {
					return fmt.Errorf("--from key is not a member of the multisig")
				}
			}
			if !found {
				return fmt.Errorf("partial proof has neither single nor multisig; cannot sign")
			}

			var sigFmtStr string
			if pp.Single != nil {
				sigFmtStr = pp.Single.SigFormat
			} else {
				sigFmtStr = pp.Multisig.SigFormat
			}

			var sig []byte
			switch sigFmtStr {
			case "SIG_FORMAT_CLI":
				hash := sha256.Sum256(payloadBytes)
				sig, _, err = clientCtx.Keyring.Sign(fromKey, hash[:], signingtypes.SignMode_SIGN_MODE_UNSPECIFIED)
			case "SIG_FORMAT_ADR036":
				signerAddr := sdk.AccAddress(secp.Address()).String()
				doc := []byte(fmt.Sprintf(`{"account_number":"0","chain_id":"","fee":{"amount":[],"gas":"0"},"memo":"","msgs":[{"type":"sign/MsgSignData","value":{"data":"%s","signer":"%s"}}],"sequence":"0"}`,
					base64.StdEncoding.EncodeToString(payloadBytes), signerAddr))
				sig, _, err = clientCtx.Keyring.Sign(fromKey, doc, signingtypes.SignMode_SIGN_MODE_UNSPECIFIED)
			default:
				return fmt.Errorf("unsupported sig_format %q", sigFmtStr)
			}
			if err != nil {
				return fmt.Errorf("sign: %w", err)
			}

			// Idempotent upsert: remove existing entry for this index, then append.
			filtered := pp.PartialSigs[:0]
			for _, p := range pp.PartialSigs {
				if p.Index != idx {
					filtered = append(filtered, p)
				}
			}
			pp.PartialSigs = append(filtered, PartialSubSignature{
				Index:        idx,
				SignatureB64: base64.StdEncoding.EncodeToString(sig),
			})

			return SavePartialProof(out, pp)
		},
	}
	flags.AddTxFlagsToCmd(cmd)
	cmd.Flags().String(flagOut, "", "Write to this path instead of overwriting the input file")
	return cmd
}

func cmdCombineProof() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "combine-proof <partial1.json> [<partial2.json> ...]",
		Short: "Merge partial proofs into an unsigned tx JSON",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out, _ := cmd.Flags().GetString(flagOut)
			if out == "" {
				return fmt.Errorf("--out is required")
			}

			merged, err := LoadPartialProof(args[0])
			if err != nil {
				return err
			}
			for _, p := range args[1:] {
				other, err := LoadPartialProof(p)
				if err != nil {
					return err
				}
				if err := AssertPartialProofsConsistent(merged, other); err != nil {
					return fmt.Errorf("%s: %w", p, err)
				}
				for _, ps := range other.PartialSigs {
					filtered := merged.PartialSigs[:0]
					for _, m := range merged.PartialSigs {
						if m.Index != ps.Index {
							filtered = append(filtered, m)
						}
					}
					merged.PartialSigs = append(filtered, ps)
				}
			}

			var legacyProof types.LegacyProof
			switch {
			case merged.Single != nil:
				sp, err := assembleSingleProof(merged.Single, merged.PartialSigs)
				if err != nil {
					return err
				}
				legacyProof = types.LegacyProof{Proof: &types.LegacyProof_Single{Single: sp}}
			case merged.Multisig != nil:
				mp, err := assembleMultisigProof(merged.Multisig, canonicalPayloadBytes(merged), merged.PartialSigs)
				if err != nil {
					return err
				}
				legacyProof = types.LegacyProof{Proof: &types.LegacyProof_Multisig{Multisig: mp}}
			}

			if err := legacyProof.ValidateBasic(); err != nil {
				return fmt.Errorf("assembled proof fails ValidateBasic: %w", err)
			}

			var unsignedMsg sdk.Msg
			switch merged.Kind {
			case "claim":
				unsignedMsg = &types.MsgClaimLegacyAccount{
					NewAddress:    merged.NewAddress,
					LegacyAddress: merged.LegacyAddress,
					LegacyProof:   legacyProof,
				}
			case "validator":
				unsignedMsg = &types.MsgMigrateValidator{
					NewAddress:    merged.NewAddress,
					LegacyAddress: merged.LegacyAddress,
					LegacyProof:   legacyProof,
				}
			default:
				return fmt.Errorf("unknown kind %q", merged.Kind)
			}

			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}
			txb := clientCtx.TxConfig.NewTxBuilder()
			if err := txb.SetMsgs(unsignedMsg); err != nil {
				return err
			}
			bts, err := clientCtx.TxConfig.TxJSONEncoder()(txb.GetTx())
			if err != nil {
				return err
			}
			return os.WriteFile(out, bts, 0o600)
		},
	}
	flags.AddTxFlagsToCmd(cmd)
	cmd.Flags().String(flagOut, "", "Output unsigned tx JSON path (required)")
	return cmd
}

func cmdSubmitProof() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "submit-proof <tx.json>",
		Short: "Sign new_signature with --from eth key, simulate gas, broadcast",
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

			var kind string
			switch msgs[0].(type) {
			case *types.MsgClaimLegacyAccount:
				kind = migrationProofKindClaim
			case *types.MsgMigrateValidator:
				kind = migrationProofKindValidator
			default:
				return fmt.Errorf("unexpected msg type %T", msgs[0])
			}

			return runMigrationTx(cmd, mpm, kind, clientCtx.FromName)
		},
	}
	flags.AddTxFlagsToCmd(cmd)
	cmd.Flags().String(flagTxTimeout, defaultTxTimeout, "How long to wait for the transaction to be included in a block")
	return cmd
}
