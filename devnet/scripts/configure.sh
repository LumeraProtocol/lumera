#!/bin/bash
set -euo pipefail

echo "Configuring Lumera for docker compose ..."

# Get the absolute path to the directory containing this script
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [ -d "${SCRIPT_DIR}/../bin" ]; then
    BIN_DIR="$(cd "${SCRIPT_DIR}/../bin" && pwd)"
else
    BIN_DIR=""
fi

# Require CONFIG_JSON environment variable
: "${CONFIG_JSON:?CONFIG_JSON environment variable must be set}"
echo "[CONFIGURE] Lumera chain config is $CONFIG_JSON"

# Require VALIDATORS_JSON environment variable
: "${VALIDATORS_JSON:?VALIDATORS_JSON environment variable must be set}"
echo "[CONFIGURE] Lumera validators config is $VALIDATORS_JSON"

if [ ! -f "${CONFIG_JSON}" ]; then
  echo "[CONFIGURE] Missing ${CONFIG_JSON}"; exit 1
fi

if [ ! -f "${VALIDATORS_JSON}" ]; then
  echo "[CONFIGURE] Missing ${VALIDATORS_JSON}"; exit 1
fi

if [ ! command -v jq >/dev/null 2>&1 ]; then
    echo "[CONFIGURE] jq is missing"
fi

CHAIN_ID="$(jq -r '.chain.id' "${CONFIG_JSON}")"
echo "[CONFIGURE] Lumera chain ID is $CHAIN_ID"

SHARED_DIR="/tmp/${CHAIN_ID}/shared"
CFG_DIR="${SHARED_DIR}/config"
RELEASE_DIR="${SHARED_DIR}/release"
SN="supernode-linux-amd64"
NM="network-maker"
NM_CFG="${BIN_DIR}/nm-config.toml"
SNCLI="sncli"
SNCLI_CFG="${BIN_DIR}/sncli-config.toml"

install_supernode() {
  if [ -n "${BIN_DIR}" ] || [ -f "${BIN_DIR}/${SN}" ]; then
    echo "[CONFIGURE] Copying supernode binary from ${BIN_DIR} to ${RELEASE_DIR}"
    cp -f "${BIN_DIR}/${SN}" "${RELEASE_DIR}/"
    chmod 755 "${RELEASE_DIR}/${SN}"
  fi
}

install_nm() {
  if [ -n "${BIN_DIR}" ] || [ -f "${BIN_DIR}/${NM}" ]; then
    # if nm-config.toml is missing - return an error
    if [ ! -f "${NM_CFG}" ]; then
      echo "[CONFIGURE] Missing ${NM_CFG}"
      exit 1
    fi
    echo "[CONFIGURE] Copying network-maker file from ${BIN_DIR} to ${RELEASE_DIR}"
    cp -f "${BIN_DIR}/${NM}" "${NM_CFG}" "${RELEASE_DIR}/"
    chmod 755 "${RELEASE_DIR}/${NM}"
  fi
}

install_sncli() {
  if [ -n "${BIN_DIR}" ] || [ -f "${BIN_DIR}/${SNCLI}" ]; then
    # if sncli-config.toml is missing - return an error
    if [ -f "${SNCLI_CFG}" ]; then
      echo "[CONFIGURE] Copying sncli config from ${BIN_DIR} to ${RELEASE_DIR}"
      cp -f "${SNCLI_CFG}" "${RELEASE_DIR}/"
    fi

    echo "[CONFIGURE] Copying sncli binary from ${BIN_DIR} to ${RELEASE_DIR}"
    cp -f "${BIN_DIR}/${SNCLI}" "${RELEASE_DIR}/"
    chmod 755 "${RELEASE_DIR}/${SNCLI}"
  fi
}

mkdir -p "${CFG_DIR}" "${RELEASE_DIR}"
cp -f "${CONFIG_JSON}" "${VALIDATORS_JSON}" "${CFG_DIR}/"
echo "[CONFIGURE] Configuration files copied to ${CFG_DIR}"

install_supernode
install_sncli
install_nm
echo "[CONFIGURE] Lumera configuration completed successfully."