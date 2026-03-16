package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	txtypes "cosmossdk.io/api/cosmos/tx/v1beta1"
	sdkbase "github.com/LumeraProtocol/sdk-go/blockchain/base"
)

var (
	txWaitClientOnce sync.Once
	txWaitClient     *sdkbase.Client
	txWaitClientErr  error
)

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
	var lastOut string
	var lastErr error

	for attempt := 0; attempt < 3; attempt++ {
		out, txHash, err := runTxWithMode(args, "sync")
		if err == nil {
			// Wait for tx inclusion before returning so the next tx sees updated state.
			if txHash != "" {
				code, rawLog, err := waitForTxResult(txHash, 45*time.Second)
				if err != nil {
					return out, fmt.Errorf("tx %s result query failed: %w", txHash, err)
				}
				if code != 0 {
					return out, fmt.Errorf("tx deliver failed code=%d raw_log=%s", code, rawLog)
				}
			}
			return out, nil
		}

		lastOut = out
		lastErr = err
		expectedSeq, gotSeq, ok := parseIncorrectAccountSequence(err)
		if !ok {
			return out, err
		}
		if attempt == 2 {
			return out, err
		}

		log.Printf("  INFO: retrying tx after sequence mismatch (expected=%d got=%d, retry %d/2)", expectedSeq, gotSeq, attempt+1)
		if waitErr := waitForNextBlock(20 * time.Second); waitErr != nil {
			log.Printf("  WARN: wait for next block after sequence mismatch: %v", waitErr)
		}
	}

	return lastOut, lastErr
}

func runTxWithAccountSequence(accountNumber, sequence uint64, args ...string) (string, error) {
	out, txHash, err := runTxNoWaitWithAccountSequence(accountNumber, sequence, args...)
	if err != nil {
		return out, err
	}
	// Wait for tx inclusion before returning so the next tx sees updated state.
	if txHash != "" {
		code, rawLog, err := waitForTxResult(txHash, 45*time.Second)
		if err != nil {
			return out, fmt.Errorf("tx %s result query failed: %w", txHash, err)
		}
		if code != 0 {
			return out, fmt.Errorf("tx deliver failed code=%d raw_log=%s", code, rawLog)
		}
	}
	return out, nil
}

func runMigrationTxWithAdaptiveAccountNumber(accountNumber, sequence uint64, args ...string) (string, error) {
	curAccNum := accountNumber
	var lastOut string
	var lastErr error

	for attempt := 0; attempt < 3; attempt++ {
		out, err := runTxWithAccountSequence(curAccNum, sequence, args...)
		if err == nil {
			return out, nil
		}
		lastOut = out
		lastErr = err

		expectedAccNum, ok := parseSignatureMismatchAccountNumber(err)
		if !ok || expectedAccNum == curAccNum {
			return out, err
		}

		log.Printf("  INFO: migration tx signer account number adjusted %d -> %d (retry %d/2)", curAccNum, expectedAccNum, attempt+1)
		curAccNum = expectedAccNum
	}

	return lastOut, lastErr
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
	gas := *flagGas
	if shouldAutoEstimateMigrationGas(args) && gas != "auto" {
		gas = "auto"
	}

	txArgs = append(txArgs,
		"--gas", gas,
		"--gas-prices", *flagGasPrices,
		"--yes",
		"--broadcast-mode", broadcastMode,
	)
	if gas == "auto" {
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
	if payload, ok := extractJSONPayload(out); ok && json.Unmarshal([]byte(payload), &txResp) == nil {
		if txResp.Code != 0 {
			return out, txResp.TxHash, fmt.Errorf("tx rejected code=%d raw_log=%s", txResp.Code, txResp.RawLog)
		}
		return out, txResp.TxHash, nil
	}

	return out, "", nil
}

// extractJSONPayload pulls the last JSON object out of mixed stdout/stderr
// command output. This is needed for migration txs because the custom CLI emits
// a gas-estimate line before the broadcast response when --gas=auto is used.
func extractJSONPayload(out string) (string, bool) {
	start := strings.IndexByte(out, '{')
	end := strings.LastIndexByte(out, '}')
	if start == -1 || end == -1 || end < start {
		return "", false
	}
	return strings.TrimSpace(out[start : end+1]), true
}

// EVM migration txs are fee-waived, but they are still fully gas-metered.
// Their touched-state set can be much larger than ordinary account txs, so the
// fixed default gas limit used elsewhere in this test tool is too low.
func shouldAutoEstimateMigrationGas(args []string) bool {
	if len(args) < 3 {
		return false
	}
	if args[0] != "tx" || args[1] != "evmigration" {
		return false
	}
	switch args[2] {
	case "claim-legacy-account", "migrate-validator":
		return true
	default:
		return false
	}
}

// --- Tx waiting and block utilities ---

// waitTx waits until a tx is queryable. This avoids depending on the CLI
// wait-tx wrapper, which currently prepends usage text to runtime errors.
func waitTx(txHash string) error {
	_, _, err := waitForTxResult(txHash, 30*time.Second)
	return err
}

func queryTxCode(txHash string) (uint32, string, error) {
	resp, err := queryTxResponse(txHash, 10*time.Second)
	if err != nil {
		return 0, "", err
	}
	return txResultCode(resp)
}

func waitForTxResult(txHash string, timeout time.Duration) (uint32, string, error) {
	resp, err := queryTxResponse(txHash, timeout)
	if err != nil {
		return 0, "", err
	}
	return txResultCode(resp)
}

func queryTxResponse(txHash string, timeout time.Duration) (*txtypes.GetTxResponse, error) {
	client, err := getTxWaitClient()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	resp, err := client.WaitForTxInclusion(ctx, txHash)
	if err != nil {
		return nil, fmt.Errorf("wait for tx inclusion %s: %w", txHash, err)
	}
	if resp == nil || resp.TxResponse == nil {
		return nil, fmt.Errorf("wait for tx inclusion %s: empty tx response", txHash)
	}
	return resp, nil
}

func txResultCode(resp *txtypes.GetTxResponse) (uint32, string, error) {
	if resp == nil || resp.TxResponse == nil {
		return 0, "", fmt.Errorf("empty tx response")
	}
	return resp.TxResponse.Code, resp.TxResponse.RawLog, nil
}

func txWaitClientConfig() sdkbase.Config {
	return sdkbase.Config{
		ChainID:     *flagChainID,
		GRPCAddr:    resolveGRPC(),
		RPCEndpoint: rpcForSDK(*flagRPC),
		Timeout:     30 * time.Second,
		WaitTx:      sdkWaitTxConfig(),
	}
}

func getTxWaitClient() (*sdkbase.Client, error) {
	txWaitClientOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		cfg := txWaitClientConfig()

		log.Printf("sdk-go tx waiter config: chain_id=%s grpc=%s rpc=%s wait_tx={setup=%s poll=%s max_retries=%d max_backoff=%s}",
			cfg.ChainID,
			cfg.GRPCAddr,
			cfg.RPCEndpoint,
			cfg.WaitTx.SubscriberSetupTimeout,
			cfg.WaitTx.PollInterval,
			cfg.WaitTx.PollMaxRetries,
			cfg.WaitTx.PollBackoffMaxInterval,
		)

		txWaitClient, txWaitClientErr = sdkbase.New(ctx, cfg, nil, "")
	})
	return txWaitClient, txWaitClientErr
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
