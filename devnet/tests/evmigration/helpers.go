package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

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
	Address string `json:"address"`
}

type migrationEstimate struct {
	WouldSucceed       bool   `json:"would_succeed"`
	RejectionReason    string `json:"rejection_reason"`
	DelegationCount    int    `json:"delegation_count"`
	UnbondingCount     int    `json:"unbonding_count"`
	RedelegationCount  int    `json:"redelegation_count"`
	AuthzGrantCount    int    `json:"authz_grant_count"`
	FeegrantCount      int    `json:"feegrant_count"`
	ValDelegationCount int    `json:"val_delegation_count"`
	IsValidator        bool   `json:"is_validator"`
}

type migrationStats struct {
	TotalMigrated           int `json:"total_migrated"`
	TotalLegacy             int `json:"total_legacy"`
	TotalLegacyStaked       int `json:"total_legacy_staked"`
	TotalValidatorsMigrated int `json:"total_validators_migrated"`
	TotalValidatorsLegacy   int `json:"total_validators_legacy"`
}

var (
	nonLegacyCoinType    uint32 = 60
	nonLegacyCoinTypeStr string = "60"
)

func useEthAlgoForNonLegacy() bool {
	return nonLegacyCoinType == 60
}

func ensureEVMMigrationRuntime(mode string) {
	coinType, version, err := detectNonLegacyCoinType()
	if err != nil {
		log.Fatalf("%s requires EVM-enabled lumerad >= %s, but version detection failed: %v",
			mode, *flagEVMCutoverVer, err)
	}
	if coinType != 60 {
		log.Fatalf("%s requires EVM-enabled lumerad >= %s; detected %s (non-legacy coin-type %d). Migration is not possible before EVM upgrade",
			mode, *flagEVMCutoverVer, version, coinType)
	}
	log.Printf("%s runtime check passed: detected lumerad %s (non-legacy coin-type 60)", mode, version)
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
		log.Printf("WARN: detect lumerad version failed (%v); using non-legacy coin-type %d for mode=%s", err, coinType, *flagMode)
	} else {
		log.Printf("detected lumerad version %s; using non-legacy coin-type %d", ver, coinType)
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
	if m := regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)$`).FindStringSubmatch(trimmed); len(m) == 4 {
		return fmt.Sprintf("v%s.%s.%s", m[1], m[2], m[3]), true
	}

	// Prefer explicit version labels in structured long output.
	labelled := regexp.MustCompile(`(?mi)^\s*version\s*[:=]\s*v?(\d+)\.(\d+)\.(\d+)\s*$`)
	if m := labelled.FindStringSubmatch(s); len(m) == 4 {
		return fmt.Sprintf("v%s.%s.%s", m[1], m[2], m[3]), true
	}

	// Fallback: find first semantic version on non-dependency lines.
	// This avoids matching build deps like "- cel.dev/expr@v0.24.0".
	anySemver := regexp.MustCompile(`v?(\d+)\.(\d+)\.(\d+)`)
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "- ") || strings.Contains(line, "@v") {
			continue
		}
		if m := anySemver.FindStringSubmatch(line); len(m) == 4 {
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

// --- CLI helpers ---

func run(args ...string) (string, error) {
	out, err := runWithFlags(true, true, args...)
	if err == nil {
		return out, nil
	}
	low := strings.ToLower(out)
	if strings.Contains(low, "unknown flag: --node") || strings.Contains(low, "unknown flag: --keyring-backend") {
		tryVariants := [][2]bool{
			{false, true},
			{true, false},
			{false, false},
		}
		for _, v := range tryVariants {
			out2, err2 := runWithFlags(v[0], v[1], args...)
			if err2 == nil {
				return out2, nil
			}
			low2 := strings.ToLower(out2)
			if !strings.Contains(low2, "unknown flag: --node") && !strings.Contains(low2, "unknown flag: --keyring-backend") {
				return out2, err2
			}
		}
	}
	return out, err
}

func runWithFlags(includeNode bool, includeKeyring bool, args ...string) (string, error) {
	baseArgs := []string{
		"--chain-id", *flagChainID,
		"--output", "json",
	}
	if includeKeyring {
		baseArgs = append(baseArgs, "--keyring-backend", "test")
	}
	if includeNode {
		baseArgs = append([]string{"--node", *flagRPC}, baseArgs...)
	}
	if *flagHome != "" {
		baseArgs = append(baseArgs, "--home", *flagHome)
	}
	allArgs := make([]string, 0, len(args)+len(baseArgs))
	allArgs = append(allArgs, args...)
	allArgs = append(allArgs, baseArgs...)
	cmd := exec.Command(*flagBin, allArgs...)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func runTx(args ...string) (string, error) {
	out, txHash, err := runTxWithMode(args, "sync")
	if err != nil {
		return out, err
	}
	// Wait for tx inclusion before returning so the next tx sees updated state.
	if txHash != "" {
		if err := waitTx(txHash); err != nil {
			log.Printf("WARN: wait-tx %s: %v", txHash, err)
		}
		code, rawLog, err := queryTxCode(txHash)
		if err != nil {
			return out, fmt.Errorf("tx %s result query failed: %w", txHash, err)
		}
		if code != 0 {
			return out, fmt.Errorf("tx deliver failed code=%d raw_log=%s", code, rawLog)
		}
	}
	return out, nil
}

func runTxNoWaitWithAccountSequence(accountNumber, sequence uint64, args ...string) (string, string, error) {
	txArgs := append([]string{}, args...)
	txArgs = append(txArgs,
		"--offline",
		"--account-number", strconv.FormatUint(accountNumber, 10),
		"--sequence", strconv.FormatUint(sequence, 10),
	)
	return runTxWithMode(txArgs, "sync")
}

func runTxWithMode(args []string, broadcastMode string) (string, string, error) {
	txArgs := append([]string{}, args...)
	txArgs = append(txArgs,
		"--gas", *flagGas,
		"--gas-prices", *flagGasPrices,
		"--yes",
		"--broadcast-mode", broadcastMode,
	)
	if *flagGas == "auto" {
		txArgs = append(txArgs, "--gas-adjustment", *flagGasAdj)
	}

	out, err := run(txArgs...)
	if err != nil {
		return out, "", fmt.Errorf("tx failed: %s\n%w", out, err)
	}

	// Check CheckTx response code from sync broadcast.
	var txResp struct {
		Code   uint32 `json:"code"`
		RawLog string `json:"raw_log"`
		TxHash string `json:"txhash"`
	}
	if err := json.Unmarshal([]byte(out), &txResp); err == nil {
		if txResp.Code != 0 {
			return out, txResp.TxHash, fmt.Errorf("tx rejected code=%d raw_log=%s", txResp.Code, txResp.RawLog)
		}
		return out, txResp.TxHash, nil
	}

	return out, "", nil
}

// waitTx uses `lumerad query wait-tx` to block until the tx is included.
func waitTx(txHash string) error {
	args := []string{
		"query", "wait-tx", txHash,
		"--timeout", "30s",
		"--node", *flagRPC,
	}
	if *flagHome != "" {
		args = append(args, "--home", *flagHome)
	}
	cmd := exec.Command(*flagBin, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("wait-tx: %s\n%w", truncate(string(out), 300), err)
	}
	return nil
}

func queryTxCode(txHash string) (uint32, string, error) {
	out, err := run("query", "tx", txHash)
	if err != nil {
		return 0, "", fmt.Errorf("query tx: %s\n%w", truncate(out, 300), err)
	}

	// Cosmos SDK v0.47 style.
	var direct struct {
		Code   uint32 `json:"code"`
		RawLog string `json:"raw_log"`
	}
	if err := json.Unmarshal([]byte(out), &direct); err == nil {
		// If fields are present at top-level, use them.
		if strings.Contains(out, "\"code\"") {
			return direct.Code, direct.RawLog, nil
		}
	}

	// Cosmos SDK v0.50+ style.
	var wrapped struct {
		TxResponse struct {
			Code   uint32 `json:"code"`
			RawLog string `json:"raw_log"`
		} `json:"tx_response"`
	}
	if err := json.Unmarshal([]byte(out), &wrapped); err != nil {
		return 0, "", fmt.Errorf("parse tx query response: %s\n%w", truncate(out, 300), err)
	}
	return wrapped.TxResponse.Code, wrapped.TxResponse.RawLog, nil
}

// waitForNextBlock waits until the chain advances at least one block from the
// current height. This is used as a simpler alternative to tx-hash polling.
func waitForNextBlock(timeout time.Duration) error {
	startHeight, err := queryLatestHeight()
	if err != nil {
		// If we can't query height, just sleep a conservative amount.
		time.Sleep(7 * time.Second)
		return nil
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		time.Sleep(time.Second)
		h, err := queryLatestHeight()
		if err == nil && h > startHeight {
			return nil
		}
	}
	return errors.New("timeout waiting for next block")
}

func queryLatestHeight() (int64, error) {
	out, err := run("query", "block")
	if err != nil {
		// Try alternative command for newer SDK.
		out, err = run("status")
		if err != nil {
			return 0, err
		}
	}
	// Try multiple JSON shapes.
	var block struct {
		Block *struct {
			Header struct {
				Height string `json:"height"`
			} `json:"header"`
		} `json:"block"`
		SyncInfo *struct {
			LatestBlockHeight string `json:"latest_block_height"`
		} `json:"sync_info"`
		SdkBlock *struct {
			Header struct {
				Height string `json:"height"`
			} `json:"header"`
		} `json:"sdk_block"`
	}
	if err := json.Unmarshal([]byte(out), &block); err != nil {
		return 0, err
	}
	var heightStr string
	if block.Block != nil {
		heightStr = block.Block.Header.Height
	} else if block.SdkBlock != nil {
		heightStr = block.SdkBlock.Header.Height
	} else if block.SyncInfo != nil {
		heightStr = block.SyncInfo.LatestBlockHeight
	}
	if heightStr == "" {
		return 0, fmt.Errorf("no height in response: %s", truncate(out, 200))
	}
	var h int64
	fmt.Sscanf(heightStr, "%d", &h)
	return h, nil
}

func getValidators() ([]string, error) {
	out, err := run("query", "staking", "validators")
	if err != nil {
		return nil, fmt.Errorf("query validators: %s\n%w", out, err)
	}

	var result struct {
		Validators []struct {
			OperatorAddress string `json:"operator_address"`
		} `json:"validators"`
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		return nil, fmt.Errorf("parse validators: %w", err)
	}

	var addrs []string
	for _, v := range result.Validators {
		addrs = append(addrs, v.OperatorAddress)
	}
	return addrs, nil
}

func queryMigrationEstimate(addr string) (migrationEstimate, error) {
	out, err := run("query", "evmigration", "migration-estimate", addr)
	if err != nil {
		return migrationEstimate{}, fmt.Errorf("query migration-estimate: %s\n%w", out, err)
	}
	var estimate migrationEstimate
	if err := json.Unmarshal([]byte(out), &estimate); err != nil {
		return migrationEstimate{}, fmt.Errorf("parse migration-estimate: %s\n%w", truncate(out, 300), err)
	}
	return estimate, nil
}

func queryAccountNumberAndSequence(addr string) (accountNumber uint64, sequence uint64, err error) {
	out, err := run("query", "auth", "account", addr)
	if err != nil {
		return 0, 0, fmt.Errorf("query auth account: %s\n%w", out, err)
	}

	var resp struct {
		Account struct {
			AccountNumber string `json:"account_number"`
			Sequence      string `json:"sequence"`
			Value         *struct {
				AccountNumber string `json:"account_number"`
				Sequence      string `json:"sequence"`
			} `json:"value"`
			BaseAccount *struct {
				AccountNumber string `json:"account_number"`
				Sequence      string `json:"sequence"`
			} `json:"base_account"`
		} `json:"account"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return 0, 0, fmt.Errorf("parse auth account: %s\n%w", truncate(out, 300), err)
	}

	accNumStr := resp.Account.AccountNumber
	seqStr := resp.Account.Sequence
	if resp.Account.Value != nil {
		if resp.Account.Value.AccountNumber != "" {
			accNumStr = resp.Account.Value.AccountNumber
		}
		if resp.Account.Value.Sequence != "" {
			seqStr = resp.Account.Value.Sequence
		}
	}
	if resp.Account.BaseAccount != nil {
		if resp.Account.BaseAccount.AccountNumber != "" {
			accNumStr = resp.Account.BaseAccount.AccountNumber
		}
		if resp.Account.BaseAccount.Sequence != "" {
			seqStr = resp.Account.BaseAccount.Sequence
		}
	}
	if accNumStr == "" || seqStr == "" {
		return 0, 0, fmt.Errorf("account_number/sequence missing in auth account response: %s", truncate(out, 300))
	}

	accountNumber, err = strconv.ParseUint(accNumStr, 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("parse account_number %q: %w", accNumStr, err)
	}
	sequence, err = strconv.ParseUint(seqStr, 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("parse sequence %q: %w", seqStr, err)
	}
	return accountNumber, sequence, nil
}

