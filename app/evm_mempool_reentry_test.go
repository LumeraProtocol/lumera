package app

import (
	"bytes"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"math/big"
	"testing"
	"time"

	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"
	lcfg "github.com/LumeraProtocol/lumera/config"
	testaccounts "github.com/LumeraProtocol/lumera/testutil/accounts"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/cosmos/cosmos-sdk/client"
	sdk "github.com/cosmos/cosmos-sdk/types"
	evmencoding "github.com/cosmos/evm/encoding"
	evmmempool "github.com/cosmos/evm/mempool"
	"github.com/cosmos/evm/mempool/txpool/legacypool"
	"github.com/cosmos/evm/x/vm/statedb"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	vmmocks "github.com/cosmos/evm/x/vm/types/mocks"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

// TestEVMMempoolReentrantInsertBlocks demonstrates the mutex re-entry hazard
// that the async broadcast queue (evmTxBroadcastDispatcher) is designed to
// prevent. When BroadcastTxFn executes synchronously inside runReorg, the
// outer Insert() still holds m.mtx. Any attempt to call Insert() again from
// within BroadcastTxFn blocks on the same mutex, creating a deadlock.
//
// This test validates the underlying mechanism by directly wiring a custom
// BroadcastTxFn that re-enters Insert and verifying it blocks until the outer
// Insert releases the lock.
func TestEVMMempoolReentrantInsertBlocks(t *testing.T) {
	chainID := ensureTestChainID(t)

	encodingCfg := evmencoding.MakeConfig(chainID.Uint64())

	vmKeeper := newVMKeeperStub()
	feeKeeper := feeMarketKeeperStub{}
	ctxProvider := func(height int64, _ bool) (sdk.Context, error) {
		blockHeight := maxInt64(height, 1)
		return sdk.Context{}.
			WithBlockHeight(blockHeight).
			WithBlockTime(time.Now()).
			WithBlockHeader(cmtproto.Header{
				Height:  blockHeight,
				AppHash: bytes.Repeat([]byte{0x1}, 32),
			}), nil
	}

	extMempool := evmmempool.NewExperimentalEVMMempool(
		ctxProvider,
		log.NewNopLogger(),
		vmKeeper,
		feeKeeper,
		encodingCfg.TxConfig,
		client.Context{}.WithTxConfig(encodingCfg.TxConfig),
		&evmmempool.EVMMempoolConfig{
			LegacyPoolConfig: &legacypool.Config{},
			BlockGasLimit:    100_000_000,
			MinTip:           uint256.NewInt(0),
		},
		10000,
	)

	legacyPool, ok := extMempool.GetTxPool().Subpools[0].(*legacypool.LegacyPool)
	require.True(t, ok, "expected legacy subpool")

	privKey, sender := testaccounts.MustGenerateEthKey(t)

	// Ensure sender has sufficient balance so txpool state validation passes.
	funded := statedb.NewEmptyAccount()
	funded.Balance = uint256.NewInt(1_000_000_000_000_000_000)
	require.NoError(t, vmKeeper.SetAccount(sdk.Context{}, sender, *funded))

	ctx := sdk.Context{}.WithBlockHeight(1)

	// Prime a nonce gap: nonce=1 is queued, nonce=0 will fill the gap and
	// trigger promotion → BroadcastTxFn inside runReorg.
	gapTx := mustMakeSignedEVMMsg(t, privKey, chainID, 1)
	require.NoError(t, extMempool.Insert(ctx, gapTx), "prime nonce-gap tx should be accepted")

	reentryBlocked := make(chan struct{})
	releaseBroadcast := make(chan struct{})
	reentrantDone := make(chan error, 1)

	legacyPool.BroadcastTxFn = func(txs []*ethtypes.Transaction) error {
		if len(txs) == 0 {
			return errors.New("expected promoted txs in broadcast callback")
		}

		innerTx := &evmtypes.MsgEthereumTx{}
		signer := ethtypes.LatestSignerForChainID(chainID)
		if err := innerTx.FromSignedEthereumTx(txs[0], signer); err != nil {
			return fmt.Errorf("wrap promoted tx: %w", err)
		}

		// Attempt to re-enter Insert while outer Insert still holds m.mtx.
		// This simulates what BroadcastTxSync → CheckTx → Insert would do.
		go func() {
			reentrantDone <- extMempool.Insert(ctx, innerTx)
		}()

		select {
		case err := <-reentrantDone:
			return fmt.Errorf("expected reentrant insert to block, got: %v", err)
		case <-time.After(250 * time.Millisecond):
			close(reentryBlocked)
		}

		<-releaseBroadcast
		return nil
	}

	fillTx := mustMakeSignedEVMMsg(t, privKey, chainID, 0)
	outerDone := make(chan error, 1)
	go func() {
		outerDone <- extMempool.Insert(ctx, fillTx)
	}()

	select {
	case <-reentryBlocked:
	case err := <-outerDone:
		t.Fatalf("outer insert unexpectedly completed early: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for reentrant insert blocking signal")
	}

	select {
	case err := <-outerDone:
		t.Fatalf("outer insert should still be blocked while broadcast is held: %v", err)
	default:
	}

	close(releaseBroadcast)
	require.NoError(t, <-outerDone, "outer insert should complete once broadcast returns")

	select {
	case <-reentrantDone:
	case <-time.After(3 * time.Second):
		t.Fatal("reentrant insert did not finish after outer insert released mutex")
	}
}

func mustMakeSignedEVMMsg(t *testing.T, privKey *ecdsa.PrivateKey, chainID *big.Int, nonce uint64) *evmtypes.MsgEthereumTx {
	t.Helper()

	sender := ethcrypto.PubkeyToAddress(privKey.PublicKey)
	tx := ethtypes.NewTx(&ethtypes.LegacyTx{
		Nonce:    nonce,
		To:       &sender,
		Value:    big.NewInt(0),
		Gas:      21_000,
		GasPrice: big.NewInt(1),
	})

	signedTx, err := ethtypes.SignTx(tx, ethtypes.NewEIP155Signer(chainID), privKey)
	require.NoError(t, err, "sign legacy tx")

	msg := &evmtypes.MsgEthereumTx{}
	signer := ethtypes.LatestSignerForChainID(chainID)
	require.NoError(t, msg.FromSignedEthereumTx(signedTx, signer), "wrap signed eth tx")
	return msg
}

func ensureTestChainID(t *testing.T) *big.Int {
	t.Helper()

	if evmtypes.GetChainConfig() == nil {
		require.NoError(t, evmtypes.SetChainConfig(evmtypes.DefaultChainConfig(lcfg.EVMChainID)))
	}

	ethCfg := evmtypes.GetEthChainConfig()
	require.NotNil(t, ethCfg)
	require.NotNil(t, ethCfg.ChainID)
	return new(big.Int).Set(ethCfg.ChainID)
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

type vmKeeperStub struct {
	*vmmocks.EVMKeeper
}

func newVMKeeperStub() *vmKeeperStub {
	return &vmKeeperStub{EVMKeeper: vmmocks.NewEVMKeeper()}
}

func (k *vmKeeperStub) GetBaseFee(sdk.Context) *big.Int { return big.NewInt(0) }
func (k *vmKeeperStub) GetParams(sdk.Context) evmtypes.Params {
	return evmtypes.DefaultParams()
}
func (k *vmKeeperStub) GetEvmCoinInfo(sdk.Context) evmtypes.EvmCoinInfo {
	return evmtypes.EvmCoinInfo{
		Denom:         lcfg.ChainDenom,
		ExtendedDenom: lcfg.ChainEVMExtendedDenom,
		DisplayDenom:  lcfg.ChainDisplayDenom,
		Decimals:      evmtypes.EighteenDecimals.Uint32(),
	}
}
func (k *vmKeeperStub) SetEvmMempool(*evmmempool.ExperimentalEVMMempool) {}
func (k *vmKeeperStub) KVStoreKeys() map[string]*storetypes.KVStoreKey {
	return map[string]*storetypes.KVStoreKey{}
}

type feeMarketKeeperStub struct{}

func (feeMarketKeeperStub) GetBlockGasWanted(sdk.Context) uint64 { return 0 }
