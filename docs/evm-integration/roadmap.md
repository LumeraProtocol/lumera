# Lumera EVM Integration Roadmap

**Last updated**: 2026-03-17
**Cosmos EVM version**: v0.6.0
**Target**: Mainnet-ready EVM integration

Legend: DONE | IN PROGRESS | TODO | DEFERRED

---

## Phase 1: Core EVM Runtime (DONE)

Everything needed to execute Ethereum transactions on the Lumera chain.

| # | Item | Status | Files / Notes |
|---|------|--------|---------------|
| 1.1 | EVM execution module (`x/vm`) wiring | DONE | `app/evm.go` — keeper, store keys, transient keys, module registration |
| 1.2 | Fee market module (`x/feemarket`) wiring | DONE | `app/evm.go` — EIP-1559 dynamic base fee |
| 1.3 | Precisebank module (`x/precisebank`) wiring | DONE | `app/evm.go` — 6-decimal `ulume` <-> 18-decimal `alume` bridge |
| 1.4 | ERC20 module (`x/erc20`) wiring | DONE | `app/evm.go` — STRv2 token pair registration |
| 1.5 | EVM chain ID configuration | DONE | `config/evm.go` — `EVMChainID = 76857769` |
| 1.6 | Denom constants (`ulume`/`alume`/`lume`) | DONE | `config/evm.go`, `config/config.go` |
| 1.7 | Bank denom metadata | DONE | `config/bank_metadata.go` |
| 1.8 | Coin type 60 / BIP44 HD path | DONE | `config/bip44.go` |
| 1.9 | `eth_secp256k1` default key type | DONE | `cmd/lumera/cmd/root.go` |
| 1.10 | EVM genesis defaults (denom, precompiles, feemarket) | DONE | `app/evm/genesis.go` |
| 1.11 | Depinject signer wiring (`MsgEthereumTx`) | DONE | `app/evm/modules.go` — `ProvideCustomGetSigners` |
| 1.12 | Codec registration (`eth_secp256k1` interfaces) | DONE | `config/codec.go` |
| 1.13 | EVM module ordering (genesis/begin/end/pre-block) | DONE | `app/app_config.go` |
| 1.14 | Module account permissions (vm, erc20, feemarket, precisebank) | DONE | `app/app_config.go` |
| 1.15 | Circular dependency resolution (EVMKeeper <-> Erc20Keeper) | DONE | `app/evm.go` — pointer-based forward references |
| 1.16 | Default keeper coin info initialization | DONE | `app/evm/config.go` — `SetKeeperDefaults` for safe early RPC |
| 1.17 | Production guard (test-only reset behind build tag) | DONE | `app/evm/prod_guard_test.go` |

---

## Phase 2: Ante Handler & Transaction Routing (DONE)

Dual-route ante pipeline for Cosmos and Ethereum transactions.

| # | Item | Status | Files / Notes |
|---|------|--------|---------------|
| 2.1 | Dual-route ante handler (EVM vs Cosmos path) | DONE | `app/evm/ante.go` |
| 2.2 | EVM path: `NewEVMMonoDecorator` | DONE | `app/evm/ante.go` — signature, nonce, fee, gas |
| 2.3 | Cosmos path: standard SDK + Lumera decorators | DONE | `app/evm/ante.go` |
| 2.4 | `RejectMessagesDecorator` (block MsgEthereumTx in Cosmos path) | DONE | `app/evm/ante.go` |
| 2.5 | `AuthzLimiterDecorator` (block EVM msgs in authz) | DONE | `app/evm/ante.go` |
| 2.6 | `MinGasPriceDecorator` (feemarket-aware) | DONE | `app/evm/ante.go` |
| 2.7 | `GasWantedDecorator` (gas accounting) | DONE | `app/evm/ante.go` |
| 2.8 | Genesis skip decorator (gentx fee bypass at height 0) | DONE | `app/evm/ante.go` — fixes Bug #3 |
| 2.9 | Pending tx listener decorator | DONE | `app/evm/ante.go` |
| 2.10 | `DelayedClaimFeeDecorator` (claim tx fee waiver) | DONE | `ante/delayed_claim_fee_decorator.go` |
| 2.11 | `EVMigrationFeeDecorator` (migration tx fee waiver) | DONE | `ante/evmigration_fee_decorator.go` |
| 2.12 | `EVMigrationValidateBasicDecorator` (unsigned migration txs) | DONE | `ante/evmigration_validate_basic_decorator.go` |
| 2.13 | Migration-only reduced Cosmos ante subchain (single branch) | DONE | `app/evm/ante.go` |

