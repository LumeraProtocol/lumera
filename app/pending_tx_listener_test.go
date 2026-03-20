package app

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"
)

// TestRegisterPendingTxListenerFanout verifies that app-level pending tx
// listeners are invoked in registration order for each announced tx hash.
func TestRegisterPendingTxListenerFanout(t *testing.T) {
	app := Setup(t)

	var called []string
	app.RegisterPendingTxListener(func(hash common.Hash) {
		called = append(called, "first:"+hash.Hex())
	})
	app.RegisterPendingTxListener(func(hash common.Hash) {
		called = append(called, "second:"+hash.Hex())
	})

	txHash := ethtypes.NewTx(&ethtypes.LegacyTx{
		Nonce:    7,
		GasPrice: big.NewInt(1),
		Gas:      21_000,
	}).Hash()

	app.onPendingTx(txHash)

	require.Equal(t, []string{
		"first:" + txHash.Hex(),
		"second:" + txHash.Hex(),
	}, called)
}

// TestBroadcastEVMTransactionsWithoutNode verifies the broadcast callback can
// still encode tx bytes with app txConfig even before SetClientCtx runs, and
// then fails cleanly because no RPC node client is configured.
func TestBroadcastEVMTransactionsWithoutNode(t *testing.T) {
	app := Setup(t)

	tx := ethtypes.NewTx(&ethtypes.LegacyTx{
		Nonce:    1,
		GasPrice: big.NewInt(1),
		Gas:      21_000,
	})

	err := app.broadcastEVMTransactions([]*ethtypes.Transaction{tx})
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to broadcast transaction")
	require.Contains(t, err.Error(), "no RPC client is defined in offline mode")
}
