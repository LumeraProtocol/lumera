# Changelog

---

## 1.20.0

Changes included since `v1.11.1` (range: `v1.11.1..v1.20.0`).

Full EVM integration documentation: [docs/evm-integration/main.md](docs/evm-integration/main.md)

- Added Cosmos EVM v0.6.0 with four new modules: `x/vm` (EVM execution), `x/feemarket` (EIP-1559 dynamic base fee), `x/precisebank` (6-decimal `ulume` ↔ 18-decimal `alume` bridge), and `x/erc20` (STRv2 token pair registration + IBC middleware).
- Added dual-route ante handler (`app/evm/ante.go`) routing Ethereum extension txs to the EVM path and all others to the Cosmos path, with pending tx listener support.
- Added app-side EVM mempool (`app/evm_mempool.go`) with Ethereum-like sender ordering, nonce-gap handling, and same-nonce replacement rules.
- Added async broadcast queue (`app/evm_broadcast.go`) to prevent mempool mutex re-entry deadlock during nonce-gap promotion.
- Added 11 static precompiles: P256, Bech32, Staking, Distribution, ICS20, Bank, Gov, Slashing, plus custom Action (`0x0901`), Supernode (`0x0902`), and Wasm (`0x0903`) precompiles for Lumera-specific EVM→Cosmos and EVM→CosmWasm calls.
- Added JSON-RPC server and indexer enabled by default with 7 namespaces; optional per-IP rate limiting proxy (`app/evm_jsonrpc_ratelimit.go`) with configurable token bucket.
- Added EVM tracing support configurable at runtime via `app.toml [evm] tracer` (json, struct, access_list, markdown).
- Added OpenRPC discovery: `rpc_discover` JSON-RPC method, `GET /openrpc.json` HTTP endpoint with CORS, gzip-compressed spec embedded in binary (315 KB → 20 KB), and build-time generation via `tools/openrpcgen`.
- Changed default key type to `eth_secp256k1` and BIP44 coin type from 118 to 60 for Ethereum-compatible wallet derivation (MetaMask, Ledger).
- Added EVM chain ID `76857769`, base fee `0.0025 ulume/gas`, min gas price floor `0.0005 ulume/gas` (prevents zero-fee spam), and base fee change denominator `16` (~6.25% adjustment per block).
- Added IBC ERC20 middleware wired on both v1 and v2 transfer stacks with governance-controlled registration policy (`all`/`allowlist`/`none`) via `MsgSetRegistrationPolicy`.
- Added `x/evmigration` module for legacy coin-type-118 → 60 account migration with dual-signature verification and multi-module atomic state re-keying (auth, bank, staking, distribution, authz, feegrant, supernode, action, claim); a separate `MsgMigrateValidator` flow re-keys the validator operator, deletes the orphaned legacy KV row, and rejects jailed validators with operator guidance.
- Added multisig migration support: `LegacyProof` proto with single-key + multisig `oneof` variants, `MaxMultisigSubKeys` param (default 20), and a K/N mirror-source consensus rule requiring sub-key count and threshold to match between legacy and new sides. Verifier helpers (`verifySecp256k1Sig`, `verifySingleKeyProof`, `verifyMultisigProof`) include duplicate-sub-key preflight, `signer_indices`/sub-key uniqueness checks, and a defense-in-depth `ValidateProofPair` at the message-server boundary.
- Added a four-step offline multisig CLI flow (`generate-proof-payload` → `sign-proof` → `combine-proof` → `submit-proof`) so co-signers can participate without sharing keys; `combine-proof` verifies each partial cryptographically before assembling the final proof, surfacing tampered partials before on-chain submission.
- Added user-facing migration helper scripts (`scripts/migrate-account.sh`, `scripts/migrate-validator.sh`, `scripts/migrate-multisig.sh`) wrapping the full pre-flight estimate → key import → snapshot → submit → verify flow, with multisig-aware K/N partials, validator-specific cap checks and downtime acknowledgment, and fail-closed query handling so script-level success implies on-chain success.
- Added `devnet/scripts/lumera-helper.sh unjail-validator` helper plus downtime warnings in the validator migration guide for operators approaching the slashing window.
- Added fee-waiving ante decorator for migration txs (`ante/evmigration_fee_decorator.go`) since new addresses have zero balance pre-migration.
- Added migration-aware mempool signer extractor (`app/evmigration_signer_extraction_adapter.go`) wired into `ExperimentalEVMMempool.CosmosPoolConfig.SignerExtractor`. Without it, the SDK's default `DefaultSignerExtractionAdapter` rejects zero-signer migration txs (`MsgClaimLegacyAccount`, `MsgMigrateValidator`) with "tx must have at least one signer" during app-side mempool admission/proposal selection, blocking `submit-proof` broadcast. The adapter synthesizes a deterministic signer from the message's `legacy_address` for migration-only txs and delegates everything else to the EVM-aware default.
- Added regression coverage for zero-signer evmigration tx admission: unit tests pin the upstream SDK mempool rejection, adapter fallback and negative cases (malformed `legacy_address`, nonexistent legacy accounts, multi-message and mixed-message txs), app tests cover `PrepareProposal` inclusion and disabled-mempool wiring, and real-node integration tests broadcast `submit-proof`-style tx bytes through CometBFT `broadcast_tx_sync`.
- Hardened the migration ante (`x/evmigration/keeper/ante.go`) to enforce the migration admission window and cheap state plausibility before mempool admission: since migration txs are fee-free and signature-free, `VerifyMigrationProofsForAnte` now rejects them with `ErrMigrationDisabled`/`ErrMigrationWindowClosed` when `EnableMigration` is off or `MigrationEndTime` has passed, and rejects proof-valid but impossible migrations such as nonexistent legacy accounts, already-migrated sources, reused destination addresses, and non-validator `MsgMigrateValidator` sources. This bounds the zero-fee mempool-spam surface to the operator-defined window (no-op under default params; mainnet sets a concrete `MigrationEndTime`) and avoids retaining txs that would fail immediately at message execution.
- Added v1.20.0 upgrade handler with store additions for feemarket, precisebank, vm, erc20, and evmigration; post-migration finalization sets Lumera EVM params, feemarket params, and ERC20 defaults.
- Added Action module precompile (`0x0901`) and Supernode module precompile (`0x0902`) giving Solidity contracts native access to `MsgRequestAction`/`MsgFinalizeAction` (including LEP-5 cascade availability commitments) and supernode queries/registration respectively.
- Added CosmWasm ↔ EVM cross-runtime bridge (Phase 1, non-payable, depth-1 reentrancy guard): `WasmPrecompile` at `0x0903` exposes `execute`, `query`, `contractInfo`, `rawQuery` to Solidity, and a custom Wasm message handler + query handler decorator (`app/wasm_evm_plugin.go`) lets CosmWasm contracts invoke EVM contracts via `ApplyMessage` with an explicitly-constructed `statedb`. Cross-runtime gas is capped at `DefaultCrossRuntimeGasCap = 3,000,000` per call.
- Added blocked-address protections: module accounts and all precompile addresses are excluded from bank sends to prevent accidental token loss.
- Added centralized bank denom metadata (`config/bank_metadata.go`) and `RegisterExtraInterfaces` for `eth_secp256k1` crypto interface registration across SDK + EVM paths.
- Added `RegisterTxService` override (`app/evm_runtime.go`) to capture the local CometBFT client for the async broadcast worker, replacing the stale HTTP client that `SetClientCtx` provides before CometBFT starts.
- Added depinject custom signer wiring for `MsgEthereumTx` and safe early-RPC keeper coin info initialization (`SetKeeperDefaults`) to prevent panics before genesis runs.
- CosmWasm (`wasmd v0.61.6` + `wasmvm v3.0.3`) and EVM coexist in the same runtime — Lumera is the only Cosmos chain shipping both simultaneously, and the cross-runtime bridge above lets contracts call across the boundary in either direction.
- Added evmigration query endpoints for migration planning and monitoring: `MigrationEstimate` (pre-migration impact analysis with delegation/unbonding/redelegation/authz/feegrant counts), `MigrationStats` (on-chain progress tracking), `LegacyAccounts` (paginated unmigrated account listing), and `MigratedAccounts` (searchable migration history).
- Added dual signature verification in evmigration: legacy proofs accept both raw SHA-256 CLI signing and ADR-036 wallet signing (Keplr/Leap); new address proofs accept both raw Keccak-256 and EIP-191 `personal_sign` (MetaMask), ensuring compatibility across all major wallet types.
- Added `app.toml` auto-config migration (`cmd/lumera/cmd/config_migrate.go`) for nodes upgrading from pre-EVM binaries — automatically detects missing `[evm]`, `[json-rpc]`, `[tls]`, and `[lumera.*]` sections and regenerates `app.toml` with Lumera defaults while preserving existing operator settings.
- Updated app-side mempool defaults to keep fresh testnet/mainnet-style homes bounded at `mempool.max-txs = 10000`; config migration now rewrites legacy no-op `max-txs = -1` to `5000` on devnet and `10000` on testnet/mainnet, while preserving the real Cosmos EVM v0.6.0 `[evm.mempool]` defaults (`global-slots = 5120`, `global-queue = 1024`).
- Added EVM mempool Prometheus metrics (`app/evm_mempool_metrics.go`): gauges for mempool size, pending/queued counts, and broadcast queue depth; labeled rejection counter (`rejections_total{source,reason}`) for observability.
- Added `MsgSetRegistrationPolicy` governance message for ERC20 IBC auto-registration: operators can toggle policy between `all`, `allowlist`, and `none` modes; pre-populated genesis allowlist includes inert base denom traces for major tokens (uatom, uosmo, uusdc, inj) ready for governance channel binding.
- Added evmigration user guides under `docs/evm-integration/user-guides/`: `migration.md` (CLI/Keplr/MetaMask account migration), `validator-migration.md`, `supernode-migration.md`, and `migration-scripts.md` reference for the helper scripts above.
- Added node operator EVM configuration guide (`docs/evm-integration/user-guides/node-evm-config-guide.md`) and tuning guide (`docs/evm-integration/user-guides/tune-guide.md`) covering `app.toml` tuning, RPC exposure, tracer config, and rate limit setup.
- Added comprehensive EVM integration test suites under `tests/integration/evm/` covering ante, contracts, feemarket, IBC ERC20, JSON-RPC, mempool, precisebank, precompiles, and VM queries.
- Added devnet evmigration end-to-end tests validating the full legacy account migration flow across a multi-validator network, plus multisig-mode coverage (single-key, multisig-of-secp256k1, and multisig-of-eth destinations) and `PermanentLocked` vesting fixtures.
- Added `make devnet-evm-upgrade` and versioned 1.11.1 devnet targets to exercise the on-chain `v1.11.1 → v1.20.0` upgrade path end-to-end against the multi-validator devnet.
- Renamed the devnet upload service from `network-maker` to `lumera-uploader` across docs, dockerfile, and lifecycle scripts; legacy binary names are still recognized by `devnet/scripts/stop.sh` for backwards compatibility.
- Updated transitive Go dependencies (CosmWasm, go-ethereum, and others) to address critical and high-severity security vulnerabilities surfaced by Go module audit.
- Bumped transitive Go modules `github.com/go-chi/chi/v5` (5.2.3 → 5.2.4) and `github.com/quic-go/quic-go` (0.54.1 → 0.59.1, with `quic-go/qpack` 0.5.1 → 0.6.0) from Dependabot; verified with `go build ./...`, `make lint`, and a live multi-validator devnet run.
- Migrated the `precompiles/solidity` example/dev toolchain from Hardhat 2 to Hardhat 3 (`@nomicfoundation/hardhat-toolbox-mocha-ethers`, ESM, `network.create()` connection API, ethers-v6-native typechain) and pinned patched transitives (`lodash-es`, `serialize-javascript`, `diff`) via npm `overrides`, cutting `npm audit` findings from 50 to 11 with 0 critical/high/moderate remaining. All 11 precompile integration tests pass against the live devnet.

