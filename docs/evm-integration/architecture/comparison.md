# Cross-Chain EVM Integration Comparison

Comparison of Lumera's Cosmos EVM integration against other Cosmos SDK chains that added EVM support: Evmos, Kava, Cronos, Canto, and Injective.

Lumera is ahead in several integration-quality dimensions:

- **Operational readiness built in**: EVM tracing is runtime-configurable (`app.toml` / `--evm.tracer`), and JSON-RPC per-IP rate limiting is already implemented at the app layer.
- **Safer cross-chain ERC20 registration**: IBC voucher → ERC20 auto-registration is governed by a governance-controlled policy (`all` / `allowlist` / `none`) with provenance-bound base-denom allowlisting (full IBC trace verification per base denom).
- **Mempool correctness hardening**: async broadcast queue prevents a known re-entry deadlock pattern in app-side EVM mempool integration.
- **Discovery + compatibility**: OpenRPC generation/serving and build-time spec sync reduce client integration friction and stale-doc drift.
- **Migration completeness**: dedicated `x/evmigration` module supports coin-type migration with dual-signature verification and multi-module atomic migration.
- **Custom module precompiles**: Purpose-built precompiles for Action (`0x0901`) and Supernode (`0x0902`) modules give Solidity contracts native access to Lumera-specific functionality.

### Component matrix

| Component                             | Lumera                                                               | Evmos                          | Kava                         | Cronos                 | Canto                  | Injective                |
| ------------------------------------- | -------------------------------------------------------------------- | ------------------------------ | ---------------------------- | ---------------------- | ---------------------- | ------------------------ |
| EVM execution module                  | x/vm (cosmos/evm v0.6.0)                                             | x/evm (Ethermint)              | x/evm (Ethermint fork)       | x/evm (Ethermint)      | x/evm (Ethermint)      | Custom EVM               |
| EIP-1559 fee market                   | x/feemarket                                                          | x/feemarket                    | x/feemarket                  | x/feemarket            | x/feemarket (zero CSR) | Custom                   |
| Token bridge/conversion               | x/erc20 (STRv2) + x/precisebank                                      | x/erc20 (STRv2)                | x/evmutil (conversion pairs) | x/cronos (auto-deploy) | x/erc20                | Native dual-denom        |
| 6-to-18 decimal bridge                | x/precisebank                                                        | Built into erc20               | x/evmutil                    | Built into x/cronos    | N/A (18-dec native)    | N/A (18-dec native)      |
| Static precompiles                    | 11 (8 standard + 3 custom)                                           | 10+                            | 8+                           | 8+                     | CSR precompile         | Custom exchange          |
| Custom module precompiles             | Yes (Action `0x0901`, Supernode `0x0902`, Wasm `0x0903`)              | Yes (staking/dist/IBC/vesting) | Yes (swap/earn)              | Partial                | CSR                    | Yes (exchange/orderbook) |
| IBC ERC20 middleware                  | Yes (v1 + v2)                                                        | Yes (STRv2)                    | No (manual bridge)           | Yes (auto-deploy)      | No                     | Limited                  |
| IBC voucher ERC20 registration policy | **Yes** (governance-controlled `all`/`allowlist`/`none`) | Not standard                   | Not standard                 | Not standard           | Not standard           | Not standard             |
| EVM-aware mempool                     | Yes (experimental + async broadcast)                                 | Experimental                   | No (standard CometBFT)       | No (standard CometBFT) | No                     | Custom orderbook         |
| EVM tracing (debug API)               | Yes (configurable via app.toml)                                      | Yes                            | Limited                      | Yes                    | Limited                | Yes                      |
| JSON-RPC rate limiting                | **Done** (per-IP token bucket proxy)                           | Yes                            | Yes                          | Yes                    | Yes                    | Yes                      |
| CORS configuration                    | **Done** (reuses `ws-origins` for OpenRPC + WS)              | Yes                            | Yes                          | Yes                    | Yes                    | Yes                      |
| EVM governance proposals              | Via gov authority on keepers                                         | Dedicated proposal types       | Yes                          | Partial                | Limited                | Yes                      |
| CosmWasm coexistence                  | **Yes** — wasmd v0.61.6 + bidirectional cross-runtime bridge        | No                             | No                           | No                     | No                     | No                       |
| OpenRPC discovery                     | Yes (unique)                                                         | No                             | No                           | No                     | No                     | No                       |
| Async broadcast queue                 | Yes (unique deadlock fix)                                            | No                             | No                           | No                     | No                     | No                       |

