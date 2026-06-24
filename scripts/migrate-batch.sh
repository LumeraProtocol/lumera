#!/usr/bin/env bash
###################################################################################
# Copyright 2026 The Lumera Protocol
#
# Batch driver for EVM migration of many legacy accounts in a single run.
#
# Operator workflow:
#   1) Provide a mnemonics file describing target legacy accounts (see FORMAT)
#   2) `report`  — Offline classification of every target (no chain calls)
#   3) `status`  — Read-only chain probe: per-target state machine
#   4) `execute` — Run the full migration lifecycle for each target
#
# This driver does NOT reimplement migration logic. Per target it:
#   - Imports signer mnemonics into a per-target EPHEMERAL keyring (mode 0700,
#     wiped on exit), as both legacy (coin-type 118 / secp256k1) and new
#     (coin-type 60 / eth_secp256k1) variants.
#   - For multisigs, reconstructs both legacy and new multisigs by pubkey-
#     matched signer order, and asserts the reconstructed legacy address
#     equals the address in the mnemonics file.
#   - Optionally tops up the legacy address from --funder, if the target cannot
#     pay for its own pubkey-publishing self-send from spendable balance.
#   - If on-chain pubkey is missing, performs a self-send to publish it
#     (multisig variant: --generate-only + sign×K + multisign + broadcast).
#   - Delegates the migration ceremony itself to:
#       scripts/migrate-multisig.sh  (multisig — runs generate / sign×K /
#                                     combine / submit in one process)
#       scripts/migrate-account.sh   (standalone single-sig)
#   - Verifies via `evmigration migration-record` chain query.
#
# === MNEMONICS FILE FORMAT ===
#
# A single JSON object. Keys are arbitrary local names. Each entry is one of:
#
#   "local" entry  (single-key, owns a mnemonic):
#     {
#       "address": "lumera1...",
#       "mnemonic": "word1 word2 ...",           (24 BIP-39 words)
#       "pubkey":   "{\"@type\":\"....secp256k1.PubKey\",\"key\":\"base64\"}",
#       "type":     "local"
#     }
#
#   "multi" entry  (multisig, NO mnemonic of its own):
#     {
#       "address": "lumera1...",
#       "mnemonic": "",
#       "pubkey":   "{\"@type\":\"....LegacyAminoPubKey\",
#                     \"threshold\":K,
#                     \"public_keys\":[...]}",
#       "type":     "multi"
#     }
#
# Each "multi" entry's signer pubkeys MUST resolve against "local" entries in
# the same file by exact pubkey-key match. Signer ORDER in public_keys is
# significant — it determines the multisig address. This driver matches by
# pubkey, never by name suffix.
#
# A "local" entry not referenced by any "multi" is treated as a standalone
# single-sig migration target.
#
# === SAFETY ===
#
# - The mnemonics file path is treated as untrusted input but never copied;
#   per-signer mnemonics are written to mode-0600 temp files in the per-target
#   ephemeral keyring dir, consumed by import_from_mnemonic, then deleted.
# - The entire per-target ephemeral keyring dir is wiped on EXIT (success,
#   failure, or signal).
# - --funder uses the OPERATOR's main keyring (separate --funder-* flags).
#   It is never imported into the ephemeral keyring.
# - --dry-run runs through all read-only steps (chain queries, address
#   reconstruction, planning) and stops before any tx broadcast.
###################################################################################

set -euo pipefail
IFS=$'\n\t'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# shellcheck source=./evmigration-common.sh disable=SC1091
source "${SCRIPT_DIR}/evmigration-common.sh"

###############################################################################
# Usage
###############################################################################
_mb_usage() {
  cat >&2 <<'USAGE'
Usage: migrate-batch.sh <subcommand> [args...]

Subcommands:
  report    Parse the mnemonics file, classify targets. No chain calls.
  status    Per-target chain-state probe. Read-only. Safe to run anywhere.
  execute   Run the full migration lifecycle for each target.

Run `migrate-batch.sh <subcommand> --help` for subcommand-specific flags.

Mnemonics file format: see header of this script.
USAGE
}

###############################################################################
# Shared helpers for status/execute
###############################################################################

