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
# Preserve any pre-existing CHAIN_ID — it may have been set by:
#   • a parent shell that exported CHAIN_ID
#   • a docker container ENV (e.g. devnet's "ENV CHAIN_ID=lumera-devnet-1")
#   • a wrapper script that sourced this file after assigning CHAIN_ID
# The default-resolution order in parse_common_flags is:
#   --chain-id flag  >  $LUMERA_CHAIN_ID env  >  preset $CHAIN_ID  >  error
CHAIN_ID="${CHAIN_ID:-}"
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
  local supports_color=0 colors=""
  if [[ -t 2 && -z "${NO_COLOR:-}" && -n "${TERM:-}" && "${TERM:-}" != "dumb" ]]; then
    supports_color=1
    if command -v tput >/dev/null 2>&1; then
      colors=$(tput colors 2>/dev/null || true)
      if [[ -n "$colors" && "$colors" -lt 8 ]]; then
        supports_color=0
      fi
    fi
  fi

  if (( supports_color == 1 )); then
    _C_INFO=$'\033[36m'   # cyan
    _C_WARN=$'\033[33m'   # yellow
    _C_ERR=$'\033[31m'    # red
    _C_LEGACY=$'\033[34m' # blue
    _C_NEW=$'\033[32m'    # green
    _C_RESET=$'\033[0m'
  else
    _C_INFO="" _C_WARN="" _C_ERR="" _C_LEGACY="" _C_NEW="" _C_RESET=""
  fi
}
_color_init

log_info()  { printf '%sINFO%s  %s\n' "$_C_INFO" "$_C_RESET" "$*" >&2; }
log_warn()  { printf '%sWARN%s  %s\n' "$_C_WARN" "$_C_RESET" "$*" >&2; }
log_error() { printf '%sERROR%s %s\n' "$_C_ERR"  "$_C_RESET" "$*" >&2; }

_role_color() { printf '%s%s%s' "$1" "$2" "$_C_RESET"; }
legacy_value() { _role_color "$_C_LEGACY" "$1"; }
new_value() { _role_color "$_C_NEW" "$1"; }

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

# Entry scripts may set these before parse_common_flags to customize --help:
#   _USAGE_DESCRIPTION   one short paragraph explaining what the script does
#   _USAGE_EXAMPLES      indented example invocations shown under "Examples:"
#   _USAGE_EXTRA_FLAGS   script-specific flag block shown after the common flags
# shellcheck disable=SC2034
_USAGE_DESCRIPTION="${_USAGE_DESCRIPTION:-}"
# shellcheck disable=SC2034
_USAGE_EXAMPLES="${_USAGE_EXAMPLES:-}"
# shellcheck disable=SC2034
_USAGE_EXTRA_FLAGS="${_USAGE_EXTRA_FLAGS:-}"

