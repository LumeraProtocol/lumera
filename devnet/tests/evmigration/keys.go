// keys.go provides key derivation, generation, import/export, signing, and
// lumerad version detection. It handles both legacy (coin-type 118 / secp256k1)
// and EVM (coin-type 60 / eth_secp256k1) key algorithms.
package main

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"strings"

	"gen/tests/common"

	cosmoshd "github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	evmsecp256k1 "github.com/cosmos/evm/crypto/ethsecp256k1"
	evmhd "github.com/cosmos/evm/crypto/hd"
	"github.com/cosmos/go-bip39"
)

// --- Key derivation from mnemonic ---

// deriveKey derives a secp256k1 private key from a mnemonic using the Cosmos HD path.
// coinType 118 = legacy Cosmos, coinType 60 = Ethereum.
func deriveKey(mnemonic string, coinType uint32) (*secp256k1.PrivKey, error) {
	seed, err := bip39.NewSeedWithErrorChecking(mnemonic, "")
	if err != nil {
		return nil, fmt.Errorf("mnemonic to seed: %w", err)
	}
	hdPath := fmt.Sprintf("m/44'/%d'/0'/0/0", coinType)
	master, ch := cosmoshd.ComputeMastersFromSeed(seed)
	derivedKey, err := cosmoshd.DerivePrivateKeyForPath(master, ch, hdPath)
	if err != nil {
		return nil, fmt.Errorf("derive key: %w", err)
	}
	privKey := &secp256k1.PrivKey{Key: derivedKey}
	return privKey, nil
}

// deriveEthKey derives an eth_secp256k1 private key from a mnemonic.
func deriveEthKey(mnemonic string, coinType uint32) (*evmsecp256k1.PrivKey, error) {
	hdPath := fmt.Sprintf("m/44'/%d'/0'/0/0", coinType)
	deriveFn := evmhd.EthSecp256k1.Derive()
	derivedKey, err := deriveFn(mnemonic, "", hdPath)
	if err != nil {
		return nil, fmt.Errorf("derive eth key: %w", err)
	}
	if len(derivedKey) != evmsecp256k1.PrivKeySize {
		return nil, fmt.Errorf("unexpected eth private key length: %d", len(derivedKey))
	}
	return &evmsecp256k1.PrivKey{Key: derivedKey}, nil
}

// generateAccount creates a new account with a random mnemonic.
// Legacy accounts always use coin-type 118.
// Non-legacy accounts use coin-type selected from lumerad version threshold.
func generateAccount(name string, isLegacy bool) (AccountRecord, error) {
	entropy, err := bip39.NewEntropy(256)
	if err != nil {
		return AccountRecord{}, fmt.Errorf("entropy: %w", err)
	}
	mnemonic, err := bip39.NewMnemonic(entropy)
	if err != nil {
		return AccountRecord{}, fmt.Errorf("mnemonic: %w", err)
	}

	coinType := uint32(118)
	if !isLegacy {
		coinType = nonLegacyCoinType
	}

	if !isLegacy && useEthAlgoForNonLegacy() {
		privKey, err := deriveEthKey(mnemonic, coinType)
		if err != nil {
			return AccountRecord{}, err
		}
		pubKey := privKey.PubKey().(*evmsecp256k1.PubKey)
		addr := sdk.AccAddress(pubKey.Address())

		return AccountRecord{
			Name:      name,
			Mnemonic:  mnemonic,
			Address:   addr.String(),
			PubKeyB64: base64.StdEncoding.EncodeToString(pubKey.Key),
			IsLegacy:  isLegacy,
		}, nil
	}

	privKey, err := deriveKey(mnemonic, coinType)
	if err != nil {
		return AccountRecord{}, err
	}
	pubKey := privKey.PubKey().(*secp256k1.PubKey)
	addr := sdk.AccAddress(pubKey.Address())

	return AccountRecord{
		Name:      name,
		Mnemonic:  mnemonic,
		Address:   addr.String(),
		PubKeyB64: base64.StdEncoding.EncodeToString(pubKey.Key),
		IsLegacy:  isLegacy,
	}, nil
}

// keyRecord holds a key entry as returned by "lumerad keys list --output json".
type keyRecord struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Address string `json:"address"`
	PubKey  string `json:"pubkey"`
}

