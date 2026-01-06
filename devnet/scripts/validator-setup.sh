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
LOCKS_DIR="${STATUS_DIR}/locks"

HERMES_SHARED_DIR="${SHARED_DIR}/hermes"
HERMES_STATUS_DIR="${STATUS_DIR}/hermes"
HERMES_RELAYER_KEY="${HERMES_RELAYER_KEY:-hermes-relayer}"
HERMES_RELAYER_FILE_NAME="${HERMES_RELAYER_KEY}"
if [[ "${HERMES_RELAYER_FILE_NAME}" != lumera-* ]]; then
  HERMES_RELAYER_FILE_NAME="lumera-${HERMES_RELAYER_FILE_NAME}"
fi
HERMES_RELAYER_MNEMONIC_FILE="${HERMES_SHARED_DIR}/${HERMES_RELAYER_FILE_NAME}.mnemonic"
HERMES_RELAYER_ADDR_FILE="${HERMES_SHARED_DIR}/${HERMES_RELAYER_FILE_NAME}.address"
HERMES_RELAYER_GENESIS_AMOUNT="${HERMES_RELAYER_GENESIS_AMOUNT:-10000000}" # in bond denom units

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
APP_TOML="${DAEMON_HOME}/config/app.toml"
GENESIS_LOCAL="${DAEMON_HOME}/config/genesis.json"
CLAIMS_LOCAL="${DAEMON_HOME}/config/claims.csv"
GENTX_LOCAL_DIR="${DAEMON_HOME}/config/gentx"

mkdir -p "${NODE_STATUS_DIR}" "${STATUS_DIR}"
mkdir -p "${LOCKS_DIR}"

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

with_lock() {
  local name="$1"
  shift
  local lock_file="${LOCKS_DIR}/${name}.lock"
  mkdir -p "${LOCKS_DIR}"
  if ! command -v flock >/dev/null 2>&1; then
    "$@"
    return
  fi
  {
    flock -x 200
    "$@"
  } 200>"${lock_file}"
}

write_with_lock() {
  local lock_name="$1"
  local dest="$2"
  local value="$3"
  with_lock "${lock_name}" bash -c 'printf "%s\n" "$1" > "$2"' _ "${value}" "${dest}"
}

copy_with_lock() {
  local lock_name="$1"
  shift
  with_lock "${lock_name}" "$@"
}

verify_gentx_file() {
  local file="$1"
  if [ ! -f "${file}" ]; then
    echo "[SETUP] ERROR: gentx file ${file} not found"
    return 1
  fi
  return 0
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

    # Treat all validators as private peers so CometBFT accepts their non-routable addresses.
    local peer_ids
    peer_ids="$(cut -d@ -f1 "${PEERS_SHARED}" | paste -sd, || true)"
    if [ -n "${peer_ids}" ]; then
      sed -i -E "s|^private_peer_ids *=.*$|private_peer_ids = \"${peer_ids}\"|g" "${CONFIG_TOML}"
      echo "[SETUP] Applied private_peer_ids to ${CONFIG_TOML}"
    fi
  fi
}

configure_node_config() {
  local api_port="${LUMERA_API_PORT:-1317}"
  local grpc_port="${LUMERA_GRPC_PORT:-9090}"
  local rpc_port="${LUMERA_RPC_PORT:-26657}"

  if ! command -v crudini >/dev/null 2>&1; then
    echo "[SETUP] ERROR: crudini not found; cannot update configs"
    exit 1
  fi

  if [ -f "${APP_TOML}" ]; then
    run crudini --set "${APP_TOML}" '' minimum-gas-prices "\"0.0025ulume\""
    run crudini --set "${APP_TOML}" api enable "true"
    run crudini --set "${APP_TOML}" api swagger "true"
    run crudini --set "${APP_TOML}" api address "\"tcp://0.0.0.0:${api_port}\""
    run crudini --set "${APP_TOML}" grpc enable "true"
    run crudini --set "${APP_TOML}" grpc address "\"0.0.0.0:${grpc_port}\""
    run crudini --set "${APP_TOML}" grpc-web enable "true"
    echo "[SETUP] Updated ${APP_TOML} with API/GRPC configuration."
  else
    echo "[SETUP] WARNING: ${APP_TOML} not found; skipping app.toml update"
  fi

  if [ -f "${CONFIG_TOML}" ]; then
    run crudini --set "${CONFIG_TOML}" rpc laddr "\"tcp://0.0.0.0:${rpc_port}\""
    echo "[SETUP] Updated ${CONFIG_TOML} RPC configuration."
  else
    echo "[SETUP] WARNING: ${CONFIG_TOML} not found; skipping config.toml update"
  fi
}

