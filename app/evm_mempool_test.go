package app

import (
	"testing"

	sdkserver "github.com/cosmos/cosmos-sdk/server"
	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"
	evmmempool "github.com/cosmos/evm/mempool"
	"github.com/stretchr/testify/require"
)

// TestEVMMempoolWiringOnAppStartup verifies app and BaseApp both reference the
// same initialized ExperimentalEVMMempool instance.
func TestEVMMempoolWiringOnAppStartup(t *testing.T) {
	app := Setup(t)

	extMempool := app.GetMempool()
	require.NotNil(t, extMempool, "GetMempool should be initialized")
	require.NotNil(t, app.Mempool(), "BaseApp mempool should be initialized")

	getMempoolCasted, ok := extMempool.(*evmmempool.ExperimentalEVMMempool)
	require.True(t, ok, "GetMempool should expose ExperimentalEVMMempool")

	baseMempoolCasted, ok := app.Mempool().(*evmmempool.ExperimentalEVMMempool)
	require.True(t, ok, "BaseApp mempool should be ExperimentalEVMMempool")

	require.Same(t, getMempoolCasted, baseMempoolCasted, "App and BaseApp mempool references should match")
}

func TestEVMMempoolDisabledWhenMaxTxsIsNegative(t *testing.T) {
	app, _ := setupWithAppOptionOverrides(
		t,
		"testing",
		false,
		5,
		map[string]interface{}{sdkserver.FlagMempoolMaxTxs: -1},
	)

	require.Nil(t, app.GetMempool(), "App EVM mempool should not be configured when app-side mempool is disabled")
	_, isNoOp := app.Mempool().(sdkmempool.NoOpMempool)
	require.True(t, isNoOp, "BaseApp mempool should remain NoOp when app-side mempool is disabled")
}
