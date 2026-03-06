package app

import (
	"errors"
	"math/big"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"cosmossdk.io/log"
	lcfg "github.com/LumeraProtocol/lumera/config"
	testaccounts "github.com/LumeraProtocol/lumera/testutil/accounts"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"
)

type testAppOptions map[string]interface{}

func (o testAppOptions) Get(key string) interface{} {
	return o[key]
}

var _ servertypes.AppOptions = testAppOptions{}

// TestConfigureEVMBroadcastOptionsFromAppOptions verifies app options drive the
// EVM mempool broadcast debug toggle and logger initialization safely.
func TestConfigureEVMBroadcastOptionsFromAppOptions(t *testing.T) {
	t.Parallel()

	baseLogger := log.NewNopLogger().With(log.ModuleKey, evmBroadcastLogModule)
	app := &App{}

	app.configureEVMBroadcastOptions(testAppOptions{
		evmMempoolBroadcastDebugAppOpt: true,
	}, baseLogger)
	require.True(t, app.evmBroadcastDebug)
	require.NotNil(t, app.evmBroadcastLogger)

	app.configureEVMBroadcastOptions(testAppOptions{
		evmMempoolBroadcastDebugAppOpt: "not-a-bool",
	}, baseLogger)
	require.False(t, app.evmBroadcastDebug)
}

// TestEVMTxBroadcastDispatcherDedupesQueuedAndInFlight verifies duplicate tx
// hashes are filtered both within a batch and while already reserved by the
// dispatcher worker.
func TestEVMTxBroadcastDispatcherDedupesQueuedAndInFlight(t *testing.T) {
	var releaseOnce sync.Once
	release := make(chan struct{})
	processed := make(chan []*ethtypes.Transaction, 2)

	dispatcher := newEVMTxBroadcastDispatcher(
		log.NewNopLogger(),
		8,
		func(txs []*ethtypes.Transaction) error {
			processed <- append([]*ethtypes.Transaction(nil), txs...)
			<-release
			return nil
		},
	)
	defer func() {
		releaseOnce.Do(func() { close(release) })
		dispatcher.stop(2 * time.Second)
	}()

	tx1 := makeLegacyTx(1)
	tx2 := makeLegacyTx(2)

	accepted, deduped, err := dispatcher.enqueue([]*ethtypes.Transaction{tx1, tx1, tx2})
	require.NoError(t, err)
	require.Equal(t, 2, accepted)
	require.Equal(t, 1, deduped)

	accepted, deduped, err = dispatcher.enqueue([]*ethtypes.Transaction{tx1, tx2})
	require.NoError(t, err)
	require.Equal(t, 0, accepted)
	require.Equal(t, 2, deduped)

	select {
	case batch := <-processed:
		require.Len(t, batch, 2)
		require.Equal(t, tx1.Hash(), batch[0].Hash())
		require.Equal(t, tx2.Hash(), batch[1].Hash())
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for first processed broadcast batch")
	}

	releaseOnce.Do(func() { close(release) })

	require.Eventually(t, func() bool {
		accepted, deduped, err := dispatcher.enqueue([]*ethtypes.Transaction{tx1})
		return err == nil && accepted == 1 && deduped == 0
	}, 2*time.Second, 10*time.Millisecond)
}

// TestEVMTxBroadcastDispatcherQueueFullReleasesPending verifies queue-full
// enqueue failures do not leave stale pending-hash reservations behind.
func TestEVMTxBroadcastDispatcherQueueFullReleasesPending(t *testing.T) {
	t.Parallel()

	dispatcher := &evmTxBroadcastDispatcher{
		logger:  log.NewNopLogger(),
		queue:   make(chan evmBroadcastBatch, 1),
		pending: make(map[common.Hash]struct{}),
	}

	dispatcher.queue <- evmBroadcastBatch{txs: []*ethtypes.Transaction{makeLegacyTx(100)}}
	tx := makeLegacyTx(1)

	accepted, deduped, err := dispatcher.enqueue([]*ethtypes.Transaction{tx})
	require.Error(t, err)
	require.Contains(t, err.Error(), "queue is full")
	require.Equal(t, 0, accepted)
	require.Equal(t, 0, deduped)

	dispatcher.mtx.Lock()
	_, pending := dispatcher.pending[tx.Hash()]
	dispatcher.mtx.Unlock()
	require.False(t, pending, "queue-full path must release pending hash reservations")

	accepted, deduped, err = dispatcher.enqueue([]*ethtypes.Transaction{tx})
	require.Error(t, err)
	require.Contains(t, err.Error(), "queue is full")
	require.Equal(t, 0, accepted)
	require.Equal(t, 0, deduped)
}

