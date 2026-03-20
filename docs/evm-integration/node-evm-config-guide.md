# Node Operator EVM Configuration Guide

This guide covers every EVM-related configuration option available in `app.toml`, relevant CometBFT settings, command-line overrides, and production tuning recommendations for Lumera node operators.

**Chain constants** (not configurable — hardcoded in `config/evm.go`):

| Constant | Value | Purpose |
|----------|-------|---------|
| EVM Chain ID | `76857769` | EIP-155 replay protection |
| Native denom | `ulume` (6 decimals) | Cosmos-side token |
| Extended denom | `alume` (18 decimals) | EVM-side token (via `x/precisebank`) |
| Key type | `eth_secp256k1` | Ethereum-compatible keys |
| Coin type | `60` | BIP44 HD path (same as Ethereum) |

---

## Automatic Config Migration (v1.12.0+)

Nodes upgrading from a pre-EVM binary (< v1.12.0) will have an `app.toml` that lacks the `[evm]`, `[evm.mempool]`, `[json-rpc]`, `[tls]`, and `[lumera.*]` sections. The Cosmos SDK only generates `app.toml` when the file does not exist, so these sections are never added automatically during a binary upgrade.

Starting with v1.12.0, `lumerad` includes a **config migration helper** (`cmd/lumera/cmd/config_migrate.go`) that runs on every startup:

1. Checks whether `evm.evm-chain-id` in the loaded config matches the Lumera constant (`76857769`).
2. If it does not match (absent section defaults to the upstream cosmos/evm value `262144`, or `0` for entirely missing keys):
   - Reads all existing settings from the current `app.toml` via Viper.
   - Merges them with Lumera's EVM defaults (correct chain ID, JSON-RPC enabled, indexer enabled, `rpc` namespace for OpenRPC).
   - Regenerates `app.toml` with the full template, preserving all operator customizations.
3. Logs an `INFO` message when migration occurs.

**No manual action is required.** After upgrading the binary and restarting, the node will automatically add the missing EVM configuration sections with safe defaults. Operators can then customize settings as described below.

---

## 1. `[evm]` — Core EVM Module

Controls the `x/vm` EVM execution engine.

```toml
[evm]
# VM tracer for debug mode. Enables debug_traceTransaction, debug_traceBlockByNumber,
# debug_traceBlockByHash, debug_traceCall JSON-RPC methods when set.
# Values: "" (disabled), "json", "struct", "access_list", "markdown"
tracer = ""

# Gas wanted for each Ethereum tx in ante handler CheckTx mode.
# 0 = use the gas limit from the tx itself.
max-tx-gas-wanted = 0

# Enable SHA3 preimage recording in the EVM.
# Only useful for certain debugging/tracing scenarios.
cache-preimage = false

# EIP-155 chain ID. Must match the network's genesis chain ID.
# Do NOT change this on an existing chain.
evm-chain-id = 76857769

# Minimum priority fee (tip) for mempool acceptance, in wei.
# 0 = no minimum tip required beyond base fee.
min-tip = 0

# Address to bind the Geth-compatible metrics server.
geth-metrics-address = "127.0.0.1:8100"
```

### Tuning notes

- **`tracer`**: Leave empty in production. Enable `"json"` temporarily for debugging specific transactions via `debug_traceTransaction`. The `"struct"` tracer is useful for programmatic analysis. Enabling any tracer adds overhead to every EVM call.
- **`max-tx-gas-wanted`**: Useful if you want to cap the gas that CheckTx considers for mempool admission. Generally leave at 0 unless you see mempool spam with inflated gas limits.
- **`min-tip`**: Increase this on validators that want to prioritize higher-fee transactions. Value is in wei (18-decimal `alume`), so `1000000000` = 1 gwei tip minimum.

---

## 2. `[evm.mempool]` — EVM Transaction Pool

Controls the app-side EVM mempool (backed by `ExperimentalEVMMempool`). These mirror geth's txpool settings.

