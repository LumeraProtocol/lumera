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

@test "parse_common_flags rejects flag-shaped value for --node" {
  run bash -c 'source '"$SCRIPTS_DIR"'/evmigration-common.sh; parse_common_flags --node --chain-id legacy new'
  [ "$status" -eq 1 ]
  [[ "$output" == *"requires a value"* ]]
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

@test "_record_present exits 2 when the query itself fails" {
  setup_shim
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    BIN='"$SHIM_BIN"'; NODE=tcp://local:1
    SHIM_EXIT=1 SHIM_STDERR="simulated rpc failure" _record_present migration-record lumera1anything
  '
  [ "$status" -eq 2 ]
  [[ "$output" == *"could not query"* ]]
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


# ---- Confirmation and mnemonic flow -----------------------------------------

@test "confirm returns 0 immediately when YES=1" {
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    YES=1
    confirm "proceed?"
  '
  [ "$status" -eq 0 ]
}

@test "confirm exits 10 on user no" {
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    YES=0
    echo "n" | confirm "proceed?"
  '
  [ "$status" -eq 10 ]
}

@test "confirm returns 0 on user yes" {
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    YES=0
    echo "y" | confirm "proceed?"
  '
  [ "$status" -eq 0 ]
}

@test "import_from_mnemonic rejects world-readable file with exit 1" {
  setup_shim
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    BIN='"$SHIM_BIN"'
    mf=$(mktemp); echo "test mnemonic" > "$mf"; chmod 0644 "$mf"
    import_from_mnemonic "$mf" k1 k2
    rm -f "$mf"
  '
  [ "$status" -eq 1 ]
  [[ "$output" == *"mode 0600"* ]]
}

@test "shim generate-proof-payload writes to --out path" {
  local tmp
  tmp=$(mktemp)
  run "$BATS_TEST_DIRNAME/fixtures/lumerad-shim.sh" \
    tx evmigration generate-proof-payload \
    --legacy lumera1x --new lumera1y --kind claim \
    --chain-id shim --out "$tmp"
  [ "$status" -eq 0 ]
  [ -f "$tmp" ]
  run jq -r '.kind' "$tmp"
  [ "$output" = "claim" ]
  rm -f "$tmp"
}

@test "shim SHIM_AUTH_TYPE=multisig returns multisig auth-account" {
  run env SHIM_AUTH_TYPE=multisig \
    "$BATS_TEST_DIRNAME/fixtures/lumerad-shim.sh" query auth account lumera1x
  [ "$status" -eq 0 ]
  echo "$output" | jq -e '.account.pub_key."@type" == "/cosmos.crypto.multisig.LegacyAminoPubKey"'
}

@test "shim SHIM_AUTH_TYPE=multisig-nested returns nested pubkey" {
  run env SHIM_AUTH_TYPE=multisig-nested \
    "$BATS_TEST_DIRNAME/fixtures/lumerad-shim.sh" query auth account lumera1x
  [ "$status" -eq 0 ]
  echo "$output" | jq -e '.account.base_vesting_account.base_account.pub_key."@type" == "/cosmos.crypto.multisig.LegacyAminoPubKey"'
}

@test "shim SHIM_AUTH_TYPE=nilpubkey returns nil pub_key" {
  run env SHIM_AUTH_TYPE=nilpubkey \
    "$BATS_TEST_DIRNAME/fixtures/lumerad-shim.sh" query auth account lumera1x
  [ "$status" -eq 0 ]
  echo "$output" | jq -e '.account.pub_key == null'
}

@test "shim keys show --output json returns alice-sub fixture" {
  run "$BATS_TEST_DIRNAME/fixtures/lumerad-shim.sh" keys show alice-sub --output json
  [ "$status" -eq 0 ]
  echo "$output" | jq -e '.name == "alice-sub"'
}

# ---- Multisig helpers -------------------------------------------------------

@test "assert_multisig passes on multisig estimate" {
  setup_shim
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    assert_multisig "$(cat '"$BATS_TEST_DIRNAME"'/fixtures/estimate-multisig.json)"
  '
  [ "$status" -eq 0 ]
}

@test "assert_multisig rejects single-sig estimate with exit 3" {
  setup_shim
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    assert_multisig "$(cat '"$BATS_TEST_DIRNAME"'/fixtures/estimate-ok.json)"
  '
  [ "$status" -eq 3 ]
  [[ "$output" == *"not a multisig"* ]]
}

@test "auth_pubkey_type identifies multisig (top-level)" {
  setup_shim
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    BIN='"$SHIM_BIN"'; NODE=tcp://local:1
    SHIM_AUTH_TYPE=multisig auth_pubkey_type lumera1x
  '
  [ "$status" -eq 0 ]
  [ "$output" = "multisig" ]
}

@test "auth_pubkey_type identifies multisig (nested base_account)" {
  setup_shim
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    BIN='"$SHIM_BIN"'; NODE=tcp://local:1
    SHIM_AUTH_TYPE=multisig-nested auth_pubkey_type lumera1x
  '
  [ "$status" -eq 0 ]
  [ "$output" = "multisig" ]
}

@test "auth_pubkey_type identifies nil pubkey" {
  setup_shim
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    BIN='"$SHIM_BIN"'; NODE=tcp://local:1
    SHIM_AUTH_TYPE=nilpubkey auth_pubkey_type lumera1x
  '
  [ "$status" -eq 0 ]
  [ "$output" = "none" ]
}

@test "auth_pubkey_type identifies single-sig" {
  setup_shim
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    BIN='"$SHIM_BIN"'; NODE=tcp://local:1
    auth_pubkey_type lumera1x
  '
  [ "$status" -eq 0 ]
  [ "$output" = "single-sig" ]
}

@test "key_pubkey_b64 extracts alice-sub's base64 key" {
  setup_shim
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    BIN='"$SHIM_BIN"'
    key_pubkey_b64 alice-sub
  '
  [ "$status" -eq 0 ]
  [ "$output" = "A1111111111111111111111111111111111111111111" ]
}

@test "key_pubkey_b64 accepts object-shaped pubkey JSON" {
  setup_shim
  local tmp; tmp=$(mktemp -d)
  cp "$BATS_TEST_DIRNAME/fixtures/lumerad-shim.sh" "$tmp/lumerad-shim.sh"
  mkdir -p "$tmp"
  cat > "$tmp/keys-show-object-sub.json" <<'JSON'
{
  "name": "object-sub",
  "type": "local",
  "address": "lumera1objectsub1qxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
  "pubkey": {
    "@type": "/cosmos.crypto.secp256k1.PubKey",
    "key": "A4444444444444444444444444444444444444444444"
  }
}
JSON
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    BIN='"$tmp"'/lumerad-shim.sh
    key_pubkey_b64 object-sub
  '
  rm -rf "$tmp"
  [ "$status" -eq 0 ]
  [ "$output" = "A4444444444444444444444444444444444444444444" ]
}

@test "assert_secp256k1_key passes for alice-sub" {
  setup_shim
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    BIN='"$SHIM_BIN"'
    assert_secp256k1_key alice-sub
  '
  [ "$status" -eq 0 ]
}

@test "assert_secp256k1_key rejects new-eth-key (exit 1)" {
  setup_shim
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    BIN='"$SHIM_BIN"'
    assert_secp256k1_key new-eth-key
  '
  [ "$status" -eq 1 ]
  [[ "$output" == *"secp256k1"* ]]
}

@test "assert_eth_key passes for new-eth-key" {
  setup_shim
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    BIN='"$SHIM_BIN"'
    assert_eth_key new-eth-key
  '
  [ "$status" -eq 0 ]
}

@test "assert_eth_key rejects wrong-algo (exit 1)" {
  setup_shim
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    BIN='"$SHIM_BIN"'
    assert_eth_key wrong-algo
  '
  [ "$status" -eq 1 ]
  [[ "$output" == *"eth_secp256k1"* ]]
}

@test "read_proof_file validates proof-template.json and emits JSON" {
  setup_shim
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    read_proof_file '"$BATS_TEST_DIRNAME"'/fixtures/proof-template.json
  '
  [ "$status" -eq 0 ]
  echo "$output" | jq -e '.kind == "claim" and .multisig.threshold == 2'
}

@test "read_proof_file exits 9 on missing required field" {
  setup_shim
  local tmp; tmp=$(mktemp)
  echo '{"kind":"claim","multisig":{"threshold":2,"sub_pub_keys_b64":["x"],"sig_format":"a"}}' > "$tmp"
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    read_proof_file '"$tmp"'
  '
  rm -f "$tmp"
  [ "$status" -eq 9 ]
  [[ "$output" == *"missing required field"* ]]
}

@test "read_proof_file exits 9 on payload_hex mismatch" {
  setup_shim
  local tmp; tmp=$(mktemp)
  jq '.payload_hex = "00"' "$BATS_TEST_DIRNAME/fixtures/proof-template.json" > "$tmp"
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    read_proof_file '"$tmp"'
  '
  rm -f "$tmp"
  [ "$status" -eq 9 ]
  [[ "$output" == *"payload_hex"* ]]
}

@test "read_proof_file exits 3 on single-key proof" {
  setup_shim
  local tmp; tmp=$(mktemp)
  echo '{"kind":"claim","legacy_address":"a","new_address":"b","chain_id":"c","evm_chain_id":"1","payload_hex":"00","single":{"sig_format":"x","signature_b64":"y"},"partial_signatures":[]}' > "$tmp"
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    read_proof_file '"$tmp"'
  '
  rm -f "$tmp"
  [ "$status" -eq 3 ]
  [[ "$output" == *"single-key"* ]]
}

@test "read_proof_file exits 9 when partial index out of range" {
  setup_shim
  local tmp; tmp=$(mktemp)
  jq '.partial_signatures = [{"index": 99, "signature_b64": "abc"}]' \
    "$BATS_TEST_DIRNAME/fixtures/proof-template.json" > "$tmp"
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    read_proof_file '"$tmp"'
  '
  rm -f "$tmp"
  [ "$status" -eq 9 ]
  [[ "$output" == *"out of range"* ]]
}

@test "read_migration_tx_file validates multisig tx JSON" {
  setup_shim
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    read_migration_tx_file '"$BATS_TEST_DIRNAME"'/fixtures/combined-tx.json
  '
  [ "$status" -eq 0 ]
  echo "$output" | jq -e '.kind == "claim" and .threshold == 2 and .num_signers == 3'
}

@test "read_migration_tx_file exits 3 on single-key proof tx" {
  setup_shim
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    read_migration_tx_file '"$BATS_TEST_DIRNAME"'/fixtures/combined-tx-single.json
  '
  [ "$status" -eq 3 ]
  [[ "$output" == *"single-key"* ]]
}

@test "summarize_partials reports threshold satisfied at 2-of-3" {
  setup_shim
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    summarize_partials \
      '"$BATS_TEST_DIRNAME"'/fixtures/partial-alice.json \
      '"$BATS_TEST_DIRNAME"'/fixtures/partial-bob.json
  '
  [ "$status" -eq 0 ]
  [[ "$output" == *"2 >= 2"* ]]
}

@test "summarize_partials returns non-zero when below threshold" {
  setup_shim
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    summarize_partials '"$BATS_TEST_DIRNAME"'/fixtures/partial-alice.json
  '
  [ "$status" -ne 0 ]
  [[ "$output" == *"no"* ]]
}

@test "summarize_partials exits 9 on cross-file chain_id mismatch" {
  setup_shim
  local tmp; tmp=$(mktemp)
  local payload ph
  payload='lumera-evm-migration:different-chain:76857769:claim:lumera1shimaddr1qxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx:lumera1newshimaddrxxxxxxxxxxxxxxxxxxxxxxxxxxxx'
  ph=$(printf '%s' "$payload" | od -An -tx1 -v | tr -d ' \n')
  jq --arg ph "$ph" '.chain_id = "different-chain" | .payload_hex = $ph' \
    "$BATS_TEST_DIRNAME/fixtures/partial-bob.json" > "$tmp"
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    summarize_partials \
      '"$BATS_TEST_DIRNAME"'/fixtures/partial-alice.json \
      '"$tmp"'
  '
  rm -f "$tmp"
  [ "$status" -eq 9 ]
  [[ "$output" == *"chain_id"* ]]
}
