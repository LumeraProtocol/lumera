#!/usr/bin/env bash
# --------------------------------------------------------------------------------------------------
# start.sh — Entrypoint for Lumera devnet validators & supernode
#
# MODES (set via START_MODE environment variable):
#   auto  (default)  If setup_complete flag is missing, launches supernode-setup.sh & validator-setup.sh
#                    scripts in the background. Then waits for setup_complete, starts lumerad,
#                    and tails logs.
#
#   bootstrap        Runs setup scripts supernode-setup.sh & validator-setup.sh in the foreground.
#                    Exits when setup_complete is created. Does NOT start lumerad.
#
#   run              Waits for setup_complete, starts lumerad, and tails logs.
#
#   wait  (optional) Wait for setup_complete and exit. 
#
# DOCKER COMPOSE:
#   - Image ENTRYPOINT should be: ["/bin/bash", "/root/scripts/start.sh"] (as in Dockerfile).
#     # One-time network bootstrap (creates final genesis & setup_complete, exits)
#     START_MODE=bootstrap docker compose up --build --abort-on-container-exit
#
#     # Steady state: start validators using finalized genesis
#     START_MODE=run       docker compose up -d
# --------------------------------------------------------------------------------------------------
set -euo pipefail

START_MODE="${START_MODE:-auto}"

SHARED_DIR="/shared"
CFG_DIR="${SHARED_DIR}/config"
CFG_CHAIN="${CFG_DIR}/config.json"
RELEASE_DIR="${SHARED_DIR}/release"
STATUS_DIR="${SHARED_DIR}/status"
SETUP_COMPLETE="${STATUS_DIR}/setup_complete"
SN="supernode-linux-amd64"
NM="network-maker"
LUMERAD="lumerad"
LUMERA_SRC_BIN="${RELEASE_DIR}/${LUMERAD}"
LUMERA_DST_BIN="/usr/local/bin/${LUMERAD}"
WASMVM_SRC_LIB="${RELEASE_DIR}/libwasmvm.x86_64.so"
WASMVM_DST_LIB="/usr/lib/libwasmvm.x86_64.so"

DAEMON="${DAEMON:-lumerad}"
DAEMON_HOME="${DAEMON_HOME:-/root/.lumera}"

SCRIPTS_DIR="/root/scripts"
LOGS_DIR="/root/logs"
VALIDATOR_LOG="${LOGS_DIR}/validator.log"
SUPERNODE_LOG="${LOGS_DIR}/supernode.log"
VALIDATOR_SETUP_OUT="${LOGS_DIR}/validator-setup.out"
SUPERNODE_SETUP_OUT="${LOGS_DIR}/supernode-setup.out"
NETWORK_MAKER_SETUP_OUT="${LOGS_DIR}/network-maker-setup.out"

LUMERA_RPC_PORT="${LUMERA_RPC_PORT:-26657}"
LUMERA_GRPC_PORT="${LUMERA_GRPC_PORT:-9090}"
LUMERA_RPC_ADDR="http://localhost:${LUMERA_RPC_PORT}"

mkdir -p "${LOGS_DIR}" "${DAEMON_HOME}/config" "${STATUS_DIR}"

# Require MONIKER env (compose already sets it)
: "${MONIKER:?MONIKER environment variable must be set}"
echo "[BOOT] ${MONIKER}: start.sh (mode=${START_MODE})"

if [ ! command -v jq >/dev/null 2>&1 ]; then
    echo "[BOOT] jq is missing"
fi

if [ ! -f "${CFG_CHAIN}" ]; then
  echo "[BOOT] Missing ${CFG_CHAIN}"; exit 1
fi

MIN_GAS_PRICE="$(jq -r '.chain.denom.minimum_gas_price' "${CFG_CHAIN}")"

wait_for_flag() {
  local f="$1"
  until [ -s "${f}" ]; do sleep 1; done
}

run() {
  echo "+ $*"
  "$@"
}

# Get current block height (integer), 0 if unknown
current_height() {
  curl -sf "${LUMERA_RPC_ADDR}/status" \
    | jq -r '.result.sync_info.latest_block_height // "0"' 2>/dev/null \
    | awk '{print ($1 ~ /^[0-9]+$/) ? $1 : 0}'
}

# Wait until height >= target (with timeout)
wait_for_height_at_least() {
  local target="$1"
  local retries="${2:-180}"  # ~180s
  local delay="${3:-1}"

  echo "[BOOT] Waiting for block height >= ${target} ..."
  for ((i=0; i<retries; i++)); do
    local h
    h="$(current_height)"
    if (( h >= target )); then
      echo "[BOOT] Height is ${h} (>= ${target}) — OK."
      return 0
    fi
    sleep "$delay"
  done
  echo "[BOOT] Timeout waiting for height >= ${target}."
  return 1
}

# Wait for N new blocks from the current height (default 5)
wait_for_n_blocks() {
  local n="${1:-5}"
  local start
  start="$(current_height)"
  local target=$(( start + n ))
  # If the chain hasn't started yet (start==0), still use +n (so target=n)
  (( target < n )) && target="$n"
  wait_for_height_at_least "$target"
}

