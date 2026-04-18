// multisig.go implements the "multisig" mode. It seeds a 2-of-3 secp256k1
// multisig legacy account, funds it, issues a 1-ulume self-send so the
// multisig pubkey is recorded on-chain (required by generate-proof-payload),
// then runs the four-step evmigration CLI flow:
//
//	generate-proof-payload → sign-proof × 2 → combine-proof → submit-proof
//
// Finally it verifies the migration record exists and that balances moved.
package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

// multisigKeyNames is a fixed set of key names used by this mode. Using
// well-known names makes reruns and manual inspection easier.
const (
	multisigSigner1Name  = "multisig-signer-1"
	multisigSigner2Name  = "multisig-signer-2"
	multisigSigner3Name  = "multisig-signer-3"
	multisigAccountName  = "multisig-account"
	multisigNewKeyName   = "multisig-new"
	multisigFundAmount   = "1000000ulume"
	multisigSelfSendAmt  = "1ulume"
)

// RunMultisigMigration is the main entry point for the "multisig" mode. It
// orchestrates the full flow end-to-end and returns an error if any step fails.
func RunMultisigMigration() error {
	log.Println("=== MULTISIG MODE ===")
	ensureEVMMigrationRuntime("multisig mode")

	if *flagFunder == "" {
		name, err := detectFunder()
		if err != nil {
			return fmt.Errorf("step 0 (detect funder): %w", err)
		}
		*flagFunder = name
		log.Printf("  auto-detected funder: %s", *flagFunder)
	}
	funderAddr, err := getAddress(*flagFunder)
	if err != nil {
		return fmt.Errorf("step 0 (funder address): %w", err)
	}
	log.Printf("  funder: %s (%s)", *flagFunder, funderAddr)

	// Step 1: Create signer keys and the multisig composite key.
	members, multisigAddr, err := createMultisigKeys()
	if err != nil {
		return fmt.Errorf("step 1 (create multisig keys): %w", err)
	}
	log.Printf("  multisig address: %s (signers: %v)", multisigAddr, members)

	// Step 2: Fund the multisig account from the funder.
	log.Printf("  funding %s with %s from %s", multisigAddr, multisigFundAmount, *flagFunder)
	if _, err := runTx("tx", "bank", "send", funderAddr, multisigAddr, multisigFundAmount, "--from", *flagFunder); err != nil {
		return fmt.Errorf("step 2 (fund multisig): %w", err)
	}
	if err := waitForNextBlock(20 * time.Second); err != nil {
		log.Printf("  WARN: wait for next block after funding: %v", err)
	}

	// Step 3: Self-send 1ulume from the multisig so its pubkey lands on-chain.
	// This is a precondition for generate-proof-payload on multisig accounts
	// (which requires the on-chain pubkey to be populated).
	if err := registerMultisigPubKey(multisigAddr, members); err != nil {
		return fmt.Errorf("step 3 (register multisig pubkey via self-send): %w", err)
	}
	if err := waitForNextBlock(20 * time.Second); err != nil {
		log.Printf("  WARN: wait for next block after self-send: %v", err)
	}

	// Step 4: Create the new EVM destination key (eth_secp256k1, coin-type 60).
	newAddr, err := createNewEVMKey()
	if err != nil {
		return fmt.Errorf("step 4 (create new EVM key): %w", err)
	}
	log.Printf("  new EVM key: %s (%s)", multisigNewKeyName, newAddr)

	// Steps 5–8: Run the four-step migration flow.
	if err := runFourStepMigration(multisigAddr, newAddr, members); err != nil {
		return fmt.Errorf("step 5 (four-step migration): %w", err)
	}
	if err := waitForNextBlock(20 * time.Second); err != nil {
		log.Printf("  WARN: wait for next block after migration tx: %v", err)
	}

	// Step 9: Verify the migration record and balances.
	if err := verifyMultisigMigration(multisigAddr, newAddr); err != nil {
		return fmt.Errorf("step 9 (verify migration): %w", err)
	}

	log.Println("=== MULTISIG MODE: SUCCESS ===")
	return nil
}

