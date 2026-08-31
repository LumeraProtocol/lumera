# Devnet EVM Host-Port Scheme: Version-Gated + ×100 Renumber

**Date:** 2026-06-22
**Status:** Design (pending implementation)

## Problem

`make devnet-new-remote-version VERSION=v1.12.0` fails on the remote host with:

```
failed to bind host port for 0.0.0.0:8545: address already in use
```

### Root cause

Three facts combine:

1. The remote devnet host already runs a **standalone, bare-metal `lumerad`** (separate from the Docker devnet) listening on `127.0.0.1:8545`.
2. The generated `devnet/docker-compose.yml` publishes host `8545:8545` for `validator_1` (from `validators.json` `json-rpc.port: 8545`).
3. Docker reserves the **host** port at container start regardless of whether the in-container process listens, and binding `0.0.0.0:8545` fails when `127.0.0.1:8545` is already held (the wildcard bind overlaps loopback).

### Two distinct defects this exposes

- **Pre-EVM installs still publish EVM ports.** EVM is enabled only from `DefaultEVMFromVersion = "v1.20.0"` (`devnet/config/config.go`). v1.12.0 is pre-EVM, so the in-container lumerad never serves JSON-RPC — yet the generator publishes 8545/8546 **unconditionally** (`devnet/generators/docker-compose.go:281-286` only checks `JSONRPC.Port > 0`, never `evmFromVersion`). These mappings are dead weight that still cause the collision.
- **Tight ×10 spacing + only http/ws.** The current scheme is base + i×10 (8545, 8555, 8565, 8575, 8585), publishing only http/ws. It leaves no room for metrics/geth-metrics ports and keeps `validator_1` on the canonical 8545 — the port most likely to be squatted by any stray node.

## Goals

1. **Version-gate** EVM host-port publishing: emit EVM port mappings only when `chainVersion >= evmFromVersion`. A pre-EVM devnet (e.g. v1.12.0) publishes **no** EVM ports → the collision disappears.
2. **Renumber** EVM host ports to a clean per-validator ×100 block that **vacates 8545**, and expose all four EVM ports (http, ws, json-rpc metrics, geth metrics) for every validator.
3. Keep `lumera-ports.md` accurate.

## Non-goals

- Changing the `lumerad testnet` command's port convention (`cmd/lumera/cmd/testnet.go`, `evmPortOffset = portOffset*100`, node 0 keeps 8545). That is a different mechanism (one host netns, N nodes) and stays as-is.
- Touching the standalone bare-metal node on the remote host. Gating + renumber make the devnet coexist with it without removing it.

## Design

### Container-side ports (fixed by lumerad / Cosmos EVM v0.6.0)

| Service | Container address |
| --- | --- |
| JSON-RPC HTTP | `0.0.0.0:8545` |
| JSON-RPC WS | `0.0.0.0:8546` |
| JSON-RPC metrics | `0.0.0.0:6065` (default `127.0.0.1`) |
| geth metrics | `0.0.0.0:8100` (default `127.0.0.1`) |

Each validator runs in its own container netns, so container-side addresses are identical across validators. Only the **host-published** port differs.

> The default metrics/geth bind addresses are `127.0.0.1`. Docker can forward a host port to a container port, but the in-container server must bind `0.0.0.0` inside its own netns or forwarded traffic is dropped. So metrics/geth addresses must be set to `0.0.0.0` in app.toml.

### Host-port scheme (contiguous block per validator, stride 100, start 8645)

For validator index `i` (0-based):

| Service | Host port | → Container |
| --- | --- | --- |
| JSON-RPC HTTP | `8645 + i*100` | `:8545` |
| JSON-RPC WS | `8646 + i*100` | `:8546` |
| JSON-RPC metrics | `8647 + i*100` | `:6065` |
| geth metrics | `8648 + i*100` | `:8100` |

Concrete:

