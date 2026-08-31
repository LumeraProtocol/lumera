#!/usr/bin/env bats
###################################################################################
# Tests for `scripts/migrate-batch.sh report`.
#
# `report` is the offline classification subcommand. It does NOT touch the
# chain and does NOT shell out to `lumerad`, so these tests do not need to
# stub the binary — only `jq` is required, which is a hard dep of the
# repo's other scripts already.
#
# What we cover here:
#   1. The parser/classifier matches the expected target counts on a
#      well-formed multi-bucket file.
#   2. Signer order is matched by PUBKEY equality, never by name suffix.
#      This is the single most important correctness invariant in the
#      whole driver: a wrong order silently derives the wrong multisig
#      address and would broadcast nonsense onto the chain.
#   3. Malformed JSON / structurally-broken entries are rejected (exit 9).
#   4. A multisig referencing an unknown signer pubkey is rejected (exit 9).
#   5. A `local` entry not referenced by any `multi` is classified as a
#      standalone single-sig migration target.
#   6. --plan-out produces a parseable JSON plan whose `targets` array
#      matches the human report.
###################################################################################

setup() {
  SCRIPTS_DIR="$(cd "$BATS_TEST_DIRNAME/../../scripts" && pwd)"
  MIGRATE_BATCH="$SCRIPTS_DIR/migrate-batch.sh"
  TMPDIR="$(mktemp -d)"
}

teardown() {
  rm -rf -- "$TMPDIR"
}

###############################################################################
# Fixture helpers
###############################################################################

# write_fixture_simple <out>
# A 1-multisig (2-of-3) fixture whose signers are in canonical order [1,2,3]
# AND whose name suffixes happen to agree with the canonical order, so this
# is the "easy" case.
write_fixture_simple() {
  local out="$1"
  cat >"$out" <<'JSON'
{
  "team_1_1": {
    "address": "lumera1signer1",
    "mnemonic": "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon art",
    "pubkey": "{\"@type\":\"/cosmos.crypto.secp256k1.PubKey\",\"key\":\"AAAA1\"}",
    "type": "local"
  },
  "team_1_2": {
    "address": "lumera1signer2",
    "mnemonic": "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon art",
    "pubkey": "{\"@type\":\"/cosmos.crypto.secp256k1.PubKey\",\"key\":\"AAAA2\"}",
    "type": "local"
  },
  "team_1_3": {
    "address": "lumera1signer3",
    "mnemonic": "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon art",
    "pubkey": "{\"@type\":\"/cosmos.crypto.secp256k1.PubKey\",\"key\":\"AAAA3\"}",
    "type": "local"
  },
  "team_1": {
    "address": "lumera1multi",
    "mnemonic": "",
    "pubkey": "{\"@type\":\"/cosmos.crypto.multisig.LegacyAminoPubKey\",\"threshold\":2,\"public_keys\":[{\"@type\":\"/cosmos.crypto.secp256k1.PubKey\",\"key\":\"AAAA1\"},{\"@type\":\"/cosmos.crypto.secp256k1.PubKey\",\"key\":\"AAAA2\"},{\"@type\":\"/cosmos.crypto.secp256k1.PubKey\",\"key\":\"AAAA3\"}]}",
    "type": "multi"
  }
}
JSON
}