ensure_hermes_relayer_account() {
  echo "[SETUP] Ensuring Hermes relayer account..."
  mkdir -p "${HERMES_SHARED_DIR}" "${HERMES_STATUS_DIR}"

  local mnemonic=""
  if [ -s "${HERMES_RELAYER_MNEMONIC_FILE}" ]; then
    mnemonic="$(cat "${HERMES_RELAYER_MNEMONIC_FILE}")"
  fi

  local relayer_addr
  relayer_addr="$(run_capture ${DAEMON} keys show "${HERMES_RELAYER_KEY}" -a --keyring-backend "${KEYRING_BACKEND}" 2>/dev/null || true)"
  relayer_addr="$(printf '%s' "${relayer_addr}" | tr -d '\r\n')"
  if [ -z "${relayer_addr}" ]; then
    if [ -n "${mnemonic}" ]; then
      printf '%s\n' "${mnemonic}" | run ${DAEMON} keys add "${HERMES_RELAYER_KEY}" --recover --keyring-backend "${KEYRING_BACKEND}" >/dev/null
    else
      local key_json
      key_json="$(run_capture ${DAEMON} keys add "${HERMES_RELAYER_KEY}" --keyring-backend "${KEYRING_BACKEND}" --output json)"
      mnemonic="$(printf '%s' "${key_json}" | jq -r '.mnemonic // empty' 2>/dev/null || true)"
    fi
  fi

  if [ -n "${mnemonic}" ]; then
    write_with_lock "hermes-mnemonic" "${HERMES_RELAYER_MNEMONIC_FILE}" "${mnemonic}"
  fi

  relayer_addr="$(run_capture ${DAEMON} keys show "${HERMES_RELAYER_KEY}" -a --keyring-backend "${KEYRING_BACKEND}" 2>/dev/null || true)"
  relayer_addr="$(printf '%s' "${relayer_addr}" | tr -d '\r\n')"
  if [ -z "${relayer_addr}" ]; then
    echo "[SETUP] ERROR: Unable to obtain Hermes relayer address"
    exit 1
  fi
  write_with_lock "hermes-addr" "${HERMES_RELAYER_ADDR_FILE}" "${relayer_addr}"

  local need_add=1
  if [ -f "${GENESIS_LOCAL}" ]; then
    if jq -e --arg addr "${relayer_addr}" '.app_state.bank.balances[]? | select(.address==$addr)' "${GENESIS_LOCAL}" >/dev/null 2>&1; then
      need_add=0
    fi
  fi

  if [ "${need_add}" -eq 1 ]; then
    echo "[SETUP] Adding Hermes relayer genesis balance: ${HERMES_RELAYER_GENESIS_AMOUNT}${DENOM}"
    set +e
    run ${DAEMON} genesis add-genesis-account "${relayer_addr}" "${HERMES_RELAYER_GENESIS_AMOUNT}${DENOM}"
    local status=$?
    set -e
    if [ ${status} -ne 0 ]; then
      echo "[SETUP] Failed to add Hermes relayer genesis account."
      exit ${status}
    fi
  else
    echo "[SETUP] Hermes relayer genesis account already present."
  fi
}

wait_for_file() {
    while [ ! -s "$1" ];
        do sleep 1;
    done;
}

init_if_needed() {
  if [ -f "${GENESIS_LOCAL}" ]; then
    echo "[SETUP] ${MONIKER} already initialized (genesis exists)."
  else
    echo "[SETUP] Initializing ${MONIKER}..."
    run ${DAEMON} init "${MONIKER}" --chain-id "${CHAIN_ID}" --overwrite
  fi

  # ensure validator key exists
  local addr
  addr="$(run_capture ${DAEMON} keys show "${KEY_NAME}" -a --keyring-backend "${KEYRING_BACKEND}" 2>/dev/null || true)"
  addr="$(printf '%s' "${addr}" | tr -d '\r\n')"
  if [ -z "${addr}" ]; then
    run ${DAEMON} keys add "${KEY_NAME}" --keyring-backend "${KEYRING_BACKEND}"
  else
    echo "[SETUP] Key ${KEY_NAME} already exists with address ${addr}"
  fi
}

