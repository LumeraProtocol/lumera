package validator

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	pkgversion "github.com/LumeraProtocol/lumera/pkg/version"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

const (
	defaultLumeraJSONRPC = "http://supernova_validator_1:8545"
	defaultTipCapWei     = int64(1_000_000_000) // 1 gwei
	defaultRPCTimeout    = 30 * time.Second
)

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *rpcError       `json:"error"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (s *lumeraValidatorSuite) TestEVMJSONRPCBasicMethods() {
	s.requireEVMVersionOrSkip()

	rpc := resolveLumeraJSONRPC(s.lumeraRPC)
	var chainID string
	err := callJSONRPC(rpc, "eth_chainId", []any{}, &chainID)
	s.Require().NoError(err, "eth_chainId")
	s.Require().True(strings.HasPrefix(chainID, "0x"), "unexpected chain id: %s", chainID)

	var blockNumber string
	err = callJSONRPC(rpc, "eth_blockNumber", []any{}, &blockNumber)
	s.Require().NoError(err, "eth_blockNumber")
	s.Require().True(strings.HasPrefix(blockNumber, "0x"), "unexpected block number: %s", blockNumber)

	var netVersion string
	err = callJSONRPC(rpc, "net_version", []any{}, &netVersion)
	s.Require().NoError(err, "net_version")
	s.Require().NotEmpty(netVersion, "net_version should not be empty")
}

func (s *lumeraValidatorSuite) TestEVMJSONRPCNamespacesExposed() {
	s.requireEVMVersionOrSkip()

	rpc := resolveLumeraJSONRPC(s.lumeraRPC)

	var modules map[string]string
	err := callJSONRPC(rpc, "rpc_modules", []any{}, &modules)
	s.Require().NoError(err, "rpc_modules")
	s.Require().NotEmpty(modules, "rpc_modules should return at least one namespace")

	expected := []string{
		"web3",
		"eth",
		"personal",
		"net",
		"txpool",
		"debug",
		"rpc",
	}
	for _, ns := range expected {
		version, ok := modules[ns]
		s.Require().True(ok, "expected JSON-RPC namespace %q to be exposed (modules=%v)", ns, modules)
		s.Require().NotEmpty(version, "namespace %q version should not be empty", ns)
	}
}

func (s *lumeraValidatorSuite) TestEVMFeeMarketBaseFeeActive() {
	s.requireEVMVersionOrSkip()

	rpc := resolveLumeraJSONRPC(s.lumeraRPC)

	var latestBlock map[string]any
	err := callJSONRPC(rpc, "eth_getBlockByNumber", []any{"latest", false}, &latestBlock)
	s.Require().NoError(err, "eth_getBlockByNumber latest")

	baseFeeHex, _ := latestBlock["baseFeePerGas"].(string)
	s.Require().NotEmpty(baseFeeHex, "baseFeePerGas should be present on latest block")
	baseFee := mustParseHexBigInt(baseFeeHex)
	s.Require().Greater(baseFee.Sign(), 0, "baseFeePerGas must be > 0")

	var feeHistory struct {
		BaseFeePerGas []string `json:"baseFeePerGas"`
	}
	err = callJSONRPC(rpc, "eth_feeHistory", []any{"0x1", "latest", []float64{50}}, &feeHistory)
	s.Require().NoError(err, "eth_feeHistory")
	s.Require().GreaterOrEqual(len(feeHistory.BaseFeePerGas), 2, "fee history should include at least 2 base fee entries")
}

func (s *lumeraValidatorSuite) TestEVMSendRawTransactionAndReceipt() {
	s.requireEVMVersionOrSkip()

	rpc := resolveLumeraJSONRPC(s.lumeraRPC)
	txHash, _, _ := s.mustSendDynamicSelfTx(rpc, big.NewInt(1))

	receipt := s.mustWaitReceipt(rpc, txHash, 60*time.Second)
	statusHex, _ := receipt["status"].(string)
	s.Equal("0x1", statusHex, "expected successful tx status")
	gotHash, _ := receipt["transactionHash"].(string)
	s.Equal(strings.ToLower(txHash), strings.ToLower(gotHash), "receipt tx hash mismatch")
	s.NotEmpty(receipt["blockHash"], "receipt missing blockHash")
	s.NotEmpty(receipt["transactionIndex"], "receipt missing transactionIndex")
}

func (s *lumeraValidatorSuite) TestEVMGetTransactionByHashRoundTrip() {
	s.requireEVMVersionOrSkip()

	rpc := resolveLumeraJSONRPC(s.lumeraRPC)
	txHash, _, _ := s.mustSendDynamicSelfTx(rpc, big.NewInt(1))
	receipt := s.mustWaitReceipt(rpc, txHash, 60*time.Second)

	var txObj map[string]any
	err := callJSONRPC(rpc, "eth_getTransactionByHash", []any{txHash}, &txObj)
	s.Require().NoError(err, "eth_getTransactionByHash")
	s.Require().NotNil(txObj, "transaction should exist by hash")

	gotHash, _ := txObj["hash"].(string)
	s.Equal(strings.ToLower(txHash), strings.ToLower(gotHash), "transaction hash mismatch")

	gotBlockHash, _ := txObj["blockHash"].(string)
	receiptBlockHash, _ := receipt["blockHash"].(string)
	s.Equal(strings.ToLower(receiptBlockHash), strings.ToLower(gotBlockHash), "block hash mismatch")

	gotTxIdx, _ := txObj["transactionIndex"].(string)
	receiptTxIdx, _ := receipt["transactionIndex"].(string)
	s.Equal(strings.ToLower(receiptTxIdx), strings.ToLower(gotTxIdx), "transactionIndex mismatch")
}

func (s *lumeraValidatorSuite) TestEVMNonceIncrementsAfterMinedTx() {
	s.requireEVMVersionOrSkip()

	rpc := resolveLumeraJSONRPC(s.lumeraRPC)
	_, sender := s.mustLoadSenderPrivKey()

	beforeLatest := s.mustGetTransactionCount(rpc, sender, "latest")
	beforePending := s.mustGetTransactionCount(rpc, sender, "pending")
	txHash, _, nonceUsed := s.mustSendDynamicSelfTx(rpc, big.NewInt(1))
	s.Equal(beforePending, nonceUsed, "tx should use pending nonce")
	s.mustWaitReceipt(rpc, txHash, 60*time.Second)
	afterLatest := s.mustGetTransactionCount(rpc, sender, "latest")

	s.GreaterOrEqual(afterLatest, beforeLatest+1, "latest nonce should increment after mined tx")
}

func (s *lumeraValidatorSuite) TestEVMBlockLookupByHashAndNumberConsistent() {
	s.requireEVMVersionOrSkip()

	rpc := resolveLumeraJSONRPC(s.lumeraRPC)
	var latestBlockNumber string
	err := callJSONRPC(rpc, "eth_blockNumber", []any{}, &latestBlockNumber)
	s.Require().NoError(err, "eth_blockNumber")
	s.Require().NotEmpty(latestBlockNumber, "latest block number should not be empty")

	var blockByNumber map[string]any
	err = callJSONRPC(rpc, "eth_getBlockByNumber", []any{latestBlockNumber, false}, &blockByNumber)
	s.Require().NoError(err, "eth_getBlockByNumber")
	s.Require().NotNil(blockByNumber, "latest block should be returned")

	blockHash, _ := blockByNumber["hash"].(string)
	blockNumberFromByNumber, _ := blockByNumber["number"].(string)
	s.Require().NotEmpty(blockHash, "block hash should be populated")
	s.Require().NotEmpty(blockNumberFromByNumber, "block number should be populated")

	var blockByHash map[string]any
	err = callJSONRPC(rpc, "eth_getBlockByHash", []any{blockHash, false}, &blockByHash)
	s.Require().NoError(err, "eth_getBlockByHash")
	s.Require().NotNil(blockByHash, "block by hash should be returned")

	blockHashFromByHash, _ := blockByHash["hash"].(string)
	blockNumberFromByHash, _ := blockByHash["number"].(string)
	s.Equal(strings.ToLower(blockHash), strings.ToLower(blockHashFromByHash), "block hash mismatch")
	s.Equal(strings.ToLower(blockNumberFromByNumber), strings.ToLower(blockNumberFromByHash), "block number mismatch")
}

// TestEVMTransactionVisibleAcrossPeerValidator sends an EVM tx to the local
// validator's JSON-RPC and then queries a *peer* validator for the receipt.
// This validates that the broadcast worker correctly propagates EVM transactions
// across the validator set — the exact path that was broken when
// broadcastEVMTransactionsSync used FromEthereumTx (missing From field).
func (s *lumeraValidatorSuite) TestEVMTransactionVisibleAcrossPeerValidator() {
	s.requireEVMVersionOrSkip()

	localRPC := resolveLumeraJSONRPC(s.lumeraRPC)
	peerRPC := s.resolvePeerJSONRPC()
	if peerRPC == "" {
		s.T().Skip("skip cross-validator test: could not resolve a peer validator JSON-RPC endpoint")
		return
	}
	s.T().Logf("local JSON-RPC: %s, peer JSON-RPC: %s", localRPC, peerRPC)

	// Send tx to local validator.
	txHash, _, _ := s.mustSendDynamicSelfTx(localRPC, big.NewInt(1))
	s.T().Logf("sent EVM tx %s to local validator", txHash)

	// Wait for receipt on local validator first (confirms inclusion).
	localReceipt := s.mustWaitReceipt(localRPC, txHash, 60*time.Second)
	statusHex, _ := localReceipt["status"].(string)
	s.Equal("0x1", statusHex, "expected successful tx status on local validator")

	// Query peer validator for the same receipt — this exercises the broadcast
	// worker path that re-gossips promoted txs to peer validators.
	peerReceipt := s.mustWaitReceipt(peerRPC, txHash, 30*time.Second)
	peerStatus, _ := peerReceipt["status"].(string)
	s.Equal("0x1", peerStatus, "expected successful tx status on peer validator")

	peerBlockHash, _ := peerReceipt["blockHash"].(string)
	localBlockHash, _ := localReceipt["blockHash"].(string)
	s.Equal(
		strings.ToLower(localBlockHash),
		strings.ToLower(peerBlockHash),
		"receipt blockHash should match across validators (same consensus block)",
	)
}

// resolvePeerJSONRPC picks a peer validator's JSON-RPC endpoint that differs
// from the local validator. Returns "" if no peer can be determined.
func (s *lumeraValidatorSuite) resolvePeerJSONRPC() string {
	localMoniker := detectValidatorMoniker()
	if localMoniker == "" {
		localMoniker = "supernova_validator_1" // default assumption
	}

	// Try validators 1-5, pick the first one that isn't the local node.
	for i := 1; i <= 5; i++ {
		peer := fmt.Sprintf("supernova_validator_%d", i)
		if peer == localMoniker {
			continue
		}
		peerRPC := fmt.Sprintf("http://%s:8545", peer)
		// Quick liveness check.
		var blockNumber string
		if err := callJSONRPC(peerRPC, "eth_blockNumber", []any{}, &blockNumber); err == nil {
			return peerRPC
		}
	}
	return ""
}

func (s *lumeraValidatorSuite) requireEVMVersionOrSkip() {
	ver, err := resolveLumeraBinaryVersion(s.lumeraBin)
	if err != nil {
		s.T().Skipf("skip EVM runtime tests: failed to resolve %s version: %v", s.lumeraBin, err)
		return
	}
	if !pkgversion.GTE(ver, firstEVMVersion) {
		s.T().Skipf("skip EVM runtime tests: %s version %s < %s", s.lumeraBin, ver, firstEVMVersion)
	}
}

func (s *lumeraValidatorSuite) mustLoadSenderPrivKey() (*ecdsa.PrivateKey, common.Address) {
	home := strings.TrimSpace(os.Getenv("LUMERA_HOME"))
	if home == "" {
		home = "/root/.lumera"
	}

	args := []string{
		"--home", home,
		"keys", "export", s.lumeraKeyName,
		"--unsafe", "--unarmored-hex", "--yes",
		"--keyring-backend", "test",
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, s.lumeraBin, args...)
	out, err := cmd.Output()
	s.Require().NoError(err, "export %s private key from test keyring", s.lumeraKeyName)

	privHex := strings.TrimSpace(string(out))
	privBz, err := hex.DecodeString(privHex)
	s.Require().NoError(err, "decode exported private key hex")
	s.Require().Len(privBz, 32, "unexpected private key byte length")

	privKey, err := crypto.ToECDSA(privBz)
	s.Require().NoError(err, "parse exported private key")
	sender := crypto.PubkeyToAddress(privKey.PublicKey)
	return privKey, sender
}

func (s *lumeraValidatorSuite) mustWaitReceipt(rpcAddr, txHash string, timeout time.Duration) map[string]any {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var receipt map[string]any
		err := callJSONRPC(rpcAddr, "eth_getTransactionReceipt", []any{txHash}, &receipt)
		if err == nil && receipt != nil {
			return receipt
		}
		time.Sleep(2 * time.Second)
	}
	s.T().Fatalf("timed out waiting for receipt for tx %s", txHash)
	return nil
}

func (s *lumeraValidatorSuite) mustSendDynamicSelfTx(rpcAddr string, value *big.Int) (string, common.Address, uint64) {
	privKey, sender := s.mustLoadSenderPrivKey()
	nonce := s.mustGetTransactionCount(rpcAddr, sender, "pending")
	chainID := s.mustGetChainID(rpcAddr)
	baseFee := s.mustGetLatestBaseFee(rpcAddr)

	tipCap := big.NewInt(defaultTipCapWei)
	feeCap := new(big.Int).Mul(baseFee, big.NewInt(2))
	feeCap.Add(feeCap, tipCap)

	to := sender
	tx := ethtypes.NewTx(&ethtypes.DynamicFeeTx{
		ChainID:   chainID,
		Nonce:     nonce,
		GasTipCap: tipCap,
		GasFeeCap: feeCap,
		Gas:       21_000,
		To:        &to,
		Value:     value,
	})

	signer := ethtypes.LatestSignerForChainID(chainID)
	signedTx, err := ethtypes.SignTx(tx, signer, privKey)
	s.Require().NoError(err, "sign dynamic fee tx")
	localHash := strings.ToLower(signedTx.Hash().Hex())

	rawBz, err := signedTx.MarshalBinary()
	s.Require().NoError(err, "marshal signed tx")
	rawHex := "0x" + hex.EncodeToString(rawBz)

	var txHash string
	for attempt := 0; attempt < 3; attempt++ {
		err = callJSONRPC(rpcAddr, "eth_sendRawTransaction", []any{rawHex}, &txHash)
		if err == nil {
			break
		}

		errMsg := strings.ToLower(err.Error())
		if strings.Contains(errMsg, "already in mempool") ||
			strings.Contains(errMsg, "already known") {
			txHash = localHash
			break
		}

		if strings.Contains(errMsg, "context deadline exceeded") {
			var txObj map[string]any
			_ = callJSONRPC(rpcAddr, "eth_getTransactionByHash", []any{localHash}, &txObj)
			if txObj != nil {
				txHash = localHash
				break
			}

			if attempt < 2 {
				time.Sleep(2 * time.Second)
				continue
			}
		}

		s.Require().NoError(err, "eth_sendRawTransaction")
	}
	if txHash == "" {
		txHash = localHash
	}
	s.Require().True(strings.HasPrefix(txHash, "0x"), "unexpected tx hash: %s", txHash)
	return txHash, sender, nonce
}

func (s *lumeraValidatorSuite) mustGetTransactionCount(rpcAddr string, addr common.Address, blockTag string) uint64 {
	var nonceHex string
	err := callJSONRPC(rpcAddr, "eth_getTransactionCount", []any{addr.Hex(), blockTag}, &nonceHex)
	s.Require().NoError(err, "eth_getTransactionCount %s %s", addr.Hex(), blockTag)
	return mustParseHexUint64(nonceHex)
}

func (s *lumeraValidatorSuite) mustGetChainID(rpcAddr string) *big.Int {
	var chainIDHex string
	err := callJSONRPC(rpcAddr, "eth_chainId", []any{}, &chainIDHex)
	s.Require().NoError(err, "eth_chainId")
	chainID := mustParseHexBigInt(chainIDHex)
	s.Require().Greater(chainID.Sign(), 0, "invalid chain id")
	return chainID
}

func (s *lumeraValidatorSuite) mustGetLatestBaseFee(rpcAddr string) *big.Int {
	var latestBlock map[string]any
	err := callJSONRPC(rpcAddr, "eth_getBlockByNumber", []any{"latest", false}, &latestBlock)
	s.Require().NoError(err, "eth_getBlockByNumber latest")

	baseFeeHex, _ := latestBlock["baseFeePerGas"].(string)
	s.Require().NotEmpty(baseFeeHex, "baseFeePerGas should be present")
	baseFee := mustParseHexBigInt(baseFeeHex)
	s.Require().Greater(baseFee.Sign(), 0, "baseFeePerGas should be > 0")
	return baseFee
}

func resolveLumeraJSONRPC(rpcAddr string) string {
	if explicit := strings.TrimSpace(os.Getenv("LUMERA_JSONRPC_ADDR")); explicit != "" {
		return explicit
	}

	// Prefer local node runtime configuration when tests run in validator containers.
	if ports, err := loadLocalLumeradPorts(); err == nil && ports.JSONRPC > 0 {
		return fmt.Sprintf("http://127.0.0.1:%d", ports.JSONRPC)
	}

	if strings.TrimSpace(rpcAddr) == "" {
		return defaultLumeraJSONRPC
	}
	if strings.Contains(rpcAddr, ":26657") {
		return strings.Replace(rpcAddr, ":26657", ":8545", 1)
	}

	u, err := url.Parse(rpcAddr)
	if err != nil || u.Host == "" {
		return defaultLumeraJSONRPC
	}
	host := u.Hostname()
	u.Host = host + ":8545"
	if u.Scheme == "" {
		u.Scheme = "http"
	}
	return u.String()
}

func callJSONRPC(rpcAddr, method string, params any, out any) error {
	body := rpcRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  method,
		Params:  params,
	}
	bz, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal %s request: %w", method, err)
	}

	req, err := http.NewRequest(http.MethodPost, rpcAddr, bytes.NewReader(bz))
	if err != nil {
		return fmt.Errorf("build %s request: %w", method, err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: defaultRPCTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("call %s: %w", method, err)
	}
	defer resp.Body.Close()

	var rpcResp rpcResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return fmt.Errorf("decode %s response: %w", method, err)
	}
	if rpcResp.Error != nil {
		return fmt.Errorf("%s rpc error %d: %s", method, rpcResp.Error.Code, rpcResp.Error.Message)
	}
	if out == nil {
		return nil
	}
	if len(rpcResp.Result) == 0 || string(rpcResp.Result) == "null" {
		return nil
	}
	if err := json.Unmarshal(rpcResp.Result, out); err != nil {
		return fmt.Errorf("decode %s result: %w", method, err)
	}
	return nil
}

func mustParseHexBigInt(v string) *big.Int {
	s := strings.TrimSpace(v)
	s = strings.TrimPrefix(strings.ToLower(s), "0x")
	if s == "" {
		return big.NewInt(0)
	}
	out, ok := new(big.Int).SetString(s, 16)
	if !ok {
		return big.NewInt(0)
	}
	return out
}

func mustParseHexUint64(v string) uint64 {
	s := strings.TrimSpace(strings.ToLower(v))
	s = strings.TrimPrefix(s, "0x")
	if s == "" {
		return 0
	}
	n, err := strconv.ParseUint(s, 16, 64)
	if err != nil {
		return 0
	}
	return n
}
