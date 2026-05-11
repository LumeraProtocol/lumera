# Lumera EVM Developer Guides

**Last updated**: 2026-05-09
**Applies to**: Lumera chain post-EVM upgrade (Cosmos EVM v0.6.0, EVM chain ID `76857769`)

This directory holds the developer-facing how-to guides for building on Lumera's EVM layer. Where [user-guides/](../user-guides/main.md) covers operating the chain (migrating accounts, configuring nodes, tuning parameters), this set covers *building against* it — discovering the JSON-RPC surface, deploying smart contracts, and standing up external explorer infrastructure. Architecture and internals live one level up under [main.md](../main.md).

## Who should read what

| You are… | Start here | Then |
| --- | --- | --- |
| A dApp / SDK developer wiring tools to Lumera | [openrpc-playground.md](openrpc-playground.md) | [remix-guide.md](remix-guide.md) for an interactive sanity check |
| A smart-contract developer deploying Solidity / Vyper | [remix-guide.md](remix-guide.md) | [openrpc-playground.md](openrpc-playground.md) once you need to script against the RPC surface |
| An infra / explorer operator | [block-explorer.md](block-explorer.md) | [openrpc-playground.md](openrpc-playground.md) for the canonical RPC method catalog |

## Guides

### [openrpc-playground.md](openrpc-playground.md) — OpenRPC discovery and the API catalog

Lumera ships a machine-readable [OpenRPC](https://open-rpc.org/) specification covering all ~743 JSON-RPC methods. The spec is regenerated on every `make build` directly from the Go RPC implementation, so it never drifts from running code. This guide documents the two access methods (`rpc_discover` over JSON-RPC port 8545, or `GET /openrpc.json` over Cosmos REST port 1317), shows how to load the spec into the OpenRPC Playground for an interactive method explorer, and covers the CORS / port choices on devnet, testnet, and mainnet.

### [remix-guide.md](remix-guide.md) — Smart contracts via Remix IDE + MetaMask

End-to-end walkthrough for deploying and interacting with a Solidity contract on Lumera using [Remix IDE](https://remix.ethereum.org) and MetaMask. Covers the MetaMask network configuration for both the public testnet and a local devnet (chain ID `76857769`, native currency `LUME` with 18 decimals on the EVM side), funding accounts with `LUME`, deploying through the Injected Provider, and reading transaction receipts via the JSON-RPC indexer.

### [block-explorer.md](block-explorer.md) — External block explorer integration (Blockscout)

Deployment plan for running Blockscout against a Lumera node. Lumera's JSON-RPC layer is pre-configured for explorer integration: built-in indexer, EVM tracing via `debug_traceTransaction`, configurable CORS, WebSocket for real-time block streaming on port 8546, per-IP rate limiting on proxy port 8547, and the chain ID `76857769`. The guide lists the explicit `app.toml` settings, deployment steps, and operational considerations (separate sentry node for the explorer, archive-mode requirement for full historical state).

## Cross-cutting facts worth knowing before you start

- **EVM chain ID is `76857769`** (set in `config/evm.go`). It's used for EIP-155 replay protection — every Ethereum-style transaction signs against this chain ID, so dev tools must be configured to match before signed transactions will be accepted.
- **Two coexisting denominations.** Cosmos-side accounts hold `ulume` (6 decimals); EVM-side balances are exposed as `alume` (18 decimals) via the `x/precisebank` module. The relation is `EVMBalance(a) = I(a) * 10^12 + F(a)` — integer `ulume` balance scaled up plus a sub-`ulume` fractional remainder. Tooling that displays balances should pick the side that matches the user's mental model.
- **`eth_secp256k1` keys, BIP44 coin type 60.** MetaMask, Hardhat, Foundry, ethers, web3, and any standard EVM tooling Just Works — Lumera does not require provider-specific shims. The same address is reachable on the Cosmos side as a `lumera1...` bech32 (the EVM hex address is the bottom 20 bytes of the keccak-256 hash of the uncompressed pubkey, identical to Ethereum's derivation).
- **Mainnet rejects `debug` / `personal` / `admin` JSON-RPC namespaces.** A startup guard in `cmd/lumera/cmd/jsonrpc_policy.go` refuses to start the node if `app.toml`'s `[json-rpc] api` includes any of these on a mainnet chain ID. Testnet and devnet allow them; plan tracing-dependent workflows accordingly.