# _mb_load_plan <mnemonics_file>
# Emits the same JSON plan structure as `report` to stdout. Fail-closed.
_mb_load_plan() {
  local mfile="$1"
  if [[ ! -r "$mfile" ]]; then
    log_error "cannot read mnemonics file: $mfile"
    exit 9
  fi
  if ! jq -e 'type == "object"' "$mfile" >/dev/null 2>&1; then
    log_error "mnemonics file is not a JSON object: $mfile"
    exit 9
  fi
  local bad
  bad=$(jq -r '
    to_entries
    | map(select(
        (.value | type) != "object"
        or (.value.type // "" | (. != "local" and . != "multi"))
        or (.value.address // "" | startswith("lumera1") | not)
        or ((.value.pubkey // "") == "")
        or ((.value.pubkey | try fromjson catch null | type) != "object")
      ))
    | map(.key) | join(", ")
  ' "$mfile")
  if [[ -n "$bad" ]]; then
    log_error "structurally invalid entries: $bad"
    exit 9
  fi
  local plan
  plan=$(jq -r '
    . as $root
    | ([ to_entries[] | select(.value.type == "local")
          | { name: .key,
              address: .value.address,
              mnemonic: (.value.mnemonic // ""),
              pubkey_b64: (.value.pubkey | fromjson | .key) } ]) as $locals
    | ($locals | map({ (.pubkey_b64): . }) | add // {}) as $by_pk
    | ([ to_entries[] | select(.value.type == "multi")
          | (.value.pubkey | fromjson) as $mpk
          | { name: .key,
              address: .value.address,
              threshold: $mpk.threshold,
              n: ($mpk.public_keys | length),
              signers: [ $mpk.public_keys[]
                          | $by_pk[.key]
                              // { name: null, address: null, mnemonic: "",
                                   pubkey_b64: .key, unresolved: true } ],
              unresolved_pubkeys: [ $mpk.public_keys[]
                                     | select(($by_pk[.key]) == null)
                                     | .key ] } ]) as $multis
    | ([ $multis[] | .signers[]?.pubkey_b64 ] | unique) as $referenced
    | ($locals | map(select(.pubkey_b64 as $pk | $referenced | index($pk) | not))) as $standalones
    | { multis: $multis,
        standalones: $standalones }
  ' "$mfile")
  local unresolved_count
  unresolved_count=$(jq -r '[.multis[] | select(.unresolved_pubkeys | length > 0)] | length' <<<"$plan")
  if (( unresolved_count > 0 )); then
    log_error "$unresolved_count multisig(s) reference signer pubkeys not present in the file:"
    jq -r '.multis[] | select(.unresolved_pubkeys | length > 0)
           | "  - \(.name) (\(.address)): \(.unresolved_pubkeys | join(","))"' <<<"$plan" >&2
    exit 9
  fi
  printf '%s' "$plan"
}

# _mb_targets_from_plan <plan_json>
# Emits a JSON array of targets {kind,name,address,threshold,n,signers,
# signer_names} with multisigs first, then standalones.
_mb_targets_from_plan() {
  jq -c '
    ([ .multis[] | { kind: "multisig",
                     name, address, threshold, n,
                     signers,
                     signer_names: [.signers[].name] } ])
    + ([ .standalones[] | { kind: "single-sig",
                            name: .name, address: .address,
                            threshold: 1, n: 1,
                            signers: [.],
                            signer_names: [.name] } ])
  ' <<<"$1"
}

# _mb_filter_targets <targets_json> <only_name>
# Returns input unchanged if only_name is empty; otherwise filters to exactly
# the one target with that name (or exits 1 if not found).
_mb_filter_targets() {
  local targets="$1" only="$2"
  if [[ -z "$only" ]]; then
    printf '%s' "$targets"
    return 0
  fi
  local out
  out=$(jq --arg n "$only" '[.[] | select(.name == $n)]' <<<"$targets")
  if [[ "$(jq -r 'length' <<<"$out")" == "0" ]]; then
    log_error "--target '$only' not found in mnemonics file"
    exit 1
  fi
  printf '%s' "$out"
}

# Self-send amount and fee used to publish a missing legacy pubkey.
# Keep these as the single source of truth for both the classifier threshold and
# the actual single-sig/multisig self-send broadcasts.
_MB_SELF_SEND_AMOUNT_ULUME=100000
_MB_SELF_SEND_FEE_ULUME=5000

# Minimum spendable ulume required for the target to broadcast its own
# self-send (publishes pubkey). Targets below this threshold MUST be funded by
# --funder even if their TOTAL balance is large (the vesting-locked case).
_MB_MIN_SELF_SEND_SPENDABLE_ULUME=$((_MB_SELF_SEND_AMOUNT_ULUME + _MB_SELF_SEND_FEE_ULUME))

# _mb_classify_target <legacy_addr>
# Probes chain and prints one of:
#   migrated       — migration-record exists
#   ready          — auth pubkey present, no migration record
#   needs-pubkey   — auth account exists (or has balance) but no pubkey
#                    AND has enough SPENDABLE balance to self-fund the
#                    self-send amount plus fee
#   needs-funding  — pubkey missing AND spendable balance is insufficient
#                    for the self-send amount plus fee. This covers two cases:
#                      a) account does not exist on auth and balance==0
#                         (classic fresh foundation account)
#                      b) account exists with non-zero TOTAL balance but
#                         that balance is vesting-locked → spendable below
#                         the self-send amount plus fee
#   unknown        — RPC failure (cannot even read bank balances)
# Plus prints `balance=<ulume>` and `spendable=<ulume>` on subsequent lines.
#
# NOTE: the existing auth_pubkey_type helper hard-exits (exit 2) when the
# account does not exist on auth, which is a LEGITIMATE state for fresh
# foundation accounts. We deliberately do NOT call it here; we probe auth
# ourselves and disambiguate "account not found" from "RPC down" by using
# `bank balances` as the RPC-liveness probe (which succeeds with an empty
# balances array for unknown addresses).
#
# WHY we look at SPENDABLE, not TOTAL: foundation multisigs on the mainnet
# launch bundle are continuous-vesting accounts. Until their end_time they
# carry a large TOTAL balance but spendable=0, so any tx they sign — even a
# self-send to themselves — is rejected at the ante handler with
# "insufficient funds" against the fee. A classifier that only looks at
# TOTAL balance silently routes those targets onto the no-funder code path,
# which then fails at broadcast. Looking at spendable lets the funder bail
# them out instead.
_mb_classify_target() {
  local addr="$1"
  local rec_json balance_json balance spendable_json spendable auth_json pk_type_str

  # 1) already migrated?
  if rec_json=$(lumerad_q_capture evmigration migration-record "$addr" 2>/dev/null); then
    if [[ -n "$rec_json" && "$(jq -r '.record.new_address // empty' <<<"$rec_json")" != "" ]]; then
      printf 'migrated\n'
      printf 'new_address=%s\n' "$(jq -r '.record.new_address' <<<"$rec_json")"
      return 0
    fi
  fi

  # 2) bank balances — also our RPC-liveness probe.
  if ! balance_json=$(lumerad_q_capture bank balances "$addr" 2>/dev/null); then
    printf 'unknown\n'
    printf 'balance=0\n'
    printf 'spendable=0\n'
    return 0
  fi
  balance=$(jq -r '[.balances[]? | select(.denom == "ulume").amount | tonumber] | add // 0' <<<"$balance_json")

  # 2b) spendable balance — same denom. For vesting-locked accounts this is
  # strictly less than balance until end_time. We tolerate RPC failure here
  # (treat as spendable=0 so we route to needs-funding, which is the safe
  # default — the funder will top up).
  spendable=0
  if spendable_json=$(lumerad_q_capture bank spendable-balances "$addr" 2>/dev/null); then
    spendable=$(jq -r '[.balances[]? | select(.denom == "ulume").amount | tonumber] | add // 0' <<<"$spendable_json")
  fi

  # 3) auth account — may legitimately be absent for a fresh foundation
  # account. We treat that as "not on auth" (acct_known=0), NOT as RPC failure.
  local acct_known=0
  pk_type_str=""
  if auth_json=$(lumerad_q_capture auth account "$addr" 2>/dev/null) && [[ -n "$auth_json" ]]; then
    acct_known=1
    # Walk the object for the first pub_key/pubkey @type we find. Matches the
    # priority-order traversal in auth_pubkey_type, without its hard-exit.
    pk_type_str=$(jq -r '
      [.. | objects | (.pub_key // .pubkey) | objects | .["@type"] // empty]
      | map(select(. != "")) | first // ""' <<<"$auth_json" 2>/dev/null || echo "")
  fi

  case "$pk_type_str" in
    *LegacyAminoPubKey*|*secp256k1*)
      printf 'ready\n'
      ;;
    *)
      # No pubkey on chain. Decide between needs-pubkey (self-fund) and
      # needs-funding (caller must provide --funder) based on whether the
      # target can pay its own self-send amount plus fee from SPENDABLE balance.
      #
      # An account is self-fundable iff:
      #   - auth knows it OR total balance > 0  (so it can plausibly broadcast)
      #   - AND spendable >= _MB_MIN_SELF_SEND_SPENDABLE_ULUME
      # Otherwise we hand it to the funder path.
      if { (( acct_known == 1 )) || [[ "$balance" != "0" ]]; } \
         && (( spendable >= _MB_MIN_SELF_SEND_SPENDABLE_ULUME )); then
        printf 'needs-pubkey\n'
      else
        printf 'needs-funding\n'
      fi
      ;;
  esac
  printf 'balance=%s\n' "$balance"
  printf 'spendable=%s\n' "$spendable"
}

###############################################################################
# report subcommand — Phase A
###############################################################################
_mb_report() {
  local mnemonics_file="" plan_out=""
  while (( $# > 0 )); do
    case "$1" in
      --mnemonics)   _require_value "$1" "$#" "${2-}"; mnemonics_file="$2"; shift 2 ;;
      --plan-out)    _require_value "$1" "$#" "${2-}"; plan_out="$2"; shift 2 ;;
      -h|--help)
        cat >&2 <<'R_USAGE'
Usage: migrate-batch.sh report --mnemonics <file> [--plan-out <file>]

Phase A — offline classification. No chain calls, no signing.

Required:
  --mnemonics <file>    Path to the operator's JSON mnemonics file.

Optional:
  --plan-out <file>     Write a machine-readable JSON plan.
R_USAGE
        exit 0 ;;
      *) log_error "report: unknown flag: $1"; exit 1 ;;
    esac
  done

  if [[ -z "$mnemonics_file" ]]; then
    log_error "report: --mnemonics is required"; exit 1
  fi
  if [[ ! -r "$mnemonics_file" ]]; then
    log_error "report: cannot read mnemonics file: $mnemonics_file"; exit 1
  fi
  require_jq

  local plan targets
  plan=$(_mb_load_plan "$mnemonics_file")
  targets=$(_mb_targets_from_plan "$plan")

  local n_multisig_signers n_multi n_stand n_total
  n_multisig_signers=$(jq -r '[.multis[].signers[]] | unique_by(.pubkey_b64) | length' <<<"$plan")
  n_multi=$(jq -r '.multis | length' <<<"$plan")
  n_stand=$(jq -r '.standalones | length' <<<"$plan")
  n_total=$(( n_multi + n_stand ))

  log_info "=== Mnemonics file: $mnemonics_file ==="
  printf 'Totals:\n'
  printf '  multisig signer keys:     %s\n' "$n_multisig_signers"
  printf '  multisig accounts:        %s\n' "$n_multi"
  printf '  standalone single-sigs:   %s\n' "$n_stand"
  printf '  total migration targets:  %s\n' "$n_total"
  printf '\n'
  printf 'Multisig threshold breakdown: %s\n' \
    "$(jq -r '.multis | group_by(.threshold) | map("T=\(.[0].threshold) x \(length)") | join(", ")' <<<"$plan")"
  printf 'Multisig signer-count breakdown: %s\n' \
    "$(jq -r '.multis | group_by(.n) | map("N=\(.[0].n) x \(length)") | join(", ")' <<<"$plan")"

  log_info ""
  log_info "=== Targets ==="
  jq -r '
    .[] |
    if .kind == "multisig" then
      "  [\(.kind)] \(.name)\n" +
      "    address:  \(.address)\n" +
      "    K-of-N:   \(.threshold)-of-\(.n)\n" +
      "    signers (canonical pubkey order, NOT name order):\n" +
      "      " + (.signer_names | join(", "))
    else
      "  [\(.kind)] \(.name)\n" +
      "    address:  \(.address)"
    end
  ' <<<"$targets"

  if [[ -n "$plan_out" ]]; then
    jq -n --argjson plan "$plan" --argjson targets "$targets" \
      '{totals: {multisig_signer_keys: ($plan.multis | [.[].signers[]] | unique_by(.pubkey_b64) | length),
                 multis: ($plan.multis | length),
                 standalone_singles: ($plan.standalones | length),
                 targets: (($plan.multis | length) + ($plan.standalones | length))},
        targets: $targets}' > "$plan_out"
    log_info ""
    log_info "Machine-readable plan written to: $plan_out"
  fi
}

###############################################################################
# status subcommand — Phase B
#
# For each target, classify on-chain state. Read-only.
###############################################################################
_mb_status() {
  local mnemonics_file="" node="${LUMERA_NODE:-tcp://localhost:26657}"
  local chain_id="${LUMERA_CHAIN_ID:-${CHAIN_ID:-}}" binary="lumerad"
  local only_target=""
  while (( $# > 0 )); do
    case "$1" in
      --mnemonics)   _require_value "$1" "$#" "${2-}"; mnemonics_file="$2"; shift 2 ;;
      --node)        _require_value "$1" "$#" "${2-}"; node="$2"; shift 2 ;;
      --chain-id)    _require_value "$1" "$#" "${2-}"; chain_id="$2"; shift 2 ;;
      --binary)      _require_value "$1" "$#" "${2-}"; binary="$2"; shift 2 ;;
      --target)      _require_value "$1" "$#" "${2-}"; only_target="$2"; shift 2 ;;
      -h|--help)
        cat >&2 <<'S_USAGE'
Usage: migrate-batch.sh status --mnemonics <file> \
  [--node <url>] [--chain-id <id>] [--target <name>] [--binary <path>]

Phase B — per-target chain-state probe. READ-ONLY. No signing, no broadcast.

Per target, classifies into one of:
  migrated       — already migrated, will be skipped by `execute`
  ready          — pubkey on chain, ready to migrate
  needs-pubkey   — has enough spendable balance for self-send amount + fee
                   but no pubkey on chain (will self-send)
  needs-funding  — pubkey missing and spendable balance < self-send amount + fee
                   (zero-balance OR vesting-locked; will need --funder during execute)
  unknown        — RPC failure (re-check)

Required:
  --mnemonics <file>    Mnemonics file (used only for the target list).

Optional:
  --node <url>          Default: $LUMERA_NODE or tcp://localhost:26657
  --chain-id <id>       Auto-detected from RPC if not provided.
  --target <name>       Probe only the named target.
  --binary <path>       Default: lumerad
S_USAGE
        exit 0 ;;
      *) log_error "status: unknown flag: $1"; exit 1 ;;
    esac
  done

  if [[ -z "$mnemonics_file" ]]; then
    log_error "status: --mnemonics is required"; exit 1
  fi
  require_jq
  # shellcheck disable=SC2034
  BIN="$binary"
  # shellcheck disable=SC2034
  NODE="$node"
  # shellcheck disable=SC2034
  CHAIN_ID="$chain_id"
  # shellcheck disable=SC2034
  KEYRING_BACKEND="test"
  # shellcheck disable=SC2034
  KEYRING_DIR=""
  # shellcheck disable=SC2034
  HOME_DIR=""
  require_binary
  resolve_chain_id

  local plan targets
  plan=$(_mb_load_plan "$mnemonics_file")
  targets=$(_mb_targets_from_plan "$plan")
  targets=$(_mb_filter_targets "$targets" "$only_target")

  local count_migrated=0 count_ready=0 count_needs_pubkey=0 count_needs_funding=0 count_unknown=0
  log_info "=== Status (chain: $CHAIN_ID, node: $NODE) ==="
  local row name addr kind status_lines status balance spendable extra
  local n_rows
  n_rows=$(jq -r 'length' <<<"$targets")
  local i=0
  while (( i < n_rows )); do
    row=$(jq -c ".[$i]" <<<"$targets")
    name=$(jq -r '.name'    <<<"$row")
    addr=$(jq -r '.address' <<<"$row")
    kind=$(jq -r '.kind'    <<<"$row")
    status_lines=$(_mb_classify_target "$addr")
    status=$(awk 'NR==1' <<<"$status_lines")
    # Parse extras by key, not by line number — _mb_classify_target emits
    # `balance=...` and `spendable=...` for non-migrated statuses and
    # `new_address=...` when migrated. Positional parsing would mis-label.
    balance=$(awk -F= 'NR>1 && $1=="balance" {print $2}' <<<"$status_lines")
    spendable=$(awk -F= 'NR>1 && $1=="spendable" {print $2}' <<<"$status_lines")
    extra=""
    if [[ "$status" == "migrated" ]]; then
      extra=" -> $(awk -F= 'NR>1 && $1=="new_address" {print $2}' <<<"$status_lines")"
      balance=""
      spendable=""
    fi
    case "$status" in
      migrated)       count_migrated=$((count_migrated+1)) ;;
      ready)          count_ready=$((count_ready+1)) ;;
      needs-pubkey)   count_needs_pubkey=$((count_needs_pubkey+1)) ;;
      needs-funding)  count_needs_funding=$((count_needs_funding+1)) ;;
      *)              count_unknown=$((count_unknown+1)) ;;
    esac
    # Surface spendable alongside balance so operators understand why a
    # vesting-locked account (balance>0, spendable=0) is needs-funding.
    printf '  %-12s [%-10s] %-30s %s%s%s%s\n' \
      "$status" "$kind" "$name" "$addr" \
      "${balance:+  balance=${balance}ulume}" \
      "${spendable:+  spendable=${spendable}ulume}" \
      "$extra"
    i=$((i+1))
  done
  printf '\nSummary: migrated=%s ready=%s needs-pubkey=%s needs-funding=%s unknown=%s\n' \
    "$count_migrated" "$count_ready" "$count_needs_pubkey" "$count_needs_funding" "$count_unknown"
  printf 'Note: needs-funding includes targets where spendable=0 even when balance>0 (vesting-locked).\n'
}