---

## Phase 3: Feemarket Configuration (DONE)

EIP-1559 fee market with Lumera-specific tuning.

| # | Item | Status | Files / Notes |
|---|------|--------|---------------|
| 3.1 | Default base fee: 0.0025 ulume/gas | DONE | `config/evm.go` |
| 3.2 | Min gas price floor: 0.0005 ulume/gas | DONE | `config/evm.go` — prevents zero-fee spam |
| 3.3 | Base fee change denominator: 16 (~6.25%/block) | DONE | `config/evm.go` — gentler than upstream 8 |
| 3.4 | Consensus max gas: 25,000,000 | DONE | `config/evm.go` |
| 3.5 | Dynamic base fee enabled by default | DONE | `app/evm/genesis.go` |
| 3.6 | Fee distribution via standard SDK path | DONE | Full effective gas price -> fee collector -> x/distribution |
| 3.7 | EVM base fee burn mechanism | TODO | Currently all fees distributed (no burn). Consider EIP-1559 burn for deflationary pressure |
| 3.8 | Raise block gas limit via governance | DEFERRED | 25M is adequate for launch; increase for heavy DeFi if needed |

---

## Phase 4: Mempool & Broadcast Infrastructure (DONE)

EVM-aware app-side mempool with deadlock prevention.

| # | Item | Status | Files / Notes |
|---|------|--------|---------------|
| 4.1 | `ExperimentalEVMMempool` integration | DONE | `app/evm_mempool.go` |
| 4.2 | EVM-aware `PrepareProposal` signer extraction | DONE | `app/evm_mempool.go` |
| 4.3 | Async broadcast dispatcher (deadlock fix) | DONE | `app/evm_broadcast.go` — Bug #5 fix |
| 4.4 | Broadcast worker `RegisterTxService` override | DONE | `app/evm_runtime.go` — local CometBFT client |
| 4.5 | `Close()` override for graceful shutdown | DONE | `app/evm_runtime.go` |
| 4.6 | `broadcast-debug` app.toml toggle | DONE | `cmd/lumera/cmd/config.go` |
| 4.7 | Default `max_txs=5000` | DONE | App config defaults |
| 4.8 | Mempool eviction / capacity pressure testing | DONE | `tests/integration/evm/mempool/capacity_pressure_test.go` — `TestMempoolCapacityRejectsOverflow` |
| 4.9 | Mempool metrics / observability | TODO | Expose mempool size, pending count, rejection rate metrics |

---

## Phase 5: JSON-RPC & Indexer (DONE)

Ethereum JSON-RPC server and transaction indexing.

| # | Item | Status | Files / Notes |
|---|------|--------|---------------|
| 5.1 | JSON-RPC server enabled by default | DONE | `cmd/lumera/cmd/config.go` |
| 5.2 | EVM indexer enabled by default | DONE | `cmd/lumera/cmd/config.go` |
| 5.3 | EVM server command wiring | DONE | `cmd/lumera/cmd/root.go`, `commands.go` |
| 5.4 | Per-IP JSON-RPC rate limiting | DONE | `app/evm_jsonrpc_ratelimit.go` — token bucket proxy |
| 5.5 | EVM tracing (debug API) configurable via app.toml | DONE | `app.toml` `[evm] tracer` field |
| 5.6 | Production CORS origin lockdown | TODO | Currently `Access-Control-Allow-Origin: *` on OpenRPC; need per-environment CORS profiles |
| 5.7 | JSON-RPC namespace exposure lockdown per env | TODO | Lock `debug`, `personal`, `admin` in production |
| 5.8 | Batch JSON-RPC request support testing | DONE | `tests/integration/evm/jsonrpc/batch_rpc_test.go` — 4 batch tests (mixed errors, single-element, duplicates) |
| 5.9 | WebSocket subscription testing | DONE | `tests/integration/evm/mempool/ws_subscription_test.go` — `newHeads`, `logs`, multi-block subscriptions |

---

## Phase 6: Static Precompiles (DONE)

