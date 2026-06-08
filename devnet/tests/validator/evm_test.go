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
	"sync"
	"time"

	pkgversion "github.com/LumeraProtocol/lumera/pkg/version"
	precompilecommon "github.com/cosmos/evm/precompiles/common"
	distributionprecompile "github.com/cosmos/evm/precompiles/distribution"
	slashingprecompile "github.com/cosmos/evm/precompiles/slashing"
	stakingprecompile "github.com/cosmos/evm/precompiles/staking"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	evmprogram "github.com/ethereum/go-ethereum/core/vm/program"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/gorilla/websocket"
)

const (
	defaultLumeraJSONRPC          = "http://supernova_validator_1:8545"
	actionPrecompileAddress       = "0x0000000000000000000000000000000000000901"
	bankPrecompileAddress         = "0x0000000000000000000000000000000000000804"
	distributionPrecompileAddress = "0x0000000000000000000000000000000000000801"
	govPrecompileAddress          = "0x0000000000000000000000000000000000000805"
	slashingPrecompileAddress     = "0x0000000000000000000000000000000000000806"
	stakingPrecompileAddress      = "0x0000000000000000000000000000000000000800"
	defaultTipCapWei              = int64(1_000_000_000) // 1 gwei
	defaultRPCTimeout             = 30 * time.Second
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

func (s *lumeraValidatorSuite) TestEVMJSONRPCRateLimitPublicProfileIfEnabled() {
	s.requireEVMVersionOrSkip()

	cfg, err := loadLocalJSONRPCRateLimitConfig()
	if err != nil {
		s.T().Skipf("skip rate-limit profile test: %v", err)
		return
	}
	if !cfg.Enabled {
		s.T().Skip("skip rate-limit profile test: lumera.json-rpc-ratelimit.enable is false")
		return
	}
	if cfg.Burst > 400 || cfg.RequestsPerSecond > 250 {
		s.T().Skipf("skip rate-limit profile test: configured burst/RPS too high for bounded probe (burst=%d rps=%d)", cfg.Burst, cfg.RequestsPerSecond)
		return
	}

	rpc := resolveLumeraJSONRPC(s.lumeraRPC)
	totalRequests := cfg.Burst + cfg.RequestsPerSecond + 25
	if totalRequests < cfg.Burst*3 {
		totalRequests = cfg.Burst * 3
	}
	if totalRequests < 50 {
		totalRequests = 50
	}
	if totalRequests > 800 {
		totalRequests = 800
	}

	var wg sync.WaitGroup
	start := make(chan struct{})
	statuses := make(chan int, totalRequests)
	for i := 0; i < totalRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			status, err := postJSONRPCStatus(rpc, "eth_chainId", []any{})
			if err == nil {
				statuses <- status
			}
		}()
	}
	close(start)
	wg.Wait()
	close(statuses)

	tooManyRequests := 0
	for status := range statuses {
		if status == http.StatusTooManyRequests {
			tooManyRequests++
		}
	}
	s.Require().Greater(tooManyRequests, 0, "expected at least one HTTP 429 from enabled JSON-RPC rate limiter")

	// The burst above intentionally drains the shared per-IP token bucket. The
	// rest of this suite reuses the same endpoint/client IP, so wait for the
	// limiter to refill before returning to avoid leaking spurious 429s into
	// later EVM tests.
	s.waitForRateLimitRefill(rpc)
}