// createMultisigKeys creates three secp256k1 signer keys and a 2-of-3 multisig
// composite key. Returns the member key names and the multisig bech32 address.
// Keys are reused from the keyring if they already exist (rerun-safe).
func createMultisigKeys() (members []string, multisigAddr string, err error) {
	memberNames := []string{multisigSigner1Name, multisigSigner2Name, multisigSigner3Name}

	// Create (or reuse) individual secp256k1 signer keys (coin-type 118 = legacy Cosmos).
	for _, name := range memberNames {
		if keyExists(name) {
			log.Printf("  key %s already in keyring, reusing", name)
			continue
		}
		rec, genErr := generateAccount(name, true /* isLegacy → coin-type 118, secp256k1 */)
		if genErr != nil {
			return nil, "", fmt.Errorf("generate key %s: %w", name, genErr)
		}
		if impErr := importKey(name, rec.Mnemonic, true); impErr != nil {
			return nil, "", fmt.Errorf("import key %s: %w", name, impErr)
		}
		log.Printf("  created signer key %s (%s)", name, rec.Address)
	}
	members = memberNames

	// Create (or reuse) the multisig composite key.
	if keyExists(multisigAccountName) {
		log.Printf("  multisig key %s already in keyring, reusing", multisigAccountName)
	} else {
		args := buildLumeraArgs(
			"keys", "add", multisigAccountName,
			"--multisig", strings.Join(memberNames, ","),
			"--multisig-threshold", "2",
			"--keyring-backend", "test",
		)
		cmd := exec.Command(*flagBin, args...)
		out, cmdErr := cmd.CombinedOutput()
		if cmdErr != nil {
			return nil, "", fmt.Errorf("keys add multisig %s: %s\n%w", multisigAccountName, string(out), cmdErr)
		}
		log.Printf("  created multisig key %s", multisigAccountName)
	}

	addr, err := getAddress(multisigAccountName)
	if err != nil {
		return nil, "", fmt.Errorf("get multisig address: %w", err)
	}
	return members, addr, nil
}

