# EVM Integration — Bugs Found and Fixed

Tracking issues discovered during EVM integration testing and devnet operation.

See [main.md](main.md) for the full integration document.

---

### 1) EVM broadcast worker: sender address not recovered

**Symptom**: All validators log `failed to broadcast promoted evm transactions … sender address is missing: invalid request` (code 18) after EVM txs land.

**Root cause**: `broadcastEVMTransactionsSync` used `msg.FromEthereumTx(ethTx)` which copies raw tx bytes but does **not** populate the `From` field. The Cosmos ante handler then rejects the message because `GetSigners()` returns an empty sender.

**Fix** (`app/evm_broadcast.go`): Replaced with `msg.FromSignedEthereumTx(ethTx, ethSigner)` which recovers the sender address from the ECDSA signature using the chain's EVM signer.

**Why tests passed**: The JSON-RPC ingestion path (`eth_sendRawTransaction` → txpool → mempool `Insert`) already uses `FromSignedEthereumTx`. The broadcast worker only re-gossips promoted txs to peer validators, so single-validator integration tests never exercise this path.

**Tests added**: `TestBroadcastEVMTxFromFieldRecovery` (unit — validates `FromSignedEthereumTx` recovers sender while `FromEthereumTx` does not), `TestEVMTransactionVisibleAcrossPeerValidator` (devnet — end-to-end cross-validator propagation).

---

### 2) Feemarket base fee decays to zero on idle devnet

**Symptom**: `TestEVMFeeMarketBaseFeeActive` fails because `eth_gasPrice` / `baseFeePerGas` returns 0 after a few hundred blocks with no EVM traffic.

**Root cause**: Devnet uses a **static genesis template** (`devnet/default-config/devnet-genesis-evm.json`) that bypasses the app's `LumeraFeemarketGenesisState()`. The template had stale values: `min_gas_price: 0` (no floor) and `base_fee_change_denominator: 8` (aggressive decay).

**Fix** (`devnet/default-config/devnet-genesis-evm.json`): Updated the template to match `config/evm.go` constants — `min_gas_price: 0.0005`, `base_fee_change_denominator: 16`.

**Lesson**: Any change to `config/evm.go` or `app/evm/genesis.go` feemarket defaults must also be mirrored in the static devnet genesis template.

---

### 3) Gentx rejected by MinGasPriceDecorator during InitGenesis

**Symptom**: After fixing the feemarket genesis params (non-zero `min_gas_price`), `lumerad` fails to start: `fee not provided … minimum global fee is 100ulume: insufficient fee`.

**Root cause**: The cosmos/evm `MinGasPriceDecorator` enforces minimum gas prices unconditionally, including during InitGenesis (block height 0). The standard Cosmos SDK fee decorators skip enforcement at genesis, but cosmos/evm's decorator does not.

**Fix** (`app/evm/ante.go`): Added `genesisSkipDecorator` — a generic wrapper that skips the inner decorator when `BlockHeight() == 0`. Applied to `MinGasPriceDecorator` in the Cosmos ante chain so gentxs are processed without fees, matching standard SDK behavior.

---

### 4) IBC transfer silently fails with out-of-gas

**Symptom**: `TestIBCTransferWithEVMModeStillRelays` fails — transfer appears to succeed but tokens never arrive on the destination chain.

**Root cause**: Two issues combined:

1. Gas estimation returned 70907 but actual execution cost 72619. The `--gas-adjustment 1.3` margin was insufficient.
2. `lumerad tx --broadcast-mode sync` exits with code 0 even when CheckTx rejects the tx. The test helper discarded command output, so the rejection was invisible.

**Fix** (`devnet/tests/ibcutil/ibcutil.go`):

- Increased `--gas-adjustment` from 1.3 to 1.5.
- Added `--output json` and JSON response parsing to detect non-zero result codes.

**Also** (`devnet/hermes/config.toml`): Reduced `clear_interval` from 100 to 10 as a safety net for missed WebSocket packet events.

---

