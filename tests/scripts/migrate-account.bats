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

@test "migrate-account.sh resolves keyring backend from client.toml" {
  local home; home=$(mktemp -d)
  mkdir -p "$home/config"
  printf 'keyring-backend = "file"\n' > "$home/config/client.toml"
  run "$SCRIPTS_DIR/migrate-account.sh" \
    --binary "$SHIM" --chain-id shim-test --home "$home" \
    --dry-run --yes legacykey newkey
  [ "$status" -eq 0 ]
  [[ "$output" == *"keyring backend: file (from $home/config/client.toml)"* ]]
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

@test "migrate-account.sh full happy path (broadcast + verify) exits 0" {
  local state_dir state_file
  state_dir=$(mktemp -d)
  state_file="$state_dir/state"
  run env \
    SHIM_STATE_FILE="$state_file" \
    SHIM_RECORD_AFTER_FIXTURE=record-post-migration \
    SHIM_BANK_AFTER_FIXTURE=bank-balances-empty \
    "$SCRIPTS_DIR/migrate-account.sh" \
    --binary "$SHIM" \
    --chain-id shim-test \
    --yes \
    legacykey newkey
  rm -rf "$state_dir"
  [ "$status" -eq 0 ]
  [[ "$output" == *"migration complete"* ]]
}

@test "migrate-account.sh executes the explicit canonical binary and reports provenance" {
  local tmp real link keyring_dir
  tmp=$(mktemp -d)
  mkdir -p "$tmp/bin with spaces" "$tmp/approved" "$tmp/keyring location"
  real="$tmp/bin with spaces/lumerad-real"
  link="$tmp/approved/lumerad"
  keyring_dir="$tmp/keyring location"
  printf '#!/usr/bin/env bash\nexec %q "$@"\n' "$SHIM" >"$real"
  chmod +x "$real"
  ln -s "$real" "$link"

  run env SHIM_VERSION=v1.20.1-operator-test \
    "$SCRIPTS_DIR/migrate-account.sh" \
    --binary "$link" \
    --chain-id shim-test \
    --keyring-backend test \
    --keyring-dir "$keyring_dir" \
    --dry-run --yes \
    legacykey newkey

  rm -rf "$tmp"
  [ "$status" -eq 0 ]
  [[ "$output" == *"Binary:          $real"* ]]
  [[ "$output" == *"Binary version:  v1.20.1-operator-test"* ]]
  [[ "$output" == *"Binary source:   explicit --binary/path ($link)"* ]]
  [[ "$output" == *"Keyring backend:  test (explicit --keyring-backend)"* ]]
  [[ "$output" == *"Keyring location: $keyring_dir (explicit --keyring-dir)"* ]]
  [[ "$output" == *"Migration preview"* ]]
}

@test "migrate-account.sh preserves a keyring permission failure end to end" {
  run env SHIM_KEYS_ERROR="failed to decrypt keyring: permission denied" \
    "$SCRIPTS_DIR/migrate-account.sh" \
    --binary "$SHIM" --chain-id shim-test --keyring-backend test \
    --dry-run --yes legacykey newkey

  [ "$status" -eq 1 ]
  [[ "$output" == *"failed to decrypt keyring: permission denied"* ]]
  [[ "$output" != *"key not found in keyring"* ]]
}
