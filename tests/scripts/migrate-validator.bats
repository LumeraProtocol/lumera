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
  run env SHIM_ESTIMATE_FIXTURE=estimate-validator-ok \
    "$SCRIPTS_DIR/migrate-validator.sh" \
    --binary "$SHIM" --chain-id shim-test \
    --yes --dry-run \
    vkey ekey
  [ "$status" -eq 10 ]
  [[ "$output" == *"node"* ]]
}
