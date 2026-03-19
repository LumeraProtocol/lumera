# Lumera EVM Integration Roadmap

**Last updated**: 2026-03-19
**Cosmos EVM version**: v0.6.0
**Target**: Mainnet-ready EVM integration

---

## Phase 1: Core EVM Runtime (DONE)

Everything needed to execute Ethereum transactions on the Lumera chain.

| | Item | Files / Notes |
|---|------|---------------|
| [x] | EVM execution module (`x/vm`) wiring | `app/evm.go` — keeper, store keys, transient keys, module registration |
| [x] | Fee market module (`x/feemarket`) wiring | `app/evm.go` — EIP-1559 dynamic base fee |
| [x] | Precisebank module (`x/precisebank`) wiring | `app/evm.go` — 6-decimal `ulume` <-> 18-decimal `alume` bridge |
| [x] | ERC20 module (`x/erc20`) wiring | `app/evm.go` — STRv2 token pair registration |
| [x] | EVM chain ID configuration | `config/evm.go` — `EVMChainID = 76857769` |
| [x] | Denom constants (`ulume`/`alume`/`lume`) | `config/evm.go`, `config/config.go` |
| [x] | Bank denom metadata | `config/bank_metadata.go` |
| [x] | Coin type 60 / BIP44 HD path | `config/bip44.go` |
| [x] | `eth_secp256k1` default key type | `cmd/lumera/cmd/root.go` |
| [x] | EVM genesis defaults (denom, precompiles, feemarket) | `app/evm/genesis.go` |
| [x] | Depinject signer wiring (`MsgEthereumTx`) | `app/evm/modules.go` — `ProvideCustomGetSigners` |
| [x] | Codec registration (`eth_secp256k1` interfaces) | `config/codec.go` |
| [x] | EVM module ordering (genesis/begin/end/pre-block) | `app/app_config.go` |
| [x] | Module account permissions (vm, erc20, feemarket, precisebank) | `app/app_config.go` |
| [x] | Circular dependency resolution (EVMKeeper <-> Erc20Keeper) | `app/evm.go` — pointer-based forward references |
| [x] | Default keeper coin info initialization | `app/evm/config.go` — `SetKeeperDefaults` for safe early RPC |
| [x] | Production guard (test-only reset behind build tag) | `app/evm/prod_guard_test.go` |

---

## Phase 2: Ante Handler & Transaction Routing (DONE)

Dual-route ante pipeline for Cosmos and Ethereum transactions.

| | Item | Files / Notes |
|---|------|---------------|
| [x] | Dual-route ante handler (EVM vs Cosmos path) | `app/evm/ante.go` |
| [x] | EVM path: `NewEVMMonoDecorator` | `app/evm/ante.go` — signature, nonce, fee, gas |
| [x] | Cosmos path: standard SDK + Lumera decorators | `app/evm/ante.go` |
| [x] | `RejectMessagesDecorator` (block MsgEthereumTx in Cosmos path) | `app/evm/ante.go` |
| [x] | `AuthzLimiterDecorator` (block EVM msgs in authz) | `app/evm/ante.go` |
| [x] | `MinGasPriceDecorator` (feemarket-aware) | `app/evm/ante.go` |
| [x] | `GasWantedDecorator` (gas accounting) | `app/evm/ante.go` |
| [x] | Genesis skip decorator (gentx fee bypass at height 0) | `app/evm/ante.go` — fixes Bug #3 |
| [x] | Pending tx listener decorator | `app/evm/ante.go` |
| [x] | `DelayedClaimFeeDecorator` (claim tx fee waiver) | `ante/delayed_claim_fee_decorator.go` |
| [x] | `EVMigrationFeeDecorator` (migration tx fee waiver) | `ante/evmigration_fee_decorator.go` |
| [x] | `EVMigrationValidateBasicDecorator` (unsigned migration txs) | `ante/evmigration_validate_basic_decorator.go` |
| [x] | Migration-only reduced Cosmos ante subchain (single branch) | `app/evm/ante.go` |

---

## Phase 3: Feemarket Configuration (DONE)

EIP-1559 fee market with Lumera-specific tuning.

