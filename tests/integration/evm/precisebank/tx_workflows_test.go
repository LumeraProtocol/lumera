//go:build integration
// +build integration

package precisebank_test

import (
	"math/big"
	"testing"
	"time"

	lcfg "github.com/LumeraProtocol/lumera/config"
	evmtest "github.com/LumeraProtocol/lumera/tests/integration/evmtest"
	testaccounts "github.com/LumeraProtocol/lumera/testutil/accounts"
	addresscodec "github.com/cosmos/cosmos-sdk/codec/address"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
)

// TestPreciseBankEVMTransferSendSplitMatrix validates tx-level send behavior
// for values that hit both pure-fractional and integer+fractional paths.
//
// Matrix:
// 1. value < conversion factor (fractional-only recipient split)
// 2. value > conversion factor (integer + fractional recipient split)
//
// For each transfer, the test also verifies sender fee accounting and global
// precisebank remainder stability.
func testPreciseBankEVMTransferSendSplitMatrix(t *testing.T, node *evmtest.Node) {
	t.Helper()
	evmtest.WaitForBlockNumberAtLeast(t, node.RPCURL(), 1, 20*time.Second)

	senderAddr := testaccounts.MustAccountAddressFromTestKeyInfo(t, node.KeyInfo())
	senderPriv := evmtest.MustDerivePrivateKey(t, node.KeyInfo().Mnemonic)
	senderBech32 := node.KeyInfo().Address

	cf := conversionFactorBigInt()
	remainderBefore := mustQueryPrecisebankRemainder(t, node)

	testCases := []struct {
		name  string
		value *big.Int
	}{
		{
			name:  "fractional-only",
			value: big.NewInt(123_456_789),
		},
		{
			name:  "integer-and-fractional",
			value: new(big.Int).Add(new(big.Int).Set(cf), big.NewInt(77)),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			recipientPriv, err := crypto.GenerateKey()
			if err != nil {
				t.Fatalf("generate recipient key: %v", err)
			}
			recipientEthAddr := crypto.PubkeyToAddress(recipientPriv.PublicKey)
			recipientBech32 := mustAccAddressBech32(t, recipientEthAddr.Bytes())

			senderBefore := mustExtendedBalanceFromSplitQueries(t, node, senderBech32)
			recipientBefore := mustExtendedBalanceFromSplitQueries(t, node, recipientBech32)

			nonce := evmtest.MustGetPendingNonceWithRetry(t, node.RPCURL(), senderAddr.Hex(), 20*time.Second)
			gasPrice := evmtest.MustGetGasPriceWithRetry(t, node.RPCURL(), 20*time.Second)

			txHash := evmtest.SendLegacyTxWithParams(t, node.RPCURL(), evmtest.LegacyTxParams{
				PrivateKey: senderPriv,
				Nonce:      nonce,
				To:         &recipientEthAddr,
				Value:      tc.value,
				Gas:        21_000,
				GasPrice:   gasPrice,
			})
			receipt := evmtest.WaitForReceipt(t, node.RPCURL(), txHash, node.WaitCh(), node.OutputBuffer(), 40*time.Second)
			evmtest.AssertReceiptMatchesTxHash(t, receipt, txHash)

			gasUsed := evmtest.MustUint64HexField(t, receipt, "gasUsed")
			effectiveGasPriceHex := evmtest.MustStringField(t, receipt, "effectiveGasPrice")
			effectiveGasPrice, err := hexutil.DecodeBig(effectiveGasPriceHex)
			if err != nil {
				t.Fatalf("decode effectiveGasPrice %q: %v", effectiveGasPriceHex, err)
			}

			feePaid := new(big.Int).Mul(new(big.Int).SetUint64(gasUsed), effectiveGasPrice)

			senderAfter := mustExtendedBalanceFromSplitQueries(t, node, senderBech32)
			recipientAfter := mustExtendedBalanceFromSplitQueries(t, node, recipientBech32)

			// Sender pays both transferred value and gas fee.
			senderDelta := new(big.Int).Sub(senderBefore, senderAfter)
			wantSenderDelta := new(big.Int).Add(new(big.Int).Set(tc.value), feePaid)
			if senderDelta.Cmp(wantSenderDelta) != 0 {
				t.Fatalf(
					"unexpected sender delta: got=%s want=%s (value=%s fee=%s)",
					senderDelta.String(),
					wantSenderDelta.String(),
					tc.value.String(),
					feePaid.String(),
				)
			}

			// Recipient gets exactly the transferred value.
			recipientDelta := new(big.Int).Sub(recipientAfter, recipientBefore)
			if recipientDelta.Cmp(tc.value) != 0 {
				t.Fatalf("unexpected recipient delta: got=%s want=%s", recipientDelta.String(), tc.value.String())
			}

			// Split balance assertions for recipient.
			wantInt := new(big.Int).Quo(new(big.Int).Set(tc.value), cf)
			wantFrac := new(big.Int).Mod(new(big.Int).Set(tc.value), cf)

			gotInt := mustQueryBankBalanceDenom(t, node, recipientBech32, lcfg.ChainDenom)
			gotFrac := mustQueryPrecisebankFractionalBalance(t, node, recipientBech32)
			if gotInt.Cmp(wantInt) != 0 {
				t.Fatalf("recipient integer split mismatch: got=%s want=%s", gotInt.String(), wantInt.String())
			}
			if gotFrac.Cmp(wantFrac) != 0 {
				t.Fatalf("recipient fractional split mismatch: got=%s want=%s", gotFrac.String(), wantFrac.String())
			}
		})
	}

	remainderAfter := mustQueryPrecisebankRemainder(t, node)
	if remainderAfter.Cmp(remainderBefore) != 0 {
		t.Fatalf("remainder changed after send matrix: before=%s after=%s", remainderBefore.String(), remainderAfter.String())
	}
}

