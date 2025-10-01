#!/bin/bash
# /root/scripts/validator-setup.sh
set -euo pipefail

# Require MONIKER env (compose already sets it)
: "${MONIKER:?MONIKER environment variable must be set}"
echo "[SETUP] Setting up validator $MONIKER"

DEFAULT_P2P_PORT=26656
SHARED_DIR="/shared"
CFG_DIR="${SHARED_DIR}/config"
CFG_CHAIN="${CFG_DIR}/config.json"
CFG_VALS="${CFG_DIR}/validators.json"
CLAIMS_SHARED="${CFG_DIR}/claims.csv"
GENESIS_SHARED="${CFG_DIR}/genesis.json"
FINAL_GENESIS_SHARED="${CFG_DIR}/final_genesis.json"
EXTERNAL_GENESIS="${CFG_DIR}/external_genesis.json"
PEERS_SHARED="${CFG_DIR}/persistent_peers.txt"
GENTX_DIR="${CFG_DIR}/gentx"
ADDR_DIR="${SHARED_DIR}/addresses"
STATUS_DIR="${SHARED_DIR}/status"
RELEASE_DIR="${SHARED_DIR}/release"
GENESIS_READY_FLAG="${STATUS_DIR}/genesis_accounts_ready"
SETUP_COMPLETE_FLAG="${STATUS_DIR}/setup_complete"
# node specific vars
NODE_STATUS_DIR="${STATUS_DIR}/${MONIKER}"
NODE_SETUP_COMPLETE_FLAG="${NODE_STATUS_DIR}/setup_complete"

# ----- read config from config.json -----
if [ ! command -v jq >/dev/null 2>&1 ]; then
    echo "[CONFIGURE] jq is missing"
fi
if [ ! -f "${CFG_CHAIN}" ]; then
  echo "[SETUP] Missing ${CFG_CHAIN}"; exit 1
fi
if [ ! -f "${CFG_VALS}" ]; then
  echo "[SETUP] Missing ${CFG_VALS}"; exit 1
fi

CHAIN_ID="$(jq -r '.chain.id' "${CFG_CHAIN}")"
DENOM="$(jq -r '.chain.denom.bond' "${CFG_CHAIN}")"
KEYRING_BACKEND="$(jq -r '.daemon.keyring_backend' "${CFG_CHAIN}")"
DAEMON="$(jq -r '.daemon.binary' "${CFG_CHAIN}")"
DAEMON_HOME_BASE="$(jq -r '.paths.base.container' "${CFG_CHAIN}")"
DAEMON_DIR="$(jq -r '.paths.directories.daemon' "${CFG_CHAIN}")"

if [ -z "${CHAIN_ID}" ] || [ -z "${DENOM}" ] || [ -z "${KEYRING_BACKEND}" ] || \
   [ -z "${DAEMON}" ] || [ -z "${DAEMON_HOME_BASE}" ] || [ -z "${DAEMON_DIR}" ]; then
  echo "[SETUP] Invalid config.json (missing required fields)"; exit 1
fi

DAEMON_HOME="${DAEMON_HOME_BASE}/${DAEMON_DIR}"
echo "[SETUP] DAEMON_HOME is $DAEMON_HOME"

CONFIG_TOML="${DAEMON_HOME}/config/config.toml"
GENESIS_LOCAL="${DAEMON_HOME}/config/genesis.json"
CLAIMS_LOCAL="${DAEMON_HOME}/config/claims.csv"
GENTX_LOCAL_DIR="${DAEMON_HOME}/config/gentx"

mkdir -p "${NODE_STATUS_DIR}" "${STATUS_DIR}"

# ----- load this validator record -----
VAL_REC_JSON="$(jq -c --arg m "$MONIKER" '[.[] | select(.moniker==$m)][0]' "${CFG_VALS}")"
if [ -z "${VAL_REC_JSON}" ] || [ "${VAL_REC_JSON}" = "null" ]; then
  echo "[SETUP] Validator with moniker=${MONIKER} not found in validators.json"; exit 1