# write_fixture_mixed <out>
# 2 multisigs with NON-SEQUENTIAL signer orders (this is the real foundation
# file shape — 23 of 28 multisigs have non-sequential orderings), plus one
# standalone single-sig.
write_fixture_mixed() {
  local out="$1"
  cat >"$out" <<'JSON'
{
  "seed_sale_1_1": {"address":"lumera1s11","mnemonic":"m1","pubkey":"{\"@type\":\"/cosmos.crypto.secp256k1.PubKey\",\"key\":\"K1\"}","type":"local"},
  "seed_sale_1_2": {"address":"lumera1s12","mnemonic":"m2","pubkey":"{\"@type\":\"/cosmos.crypto.secp256k1.PubKey\",\"key\":\"K2\"}","type":"local"},
  "seed_sale_1_3": {"address":"lumera1s13","mnemonic":"m3","pubkey":"{\"@type\":\"/cosmos.crypto.secp256k1.PubKey\",\"key\":\"K3\"}","type":"local"},
  "seed_sale_1":   {"address":"lumera1ms1","mnemonic":"","pubkey":"{\"@type\":\"/cosmos.crypto.multisig.LegacyAminoPubKey\",\"threshold\":2,\"public_keys\":[{\"@type\":\"/cosmos.crypto.secp256k1.PubKey\",\"key\":\"K2\"},{\"@type\":\"/cosmos.crypto.secp256k1.PubKey\",\"key\":\"K3\"},{\"@type\":\"/cosmos.crypto.secp256k1.PubKey\",\"key\":\"K1\"}]}","type":"multi"},

  "seed_sale_2_1": {"address":"lumera1s21","mnemonic":"m4","pubkey":"{\"@type\":\"/cosmos.crypto.secp256k1.PubKey\",\"key\":\"K4\"}","type":"local"},
  "seed_sale_2_2": {"address":"lumera1s22","mnemonic":"m5","pubkey":"{\"@type\":\"/cosmos.crypto.secp256k1.PubKey\",\"key\":\"K5\"}","type":"local"},
  "seed_sale_2_3": {"address":"lumera1s23","mnemonic":"m6","pubkey":"{\"@type\":\"/cosmos.crypto.secp256k1.PubKey\",\"key\":\"K6\"}","type":"local"},
  "seed_sale_2":   {"address":"lumera1ms2","mnemonic":"","pubkey":"{\"@type\":\"/cosmos.crypto.multisig.LegacyAminoPubKey\",\"threshold\":2,\"public_keys\":[{\"@type\":\"/cosmos.crypto.secp256k1.PubKey\",\"key\":\"K6\"},{\"@type\":\"/cosmos.crypto.secp256k1.PubKey\",\"key\":\"K5\"},{\"@type\":\"/cosmos.crypto.secp256k1.PubKey\",\"key\":\"K4\"}]}","type":"multi"},

  "lone_wolf":     {"address":"lumera1lone","mnemonic":"m7","pubkey":"{\"@type\":\"/cosmos.crypto.secp256k1.PubKey\",\"key\":\"K7\"}","type":"local"}
}
JSON
}

write_fixture_standalone() {
  local out="$1"
  cat >"$out" <<'JSON'
{
  "standalone": {
    "address": "lumera1standalone",
    "mnemonic": "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon art",
    "pubkey": "{\"@type\":\"/cosmos.crypto.secp256k1.PubKey\",\"key\":\"STANDALONE\"}",
    "type": "local"
  }
}
JSON
}

write_lumerad_status_shim() {
  local out="$1"
  cat >"$out" <<'SH'
#!/usr/bin/env bash
set -euo pipefail

if [[ "$*" == *"--help"* ]]; then
  exit 0
fi

if [[ "${1-}" == "query" && "${2-}" == "evmigration" && "${3-}" == "migration-record" ]]; then
  exit 1
fi

if [[ "${1-}" == "query" && "${2-}" == "bank" && "${3-}" == "balances" ]]; then
  printf '{"balances":[{"denom":"ulume","amount":"%s"}]}\n' "${FAKE_BALANCE_ULUME:-500000}"
  exit 0
fi

if [[ "${1-}" == "query" && "${2-}" == "bank" && "${3-}" == "spendable-balances" ]]; then
  printf '{"balances":[{"denom":"ulume","amount":"%s"}]}\n' "${FAKE_SPENDABLE_ULUME:-0}"
  exit 0
fi

if [[ "${1-}" == "query" && "${2-}" == "auth" && "${3-}" == "account" ]]; then
  printf '{"account":{"@type":"/cosmos.auth.v1beta1.BaseAccount","address":"%s"}}\n' "${4-}"
  exit 0
fi

printf 'unexpected lumerad shim args: %s\n' "$*" >&2
exit 1
SH
  chmod +x "$out"
}

###############################################################################
# Tests
###############################################################################

