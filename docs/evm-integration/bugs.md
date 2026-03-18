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