###############################################################################
# execute subcommand — Phase C
#
# Per target, lifecycle:
#   1. classify on-chain state
#   2. set up per-target ephemeral keyring (mode 0700, trap-cleanup)
#   3. import all signer mnemonics (legacy 118/secp256k1 + new 60/eth_secp256k1)
#   4. reconstruct legacy multisig in keyring; ASSERT address matches file
#   5. reconstruct new multisig in keyring
#   6. fund from --funder if balance==0 (multisig case)
#   7. self-send via legacy keys if pubkey missing on chain
#   8. delegate to migrate-multisig.sh / migrate-account.sh
#   9. verify via evmigration migration-record
#  10. clean up keyring
###############################################################################

# Module-level state for the per-target ephemeral keyring cleanup trap.
_MB_EPHEMERAL_DIR=""

# Module-level state for the optional persistent run log (--log-file).
# When _MB_LOG_FILE is non-empty, _mb_log_event appends one JSONL record per
# milestone (batch_start, target_start, target_done, etc.). The path is
# resolved to absolute at execute() entry; the file is created mode 0600 and
# never rotated by this script — operators rotate / archive themselves.
_MB_LOG_FILE=""
_MB_BATCH_ID=""

# _mb_log_event <event-name> [key value]...
#
# Append a single JSON object as one line to $_MB_LOG_FILE. No-op if the
# operator did not pass --log-file. Uses jq -nc for safe JSON escaping; do
# NOT replace with hand-rolled printf — operator-supplied values (addresses,
# tx hashes, error strings) can contain characters that would break a
# naive concatenation.
#
# Reserved keys ALWAYS present: ts (UTC ISO-8601), batch_id, event.
# Caller-provided keys are placed alongside; keys must be valid jq variable
# names (matches /^[a-zA-Z_][a-zA-Z0-9_]*$/), which this script always
# satisfies.
_mb_log_event() {
  [[ -z "$_MB_LOG_FILE" ]] && return 0
  local event="$1"; shift
  local args=(--arg ts "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
              --arg batch "$_MB_BATCH_ID"
              --arg event "$event")
  # Dynamically build a jq object expression from the remaining key/value
  # pairs. Keys map 1:1 to --arg names; missing values default to "".
  local pairs='{}'
  while (( $# >= 2 )); do
    args+=(--arg "$1" "$2")
    pairs="$pairs + {\"$1\": \$$1}"
    shift 2
  done
  # Best-effort write; do not let log-file IO errors abort the batch.
  jq -nc "${args[@]}" "{ts: \$ts, batch_id: \$batch, event: \$event} + $pairs" \
    >>"$_MB_LOG_FILE" 2>/dev/null || \
      log_warn "log-file write failed (continuing): $_MB_LOG_FILE"
}

_mb_cleanup_ephemeral() {
  if [[ -n "$_MB_EPHEMERAL_DIR" && -d "$_MB_EPHEMERAL_DIR" ]]; then
    # Defensive: refuse to nuke anything outside /tmp or our intended dir name.
    case "$_MB_EPHEMERAL_DIR" in
      */migrate-batch-keyring-*) rm -rf -- "$_MB_EPHEMERAL_DIR" ;;
      *) log_warn "refusing to clean suspicious ephemeral dir: $_MB_EPHEMERAL_DIR" ;;
    esac
  fi
  _MB_EPHEMERAL_DIR=""
}

