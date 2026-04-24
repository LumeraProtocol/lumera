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
  "$BIN" query "$@" --node "$NODE" "${_KRF[@]}" --output json
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
    log_error "legacy account is a ${k}-of-${n} multisig; use scripts/migrate-multisig.sh instead"
    log_error "see docs/evm-integration/user-guides/migration-scripts.md#multisig-migration"
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
  if ! json=$(lumerad_q evmigration "$subcmd" "$addr" 2>/dev/null); then
    log_error "could not query evmigration $subcmd for $addr"
    exit 2
  fi
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

# ---- Multisig helpers -------------------------------------------------------

# assert_multisig <estimate-json>
# Opposite of assert_single_sig. Exit 3 if the estimate's is_multisig != true.
assert_multisig() {
  local json="$1"
  if [[ "$(jq -r '.is_multisig' <<<"$json")" != "true" ]]; then
    log_error "legacy account is not a multisig; use migrate-account.sh / migrate-validator.sh for single-sig accounts"
    exit 3
  fi
}

# require_multisig_binary
# Extends require_binary by probing all four multisig tx subcommands.
require_multisig_binary() {
  require_binary
  local probe
  for probe in \
    "tx evmigration generate-proof-payload" \
    "tx evmigration sign-proof" \
    "tx evmigration combine-proof" \
    "tx evmigration submit-proof"; do
    # shellcheck disable=SC2086
    if ! "$BIN" $probe --help >/dev/null 2>&1; then
      log_error "$BIN does not support '$probe' — needs a post-EVM-upgrade build with multisig migration"
      exit 2
    fi
  done
}

# auth_account_json <addr>
# Thin wrapper around lumerad_q auth account, fails closed with exit 2 on RPC failure.
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
# Returns one of: none | single-sig | multisig | unknown.
# Searches both top-level and nested base-account shapes.
auth_pubkey_type() {
  local addr="$1"
  local json pk_type
  json=$(auth_account_json "$addr")
  # Try several known locations in priority order.
  pk_type=$(jq -r '
    [
      .account.pub_key,
      .account.base_account.pub_key,
      .account.base_vesting_account.base_account.pub_key
    ]
    | map(select(. != null))
    | (.[0] // null)
    | if . == null then "null" else (."@type" // "unknown") end
  ' <<<"$json")
  case "$pk_type" in
    null)                                                 printf 'none\n' ;;
    /cosmos.crypto.multisig.LegacyAminoPubKey)            printf 'multisig\n' ;;
    /cosmos.crypto.secp256k1.PubKey|\
    /cosmos.crypto.ethsecp256k1.PubKey|\
    /ethermint.crypto.v1.ethsecp256k1.PubKey|\
    /cosmos.evm.crypto.v1.ethsecp256k1.PubKey)            printf 'single-sig\n' ;;
    *)                                                    printf 'unknown\n' ;;
  esac
}

# key_pubkey_b64 <key-name>
# Reads lumerad keys show <key-name> --output json and extracts base64 pubkey bytes.
key_pubkey_b64() {
  local key_name="$1"
  local info pk_inner pk_key
  if ! info=$(lumerad_keys show "$key_name" --output json 2>/dev/null); then
    log_error "key not found in keyring: $key_name"
    exit 1
  fi
  # Some SDK versions return .pubkey as a stringified JSON blob; others return
  # it as an object. Accept both shapes.
  pk_inner=$(jq -c '.pubkey | if type == "string" then fromjson else . end' <<<"$info" 2>/dev/null || printf '')
  if [[ -z "$pk_inner" || "$pk_inner" == "null" ]]; then
    log_error "key '$key_name' has no pubkey field"
    exit 1
  fi
  pk_key=$(jq -r '.key // empty' <<<"$pk_inner")
  if [[ -z "$pk_key" ]]; then
    log_error "key '$key_name' has no pubkey key bytes"
    exit 1
  fi
  printf '%s\n' "$pk_key"
}

