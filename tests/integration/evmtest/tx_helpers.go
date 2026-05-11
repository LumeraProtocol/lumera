//go:build integration
// +build integration

package evmtest

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"

	lcfg "github.com/LumeraProtocol/lumera/config"
	testaccounts "github.com/LumeraProtocol/lumera/testutil/accounts"
	testjsonrpc "github.com/LumeraProtocol/lumera/testutil/jsonrpc"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	evmhd "github.com/cosmos/evm/crypto/hd"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	evmprogram "github.com/ethereum/go-ethereum/core/vm/program"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
)

// sendOneLegacyTx broadcasts a simple self-transfer legacy tx and returns its hash.
func sendOneLegacyTx(t *testing.T, rpcURL string, keyInfo testaccounts.TestKeyInfo) string {
	t.Helper()

	fromAddr := testaccounts.MustAccountAddressFromTestKeyInfo(t, keyInfo)
	privateKey := mustDerivePrivateKey(t, keyInfo.Mnemonic)

	nonce := mustGetPendingNonceWithRetry(t, rpcURL, fromAddr.Hex(), 20*time.Second)
	gasPrice := mustGetGasPriceWithRetry(t, rpcURL, 20*time.Second)
	toAddr := fromAddr

	return sendLegacyTxWithParams(t, rpcURL, legacyTxParams{
		PrivateKey: privateKey,
		Nonce:      nonce,
		To:         &toAddr,
		Value:      big.NewInt(1),
		Gas:        21_000,
		GasPrice:   gasPrice,
		Data:       nil,
	})
}

// sendOneCosmosBankTx broadcasts a simple bank MsgSend transaction and returns tx hash.
func sendOneCosmosBankTx(t *testing.T, node *evmNode) string {
	t.Helper()

	return sendOneCosmosBankTxWithFees(t, node, "1000"+lcfg.ChainDenom)
}

// sendOneCosmosBankTxWithFees broadcasts bank MsgSend with explicit fee coins.
func sendOneCosmosBankTxWithFees(t *testing.T, node *evmNode, fees string) string {
	t.Helper()

	hash, err := sendOneCosmosBankTxWithFeesResult(t, node, fees)
	if err != nil {
		t.Fatalf("broadcast cosmos tx with fees %s: %v", fees, err)
	}
	return hash
}

// sendOneCosmosBankTxWithFeesResult broadcasts bank MsgSend and returns tx hash or error.
func sendOneCosmosBankTxWithFeesResult(t *testing.T, node *evmNode, fees string) (string, error) {
	t.Helper()

	deadline := time.Now().Add(30 * time.Second)
	var lastErr error

	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		out, err := run(ctx, node.repoRoot, node.binPath,
			"tx", "bank", "send", "validator", node.keyInfo.Address, "1"+lcfg.ChainDenom,
			"--home", node.homeDir,
			"--keyring-backend", "test",
			"--chain-id", node.chainID,
			"--node", node.cometRPCURL,
			"--broadcast-mode", "sync",
			"--gas", "200000",
			"--fees", fees,
			"--yes",
			"--output", "json",
			"--log_no_color",
		)
		cancel()

		if err != nil {
			lastErr = fmt.Errorf("command failed: %w: %s", err, out)
			time.Sleep(400 * time.Millisecond)
			continue
		}

		var resp map[string]any
		if err := json.Unmarshal([]byte(out), &resp); err != nil {
			lastErr = fmt.Errorf("decode response: %w: %s", err, out)
			time.Sleep(400 * time.Millisecond)
			continue
		}

		if codeRaw, ok := resp["code"]; ok {
			switch code := codeRaw.(type) {
			case float64:
				if code != 0 {
					lastErr = fmt.Errorf("checktx rejected with code %.0f: %#v", code, resp)
					time.Sleep(400 * time.Millisecond)
					continue
				}
			case int:
				if code != 0 {
					lastErr = fmt.Errorf("checktx rejected with code %d: %#v", code, resp)
					time.Sleep(400 * time.Millisecond)
					continue
				}
			}
		}

		hash, ok := resp["txhash"].(string)
		if !ok || strings.TrimSpace(hash) == "" {
			lastErr = fmt.Errorf("missing txhash in response: %#v", resp)
			time.Sleep(400 * time.Millisecond)
			continue
		}

		return hash, nil
	}

	return "", fmt.Errorf("timed out broadcasting cosmos tx with fees %s: %w", fees, lastErr)
}