# _mb_make_ephemeral_keyring -> sets _MB_EPHEMERAL_DIR to a new mode-0700 dir.
_mb_make_ephemeral_keyring() {
  local base="${TMPDIR:-/tmp}"
  _MB_EPHEMERAL_DIR=$(mktemp -d "${base}/migrate-batch-keyring-XXXXXX")
  chmod 0700 "$_MB_EPHEMERAL_DIR"
}

# _mb_import_signer <name> <mnemonic>
# Imports the signer's mnemonic as TWO keyring entries inside _MB_EPHEMERAL_DIR:
#   legacy-<name>  (coin-type 118, secp256k1)
#   new-<name>     (coin-type 60,  eth_secp256k1)
_mb_import_signer() {
  local name="$1" mnemonic="$2"
  local mfile
  mfile=$(mktemp "${_MB_EPHEMERAL_DIR}/sig-XXXXXX.seed")
  chmod 0600 "$mfile"
  printf '%s' "$mnemonic" >"$mfile"
  # Configure import_from_mnemonic to land in the ephemeral keyring.
  # shellcheck disable=SC2034
  KEYRING_BACKEND="test"
  # shellcheck disable=SC2034
  KEYRING_DIR="$_MB_EPHEMERAL_DIR"
  # shellcheck disable=SC2034
  HOME_DIR=""
  import_from_mnemonic "$mfile" "legacy-${name}" "new-${name}"
  rm -f -- "$mfile"
}

# _mb_add_multisig <name> <members_csv> <threshold>
# Creates a multisig key inside the ephemeral keyring. Idempotent: if a key
# of that name already exists it's removed first.
#
# CRITICAL: --nosort is mandatory. Without it, `lumerad keys add --multisig`
# sorts sub-keys by ADDRESS before assembling the multisig pubkey. That
# silently produces a DIFFERENT multisig address when the input order does
# not coincide with address-sort order, AND silently rearranges the new
# multisig's public_keys[] in a different order than the legacy multisig's
# public_keys[]. The latter breaks `migrate-multisig.sh sign` with
# "legacy key X is signer index N, but new key X is signer index M" because
# the sign-proof CLI enforces that the same logical signer occupies the same
# index on both sides.
#
# The output.json file's public_keys[] order is canonical. We must preserve
# that exact order on BOTH the legacy and the new multisigs, which means
# --nosort on every `keys add --multisig` invocation.
_mb_add_multisig() {
  local name="$1" members_csv="$2" threshold="$3"
  "$BIN" keys delete "$name" --keyring-backend test --keyring-dir "$_MB_EPHEMERAL_DIR" -y >/dev/null 2>&1 || true
  "$BIN" keys add "$name" \
    --multisig "$members_csv" \
    --multisig-threshold "$threshold" \
    --nosort \
    --keyring-backend test \
    --keyring-dir "$_MB_EPHEMERAL_DIR" >/dev/null
}

# _mb_keyring_addr <name>
_mb_keyring_addr() {
  local name="$1"
  "$BIN" keys show "$name" -a \
    --keyring-backend test --keyring-dir "$_MB_EPHEMERAL_DIR" 2>/dev/null
}

