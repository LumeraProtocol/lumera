# Validator Migration Gas: Auto-Sizing + Formula

**Date:** 2026-06-22
**Status:** Design (pending implementation)

## Problem

`scripts/migrate-validator.sh` (and the account/multisig variants) broadcast the
migration tx with the CLI default **200000 gas** (`lumerad_tx` in
`scripts/evmigration-common.sh` sets no `--gas`). For a *validator* migration
that is far too low: `MsgMigrateValidator` is a tiny message (two addresses + two
proofs), but its handler re-keys **every delegation, unbonding, and redelegation**
pointing at the validator from the old valoper to the new one
(`x/evmigration/keeper/migrate_validator.go`). That is O(N) state writes in a
single tx, so gas scales with the validator's record count.

The tx-size limit is **not** the constraint (the message is small). The binding
limits are:

- **Block gas** — `config/evm.go: ChainDefaultConsensusMaxGas = 25_000_000`
  (mainnet). Devnet runs `block.max_gas = -1` (unlimited).
- **CheckTx/RPC capacity** — a migration whose handler iterates thousands of
  records can exceed the node's RPC timeout during simulate/CheckTx (observed as
  `post failed: … EOF` for an inflated 5149-record devnet validator).

The `max_validator_delegations` param (default **2500**) is the protocol guard for
this: ~7k gas/record × 2500 ≈ **17.5M gas < 25M**, i.e. the cap is sized so a
within-cap migration fits in one block.

### Real-world scale

Mainnet (`lumera-mainnet-1`, 84 validators) — largest validators by delegation
count: Innovating Capital **1597**, Aurora Staking 1507, Nodeist 1424. All well
under the 2500 cap → ~11M gas → migrate fine **with adequate gas**. The devnet's
5149-record validator is an artifact of gen-activity churn and legitimately
exceeds the cap.

Fees are **waived** for migration txs (`ante/evmigration_fee_decorator.go`), so a
generous gas *limit* costs nothing — the only ceiling is block `max_gas`.

## Goals

1. Migration scripts set an adequate gas limit automatically (no manual `--gas`).
2. Give validators a reference gas formula for manual/other tooling.
3. Document validator migration (cap, gas, downtime/jailing) in both the user
   migration guide and the shell-script guide.

## Non-goals

- No protocol change (no paced/chunked on-chain migration). Real validators fit
  under the cap; a paced design is a separate future spec if a validator ever
  legitimately exceeds the cap.
- Not reverting the devnet `max_validator_delegations` (left at 20000 for now).

## Design

### 1. Gas auto-sizing in `evmigration-common.sh`

Change the broadcast path (`lumerad_tx`) to size gas instead of using the 200000
default. Strategy: **`--gas auto` with a formula fallback**.

- Primary: broadcast with `--gas auto --gas-adjustment 1.5`. The chain simulates
  the exact gas; fees are waived so the resulting limit is free. This is
  maintenance-free (no hard-coded coefficient) and exact.
- Fallback: if the simulate fails (RPC error / `EOF` / non-numeric estimate),
  compute `gas = GAS_BASE + GAS_PER_RECORD × total_records` from the pre-flight
  estimate counts the script already fetched (`val_delegation_count +
  val_unbonding_count + val_redelegation_count` for validators; the analogous
  account/multisig counts otherwise), and broadcast with that fixed `--gas`.
- Clamp: if the chosen gas would exceed the chain's `block.max_gas` (queried from
  consensus params; skip the clamp when `max_gas = -1`), abort with a clear
  message: the validator has too many records to migrate in a single tx.

Constants (initial; `GAS_PER_RECORD` to be calibrated on a realistic-size
migration during implementation):

```
GAS_BASE       = 200000
GAS_PER_RECORD = 7000     # ~a few KVStore ops/record; analytical, confirm empirically
GAS_ADJUSTMENT = 1.5
```