launch_supernode_setup() {
  # Start optional supernode setup only in auto/run modes after init is done.
  if [ -x "${SCRIPTS_DIR}/supernode-setup.sh" ] && [ -f "${RELEASE_DIR}/${SN}" ]; then
    echo "[BOOT] ${MONIKER}: Launching Supernode setup in background..."
    export LUMERA_RPC_PORT="${LUMERA_RPC_PORT:-26657}"
    export LUMERA_GRPC_PORT="${LUMERA_GRPC_PORT:-9090}"
    nohup bash "${SCRIPTS_DIR}/supernode-setup.sh" >"${SUPERNODE_SETUP_OUT}" 2>&1 &
  fi
}

wait_for_validator_setup() {
  echo "[BOOT] ${MONIKER}: Waiting for validator setup to complete..."
  wait_for_flag "${SETUP_COMPLETE}"
  echo "[BOOT] ${MONIKER}: validator setup complete."
}

install_wasm_lib() {
  if [ -f "${WASMVM_SRC_LIB}" ]; then
    if [ -f "${WASMVM_DST_LIB}" ] && cmp -s "${WASMVM_SRC_LIB}" "${WASMVM_DST_LIB}"; then
      echo "[BOOT] libwasmvm.x86_64.so already up to date at ${WASMVM_DST_LIB}"
      return
    fi
    echo "[BOOT] Installing libwasmvm.x86_64.so to ${WASMVM_DST_LIB}"
    run cp -f "${WASMVM_SRC_LIB}" "${WASMVM_DST_LIB}"
    run chmod 755 "${WASMVM_DST_LIB}"
  else
    echo "[BOOT] ${WASMVM_SRC_LIB} not found, assuming libwasmvm.x86_64.so is already installed"
  fi
}

install_lumerad_binary() {
  run cp -f "${LUMERA_SRC_BIN}" "${LUMERA_DST_BIN}"
  run chmod +x "${LUMERA_DST_BIN}"
  install_wasm_lib
  run lumerad version || true
}

install_or_update_lumerad() {
  if [ ! -f "${LUMERA_DST_BIN}" ]; then
    if [ -f "${LUMERA_SRC_BIN}" ]; then
      echo "[BOOT] ${LUMERAD} binary not found at ${LUMERA_DST_BIN}, installing..."
      install_lumerad_binary
    else
      echo "[BOOT] ${LUMERA_SRC_BIN} not found, assuming ${LUMERAD} is already installed"
    fi
  else
    run lumerad version || true
    if [ -f "${LUMERA_SRC_BIN}" ]; then
      if cmp -s "${LUMERA_SRC_BIN}" "${LUMERA_DST_BIN}"; then
        echo "[BOOT] ${LUMERAD} binary already up to date at ${LUMERA_DST_BIN}"
      else
        echo "[BOOT] Updating ${LUMERAD} binary at ${LUMERA_DST_BIN}"
        install_lumerad_binary
      fi
    else
      echo "[BOOT] ${LUMERA_SRC_BIN} not found, assuming ${LUMERAD} is already installed"
    fi
  fi
}

launch_validator_setup() {
  install_or_update_lumerad
  if [ ! -s "${SETUP_COMPLETE}" ] && [ -x "${SCRIPTS_DIR}/validator-setup.sh" ]; then
    echo "[BOOT] ${MONIKER}: setup_complete missing; launching validator-setup in background..."
    nohup bash "${SCRIPTS_DIR}/validator-setup.sh" >"${VALIDATOR_SETUP_OUT}" 2>&1 &
  fi
}

launch_network_maker_setup() {
  if [ -x "${SCRIPTS_DIR}/network-maker-setup.sh" ] && [ -f "${RELEASE_DIR}/${NM}" ]; then
    echo "[BOOT] ${MONIKER}: Launching Network Maker setup in background..."
    nohup bash "${SCRIPTS_DIR}/network-maker-setup.sh" >"${NETWORK_MAKER_SETUP_OUT}" 2>&1 &
  fi
}

start_lumera() {
    echo "[BOOT] ${MONIKER}: Starting lumerad..."
    run "${DAEMON}" start \
        --home "${DAEMON_HOME}" \
        --minimum-gas-prices "${MIN_GAS_PRICE}" \
        --rpc.laddr "tcp://0.0.0.0:${LUMERA_RPC_PORT}" \
        --grpc.address "0.0.0.0:${LUMERA_GRPC_PORT}" >"${VALIDATOR_LOG}" 2>&1 &
}

tail_logs() {
    touch "${VALIDATOR_LOG}" "${SUPERNODE_LOG}" "${SUPERNODE_SETUP_OUT}" "${VALIDATOR_SETUP_OUT}" "${NETWORK_MAKER_SETUP_OUT}"
    exec tail -F "${VALIDATOR_LOG}" "${SUPERNODE_LOG}" "${SUPERNODE_SETUP_OUT}" "${VALIDATOR_SETUP_OUT}" "${NETWORK_MAKER_SETUP_OUT}"
}

case "${START_MODE}" in
  auto|*)
    launch_network_maker_setup
    launch_supernode_setup
    launch_validator_setup
    wait_for_validator_setup
    start_lumera
    tail_logs
    ;;

  bootstrap)
    launch_network_maker_setup
    launch_supernode_setup
    launch_validator_setup
    wait_for_validator_setup
    exit 0
    ;;

  run)
    wait_for_validator_setup
    wait_for_n_blocks 3 || { echo "[SN] Lumera chain not producing blocks in time; exiting."; exit 1; }
    start_lumera
    tail_logs
    ;;

  wait)
    wait_for_validator_setup
    exit 0
    ;;
esac