# _mb_send_with_funder <funder_key> <to_addr> <amount>
# Operator's keyring; one tx, wait for inclusion.
#
# NOTE: the funder-keyring flags are passed via an array (not ${VAR:+...}
# expansion) because this script runs with IFS=$'\n\t' set at the top,
# under which `${VAR:+--flag "$VAR"}` does NOT word-split on spaces and
# would produce a single mangled argv element like `--keyring-dir /path`.
# Do NOT collapse this into shell parameter expansion.
_mb_send_with_funder() {
  local funder="$1" to="$2" amount="$3"
  local funder_extra=()
  [[ -n "$_MB_FUNDER_KEYRING_DIR" ]] && funder_extra+=(--keyring-dir "$_MB_FUNDER_KEYRING_DIR")
  [[ -n "$_MB_FUNDER_HOME"        ]] && funder_extra+=(--home        "$_MB_FUNDER_HOME")

  local out tx_hash
  if ! out=$("$BIN" tx bank send "$funder" "$to" "$amount" \
              --node "$NODE" --chain-id "$CHAIN_ID" \
              --keyring-backend "$_MB_FUNDER_KEYRING_BACKEND" \
              "${funder_extra[@]}" \
              --fees 5000ulume --gas auto --gas-adjustment 1.3 \
              --output json -y); then
    log_error "funder send failed (broadcast exited non-zero)"
    return 1
  fi
  tx_hash=$(assert_broadcast_accepted "$out")
  wait_for_tx "$tx_hash"
}

# _mb_multisig_self_send <multisig_name> <multisig_addr> <signers_csv> <threshold>
# Performs a multisig self-send (multisig -> multisig) to publish the multisig's
# pubkey on chain. All signer keys must already be present in the ephemeral
# keyring under legacy-* names.
_mb_multisig_self_send() {
  local multi_name="$1" multi_addr="$2" signers_csv="$3" threshold="$4"
  local workdir
  workdir=$(mktemp -d "${_MB_EPHEMERAL_DIR}/selfsend-XXXXXX")
  local unsigned="${workdir}/unsigned.json"

  # 1. Unsigned tx (self-send to publish pubkey). The first
  # positional argument is the FROM key NAME (looked up in the keyring), not
  # an address. lumerad rejects an address here as "no key name or address
  # provided; have you forgotten the --from flag?".
  "$BIN" tx bank send "$multi_name" "$multi_addr" "${_MB_SELF_SEND_AMOUNT_ULUME}ulume" \
    --node "$NODE" --chain-id "$CHAIN_ID" \
    --fees "${_MB_SELF_SEND_FEE_ULUME}ulume" --gas auto --gas-adjustment 1.3 \
    --keyring-backend test --keyring-dir "$_MB_EPHEMERAL_DIR" \
    --generate-only --output json >"$unsigned"

  # 2. K partial signatures, using first K signers from the canonical list.
  local IFS_save="$IFS"; IFS=','
  # shellcheck disable=SC2206
  local signers=( $signers_csv )
  IFS="$IFS_save"
  local sigfiles=()
  local i=0
  while (( i < threshold )); do
    local signer="${signers[$i]}"
    local sigfile="${workdir}/sig-${i}.json"
    "$BIN" tx sign "$unsigned" \
      --from "legacy-${signer}" \
      --multisig "$multi_addr" \
      --node "$NODE" --chain-id "$CHAIN_ID" \
      --keyring-backend test --keyring-dir "$_MB_EPHEMERAL_DIR" \
      --output-document "$sigfile" >/dev/null
    sigfiles+=("$sigfile")
    i=$((i+1))
  done

  # 3. multisign
  local signed="${workdir}/signed.json"
  "$BIN" tx multisign "$unsigned" "$multi_name" "${sigfiles[@]}" \
    --node "$NODE" --chain-id "$CHAIN_ID" \
    --keyring-backend test --keyring-dir "$_MB_EPHEMERAL_DIR" \
    --output-document "$signed" >/dev/null

  # 4. broadcast + wait
  local out tx_hash
  if ! out=$("$BIN" tx broadcast "$signed" --node "$NODE" --output json); then
    log_error "multisig self-send broadcast failed (broadcast exited non-zero)"
    return 1
  fi
  tx_hash=$(assert_broadcast_accepted "$out")
  wait_for_tx "$tx_hash"
}