# assert_secp256k1_key <key-name>
# Confirms the key is a legacy Cosmos /cosmos.crypto.secp256k1.PubKey.
assert_secp256k1_key() {
  local key_name="$1"
  local info pk_type
  if ! info=$(lumerad_keys show "$key_name" --output json 2>/dev/null); then
    log_error "key not found in keyring: $key_name"
    exit 1
  fi
  pk_type=$(jq -r '(.pubkey | if type == "string" then fromjson else . end | ."@type") // "unknown"' <<<"$info" 2>/dev/null || printf 'unknown')
  if [[ "$pk_type" != "/cosmos.crypto.secp256k1.PubKey" ]]; then
    log_error "key '$key_name' is not secp256k1 (got $pk_type) — legacy migration requires a coin-type 118 secp256k1 key"
    exit 1
  fi
}

# assert_eth_key <key-name>
# For submit. Confirms the key is an eth_secp256k1 variant.
assert_eth_key() {
  local key_name="$1"
  local info pk_type
  if ! info=$(lumerad_keys show "$key_name" --output json 2>/dev/null); then
    log_error "key not found in keyring: $key_name"
    exit 1
  fi
  pk_type=$(jq -r '(.pubkey | if type == "string" then fromjson else . end | ."@type") // "unknown"' <<<"$info" 2>/dev/null || printf 'unknown')
  case "$pk_type" in
    /cosmos.crypto.ethsecp256k1.PubKey|\
    /ethermint.crypto.v1.ethsecp256k1.PubKey|\
    /cosmos.evm.crypto.v1.ethsecp256k1.PubKey) ;;
    *)
      log_error "key '$key_name' is not eth_secp256k1 (got $pk_type) — submit requires the new EVM destination key"
      exit 1 ;;
  esac
}

_payload_hex() {
  printf '%s' "$1" | od -An -tx1 -v | tr -d ' \n'
}