---

## 1.12.0

Changes included since `v1.11.1` (range: `v1.11.1..v1.12.0`).

- Added LEP-5 Cascade availability commitments: BLAKE3 Merkle commitments, chunk proofs, commitment validation, SVC proof verification, and integration/system/devnet coverage.
- Added Everlight Phase 1 in `x/supernode`: `STORAGE_FULL` state, reward distribution params/state/queries, periodic pool distribution, registration-fee routing, and v1.12.0 upgrade initialization.
- Added LEP-6 storage-truth foundation scaffolding in `x/audit/v1`, including storage-truth messages, scoring/state, query support, pruning, and broad keeper/system coverage.
- Replaced consensus-sensitive protobuf maps with deterministic concrete structures and added consensus-determinism CI coverage.
- Fixed audit/supernode AutoCLI query marshalling issues, including float64 query args and `get-metrics` wiring.
- Improved Everlight devnet tests, genesis defaults, epoch-boundary handling, and upgrade-handler completeness for post-1.11.1 defaults.
- Added Ledger build support and pinned release builds to Ubuntu 22.04 for glibc compatibility.

---

## 1.11.1-hotfix

Changes included since `v1.11.1` (range: `v1.11.1..v1.11.1-hotfix`).

- Fixed bad `EVIDENCE_TYPE_CASCADE_CLIENT_FAILURE` handling in audit evidence submission and regenerated affected protobuf/OpenAPI outputs.
- Included Ledger build support and build-tag workflow updates from the release branch.