Because `lumerad_tx` is shared by all three migration scripts, account and
multisig migrations get the same treatment (their gas also scales with the
migrating account's delegation records).

### 2. Reference formula (docs)

For validators submitting the tx by hand or via other tooling:

```
--gas ≈ 200000 + 7000 × (delegations + unbondings + redelegations)
```

This must stay under the chain's block `max_gas` (25M ⇒ ~3500 records), which the
2500 cap already enforces with margin. A validator over the cap cannot migrate in
a single tx.

### 3. Documentation

- `docs/evm-integration/user-guides/migration.md` — add a **Validator migration**
  section: the operator account re-keys all delegations (O(N)); the cap and its
  rationale; the gas formula; "stop the node before broadcasting — it will miss
  blocks and may be jailed; unjail after"; fees are waived.
- The shell-script migration guide (the migrate-*.sh reference doc) — note that
  the scripts now size gas automatically (`--gas auto` + fallback) and the
  `--i-have-stopped-the-node` requirement.

## Testing

- Unit/scripted: simulate the gas-selection logic (auto success → uses estimate;
  auto failure → uses formula; over-block-gas → aborts) with a fake `lumerad`.
- Live (within-cap): migrate a validator with a realistic record count (≤ cap) and
  confirm it commits, the new valoper holds all delegations, and `gas_used`
  matches the formula's ballpark (calibrates `GAS_PER_RECORD`).

## Notes / open items

- `GAS_PER_RECORD = 7000` is analytical; the devnet's only validators are
  gen-activity-inflated (>cap, can't simulate), so calibrate on a realistic-size
  migration (testnet or a freshly created small validator) during implementation.
- The devnet's inflated 5149-record validator remains un-migratable by design
  (over cap, over block-gas, over RPC CheckTx capacity) — expected.

---

## Post-implementation correction (2026-06-23, live devnet)

Live devnet measurements showed the original analytical assumptions were ~100–200x
off. Three corrections:

### 1. Gas formula recalibrated

Measured `gas_used` on the devnet:

- Account migration: ≈ **5.77M base** + **~1.33M/record**
- Validator migration (val1, 5149 records): ≈ **688k/record** (~3.54B total gas)

The original `GAS_BASE = 200000` and `GAS_PER_RECORD = 7000` were ~100–200× too
low. The constants were recalibrated to:

```
MIGRATION_GAS_BASE       = 6,000,000
MIGRATION_GAS_PER_RECORD = 1,500,000   # conservative: uses account marginal with margin
```

Reference formula: **`gas ≈ 6,000,000 + 1,500,000 × (delegations + unbondings + redelegations)`**.
`--gas auto` remains the preferred path (computes exact gas via simulate); the
formula is the fallback when auto-simulate fails.

### 2. Block gas is not a constraint

The original design stated the block gas limit was 25M (from
`config/evm.go: ChainDefaultConsensusMaxGas = 25_000_000`). Live measurement
confirms both devnet **and mainnet** (`rpc.lumera.io`, block ~5.65M) run
`block.max_gas = -1` (unlimited). The 25M figure is a code-level default constant,
not the actual consensus parameter on any live chain. Combined with waived migration
fees, **gas is not a feasibility blocker**. The `max_validator_delegations` cap
(default 2500) is a safety guard against unbounded execution time, not a
gas-fit requirement.

### 3. Real constraint: execution time and the RPC simulate timeout

`--gas auto` runs the full migration handler during the simulate RPC call.
For large validators this takes approximately as long as on-chain execution:
val1's 5149-record migration caused a **~107-second chain-wide block-production
stall** (normal block time ≈ 5.4s). CometBFT's default
`timeout_broadcast_tx_commit = 10s` is far shorter, so the simulate exceeds the
RPC write timeout and returns `Post "…:26657": EOF`.

Extrapolating linearly to mainnet's largest validator (~1597 delegations):
≈ **~1.1B gas** and **~30s** of block stall — a one-time spike, not a halt.

**Operator guidance:** raise `timeout_broadcast_tx_commit` (e.g. to `600s`) on
the node you broadcast through so `--gas auto` can complete, **or** broadcast with
a high fixed `--gas` (skips the simulate, no timeout risk).
