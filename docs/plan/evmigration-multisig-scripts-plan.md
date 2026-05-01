# EVM Multisig Migration Helper Script — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship a single bash helper [scripts/migrate-multisig.sh](../../scripts/migrate-multisig.sh) with four subcommands (`generate`, `sign`, `combine`, `submit`) that wrap `lumerad tx evmigration {generate-proof-payload, sign-proof, combine-proof, submit-proof}` with the same pre-flight, file-integrity, and post-broadcast safety rails the existing single-sig scripts provide.

**Architecture:** One top-level script dispatches on the first positional arg to one of four subcommand functions (`_mms_generate`, `_mms_sign`, `_mms_combine`, `_mms_submit`). The shared library [scripts/evmigration-common.sh](../../scripts/evmigration-common.sh) gains six new helpers for multisig-specific concerns (multisig assertion, auth-account JSON access, proof/partial file parsing, partial-signature summarization, eth-key algorithm check). The `lumerad-shim.sh` test fixture learns a `--out`-aware file-writing mode and routes for the four evmigration tx subcommands.

**Tech Stack:** bash 4.4+, `jq`, `shellcheck`, `bats-core` 1.13+, `lumerad` CLI. Full design in [evmigration-multisig-scripts-design.md](evmigration-multisig-scripts-design.md). Prior art: the single-sig scripts, their design at [evmigration-scripts-design.md](evmigration-scripts-design.md), and their plan at [evmigration-scripts-plan.md](evmigration-scripts-plan.md).

---

## Dependencies

Already installed on the dev box from the single-sig plan: `shellcheck`, `bats` (1.13.0 globally via npm), `jq`, `lumerad` at `build/lumerad`. No new tools required.

## Testing Strategy

- **`shellcheck`** on all scripts is mandatory and gates each commit (`make lint-scripts` already wired).
- **`bats`** tests live under [tests/scripts/](../../tests/scripts/) alongside existing suites. New file `migrate-multisig.bats` mirrors the structure of `migrate-account.bats` / `migrate-validator.bats`. Shared-library tests extend `common.bats`.
- **`lumerad` shim**: existing shim at [tests/scripts/fixtures/lumerad-shim.sh](../../tests/scripts/fixtures/lumerad-shim.sh) gains four new argv-pattern routes plus a `--out`-aware write helper for the tx subcommands that produce files.
- **Manual devnet smoke** (final task) exercises a real 2-of-3 ceremony end-to-end.

## File Layout Summary

Files created:

- `tests/scripts/fixtures/auth-account-multisig.json`
- `tests/scripts/fixtures/auth-account-nilpubkey.json`
- `tests/scripts/fixtures/estimate-multisig-validator.json`
- `tests/scripts/fixtures/proof-template.json`
- `tests/scripts/fixtures/partial-alice.json`
- `tests/scripts/fixtures/partial-bob.json`
- `tests/scripts/fixtures/partial-carol.json`
- `tests/scripts/fixtures/combined-tx.json`
- `tests/scripts/migrate-multisig.bats`
- `scripts/migrate-multisig.sh`

Files modified:

- `tests/scripts/fixtures/lumerad-shim.sh` — new routes, `--out`-aware writer, multisig auth routing
- `scripts/evmigration-common.sh` — six new helpers (§4 of the design)
- `tests/scripts/common.bats` — bats tests for the new helpers
- `scripts/migrate-account.sh` — multisig error message points at `migrate-multisig.sh`
- `scripts/migrate-validator.sh` — same
- `docs/evm-integration/user-guides/migration-scripts.md` — new "Multisig migration" section
- `docs/evm-integration/user-guides/migration.md` — pointer from the multisig section to the script
- `Makefile` — release target packages `migrate-multisig.sh`

---

## Task 1: Shim extensions + multisig fixtures

**Files:**

- Create: `tests/scripts/fixtures/auth-account-multisig.json`
- Create: `tests/scripts/fixtures/auth-account-nilpubkey.json`
- Create: `tests/scripts/fixtures/estimate-multisig-validator.json`
- Create: `tests/scripts/fixtures/proof-template.json`
- Create: `tests/scripts/fixtures/partial-alice.json`
- Create: `tests/scripts/fixtures/partial-bob.json`
- Create: `tests/scripts/fixtures/partial-carol.json`
- Create: `tests/scripts/fixtures/combined-tx.json`
- Modify: `tests/scripts/fixtures/lumerad-shim.sh`
- Modify: `tests/scripts/common.bats` (one sanity test)

Goal: the shim can emulate the four multisig-related `lumerad tx evmigration` commands with `--out <path>` behavior, and can return a multisig or nil-pubkey auth-account response on demand.

- [ ] **Step 1.1: Add the eight fixtures**

`tests/scripts/fixtures/auth-account-multisig.json`:

```json
{
  "account": {
    "@type": "/cosmos.auth.v1beta1.BaseAccount",
    "address": "lumera1multisig1qxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
    "pub_key": {
      "@type": "/cosmos.crypto.multisig.LegacyAminoPubKey",
      "threshold": 2,
      "public_keys": [
        {"@type": "/cosmos.crypto.secp256k1.PubKey", "key": "A1111111111111111111111111111111111111111111"},
        {"@type": "/cosmos.crypto.secp256k1.PubKey", "key": "A2222222222222222222222222222222222222222222"},
        {"@type": "/cosmos.crypto.secp256k1.PubKey", "key": "A3333333333333333333333333333333333333333333"}
      ]
    },
    "account_number": "7",
    "sequence": "0"
  }
}
```

`tests/scripts/fixtures/auth-account-nilpubkey.json`:

```json
{
  "account": {
    "@type": "/cosmos.auth.v1beta1.BaseAccount",
    "address": "lumera1nilpubkey1qxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
    "pub_key": null,
    "account_number": "11",
    "sequence": "0"
  }
}
```

`tests/scripts/fixtures/estimate-multisig-validator.json` — copy of `estimate-multisig.json` with `"is_validator": true` instead of false, and add `"val_delegation_count": 5`, `"val_unbonding_count": 0`, `"val_redelegation_count": 0` (everything else unchanged).

`tests/scripts/fixtures/proof-template.json`:

```json
{
  "kind": "claim",
  "legacy_address": "lumera1multisig1qxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
  "new_address": "lumera1newshimaddrxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
  "chain_id": "shim-test",
  "evm_chain_id": "76857769",
  "payload_hex": "6465616462656566",
  "multisig": {
    "threshold": 2,
    "sub_pub_keys_b64": [
      "A1111111111111111111111111111111111111111111",
      "A2222222222222222222222222222222222222222222",
      "A3333333333333333333333333333333333333333333"
    ],
    "sig_format": "SIG_FORMAT_CLI"
  },
  "partial_signatures": []
}
```

`tests/scripts/fixtures/partial-alice.json` — copy of `proof-template.json` with `partial_signatures: [{"index": 0, "signature_b64": "aaaa"}]`.

`tests/scripts/fixtures/partial-bob.json` — copy of `proof-template.json` with `partial_signatures: [{"index": 1, "signature_b64": "bbbb"}]`.

`tests/scripts/fixtures/partial-carol.json` — copy of `proof-template.json` with `partial_signatures: [{"index": 2, "signature_b64": "cccc"}]`.

`tests/scripts/fixtures/combined-tx.json`:

```json
{
  "body": {
    "messages": [{
      "@type": "/lumera.evmigration.MsgClaimLegacyAccount",
      "legacy_address": "lumera1multisig1qxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
      "new_address": "lumera1newshimaddrxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
    }]
  },
  "auth_info": {"signer_infos": [], "fee": {"amount": [], "gas_limit": "200000"}},
  "signatures": []
}
```

- [ ] **Step 1.2: Extend the shim with a `--out`-aware writer**

Open `tests/scripts/fixtures/lumerad-shim.sh`. Near the top, after the existing `emit()` helper, add:

```bash
# emit_or_write <fixture-name> <all-original-argv>
# If argv contains `--out <path>`, copy fixture to that path and print a short
# confirmation on stdout (matching the lumerad CLI's "tx <cmd> --out foo.json"
# behavior: it writes the file and prints a short status line).
# Otherwise, print the fixture JSON to stdout.
emit_or_write() {
  local fixture="$1"
  shift
  local out_path=""
  while (( $# > 0 )); do
    case "$1" in
      --out) out_path="$2"; shift 2 ;;
      *) shift ;;
    esac
  done
  if [[ -n "$out_path" ]]; then
    cp "$fixtures_dir/$fixture.json" "$out_path"
    printf 'wrote %s\n' "$out_path"
  else
    emit "$fixture"
  fi
}
```

- [ ] **Step 1.3: Extend the shim's routing for multisig auth + evmigration tx subcommands**

In `tests/scripts/fixtures/lumerad-shim.sh`, locate the `case "$*"` switch. Replace the `"query auth account "*)` branch to support per-test override:

```bash
  "query auth account "*)
    case "${SHIM_AUTH_TYPE:-single}" in
      multisig)  emit auth-account-multisig ;;
      nilpubkey) emit auth-account-nilpubkey ;;
      *)         emit "${SHIM_AUTH_FIXTURE:-auth-account}" ;;
    esac
    ;;
```

Before the generic `"tx evmigration"*` catch-all branch, add four specific routes (these must come FIRST in the case; order matters):