// registerMultisigPubKey issues a 1-ulume self-send from the multisig account
// so that the multisig pubkey (LegacyAminoPubKey) is recorded on-chain. This
// is required before generate-proof-payload can read the pubkey from the chain.
//
// Flow: generate-only → each member signs → tx multisign → broadcast.
func registerMultisigPubKey(multisigAddr string, members []string) error {
	log.Printf("  registering multisig pubkey via 1-ulume self-send from %s", multisigAddr)

	// Temp files for the unsigned tx and per-member signatures.
	unsignedFile := tmpFile("multisig-unsigned-*.json")
	defer os.Remove(unsignedFile)

	sigFiles := make([]string, len(members))
	for i := range members {
		sigFiles[i] = tmpFile(fmt.Sprintf("multisig-sig%d-*.json", i+1))
		defer os.Remove(sigFiles[i]) //nolint:gocritic // intentional deferred cleanup
	}
	signedFile := tmpFile("multisig-signed-*.json")
	defer os.Remove(signedFile)

	// 1. Generate unsigned tx (generate-only).
	accNum, seq, err := queryAccountNumberAndSequence(multisigAddr)
	if err != nil {
		// Account may not exist yet if funding tx hasn't landed — retry once.
		if waitErr := waitForAccountOnChain(multisigAddr, 30*time.Second); waitErr != nil {
			return fmt.Errorf("wait for multisig account on-chain: %w", waitErr)
		}
		accNum, seq, err = queryAccountNumberAndSequence(multisigAddr)
		if err != nil {
			return fmt.Errorf("query account number/sequence for %s: %w", multisigAddr, err)
		}
	}

	unsignedArgs := buildLumeraArgs(
		"tx", "bank", "send",
		multisigAddr, multisigAddr, multisigSelfSendAmt,
		"--from", multisigAccountName,
		"--keyring-backend", "test",
		"--chain-id", *flagChainID,
		"--account-number", fmt.Sprintf("%d", accNum),
		"--sequence", fmt.Sprintf("%d", seq),
		"--gas", *flagGas,
		"--gas-prices", *flagGasPrices,
		"--generate-only",
		"--output", "json",
	)
	cmd := exec.Command(*flagBin, unsignedArgs...)
	unsignedOut, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("generate unsigned self-send tx: %s\n%w", string(unsignedOut), err)
	}
	if err := os.WriteFile(unsignedFile, unsignedOut, 0o600); err != nil {
		return fmt.Errorf("write unsigned tx to %s: %w", unsignedFile, err)
	}

	// 2. Each member signs the unsigned tx.
	for i, member := range members[:2] { // threshold is 2; signer-1 and signer-2 sign
		signArgs := buildLumeraArgs(
			"tx", "sign", unsignedFile,
			"--from", member,
			"--multisig", multisigAddr,
			"--keyring-backend", "test",
			"--chain-id", *flagChainID,
			"--account-number", fmt.Sprintf("%d", accNum),
			"--sequence", fmt.Sprintf("%d", seq),
			"--sign-mode", "amino-json",
			"--output", "json",
		)
		cmd = exec.Command(*flagBin, signArgs...)
		sigOut, sigErr := cmd.CombinedOutput()
		if sigErr != nil {
			return fmt.Errorf("sign tx with %s: %s\n%w", member, string(sigOut), sigErr)
		}
		if err := os.WriteFile(sigFiles[i], sigOut, 0o600); err != nil {
			return fmt.Errorf("write signature %s to %s: %w", member, sigFiles[i], err)
		}
		log.Printf("  signed with %s -> %s", member, sigFiles[i])
	}

	// 3. Combine signatures via tx multisign.
	multisignArgs := buildLumeraArgs(
		"tx", "multisign", unsignedFile, multisigAccountName,
		sigFiles[0], sigFiles[1],
		"--keyring-backend", "test",
		"--chain-id", *flagChainID,
		"--output", "json",
	)
	cmd = exec.Command(*flagBin, multisignArgs...)
	msignOut, msignErr := cmd.CombinedOutput()
	if msignErr != nil {
		return fmt.Errorf("tx multisign: %s\n%w", string(msignOut), msignErr)
	}
	if err := os.WriteFile(signedFile, msignOut, 0o600); err != nil {
		return fmt.Errorf("write signed tx to %s: %w", signedFile, err)
	}

	// 4. Broadcast the signed tx and wait for inclusion.
	broadcastArgs := buildLumeraArgs(
		"tx", "broadcast", signedFile,
		"--broadcast-mode", "sync",
		"--output", "json",
	)
	cmd = exec.Command(*flagBin, broadcastArgs...)
	bcastOut, bcastErr := cmd.CombinedOutput()
	bcastStr := strings.TrimSpace(string(bcastOut))
	if bcastErr != nil {
		return fmt.Errorf("broadcast multisig self-send: %s\n%w", bcastStr, bcastErr)
	}

	// Extract tx hash and wait for inclusion.
	txHash := extractTxHash(bcastStr)
	if txHash != "" {
		code, rawLog, err := waitForTxResult(txHash, 45*time.Second)
		if err != nil {
			return fmt.Errorf("wait for self-send tx %s: %w", txHash, err)
		}
		if code != 0 {
			return fmt.Errorf("self-send tx failed code=%d raw_log=%s", code, rawLog)
		}
	}

	log.Printf("  multisig self-send confirmed (hash: %s)", txHash)
	return nil
}