@test "report: simple 1-multisig fixture reports correct totals" {
  local fix="$TMPDIR/fix.json"
  local plan="$TMPDIR/plan.json"
  write_fixture_simple "$fix"

  run "$MIGRATE_BATCH" report --mnemonics "$fix" --plan-out "$plan"
  [ "$status" -eq 0 ]

  local totals
  totals="$(jq -c '.totals' "$plan")"
  [ "$(jq -r '.multis' <<<"$totals")"             = "1" ]
  [ "$(jq -r '.standalone_singles' <<<"$totals")" = "0" ]
  [ "$(jq -r '.targets' <<<"$totals")"            = "1" ]
}

@test "report: signer order is matched by pubkey, not by name suffix" {
  local fix="$TMPDIR/fix.json"
  local plan="$TMPDIR/plan.json"
  write_fixture_mixed "$fix"

  run "$MIGRATE_BATCH" report --mnemonics "$fix" --plan-out "$plan"
  [ "$status" -eq 0 ]

  # seed_sale_1's public_keys order is [K2, K3, K1] which maps to signer
  # names [seed_sale_1_2, seed_sale_1_3, seed_sale_1_1]. If the driver were
  # using name-suffix order it would have produced [_1, _2, _3] instead.
  local order
  order=$(jq -r '
    .targets[]
    | select(.name == "seed_sale_1")
    | .signer_names
    | join(",")' "$plan")
  [ "$order" = "seed_sale_1_2,seed_sale_1_3,seed_sale_1_1" ]

  # seed_sale_2's public_keys order is [K6, K5, K4] (full reversal).
  order=$(jq -r '
    .targets[]
    | select(.name == "seed_sale_2")
    | .signer_names
    | join(",")' "$plan")
  [ "$order" = "seed_sale_2_3,seed_sale_2_2,seed_sale_2_1" ]
}

@test "report: unreferenced local is classified as a standalone single-sig target" {
  local fix="$TMPDIR/fix.json"
  local plan="$TMPDIR/plan.json"
  write_fixture_mixed "$fix"

  run "$MIGRATE_BATCH" report --mnemonics "$fix" --plan-out "$plan"
  [ "$status" -eq 0 ]

  # Mixed fixture has 2 multisigs + 1 standalone single-sig = 3 targets.
  [ "$(jq -r '.totals.targets' "$plan")"            = "3" ]
  [ "$(jq -r '.totals.multis' "$plan")"             = "2" ]
  [ "$(jq -r '.totals.standalone_singles' "$plan")" = "1" ]

  local standalone
  standalone=$(jq -r '.targets[] | select(.kind == "single-sig") | .name' "$plan")
  [ "$standalone" = "lone_wolf" ]
}

@test "report: rejects mnemonics file with non-object top level (exit 9)" {
  local fix="$TMPDIR/fix.json"
  printf '[]\n' >"$fix"

  run "$MIGRATE_BATCH" report --mnemonics "$fix"
  [ "$status" -eq 9 ]
}

@test "report: rejects entry with unknown type (exit 9)" {
  local fix="$TMPDIR/fix.json"
  cat >"$fix" <<'JSON'
{
  "broken": {
    "address": "lumera1broken",
    "mnemonic": "",
    "pubkey": "{}",
    "type": "weird-thing"
  }
}
JSON

  run "$MIGRATE_BATCH" report --mnemonics "$fix"
  [ "$status" -eq 9 ]
}

@test "report: rejects entry with non-JSON-object pubkey (exit 9)" {
  # Operators occasionally hand-edit the mnemonics file and end up with a
  # pubkey field that is a stray string instead of a serialized JSON object.
  # The classifier must catch this structurally (exit 9 with the entry name),
  # NOT let jq blow up later inside _mb_load_plan's pubkey | fromjson step
  # (which would surface as a jq-RC exit with no actionable error).
  local fix="$TMPDIR/fix.json"
  cat >"$fix" <<'JSON'
{
  "bad_pubkey": {
    "address": "lumera1bad",
    "mnemonic": "",
    "pubkey": "not-json-at-all",
    "type": "local"
  }
}
JSON

  run "$MIGRATE_BATCH" report --mnemonics "$fix"
  [ "$status" -eq 9 ]
  [[ "$output" == *"bad_pubkey"* ]]
}

@test "report: rejects entry missing address (exit 9)" {
  local fix="$TMPDIR/fix.json"
  cat >"$fix" <<'JSON'
{
  "no_addr": {
    "mnemonic": "",
    "pubkey": "{}",
    "type": "local"
  }
}
JSON

  run "$MIGRATE_BATCH" report --mnemonics "$fix"
  [ "$status" -eq 9 ]
}

@test "report: rejects multisig referencing unknown signer pubkey (exit 9)" {
  local fix="$TMPDIR/fix.json"
  cat >"$fix" <<'JSON'
{
  "only_signer": {
    "address": "lumera1one",
    "mnemonic": "m",
    "pubkey": "{\"@type\":\"/cosmos.crypto.secp256k1.PubKey\",\"key\":\"K1\"}",
    "type": "local"
  },
  "orphan_multi": {
    "address": "lumera1orphan",
    "mnemonic": "",
    "pubkey": "{\"@type\":\"/cosmos.crypto.multisig.LegacyAminoPubKey\",\"threshold\":1,\"public_keys\":[{\"@type\":\"/cosmos.crypto.secp256k1.PubKey\",\"key\":\"K1\"},{\"@type\":\"/cosmos.crypto.secp256k1.PubKey\",\"key\":\"K_NOT_PRESENT\"}]}",
    "type": "multi"
  }
}
JSON

  run "$MIGRATE_BATCH" report --mnemonics "$fix"
  [ "$status" -eq 9 ]
  # Error message must name the offending multisig and the missing pubkey, so
  # the operator can fix the file directly.
  [[ "$output" == *"orphan_multi"* ]]
  [[ "$output" == *"K_NOT_PRESENT"* ]]
}

@test "report: missing --mnemonics is a usage error (exit 1)" {
  run "$MIGRATE_BATCH" report
  [ "$status" -eq 1 ]
}

@test "report: --mnemonics file not readable is exit 1" {
  run "$MIGRATE_BATCH" report --mnemonics "$TMPDIR/does-not-exist.json"
  [ "$status" -eq 1 ]
}

@test "report: --plan-out file is a parseable JSON object with targets[]" {
  local fix="$TMPDIR/fix.json"
  local plan="$TMPDIR/plan.json"
  write_fixture_mixed "$fix"

  run "$MIGRATE_BATCH" report --mnemonics "$fix" --plan-out "$plan"
  [ "$status" -eq 0 ]
  [ -s "$plan" ]
  run jq -e 'type == "object" and (.targets | type == "array")' "$plan"
  [ "$status" -eq 0 ]
}

@test "status: spendable below self-send amount plus fee requires funding" {
  local fix="$TMPDIR/fix.json"
  local shim="$TMPDIR/lumerad-shim"
  write_fixture_standalone "$fix"
  write_lumerad_status_shim "$shim"

  run env FAKE_SPENDABLE_ULUME=104999 "$MIGRATE_BATCH" status \
    --mnemonics "$fix" \
    --chain-id lumera-test \
    --binary "$shim"
  [ "$status" -eq 0 ]
  [[ "$output" == *"needs-funding"* ]]
  [[ "$output" == *"spendable=104999ulume"* ]]
}

@test "status: spendable equal to self-send amount plus fee can self-send" {
  local fix="$TMPDIR/fix.json"
  local shim="$TMPDIR/lumerad-shim"
  write_fixture_standalone "$fix"
  write_lumerad_status_shim "$shim"

  run env FAKE_SPENDABLE_ULUME=105000 "$MIGRATE_BATCH" status \
    --mnemonics "$fix" \
    --chain-id lumera-test \
    --binary "$shim"
  [ "$status" -eq 0 ]
  [[ "$output" == *"needs-pubkey"* ]]
  [[ "$output" == *"spendable=105000ulume"* ]]
}
