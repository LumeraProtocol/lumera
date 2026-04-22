#!/usr/bin/env bats

setup() {
  SCRIPTS_DIR="$(cd "$BATS_TEST_DIRNAME/../../scripts" && pwd)"
  FIX_DIR="$BATS_TEST_DIRNAME/fixtures"
  SHIM="$FIX_DIR/lumerad-shim.sh"
}

@test "migrate-account.sh happy-path dry-run exits 0 and does not broadcast" {
  run "$SCRIPTS_DIR/migrate-account.sh" \
    --binary "$SHIM" \
    --chain-id shim-test \
    --dry-run --yes \
    legacykey newkey
  [ "$status" -eq 0 ]
  [[ "$output" == *"Migration preview"* ]]
}

@test "migrate-account.sh rejects multisig account with exit 3" {
  run env SHIM_ESTIMATE_FIXTURE=estimate-multisig \
    "$SCRIPTS_DIR/migrate-account.sh" \
    --binary "$SHIM" --chain-id shim-test --yes \
    legacykey newkey
  [ "$status" -eq 3 ]
  [[ "$output" == *"multisig"* ]]
}

@test "migrate-account.sh errors usage when given one positional arg" {
  run "$SCRIPTS_DIR/migrate-account.sh" --chain-id x onlyone
  [ "$status" -eq 1 ]
}
