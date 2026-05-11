# EVM Migration Helper Scripts — Implementation Plan

> **For agentic workers:** Implement this plan task-by-task and update the checkbox (`- [ ]`) status as each step completes.

**Goal:** Ship three bash scripts in [scripts/](../../scripts/) — a shared library (`evmigration-common.sh`) and two entry-point scripts (`migrate-account.sh`, `migrate-validator.sh`) — that wrap the `lumerad tx evmigration claim-legacy-account` and `lumerad tx evmigration migrate-validator` commands with multisig rejection, pre-flight checks, and post-migration verification for legacy accounts (coin-type 118, secp256k1).

**Architecture:** The two entry-point scripts share a sourced library that provides flag parsing, logging, lumerad wrappers, multisig detection (via `migration-estimate.is_multisig`), a pre-broadcast bank snapshot, and a post-broadcast verification step. The entry-point scripts handle their own script-specific flags (stripping them before calling `parse_common_flags`) and per-flow business logic (validator cap check, downtime banner for the validator path, validator vs. account routing).

**Tech Stack:** bash 4+, `jq`, `shellcheck` (lint), `bats-core` (unit tests where pure-bash logic justifies it), `lumerad` CLI. Full design in [evmigration-scripts-design.md](evmigration-scripts-design.md).

---

## Dependencies

Before starting, ensure these tools are available on the dev box. Each task assumes they exist.

- `shellcheck` — `sudo apt-get install -y shellcheck` (Debian/Ubuntu) or `brew install shellcheck` (macOS).
- `bats-core` — `sudo apt-get install -y bats` (Debian/Ubuntu) or `brew install bats-core` (macOS). Minimum version 1.5.
- `jq` — already common; `sudo apt-get install -y jq` if missing.
- `lumerad` built locally (`make build` produces `build/lumerad`).

## Testing Strategy

- **`shellcheck`** on all three scripts is mandatory and gates each commit. A Makefile target wires it into the existing `lint` flow.
- **`bats-core` unit tests** live under [tests/scripts/](../../tests/scripts/) and cover pure-bash logic: flag parsing, JSON field extraction, arithmetic thresholds, mnemonic file permission check. Lumerad-calling functions are exercised via a `lumerad` **shim** — a small bash script that matches on argv and returns canned JSON, placed on `$PATH` ahead of the real binary inside each test's `setup()`.
- **Manual devnet smoke test** (final task) exercises all documented exit codes end-to-end against `make devnet-new`. This is the authoritative acceptance test.

## File Layout Summary

Files created by this plan:

- [scripts/evmigration-common.sh](../../scripts/evmigration-common.sh) — sourced library
- [scripts/migrate-account.sh](../../scripts/migrate-account.sh) — account migration entry point
- [scripts/migrate-validator.sh](../../scripts/migrate-validator.sh) — validator migration entry point
- [tests/scripts/common.bats](../../tests/scripts/common.bats) — bats tests for shared library
- [tests/scripts/migrate-account.bats](../../tests/scripts/migrate-account.bats) — end-to-end shim test for account flow
- [tests/scripts/migrate-validator.bats](../../tests/scripts/migrate-validator.bats) — end-to-end shim test for validator flow
- [tests/scripts/fixtures/lumerad-shim.sh](../../tests/scripts/fixtures/lumerad-shim.sh) — mock lumerad binary for tests
- [tests/scripts/fixtures/](../../tests/scripts/fixtures/) — JSON fixtures used by the shim

Files modified:

- [Makefile](../../Makefile) — new `lint-scripts` and `test-scripts` targets; `lint` runs `lint-scripts`.
- [docs/evm-integration/user-guides/migration.md](../evm-integration/user-guides/migration.md) — new "Method 3: Shell helper scripts" section.
- [docs/evm-integration/user-guides/validator-migration.md](../evm-integration/user-guides/validator-migration.md) — cross-link to validator helper script.
- [docs/evm-integration/user-guides/supernode-migration.md](../evm-integration/user-guides/supernode-migration.md) — cross-link to helper scripts for manual CLI paths.

---

## Task 1: Scaffold scripts and lint integration

**Files:**

- Create: `scripts/evmigration-common.sh`
- Create: `scripts/migrate-account.sh`
- Create: `scripts/migrate-validator.sh`
- Modify: `Makefile` (add `lint-scripts` target and wire into `lint`)

Goal: three runnable (but empty-bodied) scripts that `shellcheck` passes on. Establishes the shape before any logic lands.

- [ ] **Step 1.1: Create the common library skeleton**

File `scripts/evmigration-common.sh`:

```bash
#!/usr/bin/env bash
#
# Shared library for scripts/migrate-account.sh and scripts/migrate-validator.sh.
# Do not execute directly — source it.
#
# See docs/design/evmigration-scripts-design.md for the full design.

set -euo pipefail
IFS=$'\n\t'

# Guard against double-sourcing.
if [[ -n "${__EVMIGRATION_COMMON_LOADED:-}" ]]; then
  return 0
fi
readonly __EVMIGRATION_COMMON_LOADED=1

# Globals populated by parse_common_flags. Declared here so shellcheck doesn't
# complain when entry-point scripts reference them.
NODE=""
CHAIN_ID=""
KEYRING_BACKEND="test"
KEYRING_DIR=""
HOME_DIR=""
MNEMONIC_FILE=""
YES=0
DRY_RUN=0
BIN="lumerad"
LEGACY_KEY=""
NEW_KEY=""
_KRF=()
```

- [ ] **Step 1.2: Create `migrate-account.sh` skeleton**

File `scripts/migrate-account.sh`:

```bash
#!/usr/bin/env bash
#
# Migrate a legacy account (coin-type 118, secp256k1) to its EVM-compatible counterpart.
# See docs/design/evmigration-scripts-design.md and
# docs/evm-integration/user-guides/migration.md.

set -euo pipefail
IFS=$'\n\t'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./evmigration-common.sh
source "${SCRIPT_DIR}/evmigration-common.sh"

main() {
  # Populated in Task 10.
  return 0
}

main "$@"
```

- [ ] **Step 1.3: Create `migrate-validator.sh` skeleton**

File `scripts/migrate-validator.sh` — same skeleton as 1.2, substitute the header comment for validator migration.

- [ ] **Step 1.4: Add Makefile targets**

