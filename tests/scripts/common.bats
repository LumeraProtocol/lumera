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

@test "v1.20.1 CLI omits the zero-timeout optimization" {
  setup_shim
  _read_migration_tx_timeout_flags
  [ "${#_MIGRATION_TX_TIMEOUT_FLAGS[@]}" -eq 0 ]
}

@test "new CLI enables immediate broadcast return" {
  setup_shim
  export SHIM_IMMEDIATE_BROADCAST_RETURN=1
  _read_migration_tx_timeout_flags
  [ "${#_MIGRATION_TX_TIMEOUT_FLAGS[@]}" -eq 2 ]
  [ "${_MIGRATION_TX_TIMEOUT_FLAGS[0]}" = "--tx-timeout" ]
  [ "${_MIGRATION_TX_TIMEOUT_FLAGS[1]}" = "0s" ]
}

@test "resolve_address returns keys-show output" {
  setup_shim
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    BIN='"$SHIM_BIN"'
    NODE="tcp://local:26657"
    KEYRING_BACKEND="test"
    resolve_address mykey 2>/dev/null
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
  [[ "$output" == *"no migration record found for legacy address lumera1anything"* ]]
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
  [[ "$output" == *"has no migration record as a legacy address"* ]]
  [[ "$output" == *"no migration record found by new address lumera1newxxxxxx"* ]]
}

@test "assert_destination_fresh passes when destination address does not exist" {
  setup_shim
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    BIN='"$SHIM_BIN"'; NODE=tcp://local:1
    assert_destination_fresh lumera1newshimaddrxxxxxxxxxxxxxxxxxxxxxxxxxxxx
  '
  [ "$status" -eq 0 ]
  [[ "$output" == *"destination address lumera1newshimaddrxxxxxxxxxxxxxxxxxxxxxxxxxxxx does not exist on-chain"* ]]
}