# _mb_execute_one <target_json>
# Drives the full lifecycle for one target. Returns 0 on success or already-
# migrated; non-zero on failure (caller decides whether to stop or continue).
_mb_execute_one() {
  local row="$1"
  local name addr kind threshold
  name=$(jq -r '.name'      <<<"$row")
  addr=$(jq -r '.address'   <<<"$row")
  kind=$(jq -r '.kind'      <<<"$row")
  threshold=$(jq -r '.threshold' <<<"$row")

  log_info ""
  log_info "──── target: $name ($kind) ────"
  log_info "  legacy address: $(legacy_value "$addr")"
  _mb_log_event target_start target "$name" kind "$kind" address "$addr"

  # ---- 1. classify -----------------------------------------------------------
  # _mb_classify_target emits status on line 1, then zero or more `key=value`
  # lines. For status=migrated the extra line is `new_address=...` (no balance).
  # For other statuses the extra line is `balance=...`. Parse by key so we
  # don't mis-label new_address as balance in the operator log + JSONL audit.
  local cls cls_status balance spendable new_addr_cls
  cls=$(_mb_classify_target "$addr")
  cls_status=$(awk 'NR==1' <<<"$cls")
  balance=$(awk -F= 'NR>1 && $1=="balance" {print $2}' <<<"$cls")
  spendable=$(awk -F= 'NR>1 && $1=="spendable" {print $2}' <<<"$cls")
  new_addr_cls=$(awk -F= 'NR>1 && $1=="new_address" {print $2}' <<<"$cls")

  if [[ "$cls_status" == "migrated" ]]; then
    log_info "  on-chain status: migrated  (new_address=$(new_value "$new_addr_cls"))"
    _mb_log_event classify target "$name" status "$cls_status" new_address "$new_addr_cls"
    log_info "  already migrated; skipping"
    _mb_log_event target_done target "$name" outcome skipped_already_migrated
    return 0
  fi
  log_info "  on-chain status: $cls_status  (balance=${balance}ulume spendable=${spendable:-0}ulume)"
  _mb_log_event classify target "$name" status "$cls_status" balance "$balance" spendable "${spendable:-0}"

  if [[ "$cls_status" == "unknown" ]]; then
    log_error "  could not classify on-chain state; check RPC and re-run"
    _mb_log_event target_done target "$name" outcome failed reason rpc_unknown
    return 2
  fi
  if [[ "$cls_status" == "needs-funding" && -z "${_MB_FUNDER}" ]]; then
    log_error "  needs --funder + --top-up-amount to seed fees; aborting target"
    _mb_log_event target_done target "$name" outcome failed reason needs_funder_not_provided
    return 1
  fi

  # ---- 2. ephemeral keyring --------------------------------------------------
  _mb_make_ephemeral_keyring
  log_info "  ephemeral keyring: $_MB_EPHEMERAL_DIR"
  _mb_log_event keyring_setup target "$name" ephemeral_dir "$_MB_EPHEMERAL_DIR"

  # ---- 3. import all signer mnemonics ----------------------------------------
  local signer_names=()
  local i=0
  local n_sig
  n_sig=$(jq -r '.signers | length' <<<"$row")
  while (( i < n_sig )); do
    local sname smnem
    sname=$(jq -r ".signers[$i].name"     <<<"$row")
    smnem=$(jq -r ".signers[$i].mnemonic" <<<"$row")
    if [[ -z "$smnem" ]]; then
      log_error "  signer '$sname' has empty mnemonic; aborting target"
      return 1
    fi
    _mb_import_signer "$sname" "$smnem"
    signer_names+=("$sname")
    i=$((i+1))
  done
  log_info "  imported ${#signer_names[@]} signer key(s) into ephemeral keyring"

  # ---- 4-5. reconstruct legacy + new multisig in keyring (multi targets) -----
  local legacy_key_name new_key_name
  if [[ "$kind" == "multisig" ]]; then
    local legacy_members_csv new_members_csv
    legacy_members_csv=$(printf 'legacy-%s,' "${signer_names[@]}"); legacy_members_csv="${legacy_members_csv%,}"
    new_members_csv=$(printf 'new-%s,' "${signer_names[@]}");       new_members_csv="${new_members_csv%,}"

    legacy_key_name="legacy-multi-${name}"
    new_key_name="new-multi-${name}"
    _mb_add_multisig "$legacy_key_name" "$legacy_members_csv" "$threshold"
    _mb_add_multisig "$new_key_name"    "$new_members_csv"    "$threshold"

    local rebuilt_legacy_addr rebuilt_new_addr
    rebuilt_legacy_addr=$(_mb_keyring_addr "$legacy_key_name")
    rebuilt_new_addr=$(_mb_keyring_addr   "$new_key_name")

    if [[ "$rebuilt_legacy_addr" != "$addr" ]]; then
      log_error "  reconstructed legacy multisig address mismatch:"
      log_error "    file:    $addr"
      log_error "    rebuilt: $rebuilt_legacy_addr"
      log_error "  this means signer order or threshold is wrong; aborting target"
      _mb_log_event target_done target "$name" outcome failed \
        reason legacy_multisig_address_mismatch \
        file_address "$addr" rebuilt_address "$rebuilt_legacy_addr"
      return 1
    fi
    log_info "  reconstructed legacy multisig: $(legacy_value "$rebuilt_legacy_addr")  ✓ matches file"
    log_info "  reconstructed new multisig:    $(new_value    "$rebuilt_new_addr")"
    _mb_log_event reconstructed target "$name" \
      legacy_address "$rebuilt_legacy_addr" new_address "$rebuilt_new_addr"
  else
    # Standalone single-sig — the single signer's two variants ARE the
    # legacy and new keys.
    legacy_key_name="legacy-${signer_names[0]}"
    new_key_name="new-${signer_names[0]}"
    local rebuilt_legacy_addr
    rebuilt_legacy_addr=$(_mb_keyring_addr "$legacy_key_name")
    if [[ "$rebuilt_legacy_addr" != "$addr" ]]; then
      log_error "  reconstructed legacy address mismatch:"
      log_error "    file:    $addr"
      log_error "    rebuilt: $rebuilt_legacy_addr"
      log_error "  aborting target"
      _mb_log_event target_done target "$name" outcome failed \
        reason legacy_singlesig_address_mismatch \
        file_address "$addr" rebuilt_address "$rebuilt_legacy_addr"
      return 1
    fi
    log_info "  reconstructed legacy single-sig: $(legacy_value "$rebuilt_legacy_addr")  ✓ matches file"
    _mb_log_event reconstructed target "$name" legacy_address "$rebuilt_legacy_addr"
  fi

  if (( _MB_DRY_RUN == 1 )); then
    log_info "  --dry-run: stopping before any tx"
    _mb_log_event target_done target "$name" outcome dry_run_complete
    return 0
  fi

  # ---- 6. fund if needed -----------------------------------------------------
  if [[ "$cls_status" == "needs-funding" ]]; then
    log_info "  funding $(legacy_value "$addr") with ${_MB_TOP_UP_AMOUNT} from $(legacy_value "$_MB_FUNDER")"
    _mb_log_event funding_start target "$name" funder "$_MB_FUNDER" amount "$_MB_TOP_UP_AMOUNT"
    if ! _mb_send_with_funder "$_MB_FUNDER" "$addr" "$_MB_TOP_UP_AMOUNT"; then
      log_error "  funding failed"
      _mb_log_event target_done target "$name" outcome failed reason funding_failed
      return 1
    fi
    _mb_log_event funding_done target "$name"
    # Re-classify so we proceed to self-send.
    cls_status="needs-pubkey"
  fi

  # ---- 7. self-send to publish pubkey ----------------------------------------
  if [[ "$cls_status" == "needs-pubkey" ]]; then
    _mb_log_event self_send_start target "$name" mode "$kind"
    if [[ "$kind" == "multisig" ]]; then
      local signers_csv
      signers_csv=$(IFS=','; printf '%s' "${signer_names[*]}")
      log_info "  publishing multisig pubkey via self-send (K=$threshold signers)"
      if ! _mb_multisig_self_send "$legacy_key_name" "$addr" "$signers_csv" "$threshold"; then
        log_error "  multisig self-send failed"
        _mb_log_event target_done target "$name" outcome failed reason multisig_self_send_failed
        return 1
      fi
    else
      log_info "  publishing single-sig pubkey via self-send"
      local out tx_hash
      if ! out=$("$BIN" tx bank send "$legacy_key_name" "$addr" "${_MB_SELF_SEND_AMOUNT_ULUME}ulume" \
                   --node "$NODE" --chain-id "$CHAIN_ID" \
                   --fees "${_MB_SELF_SEND_FEE_ULUME}ulume" --gas auto --gas-adjustment 1.3 \
                   --keyring-backend test --keyring-dir "$_MB_EPHEMERAL_DIR" \
                   --output json -y); then
        log_error "  single-sig self-send broadcast exited non-zero"
        _mb_log_event target_done target "$name" outcome failed reason singlesig_self_send_failed
        return 1
      fi
      tx_hash=$(assert_broadcast_accepted "$out") || {
        _mb_log_event target_done target "$name" outcome failed reason singlesig_self_send_rejected
        return 1
      }
      wait_for_tx "$tx_hash" || {
        _mb_log_event target_done target "$name" outcome failed reason singlesig_self_send_not_included
        return 1
      }
    fi
    _mb_log_event self_send_done target "$name"
  fi

  # ---- 8. delegate to existing migrate-* scripts -----------------------------
  #
  # IMPORTANT: every sub-script call below is guarded so that --continue-on-error
  # at the batch level can actually work. `set -e` would otherwise abort the
  # entire process the moment migrate-multisig.sh / migrate-account.sh exit
  # non-zero (e.g. on a transient broadcast RPC drop or the documented exit-8
  # "seed the pubkey first" race). Do NOT collapse these into bare invocations.
  #
  # NOTE on `submit --yes`: foundation accounts in this driver are NEVER
  # validators (validators have their own migration path via migrate-validator.sh
  # and require explicit --i-have-stopped-the-node). For claim-multisig submits
  # the --yes flag is safe and matches the batch-level confirmation contract.
  if [[ "$kind" == "multisig" ]]; then
    log_info "  invoking migrate-multisig.sh (generate -> sign x$threshold -> combine -> submit)"
    _mb_log_event ceremony_start target "$name" path multisig threshold "$threshold"
    local workdir proof partials=() final_tx
    workdir=$(mktemp -d "${_MB_EPHEMERAL_DIR}/migrate-XXXXXX")
    proof="${workdir}/proof.json"
    final_tx="${workdir}/tx.json"

    # generate
    if ! "${SCRIPT_DIR}/migrate-multisig.sh" generate \
           --legacy "$addr" \
           --new-key "$new_key_name" \
           --chain-id "$CHAIN_ID" --node "$NODE" \
           --keyring-backend test --keyring-dir "$_MB_EPHEMERAL_DIR" \
           --out "$proof" --binary "$BIN"; then
      log_error "  migrate-multisig.sh generate failed"
      _mb_log_event target_done target "$name" outcome failed reason migrate_multisig_generate_failed
      return 1
    fi

    # sign x threshold (using first K signers, both legacy and new sides)
    local j=0
    while (( j < threshold )); do
      local sname="${signer_names[$j]}"
      local partial="${workdir}/partial-${j}.json"
      if ! "${SCRIPT_DIR}/migrate-multisig.sh" sign "$proof" \
             --from "legacy-${sname}" \
             --new-key "new-${sname}" \
             --chain-id "$CHAIN_ID" --node "$NODE" \
             --keyring-backend test --keyring-dir "$_MB_EPHEMERAL_DIR" \
             --out "$partial" --binary "$BIN"; then
        log_error "  migrate-multisig.sh sign failed (signer #$j: $sname)"
        _mb_log_event target_done target "$name" outcome failed \
          reason migrate_multisig_sign_failed signer_index "$j" signer_name "$sname"
        return 1
      fi
      partials+=("$partial")
      j=$((j+1))
    done

    # combine
    if ! "${SCRIPT_DIR}/migrate-multisig.sh" combine "${partials[@]}" \
           --out "$final_tx" --binary "$BIN"; then
      log_error "  migrate-multisig.sh combine failed"
      _mb_log_event target_done target "$name" outcome failed reason migrate_multisig_combine_failed
      return 1
    fi

    # submit
    if ! "${SCRIPT_DIR}/migrate-multisig.sh" submit "$final_tx" \
           --chain-id "$CHAIN_ID" --node "$NODE" \
           --keyring-backend test --keyring-dir "$_MB_EPHEMERAL_DIR" \
           --binary "$BIN" --yes; then
      log_error "  migrate-multisig.sh submit failed"
      _mb_log_event target_done target "$name" outcome failed reason migrate_multisig_submit_failed
      return 1
    fi
  else
    log_info "  invoking migrate-account.sh"
    _mb_log_event ceremony_start target "$name" path single_sig
    if ! "${SCRIPT_DIR}/migrate-account.sh" "$legacy_key_name" "$new_key_name" \
           --chain-id "$CHAIN_ID" --node "$NODE" \
           --keyring-backend test --keyring-dir "$_MB_EPHEMERAL_DIR" \
           --binary "$BIN" --yes; then
      log_error "  migrate-account.sh failed"
      _mb_log_event target_done target "$name" outcome failed reason migrate_account_failed
      return 1
    fi
  fi

  # ---- 9. verify -------------------------------------------------------------
  local rec_json rec_new
  if ! rec_json=$(lumerad_q_capture evmigration migration-record "$addr"); then
    log_error "  post-check: could not query migration-record"
    _mb_log_event target_done target "$name" outcome failed reason post_check_query_failed
    return 1
  fi
  rec_new=$(jq -r '.record.new_address // empty' <<<"$rec_json")
  if [[ -z "$rec_new" ]]; then
    log_error "  post-check: no migration record for $(legacy_value "$addr")"
    _mb_log_event target_done target "$name" outcome failed reason post_check_no_record
    return 1
  fi
  log_info "  ✓ migrated: $(legacy_value "$addr") -> $(new_value "$rec_new")"
  _mb_log_event target_done target "$name" outcome success new_address "$rec_new"
  return 0
}

