#!/usr/bin/env bash
#
# Mock lumerad binary for bats tests. Routes on argv to fixtures/*.json.
# Override behavior with env vars:
#   SHIM_EXIT=<n>         force exit code n
#   SHIM_COMBINE_EXIT=<n> force exit code for tx evmigration combine-proof only
#   SHIM_FIXTURE=<name>   force a specific fixture name (without .json)
#   SHIM_STDERR=<msg>     emit this to stderr before exiting
#
# Per-command overrides let a single test change just one endpoint:
#   SHIM_ESTIMATE_FIXTURE, SHIM_RECORD_FIXTURE, SHIM_PARAMS_FIXTURE,
#   SHIM_AUTH_FIXTURE, SHIM_AUTH_NEW_FIXTURE, SHIM_BANK_FIXTURE, SHIM_TX_FIXTURE
#
# State-machine support (for verify_migration end-to-end tests):
#   SHIM_STATE_FILE=<path>           — when set, tx evmigration touches this file,
#                                      and subsequent record/bank queries switch to
#                                      their "after" fixtures.
#   SHIM_RECORD_AFTER_FIXTURE=<name> — fixture for migration-record AFTER broadcast.
#   SHIM_BANK_AFTER_FIXTURE=<name>   — fixture for legacy bank balances AFTER broadcast.
#   SHIM_BANK_NEW_FIXTURE=<name>     — fixture for bank balances of the new stub addr
#                                      (always, regardless of phase).
#
# submit-proof failure simulation (for broadcast-RPC-EOF tests):
#   SHIM_SUBMIT_EXIT=<n>      — force submit-proof to exit n (after emitting stderr)
#   SHIM_SUBMIT_STDERR=<msg>  — emit this to stderr just for submit-proof
#   SHIM_SUBMIT_LANDED=1      — combined with SHIM_SUBMIT_EXIT, simulate the case
#                                where the tx actually landed before the RPC dropped
#                                (record/bank queries flip to their *_AFTER fixtures).

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