// sendLogEmitterCreationTx deploys tiny runtime code that emits one LOG1 event.
func sendLogEmitterCreationTx(t *testing.T, rpcURL string, keyInfo testaccounts.TestKeyInfo, topicHex string) string {
	t.Helper()

	fromAddr := testaccounts.MustAccountAddressFromTestKeyInfo(t, keyInfo)
	privateKey := mustDerivePrivateKey(t, keyInfo.Mnemonic)
	nonce := mustGetPendingNonceWithRetry(t, rpcURL, fromAddr.Hex(), 20*time.Second)
	gasPrice := mustGetGasPriceWithRetry(t, rpcURL, 20*time.Second)
	data := logEmitterCreationCode(topicHex)

	return sendLegacyTxWithParams(t, rpcURL, legacyTxParams{
		PrivateKey: privateKey,
		Nonce:      nonce,
		To:         nil,
		Value:      big.NewInt(0),
		Gas:        200_000,
		GasPrice:   gasPrice,
		Data:       data,
	})
}

type legacyTxParams struct {
	PrivateKey *ecdsa.PrivateKey // Signer private key.
	Nonce      uint64            // Sender nonce.
	To         *common.Address   // Recipient; nil means contract creation.
	Value      *big.Int          // Native value transferred.
	Gas        uint64            // Gas limit.
	GasPrice   *big.Int          // Legacy gas price.
	Data       []byte            // Optional calldata / init code.
}

type dynamicFeeTxParams struct {
	PrivateKey *ecdsa.PrivateKey // Signer private key.
	Nonce      uint64            // Sender nonce.
	To         *common.Address   // Recipient; nil means contract creation.
	Value      *big.Int          // Native value transferred.
	Gas        uint64            // Gas limit.
	GasFeeCap  *big.Int          // EIP-1559 maxFeePerGas.
	GasTipCap  *big.Int          // EIP-1559 maxPriorityFeePerGas.
	Data       []byte            // Optional calldata / init code.
}

// sendLegacyTxWithParams signs and broadcasts a legacy tx with caller-supplied fields.
func sendLegacyTxWithParams(t *testing.T, rpcURL string, p legacyTxParams) string {
	t.Helper()

	txHash, err := sendLegacyTxWithParamsResult(rpcURL, p)
	if err != nil {
		t.Fatalf("send legacy tx: %v", err)
	}
	return txHash
}

// sendLegacyTxWithParamsResult signs and broadcasts a legacy tx and returns hash or error.
func sendLegacyTxWithParamsResult(rpcURL string, p legacyTxParams) (string, error) {
	tx, err := signedLegacyTxBytes(p)
	if err != nil {
		return "", err
	}

	var txHash string
	if err := testjsonrpc.Call(context.Background(), rpcURL, "eth_sendRawTransaction", []any{hexutil.Encode(tx)}, &txHash); err != nil {
		return "", fmt.Errorf("eth_sendRawTransaction nonce=%d gas_price=%s failed: %w", p.Nonce, p.GasPrice.String(), err)
	}
	if strings.TrimSpace(txHash) == "" {
		return "", errors.New("eth_sendRawTransaction returned empty tx hash")
	}

	return txHash, nil
}

// signedLegacyTxBytes signs a legacy transaction and returns raw RLP bytes.
func signedLegacyTxBytes(p legacyTxParams) ([]byte, error) {
	tx := gethtypes.NewTx(&gethtypes.LegacyTx{
		Nonce:    p.Nonce,
		To:       p.To,
		Value:    p.Value,
		Gas:      p.Gas,
		GasPrice: p.GasPrice,
		Data:     p.Data,
	})

	signedTx, err := gethtypes.SignTx(tx, gethtypes.NewEIP155Signer(new(big.Int).SetUint64(lcfg.EVMChainID)), p.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("sign tx: %w", err)
	}

	rawTxBz, err := signedTx.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("marshal signed tx: %w", err)
	}

	return rawTxBz, nil
}

// sendDynamicFeeTxWithParams signs and broadcasts an EIP-1559 (type-2)
// transaction with caller-supplied fields.
func sendDynamicFeeTxWithParams(t *testing.T, rpcURL string, p dynamicFeeTxParams) string {
	t.Helper()

	txHash, err := sendDynamicFeeTxWithParamsResult(rpcURL, p)
	if err != nil {
		t.Fatalf("send dynamic-fee tx: %v", err)
	}
	return txHash
}

// sendDynamicFeeTxWithParamsResult signs and broadcasts a type-2 tx and
// returns hash or error.
func sendDynamicFeeTxWithParamsResult(rpcURL string, p dynamicFeeTxParams) (string, error) {
	tx, err := signedDynamicFeeTxBytes(p)
	if err != nil {
		return "", err
	}

	var txHash string
	if err := testjsonrpc.Call(context.Background(), rpcURL, "eth_sendRawTransaction", []any{hexutil.Encode(tx)}, &txHash); err != nil {
		return "", fmt.Errorf(
			"eth_sendRawTransaction type=0x2 nonce=%d gas_fee_cap=%s gas_tip_cap=%s failed: %w",
			p.Nonce,
			p.GasFeeCap.String(),
			p.GasTipCap.String(),
			err,
		)
	}
	if strings.TrimSpace(txHash) == "" {
		return "", errors.New("eth_sendRawTransaction returned empty tx hash")
	}

	return txHash, nil
}

