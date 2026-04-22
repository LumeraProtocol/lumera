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