# emit_or_write <fixture-name> <all-original-argv>
# If argv contains `--out <path>`, copy fixture to that path and print a
# short confirmation on stdout (matching the lumerad CLI's tx commands
# that write their output to --out). Otherwise, emit fixture to stdout.
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
  "query auth account lumera1newshim"*)
    emit "${SHIM_AUTH_NEW_FIXTURE:-auth-account-not-found}"
    ;;
  "query auth account "*)
    case "${SHIM_AUTH_TYPE:-single}" in
      multisig)         emit auth-account-multisig ;;
      multisig-type-url) emit auth-account-multisig-type-url ;;
      multisig-amino)   emit auth-account-multisig-amino ;;
      multisig-nested)  emit auth-account-multisig-nested ;;
      nilpubkey)        emit auth-account-nilpubkey ;;
      *)                emit "${SHIM_AUTH_FIXTURE:-auth-account}" ;;
    esac
    ;;
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
    # Route on the key name (second word after "show") and presence of --output json.
    # If --output json: emit a keys-show-<name>.json fixture; fall back to a generic one.
    # If -a (or default): emit the stub address like before.
    __has_json=0; __key_name=""
    __i=0
    for __arg in "$@"; do
      __i=$((__i+1))
      if (( __i == 3 )); then
        __key_name="$__arg"
      fi
      [[ "$__arg" == "--output" ]] && __has_json=1
      [[ "$__arg" == "json" ]] && __has_json=$(( __has_json == 1 ? 2 : __has_json ))
    done
    case ",${SHIM_KEYS_MISSING:-}," in
      *,"$__key_name",*) printf 'key not found: %s\n' "$__key_name" >&2; exit 1 ;;
    esac
    if (( __has_json >= 2 )); then
      __fixture="keys-show-$__key_name"
      if [[ -f "$fixtures_dir/$__fixture.json" ]]; then
        emit "$__fixture"
      else
        # Fallback: synthesize a minimal JSON so unknown key names still return valid shape
        printf '{"name":"%s","type":"local","address":"lumera1shimaddr1qxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx","pubkey":"{\\"@type\\":\\"/cosmos.crypto.secp256k1.PubKey\\",\\"key\\":\\"A0000000000000000000000000000000000000000000\\"}"}\n' "$__key_name"
      fi
    else
      case "$*" in
        *"newkey"*|*"new-eth-key"*|*"new-msig"*|*"evm"*|*"ekey"*) printf 'lumera1newshimaddrxxxxxxxxxxxxxxxxxxxxxxxxxxxx\n' ;;
        *)                          printf 'lumera1shimaddr1qxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx\n' ;;
      esac
    fi
    ;;
  "keys add "*)
    if [[ "$*" == *"--output json"* ]]; then
      case "$*" in
        *"eth_secp256k1"*) printf '{"name":"__shim","address":"lumera1newshimaddrxxxxxxxxxxxxxxxxxxxxxxxxxxxx","mnemonic":"test mnemonic"}\n' ;;
        *)                 printf '{"name":"__shim","address":"lumera1shimaddr1qxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx","mnemonic":"test mnemonic"}\n' ;;
      esac
    else
      printf 'added key\n'
    fi
    ;;
  "keys delete "*)                                         printf 'deleted key\n' ;;
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
  "tx evmigration generate-proof-payload --help"*)       printf 'help stub\n' ;;
  "tx evmigration sign-proof --help"*)                   printf 'help stub\n' ;;
  "tx evmigration combine-proof --help"*)                printf 'help stub\n' ;;
  "tx evmigration submit-proof --help"*)                 printf 'help stub\n' ;;
  "tx evmigration generate-proof-payload"*)
    # Mirror the real Go CLI's validateSideSpec duplicate-sub-key rejection
    # (see TestValidateSideSpec_RejectsDuplicateSubKeys). The wrapper passes
    # --new-sub-pub-keys through verbatim, so detecting a duplicate here
    # exercises the wrapper's failure-propagation path end-to-end.
    args=( "$@" )
    for ((i = 0; i < ${#args[@]}; i++)); do
      if [[ "${args[i]}" == "--new-sub-pub-keys" ]]; then
        IFS=',' read -ra _keys <<< "${args[i+1]:-}"
        declare -A _seen
        for j in "${!_keys[@]}"; do
          k="${_keys[j]}"
          if [[ -n "${_seen[$k]:-}" ]]; then
            printf 'sub_pub_keys[%d] duplicates sub_pub_keys[%d]\n' "$j" "${_seen[$k]}" >&2
            exit 1
          fi
          _seen[$k]=$j
        done
        unset _seen
        break
      fi
    done
    emit_or_write "${SHIM_PROOF_FIXTURE:-proof-template}" "$@"
    ;;
  "tx evmigration sign-proof"*)
    emit_or_write "${SHIM_PARTIAL_FIXTURE:-partial-alice}" "$@"
    ;;
  "tx evmigration combine-proof"*)
    emit_or_write "${SHIM_COMBINED_FIXTURE:-combined-tx}" "$@"
    exit "${SHIM_COMBINE_EXIT:-${SHIM_EXIT:-0}}"
    ;;
  "tx evmigration submit-proof"*)
    if [[ -n "${SHIM_SUBMIT_STDERR:-}" ]]; then
      printf '%s\n' "$SHIM_SUBMIT_STDERR" >&2
    fi
    if [[ -n "${SHIM_SUBMIT_EXIT:-}" ]]; then
      # SHIM_SUBMIT_LANDED=1 simulates "the tx actually entered a block before
      # the RPC dropped"; the wrapper's recovery path can then observe an
      # on-chain migration record on retry.
      if [[ -n "${SHIM_STATE_FILE:-}" && "${SHIM_SUBMIT_LANDED:-0}" == "1" ]]; then
        touch "$SHIM_STATE_FILE"
      fi
      exit "$SHIM_SUBMIT_EXIT"
    fi
    if [[ -n "${SHIM_STATE_FILE:-}" ]]; then
      touch "$SHIM_STATE_FILE"
    fi
    emit broadcast-success
    ;;
  "tx evmigration"*)
    if [[ -n "${SHIM_STATE_FILE:-}" ]]; then
      touch "$SHIM_STATE_FILE"
    fi
    emit broadcast-success ;;
  "status"*)
    if [[ -n "${SHIM_STATUS_EXIT:-}" ]]; then
      exit "$SHIM_STATUS_EXIT"
    fi
    printf '{"node_info":{"network":"shim-test"}}\n'
    ;;
  "version"*)                                             printf 'v0.0.0-shim\n' ;;
  *) printf 'shim: unhandled args: %s\n' "$*" >&2; exit 1 ;;
esac

exit "${SHIM_EXIT:-0}"
