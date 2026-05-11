#!/usr/bin/env bats

setup() {
  SCRIPTS_DIR="$(cd "$BATS_TEST_DIRNAME/../../scripts" && pwd)"
  FIX_DIR="$BATS_TEST_DIRNAME/fixtures"
  SHIM="$FIX_DIR/lumerad-shim.sh"
}

@test "migrate-validator.sh dry-run happy path exits 0" {
  run env SHIM_ESTIMATE_FIXTURE=estimate-validator-ok \
    "$SCRIPTS_DIR/migrate-validator.sh" \
    --binary "$SHIM" --chain-id shim-test \
    --i-have-stopped-the-node --yes --dry-run \
    vkey ekey
  [ "$status" -eq 0 ]
}

@test "migrate-validator.sh rejects non-validator with exit 6" {
  run "$SCRIPTS_DIR/migrate-validator.sh" \
    --binary "$SHIM" --chain-id shim-test \
    --i-have-stopped-the-node --yes --dry-run \
    vkey ekey
  # default estimate fixture has is_validator=false
  [ "$status" -eq 6 ]
  [[ "$output" == *"not a validator"* ]]
}

@test "migrate-validator.sh rejects over-cap with exit 6" {
  run env SHIM_ESTIMATE_FIXTURE=estimate-validator-over-cap \
    "$SCRIPTS_DIR/migrate-validator.sh" \
    --binary "$SHIM" --chain-id shim-test \
    --i-have-stopped-the-node --yes --dry-run \
    vkey ekey
  [ "$status" -eq 6 ]
  [[ "$output" == *"max_validator_delegations"* ]]
}

@test "migrate-validator.sh rejects missing downtime ack" {
  # Close stdin so the script's `read -r reply` returns EOF immediately
  # instead of blocking on terminal input. The `read ... || true` in the
  # script then proceeds with an empty reply, which fails the ack check.
  run env SHIM_ESTIMATE_FIXTURE=estimate-validator-ok \
    "$SCRIPTS_DIR/migrate-validator.sh" \
    --binary "$SHIM" --chain-id shim-test \
    --yes --dry-run \
    vkey ekey </dev/null
  [ "$status" -eq 10 ]
  [[ "$output" == *"node"* ]]
}

@test "migrate-validator.sh full happy path (broadcast + verify) exits 0" {
  local state_dir state_file
  state_dir=$(mktemp -d)
  state_file="$state_dir/state"
  run env \
    SHIM_ESTIMATE_FIXTURE=estimate-validator-ok \
    SHIM_STATE_FILE="$state_file" \
    SHIM_RECORD_AFTER_FIXTURE=record-post-migration \
    SHIM_BANK_AFTER_FIXTURE=bank-balances-empty \
    "$SCRIPTS_DIR/migrate-validator.sh" \
    --binary "$SHIM" \
    --chain-id shim-test \
    --i-have-stopped-the-node --yes \
    vkey newkey
  rm -rf "$state_dir"
  [ "$status" -eq 0 ]
  [[ "$output" == *"validator migration complete"* ]]
  [[ "$output" == *"Restart lumerad"* ]]
}