_mb_execute() {
  local mnemonics_file="" node="${LUMERA_NODE:-tcp://localhost:26657}"
  local chain_id="${LUMERA_CHAIN_ID:-${CHAIN_ID:-}}" binary="lumerad"
  local only_target=""
  _MB_DRY_RUN=0
  local yes=0
  _MB_FUNDER=""
  # Must cover the self-send amount + its fee with headroom.
  # 200000ulume = self-send + fee + ~2x headroom. Lower defaults will make the
  # downstream self-send fail with insufficient funds on a freshly-funded target.
  _MB_TOP_UP_AMOUNT="200000ulume"
  _MB_FUNDER_KEYRING_BACKEND="test"
  _MB_FUNDER_KEYRING_DIR=""
  _MB_FUNDER_HOME=""
  _MB_LOG_FILE=""
  _MB_BATCH_ID=""
  local continue_on_error=0

  while (( $# > 0 )); do
    case "$1" in
      --mnemonics)               _require_value "$1" "$#" "${2-}"; mnemonics_file="$2"; shift 2 ;;
      --node)                    _require_value "$1" "$#" "${2-}"; node="$2"; shift 2 ;;
      --chain-id)                _require_value "$1" "$#" "${2-}"; chain_id="$2"; shift 2 ;;
      --binary)                  _require_value "$1" "$#" "${2-}"; binary="$2"; shift 2 ;;
      --target)                  _require_value "$1" "$#" "${2-}"; only_target="$2"; shift 2 ;;
      --funder)                  _require_value "$1" "$#" "${2-}"; _MB_FUNDER="$2"; shift 2 ;;
      --top-up-amount)           _require_value "$1" "$#" "${2-}"; _MB_TOP_UP_AMOUNT="$2"; shift 2 ;;
      --funder-keyring-backend)  _require_value "$1" "$#" "${2-}"; _MB_FUNDER_KEYRING_BACKEND="$2"; shift 2 ;;
      --funder-keyring-dir)      _require_value "$1" "$#" "${2-}"; _MB_FUNDER_KEYRING_DIR="$2"; shift 2 ;;
      --funder-home)             _require_value "$1" "$#" "${2-}"; _MB_FUNDER_HOME="$2"; shift 2 ;;
      --log-file)                _require_value "$1" "$#" "${2-}"; _MB_LOG_FILE="$2"; shift 2 ;;
      --dry-run)                 _MB_DRY_RUN=1; shift ;;
      --yes|-y)                  yes=1; shift ;;
      --continue-on-error)       continue_on_error=1; shift ;;
      -h|--help)
        cat >&2 <<'E_USAGE'
Usage: migrate-batch.sh execute --mnemonics <file> \
  [--node <url>] [--chain-id <id>] [--binary <path>] \
  [--target <name>] \
  [--funder <key>] [--top-up-amount <coins>] \
  [--funder-keyring-backend <b>] [--funder-keyring-dir <dir>] [--funder-home <dir>] \
  [--log-file <path>] \
  [--dry-run] [--yes] [--continue-on-error]

Phase C — run the migration lifecycle for each target.

Per target:
  1) classify on-chain state (skip if already migrated)
  2) set up an ephemeral mode-0700 keyring (wiped on exit)
  3) import each signer's mnemonic as legacy (118/secp256k1) + new (60/eth_secp256k1)
  4) reconstruct legacy multisig; assert address matches file (multisig targets)
  5) fund if spendable balance is below self-send amount + fee (requires --funder)
  6) self-send to publish multisig pubkey on chain (if missing)
  7) delegate to migrate-multisig.sh / migrate-account.sh
  8) verify via evmigration migration-record

Required:
  --mnemonics <file>