```toml
[evm.mempool]
# Minimum gas price to accept into the pool (in wei).
price-limit = 1

# Minimum price bump percentage to replace an existing tx (same nonce).
price-bump = 10

# Executable transaction slots guaranteed per account.
account-slots = 16

# Maximum executable transaction slots across all accounts.
global-slots = 5120

# Maximum non-executable (queued) transaction slots per account.
account-queue = 64

# Maximum non-executable transaction slots across all accounts.
global-queue = 1024

# Maximum time non-executable transactions are queued.
lifetime = "3h0m0s"
```

### Tuning notes

- **`global-slots`**: The primary knob for mempool capacity. Increase for high-throughput validators; decrease on resource-constrained sentries. The app-level `mempool.max-txs` (default `5000`) also bounds total mempool size.
- **`account-slots`**: Increase if you expect DeFi bots or relayers sending many txs per block from a single account.
- **`price-bump`**: The 10% default means a replacement tx must pay ≥110% of the original gas price. Increase to reduce churn from frequent replacements.
- **`lifetime`**: Shorten on public RPC nodes to reduce stale tx accumulation; lengthen on private validators that batch txs.

---

## 3. `[json-rpc]` — Ethereum JSON-RPC Server

Controls the HTTP and WebSocket JSON-RPC endpoints that serve Ethereum-compatible API calls.

```toml
[json-rpc]
# Enable the JSON-RPC server.
enable = true

# HTTP JSON-RPC bind address.
address = "127.0.0.1:8545"

# WebSocket JSON-RPC bind address.
ws-address = "127.0.0.1:8546"

# Allowed WebSocket origins. Add your domain for browser dApp access.
# Also controls CORS for the /openrpc.json HTTP endpoint.
ws-origins = ["127.0.0.1", "localhost"]

# Enabled JSON-RPC namespaces (comma-separated).
# Available: eth, net, web3, rpc, debug, personal, admin, txpool, miner
api = "eth,net,web3,rpc"

# Gas cap for eth_call and eth_estimateGas. 0 = unlimited.
gas-cap = 25000000

# Allow insecure account unlocking via HTTP.
allow-insecure-unlock = true

# Global timeout for eth_call / eth_estimateGas.
evm-timeout = "5s"

# Transaction fee cap for eth_sendTransaction (in ETH-equivalent).
txfee-cap = 1

# Maximum number of concurrent filters (eth_newFilter, eth_newBlockFilter, etc).
filter-cap = 200

# Maximum blocks returned by eth_feeHistory.
feehistory-cap = 100

# Maximum log entries returned by a single eth_getLogs call.
logs-cap = 10000

# Maximum block range for eth_getLogs.
block-range-cap = 10000

# HTTP read/write timeout.
http-timeout = "30s"

# HTTP idle connection timeout.
http-idle-timeout = "2m0s"

# Allow non-EIP155 (unprotected) transactions.
# Keep false in production — unprotected txs are replay-vulnerable.
allow-unprotected-txs = false

# Maximum simultaneous connections. 0 = unlimited.
max-open-connections = 0

# Enable custom Ethereum transaction indexer.
# Required for eth_getTransactionReceipt, eth_getLogs, etc.
enable-indexer = true

# Prometheus metrics endpoint for EVM/RPC performance.
metrics-address = "127.0.0.1:6065"

# Maximum requests in a single JSON-RPC batch call.
batch-request-limit = 1000

# Maximum bytes in a batched response.
batch-response-max-size = 25000000

# Enable pprof profiling in the debug namespace.
enable-profiling = false
```

### Tuning notes