# shellcheck disable=SC2120  # $1 is optional; defaults to the entry script's basename.
_usage() {
  # Default to the entry script's basename ($0 in a sourced helper resolves to
  # the script the user invoked, not to this library). Callers may override
  # explicitly, e.g. _usage migrate-validator.sh, for synthetic contexts.
  local script_name="${1:-$(basename "$0")}"
  # Compute the actual default that --chain-id will resolve to so the help
  # message tells the user what they'd get without passing the flag, instead
  # of pointing at an env var name they may or may not have set.
  local _resolved_chain_id="${LUMERA_CHAIN_ID:-${CHAIN_ID:-}}"
  local _chain_id_help
  if [[ -n "${_resolved_chain_id}" ]]; then
    _chain_id_help="default ${_resolved_chain_id}"
  else
    _chain_id_help="required, or set \$LUMERA_CHAIN_ID / \$CHAIN_ID"
  fi
  cat >&2 <<USAGE
Usage: $script_name <legacy-key> <new-key> [flags]
USAGE
  if [[ -n "${_USAGE_DESCRIPTION:-}" ]]; then
    printf '\n%s\n' "$_USAGE_DESCRIPTION" >&2
  fi
  cat >&2 <<USAGE

Flags:
  --node <url>              RPC endpoint (default \$LUMERA_NODE or tcp://localhost:26657)
                            Mainnet RPC example: https://rpc.lumera.io:443
  --chain-id <id>           Chain ID (${_chain_id_help})
  --keyring-backend <b>     test|file|os (default test)
  --keyring-dir <dir>       Keyring directory (overrides --home for keys)
  --home <dir>              lumerad home directory
  --mnemonic-file <path>    Import both keys from a mnemonic file (mode 0600 or stricter)
  --yes, -y                 Skip standard confirmation prompts
  --dry-run                 Run pre-flight only; do not broadcast
  --binary <path>           Override lumerad binary (default: lumerad on PATH)
USAGE
  if [[ -n "${_USAGE_EXTRA_FLAGS:-}" ]]; then
    printf '%s\n' "$_USAGE_EXTRA_FLAGS" >&2
  fi
  if [[ -n "${_USAGE_EXAMPLES:-}" ]]; then
    printf '\nExamples:\n%s\n' "$_USAGE_EXAMPLES" >&2
  fi
}

parse_common_flags() {
  # Reset in case of double-invocation in tests.
  # shellcheck disable=SC2034  # globals consumed by entry scripts
  NODE="${LUMERA_NODE:-tcp://localhost:26657}"
  # CHAIN_ID resolution order:
  #   1. $LUMERA_CHAIN_ID — explicit user override (env var)
  #   2. $CHAIN_ID         — preset by container ENV / wrapper script
  #   3. ""                — falls through to the --chain-id flag check, and
  #                          if still empty by then, errors at line ~225.
  # The --chain-id flag (parsed below) overrides whichever default applies.
  CHAIN_ID="${LUMERA_CHAIN_ID:-${CHAIN_ID:-}}"
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
  # Entry scripts set `IFS=$'\n\t'` to harden against paths with spaces, but
  # that disables exactly the space-based word-splitting we rely on below to
  # turn each "$probe" into multiple args (e.g., "query evmigration" → two
  # tokens). Restore default IFS just within this function so `$probe`
  # expands across spaces; it reverts on return.
  local IFS=$' \t\n'
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

# resolve_chain_id
# Pin down the effective chain ID and log it for the user. Resolution order:
#   1. $CHAIN_ID already set by parse_common_flags (from --chain-id flag,
#      $LUMERA_CHAIN_ID, or a preset $CHAIN_ID env)
#   2. Auto-detect from `lumerad status` (.node_info.network)
#
# Always logs the final chain ID so the user knows which chain they're
# migrating against — small but important: a wrong chain ID would broadcast
# a tx the chain rejects, costing gas. Showing it up front lets the user
# abort if it's wrong, before the confirm prompt.
resolve_chain_id() {
  if [[ -z "${CHAIN_ID:-}" ]]; then
    local status_json detected
    if status_json=$("$BIN" status --node "$NODE" 2>/dev/null); then
      # Cosmos SDK exposes the field as .node_info.network in JSON; fall back
      # to .NodeInfo.Network for older binaries that emit Pascal-cased keys.
      detected=$(jq -r '(.node_info.network // .NodeInfo.Network) // empty' <<<"$status_json" 2>/dev/null)
      if [[ -n "$detected" && "$detected" != "null" ]]; then
        CHAIN_ID="$detected"
        log_info "auto-detected chain ID from $NODE: $CHAIN_ID"
      fi
    fi
  else
    log_info "chain ID: $CHAIN_ID"
  fi
}

lumerad_q() {
  _read_keyring_flags
  "$BIN" query "$@" --node "$NODE" "${_KRF[@]}" --output json
}

lumerad_tx() {
  if [[ -z "${CHAIN_ID:-}" ]]; then
    log_error "chain ID is required for tx commands; pass --chain-id or set \$LUMERA_CHAIN_ID / \$CHAIN_ID"
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

# preview_tx_body <evmigration-subcommand> <args...>
# Constructs the unsigned tx via `--generate-only` and prints a human summary
# of the message body to stderr (so it appears immediately above the confirm
# prompt). Shows the message type plus any addresses encoded in the message.
# Falls back to a raw JSON dump if the structured pretty-print fails.
#
# Called before broadcasting so the user can see exactly what will be signed
# and submitted before consenting. Read-only — does not broadcast.
preview_tx_body() {
  if [[ -z "${CHAIN_ID:-}" ]]; then
    log_error "chain ID is required to generate tx body"
    return 1
  fi
  _read_keyring_flags
  local generated
  if ! generated=$("$BIN" tx "$@" \
        --node "$NODE" \
        --chain-id "$CHAIN_ID" \
        "${_KRF[@]}" \
        --generate-only \
        --output json 2>/dev/null); then
    log_warn "  could not generate tx body for preview (continuing anyway)"
    return 0
  fi
  {
    echo ""
    echo "Tx body to broadcast:"
    if ! jq -er --arg legacy_c "$_C_LEGACY" --arg new_c "$_C_NEW" --arg reset "$_C_RESET" '
      .body.messages[] |
      "  Type:           " + (."@type" // "<unknown>"),
      (if .legacy_address then "  Legacy address: \($legacy_c)\(.legacy_address)\($reset)" else empty end),
      (if .new_address    then "  New address:    \($new_c)\(.new_address)\($reset)"    else empty end)
    ' <<<"$generated" 2>/dev/null; then
      # Structured pretty-print failed (unexpected schema) — fall back to raw.
      echo "$generated" | jq -C '.body.messages' 2>/dev/null || echo "$generated"
    fi
    local fee gas
    fee=$(jq -r '.auth_info.fee.amount[0] // empty | "\(.amount)\(.denom)"' <<<"$generated" 2>/dev/null)
    gas=$(jq -r '.auth_info.fee.gas_limit // empty' <<<"$generated" 2>/dev/null)
    [[ -n "$gas" && "$gas" != "null" ]] && echo "  Gas limit:      $gas"
    [[ -n "$fee" && "$fee" != "null" ]] && [[ "$fee" != "null0" ]] && echo "  Fee:            $fee"
    echo ""
  } >&2
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
  # `// "none"` substitutes when the field is missing or null in the response,
  # so empty collections render as "none" instead of the JSON-literal "null".
  balance=$(jq -r '.balance_summary // "none"' <<<"$json")
  delegations=$(jq -r '.delegation_count // "none"' <<<"$json")
  unbonding=$(jq -r '.unbonding_count // "none"' <<<"$json")
  redelegations=$(jq -r '.redelegation_count // "none"' <<<"$json")
  authz=$(jq -r '.authz_grant_count // "none"' <<<"$json")
  feegrants=$(jq -r '.feegrant_count // "none"' <<<"$json")
  supernode=$(jq -r 'if .has_supernode then "yes" else "no" end' <<<"$json")
  would=$(jq -r 'if .would_succeed then "yes" else "no" end' <<<"$json")
  actions=$(jq -r '.action_count // "none"' <<<"$json")
  is_validator=$(jq -r 'if .is_validator then "yes" else "no" end' <<<"$json")
  is_multisig=$(jq -r 'if .is_multisig then "yes" else "no" end' <<<"$json")
  threshold=$(jq -r '.threshold // "none"' <<<"$json")
  num_signers=$(jq -r '.num_signers // "none"' <<<"$json")
  val_dels=$(jq -r '.val_delegation_count // "none"' <<<"$json")
  val_unb=$(jq -r '.val_unbonding_count // "none"' <<<"$json")
  val_red=$(jq -r '.val_redelegation_count // "none"' <<<"$json")

  {
    printf 'Migration preview for legacy account %s (coin-type 118, secp256k1):\n' "$(legacy_value "$addr")"
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

# assert_not_migrated <legacy-addr> [<expected-new-addr>]
# Aborts (exit 5) if the legacy address already has a migration record.
# When the optional <expected-new-addr> is supplied, the error message
# distinguishes the two outcomes:
#   - record's new_address == expected: "already migrated to the specified
#     destination (no-op)" + show height
#   - record's new_address != expected: "already migrated to a DIFFERENT
#     destination" + show both addresses
# Both cases exit 5 — the script can't proceed in either, but a user re-running
# after a successful migration sees the friendly version, while a user with
# the wrong destination key sees a precise mismatch.
assert_not_migrated() {
  local addr="$1" expected_new="${2:-}"
  local json
  if ! json=$(lumerad_q evmigration migration-record "$addr" 2>/dev/null); then
    log_error "could not query migration-record for $addr"
    exit 2
  fi
  local rec_legacy rec_new rec_height
  rec_legacy=$(jq -r '.record.legacy_address // empty' <<<"$json")
  if [[ -z "$rec_legacy" ]]; then
    log_info "check OK: no migration record found for legacy address $(legacy_value "$addr")"
    return 0  # no record — clean to migrate
  fi
  rec_new=$(jq -r '.record.new_address // "<missing>"' <<<"$json")
  rec_height=$(jq -r '.record.migration_height // "<unknown>"' <<<"$json")
  if [[ -n "$expected_new" && "$rec_new" == "$expected_new" ]]; then
    log_error "legacy address $(legacy_value "$addr") is already migrated to the specified destination $(new_value "$rec_new") at height $rec_height (no-op; nothing to do)"
  else
    log_error "legacy address $(legacy_value "$addr") is already migrated, but to a DIFFERENT destination:"
    log_error "  on-chain destination:  $(new_value "$rec_new") (migrated at height $rec_height)"
    if [[ -n "$expected_new" ]]; then
      log_error "  destination you asked: $(new_value "$expected_new")"
      log_error "  → either re-run with --new-key matching the on-chain destination, or use that address directly"
    fi
  fi
  exit 5
}

# assert_new_address_unused <new-addr>
# Aborts (exit 5) if the destination address already participated in any
# prior migration — either as the legacy side of an older migration, or as
# the new-side destination of someone else's migration. Uses two distinct
# chain queries: migration-record (keyed by legacy) and
# migration-record-by-new-address (keyed by new).
assert_new_address_unused() {
  local addr="$1"
  if _record_present migration-record "$addr"; then
    log_error "new address $(new_value "$addr") was previously migrated as a legacy address — pick a fresh destination key"
    exit 5
  fi
  log_info "check OK: destination address $(new_value "$addr") has no migration record as a legacy address"

  if _record_present migration-record-by-new-address "$addr"; then
    log_error "new address $(new_value "$addr") is already a migration destination — pick a fresh destination key"
    exit 5
  fi
  log_info "check OK: no migration record found by new address $(new_value "$addr")"
}

# assert_destination_fresh <new-addr>
# Aborts (exit 5) if the destination address already has an on-chain auth
# account or any bank balance. The migration creates the destination as part
# of message handling; reusing an address that already has state would mix
# the migrated state with whatever's already there, and the chain would
# reject the migration as a result. Users get a clear "pick a fresh key"
# message rather than a chain-level rejection at broadcast time.
assert_destination_fresh() {
  local addr="$1"
  local auth_json bal_json bal_amount
  # Query auth account. If the account doesn't exist, the CLI returns a
  # non-zero exit AND/OR an error printed to stderr. Either way, treat the
  # absence of a recognizable account in the response as "fresh".
  auth_json=$(lumerad_q auth account "$addr" 2>/dev/null || true)
  if [[ -z "$auth_json" ]] || [[ "$(jq -r '.account // empty' <<<"$auth_json" 2>/dev/null)" == "" ]]; then
    log_info "check OK: destination address $(new_value "$addr") does not exist on-chain"
    return 0
  fi

  # Account exists. Surface its balance so the user can see what they'd
  # collide with, then abort.
  bal_json=$(lumerad_q bank balances "$addr" 2>/dev/null || printf '{"balances":[]}')
  bal_amount=$(jq -r '
    if (.balances | length) == 0 then
      "(empty)"
    else
      .balances | map(.amount + .denom) | join(", ")
    end
  ' <<<"$bal_json" 2>/dev/null || printf '<unknown>')
  log_error "new address $(new_value "$addr") already exists on-chain — cannot be used as a migration destination"
  log_error "  current balance: $bal_amount"
  log_error "  pick a fresh key whose address has never received any chain activity"
  exit 5
}

# ---- Bank snapshot, tx polling, verification --------------------------------

snapshot_bank_balances() {
  local addr="$1"
  lumerad_q bank balances "$addr"
}

# wait_for_tx <hash>
# Waits for the tx to commit using two paths in order:
#   1. Fast path — `query tx <hash>`. Catches the case where the tx was
#      already committed by the time we got here.
#   2. Slow path — `query wait-tx <hash> --timeout`. Subscribes to the
#      CometBFT WebSocket and waits for the commit event.
#
# Important: the slow-path call goes DIRECTLY to $BIN, NOT through lumerad_q.
# Cosmos SDK's wait-tx accepts --keyring-backend without warning but then
# silently exits non-zero in ~200ms instead of waiting — passing it the
# keyring flags that lumerad_q appends for queries breaks the command. wait-tx
# is read-only and shouldn't need keyring flags anyway. This was confirmed
# experimentally by bisecting flag combinations.
#
# Returns:
#   0 — tx confirmed with code 0 (success). Logs "tx included at height X".
#   1 — tx confirmed with non-zero code (chain rejected the tx). Logs the
#       failure code + raw_log so the caller can just exit; no duplicate logging needed.
#   2 — neither path saw the tx within the timeout (may still land later).
#
# Default timeout is 90s. Override via $LUMERA_TX_WAIT_TIMEOUT (seconds).
wait_for_tx() {
  local hash="$1"
  local timeout="${LUMERA_TX_WAIT_TIMEOUT:-90}"
  local json code height
  local started=$SECONDS

  # Fast path: tx may already be committed.
  # NOTE: `lumerad q tx <missing>` exits 0 with empty stdout (error goes to
  # stderr). So a "found" check is "stdout is non-empty AND parseable JSON",
  # not just "exit 0". Empty json → both nested ifs are false → fall through.
  if json=$(lumerad_q tx "$hash" 2>/dev/null) && [[ -n "$json" ]]; then
    code=$(jq -r '.code // empty' <<<"$json" 2>/dev/null)
    if [[ "$code" == "0" ]]; then
      height=$(jq -r '.height // "<unknown>"' <<<"$json" 2>/dev/null)
      log_info "tx included at height $height (waited $(( SECONDS - started ))s)"
      return 0
    fi
    if [[ -n "$code" ]]; then
      log_error "tx $hash failed with code $code: $(jq -r '.raw_log // "<no raw_log>"' <<<"$json")"
      return 1
    fi
  fi

  # Slow path: subscribe and wait. Bypass lumerad_q so we don't pass
  # --keyring-backend, which silently breaks wait-tx (see header comment).
  if json=$("$BIN" query wait-tx "$hash" --node "$NODE" --output json --timeout "${timeout}s" 2>/dev/null) && [[ -n "$json" ]]; then
    code=$(jq -r '.code // empty' <<<"$json" 2>/dev/null)
    if [[ "$code" == "0" ]]; then
      height=$(jq -r '.height // "<unknown>"' <<<"$json" 2>/dev/null)
      log_info "tx included at height $height (waited $(( SECONDS - started ))s)"
      return 0
    fi
    if [[ -n "$code" ]]; then
      log_error "tx $hash failed with code $code: $(jq -r '.raw_log // "<no raw_log>"' <<<"$json")"
      return 1
    fi
  fi
  local elapsed=$(( SECONDS - started ))

  log_warn "tx $hash not indexed after ${elapsed}s (timeout was ${timeout}s); it may still land on chain"
  log_warn "  check status manually: $BIN q tx $hash"
  log_warn "  to wait longer on slow networks, re-run with: LUMERA_TX_WAIT_TIMEOUT=300 $(basename "$0") ..."
  log_warn "  proceeding to on-chain verification — if the migration record/balances are present, the migration succeeded"
  return 2
}

# assert_broadcast_accepted <broadcast-response-json>
# Validates the JSON response from `lumerad tx ...` (broadcast-mode=sync, the
# default). Exits non-zero on CheckTx rejection (code != 0) or missing txhash;
# returns the txhash on success via stdout so callers can capture it cleanly.
#
# Why a separate step: a tx that fails CheckTx still returns a valid JSON
# with a txhash field — but it's not in any mempool and will never land. The
# previous flow only checked for missing txhash, so a CheckTx rejection would
# slip through and only manifest as a wait_for_tx timeout 90s later. This
# catches it immediately with the right error context.
assert_broadcast_accepted() {
  local broadcast_json="$1"
  local code raw_log tx_hash
  tx_hash=$(jq -r '.txhash // empty' <<<"$broadcast_json" 2>/dev/null)
  if [[ -z "$tx_hash" || "$tx_hash" == "null" ]]; then
    log_error "broadcast returned no txhash: $broadcast_json"
    exit 2
  fi
  code=$(jq -r '.code // 0' <<<"$broadcast_json" 2>/dev/null)
  if [[ -n "$code" && "$code" != "0" ]]; then
    raw_log=$(jq -r '.raw_log // "<no raw_log>"' <<<"$broadcast_json" 2>/dev/null)
    log_error "broadcast rejected at CheckTx: code=$code raw_log=$raw_log (txhash=$tx_hash, never landed in a block)"
    exit 2
  fi
  printf '%s\n' "$tx_hash"
}

# verify_migration <legacy> <new> <pre-broadcast-legacy-balances-json>
verify_migration() {
  local legacy="$1" new="$2" snap_json="$3"

  # 1. Migration record must exist and point to <new>.
  local rec_json rec_new
  if ! rec_json=$(lumerad_q evmigration migration-record "$legacy" 2>/dev/null); then
    log_error "post-check: could not query migration-record for $(legacy_value "$legacy") — verify manually"
    exit 7
  fi
  rec_new=$(jq -r '.record.new_address // empty' <<<"$rec_json")
  if [[ "$rec_new" != "$new" ]]; then
    log_error "post-check: migration record for $(legacy_value "$legacy") does not point to $(new_value "$new") (got: '$(new_value "$rec_new")')"
    exit 7
  fi

  # 2. Legacy balances must be all zero (account removed or empty).
  local legacy_after
  if ! legacy_after=$(lumerad_q bank balances "$legacy" 2>/dev/null); then
    log_error "post-check: could not query legacy bank balances for $(legacy_value "$legacy") — verify manually"
    exit 7
  fi
  if [[ "$(jq -r '[.balances[].amount | tonumber] | add // 0' <<<"$legacy_after")" != "0" ]]; then
    log_error "post-check: legacy address $(legacy_value "$legacy") still has non-zero balance"
    exit 7
  fi

  # 3. For every {denom,amount} in snap_json, new balances must be >= amount.
  local new_after
  if ! new_after=$(lumerad_q bank balances "$new" 2>/dev/null); then
    log_error "post-check: could not query new bank balances for $(new_value "$new") — verify manually"
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

# show_migration_summary <legacy> <new>
# Pretty-prints the on-chain migration record and the new account's balance.
# Called after verify_migration succeeds so the user sees authoritative
# chain state confirming the migration. Failures here are non-fatal (the
# verify_migration assertions already passed); we just log a warning if a
# query fails and continue.
show_migration_summary() {
  local legacy="$1" new="$2"
  local rec_json balances_json

  echo "" >&2
  echo "Migration record (chain state):" >&2
  if rec_json=$(lumerad_q evmigration migration-record "$legacy" 2>/dev/null); then
    # Pretty-print known fields; gracefully tolerate schema additions/renames.
    jq -r --arg legacy_c "$_C_LEGACY" --arg new_c "$_C_NEW" --arg reset "$_C_RESET" '.record |
      "  legacy address: \($legacy_c)\(.legacy_address // "<missing>")\($reset)\n" +
      "  new address:    \($new_c)\(.new_address // "<missing>")\($reset)\n" +
      "  height:         \(.migration_height // "<missing>")\n" +
      "  unix time:      \(.migration_time // "<missing>")"
    ' <<<"$rec_json" >&2 2>/dev/null || {
      log_warn "  could not parse migration-record JSON; raw output follows"
      echo "$rec_json" >&2
    }
  else
    log_warn "  could not query migration-record for $(legacy_value "$legacy")"
  fi

  echo "" >&2
  printf 'New account balance (%s):\n' "$(new_value "$new")" >&2
  if balances_json=$(lumerad_q bank balances "$new" 2>/dev/null); then
    local pretty
    pretty=$(jq -r '
      if (.balances | length) == 0 then
        "  (empty)"
      else
        .balances | map("  " + .amount + .denom) | join("\n")
      end
    ' <<<"$balances_json" 2>/dev/null)
    if [[ -n "$pretty" ]]; then
      echo "$pretty" >&2
    else
      echo "  (could not parse balance response)" >&2
    fi
  else
    log_warn "  could not query bank balances for $(new_value "$new")"
  fi
  echo "" >&2
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

_mnemonic_key_address() {
  local name="$1"
  lumerad_keys show "$name" -a 2>/dev/null
}

_mnemonic_import_one_key() {
  local mnemonic="$1" name="$2" coin_type="$3" algo="$4" role="$5"
  local existing_addr="" temp_name="" temp_addr="" created_temp=0

  if existing_addr=$(_mnemonic_key_address "$name"); then
    if [[ "$role" == "new" ]]; then
      temp_name="__evmig_check_ekey_${name}_$$_${RANDOM}"
    else
      temp_name="__evmig_check_legacy_${name}_$$_${RANDOM}"
    fi

    printf '%s\n' "$mnemonic" | lumerad_keys add "$temp_name" \
      --recover --coin-type "$coin_type" --algo "$algo" >/dev/null
    created_temp=1
    temp_addr=$(_mnemonic_key_address "$temp_name")
    lumerad_keys delete "$temp_name" --yes >/dev/null 2>&1 || true
    created_temp=0

    if [[ "$existing_addr" != "$temp_addr" ]]; then
      if [[ "$role" == "new" ]]; then
        log_error "new EVM key '$(new_value "$name")' already exists in keyring but does not match the mnemonic"
        log_error "  existing address: $(new_value "$existing_addr")"
        log_error "  mnemonic address: $(new_value "$temp_addr")"
      else
        log_error "legacy key '$(legacy_value "$name")' already exists in keyring but does not match the mnemonic"
        log_error "  existing address: $(legacy_value "$existing_addr")"
        log_error "  mnemonic address: $(legacy_value "$temp_addr")"
      fi
      exit 1
    fi

    if [[ "$role" == "new" ]]; then
      log_info "new EVM key $(new_value "$name") already exists in keyring and matches mnemonic; reusing it"
    else
      log_info "legacy key $(legacy_value "$name") already exists in keyring and matches mnemonic; reusing it"
    fi
    return 0
  fi

  if (( created_temp == 1 )); then
    lumerad_keys delete "$temp_name" --yes >/dev/null 2>&1 || true
  fi

  printf '%s\n' "$mnemonic" | lumerad_keys add "$name" \
    --recover --coin-type "$coin_type" --algo "$algo" >/dev/null
  _MNEMONIC_CLEANUP_KEYS+=("$name")

  if [[ "$role" == "new" ]]; then
    log_info "imported new EVM key $(new_value "$name") from mnemonic for this run"
  else
    log_info "imported legacy key $(legacy_value "$name") from mnemonic for this run"
  fi
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

  local mnemonic
  mnemonic=$(< "$mfile")

  # Register cleanup before doing anything else that might fail. Only keys
  # created by this function are added to _MNEMONIC_CLEANUP_KEYS; pre-existing
  # matching keys are reused and never deleted by the trap.
  _MNEMONIC_CLEANUP_KEYS=()
  # shellcheck disable=SC2154  # rc is captured at trap runtime
  trap 'rc=$?; cleanup_mnemonic_keys; exit "$rc"' EXIT

  _mnemonic_import_one_key "$mnemonic" "$legacy_name" 118 secp256k1 legacy
  _mnemonic_import_one_key "$mnemonic" "$new_name" 60 eth_secp256k1 new

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
  # Same IFS caveat as require_binary: entry scripts use a strict IFS that
  # would prevent the intentional space-splitting on $probe below. Restore
  # default IFS locally for this function.
  local IFS=$' \t\n'
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
    log_error "legacy key not found in keyring: $(legacy_value "$key_name")"
    exit 1
  fi
  pk_type=$(jq -r '(.pubkey | if type == "string" then fromjson else . end | ."@type") // "unknown"' <<<"$info" 2>/dev/null || printf 'unknown')
  if [[ "$pk_type" != "/cosmos.crypto.secp256k1.PubKey" ]]; then
    log_error "legacy key '$(legacy_value "$key_name")' is not secp256k1 (got $pk_type) — legacy migration requires a coin-type 118 secp256k1 key"
    exit 1
  fi
}

# assert_eth_key <key-name>
# For submit. Confirms the key is an eth_secp256k1 variant.
assert_eth_key() {
  local key_name="$1"
  local info pk_type
  if ! info=$(lumerad_keys show "$key_name" --output json 2>/dev/null); then
    log_error "new EVM key not found in keyring: $(new_value "$key_name")"
    exit 1
  fi
  pk_type=$(jq -r '(.pubkey | if type == "string" then fromjson else . end | ."@type") // "unknown"' <<<"$info" 2>/dev/null || printf 'unknown')
  case "$pk_type" in
    /cosmos.crypto.ethsecp256k1.PubKey|\
    /ethermint.crypto.v1.ethsecp256k1.PubKey|\
    /cosmos.evm.crypto.v1.ethsecp256k1.PubKey) ;;
    *)
      log_error "new EVM key '$(new_value "$key_name")' is not eth_secp256k1 (got $pk_type) — submit requires the new EVM destination key"
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