# read_proof_file <path>
# Reads a v2 PartialProof JSON file, validates structure and integrity.
# v2 shape: sibling .legacy and .new SideSpecs, each single-key (pub_key) XOR
# multisig (threshold + sub_pub_keys); partials split into per-side slices
# .partial_legacy_signatures and .partial_new_signatures.
# Emits validated JSON on stdout; compact summary on stderr.
# Exit 9 on structural/integrity violations; exit 3 on single-key-on-either-side
# (this wrapper is multisig→multisig only).
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

  # Version gate — only v2 is supported.
  local version
  version=$(jq -r '.version // empty' <<<"$json")
  if [[ "$version" != "2" ]]; then
    log_error "proof file $path has unsupported version '$version' (expected 2)"
    exit 9
  fi

  # Required top-level fields
  local required=(
    ".kind" ".legacy_address" ".new_address"
    ".chain_id" ".evm_chain_id" ".payload_hex"
    ".legacy" ".new"
    ".legacy.sig_format" ".new.sig_format"
    ".partial_legacy_signatures" ".partial_new_signatures"
  )
  local field
  for field in "${required[@]}"; do
    if [[ "$(jq -r "$field // \"__missing__\"" <<<"$json")" == "__missing__" ]]; then
      log_error "missing required field in $path: $field"
      exit 9
    fi
  done

  # Reject single-key on either side — wrapper is multisig→multisig only.
  # In v2, multisig sides set both threshold and sub_pub_keys; single-key sides
  # set pub_key instead (and omit threshold/sub_pub_keys via omitempty).
  local legacy_is_multi new_is_multi
  legacy_is_multi=$(jq -r '(.legacy | has("threshold") and has("sub_pub_keys"))' <<<"$json")
  new_is_multi=$(jq -r '(.new | has("threshold") and has("sub_pub_keys"))' <<<"$json")
  if [[ "$legacy_is_multi" != "true" || "$new_is_multi" != "true" ]]; then
    log_error "proof file $path is not a multisig→multisig proof; use migrate-account.sh or migrate-validator.sh for single-key migrations"
    exit 3
  fi

  # Per-side threshold bounds and per-side partial index/signature checks.
  local s
  for s in legacy new; do
    local t n
    t=$(jq -r ".$s.threshold" <<<"$json")
    n=$(jq -r ".$s.sub_pub_keys | length" <<<"$json")
    if ! [[ "$t" =~ ^[0-9]+$ ]] || (( t < 1 || t > n )); then
      log_error "invalid multisig structure in $path on $s side: threshold=$t sub_keys=$n"
      exit 9
    fi

    local idx sig
    while IFS=$'\t' read -r idx sig; do
      [[ -z "$idx" ]] && continue
      if ! [[ "$idx" =~ ^[0-9]+$ ]] || (( idx < 0 || idx >= n )); then
        log_error "partial_${s}_signatures index $idx out of range [0,$n) in $path"
        exit 9
      fi
      if [[ -z "$sig" || "$sig" == "null" ]]; then
        log_error "partial_${s}_signatures entry with index $idx has empty signature in $path"
        exit 9
      fi
    done < <(jq -r ".partial_${s}_signatures[]? | [.index, .signature] | @tsv" <<<"$json")
  done

  # Canonical payload_hex reconstruction
  local chain_id_f evm_chain_id kind_f legacy_f new_f payload calc got
  chain_id_f=$(jq -r '.chain_id' <<<"$json")
  evm_chain_id=$(jq -r '.evm_chain_id' <<<"$json")
  kind_f=$(jq -r '.kind' <<<"$json")
  legacy_f=$(jq -r '.legacy_address' <<<"$json")
  new_f=$(jq -r '.new_address' <<<"$json")
  payload="lumera-evm-migration:${chain_id_f}:${evm_chain_id}:${kind_f}:${legacy_f}:${new_f}"
  calc=$(_payload_hex "$payload")
  got=$(jq -r '.payload_hex' <<<"$json")
  if [[ "$calc" != "$got" ]]; then
    log_error "payload_hex mismatch in $path (expected $calc, got $got)"
    exit 9
  fi

  printf '%s\n' "$json"
}

# read_migration_tx_file <path>
# Reads unsigned tx JSON produced by combine-proof. Verifies exactly one
# supported evmigration message with both legacy_proof.multisig and
# new_proof.multisig set (wrapper is multisig→multisig only).
# Rejects single-key proof txs on either side with exit 3.
# Emits compact JSON on stdout:
#   {legacy_address, new_address, kind,
#    threshold, num_signers,                    # legacy side
#    new_threshold, new_num_signers}            # new side
read_migration_tx_file() {
  local path="$1"
  if [[ ! -f "$path" ]]; then
    log_error "tx file not found: $path"
    exit 9
  fi
  local json
  if ! json=$(jq -e . "$path" 2>/dev/null); then
    log_error "tx file is not valid JSON: $path"
    exit 9
  fi

  local msg_count
  msg_count=$(jq -r '.body.messages | length' <<<"$json")
  if [[ "$msg_count" != "1" ]]; then
    log_error "expected exactly 1 message in $path, got $msg_count"
    exit 9
  fi

  local msg_type legacy new_addr kind
  msg_type=$(jq -r '.body.messages[0]."@type"' <<<"$json")
  legacy=$(jq -r '.body.messages[0].legacy_address' <<<"$json")
  new_addr=$(jq -r '.body.messages[0].new_address' <<<"$json")
  case "$msg_type" in
    /lumera.evmigration.MsgClaimLegacyAccount) kind="claim" ;;
    /lumera.evmigration.MsgMigrateValidator)   kind="validator" ;;
    *) log_error "unrecognized message type in $path: $msg_type"; exit 9 ;;
  esac

  # Both proofs must be multisig for this wrapper.
  local legacy_is_multi new_is_multi
  legacy_is_multi=$(jq -r '.body.messages[0].legacy_proof | has("multisig")' <<<"$json")
  new_is_multi=$(jq -r '.body.messages[0].new_proof | has("multisig")' <<<"$json")
  if [[ "$legacy_is_multi" != "true" || "$new_is_multi" != "true" ]]; then
    log_error "tx file $path is not a multisig→multisig migration; use migrate-account.sh / migrate-validator.sh for single-key migrations"
    exit 3
  fi

  # Field name is sub_pub_keys (no _b64 suffix) — it is the proto-JSON
  # rendering of MultisigProof.sub_pub_keys (repeated bytes; each entry is
  # base64 in JSON).
  local threshold num_signers new_threshold new_num_signers
  threshold=$(jq -r '.body.messages[0].legacy_proof.multisig.threshold' <<<"$json")
  num_signers=$(jq -r '.body.messages[0].legacy_proof.multisig.sub_pub_keys | length' <<<"$json")
  new_threshold=$(jq -r '.body.messages[0].new_proof.multisig.threshold' <<<"$json")
  new_num_signers=$(jq -r '.body.messages[0].new_proof.multisig.sub_pub_keys | length' <<<"$json")
  local v
  for v in threshold num_signers new_threshold new_num_signers; do
    if [[ -z "${!v}" || "${!v}" == "null" ]]; then
      log_error "tx file $path has incomplete multisig proof fields ($v missing)"
      exit 9
    fi
  done

  jq -nc \
    --arg legacy "$legacy" \
    --arg new "$new_addr" \
    --arg kind "$kind" \
    --argjson threshold "$threshold" \
    --argjson num_signers "$num_signers" \
    --argjson new_threshold "$new_threshold" \
    --argjson new_num_signers "$new_num_signers" \
    '{legacy_address:$legacy, new_address:$new, kind:$kind,
      threshold:$threshold, num_signers:$num_signers,
      new_threshold:$new_threshold, new_num_signers:$new_num_signers}'
}

