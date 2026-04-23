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
  --chain-id <id> --node <url> --out <path> [--sig-format SIG_FORMAT_CLI|SIG_FORMAT_ADR036] [--binary <path>]
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

  # Wire up globals used by common helpers
  # shellcheck disable=SC2034  # consumed by lumerad_q / lumerad_tx_query_style helpers
  BIN="$binary"
  # shellcheck disable=SC2034
  NODE="$node"
  # shellcheck disable=SC2034
  CHAIN_ID="$chain_id"
  # shellcheck disable=SC2034  # some helpers read KEYRING_BACKEND even on query paths
  KEYRING_BACKEND="test"

  require_multisig_binary
  require_jq

  # Check on-chain pubkey BEFORE estimate so a nil-pubkey multisig gets
  # the exit-8 "seed the pubkey first" remediation, not a confusing
  # downstream error.
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

  # Pull estimate — provides is_validator, would_succeed, is_multisig confirmation.
  local estimate
  estimate=$(preflight_estimate "$legacy")
  assert_multisig "$estimate"

  if [[ "$kind" == "validator" && "$(jq -r '.is_validator' <<<"$estimate")" != "true" ]]; then
    log_error "--kind validator specified but $legacy is not a validator operator"
    exit 6
  fi

  # Design §3.1: catch already-migrated / already-used destinations
  # and doomed ceremonies BEFORE co-signers spend time on partials.
  assert_not_migrated "$legacy"
  assert_new_address_unused "$new"
  assert_estimate_succeeds "$estimate"

  # Pass through to lumerad via the query-style helper so keyring flags
  # don't leak onto generate-proof-payload (which doesn't accept them).
  local args=(evmigration generate-proof-payload
    --legacy "$legacy"
    --new "$new"
    --kind "$kind"
    --out "$out")
  [[ -n "$sig_format" ]] && args+=(--sig-format "$sig_format")

  log_info "generating proof template at $out"
  lumerad_tx_query_style "${args[@]}"
  log_info "done — distribute $out to the K co-signers"
}
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

  # shellcheck disable=SC2034
  BIN="$binary"
  # shellcheck disable=SC2034
  CHAIN_ID="$chain_id"
  # shellcheck disable=SC2034
  KEYRING_BACKEND="$keyring_backend"
  # shellcheck disable=SC2034
  KEYRING_DIR="$keyring_dir"
  # shellcheck disable=SC2034
  HOME_DIR="$home_dir"

  require_binary
  require_jq

  # Parse + validate the input proof/partial. read_proof_file rejects
  # single-key proofs (exit 3), bad payload_hex (exit 9), missing fields
  # (exit 9), and structural issues (exit 9).
  local pjson
  pjson=$(read_proof_file "$input")

  # Confirm --from is a legacy secp256k1 sub-key, not an eth key.
  assert_secp256k1_key "$from"

  # Confirm --from's pubkey is in the proof's sub-key set.
  local from_pubkey listed
  from_pubkey=$(key_pubkey_b64 "$from")
  listed=$(jq -r '.multisig.sub_pub_keys_b64[]' <<<"$pjson")
  if ! grep -qFx "$from_pubkey" <<<"$listed"; then
    log_error "key '$from' pubkey is not among the multisig sub-keys in $input"
    exit 1
  fi

  # Pass through to lumerad tx evmigration sign-proof.
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

  # shellcheck disable=SC2034
  BIN="$binary"

  require_jq

  # Per-file + cross-file consistency check. summarize_partials prints the
  # K-of-N entry-presence matrix to stderr and exits 9 on cross-file
  # disagreement (it calls read_proof_file internally, which rejects
  # single-key proofs with exit 3 and bad payload_hex with exit 9).
  # Returns 0 iff distinct signer indices >= threshold.
  if ! summarize_partials "${positional[@]}"; then
    exit 4
  fi

  # Pass through to lumerad combine-proof. If lumerad reports fewer valid
  # signatures than the threshold, map its exit to exit 4.
  local args=(tx evmigration combine-proof "${positional[@]}" --out "$out")
  local combine_out combine_rc=0
  combine_out=$("$BIN" "${args[@]}" 2>&1) || combine_rc=$?
  printf '%s\n' "$combine_out" >&2
  if [[ "$combine_out" == *"need "*"valid partial signatures"* ]]; then
    exit 4
  fi
  if (( combine_rc != 0 )); then
    exit "$combine_rc"
  fi
  log_info "combined tx written to $out"
}
_mms_submit() {
  local input="" from="" chain_id="" node="" binary="lumerad"
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

  # shellcheck disable=SC2034
  BIN="$binary"
  # shellcheck disable=SC2034
  NODE="$node"
  # shellcheck disable=SC2034
  CHAIN_ID="$chain_id"
  # shellcheck disable=SC2034
  KEYRING_BACKEND="$keyring_backend"
  # shellcheck disable=SC2034
  KEYRING_DIR="$keyring_dir"
  # shellcheck disable=SC2034
  HOME_DIR="$home_dir"
  # shellcheck disable=SC2034
  YES="$yes"
  # shellcheck disable=SC2034
  DRY_RUN="$dry_run"

  require_multisig_binary
  require_jq

  # Parse + validate tx.json. Rejects single-key proofs (exit 3),
  # missing fields / malformed (exit 9). Emits compact summary JSON.
  local tx_meta
  tx_meta=$(read_migration_tx_file "$input")
  local legacy new kind threshold num_signers
  legacy=$(jq -r '.legacy_address' <<<"$tx_meta")
  new=$(jq -r '.new_address' <<<"$tx_meta")
  kind=$(jq -r '.kind' <<<"$tx_meta")
  threshold=$(jq -r '.threshold' <<<"$tx_meta")
  num_signers=$(jq -r '.num_signers' <<<"$tx_meta")

  # --from must be eth_secp256k1 and resolve to the tx's new_address.
  assert_eth_key "$from"
  local from_addr
  from_addr=$(resolve_address "$from")
  if [[ "$from_addr" != "$new" ]]; then
    log_error "--from '$from' resolves to $from_addr but tx new_address is $new"
    exit 1
  fi

  assert_not_migrated "$legacy"
  assert_new_address_unused "$new"

  # Fresh estimate — catches ceremony-duration chain-state drift.
  local estimate
  estimate=$(preflight_estimate "$legacy")
  assert_multisig "$estimate"
  assert_estimate_succeeds "$estimate"

  local snap
  snap=$(snapshot_bank_balances "$legacy")

  # Confirmation banner
  {
    printf '\n==== Multisig migration submit ====\n'
    printf '  Kind:      %s\n' "$kind"
    printf '  Multisig:  %s-of-%s\n' "$threshold" "$num_signers"
    printf '  Legacy:    %s\n' "$legacy"
    printf '  New:       %s\n' "$new"
    printf '  From:      %s\n' "$from"
    printf '===================================\n\n'
  } >&2

  # Validator kind needs separate downtime acknowledgement.
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