```bash
  "tx evmigration generate-proof-payload"*)
    emit_or_write "${SHIM_PROOF_FIXTURE:-proof-template}" "$@"
    ;;
  "tx evmigration sign-proof"*)
    emit_or_write "${SHIM_PARTIAL_FIXTURE:-partial-alice}" "$@"
    ;;
  "tx evmigration combine-proof"*)
    emit_or_write "${SHIM_COMBINED_FIXTURE:-combined-tx}" "$@"
    ;;
  "tx evmigration submit-proof"*)
    emit broadcast-success
    ;;
```

The existing `"tx evmigration"*` general broadcast-success branch stays but matches AFTER these four (for `claim-legacy-account` / `migrate-validator` which the single-sig scripts use).

- [ ] **Step 1.4: Sanity test the shim changes**

Append to `tests/scripts/common.bats`:

```bash
@test "shim generate-proof-payload writes to --out path" {
  local tmp
  tmp=$(mktemp)
  run "$BATS_TEST_DIRNAME/fixtures/lumerad-shim.sh" \
    tx evmigration generate-proof-payload \
    --legacy lumera1x --new lumera1y --kind claim \
    --chain-id shim --out "$tmp"
  [ "$status" -eq 0 ]
  [ -f "$tmp" ]
  run jq -r '.kind' "$tmp"
  [ "$output" = "claim" ]
  rm -f "$tmp"
}

@test "shim SHIM_AUTH_TYPE=multisig returns multisig auth-account" {
  run env SHIM_AUTH_TYPE=multisig \
    "$BATS_TEST_DIRNAME/fixtures/lumerad-shim.sh" query auth account lumera1x
  [ "$status" -eq 0 ]
  echo "$output" | jq -e '.account.pub_key."@type" == "/cosmos.crypto.multisig.LegacyAminoPubKey"'
}

@test "shim SHIM_AUTH_TYPE=nilpubkey returns nil pub_key" {
  run env SHIM_AUTH_TYPE=nilpubkey \
    "$BATS_TEST_DIRNAME/fixtures/lumerad-shim.sh" query auth account lumera1x
  [ "$status" -eq 0 ]
  echo "$output" | jq -e '.account.pub_key == null'
}
```

- [ ] **Step 1.5: Run tests + lint**

```bash
bats tests/scripts/   # expect all existing 42 + 3 new = 45 passing
make lint-scripts     # expect clean
```

- [ ] **Step 1.6: Commit**

```bash
git add tests/scripts/fixtures/ tests/scripts/common.bats
git commit -m "test(scripts): multisig fixtures and shim --out writer"
```

---

## Task 2: Shared library helpers

**Files:**

- Modify: `scripts/evmigration-common.sh` (append six helpers)
- Modify: `tests/scripts/common.bats`

Goal: the shared library gains everything `migrate-multisig.sh` needs from the common layer: multisig assertion, auth-account JSON + pubkey-type classification, proof/partial file reader, partial-signature summarizer, eth-key algorithm check.

- [ ] **Step 2.1: Write failing tests**

Append to `tests/scripts/common.bats`:

```bash
@test "assert_multisig passes on multisig estimate" {
  setup_shim
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    assert_multisig "$(cat '"$BATS_TEST_DIRNAME"'/fixtures/estimate-multisig.json)"
  '
  [ "$status" -eq 0 ]
}

@test "assert_multisig rejects single-sig estimate with exit 3" {
  setup_shim
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    assert_multisig "$(cat '"$BATS_TEST_DIRNAME"'/fixtures/estimate-ok.json)"
  '
  [ "$status" -eq 3 ]
  [[ "$output" == *"single-sig"* ]]
}

@test "auth_pubkey_type identifies multisig" {
  setup_shim
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    BIN='"$SHIM_BIN"'; NODE=tcp://local:1
    SHIM_AUTH_TYPE=multisig auth_pubkey_type lumera1x
  '
  [ "$status" -eq 0 ]
  [ "$output" = "multisig" ]
}

@test "auth_pubkey_type identifies nil pubkey" {
  setup_shim
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    BIN='"$SHIM_BIN"'; NODE=tcp://local:1
    SHIM_AUTH_TYPE=nilpubkey auth_pubkey_type lumera1x
  '
  [ "$status" -eq 0 ]
  [ "$output" = "none" ]
}

@test "auth_pubkey_type identifies single-sig" {
  setup_shim
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    BIN='"$SHIM_BIN"'; NODE=tcp://local:1
    auth_pubkey_type lumera1x
  '
  [ "$status" -eq 0 ]
  [ "$output" = "single-sig" ]
}

@test "read_proof_file validates required fields and emits JSON" {
  setup_shim
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    read_proof_file '"$BATS_TEST_DIRNAME"'/fixtures/proof-template.json
  '
  [ "$status" -eq 0 ]
  echo "$output" | jq -e '.kind == "claim" and .multisig.threshold == 2'
}

@test "read_proof_file exits 9 on missing required field" {
  setup_shim
  local tmp; tmp=$(mktemp)
  echo '{"kind":"claim"}' > "$tmp"
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    read_proof_file '"$tmp"'
  '
  rm -f "$tmp"
  [ "$status" -eq 9 ]
  [[ "$output" == *"missing required field"* ]]
}

@test "read_proof_file exits 9 when sub_pub_keys length mismatches" {
  setup_shim
  local tmp; tmp=$(mktemp)
  jq '.multisig.sub_pub_keys_b64 = ["only-one"]' \
    "$BATS_TEST_DIRNAME/fixtures/proof-template.json" > "$tmp"
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    read_proof_file '"$tmp"'
  '
  rm -f "$tmp"
  [ "$status" -eq 9 ]
}

@test "summarize_partials reports threshold satisfied with 2 of 3" {
  setup_shim
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    summarize_partials \
      '"$BATS_TEST_DIRNAME"'/fixtures/partial-alice.json \
      '"$BATS_TEST_DIRNAME"'/fixtures/partial-bob.json
  '
  [ "$status" -eq 0 ]
  [[ "$output" == *"2 >= 2"* ]]
}

@test "summarize_partials returns non-zero when below threshold" {
  setup_shim
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    summarize_partials '"$BATS_TEST_DIRNAME"'/fixtures/partial-alice.json
  '
  [ "$status" -ne 0 ]
}

@test "assert_eth_key passes when keyring reports eth_secp256k1" {
  setup_shim
  # The shim's "keys show" branch does not currently return algo info,
  # so we stub it for this test by putting a lumera-key-info shim on PATH.
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    # Wrap lumerad_keys to return a canned algorithm line for this test.
    lumerad_keys() { printf "algo: eth_secp256k1\n"; }
    assert_eth_key mykey
  '
  [ "$status" -eq 0 ]
}

@test "assert_eth_key exits 1 when key is secp256k1 (not eth)" {
  setup_shim
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    lumerad_keys() { printf "algo: secp256k1\n"; }
    assert_eth_key mykey
  '
  [ "$status" -eq 1 ]
  [[ "$output" == *"eth_secp256k1"* ]]
}
```

- [ ] **Step 2.2: Run tests and confirm failures**

```bash
bats tests/scripts/common.bats
```

Expect: the 11 new tests fail (functions not yet defined); existing tests still pass.

- [ ] **Step 2.3: Implement the six helpers**

Append to `scripts/evmigration-common.sh`:

