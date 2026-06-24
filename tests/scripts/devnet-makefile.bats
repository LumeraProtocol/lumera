#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  WORKDIR="$(mktemp -d)"
  EXTERNAL_GENESIS="$WORKDIR/genesis.json"
  jq '.app_state.claim.total_claimable_amount = "0" | .app_state.claim.params.enable_claims = false' \
    "$REPO_ROOT/devnet/default-config/devnet-genesis.json" > "$EXTERNAL_GENESIS"
  REMOTE_STAGE_DIR="$REPO_ROOT/build/devnet-remote-stage/lumera-devnet-1"
  REMOTE_CONFIG="$REMOTE_STAGE_DIR/config-no-hermes.json"
  mkdir -p "$REMOTE_STAGE_DIR"
  jq '.hermes.enabled = false' "$REPO_ROOT/devnet/config/config.json" > "$REMOTE_CONFIG"
}

teardown() {
  rm -rf "$WORKDIR"
}

@test "external version staging uses downloaded binaries without requiring claims" {
  run make -C "$REPO_ROOT" -n devnet-stage-external-version \
    VERSION=v1.12.0 \
    EXTERNAL_GENESIS_FILE="$EXTERNAL_GENESIS"

  [ "$status" -eq 0 ]
  [[ "$output" == *"devnet-build"* ]]
  [[ "$output" == *"rm -rf \"$REMOTE_STAGE_DIR\""* ]]
  [[ "$output" == *"DEVNET_DIR=\"$REMOTE_STAGE_DIR\""* ]]
  [[ "$output" == *"DEVNET_BUILD_LUMERA=0"* ]]
  [[ "$output" == *"DEVNET_BUILD_TESTS=0"* ]]
  [[ "$output" == *"DEVNET_BIN_DIR=devnet/bin-v1.12.0"* ]]
  [[ "$output" == *"jq '.hermes.enabled = false'"* ]]
  [[ "$output" == *"CONFIG_JSON=\"$REMOTE_CONFIG\""* ]]
  [[ "$output" == *"EXTERNAL_GENESIS_FILE="* ]]
  [[ "$output" != *"EXTERNAL_CLAIMS_FILE="* ]]
}

@test "remote version target syncs staged runtime and runs docker remotely" {
  run make -C "$REPO_ROOT" -n devnet-new-remote-version \
    VERSION=v1.12.0 \
    EXTERNAL_GENESIS_FILE="$EXTERNAL_GENESIS" \
    REMOTE_DEVNET_HOST=example-devnet \
    REMOTE_DEVNET_DIR=/tmp/lumera-remote

  [ "$status" -eq 0 ]
  [[ "$output" == *"DEVNET_DOCKER_BUILD=0"* ]]
  [[ "$output" == *"rsync"* ]]
  [[ "$output" == *"$REMOTE_STAGE_DIR/shared/"* ]]
  [[ "$output" == *"example-devnet:/tmp/lumera-remote/devnet/"* ]]
  [[ "$output" == *"ssh example-devnet"* ]]
  [[ "$output" == *"docker compose version"* ]]
  [[ "$output" == *"START_MODE=auto docker compose up -d"* ]]
  [[ "$output" == *"--remove-orphans"* ]]
  [[ "$output" != *"ssh example-devnet"*"go "* ]]
}
