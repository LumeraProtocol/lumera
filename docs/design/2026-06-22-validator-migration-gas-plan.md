# Validator Migration Gas Auto-Sizing — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the migration shell scripts size the migration tx's gas automatically (chain-simulated, with a record-count formula fallback), and document validator migration in both migration guides.

**Architecture:** All three migration scripts (`migrate-validator.sh`, `migrate-account.sh`, `migrate-multisig.sh`) broadcast through `lumerad_tx()` in `scripts/evmigration-common.sh`. We add a pure gas-formula helper + a block-gas-limit guard, then change `lumerad_tx` to broadcast with `--gas auto --gas-adjustment 1.5` and, if the simulate fails, retry with the formula-computed fixed gas. Fees are waived for migration txs, so a generous gas limit is free; the only ceiling is block `max_gas`.

**Tech Stack:** Bash (POSIX-ish, `set -euo pipefail`), `lumerad` CLI, `jq`. Docs in Markdown.

## Global Constraints

- Migration txs are **fee-waived** (`ante/evmigration_fee_decorator.go`); `--gas` is an execution limit, not a cost.
- Block gas ceiling: mainnet `ChainDefaultConsensusMaxGas = 25_000_000` (`config/evm.go`); devnet `block.max_gas = -1` (unlimited — skip the clamp when `-1`).
- `max_validator_delegations` cap stays at its current value (not changed by this work).
- Gas constants (env-overridable): `MIGRATION_GAS_BASE=200000`, `MIGRATION_GAS_PER_RECORD=7000`, `MIGRATION_GAS_ADJUSTMENT=1.5`. `MIGRATION_GAS_PER_RECORD` is analytical — calibrate in Task 5.
- Reference formula: `gas ≈ 200000 + 7000 × (delegations + unbondings + redelegations)`.
- Run `make lint` equivalent for shell: `shellcheck` must pass on changed scripts.

---

### Task 1: Pure gas helpers in `evmigration-common.sh`

**Files:**
- Modify: `scripts/evmigration-common.sh` (add helpers near the other tx helpers, before `lumerad_tx`)
- Test: `scripts/evmigration-common_test.sh` (new; plain-bash assertions, no framework)

**Interfaces:**
- Produces:
  - `migration_gas_for_records <records>` → prints integer gas = `MIGRATION_GAS_BASE + MIGRATION_GAS_PER_RECORD * records`.
  - `gas_exceeds_block_limit <gas> <block_max_gas>` → returns 0 (true) if `block_max_gas` is a positive integer and `gas > block_max_gas`; returns 1 otherwise (including when `block_max_gas` is `-1`/empty/non-numeric).

- [ ] **Step 1: Write the failing test**

Create `scripts/evmigration-common_test.sh`:

```bash
#!/usr/bin/env bash
# Unit tests for pure helpers in evmigration-common.sh. No network/lumerad.
set -uo pipefail
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# Source only the function definitions (the lib is a source-safe library).
# shellcheck source=./evmigration-common.sh disable=SC1091
source "${DIR}/evmigration-common.sh"

fail=0
check() { # check <label> <got> <want>
  if [[ "$2" == "$3" ]]; then echo "ok: $1"; else echo "FAIL: $1 — got '$2' want '$3'"; fail=1; fi
}

check "base+per-record 0"    "$(migration_gas_for_records 0)"    "200000"
check "base+per-record 1597" "$(migration_gas_for_records 1597)" "11379000"
check "base+per-record 2500" "$(migration_gas_for_records 2500)" "17700000"

# gas_exceeds_block_limit: returns 0 (true) only when over a positive limit.
gas_exceeds_block_limit 30000000 25000000; check "30M>25M true" "$?" "0"
gas_exceeds_block_limit 11379000 25000000; check "11M>25M false" "$?" "1"
gas_exceeds_block_limit 99999999 -1;       check "unlimited(-1) false" "$?" "1"
gas_exceeds_block_limit 99999999 "";       check "empty limit false" "$?" "1"

exit "$fail"
```

