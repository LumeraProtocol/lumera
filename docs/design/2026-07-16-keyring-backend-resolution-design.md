# Keyring-backend auto-resolution for evmigration scripts

**Date:** 2026-07-16
**Status:** Approved (design)
**Scope:** `scripts/evmigration-common.sh`, `scripts/migrate-account.sh`,
`scripts/migrate-validator.sh`, `scripts/migrate-multisig.sh`,
`tests/scripts/*.bats`

## Motivation

The migration scripts default `KEYRING_BACKEND` to `test` when the operator
does not pass `--keyring-backend`. This disagrees with bare `lumerad`, whose
built-in default is `os` (Cosmos SDK v0.53.6,
`client/config/config.go:15`). A real validator operator whose keys live in an
`os` or `file` keyring, running a migration script without the flag, gets
`--keyring-backend test` forced onto every `lumerad` call — so their key is not
found, or the wrong (empty) keyring is consulted.

Operators already express their keyring choice once, in
`<home>/config/client.toml` (`keyring-backend = "..."`). The scripts should
honor that instead of overriding it with a hardcoded `test`.

### Related, already-fixed issue

A separate bug in the same area was fixed on this branch
(`fix/evmigration-os-keyring-prompt`): the passphrase-prompt tee was gated on
`KEYRING_BACKEND == "file"`, so an `os` backend (which falls back to the
encrypted file store on a headless host and prompts identically) hit the
`else` branch that redirected stderr to a temp file, hiding the prompt and
appearing to hang. That fix introduced `_keyring_prompts_for_passphrase`
(`!= "test"`). This design builds on it.

## Resolution order

`resolve_keyring_backend` picks the effective backend, first hit wins:

1. **`--keyring-backend` flag** (explicit) — unchanged behavior.
1a. **`$LUMERA_KEYRING_BACKEND`** (added post-review, PR #193): the same env
   override `lumerad` itself honors, and the script convention shared with
   `$LUMERA_NODE` / `$LUMERA_CHAIN_ID`. Must outrank `client.toml`/disk
   because the resolved value is passed to `lumerad` as an explicit flag,
   which would otherwise override the env for the child process.
2. **`client.toml`** — `keyring-backend = "..."` read from
   `<home>/config/client.toml`. `--home` selects the home; `--keyring-dir`
   does **not** move `client.toml`.
3. **On-disk detection** — under `--keyring-dir` (else `--home`):
   `keyring-test/` present → `test`; `keyring-file/` present → `file`
   (Cosmos SDK subdir names, `crypto/keyring/keyring.go:42-43`). `os` is not
   detectable on disk (it uses the OS secret store), so it is not inferred
   here.
4. **`os`** — final fallback, matching the SDK default.

The chosen value and its source are logged (e.g.
`keyring backend: os (from client.toml)`), mirroring `resolve_chain_id`, so the
operator sees the decision before anything signs.

## Section A — Resolution mechanism (common lib)

New standalone helper in `evmigration-common.sh`, called by each entry script
after flag parsing (same pattern as `resolve_chain_id`, which is why
`parse_common_flags` stays a pure parser with no filesystem I/O):

```bash
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
    KEYRING_BACKEND="test"; log_info "keyring backend: test (detected keyring-test/ in $kr)"; return 0
  fi
  if [[ -d "$kr/keyring-file" ]]; then
    KEYRING_BACKEND="file"; log_info "keyring backend: file (detected keyring-file/ in $kr)"; return 0
  fi
  KEYRING_BACKEND="os"
  log_info "keyring backend: os (SDK default; no --keyring-backend, client.toml, or keyring dir found)"
}
```

`parse_common_flags` change:

- Add global `KEYRING_BACKEND_EXPLICIT` (declared with the other globals, reset
  to `0` at the top of `parse_common_flags`).
- In the `--keyring-backend` case, set `KEYRING_BACKEND_EXPLICIT=1` in addition
  to assigning the value.
- Keep the `KEYRING_BACKEND="test"` default as the pre-resolution baseline;
  resolution only occurs when an entry script calls `resolve_keyring_backend`,
  so the existing `parse_common_flags populates defaults` test is unaffected.

Entry-script wiring: `migrate-account.sh` and `migrate-validator.sh` call
`resolve_keyring_backend` immediately after `parse_common_flags` (before
`resolve_address`, so the resolved backend is used for key lookups).

### Trusting the client.toml value

Any non-empty value read from `client.toml` is honored verbatim (the operator
configured it; `lumerad` itself would use the same value). No allow-list check
— an invalid value would be rejected by `lumerad` downstream with its own
error, same as running `lumerad` directly.

## Section B — Multisig integration

`migrate-multisig.sh` does not use `parse_common_flags`; it has three
subcommand parsers (around lines 39, 260, 545), each with
`local keyring_backend="test"`. For each parser:

1. Track whether `--keyring-backend` was passed (local `kb_explicit`).
2. After parsing, set the globals the resolver reads: `KEYRING_BACKEND`,
   `KEYRING_BACKEND_EXPLICIT`, `HOME_DIR`, `KEYRING_DIR`.
3. Call `resolve_keyring_backend`.
4. Read the resolved `KEYRING_BACKEND` back into the local `keyring_backend`.

The subcommands that re-invoke `migrate-account.sh` / `migrate-validator.sh`
already pass `--keyring-backend "$keyring_backend"`; with the resolved value
this becomes an explicit flag to the child, so the child does not re-resolve.

## Section C — Related cleanup

Replace the two entry-script advisory guards
`[[ "$KEYRING_BACKEND" == "file" ]]`
(`migrate-validator.sh:70`, `migrate-account.sh:56`) with
`_keyring_prompts_for_passphrase`, so the "keyring may prompt; input hidden"
hint is shown for `os` as well as `file`.

## Section D — Testing (bats) and coverage audit

The canonical suite is `tests/scripts/*.bats`, wired into the Makefile
(`bats tests/scripts/`). `scripts/evmigration-common_test.sh` is a stale
hand-rolled duplicate that nothing runs.

1. **Reconcile:** delete `scripts/evmigration-common_test.sh` and revert the
   `_keyring_prompts_for_passphrase` cases that were added to it, porting that
   coverage into `tests/scripts/common.bats`.
2. **New `common.bats` tests:**
   - `_keyring_prompts_for_passphrase`: `test` → false, `file` → true,
     `os` → true, unset → false.
   - `resolve_keyring_backend`:
     - explicit flag wins (ignores `client.toml`);
     - reads value from a seeded `client.toml`;
     - `client.toml` takes precedence over an on-disk `keyring-test/`;
     - disk detection: `keyring-test/` → `test`, `keyring-file/` → `file`
       (no `client.toml`);
     - `--keyring-dir` is used for detection when set;
     - `client.toml` is read from `--home` even when `--keyring-dir` differs;
     - empty home → `os` fallback.
3. **Migration-script coverage gaps (audit result), proposed additions:**
   - **`os`-backend prompt regression** (the originating bug): assert the sign
     step tees the passphrase prompt to the terminal rather than hiding it.
     Highest-value gap — currently unguarded.
   - Non-`test` backend end-to-end: one dry-run case each in
     `migrate-account.bats` / `migrate-validator.bats` with a seeded
     `client.toml`, confirming resolution flows through.
   - `migrate-multisig.bats`: a backend-resolution case covering Section B.

## Edge cases

- `--keyring-dir` set, `--home` unset: `client.toml` read from
  `$HOME/.lumera/config/`; detection from the keyring dir. Matches SDK
  behavior.
- CI/devnet that imports keys into a `test` keyring first creates a
  `keyring-test/` dir, so detection returns `test` before the `os` fallback is
  reached — existing scripted `test` flows are preserved without passing the
  flag.
- Truly empty home with no flag and no `client.toml`: resolves to `os`
  (accepted decision), which then uses the already-hardened passphrase-prompt
  path.

## Out of scope

- Changing `lumerad`'s own defaults or `client.toml` handling.
- Detecting `os` from disk (not reliably possible).
- Validating/normalizing unusual backends (`kwallet`, `pass`) beyond passing
  them through.
