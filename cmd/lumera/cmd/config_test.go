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

	sdkCfg := cfgValue.FieldByName("Config")
	require.True(t, sdkCfg.IsValid(), "Config field not found")
	mempoolCfg := sdkCfg.FieldByName("Mempool")
	require.True(t, mempoolCfg.IsValid(), "Mempool field not found")
	require.EqualValues(t, 5000, mempoolCfg.FieldByName("MaxTxs").Int(), "unexpected app-side mempool max txs")

	lumeraCfg := cfgValue.FieldByName("Lumera")
	require.True(t, lumeraCfg.IsValid(), "Lumera field not found")
	evmMempoolCfg := lumeraCfg.FieldByName("EVMMempool")
	require.True(t, evmMempoolCfg.IsValid(), "Lumera.EVMMempool field not found")
	require.False(t, evmMempoolCfg.FieldByName("BroadcastDebug").Bool(), "broadcast debug must be disabled by default")
}
