#!/usr/bin/env bash
###################################################################################
# Copyright 2026 The Lumera Protocol
#
# Migration shell script for regular legacy accounts (single signature).
# 
# Migrate a legacy account (coin-type 118, secp256k1) to its EVM-compatible counterpart.
# See docs/design/evmigration-scripts-design.md and
# docs/evm-integration/user-guides/migration.md.

set -euo pipefail
IFS=$'\n\t'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./evmigration-common.sh disable=SC1091
source "${SCRIPT_DIR}/evmigration-common.sh"

_USAGE_DESCRIPTION="Migrate a legacy account (coin-type 118, secp256k1) to its EVM-compatible
counterpart (coin-type 60, eth_secp256k1).

Before broadcasting, the script asks the chain whether the migration would
succeed and exits early if not — for example, when the source key is a
multisig wallet, a validator's operator account, or has on-chain state that
blocks migration. This saves you the gas of a tx that would just be rejected.

Balances on both addresses are recorded before broadcast and re-checked once
the migration tx is committed to a block, so you'll see clearly whether the
funds moved as expected."

_USAGE_EXAMPLES="  # Standard migration — both keys already in the keyring:
  migrate-account.sh alice alice-new \\
    --chain-id lumera-mainnet-1 --node https://rpc.lumera.io:443

  # Import both keys from a single mnemonic file first (file mode must be 0600):
  migrate-account.sh alice alice-new \\
    --chain-id lumera-mainnet-1 --node https://rpc.lumera.io:443 \\
    --mnemonic-file /run/user/1000/alice.seed

  # Dry-run only (pre-flight + preview, no broadcast):
  migrate-account.sh alice alice-new --chain-id lumera-mainnet-1 --dry-run"

main() {
  parse_common_flags "$@"
  require_binary
  require_jq
  resolve_chain_id

  if [[ -n "$MNEMONIC_FILE" ]]; then
    import_from_mnemonic "$MNEMONIC_FILE" "$LEGACY_KEY" "$NEW_KEY"
  fi

  local legacy_addr new_addr
  legacy_addr=$(resolve_address "$LEGACY_KEY")
  new_addr=$(resolve_address "$NEW_KEY")

  log_info "legacy key $(legacy_value "$LEGACY_KEY") -> address $(legacy_value "$legacy_addr")"
  log_info "new EVM key $(new_value "$NEW_KEY") -> address $(new_value "$new_addr")"

  assert_not_migrated "$legacy_addr" "$new_addr"
  assert_new_address_unused "$new_addr"
  assert_destination_fresh "$new_addr"

  local estimate
  estimate=$(preflight_estimate "$legacy_addr")

  assert_single_sig "$estimate"

  if [[ "$(jq -r '.is_validator' <<<"$estimate")" == "true" ]]; then
    log_error "account $(legacy_value "$legacy_addr") is a validator; use scripts/migrate-validator.sh instead"
    exit 6
  fi

  if [[ "$(jq -r '.has_supernode' <<<"$estimate")" == "true" ]]; then
    log_warn "this account owns a supernode registration; it will migrate with the account"
  fi

  assert_estimate_succeeds "$estimate"

  # shellcheck disable=SC2034
  MIGRATION_RECORD_COUNT="$(jq -r '
      ((.delegation_count   // 0) | tonumber)
    + ((.unbonding_count    // 0) | tonumber)
    + ((.redelegation_count // 0) | tonumber)
  ' <<<"$estimate")"

  local snap
  snap=$(snapshot_bank_balances "$legacy_addr")

  log_info "migrating legacy account $(legacy_value "$legacy_addr") -> EVM-compatible $(new_value "$new_addr")"

  # Skip the interactive prompt in --dry-run; nothing destructive will happen.
  if (( DRY_RUN == 1 )); then
    log_info "--dry-run: stopping before broadcast"
    return 0
  fi
  preview_tx_body evmigration claim-legacy-account "$LEGACY_KEY" "$NEW_KEY"
  confirm "Proceed with migration?"

  local broadcast_json tx_hash
  broadcast_json=$(lumerad_tx evmigration claim-legacy-account "$LEGACY_KEY" "$NEW_KEY" --yes)
  tx_hash=$(assert_broadcast_accepted "$broadcast_json")

  log_info "broadcast tx $tx_hash; waiting for inclusion..."
  # rc=0 confirmed-success, rc=1 chain-rejected, rc=2 indexation-timeout (tx
  # may still land). Only treat rc=1 as fatal here; rc=2 falls through to
  # verify_migration which authoritatively reads chain state and will catch
  # any genuine non-application of the migration.
  local wait_rc=0
  wait_for_tx "$tx_hash" || wait_rc=$?
  if (( wait_rc == 1 )); then
    exit 1
  fi

  verify_migration "$legacy_addr" "$new_addr" "$snap"
  show_migration_summary "$legacy_addr" "$new_addr"

  log_info "migration complete"
  log_info "  legacy: $(legacy_value "$legacy_addr")"
  log_info "  new:    $(new_value "$new_addr")"
  log_info "  tx:     $tx_hash"
}

main "$@"