| | Item | Files / Notes |
|---|------|---------------|
| [x] | Default base fee: 0.0025 ulume/gas | `config/evm.go` |
| [x] | Min gas price floor: 0.0005 ulume/gas | `config/evm.go` — prevents zero-fee spam |
| [x] | Base fee change denominator: 16 (~6.25%/block) | `config/evm.go` — gentler than upstream 8 |
| [x] | Consensus max gas: 25,000,000 | `config/evm.go` |
| [x] | Dynamic base fee enabled by default | `app/evm/genesis.go` |
| [x] | Fee distribution via standard SDK path | Full effective gas price -> fee collector -> x/distribution |
| [ ] | Raise block gas limit via governance | DEFERRED — 25M is adequate for launch; increase for heavy DeFi if needed |

---

## Phase 4: Mempool & Broadcast Infrastructure (DONE)

EVM-aware app-side mempool with deadlock prevention.

| | Item | Files / Notes |
|---|------|---------------|
| [x] | `ExperimentalEVMMempool` integration | `app/evm_mempool.go` |
| [x] | EVM-aware `PrepareProposal` signer extraction | `app/evm_mempool.go` |
| [x] | Async broadcast dispatcher (deadlock fix) | `app/evm_broadcast.go` — Bug #5 fix |
| [x] | Broadcast worker `RegisterTxService` override | `app/evm_runtime.go` — local CometBFT client |
| [x] | `Close()` override for graceful shutdown | `app/evm_runtime.go` |
| [x] | `broadcast-debug` app.toml toggle | `cmd/lumera/cmd/config.go` |
| [x] | Default `max_txs=5000` | App config defaults |
| [x] | Mempool eviction / capacity pressure testing | `tests/integration/evm/mempool/capacity_pressure_test.go` |
| [ ] | Mempool metrics / observability | TODO — Expose mempool size, pending count, rejection rate metrics |

---

## Phase 5: JSON-RPC & Indexer (DONE)

Ethereum JSON-RPC server and transaction indexing.

| | Item | Files / Notes |
|---|------|---------------|
| [x] | JSON-RPC server enabled by default | `cmd/lumera/cmd/config.go` |
| [x] | EVM indexer enabled by default | `cmd/lumera/cmd/config.go` |
| [x] | EVM server command wiring | `cmd/lumera/cmd/root.go`, `commands.go` |
| [x] | Per-IP JSON-RPC rate limiting | `app/evm_jsonrpc_ratelimit.go` — token bucket proxy |
| [x] | EVM tracing (debug API) configurable via app.toml | `app.toml` `[evm] tracer` field |
| [x] | Production CORS origin lockdown | `app/openrpc/http.go` — reuses `[json-rpc] ws-origins` |
| [x] | JSON-RPC namespace exposure lockdown per env | `cmd/lumera/cmd/jsonrpc_policy.go` — reject `debug`, `personal`, `admin` on mainnet |
| [x] | Batch JSON-RPC request support testing | `tests/integration/evm/jsonrpc/batch_rpc_test.go` |
| [x] | WebSocket subscription testing | `tests/integration/evm/mempool/ws_subscription_test.go` |

---

## Phase 6: Static Precompiles (DONE)

Standard precompile set for EVM-to-Cosmos access.

| | Item | Files / Notes |
|---|------|---------------|
| [x] | Bank precompile | `app/evm/precompiles.go` |
| [x] | Staking precompile | `app/evm/precompiles.go` |
| [x] | Distribution precompile | `app/evm/precompiles.go` |
| [x] | Gov precompile | `app/evm/precompiles.go` |
| [x] | ICS20 precompile | `app/evm/precompiles.go` — Bug #6 fixed (store key ordering) |
| [x] | Bech32 precompile | `app/evm/precompiles.go` |
| [x] | P256 precompile | `app/evm/precompiles.go` |
| [x] | Slashing precompile | `app/evm/precompiles.go` |
| [x] | Blocked-address protections | Bank send restriction blocks sends to precompile addresses |
| [ ] | Vesting precompile | DEFERRED — Not provided by upstream cosmos/evm v0.6.0 |
| [x] | Precompile gas metering benchmarks | `tests/integration/evm/precompiles/gas_metering_test.go` |

---

## Phase 7: IBC + ERC20 Middleware (DONE)

Cross-chain token registration and transfer.