# summarize_partials <files...>
# Reads each partial via read_proof_file, enforces cross-file consistency on
# BOTH sides (legacy + new), and prints per-side K-of-N entry-presence
# matrices plus the cross-side matching-index count to stderr. Returns 0 iff
# the count of indices signed on BOTH sides meets threshold — per-side
# thresholds alone are insufficient, because the consensus mirror-source rule
# requires legacy_proof.signer_indices == new_proof.signer_indices. The
# shape-mirror rule implies legacy K==new K and legacy
# N==new N, but we check per side regardless to surface the actual gap.
summarize_partials() {
  local files=("$@")
  if (( ${#files[@]} == 0 )); then
    log_error "summarize_partials: no partial files given"
    exit 1
  fi

  local first_json
  first_json=$(read_proof_file "${files[0]}")
  local first_chain first_evm first_legacy first_new first_payload first_kind
  first_chain=$(jq -r '.chain_id' <<<"$first_json")
  first_evm=$(jq -r '.evm_chain_id' <<<"$first_json")
  first_legacy=$(jq -r '.legacy_address' <<<"$first_json")
  first_new=$(jq -r '.new_address' <<<"$first_json")
  first_payload=$(jq -r '.payload_hex' <<<"$first_json")
  first_kind=$(jq -r '.kind' <<<"$first_json")

  local first_leg_threshold first_leg_subkeys first_leg_sigfmt first_leg_subcount
  local first_new_threshold first_new_subkeys first_new_sigfmt first_new_subcount
  first_leg_threshold=$(jq -r '.legacy.threshold' <<<"$first_json")
  first_leg_subkeys=$(jq -c '.legacy.sub_pub_keys' <<<"$first_json")
  first_leg_sigfmt=$(jq -r '.legacy.sig_format' <<<"$first_json")
  first_leg_subcount=$(jq -r '.legacy.sub_pub_keys | length' <<<"$first_json")
  first_new_threshold=$(jq -r '.new.threshold' <<<"$first_json")
  first_new_subkeys=$(jq -c '.new.sub_pub_keys' <<<"$first_json")
  first_new_sigfmt=$(jq -r '.new.sig_format' <<<"$first_json")
  first_new_subcount=$(jq -r '.new.sub_pub_keys | length' <<<"$first_json")

  local -A legacy_index_to_file=()
  local -A new_index_to_file=()
  local idx
  while read -r idx; do
    [[ -z "$idx" ]] && continue
    legacy_index_to_file[$idx]="${files[0]}"
  done < <(jq -r '.partial_legacy_signatures[].index' <<<"$first_json")
  while read -r idx; do
    [[ -z "$idx" ]] && continue
    new_index_to_file[$idx]="${files[0]}"
  done < <(jq -r '.partial_new_signatures[].index' <<<"$first_json")

  local f
  for f in "${files[@]:1}"; do
    local j
    j=$(read_proof_file "$f")
    local checks=(
      "chain_id:$first_chain:.chain_id"
      "evm_chain_id:$first_evm:.evm_chain_id"
      "legacy_address:$first_legacy:.legacy_address"
      "new_address:$first_new:.new_address"
      "payload_hex:$first_payload:.payload_hex"
      "kind:$first_kind:.kind"
      "legacy.threshold:$first_leg_threshold:.legacy.threshold"
      "legacy.sig_format:$first_leg_sigfmt:.legacy.sig_format"
      "new.threshold:$first_new_threshold:.new.threshold"
      "new.sig_format:$first_new_sigfmt:.new.sig_format"
    )
    local entry
    for entry in "${checks[@]}"; do
      local name="${entry%%:*}"
      local rest="${entry#*:}"
      local expected="${rest%:*}"
      local jq_path="${rest##*:}"
      local got
      got=$(jq -r "$jq_path" <<<"$j")
      if [[ "$got" != "$expected" ]]; then
        log_error "partial $f disagrees on $name (expected $expected, got $got)"
        exit 9
      fi
    done
    local j_leg_subkeys j_new_subkeys
    j_leg_subkeys=$(jq -c '.legacy.sub_pub_keys' <<<"$j")
    if [[ "$j_leg_subkeys" != "$first_leg_subkeys" ]]; then
      log_error "partial $f disagrees on legacy.sub_pub_keys"
      exit 9
    fi
    j_new_subkeys=$(jq -c '.new.sub_pub_keys' <<<"$j")
    if [[ "$j_new_subkeys" != "$first_new_subkeys" ]]; then
      log_error "partial $f disagrees on new.sub_pub_keys"
      exit 9
    fi
    while read -r idx; do
      [[ -z "$idx" ]] && continue
      legacy_index_to_file[$idx]="$f"
    done < <(jq -r '.partial_legacy_signatures[].index' <<<"$j")
    while read -r idx; do
      [[ -z "$idx" ]] && continue
      new_index_to_file[$idx]="$f"
    done < <(jq -r '.partial_new_signatures[].index' <<<"$j")
  done

  local leg_present=${#legacy_index_to_file[@]}
  local new_present=${#new_index_to_file[@]}
  {
    printf 'Legacy-side partials (%s-of-%s required):\n' "$first_leg_threshold" "$first_leg_subcount"
    local i
    for (( i=0; i<first_leg_subcount; i++ )); do
      if [[ -n "${legacy_index_to_file[$i]:-}" ]]; then
        printf '  [X] signer %s  %s\n' "$i" "${legacy_index_to_file[$i]}"
      else
        printf '  [ ] signer %s  (missing)\n' "$i"
      fi
    done
    if (( leg_present >= first_leg_threshold )); then
      printf 'Legacy threshold satisfied: yes (%s >= %s)\n' "$leg_present" "$first_leg_threshold"
    else
      printf 'Legacy threshold satisfied: no (%s < %s)\n' "$leg_present" "$first_leg_threshold"
    fi

    printf 'New-side partials (%s-of-%s required):\n' "$first_new_threshold" "$first_new_subcount"
    for (( i=0; i<first_new_subcount; i++ )); do
      if [[ -n "${new_index_to_file[$i]:-}" ]]; then
        printf '  [X] signer %s  %s\n' "$i" "${new_index_to_file[$i]}"
      else
        printf '  [ ] signer %s  (missing)\n' "$i"
      fi
    done
    if (( new_present >= first_new_threshold )); then
      printf 'New threshold satisfied: yes (%s >= %s)\n' "$new_present" "$first_new_threshold"
    else
      printf 'New threshold satisfied: no (%s < %s)\n' "$new_present" "$first_new_threshold"
    fi

    # Shared-index count is what actually drives combine-proof's
    # intersection: legacy_proof.signer_indices must equal new_proof.
    # signer_indices at consensus (mirror-source rule), so only indices
    # signed on BOTH sides count toward the real quorum. Per-side
    # thresholds can BOTH say "yes" and this still be short.
    local shared_count=0
    local i
    for (( i=0; i<first_leg_subcount; i++ )); do
      if [[ -n "${legacy_index_to_file[$i]:-}" && -n "${new_index_to_file[$i]:-}" ]]; then
        shared_count=$(( shared_count + 1 ))
      fi
    done
    if (( shared_count >= first_leg_threshold )); then
      printf 'Matching-index threshold satisfied: yes (%s >= %s)\n' "$shared_count" "$first_leg_threshold"
    else
      printf 'Matching-index threshold satisfied: no (%s < %s) — one-sided partials do not count\n' "$shared_count" "$first_leg_threshold"
    fi
  } >&2

  # Gate return on the shared-index count, NOT per-side thresholds. This
  # mirrors what `lumerad combine-proof` enforces.
  local shared_gate=0
  local j
  for (( j=0; j<first_leg_subcount; j++ )); do
    if [[ -n "${legacy_index_to_file[$j]:-}" && -n "${new_index_to_file[$j]:-}" ]]; then
      shared_gate=$(( shared_gate + 1 ))
    fi
  done
  (( shared_gate >= first_leg_threshold ))
}