- **`address` / `ws-address`**: Bind to `0.0.0.0` only if behind a reverse proxy or firewall. Never expose raw JSON-RPC to the public internet without rate limiting.
- **`ws-origins`**: Controls allowed origins for both WebSocket connections **and** the `/openrpc.json` HTTP endpoint CORS. On production nodes, set this to your specific domains (e.g., `["https://explorer.lumera.io", "https://app.lumera.io"]`). The default `["127.0.0.1", "localhost"]` is safe but will block browser-based dApps on other origins. An empty list or `["*"]` allows all origins (suitable for dev/testnet only).
- **`api`**: On mainnet, `debug`, `personal`, and `admin` namespaces are **automatically rejected** at startup by `jsonrpc_policy.go`. On testnets all namespaces are allowed. To enable tracing on testnet, use `api = "eth,net,web3,rpc,debug"` and set `[evm] tracer`.
- **`gas-cap`**: Limits compute for `eth_call`. Reduce if public-facing nodes are hit with expensive view calls.
- **`evm-timeout`**: Reduce to `2s` or `3s` on public RPC nodes to prevent slow `eth_call` from tying up resources.
- **`logs-cap` / `block-range-cap`**: Reduce on public nodes to prevent expensive `eth_getLogs` scans. Values of `1000`–`2000` are common for public endpoints.
- **`batch-request-limit`**: Reduce to `50`–`100` on public nodes to limit batch abuse.
- **`max-open-connections`**: Set to `100`–`500` on public nodes to prevent connection exhaustion.
- **`enable-indexer`**: Must be `true` for receipt/log queries. Disabling saves disk I/O but breaks most dApp interactions.
- **`allow-insecure-unlock`**: Set to `false` in production if you do not use server-side wallets.
- **`allow-unprotected-txs`**: Keep `false`. Only enable for legacy tooling that cannot produce EIP-155 signatures.

---

## 4. `[lumera.json-rpc-ratelimit]` — Per-IP Rate Limiting Proxy

Lumera-specific reverse proxy that sits in front of JSON-RPC with per-IP token bucket rate limiting.

```toml
[lumera.json-rpc-ratelimit]
# Enable the rate-limiting proxy.
enable = false

# Address the proxy listens on.
# Clients connect here; proxy forwards to [json-rpc] address.
proxy-address = "0.0.0.0:8547"

# Sustained requests per second per IP.
requests-per-second = 50

# Burst capacity per IP (token bucket size).
burst = 100

# Time-to-live for per-IP rate limiter entries.
entry-ttl = "5m"

# Comma-separated list of trusted reverse proxy CIDRs.
# X-Forwarded-For and X-Real-IP headers are only trusted from these sources.
# When empty (default), client IP is always derived from the socket peer address.
trusted-proxies = ""
```

### Tuning notes

- **Recommended for public RPC nodes**: Enable this and point external traffic to the proxy port (`8547`), while keeping the real JSON-RPC port (`8545`) on localhost.
- **`requests-per-second`**: 50 rps is generous for individual users. Reduce to `10`–`20` for heavily loaded public endpoints.
- **`burst`**: Allows short spikes. Set to 2–3× `requests-per-second` for a reasonable burst window.
- **`entry-ttl`**: Controls memory usage. Shorter TTL frees memory faster but may re-admit recently limited IPs sooner.
- **`trusted-proxies`**: Set this to the CIDRs of your load balancer / reverse proxy (e.g. `"10.0.0.0/8, 172.16.0.0/12"`). When empty, `X-Forwarded-For` and `X-Real-IP` headers are **ignored** and the rate limiter keys on the socket peer IP — this prevents clients from bypassing rate limits by spoofing headers. Single IPs (without `/mask`) are treated as `/32` (IPv4) or `/128` (IPv6).

### Deployment pattern

When the JSON-RPC alias proxy is active (the default), rate limiting is injected directly into the public port handler — no separate port is needed:

```
Internet → [alias proxy + rate-limit @ :8545] → [internal cosmos/evm server @ loopback]
```

When the alias proxy is disabled, a standalone rate-limit proxy listens on `proxy-address`:

```
Internet → [lumera.json-rpc-ratelimit @ :8547] → [json-rpc @ 127.0.0.1:8545]
```

---

## 5. `[lumera.evm-mempool]` — Broadcast Queue Debugging

Controls the async EVM broadcast dispatcher that prevents mempool re-entry deadlock.

```toml
[lumera.evm-mempool]
# Enable detailed logs for async broadcast queue processing.
# Shows enqueue, broadcast, dedup events. Useful for diagnosing
# stuck or dropped EVM transactions.
broadcast-debug = false
```