# ----- primary validator -----
primary_validator_setup() {
  init_if_needed
  configure_node_config

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
  addr="$(run_capture ${DAEMON} keys show "${KEY_NAME}" -a --keyring-backend "${KEYRING_BACKEND}")"
  addr="$(printf '%s' "${addr}" | tr -d '\r\n')"
  if [ -z "${addr}" ]; then
    echo "[SETUP] ERROR: Unable to obtain address for ${KEY_NAME}"
    exit 1
  fi
  run ${DAEMON} genesis add-genesis-account "${addr}" "${ACCOUNT_BAL}"
  printf '%s\n' "${addr}" > "${NODE_STATUS_DIR}/genesis-address"

  # governance account
  local gov_addr
  gov_addr="$(run_capture ${DAEMON} keys show governance_key -a --keyring-backend "${KEYRING_BACKEND}" 2>/dev/null || true)"
  gov_addr="$(printf '%s' "${gov_addr}" | tr -d '\r\n')"
  if [ -z "${gov_addr}" ]; then
    run ${DAEMON} keys add governance_key --keyring-backend "${KEYRING_BACKEND}" >/dev/null
    gov_addr="$(run_capture ${DAEMON} keys show governance_key -a --keyring-backend "${KEYRING_BACKEND}")"
    gov_addr="$(printf '%s' "${gov_addr}" | tr -d '\r\n')"
  fi
  if [ -z "${gov_addr}" ]; then
    echo "[SETUP] ERROR: Unable to obtain governance key address"
    exit 1
  fi
  printf '%s\n' "${gov_addr}" > ${SHARED_DIR}/governance_address
  run ${DAEMON} genesis add-genesis-account "${gov_addr}" "1000000000000${DENOM}"

  ensure_hermes_relayer_account

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

  for file in "${GENTX_LOCAL_DIR}"/gentx-*.json; do
    [ -f "${file}" ] || continue
    verify_gentx_file "${file}" || exit 1
  done

  # collect others' gentx
  mkdir -p "${GENTX_LOCAL_DIR}"
  if compgen -G "${GENTX_DIR}/*.json" > /dev/null; then
    copy_with_lock "gentx" bash -c 'cp "$1"/*.json "$2"/' _ "${GENTX_DIR}" "${GENTX_LOCAL_DIR}" || true
    for file in "${GENTX_LOCAL_DIR}"/gentx-*.json; do
      [ -f "${file}" ] || continue
      verify_gentx_file "${file}" || exit 1
    done
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
  configure_node_config

  # copy initial genesis/claims
  cp "${GENESIS_SHARED}" "${GENESIS_LOCAL}"
  [ -f "${CLAIMS_SHARED}" ] && cp "${CLAIMS_SHARED}" "${CLAIMS_LOCAL}"

  # create key, add account, create gentx
  addr="$(run_capture ${DAEMON} keys show "${KEY_NAME}" -a --keyring-backend "${KEYRING_BACKEND}")"
  addr="$(printf '%s' "${addr}" | tr -d '\r\n')"
  if [ -z "${addr}" ]; then
    run ${DAEMON} keys add "${KEY_NAME}" --keyring-backend "${KEYRING_BACKEND}" >/dev/null
  fi
  addr="$(run_capture ${DAEMON} keys show "${KEY_NAME}" -a --keyring-backend "${KEYRING_BACKEND}")"
  addr="$(printf '%s' "${addr}" | tr -d '\r\n')"
  if [ -z "${addr}" ]; then
    echo "[SETUP] ERROR: Unable to obtain address for ${KEY_NAME}"
    exit 1
  fi
  run ${DAEMON} genesis add-genesis-account "${addr}" "${ACCOUNT_BAL}"
  ensure_hermes_relayer_account

  mkdir -p "${GENTX_LOCAL_DIR}" "${GENTX_DIR}" "${ADDR_DIR}"

  if compgen -G "${GENTX_LOCAL_DIR}/gentx-*.json" > /dev/null; then
    echo "[SETUP] gentx already exists in ${GENTX_LOCAL_DIR}, skipping generation"
  else
    run ${DAEMON} genesis gentx "${KEY_NAME}" "${STAKE_AMOUNT}" \
      --chain-id "${CHAIN_ID}" --keyring-backend "${KEYRING_BACKEND}"
  fi

  local gentx_file
  gentx_file="$(find "${GENTX_LOCAL_DIR}" -maxdepth 1 -type f -name 'gentx-*.json' -print | head -n1)"
  if [ -z "${gentx_file}" ]; then
    echo "[SETUP] ERROR: gentx generation failed for ${KEY_NAME} (no file produced)"
    exit 1
  fi
  verify_gentx_file "${gentx_file}" || exit 1

  # share gentx & address
  copy_with_lock "gentx" cp "${gentx_file}" "${GENTX_DIR}/${MONIKER}_gentx.json"
  write_with_lock "addresses" "${ADDR_DIR}/${addr}" "${ACCOUNT_BAL}"
  printf '%s\n' "${addr}" > "${NODE_STATUS_DIR}/genesis-address"

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
