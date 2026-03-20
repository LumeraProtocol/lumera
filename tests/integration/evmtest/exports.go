//go:build integration
// +build integration

package evmtest

import (
	"context"
	"crypto/ecdsa"
	"testing"
	"time"
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

func SetMempoolMaxTxsInAppToml(t *testing.T, homeDir string, maxTxs int) {
	setMempoolMaxTxsInAppToml(t, homeDir, maxTxs)
}

func SetCometTxIndexer(t *testing.T, homeDir, indexer string) {
	setCometTxIndexer(t, homeDir, indexer)
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

func SendLegacyTxWithParamsResult(rpcURL string, p LegacyTxParams) (string, error) {
	return sendLegacyTxWithParamsResult(rpcURL, p)
}

func SignedLegacyTxBytes(p LegacyTxParams) ([]byte, error) {
	return signedLegacyTxBytes(p)
}

func SendDynamicFeeTxWithParamsResult(rpcURL string, p DynamicFeeTxParams) (string, error) {
	return sendDynamicFeeTxWithParamsResult(rpcURL, p)
}

func SignedDynamicFeeTxBytes(p DynamicFeeTxParams) ([]byte, error) {
	return signedDynamicFeeTxBytes(p)
}

func MustDerivePrivateKey(t *testing.T, mnemonic string) *ecdsa.PrivateKey {
	return mustDerivePrivateKey(t, mnemonic)
}

func TopicWordBytes(topicHex string) []byte {
	return topicWordBytes(topicHex)
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