---

## 1.11.1

Changes included since `v1.11.0` (range: `v1.11.0..v1.11.1`).

- Added the v1.11.1 audit upgrade handler to enforce a `min_disk_free_percent` floor and repair missing audit params during upgrade.
- Added audit store-loader selection and tests for safe v1.11.1 upgrade startup.

---

## 1.11.0

Changes included since `v1.10.1` (range: `v1.10.1..v1.11.0`).

- Added `x/audit/v1` from scratch.
- Added epoch-based audit reporting, evidence handling, and enforcement for supernodes.
- Added audit gRPC/REST messages and queries (epoch, assignments, reports, evidence, params).

---

## 1.10.1

Changes included since `v1.10.0` (range: `v1.10.0..v1.10.1`).

- Upgrade safety: add a conditional store loader for `v1.10.1` that only renames the legacy `Consensus` store when needed and avoids panics when the new `consensus` store already exists.
- Consensus params: ensure `x/consensus` params are present and repair missing/incomplete values during the upgrade.
- Devnet tooling: adaptive store loader can be enabled to reconcile on-disk stores against expected modules when skipping intermediate upgrades.
- Devnet tests: validator IBC tests now auto-detect the validator container (RPC + key) instead of assuming `supernova_validator_1`.
- Upgrade guidance: do not apply `v1.10.0` on testnet or mainnet.

