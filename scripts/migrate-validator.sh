#!/usr/bin/env bash
###################################################################################
# Copyright 2026 The Lumera Protocol
#
# Migration shell script for legacy validator accounts.
#
# Migrate a single-signature validator operator from legacy to EVM-compatible keys.
# See docs/design/evmigration-scripts-design.md and
# docs/evm-integration/user-guides/migration.md.

set -euo pipefail
IFS=$'\n\t'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./evmigration-common.sh disable=SC1091
source "${SCRIPT_DIR}/evmigration-common.sh"

_USAGE_DESCRIPTION="Migrate a single-signature validator operator from legacy (coin-type 118)
to EVM-compatible (coin-type 60, eth_secp256k1) keys. Pre-flight runs
MigrationEstimate, rejects non-validator accounts, and aborts if the
validator's delegation+unbonding+redelegation count would exceed
max_validator_delegations. Requires the node to be stopped before
broadcast — will miss blocks and may be jailed during migration."

_USAGE_EXTRA_FLAGS="
Validator-specific flags:
  --i-have-stopped-the-node  Acknowledge downtime non-interactively (required
                             for systemd / CI / non-TTY runs; --yes alone does
                             NOT satisfy this check)"

_USAGE_EXAMPLES="  # Standard interactive migration (prompts for downtime confirmation):
  systemctl stop lumerad
  migrate-validator.sh myval myval-new \\
    --chain-id lumera-mainnet-1 --node https://rpc.lumera.io:443

  # Non-interactive (CI/systemd) — pre-acknowledge downtime:
  migrate-validator.sh myval myval-new \\
    --chain-id lumera-mainnet-1 --node https://rpc.lumera.io:443 \\
    --yes --i-have-stopped-the-node

  # Dry-run only (pre-flight, cap check, downtime prompt, no broadcast):
  migrate-validator.sh myval myval-new --chain-id ... --dry-run"

main() {
  # Pre-strip validator-only flag so parse_common_flags doesn't see it.
  local node_stopped=0
  local filtered=()
  while (( $# > 0 )); do
    case "$1" in
      --i-have-stopped-the-node) node_stopped=1; shift ;;
      *) filtered+=("$1"); shift ;;
    esac
  done
  set -- "${filtered[@]}"

  parse_common_flags "$@"
  log_run_summary "Lumera validator migration"
  log_info "[1/7] validating local prerequisites"
  require_binary
  require_jq

  log_info "[2/7] resolving chain ID from RPC endpoint $NODE"
  resolve_chain_id

  if [[ -n "$MNEMONIC_FILE" ]]; then
    import_from_mnemonic "$MNEMONIC_FILE" "$LEGACY_KEY" "$NEW_KEY"
  fi

  log_info "[3/7] loading legacy and destination keys"
  if [[ "$KEYRING_BACKEND" == "file" ]]; then
    log_info "the encrypted keyring may prompt once for each key; input is hidden while typing"
  fi
  local legacy_addr new_addr
  legacy_addr=$(resolve_address "$LEGACY_KEY")
  new_addr=$(resolve_address "$NEW_KEY")

  log_info "legacy key $(legacy_value "$LEGACY_KEY") -> address $(legacy_value "$legacy_addr")"
  log_info "new EVM key $(new_value "$NEW_KEY") -> address $(new_value "$new_addr")"

  log_info "[4/7] checking migration history and destination freshness"
  assert_not_migrated "$legacy_addr" "$new_addr"
  assert_new_address_unused "$new_addr"
  assert_destination_fresh "$new_addr"

  log_info "[5/7] requesting validator migration estimate"
  local estimate
  estimate=$(preflight_estimate "$legacy_addr")

  assert_single_sig "$estimate"

  if [[ "$(jq -r '.is_validator' <<<"$estimate")" != "true" ]]; then
    log_error "account $(legacy_value "$legacy_addr") is not a validator; use scripts/migrate-account.sh instead"
    exit 6
  fi

  local cap total
  cap=$(lumerad_q evmigration params | jq -r '.params.max_validator_delegations | tonumber')
  # The three counts are uint64 in proto, which Cosmos JSON renders as strings.
  # jq's `+` would concatenate them ("35"+"5"+"16" -> "35516") instead of adding,
  # so explicitly cast each to a number before summing.
  total=$(jq -r '
      ((.val_delegation_count   // 0) | tonumber)
    + ((.val_unbonding_count    // 0) | tonumber)
    + ((.val_redelegation_count // 0) | tonumber)
  ' <<<"$estimate")
  if (( total > cap )); then
    log_error "validator has $total delegation/unbonding/redelegation records; exceeds max_validator_delegations=$cap"
    exit 6
  fi
  if (( total > cap * 9 / 10 )); then
    log_warn "validator record count ($total) is within 10% of cap ($cap)"
  fi

  # Record count drives the gas fallback (see lumerad_tx).
  # shellcheck disable=SC2034
  MIGRATION_RECORD_COUNT="$total"

  assert_estimate_succeeds "$estimate"

  log_info "[6/7] recording the legacy balance for post-migration verification"
  local snap
  snap=$(snapshot_bank_balances "$legacy_addr")

  cat >&2 <<'BANNER'
================================================================
WARNING — VALIDATOR MIGRATION
Your validator will miss blocks and may be jailed during
migration. The node MUST be stopped before broadcasting this tx.
================================================================
BANNER

  if (( node_stopped != 1 )); then
    # Require a TTY so non-interactive runs (systemd, cron, SSH without -t)
    # fail fast instead of blocking forever on `read`. Operators running
    # without a TTY must pass --i-have-stopped-the-node explicitly.
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

  log_info "migrating legacy validator $(legacy_value "$legacy_addr") -> EVM-compatible $(new_value "$new_addr")"

  # Skip the interactive prompt in --dry-run; nothing destructive will happen.
  if (( DRY_RUN == 1 )); then
    log_info "[7/7] dry-run complete: all pre-flight checks passed; no transaction was broadcast"
    return 0
  fi
  log_info "[7/7] generating transaction preview and requesting broadcast confirmation"
  preview_tx_body evmigration migrate-validator "$LEGACY_KEY" "$NEW_KEY"
  confirm "Proceed with validator migration?"

  local broadcast_json tx_hash
  broadcast_json=$(lumerad_tx evmigration migrate-validator "$LEGACY_KEY" "$NEW_KEY" --yes)
  tx_hash=$(assert_broadcast_accepted "$broadcast_json")

  log_info "broadcast tx $tx_hash; waiting for inclusion..."
  # rc=2 means indexation timeout (tx may still land); fall through to
  # verify_migration which checks authoritative chain state. Only fatal on rc=1.
  local wait_rc=0
  wait_for_tx "$tx_hash" || wait_rc=$?
  if (( wait_rc == 1 )); then
    exit 1
  fi

  verify_migration "$legacy_addr" "$new_addr" "$snap"
  show_migration_summary "$legacy_addr" "$new_addr"

  log_info "validator migration complete — post-migration checklist:"
  log_info "  1. Import $(new_value "$NEW_KEY") into the production keyring (correct --keyring-backend)"
  log_info "  2. Restart lumerad"
  log_info "  3. Verify new operator via: lumerad query staking validator <new-valoper>"
  log_info "  4. Monitor missed-block counters for the next few blocks"
}

main "$@"
