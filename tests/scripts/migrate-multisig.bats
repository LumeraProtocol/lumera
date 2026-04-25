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

# --- submit ---------------------------------------------------------------
# submit-proof no longer takes --from — migration messages are zero-signer
# (authorization is embedded in legacy_proof/new_proof; fees waived by ante).
@test "submit dry-run exits 0 without broadcasting (claim)" {
  local state_dir; state_dir=$(mktemp -d)
  local state_file="$state_dir/state"
  run env \
    SHIM_STATE_FILE="$state_file" \
    SHIM_ESTIMATE_FIXTURE=estimate-multisig \
    "$SCRIPTS_DIR/migrate-multisig.sh" submit "$FIX_DIR/combined-tx.json" \
      --binary "$SHIM" \
      --chain-id shim-test \
      --node tcp://local:1 \
      --yes --dry-run
  [ "$status" -eq 0 ]
  [ ! -f "$state_file" ]
  rm -rf "$state_dir"
}

@test "submit happy path (broadcast + verify) exits 0" {
  local state_dir; state_dir=$(mktemp -d)
  local state_file="$state_dir/state"
  run env \
    SHIM_STATE_FILE="$state_file" \
    SHIM_ESTIMATE_FIXTURE=estimate-multisig \
    SHIM_RECORD_AFTER_FIXTURE=record-post-migration \
    SHIM_BANK_AFTER_FIXTURE=bank-balances-empty \
    "$SCRIPTS_DIR/migrate-multisig.sh" submit "$FIX_DIR/combined-tx.json" \
      --binary "$SHIM" \
      --chain-id shim-test \
      --node tcp://local:1 \
      --yes
  [ "$status" -eq 0 ]
  [[ "$output" == *"migration complete"* ]]
  rm -rf "$state_dir"
}

@test "submit rejects non-multisig tx JSON (exit 3)" {
  run "$SCRIPTS_DIR/migrate-multisig.sh" submit "$FIX_DIR/combined-tx-single.json" \
      --binary "$SHIM" \
      --chain-id shim-test \
      --node tcp://local:1 \
      --yes --dry-run
  [ "$status" -eq 3 ]
  [[ "$output" == *"multisig"*"multisig"* ]]
}

@test "submit rejects --from as unknown flag (exit 1)" {
  run "$SCRIPTS_DIR/migrate-multisig.sh" submit "$FIX_DIR/combined-tx.json" \
      --binary "$SHIM" \
      --from some-key \
      --chain-id shim-test \
      --node tcp://local:1 \
      --yes --dry-run
  [ "$status" -eq 1 ]
  [[ "$output" == *"unknown flag"* ]]
}

@test "submit aborts with exit 4 when estimate flips to would_succeed=false" {
  run env SHIM_ESTIMATE_FIXTURE=estimate-multisig-rejected \
    "$SCRIPTS_DIR/migrate-multisig.sh" submit "$FIX_DIR/combined-tx.json" \
      --binary "$SHIM" \
      --chain-id shim-test \
      --node tcp://local:1 \
      --yes --dry-run
  [ "$status" -eq 4 ]
}

@test "submit validator kind without --i-have-stopped-the-node in non-TTY exits 10" {
  local tmp; tmp=$(mktemp -d)
  jq '.body.messages[0]."@type" = "/lumera.evmigration.MsgMigrateValidator"' \
    "$FIX_DIR/combined-tx.json" > "$tmp/tx.json"
  run env SHIM_ESTIMATE_FIXTURE=estimate-multisig-validator \
    "$SCRIPTS_DIR/migrate-multisig.sh" submit "$tmp/tx.json" \
      --binary "$SHIM" \
      --chain-id shim-test \
      --node tcp://local:1 \
      --yes --dry-run </dev/null
  [ "$status" -eq 10 ]
  [[ "$output" == *"node"* ]] || [[ "$output" == *"downtime"* ]]
  rm -rf "$tmp"
}

@test "submit validator kind with --i-have-stopped-the-node proceeds" {
  local tmp; tmp=$(mktemp -d)
  jq '.body.messages[0]."@type" = "/lumera.evmigration.MsgMigrateValidator"' \
    "$FIX_DIR/combined-tx.json" > "$tmp/tx.json"
  run env SHIM_ESTIMATE_FIXTURE=estimate-multisig-validator \
    "$SCRIPTS_DIR/migrate-multisig.sh" submit "$tmp/tx.json" \
      --binary "$SHIM" \
      --chain-id shim-test \
      --node tcp://local:1 \
      --yes --dry-run --i-have-stopped-the-node
  [ "$status" -eq 0 ]
  rm -rf "$tmp"
}

