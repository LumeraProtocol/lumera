# Keyring-backend Auto-Resolution Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the evmigration migration scripts resolve the keyring backend from `--keyring-backend` → `client.toml` → on-disk detection → `os`, instead of hardcoding `test`.

**Architecture:** A new standalone `resolve_keyring_backend` helper in `scripts/evmigration-common.sh` (mirroring the existing `resolve_chain_id` idiom) is called by each entry script after flag parsing. `parse_common_flags` gains a `KEYRING_BACKEND_EXPLICIT` flag so the resolver can tell an explicit `--keyring-backend test` from the default. `migrate-multisig.sh` (which has its own parsers) wires the same resolver into its three subcommands. Test coverage is consolidated into the Makefile-wired `tests/scripts/*.bats` suite and the stale hand-rolled `scripts/evmigration-common_test.sh` is removed.

**Tech Stack:** Bash, bats 1.2.1 (`tests/scripts/`), shellcheck, Cosmos SDK v0.53.6 keyring/client-config conventions.

**Spec:** `docs/design/2026-07-16-keyring-backend-resolution-design.md`

## Global Constraints

- Target shell: bash; scripts set `set -euo pipefail` and `IFS=$'\n\t'` — any new global must be initialized at source time so `-u` never trips.
- `client.toml` lives at `<home>/config/client.toml`; `--keyring-dir` does NOT move it. Home default when `--home` unset: `$HOME/.lumera`.
- On-disk keyring subdirs (Cosmos SDK): `test` → `keyring-test/`, `file` → `keyring-file/`. `os` is not detectable on disk.
- Final fallback backend: `os` (matches bare `lumerad`).
- Resolution logs its choice and source via `log_info` to stderr, like `resolve_chain_id`.
- Canonical test suite: `tests/scripts/*.bats`, run by `bats tests/scripts/` (Makefile). `scripts/evmigration-common_test.sh` is stale and unwired.
- `make lint` must pass cleanly (shellcheck `-x` on `${MIGRATION_SCRIPTS}` + golangci-lint). 0 issues.
- Existing helper `_keyring_prompts_for_passphrase` (`!= "test"`) already exists in `evmigration-common.sh` (committed on this branch).

---

### Task 1: Consolidate helper unit tests into bats; delete stale test file

Moves the gas-helper tests (currently only in the stale file) and the `_keyring_prompts_for_passphrase` tests into the canonical `common.bats`, then deletes the stale duplicate. No production code changes — pure test-coverage move, so the ported tests pass immediately (they document already-shipped behavior).

**Files:**
- Modify: `tests/scripts/common.bats` (append new `@test` blocks before EOF)
- Delete: `scripts/evmigration-common_test.sh`

**Interfaces:**
- Consumes: `migration_gas_for_records`, `gas_exceeds_block_limit`, `_keyring_prompts_for_passphrase` (all already defined in `evmigration-common.sh`).
- Produces: nothing consumed by later tasks.

- [ ] **Step 1: Append the ported tests to `common.bats`**

Add at the end of `tests/scripts/common.bats`:

```bash
@test "migration_gas_for_records: base with zero records" {
  [ "$(migration_gas_for_records 0)" = "6000000" ]
}

@test "migration_gas_for_records: base plus per-record marginal" {
  [ "$(migration_gas_for_records 1597)" = "2401500000" ]
  [ "$(migration_gas_for_records 2500)" = "3756000000" ]
}

@test "gas_exceeds_block_limit: true only when over a positive limit" {
  run gas_exceeds_block_limit 30000000 25000000
  [ "$status" -eq 0 ]
  run gas_exceeds_block_limit 11379000 25000000
  [ "$status" -eq 1 ]
  run gas_exceeds_block_limit 99999999 -1
  [ "$status" -eq 1 ]
  run gas_exceeds_block_limit 99999999 ""
  [ "$status" -eq 1 ]
}

@test "_keyring_prompts_for_passphrase: test backend is silent" {
  KEYRING_BACKEND=test
  run _keyring_prompts_for_passphrase
  [ "$status" -eq 1 ]
}

@test "_keyring_prompts_for_passphrase: file and os backends prompt" {
  KEYRING_BACKEND=file
  run _keyring_prompts_for_passphrase
  [ "$status" -eq 0 ]
  KEYRING_BACKEND=os
  run _keyring_prompts_for_passphrase
  [ "$status" -eq 0 ]
}

@test "_keyring_prompts_for_passphrase: unset defaults to silent" {
  unset KEYRING_BACKEND
  run _keyring_prompts_for_passphrase
  [ "$status" -eq 1 ]
}
```