fi

KEY_NAME="$(echo "${VAL_REC_JSON}" | jq -r '.key_name')"
STAKE_AMOUNT="$(echo "${VAL_REC_JSON}" | jq -r '.initial_distribution.validator_stake')"
ACCOUNT_BAL="$(echo "${VAL_REC_JSON}" | jq -r '.initial_distribution.account_balance')"
P2P_HOST_PORT="$(echo "${VAL_REC_JSON}" | jq --arg port "${DEFAULT_P2P_PORT}" -r '.port // $port')"

# Determine primary (prefer .primary==true, else first element)
PRIMARY_NAME="$(jq -r '
  (map(select(.primary==true)) | if length>0 then .[0].moniker else empty end)
  // (.[0].moniker)
' "${CFG_VALS}")"
IS_PRIMARY="0"; [ "${MONIKER}" = "${PRIMARY_NAME}" ] && IS_PRIMARY="1"

echo "[SETUP] MONIKER=${MONIKER} KEY_NAME=${KEY_NAME} PRIMARY=${IS_PRIMARY} CHAIN_ID=${CHAIN_ID}"
mkdir -p "${DAEMON_HOME}/config"

# ----- helpers -----
run() {
  echo "+ $*"
  "$@"
}

run_capture() {
  echo "+ $*" >&2   # goes to stderr, not captured
  "$@"
}

write_node_markers() {
  local nodeid
  # write fixed container P2P port
  echo "${DEFAULT_P2P_PORT}" > "${NODE_STATUS_DIR}/port"

  if [ -f "${CONFIG_TOML}" ]; then
    nodeid="$(${DAEMON} tendermint show-node-id || true)"
    [ -n "${nodeid}" ] && echo "${nodeid}" > "${NODE_STATUS_DIR}/nodeid"
  fi

  echo "[SETUP] status files in ${NODE_STATUS_DIR}:"
  ls -l "${NODE_STATUS_DIR}" || true
}

build_persistent_peers() {
  : > "${PEERS_SHARED}"
  while IFS= read -r other; do
    [ -z "${other}" ] && continue
    [ "${other}" = "${MONIKER}" ] && continue
    local od="${STATUS_DIR}/${other}"
    # Use service DNS name (compose service == moniker) to avoid IP churn.
    if [ -s "${od}/nodeid" ] && [ -s "${od}/port" ]; then
      echo "$(cat "${od}/nodeid")@${other}:$(cat "${od}/port")" >> "${PEERS_SHARED}"
    fi
  done < <(jq -r '.[].moniker' "${CFG_VALS}")
  echo "[SETUP] persistent_peers:"
  cat "${PEERS_SHARED}" || true
}

apply_persistent_peers() {
  if [ -f "${PEERS_SHARED}" ] && [ -f "${CONFIG_TOML}" ]; then
    local peers; peers="$(paste -sd, "${PEERS_SHARED}" || true)"
    if [ -n "${peers}" ]; then
      sed -i -E "s|^persistent_peers *=.*$|persistent_peers = \"${peers}\"|g" "${CONFIG_TOML}"
      echo "[SETUP] Applied persistent_peers to ${CONFIG_TOML}"
    fi
  fi
}

wait_for_file() {
    while [ ! -s "$1" ];
        do sleep 1;
    done;
}

init_if_needed() {
  if [ -f "${GENESIS_LOCAL}" ]; then
    echo "[SETUP] ${MONIKER} already initialized."
    return 0
  fi
  echo "[SETUP] Initializing ${MONIKER}..."
  ${DAEMON} init "${MONIKER}" --chain-id "${CHAIN_ID}" --overwrite

  # key (idempotent)
  if ! ${DAEMON} keys show "${KEY_NAME}" --keyring-backend "${KEYRING_BACKEND}" >/dev/null 2>&1; then
    ${DAEMON} keys add "${KEY_NAME}" --keyring-backend "${KEYRING_BACKEND}" >/dev/null
  fi
}