| | Item | Files / Notes |
|---|------|---------------|
| [x] | ERC20 IBC middleware — v1 transfer stack | `app/ibc.go` |
| [x] | ERC20 IBC middleware — v2 transfer stack | `app/ibc.go` |
| [x] | Governance-controlled ERC20 registration policy | `app/evm_erc20_policy.go` — `all`/`allowlist`(default)/`none` |
| [x] | `MsgSetRegistrationPolicy` governance message | `app/evm_erc20_policy_msg.go` |
| [x] | Base denom allowlist (uatom, uosmo, uusdc) | `app/evm_erc20_policy.go` |
| [x] | IBC store keys synced to EVM snapshot | `app/evm.go` — `syncEVMStoreKeys()`, Bug #6 fix |
| [x] | EVMTransferKeeper ICS4Wrapper back-reference | `app/ibc.go` |
| [ ] | ICS20 precompile transfer tx test | TODO — Pending IBC channel config in integration test setup |

---

## Phase 8: OpenRPC Discovery (DONE)

Machine-readable API spec (unique among Cosmos EVM chains).

| | Item | Files / Notes |
|---|------|---------------|
| [x] | OpenRPC spec generation tool | `tools/openrpcgen/main.go` |
| [x] | Gzip-compressed embedded spec (`//go:embed`) | `app/openrpc/spec.go` — 315 KB → 20 KB (93% reduction) |
| [x] | `rpc_discover` JSON-RPC method | `app/openrpc/register.go` |
| [x] | `/openrpc.json` HTTP endpoint (GET + POST proxy) | `app/openrpc/http.go` — POST proxies to JSON-RPC with `rpc.discover` → `rpc_discover` rewrite |
| [x] | CORS support for OpenRPC endpoint | `app/openrpc/http.go` |
| [x] | Build-time spec sync (`make openrpc`) | `Makefile` — generates `docs/openrpc.json`, compresses to `app/openrpc/openrpc.json.gz` |
| [x] | Struct parameter expansion in generated schema | `tools/openrpcgen/main.go` — JSON Schema `properties` with per-field types |
| [x] | Ethereum type overrides (Address, Hash, hexutil, etc.) | `tools/openrpcgen/main.go` — correct string schemas with validation patterns |
| [x] | Dynamic version from `go.mod` | `tools/openrpcgen/main.go` — `runtime/debug.ReadBuildInfo()` |
| [x] | Dynamic `servers[0].url` rewriting | `app/openrpc/http.go` — rewrites based on configured JSON-RPC address |

---

## Phase 9: Store Upgrades & Migration (DONE)

Chain upgrade handling for EVM module stores.