Standard precompile set for EVM-to-Cosmos access.

| # | Item | Status | Files / Notes |
|---|------|--------|---------------|
| 6.1 | Bank precompile | DONE | `app/evm/precompiles.go` |
| 6.2 | Staking precompile | DONE | `app/evm/precompiles.go` |
| 6.3 | Distribution precompile | DONE | `app/evm/precompiles.go` |
| 6.4 | Gov precompile | DONE | `app/evm/precompiles.go` |
| 6.5 | ICS20 precompile | DONE | `app/evm/precompiles.go` — Bug #6 fixed (store key ordering) |
| 6.6 | Bech32 precompile | DONE | `app/evm/precompiles.go` |
| 6.7 | P256 precompile | DONE | `app/evm/precompiles.go` |
| 6.8 | Slashing precompile | DONE | `app/evm/precompiles.go` |
| 6.9 | Blocked-address protections | DONE | Bank send restriction blocks sends to precompile addresses |
| 6.10 | Vesting precompile | DEFERRED | Not provided by upstream cosmos/evm v0.6.0 default registry |
| 6.11 | Precompile gas metering benchmarks | DONE | `tests/integration/evm/precompiles/gas_metering_test.go` — accuracy + estimate/actual matching for all static precompiles |

---

## Phase 7: IBC + ERC20 Middleware (DONE)

Cross-chain token registration and transfer.

| # | Item | Status | Files / Notes |
|---|------|--------|---------------|
| 7.1 | ERC20 IBC middleware — v1 transfer stack | DONE | `app/ibc.go` |
| 7.2 | ERC20 IBC middleware — v2 transfer stack | DONE | `app/ibc.go` |
| 7.3 | Governance-controlled ERC20 registration policy | DONE | `app/evm_erc20_policy.go` — `all`/`allowlist`(default)/`none` |
| 7.4 | `MsgSetRegistrationPolicy` governance message | DONE | `app/evm_erc20_policy_msg.go` |
| 7.5 | Base denom allowlist (uatom, uosmo, uusdc) | DONE | `app/evm_erc20_policy.go` |
| 7.6 | IBC store keys synced to EVM snapshot | DONE | `app/evm.go` — `syncEVMStoreKeys()`, Bug #6 fix |
| 7.7 | EVMTransferKeeper ICS4Wrapper back-reference | DONE | `app/ibc.go` |
| 7.8 | ICS20 precompile transfer tx test | TODO | Pending IBC channel config in integration test setup |

---

## Phase 8: OpenRPC Discovery (DONE)

Machine-readable API spec (unique among Cosmos EVM chains).

| # | Item | Status | Files / Notes |
|---|------|--------|---------------|
| 8.1 | OpenRPC spec generation tool | DONE | `tools/openrpcgen/main.go` |
| 8.2 | Embedded spec (`//go:embed`) | DONE | `app/openrpc/spec.go` |
| 8.3 | `rpc_discover` JSON-RPC method | DONE | `app/openrpc/register.go` |
| 8.4 | `/openrpc.json` HTTP endpoint | DONE | `app/openrpc/http.go` |
| 8.5 | CORS support for OpenRPC endpoint | DONE | `app/openrpc/http.go` |
| 8.6 | Build-time spec sync (`make openrpc`) | DONE | `Makefile` |

---

## Phase 9: Store Upgrades & Migration (DONE)

Chain upgrade handling for EVM module stores.

| # | Item | Status | Files / Notes |
|---|------|--------|---------------|
| 9.1 | v1.12.0 store upgrades (feemarket, precisebank, vm, erc20, evmigration) | DONE | `app/upgrades/v1_12_0/upgrade.go` |
| 9.2 | Adaptive store upgrade manager | DONE | `app/upgrades/store_upgrade_manager.go` |
| 9.3 | EVM keeper refs in upgrade params | DONE | `app/upgrades/params/params.go` |
| 9.4 | ERC20 param finalization after skipped `InitGenesis` | DONE | `app/upgrades/v1_12_0/upgrade.go`, `app/upgrades/v1_12_0/upgrade_test.go` |
| 9.5 | Chain upgrade EVM state preservation test | DONE | `tests/integration/evm/contracts/upgrade_preservation_test.go` — deploy, restart, verify contract code + storage + receipts |

