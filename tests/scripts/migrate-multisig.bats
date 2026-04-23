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

@test "migrate-multisig.sh stubbed subcommands (sign/combine/submit) reach stub and exit 2" {
  run "$SCRIPTS_DIR/migrate-multisig.sh" sign
  [ "$status" -eq 2 ]
  [[ "$output" == *"not yet implemented"* ]]
}

@test "generate writes proof.json on happy path (multisig, claim)" {
  local tmp; tmp=$(mktemp -d)
  run env SHIM_AUTH_TYPE=multisig SHIM_ESTIMATE_FIXTURE=estimate-multisig \
    "$SCRIPTS_DIR/migrate-multisig.sh" generate \
    --binary "$SHIM" \
    --legacy lumera1shimaddr1qxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx \
    --new    lumera1newshimaddrxxxxxxxxxxxxxxxxxxxxxxxxxxxx \
    --kind   claim \
    --chain-id shim-test \
    --node tcp://local:1 \
    --out "$tmp/proof.json"
  [ "$status" -eq 0 ]
  [ -f "$tmp/proof.json" ]
  run jq -r '.kind' "$tmp/proof.json"
  [ "$output" = "claim" ]
  rm -rf "$tmp"
}

@test "generate writes proof.json on happy path (multisig, validator)" {
  local tmp; tmp=$(mktemp -d)
  run env SHIM_AUTH_TYPE=multisig SHIM_ESTIMATE_FIXTURE=estimate-multisig-validator \
    "$SCRIPTS_DIR/migrate-multisig.sh" generate \
    --binary "$SHIM" \
    --legacy lumera1shimaddr1qxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx \
    --new    lumera1newshimaddrxxxxxxxxxxxxxxxxxxxxxxxxxxxx \
    --kind   validator \
    --chain-id shim-test \
    --node tcp://local:1 \
    --out "$tmp/proof.json"
  [ "$status" -eq 0 ]
  [ -f "$tmp/proof.json" ]
  rm -rf "$tmp"
}

@test "generate aborts when chain-id is missing (exit 1)" {
  run "$SCRIPTS_DIR/migrate-multisig.sh" generate \
    --binary "$SHIM" \
    --legacy lumera1x --new lumera1y --kind claim \
    --node tcp://local:1 --out /tmp/unused.json
  [ "$status" -eq 1 ]
  [[ "$output" == *"chain-id"* ]]
}

@test "generate aborts when any required flag is missing (exit 1)" {
  run "$SCRIPTS_DIR/migrate-multisig.sh" generate \
    --binary "$SHIM" \
    --kind claim --chain-id shim --node tcp://local:1 --out /tmp/unused.json
  [ "$status" -eq 1 ]
  [[ "$output" == *"required"* ]]
}

@test "generate rejects --keyring-backend (exit 1, pure query)" {
  run "$SCRIPTS_DIR/migrate-multisig.sh" generate \
    --binary "$SHIM" \
    --legacy lumera1x --new lumera1y --kind claim \
    --chain-id shim --node tcp://local:1 --out /tmp/unused.json \
    --keyring-backend test
  [ "$status" -eq 1 ]
  [[ "$output" == *"keyring"* ]]
}

@test "generate rejects --keyring-dir (exit 1)" {
  run "$SCRIPTS_DIR/migrate-multisig.sh" generate \
    --binary "$SHIM" \
    --legacy lumera1x --new lumera1y --kind claim \
    --chain-id shim --node tcp://local:1 --out /tmp/unused.json \
    --keyring-dir /tmp/kr
  [ "$status" -eq 1 ]
  [[ "$output" == *"keyring"* ]]
}

@test "generate rejects --home (exit 1)" {
  run "$SCRIPTS_DIR/migrate-multisig.sh" generate \
    --binary "$SHIM" \
    --legacy lumera1x --new lumera1y --kind claim \
    --chain-id shim --node tcp://local:1 --out /tmp/unused.json \
    --home /tmp/home
  [ "$status" -eq 1 ]
  [[ "$output" == *"keyring"* ]]
}

@test "generate exits 8 when multisig pubkey is nil on-chain" {
  run env SHIM_AUTH_TYPE=nilpubkey \
    "$SCRIPTS_DIR/migrate-multisig.sh" generate \
    --binary "$SHIM" \
    --legacy lumera1nilpubkey1qxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx \
    --new lumera1newshimaddrxxxxxxxxxxxxxxxxxxxxxxxxxxxx \
    --kind claim --chain-id shim-test --node tcp://local:1 \
    --out /tmp/unused.json
  [ "$status" -eq 8 ]
  [[ "$output" == *"seed"* ]]
}

@test "generate exits 3 when account is single-sig" {
  run env SHIM_AUTH_TYPE=single \
    "$SCRIPTS_DIR/migrate-multisig.sh" generate \
    --binary "$SHIM" \
    --legacy lumera1x --new lumera1y --kind claim \
    --chain-id shim-test --node tcp://local:1 \
    --out /tmp/unused.json
  [ "$status" -eq 3 ]
  [[ "$output" == *"single-sig"* ]]
}

@test "generate --kind validator aborts on non-validator multisig (exit 6)" {
  run env SHIM_AUTH_TYPE=multisig SHIM_ESTIMATE_FIXTURE=estimate-multisig \
    "$SCRIPTS_DIR/migrate-multisig.sh" generate \
    --binary "$SHIM" \
    --legacy lumera1shimaddr1qxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx \
    --new lumera1newshimaddrxxxxxxxxxxxxxxxxxxxxxxxxxxxx \
    --kind validator \
    --chain-id shim-test --node tcp://local:1 \
    --out /tmp/unused.json
  [ "$status" -eq 6 ]
  [[ "$output" == *"validator"* ]]
}

@test "generate aborts with exit 4 on estimate would_succeed=false" {
  run env SHIM_AUTH_TYPE=multisig SHIM_ESTIMATE_FIXTURE=estimate-multisig-rejected \
    "$SCRIPTS_DIR/migrate-multisig.sh" generate \
    --binary "$SHIM" \
    --legacy lumera1shimaddr1qxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx \
    --new lumera1newshimaddrxxxxxxxxxxxxxxxxxxxxxxxxxxxx \
    --kind claim --chain-id shim-test --node tcp://local:1 \
    --out /tmp/unused.json
  [ "$status" -eq 4 ]
}

@test "generate aborts with exit 5 when new address already used" {
  run env SHIM_AUTH_TYPE=multisig SHIM_ESTIMATE_FIXTURE=estimate-multisig \
       SHIM_RECORD_FIXTURE=record-found \
    "$SCRIPTS_DIR/migrate-multisig.sh" generate \
    --binary "$SHIM" \
    --legacy lumera1shimaddr1qxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx \
    --new lumera1newshimaddrxxxxxxxxxxxxxxxxxxxxxxxxxxxx \
    --kind claim --chain-id shim-test --node tcp://local:1 \
    --out /tmp/unused.json
  [ "$status" -eq 5 ]
}