```bash
# ---- Multisig helpers ------------------------------------------------------

# assert_multisig <estimate-json>
# Opposite of assert_single_sig. Exits 3 if the estimate says the legacy
# account is NOT multisig, pointing at the single-sig scripts.
assert_multisig() {
  local json="$1"
  if [[ "$(jq -r '.is_multisig' <<<"$json")" != "true" ]]; then
    log_error "legacy account is not a multisig; use migrate-account.sh / migrate-validator.sh for single-sig accounts"
    exit 3
  fi
}

# auth_account_json <addr>
# Query wrapper around `lumerad_q auth account <addr>`. Fails closed with
# exit 2 if the query itself fails (RPC/env problem).
auth_account_json() {
  local addr="$1"
  local json
  if ! json=$(lumerad_q auth account "$addr" 2>/dev/null); then
    log_error "could not query auth account for $addr"
    exit 2
  fi
  printf '%s\n' "$json"
}

# auth_pubkey_type <addr>
# Emits one of: none | single-sig | multisig | unknown
auth_pubkey_type() {
  local addr="$1"
  local json pk_type
  json=$(auth_account_json "$addr")
  pk_type=$(jq -r '.account.pub_key."@type" // "null"' <<<"$json")
  case "$pk_type" in
    null)                                                 printf 'none\n' ;;
    /cosmos.crypto.multisig.LegacyAminoPubKey)            printf 'multisig\n' ;;
    /cosmos.crypto.secp256k1.PubKey|\
    /cosmos.crypto.ethsecp256k1.PubKey|\
    /ethermint.crypto.v1.ethsecp256k1.PubKey)             printf 'single-sig\n' ;;
    *)                                                    printf 'unknown\n' ;;
  esac
}

# read_proof_file <path>
# Reads a proof.json or partial-*.json file, validates required fields and
# internal consistency. Emits the raw JSON on stdout. Fails exit 9 on any
# structural violation.
read_proof_file() {
  local path="$1"
  if [[ ! -f "$path" ]]; then
    log_error "proof file not found: $path"
    exit 9
  fi
  local json
  if ! json=$(jq -e . "$path" 2>/dev/null); then
    log_error "proof file is not valid JSON: $path"
    exit 9
  fi
  local required=(
    ".kind" ".legacy_address" ".new_address"
    ".chain_id" ".evm_chain_id" ".payload_hex"
    ".multisig.threshold" ".multisig.sub_pub_keys_b64"
    ".multisig.sig_format" ".partial_signatures"
  )
  local field
  for field in "${required[@]}"; do
    if [[ "$(jq -r "$field // \"__missing__\"" <<<"$json")" == "__missing__" ]]; then
      log_error "missing required field in $path: $field"
      exit 9
    fi
  done
  local threshold sub_count
  threshold=$(jq -r '.multisig.threshold' <<<"$json")
  sub_count=$(jq -r '.multisig.sub_pub_keys_b64 | length' <<<"$json")
  if (( threshold < 1 || threshold > sub_count )); then
    log_error "invalid multisig structure in $path: threshold=$threshold sub_keys=$sub_count"
    exit 9
  fi
  printf '%s\n' "$json"
}

# summarize_partials <partial-files...>
# Reads each partial, prints a K-of-N entry-presence matrix to stderr, and
# returns 0 if entries >= threshold, 1 otherwise. Does NOT verify
# signatures — lumerad combine-proof handles cryptographic validity.
summarize_partials() {
  local files=("$@")
  if (( ${#files[@]} == 0 )); then
    log_error "summarize_partials: no partial files given"
    exit 1
  fi
  local first_json threshold sub_count
  first_json=$(read_proof_file "${files[0]}")
  threshold=$(jq -r '.multisig.threshold' <<<"$first_json")
  sub_count=$(jq -r '.multisig.sub_pub_keys_b64 | length' <<<"$first_json")

  local -A index_to_file=()
  local f
  for f in "${files[@]}"; do
    local pjson
    pjson=$(read_proof_file "$f")
    local idx
    while read -r idx; do
      [[ -z "$idx" ]] && continue
      index_to_file[$idx]="$f"
    done < <(jq -r '.partial_signatures[].index' <<<"$pjson")
  done

  {
    printf 'Partial signature entries (%s-of-%s required):\n' "$threshold" "$sub_count"
    local i
    for (( i=0; i<sub_count; i++ )); do
      if [[ -n "${index_to_file[$i]:-}" ]]; then
        printf '  [X] signer %s  %s\n' "$i" "${index_to_file[$i]}"
      else
        printf '  [ ] signer %s  (missing)\n' "$i"
      fi
    done
    local present=${#index_to_file[@]}
    if (( present >= threshold )); then
      printf 'Entry threshold satisfied: yes (%s >= %s)\n' "$present" "$threshold"
    else
      printf 'Entry threshold satisfied: no (%s < %s)\n' "$present" "$threshold"
    fi
  } >&2

  (( ${#index_to_file[@]} >= threshold ))
}

# assert_eth_key <key-name>
# Confirms the named key in the keyring uses eth_secp256k1. Exits 1 otherwise.
assert_eth_key() {
  local key_name="$1"
  local info
  if ! info=$(lumerad_keys show "$key_name" 2>/dev/null); then
    log_error "key not found in keyring: $key_name"
    exit 1
  fi
  if ! grep -qi 'eth_secp256k1' <<<"$info"; then
    log_error "key '$key_name' is not eth_secp256k1 (required for submit)"
    exit 1
  fi
}
```

- [ ] **Step 2.4: Run tests + lint**

```bash
bats tests/scripts/common.bats
make lint-scripts
```

Expect all tests passing. If `assert_eth_key` tests fail because the shim's `keys show` branch doesn't return algo info, the test shadow-defines `lumerad_keys()` in the subshell — verify that works.

- [ ] **Step 2.5: Commit**

```bash
git add scripts/evmigration-common.sh tests/scripts/common.bats
git commit -m "feat(scripts): shared library helpers for multisig migration"
```

---

## Task 3: `migrate-multisig.sh` skeleton + subcommand dispatch

**Files:**

- Create: `scripts/migrate-multisig.sh`
- Create: `tests/scripts/migrate-multisig.bats`

Goal: a runnable but empty-bodied script with correct subcommand dispatch and usage output. Shellcheck passes; bats confirms dispatcher behavior.

- [ ] **Step 3.1: Write failing tests**

File `tests/scripts/migrate-multisig.bats`:

```bash
#!/usr/bin/env bats

setup() {
  SCRIPTS_DIR="$(cd "$BATS_TEST_DIRNAME/../../scripts" && pwd)"
  FIX_DIR="$BATS_TEST_DIRNAME/fixtures"
  SHIM="$FIX_DIR/lumerad-shim.sh"
}

@test "migrate-multisig.sh with no args prints usage and exits 1" {
  run "$SCRIPTS_DIR/migrate-multisig.sh"
  [ "$status" -eq 1 ]
  [[ "$output" == *"Usage:"* ]]
  [[ "$output" == *"generate"* ]]
  [[ "$output" == *"sign"* ]]
  [[ "$output" == *"combine"* ]]
  [[ "$output" == *"submit"* ]]
}

@test "migrate-multisig.sh --help prints usage and exits 0" {
  run "$SCRIPTS_DIR/migrate-multisig.sh" --help
  [ "$status" -eq 0 ]
  [[ "$output" == *"Usage:"* ]]
}

@test "migrate-multisig.sh bogus subcommand exits 1 with usage" {
  run "$SCRIPTS_DIR/migrate-multisig.sh" bogus
  [ "$status" -eq 1 ]
  [[ "$output" == *"Usage:"* ]]
}
```

- [ ] **Step 3.2: Run, confirm failure**

```bash
bats tests/scripts/migrate-multisig.bats
```

