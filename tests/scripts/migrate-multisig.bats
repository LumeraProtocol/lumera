#!/usr/bin/env bats

setup() {
  SCRIPTS_DIR="$(cd "$BATS_TEST_DIRNAME/../../scripts" && pwd)"
  FIX_DIR="$BATS_TEST_DIRNAME/fixtures"
  SHIM="$FIX_DIR/lumerad-shim.sh"
}

@test "migrate-multisig.sh with no args prints usage and exits 1" {
  run "$SCRIPTS_DIR/migrate-multisig.sh"
  [ "$status" -eq 1 ]
  [[ "$output" == *"Usage:"* ]]
  [[ "$output" == *"generate"* ]]
  [[ "$output" == *"sign"* ]]
  [[ "$output" == *"combine"* ]]
  [[ "$output" == *"submit"* ]]
}

@test "migrate-multisig.sh --help prints usage and exits 0" {
  run "$SCRIPTS_DIR/migrate-multisig.sh" --help
  [ "$status" -eq 0 ]
  [[ "$output" == *"Usage:"* ]]
}

@test "migrate-multisig.sh -h prints usage and exits 0" {
  run "$SCRIPTS_DIR/migrate-multisig.sh" -h
  [ "$status" -eq 0 ]
  [[ "$output" == *"Usage:"* ]]
}

@test "migrate-multisig.sh bogus subcommand exits 1 with usage" {
  run "$SCRIPTS_DIR/migrate-multisig.sh" bogus
  [ "$status" -eq 1 ]
  [[ "$output" == *"Usage:"* ]]
}

@test "migrate-multisig.sh stubbed subcommand reaches stub and exits 2" {
  run "$SCRIPTS_DIR/migrate-multisig.sh" generate
  [ "$status" -eq 2 ]
  [[ "$output" == *"not yet implemented"* ]]
}
