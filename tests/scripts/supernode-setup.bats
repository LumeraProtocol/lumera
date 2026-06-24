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

@test "validator setup selects migrated evm validator key when it is active on chain" {
  run env \
    MONIKER=supernova_validator_2 \
    SUPERNODE_SETUP_LIB_ONLY=1 \
    SUPERNODE_SHARED_DIR="$SHARED_DIR" \
    bash -c '
      source "$1"
      DAEMON=lumerad
      KEY_NAME=supernova_validator_2_key
      VALIDATOR_BASE_KEY_NAME=supernova_validator_2_key
      lumerad() {
        case "$*" in
          "keys show supernova_validator_2_key -a --keyring-backend test") printf "%s\n" "lumera1legacy" ;;
          "keys show supernova_validator_2_key --bech val -a --keyring-backend test") printf "%s\n" "lumeravaloper1legacy" ;;
          "q staking validator lumeravaloper1legacy --output json") return 1 ;;
          "keys show supernova_validator_2_key_evm -a --keyring-backend test") printf "%s\n" "lumera1migrated" ;;
          "keys show supernova_validator_2_key_evm --bech val -a --keyring-backend test") printf "%s\n" "lumeravaloper1migrated" ;;
          "q staking validator lumeravaloper1migrated --output json") printf "%s\n" "{\"validator\":{\"operator_address\":\"lumeravaloper1migrated\"}}" ;;
          *) return 1 ;;
        esac
      }
      select_active_validator_key
      printf "%s\n%s\n%s\n" "$KEY_NAME" "$VAL_ADDR" "$VALOPER_ADDR"
    ' bash "$SCRIPT"

  [ "$status" -eq 0 ]
  [[ "$output" == $'supernova_validator_2_key_evm\nlumera1migrated\nlumeravaloper1migrated' ]]
}

@test "migrated validator multisig signing uses discovered evm signer keys" {
  run env \
    MONIKER=supernova_validator_2 \
    SUPERNODE_SETUP_LIB_ONLY=1 \
    SUPERNODE_SHARED_DIR="$SHARED_DIR" \
    bash -c '
      source "$1"
      DAEMON=lumerad
      KEY_NAME=supernova_validator_2_key_evm
      VALIDATOR_BASE_KEY_NAME=supernova_validator_2_key
      lumerad() {
        case "$*" in
          "keys show val2-evm-signer-1 -a --keyring-backend test") printf "%s\n" "lumera1evmsigner1" ;;
          "keys show val2-evm-signer-2 -a --keyring-backend test") printf "%s\n" "lumera1evmsigner2" ;;
          *) return 1 ;;
        esac
      }
      validator_multisig_signer_key 1
      validator_multisig_signer_key 2
    ' bash "$SCRIPT"

  [ "$status" -eq 0 ]
  [[ "$output" == $'val2-evm-signer-1\nval2-evm-signer-2' ]]
}

@test "supernode evm key recovery uses supernode keyring writer" {
  CMD_LOG="$SHARED_DIR/commands.log"
  mkdir -p "$SHARED_DIR/supernode"
  cat >"$SHARED_DIR/supernode/config.yml" <<'YAML'
supernode:
    key_name: "supernova_supernode_2_key_evm"
keyring:
    backend: "test"
    dir: "keys"
YAML

  run env \
    MONIKER=supernova_validator_2 \
    SUPERNODE_SETUP_LIB_ONLY=1 \
    SUPERNODE_SHARED_DIR="$SHARED_DIR" \
    CMD_LOG="$CMD_LOG" \
    bash -c '
      source "$1"
      SN_BASEDIR="$SHARED_DIR/supernode"
      SN_CONFIG="$SN_BASEDIR/config.yml"
      SN_KEYRING_HOME="$SN_BASEDIR/keys"
      SN=supernode-linux-amd64
      mkdir -p "$SN_KEYRING_HOME/keyring-test"
      touch "$SN_KEYRING_HOME/keyring-test/supernova_supernode_2_key_evm.info"
      supernode-linux-amd64() {
        printf "%s\n" "$*" >>"$CMD_LOG"
        case "$*" in
          "keys list -d "*)
            return 1
            ;;
          "keys recover supernova_supernode_2_key_evm -d "*)
            return 0
            ;;
        esac
      }
      ensure_supernode_evm_key_from_mnemonic supernova_supernode_2_key_evm "test test test test test test test test test test test junk"
    ' bash "$SCRIPT"

  [ "$status" -eq 0 ]
  [[ "$(cat "$CMD_LOG")" == *"keys list -d $SHARED_DIR/supernode"* ]]
  [[ "$(cat "$CMD_LOG")" == *"keys recover supernova_supernode_2_key_evm -d $SHARED_DIR/supernode"* ]]
  compgen -G "$SHARED_DIR/supernode/keys/keyring-test/supernova_supernode_2_key_evm.info.bak-"* >/dev/null
}

@test "supernode binary install does not downgrade newer installed version" {
  RELEASE_DIR="$SHARED_DIR/release"
  mkdir -p "$RELEASE_DIR" "$SHARED_DIR/bin"
  cat >"$RELEASE_DIR/supernode-linux-amd64" <<'SH'
#!/bin/sh
case "$1" in
  version) printf '%s\n' 'Version: v2.5.2' ;;
esac
SH
  cat >"$SHARED_DIR/bin/supernode-linux-amd64" <<'SH'
#!/bin/sh
case "$1" in
  version) printf '%s\n' 'Version: v2.6.0-rc1' ;;
esac
SH
  chmod +x "$RELEASE_DIR/supernode-linux-amd64" "$SHARED_DIR/bin/supernode-linux-amd64"

  run env \
    MONIKER=supernova_validator_2 \
    SUPERNODE_SETUP_LIB_ONLY=1 \
    SUPERNODE_SHARED_DIR="$SHARED_DIR" \
    bash -c '
      source "$1"
      SN_BIN_DST="$SHARED_DIR/bin/supernode-linux-amd64"
      install_supernode_binary
      "$SN_BIN_DST" version
    ' bash "$SCRIPT"

  [ "$status" -eq 0 ]
  [[ "$output" == *"newer than shared release"* ]]
  [[ "$output" == *"Version: v2.6.0-rc1"* ]]
}