| | Item | Files / Notes |
|---|------|---------------|
| [x] | v1.12.0 store upgrades (feemarket, precisebank, vm, erc20, evmigration) | `app/upgrades/v1_12_0/upgrade.go` |
| [x] | Adaptive store upgrade manager | `app/upgrades/store_upgrade_manager.go` |
| [x] | EVM keeper refs in upgrade params | `app/upgrades/params/params.go` |
| [x] | ERC20 param finalization after skipped `InitGenesis` | `app/upgrades/v1_12_0/upgrade.go` |
| [x] | Chain upgrade EVM state preservation test | `tests/integration/evm/contracts/upgrade_preservation_test.go` |
| [x] | `app.toml` config migration for pre-EVM nodes (Bug #19) | `cmd/lumera/cmd/config_migrate.go` — auto-adds [evm], [json-rpc], [tls], [lumera.*] on startup |

---

## Phase 10: Legacy Account Migration — `x/evmigration` (DONE)

Coin-type-118-to-60 account migration with dual-signature verification.

| | Item | Files / Notes |
|---|------|---------------|
| [x] | Proto definitions | `proto/lumera/evmigration/` |
| [x] | Module skeleton + depinject | `x/evmigration/module/` |
| [x] | Dual-signature verification | `x/evmigration/keeper/verify.go` |
| [x] | `MsgClaimLegacyAccount` handler | `x/evmigration/keeper/msg_server_claim_legacy.go` |
| [x] | `MsgMigrateValidator` handler | `x/evmigration/keeper/msg_server_migrate_validator.go` |
| [x] | Auth migration (vesting-aware) | `x/evmigration/keeper/migrate_auth.go` |
| [x] | Bank balance transfer | `x/evmigration/keeper/migrate_bank.go` |
| [x] | Staking re-keying (delegations, unbonding, redelegations) | `x/evmigration/keeper/migrate_staking.go` |
| [x] | Distribution reward withdrawal | `x/evmigration/keeper/migrate_distribution.go` |
| [x] | Authz grant re-keying | `x/evmigration/keeper/migrate_authz.go` |
| [x] | Feegrant allowance re-keying | `x/evmigration/keeper/migrate_feegrant.go` |
| [x] | Supernode migration | `x/evmigration/keeper/migrate_supernode.go` |
| [x] | Action migration | `x/evmigration/keeper/migrate_action.go` |
| [x] | Claim record migration | `x/evmigration/keeper/migrate_claim.go` |
| [x] | Validator record re-keying | `x/evmigration/keeper/migrate_validator.go` |
| [x] | Fee waiving ante decorators | `ante/evmigration_fee_decorator.go`, `ante/evmigration_validate_basic_decorator.go` |
| [x] | Queries (record, records, stats, estimate, legacy, migrated, params) | `x/evmigration/keeper/query.go` |
| [x] | Genesis export/import | `x/evmigration/keeper/genesis.go` |
| [x] | CLI (`claim-legacy-account`, `migrate-validator`) | `x/evmigration/client/cli/tx.go` |
| [x] | Custom signers for unsigned tx flow | `x/evmigration/module/signers.go` |
| [x] | Params (enable, end_time, rate limit, max_validator_delegations) | `x/evmigration/types/params.go` |

---

## Phase 11: Testing (DONE)

Comprehensive test coverage across all layers.

### Unit Tests (~244)

| | Area | Tests |
|---|------|-------|
| [x] | App wiring / genesis / precompiles / mempool / broadcast | 37 |
| [x] | EVM ante decorators | 28 |
| [x] | EVM module/config guard | 6 |
| [x] | Fee market | 9 |
| [x] | Precisebank | 39 |
| [x] | OpenRPC / generator | 15 |
| [x] | ERC20 policy | 14 |
| [x] | EVMigration keeper | 107 |
| [x] | EVMigration types / module / CLI | 8 |
| [x] | Ante (evmigration fee, validate-basic) | 5 |

### Integration Tests (~115)

| | Area | Tests |
|---|------|-------|
| [x] | Ante | 3 |
| [x] | Contracts (deploy, interact, ERC20 flows, concurrency, upgrade preservation) | 11 |
| [x] | Fee market | 8 |
| [x] | IBC ERC20 | 7 |
| [x] | JSON-RPC / indexer (+ batch RPC) | 23 |
| [x] | Mempool (+ capacity pressure, WS subscriptions) | 10 |
| [x] | Precisebank | 6 |
| [x] | Precompiles (+ gas metering + action module) | 21 |
| [x] | VM queries / state | 12 |
| [x] | EVMigration | 14 |

### Devnet Tests

| | Area | Tests |
|---|------|-------|
| [x] | EVM basic / fee market / cross-peer | 8 |
| [x] | IBC | 6 |
| [x] | Ports / CORS | 2 |
| [x] | EVMigration tool (prepare, estimate, migrate, migrate-validator, migrate-all, verify) | 7 modes |

### Manual Validation

| | Area |
|---|------|
| [x] | Devnet EVMigration: full cycle on 5-validator devnet (prepare → migrate-all → verify) |
| [x] | MetaMask: balance query, send tx on fresh devnet chain (genesis EVM) |
| [x] | MetaMask: balance query, send tx after v1.11.0 → v1.12.0 upgrade (config migration verified) |
| [x] | Remix IDE: Counter contract deploy + interact via Injected Provider (devnet) |
| [x] | OpenRPC Playground: spec browsing + "Try It" method execution via POST proxy |

### Remaining Gaps

| | Gap | Priority |
|---|-----|----------|
| [ ] | Multi-validator EVM consensus scenarios | Low — expand devnet tests beyond single-validator assertions |

---

## Phase 12: Custom Lumera Module Precompiles (DONE)

EVM contracts calling Lumera-specific functionality (`0x0901`–`0x09XX`).

| | Item | Files / Notes |
|---|------|---------------|
| [x] | Action precompile (full — read + write) | `precompiles/action/` — address `0x0901` |
| [x] | Action precompile integration tests | `tests/integration/evm/precompiles/action_test.go` |
| [x] | Action precompile app wiring | `app/evm.go`, `app/evm/precompiles.go` |
| [x] | Supernode precompile (full — read + write) | `precompiles/supernode/` — address `0x0902` |
| [x] | Supernode precompile integration tests | `tests/integration/evm/precompiles/supernode_test.go` |
| [x] | Supernode precompile app wiring | `app/evm.go`, `app/evm/precompiles.go` |

---

## Phase 13: CosmWasm + EVM Interaction (TODO)

Lumera is the only Cosmos EVM chain also running CosmWasm. No external precedent exists.

| | Item | Priority |
|---|------|----------|
| [ ] | Design interaction model document | Medium — Bridge? Shared queries? Explicit isolation? |
| [ ] | Cross-runtime query paths (if designed) | Medium — CosmWasm -> EVM state queries or vice versa |
| [ ] | Cross-runtime message calls (if designed) | Low — Full bidirectional contract calls |
| [ ] | Integration tests for interaction model | Medium — After design is finalized |

---

## Phase 14: Production Hardening

Final operational readiness for mainnet.

| | Item | Priority | Notes |
|---|------|----------|-------|
| [ ] | Security audit of EVM integration | **Critical** | All comparable chains had dedicated EVM audits |
| [x] | CORS origin lockdown per environment | High | `app/openrpc/http.go` — reuses `[json-rpc] ws-origins` |
| [x] | JSON-RPC namespace exposure profiles | High | `cmd/lumera/cmd/jsonrpc_policy.go` — mainnet startup guard |
| [ ] | Fee market monitoring runbook | High | Base fee tracking, gas utilization, alerting thresholds |
| [x] | Node operator EVM configuration guide | High | `docs/evm-integration/node-evm-config-guide.md` |
| [ ] | Disaster recovery procedures (EVM state) | Medium | Recovery from corrupt EVM state, indexer rebuild |
| [ ] | Load testing / performance benchmarks | Medium | TPS under mixed Cosmos+EVM workload |
| [ ] | EVM governance proposal workflows | Low | Documented gov flows for precompile toggles, param changes |

---

## Phase 15: Ecosystem & Tooling

External infrastructure for production ecosystem.

| | Item | Priority | Notes |
|---|------|----------|-------|
| [ ] | External block explorer (Blockscout / Etherscan-compat) | High | All comparable chains have this at mainnet |
| [x] | MetaMask + Remix smart contract guide | Medium | `docs/evm-integration/remix-guide.md` |
| [x] | OpenRPC Playground guide | Medium | `docs/evm-integration/openrpc-playground.md` |
| [ ] | Hardhat/Foundry getting-started guide | Medium | Developer onboarding for Solidity devs |
| [ ] | External indexer (TheGraph / SubQuery) | Low | Community-facing data availability |
| [ ] | SDK / client library examples | Low | ethers.js / web3.js examples for Lumera |
| [ ] | Faucet for testnet (EVM-compatible) | Medium | MetaMask-friendly faucet |

---

## Summary Dashboard

| Phase | Description | Status | Completion |
|-------|-------------|--------|------------|
| 1 | Core EVM Runtime | DONE | 17/17 |
| 2 | Ante Handler & Tx Routing | DONE | 13/13 |
| 3 | Feemarket Configuration | DONE | 6/7 |
| 4 | Mempool & Broadcast | DONE | 8/9 |
| 5 | JSON-RPC & Indexer | DONE | 9/9 |
| 6 | Static Precompiles | DONE | 10/11 |
| 7 | IBC + ERC20 Middleware | DONE | 7/8 |
| 8 | OpenRPC Discovery | DONE | 10/10 |
| 9 | Store Upgrades & Migration | DONE | 6/6 |
| 10 | Legacy Account Migration | DONE | 21/21 |
| 11 | Testing | DONE | 37/37 |
| 12 | Custom Lumera Precompiles | DONE | 6/6 |
| 13 | CosmWasm + EVM Interaction | TODO | 0/4 |
| 14 | Production Hardening | IN PROGRESS | 3/8 |
| 15 | Ecosystem & Tooling | IN PROGRESS | 2/7 |
| | **TOTAL** | | **155/163** |

### Before Mainnet (Critical Path)

1. **Security audit** (Phase 14) — non-negotiable for any Cosmos EVM chain
2. **Block explorer** (Phase 15) — user-facing ecosystem requirement
3. **Monitoring runbook** (Phase 14) — operator readiness

### Near-Term Priorities

1. CosmWasm + EVM interaction design (Phase 13)
2. Multi-validator EVM consensus testing (Phase 11)

### Can Wait

1. External indexer / SDK examples (Phase 15)