---

## Phase 10: Legacy Account Migration — `x/evmigration` (DONE)

Coin-type-118-to-60 account migration with dual-signature verification.

| # | Item | Status | Files / Notes |
|---|------|--------|---------------|
| 10.1 | Proto definitions | DONE | `proto/lumera/evmigration/` |
| 10.2 | Module skeleton + depinject | DONE | `x/evmigration/module/` |
| 10.3 | Dual-signature verification | DONE | `x/evmigration/keeper/verify.go` |
| 10.4 | `MsgClaimLegacyAccount` handler | DONE | `x/evmigration/keeper/msg_server_claim_legacy.go` |
| 10.5 | `MsgMigrateValidator` handler | DONE | `x/evmigration/keeper/msg_server_migrate_validator.go` |
| 10.6 | Auth migration (vesting-aware) | DONE | `x/evmigration/keeper/migrate_auth.go` |
| 10.7 | Bank balance transfer | DONE | `x/evmigration/keeper/migrate_bank.go` |
| 10.8 | Staking re-keying (delegations, unbonding, redelegations) | DONE | `x/evmigration/keeper/migrate_staking.go` |
| 10.9 | Distribution reward withdrawal | DONE | `x/evmigration/keeper/migrate_distribution.go` |
| 10.10 | Authz grant re-keying | DONE | `x/evmigration/keeper/migrate_authz.go` |
| 10.11 | Feegrant allowance re-keying | DONE | `x/evmigration/keeper/migrate_feegrant.go` |
| 10.12 | Supernode migration | DONE | `x/evmigration/keeper/migrate_supernode.go` |
| 10.13 | Action migration | DONE | `x/evmigration/keeper/migrate_action.go` |
| 10.14 | Claim record migration | DONE | `x/evmigration/keeper/migrate_claim.go` |
| 10.15 | Validator record re-keying | DONE | `x/evmigration/keeper/migrate_validator.go` |
| 10.16 | Fee waiving ante decorators | DONE | `ante/evmigration_fee_decorator.go`, `ante/evmigration_validate_basic_decorator.go` |
| 10.17 | Queries (record, records, stats, estimate, legacy, migrated, params) | DONE | `x/evmigration/keeper/query.go` |
| 10.18 | Genesis export/import | DONE | `x/evmigration/keeper/genesis.go` |
| 10.19 | CLI (`claim-legacy-account`, `migrate-validator`) | DONE | `x/evmigration/client/cli/tx.go` |
| 10.20 | Custom signers for unsigned tx flow | DONE | `x/evmigration/module/signers.go` |
| 10.21 | Params (enable, end_time, rate limit, max_validator_delegations) | DONE | `x/evmigration/types/params.go` |

---

## Phase 11: Testing (IN PROGRESS)

Comprehensive test coverage across all layers.

### Unit Tests (236 tests — DONE)

| # | Area | Tests | Status |
|---|------|-------|--------|
| 11.1 | App wiring / genesis / precompiles / mempool / broadcast | 37 | DONE |
| 11.2 | EVM ante decorators | 28 | DONE |
| 11.3 | EVM module/config guard | 6 | DONE |
| 11.4 | Fee market | 9 | DONE |
| 11.5 | Precisebank | 39 | DONE |
| 11.6 | OpenRPC / generator | 11 | DONE |
| 11.7 | ERC20 policy | 14 | DONE |
| 11.8 | EVMigration keeper | 107 | DONE |
| 11.9 | EVMigration types / module / CLI | 8 | DONE |
| 11.10 | Ante (evmigration fee, validate-basic) | 5 | DONE |

### Integration Tests (111 tests — DONE)

| # | Area | Tests | Status |
|---|------|-------|--------|
| 11.11 | Ante | 3 | DONE |
| 11.12 | Contracts (deploy, interact, ERC20 flows, concurrency, upgrade preservation) | 11 | DONE |
| 11.13 | Fee market | 8 | DONE |
| 11.14 | IBC ERC20 | 7 | DONE |
| 11.15 | JSON-RPC / indexer (+ batch RPC) | 23 | DONE |
| 11.16 | Mempool (+ capacity pressure, WS subscriptions) | 10 | DONE |
| 11.17 | Precisebank | 6 | DONE |
| 11.18 | Precompiles (+ gas metering) | 17 | DONE |
| 11.19 | VM queries / state | 12 | DONE |
| 11.20 | EVMigration | 14 | DONE |

