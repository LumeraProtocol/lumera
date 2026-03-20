# EVM Integration — Test Inventory

Complete test catalog for Lumera's Cosmos EVM integration.
See [main.md](main.md) for architecture, app changes, and operational details.

---

## Executive Summary

Lumera ships **~398 EVM-related tests** spanning unit, integration, and devnet levels — the most comprehensive pre-mainnet EVM test suite in the Cosmos ecosystem. For context:

- **Evmos** — the first Cosmos EVM chain — launched mainnet with primarily unit tests and a handful of end-to-end scripts; their integration test suite was built incrementally*after* mainnet issues surfaced (e.g., the zero-base-fee spam incident).
- **Kava** — relied heavily on simulation tests and manual QA for their EVM launch; structured integration tests came later.
- **Cronos** — forked Ethermint and inherited its test base but added few chain-specific integration tests before launch.

Lumera's suite goes beyond any of these baselines **before** mainnet:

| Capability                                                                       | Lumera                                  | Typical Cosmos EVM chain at launch        |
| -------------------------------------------------------------------------------- | --------------------------------------- | ----------------------------------------- |
| Dual-route ante handler tests (EVM + Cosmos path)                                | 28 unit + 3 integration                 | Rarely tested separately                  |
| App-side mempool (ordering, nonce gaps, replacement, capacity, WS subscriptions) | 12 integration                          | None (relies on CometBFT mempool)         |
| Async broadcast queue (deadlock prevention)                                      | 4 unit                                  | Not applicable (novel to Lumera)          |
| JSON-RPC batching, persistence across restart                                    | 23 integration                          | Basic RPC smoke tests                     |
| ERC20/IBC middleware (v1 + v2 stacks)                                            | 7 integration + 14 unit (policy)        | Partial or post-launch                    |
| Precisebank (6↔18 decimal bridge)                                               | 39 unit + 6 integration                 | Not applicable (novel to Lumera)          |
| Feemarket (EIP-1559)                                                             | 9 unit + 8 integration                  | Inherited from upstream, rarely augmented |
| Precompile coverage (9 precompiles + gas metering + action + supernode modules)   | 29 integration                          | Smoke-level                               |
| Account migration (coin-type 118→60)                                            | 102 unit + 14 integration + devnet tool | Not applicable (novel to Lumera)          |
| OpenRPC discovery + spec sync                                                    | 15 unit + 2 integration                 | No chain has this                         |
| WebSocket subscriptions (newHeads, logs, pending)                                | 4 integration                           | Untested or manual                        |
| Devnet multi-validator E2E                                                       | 12+ devnet tests                        | Manual or ad-hoc scripts                  |

Three areas are **unique to Lumera** with no equivalent in any other Cosmos EVM chain: the async broadcast queue (solving the CometBFT/EVM mempool deadlock), the precisebank 6↔18 decimal bridge, and the full account migration module. Each has dedicated test coverage.

All three previously identified critical test gaps (mempool capacity pressure, batch JSON-RPC, WebSocket subscriptions) have been closed.

---

## Test Coverage Assessment

### Coverage by area

| Category              | Area                                 | Tests                                                                   | Coverage quality                                                                                                                                                                                                                                                                    |
| --------------------- | ------------------------------------ | ----------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Unit**        | app/feemarket                        | 9                                                                       | Excellent — params validation, base fee calculation, begin/end block, GRPC queries                                                                                                                                                                                                 |
| **Unit**        | app/precisebank                      | 39                                                                      | Excellent — invariants, error parity with bank, mint/burn, lifecycle, permissions, types                                                                                                                                                                                           |
| **Unit**        | app/evm/ante                         | 28                                                                      | Excellent — path routing, authz limits, nonce, gas, sig verification, mono decorator, genesis skip, fee checker                                                                                                                                                                    |
| **Unit**        | app/evm\_broadcast, app/evm\_mempool | 14                                                                      | High — async broadcast queue, dedupe, re-entry hazard, pending tx listener, queue full/panic recovery, partial failure attempts all txs, broadcastEVMTransactionsSync regression (all txs attempted on failure)                                                                     |
| **Unit**        | app/evm, app/evm/config              | 10                                                                      | High — genesis defaults, module order, permissions, precompiles, preinstalls, static config                                                                                                                                                                                        |
| **Unit**        | app/evm\_erc20\_policy               | 14                                                                      | High — 3 modes, base denom + exact ibc/ allowlist CRUD, init defaults, governance msg handler                                                                                                                                                                                      |
| **Unit**        | app/ibc\_erc20                       | 1                                                                       | Low — wiring verification only; integration tests cover functional paths                                                                                                                                                                                                           |
| **Unit**        | app/statedb, app/blocked, app/proto  | 5                                                                       | Medium — revert-to-snapshot events, blocked addresses, proto bridge, amino codec                                                                                                                                                                                                   |
| **Unit**        | app/openrpc, tools/openrpcgen        | 16                                                                      | High — spec validation, HTTP serving, CORS origin filtering, code generator, namespace registration, proxy client timeout guard                                                                                                                                                    |
| **Unit**        | app/evm\_jsonrpc\_ratelimit          | 25                                                                      | High — right-to-left XFF parsing with trusted-hop skipping (P1 spoof prevention), multi-hop chain scenarios, production shutdown double-close guard exercising real App fields (P2), CIDR parsing, peer IP extraction                                                              |
| **Unit**        | x/evmigration                        | 114                                                                     | Excellent — auth/bank/staking/distribution/authz/feegrant/supernode/claim/action migration, validator migration (including evidence, metrics stale-key deletion, account history, third-party withdraw addr preservation, operator cross-validator delegation re-keying), signature verification, genesis, queries, params, message validation, rate limiting, pre-checks |
|                       |                                      |                                                                         |                                                                                                                                                                                                                                                                                     |
| **Integration** | evm/feemarket                        | 8                                                                       | Excellent — fee history, receipt gas price, reward percentiles, gas price, type-2 formula, reject below base, multi-block progression                                                                                                                                              |
| **Integration** | evm/precisebank                      | 6                                                                       | High — transfer/send split matrix, burn/mint workflow, fractional balance queries, remainder persistence, module account invariant                                                                                                                                                 |
| **Integration** | evm/ante                             | 3                                                                       | Medium — authz generic grant reject/allow, cosmos tx fee enforcement                                                                                                                                                                                                               |
| **Integration** | evm/jsonrpc                          | 23                                                                      | Very high — basic methods, backend methods, receipts, logs, mixed blocks, tx ordering, block lookup, persistence across restart, OpenRPC endpoint, account state, indexer disabled, batch requests                                                                                 |
| **Integration** | evm/precompiles                      | 29                                                                      | High — bank, staking, distribution, gov, bech32, p256, slashing (params, signing infos, unjail), ICS20 (denoms, denomHash, denom), action (getParams, getActionFee, getActionsByState, getActionsByCreator, requestCascade bad-sig, approveAction non-existent), supernode (getParams, listSuperNodes, getTopSuperNodesForBlock, register tx, reportMetrics tx, reportMetrics auth), delegate tx, withdraw address tx, gas metering accuracy (6 precompiles), estimate-vs-actual |
| **Integration** | evm/mempool                          | 12                                                                      | High — fee priority ordering, contention ordering, nonce gap promotion, pending subscription, disabled mode, nonce replacement, capacity overflow, rapid replacement race, newHeads/logs WS subscriptions                                                                          |
| **Integration** | evm/contracts                        | 15                                                                      | High — deploy/call/revert/persistence, CALL, DELEGATECALL, CREATE2, STATICCALL, code + storage persistence across restart, EVM state preservation across restart, concurrent mixed operations, ERC20 approve/allowance/transferFrom, contract→precompile proxy (action + supernode) |
| **Integration** | evm/ibc                              | 7                                                                       | High — registration on recv, disabled skip, invalid receiver, denom collision, round-trip transfer, secondary denom, burn-back                                                                                                                                                     |
| **Integration** | evm/vm                               | 12                                                                      | High — params, address conversion, account queries (hex/bech32), balance compat, storage key format, code/storage match JSON-RPC, historical nonce/code/storage snapshots, ERC20 balance                                                                                           |
| **Integration** | evmigration                          | 14                                                                      | High — claim legacy account (success, disabled, already migrated, same address, invalid sig, validator rejected, multi-denom, delayed vesting, account removal, validator-first after validator migration), migrate validator (success, not validator), queries                    |
|                       |                                      |                                                                         |                                                                                                                                                                                                                                                                                     |
| **Devnet**      | devnet/evm                           | 8                                                                       | High — basic methods, namespace exposure, fee market active, send raw tx, tx by hash, nonce increment, block lookup, cross-peer visibility                                                                                                                                         |
| **Devnet**      | devnet/ports                         | 2                                                                       | Medium — required ports accessible, JSON-RPC CORS MetaMask headers                                                                                                                                                                                                                 |
| **Devnet**      | devnet/evmigration                   | (tool)                                                                  | Standalone binary: prepare legacy activity (incl. cross-account dependency chains: withdraw-address chains, authz+feegrant overlap, redelegation+withdraw combos, all-validator delegations), migrate accounts, migrate validators, **migrate-all** (interleaved random order), verify (incl. JSON-RPC chain ID: `eth_chainId` + `net_version`) |
| **Devnet**      | devnet/ibc                           | 1                                                                       | Low — basic IBC connectivity                                                                                                                                                                                                                                                       |
| **Devnet**      | devnet/version                       | 1                                                                       | Low — binary version mode check                                                                                                                                                                                                                                                    |
|                       |                                      |                                                                         |                                                                                                                                                                                                                                                                                     |
|                       | **Totals**                     | **Unit: ~244 · Integration: ~125 · Devnet: 12+ · Total: ~406** |                                                                                                                                                                                                                                                                                     |