- [ ] **Step 2: Run test to verify it fails**

Run: `bash scripts/evmigration-common_test.sh`
Expected: FAIL — `migration_gas_for_records: command not found` (or non-zero exit), helpers don't exist yet.

- [ ] **Step 3: Add the helpers**

In `scripts/evmigration-common.sh`, immediately before `lumerad_tx() {`, add:

```bash
# Gas sizing for migration txs. Migration fees are waived, so the gas value is an
# execution limit only; size it to the work (delegation/unbonding/redelegation
# re-keys) rather than the 200000 CLI default. Env-overridable.
MIGRATION_GAS_BASE="${MIGRATION_GAS_BASE:-200000}"
MIGRATION_GAS_PER_RECORD="${MIGRATION_GAS_PER_RECORD:-7000}"
MIGRATION_GAS_ADJUSTMENT="${MIGRATION_GAS_ADJUSTMENT:-1.5}"

# migration_gas_for_records <records> — fallback fixed gas when --gas auto fails.
migration_gas_for_records() {
  local records="${1:-0}"
  [[ "$records" =~ ^[0-9]+$ ]] || records=0
  echo $(( MIGRATION_GAS_BASE + MIGRATION_GAS_PER_RECORD * records ))
}

# gas_exceeds_block_limit <gas> <block_max_gas> — true (0) only when block_max_gas
# is a positive integer and gas strictly exceeds it. Unlimited (-1), empty, or
# non-numeric limits are treated as "no limit" (returns 1).
gas_exceeds_block_limit() {
  local gas="${1:-0}" limit="${2:-}"
  [[ "$limit" =~ ^[0-9]+$ ]] || return 1
  (( gas > limit ))
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `bash scripts/evmigration-common_test.sh`
Expected: all `ok:` lines, exit 0.

- [ ] **Step 5: shellcheck + commit**

Run: `shellcheck scripts/evmigration-common.sh scripts/evmigration-common_test.sh`
Expected: no errors (warnings pre-existing in the lib are acceptable; new lines clean).

```bash
git add scripts/evmigration-common.sh scripts/evmigration-common_test.sh
git commit -m "feat(evmigration): add migration gas-sizing helpers"
```

---

### Task 2: Wire auto+fallback gas into `lumerad_tx`

**Files:**
- Modify: `scripts/evmigration-common.sh` (`lumerad_tx`, ~lines 331-342)
- Modify: `scripts/migrate-validator.sh` (set record count before broadcast)
- Modify: `scripts/migrate-account.sh`, `scripts/migrate-multisig.sh` (set record count before broadcast)

**Interfaces:**
- Consumes: `migration_gas_for_records`, `gas_exceeds_block_limit` (Task 1).
- Produces:
  - Global `MIGRATION_RECORD_COUNT` (integer): total records to re-key, set by each migration script after its pre-flight estimate. Default 0.
  - `lumerad_tx` broadcasts with `--gas auto --gas-adjustment ${MIGRATION_GAS_ADJUSTMENT}`; on failure, retries once with `--gas $(migration_gas_for_records "$MIGRATION_RECORD_COUNT")`. Before the fallback broadcast, if that gas `gas_exceeds_block_limit` the chain's `block.max_gas`, it aborts with a clear error.

- [ ] **Step 1: Replace the `lumerad_tx` body**

Change `lumerad_tx()` to:

```bash
lumerad_tx() {
  if [[ -z "${CHAIN_ID:-}" ]]; then
    log_error "chain ID is required for tx commands; pass --chain-id or set \$LUMERA_CHAIN_ID / \$CHAIN_ID"
    exit 1
  fi
  _read_keyring_flags

  # Primary: let the chain simulate the exact gas (fees are waived, so the
  # resulting limit is free). Falls back to a record-count formula if the
  # simulate fails (e.g. RPC timeout on a very large validator).
  local out rc
  out="$("$BIN" tx "$@" \
    --node "$NODE" \
    --chain-id "$CHAIN_ID" \
    "${_KRF[@]}" \
    --gas auto --gas-adjustment "$MIGRATION_GAS_ADJUSTMENT" \
    --output json 2>&1)"
  rc=$?
  if (( rc == 0 )); then
    printf '%s\n' "$out"
    return 0
  fi

  log_warn "  --gas auto failed (rc=${rc}); falling back to record-count gas formula"
  local fallback_gas block_max_gas
  fallback_gas="$(migration_gas_for_records "${MIGRATION_RECORD_COUNT:-0}")"
  block_max_gas="$(lumerad_q consensus params 2>/dev/null | jq -r '.params.block.max_gas // .block.max_gas // "-1"' 2>/dev/null || echo -1)"
  if gas_exceeds_block_limit "$fallback_gas" "$block_max_gas"; then
    log_error "estimated migration gas ${fallback_gas} exceeds block max_gas ${block_max_gas}"
    log_error "this account/validator has too many delegation records to migrate in a single tx"
    exit 1
  fi
  "$BIN" tx "$@" \
    --node "$NODE" \
    --chain-id "$CHAIN_ID" \
    "${_KRF[@]}" \
    --gas "$fallback_gas" \
    --output json
}
```

- [ ] **Step 2: Set `MIGRATION_RECORD_COUNT` in `migrate-validator.sh`**

In `scripts/migrate-validator.sh`, where `total` is computed from the estimate (the cap check, the block summing `val_delegation_count + val_unbonding_count + val_redelegation_count`), export it for `lumerad_tx`:

```bash
  # Record count drives the gas fallback (see lumerad_tx).
  MIGRATION_RECORD_COUNT="$total"