func isMultisigKeyRecord(k keyRecord) bool {
	pubKey := strings.ToLower(strings.TrimSpace(k.PubKey))
	return strings.Contains(pubKey, "legacyaminopubkey") || strings.Contains(pubKey, "multisig")
}

var (
	nonLegacyCoinType    uint32 = 60
	nonLegacyCoinTypeStr string = "60"
)

// useEthAlgoForNonLegacy returns true if non-legacy accounts should use eth_secp256k1.
func useEthAlgoForNonLegacy() bool {
	return nonLegacyCoinType == 60
}

// prepareRuntimeAllowed returns true if the detected coin type is compatible with prepare mode.
func prepareRuntimeAllowed(coinType uint32) bool {
	return coinType == 118
}

// ensurePrepareRuntime verifies the lumerad binary is pre-EVM (coin-type 118)
// and fatally exits if the runtime does not support prepare mode.
func ensurePrepareRuntime() {
	coinType, version, err := detectNonLegacyCoinType()
	if err != nil {
		log.Fatalf("prepare mode requires pre-EVM lumerad < %s, but version detection failed: %v",
			*flagEVMCutoverVer, err)
	}
	if !prepareRuntimeAllowed(coinType) {
		log.Fatalf("prepare mode is disabled on EVM-enabled lumerad >= %s; detected %s (evm coin-type %d). Run prepare before the EVM upgrade",
			*flagEVMCutoverVer, version, coinType)
	}
	log.Printf("prepare mode runtime check passed: detected pre-EVM lumerad %s (legacy coin-type 118)", version)
}

// ensureEVMMigrationRuntime verifies the lumerad binary is EVM-enabled (coin-type 60)
// and fatally exits if it is not.
func ensureEVMMigrationRuntime(mode string) {
	coinType, version, err := detectNonLegacyCoinType()
	if err != nil {
		log.Fatalf("%s requires EVM-enabled lumerad >= %s, but version detection failed: %v",
			mode, *flagEVMCutoverVer, err)
	}
	if coinType != 60 {
		log.Fatalf("%s requires EVM-enabled lumerad >= %s; detected %s (evm coin-type %d). Migration is not possible before EVM upgrade",
			mode, *flagEVMCutoverVer, version, coinType)
	}
	log.Printf("%s runtime check passed: detected lumerad %s (evm coin-type 60)", mode, version)
}

// initNonLegacyCoinType detects the lumerad version and sets the global
// nonLegacyCoinType variable (118 for pre-EVM, 60 for EVM-enabled).
func initNonLegacyCoinType() {
	coinType, ver, err := detectNonLegacyCoinType()
	if err != nil {
		// Sensible fallback if version probing fails.
		if *flagMode == "prepare" {
			coinType = 118
		} else {
			coinType = 60
		}
		log.Printf("WARN: detect lumerad version failed (%v); using evm coin-type %d for mode=%s", err, coinType, *flagMode)
	} else {
		log.Printf("detected lumerad version %s; using evm coin-type %d", ver, coinType)
	}
	nonLegacyCoinType = coinType
	nonLegacyCoinTypeStr = strconv.FormatUint(uint64(coinType), 10)
}

// detectNonLegacyCoinType probes the lumerad binary version and returns the
// appropriate coin type (60 if >= EVM cutover version, 118 otherwise).
func detectNonLegacyCoinType() (uint32, string, error) {
	version, err := detectLumeradVersion()
	if err != nil {
		return 0, "", err
	}
	cmp, err := common.CompareSemver(version, *flagEVMCutoverVer)
	if err != nil {
		return 0, version, err
	}
	if cmp >= 0 {
		return 60, version, nil
	}
	return 118, version, nil
}

// detectLumeradVersion runs "lumerad version" and extracts the semantic version string.
func detectLumeradVersion() (string, error) {
	tryCmds := [][]string{
		{*flagBin, "version"},
		{*flagBin, "version", "--long"},
	}
	var lastOut []byte
	var lastErr error
	for _, argv := range tryCmds {
		cmd := exec.Command(argv[0], argv[1:]...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			lastErr = err
			lastOut = out
			continue
		}
		if ver, ok := common.ExtractSemver(string(out)); ok {
			return ver, nil
		}
		lastOut = out
	}
	if lastErr != nil {
		return "", fmt.Errorf("run version command failed: %w", lastErr)
	}
	return "", fmt.Errorf("could not parse semantic version from: %s", truncate(string(lastOut), 200))
}

