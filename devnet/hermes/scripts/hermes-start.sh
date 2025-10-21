#!/usr/bin/env bash
# shellcheck disable=SC2086
# --------------------------------------------------------------------------------------------------
# Hermes+SIMD container entrypoint.
#  - Boots a local simd chain backed by ibc-go.
#  - Launches the Hermes relayer once a config file is present.
# --------------------------------------------------------------------------------------------------
set -uo pipefail

SIMD_HOME="${SIMD_HOME:-/root/.simd}"
SIMD_MONIKER="${SIMD_MONIKER:-hermes-simd}"
SIMD_CHAIN_ID="${SIMD_CHAIN_ID:-hermes-simd-1}"
SIMD_KEY_NAME="${SIMD_KEY_NAME:-validator}"
SIMD_KEYRING="${SIMD_KEYRING:-test}"
SIMD_DENOM="${SIMD_DENOM:-stake}"
SIMD_GENESIS_BALANCE="${SIMD_GENESIS_BALANCE:-100000000000${SIMD_DENOM}}"
SIMD_STAKE_AMOUNT="${SIMD_STAKE_AMOUNT:-50000000000${SIMD_DENOM}}"
RELAYER_KEY_NAME="${RELAYER_KEY_NAME:-relayer}"
DEFAULT_SIMD_RELAYER_MNEMONIC=""

SIMAPP_KEY_RELAYER_MNEMONIC="${SIMAPP_KEY_RELAYER_MNEMONIC:-${DEFAULT_SIMD_RELAYER_MNEMONIC}}"
export SIMAPP_KEY_RELAYER_MNEMONIC
export SIMAPP_SHARED_DIR="/shared"

SIMD_P2P_PORT="${SIMD_P2P_PORT:-26656}"
SIMD_RPC_PORT="${SIMD_RPC_PORT:-26657}"
SIMD_API_PORT="${SIMD_API_PORT:-1317}"
SIMD_GRPC_PORT="${SIMD_GRPC_PORT:-9090}"
SIMD_GRPC_WEB_PORT="${SIMD_GRPC_WEB_PORT:-9091}"

SIMD_LOG_DIR="${SIMD_LOG_DIR:-/var/log/simd}"
HERMES_LOG_DIR="${HERMES_LOG_DIR:-/var/log/hermes}"
LOG_DIR="/root/logs"
mkdir -p "${SIMD_HOME}" "${SIMD_LOG_DIR}" "${HERMES_LOG_DIR}" "${LOG_DIR}"

SIMD_LOG_FILE="${LOG_DIR}/simd.log"
HERMES_LOG_FILE="${LOG_DIR}/hermes.log"
ENTRY_LOG_FILE="${LOG_DIR}/entrypoint.log"
SIMAPP_INIT_LOG="${LOG_DIR}/simapp-init.log"

touch "${SIMD_LOG_FILE}" "${HERMES_LOG_FILE}" "${ENTRY_LOG_FILE}" "${SIMAPP_INIT_LOG}"
export ENTRY_LOG_FILE

log_info() {
  local msg="$1"
  local line
  line=$(printf '[%s] %s\n' "$(date -u +"%Y-%m-%dT%H:%M:%SZ")" "${msg}")
  printf '%s\n' "${line}"
  printf '%s\n' "${line}" >> "${ENTRY_LOG_FILE}"
}

fmt_cmd() {
  local out="" arg
  for arg in "$@"; do
    if [ -z "${out}" ]; then
      out=$(printf '%q' "${arg}")
    else
      out="${out} $(printf '%q' "${arg}")"
    fi
  done
  printf '%s' "${out}"
}

log_cmd_start() {
  log_info "CMD start: $(fmt_cmd "$@")"
}

log_cmd_result() {
  local rc="$1"
  shift
  if [ "${rc}" -eq 0 ]; then
    log_info "CMD success (rc=${rc}): $(fmt_cmd "$@")"
  else
    log_info "CMD failure (rc=${rc}): $(fmt_cmd "$@")"
  fi
}