// waitForRateLimitRefill probes eth_chainId until the JSON-RPC rate limiter
// stops returning 429, confirming the per-IP token bucket has refilled. It is a
// best-effort cleanup step: on timeout it logs and returns rather than failing,
// since refill behavior is not what the calling test asserts.
func (s *lumeraValidatorSuite) waitForRateLimitRefill(rpc string) {
	s.T().Helper()
	deadline := time.Now().Add(15 * time.Second)
	for {
		status, err := postJSONRPCStatus(rpc, "eth_chainId", []any{})
		if err == nil && status != http.StatusTooManyRequests {
			return
		}
		if time.Now().After(deadline) {
			s.T().Logf("rate limiter still returning 429 after refill wait (last status=%d err=%v); continuing", status, err)
			return
		}
		time.Sleep(250 * time.Millisecond)
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

func (s *lumeraValidatorSuite) TestEVMWebSocketNewHeadsSubscription() {
	s.requireEVMVersionOrSkip()

	wsAddr := resolveLumeraJSONWS(s.lumeraRPC)
	conn, _, err := websocket.DefaultDialer.Dial(wsAddr, nil)
	s.Require().NoError(err, "dial EVM JSON-RPC websocket %s", wsAddr)
	defer conn.Close()

	subscribe := rpcRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "eth_subscribe",
		Params:  []any{"newHeads"},
	}
	s.Require().NoError(conn.WriteJSON(subscribe), "eth_subscribe newHeads")

	subscriptionID := s.mustReadSubscriptionID(conn, 15*time.Second)
	s.Require().NotEmpty(subscriptionID, "subscription id should not be empty")

	rpc := resolveLumeraJSONRPC(s.lumeraRPC)
	txHash, _, _ := s.mustSendDynamicSelfTx(rpc, big.NewInt(1))
	s.mustWaitReceipt(rpc, txHash, 60*time.Second)

	header := s.mustReadNewHeadsNotification(conn, subscriptionID, 45*time.Second)
	number, _ := header["number"].(string)
	s.Require().True(strings.HasPrefix(number, "0x"), "newHeads notification missing block number: %#v", header)
	hash, _ := header["hash"].(string)
	s.Require().True(strings.HasPrefix(hash, "0x"), "newHeads notification missing block hash: %#v", header)
}

func (s *lumeraValidatorSuite) TestEVMContractDeployCallAndLogsDevnet() {
	s.requireEVMVersionOrSkip()

	rpc := resolveLumeraJSONRPC(s.lumeraRPC)
	topic := "0x" + strings.Repeat("44", 32)
	txHash := s.mustSendDynamicContractCreation(rpc, loggingConstantContractCreationCode(topic), 500_000)
	receipt := s.mustWaitReceipt(rpc, txHash, 60*time.Second)
	s.requireSuccessfulReceipt(receipt, txHash)

	contractAddress, _ := receipt["contractAddress"].(string)
	s.Require().True(
		strings.HasPrefix(contractAddress, "0x") && !strings.EqualFold(contractAddress, "0x0000000000000000000000000000000000000000"),
		"unexpected contract address in receipt: %#v",
		receipt,
	)
	s.requireReceiptHasTopic(receipt, topic)

	var callResult string
	err := callJSONRPC(rpc, "eth_call", []any{
		map[string]any{
			"to":   contractAddress,
			"data": "0x",
		},
		"latest",
	}, &callResult)
	s.Require().NoError(err, "eth_call deployed contract")
	s.requireUint256Hex(callResult, 42)

	blockNumber, _ := receipt["blockNumber"].(string)
	var logs []map[string]any
	err = callJSONRPC(rpc, "eth_getLogs", []any{map[string]any{
		"fromBlock": blockNumber,
		"toBlock":   blockNumber,
		"address":   contractAddress,
		"topics":    []any{topic},
	}}, &logs)
	s.Require().NoError(err, "eth_getLogs for deployment topic")
	s.Require().NotEmpty(logs, "expected deployment log for topic %s", topic)
}

func (s *lumeraValidatorSuite) TestEVMContractPersistsAcrossLocalLumeradRestart() {
	s.requireEVMVersionOrSkip()
	s.requireRestartTestsEnabledOrSkip()

	rpc := resolveLumeraJSONRPC(s.lumeraRPC)
	txHash := s.mustSendDynamicContractCreation(rpc, loggingConstantContractCreationCode("0x"+strings.Repeat("55", 32)), 500_000)
	receipt := s.mustWaitReceipt(rpc, txHash, 60*time.Second)
	s.requireSuccessfulReceipt(receipt, txHash)

	contractAddress, _ := receipt["contractAddress"].(string)
	s.Require().True(strings.HasPrefix(contractAddress, "0x"), "unexpected contract address: %#v", receipt)
	s.requireContractReturnsUint64(rpc, contractAddress, 42)

	s.mustRestartLocalLumerad()
	s.mustWaitJSONRPCReady(rpc, 90*time.Second)

	var codeHex string
	err := callJSONRPC(rpc, "eth_getCode", []any{contractAddress, "latest"}, &codeHex)
	s.Require().NoError(err, "eth_getCode after restart")
	s.Require().NotEqual("0x", strings.ToLower(strings.TrimSpace(codeHex)), "contract code missing after restart")
	s.requireContractReturnsUint64(rpc, contractAddress, 42)
}

func (s *lumeraValidatorSuite) TestEVMActionPrecompileQueryDevnet() {
	s.requireEVMVersionOrSkip()

	input := abiCallUint64("getActionFee(uint64)", 100)

	var resultHex string
	err := callJSONRPC(resolveLumeraJSONRPC(s.lumeraRPC), "eth_call", []any{
		map[string]any{
			"to":   actionPrecompileAddress,
			"data": "0x" + hex.EncodeToString(input),
		},
		"latest",
	}, &resultHex)
	s.Require().NoError(err, "eth_call action precompile getActionFee")

	result := common.FromHex(resultHex)
	s.Require().GreaterOrEqual(len(result), 96, "getActionFee should return three uint256 words, got %d bytes", len(result))
	totalFee := new(big.Int).SetBytes(result[:32])
	s.Require().Greater(totalFee.Sign(), 0, "unexpected total fee result: %s", totalFee)
}

func (s *lumeraValidatorSuite) TestEVMBankPrecompileTotalSupplyQueryDevnet() {
	s.requireEVMVersionOrSkip()

	input := abiCallNoArgs("totalSupply()")
	var resultHex string
	err := callJSONRPC(resolveLumeraJSONRPC(s.lumeraRPC), "eth_call", []any{
		map[string]any{
			"to":   bankPrecompileAddress,
			"data": "0x" + hex.EncodeToString(input),
		},
		"latest",
	}, &resultHex)
	s.Require().NoError(err, "eth_call bank precompile totalSupply")
	s.Require().True(strings.HasPrefix(resultHex, "0x"), "unexpected totalSupply result: %s", resultHex)
	s.Require().GreaterOrEqual(len(common.FromHex(resultHex)), 64, "totalSupply should return ABI offset and array length words")
}

func (s *lumeraValidatorSuite) TestEVMStandardPrecompileQueryMatrixDevnet() {
	s.requireEVMVersionOrSkip()

	rpc := resolveLumeraJSONRPC(s.lumeraRPC)
	_, localAddr := s.mustLoadSenderPrivKey()

	distInput, err := distributionprecompile.ABI.Pack(distributionprecompile.CommunityPoolMethod)
	s.Require().NoError(err, "pack distribution communityPool input")
	distResult := s.mustEthCallRaw(rpc, distributionPrecompileAddress, distInput, "distribution communityPool")
	var distOut struct {
		Coins []precompilecommon.DecCoin `abi:"coins"`
	}
	s.Require().NoError(
		distributionprecompile.ABI.UnpackIntoInterface(&distOut, distributionprecompile.CommunityPoolMethod, distResult),
		"unpack distribution communityPool output",
	)
	for _, coin := range distOut.Coins {
		s.Require().NotEmpty(strings.TrimSpace(coin.Denom), "communityPool returned empty denom")
		s.Require().NotNil(coin.Amount, "communityPool returned nil amount for %s", coin.Denom)
	}

	slashingInput, err := slashingprecompile.ABI.Pack(slashingprecompile.GetParamsMethod)
	s.Require().NoError(err, "pack slashing getParams input")
	slashingResult := s.mustEthCallRaw(rpc, slashingPrecompileAddress, slashingInput, "slashing getParams")
	var slashingOut struct {
		Params slashingprecompile.Params `abi:"params"`
	}
	s.Require().NoError(
		slashingprecompile.ABI.UnpackIntoInterface(&slashingOut, slashingprecompile.GetParamsMethod, slashingResult),
		"unpack slashing getParams output",
	)
	s.Require().Greater(slashingOut.Params.SignedBlocksWindow, int64(0), "slashing signedBlocksWindow must be positive")

	stakingInput, err := stakingprecompile.ABI.Pack(stakingprecompile.ValidatorMethod, localAddr)
	s.Require().NoError(err, "pack staking validator input")
	stakingResult, err := ethCallRaw(rpc, stakingPrecompileAddress, stakingInput)
	if err != nil {
		s.T().Logf("skip staking validator precompile assertion: local key %s is not an active validator: %v", localAddr.Hex(), err)
		return
	}
	var stakingOut struct {
		Validator stakingprecompile.ValidatorInfo `abi:"validator"`
	}
	err = stakingprecompile.ABI.UnpackIntoInterface(&stakingOut, stakingprecompile.ValidatorMethod, stakingResult)
	if err != nil || strings.TrimSpace(stakingOut.Validator.OperatorAddress) == "" {
		s.T().Logf("skip staking validator precompile assertion: local key %s did not decode as active validator (%v)", localAddr.Hex(), err)
		return
	}
	s.Require().Equal(strings.ToLower(localAddr.Hex()), strings.ToLower(stakingOut.Validator.OperatorAddress), "staking validator operator mismatch")
}

func (s *lumeraValidatorSuite) TestEVMGovPrecompileTxPathDevnet() {
	s.requireEVMVersionOrSkip()

	rpc := resolveLumeraJSONRPC(s.lumeraRPC)
	_, proposer := s.mustLoadSenderPrivKey()
	to := common.HexToAddress(govPrecompileAddress)
	input := abiCallAddressUint64("cancelProposal(address,uint64)", proposer, 9_999_999)

	txHash, _, _ := s.mustSendDynamicTx(rpc, &senderTx{
		To:    &to,
		Value: big.NewInt(0),
		Gas:   500_000,
		Data:  input,
	})
	receipt := s.mustWaitReceipt(rpc, txHash, 60*time.Second)
	gotHash, _ := receipt["transactionHash"].(string)
	s.Require().Equal(strings.ToLower(txHash), strings.ToLower(gotHash), "receipt tx hash mismatch")
	statusHex, _ := receipt["status"].(string)
	s.Require().Equal("0x0", statusHex, "expected gov cancelProposal for unknown proposal to fail in EVM receipt")
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

func (s *lumeraValidatorSuite) requireRestartTestsEnabledOrSkip() {
	enabled := parseBool(os.Getenv("LUMERA_DEVNET_RESTART_TESTS"), false)
	if !enabled {
		s.T().Skip("skip restart-persistence test: set LUMERA_DEVNET_RESTART_TESTS=true to restart local lumerad")
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
	txHash, sender, nonce := s.mustSendDynamicTx(rpcAddr, &senderTx{
		Value: value,
		Gas:   21_000,
	})
	return txHash, sender, nonce
}

type senderTx struct {
	To    *common.Address
	Value *big.Int
	Gas   uint64
	Data  []byte
}

func (s *lumeraValidatorSuite) mustSendDynamicContractCreation(rpcAddr string, data []byte, gas uint64) string {
	txHash, _, _ := s.mustSendDynamicTx(rpcAddr, &senderTx{
		Value: big.NewInt(0),
		Gas:   gas,
		Data:  data,
	})
	return txHash
}

func (s *lumeraValidatorSuite) mustSendDynamicTx(rpcAddr string, p *senderTx) (string, common.Address, uint64) {
	privKey, sender := s.mustLoadSenderPrivKey()
	nonce := s.mustGetTransactionCount(rpcAddr, sender, "pending")
	chainID := s.mustGetChainID(rpcAddr)
	baseFee := s.mustGetLatestBaseFee(rpcAddr)
	to := p.To
	if to == nil && len(p.Data) == 0 {
		to = &sender
	}

	value := p.Value
	if value == nil {
		value = big.NewInt(0)
	}

	tipCap := big.NewInt(defaultTipCapWei)
	feeCap := new(big.Int).Mul(baseFee, big.NewInt(2))
	feeCap.Add(feeCap, tipCap)

	tx := ethtypes.NewTx(&ethtypes.DynamicFeeTx{
		ChainID:   chainID,
		Nonce:     nonce,
		GasTipCap: tipCap,
		GasFeeCap: feeCap,
		Gas:       p.Gas,
		To:        to,
		Value:     value,
		Data:      p.Data,
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

func (s *lumeraValidatorSuite) mustReadSubscriptionID(conn *websocket.Conn, timeout time.Duration) string {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		var msg map[string]any
		if err := conn.ReadJSON(&msg); err != nil {
			continue
		}
		if errObj, ok := msg["error"].(map[string]any); ok {
			s.T().Fatalf("eth_subscribe returned error: %#v", errObj)
		}
		if result, ok := msg["result"].(string); ok {
			return result
		}
	}
	s.T().Fatalf("timed out waiting for eth_subscribe response")
	return ""
}

func (s *lumeraValidatorSuite) mustReadNewHeadsNotification(conn *websocket.Conn, subscriptionID string, timeout time.Duration) map[string]any {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		var msg map[string]any
		if err := conn.ReadJSON(&msg); err != nil {
			continue
		}
		if method, _ := msg["method"].(string); method != "eth_subscription" {
			continue
		}
		params, _ := msg["params"].(map[string]any)
		if params == nil {
			continue
		}
		if got, _ := params["subscription"].(string); got != subscriptionID {
			continue
		}
		result, _ := params["result"].(map[string]any)
		if result != nil {
			return result
		}
	}
	s.T().Fatalf("timed out waiting for newHeads notification on subscription %s", subscriptionID)
	return nil
}

func (s *lumeraValidatorSuite) requireSuccessfulReceipt(receipt map[string]any, txHash string) {
	statusHex, _ := receipt["status"].(string)
	s.Require().Equal("0x1", statusHex, "expected successful tx status")
	gotHash, _ := receipt["transactionHash"].(string)
	s.Require().Equal(strings.ToLower(txHash), strings.ToLower(gotHash), "receipt tx hash mismatch")
	s.Require().NotEmpty(receipt["blockHash"], "receipt missing blockHash")
	s.Require().NotEmpty(receipt["blockNumber"], "receipt missing blockNumber")
}

func (s *lumeraValidatorSuite) requireContractReturnsUint64(rpcAddr, contractAddress string, want uint64) {
	var callResult string
	err := callJSONRPC(rpcAddr, "eth_call", []any{
		map[string]any{
			"to":   contractAddress,
			"data": "0x",
		},
		"latest",
	}, &callResult)
	s.Require().NoError(err, "eth_call contract %s", contractAddress)
	s.requireUint256Hex(callResult, want)
}

func (s *lumeraValidatorSuite) requireReceiptHasTopic(receipt map[string]any, topic string) {
	logs, ok := receipt["logs"].([]any)
	s.Require().True(ok, "receipt logs has unexpected type: %#v", receipt["logs"])
	for _, rawLog := range logs {
		logObj, _ := rawLog.(map[string]any)
		topics, _ := logObj["topics"].([]any)
		for _, rawTopic := range topics {
			if got, _ := rawTopic.(string); strings.EqualFold(got, topic) {
				return
			}
		}
	}
	s.T().Fatalf("receipt did not contain topic %s: %#v", topic, receipt)
}

func (s *lumeraValidatorSuite) requireUint256Hex(hexValue string, want uint64) {
	got := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(hexValue)), "0x")
	if got == "" {
		s.T().Fatalf("eth_call returned empty result")
	}
	if len(got)%2 != 0 {
		got = "0" + got
	}
	if len(got) < 16 {
		got = strings.Repeat("0", 16-len(got)) + got
	}
	low64 := got[len(got)-16:]
	wantLow64 := hex.EncodeToString([]byte{
		byte(want >> 56), byte(want >> 48), byte(want >> 40), byte(want >> 32),
		byte(want >> 24), byte(want >> 16), byte(want >> 8), byte(want),
	})
	s.Require().Equal(wantLow64, low64, "unexpected uint256 return value: %s", hexValue)
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

func (s *lumeraValidatorSuite) mustEthCallRaw(rpcAddr, to string, input []byte, label string) []byte {
	result, err := ethCallRaw(rpcAddr, to, input)
	s.Require().NoError(err, "eth_call %s", label)
	s.Require().NotEmpty(result, "eth_call %s returned empty result", label)
	return result
}

func ethCallRaw(rpcAddr, to string, input []byte) ([]byte, error) {
	var resultHex string
	err := callJSONRPC(rpcAddr, "eth_call", []any{
		map[string]any{
			"to":   to,
			"data": "0x" + hex.EncodeToString(input),
		},
		"latest",
	}, &resultHex)
	if err != nil {
		return nil, err
	}
	result := common.FromHex(resultHex)
	if len(result) == 0 {
		return nil, fmt.Errorf("empty eth_call result")
	}
	return result, nil
}

func (s *lumeraValidatorSuite) mustRestartLocalLumerad() {
	script := strings.TrimSpace(os.Getenv("LUMERA_RESTART_SCRIPT"))
	if script == "" {
		script = "/root/scripts/restart.sh"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, script, "lumera")
	out, err := cmd.CombinedOutput()
	s.Require().NoError(err, "restart local lumerad with %s: %s", script, strings.TrimSpace(string(out)))
}

func (s *lumeraValidatorSuite) mustWaitJSONRPCReady(rpcAddr string, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		var blockNumber string
		lastErr = callJSONRPC(rpcAddr, "eth_blockNumber", []any{}, &blockNumber)
		if lastErr == nil && strings.HasPrefix(blockNumber, "0x") {
			return
		}
		time.Sleep(2 * time.Second)
	}
	s.T().Fatalf("timed out waiting for EVM JSON-RPC after restart: %v", lastErr)
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

func resolveLumeraJSONWS(rpcAddr string) string {
	if explicit := strings.TrimSpace(os.Getenv("LUMERA_JSONWS_ADDR")); explicit != "" {
		return explicit
	}

	if ports, err := loadLocalLumeradPorts(); err == nil && ports.JSONWS > 0 {
		return fmt.Sprintf("ws://127.0.0.1:%d", ports.JSONWS)
	}

	if strings.TrimSpace(rpcAddr) == "" {
		return "ws://supernova_validator_1:8546"
	}
	if strings.Contains(rpcAddr, ":26657") {
		return strings.Replace(strings.Replace(rpcAddr, "http://", "ws://", 1), ":26657", ":8546", 1)
	}

	u, err := url.Parse(rpcAddr)
	if err != nil || u.Host == "" {
		return "ws://supernova_validator_1:8546"
	}
	host := u.Hostname()
	u.Host = host + ":8546"
	switch u.Scheme {
	case "https", "wss":
		u.Scheme = "wss"
	default:
		u.Scheme = "ws"
	}
	return u.String()
}

func loggingConstantContractCreationCode(topicHex string) []byte {
	topic := common.FromHex(topicHex)
	runtime := evmprogram.New().
		Push(42).Push(0).Op(vm.MSTORE).
		Return(0, 32).
		Bytes()

	return evmprogram.New().
		Push(topic).Push(0).Push(0).Op(vm.LOG1).
		ReturnViaCodeCopy(runtime).
		Bytes()
}

func abiCallUint64(signature string, value uint64) []byte {
	selector := crypto.Keccak256([]byte(signature))[:4]
	arg := make([]byte, 32)
	big.NewInt(0).SetUint64(value).FillBytes(arg)
	return append(selector, arg...)
}

func abiCallNoArgs(signature string) []byte {
	return crypto.Keccak256([]byte(signature))[:4]
}

func abiCallAddressUint64(signature string, addr common.Address, value uint64) []byte {
	selector := crypto.Keccak256([]byte(signature))[:4]
	addrArg := make([]byte, 32)
	copy(addrArg[12:], addr.Bytes())
	valueArg := make([]byte, 32)
	big.NewInt(0).SetUint64(value).FillBytes(valueArg)

	out := append([]byte{}, selector...)
	out = append(out, addrArg...)
	out = append(out, valueArg...)
	return out
}

func postJSONRPCStatus(rpcAddr, method string, params any) (int, error) {
	body := rpcRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  method,
		Params:  params,
	}
	bz, err := json.Marshal(body)
	if err != nil {
		return 0, fmt.Errorf("marshal %s request: %w", method, err)
	}

	req, err := http.NewRequest(http.MethodPost, rpcAddr, bytes.NewReader(bz))
	if err != nil {
		return 0, fmt.Errorf("build %s request: %w", method, err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: defaultRPCTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("call %s: %w", method, err)
	}
	defer resp.Body.Close()
	return resp.StatusCode, nil
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
