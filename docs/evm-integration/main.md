# Lumera Cosmos EVM Integration

## Summary

Lumera now has first-class Cosmos EVM integration across runtime wiring, ante, mempool, JSON-RPC/indexer, key management, static precompiles, IBC ERC20 middleware, denom metadata, and upgrade/migration paths.

Lumera's EVM integration is designed as a deeply integrated, production-ready layer rather than a minimal add-on. Where other chains shipped bare EVM support and back-filled operational controls over months or years, Lumera launches with production-grade tracing, rate limiting, governance-controlled IBC ERC20 policy, a deadlock-free app-side mempool, OpenRPC discovery, the industry's first bidirectional CosmWasm↔EVM cross-runtime bridge, and purpose-built custom precompiles for its native modules — a combination no other Cosmos EVM chain offers today. See the [Cross-Chain EVM Integration Comparison](architecture/comparison.md) for a detailed breakdown.

## Documentation

### Architecture and implementation

- [breaking-changes.md](architecture/breaking-changes.md) — Breaking changes and operational implications: coin type, key type, gas decimals, fee market, token representation
- [app-changes.md](architecture/app-changes.md) — All app-level code changes: module wiring, ante handlers, mempool, JSON-RPC, keyring, precompiles, IBC, fee market, upgrades, OpenRPC
- [integration-semantics.md](architecture/integration-semantics.md) — Detailed behavioral semantics: modules, coin type change, key types, dual addresses, decimal bridging, EIP-1559, token representation, IBC interplay, rollout guidance
- [gap-analysis.md](architecture/gap-analysis.md) — Design document vs implementation status matrix and intentional constraints
- [comparison.md](architecture/comparison.md) — Cross-chain comparison with Evmos, Kava, Cronos, Canto, and Injective
- [roadmap.md](architecture/roadmap.md) — EVM integration roadmap and planning
- [rollout.md](architecture/rollout.md) — Staged rollout plan for `v1.20.0`, including devnet, testnet, mainnet, and account migration communications

### Precompiles

- [precompiles/precompiles.md](precompiles/precompiles.md) — All EVM precompiles: 8 standard + 3 custom Lumera precompiles (Action `0x0901`, Supernode `0x0902`, Wasm `0x0903`)

### Account migration

- [evmigration/main.md](evmigration/main.md) — Legacy account migration (`x/evmigration`): module architecture, user guide, migration portal UI, devnet tests

### Testing and operations

- [tests.md](testing/tests.md) — Full test inventory (unit, integration, devnet), coverage assessment, gaps, and next steps
- [bugs.md](testing/bugs.md) — Bugs found and fixed during EVM integration
- [security-audit.md](testing/security-audit.md) — Security audit findings and recommendations

### User Guides

- [user-guides/main.md](user-guides/main.md) — Operator and end-user guide hub: migration (Portal + Keplr, shell scripts, raw CLI, multisig), validator and supernode operator walkthroughs, node `app.toml` reference, and the mainnet parameter tuning review

### Developer Guides

- [guides/main.md](guides/main.md) — Developer guide hub: OpenRPC discovery and the JSON-RPC catalog, smart contracts via Remix + MetaMask, and Blockscout block explorer integration

## Operational Outcomes

After this integration:

- Lumera can execute Ethereum transactions and EVM bytecode natively through Cosmos EVM (`x/vm`).
- JSON-RPC/WebSocket/indexer are enabled by default, so standard Ethereum client flows work without extra node flags.
- Wallet UX is improved:
  - MetaMask-compatible account/key model (`eth_secp256k1`, BIP44 coin type 60).
  - Ethereum-style address/key expectations align with common EVM tooling.
- Smart contract developer UX is unlocked:
  - Solidity/Vyper contracts can be deployed and interacted with using standard EVM JSON-RPC methods.
  - Common toolchains (for example Hardhat/Foundry/Web3/Ethers libraries) can target Lumera via RPC.
- EIP-1559 dynamic base fee is active with Lumera defaults (base fee 0.0025, min 0.0005, denominator 16), enabling predictable fee market behavior with spam protection.
- Precisebank enables 18-decimal extended-denom accounting while preserving Cosmos bank compatibility.
- Static precompiles expose Cosmos functionality (bank/staking/distribution/gov/bech32/p256/slashing/ics20) to EVM contracts.
- IBC ERC20 middleware wiring enables ERC20-aware ICS20 receive/mapping flows for cross-chain token paths.
- Upgrade path includes EVM store migrations (v1.20.0) with adaptive store-manager support for safer network evolution.
- OpenRPC method catalog is available from the running node over:
  - JSON-RPC: `rpc_discover`
  - HTTP API server: `/openrpc.json` (CORS-enabled for browser tooling)

## Architecture Strengths

### Circular dependency resolution

The EVM keeper graph has unavoidable cycles (EVMKeeper needs Erc20Keeper for precompiles; Erc20Keeper needs EVMKeeper for contract calls). The wiring in `app/evm.go` resolves this cleanly via pointer-based forward references:

```go
EVMKeeper  = NewKeeper(..., &app.Erc20Keeper)   // populated below
Erc20Keeper = NewKeeper(..., app.EVMKeeper, &app.EVMTransferKeeper)
```

