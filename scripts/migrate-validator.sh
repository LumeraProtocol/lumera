#!/usr/bin/env bash
#
# Migrate a single-signature validator operator from legacy to EVM-compatible keys.
# See docs/design/evmigration-scripts-design.md and
# docs/evm-integration/user-guides/migration.md.

set -euo pipefail
IFS=$'\n\t'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./evmigration-common.sh disable=SC1091
source "${SCRIPT_DIR}/evmigration-common.sh"

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
  require_binary
  require_jq

  if [[ -n "$MNEMONIC_FILE" ]]; then
    import_from_mnemonic "$MNEMONIC_FILE" "$LEGACY_KEY" "$NEW_KEY"
  fi

  local legacy_addr new_addr
  legacy_addr=$(resolve_address "$LEGACY_KEY")
  new_addr=$(resolve_address "$NEW_KEY")

  assert_not_migrated "$legacy_addr"
  assert_new_address_unused "$new_addr"

  local estimate
  estimate=$(preflight_estimate "$legacy_addr")

  assert_single_sig "$estimate"

  if [[ "$(jq -r '.is_validator' <<<"$estimate")" != "true" ]]; then
    log_error "account $legacy_addr is not a validator; use scripts/migrate-account.sh instead"
    exit 6
  fi

  local cap total
  cap=$(lumerad_q evmigration params | jq -r '.params.max_validator_delegations | tonumber')
  total=$(jq -r '.val_delegation_count + .val_unbonding_count + .val_redelegation_count' <<<"$estimate")
  if (( total > cap )); then
    log_error "validator has $total delegation/unbonding/redelegation records; exceeds max_validator_delegations=$cap"
    exit 6
  fi
  if (( total > cap * 9 / 10 )); then
    log_warn "validator record count ($total) is within 10% of cap ($cap)"
  fi

  assert_estimate_succeeds "$estimate"

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

  log_info "migrating validator $legacy_addr -> $new_addr"
  confirm "Proceed with validator migration?"

  if (( DRY_RUN == 1 )); then
    log_info "--dry-run: stopping before broadcast"
    return 0
  fi

  local broadcast_json tx_hash
  broadcast_json=$(lumerad_tx evmigration migrate-validator "$LEGACY_KEY" "$NEW_KEY" --yes)
  tx_hash=$(jq -r '.txhash' <<<"$broadcast_json")
  if [[ -z "$tx_hash" || "$tx_hash" == "null" ]]; then
    log_error "broadcast returned no txhash: $broadcast_json"
    exit 2
  fi

  log_info "broadcast tx $tx_hash; waiting for inclusion..."
  wait_for_tx "$tx_hash"

  verify_migration "$legacy_addr" "$new_addr" "$snap"

  log_info "validator migration complete — post-migration checklist:"
  log_info "  1. Import $NEW_KEY into the production keyring (correct --keyring-backend)"
  log_info "  2. Restart lumerad"
  log_info "  3. Verify new operator via: lumerad query staking validator <new-valoper>"
  log_info "  4. Monitor missed-block counters for the next few blocks"
}

main "$@"