@test "assert_destination_fresh exits 5 when destination address exists" {
  setup_shim
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    BIN='"$SHIM_BIN"'; NODE=tcp://local:1
    SHIM_AUTH_NEW_FIXTURE=auth-account assert_destination_fresh lumera1newshimaddrxxxxxxxxxxxxxxxxxxxxxxxxxxxx
  '
  [ "$status" -eq 5 ]
  [[ "$output" == *"already exists on-chain"* ]]
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

@test "wait_for_tx polls query tx when wait-tx returns before the tx is indexed" {
  setup_shim
  local state_dir state_file
  state_dir=$(mktemp -d)
  state_file="$state_dir/shim-state"

  run env SHIM_STATE_FILE="$state_file" SHIM_TX_PENDING_QUERIES=3 bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    BIN='"$SHIM_BIN"'; NODE=tcp://local:1
    LUMERA_TX_WAIT_TIMEOUT=3
    wait_for_tx DEADBEEF
  '
  rm -rf "$state_dir"

  [ "$status" -eq 0 ]
  [[ "$output" == *"polling query tx"* ]]
  [[ "$output" == *"tx included at height 100"* ]]
}

@test "assert_broadcast_accepted accepts concatenated successful broadcast JSON" {
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    json=$'"'"'{"height":"0","txhash":"FIRST","code":0,"raw_log":""}\n{"height":"0","txhash":"SECOND","code":"0","raw_log":""}'"'"'
    assert_broadcast_accepted "$json"
  '
  [ "$status" -eq 0 ]
  [ "$output" = "SECOND" ]
}

@test "assert_broadcast_accepted rejects non-zero CheckTx code" {
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    json=$'"'"'{"height":"0","txhash":"BAD","code":12,"raw_log":"bad tx"}'"'"'
    assert_broadcast_accepted "$json"
  '
  [ "$status" -eq 2 ]
  [[ "$output" == *"code=12"* ]]
  [[ "$output" == *"bad tx"* ]]
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

@test "import_from_mnemonic reuses existing matching mnemonic keys" {
  setup_shim
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    BIN='"$SHIM_BIN"'
    mf=$(mktemp); echo "test mnemonic" > "$mf"; chmod 0600 "$mf"
    import_from_mnemonic "$mf" alice new-eth-key
    rm -f "$mf"
  '
  [ "$status" -eq 0 ]
  [[ "$output" == *"legacy key alice already exists in keyring and matches mnemonic; reusing it"* ]]
  [[ "$output" == *"new EVM key new-eth-key already exists in keyring and matches mnemonic; reusing it"* ]]
}

@test "import_from_mnemonic imports missing keys for this run" {
  setup_shim
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    BIN='"$SHIM_BIN"'
    SHIM_KEYS_MISSING=alice,new-eth-key
    export SHIM_KEYS_MISSING
    mf=$(mktemp); echo "test mnemonic" > "$mf"; chmod 0600 "$mf"
    import_from_mnemonic "$mf" alice new-eth-key
    rm -f "$mf"
  '
  [ "$status" -eq 0 ]
  [[ "$output" == *"imported legacy key alice from mnemonic for this run"* ]]
  [[ "$output" == *"imported new EVM key new-eth-key from mnemonic for this run"* ]]
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

@test "auth_pubkey_type identifies multisig with type_url pubkey" {
  setup_shim
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    BIN='"$SHIM_BIN"'; NODE=tcp://local:1
    SHIM_AUTH_TYPE=multisig-type-url auth_pubkey_type lumera1x
  '
  [ "$status" -eq 0 ]
  [ "$output" = "multisig" ]
}

@test "auth_pubkey_type identifies legacy amino multisig account response" {
  setup_shim
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    BIN='"$SHIM_BIN"'; NODE=tcp://local:1
    SHIM_AUTH_TYPE=multisig-amino auth_pubkey_type lumera1x
  '
  [ "$status" -eq 0 ]
  [ "$output" = "multisig" ]
}

@test "auth_multisig_threshold reads seeded on-chain threshold" {
  setup_shim
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    BIN='"$SHIM_BIN"'; NODE=tcp://local:1
    SHIM_AUTH_TYPE=multisig auth_multisig_threshold lumera1x
  '
  [ "$status" -eq 0 ]
  [ "$output" = "2" ]
}

@test "auth_multisig helpers normalize legacy amino multisig account response" {
  setup_shim
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    BIN='"$SHIM_BIN"'; NODE=tcp://local:1
    export SHIM_AUTH_TYPE=multisig-amino
    printf "%s\n" "$(auth_multisig_threshold lumera1x)" "$(auth_multisig_subkey_count lumera1x)"
  '
  [ "$status" -eq 0 ]
  [ "${lines[0]}" = "2" ]
  [ "${lines[1]}" = "3" ]
}

@test "auth_multisig_subkey_count reads seeded on-chain signer count" {
  setup_shim
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    BIN='"$SHIM_BIN"'; NODE=tcp://local:1
    SHIM_AUTH_TYPE=multisig auth_multisig_subkey_count lumera1x
  '
  [ "$status" -eq 0 ]
  [ "$output" = "3" ]
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

@test "key_multisig helpers read local EVM multisig key" {
  setup_shim
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    BIN='"$SHIM_BIN"'; KEYRING_BACKEND=test
    printf "%s\n" "$(key_multisig_threshold new-msig)" "$(key_multisig_sub_pub_keys_csv new-msig)"
  '
  [ "$status" -eq 0 ]
  [[ "$output" == $'2\nB1111111111111111111111111111111111111111111,B2222222222222222222222222222222222222222222,B3333333333333333333333333333333333333333333' ]]
}

@test "read_proof_file validates proof-template.json and emits JSON" {
  setup_shim
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    read_proof_file '"$BATS_TEST_DIRNAME"'/fixtures/proof-template.json
  '
  [ "$status" -eq 0 ]
  echo "$output" | jq -e '.kind == "claim" and .legacy.threshold == 2 and .new.threshold == 2'
}

@test "read_proof_file exits 9 on missing required field" {
  setup_shim
  local tmp; tmp=$(mktemp)
  # Valid top-level version + legacy/new skeleton but missing chain_id/payload_hex etc.
  echo '{"version":2,"kind":"claim","legacy":{"threshold":2,"sub_pub_keys":["x"],"sig_format":"a"},"new":{"threshold":2,"sub_pub_keys":["y"],"sig_format":"a"}}' > "$tmp"
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

@test "read_proof_file exits 3 on single-key-on-either-side" {
  setup_shim
  # v2 shape with the new side a single-key (pub_key set, no threshold) —
  # mirror-source rule rejects single-single and mixed; wrapper is multisig-only.
  local tmp; tmp=$(mktemp)
  jq '.new = {"pub_key": "Zm9v", "sig_format": "SIG_FORMAT_EIP191"}' \
    "$BATS_TEST_DIRNAME/fixtures/proof-template.json" > "$tmp"
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    read_proof_file '"$tmp"'
  '
  rm -f "$tmp"
  [ "$status" -eq 3 ]
  [[ "$output" == *"multisig"*"multisig"* ]]
}

@test "read_proof_file exits 9 when partial index out of range" {
  setup_shim
  local tmp; tmp=$(mktemp)
  jq '.partial_legacy_signatures = [{"index": 99, "signature": "abc"}]' \
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
  echo "$output" | jq -e '.kind == "claim" and .threshold == 2 and .num_signers == 3 and .new_threshold == 2 and .new_num_signers == 3'
}

@test "read_migration_tx_file exits 3 on single-key proof tx" {
  setup_shim
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    read_migration_tx_file '"$BATS_TEST_DIRNAME"'/fixtures/combined-tx-single.json
  '
  [ "$status" -eq 3 ]
  [[ "$output" == *"multisig"*"multisig"* ]]
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

@test "summarize_partials fails when per-side quorum is met but matching-index quorum is not" {
  setup_shim
  local tmp; tmp=$(mktemp -d)
  # Build two synthetic partials that together satisfy per-side K=2 on
  # both legacy and new, but whose intersection is only 1 index — so the
  # wrapper must fail the matching-index check even though legacy and
  # new each have two distinct signer indices.
  #   alice signs legacy[0] + new[0]
  #   bob-ish signs legacy[1] only
  #   carol-ish signs new[1] only
  # Combined: legacy_indices={0,1}, new_indices={0,1} at first glance —
  # but partial-bob.json and partial-carol.json carry signatures for BOTH
  # sides, so we synthesize a one-sided variant.
  jq '.partial_new_signatures = []' "$BATS_TEST_DIRNAME/fixtures/partial-bob.json" \
    > "$tmp/bob-legacy-only.json"
  jq '.partial_legacy_signatures = []' "$BATS_TEST_DIRNAME/fixtures/partial-carol.json" \
    > "$tmp/carol-new-only.json"
  run bash -c '
    source '"$SCRIPTS_DIR"'/evmigration-common.sh
    summarize_partials \
      '"$BATS_TEST_DIRNAME"'/fixtures/partial-alice.json \
      '"$tmp"'/bob-legacy-only.json \
      '"$tmp"'/carol-new-only.json
  '
  rm -rf "$tmp"
  # Per-side matrices both report "yes" (legacy [0,1], new [0,2]) but the
  # shared-index count is 1 — below K=2. Wrapper returns non-zero.
  [ "$status" -ne 0 ]
  [[ "$output" == *"Matching-index threshold satisfied: no"* ]]
  [[ "$output" == *"one-sided partials do not count"* ]]
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

@test "migration_gas_for_records: base with zero records" {
  [ "$(migration_gas_for_records 0)" = "6000000" ]
}

@test "migration_gas_for_records: base plus per-record marginal" {
  [ "$(migration_gas_for_records 1597)" = "2401500000" ]
  [ "$(migration_gas_for_records 2500)" = "3756000000" ]
}

@test "gas_exceeds_block_limit: true only when over a positive limit" {
  run gas_exceeds_block_limit 30000000 25000000
  [ "$status" -eq 0 ]
  run gas_exceeds_block_limit 11379000 25000000
  [ "$status" -eq 1 ]
  run gas_exceeds_block_limit 99999999 -1
  [ "$status" -eq 1 ]
  run gas_exceeds_block_limit 99999999 ""
  [ "$status" -eq 1 ]
}

@test "_keyring_prompts_for_passphrase: test backend is silent" {
  KEYRING_BACKEND=test
  run _keyring_prompts_for_passphrase
  [ "$status" -eq 1 ]
}

@test "_keyring_prompts_for_passphrase: file and os backends prompt" {
  KEYRING_BACKEND=file
  run _keyring_prompts_for_passphrase
  [ "$status" -eq 0 ]
  KEYRING_BACKEND=os
  run _keyring_prompts_for_passphrase
  [ "$status" -eq 0 ]
}

@test "_keyring_prompts_for_passphrase: unset defaults to silent" {
  unset KEYRING_BACKEND
  run _keyring_prompts_for_passphrase
  [ "$status" -eq 1 ]
}

@test "resolve_keyring_backend: explicit flag wins over client.toml" {
  local home; home=$(mktemp -d)
  mkdir -p "$home/config"
  printf 'keyring-backend = "file"\n' > "$home/config/client.toml"
  KEYRING_BACKEND=os
  KEYRING_BACKEND_EXPLICIT=1
  HOME_DIR="$home"
  KEYRING_DIR=""
  resolve_keyring_backend
  [ "$KEYRING_BACKEND" = "os" ]
}

@test "resolve_keyring_backend: reads value from client.toml" {
  local home; home=$(mktemp -d)
  mkdir -p "$home/config"
  printf 'chain-id = "x"\nkeyring-backend = "file"\noutput = "text"\n' > "$home/config/client.toml"
  KEYRING_BACKEND=test
  KEYRING_BACKEND_EXPLICIT=0
  HOME_DIR="$home"
  KEYRING_DIR=""
  resolve_keyring_backend
  [ "$KEYRING_BACKEND" = "file" ]
}

@test "resolve_keyring_backend: client.toml wins over on-disk keyring-test dir" {
  local home; home=$(mktemp -d)
  mkdir -p "$home/config" "$home/keyring-test"
  printf 'keyring-backend = "os"\n' > "$home/config/client.toml"
  KEYRING_BACKEND=test
  KEYRING_BACKEND_EXPLICIT=0
  HOME_DIR="$home"
  KEYRING_DIR=""
  resolve_keyring_backend
  [ "$KEYRING_BACKEND" = "os" ]
}

@test "resolve_keyring_backend: detects test from keyring-test dir" {
  local home; home=$(mktemp -d)
  mkdir -p "$home/keyring-test"
  KEYRING_BACKEND=""
  KEYRING_BACKEND_EXPLICIT=0
  HOME_DIR="$home"
  KEYRING_DIR=""
  resolve_keyring_backend
  [ "$KEYRING_BACKEND" = "test" ]
}

@test "resolve_keyring_backend: detects file from keyring-file dir" {
  local home; home=$(mktemp -d)
  mkdir -p "$home/keyring-file"
  KEYRING_BACKEND=""
  KEYRING_BACKEND_EXPLICIT=0
  HOME_DIR="$home"
  KEYRING_DIR=""
  resolve_keyring_backend
  [ "$KEYRING_BACKEND" = "file" ]
}

@test "resolve_keyring_backend: uses --keyring-dir for detection" {
  local home kr; home=$(mktemp -d); kr=$(mktemp -d)
  mkdir -p "$kr/keyring-file"
  KEYRING_BACKEND=""
  KEYRING_BACKEND_EXPLICIT=0
  HOME_DIR="$home"
  KEYRING_DIR="$kr"
  resolve_keyring_backend
  [ "$KEYRING_BACKEND" = "file" ]
}

@test "resolve_keyring_backend: client.toml read from --home even when keyring-dir differs" {
  local home kr; home=$(mktemp -d); kr=$(mktemp -d)
  mkdir -p "$home/config"
  printf 'keyring-backend = "file"\n' > "$home/config/client.toml"
  KEYRING_BACKEND=""
  KEYRING_BACKEND_EXPLICIT=0
  HOME_DIR="$home"
  KEYRING_DIR="$kr"
  resolve_keyring_backend
  [ "$KEYRING_BACKEND" = "file" ]
}

@test "resolve_keyring_backend: empty home falls back to os" {
  local home; home=$(mktemp -d)
  KEYRING_BACKEND=""
  KEYRING_BACKEND_EXPLICIT=0
  HOME_DIR="$home"
  KEYRING_DIR=""
  resolve_keyring_backend
  [ "$KEYRING_BACKEND" = "os" ]
}