### Gaps and next steps

**Moderate test gaps** — all previously moderate gaps have been addressed:

- ~~Precompile gas metering accuracy validation~~ — Covered by`PrecompileGasMeteringAccuracy` and`PrecompileGasEstimateMatchesActual`
- ~~Multi-validator EVM consensus scenarios~~ — Single-node integration framework validates cross-block state consistency; multi-validator coverage deferred to devnet systemtests
- ~~Chain upgrade with EVM state preservation~~ — Covered by`TestEVMStatePreservationAcrossRestart`
- ~~Concurrent operation race condition detection~~ — Covered by`TestConcurrentMixedEVMOperations`
- ~~ERC20 allowance/transferFrom/approve flows~~ — Covered by`TestERC20ApproveAllowanceTransferFrom`

**Recommended next steps** — see [Recommended Next Steps](#recommended-next-steps) below.

### Key architectural strengths

1. **Async broadcast queue** — Novel solution to the cosmos/evm mempool deadlock. Decouples txpool promotion from CometBFT`CheckTx` via bounded channel + single background worker.
2. **Min gas price floor** — Prevents base fee decay to zero on quiet chains (Evmos experienced spam attacks from this).
3. **Tracing + rate limiting already implemented** — Runtime-configurable EVM tracing and app-layer JSON-RPC per-IP rate limiting are integrated now, not deferred.
4. **Governance-controlled IBC voucher ERC20 policy** — Three-mode policy (`all`/`allowlist`/`none`) for auto-registration risk control.
5. **Dual CosmWasm + EVM runtime** — Unique among Cosmos EVM chains.
6. **IBC v1 + v2 ERC20 middleware** — Both transfer stack versions have ERC20 token registration middleware.
7. **OpenRPC discovery** — Machine-readable API spec with build-time synchronization. Unique across all Cosmos EVM chains.
8. **Account migration module** — Purpose-built`x/evmigration` for coin-type-118-to-60 transition with dual-signature verification and atomic state migration across 8 SDK modules.

### Bottom line

Lumera's EVM integration is **architecturally excellent and feature-complete** for its current scope, and it is already ahead in several operator-facing areas (tracing, rate limiting, governance-controlled ERC20 voucher policy, and mempool hardening). The main remaining gap versus mature production Cosmos EVM chains is **final operational hardening and ecosystem surface**: security audit, CORS/namespace lock-down playbooks, monitoring, and external block explorer.

---

## Unit Tests

### A) App wiring/config/genesis and command-level tests

Purpose: verifies that EVM runtime/CLI wiring is correctly initialized (genesis overrides, module order, precompiles, mempool, listeners, and command defaults).
Primary files:

- `app/evm_test.go`
- `app/evm_static_precompiles_test.go`
- `app/blocked_addresses_test.go`
- `app/evm_mempool_test.go`
- `app/evm_mempool_reentry_test.go`
- `app/evm_broadcast_test.go`
- `app/pending_tx_listener_test.go`
- `app/ibc_erc20_middleware_test.go`
- `app/ibc_test.go`
- `app/vm_preinstalls_test.go`
- `app/amino_codec_test.go`
- `app/statedb_events_test.go`
- `app/evm_erc20_policy.go`
- `app/evm_erc20_policy_msg.go`
- `app/evm_erc20_policy_test.go`
- `proto/lumera/erc20policy/tx.proto`
- `x/erc20policy/types/tx.pb.go`
- `x/erc20policy/types/codec.go`
- `cmd/lumera/cmd/config_test.go`
- `cmd/lumera/cmd/root_test.go`
- `app/upgrades/upgrades_test.go`
- `app/upgrades/v1_12_0/upgrade_test.go`

| Test                                                                  | Description                                                                                                                                                                                                                     |
| --------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `TestRegisterEVMDefaultGenesis`                                     | Verifies EVM-related modules are registered and expose Lumera-specific default genesis values.                                                                                                                                  |
| `TestEVMModuleOrderAndPermissions`                                  | Verifies module order constraints and module-account permissions for EVM modules.                                                                                                                                               |
| `TestEVMStoresAndModuleAccountsInitialized`                         | Verifies EVM KV/transient stores and module accounts are initialized in app startup.                                                                                                                                            |
| `TestEVMStaticPrecompilesConfigured`                                | Verifies expected static precompiles are configured on the EVM keeper.                                                                                                                                                          |
| `TestBlockedAddressesMatrix`                                        | Verifies blocked-address set contains expected module/precompile addresses.                                                                                                                                                     |
| `TestPrecompileSendRestriction`                                     | Verifies bank send restriction blocks sends to EVM precompile addresses.                                                                                                                                                        |
| `TestEVMMempoolWiringOnAppStartup`                                  | Verifies app-side EVM mempool wiring occurs at startup with expected handlers.                                                                                                                                                  |
| `TestEVMMempoolReentrantInsertBlocks`                               | Demonstrates mutex re-entry hazard that the async broadcast queue prevents.                                                                                                                                                     |
| `TestConfigureEVMBroadcastOptionsFromAppOptions`                    | Verifies broadcast debug flag parsing from app options (bool, string, nil).                                                                                                                                                     |
| `TestEVMTxBroadcastDispatcherDedupesQueuedAndInFlight`              | Verifies dispatcher deduplicates queued and in-flight tx hashes.                                                                                                                                                                |
| `TestEVMTxBroadcastDispatcherQueueFullReleasesPending`              | Verifies queue-full path releases pending hash reservations.                                                                                                                                                                    |
| `TestEVMTxBroadcastDispatcherReleasesPendingAfterProcessError`      | Verifies pending hashes are released after broadcast process errors.                                                                                                                                                            |
| `TestEVMTxBroadcastDispatcherEnqueueRemainsNonBlocking`             | Verifies enqueue does not block while worker is processing.                                                                                                                                                                     |
| `TestBroadcastEVMTxFromFieldRecovery`                               | Regression guard:`FromEthereumTx` leaves `From` empty; `FromSignedEthereumTx` recovers the sender.                                                                                                                        |
| `TestRegisterPendingTxListenerFanout`                               | Verifies registered pending-tx listeners are invoked for each pending hash event.                                                                                                                                               |
| `TestIBCERC20MiddlewareWiring`                                      | Verifies IBC transfer stack includes ERC20 middleware wiring in app composition.                                                                                                                                                |
| `TestIsInterchainAccount`                                           | Verifies ICA account type detection helper behavior.                                                                                                                                                                            |
| `TestIsInterchainAccountAddr`                                       | Verifies ICA detection by address lookup through account keeper.                                                                                                                                                                |
| `TestEVMAddPreinstallsMatrix`                                       | Verifies preinstall contract registration matrix in VM keeper setup paths.                                                                                                                                                      |
| `TestRegisterLumeraLegacyAminoCodecEnablesEthSecp256k1StdSignature` | Verifies legacy Amino registration covers eth_secp256k1 so SDK ante tx-size signature marshaling does not panic.                                                                                                                |
| `TestInitAppConfigEVMDefaults`                                      | Verifies default app config enables EVM/JSON-RPC values expected by Lumera.                                                                                                                                                     |
| `TestNeedsConfigMigration_LegacyConfig`                             | Empty Viper (pre-EVM app.toml with no EVM sections) triggers config migration. (Bug #19)                                                                                                                                       |
| `TestNeedsConfigMigration_UpstreamDefault`                          | Upstream cosmos/evm default chain ID (262144) triggers config migration even when other sections exist. (Bug #19)                                                                                                               |
| `TestNeedsConfigMigration_PartialManualEdit`                        | Correct evm-chain-id but missing [json-rpc] section still triggers migration. (Bug #19)                                                                                                                                         |
| `TestNeedsConfigMigration_MissingLumeraSection`                     | Correct [evm] and [json-rpc] but missing [lumera.*] section triggers migration. (Bug #19)                                                                                                                                       |
| `TestNeedsConfigMigration_OperatorDisabledJSONRPC`                   | Operator who explicitly set `json-rpc.enable = false` does NOT trigger migration — choice is respected. (Bug #19)                                                                                                               |
| `TestNeedsConfigMigration_FullyMigrated`                            | Fully migrated config with all sentinel keys set does NOT trigger migration. (Bug #19)                                                                                                                                          |
| `TestMigrateAppConfig_LegacyTomlOnDisk`                             | Full migration flow: writes legacy app.toml, runs migrator, verifies disk and in-memory Viper state contain correct EVM config while preserving operator settings. (Bug #19)                                                    |
| `TestNewRootCmdStartWiresEVMFlags`                                  | Verifies start/root command exposes key EVM JSON-RPC flags.                                                                                                                                                                     |
| `TestNewRootCmdDefaultKeyTypeOverridden`                            | Verifies root command default key algorithm is overridden to `eth_secp256k1`.                                                                                                                                                 |
| `TestRevertToSnapshot_ProcessedEventsInvariant`                     | Adapted from cosmos/evm v0.6.0: verifies StateDB event-tracking invariant after snapshot reverts during precompile calls.                                                                                                       |
| `TestERC20Policy_DefaultModeIsAllowlist`                            | Verifies default policy mode is "allowlist" when no mode is set in KV store.                                                                                                                                                    |
| `TestERC20Policy_AllMode_DelegatesToInner`                          | "all" mode delegates `OnRecvPacket` unconditionally to inner keeper.                                                                                                                                                          |
| `TestERC20Policy_NoneMode_SkipsRegistration`                        | "none" mode returns original ack without delegating for unregistered IBC denoms.                                                                                                                                                |
| `TestERC20Policy_NoneMode_PassesThroughNonIBC`                      | Non-IBC denoms always pass through regardless of mode.                                                                                                                                                                          |
| `TestERC20Policy_NoneMode_PassesThroughAlreadyRegistered`           | Already-registered IBC denoms pass through even in "none" mode.                                                                                                                                                                 |
| `TestERC20Policy_AllowlistMode_BlocksUnlisted`                      | "allowlist" mode blocks unlisted IBC denoms.                                                                                                                                                                                    |
| `TestERC20Policy_AllowlistMode_AllowsListed`                        | "allowlist" mode allows governance-approved denoms.                                                                                                                                                                             |
| `TestERC20Policy_PassthroughMethods`                                | `OnAcknowledgementPacket`, `OnTimeoutPacket`, `Logger` pass through to inner keeper.                                                                                                                                      |
| `TestERC20Policy_AllowlistCRUD`                                     | Allowlist add/remove/list operations work correctly.                                                                                                                                                                            |
| `TestERC20Policy_AllowlistMode_AllowsBaseDenom`                     | "allowlist" mode allows IBC denoms whose base denom (e.g. "uatom") is in the base denom allowlist.                                                                                                                              |
| `TestERC20Policy_AllowlistMode_BlocksUnlistedBaseDenom`             | "allowlist" mode blocks IBC denoms whose base denom is not in either allowlist.                                                                                                                                                 |
| `TestERC20Policy_BaseDenomCRUD`                                     | Base denom allowlist add/remove/list operations work correctly.                                                                                                                                                                 |
| `TestERC20Policy_InitDefaults`                                      | `initERC20PolicyDefaults` sets mode to "allowlist" and populates `DefaultAllowedBaseDenoms`; is idempotent.                                                                                                                 |
| `TestERC20PolicyMsg_SetRegistrationPolicy`                          | Governance message handler: authority validation, mode changes, ibc denom add/remove, base denom add/remove, error cases.                                                                                                       |
| `TestV1120SkipsEVMInitGenesis`                                      | Verifies the v1.12.0 upgrade handler pre-populates `fromVM` with EVM module consensus versions to skip `InitGenesis`, preventing upstream `DefaultParams().EvmDenom = "aatom"` from polluting the EVM coin info KV store. |
| `TestV1120InitializesERC20ParamsWhenInitGenesisIsSkipped`           | Verifies the v1.12.0 upgrade handler backfills `x/erc20` default params after skipping `InitGenesis`, so upgraded chains do not come up with `EnableErc20=false` and `PermissionlessRegistration=false`.                |

### B) EVM ante unit tests (`app/evm`)

Purpose: verifies dual-route ante behavior and decorator-level Ethereum/Cosmos transaction validation logic.
Primary files:

- `app/evm/ante_decorators_test.go`
- `app/evm/ante_fee_checker_test.go`
- `app/evm/ante_gas_wanted_test.go`
- `app/evm/ante_handler_test.go`
- `app/evm/ante_min_gas_price_test.go`
- `app/evm/ante_mono_decorator_test.go`
- `app/evm/ante_nonce_test.go`
- `app/evm/ante_sigverify_test.go`

| Test                                                            | Description                                                                              |
| --------------------------------------------------------------- | ---------------------------------------------------------------------------------------- |
| `TestRejectMessagesDecorator`                                 | Verifies Cosmos ante path rejects blocked message types (for example MsgEthereumTx).     |
| `TestAuthzLimiterDecorator`                                   | Verifies authz limiter blocks grants for restricted message types.                       |
| `TestDynamicFeeCheckerMatrix`                                 | Verifies dynamic fee checker decisions across representative gas-fee inputs.             |
| `TestGasWantedDecoratorMatrix`                                | Verifies gas-wanted accounting updates are applied correctly per tx path.                |
| `TestNewAnteHandlerRequiredDependencies`                      | Verifies NewAnteHandler fails fast when required keeper/dependency inputs are missing.   |
| `TestNewAnteHandlerRoutesEthereumExtension`                   | Verifies extension option routes Ethereum txs to EVM ante chain.                         |
| `TestNewAnteHandlerRoutesDynamicFeeExtensionToCosmosPath`     | Verifies dynamic-fee extension routes tx to Cosmos ante path.                            |
| `TestNewAnteHandlerDefaultRouteWithoutExtension`              | Verifies txs without EVM extension use default Cosmos ante path.                         |
| `TestNewAnteHandlerPendingTxListenerTriggeredForEVMCheckTx`   | Verifies pending-tx listener fires for EVM CheckTx path.                                 |
| `TestNewAnteHandlerPendingTxListenerNotTriggeredOnCosmosPath` | Verifies pending-tx listener does not trigger on Cosmos ante path.                       |
| `TestMinGasPriceDecoratorMatrix`                              | Verifies min gas price decorator behavior across accepted/rejected fee cases.            |
| `TestEVMMonoDecoratorMatrix`                                  | Verifies EVM mono decorator baseline validation matrix.                                  |
| `TestEVMMonoDecoratorRejectsInvalidTxType`                    | Verifies EVM mono decorator rejects unsupported tx types.                                |
| `TestEVMMonoDecoratorRejectsNonEthereumMessage`               | Verifies EVM mono decorator rejects non-Ethereum message payloads.                       |
| `TestEVMMonoDecoratorRejectsSenderMismatch`                   | Verifies EVM mono decorator rejects signer/from mismatches.                              |
| `TestEVMMonoDecoratorRejectsInsufficientBalance`              | Verifies EVM mono decorator rejects txs with insufficient sender balance for fees/value. |
| `TestEVMMonoDecoratorRejectsNonEOASender`                     | Verifies EVM mono decorator rejects non-EOA senders where required.                      |
| `TestEVMMonoDecoratorAllowsDelegatedCodeSender`               | Verifies delegated-code sender case is accepted when rules permit it.                    |
| `TestEVMMonoDecoratorRejectsGasFeeCapBelowBaseFee`            | Verifies tx is rejected when fee cap is below current base fee.                          |
| `TestIncrementNonceMatrix`                                    | Verifies nonce increment semantics across successful tx paths.                           |
| `TestSigVerificationGasConsumerMatrix`                        | Verifies signature verification gas charging across key/signature types.                 |

### C) EVM module/config guard and genesis tests (`app/evm`)

Purpose: verifies EVM module registration/genesis defaults and production guardrails around test-only global resets.
Primary files:

- `app/evm/config_modules_genesis_test.go`
- `app/evm/prod_guard_test.go`

| Test                                       | Description                                                                                                                                                                      |
| ------------------------------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `TestConfigureNoOp`                      | Verifies `Configure()` remains a safe no-op with current x/vm global config lifecycle.                                                                                         |
| `TestProvideCustomGetSigners`            | Verifies custom signer provider exposes MsgEthereumTx custom get-signer registration.                                                                                            |
| `TestLumeraGenesisDefaults`              | Verifies Lumera EVM and feemarket genesis defaults match expected chain settings.                                                                                                |
| `TestRegisterModulesMatrix`              | Verifies CLI-side registration map includes all EVM modules and wrappers.                                                                                                        |
| `TestUpstreamDefaultEvmDenomIsNotLumera` | Documents that cosmos/evm v0.6.0 `DefaultParams().EvmDenom` = `"aatom"` (not `"ulume"`), validating why the v1.12.0 upgrade handler must skip InitGenesis for EVM modules. |
| `TestResetGlobalStateRequiresTestTag`    | Verifies reset helper is guarded and requires `test` build tag.                                                                                                                |
| `TestSetKeeperDefaultsRequiresTestTag`   | Verifies keeper-default mutation helper is guarded behind `test` tag.                                                                                                          |

### D) Fee market unit tests

Purpose: verifies feemarket arithmetic, lifecycle hooks, query APIs, and type validation invariants.
Primary files:

- `app/feemarket_test.go`
- `app/feemarket_types_test.go`

| Test                                               | Description                                                                         |
| -------------------------------------------------- | ----------------------------------------------------------------------------------- |
| `TestFeeMarketCalculateBaseFee`                  | Verifies base-fee calculation matrix across target gas and min-gas-price scenarios. |
| `TestFeeMarketBeginBlockUpdatesBaseFee`          | Verifies BeginBlock updates base fee from prior gas usage inputs.                   |
| `TestFeeMarketEndBlockGasWantedClamp`            | Verifies EndBlock clamps block gas wanted using configured multiplier logic.        |
| `TestFeeMarketQueryMethods`                      | Verifies keeper query methods return consistent params/base-fee/block-gas values.   |
| `TestFeeMarketUpdateParamsAuthority`             | Verifies only authorized authority can update feemarket params.                     |
| `TestFeeMarketGRPCQueryClient`                   | Verifies gRPC query client paths for feemarket endpoints.                           |
| `TestFeeMarketTypesParamsValidateMatrix`         | Verifies feemarket params validation rules across valid/invalid combinations.       |
| `TestFeeMarketTypesMsgUpdateParamsValidateBasic` | Verifies basic validation for fee market MsgUpdateParams messages.                  |
| `TestFeeMarketTypesGenesisValidateMatrix`        | Verifies genesis validation matrix for feemarket state.                             |

### E) Precisebank unit tests

Purpose: verifies precisebank fractional accounting, bank parity behavior, mint/burn transitions, and type-level invariants.
Primary files:

- `app/precisebank_test.go`
- `app/precisebank_fractional_test.go`
- `app/precisebank_mint_burn_behavior_test.go`
- `app/precisebank_mint_burn_parity_test.go`
- `app/precisebank_types_test.go`

| Test                                                                    | Description                                                                              |
| ----------------------------------------------------------------------- | ---------------------------------------------------------------------------------------- |
| `TestPreciseBankSplitAndRecomposeBalance`                             | Verifies extended balance splits into integer+fractional parts and recomposes correctly. |
| `TestPreciseBankSendExtendedCoinBorrowCarry`                          | Verifies fractional borrow/carry behavior during extended-denom transfers.               |
| `TestPreciseBankMintTransferBurnRestoresReserveAndRemainder`          | Verifies reserve/remainder bookkeeping round-trips after mint-transfer-burn sequence.    |
| `TestPreciseBankSendCoinsErrorParityWithBank`                         | Verifies send error messages/parity match bank keeper behavior.                          |
| `TestPreciseBankSendCoinsFromModuleToAccountBlockedRecipientParity`   | Verifies blocked-recipient behavior matches bank keeper for module-to-account sends.     |
| `TestPreciseBankSendCoinsFromModuleToAccountMissingModulePanicParity` | Verifies missing sender module panic parity with bank keeper.                            |
| `TestPreciseBankSendCoinsFromAccountToModuleMissingModulePanicParity` | Verifies missing recipient module panic parity with bank keeper.                         |
| `TestPreciseBankSendCoinsFromModuleToModuleMissingModulePanicParity`  | Verifies module-to-module missing-account panic parity with bank keeper.                 |
| `TestPreciseBankSendCoinsFromModuleToModuleErrorParityWithBank`       | Verifies module-to-module error-path parity with bank keeper.                            |
| `TestPreciseBankSendCoinsFromAccountToPrecisebankModuleBlocked`       | Verifies direct sends to precisebank module account are blocked as expected.             |
| `TestPreciseBankSendCoinsFromPrecisebankModuleToAccountBlocked`       | Verifies restricted sends from precisebank module account are blocked as expected.       |
| `TestPreciseBankMintCoinsToPrecisebankModulePanic`                    | Verifies minting directly into precisebank module account triggers expected panic.       |
| `TestPreciseBankBurnCoinsFromPrecisebankModulePanic`                  | Verifies burning directly from precisebank module account triggers expected panic.       |
| `TestPreciseBankRemainderAmountLifecycle`                             | Verifies remainder amount updates correctly through lifecycle operations.                |
| `TestPreciseBankInvalidRemainderAmountPanics`                         | Verifies invalid remainder values trigger expected panic behavior.                       |
| `TestPreciseBankReserveAddressHiddenForExtendedDenom`                 | Verifies reserve internals are hidden behind extended-denom abstractions.                |
| `TestPreciseBankGetBalanceAndSpendableCoin`                           | Verifies balance/spendable responses for extended-denom accounts.                        |
| `TestPreciseBankSetGetFractionalBalanceMatrix`                        | Verifies set/get fractional balance matrix across representative values.                 |
| `TestPreciseBankSetFractionalBalanceEmptyAddrPanics`                  | Verifies empty address input panics in fractional balance setter.                        |
| `TestPreciseBankSetFractionalBalanceZeroDeletes`                      | Verifies setting zero fractional balance removes persisted entry.                        |
| `TestPreciseBankIterateFractionalBalancesAndAggregateSum`             | Verifies iteration and aggregate sum over fractional balance entries.                    |
| `TestPreciseBankMintCoinsPermissionMatrix`                            | Verifies mint permission checks by module/denom path.                                    |
| `TestPreciseBankBurnCoinsPermissionMatrix`                            | Verifies burn permission checks by module/denom path.                                    |
| `TestPreciseBankMintExtendedCoinStateTransitions`                     | Verifies state transitions for minting extended-denom coins.                             |
| `TestPreciseBankBurnExtendedCoinStateTransitions`                     | Verifies state transitions for burning extended-denom coins.                             |
| `TestPreciseBankMintCoinsStateMatrix`                                 | Verifies mint state matrix across integer/fractional edge cases.                         |
| `TestPreciseBankMintCoinsMissingModulePanicParity`                    | Verifies missing-module panic parity for mint path.                                      |
| `TestPreciseBankBurnCoinsMissingModulePanicParity`                    | Verifies missing-module panic parity for burn path.                                      |
| `TestPreciseBankMintCoinsInvalidCoinsErrorParity`                     | Verifies invalid coin error parity for mint path.                                        |
| `TestPreciseBankBurnCoinsInvalidCoinsErrorParity`                     | Verifies invalid coin error parity for burn path.                                        |
| `TestPreciseBankTypesConversionFactorInvariants`                      | Verifies conversion factor constants and invariants for precisebank math.                |
| `TestPreciseBankTypesNewFractionalBalance`                            | Verifies constructor behavior for fractional balance type.                               |
| `TestPreciseBankTypesFractionalBalanceValidateMatrix`                 | Verifies validation matrix for single fractional balance entries.                        |
| `TestPreciseBankTypesFractionalBalancesValidateMatrix`                | Verifies validation matrix for collections of fractional balances.                       |
| `TestPreciseBankTypesFractionalBalancesSumAndOverflow`                | Verifies sum/overflow behavior in fractional balance aggregation.                        |
| `TestPreciseBankTypesGenesisValidateMatrix`                           | Verifies precisebank genesis validation matrix.                                          |
| `TestPreciseBankTypesGenesisTotalAmountWithRemainder`                 | Verifies total-amount computation with remainder in genesis state.                       |
| `TestPreciseBankTypesFractionalBalanceKey`                            | Verifies deterministic key derivation for fractional balance store entries.              |
| `TestPreciseBankTypesSumExtendedCoin`                                 | Verifies helper math for summing extended-denom coin amounts.                            |

### F) OpenRPC/generator unit tests

Purpose: verifies OpenRPC registration, embedded-spec serving semantics, CORS behavior, and spec generator output constraints expected by OpenRPC clients.
Primary files:

- `app/openrpc/openrpc_test.go`
- `app/openrpc/http_test.go`
- `tools/openrpcgen/main_test.go`

| Test                                                   | Description                                                                                         |
| ------------------------------------------------------ | --------------------------------------------------------------------------------------------------- |
| `TestDiscoverDocumentValid`                          | Verifies embedded OpenRPC JSON is valid and parseable.                                              |
| `TestEnsureNamespaceEnabled`                         | Verifies `rpc` namespace append helper is idempotent and stable.                                  |
| `TestRegisterJSONRPCNamespaceIdempotent`             | Verifies repeated JSON-RPC namespace registration is safe.                                          |
| `TestServeHTTPGet`                                   | Verifies `/openrpc.json` GET response shape/content type and CORS headers.                        |
| `TestServeHTTPHead`                                  | Verifies `/openrpc.json` HEAD behavior and headers.                                               |
| `TestServeHTTPMethodNotAllowed`                      | Verifies unsupported methods return `405` with correct `Allow` list.                            |
| `TestServeHTTPOptions`                               | Verifies CORS preflight (`OPTIONS`) returns `204` and expected CORS headers.                    |
| `TestServeHTTPCORSAllowedOrigin`                     | Verifies allowed origin from ws-origins list is echoed back in CORS header.                       |
| `TestServeHTTPCORSBlockedOrigin`                     | Verifies unlisted origin gets no `Access-Control-Allow-Origin` header.                          |
| `TestServeHTTPCORSNoOriginHeader`                    | Verifies non-browser requests (no Origin) are allowed through.                                    |
| `TestServeHTTPCORSWildcardInList`                    | Verifies `*` in origins list allows all origins.                                                |
| `TestCollectMethodsPrefersOverrideExamples`          | Verifies generator prefers curated overrides from `docs/openrpc_examples_overrides.json`.         |
| `TestAlignExampleParamNamesRemapsIndexedArgs`        | Verifies generator remaps generic `argN` names to human-readable parameter names.                 |
| `TestExampleObjectSerializesNullValue`               | Verifies generator keeps explicit `result.value: null` instead of dropping the field.             |
| `TestCollectMethodsExamplesAlwaysIncludeParamsField` | Verifies generator always emits `params` in examples (empty array when method has no parameters). |

### G) EVM migration unit tests

Purpose: validates the `x/evmigration` module — dual-signature verification, account/bank/staking/distribution/authz/feegrant/supernode/action/claim migration, preChecks, and full ClaimLegacyAccount message handler flow.
Files: `x/evmigration/keeper/verify_test.go`, `x/evmigration/keeper/migrate_test.go`, `x/evmigration/keeper/msg_server_claim_legacy_test.go`, `x/evmigration/keeper/msg_server_migrate_validator_test.go`, `x/evmigration/keeper/query_test.go`

| Test                                                             | Description                                                                                                                                              |
| ---------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `TestVerifyLegacySignature_Valid`                              | Verifies a correctly signed migration message passes verification.                                                                                       |
| `TestVerifyLegacySignature_InvalidPubKeySize`                  | Rejects public keys that are not exactly 33 bytes (compressed secp256k1).                                                                                |
| `TestVerifyLegacySignature_PubKeyAddressMismatch`              | Rejects when the public key does not derive to the claimed legacy address.                                                                               |
| `TestVerifyLegacySignature_InvalidSignature`                   | Rejects a signature produced by a different private key.                                                                                                 |
| `TestVerifyLegacySignature_WrongMessage`                       | Rejects a valid signature produced over a different new address.                                                                                         |
| `TestVerifyLegacySignature_EmptySignature`                     | Rejects a nil/empty signature.                                                                                                                           |
| `TestMigrateAuth_BaseAccount`                                  | Verifies BaseAccount removal and new account creation.                                                                                                   |
| `TestMigrateAuth_ContinuousVesting`                            | Verifies ContinuousVestingAccount parameters are captured in VestingInfo.                                                                                |
| `TestMigrateAuth_DelayedVesting`                               | Verifies DelayedVestingAccount parameters are captured in VestingInfo.                                                                                   |
| `TestMigrateAuth_PeriodicVesting`                              | Verifies PeriodicVestingAccount parameters including periods are captured.                                                                               |
| `TestMigrateAuth_PermanentLocked`                              | Verifies PermanentLockedAccount parameters are captured in VestingInfo.                                                                                  |
| `TestMigrateAuth_ModuleAccount`                                | Verifies module accounts are rejected.                                                                                                                   |
| `TestMigrateAuth_AccountNotFound`                              | Verifies error when legacy account does not exist.                                                                                                       |
| `TestMigrateAuth_NewAddressAlreadyExists`                      | Verifies existing new address account is reused.                                                                                                         |
| `TestFinalizeVestingAccount_Continuous`                        | Verifies ContinuousVestingAccount is recreated from VestingInfo.                                                                                         |
| `TestFinalizeVestingAccount_AccountNotFound`                   | Verifies error when new account does not exist at finalization.                                                                                          |
| `TestMigrateBank_WithBalance`                                  | Verifies all balances are transferred via SendCoins.                                                                                                     |
| `TestMigrateBank_ZeroBalance`                                  | Verifies SendCoins is not called when balance is zero.                                                                                                   |
| `TestMigrateBank_MultiDenom`                                   | Verifies multi-denom balances are transferred correctly.                                                                                                 |
| `TestMigrateDistribution_WithDelegations`                      | Verifies pending rewards are withdrawn for all delegations.                                                                                              |
| `TestMigrateDistribution_NoDelegations`                        | Verifies no-op when there are no delegations.                                                                                                            |
| `TestMigrateAuthz_AsGranter`                                   | Verifies grants where legacy is the granter are re-keyed.                                                                                                |
| `TestMigrateAuthz_AsGrantee`                                   | Verifies grants where legacy is the grantee are re-keyed.                                                                                                |
| `TestMigrateAuthz_NoGrants`                                    | Verifies no-op when there are no authz grants.                                                                                                           |
| `TestMigrateFeegrant_AsGranter`                                | Verifies fee allowances where legacy is the granter are re-created.                                                                                      |
| `TestMigrateFeegrant_NoAllowances`                             | Verifies no-op when there are no fee allowances.                                                                                                         |
| `TestMigrateSupernode_Found`                                   | Verifies supernode account field is updated.                                                                                                             |
| `TestMigrateSupernode_NotFound`                                | Verifies no-op when legacy is not a supernode.                                                                                                           |
| `TestMigrateActions_CreatorAndSuperNodes`                      | Verifies Creator and SuperNodes fields are updated.                                                                                                      |
| `TestMigrateActions_NoMatch`                                   | Verifies no-op when no actions reference legacy address.                                                                                                 |
| `TestMigrateClaim_Found`                                       | Verifies claim record DestAddress is updated.                                                                                                            |
| `TestMigrateClaim_NotFound`                                    | Verifies no-op when there is no claim record.                                                                                                            |
| `TestMigrateStaking_ActiveDelegations`                         | Verifies full staking migration: delegation re-keying, starting info, withdraw addr (uses origWithdrawAddr parameter, see bug #16).                      |
| `TestMigrateStaking_NoDelegations`                             | Verifies no-op when delegator has no delegations (nil origWithdrawAddr defaults to self).                                                                |
| `TestMigrateStaking_ThirdPartyWithdrawAddress`                 | Verifies third-party withdraw address is preserved via origWithdrawAddr parameter (see bug #16).                                                         |
| `TestMigrateStaking_MigratedThirdPartyWithdrawAddress`         | Verifies migrated third-party withdraw address is resolved to its new address via MigrationRecords (bug #16 regression test).                            |
| `TestPreChecks_MigrationDisabled`                              | Verifies rejection when enable_migration is false.                                                                                                       |
| `TestPreChecks_MigrationWindowClosed`                          | Verifies rejection after the configured end time.                                                                                                        |
| `TestPreChecks_BlockRateLimitExceeded`                         | Verifies rejection when per-block migration count exceeds limit.                                                                                         |
| `TestPreChecks_SameAddress`                                    | Verifies rejection when legacy and new addresses are identical.                                                                                          |
| `TestPreChecks_AlreadyMigrated`                                | Verifies a legacy address cannot be migrated twice.                                                                                                      |
| `TestPreChecks_NewAddressWasMigrated`                          | Verifies new address cannot be a previously-migrated legacy address.                                                                                     |
| `TestPreChecks_ModuleAccount`                                  | Verifies module accounts cannot be migrated.                                                                                                             |
| `TestPreChecks_LegacyAccountNotFound`                          | Verifies error when legacy account does not exist in x/auth.                                                                                             |
| `TestClaimLegacyAccount_ValidatorMustUseMigrateValidator`      | Verifies validator operators are directed to MigrateValidator.                                                                                           |
| `TestClaimLegacyAccount_InvalidSignature`                      | Verifies invalid legacy signature is rejected.                                                                                                           |
| `TestClaimLegacyAccount_Success`                               | Verifies full happy-path: preChecks, signature, migration, record, counters.                                                                             |
| `TestClaimLegacyAccount_FailAtDistribution`                    | Failure at step 1 (reward withdrawal) propagates error, no record stored.                                                                                |
| `TestClaimLegacyAccount_FailAtStaking`                         | Failure at step 2 (delegation re-keying) propagates error, no record stored.                                                                             |
| `TestClaimLegacyAccount_FailAtBank`                            | Failure at step 3b (bank transfer) after auth removal propagates error, no record stored. Critical atomicity test.                                       |
| `TestClaimLegacyAccount_FailAtAuthz`                           | Failure at step 4 (authz grant re-keying) propagates error, no record stored.                                                                            |
| `TestClaimLegacyAccount_FailAtFeegrant`                        | Failure at step 5 (feegrant migration) propagates error, no record stored.                                                                               |
| `TestClaimLegacyAccount_FailAtSupernode`                       | Failure at step 6 (supernode migration) propagates error, no record stored.                                                                              |
| `TestClaimLegacyAccount_FailAtActions`                         | Failure at step 7 (action migration) propagates error, no record stored.                                                                                 |
| `TestClaimLegacyAccount_FailAtClaim`                           | Failure at step 8 (claim migration, last before finalize) propagates error, no record stored.                                                            |
| `TestClaimLegacyAccount_WithDelegations`                       | Verifies rewards withdrawal and delegation re-keying during claim.                                                                                       |
| `TestClaimLegacyAccount_MigratedThirdPartyWithdrawAddress`     | End-to-end message-server test: third-party withdraw addr resolved to migrated destination via MigrationRecords (bug #16 regression).                    |
| `TestMigrateValidator_NotValidator`                            | Verifies rejection when legacy address is not a validator operator.                                                                                      |
| `TestMigrateValidator_UnbondingValidator`                      | Verifies rejection when validator is unbonding or unbonded.                                                                                              |
| `TestMigrateValidator_TooManyDelegators`                       | Verifies rejection when delegation records exceed MaxValidatorDelegations.                                                                               |
| `TestMigrateValidator_Success`                                 | Verifies full validator migration: commission, record, delegations, distribution, supernode, account.                                                    |
| `TestMigrateValidator_ThirdPartyWithdrawAddrPreserved`         | Verifies temporary redirect→withdraw→restore for delegators with already-migrated third-party withdraw addresses (bug #18 regression).                   |
| `TestQueryMigrationRecord_Found`                               | Verifies query returns a stored migration record.                                                                                                        |
| `TestQueryMigrationRecord_NotFound`                            | Verifies query returns empty response for unknown address.                                                                                               |
| `TestQueryMigrationRecords_Paginated`                          | Verifies paginated listing of all migration records.                                                                                                     |
| `TestQueryMigrationStats`                                      | Verifies counters and computed stats are returned.                                                                                                       |
| `TestQueryMigrationEstimate_NonValidator`                      | Verifies estimate for non-validator address with delegations.                                                                                            |
| `TestQueryMigrationEstimate_AlreadyMigrated`                   | Verifies already-migrated addresses report would_succeed=false.                                                                                          |
| `TestQueryLegacyAccounts_WithSecp256k1`                        | Verifies accounts with secp256k1 pubkeys are listed as legacy.                                                                                           |
| `TestQueryLegacyAccounts_Pagination`                           | Multi-page offset pagination: page 1 has NextKey, page 2 returns remainder without NextKey.                                                              |
| `TestQueryLegacyAccounts_Empty`                                | Empty response when no legacy accounts exist; Total=0, no NextKey.                                                                                       |
| `TestQueryLegacyAccounts_OffsetBeyondTotal`                    | Offset beyond total returns empty slice without panic.                                                                                                   |
| `TestQueryLegacyAccounts_DefaultLimit`                         | Nil pagination uses default limit (100) without panic.                                                                                                   |
| `TestQueryMigratedAccounts`                                    | Verifies paginated listing of migrated account records.                                                                                                  |
| `TestGenesis`                                                  | Full genesis round-trip: params, migration records, and counters survive InitGenesis/ExportGenesis.                                                      |
| `TestGenesis_DefaultEmpty`                                     | Default empty genesis round-trip: zero records and counters exported correctly.                                                                          |
| `TestMigrateValidator_FailAtValidatorRecord`                   | Failure at step V2 (validator record re-key) propagates error, no record/counter stored.                                                                 |
| `TestMigrateValidator_FailAtValidatorDistribution`             | Failure at step V3 (distribution re-key) propagates error, no record/counter stored.                                                                     |
| `TestMigrateValidator_FailAtValidatorDelegations`              | Failure at step V4 (delegation re-key) propagates error, no record/counter stored.                                                                       |
| `TestMigrateValidator_FailAtValidatorSupernode`                | Failure at step V5 (supernode re-key) propagates error, no record/counter stored.                                                                        |
| `TestMigrateValidator_FailAtValidatorActions`                  | Failure at step V6 (action re-key) propagates error, no record/counter stored.                                                                           |
| `TestMigrateValidator_FailAtAuth`                              | Failure at step V7 (auth migration) propagates error, no record/counter stored.                                                                          |
| `TestMigrateStaking_WithUnbondingDelegation`                   | Unbonding delegations re-keyed with queue and UnbondingId indexes.                                                                                       |
| `TestMigrateStaking_WithRedelegation`                          | Redelegations re-keyed with queue and UnbondingId indexes.                                                                                               |
| `TestMigrateValidatorDelegations_WithUnbondingAndRedelegation` | Validator delegation re-key covers unbonding/redelegation with UnbondingId.                                                                              |
| `TestMigrateValidatorSupernode_WithMetrics`                    | Supernode metrics state re-keyed when metrics exist; old key deleted via DeleteMetricsState.                                                             |
| `TestMigrateValidatorSupernode_MetricsWriteFails`              | Metrics write failure propagates as error.                                                                                                               |
| `TestMigrateValidatorSupernode_NotFound`                       | No-op when validator is not a supernode.                                                                                                                 |
| `TestMigrateValidatorSupernode_EvidenceAddressMigrated`        | Evidence entries matching old valoper get ValidatorAddress updated to new valoper; non-matching entries preserved unchanged.                             |
| `TestMigrateValidatorSupernode_AccountHistoryMigrated`         | PrevSupernodeAccounts entries matching old account updated to new account; new migration history entry appended with current block height.               |
| `TestMigrateValidatorSupernode_IndependentAccountPreserved`    | Validator migration preserves an already-migrated or otherwise independent supernode account instead of overwriting it with the validator's new address. |
| `TestFinalizeVestingAccount_Delayed`                           | DelayedVestingAccount correctly recreated at new address.                                                                                                |
| `TestFinalizeVestingAccount_Periodic`                          | PeriodicVestingAccount recreated with original periods.                                                                                                  |
| `TestFinalizeVestingAccount_PermanentLocked`                   | PermanentLockedAccount correctly recreated at new address.                                                                                               |
| `TestFinalizeVestingAccount_NonBaseAccountFallback`            | Non-BaseAccount fallback extracts base account and recreates vesting.                                                                                    |
| `TestQueryParams_NilRequest`                                   | Nil request returns InvalidArgument error.                                                                                                               |
| `TestQueryParams_Valid`                                        | Valid request returns stored params.                                                                                                                     |
| `TestUpdateParams_InvalidAuthority`                            | Non-authority address rejected with ErrInvalidSigner.                                                                                                    |
| `TestUpdateParams_ValidAuthority`                              | Correct authority updates params successfully.                                                                                                           |

Additional regression coverage:

- `x/supernode/v1/keeper/supernode_by_account_internal_test.go` adds a`TestKeeper_GetSuperNodeByAccount` subtest that verifies`DeleteSuperNode` removes both the primary record and the secondary`SuperNodeByAccountKey` index entry, then allows the same supernode account to be reattached under the migrated validator operator without a false collision.

### H) EVM migration integration tests

Purpose: end-to-end integration tests for the `x/evmigration` module using real keepers wired via `app.Setup(t)`.
File: `tests/integration/evmigration/migration_test.go`
Run: `go test -tags=test ./tests/integration/evmigration/... -v`

| Test                                                        | Description                                                                                                                                                                                                          |
| ----------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `TestClaimLegacyAccount_Success`                          | End-to-end migration: balances move, migration record stored, counter incremented.                                                                                                                                   |
| `TestClaimLegacyAccount_MigrationDisabled`                | Rejection when enable_migration is false with real params.                                                                                                                                                           |
| `TestClaimLegacyAccount_AlreadyMigrated`                  | Double migration and NewAddressWasMigrated with real state.                                                                                                                                                          |
| `TestClaimLegacyAccount_SameAddress`                      | Rejection when legacy and new addresses are identical.                                                                                                                                                               |
| `TestClaimLegacyAccount_InvalidSignature`                 | Rejection with a bad legacy signature against real auth state.                                                                                                                                                       |
| `TestClaimLegacyAccount_ValidatorMustUseMigrateValidator` | Validator operators rejected from ClaimLegacyAccount with real staking state.                                                                                                                                        |
| `TestClaimLegacyAccount_MultiDenom`                       | Multi-denomination balance transfer verified with real bank module.                                                                                                                                                  |
| `TestClaimLegacyAccount_LegacyAccountRemoved`             | Legacy auth account removed and new account exists after migration.                                                                                                                                                  |
| `TestClaimLegacyAccount_AfterValidatorMigration`          | Fresh-state validator-first flow: migrate validator first, then migrate a legacy delegator account; verifies claim succeeds, rewards/delegation state remain valid, and delegation points to the migrated validator. |
| `TestMigrateValidator_Success`                            | End-to-end validator migration: bonded validator with self-delegation + external delegator; verifies record re-keyed, delegations re-keyed, distribution state migrated, balances moved, counters incremented.       |
| `TestMigrateValidator_NotValidator`                       | Rejection when legacy address is not a validator operator with real staking state.                                                                                                                                   |
| `TestQueryMigrationRecord_Integration`                    | Query server returns record after real migration, nil before.                                                                                                                                                        |
| `TestQueryMigrationEstimate_Integration`                  | Estimate query with real staking state reports correct values.                                                                                                                                                       |

---

## Integration Tests

All integration tests are under `tests/integration/evm`.
Most packages use `-tags='integration test'`. The IBC ERC20 middleware package currently uses `-tags='test'`.

### A) Ante integration

Purpose: validates Cosmos-path ante behavior after EVM integration, including fee enforcement and authz message filtering.
Suite: `tests/integration/evm/ante/suite_test.go`

| Test                                         | Description                                                                                  |
| -------------------------------------------- | -------------------------------------------------------------------------------------------- |
| `CosmosTxFeeEnforcement`                   | Verifies low-fee Cosmos txs are rejected and valid-fee txs pass under current ante settings. |
| `AuthzGenericGrantRejectsBlockedMsgTypes`  | Ensures authz generic grants cannot authorize blocked EVM message types.                     |
| `AuthzGenericGrantAllowsNonBlockedMsgType` | Ensures authz generic grants still work for allowed non-EVM message types.                   |

### B) Contracts integration

Purpose: exercises contract lifecycle paths (deploy/call/revert) and persistence guarantees across restarts.
Suite: `tests/integration/evm/contracts/suite_test.go`

| Test                                         | Description                                                                        |
| -------------------------------------------- | ---------------------------------------------------------------------------------- |
| `ContractDeployCallAndLogsE2E`             | Deploys a contract, executes calls, and validates receipt/log behavior end to end. |
| `ContractRevertTxReceiptAndGasE2E`         | Sends a reverting tx and checks expected revert/receipt/gas semantics.             |
| `CALLBetweenContracts`                     | Deploys caller/callee pair, validates CALL opcode returns data cross-contract.     |
| `DELEGATECALLPreservesContext`             | Verifies DELEGATECALL writes to proxy's storage, not target contract's storage.    |
| `CREATE2DeterministicAddress`              | Factory deploys child via CREATE2; verifies deterministic address off-chain.       |
| `STATICCALLCannotModifyState`              | Confirms STATICCALL reverts when the target contract attempts SSTORE.              |
| `TestContractCodePersistsAcrossRestart`    | Confirms deployed runtime bytecode remains queryable after node restart.           |
| `TestContractStoragePersistsAcrossRestart` | Confirms contract storage values remain intact after node restart.                 |
| `TestEVMStatePreservationAcrossRestart`    | Deploys contract, restarts node, verifies code/storage/receipts survive intact.    |
| `TestConcurrentMixedEVMOperations`         | 5 concurrent goroutines (transfers + deploys) verify no panics/deadlocks/lost txs. |
| `TestERC20ApproveAllowanceTransferFrom`    | Full ERC20 flow: deploy, approve, check allowance, transferFrom, verify balances.  |
| `ContractProxiesActionGetParams`           | Deploys STATICCALL proxy → action precompile (0x0901), verifies getParams() response. |
| `ContractProxiesSupernodeGetParams`        | Deploys STATICCALL proxy → supernode precompile (0x0902), verifies getParams() response. |
| `ContractProxiesActionGetActionFee`        | Proxy forwards getActionFee(100) with ABI-encoded args, validates fee arithmetic. |
| `ContractQueriesBothPrecompiles`           | Two proxies query action + supernode precompiles in same test, cross-validates results. |

### C) Fee market integration

Purpose: validates EIP-1559 RPC behavior, effective gas price accounting, and dynamic-fee admission rules.
Suite: `tests/integration/evm/feemarket/suite_test.go`

| Test                                             | Description                                                                 |
| ------------------------------------------------ | --------------------------------------------------------------------------- |
| `FeeHistoryReportsCanonicalShape`              | Checks `eth_feeHistory` response shape and core fields for compatibility. |
| `ReceiptEffectiveGasPriceRespectsBlockBaseFee` | Verifies receipt `effectiveGasPrice` reflects block base fee constraints. |
| `FeeHistoryRewardPercentilesShape`             | Validates reward percentile formatting/structure in fee history results.    |
| `MaxPriorityFeePerGasReturnsValidHex`          | Ensures `eth_maxPriorityFeePerGas` returns a valid hex value.             |
| `GasPriceIsAtLeastLatestBaseFee`               | Ensures `eth_gasPrice` is not below current base fee expectations.        |
| `DynamicFeeType2EffectiveGasPriceFormula`      | Verifies type-2 tx effective gas price calculation is correct.              |
| `DynamicFeeType2RejectsFeeCapBelowBaseFee`     | Ensures txs with fee cap below base fee are rejected.                       |

### D) IBC ERC20 middleware integration

Purpose: validates ERC20 middleware behavior on ICS20 receive and edge-case handling for mapping registration.
Suite: `tests/integration/evm/ibc/suite_test.go`

| Test                                 | Description                                                                                |
| ------------------------------------ | ------------------------------------------------------------------------------------------ |
| `RegistersTokenPairOnRecv`         | Ensures valid incoming ICS20 transfers auto-register ERC20 token pairs/maps.               |
| `NoRegistrationWhenDisabled`       | Ensures registration is skipped when ERC20 middleware feature is disabled.                 |
| `NoRegistrationForInvalidReceiver` | Ensures invalid receiver payloads do not create token mappings.                            |
| `DenomCollisionKeepsExistingMap`   | Ensures existing denom-map collisions are preserved and not overwritten.                   |
| `RoundTripTransfer`                | Full IBC forward+reverse transfer with ERC20 registration, BalanceOf, and balance restore. |
| `SecondaryDenomRegistration`       | Verifies non-native denom (ufoo) gets ERC20 auto-registration and dynamic precompile.      |
| `TransferBackBurnsVoucher`         | Verifies return transfer zeros bank and ERC20 balances while token pair persists.          |

### E) JSON-RPC/indexer integration

Purpose: validates JSON-RPC compatibility, tx/receipt lookup/indexer behavior, mixed Cosmos+EVM block behavior, and restart durability.
Suites:

- `tests/integration/evm/jsonrpc/suite_test.go`
- `tests/integration/evm/jsonrpc/mixed_block_suite_test.go`

| Test                                           | Description                                                                                         |
| ---------------------------------------------- | --------------------------------------------------------------------------------------------------- |
| `BasicRPCMethods`                            | Verifies baseline RPC methods (`eth_chainId`, `eth_blockNumber`, etc.) return expected values.  |
| `BackendBlockCountAndUncleSemantics`         | Validates block-count and uncle-related method semantics on this backend.                           |
| `BackendNetAndWeb3UtilityMethods`            | Verifies `net_*` and `web3_*` utility methods return sane values.                               |
| `BlockLookupIncludesTransaction`             | Ensures block queries include expected transaction objects/hashes.                                  |
| `TransactionLookupByBlockAndIndex`           | Validates tx lookup by block hash/number + index works correctly.                                   |
| `MultiTxOrderingSameBlock`                   | Verifies deterministic `transactionIndex` ordering for multiple txs in one block.                 |
| `ReceiptIncludesCanonicalFields`             | Ensures receipts expose canonical Ethereum fields and expected encodings.                           |
| `MixedCosmosAndEVMTransactionsCanShareBlock` | Confirms Cosmos and EVM txs can be included together in the same committed block.                   |
| `MixedBlockOrderingPersistsAcrossRestart`    | Confirms mixed-block tx ordering is preserved across restart.                                       |
| `TestEOANonceByBlockTagAndRestart`           | Verifies nonce query semantics by block tag and restart persistence.                                |
| `TestSelfTransferFeeAccounting`              | Verifies self-transfer balance delta equals `gasUsed * effectiveGasPrice`.                        |
| `TestIndexerDisabledLookupUnavailable`       | Verifies tx/receipt lookups are unavailable when indexers are disabled.                             |
| `TestLogsIndexerPathAcrossRestart`           | Verifies `eth_getLogs` indexer queries remain correct across restart.                             |
| `TestReceiptPersistsAcrossRestart`           | Verifies `eth_getTransactionReceipt` remains available after restart.                             |
| `TestIndexerStartupSmoke`                    | Smoke-tests JSON-RPC/WebSocket/indexer startup path and startup logs.                               |
| `TestTransactionByHashPersistsAcrossRestart` | Verifies `eth_getTransactionByHash` consistency before/after restart.                             |
| `OpenRPCDiscoverMethodCatalog`               | Verifies `rpc_discover` returns non-empty, deduplicated catalog with required namespace coverage. |
| `OpenRPCDiscoverMatchesEmbeddedSpec`         | Verifies runtime `rpc_discover` output matches the embedded OpenRPC document in the node binary.  |
| `TestOpenRPCHTTPDocumentEndpoint`            | Verifies `/openrpc.json` (API server) is served and matches JSON-RPC `rpc_discover` method set. |
| `BatchJSONRPCReturnsAllResponses`            | Sends a batch of 4 different methods and verifies all responses return with correct IDs.            |
| `BatchJSONRPCMixedErrorsAndResults`          | Batch with valid + invalid requests; verifies per-request errors don't break the batch.             |
| `BatchJSONRPCSingleElementBatch`             | Edge case: single-element batch array returns one response correctly.                               |
| `BatchJSONRPCDuplicateMethods`               | Batch of 3 identical `eth_blockNumber` calls returns 3 independent results.                       |

### F) Mempool integration

Purpose: validates app-side EVM mempool behavior for ordering, pending visibility, nonce handling, and replacement policy.
Suite: `tests/integration/evm/mempool/suite_test.go`

| Test                                        | Description                                                                           |
| ------------------------------------------- | ------------------------------------------------------------------------------------- |
| `DeterministicOrderingUnderContention`    | Verifies deterministic inclusion ordering under concurrent submission pressure.       |
| `EVMFeePriorityOrderingSameBlock`         | Verifies higher-fee tx priority ordering when txs land in the same block.             |
| `PendingTxSubscriptionEmitsHash`          | Verifies pending subscription emits tx hashes for pending EVM txs.                    |
| `NonceGapPromotionAfterGapFilled`         | Verifies queued nonce-gap txs are promoted once missing nonce is filled.              |
| `TestMempoolDisabledWithJSONRPCFailsFast` | Verifies txpool namespace behavior when app-side mempool is disabled.                 |
| `TestNonceReplacementRequiresPriceBump`   | Verifies same-nonce replacement requires configured fee bump threshold.               |
| `TestMempoolCapacityRejectsOverflow`      | Floods a low-capacity mempool until rejection, verifying max-txs enforcement.         |
| `RapidReplacementRace`                    | Concurrent goroutines race to replace the same nonce; verifies no panics/deadlock.    |
| `NewHeadsSubscriptionEmitsBlocks`         | WS `newHeads` subscription receives block header with expected fields.              |
| `LogsSubscriptionEmitsEvents`             | WS `logs` subscription receives LOG1 event from a deployed contract.                |
| `NewHeadsSubscriptionMultipleBlocks`      | WS `newHeads` delivers 3 consecutive headers with monotonically increasing numbers. |

### G) Precisebank integration

Purpose: validates transaction-level and query-level behavior of fractional balance accounting under EVM flows.
Suite: `tests/integration/evm/precisebank/suite_test.go`

| Test                                                    | Description                                                                       |
| ------------------------------------------------------- | --------------------------------------------------------------------------------- |
| `PreciseBankFractionalBalanceQueryMatrix`             | Verifies fractional-balance query responses across representative account states. |
| `PreciseBankFractionalBalanceRejectsInvalidAddress`   | Verifies invalid address formats are rejected by precisebank queries.             |
| `PreciseBankEVMTransferSendSplitMatrix`               | Verifies integer/fractional split behavior across EVM transfer scenarios.         |
| `PreciseBankSecondarySenderBurnMintWorkflow`          | Verifies mint/send/burn workflow behavior using secondary sender flows.           |
| `TestPreciseBankRemainderQueryPersistsAcrossRestart`  | Verifies precisebank remainder query results persist after restart.               |
| `TestPreciseBankModuleAccountFractionalBalanceIsZero` | Verifies module account fractional balance invariants remain zero as expected.    |

### H) Precompiles integration

Purpose: validates static precompile read/write paths exposed to EVM callers.
Suite: `tests/integration/evm/precompiles/suite_test.go`

| Test                                                         | Description                                                                               |
| ------------------------------------------------------------ | ----------------------------------------------------------------------------------------- |
| `BankPrecompileBalancesViaEthCall`                         | Verifies bank precompile balance queries via `eth_call`.                                |
| `DistributionPrecompileQueryPathsViaEthCall`               | Verifies distribution precompile query methods via `eth_call`.                          |
| `GovPrecompileQueryPathsViaEthCall`                        | Verifies governance precompile query methods via `eth_call`.                            |
| `StakingPrecompileValidatorViaEthCall`                     | Verifies staking precompile validator query behavior via `eth_call`.                    |
| `Bech32PrecompileRoundTripViaEthCall`                      | Verifies Bech32 precompile address conversion round-trips correctly.                      |
| `P256PrecompileVerifyViaEthCall`                           | Verifies P256 precompile signature verification behavior.                                 |
| `StakingPrecompileDelegateTxPath`                          | Verifies staking delegate tx path through precompile execution.                           |
| `DistributionPrecompileSetWithdrawAddressTxPath`           | Verifies distribution withdraw-address tx path via precompile.                            |
| `GovPrecompileCancelProposalTxPathFailsForUnknownProposal` | Verifies expected failure behavior for canceling unknown proposals.                       |
| `SlashingPrecompileGetParamsViaEthCall`                    | Verifies slashing precompile `getParams` returns valid slashing parameters.             |
| `SlashingPrecompileGetSigningInfosViaEthCall`              | Verifies `getSigningInfos` returns signing info for genesis validator.                  |
| `SlashingPrecompileUnjailTxPathFailsWhenNotJailed`         | Verifies unjail tx reverts when validator is not jailed.                                  |
| `ICS20PrecompileDenomsViaEthCall`                          | Verifies ICS20 `denoms` query returns well-formed response (empty list on fresh chain). |
| `ICS20PrecompileDenomHashViaEthCall`                       | Verifies ICS20 `denomHash` query for non-existent trace returns empty hash.             |
| `ICS20PrecompileDenomViaEthCall`                           | Verifies ICS20 `denom` query for non-existent hash returns default struct.              |
| `SupernodeRegisterTxPath`                                  | Registers supernode via precompile tx, verifies receipt success and listSuperNodes count.  |
| `SupernodeReportMetricsTxPath`                             | Reports metrics via precompile tx from the registered supernode account, verifies success. |
| `SupernodeReportMetricsTxPathFailsForWrongCaller`          | Verifies non-supernode account cannot report metrics (auth check on contract.Caller()).    |
| `ActionRequestCascadeTxPathFailsWithBadSignature`          | Verifies requestCascade rejects invalid signature format via tx path.                     |
| `ActionApproveActionTxPathFailsForNonExistent`             | Verifies approveAction reverts for non-existent action ID.                                |
| `PrecompileGasMeteringAccuracy`                            | Verifies each precompile consumes bounded, non-trivial gas (6 precompiles).               |
| `PrecompileGasEstimateMatchesActual`                       | Verifies eth_estimateGas is within 3x of actual gasUsed for bank precompile.              |

### I) VM query/state integration

Purpose: validates `x/vm` query APIs and consistency against JSON-RPC/accounting/state snapshots.
Suite: `tests/integration/evm/vm/suite_test.go`

| Test                                               | Description                                                                   |
| -------------------------------------------------- | ----------------------------------------------------------------------------- |
| `VMQueryParamsAndConfigBasic`                    | Verifies vm params/config query endpoints return expected baseline values.    |
| `VMAddressConversionRoundTrip`                   | Verifies VM address conversion utilities round-trip correctly.                |
| `VMQueryAccountMatchesEthRPC`                    | Verifies VM account query fields match equivalent JSON-RPC account state.     |
| `VMQueryAccountRejectsInvalidAddress`            | Verifies VM account query rejects invalid addresses.                          |
| `VMQueryAccountAcceptsHexAndBech32`              | Verifies VM account query accepts both hex and Bech32 forms where supported.  |
| `VMBalanceBankMatchesBankQuery`                  | Verifies VM bank-balance query is consistent with bank module query results.  |
| `VMStorageQueryKeyFormatEquivalence`             | Verifies storage queries are equivalent across supported key encodings.       |
| `VMQueryCodeAndStorageMatchJSONRPC`              | Verifies VM code/storage queries align with JSON-RPC responses.               |
| `VMQueryAccountHistoricalHeightNonceProgression` | Verifies historical-height account queries show expected nonce progression.   |
| `VMQueryHistoricalCodeAndStorageSnapshots`       | Verifies historical code/storage snapshots are queryable and consistent.      |
| `VMBalanceERC20MatchesEthCall`                   | Verifies VM ERC20 balance query matches direct contract `eth_call` results. |
| `VMBalanceERC20RejectsNonERC20Runtime`           | Verifies ERC20 balance query fails cleanly for non-ERC20 runtimes.            |

---

## Devnet Tests

Devnet tests run inside the Docker multi-validator testnet (`make devnet-new`).
Test source: `devnet/tests/validator/evm_test.go`

| Test                                             | Description                                                                                                                                                         |
| ------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `TestEVMFeeMarketBaseFeeActive`                | Validates `eth_gasPrice` returns a non-zero base fee on an active devnet.                                                                                         |
| `TestEVMDynamicFeeTxE2E`                       | Sends a type-2 (EIP-1559) self-transfer and verifies receipt status 0x1.                                                                                            |
| `TestEVMTransactionVisibleAcrossPeerValidator` | Sends a tx to the local validator and verifies the receipt is visible on a peer validator with matching blockHash — exercises the broadcast worker re-gossip path. |

### EVM Migration Devnet Tests

See [devnet-tests.md](devnet-tests.md) for full details on the EVM migration devnet test binary (modes, usage, and coverage).

## Recommended Next Steps

### High priority (before mainnet)

1. **Security audit of EVM integration layer** — All comparable chains (Evmos, Kava, Cronos) underwent dedicated EVM audits before mainnet.
2. **Production JSON-RPC hardening profile** — Rate limiting is implemented, but deployment profiles should explicitly lock CORS origins and namespace exposure (`debug`, `personal`, `admin`) per environment.

### Medium priority

1. **Lumera module precompiles** — Action module precompile implemented at `0x0901` with typed Cascade/Sense request/finalize + generic approve + all queries (`precompiles/action/`). Supernode module precompile implemented at `0x0902` with register/deregister/start/stop/update/reportMetrics + all queries (`precompiles/supernode/`). Other chains (Evmos: staking/distribution/IBC/vesting, Kava: swap/earn) ship custom precompiles at launch.
2. **CosmWasm + EVM interaction design** — Document whether/how CosmWasm contracts and EVM contracts can interact. Consider a bridge mechanism, shared query paths, or explicit isolation. Lumera is the only Cosmos EVM chain also running CosmWasm, so there is no precedent to follow.
3. **Chain upgrade EVM state preservation test** — Deploy a contract, perform upgrade, verify contract still works. No test currently validates EVM state survives a chain upgrade.
4. **External block explorer integration** — Blockscout or Etherscan-compatible explorer. All comparable chains have this at mainnet.

### Low priority

1. **Precompile gas metering benchmarks** — Validate actual gas consumption vs expected for each precompile and compare against upstream Cosmos EVM defaults.
2. **Ops monitoring runbook** — Document fee market monitoring (base fee tracking, gas utilization trends), alerting thresholds, and common failure mode diagnosis.
3. **EVM governance proposals** — Mechanism to toggle precompiles and adjust EVM params via on-chain governance (Evmos has dedicated governance proposals for this).