---

## 1.10.0

Changes included since `v1.9.1` (range: `v1.9.1..v1.10.0`).

- Cosmos SDK: upgraded from v0.50.14 to v0.53.5, CometBFT upgraded to v0.38.20
- enabled unordered
- migrated consensus params from `x/params` to `x/consensus` via baseapp.MigrateParams; removed `x/params` usage.
- IBC: upgraded to IBC-Go from v10.3.0 to v10.5.0 with IBC v2 readiness (Router v2, v2 packet/event handling helpers).
- Wasm: upgraded wasmd from v0.55.0-ibc2.0 to v0.61.6 and wasmvm from v3.0.0-ibc2.0 to v3.0.2.
- Module changes: removed `x/crisis`, deleted its store key and disabled crisis invariants by default.
- Client/indexer impact: legacy tx logs removed in SDK v0.53.
- Unordered transactions feature (SDK v0.52) is enabled: "fire-and-forget" tx submission model with timeout_timestamp as TTL/replay protection, useful for throughput-focused clients where strict ordering is not required.

---

## 1.9.1

Changes included since `v1.9.0` (range: `v1.9.0..v1.9.1`).

- Action/ICA: persist `app_pubkey` on new actions, expose `app_pubkey` in action query responses, and regenerate action protobufs.
- Action/crypto: refreshed signature verification paths (ADR-36 fallback, DER→RS64) and added coverage for app_pubkey validation/caching + query output.
- Devnet/Hermes: added ICA cascade flow tests and IBC helpers; updated Hermes configs/scripts and devnet setup scripts; removed legacy `devnet/tests/test-channel.sh`.
- Dependencies/docs: updated devnet and root Go module files and refreshed `readme.md`.

---

## 1.9.0

Changes included since `v1.8.5` (range: `v1.8.5..v1.9.0`).

