#!/usr/bin/env bash
# Idempotent bootstrap of a local simd home used by Hermes testing container.

set -euo pipefail

log() {
  echo "[SIMAPP] $*" >&2
}

run() {
  log "$*"
  "$@"
}

run_capture() {
  log "$*"
  "$@"
}

SIMD_BIN="${SIMD_BIN:-$(command -v simd 2>/dev/null || true)}"
if [ -z "${SIMD_BIN}" ]; then
  log "simd binary not found (SIMD_BIN unset and simd not in PATH)"
  exit 1
fi

SIMAPP_HOME="${SIMAPP_HOME:-${SIMD_HOME:-/root/.simd}}"
CHAIN_ID="${CHAIN_ID:-${SIMD_CHAIN_ID:-simapp-1}}"
MONIKER="${MONIKER:-${SIMD_MONIKER:-simapp-node}}"
KEY_NAME="${KEY_NAME:-${SIMD_KEY_NAME:-validator}}"
RELAYER_KEY_NAME="${RELAYER_KEY_NAME:-${KEY_NAME}}"
KEYRING="${KEYRING:-${SIMD_KEYRING:-test}}"
DENOM="${DENOM:-${SIMD_DENOM:-stake}}"
STAKE_DENOM="${STAKE_DENOM:-${DENOM}}"
ACCOUNT_BALANCE="${ACCOUNT_BALANCE:-${SIMD_GENESIS_BALANCE:-100000000000${DENOM}}}"
STAKING_AMOUNT="${STAKING_AMOUNT:-${SIMD_STAKE_AMOUNT:-50000000000${DENOM}}}"
SIMAPP_SHARED_DIR="${SIMAPP_SHARED_DIR:-/shared}"

GENESIS_FILE="${SIMAPP_HOME}/config/genesis.json"

key_mnemonic_file() {
  local name="$1"
  printf '%s/hermes/simd-%s.mnemonic' "${SIMAPP_SHARED_DIR}" "${name}"
}

record_key_mnemonic() {
  local name="$1"
  local mnemonic="$2"
  [ -z "${mnemonic}" ] && return 0
  local file
  file="$(key_mnemonic_file "${name}")"
  mkdir -p "$(dirname "${file}")"
  printf '%s\n' "${mnemonic}" > "${file}"
}

RELAYER_MNEMONIC_FILE="${SIMAPP_SHARED_DIR}/hermes/hermes-relayer.mnemonic"

record_relayer_mnemonic() {
  local mnemonic="$1"
  [ -z "${mnemonic}" ] && return 0
  record_key_mnemonic "${RELAYER_KEY_NAME}" "${mnemonic}"
  mkdir -p "$(dirname "${RELAYER_MNEMONIC_FILE}")"
  printf '%s\n' "${mnemonic}" > "${RELAYER_MNEMONIC_FILE}"
}

if [ -z "${SIMAPP_KEY_RELAYER_MNEMONIC:-}" ] && [ -s "${RELAYER_MNEMONIC_FILE}" ]; then
  SIMAPP_KEY_RELAYER_MNEMONIC="$(cat "${RELAYER_MNEMONIC_FILE}")"
  export SIMAPP_KEY_RELAYER_MNEMONIC
fi

if [ -f "${GENESIS_FILE}" ]; then
  if [ -n "${SIMAPP_KEY_RELAYER_MNEMONIC:-}" ]; then
    record_relayer_mnemonic "${SIMAPP_KEY_RELAYER_MNEMONIC}"
  fi
  log "simd already initialised at ${GENESIS_FILE}"
  exit 0
fi

log "Initialising simd home at ${SIMAPP_HOME}"

if [ -d "${SIMAPP_HOME}" ]; then
  if findmnt -rn -T "${SIMAPP_HOME}" >/dev/null 2>&1; then
    log "${SIMAPP_HOME} is a mount point; clearing existing contents"
    find "${SIMAPP_HOME}" -mindepth 1 -maxdepth 1 -exec rm -rf {} +
  else
    run rm -rf "${SIMAPP_HOME}"
  fi
fi

