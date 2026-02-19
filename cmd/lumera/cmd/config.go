package cmd

import (
	cmtcfg "github.com/cometbft/cometbft/config"
	serverconfig "github.com/cosmos/cosmos-sdk/server/config"
	cosmosevmserverconfig "github.com/cosmos/evm/server/config"

	lcfg "github.com/LumeraProtocol/lumera/config"
)

// initCometBFTConfig helps to override default CometBFT Config values.
// return cmtcfg.DefaultConfig if no custom configuration is required for the application.
func initCometBFTConfig() *cmtcfg.Config {
	cfg := cmtcfg.DefaultConfig()

	// these values put a higher strain on node memory
	// cfg.P2P.MaxNumInboundPeers = 100
	// cfg.P2P.MaxNumOutboundPeers = 40

	return cfg
}

// initAppConfig helps to override default appConfig template and configs.
// return "", nil if no custom configuration is required for the application.
func initAppConfig() (string, interface{}) {
	type CustomAppConfig struct {
		serverconfig.Config `mapstructure:",squash"`

		EVM     cosmosevmserverconfig.EVMConfig     `mapstructure:"evm"`
		JSONRPC cosmosevmserverconfig.JSONRPCConfig `mapstructure:"json-rpc"`
		TLS     cosmosevmserverconfig.TLSConfig     `mapstructure:"tls"`
	}

	srvCfg := serverconfig.DefaultConfig()
	// Enable app-side mempool by default so EVM mempool integration paths
	// (pending tx subscriptions, nonce-gap handling, replacement rules) work
	// out-of-the-box without extra start flags.
	srvCfg.Mempool.MaxTxs = 5000
	evmCfg := cosmosevmserverconfig.DefaultEVMConfig()
	evmCfg.EVMChainID = lcfg.EVMChainID

	jsonRPCCfg := cosmosevmserverconfig.DefaultJSONRPCConfig()
	// Run JSON-RPC + indexer without extra start flags; defaults can still be
	// overridden via app.toml or CLI.
	jsonRPCCfg.Enable = true
	jsonRPCCfg.EnableIndexer = true

	customAppConfig := CustomAppConfig{
		Config:  *srvCfg,
		EVM:     *evmCfg,
		JSONRPC: *jsonRPCCfg,
		TLS:     *cosmosevmserverconfig.DefaultTLSConfig(),
	}

	customAppTemplate := serverconfig.DefaultConfigTemplate + cosmosevmserverconfig.DefaultEVMConfigTemplate

	return customAppTemplate, customAppConfig
}
