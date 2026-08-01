#!/usr/bin/env bats

@test "compatibility manifest schema rejects unsafe runtime paths" {
  run python3 tests/scripts/compatibility_manifest_schema_test.py

  [ "$status" -eq 0 ]
}