// createNewEVMKey creates (or reuses) the eth_secp256k1 destination key.
// Returns the bech32 address of the new key.
func createNewEVMKey() (string, error) {
	if keyExists(multisigNewKeyName) {
		addr, err := getAddress(multisigNewKeyName)
		if err != nil {
			return "", fmt.Errorf("get address for existing new EVM key %s: %w", multisigNewKeyName, err)
		}
		log.Printf("  new EVM key %s already in keyring (%s), reusing", multisigNewKeyName, addr)
		return addr, nil
	}

	// Generate a new eth_secp256k1 key (coin-type 60).
	rec, err := generateAccount(multisigNewKeyName, false /* not legacy → eth_secp256k1, coin-type 60 */)
	if err != nil {
		return "", fmt.Errorf("generate new EVM key: %w", err)
	}
	if err := importKey(multisigNewKeyName, rec.Mnemonic, false); err != nil {
		return "", fmt.Errorf("import new EVM key %s: %w", multisigNewKeyName, err)
	}
	addr, err := getAddress(multisigNewKeyName)
	if err != nil {
		return "", fmt.Errorf("get address for new EVM key %s: %w", multisigNewKeyName, err)
	}
	return addr, nil
}

// runFourStepMigration executes the four-step CLI migration flow:
//
//  1. generate-proof-payload -> proof.json
//  2. sign-proof proof.json  --from multisig-signer-1
//  3. sign-proof proof.json  --from multisig-signer-3  (any 2-of-3)
//  4. combine-proof proof.json --out tx.json
//  5. submit-proof tx.json   --from multisig-new
func runFourStepMigration(multisigAddr, newAddr string, members []string) error {
	proofFile := tmpFile("multisig-proof-*.json")
	defer os.Remove(proofFile)
	txFile := tmpFile("multisig-tx-*.json")
	defer os.Remove(txFile)

	// Step 5a: generate-proof-payload
	log.Printf("  [migration step 1] generate-proof-payload: %s -> %s", multisigAddr, newAddr)
	genArgs := buildLumeraArgs(
		"tx", "evmigration", "generate-proof-payload",
		"--legacy", multisigAddr,
		"--new", newAddr,
		"--kind", "claim",
		"--out", proofFile,
		"--keyring-backend", "test",
	)
	cmd := exec.Command(*flagBin, genArgs...)
	genOut, genErr := cmd.CombinedOutput()
	if genErr != nil {
		return fmt.Errorf("generate-proof-payload: %s\n%w", string(genOut), genErr)
	}
	log.Printf("  proof payload written to %s", proofFile)

	// Step 5b: sign-proof with signer-1 (index 0).
	log.Printf("  [migration step 2] sign-proof with %s", members[0])
	if err := runSignProof(proofFile, members[0]); err != nil {
		return fmt.Errorf("sign-proof (%s): %w", members[0], err)
	}

	// Step 5c: sign-proof with signer-3 (index 2) — gives us indices 0 and 2
	// for a 2-of-3 threshold. Any two signers satisfy the threshold.
	log.Printf("  [migration step 3] sign-proof with %s", members[2])
	if err := runSignProof(proofFile, members[2]); err != nil {
		return fmt.Errorf("sign-proof (%s): %w", members[2], err)
	}

	// Step 5d: combine-proof (merges both partial sigs into an unsigned tx JSON).
	log.Printf("  [migration step 4] combine-proof -> %s", txFile)
	combineArgs := buildLumeraArgs(
		"tx", "evmigration", "combine-proof", proofFile,
		"--out", txFile,
		"--keyring-backend", "test",
	)
	cmd = exec.Command(*flagBin, combineArgs...)
	combineOut, combineErr := cmd.CombinedOutput()
	if combineErr != nil {
		return fmt.Errorf("combine-proof: %s\n%w", string(combineOut), combineErr)
	}
	log.Printf("  unsigned tx written to %s", txFile)

	// Step 5e: submit-proof — signs new_signature with the EVM key and broadcasts.
	log.Printf("  [migration step 5] submit-proof with %s", multisigNewKeyName)
	submitArgs := buildLumeraArgs(
		"tx", "evmigration", "submit-proof", txFile,
		"--from", multisigNewKeyName,
		"--keyring-backend", "test",
		"--gas", "auto",
		"--gas-adjustment", *flagGasAdj,
		"--gas-prices", *flagGasPrices,
		"--yes",
		"--broadcast-mode", "sync",
	)
	cmd = exec.Command(*flagBin, submitArgs...)
	submitOut, submitErr := cmd.CombinedOutput()
	submitStr := strings.TrimSpace(string(submitOut))
	if submitErr != nil {
		return fmt.Errorf("submit-proof: %s\n%w", submitStr, submitErr)
	}

	txHash := extractTxHash(submitStr)
	if txHash != "" {
		code, rawLog, err := waitForTxResult(txHash, 45*time.Second)
		if err != nil {
			return fmt.Errorf("wait for submit-proof tx %s: %w", txHash, err)
		}
		if code != 0 {
			return fmt.Errorf("submit-proof tx failed code=%d raw_log=%s", code, rawLog)
		}
	}

	log.Printf("  submit-proof confirmed (hash: %s)", txHash)
	return nil
}

