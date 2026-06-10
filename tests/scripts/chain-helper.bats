#!/usr/bin/env bats

setup() {
  SCRIPTS_DIR="$(cd "$BATS_TEST_DIRNAME/../../scripts" && pwd)"
  TMPDIR="$(mktemp -d)"
  FAKE_LUMERAD="$TMPDIR/lumerad"
  FAKE_GRPCURL="$TMPDIR/grpcurl"

  cat >"$FAKE_LUMERAD" <<'SH'
#!/usr/bin/env bash
set -euo pipefail

strip_flags() {
  local out=()
  while (($#)); do
    case "$1" in
      --node|--chain-id|--output|--page-limit)
        shift 2
        ;;
      --limit)
        printf 'unknown flag: --limit\n' >&2
        exit 64
        ;;
      *)
        out+=("$1")
        shift
        ;;
    esac
  done
  printf '%s\n' "${out[@]}"
}

cmd=("$@")
mapfile -t args < <(strip_flags "${cmd[@]}")

if [[ "${args[0]}" == "query" && "${args[1]}" == "staking" && "${args[2]}" == "validators" ]]; then
  cat <<'JSON'
{
  "validators": [
    {"operator_address": "lumeravaloper1alpha", "description": {"moniker": "alpha"}, "jailed": false},
    {"operator_address": "lumeravaloper1beta", "description": {"moniker": "beta"}, "jailed": true}
  ],
  "pagination": {"total": "2"}
}
JSON
  exit 0
fi

if [[ "${args[0]}" == "query" && "${args[1]}" == "auth" && "${args[2]}" == "accounts" ]]; then
  cat <<'JSON'
{"accounts": [{}], "pagination": {"total": "160021"}}
JSON
  exit 0
fi

if [[ "${args[0]}" == "query" && "${args[1]}" == "supernode" && "${args[2]}" == "list-supernodes" ]]; then
  # Current state is the highest-height entry per supernode; states are
  # intentionally out of order to exercise max_by(height).
  cat <<'JSON'
{
  "supernodes": [
    {"validator_address": "lumeravaloper1alpha", "states": [
      {"state": "SUPERNODE_STATE_ACTIVE", "height": "10"},
      {"state": "SUPERNODE_STATE_POSTPONED", "height": "30"},
      {"state": "SUPERNODE_STATE_ACTIVE", "height": "20"}
    ]},
    {"validator_address": "lumeravaloper1beta", "states": [
      {"state": "SUPERNODE_STATE_ACTIVE", "height": "5"}
    ]},
    {"validator_address": "lumeravaloper1gamma", "states": [
      {"state": "SUPERNODE_STATE_DISABLED", "height": "7"},
      {"state": "SUPERNODE_STATE_ACTIVE", "height": "99"}
    ]}
  ],
  "pagination": {"total": "3"}
}
JSON
  exit 0
fi

if [[ "${args[0]}" == "debug" && "${args[1]}" == "addr" ]]; then
  case "${args[2]}" in
    lumeravaloper1alpha)
      printf 'Bech32 Acc: lumera1alpha\nBech32 Val: lumeravaloper1alpha\n'
      ;;
    lumeravaloper1beta)
      printf 'Bech32 Acc: lumera1beta\nBech32 Val: lumeravaloper1beta\n'
      ;;
    *)
      printf 'unknown address\n' >&2
      exit 1
      ;;
  esac
  exit 0
fi

if [[ "${args[0]}" == "query" && "${args[1]}" == "evmigration" && "${args[2]}" == "migration-estimate" ]]; then
  if [[ "${FAKE_EXACT_UNAVAILABLE:-0}" == "1" ]]; then
    printf 'unknown query route\n' >&2
    exit 1
  fi

  case "${args[3]}" in
    lumera1alpha)
      cat <<'JSON'
{"is_validator": true, "val_delegation_count": "12", "val_unbonding_count": "3", "val_redelegation_count": "5"}
JSON
      ;;
    lumera1beta)
      cat <<'JSON'
{"is_validator": true, "val_delegation_count": "20", "val_unbonding_count": "8", "val_redelegation_count": "5"}
JSON
      ;;
    *)
      printf '{}\n'
      ;;
  esac
  exit 0
fi

if [[ "${args[0]}" == "query" && "${args[1]}" == "evmigration" && "${args[2]}" == "params" ]]; then
  cat <<'JSON'
{"params": {"max_validator_delegations": "2000"}}
JSON
  exit 0
fi

