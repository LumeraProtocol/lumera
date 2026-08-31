#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  START_SCRIPT="$REPO_ROOT/devnet/scripts/start.sh"
}

@test "run mode starts lumerad before waiting for blocks" {
  run_block="$(awk '
    /^[[:space:]]*run\)/ { in_run=1; next }
    in_run && /^[[:space:]]*;;/ { exit }
    in_run { print }
  ' "$START_SCRIPT")"

  [ -n "$run_block" ]

  start_line="$(printf '%s\n' "$run_block" | awk '/start_lumera/ { print NR; exit }')"
  wait_line="$(printf '%s\n' "$run_block" | awk '/wait_for_n_blocks 3/ { print NR; exit }')"

  [ -n "$start_line" ]
  [ -n "$wait_line" ]
  [ "$start_line" -lt "$wait_line" ]
}