### 5) EVM mempool deadlock on nonce-gap promotion (BroadcastTxFn re-entry)

**Symptom**: The chain hangs permanently when an EVM transaction fills a nonce gap in the txpool. All block production stops and the node becomes unresponsive.

**Root cause**: The cosmos/evm `ExperimentalEVMMempool` calls `BroadcastTxFn` synchronously from inside `runReorg` while holding the mempool mutex (`m.mtx`). If `BroadcastTxFn` submits the promoted tx via CometBFT's local ABCI client, the resulting `CheckTx` calls back into `Insert()` on the same mempool — which tries to acquire `m.mtx` again. Since Go's `sync.Mutex` is not reentrant, this deadlocks the goroutine and halts the chain.

The call stack that deadlocks:

```text
Insert() → [acquires m.mtx] → runReorg() → BroadcastTxFn()
  → BroadcastTxSync() → local ABCI CheckTx → Insert() → [blocks on m.mtx] ← DEADLOCK
```

**Fix** (`app/evm_broadcast.go`): Implemented `evmTxBroadcastDispatcher` — an async broadcast queue that decouples txpool promotion from CometBFT CheckTx submission:

1. `BroadcastTxFn` (called inside `runReorg`) enqueues promoted txs into a bounded channel and returns immediately — never blocking `Insert()`.
2. A single background worker goroutine drains the channel and submits txs via `BroadcastTxSync` after the mutex is released.
3. Tx hashes are tracked in a `pending` set for deduplication; hashes are released after processing or on queue-full/error paths.

Additionally, `RegisterTxService` override in `app/evm_runtime.go` ensures the broadcast worker uses the local CometBFT client (not the stale HTTP client from `SetClientCtx` which runs before CometBFT starts).

**Tests**: The re-entry hazard is validated by `TestEVMMempoolReentrantInsertBlocks` (unit), and the full promotion-to-inclusion path is validated by `NonceGapPromotionAfterGapFilled` (integration).

---

### 6) ICS20 precompile panics: IBC store keys not registered in EVM snapshot

**Symptom**: Any call to the ICS20 precompile (queries or transactions) causes a panic: `kv store with key KVStoreKey{…, transfer} has not been registered in stores`. The node process crashes on `eth_sendRawTransaction`; `eth_call` returns the panic as an error.

