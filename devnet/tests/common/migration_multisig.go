package common

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// evmMultisigMemberNames returns the keyring names for the destination EVM
// multisig sub-keys derived from a legacy multisig base name. They mirror the
// legacy members one-for-one ("evm-<base>-signer-{1..N}") so signer index i on
// the legacy side pairs with index i on the new side during the proof flow.
func evmMultisigMemberNames(legacyBase string, signers int) []string {
	names := make([]string, 0, signers)
	for i := 1; i <= signers; i++ {
		names = append(names, fmt.Sprintf("evm-%s-signer-%d", legacyBase, i))
	}
	return names
}

// evmMultisigCompositeName returns the destination EVM multisig composite key
// name for a legacy multisig base name.
func evmMultisigCompositeName(legacyBase string) string {
	return "evm-" + legacyBase
}

// generateProofPayloadArgs builds `tx evmigration generate-proof-payload`. The
// new-side signer pubkeys are supplied as a CSV of keyring key names (resolved
// from the local keyring) and --new-threshold sets the destination K.
func generateProofPayloadArgs(legacyAddr, kind, outPath, newSubPubKeysCSV string, newThreshold int, keyringBackend, home string) []string {
	args := []string{
		"tx", "evmigration", "generate-proof-payload",
		"--legacy", legacyAddr,
		"--kind", kind,
		"--out", outPath,
		"--new-sub-pub-keys", newSubPubKeysCSV,
		"--new-threshold", strconv.Itoa(newThreshold),
		"--keyring-backend", keyringBackend,
		"--output", "json",
	}
	if home != "" {
		args = append(args, "--home", home)
	}
	return args
}

// signProofArgs builds `tx evmigration sign-proof` signing both the legacy side
// (--from, a Cosmos secp256k1 sub-key) and the new side (--new-key, an
// eth_secp256k1 sub-key) in one invocation.
func signProofArgs(input, fromKey, newKey, outPath, chainID, keyringBackend, home string) []string {
	args := []string{
		"tx", "evmigration", "sign-proof", input,
		"--from", fromKey,
		"--new-key", newKey,
		"--out", outPath,
		"--chain-id", chainID,
		"--keyring-backend", keyringBackend,
	}
	if home != "" {
		args = append(args, "--home", home)
	}
	return args
}

// combineProofArgs builds `tx evmigration combine-proof <partials...> --out tx`.
func combineProofArgs(partials []string, outPath string) []string {
	args := []string{"tx", "evmigration", "combine-proof"}
	args = append(args, partials...)
	return append(args, "--out", outPath)
}

// submitProofArgs builds `tx evmigration submit-proof <tx>`.
func submitProofArgs(txPath, chainID, node, keyringBackend string) []string {
	return []string{
		"tx", "evmigration", "submit-proof", txPath,
		"--chain-id", chainID,
		"--node", node,
		"--keyring-backend", keyringBackend,
		"--output", "json",
		"--yes",
	}
}

// MultisigProofResult is the outcome of a multisig proof migration.
type MultisigProofResult struct {
	NewName    string
	NewAddress string
	TxHash     string
}

