//go:build integration
// +build integration

package jsonrpc_test

import (
	evmtest "github.com/LumeraProtocol/lumera/tests/integration/evmtest"
	"strings"
	"testing"
	"time"
)

// TestReceiptIncludesCanonicalFields validates that receipts include the
// canonical Ethereum fields expected by downstream tooling.
func testReceiptIncludesCanonicalFields(t *testing.T, node *evmtest.Node) {
	t.Helper()

	txHash := evmtest.SendOneLegacyTx(t, node.RPCURL(), node.KeyInfo())
	receipt := evmtest.WaitForReceipt(t, node.RPCURL(), txHash, node.WaitCh(), node.OutputBuffer(), 40*time.Second)
	evmtest.AssertReceiptMatchesTxHash(t, receipt, txHash)

	requiredFields := []string{
		"status",
		"cumulativeGasUsed",
		"logsBloom",
		"logs",
		"gasUsed",
		"blockHash",
		"blockNumber",
		"transactionIndex",
		"effectiveGasPrice",
		"from",
		"to",
		"type",
	}
	for _, field := range requiredFields {
		v, ok := receipt[field]
		if !ok || v == nil {
			t.Fatalf("receipt field %q is missing: %#v", field, receipt)
		}
	}

	if from := evmtest.MustStringField(t, receipt, "from"); strings.TrimSpace(from) == "" {
		t.Fatalf("receipt field from is unexpectedly empty: %#v", receipt)
	}
	if to := evmtest.MustStringField(t, receipt, "to"); strings.TrimSpace(to) == "" {
		t.Fatalf("receipt field to is unexpectedly empty: %#v", receipt)
	}
}