Enable temporarily when troubleshooting EVM transactions that appear to be accepted but never included in a block.

---

## 6. EIP-1559 Fee Market

The fee market is configured via genesis parameters (governable on-chain), not `app.toml`. Lumera's defaults differ from upstream Cosmos EVM:

| Parameter | Lumera Default | Upstream Default | Why |
|-----------|---------------|-----------------|-----|
| Base fee | 0.0025 ulume/gas | 1000000000 wei | Calibrated for ulume's 6-decimal precision |
| Min gas price | 0.0005 ulume/gas | 0 | Prevents base fee decaying to zero on idle chains |
| Change denominator | 16 (~6.25%/block) | 8 (~12.5%/block) | Gentler fee swings for a new chain |
| Max block gas | 25,000,000 | 30,000,000 | Conservative; increase via governance if needed |

**Operators cannot change these in `app.toml`** — they are consensus parameters. To modify, submit a governance proposal to update `x/feemarket` params.

### Monitoring recommendations

- Track `base_fee` via `eth_gasPrice` or `feemarket` query — sustained high fees indicate block gas limit is too low
- Track block gas utilization — sustained >50% target means base fee will keep rising
- Alert on base fee hitting the min floor — indicates very low network activity

---

## 7. Static Precompiles

Lumera enables 10 static precompiles at genesis. These are not configurable via `app.toml` — they are set in the EVM genesis state and can be toggled via governance.

| Address | Precompile | Purpose |
|---------|-----------|---------|
| `0x0100` | P256 | ECDSA P-256 signature verification |
| `0x0200` | Bech32 | Cosmos address codec (hex ↔ bech32) |
| `0x0300` | Staking | Delegate, undelegate, redelegate from EVM |
| `0x0400` | Distribution | Claim staking rewards from EVM |
| `0x0500` | ICS20 | IBC token transfers from EVM |
| `0x0600` | Bank | Native token transfers from EVM |
| `0x0700` | Governance | Submit votes from EVM |
| `0x0800` | Slashing | Query validator slashing info from EVM |
| `0x0901` | Action | Request/finalize/approve Cascade & Sense actions from EVM |
| `0x0902` | Supernode | Register/manage supernodes and query metrics from EVM |

**Note**: Native sends to precompile addresses are blocked by a bank send restriction to prevent accidental token loss.

---

## 8. Tracer Configuration

EVM tracing enables `debug_*` JSON-RPC methods for transaction-level execution analysis.

### Enabling tracing

1. Set the tracer type in `app.toml`:
   ```toml
   [evm]
   tracer = "json"
   ```

2. Enable the `debug` namespace in JSON-RPC:
   ```toml
   [json-rpc]
   api = "eth,net,web3,rpc,debug"
   ```

3. Restart the node.

### Tracer types

| Tracer | Output | Use case |
|--------|--------|----------|
| `json` | JSON opcode log | Human-readable debugging, compatible with most tools |
| `struct` | Structured Go objects | Programmatic analysis in Go tooling |
| `access_list` | EIP-2930 access list | Generate access lists for gas optimization |
| `markdown` | Markdown table | Documentation / reports |

### Security warning

**Never enable `debug` namespace on mainnet public RPC.** The `jsonrpc_policy.go` startup guard will reject this configuration on mainnet chains (`lumera-mainnet*` chain IDs). On testnets, tracing is allowed but adds significant CPU and memory overhead per traced call.

---

## 9. JSON-RPC Namespace Security Policy

Lumera enforces namespace restrictions based on chain ID at node startup (`cmd/lumera/cmd/jsonrpc_policy.go`):

| Chain type | Allowed | Blocked |
|-----------|---------|---------|
| Mainnet (`lumera-mainnet*`) | `eth`, `net`, `web3`, `rpc`, `txpool`, `miner` | `admin`, `debug`, `personal` |
| Testnet / Local | All namespaces | None |

If a mainnet node's `app.toml` includes a blocked namespace, the node **refuses to start** with a clear error message. This is a safety net — not a substitute for firewall rules.

