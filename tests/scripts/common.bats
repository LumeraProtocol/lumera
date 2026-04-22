#!/usr/bin/env bats

setup() {
  SCRIPTS_DIR="$(cd "$BATS_TEST_DIRNAME/../../scripts" && pwd)"
  # shellcheck source=../../scripts/evmigration-common.sh
  source "$SCRIPTS_DIR/evmigration-common.sh"
}

@test "log_info writes prefixed message to stderr" {
  run bash -c 'source '"$SCRIPTS_DIR"'/evmigration-common.sh; log_info "hello" 2>&1 1>/dev/null'
  [ "$status" -eq 0 ]
  [[ "$output" == *"INFO"* ]]
  [[ "$output" == *"hello"* ]]
}

@test "log_warn writes prefixed message to stderr" {
  run bash -c 'source '"$SCRIPTS_DIR"'/evmigration-common.sh; log_warn "careful" 2>&1 1>/dev/null'
  [ "$status" -eq 0 ]
  [[ "$output" == *"WARN"* ]]
  [[ "$output" == *"careful"* ]]
}

@test "log_error writes prefixed message to stderr" {
  run bash -c 'source '"$SCRIPTS_DIR"'/evmigration-common.sh; log_error "bad" 2>&1 1>/dev/null'
  [ "$status" -eq 0 ]
  [[ "$output" == *"ERROR"* ]]
  [[ "$output" == *"bad"* ]]
}

@test "log_* output contains no ANSI escapes when stderr is not a TTY" {
  run bash -c 'source '"$SCRIPTS_DIR"'/evmigration-common.sh; log_info "x"; log_warn "y"; log_error "z"' 2>&1
  [ "$status" -eq 0 ]
  # No ESC byte (0x1b / \033) anywhere in the captured output.
  [[ "$output" != *$'\033'* ]]
}

@test "NO_COLOR=1 suppresses color codes" {
  run bash -c 'NO_COLOR=1 source '"$SCRIPTS_DIR"'/evmigration-common.sh; log_info "x" 2>&1'
  [ "$status" -eq 0 ]
  [[ "$output" != *$'\033'* ]]
}

@test "parse_common_flags populates defaults" {
  parse_common_flags legacy new
  [ "$LEGACY_KEY" = "legacy" ]
  [ "$NEW_KEY" = "new" ]
  [ "$KEYRING_BACKEND" = "test" ]
  [ "$YES" = "0" ]
  [ "$DRY_RUN" = "0" ]
  [ "$BIN" = "lumerad" ]
}

@test "parse_common_flags handles all supported flags" {
  parse_common_flags \
    --node tcp://node:26657 \
    --chain-id lumera-devnet \
    --keyring-backend file \
    --keyring-dir /tmp/kr \
    --home /tmp/home \
    --mnemonic-file /tmp/m \
    --yes --dry-run \
    --binary /opt/lumerad \
    mykey1 mykey2
  [ "$NODE" = "tcp://node:26657" ]
  [ "$CHAIN_ID" = "lumera-devnet" ]
  [ "$KEYRING_BACKEND" = "file" ]
  [ "$KEYRING_DIR" = "/tmp/kr" ]
  [ "$HOME_DIR" = "/tmp/home" ]
  [ "$MNEMONIC_FILE" = "/tmp/m" ]
  [ "$YES" = "1" ]
  [ "$DRY_RUN" = "1" ]
  [ "$BIN" = "/opt/lumerad" ]
  [ "$LEGACY_KEY" = "mykey1" ]
  [ "$NEW_KEY" = "mykey2" ]
}

@test "parse_common_flags rejects unknown flag with exit 1" {
  run bash -c 'source '"$SCRIPTS_DIR"'/evmigration-common.sh; parse_common_flags --bogus k1 k2'
  [ "$status" -eq 1 ]
  [[ "$output" == *"unknown flag"* ]]
}

@test "parse_common_flags rejects missing positional with exit 1" {
  run bash -c 'source '"$SCRIPTS_DIR"'/evmigration-common.sh; parse_common_flags onlyone'
  [ "$status" -eq 1 ]
}

@test "parse_common_flags defaults NODE from env" {
  LUMERA_NODE="tcp://from-env:26657" parse_common_flags legacy new
  [ "$NODE" = "tcp://from-env:26657" ]
}

@test "parse_common_flags aborts when --node has no value" {
  run bash -c 'source '"$SCRIPTS_DIR"'/evmigration-common.sh; parse_common_flags --node' 2>&1
  [ "$status" -eq 1 ]
  [[ "$output" == *"--node requires a value"* ]]
}

@test "parse_common_flags aborts when --chain-id has no value" {
  run bash -c 'source '"$SCRIPTS_DIR"'/evmigration-common.sh; parse_common_flags legacy new --chain-id' 2>&1
  [ "$status" -eq 1 ]
  [[ "$output" == *"--chain-id requires a value"* ]]
}

