//go:build integration
// +build integration

package feemarket_test

import (
	"bytes"
	lcfg "github.com/LumeraProtocol/lumera/config"
	evmtest "github.com/LumeraProtocol/lumera/tests/integration/evmtest"
	testaccounts "github.com/LumeraProtocol/lumera/testutil/accounts"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

// TestFeeHistoryReportsCanonicalShape checks basic eth_feeHistory response
// invariants (array sizes, numeric formats, and non-zero base-fee presence).
func testFeeHistoryReportsCanonicalShape(t *testing.T, node *evmtest.Node) {
	t.Helper()

	// Produce a few blocks with EVM tx load so gas usage and fee history are populated.
	for i := 0; i < 3; i++ {
		txHash := evmtest.SendOneLegacyTx(t, node.RPCURL(), node.KeyInfo())
		evmtest.WaitForReceipt(t, node.RPCURL(), txHash, node.WaitCh(), node.OutputBuffer(), 40*time.Second)
	}

	var resp map[string]any
	evmtest.MustJSONRPC(t, node.RPCURL(), "eth_feeHistory", []any{"0x3", "latest", []any{}}, &resp)
	if resp == nil {
		t.Fatalf("eth_feeHistory returned nil response")
	}

	oldest := evmtest.MustStringField(t, resp, "oldestBlock")
	if _, err := hexutil.DecodeUint64(oldest); err != nil {
		t.Fatalf("invalid oldestBlock %q: %v", oldest, err)
	}

	baseFeesRaw, ok := resp["baseFeePerGas"].([]any)
	if !ok {
		t.Fatalf("baseFeePerGas has unexpected shape: %#v", resp["baseFeePerGas"])
	}
	if len(baseFeesRaw) != 4 { // blockCount + 1
		t.Fatalf("baseFeePerGas length mismatch: got %d want 4", len(baseFeesRaw))
	}
	nonZeroFound := false
	for i, v := range baseFeesRaw {
		feeHex, ok := v.(string)
		if !ok {
			t.Fatalf("baseFeePerGas[%d] is not string: %#v", i, v)
		}
		fee, err := hexutil.DecodeBig(feeHex)
		if err != nil {
			t.Fatalf("invalid baseFeePerGas[%d]=%q: %v", i, feeHex, err)
		}
		if fee.Sign() > 0 {
			nonZeroFound = true
		}
	}
	if !nonZeroFound {
		t.Fatalf("expected at least one non-zero baseFeePerGas entry: %#v", baseFeesRaw)
	}

	ratiosRaw, ok := resp["gasUsedRatio"].([]any)
	if !ok {
		t.Fatalf("gasUsedRatio has unexpected shape: %#v", resp["gasUsedRatio"])
	}
	if len(ratiosRaw) != 3 { // blockCount
		t.Fatalf("gasUsedRatio length mismatch: got %d want 3", len(ratiosRaw))
	}
}

// TestReceiptEffectiveGasPriceRespectsBlockBaseFee verifies that receipt
// effectiveGasPrice is not below block baseFeePerGas for included txs.
func testReceiptEffectiveGasPriceRespectsBlockBaseFee(t *testing.T, node *evmtest.Node) {
	t.Helper()

	txHash := evmtest.SendOneLegacyTx(t, node.RPCURL(), node.KeyInfo())
	receipt := evmtest.WaitForReceipt(t, node.RPCURL(), txHash, node.WaitCh(), node.OutputBuffer(), 40*time.Second)
	evmtest.AssertReceiptMatchesTxHash(t, receipt, txHash)

	effectiveGasPriceHex := evmtest.MustStringField(t, receipt, "effectiveGasPrice")
	effectiveGasPrice, err := hexutil.DecodeBig(effectiveGasPriceHex)
	if err != nil {
		t.Fatalf("invalid effectiveGasPrice %q: %v", effectiveGasPriceHex, err)
	}
	if effectiveGasPrice.Sign() <= 0 {
		t.Fatalf("effectiveGasPrice should be positive, got %s", effectiveGasPrice)
	}

	blockNumberHex := evmtest.MustStringField(t, receipt, "blockNumber")
	block := evmtest.MustGetBlock(t, node.RPCURL(), "eth_getBlockByNumber", []any{blockNumberHex, false})
	baseFeeHex := evmtest.MustStringField(t, block, "baseFeePerGas")
	baseFee, err := hexutil.DecodeBig(baseFeeHex)
	if err != nil {
		t.Fatalf("invalid block baseFeePerGas %q: %v", baseFeeHex, err)
	}

	if effectiveGasPrice.Cmp(baseFee) < 0 {
		t.Fatalf(
			"effectiveGasPrice must be >= base fee: effective=%s base=%s",
			effectiveGasPrice.String(),
			baseFee.String(),
		)
	}

	// Fee history for this height should include the block base fee.
	var resp map[string]any
	evmtest.MustJSONRPC(t, node.RPCURL(), "eth_feeHistory", []any{"0x1", blockNumberHex, []any{}}, &resp)

	baseFeesRaw, ok := resp["baseFeePerGas"].([]any)
	if !ok || len(baseFeesRaw) < 1 {
		t.Fatalf("unexpected baseFeePerGas from feeHistory: %#v", resp["baseFeePerGas"])
	}
	feeHistoryBaseHex, ok := baseFeesRaw[0].(string)
	if !ok {
		t.Fatalf("unexpected feeHistory baseFee entry: %#v", baseFeesRaw[0])
	}
	feeHistoryBase, err := hexutil.DecodeBig(feeHistoryBaseHex)
	if err != nil {
		t.Fatalf("invalid feeHistory baseFee %q: %v", feeHistoryBaseHex, err)
	}
	if feeHistoryBase.Cmp(baseFee) != 0 {
		t.Fatalf(
			"feeHistory base fee mismatch: feeHistory=%s block=%s",
			feeHistoryBase.String(),
			baseFee.String(),
		)
	}

}

// TestFeeHistoryRewardPercentilesShape verifies percentile reward matrix shape
// and value decodability when fee history is requested with reward percentiles.
func testFeeHistoryRewardPercentilesShape(t *testing.T, node *evmtest.Node) {
	t.Helper()

	// Generate EVM activity so fee history contains non-empty sampled blocks.
	for i := 0; i < 2; i++ {
		txHash := evmtest.SendOneLegacyTx(t, node.RPCURL(), node.KeyInfo())
		evmtest.WaitForReceipt(t, node.RPCURL(), txHash, node.WaitCh(), node.OutputBuffer(), 40*time.Second)
	}

	var resp map[string]any
	evmtest.MustJSONRPC(t, node.RPCURL(), "eth_feeHistory", []any{"0x2", "latest", []any{10.0, 50.0, 90.0}}, &resp)
	if resp == nil {
		t.Fatalf("eth_feeHistory returned nil response")
	}

	rewardRowsRaw, ok := resp["reward"].([]any)
	if !ok {
		t.Fatalf("reward has unexpected shape: %#v", resp["reward"])
	}
	if len(rewardRowsRaw) != 2 {
		t.Fatalf("reward rows length mismatch: got %d want 2", len(rewardRowsRaw))
	}

	for i, rowRaw := range rewardRowsRaw {
		row, ok := rowRaw.([]any)
		if !ok {
			t.Fatalf("reward[%d] has unexpected shape: %#v", i, rowRaw)
		}
		if len(row) != 3 {
			t.Fatalf("reward[%d] percentile count mismatch: got %d want 3", i, len(row))
		}
		for j, cell := range row {
			feeHex, ok := cell.(string)
			if !ok {
				t.Fatalf("reward[%d][%d] is not string: %#v", i, j, cell)
			}
			if _, err := hexutil.DecodeBig(feeHex); err != nil {
				t.Fatalf("invalid reward[%d][%d]=%q: %v", i, j, feeHex, err)
			}
		}
	}
}

// TestMaxPriorityFeePerGasReturnsValidHex checks response format and non-negative
// semantics of eth_maxPriorityFeePerGas.
func testMaxPriorityFeePerGasReturnsValidHex(t *testing.T, node *evmtest.Node) {
	t.Helper()

	// Ensure at least one block with EVM activity has been produced before querying.
	txHash := evmtest.SendOneLegacyTx(t, node.RPCURL(), node.KeyInfo())
	evmtest.WaitForReceipt(t, node.RPCURL(), txHash, node.WaitCh(), node.OutputBuffer(), 40*time.Second)

	var feeHex string
	evmtest.MustJSONRPC(t, node.RPCURL(), "eth_maxPriorityFeePerGas", []any{}, &feeHex)
	fee, err := hexutil.DecodeBig(feeHex)
	if err != nil {
		t.Fatalf("invalid eth_maxPriorityFeePerGas %q: %v", feeHex, err)
	}
	if fee.Sign() < 0 {
		t.Fatalf("eth_maxPriorityFeePerGas must be non-negative, got %s", fee.String())
	}
}

// TestGasPriceIsAtLeastLatestBaseFee ensures eth_gasPrice respects base-fee
// floor semantics on the latest block.
func testGasPriceIsAtLeastLatestBaseFee(t *testing.T, node *evmtest.Node) {
	t.Helper()

	// Create at least one tx so latest block has deterministic EVM activity.
	txHash := evmtest.SendOneLegacyTx(t, node.RPCURL(), node.KeyInfo())
	receipt := evmtest.WaitForReceipt(t, node.RPCURL(), txHash, node.WaitCh(), node.OutputBuffer(), 40*time.Second)
	evmtest.AssertReceiptMatchesTxHash(t, receipt, txHash)

	var gasPriceHex string
	evmtest.MustJSONRPC(t, node.RPCURL(), "eth_gasPrice", []any{}, &gasPriceHex)
	gasPrice, err := hexutil.DecodeBig(gasPriceHex)
	if err != nil {
		t.Fatalf("invalid eth_gasPrice %q: %v", gasPriceHex, err)
	}

	latestBlock := evmtest.MustGetBlock(t, node.RPCURL(), "eth_getBlockByNumber", []any{"latest", false})
	baseFeeHex := evmtest.MustStringField(t, latestBlock, "baseFeePerGas")
	baseFee, err := hexutil.DecodeBig(baseFeeHex)
	if err != nil {
		t.Fatalf("invalid latest baseFeePerGas %q: %v", baseFeeHex, err)
	}

	if gasPrice.Cmp(baseFee) < 0 {
		t.Fatalf("eth_gasPrice must be >= latest base fee: gasPrice=%s baseFee=%s", gasPrice.String(), baseFee.String())
	}
}

// TestDynamicFeeType2EffectiveGasPriceFormula verifies type-2 tx processing and
// receipt effective gas price calculation:
// effectiveGasPrice == min(maxFeePerGas, blockBaseFee + maxPriorityFeePerGas).
func testDynamicFeeType2EffectiveGasPriceFormula(t *testing.T, node *evmtest.Node) {
	t.Helper()

	fromAddr := testaccounts.MustAccountAddressFromTestKeyInfo(t, node.KeyInfo())
	privateKey := evmtest.MustDerivePrivateKey(t, node.KeyInfo().Mnemonic)
	nonce := evmtest.MustGetPendingNonceWithRetry(t, node.RPCURL(), fromAddr.Hex(), 20*time.Second)

	latestBlock := evmtest.MustGetBlock(t, node.RPCURL(), "eth_getBlockByNumber", []any{"latest", false})
	baseFee := mustHexBig(t, evmtest.MustStringField(t, latestBlock, "baseFeePerGas"))

	tipCap := big.NewInt(2_000_000_000)
	maxFeeCap := new(big.Int).Add(baseFee, new(big.Int).Mul(tipCap, big.NewInt(2)))
	to := common.HexToAddress(fromAddr.Hex())

	txHash := evmtest.SendDynamicFeeTxWithParams(t, node.RPCURL(), evmtest.DynamicFeeTxParams{
		PrivateKey: privateKey,
		Nonce:      nonce,
		To:         &to,
		Value:      big.NewInt(0),
		Gas:        100_000,
		GasFeeCap:  maxFeeCap,
		GasTipCap:  tipCap,
		Data:       nil,
	})

	receipt := evmtest.WaitForReceipt(t, node.RPCURL(), txHash, node.WaitCh(), node.OutputBuffer(), 45*time.Second)
	evmtest.AssertReceiptMatchesTxHash(t, receipt, txHash)
	effectiveGasPrice := mustHexBig(t, evmtest.MustStringField(t, receipt, "effectiveGasPrice"))

	txObj := evmtest.WaitForTransactionByHash(t, node.RPCURL(), txHash, node.WaitCh(), node.OutputBuffer(), 45*time.Second)
	evmtest.AssertTxObjectMatchesHash(t, txObj, txHash)
	txType := evmtest.MustStringField(t, txObj, "type")
	if !strings.EqualFold(txType, "0x2") {
		t.Fatalf("expected type-2 tx, got type=%s tx=%#v", txType, txObj)
	}

	txMaxFee := mustHexBig(t, evmtest.MustStringField(t, txObj, "maxFeePerGas"))
	txMaxPriorityFee := mustHexBig(t, evmtest.MustStringField(t, txObj, "maxPriorityFeePerGas"))

	blockNumberHex := evmtest.MustStringField(t, receipt, "blockNumber")
	includedBlock := evmtest.MustGetBlock(t, node.RPCURL(), "eth_getBlockByNumber", []any{blockNumberHex, false})
	includedBaseFee := mustHexBig(t, evmtest.MustStringField(t, includedBlock, "baseFeePerGas"))

	expectedEffective := new(big.Int).Add(includedBaseFee, txMaxPriorityFee)
	if expectedEffective.Cmp(txMaxFee) > 0 {
		expectedEffective = new(big.Int).Set(txMaxFee)
	}

	if effectiveGasPrice.Cmp(expectedEffective) != 0 {
		t.Fatalf(
			"unexpected effectiveGasPrice: got=%s want=%s (baseFee=%s maxFee=%s tip=%s)",
			effectiveGasPrice.String(),
			expectedEffective.String(),
			includedBaseFee.String(),
			txMaxFee.String(),
			txMaxPriorityFee.String(),
		)
	}
}

// TestDynamicFeeType2RejectsFeeCapBelowBaseFee ensures tx submission fails when
// maxFeePerGas is strictly below current block base fee.
func testDynamicFeeType2RejectsFeeCapBelowBaseFee(t *testing.T, node *evmtest.Node) {
	t.Helper()

	// Produce one tx first so latest base fee context is initialized/stable.
	seedTxHash := evmtest.SendOneLegacyTx(t, node.RPCURL(), node.KeyInfo())
	evmtest.WaitForReceipt(t, node.RPCURL(), seedTxHash, node.WaitCh(), node.OutputBuffer(), 40*time.Second)

	fromAddr := testaccounts.MustAccountAddressFromTestKeyInfo(t, node.KeyInfo())
	privateKey := evmtest.MustDerivePrivateKey(t, node.KeyInfo().Mnemonic)
	nonce := evmtest.MustGetPendingNonceWithRetry(t, node.RPCURL(), fromAddr.Hex(), 20*time.Second)

	latestBlock := evmtest.MustGetBlock(t, node.RPCURL(), "eth_getBlockByNumber", []any{"latest", false})
	baseFee := mustHexBig(t, evmtest.MustStringField(t, latestBlock, "baseFeePerGas"))
	if baseFee.Sign() <= 0 {
		t.Fatalf("expected positive baseFeePerGas, got %s", baseFee.String())
	}

	feeCapBelowBase := new(big.Int).Sub(baseFee, big.NewInt(1))
	// Keep tip <= fee cap so the tx fails on "fee cap below base fee" rather
	// than the unrelated "tip higher than max fee" validation.
	tipCap := new(big.Int).Set(feeCapBelowBase)
	to := common.HexToAddress(fromAddr.Hex())

	txHash, err := evmtest.SendDynamicFeeTxWithParamsResult(node.RPCURL(), evmtest.DynamicFeeTxParams{
		PrivateKey: privateKey,
		Nonce:      nonce,
		To:         &to,
		Value:      big.NewInt(0),
		Gas:        100_000,
		GasFeeCap:  feeCapBelowBase,
		GasTipCap:  tipCap,
		Data:       nil,
	})
	if err == nil {
		t.Fatalf("expected rejection for fee cap below base fee, got tx hash %s", txHash)
	}
	if !strings.Contains(strings.ToLower(err.Error()), "max fee per gas less than block base fee") {
		t.Fatalf("unexpected error for below-base-fee tx: %v", err)
	}
}

// TestBaseFeeProgressesAcrossMultiBlockLoadPattern validates long-run base-fee
// behavior under sustained high usage followed by sustained empty blocks.
func testBaseFeeProgressesAcrossMultiBlockLoadPattern(t *testing.T, node *evmtest.Node) {
	t.Helper()

	const (
		highUsageBursts       = 8
		heavyTxPerBurst       = 26
		lowEmptyBlocks        = 14
		minObservedBlocks     = 20
		heavyPayloadBytes     = 12 * 1024
		heavyTransferGasLimit = 260_000
		gasPriceMultiplier    = 6
	)

	evmtest.WaitForBlockNumberAtLeast(t, node.RPCURL(), 1, 20*time.Second)

	fromAddr := testaccounts.MustAccountAddressFromTestKeyInfo(t, node.KeyInfo())
	privateKey := evmtest.MustDerivePrivateKey(t, node.KeyInfo().Mnemonic)
	toAddr := common.HexToAddress(fromAddr.Hex())
	nextNonce := evmtest.MustGetPendingNonceWithRetry(t, node.RPCURL(), fromAddr.Hex(), 20*time.Second)

	minBaseFeeFloorWei := mustULumeDecToWei(t, lcfg.FeeMarketMinGasPrice)
	startHeight := evmtest.MustGetBlockNumber(t, node.RPCURL())
	startBaseFee := mustBaseFeeAtHeight(t, node, startHeight)
	if startBaseFee.Cmp(minBaseFeeFloorWei) < 0 {
		t.Fatalf(
			"start base fee below configured floor: start=%s floor=%s height=%d",
			startBaseFee.String(),
			minBaseFeeFloorWei.String(),
			startHeight,
		)
	}

	heavyPayload := bytes.Repeat([]byte{0x01}, heavyPayloadBytes)
	var lastTxHash string

	for burst := 0; burst < highUsageBursts; burst++ {
		burstStartHeight := evmtest.MustGetBlockNumber(t, node.RPCURL())
		gasPrice := evmtest.MustGetGasPriceWithRetry(t, node.RPCURL(), 20*time.Second)
		burstGasPrice := new(big.Int).Mul(gasPrice, big.NewInt(gasPriceMultiplier))

		for i := 0; i < heavyTxPerBurst; i++ {
			lastTxHash = evmtest.SendLegacyTxWithParams(t, node.RPCURL(), evmtest.LegacyTxParams{
				PrivateKey: privateKey,
				Nonce:      nextNonce,
				To:         &toAddr,
				Value:      big.NewInt(1),
				Gas:        heavyTransferGasLimit,
				GasPrice:   burstGasPrice,
				Data:       heavyPayload,
			})
			nextNonce++
		}

		evmtest.WaitForBlockNumberAtLeast(t, node.RPCURL(), burstStartHeight+1, 45*time.Second)
	}

	if strings.TrimSpace(lastTxHash) == "" {
		t.Fatal("failed to submit high-usage burst transactions")
	}

	finalHighReceipt := evmtest.WaitForReceipt(t, node.RPCURL(), lastTxHash, node.WaitCh(), node.OutputBuffer(), 120*time.Second)
	highPhaseEndHeight := evmtest.MustUint64HexField(t, finalHighReceipt, "blockNumber")
	highPhaseEndBaseFee := mustBaseFeeAtHeight(t, node, highPhaseEndHeight)
	if highPhaseEndBaseFee.Cmp(startBaseFee) <= 0 {
		t.Fatalf(
			"expected base fee increase after sustained high usage: start=%s end=%s start_height=%d end_height=%d",
			startBaseFee.String(),
			highPhaseEndBaseFee.String(),
			startHeight,
			highPhaseEndHeight,
		)
	}

	lowPhaseTargetHeight := highPhaseEndHeight + lowEmptyBlocks
	evmtest.WaitForBlockNumberAtLeast(t, node.RPCURL(), lowPhaseTargetHeight, 120*time.Second)
	lowPhaseEndHeight := evmtest.MustGetBlockNumber(t, node.RPCURL())
	lowPhaseEndBaseFee := mustBaseFeeAtHeight(t, node, lowPhaseEndHeight)
	if lowPhaseEndBaseFee.Cmp(highPhaseEndBaseFee) > 0 {
		t.Fatalf(
			"expected base fee to decrease or stay flat after sustained empty period: high_end=%s low_end=%s high_height=%d low_height=%d",
			highPhaseEndBaseFee.String(),
			lowPhaseEndBaseFee.String(),
			highPhaseEndHeight,
			lowPhaseEndHeight,
		)
	}

	observedBlocks := lowPhaseEndHeight - startHeight
	if observedBlocks < minObservedBlocks {
		t.Fatalf(
			"insufficient consecutive blocks observed: got=%d want_at_least=%d start_height=%d end_height=%d",
			observedBlocks,
			minObservedBlocks,
			startHeight,
			lowPhaseEndHeight,
		)
	}

	for height := startHeight; height <= lowPhaseEndHeight; height++ {
		fee := mustBaseFeeAtHeight(t, node, height)
		if fee.Cmp(minBaseFeeFloorWei) < 0 {
			t.Fatalf(
				"base fee dropped below configured floor: fee=%s floor=%s height=%d",
				fee.String(),
				minBaseFeeFloorWei.String(),
				height,
			)
		}
	}
}

func mustBaseFeeAtHeight(t *testing.T, node *evmtest.Node, height uint64) *big.Int {
	t.Helper()

	block := evmtest.MustGetBlock(t, node.RPCURL(), "eth_getBlockByNumber", []any{hexutil.EncodeUint64(height), false})
	return mustHexBig(t, evmtest.MustStringField(t, block, "baseFeePerGas"))
}

func mustULumeDecToWei(t *testing.T, decValue string) *big.Int {
	t.Helper()

	parsed, ok := new(big.Rat).SetString(decValue)
	if !ok {
		t.Fatalf("invalid decimal value %q", decValue)
	}

	scaled := new(big.Rat).Mul(parsed, new(big.Rat).SetInt(big.NewInt(1_000_000_000_000)))
	if scaled.Denom().Cmp(big.NewInt(1)) != 0 {
		t.Fatalf("decimal value %q is not convertible to exact wei integer: %s", decValue, scaled.RatString())
	}

	return new(big.Int).Set(scaled.Num())
}

func mustHexBig(t *testing.T, hexValue string) *big.Int {
	t.Helper()
	v, err := hexutil.DecodeBig(hexValue)
	if err != nil {
		t.Fatalf("invalid hex big %q: %v", hexValue, err)
	}
	return v
}
