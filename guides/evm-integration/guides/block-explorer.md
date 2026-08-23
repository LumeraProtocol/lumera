# External Block Explorer Integration

Plan for deploying an EVM-compatible block explorer (Blockscout) for the Lumera chain.

## Existing Infrastructure

Lumera's JSON-RPC layer is well-prepared for external block explorer integration:

| Capability | Status | Details |
| --- | --- | --- |
| JSON-RPC namespaces | Ready | `eth`, `net`, `web3`, `rpc` enabled by default |
| EVM tracing | Ready | `debug_traceTransaction` supported (configurable via `--evm.tracer`) |
| JSON-RPC indexer | Ready | Built-in indexer enabled by default (`enable-indexer = true` in app.toml) |
| CORS | Ready | Configurable allowed origins in `app/openrpc/http.go` |
| WebSocket | Ready | Available on port 8546 (Blockscout uses this for real-time block streaming) |
| Chain ID | Ready | `76857769` |
| Rate limiting | Ready | Per-IP token bucket on proxy port 8547 (`app/evm_jsonrpc_ratelimit.go`) |

## Deployment Steps

### 1. Deploy Blockscout instance (infra, not code)

Blockscout is a standalone Docker application that connects to the node's JSON-RPC endpoint. The main work is operational:

- Docker Compose with Blockscout + PostgreSQL + smart-contract-verifier
- Point `ETHEREUM_JSONRPC_HTTP_URL` at the Lumera node's port 8545
- Point `ETHEREUM_JSONRPC_WS_URL` at port 8546
- Set `CHAIN_ID=76857769`, `COIN=LUME`, `COIN_NAME=Lumera`

### 2. Dedicated archive node with `debug` namespace

Blockscout calls `debug_traceTransaction` for internal transaction indexing. Lumera's mainnet policy (`cmd/lumera/cmd/jsonrpc_policy.go`) blocks the `debug` namespace on mainnet for security. A **dedicated archive node** is needed:

- Enable `debug` namespace in `app.toml` (`api = "eth,net,web3,rpc,debug"`)
- Enable EVM tracer (`tracer = "json"` under `[evm]`)
- Network-isolate this node so only Blockscout can reach it (not public-facing)
- Ensure sufficient disk for archive mode (full historical state)

### 3. Verify Blockscout RPC compatibility

Blockscout requires these `eth_` methods, which cosmos/evm v0.6.0 should provide:

| Method | Purpose |
| --- | --- |
| `eth_getBlockByNumber` | Block indexing (with full tx objects) |
| `eth_getTransactionReceipt` | Tx receipt + logs indexing |
| `eth_getLogs` | Filter-based log queries |
| `eth_newBlockFilter` / `eth_getFilterChanges` | Block polling (or WebSocket `eth_subscribe`) |
| `debug_traceTransaction` | Internal transaction indexing |

**Risk**: Subtle differences in `debug_traceTransaction` output format between cosmos/evm and geth. Evmos had to patch tracer responses for Blockscout compatibility. Test early on devnet.

### 4. Handle the Cosmos/EVM duality

Lumera has both Cosmos txs and EVM txs. Blockscout only indexes the EVM side.

**Options**:

- **Dual explorer**: Blockscout for EVM txs + Ping.pub/Mintscan for Cosmos txs. This is what Evmos and Kava do.
- **Unified explorer**: Evaluate [Celatone](https://github.com/alleslabs/celatone-frontend) which handles both Cosmos + EVM in a single UI. No Cosmos EVM chain has deployed this yet, so it would be a first.
- **Minimum viable**: Blockscout only, with documentation that Cosmos-native txs (staking, governance, IBC) are visible via the Cosmos REST/gRPC API or CLI.

### 5. Smart contract verification

Blockscout supports source verification via its `smart-contract-verifier` microservice:

- Configure with correct Solidity compiler versions used in the ecosystem
- Enable Sourcify integration for standardized verification
- Document verification workflow for contract deployers

## Testing Plan

1. **Devnet deployment**: Stand up Blockscout against a local devnet (`make devnet-new`) and verify block/tx indexing
2. **Trace compatibility**: Deploy a contract, execute transactions, verify `debug_traceTransaction` output is parsed correctly by Blockscout
3. **ERC20 token display**: Verify IBC-originated ERC20 token pairs show correct metadata
4. **WebSocket real-time**: Confirm new blocks stream to Blockscout via WebSocket subscription
5. **Contract verification**: Test source verification flow end-to-end

## Scope

This is primarily an infrastructure/DevOps task — no protocol-level code changes are expected. The main effort is deploying, configuring, and testing the Blockscout stack against Lumera's existing JSON-RPC endpoint.