**Root cause**: In `app/app.go`, `registerEVMModules` (which captures `app.kvStoreKeys()` for the EVM keeper's snapshot multi-store) runs **before** `registerIBCModules` (which registers the `"transfer"` and `"ibc"` store keys). Since the EVM keeper snapshots the store key set at initialization, any store keys registered later are invisible to EVM execution.

```text
app.go:
  registerEVMModules()   ← captures kvStoreKeys() — no "transfer", no "ibc"
  registerIBCModules()   ← registers "transfer" + "ibc" store keys (too late)
```

**Impact**: The ICS20 precompile is effectively non-functional. All six methods (`transfer`, `denom`, `denoms`, `denomHash`, `denomTrace`, `denomTraces`) panic when invoked via the EVM.

**Fix** (`app/evm.go`, `app/app.go`): Added `syncEVMStoreKeys()` — called immediately after `registerIBCModules()`, it iterates all registered store keys and adds any missing ones to the EVM keeper's `KVStoreKeys()` map. Since the keeper stores the map by reference and the snapshot multi-store reads it lazily (when `StateDB` is created), the IBC store keys are visible to all subsequent EVM execution.

**Tests**: Three ICS20 query tests (`ICS20PrecompileDenomsViaEthCall`, `ICS20PrecompileDenomHashViaEthCall`, `ICS20PrecompileDenomViaEthCall`) previously detected this bug and used `t.Skip`. With the fix applied, these tests should pass. The ICS20 transfer tx test remains excluded from the suite pending a separate IBC channel configuration requirement.

---

### 7) Upgrade handler seeds `aatom` denom instead of `alume` in EVM coin info

**Symptom**: After v1.12.0 chain upgrade, Cosmos txs fail with `"provided fee < minimum global fee (2567ulume < 43aatom)"`. The feemarket `MinGasPriceDecorator` reads `GetEVMCoinDenom()` which returns `"aatom"` — the wrong denom for Lumera.

**Root cause**: During `RunMigrations`, the SDK calls `DefaultGenesis()` → `InitGenesis()` for new modules not present in `fromVM`. cosmos/evm v0.6.0's `DefaultParams().EvmDenom = DefaultEVMExtendedDenom = "aatom"`, so the upstream `InitGenesis` writes `aatom` into the EVM coin info KV store. The post-migration `SetParams` + `InitEvmCoinInfo` with Lumera params runs after, but the global `evmCoinInfo` is already sealed by `sync.Once` in `PreBlock`.

**Fix** (`app/upgrades/v1_12_0/upgrade.go`): Pre-populate `fromVM` with consensus versions for all four EVM modules (`vm`, `feemarket`, `precisebank`, `erc20`) before calling `RunMigrations`. Per Cosmos SDK docs, `fromVM[module] = ConsensusVersion` causes `RunMigrations` to skip `InitGenesis` for that module. The handler then manually sets Lumera-specific params and initializes coin info with the correct `ulume`/`alume` denoms.

**Tests**: `TestUpstreamDefaultEvmDenomIsNotLumera` (sentinel: detects if upstream changes their default), `TestV1120SkipsEVMInitGenesis` (verifies fromVM skip pattern is in place).

---

### 8) Upgrade handler leaves `x/erc20` disabled after skipped `InitGenesis`

**Symptom**: After the v1.12.0 upgrade, ERC20 registration/conversion behavior can appear silently disabled even though the module store exists. Querying ERC20 params reads back `EnableErc20=false` and `PermissionlessRegistration=false`.

**Root cause**: The same `fromVM[module] = ConsensusVersion` pattern used to skip unsafe upstream `InitGenesis` for new EVM modules also skips `x/erc20` parameter initialization. Unlike `x/precisebank`, `x/erc20` persists booleans in its own KV store and interprets missing keys as `false`, so a brand-new upgraded store comes up effectively disabled unless the upgrade handler writes defaults explicitly.

**Fix** (`app/upgrades/v1_12_0/upgrade.go`, `app/upgrades/params/params.go`, `app/app.go`): Wire the ERC20 keeper into the upgrade params bundle and explicitly call `Erc20Keeper.SetParams(ctx, erc20types.DefaultParams())` after `RunMigrations`. This preserves the `InitGenesis` skip for denom/coin-info safety while restoring the intended default ERC20 behavior.

**Tests**: `TestV1120InitializesERC20ParamsWhenInitGenesisIsSkipped` reproduces the skipped-`InitGenesis` state by clearing the ERC20 param keys, runs the v1.12.0 handler, and verifies the default params are restored.

---

### 9) Validator migration fails when the supernode account was already migrated first

**Symptom**: `tests_evmigration -mode=migrate-validator` fails on a validator that already has a migrated EVM supernode account with:

`migrate validator supernode: supernode account <addr> already associated with another validator`

This shows up even though the supernode account belongs to the same logical validator/supernode pair and was migrated correctly earlier by the supernode process.

**Root cause**: `MigrateValidatorSupernode` preserved the already-migrated independent `SupernodeAccount` correctly, but then wrote the re-keyed supernode record under the new valoper without first removing the old supernode record and its `SuperNodeByAccountKey` secondary index entry. `SetSuperNode` saw the stale old-valoper index for that same account and treated it as a collision with "another validator".

**Fix** (`x/evmigration/keeper/migrate_validator.go`, `x/supernode/v1/keeper/supernode.go`): Added `DeleteSuperNode` to remove both the primary supernode record and the secondary account index, and changed validator supernode migration to delete the old valoper entry before writing the re-keyed record under the new valoper.

**Tests**: `TestMigrateValidatorSupernode_IndependentAccountPreserved` verifies validator migration does not overwrite an already-migrated independent supernode account. `x/supernode/v1/keeper/supernode_by_account_internal_test.go` also adds a regression subtest that verifies deleting the old supernode removes the stale account index and allows the same account to be reattached under the migrated validator.

---

### 10) Validator migration leaves redelegation destination validators on legacy valopers

**Symptom**: `tests_evmigration -mode=migrate-validator` fails post-migration checks for legacy accounts with redelegations after one or more destination validators are migrated later, for example:

`expected redelegation on new address for <new-src-valoper>-><new-dst-valoper>, got 0`

On-chain inspection showed the redelegation had moved to the new delegator address and, when applicable, to the migrated source validator, but its `validator_dst_address` still pointed at the old legacy destination valoper.

**Root cause**: `MigrateValidatorDelegations` only re-keyed redelegations returned by `GetRedelegationsFromSrcValidator(oldValAddr)`. That covers records where the migrating validator is the redelegation source, but misses redelegations where the migrating validator appears only as the destination. As a result, destination-side validator migration left those redelegation records referencing the legacy valoper.

**Fix** (`x/evmigration/keeper/migrate_validator.go`): Changed validator migration to iterate all redelegations and re-key any record where the migrating validator appears as either `ValidatorSrcAddress` or `ValidatorDstAddress`.

**Tests**: `TestMigrateValidatorDelegations_WithUnbondingAndRedelegation` now covers both cases:
- migrated validator as redelegation source
- migrated validator as redelegation destination

**Important note**: This fix prevents new bad migrations, but it does not repair redelegation records that were already migrated incorrectly on an existing chain. Those require a fresh devnet run or a dedicated repair path.

---

### 11) Distribution withdraw address sends reward dust to already-migrated legacy address

**Symptom**: `tests_evmigration -mode=verify` reports `[bank] still has balance: 1 ulume` on a legacy address that was fully migrated. The address should have zero balance.

**Root cause**: Cross-account dependency during ordered migration. When Account A's legacy address was set as Account B's distribution **withdraw address** (third-party), and Account A migrated first, the subsequent migration of Account B triggered `WithdrawDelegationRewards` in Step 1 (`MigrateDistribution`). The distribution module sent B's rewards to B's withdraw address — which was A's now-dead legacy address.

Confirmed via on-chain events: at height 208, `MsgClaimLegacyAccount` for a different account emitted `withdraw_rewards: 1ulume` with `coin_received.receiver` pointing to the already-migrated legacy address.

**Fix** (`x/evmigration/keeper/migrate_distribution.go`, `x/evmigration/keeper/migrate_staking.go`):

1. Added `redirectWithdrawAddrIfMigrated()` — called at the start of `MigrateDistribution`, before any reward withdrawal. It checks if the delegator's withdraw address is a previously-migrated legacy address (via `MigrationRecords`). If so, it resets the withdraw address to self, ensuring rewards land in the account being migrated.

2. Updated `migrateWithdrawAddress()` in `MigrateStaking` — when the third-party withdraw address is a migrated legacy address, it now follows the `MigrationRecord` to resolve to the corresponding new address, so future rewards reach the correct destination.

**Tests**: Updated `TestMigrateDistribution_WithDelegations`, `TestMigrateDistribution_NoDelegations`, and all `TestClaimLegacyAccount_*` mock expectations to account for the new `GetDelegatorWithdrawAddr` call in `redirectWithdrawAddrIfMigrated`.

---

### 12) Verify mode redelegation query uses non-existent CLI command

**Symptom**: `tests_evmigration -mode=verify` logs `WARN: redelegation query: exit status 1` for every migrated address, silently skipping redelegation verification.

**Root cause**: `verifyRedelegationCount` called `lumerad query staking redelegations <addr>` (plural). In Cosmos SDK v0.53.6, the autocli registers only `redelegation` (singular) with `src-validator-addr` as a required positional argument. The plural form does not exist.

**Fix** (`devnet/tests/evmigration/verify.go`): Replaced with `getValidators()` + `queryAnyRedelegationCount()` which iterates all validator pairs using the correct `lumerad query staking redelegation <addr> <src> <dst>` command.

---

### 13) Supernode `reportMetrics` precompile bypasses caller authentication

**Severity**: Critical

**Symptom**: Any EVM account can submit metrics for any registered supernode by passing the public supernode account address in calldata.

**Root cause**: `ReportMetrics` in `precompiles/supernode/tx.go` took `supernodeAccount` from `args[1]` (calldata) and passed it to the keeper message without binding it to `contract.Caller()`. Every other tx method in the file derives `creator` from `evmAddrToBech32(contract.Caller())`. The keeper's check (`msg.SupernodeAccount != sn.SupernodeAccount`) only verifies the provided address matches on-chain state — but that value is publicly queryable, so the check is not an auth gate.

**Fix** (`precompiles/supernode/tx.go`): Replaced `args[1]` usage with `evmAddrToBech32(p.addrCdc, contract.Caller())` so the authoritative supernode account is derived from the EVM tx signer. The calldata parameter is accepted for ABI compatibility but ignored.

**Tests added**: `SupernodeReportMetricsTxPath` (success path), `SupernodeReportMetricsTxPathFailsForWrongCaller` (verifies a different EVM account is rejected).

---

### 14) `finalizeCascade`/`finalizeSense` precompiles emit success on soft rejection

**Severity**: High

**Symptom**: When the keeper records evidence instead of failing (e.g., supernode not in top-10, Kademlia ID verification failure), the precompile emits `ActionFinalized` event and returns `true`, misleading EVM callers and indexers.

**Root cause**: The Cosmos keeper intentionally returns `nil` error for evidence-recording rejections to avoid tx reverts (which would discard the evidence). The precompile treated `nil` error as unconditional success, emitting the event and packing `true` regardless of whether the action state actually changed to `Done`.

**Fix** (`precompiles/action/tx_cascade.go`, `tx_sense.go`): After the keeper call, the precompile now checks whether the action reached `ActionStateDone`. The `ActionFinalized` event is only emitted and `true` returned when finalization actually completed. Soft rejections return `false` without an event, preserving the evidence recording.

---

### 15) `requestCascade` ABI declares `bytes` for signature field but keeper expects dot-delimited string

**Severity**: Medium

**Symptom**: Solidity callers following the ABI and passing raw `bytes` for the `signatures` parameter produce data that fails keeper validation.

**Root cause**: The ABI in `precompiles/action/abi.json` declared `signatures` as `type: "bytes"`. The precompile coerced `[]byte` to `string`. But the keeper's `RegisterAction` handler expects `Base64(rq_ids).creator_signature` — a dot-delimited textual format. A Solidity caller passing `abi.encode(someBytes)` would never produce a valid dot-separated string.

**Fix** (`precompiles/action/abi.json`, `tx_cascade.go`, `IAction.sol`): Changed the signature parameter from `bytes` to `string` across ABI, precompile, and Solidity interface. Callers now pass the dot-delimited format directly as a string.

**Tests added**: `ActionRequestCascadeTxPathFailsWithBadSignature` (verifies invalid signature format is rejected via tx path).

---

### 16) Withdraw address lost when third-party target was already migrated

**Symptom**: `tests_evmigration -mode=migrate` reports `withdraw-addr mismatch: expected <B_new> got <A_new>` for accounts whose withdraw address pointed to a previously-migrated legacy address.

**Root cause**: A temporal coupling between `MigrateDistribution` (Step 1) and `MigrateStaking` (Step 2) inside `migrateAccount`. When Account A has a third-party withdraw address pointing to already-migrated Account B:

1. `MigrateDistribution` calls `redirectWithdrawAddrIfMigrated()`, which correctly resets A's withdraw address to **self** (A's legacy address) so that `WithdrawDelegationRewards` deposits into A's legacy balance instead of B's dead address.
2. `MigrateStaking` then calls `migrateWithdrawAddress()`, which re-reads the withdraw address from state and now sees **self** (due to step 1's temporary redirect). The `withdrawAddr.Equals(legacyAddr)` check returns true, so the function sets the withdraw address to A's new address — the third-party resolution code is never reached.

Net effect: A's post-migration withdraw address becomes A_new (self) instead of B_new (the resolved third-party destination).

**Fix** (`x/evmigration/keeper/msg_server_claim_legacy.go`, `x/evmigration/keeper/migrate_staking.go`): Snapshot the original withdraw address in `migrateAccount` **before** `MigrateDistribution` runs, then pass it to `MigrateStaking` → `migrateWithdrawAddress`. This decouples the permanent withdraw-address migration from the temporary redirect, so the third-party resolution path is reached correctly.

**Tests**: Added `TestClaimLegacyAccount_MigratedThirdPartyWithdrawAddress` — end-to-end message-server test that seeds a migration record for the third-party withdraw address, runs the full `ClaimLegacyAccount` flow, and asserts `SetDelegatorWithdrawAddr` resolves to the migrated destination (pins the cross-step snapshot→redirect→resolve interaction). Added `TestMigrateStaking_MigratedThirdPartyWithdrawAddress` — unit test for the helper in isolation. Updated `TestMigrateStaking_*` and `TestClaimLegacyAccount_FailAtStaking`/`FailAtDistribution` mock expectations to match the new `origWithdrawAddr` parameter. Tightened integration test `TestClaimLegacyAccount_ValidatorMustUseMigrateValidator` to assert `ErrUseValidatorMigration` specifically.

---

### 17) Devnet migrate post-check: stale redelegation pair from prepare rerun-conflict

**Symptom**: `tests_evmigration -mode=migrate` reports `expected redelegation on new address for <src-valoper>-><dst-valoper>, got 0` even though the migration tx succeeded and `redelegations_to_migrate: 1` was confirmed by the estimate query.

**Root cause**: Devnet verifier/data-tracking mismatch, not a keeper bug. When the prepare phase is rerun and encounters a `isPrepareRerunConflict` error for a redelegation attempt, the conflict handler in the extra-activity path (`prepare.go` line 496) called `queryAnyRedelegationCount` to confirm *some* redelegation exists, then recorded the **randomly-chosen** `srcVal`/`dstVal` pair — not the pair that actually exists on-chain.

The migration estimate (which counts all redelegations regardless of pair via `GetRedelegations(ctx, addr, ...)`) correctly reported 1. The keeper-side migration faithfully re-keyed whatever on-chain redelegation existed. But the post-migration validator queried the **recorded** exact pair (`migrate.go` line 499: `queryRedelegationCount(rec.NewAddress, currentSrc, currentDst)`), which didn't match the actual on-chain pair, and returned 0.

**Fix** (`devnet/tests/evmigration/prepare.go`, `devnet/tests/evmigration/migrate.go`, `devnet/tests/evmigration/query_state.go`):

1. **Prepare rerun-conflict handler** (extra-activity path): Added an exact-pair check before recording the marker — only calls `addRedelegation(srcVal, dstVal, "")` if `queryRedelegationCount(rec.Address, srcVal, dstVal) > 0`, matching the pattern already used in the primary prepare path. With this fix, all recorded redelegation entries are exact-pair verified at recording time, so no post-migration fallback is needed.
2. **Post-migration validator**: No weakening — the exact-pair check remains strict. Every recorded pair must be found on the new address after migration; misses always fail.

Also applied `resolvePostMigrationAddress(expected)` to the withdraw-address post-check to handle the same class of already-migrated third-party issue (bug #16 fix interplay).

---

### 18) Validator migration Step V1 sends reward dust to already-migrated withdraw addresses

**Symptom**: `tests_evmigration -mode=verify` reports `[bank] still has balance: 13 ulume` and `[bank] still has balance: 5 ulume` on legacy addresses that were fully migrated. Traced via `lumerad query txs` to a `MsgMigrateValidator` at height 252 — the dust was deposited *after* the affected accounts had already migrated (at heights 242 and 246).

**Root cause**: Variant of bug #11 specific to `MsgMigrateValidator` Step V1. When a validator migrates, it calls `WithdrawDelegationRewards` for **every delegator** of that validator (line 91 of `msg_server_migrate_validator.go`). If a delegator's withdraw address points to an already-migrated legacy address, the rewards are deposited into the dead address — because `redirectWithdrawAddrIfMigrated` only runs inside `MigrateDistribution` (the regular account migration path), not during the validator migration's bulk reward withdrawal.

The `migrate-all` mode's random interleaving made this bug observable: some delegators migrated before their validator, then the validator migration withdrew rewards to the delegators' third-party withdraw addresses (which were already dead).

**Fix** (`x/evmigration/keeper/msg_server_migrate_validator.go`, `x/evmigration/keeper/migrate_distribution.go`): Added `temporaryRedirectWithdrawAddr(ctx, delAddr)` — a new helper that redirects to self **temporarily** for the withdrawal, then **restores** the original third-party address afterward. This prevents dust on dead addresses while preserving the delegator's intended withdraw target for their own later migration (where `migrateAccount` snapshots it via `origWithdrawAddr` before `MigrateDistribution` runs). Using the permanent `redirectWithdrawAddrIfMigrated` here would have caused the same clobbering bug that #16 fixed for regular account migration.

**Tests**: `TestMigrateValidator_ThirdPartyWithdrawAddrPreserved` — sets up a third-party delegator whose withdraw address points to an already-migrated account, verifies the redirect→withdraw→restore sequence via ordered mock expectations (redirect to self, withdraw rewards, restore original address).

---

### 19) MetaMask/EVM clients see wrong chain ID after upgrade (app.toml missing `[evm]` section)

**Symptom**: After upgrading from a pre-EVM binary (< v1.12.0), MetaMask transactions fail. `eth_chainId` returns `0x494c1a9` (76857769, correct), but the JSON-RPC backend internally uses chain ID `262144` (the cosmos/evm upstream default) for transaction validation. MetaMask sends transactions signed with chain ID `76857769`; the backend's `SendRawTransaction` rejects them with `incorrect chain-id; expected 262144, got 76857769`. `net_version` also returns `262144` instead of `76857769`.

**Root cause**: The JSON-RPC backend reads `evm-chain-id` from `app.toml` (`rpc/backend/backend.go:207`). Nodes that existed before the EVM upgrade keep their old `app.toml`, which has no `[evm]` section. The Cosmos SDK only generates `app.toml` when the file does not exist (`server/util.go:284`), so the new EVM sections are never added. The backend falls back to `cosmosevmserverconfig.DefaultEVMChainID = 262144`.

Meanwhile, the EVM keeper (initialized in `x/vm/keeper/keeper.go:119`) correctly calls `SetChainConfig(DefaultChainConfig(76857769))` using the Lumera constant. This creates a split: on-chain state uses `76857769`, but the JSON-RPC transport layer uses `262144`.

**Fix** (`cmd/lumera/cmd/config_migrate.go`): Added `migrateAppConfigIfNeeded()`, called from the root command's `PersistentPreRunE` after `InterceptConfigsPreRunHandler`. On every startup it checks whether `evm.evm-chain-id` in Viper matches `config.EVMChainID` (76857769). If not, it reads all existing settings from `app.toml` via Viper unmarshal, overwrites `EVM.EVMChainID` with the Lumera constant, ensures `JSONRPC.Enable`/`JSONRPC.EnableIndexer`/`rpc` API namespace are set, and regenerates `app.toml` with the full template (SDK + EVM + Lumera sections), preserving all operator customizations.

**Tests**: `testBasicRPCMethods` (integration, `tests/integration/evm/jsonrpc/basic_methods_test.go`) — validates `eth_chainId` and `net_version` both return `76857769`. `verifyJSONRPCChainID` (devnet, `devnet/tests/evmigration/verify.go`) — runtime check after upgrade that both JSON-RPC methods return the correct chain ID.

---

### 20) JSON-RPC rate limiter does not front the public RPC port (security audit finding #1)

**Symptom**: Operators enable the built-in rate limiter expecting their public JSON-RPC port to be protected, but attackers can bypass rate limiting by using the normal public alias proxy port instead of the separate rate-limit proxy port.

**Root cause**: The alias proxy (`app/evm_jsonrpc_alias.go`) listens on the operator-configured public `json-rpc.address` and forwards to an internal loopback. The rate-limit proxy (`app/evm_jsonrpc_ratelimit.go`) listens on its own separate `lumera.json-rpc-ratelimit.proxy-address` (default `:8547`) and also forwards to the internal loopback. The two proxies operate independently — public traffic hits the alias proxy (no rate limiting), while the rate-limit proxy sits on a different port that external clients don't use by default.

**Fix** (`app/evm_jsonrpc_ratelimit.go`, `app/evm_jsonrpc_alias.go`, `app/app.go`): Refactored the proxy stack so rate limiting is injected directly into the alias proxy's HTTP handler when enabled. `startJSONRPCProxyStack` decides the topology: when the alias proxy is active, rate limiting wraps its handler (one server, one port, rate-limited); when no alias proxy is active, a standalone rate-limit proxy is started as a fallback. The separate `proxy-address` config is only used in the standalone fallback mode.

**Tests**: Existing rate-limiter unit tests (`TestExtractIP_*`, `TestStopJSONRPCRateLimitProxy_*`) validate the middleware and lifecycle. The architectural fix ensures the rate limiter is always in the request path of the public endpoint.

---

### 21) Validator migration gas pre-check undercounts destination-side redelegations (security audit finding #2)

**Symptom**: A validator with many destination-side redelegations (other delegators redelegating TO this validator) can pass the `MaxValidatorDelegations` safety check and execute a migration that consumes more gas and state writes than governance intended.

**Root cause**: The pre-check in `MsgMigrateValidator` used `GetRedelegationsFromSrcValidator` which only counts redelegations where the validator is the source. But the actual migration logic (`MigrateValidatorDelegations`) uses `IterateRedelegations` and re-keys redelegations where the validator appears as either source OR destination. The `MigrationEstimate` query had the same undercount.

**Fix** (`x/evmigration/keeper/msg_server_migrate_validator.go`, `x/evmigration/keeper/query.go`): Replaced `GetRedelegationsFromSrcValidator` with `IterateRedelegations` checking both `ValidatorSrcAddress` and `ValidatorDstAddress` in the pre-check and estimate query, matching the execution logic.

**Tests**: Updated mock expectations in `msg_server_migrate_validator_test.go` and `msg_server_claim_legacy_test.go` to use `IterateRedelegations`.

---

### 22) Migration proofs lack chain ID domain separation (security audit finding #4)

**Symptom**: A migration proof signed for one Lumera network (e.g., testnet) could be replayed on another network (e.g., mainnet) because the signed payload did not include any chain-specific data.

**Root cause**: The migration payload was `lumera-evm-migration:<kind>:<legacyAddr>:<newAddr>` — no chain ID, no EVM chain ID, no deadline.

**Fix** (`x/evmigration/keeper/verify.go`, `x/evmigration/keeper/msg_server_claim_legacy.go`, `x/evmigration/keeper/msg_server_migrate_validator.go`, `x/evmigration/client/cli/tx.go`): Extended the payload format to `lumera-evm-migration:<chainID>:<evmChainID>:<kind>:<legacyAddr>:<newAddr>`. Both the Cosmos chain ID (distinguishes networks) and the EVM chain ID (distinguishes execution domains) are included. Callers pass `ctx.ChainID()` and `lcfg.EVMChainID`. The CLI uses `clientCtx.ChainID`. This is a breaking change to the proof format — existing pre-signed proofs are invalid.

**Tests**: Updated all verify tests and signing helpers in `verify_test.go`, `msg_server_claim_legacy_test.go`, and `msg_server_migrate_validator_test.go` to include chain IDs. Test context wired with `WithChainID(testChainID)`.