if [[ "${args[0]}" == "query" && "${args[1]}" == "staking" && "${args[2]}" == "delegations-to" ]]; then
  # Single-element pages with a larger pagination.total: this models a real node
  # answering --page-count-total --page-limit 1, and guards against the regression
  # where the count came from the (truncated) array length instead of the total.
  case "${args[3]}" in
    lumeravaloper1alpha)
      cat <<'JSON'
{"delegation_responses": [{}], "pagination": {"total": "4"}}
JSON
      ;;
    lumeravaloper1beta)
      cat <<'JSON'
{"delegation_responses": [{}], "pagination": {"total": "7"}}
JSON
      ;;
  esac
  exit 0
fi

if [[ "${args[0]}" == "query" && "${args[1]}" == "staking" && "${args[2]}" == "unbonding-delegations-from" ]]; then
  case "${args[3]}" in
    lumeravaloper1alpha)
      # Empty set: a real node omits pagination.total here; the script must read 0.
      cat <<'JSON'
{"unbonding_responses": [], "pagination": {}}
JSON
      ;;
    lumeravaloper1beta)
      cat <<'JSON'
{"unbonding_responses": [{}], "pagination": {"total": "1"}}
JSON
      ;;
  esac
  exit 0
fi

printf 'unexpected command: %s\n' "$*" >&2
exit 1
SH
  chmod +x "$FAKE_LUMERAD"

  cat >"$FAKE_GRPCURL" <<'SH'
#!/usr/bin/env bash
set -euo pipefail

request=""
while (($#)); do
  case "$1" in
    -d)
      request="$2"
      shift 2
      ;;
    -max-time|-import-path|-proto|-protoset)
      shift 2
      ;;
    -plaintext|-insecure)
      shift
      ;;
    *)
      shift
      ;;
  esac
done

src="$(jq -r '.srcValidatorAddr // ""' <<<"$request")"
key="$(jq -r '.pagination.key // ""' <<<"$request")"

case "$src:$key" in
  lumeravaloper1alpha:)
    cat <<'JSON'
{
  "redelegationResponses": [
    {
      "redelegation": {
        "delegatorAddress": "lumera1delegator1",
        "validatorSrcAddress": "lumeravaloper1alpha",
        "validatorDstAddress": "lumeravaloper1beta"
      }
    }
  ]
}
JSON
    ;;
  lumeravaloper1beta:)
    cat <<'JSON'
{
  "redelegationResponses": [
    {
      "redelegation": {
        "delegatorAddress": "lumera1delegator2",
        "validatorSrcAddress": "lumeravaloper1beta",
        "validatorDstAddress": "lumeravaloper1alpha"
      }
    },
    {
      "redelegation": {
        "delegatorAddress": "lumera1delegator3",
        "validatorSrcAddress": "lumeravaloper1beta",
        "validatorDstAddress": "lumeravaloper1alpha"
      }
    }
  ]
}
JSON
    ;;
  *)
    printf '{"redelegationResponses":[]}\n'
    ;;
esac
SH
  chmod +x "$FAKE_GRPCURL"
}

teardown() {
  rm -rf "$TMPDIR"
}

@test "max-validator-delegations uses exact evmigration estimates" {
  run "$SCRIPTS_DIR/chain-helper.sh" max-validator-delegations \
    --binary "$FAKE_LUMERAD" \
    --grpcurl "$FAKE_GRPCURL" \
    --chain-id test-chain \
    --node tcp://test:26657 \
    --json

  [ "$status" -eq 0 ]
  echo "$output" | jq -e '
    .command == "max-validator-delegations"
    and .mode == "evmigration-estimate"
    and .exact == true
    and .max_observed == 33
    and .suggested_cap == 43
    and .validators[0].operator_address == "lumeravaloper1beta"
    and .validators[0].total == 33
  '
}

@test "max-validator-delegations uses staking and grpcurl on pre-evm chain" {
  run env FAKE_EXACT_UNAVAILABLE=1 \
    "$SCRIPTS_DIR/chain-helper.sh" max-validator-delegations \
    --binary "$FAKE_LUMERAD" \
    --grpcurl "$FAKE_GRPCURL" \
    --chain-id test-chain \
    --node tcp://test:26657 \
    --json

  [ "$status" -eq 0 ]
  echo "$output" | jq -e '
    .mode == "staking-pre-evm"
    and .exact == true
    and .max_observed == 11
    and .suggested_cap == 15
    and .validators[0].operator_address == "lumeravaloper1beta"
    and .validators[0].val_redelegation_count == 3
  '
}