log_cmd_output() {
  local label="$1"
  local payload="$2"
  local lines_logged=0
  if [ -z "${payload}" ]; then
    return 0
  fi
  while IFS= read -r line; do
    log_info "${label}: ${line}"
    lines_logged=$((lines_logged + 1))
    if [ "${lines_logged}" -ge 40 ]; then
      log_info "${label}: ... (truncated after 40 lines)"
      break
    fi
  done <<< "${payload}"
}

ran() {
  local cmd=("$@")
  log_cmd_start "${cmd[@]}"
  "${cmd[@]}"
  local rc=$?
  log_cmd_result "${rc}" "${cmd[@]}"
  return "${rc}"
}

ran_capture() {
  local cmd=("$@")
  log_cmd_start "${cmd[@]}"
  local output rc
  if output=$("${cmd[@]}" 2>&1); then
    rc=0
  else
    rc=$?
  fi
  log_cmd_output "CMD output" "${output}"
  log_cmd_result "${rc}" "${cmd[@]}"
  printf '%s' "${output}"
  return "${rc}"
}

SHARED_DIR="/shared"
HERMES_SHARED_DIR="${SHARED_DIR}/hermes"
CONFIG_JSON="${SHARED_DIR}/config/config.json"
VALIDATORS_JSON="${SHARED_DIR}/config/validators.json"
mkdir -p "${HERMES_SHARED_DIR}"

HERMES_RELAYER_MNEMONIC_FILE="${HERMES_SHARED_DIR}/hermes-relayer.mnemonic"

if [ -z "${SIMAPP_KEY_RELAYER_MNEMONIC:-}" ] && [ -s "${HERMES_RELAYER_MNEMONIC_FILE}" ]; then
  SIMAPP_KEY_RELAYER_MNEMONIC="$(cat "${HERMES_RELAYER_MNEMONIC_FILE}")"
fi

if command -v jq >/dev/null 2>&1 && [ -f "${CONFIG_JSON}" ]; then
  LUMERA_CHAIN_ID="${LUMERA_CHAIN_ID:-$(jq -r '.chain.id' "${CONFIG_JSON}")}"
  LUMERA_BOND_DENOM="${LUMERA_BOND_DENOM:-$(jq -r '.chain.denom.bond' "${CONFIG_JSON}")}"
fi

if [ -z "${LUMERA_CHAIN_ID:-}" ] || [ "${LUMERA_CHAIN_ID}" = "null" ]; then
  LUMERA_CHAIN_ID="lumera-devnet-1"
fi

if [ -z "${LUMERA_BOND_DENOM:-}" ] || [ "${LUMERA_BOND_DENOM}" = "null" ]; then
  LUMERA_BOND_DENOM="ulume"
fi

if command -v jq >/dev/null 2>&1 && [ -f "${VALIDATORS_JSON}" ]; then
  FIRST_VALIDATOR_SERVICE="$(jq -r '([.[] | select(."network-maker"==true) | .name] | first) // empty' "${VALIDATORS_JSON}")"
  if [ -z "${FIRST_VALIDATOR_SERVICE}" ] || [ "${FIRST_VALIDATOR_SERVICE}" = "null" ]; then
    FIRST_VALIDATOR_SERVICE="$(jq -r '.[0].name // empty' "${VALIDATORS_JSON}")"
  fi
fi

if [ -z "${FIRST_VALIDATOR_SERVICE:-}" ] || [ "${FIRST_VALIDATOR_SERVICE}" = "null" ]; then
  FIRST_VALIDATOR_SERVICE="supernova_validator_1"
fi

# Inside the compose network every validator exposes the default container ports.
LUMERA_RPC_PORT="${LUMERA_RPC_PORT:-26657}"
LUMERA_GRPC_PORT="${LUMERA_GRPC_PORT:-9090}"

LUMERA_RPC_ADDR="http://${FIRST_VALIDATOR_SERVICE}:${LUMERA_RPC_PORT}"
LUMERA_GRPC_ADDR="http://${FIRST_VALIDATOR_SERVICE}:${LUMERA_GRPC_PORT}"
LUMERA_WS_ADDR="ws://${FIRST_VALIDATOR_SERVICE}:${LUMERA_RPC_PORT}/websocket"
LUMERA_ACCOUNT_PREFIX="${LUMERA_ACCOUNT_PREFIX:-lumera}"
HERMES_KEY_NAME="${HERMES_KEY_NAME:-${RELAYER_KEY_NAME}}"

