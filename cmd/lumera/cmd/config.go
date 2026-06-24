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
	Enable         bool   `mapstructure:"enable"`
	ProxyAddress   string `mapstructure:"proxy-address"`
	RequestsPerSec int    `mapstructure:"requests-per-second"`
	Burst          int    `mapstructure:"burst"`
	EntryTTL       string `mapstructure:"entry-ttl"`
	TrustedProxies string `mapstructure:"trusted-proxies"`
}

type LumeraConfig struct {
	EVMMempool       LumeraEVMMempoolConfig       `mapstructure:"evm-mempool"`
	JSONRPCRateLimit LumeraJSONRPCRateLimitConfig `mapstructure:"json-rpc-ratelimit"`
}

const lumeraConfigTemplate = `
###############################################################################
###                           Lumera Configuration                          ###
###############################################################################

[lumera.evm-mempool]
# Enables detailed logs for async EVM mempool broadcast queue processing.
broadcast-debug = {{ .Lumera.EVMMempool.BroadcastDebug }}

[lumera.json-rpc-ratelimit]
# Per-IP token bucket rate limiting for the EVM JSON-RPC endpoint.
# When the public JSON-RPC alias proxy is active (the default startup topology),
# enabling this wraps the public json-rpc.address listener directly. proxy-address
# is used only for the standalone fallback topology when the alias proxy is inactive.

# Enable JSON-RPC rate limiting (default: false).
enable = {{ .Lumera.JSONRPCRateLimit.Enable }}

# Standalone fallback proxy listen address. Not used when the public alias proxy
# is active and rate limiting wraps json-rpc.address directly.
proxy-address = "{{ .Lumera.JSONRPCRateLimit.ProxyAddress }}"

# Sustained requests per second allowed per IP.
requests-per-second = {{ .Lumera.JSONRPCRateLimit.RequestsPerSec }}

# Maximum burst size per IP (token bucket capacity).
burst = {{ .Lumera.JSONRPCRateLimit.Burst }}

# Time-to-live for per-IP rate limiter entries (Go duration, e.g. "5m", "1h").
# Entries are evicted after this duration of inactivity.
entry-ttl = "{{ .Lumera.JSONRPCRateLimit.EntryTTL }}"

# Comma-separated list of trusted reverse proxy CIDRs (e.g. "10.0.0.0/8, 172.16.0.0/12").
# When set, X-Forwarded-For and X-Real-IP headers are only trusted from these sources.
# When empty (default), client IP is always derived from the socket peer address.
trusted-proxies = "{{ .Lumera.JSONRPCRateLimit.TrustedProxies }}"
`

// initCometBFTConfig helps to override default CometBFT Config values.
// return cmtcfg.DefaultConfig if no custom configuration is required for the application.
func initCometBFTConfig() *cmtcfg.Config {
	cfg := cmtcfg.DefaultConfig()

	// these values put a higher strain on node memory
	// cfg.P2P.MaxNumInboundPeers = 100
	// cfg.P2P.MaxNumOutboundPeers = 40

	// RPC hardening — defense-in-depth against misbehaving WebSocket clients.
	//
	// Default CometBFT behaviour keeps a WebSocket connection open even when the
	// client is not draining its subscription buffer, relying on the OS / peer to
	// eventually close the socket. In practice this can take hours, and a buggy
	// client (or one with a goroutine/socket leak — see sdk-go PR #17) can
	// accumulate thousands of dormant subscriptions on `:26657` and saturate the
	// listen backlog of a validator's RPC.
	//
	// Observed on lumera-devnet-1 val3: ~5,000 ESTABLISHED WS connections, listen
	// queue full (Recv-Q=4097/4096), external RPC port unresponsive — while the
	// chain itself kept producing blocks. Root cause was a client-side leak in
	// sdk-go's wait-tx subscriber (fixed in sdk-go PR #17), but the server had no
	// circuit breaker: it accepted, kept, and buffered for every slow subscriber
	// indefinitely.
	//
	// Setting CloseOnSlowClient=true tells CometBFT to forcibly close any WS
	// subscriber that cannot keep up with its subscription buffer. This is the
	// upstream-recommended defense against the failure mode and protects mainnet
	// validators from any third-party client (indexer, relayer, custom tooling)
	// that exhibits the same pattern — sdk-go-based or otherwise.
	//
	// Trade-off: well-behaved clients that subscribe to a high-volume query and
	// briefly stall (network blip, GC pause >subscription_buffer_size events)
	// will be disconnected and must reconnect. They already had to handle
	// reconnect for normal operational reasons (validator restart, network), so
	// this is not a new client-side requirement.
	//
	// Subscription caps are left at CometBFT defaults (100 clients × 5 subs each),
	// which are appropriate; no live evidence justifies changing them.
	cfg.RPC.CloseOnSlowClient = true

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
	srvCfg.Mempool.MaxTxs = 10000
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
				TrustedProxies: "",
			},
		},
	}

	customAppTemplate := serverconfig.DefaultConfigTemplate + cosmosevmserverconfig.DefaultEVMConfigTemplate + lumeraConfigTemplate

	return customAppTemplate, customAppConfig
}
