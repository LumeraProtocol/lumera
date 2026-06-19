# Unit Tests: App Wiring, Config, Genesis & Commands

Purpose: verifies that EVM runtime/CLI wiring is correctly initialized (genesis overrides, module order, precompiles, mempool, listeners, and command defaults).

Primary files:
- `app/evm_test.go`
- `app/evm_static_precompiles_test.go`
- `app/blocked_addresses_test.go`
- `app/evm_mempool_test.go`
- `app/evm_mempool_reentry_test.go`
- `app/evm_broadcast_test.go`
- `app/evm_mempool_metrics_test.go`
- `app/pending_tx_listener_test.go`
- `app/ibc_erc20_middleware_test.go`
- `app/ibc_test.go`
- `app/vm_preinstalls_test.go`
- `app/amino_codec_test.go`
- `app/statedb_events_test.go`
- `app/evm_erc20_policy.go`
- `app/evm_erc20_policy_msg.go`
- `app/evm_erc20_policy_test.go`
- `app/wasm_evm_plugin_test.go`
- `precompiles/crossruntime/guard_test.go`
- `precompiles/crossruntime/addr_test.go`
- `proto/lumera/erc20policy/tx.proto`
- `x/erc20policy/types/tx.pb.go`
- `x/erc20policy/types/codec.go`
- `cmd/lumera/cmd/config_test.go`
- `cmd/lumera/cmd/root_test.go`
- `app/upgrades/upgrades_test.go`
- `app/upgrades/v1_20_0/upgrade_test.go`