// signedDynamicFeeTxBytes signs a type-2 transaction and returns raw RLP bytes.
func signedDynamicFeeTxBytes(p dynamicFeeTxParams) ([]byte, error) {
	tx := gethtypes.NewTx(&gethtypes.DynamicFeeTx{
		ChainID:   new(big.Int).SetUint64(lcfg.EVMChainID),
		Nonce:     p.Nonce,
		To:        p.To,
		Value:     p.Value,
		Gas:       p.Gas,
		GasFeeCap: p.GasFeeCap,
		GasTipCap: p.GasTipCap,
		Data:      p.Data,
	})

	signer := gethtypes.LatestSignerForChainID(new(big.Int).SetUint64(lcfg.EVMChainID))
	signedTx, err := gethtypes.SignTx(tx, signer, p.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("sign dynamic-fee tx: %w", err)
	}

	rawTxBz, err := signedTx.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("marshal signed dynamic-fee tx: %w", err)
	}

	return rawTxBz, nil
}

// mustGetPendingNonceWithRetry polls pending nonce until node is ready.
func mustGetPendingNonceWithRetry(t *testing.T, rpcURL, fromHex string, timeout time.Duration) uint64 {
	t.Helper()

	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		var nonceHex string
		err := testjsonrpc.Call(context.Background(), rpcURL, "eth_getTransactionCount", []any{fromHex, "pending"}, &nonceHex)
		if err == nil {
			nonce, decodeErr := hexutil.DecodeUint64(nonceHex)
			if decodeErr == nil {
				return nonce
			}
			lastErr = fmt.Errorf("decode nonce %q: %w", nonceHex, decodeErr)
		} else {
			lastErr = err
		}
		time.Sleep(400 * time.Millisecond)
	}

	t.Fatalf("failed to get pending nonce for %s within %s: %v", fromHex, timeout, lastErr)
	return 0
}

// mustGetGasPriceWithRetry polls gas price until available.
func mustGetGasPriceWithRetry(t *testing.T, rpcURL string, timeout time.Duration) *big.Int {
	t.Helper()

	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		var gasPriceHex string
		err := testjsonrpc.Call(context.Background(), rpcURL, "eth_gasPrice", []any{}, &gasPriceHex)
		if err == nil {
			gasPrice, decodeErr := hexutil.DecodeBig(gasPriceHex)
			if decodeErr == nil {
				return gasPrice
			}
			lastErr = fmt.Errorf("decode gas price %q: %w", gasPriceHex, decodeErr)
		} else {
			lastErr = err
		}
		time.Sleep(400 * time.Millisecond)
	}

	t.Fatalf("failed to get gas price within %s: %v", timeout, lastErr)
	return nil
}

// mustDerivePrivateKey derives an eth_secp256k1 key from mnemonic + default path.
func mustDerivePrivateKey(t *testing.T, mnemonic string) *ecdsa.PrivateKey {
	t.Helper()

	derivedKey, err := evmhd.EthSecp256k1.Derive()(mnemonic, keyring.DefaultBIP39Passphrase, evmhd.BIP44HDPath)
	if err != nil {
		t.Fatalf("derive eth_secp256k1 key: %v", err)
	}

	privateKey, err := ethcrypto.ToECDSA(derivedKey)
	if err != nil {
		t.Fatalf("to ecdsa: %v", err)
	}

	return privateKey
}

// logEmitterCreationCode returns init code for a contract that emits LOG1 then returns empty runtime.
func logEmitterCreationCode(topicHex string) []byte {
	topic := topicWordBytes(topicHex)

	/*
		Creation code only (no persistent runtime):
		- PUSH32 <topic>, PUSH1 0, PUSH1 0, LOG1
		  Emit a single log entry during contract creation.
		- PUSH1 0, PUSH1 0, RETURN
		  Return empty bytecode so the deployed contract code size is zero.

		This is intentionally minimal for indexer/log tests where we only care
		about deterministic receipt/log behavior of a deploy transaction.
	*/
	return evmprogram.New().
		Push(topic).Push(0).Push(0).Op(vm.LOG1).
		Return(0, 0).
		Bytes()
}

// topicWordBytes returns a 32-byte topic word, left-padded/truncated from a hex string.
func topicWordBytes(topicHex string) []byte {
	trimmed := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(topicHex)), "0x")
	if len(trimmed)%2 != 0 {
		trimmed = "0" + trimmed
	}

	decoded, err := hex.DecodeString(trimmed)
	if err != nil {
		panic(fmt.Sprintf("invalid topic hex %q: %v", topicHex, err))
	}

	if len(decoded) > 32 {
		decoded = decoded[len(decoded)-32:]
	}
	if len(decoded) < 32 {
		padded := make([]byte, 32)
		copy(padded[32-len(decoded):], decoded)
		decoded = padded
	}

	return decoded
}