```

Place this assignment after `total` is computed and before `preview_tx_body` / the broadcast.

- [ ] **Step 3: Set `MIGRATION_RECORD_COUNT` in `migrate-account.sh` and `migrate-multisig.sh`**

In each, after the pre-flight estimate is fetched, sum the relevant counts and assign. For `migrate-account.sh` (delegator-side records):

```bash
  MIGRATION_RECORD_COUNT="$(jq -r '
      ((.delegation_count    // 0) | tonumber)
    + ((.unbonding_count     // 0) | tonumber)
    + ((.redelegation_count  // 0) | tonumber)
  ' <<<"$estimate")"
```

Use the field names present in that script's estimate JSON (verify with `lumerad query evmigration migration-estimate <addr> -o json | jq`). If a count field is absent, the `// 0` default keeps it safe. For `migrate-multisig.sh`, mirror this using its estimate variable.

- [ ] **Step 4: Verify scripts still parse + shellcheck**

Run: `bash -n scripts/evmigration-common.sh scripts/migrate-validator.sh scripts/migrate-account.sh scripts/migrate-multisig.sh`
Expected: no output (all parse).
Run: `shellcheck -S error scripts/evmigration-common.sh scripts/migrate-validator.sh scripts/migrate-account.sh scripts/migrate-multisig.sh`
Expected: no errors.

- [ ] **Step 5: Commit**

```bash
git add scripts/evmigration-common.sh scripts/migrate-validator.sh scripts/migrate-account.sh scripts/migrate-multisig.sh
git commit -m "feat(evmigration): auto-size migration gas (--gas auto + record-count fallback)"
```

---

### Task 3: Validator-migration section in the user guide

**Files:**
- Modify: `docs/evm-integration/user-guides/migration.md`

- [ ] **Step 1: Add the section**

Add a `## Validator migration` section covering, in prose + a short list:

```markdown
## Validator migration

Migrating a validator's operator account (legacy coin-type 118 → EVM coin-type
60) changes its valoper address, so the chain re-keys **every delegation,
unbonding, and redelegation** pointing at the validator from the old valoper to
the new one. The work — and therefore the gas — scales with the validator's
record count.

- **Stop the node first.** The migration requires the validator node to be
  stopped before broadcasting (`--i-have-stopped-the-node`). The validator will
  miss blocks during the migration and may be jailed; unjail it afterward
  (`lumerad tx slashing unjail`).
- **Fees are waived.** Migration txs pay no fee, so the gas value is only an
  execution limit. `scripts/migrate-validator.sh` sets it automatically.
- **Gas formula** (if submitting by hand):
  `gas ≈ 200000 + 7000 × (delegations + unbondings + redelegations)`.
- **Size limit.** This gas must stay under the chain's block `max_gas` (25M ⇒
  roughly 3500 records). The `max_validator_delegations` parameter (default
  2500) enforces this with margin; a validator above the cap cannot migrate in a
  single tx.
```

- [ ] **Step 2: Commit**

```bash
git add docs/evm-integration/user-guides/migration.md
git commit -m "docs(evm): document validator migration (gas, downtime, cap)"
```

---

### Task 4: Shell-script guide note

**Files:**
- Modify: the migration shell-script guide — `docs/design/evmigration-scripts-design.md` (referenced from the script headers). If a user-facing shell guide exists under `docs/evm-integration/`, update that instead; otherwise update the design doc's usage section.

- [ ] **Step 1: Add the gas note**

Document that the scripts now size gas automatically:

```markdown
### Gas

The migration scripts no longer use the CLI default (200000) gas. `lumerad_tx`
broadcasts with `--gas auto --gas-adjustment 1.5` (the chain simulates the exact
gas; migration fees are waived so this is free). If the simulate fails — e.g. an
RPC timeout on a validator with a very large delegation set — it falls back to a
record-count formula, `200000 + 7000 × records`, and aborts if that would exceed
the chain's block `max_gas`. Override the constants via `MIGRATION_GAS_BASE`,
`MIGRATION_GAS_PER_RECORD`, `MIGRATION_GAS_ADJUSTMENT`.
```

- [ ] **Step 2: Commit**

```bash
git add docs/design/evmigration-scripts-design.md
git commit -m "docs(evmigration): note auto gas-sizing in the scripts guide"
```

---

### Task 5: Calibrate `MIGRATION_GAS_PER_RECORD` (live)

**Files:**
- Modify (if needed): `scripts/evmigration-common.sh` (constant)

- [ ] **Step 1: Migrate a within-cap validator and read gas_used**

On a chain with a realistic-size validator (≤ cap; e.g. a freshly created devnet validator with a known small number of delegators, or testnet), run `migrate-validator.sh` and capture the committed tx's `gas_used` and the record count.

Run: `lumerad query tx <hash> -o json | jq '{gas_used, gas_wanted}'`

- [ ] **Step 2: Compare to the formula and adjust**

Compute `per_record ≈ (gas_used - 200000) / records`. If it differs materially from 7000, update `MIGRATION_GAS_PER_RECORD` and re-run Task 1's test with the new expected values.

- [ ] **Step 3: Commit (only if the constant changed)**

```bash
git add scripts/evmigration-common.sh scripts/evmigration-common_test.sh
git commit -m "chore(evmigration): calibrate gas-per-record from measured migration"
```

---

## Self-Review

**Spec coverage:** gas auto+fallback (Tasks 1-2) ✓; reference formula (Task 1 constants + Task 3 docs) ✓; clamp to block max_gas (Task 2) ✓; user migration guide (Task 3) ✓; shell-script guide (Task 4) ✓; calibration open item (Task 5) ✓; cap unchanged (Global Constraints) ✓.

**Placeholder scan:** field names in Task 3 (account/multisig estimate) are flagged to verify against live JSON — acceptable (the `// 0` defaults make it safe); no TBD/TODO code steps.

**Type consistency:** `migration_gas_for_records`, `gas_exceeds_block_limit`, `MIGRATION_RECORD_COUNT`, `MIGRATION_GAS_*` used consistently across Tasks 1-2 and the test. `11379000 = 200000 + 7000×1597` and `17700000 = 200000 + 7000×2500` verified.