### What Lumera has that other chains don't

1. **CosmWasm ↔ EVM cross-runtime bridge** — Lumera is the only chain in this comparison running both CosmWasm smart contracts and the EVM simultaneously, and the only one with a bidirectional bridge between the two runtimes. Solidity contracts can execute and query CosmWasm contracts via the Wasm precompile (`0x0903`), and CosmWasm contracts can call and query EVM contracts via custom message/query handlers. No other Cosmos EVM chain has this capability — Lumera built the industry's first cross-runtime bridge with no external precedent. See [precompiles/wasm-precompile.md](precompiles/wasm-precompile.md).
2. **OpenRPC discovery** — Full OpenRPC spec generation (`tools/openrpcgen`), embedded spec in the binary (`app/openrpc/openrpc.json`), HTTP endpoint at`/openrpc.json`, and runtime`rpc_discover` JSON-RPC method. No other Cosmos EVM chain provides machine-readable API discovery.
3. **Async broadcast queue (mempool deadlock fix)** — The`evmTxBroadcastDispatcher` in`app/evm_broadcast.go` decouples txpool nonce-gap promotion from CometBFT's`CheckTx` call, preventing a mutex re-entry deadlock that affects the cosmos/evm experimental mempool. Other chains either don't use the app-side EVM mempool at all (Kava, Cronos, Canto) or haven't publicly addressed this deadlock (Evmos).
4. **Min gas price floor** —`FeeMarketMinGasPrice = 0.0005 ulume/gas` prevents base fee decay to zero during low-activity periods. Evmos experienced zero-base-fee spam attacks because it lacked this floor. Lumera learned from that and ships with the floor from day one.
5. **IBC v2 ERC20 middleware** — ERC20 token registration middleware is wired on both IBC v1 and v2 transfer stacks. Most chains only have v1 support.
6. **Governance-controlled IBC voucher ERC20 registration policy** — Lumera ships a first-class policy layer (`all` /`allowlist` default /`none`) controlled via governance message (`MsgSetRegistrationPolicy`) with exact `ibc/` and provenance-bound base-denom allowlisting (full denom trace verification per base denom).
7. **Account migration module** — Purpose-built`x/evmigration` for the coin-type-118-to-60 transition with dual-signature verification. No other chain has published a comparable migration mechanism. Kava had a similar challenge but handled it differently (via`x/evmutil` conversion pairs rather than account migration).
8. **Production-focused operator controls from day one** — tracing is runtime-configurable and JSON-RPC rate limiting is integrated at app level, reducing operational drift between dev/test and production.

### What other chains have that Lumera is missing

1. **Custom module precompiles** — Evmos ships staking/distribution/IBC/vesting/gov precompiles. Kava has swap/earn. Lumera now has 8 standard precompiles plus 3 Lumera-specific precompiles (Action at `0x0901`, Supernode at `0x0902`, Wasm at `0x0903`), exceeding the custom precompile coverage of all comparable chains at launch.
2. **EVM governance proposal types** — Evmos has dedicated governance proposals for toggling precompiles and adjusting EVM parameters. Lumera can achieve the same through standard`MsgUpdateParams` with gov authority on all EVM keepers, but lacks dedicated proposal types or documented governance workflows for EVM-specific changes.
3. **External block explorer** — All comparable chains have Blockscout, Etherscan-compatible, or custom block explorers at mainnet. Lumera does not yet have one.
4. **Vesting precompile** — Evmos provides a vesting precompile. Lumera intentionally excludes it because the upstream cosmos/evm v0.6.0 default registry doesn't provide it.

### Gas configuration comparison