- Supernode: added self-reported metrics with validation `MsgReportSupernodeMetrics`, enforcing staleness/compliance in EndBlock, storing typed metrics (version/cpu/mem/disk/peers, tri-state open_ports), exposing a `GetMetrics` query with refreshed parameter defaults, and expanded system tests/docs.
- Revamped action queries with secondary indices (state/creator/type/block height/supernode), bounded prefix iterators, and a new `ListActionsByCreator` endpoint for paginated lookups.
- Enforced a unique supernodeAccount→validator index with lookup helpers; on-chain upgrade `v1.9.0` backfills the new action and supernode indices without store key changes.
- Testing tightened with supernode metrics system tests and simulation coverage for the validator↔supernode 1:1 invariant.
- Hardened actions for interchain-account use: `MsgRequestAction` now requires `app_pubkey` when the creator is an ICA, verifies app-level signatures (ADR-36 fallback), `MsgApproveAction` now returns `actionId`/`status`.
- Action tickets: added `fileSizeKbs` to action requests and keeper/simulation handling.
- Devnet tooling: added Network-Maker UI support (enhanced compose generator, multi-account provisioning, start/stop/restart scripts) to streamline automation.

---

## 1.8.5

Changes included since `v1.8.4` (range: `v1.8.4..v1.8.5`).

- Register every upgrade handler at startup (before Load) so state-sync nodes always have handlers available, even without an on-disk plan.
- Fixed x/upgrade downgrade verification panics on state-synced nodes that already applied v1.8.4 but lacked a registered handler.
- Standardised migration-only upgrades with `standardUpgradeHandler`.
- Devnet Docker tests: `network-maker-setup.sh` now provisions **multiple** network-maker accounts per validator (configurable in `config/config.json`), funds them, and writes them into generated configs for automated Network-Maker scenarios.
- Compression: Action/Sense ID generation now uses the DataDog zstd binding with bounded high-compression helpers and clearer error handling.

---

## 1.8.4

Changes included since `v1.8.0` (range: `v1.8.0..v1.8.4`).

- Added a legacy type URL aliasing framework (`internal/legacyalias`) and wired it into module registration so pre-versioned Action/Supernode messages stored on-chain continue to decode after the versioned protobuf migration. AutoCLI now wraps the codec resolvers with the legacy-aware resolver to keep CLI and REST queries seamless.
- Introduced a protobuf enum bridge (`internal/protobridge`) that double-registers enum descriptors with both gogoproto and the standard protobuf runtime, eliminating REST/GRPC mismatches when the gateway still consults the legacy registry.
- Normalised `Action.price` to a plain string in the proto definition and regenerated bindings (`x/action/v1/types`), improving protobuf compatibility with external tooling while keeping existing data accessible through the legacy alias layer.
- Reworked upgrade handling:
  - Added a shared `AppUpgradeParams` bundle and refactored every versioned upgrade handler to consume it.
  - Consolidated handler/store-loader registration into a single path (`app.setupUpgrades`) that selects the appropriate configuration per plan and panics early when the binary is missing a scheduled plan.
  - Recorded explicit network-specific rules: v1.8.0 handlers run only on devnet/testnet; v1.8.4 registers the handler everywhere but only loads the store changes (PFM store addition, legacy NFT removal) on mainnet.
  - Added a dedicated `app/upgrades/v1_8_4` package for the new upgrade flow.
- Tweaked devnet tooling (`Makefile.devnet`, query helpers) to support the upgraded workflow and verified the new upgrade sequence via dockerised devnet tests (1.7.2 -> 1.8.0 -> 1.8.4).
- Signatures: added ADR-36/Keplr arbitrary-signing support and DER→RS64 coercion in signature verification; strengthened Cascade/Sense flows with Kademlia ID checks based on `Signatures` and counters.

---

## 1.8.0

Changes included since `v1.7.2` (range: `v1.7.2..v1.8.0`).

🚀 This release delivers major upgrades across Lumera’s blockchain core, IBC, CosmWasm, Ignite CLI, governance automation, and devnet infrastructure — improving performance, reliability, and developer experience.

### 🔗 IBC Upgrade (v8 → v10), Router v2 API

IBC v2.0 brings improved cross‑chain routing and middleware support, laying the groundwork for integration with non‑Cosmos ecosystems. In particular, the standardized ICS‑30 and ICS‑27 layers plus Router V2 make it easier for projects to build IBC bridges to other environments like Ethereum (via light‑client bridges) or Solana (through proof verification modules). &#x20;

