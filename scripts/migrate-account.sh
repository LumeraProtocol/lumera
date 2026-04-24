#!/usr/bin/env bash
#
# Migrate a single-signature legacy account to its EVM-compatible counterpart.
# See docs/design/evmigration-scripts-design.md and
# docs/evm-integration/user-guides/migration.md.

set -euo pipefail
IFS=$'\n\t'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./evmigration-common.sh disable=SC1091
source "${SCRIPT_DIR}/evmigration-common.sh"

_USAGE_DESCRIPTION="Migrate a single-signature legacy account (coin-type 118) to its EVM-compatible
counterpart (coin-type 60, eth_secp256k1). Pre-flight runs MigrationEstimate
and aborts on multisig accounts, validator operators, or any state the chain
would reject. Takes a pre-broadcast balance snapshot and verifies after tx
inclusion that balances moved correctly."

_USAGE_EXAMPLES="  # Standard migration — both keys already in the keyring:
  migrate-account.sh alice alice-new \\
    --chain-id lumera-mainnet-1 --node tcp://rpc.lumera:26657

  # Import both keys from a single mnemonic file first (file mode must be 0600):
  migrate-account.sh alice alice-new \\
    --chain-id lumera-mainnet-1 --mnemonic-file /run/user/1000/alice.seed

  # Dry-run only (pre-flight + preview, no broadcast):
  migrate-account.sh alice alice-new --chain-id lumera-mainnet-1 --dry-run"

main() {
  parse_common_flags "$@"
  require_binary
  require_jq

  if [[ -n "$MNEMONIC_FILE" ]]; then
    import_from_mnemonic "$MNEMONIC_FILE" "$LEGACY_KEY" "$NEW_KEY"
  fi

  assert_secp256k1_key "$LEGACY_KEY"
  assert_eth_key "$NEW_KEY"

  local legacy_addr new_addr
  legacy_addr=$(resolve_address "$LEGACY_KEY")
  new_addr=$(resolve_address "$NEW_KEY")

  assert_not_migrated "$legacy_addr"
  assert_new_address_unused "$new_addr"

  local estimate
  estimate=$(preflight_estimate "$legacy_addr")

  assert_single_sig "$estimate"

  if [[ "$(jq -r '.is_validator' <<<"$estimate")" == "true" ]]; then
    log_error "account $legacy_addr is a validator; use scripts/migrate-validator.sh instead"
    exit 6
  fi

  if [[ "$(jq -r '.has_supernode' <<<"$estimate")" == "true" ]]; then
    log_warn "this account owns a supernode registration; it will migrate with the account"
  fi

  assert_estimate_succeeds "$estimate"

  local snap
  snap=$(snapshot_bank_balances "$legacy_addr")

  log_info "migrating $legacy_addr -> $new_addr"
  confirm "Proceed with migration?"

  if (( DRY_RUN == 1 )); then
    log_info "--dry-run: stopping before broadcast"
    return 0
  fi

  local broadcast_json tx_hash
  broadcast_json=$(lumerad_tx evmigration claim-legacy-account "$LEGACY_KEY" "$NEW_KEY" --yes)
  tx_hash=$(jq -r '.txhash' <<<"$broadcast_json")
  if [[ -z "$tx_hash" || "$tx_hash" == "null" ]]; then
    log_error "broadcast returned no txhash: $broadcast_json"
    exit 2
  fi

  log_info "broadcast tx $tx_hash; waiting for inclusion..."
  wait_for_tx "$tx_hash"

  verify_migration "$legacy_addr" "$new_addr" "$snap"

  log_info "migration complete"
  log_info "  legacy: $legacy_addr"
  log_info "  new:    $new_addr"
  log_info "  tx:     $tx_hash"
}

main "$@"
