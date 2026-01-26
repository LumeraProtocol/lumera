# Benefits of upgrading Lumera from Cosmos SDK 0.50.x to 0.53.5

## Cosmos SDK upgrade benefits

### App wiring modernization

- **Declarative app wiring (depinject + core/appconfig)**: Replaces much of historical app.go wiring with declarative config (Go/YAML/JSON), improving modularity and reducing initialization boilerplate.
- **runtime/v2 AppBuilder support**: Enables hybrid setups (part declarative, part classic app.go), making it easier to evolve app composition over time.

### Interchain readiness

- **IBC v2 readiness**: Aligns with the IBC-Go v10 stack and its v2 features while keeping IBC v1 compatibility for existing channels.
- **Interchain feature enablement**: Paves the way for modern interchain features (ICA, IBC v2 routes, improved packet workflows) without heavy custom shims.

### Core modules and optional features

- **Module parity and compatibility**: Brings core modules (gov, staking, auth, bank, feegrant, evidence, circuit, etc.) to current versions, improving ecosystem alignment and upstream support.

#### New optional modules and features

- **x/epochs**: Standardized on-chain timers ("run logic every N period") that other modules can hook into.
- **x/protocolpool**: Separates community-pool accounting from distribution into a dedicated module account, making pool assets easier to track and manage.
- **Unordered transactions (opt-in)**: "Fire-and-forget" submission model with `timeout_timestamp` as TTL/replay protection, useful for throughput-focused clients where strict ordering is not required.

### Parameter and consensus-params cleanup

- **x/params deprecation**: `x/params` is deprecated as of SDK v0.53 (planned removal next release), so upgrading aligns Lumera with current ecosystem cleanup.
- **Consensus params migration**: The SDK calls out moving CometBFT consensus params from `x/params` to `x/consensus`; upgrade handlers should ensure `ConsensusParamsKeeper.Set()` is invoked appropriately.

### Deprecations that reduce bloat

- **x/crisis deprecation/removal**: `x/crisis` is deprecated as of SDK v0.53 (planned removal next release). Lumera removes it in the v1.10.0 upgrade, deletes its store key, and disables crisis invariants by default.

### Testing, stability, and operations

- **Testing and tooling maturity**: Updated test helpers, BaseApp options, and CLI utilities reduce local workarounds and make test scaffolding more consistent.
- **Performance and stability**: Accumulates patch-level fixes across the 0.5x series, improving reliability for production workloads and upgrade safety.
- **Operational consistency**: Keeps Lumera closer to current Cosmos SDK practices, making it easier to adopt upstream fixes and collaborate with the ecosystem.
- **Upgrade process enhancements (ADR-047)**: Upgrade module and cosmovisor flow support richer upgrade-plan information, enabling more structured and automatable upgrade operations.

### Client and indexer impact

- **Legacy tx logs removed**: SDK v0.53 no longer constructs "legacy logs" for tx query/broadcast responses, which can affect indexers or clients parsing legacy log shapes.

## IBC upgrade benefits

### Benefits of upgrading to IBC v2 (IBC-Go v10)

- **Backward compatibility**: Keep IBC v1 channels while enabling IBC v2, so Lumera can upgrade without forcing counterparties to move immediately.
- **Simplified handshakes**: Reduce connection and channel setup time, lowering relayer overhead and speeding new integrations.
- **Unified routing**: Support multiple Lumera services (Cascade, Sense, Inference, NFT metadata) over a single connection, improving composability and reducing channel sprawl.
- **Payload flexibility**: Allow app-specific encodings and multi-action workflows (e.g., payment + service request in one flow) with less protocol friction.
- **Faster feature adoption**: Enable ICA, queries, and cross-chain calls without reworking the IBC stack as new apps are added.
- **Lower maintenance risk**: Trim the IBC module stack, reducing upgrade risk during the Cosmos SDK 0.53.5 transition while improving long-term scalability.

### Benefits of upgrading to IBC-Go v10.5.0

- **Upstream fixes**: Pick up v10-series fixes and maintenance improvements, reducing the risk of carrying known IBC bugs.
- **Developer ergonomics**: Use newer helper APIs (including v1/v2 event parsing) to reduce custom code in Lumera.
- **Ecosystem alignment**: Stay on the latest v10 patch level for relayer compatibility and support.
- **ICA integrity hardening**: Adds extra validations for ProtoJSON-encoded ICA packets; release notes flag this as state-breaking and best rolled out via a coordinated chain upgrade.

## Wasm upgrade benefits

### Benefits of upgrading wasmd from 0.55 to 0.61.6

- **Ecosystem alignment**: Match current CosmWasm/wasmd releases commonly used alongside Cosmos SDK 0.53.5.
- **Consensus-sensitive VM upgrade**: wasmvm minor version bumps are generally consensus-breaking per wasmd compatibility notes, so this is a deliberate chain-level upgrade rather than a routine dependency bump.
- **Contract compatibility**: The contract-host interface has been stable since CosmWasm 1.0; CosmWasm 2.0 contracts remain compatible at the Wasm interface level (capabilities gated via feature flags).
- **Operational tuning hooks**: wasmd exposes knobs like `memory_cache_size` (module caching for faster instantiation) and `query_gas_limit` for performance and safety tradeoffs.
- **Security posture**: Includes fixes for consensus-critical issues (for example, non-deterministic crashes during wasm execution simulation patched in wasmvm/wasmd), strengthening chain safety.

### Benefits of upgrading wasmvm to 3.0.2

- **Runtime stability**: Pick up wasmvm bug fixes and runtime hardening from the 3.x line.
- **Compatibility**: Keep contract execution compatible with modern wasm tooling and wasmd expectations.
- **Operational reliability**: Reduce risk of edge-case failures in contract execution and state sync workflows.

## Notable changes in Lumera

- **IBC-Go v10.5.0 alignment**: Updated IBC dependencies and test utilities to the v10 line, including v2 packet/event handling and mocks.
- **Wasm stack refresh**: Bumped wasmd/wasmvm to the current compatible versions, keeping CosmWasm integration aligned with the SDK upgrade.
- **Test harness updates**: Ported and adjusted wasm and IBC testing helpers to match newer SDK/IBC behaviors, reducing drift from upstream.
- **Crisis module removal**: The v1.10.0 upgrade drops `x/crisis`, deletes its store key, and removes invariant checks via the crisis module.