// runSignProof appends one sub-signature to the PartialProof file at path.
func runSignProof(proofPath, fromKey string) error {
	signArgs := buildLumeraArgs(
		"tx", "evmigration", "sign-proof", proofPath,
		"--from", fromKey,
		"--keyring-backend", "test",
	)
	cmd := exec.Command(*flagBin, signArgs...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("sign-proof --from %s: %s\n%w", fromKey, string(out), err)
	}
	return nil
}

// verifyMultisigMigration checks that the migration record exists and that the
// multisig address no longer holds its original balance (funds moved to newAddr).
func verifyMultisigMigration(multisigAddr, newAddr string) error {
	log.Println("  --- verifying migration ---")

	// 1. Migration record must exist and point to newAddr.
	exists, recordNewAddr := queryMigrationRecord(multisigAddr)
	if !exists {
		return fmt.Errorf("migration record missing for %s", multisigAddr)
	}
	if recordNewAddr != newAddr {
		return fmt.Errorf("migration record points to %s, expected %s", recordNewAddr, newAddr)
	}
	log.Printf("  migration record OK: %s -> %s", multisigAddr, recordNewAddr)

	// 2. Legacy address balance should be 0 (or very small — fees may leave dust).
	legacyBal, err := queryBalance(multisigAddr)
	if err != nil {
		log.Printf("  WARN: query legacy balance: %v", err)
	} else {
		log.Printf("  legacy balance after migration: %d ulume", legacyBal)
	}

	// 3. New address should have received funds.
	newBal, err := queryBalance(newAddr)
	if err != nil {
		return fmt.Errorf("query new address balance: %w", err)
	}
	if newBal <= 0 {
		return fmt.Errorf("new address %s has zero balance after migration", newAddr)
	}
	log.Printf("  new address balance after migration: %d ulume", newBal)

	log.Println("  migration verification PASSED")
	return nil
}

// --- Helpers ---

// buildLumeraArgs builds the argument list for a lumerad command, prepending
// node and home flags when set.
func buildLumeraArgs(args ...string) []string {
	var extra []string
	if *flagRPC != "" {
		extra = append(extra, "--node", *flagRPC)
	}
	if *flagHome != "" {
		extra = append(extra, "--home", *flagHome)
	}
	return append(args, extra...)
}

// tmpFile creates a temporary file with the given pattern and returns its path.
// The caller is responsible for removing it.
func tmpFile(pattern string) string {
	f, err := os.CreateTemp("", pattern)
	if err != nil {
		log.Fatalf("create temp file %s: %v", pattern, err)
	}
	f.Close()
	return f.Name()
}

// extractTxHash extracts the txhash value from a JSON broadcast response.
// Returns empty string if not found.
func extractTxHash(out string) string {
	// Quick scan for "txhash":"<hash>" without pulling in encoding/json.
	const marker = `"txhash":"`
	idx := strings.Index(out, marker)
	if idx < 0 {
		return ""
	}
	rest := out[idx+len(marker):]
	end := strings.IndexByte(rest, '"')
	if end < 0 {
		return ""
	}
	return strings.TrimSpace(rest[:end])
}