Optional:
  --target <name>            Process ONLY the named target. Highly recommended
                             for the first run.
  --funder <key>             Operator keyring key that pays fees for targets
                             classified needs-funding. Lives in the OPERATOR's
                             main keyring (NOT the ephemeral one).
  --top-up-amount <coins>    Amount to send to a needs-funding target before
                             self-send. Default: 200000ulume (covers the
                             self-send amount + fee + headroom).
  --funder-keyring-*         How to reach the funder key (defaults: backend=test,
                             dir=$HOME default).
  --log-file <path>          Append JSONL audit records (one event per line)
                             of every lifecycle milestone: batch_start /
                             target_start / classify / keyring_setup /
                             reconstructed / funding_start / funding_done /
                             self_send_* / ceremony_start / target_done /
                             batch_done. Created mode 0600 if missing. The
                             file is APPEND-ONLY; safe to re-run the batch
                             against the same file. Operator handles rotation.
  --dry-run                  Run read-only steps (incl. address reconstruction
                             + assertion), stop before any broadcast.
  --yes                      Skip interactive confirmation before the batch.
  --continue-on-error        Don't stop the batch when a target fails.

Exit codes:
  0  all targets migrated (or already migrated)
  1  at least one target failed
  2  fatal RPC / config error before processing
  9  mnemonics file is structurally invalid
E_USAGE
        exit 0 ;;
      *) log_error "execute: unknown flag: $1"; exit 1 ;;
    esac
  done

  if [[ -z "$mnemonics_file" ]]; then
    log_error "execute: --mnemonics is required"; exit 1
  fi
  require_jq
  # shellcheck disable=SC2034
  BIN="$binary"
  # shellcheck disable=SC2034
  NODE="$node"
  # shellcheck disable=SC2034
  CHAIN_ID="$chain_id"
  # shellcheck disable=SC2034
  KEYRING_BACKEND="test"
  # shellcheck disable=SC2034
  KEYRING_DIR=""
  # shellcheck disable=SC2034
  HOME_DIR=""
  require_binary
  require_multisig_binary
  resolve_chain_id
  chain_id="$CHAIN_ID"

  local plan targets
  plan=$(_mb_load_plan "$mnemonics_file")
  targets=$(_mb_targets_from_plan "$plan")
  targets=$(_mb_filter_targets "$targets" "$only_target")

  local n_rows
  n_rows=$(jq -r 'length' <<<"$targets")

  log_info "=== Plan ==="
  log_info "  mnemonics file : $mnemonics_file"
  log_info "  chain id       : $CHAIN_ID"
  log_info "  node           : $NODE"
  log_info "  target count   : $n_rows"
  log_info "  funder         : ${_MB_FUNDER:-<none>}"
  log_info "  top-up amount  : $_MB_TOP_UP_AMOUNT"
  log_info "  dry-run        : $(( _MB_DRY_RUN ))"

  # Initialize the optional JSONL run log. Generate a batch_id we can use to
  # correlate every event in this run. mktemp-style randomness via /dev/urandom
  # is preferred over $$/timestamp because two batches can start in the same
  # second, and we want grep-by-batch to be unambiguous.
  if [[ -n "$_MB_LOG_FILE" ]]; then
    # Resolve to absolute path so a later cd cannot redirect appends.
    case "$_MB_LOG_FILE" in
      /*) ;;
      *)  _MB_LOG_FILE="$(pwd)/$_MB_LOG_FILE" ;;
    esac
    if [[ ! -e "$_MB_LOG_FILE" ]]; then
      ( umask 0177 && : >>"$_MB_LOG_FILE" ) || {
        log_error "execute: cannot create --log-file: $_MB_LOG_FILE"
        exit 1
      }
    fi
    if [[ ! -w "$_MB_LOG_FILE" ]]; then
      log_error "execute: --log-file not writable: $_MB_LOG_FILE"
      exit 1
    fi
    _MB_BATCH_ID="$(od -An -tx1 -N8 /dev/urandom 2>/dev/null | tr -d ' \n' || echo "$$-$(date +%s)")"
    log_info "  log file       : $_MB_LOG_FILE"
    log_info "  batch id       : $_MB_BATCH_ID"
  fi
  log_info ""

  # Mainnet guard. This driver has only been validated on devnet/testnet.
  # Refuse mainnet chain-ids unless the operator explicitly opts in via env var.
  # Keep the chain-id patterns aligned with the public Lumera mainnet identifier;
  # if mainnet is renamed, update both this allowlist and the README.
  case "$CHAIN_ID" in
    lumera-mainnet*|lumera-1)
      if [[ "${LUMERA_BATCH_MAINNET_OK:-}" != "i-understand" ]]; then
        log_error "execute: chain-id '$CHAIN_ID' looks like mainnet."
        log_error "  this driver is currently scoped to testnet/devnet."
        log_error "  to override (you accept full responsibility), set:"
        log_error "    LUMERA_BATCH_MAINNET_OK=i-understand"
        exit 1
      fi
      log_warn "execute: LUMERA_BATCH_MAINNET_OK=i-understand set; proceeding on $CHAIN_ID"
      ;;
  esac

  # Show the full target list BEFORE the confirmation prompt, so the operator
  # can sanity-check WHICH 31 (or whatever count) are about to be touched.
  log_info "=== Targets to process ==="
  jq -r --argjson width 38 '
    to_entries[]
    | "  \(.key + 1 | tostring | (. + "."))  [\(.value.kind)]  \(.value.name)"
      + "  -> "  + .value.address
  ' <<<"$targets"
  log_info ""

  if (( yes == 0 )) && (( _MB_DRY_RUN == 0 )); then
    confirm "Proceed with migration of $n_rows target(s)?"
  fi

  _mb_log_event batch_start \
    chain_id "$CHAIN_ID" node "$NODE" target_count "$n_rows" \
    funder "${_MB_FUNDER:-}" top_up_amount "$_MB_TOP_UP_AMOUNT" \
    dry_run "$_MB_DRY_RUN"

  # Install the trap once. _mb_cleanup_ephemeral is a no-op when there's
  # nothing to clean. We CHAIN to the EXIT trap already installed by
  # evmigration-common.sh (which calls cleanup_mnemonic_keys + removes the
  # lumerad_q_capture stderr scratch file). Overwriting that trap would
  # silently leak the scratch file and lose the original exit-code propagation.
  trap 'rc=$?; _mb_cleanup_ephemeral; cleanup_mnemonic_keys; exit "$rc"' EXIT

  local i=0 ok=0 failed=0
  while (( i < n_rows )); do
    local row
    row=$(jq -c ".[$i]" <<<"$targets")
    local rc=0
    _mb_execute_one "$row" || rc=$?
    _mb_cleanup_ephemeral
    if (( rc == 0 )); then
      ok=$((ok+1))
    else
      failed=$((failed+1))
      if (( continue_on_error == 0 )); then
        log_error "stopping batch on first failure (use --continue-on-error to override)"
        break
      fi
    fi
    i=$((i+1))
  done

  log_info ""
  log_info "=== Batch summary ==="
  log_info "  succeeded : $ok"
  log_info "  failed    : $failed"
  log_info "  remaining : $(( n_rows - ok - failed ))"
  _mb_log_event batch_done \
    succeeded "$ok" failed "$failed" remaining "$(( n_rows - ok - failed ))"
  if (( failed > 0 )); then
    return 1
  fi
}

###############################################################################
# Dispatch
###############################################################################
if (( $# == 0 )); then
  _mb_usage
  exit 1
fi

subcommand="$1"
shift
case "$subcommand" in
  report)        _mb_report  "$@" ;;
  status)        _mb_status  "$@" ;;
  execute)       _mb_execute "$@" ;;
  -h|--help)     _mb_usage ;;
  *)             log_error "unknown subcommand: $subcommand"; _mb_usage; exit 1 ;;
esac
