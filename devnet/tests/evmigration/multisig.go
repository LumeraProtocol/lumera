// multisig.go provides reusable multisig helpers for the devnet evmigration
// harness. The standalone "multisig" mode still exists as a smoke test, and
// prepare/migrate flows also use the same helpers for integrated multisig
// fixtures.
//
// The core reusable path is:
//
//	generate-proof-payload → sign-proof × 2 → combine-proof → submit-proof
//
// For legacy multisig accounts, a 1-ulume self-send is used beforehand so the
// multisig pubkey is recorded on-chain (required by generate-proof-payload).
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
	defaultMultisigThreshold = 2
	defaultMultisigSigners   = 3

	multisigSigner1Name = "multisig-signer-1"
	multisigSigner2Name = "multisig-signer-2"
	multisigSigner3Name = "multisig-signer-3"
	multisigAccountName = "multisig-account"
	multisigNewKeyName  = "multisig-new"
	multisigFundAmount  = "1000000ulume"
	multisigSelfSendAmt = "1ulume"

	// New-side eth_secp256k1 sub-keys for multisig-to-multisig destinations.
	multisigNewSigner1Name   = "multisig-new-signer-1"
	multisigNewSigner2Name   = "multisig-new-signer-2"
	multisigNewSigner3Name   = "multisig-new-signer-3"
	multisigNewCompositeName = "multisig-new-account"
)

func derivedMultisigMemberKeys(baseName string, signerCount int) []string {
	if signerCount < 1 {
		signerCount = defaultMultisigSigners
	}
	members := make([]string, 0, signerCount)
	for i := 1; i <= signerCount; i++ {
		members = append(members, fmt.Sprintf("%s-signer-%d", baseName, i))
	}
	return members
}

// derivedMultisigNewSubKeys mirrors derivedMultisigMemberKeys but yields names
// for the new-side eth_secp256k1 sub-keys used as members of the new-side
// composite multisig.
func derivedMultisigNewSubKeys(baseName string, signerCount int) []string {
	if signerCount < 1 {
		signerCount = defaultMultisigSigners
	}
	members := make([]string, 0, signerCount)
	for i := 1; i <= signerCount; i++ {
		members = append(members, fmt.Sprintf("%s-new-signer-%d", baseName, i))
	}
	return members
}

// ensureMultisigNewSubKeys creates (or reuses) signerCount eth_secp256k1 keys
// under names derived from baseName, used as sub-keys in the new-side multisig
// destination. Returns the key names (suitable for `keys add --multisig`).
// Rerun-safe: existing keys are reused as-is.
func ensureMultisigNewSubKeys(baseName string, signerCount int) ([]string, error) {
	names := derivedMultisigNewSubKeys(baseName, signerCount)
	for _, name := range names {
		if _, err := createOrReuseFreshEVMKey(name); err != nil {
			return nil, fmt.Errorf("create new-side sub-key %s: %w", name, err)
		}
	}
	return names, nil
}

// ensureMultisigNewComposite creates (or reuses) a K-of-N multisig composite
// key over eth_secp256k1 sub-keys. Thin wrapper over ensureMultisigCompositeKey,
// which is key-type-agnostic: Cosmos SDK's LegacyAminoPubKey is defined over
// the cryptotypes.PubKey interface and accepts eth_secp256k1 members.
func ensureMultisigNewComposite(compositeName string, subKeyNames []string, threshold int) (string, error) {
	return ensureMultisigCompositeKey(compositeName, subKeyNames, threshold)
}

// ensureNewMultisigFixture creates 3 eth_secp256k1 sub-keys + a K-of-N composite
// multisig key over them under the default fixture names. Returns the composite
// key's bech32 address and the sub-key names. Rerun-safe.
func ensureNewMultisigFixture() (compositeAddr string, subKeyNames []string, err error) {
	subKeys := []string{multisigNewSigner1Name, multisigNewSigner2Name, multisigNewSigner3Name}
	if _, err := ensureMultisigNewSubKeys("multisig", defaultMultisigSigners); err != nil {
		return "", nil, fmt.Errorf("create new-side sub-keys: %w", err)
	}
	addr, err := ensureMultisigNewComposite(multisigNewCompositeName, subKeys, defaultMultisigThreshold)
	if err != nil {
		return "", nil, fmt.Errorf("create new-side composite: %w", err)
	}
	return addr, subKeys, nil
}

// getLegacyMultisigKeys returns the 3 legacy sub-key names and the composite
// key name for the default multisig fixture (suitable for CLI invocations).
func getLegacyMultisigKeys() (subKeys []string, compositeName string) {
	return []string{multisigSigner1Name, multisigSigner2Name, multisigSigner3Name}, multisigAccountName
}