@test "max-validator-delegations shows progress notes during human pre-evm scan" {
  run env FAKE_EXACT_UNAVAILABLE=1 \
    "$SCRIPTS_DIR/chain-helper.sh" max-validator-delegations \
    --binary "$FAKE_LUMERAD" \
    --grpcurl "$FAKE_GRPCURL" \
    --chain-id test-chain \
    --node tcp://test:26657

  [ "$status" -eq 0 ]
  [[ "$output" == *"INFO: found 2 validators"* ]]
  [[ "$output" == *"INFO: evmigration migration-estimate unavailable; using pre-EVM staking + gRPC redelegation scan"* ]]
  [[ "$output" != *"INFO: scanning redelegations 1/2"* ]]
  [[ "$output" == *"INFO: scanned redelegations 1/2: alpha count=1"* ]]
  [[ "$output" == *"INFO: scanned redelegations 2/2: beta count=2"* ]]
  [[ "$output" == *"INFO: counting staking records 1/2: alpha count=7 (deleg=4 unbond=0 redel=3)"* ]]
  [[ "$output" == *"INFO: counting staking records 2/2: beta count=11 (deleg=7 unbond=1 redel=3)"* ]]
  [[ "$output" == *"mode: staking-pre-evm"* ]]
}

@test "max-validator-delegations prints validator stats before summary in human output" {
  run env FAKE_EXACT_UNAVAILABLE=1 \
    "$SCRIPTS_DIR/chain-helper.sh" max-validator-delegations \
    --binary "$FAKE_LUMERAD" \
    --grpcurl "$FAKE_GRPCURL" \
    --chain-id test-chain \
    --node tcp://test:26657

  [ "$status" -eq 0 ]
  table_line="$(printf '%s\n' "$output" | grep -n '^rank ' | cut -d: -f1)"
  validator_line="$(printf '%s\n' "$output" | grep -n '^1[[:space:]]' | cut -d: -f1)"
  summary_line="$(printf '%s\n' "$output" | grep -n '^command: max-validator-delegations' | cut -d: -f1)"

  [ "$table_line" -lt "$summary_line" ]
  [ "$validator_line" -lt "$summary_line" ]
}

@test "max-validator-delegations refuses unsafe fallback when exact pre-evm redelegation scan is unavailable" {
  run env FAKE_EXACT_UNAVAILABLE=1 \
    "$SCRIPTS_DIR/chain-helper.sh" max-validator-delegations \
    --binary "$FAKE_LUMERAD" \
    --grpcurl "$TMPDIR/missing-grpcurl" \
    --chain-id test-chain \
    --node tcp://test:26657

  [ "$status" -eq 3 ]
  [[ "$output" == *"--allow-partial"* ]]
  [[ "$output" == *"grpcurl"* ]]
}

@test "max-validator-delegations can emit an explicit partial estimate" {
  run env FAKE_EXACT_UNAVAILABLE=1 \
    "$SCRIPTS_DIR/chain-helper.sh" max-validator-delegations \
    --binary "$FAKE_LUMERAD" \
    --grpcurl "$TMPDIR/missing-grpcurl" \
    --chain-id test-chain \
    --node tcp://test:26657 \
    --allow-partial \
    --json

  [ "$status" -eq 0 ]
  echo "$output" | jq -e '
    .mode == "staking-partial"
    and .exact == false
    and .max_observed == 8
    and .validators[0].operator_address == "lumeravaloper1beta"
    and (.warnings[0] | contains("not safe"))
  '
}

@test "stats reports accounts, validators, and supernode states (json)" {
  run "$SCRIPTS_DIR/chain-helper.sh" stats \
    --binary "$FAKE_LUMERAD" \
    --chain-id test-chain \
    --node tcp://test:26657 \
    --json

  [ "$status" -eq 0 ]
  echo "$output" | jq -e '
    .command == "stats"
    and .accounts.total == 160021
    and .validators.total == 2
    and .validators.jailed == 1
    and .validators.not_jailed == 1
    and .supernodes.total == 3
    and ((.supernodes.by_state[] | select(.state == "SUPERNODE_STATE_ACTIVE") | .count) == 2)
    and ((.supernodes.by_state[] | select(.state == "SUPERNODE_STATE_POSTPONED") | .count) == 1)
  '
}

@test "stats human output groups supernodes by current (highest-height) state" {
  run "$SCRIPTS_DIR/chain-helper.sh" stats \
    --binary "$FAKE_LUMERAD" \
    --chain-id test-chain \
    --node tcp://test:26657

  [ "$status" -eq 0 ]
  [[ "$output" == *"total: 160021"* ]]
  [[ "$output" == *"jailed:     1"* ]]
  [[ "$output" == *"SUPERNODE_STATE_ACTIVE: 2"* ]]
  [[ "$output" == *"SUPERNODE_STATE_POSTPONED: 1"* ]]
}
