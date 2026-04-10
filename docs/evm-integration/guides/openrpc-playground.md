# OpenRPC Discovery and Playground Guide

Lumera exposes a machine-readable API catalog via the [OpenRPC](https://open-rpc.org/) specification. This allows wallets, developer tools, and code generators to automatically discover every JSON-RPC method the node supports — including parameters, return types, and usage examples.

---

## Two access methods

| Method | Endpoint | Port (default) | Protocol | Use case |
|--------|----------|----------------|----------|----------|
| **JSON-RPC** | `rpc_discover` / `rpc.discover` | 8545 (EVM JSON-RPC) | POST | Programmatic discovery from dApps, scripts, or the OpenRPC Playground |
| **HTTP** | `/openrpc.json` | 1317 (Cosmos REST API) | GET/POST | Browser access, curl, CI pipelines, static documentation, and OpenRPC Playground proxying |

Both return the same embedded spec (~743 methods, ~5000 lines). The spec is regenerated on every `make build` from the actual Go RPC implementation, so it never drifts from the running code.

---

## Quick start

### Via JSON-RPC (`rpc_discover` or `rpc.discover`)

```bash
# From any machine that can reach the JSON-RPC port:
curl -s -X POST http://localhost:8545 \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"rpc_discover","params":[],"id":1}' | jq '.result.info'
```

Expected output:

```json
{
  "title": "Lumera Cosmos EVM JSON-RPC API",
  "version": "cosmos/evm v0.6.0",
  "description": "Auto-generated method catalog from Cosmos EVM JSON-RPC namespace implementations."
}
```

### Via HTTP (`/openrpc.json`)

```bash
# From any machine that can reach the REST API port:
curl -s http://localhost:1317/openrpc.json | jq '.info'
```

> **Note**: The HTTP endpoint is served by the Cosmos REST API server (port 1317), not the EVM JSON-RPC server (port 8545). Both must have `api.enable = true` and `json-rpc.enable = true` respectively in `app.toml`.

---

## Using the OpenRPC Playground

The [OpenRPC Playground](https://playground.open-rpc.org) is a browser-based interactive explorer that renders the spec as a searchable method list with live request execution.

### Connect via the `?url=` parameter

The playground loads the spec from an HTTP URL passed via the `url` query parameter.

You can point it at the **REST API** port (1317), which serves the `/openrpc.json` endpoint:

```text
https://playground.open-rpc.org/?url=http://localhost:1317/openrpc.json
```

You can also point it directly at the **JSON-RPC** port, which now supports both discovery names and works with **Try It**:

```text
https://playground.open-rpc.org/?url=http://localhost:8555
```

For a devnet validator, use the corresponding host-mapped REST API or JSON-RPC port (see devnet section below).

> **Why REST API works with the playground:** The playground loads the spec from `GET /openrpc.json`, then sends "Try It" POST requests back to the same endpoint. Lumera proxies `POST /openrpc.json` to the internal JSON-RPC server and rewrites `rpc.discover` to `rpc_discover` for compatibility with OpenRPC tooling.

If you bypass `/openrpc.json` and point tooling directly at the JSON-RPC port, Lumera accepts both the native `rpc_discover` name and the OpenRPC-style `rpc.discover` alias, and the playground's **Try It** requests execute against that same JSON-RPC endpoint.

### Browse and execute

- The left panel lists all available methods grouped by namespace (`eth`, `net`, `web3`, `debug`, `txpool`, `rpc`, etc.)
- Click a method to see its parameters, return type, and examples
- Click **"Try It"** to execute the method against the connected node
- Results appear inline with syntax highlighting

---

## Devnet validator access

Each devnet validator maps its container ports to unique host ports. The relevant ports for OpenRPC:

| Validator | JSON-RPC (8545) | REST API (1317) | WebSocket (8546) |
|-----------|-----------------|-----------------|------------------|
| validator_1 | `localhost:8545` | `localhost:1327` | `localhost:8546` |
| validator_2 | `localhost:8555` | `localhost:1337` | `localhost:8556` |
| validator_3 | `localhost:8565` | `localhost:1347` | `localhost:8566` |
| validator_4 | `localhost:8575` | `localhost:1357` | `localhost:8576` |
| validator_5 | `localhost:8585` | `localhost:1367` | `localhost:8586` |

> Port mappings are defined in `devnet/docker-compose.yml`. Verify with:
>
> ```bash
> docker compose -f devnet/docker-compose.yml port supernova_validator_2 8545
> docker compose -f devnet/docker-compose.yml port supernova_validator_2 1317
> ```

### Example: validator 2

**Playground URL**: <https://playground.open-rpc.org/?url=http://localhost:1337/openrpc.json>

**CLI quick test**:

```bash
# rpc_discover via JSON-RPC
curl -s -X POST http://localhost:8555 \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"rpc_discover","params":[],"id":1}' | jq '.result.methods | length'
# Expected: 743 (or similar)

# /openrpc.json via REST API
curl -s http://localhost:1337/openrpc.json | jq '.methods | length'
```

### WSL users

If running the devnet inside WSL2, `localhost` port forwarding works automatically on recent Windows builds. Open the playground in your Windows browser with `?url=http://localhost:1337/openrpc.json` directly.

If port forwarding is not working, use the WSL IP address:

```bash
# Find the WSL IP
hostname -I | awk '{print $1}'
# Then use http://<wsl-ip>:8555 in the playground
```

---

## CORS configuration

The `/openrpc.json` HTTP endpoint and WebSocket server share the same CORS origin list, configured in `app.toml`:

```toml
[json-rpc]
ws-origins = ["127.0.0.1", "localhost"]
```

| Setting | Effect on Playground |
|---------|---------------------|
| `["127.0.0.1", "localhost"]` (default) | Works from local browser only |
| `["*"]` | Allows any origin (devnet/testnet only) |
| `["https://playground.open-rpc.org"]` | Allows the hosted playground specifically |

For **devnet**, `ws-origins` is typically set to allow all origins. For **production**, restrict to specific domains.

> **Note**: `POST /openrpc.json` uses the REST API server's CORS policy (reused from `[json-rpc] ws-origins`). Direct POSTs to the JSON-RPC port still use the native JSON-RPC CORS behavior; Lumera accepts both `rpc_discover` and `rpc.discover` there for compatibility.

---

## Configuration requirements

For OpenRPC to work, ensure these are set in `app.toml`:

```toml
[json-rpc]
enable = true
# The "rpc" namespace must be in the API list:
api = "eth,net,web3,rpc"
```

The `rpc` namespace is included by default in Lumera's config (added by `EnsureNamespaceEnabled` during config initialization and migration). If you customized the `api` list, make sure `rpc` is still included.

The HTTP endpoint (`/openrpc.json`) additionally requires:

```toml
[api]
enable = true
```

---

## Regenerating the spec

The OpenRPC spec is embedded in the binary at build time. To regenerate after adding or modifying JSON-RPC methods:

```bash
make openrpc
# Regenerates: docs/openrpc.json + app/openrpc/openrpc.json
# Next `make build` will embed the updated spec
```

The spec is also regenerated automatically as a dependency of `make build`.

---

## Useful queries

```bash
# List all available methods
curl -s -X POST http://localhost:8555 \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"rpc_discover","params":[],"id":1}' \
  | jq '[.result.methods[].name] | sort'

# List methods by namespace
curl -s -X POST http://localhost:8555 \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"rpc_discover","params":[],"id":1}' \
  | jq '[.result.methods[].name] | group_by(split("_")[0]) | map({namespace: .[0] | split("_")[0], count: length})'

# Get details for a specific method
curl -s -X POST http://localhost:8555 \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"rpc_discover","params":[],"id":1}' \
  | jq '.result.methods[] | select(.name == "eth_sendRawTransaction")'
```