// TestPreciseBankSecondarySenderBurnMintWorkflow validates tx-level burn/mint
// behavior for a non-validator sender account.
//
// Workflow:
//  1. Fund a secondary EOA from validator.
//  2. Have that EOA send value to a third EOA.
//  3. Assert deterministic deltas:
//     - secondary sender decreases by (value + fee)
//     - recipient increases by value
//     - precisebank remainder remains stable.
func testPreciseBankSecondarySenderBurnMintWorkflow(t *testing.T, node *evmtest.Node) {
	t.Helper()
	evmtest.WaitForBlockNumberAtLeast(t, node.RPCURL(), 1, 20*time.Second)

	senderAddr := testaccounts.MustAccountAddressFromTestKeyInfo(t, node.KeyInfo())
	senderPriv := evmtest.MustDerivePrivateKey(t, node.KeyInfo().Mnemonic)

	secondaryPriv, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("generate secondary key: %v", err)
	}
	secondaryEthAddr := crypto.PubkeyToAddress(secondaryPriv.PublicKey)
	secondaryBech32 := mustAccAddressBech32(t, secondaryEthAddr.Bytes())

	recipientPriv, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("generate recipient key: %v", err)
	}
	recipientEthAddr := crypto.PubkeyToAddress(recipientPriv.PublicKey)
	recipientBech32 := mustAccAddressBech32(t, recipientEthAddr.Bytes())

	cf := conversionFactorBigInt()

	remainderBefore := mustQueryPrecisebankRemainder(t, node)

	// Step 1: fund secondary account with enough headroom for transfer value and
	// dynamic gas fees.
	fundAmount := new(big.Int).Add(big.NewInt(1_000_000_000_000_000), new(big.Int).Mul(big.NewInt(3), cf))
	fundNonce := evmtest.MustGetPendingNonceWithRetry(t, node.RPCURL(), senderAddr.Hex(), 20*time.Second)
	fundGasPrice := evmtest.MustGetGasPriceWithRetry(t, node.RPCURL(), 20*time.Second)

	fundHash := evmtest.SendLegacyTxWithParams(t, node.RPCURL(), evmtest.LegacyTxParams{
		PrivateKey: senderPriv,
		Nonce:      fundNonce,
		To:         &secondaryEthAddr,
		Value:      fundAmount,
		Gas:        21_000,
		GasPrice:   fundGasPrice,
	})
	fundReceipt := evmtest.WaitForReceipt(t, node.RPCURL(), fundHash, node.WaitCh(), node.OutputBuffer(), 40*time.Second)
	evmtest.AssertReceiptMatchesTxHash(t, fundReceipt, fundHash)

	secondaryBefore := mustExtendedBalanceFromSplitQueries(t, node, secondaryBech32)
	recipientBefore := mustExtendedBalanceFromSplitQueries(t, node, recipientBech32)

	// Step 2: secondary account sends to recipient, exercising sender burn +
	// recipient mint through tx-level state transition.
	sendAmount := new(big.Int).Add(new(big.Int).Set(cf), big.NewInt(42))
	sendNonce := evmtest.MustGetPendingNonceWithRetry(t, node.RPCURL(), secondaryEthAddr.Hex(), 20*time.Second)
	sendGasPrice := evmtest.MustGetGasPriceWithRetry(t, node.RPCURL(), 20*time.Second)

	sendHash := evmtest.SendLegacyTxWithParams(t, node.RPCURL(), evmtest.LegacyTxParams{
		PrivateKey: secondaryPriv,
		Nonce:      sendNonce,
		To:         &recipientEthAddr,
		Value:      sendAmount,
		Gas:        21_000,
		GasPrice:   sendGasPrice,
	})
	sendReceipt := evmtest.WaitForReceipt(t, node.RPCURL(), sendHash, node.WaitCh(), node.OutputBuffer(), 40*time.Second)
	evmtest.AssertReceiptMatchesTxHash(t, sendReceipt, sendHash)

	gasUsed := evmtest.MustUint64HexField(t, sendReceipt, "gasUsed")
	effectiveGasPriceHex := evmtest.MustStringField(t, sendReceipt, "effectiveGasPrice")
	effectiveGasPrice, err := hexutil.DecodeBig(effectiveGasPriceHex)
	if err != nil {
		t.Fatalf("decode effectiveGasPrice %q: %v", effectiveGasPriceHex, err)
	}
	feePaid := new(big.Int).Mul(new(big.Int).SetUint64(gasUsed), effectiveGasPrice)

	secondaryAfter := mustExtendedBalanceFromSplitQueries(t, node, secondaryBech32)
	recipientAfter := mustExtendedBalanceFromSplitQueries(t, node, recipientBech32)
	remainderAfter := mustQueryPrecisebankRemainder(t, node)

	secondaryDelta := new(big.Int).Sub(secondaryBefore, secondaryAfter)
	wantSecondaryDelta := new(big.Int).Add(new(big.Int).Set(sendAmount), feePaid)
	if secondaryDelta.Cmp(wantSecondaryDelta) != 0 {
		t.Fatalf(
			"unexpected secondary sender delta: got=%s want=%s (value=%s fee=%s)",
			secondaryDelta.String(),
			wantSecondaryDelta.String(),
			sendAmount.String(),
			feePaid.String(),
		)
	}

	recipientDelta := new(big.Int).Sub(recipientAfter, recipientBefore)
	if recipientDelta.Cmp(sendAmount) != 0 {
		t.Fatalf("unexpected recipient delta: got=%s want=%s", recipientDelta.String(), sendAmount.String())
	}

	if remainderAfter.Cmp(remainderBefore) != 0 {
		t.Fatalf("remainder changed after secondary workflow: before=%s after=%s", remainderBefore.String(), remainderAfter.String())
	}
}

func mustExtendedBalanceFromSplitQueries(t *testing.T, node *evmtest.Node, bech32Addr string) *big.Int {
	t.Helper()

	cf := conversionFactorBigInt()
	integerPart := mustQueryBankBalanceDenom(t, node, bech32Addr, lcfg.ChainDenom)
	fractionalPart := mustQueryPrecisebankFractionalBalance(t, node, bech32Addr)

	full := new(big.Int).Mul(new(big.Int).Set(integerPart), cf)
	full.Add(full, fractionalPart)
	return full
}

func mustAccAddressBech32(t *testing.T, bz []byte) string {
	t.Helper()

	codec := addresscodec.NewBech32Codec(lcfg.Bech32AccountAddressPrefix)
	addr, err := codec.BytesToString(bz)
	if err != nil {
		t.Fatalf("encode bech32 address: %v", err)
	}
	return addr
}