// TestEVMTxBroadcastDispatcherReleasesPendingAfterProcessError verifies a
// failed process callback still clears pending reservations so the tx can be
// retried later.
func TestEVMTxBroadcastDispatcherReleasesPendingAfterProcessError(t *testing.T) {
	processed := make(chan struct{}, 2)
	dispatcher := newEVMTxBroadcastDispatcher(
		log.NewNopLogger(),
		4,
		func(_ []*ethtypes.Transaction) error {
			processed <- struct{}{}
			return errors.New("boom")
		},
	)
	defer dispatcher.stop(2 * time.Second)

	tx := makeLegacyTx(7)

	accepted, deduped, err := dispatcher.enqueue([]*ethtypes.Transaction{tx})
	require.NoError(t, err)
	require.Equal(t, 1, accepted)
	require.Equal(t, 0, deduped)

	select {
	case <-processed:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for dispatcher process callback")
	}

	require.Eventually(t, func() bool {
		accepted, deduped, err := dispatcher.enqueue([]*ethtypes.Transaction{tx})
		return err == nil && accepted == 1 && deduped == 0
	}, 2*time.Second, 10*time.Millisecond)
}

// TestEVMTxBroadcastDispatcherStopTimeoutSlowProcessing verifies Stop waits for
// timeout when the worker is still processing a batch (slow/blocking path).
func TestEVMTxBroadcastDispatcherStopTimeoutSlowProcessing(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})

	dispatcher := newEVMTxBroadcastDispatcher(
		log.NewNopLogger(),
		2,
		func(_ []*ethtypes.Transaction) error {
			close(started)
			<-release
			return nil
		},
	)

	accepted, deduped, err := dispatcher.enqueue([]*ethtypes.Transaction{makeLegacyTx(33)})
	require.NoError(t, err)
	require.Equal(t, 1, accepted)
	require.Equal(t, 0, deduped)

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for worker to start processing")
	}

	stopTimeout := 75 * time.Millisecond
	start := time.Now()
	dispatcher.stop(stopTimeout)
	elapsed := time.Since(start)
	require.GreaterOrEqual(t, elapsed, stopTimeout, "stop should wait for timeout when worker is still busy")

	close(release)
	select {
	case <-dispatcher.doneCh:
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not exit after releasing processing")
	}
}

// TestEVMTxBroadcastDispatcherStopFastAfterPanic verifies Stop returns quickly
// when the worker has already exited due to panic.
func TestEVMTxBroadcastDispatcherStopFastAfterPanic(t *testing.T) {
	started := make(chan struct{})

	dispatcher := newEVMTxBroadcastDispatcher(
		log.NewNopLogger(),
		2,
		func(_ []*ethtypes.Transaction) error {
			close(started)
			panic("boom")
		},
	)

	accepted, deduped, err := dispatcher.enqueue([]*ethtypes.Transaction{makeLegacyTx(44)})
	require.NoError(t, err)
	require.Equal(t, 1, accepted)
	require.Equal(t, 0, deduped)

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for panic callback to start")
	}

	start := time.Now()
	dispatcher.stop(2 * time.Second)
	require.Less(t, time.Since(start), 300*time.Millisecond, "stop should return quickly after panic exit")
}