@test "submit exits 1 with no positional" {
  run "$SCRIPTS_DIR/migrate-multisig.sh" submit \
    --binary "$SHIM" --chain-id shim --node tcp://local:1
  [ "$status" -eq 1 ]
}

# --- generate -------------------------------------------------------------
# generate now requires --new-sub-pub-keys + --new-threshold (multisig-only
# wrapper). Keyring flags are now accepted (the CLI needs keyring access to
# resolve key-names passed in --new-sub-pub-keys).
@test "generate writes proof.json on happy path (multisig, claim)" {
  local tmp; tmp=$(mktemp -d)
  run env SHIM_AUTH_TYPE=multisig SHIM_ESTIMATE_FIXTURE=estimate-multisig \
    "$SCRIPTS_DIR/migrate-multisig.sh" generate \
    --binary "$SHIM" \
    --legacy lumera1shimaddr1qxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx \
    --new    lumera1newshimaddrxxxxxxxxxxxxxxxxxxxxxxxxxxxx \
    --new-sub-pub-keys k1,k2,k3 \
    --new-threshold 2 \
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
    --new-sub-pub-keys k1,k2,k3 \
    --new-threshold 2 \
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
    --legacy lumera1x --new lumera1y \
    --new-sub-pub-keys k1,k2,k3 --new-threshold 2 --kind claim \
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

@test "generate aborts when --new-sub-pub-keys is missing (exit 1)" {
  run "$SCRIPTS_DIR/migrate-multisig.sh" generate \
    --binary "$SHIM" \
    --legacy lumera1x --new lumera1y \
    --new-threshold 2 --kind claim \
    --chain-id shim --node tcp://local:1 --out /tmp/unused.json
  [ "$status" -eq 1 ]
  [[ "$output" == *"new-sub-pub-keys"* ]]
}

@test "generate aborts when --new-threshold is missing (exit 1)" {
  run "$SCRIPTS_DIR/migrate-multisig.sh" generate \
    --binary "$SHIM" \
    --legacy lumera1x --new lumera1y \
    --new-sub-pub-keys k1,k2,k3 --kind claim \
    --chain-id shim --node tcp://local:1 --out /tmp/unused.json
  [ "$status" -eq 1 ]
  [[ "$output" == *"new-threshold"* ]]
}

@test "generate exits 8 when multisig pubkey is nil on-chain" {
  run env SHIM_AUTH_TYPE=nilpubkey \
    "$SCRIPTS_DIR/migrate-multisig.sh" generate \
    --binary "$SHIM" \
    --legacy lumera1nilpubkey1qxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx \
    --new lumera1newshimaddrxxxxxxxxxxxxxxxxxxxxxxxxxxxx \
    --new-sub-pub-keys k1,k2,k3 --new-threshold 2 \
    --kind claim --chain-id shim-test --node tcp://local:1 \
    --out /tmp/unused.json
  [ "$status" -eq 8 ]
  [[ "$output" == *"seed"* ]]
}

@test "generate exits 3 when account is single-sig" {
  run env SHIM_AUTH_TYPE=single \
    "$SCRIPTS_DIR/migrate-multisig.sh" generate \
    --binary "$SHIM" \
    --legacy lumera1x --new lumera1y \
    --new-sub-pub-keys k1,k2,k3 --new-threshold 2 --kind claim \
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
    --new-sub-pub-keys k1,k2,k3 --new-threshold 2 \
    --kind validator \
    --chain-id shim-test --node tcp://local:1 \
    --out /tmp/unused.json
  [ "$status" -eq 6 ]
  [[ "$output" == *"validator"* ]]
}

@test "generate rejects duplicate --new-sub-pub-keys (propagates from CLI)" {
  # The shim mirrors the real Go CLI's validateSideSpec duplicate-sub-key
  # rejection when --new-sub-pub-keys contains the same entry twice. This
  # bats test pins that the wrapper propagates the exit code + stderr cleanly,
  # complementing the unit-level Go coverage in
  # x/evmigration/client/cli/tx_multisig_internal_test.go::TestValidateSideSpec_RejectsDuplicateSubKeys.
  run env SHIM_AUTH_TYPE=multisig SHIM_ESTIMATE_FIXTURE=estimate-multisig \
    "$SCRIPTS_DIR/migrate-multisig.sh" generate \
    --binary "$SHIM" \
    --legacy lumera1shimaddr1qxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx \
    --new lumera1newshimaddrxxxxxxxxxxxxxxxxxxxxxxxxxxxx \
    --new-sub-pub-keys k1,k2,k1 \
    --new-threshold 2 \
    --kind claim \
    --chain-id shim-test --node tcp://local:1 \
    --out /tmp/unused.json
  [ "$status" -ne 0 ]
  [[ "$output" == *"sub_pub_keys[2] duplicates sub_pub_keys[0]"* ]]
}

@test "generate aborts with exit 4 on estimate would_succeed=false" {
  run env SHIM_AUTH_TYPE=multisig SHIM_ESTIMATE_FIXTURE=estimate-multisig-rejected \
    "$SCRIPTS_DIR/migrate-multisig.sh" generate \
    --binary "$SHIM" \
    --legacy lumera1shimaddr1qxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx \
    --new lumera1newshimaddrxxxxxxxxxxxxxxxxxxxxxxxxxxxx \
    --new-sub-pub-keys k1,k2,k3 --new-threshold 2 \
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
    --new-sub-pub-keys k1,k2,k3 --new-threshold 2 \
    --kind claim --chain-id shim-test --node tcp://local:1 \
    --out /tmp/unused.json
  [ "$status" -eq 5 ]
}

# --- sign -----------------------------------------------------------------
# sign now accepts either --from (legacy sub-key) or --new-key (eth sub-key)
# or both. At least one is required.
@test "sign happy path writes a partial (alice-sub in legacy set)" {
  local tmp; tmp=$(mktemp -d)
  cp "$FIX_DIR/proof-template.json" "$tmp/proof.json"
  run "$SCRIPTS_DIR/migrate-multisig.sh" sign "$tmp/proof.json" \
      --binary "$SHIM" \
      --from alice-sub \
      --chain-id shim-test \
      --out "$tmp/alice-partial.json"
  [ "$status" -eq 0 ]
  [ -f "$tmp/alice-partial.json" ]
  rm -rf "$tmp"
}

@test "sign rejects tampered payload_hex (exit 9)" {
  local tmp; tmp=$(mktemp -d)
  jq '.payload_hex = "00"' "$FIX_DIR/proof-template.json" > "$tmp/bad.json"
  run "$SCRIPTS_DIR/migrate-multisig.sh" sign "$tmp/bad.json" \
      --binary "$SHIM" \
      --from alice-sub \
      --chain-id shim-test \
      --out "$tmp/out.json"
  [ "$status" -eq 9 ]
  [[ "$output" == *"payload_hex"* ]]
  rm -rf "$tmp"
}

@test "sign rejects v1-shape proof file (unsupported version)" {
  local tmp; tmp=$(mktemp -d)
  # v1-style proof: no version, top-level multisig, flat partial_signatures.
  # read_proof_file gates on version=2.
  cat > "$tmp/v1.json" <<'EOF'
{
  "kind": "claim",
  "legacy_address": "lumera1",
  "new_address": "lumera2",
  "chain_id": "shim",
  "evm_chain_id": "76857769",
  "payload_hex": "0000",
  "multisig": {"threshold": 2, "sub_pub_keys_b64": ["x"], "sig_format": "SIG_FORMAT_CLI"},
  "partial_signatures": []
}
EOF
  run "$SCRIPTS_DIR/migrate-multisig.sh" sign "$tmp/v1.json" \
      --binary "$SHIM" \
      --from alice-sub \
      --chain-id shim-test \
      --out "$tmp/out.json"
  [ "$status" -eq 9 ]
  [[ "$output" == *"unsupported version"* ]]
  rm -rf "$tmp"
}

@test "sign rejects --from not in legacy sub-key set (exit 1)" {
  local tmp; tmp=$(mktemp -d)
  cp "$FIX_DIR/proof-template.json" "$tmp/proof.json"
  run "$SCRIPTS_DIR/migrate-multisig.sh" sign "$tmp/proof.json" \
      --binary "$SHIM" \
      --from wrong-sub \
      --chain-id shim-test \
      --out "$tmp/out.json"
  [ "$status" -eq 1 ]
  [[ "$output" == *"legacy.sub_pub_keys"* ]]
  rm -rf "$tmp"
}

@test "sign rejects eth_secp256k1 key as --from (exit 1)" {
  local tmp; tmp=$(mktemp -d)
  cp "$FIX_DIR/proof-template.json" "$tmp/proof.json"
  run "$SCRIPTS_DIR/migrate-multisig.sh" sign "$tmp/proof.json" \
      --binary "$SHIM" \
      --from new-eth-key \
      --chain-id shim-test \
      --out "$tmp/out.json"
  [ "$status" -eq 1 ]
  [[ "$output" == *"secp256k1"* ]]
  rm -rf "$tmp"
}

@test "sign exits 1 when neither --from nor --new-key is set" {
  run "$SCRIPTS_DIR/migrate-multisig.sh" sign "$FIX_DIR/proof-template.json" \
    --binary "$SHIM" --chain-id shim --out /tmp/unused.json
  [ "$status" -eq 1 ]
  [[ "$output" == *"--from"* ]] || [[ "$output" == *"--new-key"* ]]
}

@test "sign exits 1 with no positional argument" {
  run "$SCRIPTS_DIR/migrate-multisig.sh" sign \
    --binary "$SHIM" --from alice-sub --chain-id shim --out /tmp/unused.json
  [ "$status" -eq 1 ]
}

@test "sign exits 1 with multiple positional arguments" {
  run "$SCRIPTS_DIR/migrate-multisig.sh" sign \
    "$FIX_DIR/proof-template.json" "$FIX_DIR/partial-alice.json" \
    --binary "$SHIM" --from alice-sub --chain-id shim --out /tmp/unused.json
  [ "$status" -eq 1 ]
}

# --- combine --------------------------------------------------------------
@test "combine happy path assembles tx.json with 2 of 3 partials" {
  local tmp; tmp=$(mktemp -d)
  run "$SCRIPTS_DIR/migrate-multisig.sh" combine \
    "$FIX_DIR/partial-alice.json" "$FIX_DIR/partial-bob.json" \
    --binary "$SHIM" \
    --out "$tmp/tx.json"
  [ "$status" -eq 0 ]
  [ -f "$tmp/tx.json" ]
  [[ "$output" == *"Legacy threshold satisfied: yes"* ]]
  [[ "$output" == *"New threshold satisfied: yes"* ]]
  rm -rf "$tmp"
}

@test "combine exits 4 when fewer than K entries (single partial, K=2)" {
  local tmp; tmp=$(mktemp -d)
  run "$SCRIPTS_DIR/migrate-multisig.sh" combine \
    "$FIX_DIR/partial-alice.json" \
    --binary "$SHIM" \
    --out "$tmp/tx.json"
  [ "$status" -eq 4 ]
  [[ "$output" == *"threshold satisfied: no"* ]]
  [ ! -f "$tmp/tx.json" ]
  rm -rf "$tmp"
}

@test "combine exits 9 on cross-file chain_id mismatch" {
  local tmp; tmp=$(mktemp -d)
  local payload ph
  payload='lumera-evm-migration:different-chain:76857769:claim:lumera1shimaddr1qxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx:lumera1newshimaddrxxxxxxxxxxxxxxxxxxxxxxxxxxxx'
  ph=$(printf '%s' "$payload" | od -An -tx1 -v | tr -d ' \n')
  jq --arg ph "$ph" '.chain_id = "different-chain" | .payload_hex = $ph' \
    "$FIX_DIR/partial-bob.json" > "$tmp/bob-bad.json"
  run "$SCRIPTS_DIR/migrate-multisig.sh" combine \
    "$FIX_DIR/partial-alice.json" "$tmp/bob-bad.json" \
    --binary "$SHIM" \
    --out "$tmp/tx.json"
  [ "$status" -eq 9 ]
  [[ "$output" == *"chain_id"* ]]
  rm -rf "$tmp"
}

@test "combine exits 4 when lumerad reports below-threshold valid sigs" {
  local tmp; tmp=$(mktemp -d)
  run env SHIM_COMBINE_EXIT=1 SHIM_STDERR="Error: need 2 valid partial signatures on legacy side, have 1" \
    "$SCRIPTS_DIR/migrate-multisig.sh" combine \
      "$FIX_DIR/partial-alice.json" "$FIX_DIR/partial-bob.json" \
      --binary "$SHIM" \
      --out "$tmp/tx.json"
  [ "$status" -eq 4 ]
  rm -rf "$tmp"
}

@test "combine exits 1 with no partials" {
  run "$SCRIPTS_DIR/migrate-multisig.sh" combine \
    --binary "$SHIM" --out /tmp/unused.json
  [ "$status" -eq 1 ]
}

@test "combine exits 1 without --out" {
  run "$SCRIPTS_DIR/migrate-multisig.sh" combine \
    "$FIX_DIR/partial-alice.json" "$FIX_DIR/partial-bob.json" \
    --binary "$SHIM"
  [ "$status" -eq 1 ]
}

@test "combine passes file list through to lumerad" {
  local tmp; tmp=$(mktemp -d)
  run "$SCRIPTS_DIR/migrate-multisig.sh" combine \
    "$FIX_DIR/partial-alice.json" "$FIX_DIR/partial-bob.json" "$FIX_DIR/partial-carol.json" \
    --binary "$SHIM" \
    --out "$tmp/tx.json"
  [ "$status" -eq 0 ]
  [ -f "$tmp/tx.json" ]
  rm -rf "$tmp"
}
