#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  SCRIPT="$REPO_ROOT/devnet/scripts/supernode-setup.sh"
  SHARED_DIR="$(mktemp -d)"
  mkdir -p "$SHARED_DIR/config" "$SHARED_DIR/status/supernova_validator_2"
  cat >"$SHARED_DIR/config/config.json" <<'JSON'
{
  "chain": {
    "denom": {
      "bond": "ulume"
    }
  }
}
JSON
  cat >"$SHARED_DIR/config/validators.json" <<'JSON'
[
  {
    "moniker": "supernova_validator_2",
    "multisig": {
      "enabled": true,
      "threshold": 2
    }
  }
]
JSON
  cat >"$SHARED_DIR/status/supernova_validator_2/accounts.json" <<'JSON'
[
  {
    "name": "supernova_validator_2_key",
    "address": "lumera1locked"
  },
  {
    "name": "prepare-funder-supernova_validator_2",
    "address": "lumera1liquid"
  }
]
JSON
}

teardown() {
  rm -rf "$SHARED_DIR"
}

@test "supernode setup can be sourced without running main" {
  run env \
    MONIKER=supernova_validator_2 \
    SUPERNODE_SETUP_LIB_ONLY=1 \
    SUPERNODE_SHARED_DIR="$SHARED_DIR" \
    bash -c 'source "$1"; type supernode_funding_key_name' bash "$SCRIPT"

  [ "$status" -eq 0 ]
  [[ "$output" == *"supernode_funding_key_name is a function"* ]]
}

@test "multisig supernode account funding uses liquid prepare-funder" {
  run env \
    MONIKER=supernova_validator_2 \
    SUPERNODE_SETUP_LIB_ONLY=1 \
    SUPERNODE_SHARED_DIR="$SHARED_DIR" \
    bash -c 'source "$1"; supernode_funding_key_name; supernode_funding_address' bash "$SCRIPT"

  [ "$status" -eq 0 ]
  [[ "$output" == *"prepare-funder-supernova_validator_2"* ]]
  [[ "$output" == *"lumera1liquid"* ]]
  [[ "$output" != *"lumera1locked"* ]]
}

@test "multisig bank send is signed by prepare-funder key" {
  run env \
    MONIKER=supernova_validator_2 \
    SUPERNODE_SETUP_LIB_ONLY=1 \
    SUPERNODE_SHARED_DIR="$SHARED_DIR" \
    bash -c '
      source "$1"
      run_capture() { printf "%s\n" "$*"; }
      bank_send_from_validator lumera1dest 100ulume
    ' bash "$SCRIPT"

  [ "$status" -eq 0 ]
  [[ "$output" == *"tx bank send prepare-funder-supernova_validator_2 lumera1dest 100ulume"* ]]
  [[ "$output" != *"tx bank send lumera1locked"* ]]
}

@test "evm gas price detection keeps cosmos tx fee denom" {
  run env \
    MONIKER=supernova_validator_2 \
    SUPERNODE_SETUP_LIB_ONLY=1 \
    SUPERNODE_SHARED_DIR="$SHARED_DIR" \
    bash -c '
      source "$1"
      DAEMON=lumerad
      DENOM=ulume
      TX_GAS_PRICES=0.03ulume
      lumerad() {
        case "$*" in
          "q feemarket params --output json") printf "%s\n" "{\"params\":{\"base_fee\":\"0.002500000000000000\",\"min_gas_price\":\"0.000500000000000000\"}}" ;;
          "q evm config --output json") printf "%s\n" "{\"config\":{\"denom\":\"alume\"}}" ;;
        esac
      }
      update_gas_prices_for_evm >/dev/null
      printf "%s\n" "$TX_GAS_PRICES"
    ' bash "$SCRIPT"

  [ "$status" -eq 0 ]
  [[ "$output" == "0.005ulume" ]]
}

@test "multisig registration feegrant is signed by prepare-funder key" {
  CMD_LOG="$SHARED_DIR/commands.log"

  run env \
    MONIKER=supernova_validator_2 \
    SUPERNODE_SETUP_LIB_ONLY=1 \
    SUPERNODE_SHARED_DIR="$SHARED_DIR" \
    CMD_LOG="$CMD_LOG" \
    bash -c '
      source "$1"
      DAEMON=lumerad
      CHAIN_ID=lumera-devnet-1
      KEYRING_BACKEND=test
      TX_GAS_PRICES=0.03ulume
      VAL_ADDR=lumera1locked
      run_capture() {
        printf "%s\n" "$*" >>"$CMD_LOG"
        case "$*" in
          *"q feegrant grant"*) return 1 ;;
          *"tx feegrant grant"*) printf "%s\n" "{\"txhash\":\"abc\"}" ;;
          *) printf "%s\n" "$*" ;;
        esac
      }
      wait_for_tx() { :; }
      ensure_multisig_registration_feegrant
    ' bash "$SCRIPT"

  [ "$status" -eq 0 ]
  [[ "$(cat "$CMD_LOG")" == *"tx feegrant grant prepare-funder-supernova_validator_2 lumera1locked"* ]]
  [[ "$(cat "$CMD_LOG")" == *"--gas 120000"* ]]
}

@test "multisig registration tx uses prepare-funder feegranter" {
  CMD_LOG="$SHARED_DIR/commands.log"

  run env \
    MONIKER=supernova_validator_2 \
    SUPERNODE_SETUP_LIB_ONLY=1 \
    SUPERNODE_SHARED_DIR="$SHARED_DIR" \
    CMD_LOG="$CMD_LOG" \
    bash -c '
      source "$1"
      DAEMON=lumerad
      CHAIN_ID=lumera-devnet-1
      KEYRING_BACKEND=test
      TX_GAS_PRICES=0.03ulume
      KEY_NAME=supernova_validator_2_key
      VAL_ADDR=lumera1locked
      VALOPER_ADDR=lumeravaloper1locked
      SN_ENDPOINT=172.28.0.12:4444
      SN_ADDR=lumera1supernode
      run_capture() {
        printf "%s\n" "$*" >>"$CMD_LOG"
        case "$*" in
          *"q auth account"*) printf "%s\n" "{\"account\":{\"account_number\":\"6\",\"sequence\":\"1\"}}" ;;
          *"q feegrant grant"*) return 1 ;;
          *"tx feegrant grant"*) printf "%s\n" "{\"txhash\":\"grant\"}" ;;
          *"tx broadcast"*) printf "%s\n" "{\"txhash\":\"abc\"}" ;;
          *) printf "%s\n" "$*" ;;
        esac
      }
      wait_for_tx() { :; }
      multisig_sign_unsigned() { printf "%s\n" signed; }
      register_supernode_multisig
    ' bash "$SCRIPT"

  [ "$status" -eq 0 ]
  [[ "$(cat "$CMD_LOG")" == *"tx feegrant grant prepare-funder-supernova_validator_2 lumera1locked"* ]]
  [[ "$(cat "$CMD_LOG")" == *"--fee-granter lumera1liquid"* ]]
}