### Devnet Tests (12+ tests — DONE)

| # | Area | Tests | Status |
|---|------|-------|--------|
| 11.21 | EVM basic / fee market / cross-peer | 8 | DONE |
| 11.22 | IBC | 6 | DONE |
| 11.23 | Ports / CORS | 2 | DONE |
| 11.24 | EVMigration tool (prepare, migrate, migrate-validator, cleanup) | 4 modes | DONE |

### Recently Completed Test Gaps

| # | Gap | Status | Implementation |
|---|-----|--------|----------------|
| 11.25 | Mempool eviction / capacity stress tests | DONE | `tests/integration/evm/mempool/capacity_pressure_test.go` |
| 11.26 | Batch JSON-RPC tests | DONE | `tests/integration/evm/jsonrpc/batch_rpc_test.go` |
| 11.27 | WebSocket subscription tests | DONE | `tests/integration/evm/mempool/ws_subscription_test.go` |
| 11.28 | Precompile gas metering benchmarks | DONE | `tests/integration/evm/precompiles/gas_metering_test.go` |
| 11.29 | Chain upgrade EVM state preservation | DONE | `tests/integration/evm/contracts/upgrade_preservation_test.go` |
| 11.31 | Concurrent operation race detection | DONE | `tests/integration/evm/contracts/concurrent_operations_test.go` |
| 11.32 | ERC20 allowance/transferFrom/approve flows | DONE | `tests/integration/evm/contracts/erc20_flows_test.go` (contract-based, not precompile) |

### Remaining Test Gaps (TODO)

| # | Gap | Priority | Notes |
|---|-----|----------|-------|
| 11.30 | Multi-validator EVM consensus scenarios | Low | Expand devnet tests beyond single-validator assertions |

---

## Phase 12: Custom Lumera Module Precompiles (TODO)

EVM contracts calling Lumera-specific functionality. Other chains (Evmos, Kava) ship these at launch.

| # | Item | Status | Priority | Notes |
|---|------|--------|----------|-------|
| 12.1 | Supernode precompile (read-only queries) | TODO | Medium | Query supernode status, metrics from EVM |
| 12.2 | Action precompile (read-only queries) | TODO | Medium | Query action status/results from EVM |
| 12.3 | LumeraID precompile (read-only queries) | TODO | Medium | Verify PastelID from EVM contracts |
| 12.4 | Claim precompile (read-only queries) | TODO | Low | Query claim records |
| 12.5 | Supernode precompile (write paths) | TODO | Low | Register/update supernodes from EVM |
| 12.6 | Action precompile (write paths) | TODO | Low | Submit actions from EVM |
| 12.7 | Precompile integration tests | TODO | Medium | After each precompile is implemented |

---

## Phase 13: CosmWasm + EVM Interaction (TODO)

Lumera is the only Cosmos EVM chain also running CosmWasm. No external precedent exists.

| # | Item | Status | Priority | Notes |
|---|------|--------|----------|-------|
| 13.1 | Design interaction model document | TODO | Medium | Bridge? Shared queries? Explicit isolation? |
| 13.2 | Cross-runtime query paths (if designed) | TODO | Medium | CosmWasm -> EVM state queries or vice versa |
| 13.3 | Cross-runtime message calls (if designed) | TODO | Low | Full bidirectional contract calls |
| 13.4 | Integration tests for interaction model | TODO | Medium | After design is finalized |

---

## Phase 14: Production Hardening (TODO)

Final operational readiness for mainnet.

| # | Item | Status | Priority | Notes |
|---|------|--------|----------|-------|
| 14.1 | Security audit of EVM integration | TODO | **Critical** | All comparable chains (Evmos, Kava, Cronos) had dedicated EVM audits |
| 14.2 | CORS origin lockdown per environment | TODO | High | Replace `*` with env-specific origins |
| 14.3 | JSON-RPC namespace exposure profiles | TODO | High | Lock `debug`/`personal`/`admin` in production |
| 14.4 | Fee market monitoring runbook | TODO | High | Base fee tracking, gas utilization, alerting thresholds |
| 14.5 | Node operator EVM configuration guide | TODO | High | app.toml tuning, RPC exposure, tracer config |
| 14.6 | Disaster recovery procedures (EVM state) | TODO | Medium | Recovery from corrupt EVM state, indexer rebuild |
| 14.7 | Load testing / performance benchmarks | TODO | Medium | TPS under mixed Cosmos+EVM workload |
| 14.8 | EVM governance proposal workflows | TODO | Low | Documented gov flows for precompile toggles, param changes |