LUMERA_MNEMONIC_FILE="${HERMES_RELAYER_MNEMONIC_FILE}"
SIMD_MNEMONIC_FILE="${HERMES_RELAYER_MNEMONIC_FILE}"

HERMES_TEMPLATE_PATH="${HERMES_TEMPLATE_PATH:-/root/scripts/hermes-config-template.toml}"

export HERMES_CONFIG_PATH
export HERMES_TEMPLATE_PATH
export LUMERA_CHAIN_ID LUMERA_BOND_DENOM LUMERA_RPC_ADDR LUMERA_GRPC_ADDR LUMERA_WS_ADDR LUMERA_ACCOUNT_PREFIX
export SIMD_CHAIN_ID SIMD_DENOM SIMD_RPC_PORT SIMD_GRPC_PORT
export HERMES_KEY_NAME LUMERA_MNEMONIC_FILE SIMD_MNEMONIC_FILE

if [ -n "${SIMAPP_KEY_RELAYER_MNEMONIC:-}" ] && [ ! -s "${SIMD_MNEMONIC_FILE}" ]; then
  printf '%s\n' "${SIMAPP_KEY_RELAYER_MNEMONIC}" > "${SIMD_MNEMONIC_FILE}"
fi

HERMES_CONFIG_DEFAULT="/root/.hermes/config.toml"
HERMES_CONFIG_PATH="${HERMES_CONFIG:-${HERMES_CONFIG_DEFAULT}}"

log_info "SIMD home: ${SIMD_HOME}"
log_info "Hermes config: ${HERMES_CONFIG_PATH}"

init_simd_home() {
  export SIMD_HOME SIMAPP_HOME="${SIMD_HOME}"
  export SIMD_CHAIN_ID CHAIN_ID="${SIMD_CHAIN_ID}"
  export SIMD_MONIKER MONIKER="${SIMD_MONIKER}"
  export SIMD_KEY_NAME KEY_NAME="${SIMD_KEY_NAME}"
  export RELAYER_KEY_NAME
  export SIMD_KEYRING KEYRING="${SIMD_KEYRING}"
  export SIMD_DENOM DENOM="${SIMD_DENOM}" STAKE_DENOM="${SIMD_DENOM}"
  export SIMD_GENESIS_BALANCE ACCOUNT_BALANCE="${SIMD_GENESIS_BALANCE}"
  export SIMD_STAKE_AMOUNT STAKING_AMOUNT="${SIMD_STAKE_AMOUNT}"
  export SIMAPP_KEY_RELAYER_MNEMONIC
  export SIMAPP_SHARED_DIR="${SHARED_DIR}"

  log_info "Ensuring simd home initialised (logs -> ${SIMAPP_INIT_LOG})"
  if ran /root/scripts/init-simapp.sh >>"${SIMAPP_INIT_LOG}" 2>&1; then
    log_info "simd home initialised"
  else
    local rc=$?
    log_info "init-simapp.sh failed with exit code ${rc} (see ${SIMAPP_INIT_LOG})"
    return ${rc}
  fi
}

wait_for_lumera_rpc() {
  local url="${LUMERA_RPC_ADDR}/status"
  log_info "Waiting for Lumera RPC (${url})..."
  for _ in $(seq 1 120); do
    if curl -sf "${url}" >/dev/null 2>&1; then
      log_info "Lumera RPC is reachable."
      return 0
    fi
    sleep 2
  done
  log_info "Lumera RPC did not become ready in time."
  return 1
}

current_lumera_height() {
  local url="${LUMERA_RPC_ADDR}/status"
  curl -sf "${url}" 2>/dev/null \
    | jq -r '.result.sync_info.latest_block_height // "0"' 2>/dev/null \
    | awk '($1 ~ /^[0-9]+$/) { print $1; next } { print 0 }'
}

wait_for_lumera_height() {
  local target="$1"
  local retries="${2:-180}"
  local delay="${3:-2}"

  log_info "Waiting for Lumera height to reach at least ${target}..."
  for _ in $(seq 1 "${retries}"); do
    local height
    height="$(current_lumera_height)"
    if (( height >= target )); then
      log_info "Lumera height is ${height} (>= ${target})."
      return 0
    fi
    sleep "${delay}"
  done
  log_info "Timed out waiting for Lumera height ${target}; continuing anyway."
  return 1
}

