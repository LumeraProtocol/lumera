# Changelog

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

Changes included since `v1.8.4` (range: `v1.8.4..HEAD`).

- Register every upgrade handler at startup (before Load) so state-sync nodes always have handlers available, even without an on-disk plan.
- Fixed x/upgrade downgrade verification panics on state-synced nodes that already applied v1.8.4 but lacked a registered handler.
- Standardised migration-only upgrades with `standardUpgradeHandler`.
- Devnet Docker tests: `network-maker-setup.sh` now provisions **multiple** network-maker accounts per validator (configurable in `config/config.json`), funds them, and writes them into generated configs for automated Network-Maker scenarios.
- Compression: Action/Sense ID generation now uses the DataDog zstd binding with bounded high-compression helpers and clearer error handling.

---

## 1.8.4

Changes included since `v1.8.0` (range: `v1.8.0..HEAD`).

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

Changes included since `v1.7.2` (range: `v1.7.2..HEAD`).

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

* Enable/disable each component via flags or environment variables when bringing up the devnet (kept generic here to avoid locking to specific flag names).
* **Optional service installers**: add-on installation toggles for**Supernode**,**Network‑Maker**, and**SN Client** (enable via flags/env when bringing up the devnet). Network‑Maker installation on a selected validator is driven by the`network-maker` flag in`validators.json`.

#### 📋 Devnet Architecture Overview

```
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

Changes included since `v1.7.0` (range: `v1.7.0..HEAD`).

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