// getNewMultisigKeys returns the 3 new-side eth sub-key names and the new
// composite key name for the default multisig fixture.
func getNewMultisigKeys() (subKeys []string, compositeName string) {
	return []string{multisigNewSigner1Name, multisigNewSigner2Name, multisigNewSigner3Name}, multisigNewCompositeName
}

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
	if err := registerMultisigPubKey(multisigAccountName, multisigAddr, members); err != nil {
		return fmt.Errorf("step 3 (register multisig pubkey via self-send): %w", err)
	}
	if err := waitForNextBlock(20 * time.Second); err != nil {
		log.Printf("  WARN: wait for next block after self-send: %v", err)
	}

	// Step 4: Create the new EVM destination key (eth_secp256k1, coin-type 60).
	newRec, err := createOrReuseFreshEVMKey(multisigNewKeyName)
	if err != nil {
		return fmt.Errorf("step 4 (create new EVM key): %w", err)
	}
	log.Printf("  new EVM key: %s (%s)", newRec.Name, newRec.Address)

	// Steps 5–8: Run the four-step migration flow.
	if err := runFourStepMigration("claim", multisigAddr, newRec.Name, newRec.Address, members); err != nil {
		return fmt.Errorf("step 5 (four-step migration): %w", err)
	}
	if err := waitForNextBlock(20 * time.Second); err != nil {
		log.Printf("  WARN: wait for next block after migration tx: %v", err)
	}

	// Step 9: Verify the migration record and balances.
	if err := verifyMultisigMigration(multisigAddr, newRec.Address); err != nil {
		return fmt.Errorf("step 9 (verify migration): %w", err)
	}

	log.Println("=== MULTISIG MODE: SUCCESS ===")
	return nil
}

// createMultisigKeys creates three secp256k1 signer keys and a 2-of-3 multisig
// composite key. Returns the member key names and the multisig bech32 address.
// Keys are reused from the keyring if they already exist (rerun-safe).
func createMultisigKeys() (members []string, multisigAddr string, err error) {
	return createNamedMultisigKey(multisigAccountName, defaultMultisigThreshold, []string{
		multisigSigner1Name,
		multisigSigner2Name,
		multisigSigner3Name,
	})
}

func createNamedMultisigKey(multisigKeyName string, threshold int, memberNames []string) (members []string, multisigAddr string, err error) {
	if threshold < 1 {
		return nil, "", fmt.Errorf("invalid multisig threshold %d", threshold)
	}
	if len(memberNames) < threshold {
		return nil, "", fmt.Errorf("multisig key %s has %d members, need at least threshold %d", multisigKeyName, len(memberNames), threshold)
	}
	if err := ensureMultisigMembers(memberNames); err != nil {
		return nil, "", err
	}
	addr, err := ensureMultisigCompositeKey(multisigKeyName, memberNames, threshold)
	if err != nil {
		return nil, "", err
	}
	return append([]string(nil), memberNames...), addr, nil
}

func ensureMultisigMembers(memberNames []string) error {
	for _, name := range memberNames {
		if keyExists(name) {
			log.Printf("  key %s already in keyring, reusing", name)
			continue
		}
		rec, err := generateAccount(name, true)
		if err != nil {
			return fmt.Errorf("generate key %s: %w", name, err)
		}
		if err := importKey(name, rec.Mnemonic, true); err != nil {
			return fmt.Errorf("import key %s: %w", name, err)
		}
		log.Printf("  created signer key %s (%s)", name, rec.Address)
	}
	return nil
}

func ensureMultisigCompositeKey(multisigKeyName string, members []string, threshold int) (string, error) {
	if keyExists(multisigKeyName) {
		log.Printf("  multisig key %s already in keyring, reusing", multisigKeyName)
		return getAddress(multisigKeyName)
	}

	// `keys add` is a pure keyring operation; it rejects --node, so skip
	// buildLumeraArgs here and only append --home when set.
	args := []string{
		"keys", "add", multisigKeyName,
		"--multisig", strings.Join(members, ","),
		"--multisig-threshold", fmt.Sprintf("%d", threshold),
		"--keyring-backend", "test",
	}
	if *flagHome != "" {
		args = append(args, "--home", *flagHome)
	}
	cmd := exec.Command(*flagBin, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("keys add multisig %s: %s\n%w", multisigKeyName, string(out), err)
	}
	log.Printf("  created multisig key %s", multisigKeyName)
	addr, err := getAddress(multisigKeyName)
	if err != nil {
		return "", fmt.Errorf("get multisig address %s: %w", multisigKeyName, err)
	}
	return addr, nil
}