- Upgraded to**ibc-go v10.3.0** with full Router V2 (`SetRouterV2`) support.
- Added**Packet-Forward Middleware (PFM)**\*\* — an ICS‑30 compliant IBC middleware capable of routing packets received on one chain to another counterparty chain over IBC, allowing intermediary routing where direct connections are missing. This enables chains to serve as intermediaries between two networks that do not share a direct IBC connection.
- Implemented**Interchain Accounts (ICS-27)** for both Controller and Host — allows one blockchain to perform actions on another blockchain as if it had a wallet there. It extends IBC beyond simple token transfers (ICS‑20) by letting a controller chain manage an account on a host chain through IBC packets.
- Updated**ICS-20 Transfer Module** with middleware and`ICS4Wrapper` — an interface abstraction introduced in IBC-Go to allow middleware layers (like ICS-27 or PFM) to wrap and intercept low-level packet operations such as sending and acknowledging IBC packets without modifying the core IBC modules.
- Integrated**IBC Callbacks Middleware** (`ibccallbacks.NewIBCMiddleware`) — a middleware that wraps the IBC ICS4 stack to expose pre/post hooks around packet lifecycle events (send, recv, ack, timeout). It enables cross‑module orchestration, telemetry, and custom reactions to IBC traffic without modifying core IBC or app modules (`ibccallbacks.NewIBCMiddleware`).
- Registered**light clients**: Tendermint (`ibctm`) — verifies consensus states from Tendermint-based blockchains, and Solomachine (`solomachine`)— verifies off-chain or non-Tendermint entities (like relayers, wallets, or standalone machines).
- Removed obsolete**capability keepers** and introduced new middleware stacks.

### 🧰 CosmWasm & WasmVM Upgrades

- Upgraded to**wasmd v0.55.0-ibc2.0** and**wasmvm v3.0.0-ibc2.0**.
- Added**IBC middleware for Wasm contracts** (contract-level callbacks).
- Registered **Wasm** **snapshot extensions** for full contract state backups. Now the Lumera chain can export and restore entire contract states, including code, metadata, and storage, as part of the Cosmos SDK;s snapshot system.
- Benefits of this upgrade:
  - **IBC v2 compatibility**: Aligns contracts/runtime with IBC-Go v10 Router v2 and middleware semantics, avoiding legacy capability/fee middleware assumptions.
  - **Stronger correctness & safety**: Picks up numerous bug fixes in the 0.53→0.55 line of wasmd and in wasmvm 3.x.
  - **Cleaner middleware integration**: Uses modern`ICS4Wrapper` patterns for ICA (ICS‑27),`PFM`, and callbacks, simplifying contract-level IBC orchestration.
  - **Operational parity**: Matches ecosystem baselines (chains/relayers commonly testing against wasmd ≥0.55, wasmvm ≥3.0.0‑ibc2.0), reducing integration friction.
  - **Future‑proofing**: Unblocks subsequent SDK/IBC upgrades and enables contract‑level IBC features introduced with IBC v2 APIs.

### ⚙️ Ignite CLI Upgrade (v28 → v29)