func queryMigrationStats() (migrationStats, error) {
	out, err := run("query", "evmigration", "migration-stats")
	if err != nil {
		return migrationStats{}, fmt.Errorf("query migration-stats: %s\n%w", out, err)
	}
	var stats migrationStats
	if err := json.Unmarshal([]byte(out), &stats); err != nil {
		return migrationStats{}, fmt.Errorf("parse migration-stats: %s\n%w", truncate(out, 300), err)
	}
	return stats, nil
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
func signMigrationMessage(mnemonic, legacyAddr, newAddr string) (string, error) {
	privKey, err := deriveKey(mnemonic, 118)
	if err != nil {
		return "", fmt.Errorf("derive legacy key: %w", err)
	}

	msg := fmt.Sprintf("lumera-evm-migration:%s:%s", legacyAddr, newAddr)
	hash := sha256.Sum256([]byte(msg))
	sig, err := privKey.Sign(hash[:])
	if err != nil {
		return "", fmt.Errorf("sign: %w", err)
	}
	return base64.StdEncoding.EncodeToString(sig), nil
}

func signMigrationMessageWithPrivHex(privHex, legacyAddr, newAddr string) (sigB64 string, pubKeyB64 string, err error) {
	privBz, err := hex.DecodeString(strings.TrimSpace(privHex))
	if err != nil {
		return "", "", fmt.Errorf("decode private key hex: %w", err)
	}
	if len(privBz) != 32 {
		return "", "", fmt.Errorf("unexpected private key length: %d", len(privBz))
	}
	privKey := &secp256k1.PrivKey{Key: privBz}
	pubKey := privKey.PubKey().(*secp256k1.PubKey)

	msg := fmt.Sprintf("lumera-evm-migration:%s:%s", legacyAddr, newAddr)
	hash := sha256.Sum256([]byte(msg))
	sig, err := privKey.Sign(hash[:])
	if err != nil {
		return "", "", fmt.Errorf("sign: %w", err)
	}

	return base64.StdEncoding.EncodeToString(sig), base64.StdEncoding.EncodeToString(pubKey.Key), nil
}

// --- File I/O ---

func saveAccounts(path string, af *AccountsFile) {
	data, err := json.MarshalIndent(af, "", "  ")
	if err != nil {
		log.Fatalf("marshal accounts: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		log.Fatalf("write %s: %v", path, err)
	}
}

func loadAccounts(path string) *AccountsFile {
	data, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("read %s: %v", path, err)
	}
	var af AccountsFile
	if err := json.Unmarshal(data, &af); err != nil {
		log.Fatalf("parse %s: %v", path, err)
	}
	return &af
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func queryDelegationCount(addr string) (int, error) {
	out, err := run("query", "staking", "delegations", addr)
	if err != nil {
		return 0, fmt.Errorf("query delegations: %s\n%w", out, err)
	}
	var resp struct {
		DelegationResponses []json.RawMessage `json:"delegation_responses"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return 0, err
	}
	return len(resp.DelegationResponses), nil
}

func queryDelegationToValidatorCount(addr string, valoper string) (int, error) {
	out, err := run("query", "staking", "delegations", addr)
	if err != nil {
		return 0, fmt.Errorf("query delegations: %s\n%w", out, err)
	}
	var resp struct {
		DelegationResponses []struct {
			Delegation struct {
				ValidatorAddress string `json:"validator_address"`
			} `json:"delegation"`
		} `json:"delegation_responses"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return 0, err
	}
	n := 0
	for _, d := range resp.DelegationResponses {
		if d.Delegation.ValidatorAddress == valoper {
			n++
		}
	}
	return n, nil
}

func queryUnbondingCount(addr string) (int, error) {
	out, err := run("query", "staking", "unbonding-delegations", addr)
	if err != nil {
		return 0, fmt.Errorf("query unbonding delegations: %s\n%w", out, err)
	}
	var resp struct {
		UnbondingResponses []json.RawMessage `json:"unbonding_responses"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return 0, err
	}
	return len(resp.UnbondingResponses), nil
}

func queryUnbondingFromValidatorCount(addr string, valoper string) (int, error) {
	out, err := run("query", "staking", "unbonding-delegations", addr)
	if err != nil {
		return 0, fmt.Errorf("query unbonding delegations: %s\n%w", out, err)
	}
	var resp struct {
		UnbondingResponses []struct {
			ValidatorAddress string `json:"validator_address"`
		} `json:"unbonding_responses"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return 0, err
	}
	n := 0
	for _, u := range resp.UnbondingResponses {
		if u.ValidatorAddress == valoper {
			n++
		}
	}
	return n, nil
}

func queryRedelegationCount(addr string, srcVal string, dstVal string) (int, error) {
	out, err := run("query", "staking", "redelegation", addr, srcVal, dstVal)
	if err != nil {
		low := strings.ToLower(out)
		if strings.Contains(low, "not found") || strings.Contains(low, "no redelegation") {
			return 0, nil
		}
		return 0, fmt.Errorf("query redelegation: %s\n%w", out, err)
	}
	var resp struct {
		RedelegationResponses []json.RawMessage `json:"redelegation_responses"`
		Redelegation          json.RawMessage   `json:"redelegation"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return 0, err
	}
	if len(resp.RedelegationResponses) > 0 {
		return len(resp.RedelegationResponses), nil
	}
	if len(resp.Redelegation) > 0 && string(resp.Redelegation) != "null" {
		return 1, nil
	}
	return 0, nil
}

func queryAnyRedelegationCount(addr string, validators []string) (int, error) {
	total := 0
	var firstErr error
	for _, src := range validators {
		for _, dst := range validators {
			if src == dst {
				continue
			}
			n, err := queryRedelegationCount(addr, src, dst)
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			total += n
		}
	}
	if total > 0 {
		return total, nil
	}
	if firstErr != nil {
		return 0, firstErr
	}
	return 0, nil
}

func queryValidatorDelegationsToCount(valoper string) (int, error) {
	out, err := run("query", "staking", "delegations-to", valoper)
	if err != nil {
		return 0, fmt.Errorf("query delegations-to: %s\n%w", out, err)
	}
	var resp struct {
		DelegationResponses []json.RawMessage `json:"delegation_responses"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return 0, err
	}
	return len(resp.DelegationResponses), nil
}

func queryWithdrawAddress(addr string) (string, error) {
	out, err := run("query", "distribution", "withdraw-addr", addr)
	if err != nil {
		out2, err2 := run("query", "distribution", "delegator-withdraw-address", "--delegator-address", addr)
		if err2 == nil {
			out, err = out2, nil
		}
	}
	if err != nil {
		return "", fmt.Errorf("query withdraw-addr: %s\n%w", out, err)
	}
	var resp struct {
		WithdrawAddress string `json:"withdraw_address"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return "", err
	}
	return resp.WithdrawAddress, nil
}

func queryAuthzGrantExists(granter, grantee string) (bool, error) {
	out, err := run("query", "authz", "grants", granter, grantee, "/cosmos.bank.v1beta1.MsgSend")
	if err != nil {
		low := strings.ToLower(out)
		if strings.Contains(low, "not found") || strings.Contains(low, "no authorization") {
			return false, nil
		}
		return false, fmt.Errorf("query authz grants: %s\n%w", out, err)
	}
	var resp struct {
		Grants []json.RawMessage `json:"grants"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return false, err
	}
	return len(resp.Grants) > 0, nil
}

func queryBalance(addr string) (int64, error) {
	out, err := run("query", "bank", "balance", addr, "ulume")
	if err != nil {
		// Try alternative: some SDK versions use "balances" with --denom.
		out, err = run("query", "bank", "balances", addr, "--denom", "ulume")
		if err != nil {
			return 0, fmt.Errorf("query balance: %s\n%w", truncate(out, 300), err)
		}
	}
	var resp struct {
		Balance *struct {
			Amount string `json:"amount"`
		} `json:"balance"`
		Amount string `json:"amount"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return 0, fmt.Errorf("parse balance: %s\n%w", truncate(out, 300), err)
	}
	amtStr := resp.Amount
	if resp.Balance != nil && resp.Balance.Amount != "" {
		amtStr = resp.Balance.Amount
	}
	if amtStr == "" {
		return 0, nil
	}
	amt, err := strconv.ParseInt(amtStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse amount %q: %w", amtStr, err)
	}
	return amt, nil
}

func queryFeegrantAllowanceExists(granter, grantee string) (bool, error) {
	out, err := run("query", "feegrant", "grant", granter, grantee)
	if err != nil {
		low := strings.ToLower(out)
		if strings.Contains(low, "not found") || strings.Contains(low, "no allowance") || strings.Contains(low, "fee-grant not found") {
			return false, nil
		}
		return false, fmt.Errorf("query feegrant grant: %s\n%w", out, err)
	}
	return true, nil
}

// queryClaimRecord returns the claim record for the given Pastel (old) address.
// Returns (claimed, destAddress, vestedTier, err). If the record does not exist, returns an error.
func queryClaimRecord(oldAddress string) (claimed bool, destAddress string, vestedTier uint32, err error) {
	out, err := run("query", "claim", "claim-record", oldAddress)
	if err != nil {
		return false, "", 0, fmt.Errorf("query claim record: %s\n%w", truncate(out, 300), err)
	}
	var resp struct {
		Record struct {
			Claimed      bool   `json:"claimed"`
			DestAddress  string `json:"destAddress"`
			NewAddress   string `json:"newAddress"`
			VestedTier   uint32 `json:"vestedTier"`
			VestedTierSn uint32 `json:"vested_tier"`
		} `json:"record"`
		ClaimRecordCamel struct {
			Claimed      bool   `json:"claimed"`
			DestAddress  string `json:"destAddress"`
			NewAddress   string `json:"newAddress"`
			VestedTier   uint32 `json:"vestedTier"`
			VestedTierSn uint32 `json:"vested_tier"`
		} `json:"claimRecord"`
		ClaimRecord struct {
			Claimed      bool   `json:"claimed"`
			DestAddress  string `json:"dest_address"`
			NewAddress   string `json:"new_address"`
			VestedTier   uint32 `json:"vested_tier"`
			VestedTierCm uint32 `json:"vestedTier"`
		} `json:"claim_record"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return false, "", 0, fmt.Errorf("parse claim record: %s\n%w", truncate(out, 300), err)
	}

	claimed = resp.Record.Claimed || resp.ClaimRecord.Claimed || resp.ClaimRecordCamel.Claimed
	destAddress = resp.Record.DestAddress
	if destAddress == "" {
		destAddress = resp.Record.NewAddress
	}
	if destAddress == "" {
		destAddress = resp.ClaimRecord.DestAddress
	}
	if destAddress == "" {
		destAddress = resp.ClaimRecord.NewAddress
	}
	if destAddress == "" {
		destAddress = resp.ClaimRecordCamel.DestAddress
	}
	if destAddress == "" {
		destAddress = resp.ClaimRecordCamel.NewAddress
	}
	vestedTier = resp.Record.VestedTier
	if vestedTier == 0 {
		vestedTier = resp.Record.VestedTierSn
	}
	if vestedTier == 0 {
		vestedTier = resp.ClaimRecord.VestedTier
	}
	if vestedTier == 0 {
		vestedTier = resp.ClaimRecord.VestedTierCm
	}
	if vestedTier == 0 {
		vestedTier = resp.ClaimRecordCamel.VestedTier
	}
	if vestedTier == 0 {
		vestedTier = resp.ClaimRecordCamel.VestedTierSn
	}
	return claimed, destAddress, vestedTier, nil
}

func queryHasAnyBalance(addr string) (bool, error) {
	out, err := run("query", "bank", "balances", addr)
	if err != nil {
		return false, fmt.Errorf("query bank balances: %s\n%w", out, err)
	}
	var resp struct {
		Balances []json.RawMessage `json:"balances"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return false, err
	}
	return len(resp.Balances) > 0, nil
}

// queryClaimedCountByTier returns number of claimed records for a delayed vesting tier.
func queryClaimedCountByTier(tier uint32) (int, error) {
	out, err := run("query", "claim", "list-claimed", fmt.Sprintf("%d", tier))
	if err != nil {
		return 0, fmt.Errorf("query list-claimed tier=%d: %s\n%w", tier, truncate(out, 300), err)
	}
	var resp struct {
		Claims []json.RawMessage `json:"claims"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return 0, fmt.Errorf("parse list-claimed tier=%d: %s\n%w", tier, truncate(out, 300), err)
	}
	return len(resp.Claims), nil
}

func queryHasAnyDelayedClaim() (bool, error) {
	for _, tier := range []uint32{1, 2, 3} {
		n, err := queryClaimedCountByTier(tier)
		if err != nil {
			return false, err
		}
		if n > 0 {
			return true, nil
		}
	}
	return false, nil
}
