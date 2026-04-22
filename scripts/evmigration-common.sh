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
  # shellcheck disable=SC2034
  NODE="${LUMERA_NODE:-tcp://localhost:26657}"
  # shellcheck disable=SC2034
  CHAIN_ID="${LUMERA_CHAIN_ID:-}"
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

  local positional=()
  # shellcheck disable=SC2034
  while (( $# > 0 )); do
    case "$1" in
      --node)            NODE="$2"; shift 2 ;;
      --chain-id)        CHAIN_ID="$2"; shift 2 ;;
      --keyring-backend) KEYRING_BACKEND="$2"; shift 2 ;;
      --keyring-dir)     KEYRING_DIR="$2"; shift 2 ;;
      --home)            HOME_DIR="$2"; shift 2 ;;
      --mnemonic-file)   MNEMONIC_FILE="$2"; shift 2 ;;
      --yes|-y)          YES=1; shift ;;
      --dry-run)         DRY_RUN=1; shift ;;
      --binary)          BIN="$2"; shift 2 ;;
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