- Migrated to**Ignite v29.x**.
- Adopted**Buf-only protobuf generation (buf v2)** — faster builds, schema validation, breaking-change detection, and improved CI/CD integration.
- Removed api/lumera and Pulsar-generated files.
- Adopted new \`appconfig\` pattern for module initialization.
- Separated**DepInject wiring** for`supernode`,`claim`, and`action` modules.

### ❌ Module Removals & Cleanup

- Removed obsolete**NFT module**.
- Removed legacy**v1.0.0 upgrade handler**.
- Implemented**v1.8.0 upgrade handler** to migrate IBC, add PFM store key, and remove NFT state.

### 🧪 Tests

- **IBC v10 Test Harness & Unit Tests**: The test harness has been enhanced with customized IBC v10 unit tests for Lumera. It now fully aligns with ibc-go v10 requirements for integration and router testing. `testing_app.go` exposes the IBC router (`GetIBCRouter`) and executes via `FinalizeBlock` instead of `DeliverTx`, supporting the v10 optimistic-exec model. `chain.go` seeds multiple sender accounts and handles both legacy and v2 packet queues (`PendingSendPackets`, `PendingSendPacketsV2`). `path.go` ports v10 relay helpers for draining queues through `RelayAndAckPendingPackets` and forwarding v2 payloads via the new endpoints.
- **IBC v2 Endpoint Support**: New `endpoint_v2.go` implements wrappers for `MsgRegisterCounterparty`, `MsgSendPacket`, `MsgRecvPacket`, `MsgAcknowledgement`, and `MsgTimeout` to test the v2 channel API and v2 proof verification.
- **Event Capture & Pending Queues**: Updated `wasm.go` records both v1 and v2 send events, tracks pending packets for relay helpers, and adds contract lifecycle/storage helpers for integration tests.
- **Integration Tests Updated**: `relay_test.go`, `ibc_integration_test.go`, `relay_pingpong_test.go`, and `ibc_callbacks_test.go` migrated to v10 helpers, asserting queue lengths and balances for Router V2 and callback verification. System-level tests also use new helper APIs.
- **New IBC-Focused Tests**: Added multiple v10 and Router V2 test cases:

  - `TestChangeValSet` — exercises IBC client updates after validator power changes, validating val-set tracking.
  - `TestJailProposerValidator` — confirms the client continues to update even if the proposer validator is jailed.
  - `TestParsePacketsFromEvents` — validates event parsing helpers that separate v1/v2 packet events for relays.
  - `TestIBC2SendReceiveMsg` — contract-to-contract transfer through Router V2 channels,  confirming the new packet queues and v2 endpoint helpers move payloads and increment counters.
  - `TestIBC2TimeoutMsg` — ensures proper timeout callback handling for IBC v2 packets.
- Together these changes align Lumera’s testing framework with **ibc-go v10**, giving coverage for Router V2 and ICS4Wrapper behavior while ensuring both classic and v2 packets can be emitted, captured, and relayed inside wasm-centric integration suites.
- **Wasm Test Updates**: `tests/system/wasm/ibc2_test.go` is new and covers contract-to-contract flows over Router V2 channels. `TestIBC2SendReceiveMsg` runs 100 relay iterations with `RelayPendingPacketsV2` to verify bidirectional v2 payloads. `TestIBC2TimeoutMsg` triggers packet timeouts and confirms timeout callback counters increment. Existing suites (`relay_test.go`, `relay_pingpong_test.go`, `ibc_integration_test.go`, and `common_test.go`) were upgraded to the new queue plumbing, coordinator setup, and harness helpers. `wasm.go` now records v1/v2 events with `CaptureIBCEvents` and `CaptureIBCEventsV2`, while `path.go` and `endpoint_v2.go` provide v2 send/recv/ack/timeout helpers. These updates extend coverage from simple ICS‑20 transfers to full ibc‑go v10 Router V2 behavior, contract callbacks, and timeout handling.
- Added **unit and integration tests** validating expired action refunds.

### ⚙️ Core System Improvements

- Implemented \`QueryServer\` for Supernode and Action modules.
- Added \*\*AnteHandler improvement: \*\*``
  - Detects duplicate IBC relay transactions (`MsgRecvPacket`,`MsgAcknowledgement`,`MsgTimeout`) by checking packet commitment/receipt/ack state**before** execution.
  - Short-circuits redundant relays to avoid paying gas for no-op execution, reducing mempool and consensus load.
  - Mitigates race conditions when multiple relayers submit the**same** proof within the same block/height (relay collisions).
  - Emits clear logs/telemetry for deduped transactions and returns a deterministic, non-state-changing error.
  - Improves end-to-end reliability for`RelayAndAckPendingPackets` and Router V2 flows in tests and devnet.
- Implemented**refund of expired action fees** — fees are returned to creators before marking the action as expired.
- Wired**AppModule.EndBlock** to**keeper EndBlocker** for expiration processing.

### 🧩 Devnet Testing Infrastructure

#### 🔧 Makefile & Build System

- Added modular \`Makefile.devnet\` with lifecycle targets:
  - `devnet-build`,`devnet-up`,`devnet-down`,`devnet-upgrade`,`devnet-clean`,`devnet-new`.
- Supports external genesis, configurable binaries (`DEVNET_BIN_DIR`), and Docker Compose integration.

#### 🐳 Devnet Docker System

- Multi-validator architecture (5 validators + Hermes relayer).
- Persistent volumes, full port mapping, structured logs.

#### 🔗 Hermes / Simd Integration

- Hermes v1.13.3 with IBC-Go v10.3.0 and`simd` built from source.
- Automated channel creation and metadata validation.

#### 🏦 Governance Upgrade Automation

- End-to-end automation of proposal → deposit → vote → upgrade testing.
- Auto-detects duplicate proposals and validates upgrade heights.
- Dynamic deposit retrieval and pre-funded validator key voting.
- Retry-safe logic and event validation.

#### 🚀 Start Script Enhancements

- `start.sh` supports`auto`,`bootstrap`,`run`, and`wait` modes.
- Auto-installs binaries, monitors height, coordinates services.

- Enable/disable each component via flags or environment variables when bringing up the devnet (kept generic here to avoid locking to specific flag names).
- **Optional service installers**: add-on installation toggles for**Supernode**,**Network‑Maker**, and**SN Client** (enable via flags/env when bringing up the devnet). Network‑Maker installation on a selected validator is driven by the`network-maker` flag in`validators.json`.

#### 📋 Devnet Architecture Overview

```go
┌─────────────────────────────────────────────────────────────┐
│                    Lumera Devnet Architecture               │
├─────────────────────────────────────────────────────────────┤
│ Build System   │ Container Orchestration │ Testing          │
├────────────────┼──────────────────-──────┼──────────────────┤
│ • Makefile     │ • Docker Compose        │ • Go Tests       │
│ • Targets      │ • Multi-validator       │ • Shell Tests    │
│ • Devnet Mgmt  │ • Hermes Integration    │ • Governance     │
│ • Upgrade Proc │ • Networking            │ • IBC Validation │
└─────────────────────────────────────────────────────────────┘
```

### 📝 Summary of Breaking Changes

| Component   | Old Version     | New Version    |
| ----------- | --------------- | -------------- |
| Cosmos SDK  | v0.50.12        | v0.50.14       |
| IBC         | v8.5.1          | v10.3.0        |
| CosmWasm    | v0.53.0         | v0.55.0-ibc2.0 |
| wasmvm      | v2.1.2          | v3.0.0-ibc2.0  |
| Ignite      | v28.x           | v29.x          |
| Proto Build | Pulsar + Buf v1 | Buf v2 only    |
| NFT Module  | Present         | Removed        |

---

## 1.7.2

Changes included since `v1.7.0` (range: `v1.7.0..v1.7.2`).

Added

- On-chain upgrade handler`v1.7.2` wired and registered; migrations only, no store key changes (app/upgrades/v1_7_2/upgrade.go; app/app.go).
- Supernode account history recorded on register/update (proto/lumera/supernode/supernode_account_history.proto; x/supernode/v1/keeper/msg_server_update_supernode.go).
- Supernode messages support`p2p_port` (update and register) with keeper handling (proto/lumera/supernode/tx.proto; x/supernode/v1/keeper/msg_server_update_supernode.go; x/supernode/v1/keeper/msg_server_register_supernode.go).
- Action metadata adds`public` flag (proto/lumera/action/metadata.proto; x/action/v1/types/metadata.pb.go).

Changed

- Supernode type field`version` renamed to`note` in chain types and handlers (proto/lumera/supernode/super_node.proto; x/supernode/v1/types/super_node.go; x/supernode/v1/types/message_update_supernode.go).
- Supernode state transitions and event attributes standardized across keeper and msg servers (x/supernode/v1/keeper/supernode.go; x/supernode/v1/keeper/hooks.go; x/supernode/v1/types/events.go).

Fixed

- Supernode staking hooks correctness for eligibility-driven activation/stop (x/supernode/v1/keeper/hooks.go).
- Action fee distribution panic avoided (x/action/v1/module/module.go).

CLI

- Supernode CLI:
  - Added query:`get-supernode-by-address [supernode-address]` (x/supernode/v1/module/autocli.go).
  - Standardized command names:`get-supernode`,`list-supernodes`,`get-top-supernodes-for-block` (x/supernode/v1/module/autocli.go).
  - `update-supernode` switched positional arg from`version` to`note`; added optional`--p2p-port` flag.`register-supernode` also supports optional`--p2p-port` (x/supernode/v1/module/autocli.go).
- Action CLI:
  - Added`action [action-id]` query (x/action/v1/module/autocli.go).
  - `finalize-action` now takes`[action-id] [action-type] [metadata]` (x/action/v1/module/autocli.go).
- Testnet CLI: default denom set to`ulume` for gas price and initial balances (cmd/lumera/cmd/testnet.go).

---

## 1.7.0

Added

- SuperNode Dual-Source Stake Validation: eligibility can be met by self-delegation plus supernode-account delegation (x/supernode/v1/keeper/supernode.go: CheckValidatorSupernodeEligibility).

Changed

- App wiring and upgrade handler for`v1.7.0` (migrations only; no store upgrades) (app/upgrades/v1_7_0/upgrade.go; app/app.go).
