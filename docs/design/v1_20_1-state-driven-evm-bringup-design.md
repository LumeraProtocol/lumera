# v1.20.1 State-Driven EVM Bring-Up — Design

## Problem

The v1.20.1 upgrade currently routes by **network** (`IsMainnet`), not by **state**:

- **Mainnet**: v1.20.1 carries the full EVM bring-up (reuses `v1_20_0.StoreUpgrades` + `v1_20_0.CreateUpgradeHandler`), because mainnet skips v1.20.0.
- **Testnet / devnet**: v1.20.1 is a migration-only hotfix (`standardUpgradeHandler`, no `StoreUpgrade`), because those networks already ran v1.20.0.

Consequence: a chain that upgrades **directly 1.12.0 → 1.20.1 on a non-mainnet chain-id** (e.g. a devnet rehearsal of the mainnet one-hop) gets neither the EVM stores nor the bring-up logic. The upgrade is broken — EVM store keys are never mounted, EVM params are never finalized, and RunMigrations would run upstream EVM `InitGenesis` with the wrong (`aatom`) denom.

## Goal

Make v1.20.1 carry the EVM bring-up **whenever EVM has not yet been initialized, on any network, with no env flag** — and remain a pure migration-only hotfix when EVM is already present. Both layers of the upgrade (store mounting and handler logic) must key off **chain state**, not chain-id.

## Detection signal

The SDK-canonical "has this module been initialized" signal is the `fromVM` (module → consensus version) map passed to the handler:

- A chain that ran v1.20.0 has `fromVM["evm"]` set (v1.20.0 registered all four EVM modules).
- A chain that skipped v1.20.0 does not.

`evmtypes.ModuleName == "evm"`. The presence of that key in `fromVM` is the state signal for the handler. Upgrades are atomic (a failed handler aborts the block with no commit), so store presence and `fromVM` presence are always consistent — v1.20.0 either fully ran (stores mounted + module versions registered) or not at all.

## Design

### 1. New `app/upgrades/v1_20_1` package — state-driven handler

```
const UpgradeName = "v1.20.1"

CreateUpgradeHandler(p):
  return func(ctx, plan, fromVM):
    if "evm" NOT in fromVM:                          // EVM never brought up (one-hop)
        → return v1_20_0.CreateUpgradeHandler(p)(ctx, plan, fromVM)   // full bring-up
    else:                                            // already ran v1.20.0
        → return p.ModuleManager.RunMigrations(ctx, p.Configurator, fromVM)  // hotfix only
```

This one handler replaces **both** the mainnet `v1_20_0` handler reuse and the non-mainnet `standardUpgradeHandler`. No import cycle: `v1_20_1` imports `v1_20_0`; `v1_20_0` does not import `v1_20_1`.

A small exported predicate `evmAlreadyInitialized(fromVM) bool` is extracted so the branch is unit-testable without wiring keepers.

### 2. Add-only, state-driven store loader

New loader `AddOnlyStoreLoader(height, baseUpgrades, logger)` in `store_upgrade_manager.go`:

- At load time, read committed store names (reuse `loadExistingStoreNames`).
- Compute `Added = baseUpgrades.Added \ existing` — the declared EVM stores that are **absent** from committed state.
- Never computes `Deleted` or `Renamed`. It can only add stores; it can never wipe one.
- If `Added` is empty → `DefaultStoreLoader` (no-op). Otherwise `ms.LoadLatestVersionAndUpgrade(&StoreUpgrades{Added})`.

This is a deliberately narrowed form of `AdaptiveStoreLoader`: it keeps the state-driven "add what's missing" half and drops the `Deleted = existing − expected` half, because that half would silently `deleteKVStore()` any store the running binary fails to register — turning a wiring regression into irreversible, consensus-splitting data loss on mainnet. Add-only makes that class of bug impossible.

### 3. Routing changes

- `SetupUpgrades`, `upgradeNameV1201` case: return `StoreUpgrade = &v1_20_0.StoreUpgrades` **on all networks** and `Handler = v1_20_1.CreateUpgradeHandler(params)`. The `IsMainnet` branch is removed.
- `StoreLoaderForUpgrade`: route `v1.20.1` to `AddOnlyStoreLoader` **before** the adaptive/non-adaptive split, so it applies uniformly regardless of the `LUMERA_ENABLE_STORE_UPGRADE_MANAGER` env flag or chain-id.
- `v1.20.0` routing is unchanged: mainnet still skips it (`NoUpgradeConfig`); testnet/existing-devnet keep it in history.
- `app.go setupUpgrades` needs no change: v1.20.1 now always has a non-nil `StoreUpgrade`, so it passes the existing `StoreUpgrade == nil && !useAdaptive → return` guard and reaches the loader selection, where `StoreLoaderForUpgrade` hands back the add-only loader.

### 4. Behavior matrix

| Chain state | Stores (add-only loader) | Handler (`fromVM`-gated) |
|---|---|---|
| One-hop 1.12.0 → 1.20.1 (mainnet, or any non-mainnet rehearsal) | EVM keys absent → **added** | `"evm"` absent → **full v1.20.0 bring-up** |
| Already ran 1.20.0, then 1.20.1 hotfix | EVM keys present → **no-op** | `"evm"` present → **migrations only** |
| Staged 1.20.0 → 1.20.1 (unchanged) | 1.20.0 mounts; 1.20.1 no-op | 1.20.0 brings up; 1.20.1 migrates only |

## Testing

- `evmAlreadyInitialized`: true when `fromVM` has `"evm"`, false otherwise (table test).
- Add-only store computation: `Added = base \ existing`; empty when all present; full when all absent; partial mix.
- `SetupUpgrades` routing for v1.20.1: `StoreUpgrade` non-nil and contains the five EVM store keys for **mainnet, testnet, and devnet** chain-ids; `Handler` non-nil for all.
- The full bring-up internals (migration_end_time per network, ERC20 params, denom metadata) remain covered by the existing `v1_20_0/upgrade_test.go` — not re-tested.

### Existing tests affected

- `TestV1201MigrationOnlyOnNonMainnet` asserts `config.StoreUpgrade == nil` on non-mainnet. That assertion **flips** — v1.20.1 now declares the EVM store set on every network. Rewrite it (e.g. `TestV1201CarriesEVMBringupOnAllNetworks`) to assert the store set is present on testnet/devnet too.
- `TestV1201CarriesEVMBringupOnMainnet` still passes (mainnet still gets `StoreUpgrade` + a non-nil handler).

## Docs

- Update the upgrade overview table in `upgrades.go` (the `v1.20.1` row) to describe state-driven bring-up rather than mainnet-only.
- No user-facing doc changes; this is upgrade-routing internals.

## Out of scope

- No change to `v1.20.0` routing or handler.
- No change to the full `AdaptiveStoreLoader` / its devnet+env gating (still available for the general "skip intermediate upgrades" devnet use case).
- No mainnet use of the delete-capable adaptive diff.
