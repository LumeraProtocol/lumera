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
