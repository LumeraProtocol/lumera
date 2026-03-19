# Lumera Ports: Defaults, Config Keys, and CLI Flags

This document lists network ports used by `lumerad`, with:

- **Default bind address/port**
- **Config file option** (`config.toml` /`app.toml`)
- **Command-line flag** (when available)

---

## Quick reference

| Service                       | Default                                           | Config key                                                    | CLI flag                                       |
| ----------------------------- | ------------------------------------------------- | ------------------------------------------------------------- | ---------------------------------------------- |
| P2P (CometBFT)                | `tcp://0.0.0.0:26656`                           | `config.toml` → `[p2p] laddr`                            | `--p2p.laddr`                                |
| RPC (CometBFT HTTP/WebSocket) | `tcp://127.0.0.1:26657`                         | `config.toml` → `[rpc] laddr`                            | `--rpc.laddr`                                |
| ABCI app socket               | `tcp://0.0.0.0:26658`                           | `config.toml` / startup (`address` / `proxy_app`)       | `--address`, `--proxy_app`                 |
| Cosmos API (REST)             | `tcp://0.0.0.0:1317` (commonly used)            | `app.toml` → `[api] address`                             | `--api.enable` (enable), address from config |
| gRPC                          | `localhost:9090`                                | `app.toml` → `[grpc] address`                            | `--grpc.enable`, `--grpc.address`          |
| gRPC-Web                      | `0.0.0.0:9900`                                  | `app.toml` → `[grpc-web] address`                        | `--grpc-web.enable`, `--grpc-web.address`  |
| Ethereum JSON-RPC (HTTP)      | `127.0.0.1:8545`                                | `app.toml` → `[json-rpc] address`                        | `--json-rpc.enable`, `--json-rpc.address`  |
| Ethereum JSON-RPC (WS)        | `127.0.0.1:8546`                                | `app.toml` → `[json-rpc] ws-address`                     | `--json-rpc.ws-address`                      |
| CometBFT pprof                | disabled unless set                               | `config.toml` → `[rpc] pprof_laddr`                      | `--rpc.pprof_laddr`                          |
| EVM geth metrics              | `127.0.0.1:8100`                                | `app.toml` → `[evm] geth-metrics-address`                | `--evm.geth-metrics-address`                 |
| EVM JSON-RPC rate-limit proxy | `0.0.0.0:8547` (disabled by default)            | `app.toml` → `[lumera.json-rpc-ratelimit] proxy-address` | — (config only)                               |
| EVM JSON-RPC metrics          | (app config; testnet commonly `127.0.0.1:6065`) | `app.toml` → `[json-rpc] metrics-address`                | `--metrics` (enables metrics server)         |

> Notes:
>
> - Some services are disabled by default and only bind when enabled (e.g., API, gRPC, gRPC-Web, JSON-RPC depending on config).
> - Lumera app defaults enable EVM JSON-RPC and indexer in app config initialization; runtime can still override via flags or`app.toml`.

---

## Detailed port table (with descriptions)

| Port / Endpoint           | Service                       | What it is used for                                                                                |
| ------------------------- | ----------------------------- | -------------------------------------------------------------------------------------------------- |
| `26656`                 | CometBFT P2P                  | Peer discovery, gossip, block/tx propagation between validators/full nodes.                        |
| `26657`                 | CometBFT RPC (HTTP/WS)        | Node status, blocks, tx query, broadcast endpoints (`/status`, `/block`, `/broadcast_tx_*`). |
| `26658`                 | ABCI app socket               | Internal CometBFT ↔ app communication (not for public clients).                                   |
| `1317`                  | Cosmos REST API               | Cosmos SDK REST + gRPC-gateway routes (module query endpoints).                                    |
| `9090`                  | Cosmos gRPC                   | Native protobuf gRPC for SDK queries/tx workflows.                                                 |
| `9900`                  | Cosmos gRPC-Web               | Browser-compatible gRPC over HTTP/1.1 for web clients.                                             |
| `8545`                  | EVM JSON-RPC HTTP             | Ethereum-compatible HTTP RPC (`eth_*`, `net_*`, `web3_*`, etc.).                             |
| `8546`                  | EVM JSON-RPC WS               | Ethereum WebSocket RPC, subscriptions (`eth_subscribe`, pending tx, logs, heads).                |
| `8547`                  | EVM JSON-RPC rate-limit proxy | Per-IP rate-limiting reverse proxy forwarding to `:8545`. Disabled by default.                   |
| `6060` (example)        | CometBFT pprof                | Runtime profiling/debug endpoints (`/debug/pprof/*`). Disabled unless configured.                |
| `8100`                  | EVM geth metrics              | EVM/geth metrics endpoint for monitoring pipelines.                                                |
| `6065` (common testnet) | EVM JSON-RPC metrics          | Metrics endpoint for JSON-RPC server (when enabled).                                               |

---

## Example requests by port

> Replace host/port if your node uses non-default values.

### 26656 (P2P)

P2P is not an HTTP API. Basic reachability check:

```bash
nc -vz 127.0.0.1 26656
```

### 26657 (CometBFT RPC)

```bash
# Node status
curl -s http://127.0.0.1:26657/status | jq

# Latest block
curl -s "http://127.0.0.1:26657/block" | jq
```

### 26658 (ABCI socket)

ABCI is internal transport; typically no direct client request. Reachability check only:

```bash
nc -vz 127.0.0.1 26658
```

### 1317 (Cosmos REST)

```bash
# Bank balances (example)
curl -s "http://127.0.0.1:1317/cosmos/bank/v1beta1/balances/<address>" | jq
```

### 9090 (gRPC)

```bash
# List protobuf services
grpcurl -plaintext 127.0.0.1:9090 list
```

### 9900 (gRPC-Web)

gRPC-Web uses HTTP transport with gRPC-Web headers and protobuf-framed payloads.
It does **not** use JSON-RPC request bodies.

Basic reachability:

```bash
nc -vz 127.0.0.1 9900
```

CORS preflight example:

```bash
curl -i -X OPTIONS http://127.0.0.1:9900/cosmos.bank.v1beta1.Query/Balance \
  -H 'Origin: http://localhost:3000' \
  -H 'Access-Control-Request-Method: POST' \
  -H 'Access-Control-Request-Headers: content-type,x-grpc-web,x-user-agent'
```

Example gRPC-Web POST (binary framed protobuf body):

```bash
curl -i http://127.0.0.1:9900/cosmos.bank.v1beta1.Query/Balance \
  -H 'Content-Type: application/grpc-web+proto' \
  -H 'X-Grpc-Web: 1' \
  -H 'X-User-Agent: grpc-web-javascript/0.1' \
  --data-binary @balance_request.bin
```

If you want CLI-friendly JSON input, use gRPC on `9090` with `grpcurl`:

```bash
grpcurl -plaintext \
  -d '{"address":"<addr>","denom":"ulume"}' \
  127.0.0.1:9090 cosmos.bank.v1beta1.Query/Balance
```

### 8545 (EVM JSON-RPC HTTP)

```bash
# Chain ID
curl -s -X POST http://127.0.0.1:8545 \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","method":"eth_chainId","params":[],"id":1}' | jq

# Latest block number
curl -s -X POST http://127.0.0.1:8545 \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":2}' | jq
```

### 8546 (EVM JSON-RPC WebSocket)

```bash
# Example with websocat: subscribe to new heads
printf '{"jsonrpc":"2.0","id":1,"method":"eth_subscribe","params":["newHeads"]}\n' \
  | websocat ws://127.0.0.1:8546
```

### 8547 (EVM JSON-RPC rate-limit proxy)

Disabled by default. Enable in `app.toml` → `[lumera.json-rpc-ratelimit]`.
When enabled, use this port instead of `8545` for external/public-facing traffic.

```bash
# Same JSON-RPC calls as 8545, routed through the rate-limiting proxy
curl -s -X POST http://127.0.0.1:8547 \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' | jq

# When rate limit is exceeded, returns HTTP 429:
# {"jsonrpc":"2.0","error":{"code":-32005,"message":"rate limit exceeded"},"id":null}
```

### 6060 (pprof)

```bash
# Profile index (when enabled)
curl -s http://127.0.0.1:6060/debug/pprof/ | head
```

### 8100 (geth metrics)

```bash
# Metrics payload (format depends on config/runtime)
curl -s http://127.0.0.1:8100/metrics | head
```

### 6065 (JSON-RPC metrics)

```bash
# JSON-RPC metrics endpoint (when --metrics is enabled)
curl -s http://127.0.0.1:6065/metrics | head
```

---

## Detailed service mapping

## 1) P2P listener (peer gossip)

- **Purpose:** node-to-node networking.
- **Default:**`tcp://0.0.0.0:26656`
- **Config:**`config.toml` →`[p2p] laddr`
- **CLI:**`--p2p.laddr`
- Related:
  - `--p2p.external-address`
  - `--p2p.seeds`
  - `--p2p.persistent_peers`

## 2) CometBFT RPC listener

- **Purpose:** status, block, tx query endpoints (HTTP + WebSocket).
- **Default:**`tcp://127.0.0.1:26657`
- **Config:**`config.toml` →`[rpc] laddr`
- **CLI:**`--rpc.laddr`
- Related:
  - `--rpc.unsafe`
  - `--rpc.grpc_laddr` (BroadcastTx gRPC endpoint)

## 3) ABCI app listener

- **Purpose:** CometBFT ↔ app communication.
- **Default:**`tcp://0.0.0.0:26658`
- **Config:** startup transport/proxy settings
- **CLI:**`--address`,`--proxy_app`,`--transport`,`--abci`

## 4) Cosmos SDK REST API

- **Purpose:** REST/HTTP API.
- **Common default:**`tcp://0.0.0.0:1317` in testnet tooling.
- **Config:**`app.toml` →`[api] address`
- **CLI:**`--api.enable` (enable/disable)
- Related:
  - `--api.enabled-unsafe-cors`