# ----- primary validator -----
primary_validator_setup() {
  init_if_needed

  # must have external genesis + claims ready
  if [ ! -f "${EXTERNAL_GENESIS}" ]; then
    echo "ERROR: ${EXTERNAL_GENESIS} not found. Provide existing genesis."; exit 1
  fi
  cp "${EXTERNAL_GENESIS}" "${GENESIS_LOCAL}"
  [ -f "${CLAIMS_SHARED}" ] && cp "${CLAIMS_SHARED}" "${CLAIMS_LOCAL}"

  # unify denoms (bond/mint/crisis/gov)
  tmp="${DAEMON_HOME}/config/tmp_genesis.json"
  cat "${GENESIS_LOCAL}" | jq \
    --arg denom "${DENOM}" '
      .app_state.staking.params.bond_denom = $denom
      | .app_state.mint.params.mint_denom = $denom
      | .app_state.crisis.constant_fee.denom = $denom
      | .app_state.gov.params.min_deposit[0].denom = $denom
      | .app_state.gov.params.expedited_min_deposit[0].denom = $denom
    ' > "${tmp}"
  mv "${tmp}" "${GENESIS_LOCAL}"

  # primary’s own account
  echo "[SETUP] Creating key/account for ${KEY_NAME}..."
  ADDR="$(run_capture ${DAEMON} keys show "${KEY_NAME}" -a --keyring-backend "${KEYRING_BACKEND}")"
  run ${DAEMON} genesis add-genesis-account "${ADDR}" "${ACCOUNT_BAL}"
  echo "${ADDR}" > "${NODE_STATUS_DIR}/genesis-address"

  # governance account
  if ! run ${DAEMON} keys show governance_key --keyring-backend "${KEYRING_BACKEND}" >/dev/null 2>&1; then
    run ${DAEMON} keys add governance_key --keyring-backend "${KEYRING_BACKEND}" >/dev/null
  fi
  GOV_ADDR="$(run_capture ${DAEMON} keys show governance_key -a --keyring-backend "${KEYRING_BACKEND}")"
  echo "${GOV_ADDR}" > ${SHARED_DIR}/governance_address
  run ${DAEMON} genesis add-genesis-account "${GOV_ADDR}" "1000000000000${DENOM}"

  # share initial genesis to secondaries & flag
  cp "${GENESIS_LOCAL}" "${GENESIS_SHARED}"
  mkdir -p "${GENTX_DIR}" "${ADDR_DIR}"
  echo "true" > "${GENESIS_READY_FLAG}"

  # write own markers before waiting for peers
  write_node_markers

  # wait for all other nodes to publish nodeid/ip
  total="$(jq -r 'length' "${CFG_VALS}")"
  echo "[SETUP] Waiting for other node IDs/IPs..."
  while true; do
    found=0
    while IFS= read -r other; do
      [ "${other}" = "${MONIKER}" ] && continue
      od="${STATUS_DIR}/${other}"
      [[ -s "${od}/nodeid" ]] && found=$((found+1))
    done < <(jq -r '.[].moniker' "${CFG_VALS}")
    [ "${found}" -ge $((total-1)) ] && break
    sleep 1
  done

  # collect gentx/addresses from secondaries
  echo "[SETUP] Collecting addresses & gentx from secondaries..."
  if compgen -G "${ADDR_DIR}/*" > /dev/null; then
    while IFS= read -r file; do
      [ -f "$file" ] || continue
      bal="$(cat "$file")"; addr="$(basename "$file")"
      run ${DAEMON} genesis add-genesis-account "${addr}" "${bal}"
    done < <(find ${ADDR_DIR} -type f)
  fi

  # primary gentx
  run ${DAEMON} genesis gentx "${KEY_NAME}" "${STAKE_AMOUNT}" \
    --chain-id "${CHAIN_ID}" \
    --keyring-backend "${KEYRING_BACKEND}"

  # collect others' gentx
  mkdir -p "${GENTX_LOCAL_DIR}"
  if compgen -G "${GENTX_DIR}/*.json" > /dev/null; then
    cp "${GENTX_DIR}"/*.json "${GENTX_LOCAL_DIR}/" || true
  fi
  run ${DAEMON} genesis collect-gentxs

  # publish final genesis
  cp "${GENESIS_LOCAL}" "${FINAL_GENESIS_SHARED}"
  echo "[SETUP] Final genesis published to ${FINAL_GENESIS_SHARED}"

  # build & apply persistent peers
  build_persistent_peers
  apply_persistent_peers

  echo "true" > "${SETUP_COMPLETE_FLAG}"
  echo "true" > "${NODE_SETUP_COMPLETE_FLAG}"
  echo "[SETUP] Primary setup complete."
}

# ----- secondary validator -----
secondary_validator_setup() {
  # wait for primary to publish accounts genesis
  echo "[SETUP] Waiting for primary genesis_accounts_ready..."
  wait_for_file "${GENESIS_READY_FLAG}"
  wait_for_file "${GENESIS_SHARED}"
  
  init_if_needed

  # copy initial genesis/claims
  cp "${GENESIS_SHARED}" "${GENESIS_LOCAL}"
  [ -f "${CLAIMS_SHARED}" ] && cp "${CLAIMS_SHARED}" "${CLAIMS_LOCAL}"

  # create key, add account, create gentx
  if ! run ${DAEMON} keys show "${KEY_NAME}" --keyring-backend "${KEYRING_BACKEND}" >/dev/null 2>&1; then
    run ${DAEMON} keys add "${KEY_NAME}" --keyring-backend "${KEYRING_BACKEND}" >/dev/null
  fi
  ADDR="$(run_capture ${DAEMON} keys show "${KEY_NAME}" -a --keyring-backend "${KEYRING_BACKEND}")"
  run ${DAEMON} genesis add-genesis-account "${ADDR}" "${ACCOUNT_BAL}"

  mkdir -p "${GENTX_LOCAL_DIR}" "${GENTX_DIR}" "${ADDR_DIR}"

  if compgen -G "${GENTX_LOCAL_DIR}/gentx-*.json" > /dev/null; then
    echo "[SETUP] gentx already exists in ${GENTX_LOCAL_DIR}, skipping generation"
  else
    run ${DAEMON} genesis gentx "${KEY_NAME}" "${STAKE_AMOUNT}" \
      --chain-id "${CHAIN_ID}" --keyring-backend "${KEYRING_BACKEND}"
  fi

  # share gentx & address
  cp "${GENTX_LOCAL_DIR}"/gentx-*.json "${GENTX_DIR}"/${MONIKER}_gentx.json
  echo "${ACCOUNT_BAL}" > "${ADDR_DIR}/${ADDR}"
  echo "${ADDR}" > "${NODE_STATUS_DIR}/genesis-address"

  # write own markers for peer discovery
  write_node_markers

  # wait for persistent_peers and apply
  wait_for_file "${PEERS_SHARED}"
  apply_persistent_peers

  # wait for final genesis
  echo "[SETUP] Waiting for final genesis from primary..."
  wait_for_file "${FINAL_GENESIS_SHARED}"
  cp "${FINAL_GENESIS_SHARED}" "${GENESIS_LOCAL}"
  wait_for_file "${SETUP_COMPLETE_FLAG}"

  echo "[SETUP] Secondary setup complete."
  echo "true" > "${NODE_SETUP_COMPLETE_FLAG}"
}

# ----- main -----
if [ "${IS_PRIMARY}" = "1" ]; then
  primary_validator_setup
else
  secondary_validator_setup
fi