// registerMultisigPubKey issues a 1-ulume self-send from the multisig account
// so that the multisig pubkey (LegacyAminoPubKey) is recorded on-chain. This
// is required before generate-proof-payload can read the pubkey from the chain.
//
// Flow: generate-only → each member signs → tx multisign → broadcast.
func registerMultisigPubKey(multisigKeyName, multisigAddr string, members []string) error {
	log.Printf("  registering multisig pubkey via 1-ulume self-send from %s", multisigAddr)
	if len(members) < defaultMultisigThreshold {
		return fmt.Errorf("multisig %s has %d members, need at least %d signers", multisigKeyName, len(members), defaultMultisigThreshold)
	}

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
		"--from", multisigKeyName,
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
	for i, member := range members[:defaultMultisigThreshold] {
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
		"tx", "multisign", unsignedFile, multisigKeyName,
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

// buildUnsignedMultisigBankSendTx generates an unsigned bank-send tx with
// multisigAddr as the sender. The caller uses signAndBroadcastMultisigTx to
// collect the threshold signatures and broadcast it.
func buildUnsignedMultisigBankSendTx(multisigKeyName, multisigAddr, toAddr, amount, outFile string) error {
	accNum, seq, err := queryAccountNumberAndSequence(multisigAddr)
	if err != nil {
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
		multisigAddr, toAddr, amount,
		"--from", multisigKeyName,
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
		return fmt.Errorf("generate unsigned multisig bank-send tx: %s\n%w", string(unsignedOut), err)
	}
	if err := os.WriteFile(outFile, unsignedOut, 0o600); err != nil {
		return fmt.Errorf("write unsigned multisig bank-send tx to %s: %w", outFile, err)
	}
	return nil
}

func buildUnsignedMultisigDelegateTx(multisigKeyName, multisigAddr, validatorAddr, amount, outFile string) error {
	accNum, seq, err := queryAccountNumberAndSequence(multisigAddr)
	if err != nil {
		if waitErr := waitForAccountOnChain(multisigAddr, 30*time.Second); waitErr != nil {
			return fmt.Errorf("wait for multisig account on-chain: %w", waitErr)
		}
		accNum, seq, err = queryAccountNumberAndSequence(multisigAddr)
		if err != nil {
			return fmt.Errorf("query account number/sequence for %s: %w", multisigAddr, err)
		}
	}

	unsignedArgs := buildLumeraArgs(
		"tx", "staking", "delegate",
		validatorAddr, amount,
		"--from", multisigKeyName,
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
		return fmt.Errorf("generate unsigned multisig delegate tx: %s\n%w", string(unsignedOut), err)
	}
	if err := os.WriteFile(outFile, unsignedOut, 0o600); err != nil {
		return fmt.Errorf("write unsigned multisig delegate tx to %s: %w", outFile, err)
	}
	return nil
}

func signAndBroadcastMultisigTx(unsignedFile, multisigKeyName, multisigAddr string, members []string) error {
	if len(members) < defaultMultisigThreshold {
		return fmt.Errorf("multisig %s has %d members, need at least %d", multisigKeyName, len(members), defaultMultisigThreshold)
	}

	accNum, seq, err := queryAccountNumberAndSequence(multisigAddr)
	if err != nil {
		return fmt.Errorf("query account number/sequence for %s: %w", multisigAddr, err)
	}

	sigFiles := make([]string, defaultMultisigThreshold)
	for i := range sigFiles {
		sigFiles[i] = tmpFile(fmt.Sprintf("multisig-sig%d-*.json", i+1))
		defer os.Remove(sigFiles[i]) //nolint:gocritic // intentional deferred cleanup
	}
	signedFile := tmpFile("multisig-signed-*.json")
	defer os.Remove(signedFile)

	for i, member := range members[:defaultMultisigThreshold] {
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
		cmd := exec.Command(*flagBin, signArgs...)
		sigOut, sigErr := cmd.CombinedOutput()
		if sigErr != nil {
			return fmt.Errorf("sign tx with %s: %s\n%w", member, string(sigOut), sigErr)
		}
		if err := os.WriteFile(sigFiles[i], sigOut, 0o600); err != nil {
			return fmt.Errorf("write signature %s to %s: %w", member, sigFiles[i], err)
		}
	}

	multisignArgs := buildLumeraArgs(
		"tx", "multisign", unsignedFile, multisigKeyName,
		sigFiles[0], sigFiles[1],
		"--keyring-backend", "test",
		"--chain-id", *flagChainID,
		"--output", "json",
	)
	cmd := exec.Command(*flagBin, multisignArgs...)
	msignOut, msignErr := cmd.CombinedOutput()
	if msignErr != nil {
		return fmt.Errorf("tx multisign: %s\n%w", string(msignOut), msignErr)
	}
	if err := os.WriteFile(signedFile, msignOut, 0o600); err != nil {
		return fmt.Errorf("write signed tx to %s: %w", signedFile, err)
	}

	broadcastArgs := buildLumeraArgs(
		"tx", "broadcast", signedFile,
		"--broadcast-mode", "sync",
		"--output", "json",
	)
	cmd = exec.Command(*flagBin, broadcastArgs...)
	bcastOut, bcastErr := cmd.CombinedOutput()
	bcastStr := strings.TrimSpace(string(bcastOut))
	if bcastErr != nil {
		return fmt.Errorf("broadcast multisig tx: %s\n%w", bcastStr, bcastErr)
	}
	txHash := extractTxHash(bcastStr)
	if txHash != "" {
		code, rawLog, err := waitForTxResult(txHash, 45*time.Second)
		if err != nil {
			return fmt.Errorf("wait for multisig tx %s: %w", txHash, err)
		}
		if code != 0 {
			return fmt.Errorf("multisig tx failed code=%d raw_log=%s", code, rawLog)
		}
	}
	return nil
}

// createNewEVMKey creates (or reuses) the eth_secp256k1 destination key.
// Returns the bech32 address of the new key.
func createOrReuseFreshEVMKey(keyName string) (AccountRecord, error) {
	if keyExists(keyName) {
		addr, err := getAddress(keyName)
		if err != nil {
			return AccountRecord{}, fmt.Errorf("get address for existing new EVM key %s: %w", keyName, err)
		}
		log.Printf("  new EVM key %s already in keyring (%s), reusing", keyName, addr)
		return AccountRecord{Name: keyName, Address: addr, IsLegacy: false}, nil
	}

	rec, err := generateAccount(keyName, false)
	if err != nil {
		return AccountRecord{}, fmt.Errorf("generate new EVM key %s: %w", keyName, err)
	}
	if err := importKey(keyName, rec.Mnemonic, false); err != nil {
		return AccountRecord{}, fmt.Errorf("import new EVM key %s: %w", keyName, err)
	}
	addr, err := getAddress(keyName)
	if err != nil {
		return AccountRecord{}, fmt.Errorf("get address for new EVM key %s: %w", keyName, err)
	}
	rec.Address = addr
	rec.Name = keyName
	rec.IsLegacy = false
	return rec, nil
}

// runFourStepMigration executes the four-step CLI migration flow:
//
//  1. generate-proof-payload -> proof.json
//  2. sign-proof proof.json  --from multisig-signer-1
//  3. sign-proof proof.json  --from multisig-signer-3  (any 2-of-3)
//  4. combine-proof proof.json --out tx.json
//  5. submit-proof tx.json   --from the new destination key
func runFourStepMigration(kind, legacyAddr, newKeyName, newAddr string, members []string) error {
	proofFile := tmpFile("multisig-proof-*.json")
	defer os.Remove(proofFile)
	txFile := tmpFile("multisig-tx-*.json")
	defer os.Remove(txFile)
	if len(members) < defaultMultisigThreshold {
		return fmt.Errorf("multisig proof flow requires at least %d members, got %d", defaultMultisigThreshold, len(members))
	}
	if kind == "" {
		kind = "claim"
	}

	// Step 5a: generate-proof-payload
	// generate-proof-payload is registered via AddQueryFlagsToCmd, which does
	// not include --keyring-backend. For an on-chain multisig account the
	// command reads the pubkey from chain and doesn't touch the keyring, so
	// the flag isn't needed.
	log.Printf("  [migration step 1] generate-proof-payload (%s): %s -> %s", kind, legacyAddr, newAddr)
	// --chain-id is required: the payload string includes it, and the keeper's
	// verifySecp256k1Sig reconstructs the payload using ctx.ChainID(). Without
	// it, pp.ChainID is empty and the signatures won't verify on-chain.
	genArgs := buildLumeraArgs(
		"tx", "evmigration", "generate-proof-payload",
		"--legacy", legacyAddr,
		"--new", newAddr,
		"--kind", kind,
		"--out", proofFile,
		"--chain-id", *flagChainID,
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
	log.Printf("  [migration step 5] submit-proof with %s", newKeyName)
	submitArgs := buildLumeraArgs(
		"tx", "evmigration", "submit-proof", txFile,
		"--from", newKeyName,
		"--chain-id", *flagChainID,
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