// errNoSingleSigValidatorFunder signals that the local keyring has no
// single-sig key matching an on-chain validator operator address. This is the
// expected state on a multisig-validator host: the validator's composite key
// can't discharge a single --from signer, and no other key is guaranteed to
// hold enough balance to seed test fixtures. Callers that can gracefully skip
// (e.g. prepare mode) check for this sentinel.
var errNoSingleSigValidatorFunder = errors.New("no single-sig validator funder key on this host")

// detectFunder picks a funder from the local keyring by finding the first key
// whose address matches an active validator's operator address (i.e. a genesis
// validator account that is guaranteed to have funds). Returns
// errNoSingleSigValidatorFunder when no such key exists — callers decide
// whether to fatal or skip.
func detectFunder() (string, error) {
	keys, err := listKeys()
	if err != nil {
		return "", fmt.Errorf("list keys: %w", err)
	}
	if len(keys) == 0 {
		return "", fmt.Errorf("no keys found in keyring")
	}

	validators, err := getValidators()
	if err != nil {
		return "", fmt.Errorf("get validators: %w", err)
	}

	valAccAddrs := make(map[string]struct{}, len(validators))
	for _, valoper := range validators {
		valAddr, err := sdk.ValAddressFromBech32(valoper)
		if err != nil {
			continue
		}
		valAccAddrs[sdk.AccAddress(valAddr).String()] = struct{}{}
	}

	// Only accept a single-sig key whose address matches a validator operator.
	// Any other candidate (sub-signer key, hermes, gov, etc.) isn't
	// guaranteed to have enough genesis balance to seed fixtures.
	for _, k := range keys {
		if isMultisigKeyRecord(k) {
			continue
		}
		if _, ok := valAccAddrs[k.Address]; ok {
			return k.Name, nil
		}
	}
	return "", errNoSingleSigValidatorFunder
}

// findLocalMultisigValidator returns the multisig composite key that matches
// an on-chain validator's operator address (if one exists on this host's
// keyring). Used by prepare's bootstrap path to seed a single-sig funder from
// the composite's genesis balance.
func findLocalMultisigValidator() (keyName, addr string, err error) {
	keys, err := listKeys()
	if err != nil {
		return "", "", fmt.Errorf("list keys: %w", err)
	}
	validators, err := getValidators()
	if err != nil {
		return "", "", fmt.Errorf("get validators: %w", err)
	}
	valAccAddrs := make(map[string]struct{}, len(validators))
	for _, valoper := range validators {
		valAddr, err := sdk.ValAddressFromBech32(valoper)
		if err != nil {
			continue
		}
		valAccAddrs[sdk.AccAddress(valAddr).String()] = struct{}{}
	}
	for _, k := range keys {
		if !isMultisigKeyRecord(k) {
			continue
		}
		if _, ok := valAccAddrs[k.Address]; ok {
			return k.Name, k.Address, nil
		}
	}
	return "", "", fmt.Errorf("no multisig validator key found in local keyring")
}

// listKeys returns all keys from the lumerad test keyring.
func listKeys() ([]keyRecord, error) {
	args := []string{"keys", "list", "--keyring-backend", "test", "--output", "json"}
	if *flagHome != "" {
		args = append(args, "--home", *flagHome)
	}
	cmd := exec.Command(*flagBin, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("keys list: %s\n%w", string(out), err)
	}

	var rows []keyRecord
	if err := json.Unmarshal(out, &rows); err == nil {
		return rows, nil
	}

	// Fallback shape used by some builds: {"keys":[...]}.
	var wrapped struct {
		Keys []keyRecord `json:"keys"`
	}
	if err := json.Unmarshal(out, &wrapped); err == nil && len(wrapped.Keys) > 0 {
		return wrapped.Keys, nil
	}

	return nil, fmt.Errorf("unexpected keys list json: %s", truncate(string(out), 300))
}