// MigrateMultisigProof runs the full four-step multisig migration for a legacy
// K-of-N multisig account whose member sub-keys (members) are already in the
// keyring. It creates a fresh destination EVM multisig, then runs
// generate-proof-payload -> sign-proof (×threshold) -> combine-proof ->
// submit-proof, writing intermediate artifacts under workDir. It signs with the
// first `threshold` members, pairing legacy index i with new index i (both sides
// built --nosort in member order). Returns the destination key name/address and
// the submit tx hash.
func (m *Multisig) MigrateMultisigProof(legacyBase, legacyAddr string, members []string, threshold, signers int, workDir string) (MultisigProofResult, error) {
	if len(members) < threshold {
		return MultisigProofResult{}, fmt.Errorf("multisig %s has %d members, need at least %d to sign", legacyBase, len(members), threshold)
	}

	// 1. Build the destination EVM multisig (fresh eth_secp256k1 sub-keys + composite).
	newMembers := evmMultisigMemberNames(legacyBase, signers)
	for _, name := range newMembers {
		if !m.CLI.HasKey(name) {
			if _, err := m.CLI.AddKeyWithStyle(name, KeyStyleEVM); err != nil {
				return MultisigProofResult{}, fmt.Errorf("create EVM sub-key %s: %w", name, err)
			}
		}
	}
	compositeName := evmMultisigCompositeName(legacyBase)
	newAddr, err := m.CreateMultisigKey(compositeName, newMembers, threshold)
	if err != nil {
		return MultisigProofResult{}, fmt.Errorf("create destination EVM multisig %s: %w", compositeName, err)
	}

	// 2. generate-proof-payload.
	proofPath := filepath.Join(workDir, "proof.json")
	newSubCSV := strings.Join(newMembers, ",")
	if out, err := m.exec(m.CLI.Bin, append(generateProofPayloadArgs(legacyAddr, "claim", proofPath, newSubCSV, threshold, m.keyring(), m.CLI.Home), m.nodeArgs()...)...); err != nil {
		return MultisigProofResult{}, fmt.Errorf("generate-proof-payload: %s: %w", truncate(out, 200), err)
	}

	// 3. sign-proof for the first `threshold` member pairs.
	partials := make([]string, 0, threshold)
	for i := 0; i < threshold; i++ {
		partial := filepath.Join(workDir, fmt.Sprintf("partial-%d.json", i+1))
		args := signProofArgs(proofPath, members[i], newMembers[i], partial, m.CLI.ChainID, m.keyring(), m.CLI.Home)
		if out, err := m.exec(m.CLI.Bin, args...); err != nil {
			return MultisigProofResult{}, fmt.Errorf("sign-proof %s: %s: %w", members[i], truncate(out, 200), err)
		}
		partials = append(partials, partial)
	}

	// 4. combine-proof.
	txPath := filepath.Join(workDir, "tx.json")
	if out, err := m.exec(m.CLI.Bin, combineProofArgs(partials, txPath)...); err != nil {
		return MultisigProofResult{}, fmt.Errorf("combine-proof: %s: %w", truncate(out, 200), err)
	}

	// 5. submit-proof.
	out, err := m.exec(m.CLI.Bin, submitProofArgs(txPath, m.CLI.ChainID, m.CLI.RPC, m.keyring())...)
	if err != nil {
		return MultisigProofResult{}, fmt.Errorf("submit-proof: %s: %w", truncate(out, 200), err)
	}
	txHash, code, rawLog, ok := parseSyncBroadcast(out)
	if !ok {
		return MultisigProofResult{}, fmt.Errorf("submit-proof: no tx response: %s", truncate(out, 200))
	}
	if code != 0 {
		return MultisigProofResult{}, fmt.Errorf("submit-proof rejected code=%d raw_log=%s", code, rawLog)
	}
	if err := m.CLI.WaitForTxInclusion(txHash, 60*time.Second); err != nil {
		return MultisigProofResult{}, fmt.Errorf("submit-proof wait inclusion: %w", err)
	}

	return MultisigProofResult{NewName: compositeName, NewAddress: newAddr, TxHash: txHash}, nil
}

// MigrateMultisigProofTempDir runs MigrateMultisigProof using a fresh temp work
// directory cleaned up afterward. This is the convenience entry point for
// callers that do not need to retain the proof artifacts.
func (c *ChainCLI) MigrateMultisigProof(legacyBase, legacyAddr string, members []string, threshold, signers int) (MultisigProofResult, error) {
	dir, err := os.MkdirTemp("", "evmig-proof-"+legacyBase+"-*")
	if err != nil {
		return MultisigProofResult{}, fmt.Errorf("create work dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	return NewMultisig(c).MigrateMultisigProof(legacyBase, legacyAddr, members, threshold, signers, dir)
}
