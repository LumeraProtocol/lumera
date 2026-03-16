package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

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

type keyRecord struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Address string `json:"address"`
	PubKey  string `json:"pubkey"`
}

var (
	nonLegacyCoinType    uint32 = 60
	nonLegacyCoinTypeStr string = "60"
)

func useEthAlgoForNonLegacy() bool {
	return nonLegacyCoinType == 60
}

func prepareRuntimeAllowed(coinType uint32) bool {
	return coinType == 118
}

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

func detectNonLegacyCoinType() (uint32, string, error) {
	version, err := detectLumeradVersion()
	if err != nil {
		return 0, "", err
	}
	cmp, err := compareSemver(version, *flagEVMCutoverVer)
	if err != nil {
		return 0, version, err
	}
	if cmp >= 0 {
		return 60, version, nil
	}
	return 118, version, nil
}

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
		if ver, ok := extractSemver(string(out)); ok {
			return ver, nil
		}
		lastOut = out
	}
	if lastErr != nil {
		return "", fmt.Errorf("run version command failed: %w", lastErr)
	}
	return "", fmt.Errorf("could not parse semantic version from: %s", truncate(string(lastOut), 200))
}

func extractSemver(s string) (string, bool) {
	// Best case: plain `lumerad version` outputs just "1.11.0" (or with leading v).
	trimmed := strings.TrimSpace(s)
	if m := semverExact.FindStringSubmatch(trimmed); len(m) == 4 {
		return fmt.Sprintf("v%s.%s.%s", m[1], m[2], m[3]), true
	}

	// Prefer explicit "version:" label in structured long output.
	// Uses a word-boundary anchor so "cosmos_sdk_version:" is not matched.
	if m := semverLabelled.FindStringSubmatch(s); len(m) == 4 {
		return fmt.Sprintf("v%s.%s.%s", m[1], m[2], m[3]), true
	}

	// Fallback: find first semantic version on non-dependency lines.
	// Skip build deps ("- ...@v...") and SDK version lines to avoid
	// misidentifying the Cosmos SDK version as the app version.
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "- ") || strings.Contains(line, "@v") {
			continue
		}
		if strings.Contains(line, "sdk_version") {
			continue
		}
		if m := semverAny.FindStringSubmatch(line); len(m) == 4 {
			return fmt.Sprintf("v%s.%s.%s", m[1], m[2], m[3]), true
		}
	}
	return "", false
}

func compareSemver(a, b string) (int, error) {
	parse := func(v string) ([3]int, error) {
		s, ok := extractSemver(v)
		if !ok {
			return [3]int{}, fmt.Errorf("invalid semver %q", v)
		}
		s = strings.TrimPrefix(s, "v")
		parts := strings.Split(s, ".")
		if len(parts) != 3 {
			return [3]int{}, fmt.Errorf("invalid semver %q", v)
		}
		maj, err := strconv.Atoi(parts[0])
		if err != nil {
			return [3]int{}, err
		}
		min, err := strconv.Atoi(parts[1])
		if err != nil {
			return [3]int{}, err
		}
		pat, err := strconv.Atoi(parts[2])
		if err != nil {
			return [3]int{}, err
		}
		return [3]int{maj, min, pat}, nil
	}

	av, err := parse(a)
	if err != nil {
		return 0, err
	}
	bv, err := parse(b)
	if err != nil {
		return 0, err
	}

	for i := 0; i < 3; i++ {
		if av[i] < bv[i] {
			return -1, nil
		}
		if av[i] > bv[i] {
			return 1, nil
		}
	}
	return 0, nil
}

// detectFunder picks a funder from the local keyring by finding the first key
// whose address matches an active validator's operator address (i.e. a genesis
// validator account that is guaranteed to have funds).
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
		// Fall back to first key if we can't query validators.
		return keys[0].Name, nil
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
		if _, ok := valAccAddrs[k.Address]; ok {
			return k.Name, nil
		}
	}

	// No validator key found; fall back to first key.
	return keys[0].Name, nil
}

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

func exportPrivateKeyHex(name string) (string, error) {
	args := []string{
		"keys", "export", name,
		"--unsafe", "--unarmored-hex", "--yes",
		"--keyring-backend", "test",
	}
	if *flagHome != "" {
		args = append(args, "--home", *flagHome)
	}
	cmd := exec.Command(*flagBin, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("keys export %s: %s\n%w", name, string(out), err)
	}
	return strings.TrimSpace(string(out)), nil
}

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

// --- Signing ---

// signMigrationMessage creates a legacy signature for the migration message.
func signMigrationMessage(kind, mnemonic, legacyAddr, newAddr string) (string, error) {
	privKey, err := deriveKey(mnemonic, 118)
	if err != nil {
		return "", fmt.Errorf("derive legacy key: %w", err)
	}

	msg := fmt.Sprintf("lumera-evm-migration:%s:%s:%s", kind, legacyAddr, newAddr)
	hash := sha256.Sum256([]byte(msg))
	sig, err := privKey.Sign(hash[:])
	if err != nil {
		return "", fmt.Errorf("sign: %w", err)
	}
	return base64.StdEncoding.EncodeToString(sig), nil
}

func signMigrationMessageWithPrivHex(kind, privHex, legacyAddr, newAddr string) (sigB64 string, pubKeyB64 string, err error) {
	privBz, err := hex.DecodeString(strings.TrimSpace(privHex))
	if err != nil {
		return "", "", fmt.Errorf("decode private key hex: %w", err)
	}
	if len(privBz) != 32 {
		return "", "", fmt.Errorf("unexpected private key length: %d", len(privBz))
	}
	privKey := &secp256k1.PrivKey{Key: privBz}
	pubKey := privKey.PubKey().(*secp256k1.PubKey)

	msg := fmt.Sprintf("lumera-evm-migration:%s:%s:%s", kind, legacyAddr, newAddr)
	hash := sha256.Sum256([]byte(msg))
	sig, err := privKey.Sign(hash[:])
	if err != nil {
		return "", "", fmt.Errorf("sign: %w", err)
	}

	return base64.StdEncoding.EncodeToString(sig), base64.StdEncoding.EncodeToString(pubKey.Key), nil
}

func signStringWithLegacyKey(mnemonic, payload string) (string, error) {
	privKey, err := deriveKey(mnemonic, 118)
	if err != nil {
		return "", fmt.Errorf("derive legacy key: %w", err)
	}
	sig, err := privKey.Sign([]byte(payload))
	if err != nil {
		return "", fmt.Errorf("sign payload: %w", err)
	}
	return base64.StdEncoding.EncodeToString(sig), nil
}

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

// --- Compiled regexps for semver parsing ---

var (
	semverExact    = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)$`)
	semverLabelled = regexp.MustCompile(`(?mi)^\s*version\s*[:=]\s*v?(\d+)\.(\d+)\.(\d+)\s*$`)
	semverAny      = regexp.MustCompile(`v?(\d+)\.(\d+)\.(\d+)`)
)
