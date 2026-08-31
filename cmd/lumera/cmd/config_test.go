package cmd

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	appopenrpc "github.com/LumeraProtocol/lumera/app/openrpc"
	lcfg "github.com/LumeraProtocol/lumera/config"
)

// TestInitAppConfigEVMDefaults verifies command-layer app config enables the
// expected Cosmos EVM defaults used by `lumerad start`.
func TestInitAppConfigEVMDefaults(t *testing.T) {
	t.Parallel()

	template, cfg := initAppConfig()

	require.Contains(t, template, "[json-rpc]")
	require.Contains(t, template, "enable-indexer = {{ .JSONRPC.EnableIndexer }}")
	require.Contains(t, template, "[evm.mempool]")
	require.Contains(t, template, "[lumera.evm-mempool]")
	require.Contains(t, template, "broadcast-debug = {{ .Lumera.EVMMempool.BroadcastDebug }}")

	cfgValue := reflect.ValueOf(cfg)
	require.Equal(t, reflect.Struct, cfgValue.Kind())

	jsonRPCCfg := cfgValue.FieldByName("JSONRPC")
	require.True(t, jsonRPCCfg.IsValid(), "JSONRPC field not found")
	require.True(t, jsonRPCCfg.FieldByName("Enable").Bool(), "json-rpc must be enabled by default")
	require.True(t, jsonRPCCfg.FieldByName("EnableIndexer").Bool(), "json-rpc indexer must be enabled by default")
	apiNamespaces, ok := jsonRPCCfg.FieldByName("API").Interface().([]string)
	require.True(t, ok, "json-rpc.api must be []string")
	require.Contains(t, apiNamespaces, appopenrpc.Namespace, "json-rpc.api must include rpc namespace for OpenRPC discovery")
	require.NotContains(t, apiNamespaces, "admin", "json-rpc.api must not include admin by default")
	require.NotContains(t, apiNamespaces, "debug", "json-rpc.api must not include debug by default")
	require.NotContains(t, apiNamespaces, "personal", "json-rpc.api must not include personal by default")

	evmCfg := cfgValue.FieldByName("EVM")
	require.True(t, evmCfg.IsValid(), "EVM field not found")
	require.Equal(t, uint64(lcfg.EVMChainID), evmCfg.FieldByName("EVMChainID").Uint(), "unexpected EVM chain ID")
	evmMempoolCfg := evmCfg.FieldByName("Mempool")
	require.True(t, evmMempoolCfg.IsValid(), "EVM.Mempool field not found")
	require.EqualValues(t, 1, evmMempoolCfg.FieldByName("PriceLimit").Uint(), "unexpected evm mempool price limit")
	require.EqualValues(t, 10, evmMempoolCfg.FieldByName("PriceBump").Uint(), "unexpected evm mempool price bump")
	require.EqualValues(t, 16, evmMempoolCfg.FieldByName("AccountSlots").Uint(), "unexpected evm mempool account slots")
	require.EqualValues(t, 5120, evmMempoolCfg.FieldByName("GlobalSlots").Uint(), "unexpected evm mempool global slots")
	require.EqualValues(t, 64, evmMempoolCfg.FieldByName("AccountQueue").Uint(), "unexpected evm mempool account queue")
	require.EqualValues(t, 1024, evmMempoolCfg.FieldByName("GlobalQueue").Uint(), "unexpected evm mempool global queue")
	require.False(t, evmMempoolCfg.FieldByName("InsertQueueSize").IsValid(),
		"Cosmos EVM v0.6.0 must not grow an unreviewed insert-queue-size config knob")

	sdkCfg := cfgValue.FieldByName("Config")
	require.True(t, sdkCfg.IsValid(), "Config field not found")
	mempoolCfg := sdkCfg.FieldByName("Mempool")
	require.True(t, mempoolCfg.IsValid(), "Mempool field not found")
	require.EqualValues(t, 10000, mempoolCfg.FieldByName("MaxTxs").Int(), "unexpected app-side mempool max txs")

	lumeraCfg := cfgValue.FieldByName("Lumera")
	require.True(t, lumeraCfg.IsValid(), "Lumera field not found")
	lumeraEVMMempoolCfg := lumeraCfg.FieldByName("EVMMempool")
	require.True(t, lumeraEVMMempoolCfg.IsValid(), "Lumera.EVMMempool field not found")
	require.False(t, lumeraEVMMempoolCfg.FieldByName("BroadcastDebug").Bool(), "broadcast debug must be disabled by default")
}

// TestInitCometBFTConfigRPCHardening locks in the RPC defense-in-depth
// overrides applied in initCometBFTConfig. These protect lumerad's RPC
// (port :26657) from misbehaving WebSocket clients accumulating dormant
// subscriptions (see sdk-go PR #17 / lumera-devnet-1 val3 incident, where
// ~5,000 ESTABLISHED conns saturated the listen backlog while the chain
// itself stayed healthy). Changing these defaults should require an
// explicit decision recorded against this test.
func TestInitCometBFTConfigRPCHardening(t *testing.T) {
	t.Parallel()

	cfg := initCometBFTConfig()

	require.NotNil(t, cfg, "initCometBFTConfig must return a config")
	require.NotNil(t, cfg.RPC, "RPC config must not be nil")

	require.True(t, cfg.RPC.CloseOnSlowClient,
		"experimental_close_on_slow_client must default to true to prevent slow WS clients from holding subscription buffers indefinitely")

	// Subscription caps are upstream CometBFT defaults — we intentionally do
	// not narrow them; this assertion guards against accidentally loosening
	// them, which would re-open the saturation attack surface.
	require.Equal(t, 100, cfg.RPC.MaxSubscriptionClients,
		"max_subscription_clients must stay at the upstream default; relaxing it re-opens the val3-style saturation surface")
	require.Equal(t, 5, cfg.RPC.MaxSubscriptionsPerClient,
		"max_subscriptions_per_client must stay at the upstream default")
}
