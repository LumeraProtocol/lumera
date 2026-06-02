package common

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// ChainCLI wraps the lumerad command-line interface with explicit connection
// configuration instead of package globals, so multiple tools can share it.
// It covers the read-only queries and bank/keyring operations the activity
// generator's funding phase needs; it intentionally has no migration-specific
// gas logic.
type ChainCLI struct {
	Bin            string
	ChainID        string
	RPC            string
	Home           string
	KeyringBackend string

	Gas           string // e.g. "auto" or a fixed limit
	GasPrices     string // e.g. "0.025ulume"
	GasAdjustment string // used only when Gas == "auto"
}

// Run executes a query/command with standard node+chain flags, retrying with
// reduced flag sets if the subcommand rejects --node/--keyring-backend (some
// pure keyring commands do).
func (c *ChainCLI) Run(args ...string) (string, error) {
	out, err := c.runWithFlags(true, true, args...)
	if err == nil {
		return out, nil
	}
	low := strings.ToLower(out)
	if strings.Contains(low, "unknown flag: --node") || strings.Contains(low, "unknown flag: --keyring-backend") {
		for _, v := range [][2]bool{{false, true}, {true, false}, {false, false}} {
			out2, err2 := c.runWithFlags(v[0], v[1], args...)
			low2 := strings.ToLower(out2)
			if err2 == nil {
				return out2, nil
			}
			if !strings.Contains(low2, "unknown flag: --node") && !strings.Contains(low2, "unknown flag: --keyring-backend") {
				return out2, err2
			}
		}
	}
	return out, err
}