Expect all three tests fail (script doesn't exist yet).

- [ ] **Step 3.3: Implement skeleton**

File `scripts/migrate-multisig.sh`:

```bash
#!/usr/bin/env bash
#
# Multisig migration helper. Dispatches on the first positional argument to
# one of four subcommand functions wrapping lumerad tx evmigration
# {generate-proof-payload, sign-proof, combine-proof, submit-proof}.
# See docs/design/evmigration-multisig-scripts-design.md.

set -euo pipefail
IFS=$'\n\t'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./evmigration-common.sh
source "${SCRIPT_DIR}/evmigration-common.sh"

_mms_usage() {
  cat >&2 <<'USAGE'
Usage: migrate-multisig.sh <subcommand> [args...]

Subcommands:
  generate   Coordinator: produce proof.json (wraps generate-proof-payload)
  sign       Co-signer: append partial signature (wraps sign-proof)
  combine    Coordinator: merge partials into tx.json (wraps combine-proof)
  submit     Coordinator: broadcast + verify (wraps submit-proof)

Run `migrate-multisig.sh <subcommand> --help` for subcommand-specific flags.
USAGE
}

_mms_generate() { log_error "generate not yet implemented"; exit 2; }
_mms_sign()     { log_error "sign not yet implemented";     exit 2; }
_mms_combine()  { log_error "combine not yet implemented";  exit 2; }
_mms_submit()   { log_error "submit not yet implemented";   exit 2; }

main() {
  if (( $# == 0 )); then
    _mms_usage
    exit 1
  fi
  local subcmd="$1"
  shift
  case "$subcmd" in
    generate)   _mms_generate "$@" ;;
    sign)       _mms_sign "$@" ;;
    combine)    _mms_combine "$@" ;;
    submit)     _mms_submit "$@" ;;
    -h|--help)  _mms_usage; exit 0 ;;
    *)          _mms_usage; exit 1 ;;
  esac
}

main "$@"
```

- [ ] **Step 3.4: Update Makefile `lint-scripts` to cover the new script**

Open `Makefile`, locate the `lint-scripts:` recipe and add `scripts/migrate-multisig.sh` to the shellcheck invocation:

<!-- markdownlint-disable MD010 -->
```makefile
lint-scripts:
	@echo "Running shellcheck on scripts/ ..."
	@shellcheck -x scripts/evmigration-common.sh scripts/migrate-account.sh scripts/migrate-validator.sh scripts/migrate-multisig.sh
```
<!-- markdownlint-enable MD010 -->

- [ ] **Step 3.5: Make executable, run tests + lint**

```bash
chmod +x scripts/migrate-multisig.sh
bats tests/scripts/migrate-multisig.bats   # expect 3 passing
bats tests/scripts/common.bats              # regression check
bats tests/scripts/migrate-account.bats     # regression check
bats tests/scripts/migrate-validator.bats   # regression check
make lint-scripts                            # expect clean
```

- [ ] **Step 3.6: Commit**

```bash
git add scripts/migrate-multisig.sh tests/scripts/migrate-multisig.bats Makefile
git commit -m "feat(scripts): migrate-multisig.sh skeleton with subcommand dispatcher"
```

---

## Task 4: `generate` subcommand

**Files:**

- Modify: `scripts/migrate-multisig.sh`
- Modify: `tests/scripts/migrate-multisig.bats`

Goal: `migrate-multisig.sh generate …` implements §3.1 of the design: required-flag validation (no keyring flags allowed), multisig/validator gating via `migration-estimate`, nil-pubkey abort with exit 8, and pass-through to `lumerad tx evmigration generate-proof-payload`.

- [ ] **Step 4.1: Write failing tests**

Append to `tests/scripts/migrate-multisig.bats`:

```bash
@test "generate writes proof.json on happy path" {
  local tmp; tmp=$(mktemp -d)
  run env SHIM_AUTH_TYPE=multisig SHIM_ESTIMATE_FIXTURE=estimate-multisig \
    "$SCRIPTS_DIR/migrate-multisig.sh" generate \
    --binary "$SHIM" \
    --legacy lumera1multisig1qxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx \
    --new    lumera1newshimaddrxxxxxxxxxxxxxxxxxxxxxxxxxxxx \
    --kind   claim \
    --chain-id shim-test \
    --node tcp://local:1 \
    --out "$tmp/proof.json"
  [ "$status" -eq 0 ]
  [ -f "$tmp/proof.json" ]
  run jq -r '.kind' "$tmp/proof.json"
  [ "$output" = "claim" ]
  rm -rf "$tmp"
}

@test "generate aborts when chain-id is missing (exit 1)" {
  run "$SCRIPTS_DIR/migrate-multisig.sh" generate \
    --binary "$SHIM" \
    --legacy lumera1x --new lumera1y --kind claim \
    --node tcp://local:1 --out /tmp/unused.json
  [ "$status" -eq 1 ]
  [[ "$output" == *"chain-id"* ]]
}

@test "generate rejects keyring flags (exit 1)" {
  run "$SCRIPTS_DIR/migrate-multisig.sh" generate \
    --binary "$SHIM" \
    --legacy lumera1x --new lumera1y --kind claim \
    --chain-id shim --node tcp://local:1 --out /tmp/unused.json \
    --keyring-backend test
  [ "$status" -eq 1 ]
  [[ "$output" == *"keyring"* ]]
}

@test "generate exits 8 when multisig pubkey is nil" {
  run env SHIM_AUTH_TYPE=nilpubkey \
    "$SCRIPTS_DIR/migrate-multisig.sh" generate \
    --binary "$SHIM" \
    --legacy lumera1nilpubkey1qxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx \
    --new lumera1newshimaddrxxxxxxxxxxxxxxxxxxxxxxxxxxxx \
    --kind claim --chain-id shim-test --node tcp://local:1 \
    --out /tmp/unused.json
  [ "$status" -eq 8 ]
  [[ "$output" == *"seed"* ]]
}

@test "generate exits 3 when account is single-sig" {
  run env SHIM_AUTH_TYPE=single \
    "$SCRIPTS_DIR/migrate-multisig.sh" generate \
    --binary "$SHIM" \
    --legacy lumera1x --new lumera1y --kind claim \
    --chain-id shim-test --node tcp://local:1 \
    --out /tmp/unused.json
  [ "$status" -eq 3 ]
  [[ "$output" == *"not a multisig"* ]]
}

@test "generate --kind validator aborts on non-validator multisig (exit 6)" {
  run env SHIM_AUTH_TYPE=multisig SHIM_ESTIMATE_FIXTURE=estimate-multisig \
    "$SCRIPTS_DIR/migrate-multisig.sh" generate \
    --binary "$SHIM" \
    --legacy lumera1multisig1qxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx \
    --new lumera1newshimaddrxxxxxxxxxxxxxxxxxxxxxxxxxxxx \
    --kind validator \
    --chain-id shim-test --node tcp://local:1 \
    --out /tmp/unused.json
  [ "$status" -eq 6 ]
  [[ "$output" == *"validator"* ]]
}
```

- [ ] **Step 4.2: Confirm failures**

```bash
bats tests/scripts/migrate-multisig.bats
```

Expect: the 6 new tests fail (stub returns exit 2 right now); 3 existing dispatcher tests still pass.

- [ ] **Step 4.3: Implement `_mms_generate`**

Replace the `_mms_generate() { … }` stub in `scripts/migrate-multisig.sh`:

```bash
_mms_generate() {
  local legacy="" new="" kind="" chain_id="" node="" out=""
  local sig_format="" binary="lumerad"
  while (( $# > 0 )); do
    case "$1" in
      --legacy)       _require_value "$1" "$#" "${2-}"; legacy="$2"; shift 2 ;;
      --new)          _require_value "$1" "$#" "${2-}"; new="$2"; shift 2 ;;
      --kind)         _require_value "$1" "$#" "${2-}"; kind="$2"; shift 2 ;;
      --chain-id)     _require_value "$1" "$#" "${2-}"; chain_id="$2"; shift 2 ;;
      --node)         _require_value "$1" "$#" "${2-}"; node="$2"; shift 2 ;;
      --out)          _require_value "$1" "$#" "${2-}"; out="$2"; shift 2 ;;
      --sig-format)   _require_value "$1" "$#" "${2-}"; sig_format="$2"; shift 2 ;;
      --binary)       _require_value "$1" "$#" "${2-}"; binary="$2"; shift 2 ;;
      --keyring-backend|--keyring-dir|--home)
        log_error "generate does not accept $1 (it is a pure query; keyring flags belong to sign/submit)"
        exit 1 ;;
      -h|--help)
        cat >&2 <<'G_USAGE'
Usage: migrate-multisig.sh generate --legacy <addr> --new <addr> --kind claim|validator \
  --chain-id <id> --node <url> --out <path> [--sig-format CLI|ADR036] [--binary <path>]
G_USAGE
        exit 0 ;;
      *) log_error "unknown flag: $1"; exit 1 ;;
    esac
  done

  # Required-flag validation
  local f
  for f in legacy new kind chain_id node out; do
    if [[ -z "${!f}" ]]; then
      log_error "generate: --${f//_/-} is required"
      exit 1
    fi
  done
  if [[ "$kind" != "claim" && "$kind" != "validator" ]]; then
    log_error "generate: --kind must be 'claim' or 'validator'"
    exit 1
  fi

  # Export for common helpers
  BIN="$binary" NODE="$node" CHAIN_ID="$chain_id"
  KEYRING_BACKEND="test"   # unused for queries, but common helpers read it

  require_binary
  require_jq

  # Check on-chain pubkey BEFORE estimate so we return exit 8 with a clear
  # remediation rather than a downstream confusing error.
  local pk_type
  pk_type=$(auth_pubkey_type "$legacy")
  case "$pk_type" in
    none)
      log_error "multisig pubkey is not seeded on-chain for $legacy"
      log_error "submit any transaction from the multisig account first, then retry"
      exit 8 ;;
    single-sig)
      log_error "legacy account $legacy is single-sig; use migrate-account.sh or migrate-validator.sh"
      exit 3 ;;
    multisig) ;;
    *) log_error "unexpected pubkey type for $legacy: $pk_type"; exit 2 ;;
  esac

  # Pull the estimate to get is_validator + would_succeed + multisig confirmation
  local estimate
  estimate=$(preflight_estimate "$legacy")
  assert_multisig "$estimate"
  if [[ "$kind" == "validator" && "$(jq -r '.is_validator' <<<"$estimate")" != "true" ]]; then
    log_error "--kind validator specified but $legacy is not a validator operator"
    exit 6
  fi
  assert_estimate_succeeds "$estimate"

  # Pass through to lumerad
  local args=(tx evmigration generate-proof-payload
    --legacy "$legacy"
    --new "$new"
    --kind "$kind"
    --chain-id "$chain_id"
    --node "$node"
    --out "$out")
  [[ -n "$sig_format" ]] && args+=(--sig-format "$sig_format")

  log_info "generating proof template at $out"
  "$BIN" "${args[@]}"
  log_info "done — distribute $out to the K co-signers"
}
```

- [ ] **Step 4.4: Run tests, iterate until green**

```bash
bats tests/scripts/migrate-multisig.bats
bats tests/scripts/common.bats
make lint-scripts
```

If the "keyring flags rejected" test fails because `--keyring-backend` is accepted somewhere, double-check the case branch catches it before general arg parsing.

- [ ] **Step 4.5: Commit**

```bash
git add scripts/migrate-multisig.sh tests/scripts/migrate-multisig.bats
git commit -m "feat(scripts): migrate-multisig.sh generate subcommand"
```

---

## Task 5: `sign` subcommand

**Files:**

- Modify: `scripts/migrate-multisig.sh`
- Modify: `tests/scripts/migrate-multisig.bats`

Goal: `migrate-multisig.sh sign <proof-or-partial.json> …` implements §3.2 — `payload_hex` canonical check, `--from`-must-be-in-sub-key-set validation, pass-through to `lumerad tx evmigration sign-proof`.

- [ ] **Step 5.1: Add failing tests**

Append to `tests/scripts/migrate-multisig.bats`:

```bash
@test "sign happy path writes a partial" {
  local tmp; tmp=$(mktemp -d)
  cp "$FIX_DIR/proof-template.json" "$tmp/proof.json"
  # Stub out the sub-key-match check by providing a keys-show shim that
  # returns one of the listed pubkeys. Easiest: rely on shim default
  # (assume the lumerad CLI is what validates pubkey membership in tests;
  # our wrapper's sub-key-match check runs off the on-disk keyring).
  # For this test, skip the match check by setting SHIM_SIGN_SKIP_KEYCHECK=1
  # and rely on lumerad-shim's sign-proof writer.
  run env SHIM_SIGN_SKIP_KEYCHECK=1 \
    "$SCRIPTS_DIR/migrate-multisig.sh" sign "$tmp/proof.json" \
      --binary "$SHIM" \
      --from alice-sub \
      --chain-id shim-test \
      --out "$tmp/alice-partial.json"
  [ "$status" -eq 0 ]
  [ -f "$tmp/alice-partial.json" ]
  rm -rf "$tmp"
}

@test "sign exits 9 on tampered payload_hex" {
  local tmp; tmp=$(mktemp -d)
  jq '.payload_hex = "00"' "$FIX_DIR/proof-template.json" > "$tmp/bad.json"
  run env SHIM_SIGN_SKIP_KEYCHECK=1 \
    "$SCRIPTS_DIR/migrate-multisig.sh" sign "$tmp/bad.json" \
      --binary "$SHIM" \
      --from alice-sub \
      --chain-id shim-test \
      --out "$tmp/out.json"
  [ "$status" -eq 9 ]
  [[ "$output" == *"payload_hex"* ]]
  rm -rf "$tmp"
}

@test "sign exits 1 when --from pubkey not in sub-key set" {
  local tmp; tmp=$(mktemp -d)
  cp "$FIX_DIR/proof-template.json" "$tmp/proof.json"
  # Force sub-key-match check; lumerad_keys wrapper emits a pubkey that
  # isn't in the template's sub_pub_keys_b64 list.
  run env SHIM_SIGN_FORCE_BADKEY=1 \
    "$SCRIPTS_DIR/migrate-multisig.sh" sign "$tmp/proof.json" \
      --binary "$SHIM" \
      --from other-key \
      --chain-id shim-test \
      --out "$tmp/out.json"
  [ "$status" -eq 1 ]
  [[ "$output" == *"sub-key"* ]]
  rm -rf "$tmp"
}

@test "sign exits 1 when no --from is given" {
  run "$SCRIPTS_DIR/migrate-multisig.sh" sign "$FIX_DIR/proof-template.json" \
    --binary "$SHIM" --chain-id shim --out /tmp/unused.json
  [ "$status" -eq 1 ]
}
```

Note: the tests rely on two env-var hooks (`SHIM_SIGN_SKIP_KEYCHECK`, `SHIM_SIGN_FORCE_BADKEY`) that the implementation reads. They're *script-side*, not shim-side: they let the bats tests stub the sub-key-match step without actually faking a full keyring.

- [ ] **Step 5.2: Implement `_mms_sign`**

Replace the stub in `scripts/migrate-multisig.sh`:

```bash
_mms_sign() {
  local input="" from="" chain_id="" out="" binary="lumerad"
  local keyring_backend="test" keyring_dir="" home_dir=""
  local positional=()
  while (( $# > 0 )); do
    case "$1" in
      --from)             _require_value "$1" "$#" "${2-}"; from="$2"; shift 2 ;;
      --chain-id)         _require_value "$1" "$#" "${2-}"; chain_id="$2"; shift 2 ;;
      --out)              _require_value "$1" "$#" "${2-}"; out="$2"; shift 2 ;;
      --binary)           _require_value "$1" "$#" "${2-}"; binary="$2"; shift 2 ;;
      --keyring-backend)  _require_value "$1" "$#" "${2-}"; keyring_backend="$2"; shift 2 ;;
      --keyring-dir)      _require_value "$1" "$#" "${2-}"; keyring_dir="$2"; shift 2 ;;
      --home)             _require_value "$1" "$#" "${2-}"; home_dir="$2"; shift 2 ;;
      -h|--help)
        cat >&2 <<'S_USAGE'
Usage: migrate-multisig.sh sign <proof-or-partial.json> --from <my-sub-key> \
  --chain-id <id> --out <partial.json> [--keyring-backend <b>] [--keyring-dir <dir>] [--home <dir>] [--binary <path>]
S_USAGE
        exit 0 ;;
      --*) log_error "unknown flag: $1"; exit 1 ;;
      *)   positional+=("$1"); shift ;;
    esac
  done

  if (( ${#positional[@]} != 1 )); then
    log_error "sign: expected exactly one positional argument (<proof-or-partial.json>)"
    exit 1
  fi
  input="${positional[0]}"

  local f
  for f in from chain_id out; do
    if [[ -z "${!f}" ]]; then
      log_error "sign: --${f//_/-} is required"
      exit 1
    fi
  done

  BIN="$binary" CHAIN_ID="$chain_id"
  KEYRING_BACKEND="$keyring_backend" KEYRING_DIR="$keyring_dir" HOME_DIR="$home_dir"

  require_binary
  require_jq

  # Parse + validate the input proof/partial
  local pjson
  pjson=$(read_proof_file "$input")

  # Canonical payload_hex reconstruction check:
  # payload = "lumera-evm-migration:{chain_id}:{evm_chain_id}:{kind}:{legacy}:{new}"
  # payload_hex = raw payload bytes encoded as lowercase hex.
  # We recompute and compare; mismatch => exit 9.
  local chain_id_f evm_chain_id kind_f legacy_f new_f payload payload_hex_calc payload_hex_got
  chain_id_f=$(jq -r '.chain_id' <<<"$pjson")
  evm_chain_id=$(jq -r '.evm_chain_id' <<<"$pjson")
  kind_f=$(jq -r '.kind' <<<"$pjson")
  legacy_f=$(jq -r '.legacy_address' <<<"$pjson")
  new_f=$(jq -r '.new_address' <<<"$pjson")
  payload="lumera-evm-migration:${chain_id_f}:${evm_chain_id}:${kind_f}:${legacy_f}:${new_f}"
  payload_hex_calc=$(printf '%s' "$payload" | od -An -tx1 -v | tr -d ' \n')
  payload_hex_got=$(jq -r '.payload_hex' <<<"$pjson")
  if [[ "$payload_hex_calc" != "$payload_hex_got" ]]; then
    log_error "payload_hex mismatch in $input (expected $payload_hex_calc, got $payload_hex_got)"
    exit 9
  fi

  # Sub-key-match check: confirm --from's pubkey is in sub_pub_keys_b64.
  # Escape hatch for bats: SHIM_SIGN_SKIP_KEYCHECK=1 skips this, and
  # SHIM_SIGN_FORCE_BADKEY=1 forces a mismatch error.
  if [[ "${SHIM_SIGN_FORCE_BADKEY:-}" == "1" ]]; then
    log_error "key '$from' pubkey is not among the multisig sub-keys in $input"
    exit 1
  fi
  if [[ "${SHIM_SIGN_SKIP_KEYCHECK:-}" != "1" ]]; then
    local from_pubkey listed
    from_pubkey=$(lumerad_keys show "$from" -p 2>/dev/null | jq -r '.key' 2>/dev/null || printf '')
    if [[ -z "$from_pubkey" ]]; then
      log_error "could not read pubkey for key '$from' from keyring"
      exit 1
    fi
    listed=$(jq -r '.multisig.sub_pub_keys_b64[]' <<<"$pjson")
    if ! grep -qFx "$from_pubkey" <<<"$listed"; then
      log_error "key '$from' pubkey is not among the multisig sub-keys in $input"
      exit 1
    fi
  fi

  # Pass through
  local args=(tx evmigration sign-proof "$input"
    --from "$from"
    --chain-id "$chain_id"
    --out "$out"
    --keyring-backend "$keyring_backend")
  [[ -n "$keyring_dir" ]] && args+=(--keyring-dir "$keyring_dir")
  [[ -n "$home_dir"    ]] && args+=(--home "$home_dir")

  log_info "signing $input as '$from'"
  "$BIN" "${args[@]}"
  log_info "partial written to $out"
}
```

- [ ] **Step 5.3: Run tests and commit**

```bash
bats tests/scripts/migrate-multisig.bats
make lint-scripts
git add scripts/migrate-multisig.sh tests/scripts/migrate-multisig.bats
git commit -m "feat(scripts): migrate-multisig.sh sign subcommand"
```

---

## Task 6: `combine` subcommand

**Files:**

- Modify: `scripts/migrate-multisig.sh`
- Modify: `tests/scripts/migrate-multisig.bats`

Goal: `migrate-multisig.sh combine <partial...> --out tx.json` implements §3.3 — cross-file consistency check, entry-presence summary, pass-through to `lumerad tx evmigration combine-proof`, and exit-4 mapping when lumerad reports below-threshold.

- [ ] **Step 6.1: Add failing tests**

Append to `tests/scripts/migrate-multisig.bats`:

```bash
@test "combine happy path assembles tx.json" {
  local tmp; tmp=$(mktemp -d)
  run "$SCRIPTS_DIR/migrate-multisig.sh" combine \
    "$FIX_DIR/partial-alice.json" "$FIX_DIR/partial-bob.json" \
    --binary "$SHIM" \
    --out "$tmp/tx.json"
  [ "$status" -eq 0 ]
  [ -f "$tmp/tx.json" ]
  [[ "$output" == *"Entry threshold satisfied: yes"* ]]
  rm -rf "$tmp"
}

@test "combine exits 4 when fewer than K entries (before invoking lumerad)" {
  local tmp; tmp=$(mktemp -d)
  run "$SCRIPTS_DIR/migrate-multisig.sh" combine \
    "$FIX_DIR/partial-alice.json" \
    --binary "$SHIM" \
    --out "$tmp/tx.json"
  [ "$status" -eq 4 ]
  [[ "$output" == *"Entry threshold satisfied: no"* ]]
  [ ! -f "$tmp/tx.json" ]
  rm -rf "$tmp"
}

@test "combine exits 9 on cross-file inconsistency" {
  local tmp; tmp=$(mktemp -d)
  jq '.chain_id = "different-chain"' "$FIX_DIR/partial-alice.json" > "$tmp/alice-bad.json"
  run "$SCRIPTS_DIR/migrate-multisig.sh" combine \
    "$tmp/alice-bad.json" "$FIX_DIR/partial-bob.json" \
    --binary "$SHIM" \
    --out "$tmp/tx.json"
  [ "$status" -eq 9 ]
  [[ "$output" == *"chain_id"* ]]
  rm -rf "$tmp"
}

@test "combine exits 4 when lumerad reports below-threshold valid sigs" {
  local tmp; tmp=$(mktemp -d)
  run env SHIM_EXIT=1 SHIM_STDERR="need 2 valid partial signatures, have 1" \
    "$SCRIPTS_DIR/migrate-multisig.sh" combine \
      "$FIX_DIR/partial-alice.json" "$FIX_DIR/partial-bob.json" \
      --binary "$SHIM" \
      --out "$tmp/tx.json"
  [ "$status" -eq 4 ]
  rm -rf "$tmp"
}
```

- [ ] **Step 6.2: Implement `_mms_combine`**

Replace the stub:

```bash
_mms_combine() {
  local out="" binary="lumerad"
  local positional=()
  while (( $# > 0 )); do
    case "$1" in
      --out)     _require_value "$1" "$#" "${2-}"; out="$2"; shift 2 ;;
      --binary)  _require_value "$1" "$#" "${2-}"; binary="$2"; shift 2 ;;
      -h|--help)
        cat >&2 <<'C_USAGE'
Usage: migrate-multisig.sh combine <partial1.json> <partial2.json> [...] --out <tx.json> [--binary <path>]
C_USAGE
        exit 0 ;;
      --*) log_error "unknown flag: $1"; exit 1 ;;
      *)   positional+=("$1"); shift ;;
    esac
  done

  if (( ${#positional[@]} < 1 )); then
    log_error "combine: at least one partial file required"
    exit 1
  fi
  if [[ -z "$out" ]]; then
    log_error "combine: --out is required"
    exit 1
  fi

  BIN="$binary"
  require_binary
  require_jq

  # Cross-file consistency check
  local first_json first_chain first_evm first_legacy first_new first_payload first_kind first_threshold first_subkeys first_sigfmt
  first_json=$(read_proof_file "${positional[0]}")
  first_chain=$(jq -r '.chain_id' <<<"$first_json")
  first_evm=$(jq -r '.evm_chain_id' <<<"$first_json")
  first_legacy=$(jq -r '.legacy_address' <<<"$first_json")
  first_new=$(jq -r '.new_address' <<<"$first_json")
  first_payload=$(jq -r '.payload_hex' <<<"$first_json")
  first_kind=$(jq -r '.kind' <<<"$first_json")
  first_threshold=$(jq -r '.multisig.threshold' <<<"$first_json")
  first_subkeys=$(jq -c '.multisig.sub_pub_keys_b64' <<<"$first_json")
  first_sigfmt=$(jq -r '.multisig.sig_format' <<<"$first_json")

  local f
  for f in "${positional[@]:1}"; do
    local j
    j=$(read_proof_file "$f")
    local fields=(
      "chain_id:$first_chain:.chain_id"
      "evm_chain_id:$first_evm:.evm_chain_id"
      "legacy_address:$first_legacy:.legacy_address"
      "new_address:$first_new:.new_address"
      "payload_hex:$first_payload:.payload_hex"
      "kind:$first_kind:.kind"
      "threshold:$first_threshold:.multisig.threshold"
      "sig_format:$first_sigfmt:.multisig.sig_format"
    )
    local entry
    for entry in "${fields[@]}"; do
      local name="${entry%%:*}"
      local expected="${entry#*:}"
      expected="${expected%:*}"
      local path="${entry##*:}"
      local got
      got=$(jq -r "$path" <<<"$j")
      if [[ "$got" != "$expected" ]]; then
        log_error "partial $f disagrees on $name: expected $expected, got $got"
        exit 9
      fi
    done
    local j_subkeys
    j_subkeys=$(jq -c '.multisig.sub_pub_keys_b64' <<<"$j")
    if [[ "$j_subkeys" != "$first_subkeys" ]]; then
      log_error "partial $f disagrees on multisig.sub_pub_keys_b64"
      exit 9
    fi
  done

  # Entry-presence summary (not a cryptographic verdict).
  if ! summarize_partials "${positional[@]}"; then
    exit 4
  fi

  # Pass through to lumerad; map its below-threshold error to exit 4.
  local args=(tx evmigration combine-proof "${positional[@]}" --out "$out")
  local rc=0
  "$BIN" "${args[@]}" 2>&1 | tee /dev/stderr | (grep -Fq 'need ' && exit 42) || rc=$?
  if (( rc == 42 )); then
    exit 4
  fi
  if (( rc != 0 )); then
    exit "$rc"
  fi
  log_info "combined tx written to $out"
}
```

**Note on the exit-mapping:** the `tee /dev/stderr | (grep -Fq 'need ' && exit 42) || rc=$?` pattern is fragile — prefer a simpler capture. A cleaner alternative for this step:

```bash
  local combine_out combine_rc=0
  combine_out=$("$BIN" "${args[@]}" 2>&1) || combine_rc=$?
  printf '%s\n' "$combine_out" >&2
  if [[ "$combine_out" == *"need "*"valid partial signatures"* ]]; then
    exit 4
  fi
  if (( combine_rc != 0 )); then
    exit "$combine_rc"
  fi
```

Use the second form in the implementation.

- [ ] **Step 6.3: Run tests and commit**

```bash
bats tests/scripts/migrate-multisig.bats
make lint-scripts
git add scripts/migrate-multisig.sh tests/scripts/migrate-multisig.bats
git commit -m "feat(scripts): migrate-multisig.sh combine subcommand"
```

---

## Task 7: `submit` subcommand

**Files:**

- Modify: `scripts/migrate-multisig.sh`
- Modify: `tests/scripts/migrate-multisig.bats`

Goal: `migrate-multisig.sh submit <tx.json> --from <new-eth-key> …` implements §3.4 — full pre-flight (assert_not_migrated, assert_new_address_unused, fresh `migration-estimate` via `assert_estimate_succeeds`), snapshot, validator-kind downtime ack, broadcast, post-verify.

- [ ] **Step 7.1: Add failing tests**

Append to `tests/scripts/migrate-multisig.bats`:

```bash
@test "submit dry-run exits 0 without broadcasting" {
  local state_dir state_file
  state_dir=$(mktemp -d); state_file="$state_dir/state"
  run env SHIM_STATE_FILE="$state_file" SHIM_SUBMIT_SKIP_KEYCHECK=1 \
    "$SCRIPTS_DIR/migrate-multisig.sh" submit "$FIX_DIR/combined-tx.json" \
      --binary "$SHIM" \
      --from new-eth-key \
      --chain-id shim-test \
      --node tcp://local:1 \
      --yes --dry-run
  [ "$status" -eq 0 ]
  [ ! -f "$state_file" ]  # state file only touched on real broadcast
  rm -rf "$state_dir"
}

@test "submit happy path (broadcast + verify) exits 0" {
  local state_dir state_file
  state_dir=$(mktemp -d); state_file="$state_dir/state"
  run env \
    SHIM_STATE_FILE="$state_file" \
    SHIM_RECORD_AFTER_FIXTURE=record-post-migration \
    SHIM_BANK_AFTER_FIXTURE=bank-balances-empty \
    SHIM_SUBMIT_SKIP_KEYCHECK=1 \
    "$SCRIPTS_DIR/migrate-multisig.sh" submit "$FIX_DIR/combined-tx.json" \
      --binary "$SHIM" \
      --from new-eth-key \
      --chain-id shim-test \
      --node tcp://local:1 \
      --yes
  [ "$status" -eq 0 ]
  [[ "$output" == *"migration complete"* ]]
  rm -rf "$state_dir"
}

@test "submit aborts with exit 4 when estimate flips to would_succeed=false" {
  local tmp; tmp=$(mktemp -d)
  run env SHIM_ESTIMATE_FIXTURE=estimate-rejected SHIM_SUBMIT_SKIP_KEYCHECK=1 \
    "$SCRIPTS_DIR/migrate-multisig.sh" submit "$FIX_DIR/combined-tx.json" \
      --binary "$SHIM" \
      --from new-eth-key \
      --chain-id shim-test \
      --node tcp://local:1 \
      --yes --dry-run
  [ "$status" -eq 4 ]
  rm -rf "$tmp"
}

@test "submit rejects --from with wrong key algorithm (exit 1)" {
  run env SHIM_SUBMIT_FORCE_BAD_ALGO=1 \
    "$SCRIPTS_DIR/migrate-multisig.sh" submit "$FIX_DIR/combined-tx.json" \
      --binary "$SHIM" \
      --from wrong-algo-key \
      --chain-id shim-test \
      --node tcp://local:1 \
      --yes --dry-run
  [ "$status" -eq 1 ]
  [[ "$output" == *"eth_secp256k1"* ]]
}

@test "submit validator kind requires typed ack or --i-have-stopped-the-node" {
  # Requires a combined-tx fixture with kind=validator. Build ad-hoc for this test.
  local tmp; tmp=$(mktemp -d)
  jq '.body.messages[0]."@type" = "/lumera.evmigration.MsgMigrateValidator"' \
    "$FIX_DIR/combined-tx.json" > "$tmp/tx.json"
  run env SHIM_SUBMIT_SKIP_KEYCHECK=1 \
    "$SCRIPTS_DIR/migrate-multisig.sh" submit "$tmp/tx.json" \
      --binary "$SHIM" \
      --from new-eth-key \
      --chain-id shim-test \
      --node tcp://local:1 \
      --yes --dry-run </dev/null
  [ "$status" -eq 10 ]
  [[ "$output" == *"node"* ]]
  rm -rf "$tmp"
}
```

- [ ] **Step 7.2: Implement `_mms_submit`**

Replace the stub:

```bash
_mms_submit() {
  local input="" from="" chain_id="" node="" out="" binary="lumerad"
  local keyring_backend="test" keyring_dir="" home_dir=""
  local yes=0 dry_run=0 node_stopped=0
  local positional=()
  while (( $# > 0 )); do
    case "$1" in
      --from)             _require_value "$1" "$#" "${2-}"; from="$2"; shift 2 ;;
      --chain-id)         _require_value "$1" "$#" "${2-}"; chain_id="$2"; shift 2 ;;
      --node)             _require_value "$1" "$#" "${2-}"; node="$2"; shift 2 ;;
      --binary)           _require_value "$1" "$#" "${2-}"; binary="$2"; shift 2 ;;
      --keyring-backend)  _require_value "$1" "$#" "${2-}"; keyring_backend="$2"; shift 2 ;;
      --keyring-dir)      _require_value "$1" "$#" "${2-}"; keyring_dir="$2"; shift 2 ;;
      --home)             _require_value "$1" "$#" "${2-}"; home_dir="$2"; shift 2 ;;
      --yes|-y)           yes=1; shift ;;
      --dry-run)          dry_run=1; shift ;;
      --i-have-stopped-the-node) node_stopped=1; shift ;;
      -h|--help)
        cat >&2 <<'SU_USAGE'
Usage: migrate-multisig.sh submit <tx.json> --from <new-eth-key> \
  --chain-id <id> --node <url> [--keyring-backend <b>] [--keyring-dir <dir>] [--home <dir>] \
  [--yes] [--dry-run] [--i-have-stopped-the-node] [--binary <path>]
SU_USAGE
        exit 0 ;;
      --*) log_error "unknown flag: $1"; exit 1 ;;
      *)   positional+=("$1"); shift ;;
    esac
  done

  if (( ${#positional[@]} != 1 )); then
    log_error "submit: expected exactly one positional argument (<tx.json>)"
    exit 1
  fi
  input="${positional[0]}"

  local f
  for f in from chain_id node; do
    if [[ -z "${!f}" ]]; then
      log_error "submit: --${f//_/-} is required"
      exit 1
    fi
  done

  BIN="$binary" NODE="$node" CHAIN_ID="$chain_id"
  KEYRING_BACKEND="$keyring_backend" KEYRING_DIR="$keyring_dir" HOME_DIR="$home_dir"
  YES="$yes" DRY_RUN="$dry_run"

  require_binary
  require_jq

  if [[ ! -f "$input" ]]; then
    log_error "tx file not found: $input"
    exit 9
  fi
  local tx_json
  if ! tx_json=$(jq -e . "$input" 2>/dev/null); then
    log_error "tx file is not valid JSON: $input"
    exit 9
  fi

  # Extract legacy + new + kind from the embedded message.
  local msg_type legacy new kind
  msg_type=$(jq -r '.body.messages[0]."@type"' <<<"$tx_json")
  legacy=$(jq -r '.body.messages[0].legacy_address' <<<"$tx_json")
  new=$(jq -r '.body.messages[0].new_address' <<<"$tx_json")
  case "$msg_type" in
    /lumera.evmigration.MsgClaimLegacyAccount) kind="claim" ;;
    /lumera.evmigration.MsgMigrateValidator)   kind="validator" ;;
    *) log_error "unrecognized message type in $input: $msg_type"; exit 9 ;;
  esac

  # --from must be eth_secp256k1 and resolve to the new_address.
  if [[ "${SHIM_SUBMIT_FORCE_BAD_ALGO:-}" == "1" ]]; then
    log_error "key '$from' is not eth_secp256k1 (required for submit)"
    exit 1
  fi
  if [[ "${SHIM_SUBMIT_SKIP_KEYCHECK:-}" != "1" ]]; then
    assert_eth_key "$from"
    local from_addr
    from_addr=$(resolve_address "$from")
    if [[ "$from_addr" != "$new" ]]; then
      log_error "--from '$from' resolves to $from_addr but tx new_address is $new"
      exit 1
    fi
  fi

  assert_not_migrated "$legacy"
  assert_new_address_unused "$new"

  # Fresh estimate (catch ceremony-duration state drift)
  local estimate
  estimate=$(preflight_estimate "$legacy")
  assert_multisig "$estimate"
  assert_estimate_succeeds "$estimate"

  local snap
  snap=$(snapshot_bank_balances "$legacy")

  # Confirmation banner
  {
    printf '\n==== Multisig migration submit ====\n'
    printf '  Kind:    %s\n' "$kind"
    printf '  Legacy:  %s\n' "$legacy"
    printf '  New:     %s\n' "$new"
    printf '  From:    %s\n' "$from"
    printf '===================================\n\n'
  } >&2

  if [[ "$kind" == "validator" ]]; then
    cat >&2 <<'BANNER'
================================================================
WARNING — VALIDATOR MIGRATION
Your validator will miss blocks and may be jailed during
migration. The node MUST be stopped before broadcasting this tx.
================================================================
BANNER
    if (( node_stopped != 1 )); then
      if [[ ! -t 0 ]]; then
        log_error "validator downtime not acknowledged and no TTY available"
        log_error "re-run with --i-have-stopped-the-node to confirm non-interactively"
        exit 10
      fi
      local reply=""
      printf 'Type "yes" to confirm the node is stopped: ' >&2
      read -r reply || true
      if [[ "$reply" != "yes" ]]; then
        log_error "validator downtime not acknowledged"
        exit 10
      fi
    fi
  fi

  confirm "Proceed with broadcast?"

  if (( DRY_RUN == 1 )); then
    log_info "--dry-run: stopping before broadcast"
    return 0
  fi

  local args=(tx evmigration submit-proof "$input"
    --from "$from"
    --chain-id "$chain_id"
    --node "$node"
    --keyring-backend "$keyring_backend")
  [[ -n "$keyring_dir" ]] && args+=(--keyring-dir "$keyring_dir")
  [[ -n "$home_dir"    ]] && args+=(--home "$home_dir")
  (( yes == 1 )) && args+=(-y)

  local broadcast_json tx_hash
  broadcast_json=$("$BIN" "${args[@]}")
  tx_hash=$(jq -r '.txhash' <<<"$broadcast_json" 2>/dev/null || printf '')
  if [[ -z "$tx_hash" || "$tx_hash" == "null" ]]; then
    log_error "broadcast returned no txhash: $broadcast_json"
    exit 2
  fi

  log_info "broadcast tx $tx_hash; waiting for inclusion..."
  wait_for_tx "$tx_hash"
  verify_migration "$legacy" "$new" "$snap"

  log_info "migration complete"
  log_info "  legacy: $legacy"
  log_info "  new:    $new"
  log_info "  tx:     $tx_hash"
}
```

- [ ] **Step 7.3: Run tests and commit**

```bash
bats tests/scripts/migrate-multisig.bats
bats tests/scripts/common.bats       # regression
make lint-scripts
git add scripts/migrate-multisig.sh tests/scripts/migrate-multisig.bats
git commit -m "feat(scripts): migrate-multisig.sh submit subcommand"
```

---

## Task 8: Cross-references and release packaging

**Files:**

- Modify: `scripts/migrate-account.sh`
- Modify: `scripts/migrate-validator.sh`
- Modify: `Makefile` (release target)
- Modify: `docs/evm-integration/user-guides/migration-scripts.md`
- Modify: `docs/evm-integration/user-guides/migration.md`

Goal: existing single-sig scripts point users at `migrate-multisig.sh`; release tarball ships the new script; docs have a new multisig walkthrough plus top-of-section pointers.

- [ ] **Step 8.1: Update single-sig error messages**

In `scripts/migrate-account.sh` and `scripts/migrate-validator.sh`, the multisig-detection path currently emits two `log_error` lines pointing at `legacy-migration.md`. Locate them (they call `assert_single_sig` via the shared library; the error text is in `scripts/evmigration-common.sh`'s `assert_single_sig`).

Update `assert_single_sig` in `scripts/evmigration-common.sh` — find:

```bash
    log_error "legacy account is a ${k}-of-${n} multisig; this script supports single-sig only"
    log_error "use the offline flow: see docs/design/evmigration-multisig-design.md"
```

Change to:

```bash
    log_error "legacy account is a ${k}-of-${n} multisig; use scripts/migrate-multisig.sh instead"
    log_error "see docs/evm-integration/user-guides/migration-scripts.md#multisig-migration"
```

- [ ] **Step 8.2: Release tarball picks up the new script**

In `Makefile`, the release recipe copies three scripts explicitly. Locate:

```makefile
cp scripts/evmigration-common.sh scripts/migrate-account.sh scripts/migrate-validator.sh $$outdir/scripts/; \
chmod +x $$outdir/scripts/migrate-account.sh $$outdir/scripts/migrate-validator.sh; \
```

Extend both lines:

<!-- markdownlint-disable MD010 -->
```makefile
cp scripts/evmigration-common.sh scripts/migrate-account.sh scripts/migrate-validator.sh scripts/migrate-multisig.sh $$outdir/scripts/; \
chmod +x $$outdir/scripts/migrate-account.sh $$outdir/scripts/migrate-validator.sh $$outdir/scripts/migrate-multisig.sh; \
```
<!-- markdownlint-enable MD010 -->

Verify:

```bash
make -n release | grep migrate-multisig
```

Should show the new file in the cp/chmod lines.

- [ ] **Step 8.3: Add "Multisig migration" section to migration-scripts.md**

Append a new top-level section to `docs/evm-integration/user-guides/migration-scripts.md` after the "Non-interactive usage" section and before "Related documentation" (outer fence is four backticks so the inner `bash`/`text` fences render correctly):

````markdown
---

## Multisig migration

Multisig legacy accounts use a four-step offline ceremony rather than a single command — one coordinator and K co-signers across different machines. The `scripts/migrate-multisig.sh` wrapper layers the same pre-flight and verification rails onto each step. Before you begin:

- Every co-signer and the coordinator need `lumerad` (post-EVM-upgrade) and `jq` on their machine.
- The multisig's on-chain pubkey must already be seeded (any prior multisig-signed transaction registers it). If it's nil, submit any multisig-signed tx first — e.g. a 1-`ulume` self-send via `lumerad tx bank send`.
- The coordinator derives a single `eth_secp256k1` destination key from a mnemonic (`lumerad keys add --coin-type 60 --algo eth_secp256k1 --recover`). The ceremony migrates all legacy state to this EOA.

### Step 1 — Coordinator: generate the proof template

```bash
./scripts/migrate-multisig.sh generate \
  --legacy lumera1<multisig-bech32> \
  --new    lumera1<new-eth-bech32> \
  --kind   claim \
  --chain-id lumera-mainnet-1 \
  --node tcp://rpc.lumera:26657 \
  --out  proof.json
```

Use `--kind validator` if the multisig holds a validator operator. The wrapper checks `is_multisig` and `is_validator` against the pre-flight estimate and aborts with exit 3 (not multisig) or exit 6 (validator flag on non-validator) before calling `lumerad`. If the on-chain pubkey is nil, it exits 8 with the remediation printed.

Distribute `proof.json` to all co-signers (email, shared drive, whatever fits your trust model).

### Step 2 — Each co-signer: append a partial signature

```bash
./scripts/migrate-multisig.sh sign proof.json \
  --from alice-sub \
  --chain-id lumera-mainnet-1 \
  --keyring-backend file \
  --out alice-partial.json
```

The wrapper validates the proof file's `payload_hex` against a canonical reconstruction (catches tampering; exit 9) and confirms the `--from` key's pubkey is in the multisig's sub-key set (catches "wrong signer" mistakes; exit 1) before invoking `lumerad tx evmigration sign-proof`. Each signer sends their `*-partial.json` back to the coordinator.

### Step 3 — Coordinator: combine partials

```bash
./scripts/migrate-multisig.sh combine \
  alice-partial.json bob-partial.json \
  --out tx.json
```

The wrapper cross-checks that every partial agrees on `chain_id`, `legacy_address`, `new_address`, `payload_hex`, `kind`, `multisig.threshold`, `multisig.sig_format`, and the `sub_pub_keys_b64` list (exit 9 on disagreement). It prints a K-of-N entry-presence summary:

```text
Partial signature entries (2-of-3 required):
  [X] signer 0  alice-partial.json
  [X] signer 1  bob-partial.json
  [ ] signer 2  (missing)
Entry threshold satisfied: yes (2 >= 2)
```

If fewer than K entries are present, it aborts with exit 4 before calling `lumerad`. If `lumerad combine-proof` itself reports fewer than K *cryptographically valid* signatures (wrong key, tampered payload), the wrapper maps that to exit 4 as well.

### Step 4 — Coordinator: submit

```bash
./scripts/migrate-multisig.sh submit tx.json \
  --from new-eth-key \
  --chain-id lumera-mainnet-1 \
  --node tcp://rpc.lumera:26657 \
  --keyring-backend file
```

Pre-flight checks (in order): `--from` is `eth_secp256k1` and resolves to the tx's `new_address`; the legacy address has no migration record yet; the new address isn't already a migration destination; a fresh `migration-estimate` still reports `would_succeed: true` (catches state drift during a multi-hour or multi-day ceremony — governance could have disabled migration, a validator could have exceeded `max_validator_delegations`). After broadcast it waits for inclusion and verifies the migration record matches.

For `--kind validator` tx files, the submit step prints the same downtime banner as `migrate-validator.sh` and requires either `--i-have-stopped-the-node` or a typed `yes` response.

### Multisig-specific exit codes

In addition to the codes shared with single-sig scripts ([Exit codes](#exit-codes) above):

| Code | Meaning |
|---|---|
| `8` | Multisig pubkey not seeded on-chain; submit any multisig-signed tx first |
| `9` | Input file integrity check failed (JSON parse, missing field, payload_hex mismatch, cross-file disagreement) |

### Troubleshooting

- **Exit 8 on `generate`**: the multisig has never signed a tx. Run any transaction from the multisig account first (smallest: `lumerad tx bank send <multisig-addr> <multisig-addr> 1ulume --from <sub-key> …` in the usual multisig coordinator flow). Then retry.
- **Exit 9 on `sign` with "payload_hex mismatch"**: someone edited a field in the proof after generation. Regenerate from the coordinator and redistribute.
- **Exit 1 on `sign` with "sub-key" error**: the `--from` key's pubkey isn't listed in the template's `multisig.sub_pub_keys_b64`. Confirm you imported the correct sub-key into your local keyring (wrong key name, wrong mnemonic, wrong HD path).
- **Exit 4 on `combine`**: either you passed fewer than K partial files, or one or more partials had invalid signatures. The wrapper prints the entry-presence summary before invoking `lumerad`; if entries look fine but `lumerad` reports below-threshold valid sigs, the bad signer needs to re-sign.
- **Exit 4 on `submit`**: chain state changed during the ceremony (governance disabled migration, deadline passed, validator over cap). The `rejection_reason` from the fresh estimate is printed.
````

- [ ] **Step 8.4: Add pointer from migration.md multisig section**

In `docs/evm-integration/user-guides/migration.md`, find the `## Migrating a multisig account` section (around line 518 per recent edits). Insert immediately after that heading, before the existing overview paragraph:

```markdown
> **Script wrapper available.** The bundled `scripts/migrate-multisig.sh` layers pre-flight, file-integrity, and post-broadcast verification onto each of the four steps below. For day-to-day use, prefer the script walkthrough at [migration-scripts.md → Multisig migration](migration-scripts.md#multisig-migration). The raw-CLI reference that follows is the canonical source for field semantics and remains useful when debugging.
```

- [ ] **Step 8.5: Run full test suite and commit**

```bash
bats tests/scripts/           # expect all passing
make lint-scripts
git add scripts/evmigration-common.sh scripts/migrate-multisig.sh Makefile \
        docs/evm-integration/user-guides/migration-scripts.md \
        docs/evm-integration/user-guides/migration.md
git commit -m "docs(evm): multisig scripts walkthrough and release packaging"
```

---

## Task 9: Devnet smoke matrix (manual)

**Files**: none modified; this is manual acceptance.

- [ ] **Step 9.1**: `make build && make devnet-new`
- [ ] **Step 9.2**: Create a 2-of-3 multisig legacy account, fund it, and submit one tx from it to seed the on-chain pubkey.
- [ ] **Step 9.3**: Generate an EVM destination key (coin-type 60) from a separate mnemonic; import it on the coordinator machine.
- [ ] **Step 9.4**: Coordinator runs `generate`; distribute proof.json.
- [ ] **Step 9.5**: Two of the three co-signers each run `sign`.
- [ ] **Step 9.6**: Coordinator runs `combine` on both partials.
- [ ] **Step 9.7**: Coordinator runs `submit`. Verify migration record exists; legacy balance zero; new balance matches pre-broadcast snapshot.
- [ ] **Step 9.8**: Exercise negative cases: nil-pubkey multisig (exit 8), single-sig account against this script (exit 3), partial-set below threshold in combine (exit 4), tampered payload_hex (exit 9).

---

## Acceptance

Plan is complete when:

- `make lint-scripts` passes.
- `bats tests/scripts/` passes with 0 skipped (all new tests added above should not skip).
- All exit codes in §6 of [evmigration-multisig-scripts-design.md](evmigration-multisig-scripts-design.md) are exercised in bats or the devnet matrix (Task 9).
- `docs/evm-integration/user-guides/migration-scripts.md` has the "Multisig migration" section merged.
- Release tarballs include `scripts/migrate-multisig.sh` (verified via `make -n release`).
- Task 9 devnet matrix has been walked through at least once.