mkdir -p "${SIMAPP_HOME}"

run "${SIMD_BIN}" --home "${SIMAPP_HOME}" init "${MONIKER}" --chain-id "${CHAIN_ID}"
run "${SIMD_BIN}" --home "${SIMAPP_HOME}" config set client chain-id "${CHAIN_ID}"
run "${SIMD_BIN}" --home "${SIMAPP_HOME}" config set client keyring-backend "${KEYRING}"
run "${SIMD_BIN}" --home "${SIMAPP_HOME}" config set app api.enable true || true

ensure_key() {
  local name="$1"
  local upper var mnemonic json mnemonic_file
  upper=$(echo "${name}" | tr '[:lower:]-' '[:upper:]_')
  var="SIMAPP_KEY_${upper}_MNEMONIC"
  mnemonic="${!var:-}"
  mnemonic_file="$(key_mnemonic_file "${name}")"

  if [ -z "${mnemonic}" ] && [ -s "${mnemonic_file}" ]; then
    mnemonic="$(cat "${mnemonic_file}")"
    printf -v "${var}" '%s' "${mnemonic}"
    export "${var}"
  fi

  if "${SIMD_BIN}" --home "${SIMAPP_HOME}" keys show "${name}" --keyring-backend "${KEYRING}" >/dev/null 2>&1; then
    record_key_mnemonic "${name}" "${mnemonic}"
    if [ "${name}" = "${RELAYER_KEY_NAME}" ]; then
      record_relayer_mnemonic "${mnemonic}"
    fi
    return 0
  fi

  if [ -n "${mnemonic}" ]; then
    log "Restoring key ${name} from mnemonic"
    printf '%s\n' "${mnemonic}" | "${SIMD_BIN}" --home "${SIMAPP_HOME}" keys add "${name}" --keyring-backend "${KEYRING}" --recover >/dev/null
  else
    log "Creating key ${name}"
    json=$(run_capture "${SIMD_BIN}" --home "${SIMAPP_HOME}" keys add "${name}" --keyring-backend "${KEYRING}" --output json)
    if command -v jq >/dev/null 2>&1; then
      mnemonic=$(printf '%s' "${json}" | jq -r '.mnemonic // empty' 2>/dev/null || true)
      if [ -n "${mnemonic}" ]; then
        printf -v "${var}" '%s' "${mnemonic}"
        export "${var}"
      fi
    fi
  fi

  record_key_mnemonic "${name}" "${mnemonic}"

  if [ "${name}" = "${RELAYER_KEY_NAME}" ]; then
    record_relayer_mnemonic "${mnemonic}"
  fi
}

ensure_key "${KEY_NAME}"
if [ "${RELAYER_KEY_NAME}" != "${KEY_NAME}" ]; then
  ensure_key "${RELAYER_KEY_NAME}"
fi

run "${SIMD_BIN}" --home "${SIMAPP_HOME}" genesis add-genesis-account "${KEY_NAME}" "${ACCOUNT_BALANCE}" --keyring-backend "${KEYRING}"
if [ "${RELAYER_KEY_NAME}" != "${KEY_NAME}" ]; then
  run "${SIMD_BIN}" --home "${SIMAPP_HOME}" genesis add-genesis-account "${RELAYER_KEY_NAME}" "${ACCOUNT_BALANCE}" --keyring-backend "${KEYRING}"
fi

run "${SIMD_BIN}" --home "${SIMAPP_HOME}" genesis gentx "${KEY_NAME}" "${STAKING_AMOUNT}" --chain-id "${CHAIN_ID}" --keyring-backend "${KEYRING}"
run "${SIMD_BIN}" --home "${SIMAPP_HOME}" genesis collect-gentxs

if [ ! -f "${GENESIS_FILE}" ]; then
  log "Failed to create genesis at ${GENESIS_FILE}"
  exit 1
fi

if [ -n "${SIMAPP_KEY_RELAYER_MNEMONIC:-}" ]; then
  record_relayer_mnemonic "${SIMAPP_KEY_RELAYER_MNEMONIC}"
fi

log "simd home initialised at ${SIMAPP_HOME}"