Both keepers are usable at runtime without `nil`-pointer races because the IBC transfer keeper (the last link in the cycle) is resolved before any block execution begins.

### Dual-route ante handler with explicit extension routing

Transaction routing is deterministic and non-ambiguous. The ante handler in `app/evm/ante.go` inspects `ExtensionOptions[0].TypeUrl` to choose between three paths:

| Extension                       | Route               | Decorators                                     |
| ------------------------------- | ------------------- | ---------------------------------------------- |
| `ExtensionOptionsEthereumTx`  | EVM path            | EVMMonoDecorator + pending tx listener         |
| `ExtensionOptionDynamicFeeTx` | Cosmos path         | Full Lumera + EVM-aware Cosmos decorator chain |
| _(none)_                      | Default Cosmos path | Same Cosmos chain, DynamicFeeChecker disabled  |

This prevents Ethereum messages from leaking into the Cosmos validation path (or vice versa) and ensures fee semantics match the transaction type.

### Module ordering correctness

The genesis/begin/end block ordering in `app/app_config.go` satisfies all dependency constraints:

- **EVM initializes first in genesis** (before erc20, precisebank, genutil) so coin info is available for all downstream consumers.
- **FeeMarket EndBlocker runs last** to capture full block gas usage for accurate base fee calculation. (evmigration runs just before it; its EndBlocker is a no-op.)
- **EVM PreBlocker** runs after upgrade and auth to ensure coin info is populated before early RPC queries hit the node.

### Production guardrails

Build-tag protection (`//go:build !test` in `app/evm/defaults_prod.go`) prevents test-only global state resets from compiling into production binaries. The `SetKeeperDefaults` function initializes EVM coin info on app startup to prevent RPC panics before genesis runs. Both guardrails have dedicated unit tests.

### Async broadcast queue prevents mempool deadlock

The EVM txpool's `runReorg` calls `BroadcastTxFn` synchronously while holding the mempool mutex (`m.mtx`). If `BroadcastTxFn` submits a tx via CometBFT's local ABCI client, `CheckTx` calls back into `Insert()` on the same mempool — which tries to acquire `m.mtx` again, deadlocking the chain.

The `evmTxBroadcastDispatcher` in `app/evm_broadcast.go` breaks this cycle:

1. `BroadcastTxFn` (called inside `runReorg`) enqueues promoted txs into a bounded channel and returns immediately — never blocking `Insert()`.
2. A single background worker goroutine drains the channel and submits txs via `BroadcastTxSync` after the mutex is released.
3. Tx hashes are tracked in a `pending` set for deduplication; hashes are released after processing or on queue-full/error paths.

The `RegisterTxService` override in `app/evm_runtime.go` ensures the broadcast worker uses the local CometBFT client (not the stale HTTP client that `SetClientCtx` provides before CometBFT starts). The re-entry hazard is validated by `TestEVMMempoolReentrantInsertBlocks`, and the full promotion-to-inclusion path is validated by the `NonceGapPromotionAfterGapFilled` integration test.

### Precompile address protection

Bank send restrictions block token sends to all 8 precompile addresses plus module accounts. This prevents accidental token loss to system addresses that cannot sign outbound transactions.

### IBC-EVM middleware layering

The transfer stack is properly layered for both IBC v1 and v2:

```text
v1: EVMTransferKeeper -> ERC20IBCMiddleware -> CallbacksMiddleware -> PFM
v2: TransferV2Module -> CallbacksV2Middleware -> ERC20IBCMiddlewareV2
```

The `EVMTransferKeeper` maintains an `ICS4Wrapper` back-reference for callback chains, ensuring packet acknowledgments propagate correctly through the full middleware stack.

### OpenRPC build-time synchronization

The OpenRPC spec is regenerated on every `make build` via the `tools/openrpcgen` tool, which uses Go reflection and AST parsing to introspect the actual RPC implementation types. The generator expands struct parameters into full JSON Schema `properties` with per-field types and validation patterns for well-known Ethereum types (`common.Address`, `hexutil.Big`, etc.). The spec version is derived from `go.mod` at build time via `runtime/debug.ReadBuildInfo()`. The generated spec is gzip-compressed and `//go:embed`-ded into the binary (315 KB → 20 KB), then decompressed once at startup. This eliminates stale-spec drift: the running node always serves a spec that matches its compiled RPC surface.

### 18-decimal precision bridge design

The `x/precisebank` module preserves Cosmos bank invariants (6-decimal `ulume`) while exposing 18-decimal `alume` to EVM. The arithmetic model (`EVMBalance(a) = I(a) * 10^12 + F(a)`) keeps canonical supply accounting in `x/bank` and tracks only sub-`ulume` fractional remainders in precisebank state. This avoids dual-supply risks and keeps the Cosmos-side accounting simple.

---

### Core implementation quality

The EVM core wiring audit found **zero critical issues** across all app-level EVM files:

- **Correctness**: Keeper wiring, circular dependency resolution, dual-route ante handler, module ordering, store upgrades — all verified correct.
- **Thread safety**: No race conditions. Broadcast queue properly synchronized. Keeper access serialized via SDK context.
- **Error handling**: Comprehensive — no silent failures found.
- **Code quality**: Well-documented, follows cosmos/evm best practices, includes build-tag guards for test isolation.
