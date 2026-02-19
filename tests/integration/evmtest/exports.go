//go:build integration
// +build integration

package evmtest

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"math/big"
	"testing"
	"time"

	testaccounts "github.com/LumeraProtocol/lumera/testutil/accounts"
)

const EVMChainID = evmChainID

type Node = evmNode

type LegacyTxParams = legacyTxParams

type DynamicFeeTxParams = dynamicFeeTxParams

func NewEVMNode(t *testing.T, chainID string, haltHeight int) *Node {
	return newEVMNode(t, chainID, haltHeight)
}

func SetIndexerEnabledInAppToml(t *testing.T, homeDir string, enabled bool) {
	setIndexerEnabledInAppToml(t, homeDir, enabled)
}

func SetEVMMempoolPriceBumpInAppToml(t *testing.T, homeDir string, priceBump uint64) {
	setEVMMempoolPriceBumpInAppToml(t, homeDir, priceBump)
}

func SetCometTxIndexer(t *testing.T, homeDir, indexer string) {
	setCometTxIndexer(t, homeDir, indexer)
}

func SendOneLegacyTx(t *testing.T, rpcURL string, keyInfo testaccounts.TestKeyInfo) string {
	return sendOneLegacyTx(t, rpcURL, keyInfo)
}

func SendOneCosmosBankTx(t *testing.T, node *Node) string {
	return sendOneCosmosBankTx(t, node)
}

func SendOneCosmosBankTxWithFees(t *testing.T, node *Node, fees string) string {
	return sendOneCosmosBankTxWithFees(t, node, fees)
}

func SendOneCosmosBankTxWithFeesResult(t *testing.T, node *Node, fees string) (string, error) {
	return sendOneCosmosBankTxWithFeesResult(t, node, fees)
}

func SendLogEmitterCreationTx(t *testing.T, rpcURL string, keyInfo testaccounts.TestKeyInfo, topicHex string) string {
	return sendLogEmitterCreationTx(t, rpcURL, keyInfo, topicHex)
}

func SendLegacyTxWithParams(t *testing.T, rpcURL string, p LegacyTxParams) string {
	return sendLegacyTxWithParams(t, rpcURL, p)
}

func SendLegacyTxWithParamsResult(rpcURL string, p LegacyTxParams) (string, error) {
	return sendLegacyTxWithParamsResult(rpcURL, p)
}

func SignedLegacyTxBytes(p LegacyTxParams) ([]byte, error) {
	return signedLegacyTxBytes(p)
}

func SendDynamicFeeTxWithParams(t *testing.T, rpcURL string, p DynamicFeeTxParams) string {
	return sendDynamicFeeTxWithParams(t, rpcURL, p)
}

func SendDynamicFeeTxWithParamsResult(rpcURL string, p DynamicFeeTxParams) (string, error) {
	return sendDynamicFeeTxWithParamsResult(rpcURL, p)
}

func SignedDynamicFeeTxBytes(p DynamicFeeTxParams) ([]byte, error) {
	return signedDynamicFeeTxBytes(p)
}

func MustGetPendingNonceWithRetry(t *testing.T, rpcURL, fromHex string, timeout time.Duration) uint64 {
	return mustGetPendingNonceWithRetry(t, rpcURL, fromHex, timeout)
}

func MustGetGasPriceWithRetry(t *testing.T, rpcURL string, timeout time.Duration) *big.Int {
	return mustGetGasPriceWithRetry(t, rpcURL, timeout)
}

func MustDerivePrivateKey(t *testing.T, mnemonic string) *ecdsa.PrivateKey {
	return mustDerivePrivateKey(t, mnemonic)
}

func TopicWordBytes(topicHex string) []byte {
	return topicWordBytes(topicHex)
}

func WaitForReceipt(
	t *testing.T,
	rpcURL, txHash string,
	waitCh <-chan error,
	output *bytes.Buffer,
	timeout time.Duration,
) map[string]any {
	return waitForReceipt(t, rpcURL, txHash, waitCh, output, timeout)
}

func WaitForTransactionByHash(
	t *testing.T,
	rpcURL, txHash string,
	waitCh <-chan error,
	output *bytes.Buffer,
	timeout time.Duration,
) map[string]any {
	return waitForTransactionByHash(t, rpcURL, txHash, waitCh, output, timeout)
}

func MustGetBlock(t *testing.T, rpcURL, method string, params []any) map[string]any {
	return mustGetBlock(t, rpcURL, method, params)
}

func MustGetLogs(t *testing.T, rpcURL string, filter map[string]any) []map[string]any {
	return mustGetLogs(t, rpcURL, filter)
}

func AssertReceiptMatchesTxHash(t *testing.T, receipt map[string]any, txHash string) {
	assertReceiptMatchesTxHash(t, receipt, txHash)
}

func AssertTxObjectMatchesHash(t *testing.T, txObj map[string]any, txHash string) {
	assertTxObjectMatchesHash(t, txObj, txHash)
}

func AssertTxFieldStable(t *testing.T, field string, before, after map[string]any) {
	assertTxFieldStable(t, field, before, after)
}

func AssertBlockContainsTxHash(t *testing.T, block map[string]any, txHash string) {
	assertBlockContainsTxHash(t, block, txHash)
}

func AssertBlockContainsFullTx(t *testing.T, block map[string]any, txHash string) {
	assertBlockContainsFullTx(t, block, txHash)
}

func MustStringField(t *testing.T, m map[string]any, field string) string {
	return mustStringField(t, m, field)
}

func MustUint64HexField(t *testing.T, m map[string]any, field string) uint64 {
	return mustUint64HexField(t, m, field)
}

func WaitForJSONRPC(t *testing.T, rpcURL string, waitCh <-chan error, output *bytes.Buffer) {
	waitForJSONRPC(t, rpcURL, waitCh, output)
}

func MustJSONRPC(t *testing.T, rpcURL, method string, params []any, out any) {
	mustJSONRPC(t, rpcURL, method, params, out)
}

func MustGetBlockNumber(t *testing.T, rpcURL string) uint64 {
	return mustGetBlockNumber(t, rpcURL)
}

func WaitForBlockNumberAtLeast(t *testing.T, rpcURL string, minBlock uint64, timeout time.Duration) {
	waitForBlockNumberAtLeast(t, rpcURL, minBlock, timeout)
}

func WaitForCosmosTxHeight(t *testing.T, node *Node, txHash string, timeout time.Duration) uint64 {
	return waitForCosmosTxHeight(t, node, txHash, timeout)
}

func MustGetCometBlockTxs(t *testing.T, node *Node, height uint64) []string {
	return mustGetCometBlockTxs(t, node, height)
}

func AssertContains(t *testing.T, output, needle string) {
	assertContains(t, output, needle)
}

func CometTxHashesFromBase64(t *testing.T, txs []string) []string {
	return cometTxHashesFromBase64(t, txs)
}

func RunCommand(ctx context.Context, workDir, bin string, args ...string) (string, error) {
	return run(ctx, workDir, bin, args...)
}