| Parameter                   | Lumera                        | Evmos                 | Kava        | Cronos     |
| --------------------------- | ----------------------------- | --------------------- | ----------- | ---------- |
| Default base fee            | 0.0025 ulume (2.5 gwei equiv) | ~10 gwei              | ~0.25 ukava | Variable   |
| Min gas price floor         | 0.0005 ulume                  | 0 (no floor)          | N/A         | N/A        |
| Base fee change denominator | 16 (~6.25% adjustment)        | 8 (~12.5%)            | 8           | 8          |
| Consensus max gas           | 25,000,000                    | 30,000,000-40,000,000 | 25,000,000  | 25,000,000 |

Lumera's fee market choices are well-tuned. The gentler change denominator (16 vs 8) reduces fee volatility. The min gas price floor prevents the zero-base-fee problem that Evmos experienced. The 25M block gas limit matches Kava and Cronos and is upgradeable via governance.

### Token conversion approach comparison

Three primary approaches exist across Cosmos EVM chains:

| Approach                                         | Used by               | How it works                                                                                                                                               |
| ------------------------------------------------ | --------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **STRv2** (Single Token Representation v2) | Evmos, Lumera         | One canonical supply in bank module. ERC20 interface is a "view" over bank balances — no mint/burn conversion needed. Balances always consistent.         |
| **Conversion pairs**                       | Kava (`x/evmutil`)  | Explicit conversion pairs. Users must actively bridge between Cosmos-native and EVM-native representations. Higher UX friction but simpler implementation. |
| **Auto-deploy**                            | Cronos (`x/cronos`) | Automatically deploys an ERC20 contract for each IBC token received. More flexible but introduces contract risk and gas overhead.                          |

Lumera uses STRv2 via `x/erc20` from cosmos/evm, supplemented by `x/precisebank` for 6-to-18 decimal bridging. This is the most seamless approach for end users because bank balances and ERC20 balances are always in sync without manual conversion.

### Wallet compatibility

All chains in the comparison support MetaMask and Ethereum-compatible wallets via:

| Requirement                                 | Lumera status    |
| ------------------------------------------- | ---------------- |
| EIP-155 chain ID                            | 76857769         |
| BIP44 coin type 60                          | Yes (default)    |
| eth_secp256k1 key type                      | Yes (default)    |
| JSON-RPC `eth_*` namespace                | Yes (cosmos/evm) |
| EIP-1559 type-2 transactions                | Yes (feemarket)  |
| EIP-712 typed data signing                  | Yes (cosmos/evm) |
| eth_chainId / eth_gasPrice / eth_feeHistory | Yes              |

Lumera's coin type 60 and `eth_secp256k1` default key type mean MetaMask-generated keys work natively. The chain ID 76857769 needs to be added to MetaMask as a custom network.

### Indexer and data availability

| Feature                           | Lumera                     | Evmos             | Kava              | Cronos               |
| --------------------------------- | -------------------------- | ----------------- | ----------------- | -------------------- |
| Tx receipts                       | Built-in (cosmos/evm)      | Built-in          | Built-in          | Built-in + Etherscan |
| Log indexing                      | Built-in (tested)          | Built-in          | Built-in          | Built-in + external  |
| Tx hash lookup                    | Built-in (tested)          | Built-in          | Built-in          | Built-in             |
| Receipt persistence               | Built-in (tested)          | Built-in          | Built-in          | Built-in             |
| Historical state queries          | Pruning-dependent (tested) | Pruning-dependent | Pruning-dependent | Archive nodes        |
| Indexer disable mode              | Yes (tested)               | Yes               | No                | No                   |
| External indexer (TheGraph, etc.) | Not yet                    | Community         | Community         | Official (Cronoscan) |

Lumera's integration test coverage for indexer functionality (`logs_indexer_test.go`, `txhash_persistence_test.go`, `receipt_persistence_test.go`, `indexer_disabled_test.go`, `query_historical_test.go`) is more thorough than most chains had at equivalent maturity.

---

### Core implementation quality

The EVM core wiring audit found **zero critical issues** across all app-level EVM files:

- **Correctness**: Keeper wiring, circular dependency resolution, dual-route ante handler, module ordering, store upgrades — all verified correct.
- **Thread safety**: No race conditions. Broadcast queue properly synchronized. Keeper access serialized via SDK context.
- **Error handling**: Comprehensive — no silent failures found.
- **Code quality**: Well-documented, follows cosmos/evm best practices, includes build-tag guards for test isolation.