// deriveAddressFromMnemonic derives the bech32 address for a mnemonic using
// the appropriate coin type and key algorithm.
func deriveAddressFromMnemonic(mnemonic string, isLegacy bool) (string, error) {
	coinType := uint32(118)
	if !isLegacy {
		coinType = nonLegacyCoinType
	}

	if !isLegacy && useEthAlgoForNonLegacy() {
		privKey, err := deriveEthKey(mnemonic, coinType)
		if err != nil {
			return "", err
		}
		pubKey := privKey.PubKey().(*evmsecp256k1.PubKey)
		return sdk.AccAddress(pubKey.Address()).String(), nil
	}

	privKey, err := deriveKey(mnemonic, coinType)
	if err != nil {
		return "", err
	}
	pubKey := privKey.PubKey().(*secp256k1.PubKey)
	return sdk.AccAddress(pubKey.Address()).String(), nil
}

// importKey imports a mnemonic into the lumerad keyring under the given name.
// Legacy accounts use coin-type 118; non-legacy uses the detected runtime coin-type.
func importKey(name, mnemonic string, isLegacy bool) error {
	coinType := "118"
	if !isLegacy {
		coinType = nonLegacyCoinTypeStr
	}
	args := []string{"keys", "add", name,
		"--keyring-backend", "test",
		"--recover",
		"--coin-type", coinType,
	}
	if !isLegacy && useEthAlgoForNonLegacy() {
		args = append(args, "--algo", "eth_secp256k1")
	}
	if *flagHome != "" {
		args = append(args, "--home", *flagHome)
	}
	cmd := exec.Command(*flagBin, args...)
	cmd.Stdin = strings.NewReader(mnemonic + "\n")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("keys add --recover %s: %s\n%w", name, string(out), err)
	}
	return nil
}

// keyExists returns true if a key with the given name already exists in the keyring.
func keyExists(name string) bool {
	_, err := getAddress(name)
	return err == nil
}

// ensureAccount returns an AccountRecord for the given name. If the key already
// exists in the keyring (e.g. from a previous interrupted run), it reuses it.
// Otherwise it generates a new key and imports it into the keyring.
func ensureAccount(name string, isLegacy bool) (AccountRecord, error) {
	if addr, err := getAddress(name); err == nil {
		log.Printf("  key %s already in keyring (%s), reusing", name, addr)
		return AccountRecord{
			Name:     name,
			Address:  addr,
			IsLegacy: isLegacy,
		}, nil
	}
	rec, err := generateAccount(name, isLegacy)
	if err != nil {
		return AccountRecord{}, err
	}
	if err := importKey(name, rec.Mnemonic, isLegacy); err != nil {
		return AccountRecord{}, fmt.Errorf("import key %s: %w", name, err)
	}
	return rec, nil
}

// deleteKey removes a key from the lumerad keyring. Returns nil if the key
// does not exist.
func deleteKey(name string) error {
	args := []string{"keys", "delete", name,
		"--keyring-backend", "test",
		"--yes",
	}
	if *flagHome != "" {
		args = append(args, "--home", *flagHome)
	}
	cmd := exec.Command(*flagBin, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		low := strings.ToLower(string(out))
		if strings.Contains(low, "not found") || strings.Contains(low, "no such key") {
			return nil
		}
		return fmt.Errorf("keys delete %s: %s\n%w", name, string(out), err)
	}
	return nil
}

// getAddress returns the bech32 address for a key name in the test keyring.
func getAddress(name string) (string, error) {
	args := []string{"keys", "show", name, "--keyring-backend", "test", "--address"}
	if *flagHome != "" {
		args = append(args, "--home", *flagHome)
	}
	cmd := exec.Command(*flagBin, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("keys show %s: %s\n%w", name, string(out), err)
	}
	return strings.TrimSpace(string(out)), nil
}

// signStringWithPrivHex signs an arbitrary payload using a raw private key hex.
// Returns a base64-encoded signature.
func signStringWithPrivHex(privHex, payload string) (string, error) {
	privBz, err := hex.DecodeString(strings.TrimSpace(privHex))
	if err != nil {
		return "", fmt.Errorf("decode private key hex: %w", err)
	}
	if len(privBz) != 32 {
		return "", fmt.Errorf("unexpected private key length: %d", len(privBz))
	}
	privKey := &secp256k1.PrivKey{Key: privBz}
	sig, err := privKey.Sign([]byte(payload))
	if err != nil {
		return "", fmt.Errorf("sign payload: %w", err)
	}
	return base64.StdEncoding.EncodeToString(sig), nil
}