func (c *ChainCLI) runWithFlags(includeNode, includeKeyring bool, args ...string) (string, error) {
	base := []string{"--chain-id", c.ChainID, "--output", "json"}
	if includeKeyring {
		base = append(base, "--keyring-backend", c.keyringBackend())
	}
	if includeNode {
		base = append([]string{"--node", c.RPC}, base...)
	}
	if c.Home != "" {
		base = append(base, "--home", c.Home)
	}
	all := append(append([]string{}, args...), base...)
	out, err := exec.Command(c.Bin, all...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func (c *ChainCLI) keyringBackend() string {
	if c.KeyringBackend == "" {
		return "test"
	}
	return c.KeyringBackend
}

// LatestHeight returns the current chain height.
func (c *ChainCLI) LatestHeight() (int64, error) {
	out, err := c.Run("query", "block")
	if err != nil {
		if out, err = c.Run("status"); err != nil {
			return 0, err
		}
	}
	var resp struct {
		Block *struct {
			Header struct {
				Height string `json:"height"`
			} `json:"header"`
		} `json:"block"`
		SdkBlock *struct {
			Header struct {
				Height string `json:"height"`
			} `json:"header"`
		} `json:"sdk_block"`
		SyncInfo *struct {
			LatestBlockHeight string `json:"latest_block_height"`
		} `json:"sync_info"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return 0, fmt.Errorf("parse height: %s: %w", truncate(out, 200), err)
	}
	var heightStr string
	switch {
	case resp.Block != nil:
		heightStr = resp.Block.Header.Height
	case resp.SdkBlock != nil:
		heightStr = resp.SdkBlock.Header.Height
	case resp.SyncInfo != nil:
		heightStr = resp.SyncInfo.LatestBlockHeight
	}
	if heightStr == "" {
		return 0, fmt.Errorf("no height in response: %s", truncate(out, 200))
	}
	return strconv.ParseInt(heightStr, 10, 64)
}

// WaitForNextBlock blocks until the chain advances at least one block or the
// timeout elapses.
func (c *ChainCLI) WaitForNextBlock(timeout time.Duration) error {
	start, err := c.LatestHeight()
	if err != nil {
		time.Sleep(5 * time.Second)
		return nil
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		time.Sleep(time.Second)
		if h, err := c.LatestHeight(); err == nil && h > start {
			return nil
		}
	}
	return errors.New("timeout waiting for next block")
}

// Validators returns all validator operator addresses.
func (c *ChainCLI) Validators() ([]string, error) {
	out, err := c.Run("query", "staking", "validators")
	if err != nil {
		return nil, fmt.Errorf("query validators: %s: %w", truncate(out, 200), err)
	}
	return ParseValidatorAddresses([]byte(out))
}

// AccountNumberAndSequence returns the on-chain account number and sequence.
func (c *ChainCLI) AccountNumberAndSequence(addr string) (uint64, uint64, error) {
	out, err := c.Run("query", "auth", "account", addr)
	if err != nil {
		return 0, 0, fmt.Errorf("query auth account: %s: %w", truncate(out, 200), err)
	}
	return ParseAuthAccountNumberAndSequence(out)
}

// Balance returns the ulume bank balance for an address.
func (c *ChainCLI) Balance(addr string) (int64, error) {
	out, err := c.Run("query", "bank", "balance", addr, ChainDenom)
	if err != nil {
		if out, err = c.Run("query", "bank", "balances", addr, "--denom", ChainDenom); err != nil {
			return 0, fmt.Errorf("query balance: %s: %w", truncate(out, 200), err)
		}
	}
	return ParseBankBalance(out)
}

// ShowAddress returns the bech32 address for a keyring key name.
func (c *ChainCLI) ShowAddress(name string) (string, error) {
	args := append([]string{"keys", "show", name, "--keyring-backend", c.keyringBackend(), "--address"}, c.homeArgs()...)
	out, err := exec.Command(c.Bin, args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("keys show %s: %s: %w", name, strings.TrimSpace(string(out)), err)
	}
	return strings.TrimSpace(string(out)), nil
}

// GeneratedKey is the result of creating a new keyring key.
type GeneratedKey struct {
	Name     string `json:"name"`
	Address  string `json:"address"`
	PubKey   string `json:"pubkey"`
	Mnemonic string `json:"mnemonic"`
}

// AddKey creates a new key in the keyring using the legacy Cosmos key style.
// New callers that know the runtime style should prefer AddKeyWithStyle.
func (c *ChainCLI) AddKey(name string) (GeneratedKey, error) {
	return c.AddKeyWithStyle(name, KeyStyleLegacy)
}

// AddKeyWithStyle creates a new key in the keyring using explicit coin-type
// and algorithm flags so generated accounts match the detected chain runtime.
func (c *ChainCLI) AddKeyWithStyle(name string, style KeyStyle) (GeneratedKey, error) {
	// Pass --algo explicitly for BOTH styles: an EVM-enabled binary defaults its
	// signing algo to eth_secp256k1, so omitting --algo for a legacy (coin-118)
	// account would wrongly derive an eth_secp256k1 address on that binary.
	args := []string{
		"keys", "add", name,
		"--keyring-backend", c.keyringBackend(),
		"--output", "json",
		"--coin-type", strconv.FormatUint(uint64(style.CoinType), 10),
		"--algo", style.Algo,
	}
	args = append(args, c.homeArgs()...)
	out, err := exec.Command(c.Bin, args...).CombinedOutput()
	if err != nil {
		return GeneratedKey{}, fmt.Errorf("keys add %s: %s: %w", name, strings.TrimSpace(string(out)), err)
	}
	payload, ok := ExtractJSONPayload(string(out))
	if !ok {
		return GeneratedKey{}, fmt.Errorf("keys add %s: no JSON in output: %s", name, truncate(string(out), 200))
	}
	var gk GeneratedKey
	if err := json.Unmarshal([]byte(payload), &gk); err != nil {
		return GeneratedKey{}, fmt.Errorf("parse keys add output: %w", err)
	}
	gk.Name = name
	return gk, nil
}

// ImportKey restores a key from a mnemonic, returning its address. It is
// idempotent enough for reruns: callers should delete a stale key first if the
// name already exists.
func (c *ChainCLI) ImportKey(name, mnemonic string) (string, error) {
	args := append([]string{"keys", "add", name, "--keyring-backend", c.keyringBackend(), "--recover", "--output", "json"}, c.homeArgs()...)
	cmd := exec.Command(c.Bin, args...)
	cmd.Stdin = strings.NewReader(mnemonic + "\n")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("keys add --recover %s: %s: %w", name, strings.TrimSpace(string(out)), err)
	}
	return c.ShowAddress(name)
}

// HasKey reports whether a key name already exists in the keyring.
func (c *ChainCLI) HasKey(name string) bool {
	_, err := c.ShowAddress(name)
	return err == nil
}

// SendBankNoWait broadcasts a bank send from the funder key to an address using
// explicit offline account number and sequence, in sync mode, returning the tx
// hash. It does not wait for inclusion (burst broadcast).
func (c *ChainCLI) SendBankNoWait(funderKey string, accNum, seq uint64, to, amount string) (string, error) {
	args := []string{
		"tx", "bank", "send", funderKey, to, amount,
		"--from", funderKey,
		"--offline",
		"--account-number", strconv.FormatUint(accNum, 10),
		"--sequence", strconv.FormatUint(seq, 10),
		"--gas", c.gas(),
		"--gas-prices", c.GasPrices,
		"--yes",
		"--broadcast-mode", "sync",
	}
	if c.gas() == "auto" {
		args = append(args, "--gas-adjustment", c.GasAdjustment)
	}
	out, err := c.Run(args...)
	if err != nil {
		return "", fmt.Errorf("bank send: %s: %w", truncate(out, 200), err)
	}
	payload, ok := ExtractJSONPayload(out)
	if !ok {
		return "", nil
	}
	var resp struct {
		Code   uint32 `json:"code"`
		RawLog string `json:"raw_log"`
		TxHash string `json:"txhash"`
	}
	if err := json.Unmarshal([]byte(payload), &resp); err != nil {
		return "", nil
	}
	if resp.Code != 0 {
		return resp.TxHash, fmt.Errorf("bank send rejected code=%d raw_log=%s", resp.Code, resp.RawLog)
	}
	return resp.TxHash, nil
}

// SubmitTx broadcasts a tx (online, sync) signed by the --from key contained in
// args, waits for inclusion so a dependent tx from the same signer sees the
// advanced sequence, and retries on account-sequence mismatch. Callers pass the
// command and --from but not the gas/broadcast flags. Returns the tx hash.
func (c *ChainCLI) SubmitTx(args ...string) (string, error) {
	full := append(append([]string{}, args...),
		"--gas", c.gas(),
		"--gas-prices", c.GasPrices,
		"--yes",
		"--broadcast-mode", "sync",
	)
	if c.gas() == "auto" {
		full = append(full, "--gas-adjustment", c.GasAdjustment)
	}

	var lastErr error
	for attempt := range 3 {
		out, err := c.Run(full...)
		if err != nil {
			lastErr = fmt.Errorf("broadcast: %s: %w", truncate(out, 200), err)
			if _, _, isSeq := ParseIncorrectAccountSequence(lastErr); isSeq && attempt < 2 {
				_ = c.WaitForNextBlock(15 * time.Second)
				continue
			}
			return "", lastErr
		}
		txHash, code, rawLog, ok := parseSyncBroadcast(out)
		if !ok {
			return "", nil
		}
		if code != 0 {
			rejErr := fmt.Errorf("tx rejected code=%d raw_log=%s", code, rawLog)
			if _, _, isSeq := ParseIncorrectAccountSequence(rejErr); isSeq && attempt < 2 {
				lastErr = rejErr
				_ = c.WaitForNextBlock(15 * time.Second)
				continue
			}
			return txHash, rejErr
		}
		if txHash != "" {
			if werr := c.WaitForTxInclusion(txHash, 30*time.Second); werr != nil {
				return txHash, werr
			}
		}
		return txHash, nil
	}
	return "", lastErr
}

// WaitForTxInclusion polls `query tx <hash>` until the tx is committed (or the
// timeout elapses), returning an error if the committed tx failed.
func (c *ChainCLI) WaitForTxInclusion(txHash string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		out, err := c.Run("query", "tx", txHash)
		if err == nil {
			if _, code, rawLog, ok := parseSyncBroadcast(out); ok {
				if code != 0 {
					return fmt.Errorf("tx %s failed code=%d raw_log=%s", txHash, code, rawLog)
				}
				return nil
			}
		}
		time.Sleep(time.Second)
	}
	return fmt.Errorf("timeout waiting for tx %s inclusion", txHash)
}

// parseSyncBroadcast extracts the tx hash, result code, and raw log from a CLI
// broadcast or `query tx` JSON response.
func parseSyncBroadcast(out string) (txHash string, code uint32, rawLog string, ok bool) {
	payload, has := ExtractJSONPayload(out)
	if !has {
		return "", 0, "", false
	}
	var resp struct {
		Code       uint32 `json:"code"`
		RawLog     string `json:"raw_log"`
		TxHash     string `json:"txhash"`
		TxResponse *struct {
			Code   uint32 `json:"code"`
			RawLog string `json:"raw_log"`
			TxHash string `json:"txhash"`
		} `json:"tx_response"`
	}
	if json.Unmarshal([]byte(payload), &resp) != nil {
		return "", 0, "", false
	}
	if resp.TxResponse != nil {
		return resp.TxResponse.TxHash, resp.TxResponse.Code, resp.TxResponse.RawLog, true
	}
	return resp.TxHash, resp.Code, resp.RawLog, true
}

// HasRedelegation reports whether a redelegation from src to dst already exists
// for the delegator (an in-progress redelegation blocks a new one).
func (c *ChainCLI) HasRedelegation(delegator, src, dst string) (bool, error) {
	out, err := c.Run("query", "staking", "redelegation", delegator, src, dst)
	if err != nil {
		low := strings.ToLower(out)
		if strings.Contains(low, "not found") || strings.Contains(low, "no redelegation") {
			return false, nil
		}
		return false, fmt.Errorf("query redelegation: %s: %w", truncate(out, 200), err)
	}
	n, err := ParseRedelegationCount(out)
	return n > 0, err
}

// HasAuthzGrant reports whether granter already granted grantee an authorization
// for the given message type.
func (c *ChainCLI) HasAuthzGrant(granter, grantee, msgType string) (bool, error) {
	out, err := c.Run("query", "authz", "grants", granter, grantee, msgType)
	if err != nil {
		low := strings.ToLower(out)
		if strings.Contains(low, "not found") || strings.Contains(low, "no authorization") {
			return false, nil
		}
		return false, fmt.Errorf("query authz grants: %s: %w", truncate(out, 200), err)
	}
	n, err := ParseAuthzGrantCount(out)
	return n > 0, err
}

// HasFeegrant reports whether granter already issued a fee allowance to grantee.
func (c *ChainCLI) HasFeegrant(granter, grantee string) (bool, error) {
	out, err := c.Run("query", "feegrant", "grant", granter, grantee)
	if err != nil {
		low := strings.ToLower(out)
		if strings.Contains(low, "not found") || strings.Contains(low, "no allowance") {
			return false, nil
		}
		return false, fmt.Errorf("query feegrant: %s: %w", truncate(out, 200), err)
	}
	return ParseFeegrantExists(out)
}

func (c *ChainCLI) gas() string {
	if c.Gas == "" {
		return "auto"
	}
	return c.Gas
}

func (c *ChainCLI) homeArgs() []string {
	if c.Home == "" {
		return nil
	}
	return []string{"--home", c.Home}
}