---

## 10. Command-Line Overrides

These flags override `app.toml` values without editing the file. Useful for one-off debugging or container deployments.

```bash
# EVM module flags
lumerad start --evm.tracer json
lumerad start --evm.max-tx-gas-wanted 500000
lumerad start --evm.cache-preimage true
lumerad start --evm.evm-chain-id 76857769
lumerad start --evm.min-tip 1000000000

# JSON-RPC flags
lumerad start --json-rpc.enable true
lumerad start --json-rpc.address "0.0.0.0:8545"
lumerad start --json-rpc.ws-address "0.0.0.0:8546"
lumerad start --json-rpc.api "eth,net,web3,rpc,debug"
lumerad start --json-rpc.gas-cap 10000000
lumerad start --json-rpc.evm-timeout "3s"
```

---

## 11. CometBFT Settings (`config.toml`)

These CometBFT settings interact with EVM performance:

| Setting | Section | Default | EVM relevance |
|---------|---------|---------|---------------|
| `timeout_commit` | `[consensus]` | `5s` | Determines block time; shorter = faster EVM tx confirmation |
| `max_tx_bytes` | `[mempool]` | `1048576` | Max single tx size; large contract deploys may need increase |
| `max_txs_in_block` | `[mempool]` | `0` (unlimited) | Combined with app-side `max-txs` for total throughput |

---

## 12. Production Deployment Checklist

### Validator node

```toml
# Minimal RPC exposure — validators should not serve public JSON-RPC
[json-rpc]
enable = true                    # needed for local tooling
address = "127.0.0.1:8545"      # localhost only
ws-address = "127.0.0.1:8546"
api = "eth,net,web3,rpc"

[evm]
tracer = ""                      # no tracing overhead

[lumera.json-rpc-ratelimit]
enable = false                   # not needed on localhost
```

### Public RPC / sentry node

```toml
[json-rpc]
enable = true
address = "127.0.0.1:8545"      # behind rate-limit proxy
ws-address = "127.0.0.1:8546"
api = "eth,net,web3,rpc"
gas-cap = 10000000               # reduced for public safety
evm-timeout = "3s"               # tighter timeout
logs-cap = 2000                  # prevent expensive scans
block-range-cap = 2000
batch-request-limit = 50         # limit batch abuse
max-open-connections = 200

[evm]
tracer = ""

[lumera.json-rpc-ratelimit]
enable = true
proxy-address = "0.0.0.0:8547"  # public-facing port
requests-per-second = 20
burst = 50
entry-ttl = "5m"
trusted-proxies = ""             # set to LB CIDRs if behind a reverse proxy
```

### Archive / debugging node

```toml
[json-rpc]
enable = true
address = "127.0.0.1:8545"
api = "eth,net,web3,rpc,debug"   # debug enabled (testnet only)
gas-cap = 50000000               # higher for tracing
evm-timeout = "30s"              # longer for trace calls
logs-cap = 50000
block-range-cap = 50000

[evm]
tracer = "json"                  # enable tracing
cache-preimage = true            # for sha3 preimage lookups

[lumera.evm-mempool]
broadcast-debug = true           # for tx lifecycle debugging
```

---

## 13. Metrics & Monitoring

Lumera exposes two metrics endpoints for EVM observability:

| Endpoint | Default Address | Contents |
|----------|----------------|----------|
| EVM/RPC Metrics | `127.0.0.1:6065` | JSON-RPC request counts, latencies, error rates |
| Geth Metrics | `127.0.0.1:8100` | Internal EVM engine metrics |

Both are Prometheus-compatible. Add these to your monitoring stack alongside the standard CometBFT metrics (default `127.0.0.1:26660`).

### Key metrics to watch

- **JSON-RPC request rate & errors** — spike in errors may indicate client compatibility issues
- **EVM gas per block** — sustained high utilization triggers base fee increases
- **Mempool size** — growing queue suggests blocks are full or txs are stuck
- **Base fee** — track via `eth_gasPrice`; sudden spikes indicate demand surge or attack
