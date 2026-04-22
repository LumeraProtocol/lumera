#!/usr/bin/env bash
#
# Mock lumerad binary for bats tests. Routes on argv to fixtures/*.json.
# Override behavior with env vars:
#   SHIM_EXIT=<n>         force exit code n
#   SHIM_FIXTURE=<name>   force a specific fixture name (without .json)
#   SHIM_STDERR=<msg>     emit this to stderr before exiting
#
# Per-command overrides let a single test change just one endpoint:
#   SHIM_ESTIMATE_FIXTURE, SHIM_RECORD_FIXTURE, SHIM_PARAMS_FIXTURE,
#   SHIM_AUTH_FIXTURE, SHIM_BANK_FIXTURE, SHIM_TX_FIXTURE
#
# State-machine support (for verify_migration end-to-end tests):
#   SHIM_STATE_FILE=<path>           — when set, tx evmigration touches this file,
#                                      and subsequent record/bank queries switch to
#                                      their "after" fixtures.
#   SHIM_RECORD_AFTER_FIXTURE=<name> — fixture for migration-record AFTER broadcast.
#   SHIM_BANK_AFTER_FIXTURE=<name>   — fixture for legacy bank balances AFTER broadcast.
#   SHIM_BANK_NEW_FIXTURE=<name>     — fixture for bank balances of the new stub addr
#                                      (always, regardless of phase).

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
  "query evmigration migration-estimate "*)               emit "${SHIM_ESTIMATE_FIXTURE:-estimate-ok}" ;;
  "query evmigration migration-record "*)
    if [[ -n "${SHIM_STATE_FILE:-}" && -f "${SHIM_STATE_FILE}" && -n "${SHIM_RECORD_AFTER_FIXTURE:-}" ]]; then
      emit "$SHIM_RECORD_AFTER_FIXTURE"
    else
      emit "${SHIM_RECORD_FIXTURE:-record-not-found}"
    fi
    ;;
  "query evmigration migration-record-by-new-address "*)  emit "${SHIM_RECORD_FIXTURE:-record-not-found}" ;;
  "query evmigration params"*)                            emit "${SHIM_PARAMS_FIXTURE:-params}" ;;
  "query auth account "*)                                 emit "${SHIM_AUTH_FIXTURE:-auth-account}" ;;
  "query bank balances lumera1newshim"*)
    emit "${SHIM_BANK_NEW_FIXTURE:-bank-balances}" ;;
  "query bank balances "*)
    if [[ -n "${SHIM_STATE_FILE:-}" && -f "${SHIM_STATE_FILE}" && -n "${SHIM_BANK_AFTER_FIXTURE:-}" ]]; then
      emit "$SHIM_BANK_AFTER_FIXTURE"
    else
      emit "${SHIM_BANK_FIXTURE:-bank-balances}"
    fi
    ;;
  "query tx "*)                                           emit "${SHIM_TX_FIXTURE:-tx-success}" ;;
  "keys show "*)
    case "$*" in
      *"newkey"*) printf 'lumera1newshimaddrxxxxxxxxxxxxxxxxxxxxxxxxxxxx\n' ;;
      *)          printf 'lumera1shimaddr1qxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx\n' ;;
    esac
    ;;
  "debug addr "*)
    cat <<'ADDR'
Address: [1 2 3]
Address (hex): 0102030405060708090A0B0C0D0E0F1011121314
Bech32 Acc: lumera1shimaccxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
Bech32 Val: lumeravalopershimxxxxxxxxxxxxxxxxxxxxxxxxxx
Bech32 Con: lumeravalconsshimxxxxxxxxxxxxxxxxxxxxxxxxxxx
ADDR
    ;;
  "query evmigration --help"*)                            printf 'help stub\n' ;;
  "tx evmigration claim-legacy-account --help"*)         printf 'help stub\n' ;;
  "tx evmigration migrate-validator --help"*)            printf 'help stub\n' ;;
  "tx evmigration"*)
    if [[ -n "${SHIM_STATE_FILE:-}" ]]; then
      touch "$SHIM_STATE_FILE"
    fi
    emit broadcast-success ;;
  "version"*)                                             printf 'v0.0.0-shim\n' ;;
  *) printf 'shim: unhandled args: %s\n' "$*" >&2; exit 1 ;;
esac

exit "${SHIM_EXIT:-0}"