## 5) Cosmos SDK gRPC API

- **Purpose:** gRPC query/tx services.
- **Default:**`localhost:9090`
- **Config:**`app.toml` →`[grpc] address`
- **CLI:**`--grpc.enable`,`--grpc.address`

## 6) Cosmos SDK gRPC-Web API

- **Purpose:** browser-compatible gRPC over HTTP.
- **Default:**`0.0.0.0:9900`
- **Config:**`app.toml` →`[grpc-web] address`
- **CLI:**`--grpc-web.enable`,`--grpc-web.address`

## 7) EVM JSON-RPC HTTP

- **Purpose:** Ethereum-compatible RPC (e.g.,`eth_*`,`net_*`,`web3_*`).
- **Default:**`127.0.0.1:8545`
- **Config:**`app.toml` →`[json-rpc] address`
- **CLI:**`--json-rpc.enable`,`--json-rpc.address`
- Related namespace/config flags:
  - `--json-rpc.api`
  - `--json-rpc.enable-indexer`
  - `--json-rpc.http-timeout`
  - `--json-rpc.http-idle-timeout`
  - `--json-rpc.max-open-connections`

## 8) EVM JSON-RPC WebSocket

- **Purpose:** subscriptions (`eth_subscribe`) and WS transport.
- **Default:**`127.0.0.1:8546`
- **Config:**`app.toml` →`[json-rpc] ws-address`
- **CLI:**`--json-rpc.ws-address`,`--json-rpc.ws-origins`

## 9) EVM JSON-RPC rate-limit proxy

- **Purpose:** Per-IP token bucket rate limiting for the EVM JSON-RPC endpoint. Reverse-proxies requests to the internal JSON-RPC server (`:8545`).
- **Default:**`0.0.0.0:8547` (disabled by default — must set`enable = true`)
- **Config:**`app.toml` →`[lumera.json-rpc-ratelimit]`
  - `enable` — toggle (default:`false`)
  - `proxy-address` — listen address
  - `requests-per-second` — sustained rate per IP (default:`50`)
  - `burst` — max burst per IP (default:`100`)
  - `entry-ttl` — inactivity TTL for per-IP state (default:`5m`)
- **CLI:** none (config-only)
- **Note:** When enabled, external clients should connect to this port; keep`:8545` on loopback for internal/trusted access.

## 10) CometBFT pprof listener

- **Purpose:** Go pprof diagnostics for RPC process.
- **Default:** disabled unless set.
- **Config:**`config.toml` →`[rpc] pprof_laddr`
- **CLI:**`--rpc.pprof_laddr`

## 11) EVM geth metrics listener

- **Purpose:** EVM/geth metrics endpoint.
- **Default:**`127.0.0.1:8100`
- **Config:**`app.toml` →`[evm] geth-metrics-address`
- **CLI:**`--evm.geth-metrics-address`

## 12) EVM JSON-RPC metrics listener

- **Purpose:** metrics endpoint for JSON-RPC server.
- **Common testnet port:**`127.0.0.1:6065`
- **Config:**`app.toml` →`[json-rpc] metrics-address`
- **CLI:**`--metrics` (enables EVM RPC metrics server)

---

## Configuration file locations

Given `--home <HOME>` (default `~/.lumera`), config files are typically:

- `<HOME>/config/config.toml`
- `<HOME>/config/app.toml`

---

## Testnet single-machine port conventions

`lumerad testnet` uses these base ports per node (with offsets):

- P2P:`26656 + i`
- RPC:`26657 + i`
- API:`1317 + i`
- gRPC:`9090 + i`
- pprof:`6060 + i`
- JSON-RPC HTTP:`8545 + (i * 100)`
- JSON-RPC WS:`8546 + (i * 100)`
- JSON-RPC metrics:`6065 + (i * 100)`
- geth metrics:`8100 + (i * 100)`

(Using `i*100` for EVM ports avoids JSON-RPC/WS collisions across nodes.)

---

## Security recommendations

- Keep sensitive endpoints on loopback unless explicitly needed:
  - `--rpc.laddr tcp://127.0.0.1:26657`
  - `--json-rpc.address 127.0.0.1:8545`
  - `--json-rpc.ws-address 127.0.0.1:8546`
- Expose P2P publicly only when operating a network node.
- Avoid`--rpc.unsafe` on public interfaces.
- If exposing API/gRPC publicly, place behind firewall/reverse proxy/TLS.
- For public EVM JSON-RPC access, enable the rate-limiting proxy (`[lumera.json-rpc-ratelimit] enable = true`) and expose`:8547` instead of`:8545` directly.

---

## Source hints in repository

- `cmd/lumera/cmd/testnet.go` (testnet default and offset logic)
- `cmd/lumera/cmd/config.go` (app config sections/default wiring)
- `lumerad start --help` (runtime flags and defaults)
- `devnet/tests/validator/ports_config.go` (port parsing and practical defaults)