- [ ] **Step 2: Delete the stale hand-rolled test file**

Run: `git rm scripts/evmigration-common_test.sh`
Expected: file staged for deletion.

- [ ] **Step 3: Run the bats suite to verify the ported tests pass**

Run: `bats tests/scripts/common.bats`
Expected: PASS — all tests green, including the 6 new blocks.

- [ ] **Step 4: Run shellcheck on scripts to confirm nothing references the deleted file**

Run: `make lint`
Expected: `0 issues.` (The Makefile's `MIGRATION_SCRIPTS` list must no longer include the deleted file — if shellcheck errors that the file is missing, remove it from the `MIGRATION_SCRIPTS` variable in the `Makefile` and re-run.)

- [ ] **Step 5: Commit**

```bash
git add tests/scripts/common.bats
git commit -m "test(evmigration): consolidate helper tests into bats; drop stale test file"
```

---

### Task 2: Add `resolve_keyring_backend` and explicit-flag tracking

Adds the resolver to the common lib and the `KEYRING_BACKEND_EXPLICIT` global that lets it distinguish an explicit flag from the default.

**Files:**
- Modify: `scripts/evmigration-common.sh` (globals block ~line 40; `parse_common_flags`; new function after `_keyring_prompts_for_passphrase`)
- Test: `tests/scripts/common.bats`

**Interfaces:**
- Consumes: globals `KEYRING_BACKEND`, `HOME_DIR`, `KEYRING_DIR`, `log_info`.
- Produces:
  - global `KEYRING_BACKEND_EXPLICIT` (int, `0`/`1`; initialized to `0` at source time and reset in `parse_common_flags`; set to `1` when `--keyring-backend` is parsed).
  - `resolve_keyring_backend()` — reads the globals above and sets `KEYRING_BACKEND` to the resolved value (`test`/`file`/`os`/verbatim client.toml value). No args, no stdout, logs to stderr.

- [ ] **Step 1: Write the failing tests in `common.bats`**

Append to `tests/scripts/common.bats`:

```bash
@test "resolve_keyring_backend: explicit flag wins over client.toml" {
  local home; home=$(mktemp -d)
  mkdir -p "$home/config"
  printf 'keyring-backend = "file"\n' > "$home/config/client.toml"
  KEYRING_BACKEND=os
  KEYRING_BACKEND_EXPLICIT=1
  HOME_DIR="$home"
  KEYRING_DIR=""
  resolve_keyring_backend
  [ "$KEYRING_BACKEND" = "os" ]
}

@test "resolve_keyring_backend: reads value from client.toml" {
  local home; home=$(mktemp -d)
  mkdir -p "$home/config"
  printf 'chain-id = "x"\nkeyring-backend = "file"\noutput = "text"\n' > "$home/config/client.toml"
  KEYRING_BACKEND=test
  KEYRING_BACKEND_EXPLICIT=0
  HOME_DIR="$home"
  KEYRING_DIR=""
  resolve_keyring_backend
  [ "$KEYRING_BACKEND" = "file" ]
}

@test "resolve_keyring_backend: client.toml wins over on-disk keyring-test dir" {
  local home; home=$(mktemp -d)
  mkdir -p "$home/config" "$home/keyring-test"
  printf 'keyring-backend = "os"\n' > "$home/config/client.toml"
  KEYRING_BACKEND=test
  KEYRING_BACKEND_EXPLICIT=0
  HOME_DIR="$home"
  KEYRING_DIR=""
  resolve_keyring_backend
  [ "$KEYRING_BACKEND" = "os" ]
}

@test "resolve_keyring_backend: detects test from keyring-test dir" {
  local home; home=$(mktemp -d)
  mkdir -p "$home/keyring-test"
  KEYRING_BACKEND=""
  KEYRING_BACKEND_EXPLICIT=0
  HOME_DIR="$home"
  KEYRING_DIR=""
  resolve_keyring_backend
  [ "$KEYRING_BACKEND" = "test" ]
}

@test "resolve_keyring_backend: detects file from keyring-file dir" {
  local home; home=$(mktemp -d)
  mkdir -p "$home/keyring-file"
  KEYRING_BACKEND=""
  KEYRING_BACKEND_EXPLICIT=0
  HOME_DIR="$home"
  KEYRING_DIR=""
  resolve_keyring_backend
  [ "$KEYRING_BACKEND" = "file" ]
}

@test "resolve_keyring_backend: uses --keyring-dir for detection" {
  local home kr; home=$(mktemp -d); kr=$(mktemp -d)
  mkdir -p "$kr/keyring-file"
  KEYRING_BACKEND=""
  KEYRING_BACKEND_EXPLICIT=0
  HOME_DIR="$home"
  KEYRING_DIR="$kr"
  resolve_keyring_backend
  [ "$KEYRING_BACKEND" = "file" ]
}

@test "resolve_keyring_backend: client.toml read from --home even when keyring-dir differs" {
  local home kr; home=$(mktemp -d); kr=$(mktemp -d)
  mkdir -p "$home/config"
  printf 'keyring-backend = "file"\n' > "$home/config/client.toml"
  KEYRING_BACKEND=""
  KEYRING_BACKEND_EXPLICIT=0
  HOME_DIR="$home"
  KEYRING_DIR="$kr"
  resolve_keyring_backend
  [ "$KEYRING_BACKEND" = "file" ]
}

@test "resolve_keyring_backend: empty home falls back to os" {
  local home; home=$(mktemp -d)
  KEYRING_BACKEND=""
  KEYRING_BACKEND_EXPLICIT=0
  HOME_DIR="$home"
  KEYRING_DIR=""
  resolve_keyring_backend
  [ "$KEYRING_BACKEND" = "os" ]
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `bats tests/scripts/common.bats -f resolve_keyring_backend`
Expected: FAIL — `resolve_keyring_backend: command not found` (and/or `KEYRING_BACKEND_EXPLICIT: unbound variable`).

- [ ] **Step 3: Add the `KEYRING_BACKEND_EXPLICIT` global**

In `scripts/evmigration-common.sh`, in the globals block near the other `# shellcheck disable=SC2034` declarations (just after the `KEYRING_BACKEND="test"` global around line 32), add:

```bash
# shellcheck disable=SC2034
# 1 once --keyring-backend is passed explicitly; gates resolve_keyring_backend.
KEYRING_BACKEND_EXPLICIT=0
```

- [ ] **Step 4: Reset and set the flag in `parse_common_flags`**

In `parse_common_flags`, in the reset block (where `KEYRING_BACKEND="test"` is reassigned), add right after it:

```bash
  KEYRING_BACKEND_EXPLICIT=0
```

Then change the `--keyring-backend` case from:

```bash
      --keyring-backend) _require_value "$1" "$#" "${2-}"; KEYRING_BACKEND="$2"; shift 2 ;;
```

to:

```bash
      --keyring-backend) _require_value "$1" "$#" "${2-}"; KEYRING_BACKEND="$2"; KEYRING_BACKEND_EXPLICIT=1; shift 2 ;;
```

- [ ] **Step 5: Implement `resolve_keyring_backend`**

In `scripts/evmigration-common.sh`, immediately after the `_keyring_prompts_for_passphrase` function, add:

```bash
# resolve_keyring_backend
# Pin the effective keyring backend and log its source, when the user did not
# pass --keyring-backend. Resolution order (first hit wins):
#   1. explicit --keyring-backend (KEYRING_BACKEND_EXPLICIT=1)
#   2. keyring-backend from <home>/config/client.toml (--home selects home;
#      --keyring-dir does NOT move client.toml)
#   3. on-disk detection under --keyring-dir (else --home):
#      keyring-test/ -> test, keyring-file/ -> file (os is not on-disk-detectable)
#   4. os — the Cosmos SDK default
# Mirrors resolve_chain_id: logs the decision so the operator sees it before signing.
resolve_keyring_backend() {
  if (( KEYRING_BACKEND_EXPLICIT == 1 )); then
    log_info "keyring backend: $KEYRING_BACKEND (from --keyring-backend)"
    return 0
  fi

  local home="${HOME_DIR:-$HOME/.lumera}"
  local client_toml="$home/config/client.toml" v
  if [[ -f "$client_toml" ]]; then
    v=$(sed -n 's/^[[:space:]]*keyring-backend[[:space:]]*=[[:space:]]*"\([^"]*\)".*/\1/p' \
        "$client_toml" | head -n1)
    if [[ -n "$v" ]]; then
      KEYRING_BACKEND="$v"
      log_info "keyring backend: $v (from $client_toml)"
      return 0
    fi
  fi

  local kr="${KEYRING_DIR:-$home}"
  if [[ -d "$kr/keyring-test" ]]; then
    KEYRING_BACKEND="test"
    log_info "keyring backend: test (detected keyring-test/ in $kr)"
    return 0
  fi
  if [[ -d "$kr/keyring-file" ]]; then
    KEYRING_BACKEND="file"
    log_info "keyring backend: file (detected keyring-file/ in $kr)"
    return 0
  fi

  KEYRING_BACKEND="os"
  log_info "keyring backend: os (SDK default; no --keyring-backend, client.toml, or keyring dir found)"
}
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `bats tests/scripts/common.bats -f resolve_keyring_backend`
Expected: PASS — all 8 `resolve_keyring_backend` tests green.

- [ ] **Step 7: Run the full common suite + lint**

Run: `bats tests/scripts/common.bats && make lint`
Expected: all common.bats tests PASS (including the unchanged `parse_common_flags populates defaults`); `0 issues.`

- [ ] **Step 8: Commit**

```bash
git add scripts/evmigration-common.sh tests/scripts/common.bats
git commit -m "feat(evmigration): resolve keyring backend from client.toml/disk, default os"
```

---

### Task 3: Wire resolver into account/validator scripts + advisory cleanup

Calls `resolve_keyring_backend` in the two `parse_common_flags`-based entry scripts and generalizes their `== "file"` advisory guards. Adds an end-to-end resolution test and a `lumerad_tx` branch regression test.

**Files:**
- Modify: `scripts/migrate-account.sh` (after `parse_common_flags`; advisory guard ~line 56)
- Modify: `scripts/migrate-validator.sh` (after `parse_common_flags`; advisory guard ~line 70)
- Test: `tests/scripts/migrate-account.bats`, `tests/scripts/migrate-validator.bats`, `tests/scripts/common.bats`

**Interfaces:**
- Consumes: `resolve_keyring_backend`, `_keyring_prompts_for_passphrase` (Task 2 / existing).
- Produces: nothing new consumed by later tasks.

- [ ] **Step 1: Write the failing e2e + regression tests**

Append to `tests/scripts/migrate-account.bats`:

```bash
@test "migrate-account.sh resolves keyring backend from client.toml" {
  local home; home=$(mktemp -d)
  mkdir -p "$home/config"
  printf 'keyring-backend = "file"\n' > "$home/config/client.toml"
  run "$SCRIPTS_DIR/migrate-account.sh" \
    --binary "$SHIM" --chain-id shim-test --home "$home" \
    --dry-run --yes legacykey newkey
  [ "$status" -eq 0 ]
  [[ "$output" == *"keyring backend: file (from $home/config/client.toml)"* ]]
}
```

Append to `tests/scripts/common.bats`:

```bash
@test "lumerad_tx announces keyring unlock for a prompting backend" {
  setup_shim
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    BIN='"$SHIM_BIN"'
    NODE="tcp://example:1234"
    CHAIN_ID="shim-test"
    KEYRING_BACKEND="os"
    KEYRING_BACKEND_EXPLICIT=1
    lumerad_tx evmigration migrate-validator k1 k2 --yes 2>&1 1>/dev/null || true
  '
  [[ "$output" == *"unlocking keyring to sign and simulate"* ]]
}

@test "lumerad_tx stays quiet about unlock for the test backend" {
  setup_shim
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    BIN='"$SHIM_BIN"'
    NODE="tcp://example:1234"
    CHAIN_ID="shim-test"
    KEYRING_BACKEND="test"
    KEYRING_BACKEND_EXPLICIT=1
    lumerad_tx evmigration migrate-validator k1 k2 --yes 2>&1 1>/dev/null || true
  '
  [[ "$output" != *"unlocking keyring to sign and simulate"* ]]
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `bats tests/scripts/migrate-account.bats -f "resolves keyring backend" && bats tests/scripts/common.bats -f "lumerad_tx announces"`
Expected: the migrate-account test FAILS (no `keyring backend:` line — resolver not called yet). The `lumerad_tx announces` test PASSES already (the committed fix uses `_keyring_prompts_for_passphrase`); the `stays quiet` test PASSES too. (These two are regression guards for shipped behavior — that they pass now is correct.)

- [ ] **Step 3: Call `resolve_keyring_backend` in `migrate-account.sh`**

In `scripts/migrate-account.sh`, immediately after `parse_common_flags "$@"` (before `log_run_summary`), add:

```bash
  resolve_keyring_backend
```

- [ ] **Step 4: Generalize the account advisory guard**

In `scripts/migrate-account.sh`, change:

```bash
  if [[ "$KEYRING_BACKEND" == "file" ]]; then
    log_info "the encrypted keyring may prompt once for each key; input is hidden while typing"
  fi
```

to:

```bash
  if _keyring_prompts_for_passphrase; then
    log_info "the encrypted keyring may prompt once for each key; input is hidden while typing"
  fi
```

- [ ] **Step 5: Repeat Steps 3–4 for `migrate-validator.sh`**

In `scripts/migrate-validator.sh`, add `resolve_keyring_backend` immediately after `parse_common_flags "$@"`, and change the identical `[[ "$KEYRING_BACKEND" == "file" ]]` advisory guard (near line 70) to `if _keyring_prompts_for_passphrase; then`.

- [ ] **Step 6: Run the affected suites**

Run: `bats tests/scripts/migrate-account.bats tests/scripts/migrate-validator.bats tests/scripts/common.bats`
Expected: PASS — including the new migrate-account resolution test now that the resolver is wired in.

- [ ] **Step 7: Lint**

Run: `make lint`
Expected: `0 issues.`

- [ ] **Step 8: Commit**

```bash
git add scripts/migrate-account.sh scripts/migrate-validator.sh tests/scripts/migrate-account.bats tests/scripts/common.bats
git commit -m "feat(evmigration): auto-resolve keyring backend in account/validator scripts"
```

---

### Task 4: Wire resolver into `migrate-multisig.sh`

`migrate-multisig.sh` has three subcommand parsers, each with `local keyring_backend="test"` and its own `--keyring-backend` flag. Each must resolve when the flag is absent.

**Files:**
- Modify: `scripts/migrate-multisig.sh` (three parsers ~lines 39, 260, 545; and the point after each where `KEYRING_BACKEND="$keyring_backend"` is set ~lines 114, 338, 620)
- Test: `tests/scripts/migrate-multisig.bats`

**Interfaces:**
- Consumes: `resolve_keyring_backend`, globals `KEYRING_BACKEND`, `KEYRING_BACKEND_EXPLICIT`, `HOME_DIR`, `KEYRING_DIR`.
- Produces: nothing consumed by later tasks.

- [ ] **Step 1: Write the failing test**

Append to `tests/scripts/migrate-multisig.bats` (match the invocation style already used in that file — inspect an existing `@test` there for the exact subcommand, shim env vars, and required flags, and mirror it; the assertion below is the new part):

```bash
@test "migrate-multisig.sh resolves keyring backend from client.toml" {
  local home; home=$(mktemp -d)
  mkdir -p "$home/config"
  printf 'keyring-backend = "file"\n' > "$home/config/client.toml"
  # Use the same subcommand + shim setup as the existing dry-run/preview test
  # in this file, adding: --home "$home" and NO --keyring-backend flag.
  run "$SCRIPTS_DIR/migrate-multisig.sh" <same-subcommand-and-flags-as-existing-test> \
    --home "$home"
  [[ "$output" == *"keyring backend: file (from $home/config/client.toml)"* ]]
}
```

Note: before writing this test, open `tests/scripts/migrate-multisig.bats` and copy the exact subcommand name, shim binary flag (`--binary "$SHIM"` or the file's local equivalent), and any `SHIM_*`/fixture env vars from the lightest-weight existing passing test, so this test reaches the resolver without tripping unrelated validation.

- [ ] **Step 2: Run to verify it fails**

Run: `bats tests/scripts/migrate-multisig.bats -f "resolves keyring backend"`
Expected: FAIL — no `keyring backend: file (from ...)` line in output.

- [ ] **Step 3: Add explicit-flag tracking to each parser**

In `scripts/migrate-multisig.sh`, for each of the three parsers, change the local default:

```bash
  local keyring_backend="test" keyring_dir="" home_dir=""
```

to add an explicit tracker:

```bash
  local keyring_backend="test" keyring_dir="" home_dir="" kb_explicit=0
```

and in each parser's `--keyring-backend` case, set the tracker:

```bash
      --keyring-backend)  _require_value "$1" "$#" "${2-}"; keyring_backend="$2"; kb_explicit=1; shift 2 ;;
```

- [ ] **Step 4: Resolve after each parser populates its locals**

In each of the three flows, at the point where `KEYRING_BACKEND="$keyring_backend"` is set (and where `keyring_dir`/`home_dir` are known), replace that single assignment with:

```bash
  KEYRING_BACKEND="$keyring_backend"
  KEYRING_BACKEND_EXPLICIT="$kb_explicit"
  HOME_DIR="$home_dir"
  KEYRING_DIR="$keyring_dir"
  resolve_keyring_backend
  keyring_backend="$KEYRING_BACKEND"
```

This keeps the local `keyring_backend` (used later to build `--keyring-backend "$keyring_backend"` args for sub-invocations and sub-lumerad calls) equal to the resolved value, so re-invoked child scripts receive it as an explicit flag and do not re-resolve.

- [ ] **Step 5: Run the multisig suite**

Run: `bats tests/scripts/migrate-multisig.bats`
Expected: PASS — the new resolution test plus all existing multisig tests (which pass `--keyring-backend` explicitly and so still see their chosen backend).

- [ ] **Step 6: Lint**

Run: `make lint`
Expected: `0 issues.`

- [ ] **Step 7: Commit**

```bash
git add scripts/migrate-multisig.sh tests/scripts/migrate-multisig.bats
git commit -m "feat(evmigration): auto-resolve keyring backend in migrate-multisig subcommands"
```

---

### Task 5: Full-suite verification

**Files:** none (verification only)

- [ ] **Step 1: Run the entire scripts test suite**

Run: `bats tests/scripts/`
Expected: PASS — every file green.

- [ ] **Step 2: Run lint**

Run: `make lint`
Expected: `0 issues.`

- [ ] **Step 3: Sanity-check the resolution log end-to-end (no flag, no client.toml)**

Run:
```bash
scripts/migrate-account.sh --binary tests/scripts/fixtures/lumerad-shim.sh \
  --chain-id shim-test --home "$(mktemp -d)" --dry-run --yes legacykey newkey 2>&1 \
  | grep 'keyring backend:'
```
Expected: a line `keyring backend: os (SDK default; ...)` — confirming the fallback fires when nothing is configured.

---

## Self-Review

**Spec coverage:**
- Section A (resolver + explicit tracking) → Task 2. ✓
- Section B (multisig three parsers) → Task 4. ✓
- Section C (advisory guard cleanup) → Task 3 Steps 4–5. ✓
- Section D.1 (delete stale file, port coverage) → Task 1. ✓
- Section D.2 (`_keyring_prompts_for_passphrase` + `resolve_keyring_backend` bats) → Task 1 + Task 2. ✓
- Section D.3 (os-prompt regression, non-test e2e, multisig resolution) → Task 3 (regression + account e2e), Task 4 (multisig). Validator e2e is covered by the shared resolver + Task 3 Step 5 wiring; the account e2e test exercises the identical code path. ✓
- Resolution order, `--home` vs `--keyring-dir`, os fallback → Task 2 tests. ✓

**Placeholder scan:** Task 4 Step 1 intentionally references "same subcommand as existing test" with an explicit instruction to copy the exact invocation from the file — this is a lookup instruction, not a code placeholder, because the multisig subcommand surface must be read from the live file to avoid tripping unrelated validation. All other steps contain complete code.

**Type/name consistency:** `resolve_keyring_backend`, `KEYRING_BACKEND_EXPLICIT`, `_keyring_prompts_for_passphrase`, `HOME_DIR`, `KEYRING_DIR`, `KEYRING_BACKEND` used identically across Tasks 2–4. Log strings (`keyring backend: <v> (from <path>)`) match between the implementation (Task 2 Step 5) and the assertions (Tasks 3–4). ✓
