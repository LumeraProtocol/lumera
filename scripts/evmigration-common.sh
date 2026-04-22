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

# _require_value <flag-name> <arg-count> <next-arg-or-empty>
# Aborts if the next argument is missing or looks like another flag.
_require_value() {
  if (( $2 < 2 )); then
    log_error "$1 requires a value"
    _usage
    exit 1
  fi
  if [[ "${3-}" == --* ]]; then
    log_error "$1 requires a value (got flag-shaped arg: $3)"
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
      --node)            _require_value "$1" "$#" "${2-}"; NODE="$2"; shift 2 ;;
      --chain-id)        _require_value "$1" "$#" "${2-}"; CHAIN_ID="$2"; shift 2 ;;
      --keyring-backend) _require_value "$1" "$#" "${2-}"; KEYRING_BACKEND="$2"; shift 2 ;;
      --keyring-dir)     _require_value "$1" "$#" "${2-}"; KEYRING_DIR="$2"; shift 2 ;;
      --home)            _require_value "$1" "$#" "${2-}"; HOME_DIR="$2"; shift 2 ;;
      --mnemonic-file)   _require_value "$1" "$#" "${2-}"; MNEMONIC_FILE="$2"; shift 2 ;;
      --yes|-y)          YES=1; shift ;;
      --dry-run)         DRY_RUN=1; shift ;;
      --binary)          _require_value "$1" "$#" "${2-}"; BIN="$2"; shift 2 ;;
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
  local probe
  for probe in \
    "query evmigration" \
    "tx evmigration claim-legacy-account" \
    "tx evmigration migrate-validator"; do
    # shellcheck disable=SC2086  # intentional word-splitting of probe
    if ! "$BIN" $probe --help >/dev/null 2>&1; then
      log_error "$BIN does not support '$probe' — needs a post-EVM-upgrade build"
      exit 2
    fi
  done
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
  local actions is_validator is_multisig threshold num_signers val_dels val_unb val_red
  balance=$(jq -r '.balance_summary' <<<"$json")
  delegations=$(jq -r '.delegation_count' <<<"$json")
  unbonding=$(jq -r '.unbonding_count' <<<"$json")
  redelegations=$(jq -r '.redelegation_count' <<<"$json")
  authz=$(jq -r '.authz_grant_count' <<<"$json")
  feegrants=$(jq -r '.feegrant_count' <<<"$json")
  supernode=$(jq -r 'if .has_supernode then "yes" else "no" end' <<<"$json")
  would=$(jq -r 'if .would_succeed then "yes" else "no" end' <<<"$json")
  actions=$(jq -r '.action_count' <<<"$json")
  is_validator=$(jq -r 'if .is_validator then "yes" else "no" end' <<<"$json")
  is_multisig=$(jq -r 'if .is_multisig then "yes" else "no" end' <<<"$json")
  threshold=$(jq -r '.threshold' <<<"$json")
  num_signers=$(jq -r '.num_signers' <<<"$json")
  val_dels=$(jq -r '.val_delegation_count' <<<"$json")
  val_unb=$(jq -r '.val_unbonding_count' <<<"$json")
  val_red=$(jq -r '.val_redelegation_count' <<<"$json")

  {
    printf 'Migration preview for %s:\n' "$addr"
    printf '  Validator:         %s\n' "$is_validator"
    if [[ "$is_validator" == "yes" ]]; then
      printf '  Val delegations:   %s (to validator)\n' "$val_dels"
      printf '  Val unbondings:    %s (to validator)\n' "$val_unb"
      printf '  Val redelegations: %s (src or dst)\n' "$val_red"
    fi
    printf '  Multisig:          %s' "$is_multisig"
    if [[ "$is_multisig" == "yes" ]]; then
      printf ' (%s-of-%s)' "$threshold" "$num_signers"
    fi
    printf '\n'
    printf '  Balance:           %s\n' "$balance"
    printf '  Delegations:       %s\n' "$delegations"
    printf '  Unbonding:         %s\n' "$unbonding"
    printf '  Redelegations:     %s\n' "$redelegations"
    printf '  Authz grants:      %s\n' "$authz"
    printf '  Feegrants:         %s\n' "$feegrants"
    printf '  Actions:           %s\n' "$actions"
    printf '  Supernode:         %s\n' "$supernode"
    printf '  Would succeed:     %s\n' "$would"
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

# ---- Bank snapshot, tx polling, verification --------------------------------

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
  local rec_json rec_new
  if ! rec_json=$(lumerad_q evmigration migration-record "$legacy" 2>/dev/null); then
    log_error "post-check: could not query migration-record for $legacy — verify manually"
    exit 7
  fi
  rec_new=$(jq -r '.record.new_address // empty' <<<"$rec_json")
  if [[ "$rec_new" != "$new" ]]; then
    log_error "post-check: migration record for $legacy does not point to $new (got: '$rec_new')"
    exit 7
  fi

  # 2. Legacy balances must be all zero (account removed or empty).
  local legacy_after
  if ! legacy_after=$(lumerad_q bank balances "$legacy" 2>/dev/null); then
    log_error "post-check: could not query legacy bank balances for $legacy — verify manually"
    exit 7
  fi
  if [[ "$(jq -r '[.balances[].amount | tonumber] | add // 0' <<<"$legacy_after")" != "0" ]]; then
    log_error "post-check: legacy address $legacy still has non-zero balance"
    exit 7
  fi

  # 3. For every {denom,amount} in snap_json, new balances must be >= amount.
  local new_after
  if ! new_after=$(lumerad_q bank balances "$new" 2>/dev/null); then
    log_error "post-check: could not query new bank balances for $new — verify manually"
    exit 7
  fi
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

# ---- Confirmation -----------------------------------------------------------

# confirm <prompt>
# Returns 0 on user confirmation or when YES=1 is set; exits 10 on refusal.
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
  # shellcheck disable=SC2154  # rc is captured at trap runtime
  trap 'rc=$?; cleanup_mnemonic_keys; exit "$rc"' EXIT

  printf '%s\n' "$mnemonic" | lumerad_keys add "$legacy_name" \
    --recover --coin-type 118 --algo secp256k1
  printf '%s\n' "$mnemonic" | lumerad_keys add "$new_name" \
    --recover --coin-type 60 --algo eth_secp256k1

  unset mnemonic
}
