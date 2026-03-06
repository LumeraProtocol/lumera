package cmd

import (
	cmtcfg "github.com/cometbft/cometbft/config"
	serverconfig "github.com/cosmos/cosmos-sdk/server/config"
	cosmosevmserverconfig "github.com/cosmos/evm/server/config"

	appopenrpc "github.com/LumeraProtocol/lumera/app/openrpc"
	lcfg "github.com/LumeraProtocol/lumera/config"
)

type LumeraEVMMempoolConfig struct {
	BroadcastDebug bool `mapstructure:"broadcast-debug"`
}

type LumeraJSONRPCRateLimitConfig struct {
	Enable           bool   `mapstructure:"enable"`
	ProxyAddress     string `mapstructure:"proxy-address"`
	RequestsPerSec   int    `mapstructure:"requests-per-second"`
	Burst            int    `mapstructure:"burst"`
	EntryTTL         string `mapstructure:"entry-ttl"`
}

type LumeraConfig struct {
	EVMMempool       LumeraEVMMempoolConfig        `mapstructure:"evm-mempool"`
	JSONRPCRateLimit LumeraJSONRPCRateLimitConfig   `mapstructure:"json-rpc-ratelimit"`
}

const lumeraConfigTemplate = `
###############################################################################
###                           Lumera Configuration                          ###
###############################################################################

[lumera.evm-mempool]
# Enables detailed logs for async EVM mempool broadcast queue processing.
broadcast-debug = {{ .Lumera.EVMMempool.BroadcastDebug }}

[lumera.json-rpc-ratelimit]
# Rate-limiting reverse proxy for the EVM JSON-RPC endpoint.
# When enabled, a proxy server listens on proxy-address and forwards requests
# to the internal JSON-RPC server with per-IP token bucket rate limiting.

# Enable the rate-limiting proxy (default: false).
enable = {{ .Lumera.JSONRPCRateLimit.Enable }}

# Address the rate-limiting proxy listens on.
proxy-address = "{{ .Lumera.JSONRPCRateLimit.ProxyAddress }}"

# Sustained requests per second allowed per IP.
requests-per-second = {{ .Lumera.JSONRPCRateLimit.RequestsPerSec }}

# Maximum burst size per IP (token bucket capacity).
burst = {{ .Lumera.JSONRPCRateLimit.Burst }}

# Time-to-live for per-IP rate limiter entries (Go duration, e.g. "5m", "1h").
# Entries are evicted after this duration of inactivity.
entry-ttl = "{{ .Lumera.JSONRPCRateLimit.EntryTTL }}"
`

// initCometBFTConfig helps to override default CometBFT Config values.
// return cmtcfg.DefaultConfig if no custom configuration is required for the application.
func initCometBFTConfig() *cmtcfg.Config {
	cfg := cmtcfg.DefaultConfig()

	// these values put a higher strain on node memory
	// cfg.P2P.MaxNumInboundPeers = 100
	// cfg.P2P.MaxNumOutboundPeers = 40

	return cfg
}

// CustomAppConfig extends the SDK server config with EVM and Lumera sections.
type CustomAppConfig struct {
	serverconfig.Config `mapstructure:",squash"`

	EVM     cosmosevmserverconfig.EVMConfig     `mapstructure:"evm"`
	JSONRPC cosmosevmserverconfig.JSONRPCConfig `mapstructure:"json-rpc"`
	TLS     cosmosevmserverconfig.TLSConfig     `mapstructure:"tls"`
	Lumera  LumeraConfig                        `mapstructure:"lumera"`
}

// initAppConfig helps to override default appConfig template and configs.
// return "", nil if no custom configuration is required for the application.
func initAppConfig() (string, interface{}) {
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
	jsonRPCCfg.API = appopenrpc.EnsureNamespaceEnabled(jsonRPCCfg.API)

	customAppConfig := CustomAppConfig{
		Config:  *srvCfg,
		EVM:     *evmCfg,
		JSONRPC: *jsonRPCCfg,
		TLS:     *cosmosevmserverconfig.DefaultTLSConfig(),
		Lumera: LumeraConfig{
			EVMMempool: LumeraEVMMempoolConfig{
				BroadcastDebug: false,
			},
			JSONRPCRateLimit: LumeraJSONRPCRateLimitConfig{
				Enable:         false,
				ProxyAddress:   "0.0.0.0:8547",
				RequestsPerSec: 50,
				Burst:          100,
				EntryTTL:       "5m",
			},
		},
	}

	customAppTemplate := serverconfig.DefaultConfigTemplate + cosmosevmserverconfig.DefaultEVMConfigTemplate + lumeraConfigTemplate

	return customAppTemplate, customAppConfig
}