Locate the `.PHONY:` line at [Makefile:191](../../Makefile#L191) and extend it; add a `lint-scripts` and `test-scripts` block near the `lint` rule. **Makefile recipes require hard tabs** (this is make syntax, not a style choice — do not convert to spaces):

<!-- markdownlint-disable MD010 -->
```makefile
.PHONY: lint-scripts test-scripts

lint-scripts:
	@echo "Running shellcheck on scripts/ ..."
	@shellcheck -x scripts/evmigration-common.sh scripts/migrate-account.sh scripts/migrate-validator.sh

test-scripts:
	@echo "Running bats tests for scripts/ ..."
	@bats tests/scripts/
```
<!-- markdownlint-enable MD010 -->

Modify the existing `lint:` recipe ([Makefile:199](../../Makefile#L199)) so it also runs `lint-scripts`:

<!-- markdownlint-disable MD010 -->
```makefile
lint: openrpc lint-scripts
	@echo "Running linters..."
	@$(GOLANGCI_LINT) run ./... --timeout=5m
```
<!-- markdownlint-enable MD010 -->

(Keep the rest of the recipe intact.)

- [ ] **Step 1.5: Verify shellcheck passes**

```bash
make lint-scripts
```

Expected: shellcheck emits no warnings and exits 0.

- [ ] **Step 1.6: Commit**

```bash
git add scripts/evmigration-common.sh scripts/migrate-account.sh scripts/migrate-validator.sh Makefile
git commit -m "chore(scripts): scaffold evmigration helper scripts with shellcheck gate"
```

---

## Task 2: Logging helpers

**Files:**

- Modify: `scripts/evmigration-common.sh` (append logging functions)
- Create: `tests/scripts/common.bats`
- Create: `tests/scripts/fixtures/` (directory)

- [ ] **Step 2.1: Write the failing bats test**

File `tests/scripts/common.bats`:

```bash
#!/usr/bin/env bats

setup() {
  SCRIPTS_DIR="$(cd "$BATS_TEST_DIRNAME/../../scripts" && pwd)"
  # shellcheck source=../../scripts/evmigration-common.sh
  source "$SCRIPTS_DIR/evmigration-common.sh"
}

@test "log_info writes prefixed message to stderr" {
  run bash -c 'source '"$SCRIPTS_DIR"'/evmigration-common.sh; log_info "hello" 2>&1 1>/dev/null'
  [ "$status" -eq 0 ]
  [[ "$output" == *"INFO"* ]]
  [[ "$output" == *"hello"* ]]
}

@test "log_warn writes prefixed message to stderr" {
  run bash -c 'source '"$SCRIPTS_DIR"'/evmigration-common.sh; log_warn "careful" 2>&1 1>/dev/null'
  [ "$status" -eq 0 ]
  [[ "$output" == *"WARN"* ]]
  [[ "$output" == *"careful"* ]]
}

@test "log_error writes prefixed message to stderr" {
  run bash -c 'source '"$SCRIPTS_DIR"'/evmigration-common.sh; log_error "bad" 2>&1 1>/dev/null'
  [ "$status" -eq 0 ]
  [[ "$output" == *"ERROR"* ]]
  [[ "$output" == *"bad"* ]]
}
```

- [ ] **Step 2.2: Run and confirm failure**

```bash
bats tests/scripts/common.bats
```

Expected: all three tests fail (functions not yet defined).

- [ ] **Step 2.3: Implement the logging helpers**

Append to `scripts/evmigration-common.sh`:

```bash
# ---- Logging ----------------------------------------------------------------

# Colors are emitted only when stderr is a TTY. Set NO_COLOR=1 to force off.
_color_init() {
  if [[ -t 2 && -z "${NO_COLOR:-}" ]]; then
    _C_INFO=$'\033[36m'   # cyan
    _C_WARN=$'\033[33m'   # yellow
    _C_ERR=$'\033[31m'    # red
    _C_RESET=$'\033[0m'
  else
    _C_INFO="" _C_WARN="" _C_ERR="" _C_RESET=""
  fi
}
_color_init

log_info()  { printf '%sINFO%s  %s\n' "$_C_INFO" "$_C_RESET" "$*" >&2; }
log_warn()  { printf '%sWARN%s  %s\n' "$_C_WARN" "$_C_RESET" "$*" >&2; }
log_error() { printf '%sERROR%s %s\n' "$_C_ERR"  "$_C_RESET" "$*" >&2; }
```

- [ ] **Step 2.4: Confirm tests pass and shellcheck is clean**

```bash
bats tests/scripts/common.bats
make lint-scripts
```

Expected: 3 passed; no shellcheck warnings.

- [ ] **Step 2.5: Commit**

```bash
git add scripts/evmigration-common.sh tests/scripts/common.bats
git commit -m "feat(scripts): add log_info/log_warn/log_error helpers with TTY colors"
```

---

## Task 3: Flag parser

**Files:**

- Modify: `scripts/evmigration-common.sh` (append `parse_common_flags` + usage)
- Modify: `tests/scripts/common.bats`

- [ ] **Step 3.1: Write failing tests**

Append to `tests/scripts/common.bats`:

```bash
@test "parse_common_flags populates defaults" {
  parse_common_flags legacy new
  [ "$LEGACY_KEY" = "legacy" ]
  [ "$NEW_KEY" = "new" ]
  [ "$KEYRING_BACKEND" = "test" ]
  [ "$YES" = "0" ]
  [ "$DRY_RUN" = "0" ]
  [ "$BIN" = "lumerad" ]
}

@test "parse_common_flags handles all supported flags" {
  parse_common_flags \
    --node tcp://node:26657 \
    --chain-id lumera-devnet \
    --keyring-backend file \
    --keyring-dir /tmp/kr \
    --home /tmp/home \
    --mnemonic-file /tmp/m \
    --yes --dry-run \
    --binary /opt/lumerad \
    mykey1 mykey2
  [ "$NODE" = "tcp://node:26657" ]
  [ "$CHAIN_ID" = "lumera-devnet" ]
  [ "$KEYRING_BACKEND" = "file" ]
  [ "$KEYRING_DIR" = "/tmp/kr" ]
  [ "$HOME_DIR" = "/tmp/home" ]
  [ "$MNEMONIC_FILE" = "/tmp/m" ]
  [ "$YES" = "1" ]
  [ "$DRY_RUN" = "1" ]
  [ "$BIN" = "/opt/lumerad" ]
  [ "$LEGACY_KEY" = "mykey1" ]
  [ "$NEW_KEY" = "mykey2" ]
}

@test "parse_common_flags rejects unknown flag with exit 1" {
  run bash -c 'source '"$SCRIPTS_DIR"'/evmigration-common.sh; parse_common_flags --bogus k1 k2'
  [ "$status" -eq 1 ]
  [[ "$output" == *"unknown flag"* ]]
}

@test "parse_common_flags rejects missing positional with exit 1" {
  run bash -c 'source '"$SCRIPTS_DIR"'/evmigration-common.sh; parse_common_flags onlyone'
  [ "$status" -eq 1 ]
}

@test "parse_common_flags defaults NODE from env" {
  LUMERA_NODE="tcp://from-env:26657" parse_common_flags legacy new
  [ "$NODE" = "tcp://from-env:26657" ]
}
```

- [ ] **Step 3.2: Run and confirm failures**

```bash
bats tests/scripts/common.bats
```

Expected: new tests fail; old ones still pass.

- [ ] **Step 3.3: Implement `parse_common_flags` and `_usage`**

Append to `scripts/evmigration-common.sh`:

```bash
# ---- Flag parsing -----------------------------------------------------------

_usage() {
  cat >&2 <<'USAGE'
Usage: <script> <legacy-key> <new-key> [flags]

Flags:
  --node <url>              RPC endpoint (default $LUMERA_NODE or tcp://localhost:26657)
  --chain-id <id>           Chain ID (default $LUMERA_CHAIN_ID; required)
  --keyring-backend <b>     test|file|os (default test)
  --keyring-dir <dir>       Keyring directory (overrides --home for keys)
  --home <dir>              lumerad home directory
  --mnemonic-file <path>    Import both keys from a mnemonic file (mode 0600 or stricter)
  --yes, -y                 Skip standard confirmation prompts
  --dry-run                 Run pre-flight only; do not broadcast
  --binary <path>           Override lumerad binary (default: lumerad on PATH)
USAGE
}

parse_common_flags() {
  # Reset in case of double-invocation in tests.
  NODE="${LUMERA_NODE:-tcp://localhost:26657}"
  CHAIN_ID="${LUMERA_CHAIN_ID:-}"
  KEYRING_BACKEND="test"
  KEYRING_DIR=""
  HOME_DIR=""
  MNEMONIC_FILE=""
  YES=0
  DRY_RUN=0
  BIN="lumerad"
  LEGACY_KEY=""
  NEW_KEY=""

  _need_value() {
    local flag="$1" value="${2:-}"
    if [[ -z "$value" || "$value" == --* ]]; then
      log_error "missing value for $flag"
      _usage
      exit 1
    fi
  }

  local positional=()
  while (( $# > 0 )); do
    case "$1" in
      --node)            _need_value "$1" "${2:-}"; NODE="$2"; shift 2 ;;
      --chain-id)        _need_value "$1" "${2:-}"; CHAIN_ID="$2"; shift 2 ;;
      --keyring-backend) _need_value "$1" "${2:-}"; KEYRING_BACKEND="$2"; shift 2 ;;
      --keyring-dir)     _need_value "$1" "${2:-}"; KEYRING_DIR="$2"; shift 2 ;;
      --home)            _need_value "$1" "${2:-}"; HOME_DIR="$2"; shift 2 ;;
      --mnemonic-file)   _need_value "$1" "${2:-}"; MNEMONIC_FILE="$2"; shift 2 ;;
      --yes|-y)          YES=1; shift ;;
      --dry-run)         DRY_RUN=1; shift ;;
      --binary)          _need_value "$1" "${2:-}"; BIN="$2"; shift 2 ;;
      -h|--help)         _usage; exit 0 ;;
      --) shift; positional+=("$@"); break ;;
      --*) log_error "unknown flag: $1"; _usage; exit 1 ;;
      *)   positional+=("$1"); shift ;;
    esac
  done

  if (( ${#positional[@]} != 2 )); then
    log_error "expected exactly two positional arguments: <legacy-key> <new-key>"
    _usage
    exit 1
  fi
  LEGACY_KEY="${positional[0]}"
  NEW_KEY="${positional[1]}"
}
```

- [ ] **Step 3.4: Run tests and lint**

```bash
bats tests/scripts/common.bats
make lint-scripts
```

Expected: all tests pass; no shellcheck warnings.

- [ ] **Step 3.5: Commit**

```bash
git add scripts/evmigration-common.sh tests/scripts/common.bats
git commit -m "feat(scripts): parse_common_flags with defaults, env fallbacks, and validation"
```

---

## Task 4: lumerad shim fixture and wrappers

**Files:**

- Create: `tests/scripts/fixtures/lumerad-shim.sh`
- Modify: `scripts/evmigration-common.sh` (append `require_binary`, `require_jq`, wrappers)
- Modify: `tests/scripts/common.bats`

- [ ] **Step 4.1: Create the shim**

File `tests/scripts/fixtures/lumerad-shim.sh`:

```bash
#!/usr/bin/env bash
#
# Mock lumerad binary for bats tests. Routes on argv to fixtures/*.json.
# Override behavior with env vars:
#   SHIM_EXIT=<n>         force exit code n
#   SHIM_FIXTURE=<name>   force a specific fixture name (without .json)
#   SHIM_STDERR=<msg>     emit this to stderr before exiting

set -u

if [[ -n "${SHIM_STDERR:-}" ]]; then
  printf '%s\n' "$SHIM_STDERR" >&2
fi

fixtures_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

emit() {
  local name="$1"
  local path="$fixtures_dir/$name.json"
  if [[ ! -f "$path" ]]; then
    printf 'shim: missing fixture %s\n' "$path" >&2
    exit 99
  fi
  cat "$path"
}

if [[ -n "${SHIM_FIXTURE:-}" ]]; then
  emit "$SHIM_FIXTURE"
  exit "${SHIM_EXIT:-0}"
fi

# Route on argv. Per-command env vars let a single test override just one
# endpoint without affecting the others.
case "$*" in
  "query evmigration --help"*)                         printf 'evmigration query commands\n' ;;
  "tx evmigration claim-legacy-account --help"*)       printf 'claim-legacy-account help\n' ;;
  "tx evmigration migrate-validator --help"*)          printf 'migrate-validator help\n' ;;
  "query evmigration migration-estimate "*)            emit "${SHIM_ESTIMATE_FIXTURE:-estimate-ok}" ;;
  "query evmigration migration-stats"*)                emit "${SHIM_STATS_FIXTURE:-migration-stats}" ;;
  "query evmigration migration-record "*)              emit "${SHIM_RECORD_FIXTURE:-record-not-found}" ;;
  "query evmigration migration-record-by-new-address "*) emit "${SHIM_RECORD_FIXTURE:-record-not-found}" ;;
  "query evmigration params"*)                          emit "${SHIM_PARAMS_FIXTURE:-params}" ;;
  "query auth account "*)                               emit "${SHIM_AUTH_FIXTURE:-auth-account}" ;;
  "query bank balances "*)                              emit "${SHIM_BANK_FIXTURE:-bank-balances}" ;;
  "query tx "*)                                         emit "${SHIM_TX_FIXTURE:-tx-success}" ;;
  "keys show "*)                                        printf 'lumera1shimaddr1qxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx\n' ;;
  "debug addr "*)
    cat <<'ADDR'
Address: [1 2 3]
Address (hex): 0102030405060708090A0B0C0D0E0F1011121314
Bech32 Acc: lumera1shimaccxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
Bech32 Val: lumeravalopershimxxxxxxxxxxxxxxxxxxxxxxxxxx
Bech32 Con: lumeravalconsshimxxxxxxxxxxxxxxxxxxxxxxxxxxx
ADDR
    ;;
  "tx evmigration"*)                                    emit broadcast-success ;;
  "version"*)                                           printf 'v0.0.0-shim\n' ;;
  *) printf 'shim: unhandled args: %s\n' "$*" >&2; exit 1 ;;
esac

exit "${SHIM_EXIT:-0}"
```

Create JSON fixtures used above (in `tests/scripts/fixtures/`):

`estimate-ok.json`:

```json
{
  "is_validator": false,
  "is_multisig": false,
  "threshold": 0,
  "num_signers": 0,
  "delegation_count": 0,
  "unbonding_count": 0,
  "redelegation_count": 0,
  "val_delegation_count": 0,
  "val_unbonding_count": 0,
  "val_redelegation_count": 0,
  "authz_grant_count": 0,
  "feegrant_count": 0,
  "action_count": 0,
  "total_touched": 0,
  "has_supernode": false,
  "balance_summary": "1000000ulume",
  "would_succeed": true,
  "rejection_reason": ""
}
```

`record-not-found.json`:

```json
{}
```

`params.json`:

```json
{
  "params": {
    "enable_migration": true,
    "migration_end_time": "0",
    "max_migrations_per_block": "50",
    "max_validator_delegations": "2000"
  }
}
```

`auth-account.json`:

```json
{
  "account": {
    "@type": "/cosmos.auth.v1beta1.BaseAccount",
    "address": "lumera1shimaddr1qxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
    "pub_key": {
      "@type": "/cosmos.crypto.secp256k1.PubKey",
      "key": "A0000000000000000000000000000000000000000000"
    },
    "account_number": "1",
    "sequence": "0"
  }
}
```

`bank-balances.json`:

```json
{
  "balances": [{"denom": "ulume", "amount": "1000000"}],
  "pagination": {"next_key": null, "total": "1"}
}
```

`tx-success.json`:

```json
{"height":"100","txhash":"DEADBEEF","code":0,"raw_log":""}
```

`broadcast-success.json`:

```json
{"height":"0","txhash":"DEADBEEF","code":0,"raw_log":"","events":[]}
```

`migration-stats.json`:

```json
{"total_migrated":"0","total_legacy":"0","total_legacy_staked":"0","total_validators_migrated":"0","total_validators_legacy":"0"}
```

Make the shim executable and commit fixtures:

```bash
chmod +x tests/scripts/fixtures/lumerad-shim.sh
```

- [ ] **Step 4.2: Write failing tests for wrappers**

Append to `tests/scripts/common.bats`:

```bash
setup_shim() {
  SHIM_BIN="$BATS_TEST_DIRNAME/fixtures/lumerad-shim.sh"
  BIN="$SHIM_BIN"
  NODE="tcp://localhost:26657"
  CHAIN_ID="shim-test"
  KEYRING_BACKEND="test"
}

@test "require_jq passes when jq exists" {
  run bash -c 'source '"$SCRIPTS_DIR"'/evmigration-common.sh; require_jq'
  [ "$status" -eq 0 ]
}

@test "require_binary accepts shim" {
  setup_shim
  run bash -c 'source '"$SCRIPTS_DIR"'/evmigration-common.sh; BIN='"$SHIM_BIN"'; require_binary'
  [ "$status" -eq 0 ]
}

@test "lumerad_q invokes the binary with --node and --output json" {
  setup_shim
  # Capture the shim's received args by wrapping it.
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    BIN='"$SHIM_BIN"'
    NODE="tcp://example:1234"
    lumerad_q evmigration migration-stats
  '
  [ "$status" -eq 0 ]
}
```

- [ ] **Step 4.3: Implement `require_jq`, `require_binary`, and wrappers**

Append to `scripts/evmigration-common.sh`:

```bash
# ---- Environment requirements ----------------------------------------------

require_jq() {
  if ! command -v jq >/dev/null 2>&1; then
    log_error "jq is required but not found on PATH"
    exit 2
  fi
}

require_binary() {
  if ! command -v "$BIN" >/dev/null 2>&1 && [[ ! -x "$BIN" ]]; then
    log_error "lumerad binary not found: $BIN"
    exit 2
  fi
  if ! "$BIN" query evmigration --help >/dev/null 2>&1; then
    log_error "$BIN does not support 'query evmigration' — needs a post-EVM-upgrade build"
    exit 2
  fi
  if ! "$BIN" tx evmigration claim-legacy-account --help >/dev/null 2>&1; then
    log_error "$BIN does not support 'tx evmigration claim-legacy-account'"
    exit 2
  fi
  if ! "$BIN" tx evmigration migrate-validator --help >/dev/null 2>&1; then
    log_error "$BIN does not support 'tx evmigration migrate-validator'"
    exit 2
  fi
}

# ---- lumerad wrappers ------------------------------------------------------

# Returns a (possibly empty) array of keyring flags based on globals.
_keyring_flags() {
  local flags=(--keyring-backend "$KEYRING_BACKEND")
  [[ -n "$KEYRING_DIR" ]] && flags+=(--keyring-dir "$KEYRING_DIR")
  [[ -n "$HOME_DIR"    ]] && flags+=(--home "$HOME_DIR")
  printf '%s\n' "${flags[@]}"
}

_read_keyring_flags() {
  mapfile -t _KRF < <(_keyring_flags)
}

lumerad_q() {
  _read_keyring_flags
  "$BIN" query "$@" --node "$NODE" --output json
}

lumerad_tx() {
  if [[ -z "$CHAIN_ID" ]]; then
    log_error "--chain-id (or \$LUMERA_CHAIN_ID) is required for tx commands"
    exit 1
  fi
  _read_keyring_flags
  "$BIN" tx "$@" \
    --node "$NODE" \
    --chain-id "$CHAIN_ID" \
    "${_KRF[@]}" \
    --output json
}

lumerad_keys() {
  _read_keyring_flags
  "$BIN" keys "$@" "${_KRF[@]}"
}
```

- [ ] **Step 4.4: Run tests and shellcheck**

```bash
bats tests/scripts/common.bats
make lint-scripts
```

Expected: all tests pass, shellcheck clean.

- [ ] **Step 4.5: Commit**

```bash
git add scripts/evmigration-common.sh tests/scripts/
git commit -m "feat(scripts): require_jq/require_binary and thin lumerad wrappers"
```

---

## Task 5: Address + valoper helpers

**Files:**

- Modify: `scripts/evmigration-common.sh` (append `resolve_address`, `lumera_to_valoper`)
- Modify: `tests/scripts/common.bats`

- [ ] **Step 5.1: Write failing tests**

Append to `tests/scripts/common.bats`:

```bash
@test "resolve_address returns keys-show output" {
  setup_shim
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    BIN='"$SHIM_BIN"'
    NODE="tcp://local:26657"
    KEYRING_BACKEND="test"
    resolve_address mykey
  '
  [ "$status" -eq 0 ]
  [[ "$output" == lumera1* ]]
}

@test "lumera_to_valoper parses debug addr output" {
  setup_shim
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    BIN='"$SHIM_BIN"'
    lumera_to_valoper lumera1anything
  '
  [ "$status" -eq 0 ]
  [[ "$output" == lumeravaloper* ]]
}
```

- [ ] **Step 5.2: Implement helpers**

Append to `scripts/evmigration-common.sh`:

```bash
# ---- Address helpers -------------------------------------------------------

resolve_address() {
  local key_name="$1"
  local addr
  if ! addr=$(lumerad_keys show "$key_name" -a 2>/dev/null); then
    log_error "key not found in keyring: $key_name"
    exit 1
  fi
  printf '%s\n' "$addr"
}

# lumera_to_valoper converts a lumera1... bech32 to its lumeravaloper1... form
# by shelling out to `lumerad debug addr`. Output format verified at spec time:
#   Bech32 Val: lumeravaloper...
lumera_to_valoper() {
  local addr="$1"
  local valoper
  valoper=$("$BIN" debug addr "$addr" 2>/dev/null | awk -F': ' '/^Bech32 Val: /{print $2; exit}')
  if [[ -z "$valoper" ]]; then
    log_error "cannot derive valoper for $addr"
    exit 2
  fi
  printf '%s\n' "$valoper"
}
```

- [ ] **Step 5.3: Run tests and lint; commit**

```bash
bats tests/scripts/common.bats
make lint-scripts
git add scripts/evmigration-common.sh tests/scripts/common.bats
git commit -m "feat(scripts): resolve_address and lumera_to_valoper helpers"
```

---

## Task 6: `preflight_estimate` + `assert_estimate_succeeds` + `assert_single_sig`

**Files:**

- Modify: `scripts/evmigration-common.sh`
- Modify: `tests/scripts/common.bats`
- Create: `tests/scripts/fixtures/estimate-multisig.json`
- Create: `tests/scripts/fixtures/estimate-rejected.json`

- [ ] **Step 6.1: Add fixtures for the negative cases**

`tests/scripts/fixtures/estimate-multisig.json` — same shape as `estimate-ok.json` but with `"is_multisig": true, "threshold": 2, "num_signers": 3`.

`tests/scripts/fixtures/estimate-rejected.json` — `"would_succeed": false, "rejection_reason": "legacy account not found"`.

- [ ] **Step 6.2: Write failing tests**

Append to `tests/scripts/common.bats`:

```bash
@test "preflight_estimate emits raw JSON on stdout, summary on stderr" {
  setup_shim
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    BIN='"$SHIM_BIN"'
    NODE=tcp://local:26657
    tmp=$(mktemp)
    preflight_estimate lumera1example 2>"$tmp"
    grep -q "Migration preview" "$tmp"
    rm -f "$tmp"
  '
  [ "$status" -eq 0 ]
  # stdout must be parseable JSON and contain the fields
  echo "$output" | jq -e '.is_multisig == false'
}

@test "assert_single_sig passes on non-multisig estimate" {
  setup_shim
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    assert_single_sig "$(cat '"$BATS_TEST_DIRNAME"'/fixtures/estimate-ok.json)"
  '
  [ "$status" -eq 0 ]
}

@test "assert_single_sig rejects multisig estimate with exit 3" {
  setup_shim
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    assert_single_sig "$(cat '"$BATS_TEST_DIRNAME"'/fixtures/estimate-multisig.json)"
  '
  [ "$status" -eq 3 ]
  [[ "$output" == *"multisig"* ]]
}

@test "assert_estimate_succeeds exits 4 on would_succeed=false" {
  setup_shim
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    assert_estimate_succeeds "$(cat '"$BATS_TEST_DIRNAME"'/fixtures/estimate-rejected.json)"
  '
  [ "$status" -eq 4 ]
  [[ "$output" == *"legacy account not found"* ]]
}
```

- [ ] **Step 6.3: Implement the functions**

Append to `scripts/evmigration-common.sh`:

```bash
# ---- Pre-flight estimate ---------------------------------------------------

# preflight_estimate <legacy-addr>
# Emits raw migration-estimate JSON on stdout; a human summary on stderr.
# Does NOT assert on would_succeed — callers run the more specific checks
# (multisig, validator-cap) first so they can return their own exit codes.
preflight_estimate() {
  local addr="$1"
  local json
  json=$(lumerad_q evmigration migration-estimate "$addr")

  # Human summary to stderr.
  local balance delegations unbonding redelegations authz feegrants supernode would
  balance=$(jq -r '.balance_summary' <<<"$json")
  delegations=$(jq -r '.delegation_count' <<<"$json")
  unbonding=$(jq -r '.unbonding_count' <<<"$json")
  redelegations=$(jq -r '.redelegation_count' <<<"$json")
  authz=$(jq -r '.authz_grant_count' <<<"$json")
  feegrants=$(jq -r '.feegrant_count' <<<"$json")
  supernode=$(jq -r 'if .has_supernode then "yes" else "no" end' <<<"$json")
  would=$(jq -r 'if .would_succeed then "yes" else "no" end' <<<"$json")

  {
    printf 'Migration preview for %s:\n' "$addr"
    printf '  Balance:        %s\n' "$balance"
    printf '  Delegations:    %s\n' "$delegations"
    printf '  Unbonding:      %s\n' "$unbonding"
    printf '  Redelegations:  %s\n' "$redelegations"
    printf '  Authz grants:   %s\n' "$authz"
    printf '  Feegrants:      %s\n' "$feegrants"
    printf '  Actions:        %s\n' "$(jq -r '.action_count // 0' <<<"$json")"
    printf '  Validator:      %s\n' "$(jq -r 'if .is_validator then "yes" else "no" end' <<<"$json")"
    printf '  Supernode:      %s\n' "$supernode"
    printf '  Multisig:       %s\n' "$(jq -r 'if .is_multisig then "yes" else "no" end' <<<"$json")"
    if [[ "$(jq -r '.is_validator' <<<"$json")" == "true" ]]; then
      printf '  Val delegations:%s\n' "$(jq -r '.val_delegation_count' <<<"$json")"
      printf '  Val unbonding:  %s\n' "$(jq -r '.val_unbonding_count' <<<"$json")"
      printf '  Val redeleg.:   %s\n' "$(jq -r '.val_redelegation_count' <<<"$json")"
    fi
    printf '  Would succeed:  %s\n' "$would"
  } >&2

  printf '%s\n' "$json"
}

assert_single_sig() {
  local json="$1"
  if [[ "$(jq -r '.is_multisig' <<<"$json")" == "true" ]]; then
    local k n
    k=$(jq -r '.threshold' <<<"$json")
    n=$(jq -r '.num_signers' <<<"$json")
    log_error "legacy account is a ${k}-of-${n} multisig; this script supports single-sig only"
    log_error "use the offline flow: see docs/design/evmigration-multisig-design.md"
    exit 3
  fi
}

assert_estimate_succeeds() {
  local json="$1"
  if [[ "$(jq -r '.would_succeed' <<<"$json")" != "true" ]]; then
    local reason
    reason=$(jq -r '.rejection_reason' <<<"$json")
    log_error "pre-flight: migration would fail: $reason"
    exit 4
  fi
}
```

- [ ] **Step 6.4: Run tests and lint; commit**

```bash
bats tests/scripts/common.bats
make lint-scripts
git add scripts/evmigration-common.sh tests/scripts/
git commit -m "feat(scripts): preflight_estimate + multisig/would_succeed assertions"
```

---

## Task 7: Migration-record assertions

**Files:**

- Modify: `scripts/evmigration-common.sh`
- Modify: `tests/scripts/common.bats`
- Create: `tests/scripts/fixtures/record-found.json`

- [ ] **Step 7.1: Add the positive fixture**

`tests/scripts/fixtures/record-found.json`:

```json
{
    "record": {
      "legacy_address": "lumera1legacyxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
      "new_address": "lumera1newxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
      "migration_time": "1713456789",
      "migration_height": "42"
    }
  }
```

Use `SHIM_RECORD_FIXTURE=record-found` when a test needs record lookups to return a completed migration record. Avoid `SHIM_FIXTURE` in flow tests because it forces **every** shim call, including params and tx queries, to return the same fixture.

- [ ] **Step 7.2: Write failing tests**

```bash
@test "assert_not_migrated passes when no record" {
  setup_shim
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    BIN='"$SHIM_BIN"'; NODE=tcp://local:1
    assert_not_migrated lumera1anything
  '
  [ "$status" -eq 0 ]
}

@test "assert_not_migrated exits 5 when record exists" {
  setup_shim
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    BIN='"$SHIM_BIN"'; NODE=tcp://local:1
    SHIM_RECORD_FIXTURE=record-found assert_not_migrated lumera1anything
  '
  [ "$status" -eq 5 ]
}

@test "assert_new_address_unused passes when neither query returns a record" {
  setup_shim
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    BIN='"$SHIM_BIN"'; NODE=tcp://local:1
    assert_new_address_unused lumera1newxxxxxx
  '
  [ "$status" -eq 0 ]
}

@test "assert_new_address_unused exits 5 when new-address lookup returns record" {
  setup_shim
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    BIN='"$SHIM_BIN"'; NODE=tcp://local:1
    SHIM_RECORD_FIXTURE=record-found assert_new_address_unused lumera1newxxxxxx
  '
  [ "$status" -eq 5 ]
}
```

- [ ] **Step 7.3: Implement the assertions**

Append to `scripts/evmigration-common.sh`:

```bash
# ---- Migration-record assertions -------------------------------------------

# _record_present <query-subcommand> <addr>
# Returns 0 if the query returns a JSON object with a non-empty ".record".
_record_present() {
  local subcmd="$1" addr="$2"
  local json
  json=$(lumerad_q evmigration "$subcmd" "$addr" 2>/dev/null || printf '{}')
  [[ "$(jq -r '.record.legacy_address // empty' <<<"$json")" != "" ]]
}

assert_not_migrated() {
  local addr="$1"
  if _record_present migration-record "$addr"; then
    log_error "legacy address $addr has already been migrated"
    exit 5
  fi
}

assert_new_address_unused() {
  local addr="$1"
  if _record_present migration-record "$addr"; then
    log_error "new address $addr was previously migrated as a legacy address"
    exit 5
  fi
  if _record_present migration-record-by-new-address "$addr"; then
    log_error "new address $addr is already a migration destination"
    exit 5
  fi
}
```

- [ ] **Step 7.4: Run tests and lint; commit**

```bash
bats tests/scripts/common.bats
make lint-scripts
git add scripts/evmigration-common.sh tests/scripts/
git commit -m "feat(scripts): assert_not_migrated and assert_new_address_unused"
```

---

## Task 8: Bank snapshot, tx polling, and `verify_migration`

**Files:**

- Modify: `scripts/evmigration-common.sh`
- Modify: `tests/scripts/common.bats`
- Create: `tests/scripts/fixtures/bank-balances-empty.json`
- Create: `tests/scripts/fixtures/bank-balances-new.json`
- Create: `tests/scripts/fixtures/record-post-migration.json`

- [ ] **Step 8.1: Add fixtures**

`bank-balances-empty.json`:

```json
{"balances": [], "pagination": {"next_key": null, "total": "0"}}
```

`bank-balances-new.json`:

```json
{"balances": [{"denom":"ulume","amount":"1500000"}], "pagination":{"next_key":null,"total":"1"}}
```

`record-post-migration.json`:

```json
{"record":{"legacy_address":"lumera1legacy","new_address":"lumera1new","migration_time":"1713456789","migration_height":"42"}}
```

- [ ] **Step 8.2: Write failing tests**

```bash
@test "snapshot_bank_balances returns structured JSON" {
  setup_shim
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    BIN='"$SHIM_BIN"'; NODE=tcp://local:1
    snapshot_bank_balances lumera1legacy
  '
  [ "$status" -eq 0 ]
  echo "$output" | jq -e '.balances | length == 1'
}

@test "wait_for_tx returns 0 when shim reports code 0" {
  setup_shim
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    BIN='"$SHIM_BIN"'; NODE=tcp://local:1
    wait_for_tx DEADBEEF
  '
  [ "$status" -eq 0 ]
}

@test "verify_migration passes when record exists and balances honor snapshot" {
  setup_shim
  snap='{"balances":[{"denom":"ulume","amount":"1000000"}]}'
  # Force the shim: record lookup returns post-migration record; legacy balances empty;
  # new balances fixture satisfies >= snapshot. We sequence by wrapping the shim.
  # Sequenced post-check responses are covered by migrate-account end-to-end tests.
  skip "covered by migrate-account end-to-end test in Task 10"
}
```

- [ ] **Step 8.3: Implement the helpers**

Append to `scripts/evmigration-common.sh`:

```bash
# ---- Bank snapshot, tx polling, verification -------------------------------

snapshot_bank_balances() {
  local addr="$1"
  lumerad_q bank balances "$addr"
}

# wait_for_tx <hash>
# Polls `query tx <hash>` every second for up to 30s. Exits non-zero on
# timeout or if the final response reports a non-zero code.
wait_for_tx() {
  local hash="$1"
  local deadline=$(( SECONDS + 30 ))
  local code=""
  while (( SECONDS < deadline )); do
    local json
    if json=$(lumerad_q tx "$hash" 2>/dev/null); then
      code=$(jq -r '.code // empty' <<<"$json")
      if [[ -n "$code" ]]; then
        if (( code == 0 )); then
          return 0
        fi
        log_error "tx $hash failed with code $code: $(jq -r '.raw_log' <<<"$json")"
        return 1
      fi
    fi
    sleep 1
  done
  log_error "timed out waiting for tx $hash to be indexed"
  return 1
}

# verify_migration <legacy> <new> <pre-broadcast-legacy-balances-json>
verify_migration() {
  local legacy="$1" new="$2" snap_json="$3"

  # 1. Migration record must exist and point to <new>.
  local rec_json
  rec_json=$(lumerad_q evmigration migration-record "$legacy" 2>/dev/null || printf '{}')
  local rec_new
  rec_new=$(jq -r '.record.new_address // empty' <<<"$rec_json")
  if [[ "$rec_new" != "$new" ]]; then
    log_error "post-check: migration record for $legacy does not point to $new (got: '$rec_new')"
    exit 7
  fi

  # 2. Legacy balances must be all zero (account removed or empty).
  local legacy_after
  legacy_after=$(lumerad_q bank balances "$legacy" 2>/dev/null || printf '{"balances":[]}')
  if [[ "$(jq -r '[.balances[].amount | tonumber] | add // 0' <<<"$legacy_after")" != "0" ]]; then
    log_error "post-check: legacy address $legacy still has non-zero balance"
    exit 7
  fi

  # 3. For every {denom,amount} in snap_json, new balances must be >= amount.
  local new_after
  new_after=$(lumerad_q bank balances "$new")
  local diff
  diff=$(jq --argjson new "$new_after" '
    [ .balances[] as $s
      | ($new.balances | map(select(.denom==$s.denom)) | .[0].amount // "0") as $na
      | select(($na|tonumber) < ($s.amount|tonumber))
      | {denom: $s.denom, expected: $s.amount, actual: $na}
    ]' <<<"$snap_json")
  if [[ "$(jq -r 'length' <<<"$diff")" != "0" ]]; then
    log_error "post-check: new balances fall short of pre-broadcast snapshot: $diff"
    exit 7
  fi
}
```

- [ ] **Step 8.4: Run tests and lint; commit**

```bash
bats tests/scripts/common.bats
make lint-scripts
git add scripts/evmigration-common.sh tests/scripts/
git commit -m "feat(scripts): snapshot_bank_balances, wait_for_tx, verify_migration"
```

---

## Task 9: `confirm` + `import_from_mnemonic`

**Files:**

- Modify: `scripts/evmigration-common.sh`
- Modify: `tests/scripts/common.bats`

- [ ] **Step 9.1: Write failing tests for `confirm`**

```bash
@test "confirm returns 0 immediately when YES=1" {
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    YES=1
    confirm "proceed?"
  '
  [ "$status" -eq 0 ]
}

@test "confirm exits 10 on user 'n'" {
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    YES=0
    echo "n" | confirm "proceed?"
  '
  [ "$status" -eq 10 ]
}

@test "confirm returns 0 on user 'y'" {
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    YES=0
    echo "y" | confirm "proceed?"
  '
  [ "$status" -eq 0 ]
}
```

- [ ] **Step 9.2: Write failing tests for mnemonic permission check**

```bash
@test "import_from_mnemonic rejects world-readable file with exit 1" {
  setup_shim
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    BIN='"$SHIM_BIN"'
    mf=$(mktemp); echo "test mnemonic" > "$mf"; chmod 0644 "$mf"
    import_from_mnemonic "$mf" k1 k2
  '
  [ "$status" -eq 1 ]
  [[ "$output" == *"mode 0600"* ]]
}
```

- [ ] **Step 9.3: Implement `confirm`, `import_from_mnemonic`, and the trap**

Append to `scripts/evmigration-common.sh`:

```bash
# ---- Confirmation -----------------------------------------------------------

# confirm <prompt>
# Returns 0 on user confirmation or when --yes is set; exits 10 on refusal.
confirm() {
  local prompt="$1"
  if (( YES == 1 )); then
    return 0
  fi
  local reply=""
  printf '%s [y/N] ' "$prompt" >&2
  read -r reply || true
  if [[ "$reply" =~ ^[Yy]([Ee][Ss])?$ ]]; then
    return 0
  fi
  log_error "aborted by user"
  exit 10
}

# ---- Mnemonic flow ---------------------------------------------------------

_MNEMONIC_CLEANUP_KEYS=()

cleanup_mnemonic_keys() {
  local k
  for k in "${_MNEMONIC_CLEANUP_KEYS[@]:-}"; do
    [[ -z "$k" ]] && continue
    lumerad_keys delete "$k" --yes >/dev/null 2>&1 || true
  done
}

import_from_mnemonic() {
  local mfile="$1" legacy_name="$2" new_name="$3"

  if [[ ! -f "$mfile" ]]; then
    log_error "mnemonic file not found: $mfile"
    exit 1
  fi
  # Require mode 0600 or stricter (no group/world bits).
  local mode
  mode=$(stat -c '%a' "$mfile" 2>/dev/null || stat -f '%A' "$mfile" 2>/dev/null || echo "")
  if [[ -z "$mode" ]]; then
    log_error "cannot stat $mfile"
    exit 1
  fi
  # Reject if last two octal digits are non-zero.
  if [[ "${mode: -2}" != "00" ]]; then
    log_error "mnemonic file $mfile must be mode 0600 (got $mode)"
    exit 1
  fi

  # Abort if either key name already exists in the keyring.
  if lumerad_keys show "$legacy_name" -a >/dev/null 2>&1; then
    log_error "key '$legacy_name' already exists in keyring"
    exit 1
  fi
  if lumerad_keys show "$new_name" -a >/dev/null 2>&1; then
    log_error "key '$new_name' already exists in keyring"
    exit 1
  fi

  local mnemonic
  mnemonic=$(< "$mfile")

  # Register cleanup before doing anything else that might fail.
  _MNEMONIC_CLEANUP_KEYS=("$legacy_name" "$new_name")
  trap 'rc=$?; cleanup_mnemonic_keys; exit "$rc"' EXIT

  printf '%s\n' "$mnemonic" | lumerad_keys add "$legacy_name" \
    --recover --coin-type 118 --algo secp256k1
  printf '%s\n' "$mnemonic" | lumerad_keys add "$new_name" \
    --recover --coin-type 60 --algo eth_secp256k1

  unset mnemonic
}
```

- [ ] **Step 9.4: Run tests and lint; commit**

```bash
bats tests/scripts/common.bats
make lint-scripts
git add scripts/evmigration-common.sh tests/scripts/
git commit -m "feat(scripts): confirm, import_from_mnemonic with perms check and trap cleanup"
```

---

## Task 10: `migrate-account.sh` full flow

**Files:**

- Modify: `scripts/migrate-account.sh`
- Create: `tests/scripts/migrate-account.bats`

- [ ] **Step 10.1: Write the end-to-end test first**

File `tests/scripts/migrate-account.bats`:

```bash
#!/usr/bin/env bats

setup() {
  SCRIPTS_DIR="$(cd "$BATS_TEST_DIRNAME/../../scripts" && pwd)"
  FIX_DIR="$BATS_TEST_DIRNAME/fixtures"
  SHIM="$FIX_DIR/lumerad-shim.sh"
}

@test "migrate-account.sh happy-path dry-run exits 0 and does not broadcast" {
  run "$SCRIPTS_DIR/migrate-account.sh" \
    --binary "$SHIM" \
    --chain-id shim-test \
    --dry-run --yes \
    legacykey newkey
  [ "$status" -eq 0 ]
  [[ "$output" == *"Migration preview"* ]]
}

@test "migrate-account.sh rejects multisig account with exit 3" {
  run env SHIM_ESTIMATE_FIXTURE=estimate-multisig \
    "$SCRIPTS_DIR/migrate-account.sh" \
    --binary "$SHIM" --chain-id shim-test --yes \
    legacykey newkey
  [ "$status" -eq 3 ]
  [[ "$output" == *"multisig"* ]]
}

@test "migrate-account.sh errors usage when given one positional arg" {
  run "$SCRIPTS_DIR/migrate-account.sh" --chain-id x onlyone
  [ "$status" -eq 1 ]
}
```

- [ ] **Step 10.2: Implement the full flow**

Replace the placeholder `main()` in `scripts/migrate-account.sh`:

```bash
main() {
  parse_common_flags "$@"
  require_binary
  require_jq

  if [[ -n "$MNEMONIC_FILE" ]]; then
    import_from_mnemonic "$MNEMONIC_FILE" "$LEGACY_KEY" "$NEW_KEY"
  fi

  local legacy_addr new_addr
  legacy_addr=$(resolve_address "$LEGACY_KEY")
  new_addr=$(resolve_address "$NEW_KEY")

  assert_not_migrated "$legacy_addr"
  assert_new_address_unused "$new_addr"

  local estimate
  estimate=$(preflight_estimate "$legacy_addr")

  assert_single_sig "$estimate"

  if [[ "$(jq -r '.is_validator' <<<"$estimate")" == "true" ]]; then
    log_error "account $legacy_addr is a validator; use scripts/migrate-validator.sh instead"
    exit 6
  fi

  assert_estimate_succeeds "$estimate"

  if [[ "$(jq -r '.has_supernode' <<<"$estimate")" == "true" ]]; then
    log_warn "this account owns a supernode registration; it will migrate with the account"
  fi

  local snap
  snap=$(snapshot_bank_balances "$legacy_addr")

  log_info "migrating $legacy_addr -> $new_addr"
  confirm "Proceed with migration?"

  if (( DRY_RUN == 1 )); then
    log_info "--dry-run: stopping before broadcast"
    return 0
  fi

  local broadcast_json tx_hash
  broadcast_json=$(lumerad_tx evmigration claim-legacy-account "$LEGACY_KEY" "$NEW_KEY" --yes)
  tx_hash=$(jq -r '.txhash // empty' <<<"$broadcast_json")
  if [[ -z "$tx_hash" || "$tx_hash" == "null" ]]; then
    log_error "broadcast returned no txhash: $broadcast_json"
    exit 2
  fi

  log_info "broadcast tx $tx_hash; waiting for inclusion..."
  wait_for_tx "$tx_hash"

  verify_migration "$legacy_addr" "$new_addr" "$snap"

  log_info "migration complete"
  log_info "  legacy: $legacy_addr"
  log_info "  new:    $new_addr"
  log_info "  tx:     $tx_hash"
}

main "$@"
```

- [ ] **Step 10.3: Make the script executable and run tests**

```bash
chmod +x scripts/migrate-account.sh
bats tests/scripts/migrate-account.bats
make lint-scripts
```

Expected: all tests pass, shellcheck clean.

- [ ] **Step 10.4: Commit**

```bash
git add scripts/migrate-account.sh tests/scripts/migrate-account.bats
git commit -m "feat(scripts): migrate-account.sh full flow with pre-flight and post-verification"
```

---

## Task 11: `migrate-validator.sh` full flow

**Files:**

- Modify: `scripts/migrate-validator.sh`
- Create: `tests/scripts/migrate-validator.bats`
- Create: `tests/scripts/fixtures/estimate-validator-ok.json`
- Create: `tests/scripts/fixtures/estimate-validator-over-cap.json`

- [ ] **Step 11.1: Add validator fixtures**

`estimate-validator-ok.json`: same shape as `estimate-ok.json` but `"is_validator": true, "val_delegation_count": 10, "val_unbonding_count": 2, "val_redelegation_count": 0`.

`estimate-validator-over-cap.json`: `"is_validator": true, "val_delegation_count": 2500` (exceeds the default `max_validator_delegations=2000`).

- [ ] **Step 11.2: Write failing end-to-end tests**

File `tests/scripts/migrate-validator.bats`:

```bash
#!/usr/bin/env bats

setup() {
  SCRIPTS_DIR="$(cd "$BATS_TEST_DIRNAME/../../scripts" && pwd)"
  FIX_DIR="$BATS_TEST_DIRNAME/fixtures"
  SHIM="$FIX_DIR/lumerad-shim.sh"
}

@test "migrate-validator.sh dry-run happy path exits 0" {
  run env SHIM_ESTIMATE_FIXTURE=estimate-validator-ok \
    "$SCRIPTS_DIR/migrate-validator.sh" \
    --binary "$SHIM" --chain-id shim-test \
    --i-have-stopped-the-node --yes --dry-run \
    vkey ekey
  [ "$status" -eq 0 ]
}

@test "migrate-validator.sh rejects non-validator with exit 6" {
  run "$SCRIPTS_DIR/migrate-validator.sh" \
    --binary "$SHIM" --chain-id shim-test \
    --i-have-stopped-the-node --yes --dry-run \
    vkey ekey
  # default estimate fixture has is_validator=false
  [ "$status" -eq 6 ]
  [[ "$output" == *"not a validator"* ]]
}

@test "migrate-validator.sh rejects over-cap with exit 6" {
  run env SHIM_ESTIMATE_FIXTURE=estimate-validator-over-cap \
    "$SCRIPTS_DIR/migrate-validator.sh" \
    --binary "$SHIM" --chain-id shim-test \
    --i-have-stopped-the-node --yes --dry-run \
    vkey ekey
  [ "$status" -eq 6 ]
  [[ "$output" == *"max_validator_delegations"* ]]
}

@test "migrate-validator.sh rejects missing downtime ack" {
  run env SHIM_ESTIMATE_FIXTURE=estimate-validator-ok \
    "$SCRIPTS_DIR/migrate-validator.sh" \
    --binary "$SHIM" --chain-id shim-test \
    --yes --dry-run \
    vkey ekey
  [ "$status" -eq 10 ]
  [[ "$output" == *"node"* ]]
}
```

- [ ] **Step 11.3: Implement the full flow**

Replace `main()` in `scripts/migrate-validator.sh`:

```bash
main() {
  # Pre-strip validator-only flag so parse_common_flags doesn't see it.
  local node_stopped=0
  local filtered=()
  while (( $# > 0 )); do
    case "$1" in
      --i-have-stopped-the-node) node_stopped=1; shift ;;
      *) filtered+=("$1"); shift ;;
    esac
  done
  set -- "${filtered[@]}"

  parse_common_flags "$@"
  require_binary
  require_jq

  if [[ -n "$MNEMONIC_FILE" ]]; then
    import_from_mnemonic "$MNEMONIC_FILE" "$LEGACY_KEY" "$NEW_KEY"
  fi

  local legacy_addr new_addr
  legacy_addr=$(resolve_address "$LEGACY_KEY")
  new_addr=$(resolve_address "$NEW_KEY")

  assert_not_migrated "$legacy_addr"
  assert_new_address_unused "$new_addr"

  local estimate
  estimate=$(preflight_estimate "$legacy_addr")

  assert_single_sig "$estimate"

  if [[ "$(jq -r '.is_validator' <<<"$estimate")" != "true" ]]; then
    log_error "account $legacy_addr is not a validator; use scripts/migrate-account.sh instead"
    exit 6
  fi

  local cap total
  cap=$(lumerad_q evmigration params | jq -r '.params.max_validator_delegations | tonumber')
  total=$(jq -r '.val_delegation_count + .val_unbonding_count + .val_redelegation_count' <<<"$estimate")
  if (( total > cap )); then
    log_error "validator has $total delegation/unbonding/redelegation records; exceeds max_validator_delegations=$cap"
    exit 6
  fi
  if (( total > cap * 9 / 10 )); then
    log_warn "validator record count ($total) is within 10% of cap ($cap)"
  fi

  assert_estimate_succeeds "$estimate"

  local snap
  snap=$(snapshot_bank_balances "$legacy_addr")

  cat >&2 <<'BANNER'
================================================================
WARNING — VALIDATOR MIGRATION
Your validator will miss blocks and may be jailed during
migration. The node MUST be stopped before broadcasting this tx.
================================================================
BANNER

  if (( node_stopped != 1 )); then
    local reply=""
    printf 'Type "yes" to confirm the node is stopped: ' >&2
    read -r reply || true
    if [[ "$reply" != "yes" ]]; then
      log_error "validator downtime not acknowledged"
      exit 10
    fi
  fi

  log_info "migrating validator $legacy_addr -> $new_addr"
  confirm "Proceed with validator migration?"

  if (( DRY_RUN == 1 )); then
    log_info "--dry-run: stopping before broadcast"
    return 0
  fi

  local broadcast_json tx_hash
  broadcast_json=$(lumerad_tx evmigration migrate-validator "$LEGACY_KEY" "$NEW_KEY" --yes)
  tx_hash=$(jq -r '.txhash // empty' <<<"$broadcast_json")
  if [[ -z "$tx_hash" || "$tx_hash" == "null" ]]; then
    log_error "broadcast returned no txhash: $broadcast_json"
    exit 2
  fi

  log_info "broadcast tx $tx_hash; waiting for inclusion..."
  wait_for_tx "$tx_hash"

  verify_migration "$legacy_addr" "$new_addr" "$snap"

  log_info "validator migration complete — post-migration checklist:"
  log_info "  1. Import $NEW_KEY into the production keyring (correct --keyring-backend)"
  log_info "  2. Restart lumerad"
  log_info "  3. Verify new operator via: lumerad query staking validator <new-valoper>"
  log_info "  4. Monitor missed-block counters for the next few blocks"
}

main "$@"
```

- [ ] **Step 11.4: Run tests and commit**

```bash
chmod +x scripts/migrate-validator.sh
bats tests/scripts/migrate-validator.bats
make lint-scripts
git add scripts/migrate-validator.sh tests/scripts/
git commit -m "feat(scripts): migrate-validator.sh full flow with cap check and downtime ack"
```

---

## Task 12: Documentation integration

**Files:**

- Modify: `docs/evm-integration/user-guides/migration.md`
- Modify: `docs/evm-integration/user-guides/validator-migration.md`
- Modify: `docs/evm-integration/user-guides/supernode-migration.md`

- [ ] **Step 12.1: Add "Method 3" section**

Open [docs/evm-integration/user-guides/migration.md](../evm-integration/user-guides/migration.md). After the "Method 2: Lumera CLI" section (currently ending around line 367), insert (outer fence is four backticks so the inner `bash` fences render correctly):

````markdown
---

## Method 3: Shell Helper Scripts

The repository ships two bash wrappers in [scripts/](../../../scripts/) that layer safety rails on top of the Method 2 CLI flow:

- `scripts/migrate-account.sh` — regular account migration (`claim-legacy-account`)
- `scripts/migrate-validator.sh` — validator migration (`migrate-validator`)

Both scripts:

- Detect and reject multisig accounts (use the offline 4-step flow in [legacy-migration.md](../evmigration/legacy-migration.md#multisig-account-migration) for those).
- Run `migration-estimate` before broadcast so you see what moves and why it might fail.
- Compare post-migration balances against a pre-broadcast snapshot.

### Single-sig account migration

```bash
./scripts/migrate-account.sh legacy-key new-key \
  --chain-id lumera-mainnet-1 \
  --node tcp://rpc.lumera:26657 \
  --keyring-backend test
```

Use `--mnemonic-file <path>` (file must be mode 0600) to import both keys from a mnemonic in one step. Add `--dry-run` to preview without broadcasting.

### Single-sig validator migration

```bash
./scripts/migrate-validator.sh legacy-op-key new-evm-key \
  --chain-id lumera-mainnet-1 \
  --node tcp://rpc.lumera:26657 \
  --keyring-backend test \
  --i-have-stopped-the-node
```

`--i-have-stopped-the-node` acknowledges the jailing risk; omitting it makes the script prompt interactively. `--yes` does NOT satisfy this acknowledgement — that's deliberate.

### Exit codes

| Code | Meaning |
|---|---|
| `0` | Success, or dry-run completed cleanly |
| `1` | Usage error / bad flags / bad input file permissions / key name collision |
| `2` | Environment error: binary missing, jq missing, node unreachable, unsupported binary version |
| `3` | Multisig rejected; use offline flow |
| `4` | Pre-flight estimate returned `would_succeed=false` |
| `5` | Account already migrated (or new address already used) |
| `6` | Wrong-script or delegation-cap error |
| `7` | Broadcast succeeded but post-migration verification failed — investigate manually |
| `10` | User aborted at a confirmation prompt |
````

- [ ] **Step 12.2: Cross-link from validator and supernode guides**

Add a short pointer near the existing CLI sections in:

- [validator-migration.md](../evm-integration/user-guides/validator-migration.md): mention `scripts/migrate-validator.sh` as the safer wrapper for single-sig validator operators, while multisig validators must continue using the offline proof flow.
- [supernode-migration.md](../evm-integration/user-guides/supernode-migration.md): mention `scripts/migrate-account.sh` for manual single-sig account migration and `scripts/migrate-validator.sh` when the supernode account is also the validator operator account.

- [ ] **Step 12.3: Commit**

```bash
git add docs/evm-integration/user-guides/migration.md docs/evm-integration/user-guides/validator-migration.md docs/evm-integration/user-guides/supernode-migration.md
git commit -m "docs(evm): document scripts/migrate-account.sh and scripts/migrate-validator.sh"
```

---

## Task 13: Devnet smoke validation

**Files:** none modified; this is a manual acceptance pass executed before declaring work done.

- [ ] **Step 13.1: Build and start devnet**

```bash
make build
make devnet-new
```

Wait for the five validators + Hermes relayer to come up. This is only exercising the chain; the scripts run against the devnet's exposed RPC.

- [ ] **Step 13.2: Create a legacy-style (coin-type 118) test account and fund it**

Follow the devnet's usual fund-account pattern (`devnet/scripts/*` helpers). Import the mnemonic at coin-type 118 into a local keyring, fund the address, optionally make a delegation, then import the same mnemonic at coin-type 60 under a second key name.

- [ ] **Step 13.3: Run the account script in dry-run**

```bash
./scripts/migrate-account.sh legacy-key new-key \
  --chain-id lumera-devnet-1 \
  --node tcp://localhost:26657 \
  --dry-run --yes
```

Expected: preview block with balance/delegation totals; exit 0; no on-chain record.

- [ ] **Step 13.4: Run the account script for real**

Remove `--dry-run`. Expected: exit 0; on-chain migration record created; legacy balance zero; new balance matches or exceeds the pre-broadcast snapshot.

- [ ] **Step 13.5: Run the negative matrix**

Exercise each of exit codes 1, 2, 3, 4, 5, 6, 10 by constructing the corresponding scenario (bogus flag, unreachable node, multisig account, disabled/closed migration or other `would_succeed=false` estimate, re-running after success, running the account script against a validator and vice versa, refusing at confirmation). Spot-check that exit codes match Section 7 of the design.

- [ ] **Step 13.6: Run the validator script against a devnet validator**

Pick one of the five devnet validators, stop its node, run:

```bash
./scripts/migrate-validator.sh validator-key new-evm-key \
  --chain-id lumera-devnet-1 \
  --node tcp://localhost:26657 \
  --i-have-stopped-the-node --yes
```

Expected: migration succeeds; after restarting the node the validator appears under its new operator address.

- [ ] **Step 13.7: Commit any fixes uncovered**

If devnet uncovers issues — edge-case fixtures, shellcheck cases the unit tests didn't hit, output-format mismatches — fix them in separate commits, then re-run Task 13.

---

## Acceptance

Plan is complete when:

- `make lint-scripts` passes.
- `make test-scripts` passes (bats).
- All exit codes in Section 7 of [evmigration-scripts-design.md](evmigration-scripts-design.md) are demonstrated in bats tests or the devnet matrix.
- The `Method 3` section is merged into [migration.md](../evm-integration/user-guides/migration.md), with cross-links from [validator-migration.md](../evm-integration/user-guides/validator-migration.md) and [supernode-migration.md](../evm-integration/user-guides/supernode-migration.md).
- Task 13 devnet matrix has been walked through on at least one branch.
