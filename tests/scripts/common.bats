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