wait_for_lumera_blocks() {
  local blocks="${1:-5}"
  local start
  start="$(current_lumera_height)"
  local target=$(( start + blocks ))
  if (( target < blocks )); then
    target="${blocks}"
  fi
  wait_for_lumera_height "${target}"
}

start_simd() {
  log_info "Starting simd node..."
  simd start \
    --home "${SIMD_HOME}" \
    --pruning=nothing \
    --rpc.laddr "tcp://0.0.0.0:${SIMD_RPC_PORT}" \
    --grpc.address "0.0.0.0:${SIMD_GRPC_PORT}" \
    --address "tcp://0.0.0.0:${SIMD_P2P_PORT}" \
    --api.enable true \
    --api.address "tcp://0.0.0.0:${SIMD_API_PORT}" \
    --minimum-gas-prices "0${SIMD_DENOM}" \
    >"${SIMD_LOG_FILE}" 2>&1 &
  SIMD_PID=$!
  echo "${SIMD_PID}" > /var/run/simd.pid
  log_info "simd started with PID ${SIMD_PID}, logs -> ${SIMD_LOG_FILE}"
}

wait_for_simd() {
  local url="http://127.0.0.1:${SIMD_RPC_PORT}/status"
  log_info "Waiting for simd RPC (${url})..."
  for _ in $(seq 1 120); do
    if curl -sf "${url}" >/dev/null 2>&1; then
      log_info "simd RPC is online."
      return 0
    fi
    sleep 1
  done
  log_info "simd RPC did not become ready in time."
  return 1
}

start_hermes() {
  log_info "Starting Hermes relayer..."
  hermes --config "${HERMES_CONFIG_PATH}" start >"${HERMES_LOG_FILE}" 2>&1 &
  HERMES_PID=$!
  echo "${HERMES_PID}" > /var/run/hermes.pid
  log_info "Hermes started with PID ${HERMES_PID}, logs -> ${HERMES_LOG_FILE}"
}

cleanup() {
  log_info "Caught termination, stopping processes..."
  [[ -n "${HERMES_PID:-}" ]] && kill "${HERMES_PID}" >/dev/null 2>&1 || true
  [[ -n "${SIMD_PID:-}" ]] && kill "${SIMD_PID}" >/dev/null 2>&1 || true
  [[ -n "${TAIL_PID:-}" ]] && kill "${TAIL_PID}" >/dev/null 2>&1 || true
  wait || true
}

trap cleanup EXIT INT TERM

if init_simd_home; then
  log_info "simd home ready"
else
  log_info "init_simd_home failed; continuing to keep container alive"
fi
if start_simd; then
  if wait_for_simd; then
    :
  else
    log_info "wait_for_simd timed out; continuing to keep container alive"
  fi
else
  log_info "start_simd failed; continuing to keep container alive"
fi

if wait_for_lumera_rpc; then
  :
else
  log_info "Lumera RPC unreachable after timeout"
fi
wait_for_lumera_blocks 5 || true

if ran /root/scripts/hermes-configure.sh; then
  log_info "Hermes config ensured"
else
  log_info "hermes-configure.sh failed; continuing"
fi

if ran /root/scripts/hermes-channel.sh; then
  log_info "Hermes channel/keys ensured"
else
  log_info "hermes-channel.sh reported failure"
fi

if start_hermes; then
  :
else
  log_info "start_hermes failed; continuing to keep container alive"
fi

log_info "Tailing logs (simd + hermes)..."
log_cmd_start tail -F "${SIMD_LOG_FILE}" "${HERMES_LOG_FILE}"
tail -F "${SIMD_LOG_FILE}" "${HERMES_LOG_FILE}" &
TAIL_PID=$!

log_info "Processes started (simd PID=${SIMD_PID:-n/a}, hermes PID=${HERMES_PID:-n/a})."
wait -n "${SIMD_PID:-}" "${HERMES_PID:-}" || true

log_info "Entering idle loop to keep container alive. Use Ctrl+C to exit."
while true; do
  sleep 300 &
  wait $! || true
done