| Test | Description |
| --- | --- |
| `TestRegisterEVMDefaultGenesis` | Verifies EVM-related modules are registered and expose Lumera-specific default genesis values. |
| `TestEVMModuleOrderAndPermissions` | Verifies module order constraints and module-account permissions for EVM modules. |
| `TestEVMStoresAndModuleAccountsInitialized` | Verifies EVM KV/transient stores and module accounts are initialized in app startup. |
| `TestEVMStaticPrecompilesConfigured` | Verifies expected static precompiles are configured on the EVM keeper. |
| `TestBlockedAddressesMatrix` | Verifies blocked-address set contains expected module/precompile addresses. |
| `TestPrecompileSendRestriction` | Verifies bank send restriction blocks sends to EVM precompile addresses. |
| `TestEVMMempoolWiringOnAppStartup` | Verifies app-side EVM mempool wiring occurs at startup with expected handlers. |
| `TestEVMMempoolDisabledWhenMaxTxsIsNegative` | Verifies `mempool.max-txs = -1` leaves the app-side EVM mempool disabled and BaseApp on `NoOpMempool`. |
| `TestEVMMempoolReentrantInsertBlocks` | Demonstrates mutex re-entry hazard that the async broadcast queue prevents. |
| `TestConfigureEVMBroadcastOptionsFromAppOptions` | Verifies broadcast debug flag parsing from app options (bool, string, nil). |
| `TestEVMTxBroadcastDispatcherDedupesQueuedAndInFlight` | Verifies dispatcher deduplicates queued and in-flight tx hashes. |
| `TestEVMTxBroadcastDispatcherQueueFullReleasesPending` | Verifies queue-full path releases pending hash reservations. |
| `TestEVMTxBroadcastDispatcherReleasesPendingAfterProcessError` | Verifies pending hashes are released after broadcast process errors. |
| `TestEVMTxBroadcastDispatcherEnqueueRemainsNonBlocking` | Verifies enqueue does not block while worker is processing. |
| `TestBroadcastEVMTxFromFieldRecovery` | Regression guard: `FromEthereumTx` leaves `From` empty; `FromSignedEthereumTx` recovers the sender. |
| `TestEVMMempoolMetricsDescribeReturnsAllDescriptors` | Verifies Describe emits all 5 expected metric descriptors (size, pending, queued, broadcast_queue_depth, rejections_total). |
| `TestEVMMempoolMetricsCollectReturnsGaugesAndCounter` | Verifies Collect emits 4 gauges + 1 counter with sensible initial values. |
| `TestEVMMempoolMetricsIncRejections` | Verifies rejection counter increments correctly for single and bulk operations. |
| `TestEVMMempoolMetricsIncRejectionsBy_ZeroAndNegativeIgnored` | Verifies zero and negative values do not modify the rejection counter. |
| `TestEVMMempoolMetricsNilBroadcastQueueLenFn` | Verifies nil broadcastQueueLenFn produces zero broadcast_queue_depth without panic. |
| `TestEVMMempoolMetricsWiredOnAppStartup` | Verifies metrics collector is initialized and wired into App struct during startup. |
| `TestEVMMempoolMetricsBroadcastQueueDepthReportsLive` | Verifies broadcast_queue_depth gauge reads live value from provided function on each scrape. |
| `TestEVMMempoolMetricsSizeExcludesQueued` | Verifies size gauge reflects only proposal-eligible txs (pending + cosmos pool), not queued nonce-gap txs. |
| `TestEVMMempoolMetricsCheckTxWrapperIncrementsRejections` | Verifies the CheckTx handler wrapper increments the rejection counter on invalid tx submission. |
| `TestRegisterPendingTxListenerFanout` | Verifies registered pending-tx listeners are invoked for each pending hash event. |
| `TestIBCERC20MiddlewareWiring` | Verifies IBC transfer stack includes ERC20 middleware wiring in app composition. |
| `TestIsInterchainAccount` | Verifies ICA account type detection helper behavior. |
| `TestIsInterchainAccountAddr` | Verifies ICA detection by address lookup through account keeper. |
| `TestEVMAddPreinstallsMatrix` | Verifies preinstall contract registration matrix in VM keeper setup paths. |
| `TestRegisterLumeraLegacyAminoCodecEnablesEthSecp256k1StdSignature` | Verifies legacy Amino registration covers eth_secp256k1 so SDK ante tx-size signature marshaling does not panic. |
| `TestInitAppConfigEVMDefaults` | Verifies default app config enables EVM/JSON-RPC values expected by Lumera, including `mempool.max-txs = 10000` and the real Cosmos EVM v0.6.0 `[evm.mempool]` defaults (`global-slots = 5120`, `global-queue = 1024`, no `insert-queue-size`). |
| `TestNeedsConfigMigration_LegacyConfig` | Empty Viper (pre-EVM app.toml with no EVM sections) triggers config migration. (Bug #19) |
| `TestNeedsConfigMigration_UpstreamDefault` | Upstream cosmos/evm default chain ID (262144) triggers config migration even when other sections exist. (Bug #19) |
| `TestNeedsConfigMigration_PartialManualEdit` | Correct evm-chain-id but missing [json-rpc] section still triggers migration. (Bug #19) |
| `TestNeedsConfigMigration_MissingLumeraSection` | Correct [evm] and [json-rpc] but missing [lumera.*] section triggers migration. (Bug #19) |
| `TestNeedsConfigMigration_OperatorDisabledJSONRPC` | Operator who explicitly set `json-rpc.enable = false` does NOT trigger migration — choice is respected. (Bug #19) |
| `TestNeedsConfigMigration_FullyMigrated` | Fully migrated config with all sentinel keys set does NOT trigger migration. (Bug #19) |
| `TestMigrateAppConfig_LegacyTomlOnDisk` | Full migration flow: writes legacy app.toml, runs migrator, verifies disk and in-memory Viper state contain correct EVM config while preserving operator settings. (Bug #19) |
| `TestMigrateAppConfig_LegacyNegativeMaxTxsUsesNetworkDefault` | Verifies config migration rewrites legacy `mempool.max-txs = -1` to `5000` for devnet and `10000` for testnet/mainnet, while emitting only real Cosmos EVM mempool keys. |
| `TestNewRootCmdStartWiresEVMFlags` | Verifies start/root command exposes key EVM JSON-RPC flags. |
| `TestNewRootCmdDefaultKeyTypeOverridden` | Verifies root command default key algorithm is overridden to `eth_secp256k1`. |
| `TestRevertToSnapshot_ProcessedEventsInvariant` | Adapted from cosmos/evm v0.6.0: verifies StateDB event-tracking invariant after snapshot reverts during precompile calls. |
| `TestERC20Policy_DefaultModeIsAllowlist` | Verifies default policy mode is "allowlist" when no mode is set in KV store. |
| `TestERC20Policy_AllMode_DelegatesToInner` | "all" mode delegates `OnRecvPacket` unconditionally to inner keeper. |
| `TestERC20Policy_NoneMode_SkipsRegistration` | "none" mode returns original ack without delegating for unregistered IBC denoms. |
| `TestERC20Policy_NoneMode_PassesThroughNonIBC` | Non-IBC denoms always pass through regardless of mode. |
| `TestERC20Policy_NoneMode_PassesThroughAlreadyRegistered` | Already-registered IBC denoms pass through even in "none" mode. |
| `TestERC20Policy_AllowlistMode_BlocksUnlisted` | "allowlist" mode blocks unlisted IBC denoms. |
| `TestERC20Policy_AllowlistMode_AllowsListed` | "allowlist" mode allows governance-approved denoms. |
| `TestERC20Policy_PassthroughMethods` | `OnAcknowledgementPacket`, `OnTimeoutPacket`, `Logger` pass through to inner keeper. |
| `TestERC20Policy_AllowlistCRUD` | Allowlist add/remove/list operations work correctly. |
| `TestERC20Policy_AllowlistMode_DirectTransferAllowed` | Allows IBC denom whose base denom and full trace match an allowed entry. |
| `TestERC20Policy_AllowlistMode_BlocksWrongChannel` | Blocks IBC denom arriving via non-allowed channel even with allowed base denom. |
| `TestERC20Policy_AllowlistMode_BlocksMultiHopOnSameChannel` | Single-hop trace blocks multi-hop uatom relayed through the same destination channel. |
| `TestERC20Policy_AllowlistMode_MultiHopTraceAllowed` | 2-hop trace restriction matches correct multi-hop path. |
| `TestERC20Policy_AllowlistMode_EmptyTracePlaceholder` | Entry with empty trace never matches any real IBC packet. |
| `TestERC20Policy_BaseDenomTraceCRUD` | Trace-bound base denom add/remove/list operations work correctly, including `removeAllBaseDenomTraces`. |
| `TestERC20Policy_InitDefaults` | `initERC20PolicyDefaults` sets mode to "allowlist" and populates `DefaultAllowedBaseDenomTraces` with empty traces (inert placeholders); is idempotent. |
| `TestERC20PolicyMsg_SetRegistrationPolicy` | Governance message handler: authority validation, mode changes, ibc denom add/remove, base denom trace add/remove, validation errors. |
| `TestV1200SkipsEVMInitGenesis` | Verifies the v1.20.0 upgrade handler pre-populates `fromVM` with EVM module consensus versions to skip `InitGenesis`. |
| `TestV1200InitializesERC20ParamsWhenInitGenesisIsSkipped` | Verifies the v1.20.0 upgrade handler backfills Lumera ERC20 params. Bugs #8, #24, #25. |
| `TestParseEVMAddress_Valid` | Verifies strict EVM address parser accepts valid 40-char hex with/without 0x prefix. |
| `TestParseEVMAddress_Invalid` | Verifies strict parser rejects too-short, too-long, invalid hex, and empty addresses. |
| `TestParseHexBytes_Valid` | Verifies hex bytes parser handles 0x-prefixed, bare hex, and empty inputs. |
| `TestParseHexBytes_Invalid` | Verifies hex bytes parser rejects invalid hex and odd-length inputs. |
| `TestGasCapForCall_*` (4 cases) | Verifies gas cap helper returns min(remaining, DefaultCrossRuntimeGasCap=3M) correctly. |
| `TestGetCrossRuntimeDepth_ZeroByDefault` | Verifies fresh context has depth 0. |
| `TestWithIncrementedDepth_*` (3 cases) | Verifies depth increments, double-increments, and does not mutate parent context. |
| `TestCheckAndIncrementDepth_SucceedsAtZero` | Verifies increment succeeds at depth 0. |
| `TestCheckAndIncrementDepth_FailsAtMax` | Verifies increment returns `ErrReentrancyNotAllowed` at depth 1 (MaxCrossRuntimeDepth). |
| `TestCheckAndIncrementDepth_FailsBeyondMax` | Verifies increment fails at depth > max. |
| `TestCheckAndIncrementDepth_DoesNotMutateOnError` | Verifies context is unchanged when reentrancy check fails. |
| `TestEVMAddrToBech32_Roundtrip` | Verifies EVM address -> bech32 -> EVM address roundtrip. |
| `TestBech32ToEVMAddr_InvalidBech32` | Verifies invalid bech32 string returns error. |
| `TestBech32ToEVMAddr_WrongPrefix` | Verifies wrong bech32 prefix (cosmos vs lumera) returns error. |
| `TestAccAddrToEVMAddr_20Bytes` | Verifies SDK AccAddress -> EVM address conversion with full 20-byte input. |
| `TestEVMAddrToBech32_ZeroAddress` | Verifies zero address roundtrips correctly. |