# ---- require_jq / require_binary / lumerad wrappers -------------------------

setup_shim() {
  SHIM_BIN="$BATS_TEST_DIRNAME/fixtures/lumerad-shim.sh"
  BIN="$SHIM_BIN"
  NODE="tcp://localhost:26657"
  CHAIN_ID="shim-test"
  KEYRING_BACKEND="test"
}

@test "require_jq passes when jq exists" {
  run bash -c 'source '"$SCRIPTS_DIR"'/evmigration-common.sh; require_jq'
  [ "$status" -eq 0 ]
}

@test "require_binary accepts shim" {
  setup_shim
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    BIN='"$SHIM_BIN"'
    require_binary
  '
  [ "$status" -eq 0 ]
}

@test "lumerad_q invokes the binary with routed query" {
  setup_shim
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    BIN='"$SHIM_BIN"'
    NODE="tcp://example:1234"
    KEYRING_BACKEND="test"
    lumerad_q evmigration migration-record lumera1anything
  '
  [ "$status" -eq 0 ]
  # Output should be a JSON object (from record-not-found fixture = "{}").
  [[ "$output" == *"{"* ]]
}

@test "resolve_address returns keys-show output" {
  setup_shim
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    BIN='"$SHIM_BIN"'
    NODE="tcp://local:26657"
    KEYRING_BACKEND="test"
    resolve_address mykey
  '
  [ "$status" -eq 0 ]
  [[ "$output" == lumera1* ]]
}

@test "lumera_to_valoper parses debug addr output" {
  setup_shim
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    BIN='"$SHIM_BIN"'
    lumera_to_valoper lumera1anything
  '
  [ "$status" -eq 0 ]
  [[ "$output" == lumeravaloper* ]]
}

@test "preflight_estimate emits raw JSON on stdout, summary on stderr" {
  setup_shim
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    BIN='"$SHIM_BIN"'
    NODE=tcp://local:26657
    preflight_estimate lumera1example 2>/dev/null
  '
  [ "$status" -eq 0 ]
  # stdout must be parseable JSON and contain the fields
  echo "$output" | jq -e '.is_multisig == false'
}

@test "assert_single_sig passes on non-multisig estimate" {
  setup_shim
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    assert_single_sig "$(cat '"$BATS_TEST_DIRNAME"'/fixtures/estimate-ok.json)"
  '
  [ "$status" -eq 0 ]
}

@test "assert_single_sig rejects multisig estimate with exit 3" {
  setup_shim
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    assert_single_sig "$(cat '"$BATS_TEST_DIRNAME"'/fixtures/estimate-multisig.json)"
  '
  [ "$status" -eq 3 ]
  [[ "$output" == *"multisig"* ]]
}

@test "assert_estimate_succeeds exits 4 on would_succeed=false" {
  setup_shim
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    assert_estimate_succeeds "$(cat '"$BATS_TEST_DIRNAME"'/fixtures/estimate-rejected.json)"
  '
  [ "$status" -eq 4 ]
  [[ "$output" == *"legacy account not found"* ]]
}

@test "assert_not_migrated passes when no record" {
  setup_shim
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    BIN='"$SHIM_BIN"'; NODE=tcp://local:1
    assert_not_migrated lumera1anything
  '
  [ "$status" -eq 0 ]
}

@test "assert_not_migrated exits 5 when record exists" {
  setup_shim
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    BIN='"$SHIM_BIN"'; NODE=tcp://local:1
    SHIM_RECORD_FIXTURE=record-found assert_not_migrated lumera1anything
  '
  [ "$status" -eq 5 ]
}

@test "assert_new_address_unused passes when neither query returns a record" {
  setup_shim
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    BIN='"$SHIM_BIN"'; NODE=tcp://local:1
    assert_new_address_unused lumera1newxxxxxx
  '
  [ "$status" -eq 0 ]
}

@test "assert_new_address_unused exits 5 when new-address lookup returns record" {
  setup_shim
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    BIN='"$SHIM_BIN"'; NODE=tcp://local:1
    SHIM_RECORD_FIXTURE=record-found assert_new_address_unused lumera1newxxxxxx
  '
  [ "$status" -eq 5 ]
}

# ---- Bank snapshot, tx polling, verification ---------------------------------

@test "snapshot_bank_balances returns structured JSON" {
  setup_shim
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    BIN='"$SHIM_BIN"'; NODE=tcp://local:1
    snapshot_bank_balances lumera1legacy
  '
  [ "$status" -eq 0 ]
  echo "$output" | jq -e '.balances | length == 1'
}

@test "wait_for_tx returns 0 when shim reports code 0" {
  setup_shim
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    BIN='"$SHIM_BIN"'; NODE=tcp://local:1
    wait_for_tx DEADBEEF
  '
  [ "$status" -eq 0 ]
}

@test "verify_migration is exercised end-to-end in Task 10 integration tests" {
  skip "covered by migrate-account.bats and migrate-validator.bats"
}
