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

# Globals populated by parse_common_flags. Declared here so sourcing scripts
# can reference them. These are intentionally unused in this file.
# shellcheck disable=SC2034
NODE=""
# shellcheck disable=SC2034
CHAIN_ID=""
# shellcheck disable=SC2034
KEYRING_BACKEND="test"
# shellcheck disable=SC2034
KEYRING_DIR=""
# shellcheck disable=SC2034
HOME_DIR=""
# shellcheck disable=SC2034
MNEMONIC_FILE=""
# shellcheck disable=SC2034
YES=0
# shellcheck disable=SC2034
DRY_RUN=0
# shellcheck disable=SC2034
BIN="lumerad"
# shellcheck disable=SC2034
LEGACY_KEY=""
# shellcheck disable=SC2034
NEW_KEY=""

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

# ---- Flag parsing -----------------------------------------------------------

# _require_value <flag-name> <arg-count>
# Aborts with a guided error if the value side of a two-arg flag is missing.
_require_value() {
  if (( $2 < 2 )); then
    log_error "$1 requires a value"
    _usage
    exit 1
  fi
}

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
  # shellcheck disable=SC2034  # globals consumed by entry scripts
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

  local positional=()
  # shellcheck disable=SC2034  # globals consumed by entry scripts
  while (( $# > 0 )); do
    case "$1" in
      --node)            _require_value "$1" "$#"; NODE="$2"; shift 2 ;;
      --chain-id)        _require_value "$1" "$#"; CHAIN_ID="$2"; shift 2 ;;
      --keyring-backend) _require_value "$1" "$#"; KEYRING_BACKEND="$2"; shift 2 ;;
      --keyring-dir)     _require_value "$1" "$#"; KEYRING_DIR="$2"; shift 2 ;;
      --home)            _require_value "$1" "$#"; HOME_DIR="$2"; shift 2 ;;
      --mnemonic-file)   _require_value "$1" "$#"; MNEMONIC_FILE="$2"; shift 2 ;;
      --yes|-y)          YES=1; shift ;;
      --dry-run)         DRY_RUN=1; shift ;;
      --binary)          _require_value "$1" "$#"; BIN="$2"; shift 2 ;;
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
  # shellcheck disable=SC2034
  LEGACY_KEY="${positional[0]}"
  # shellcheck disable=SC2034
  NEW_KEY="${positional[1]}"
}

# ---- Environment requirements -----------------------------------------------

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
}

# ---- lumerad wrappers -------------------------------------------------------

# Returns (via stdout, one per line) the keyring flags derived from globals.
_keyring_flags() {
  local flags=(--keyring-backend "$KEYRING_BACKEND")
  [[ -n "${KEYRING_DIR:-}" ]] && flags+=(--keyring-dir "$KEYRING_DIR")
  [[ -n "${HOME_DIR:-}"    ]] && flags+=(--home "$HOME_DIR")
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
  if [[ -z "${CHAIN_ID:-}" ]]; then
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
    printf '  Authz:          %s\n' "$authz"
    printf '  Feegrants:      %s\n' "$feegrants"
    printf '  Supernode:      %s\n' "$supernode"
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