| Validator | http | ws | metrics | geth |
| --- | --- | --- | --- | --- |
| validator_1 (i=0) | 8645 | 8646 | 8647 | 8648 |
| validator_2 (i=1) | 8745 | 8746 | 8747 | 8748 |
| validator_3 (i=2) | 8845 | 8846 | 8847 | 8848 |
| validator_4 (i=3) | 8945 | 8946 | 8947 | 8948 |
| validator_5 (i=4) | 9045 | 9046 | 9047 | 9048 |

8545 is left free. The 8645–9048 range does not collide with existing devnet host ports (rest 13xx, grpc 909x, debug 4000x, supernode 74xx/18xx, cometbft 266xx).

### Components to change

1. **`devnet/config/validators.json`** — replace each validator's `json-rpc` block with the four-port scheme above (add `metrics_port`, `geth_metrics_port`; renumber `port`/`ws_port`).

2. **`devnet/config/config.go`** — extend the per-validator `JSONRPC` struct (currently `Port`, `WSPort`) with `MetricsPort` and `GethMetricsPort` (json tags `metrics_port`, `geth_metrics_port`, `omitempty`).

3. **`devnet/generators/docker-compose.go`** —
   - Add constants `DefaultJSONRPCMetricsPort = 6065`, `DefaultGethMetricsPort = 8100`.
   - Add a Go semver-compare helper (or use `golang.org/x/mod/semver` if already a dependency) for `chainVersion >= evmFromVersion`.
   - Wrap the whole EVM port-emission block in the version gate. When gated off (pre-EVM), emit **none** of the four EVM mappings.
   - When on, emit all four: `port→8545`, `ws_port→8546`, `metrics_port→6065`, `geth_metrics_port→8100`.

4. **`devnet/config/config.json`** — chain-level `json-rpc`: add `metrics_address: "0.0.0.0:6065"` and a geth metrics address (`[evm] geth-metrics-address` equivalent) `"0.0.0.0:8100"`, plus a flag indicating metrics should be enabled.

5. **`devnet/scripts/validator-setup.sh`** — write `[json-rpc] metrics-address` and `[evm] geth-metrics-address` to app.toml (bind `0.0.0.0`). Source the values from chain config like the existing json-rpc keys.

6. **`devnet/scripts/start.sh`** — pass `--metrics` to `lumerad start` so the JSON-RPC metrics server actually starts (gated on EVM version via existing bash `version_ge`). Verify geth-metrics server enablement (may require a geth metrics global enable in addition to the address).

7. **Tests** — update hardcoded expectations: `devnet/tests/validator/ports_config.go`, `devnet/tests/validator/evm_test.go`, `devnet/tests/validator/ports_test.go`, and any genesis/fixtures referencing 854x–858x. Add a generator test asserting the version gate (pre-EVM → no EVM ports; EVM → four ports at the computed numbers).

8. **`docs/lumera-ports.md`** — add a new "Devnet docker host-port scheme" section documenting the contiguous ×100 block and the gate. Leave the existing `lumerad testnet` convention section unchanged.

## Behavior / verification

- **v1.12.0 (pre-EVM):** generator emits zero EVM host ports → `make devnet-new-remote-version VERSION=v1.12.0` no longer collides with the standalone node. Other devnet ports unchanged.
- **>= v1.20.0 (EVM):** each validator publishes its 8645+ block; `curl host:8645 eth_chainId`, ws on 8646, metrics on 8647, geth on 8648 (and the +100 bands for validators 2–5).
- Generator unit test covers both branches.

## Error handling / edge cases

- If `chainVersion` can't be resolved, fail closed (treat as pre-EVM / no EVM ports) rather than publishing ports that won't serve.
- A validator with `json-rpc.port == 0` still emits nothing for that service (preserves the existing opt-out).
- Metrics/geth host ports are published but their servers only run when EVM is enabled AND `--metrics`/geth address are set — gate the start.sh flags on the same version check to avoid flags on a pre-EVM binary that doesn't recognize them.