// TestEVMTxBroadcastDispatcherEnqueueRemainsNonBlocking verifies enqueue stays
// non-blocking while the single worker is busy, as long as queue capacity
// remains available.
func TestEVMTxBroadcastDispatcherEnqueueRemainsNonBlocking(t *testing.T) {
	var startedOnce sync.Once
	// started is used to signal the worker has started processing the first batch
	started := make(chan struct{})
	// release is used to unblock the worker to allow the test to complete
	release := make(chan struct{})
	// concurrent and maxConcurrent track the current and max observed concurrency of
	// the worker to assert batches are processed sequentially.
	var concurrent atomic.Int32
	var maxConcurrent atomic.Int32

	dispatcher := newEVMTxBroadcastDispatcher(
		log.NewNopLogger(),
		2,
		// This callback tracks the max concurrency to assert the worker processes
		// batches sequentially, and uses channels to coordinate test timing.
		func(_ []*ethtypes.Transaction) error {
			current := concurrent.Add(1)
			for {
				previous := maxConcurrent.Load()
				if current <= previous || maxConcurrent.CompareAndSwap(previous, current) {
					break
				}
			}

			startedOnce.Do(func() { close(started) })
			<-release
			concurrent.Add(-1)
			return nil
		},
	)
	defer func() {
		close(release)
		dispatcher.stop(2 * time.Second)
	}()

	accepted, deduped, err := dispatcher.enqueue([]*ethtypes.Transaction{makeLegacyTx(1)})
	require.NoError(t, err)
	require.Equal(t, 1, accepted)
	require.Equal(t, 0, deduped)

	// Wait for the worker to start processing the first batch before enqueueing the second batch to assert it remains non-blocking.
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for worker to start processing first batch")
	}

	done := make(chan struct{})
	resultCh := make(chan struct {
		accepted int
		deduped  int
		err      error
	}, 1)
	// Enqueueing a second batch while the first is still processing should succeed and remain non-blocking because the queue has capacity of 2.
	go func() {
		defer close(done)
		accepted, deduped, err := dispatcher.enqueue([]*ethtypes.Transaction{makeLegacyTx(2)})
		resultCh <- struct {
			accepted int
			deduped  int
			err      error
		}{
			accepted: accepted,
			deduped:  deduped,
			err:      err,
		}
	}()

	select {
	case <-done:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("enqueue should not block while worker is busy if queue has capacity")
	}

	result := <-resultCh
	require.NoError(t, result.err)
	require.Equal(t, 1, result.accepted)
	require.Equal(t, 0, result.deduped)

	require.Equal(t, int32(1), maxConcurrent.Load(), "dispatcher worker must process batches sequentially")
}

// TestBroadcastEVMTxFromFieldRecovery verifies that wrapping a signed Ethereum
// tx via FromSignedEthereumTx populates the From field (sender address), and
// that the older FromEthereumTx method does NOT. This is a regression guard for
// the bug where broadcastEVMTransactionsSync used FromEthereumTx, causing
// "sender address is missing" rejections on peer validators.
func TestBroadcastEVMTxFromFieldRecovery(t *testing.T) {
	t.Parallel()

	chainID := big.NewInt(int64(lcfg.EVMChainID))
	privKey, sender := testaccounts.MustGenerateEthKey(t)

	tx := ethtypes.NewTx(&ethtypes.LegacyTx{
		Nonce:    0,
		GasPrice: big.NewInt(1),
		Gas:      21_000,
		To:       &sender,
		Value:    big.NewInt(0),
	})
	// Sign the tx to produce a valid signature that FromSignedEthereumTx can recover from.
	signedTx, err := ethtypes.SignTx(tx, ethtypes.NewEIP155Signer(chainID), privKey)
	require.NoError(t, err)

	// FromEthereumTx does NOT populate From — this was the root cause of the bug.
	msgBroken := &evmtypes.MsgEthereumTx{}
	msgBroken.FromEthereumTx(signedTx)
	require.Empty(t, msgBroken.From, "FromEthereumTx must NOT set From (documents the upstream behavior)")

	// FromSignedEthereumTx recovers the sender from the ECDSA signature.
	msgFixed := &evmtypes.MsgEthereumTx{}
	ethSigner := ethtypes.LatestSignerForChainID(chainID)
	require.NoError(t, msgFixed.FromSignedEthereumTx(signedTx, ethSigner))
	require.NotEmpty(t, msgFixed.From, "FromSignedEthereumTx must populate From")

	recoveredAddr := common.BytesToAddress(msgFixed.From)
	require.Equal(t, sender, recoveredAddr, "recovered sender must match signing key")
}

func makeLegacyTx(nonce uint64) *ethtypes.Transaction {
	return ethtypes.NewTx(&ethtypes.LegacyTx{
		Nonce:    nonce,
		GasPrice: big.NewInt(1),
		Gas:      21_000,
		Value:    big.NewInt(1),
	})
}
