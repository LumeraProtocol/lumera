#!/usr/bin/env bash
#
# Multisig migration helper. Dispatches on the first positional argument to
# one of four subcommand functions wrapping lumerad tx evmigration
# {generate-proof-payload, sign-proof, combine-proof, submit-proof}.
# See docs/design/evmigration-multisig-scripts-design.md.

set -euo pipefail
IFS=$'\n\t'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./evmigration-common.sh disable=SC1091
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