---

## Phase 15: Ecosystem & Tooling (TODO)

External infrastructure for production ecosystem.

| # | Item | Status | Priority | Notes |
|---|------|--------|----------|-------|
| 15.1 | External block explorer (Blockscout / Etherscan-compat) | TODO | High | All comparable chains have this at mainnet |
| 15.2 | MetaMask network configuration guide | TODO | Medium | Chain ID 76857769, RPC endpoints, native token |
| 15.3 | Hardhat/Foundry getting-started guide | TODO | Medium | Developer onboarding for Solidity devs |
| 15.4 | External indexer (TheGraph / SubQuery) | TODO | Low | Community-facing data availability |
| 15.5 | SDK / client library examples | TODO | Low | ethers.js / web3.js examples for Lumera |
| 15.6 | Faucet for testnet (EVM-compatible) | TODO | Medium | MetaMask-friendly faucet |

---

## Bugs Found and Fixed

| # | Bug | Status | Phase |
|---|-----|--------|-------|
| B1 | EVM broadcast worker: sender address not recovered | FIXED | Phase 4 |
| B2 | Feemarket base fee decays to zero on idle devnet | FIXED | Phase 3 |
| B3 | Gentx rejected by MinGasPriceDecorator during InitGenesis | FIXED | Phase 2 |
| B4 | IBC transfer silently fails with out-of-gas | FIXED | Phase 7 |
| B5 | EVM mempool deadlock on nonce-gap promotion | FIXED | Phase 4 |
| B6 | ICS20 precompile panics: IBC store keys not registered | FIXED | Phase 6/7 |
| B7 | Upgrade handler seeds `aatom` denom instead of `alume` in EVM coin info | FIXED | Phase 9 |
| B8 | Upgrade handler leaves `x/erc20` disabled after skipped `InitGenesis` | FIXED | Phase 9 |

---

## Summary Dashboard

| Phase | Description | Status | Completion |
|-------|-------------|--------|------------|
| 1 | Core EVM Runtime | DONE | 17/17 |
| 2 | Ante Handler & Tx Routing | DONE | 13/13 |
| 3 | Feemarket Configuration | DONE | 6/8 |
| 4 | Mempool & Broadcast | DONE | 8/9 |
| 5 | JSON-RPC & Indexer | DONE | 7/9 |
| 6 | Static Precompiles | DONE | 10/11 |
| 7 | IBC + ERC20 Middleware | DONE | 7/8 |
| 8 | OpenRPC Discovery | DONE | 6/6 |
| 9 | Store Upgrades & Migration | DONE | 4/4 |
| 10 | Legacy Account Migration | DONE | 21/21 |
| 11 | Testing | IN PROGRESS | 31/32 |
| 12 | Custom Lumera Precompiles | TODO | 0/7 |
| 13 | CosmWasm + EVM Interaction | TODO | 0/4 |
| 14 | Production Hardening | TODO | 0/8 |
| 15 | Ecosystem & Tooling | TODO | 0/6 |
| | **TOTAL** | | **130/155** |

### Before Mainnet (Critical Path)

1. **Security audit** (Phase 14.1) — non-negotiable for any Cosmos EVM chain
2. **Production JSON-RPC hardening** (Phase 14.2, 14.3) — CORS + namespace lockdown
3. **Block explorer** (Phase 15.1) — user-facing ecosystem requirement
4. **Monitoring runbook** (Phase 14.4) — operator readiness

### Near-Term Priorities

1. Custom module precompiles — read-only queries first (Phase 12.1-12.3)
2. CosmWasm + EVM interaction design (Phase 13.1)
3. Multi-validator EVM consensus testing (Phase 11.30)

### Can Wait

1. EVM base fee burn mechanism (Phase 3.7)
2. Write-path precompiles (Phase 12.5-12.6)
3. External indexer / SDK examples (Phase 15.4-15.5)
