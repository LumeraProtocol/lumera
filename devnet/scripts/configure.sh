#!/bin/bash
set -euo pipefail

echo "Configuring Lumera for docker compose ..."

# --- parse args -----------------------------------
BIN_DIR_ARG=""
show_help() {
  cat <<'EOF'
Usage: configure.sh [--bin-dir DIR]

Options:
  -b, --bin-dir DIR   Directory containing binaries/configs to copy - absolute or relative to the repo root
                      (supernode-linux-amd64, sncli, network-maker, configs).
  -h, --help          Show this help and exit.
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    -b|--bin-dir)
      [[ -n "${2:-}" && "${2:0:1}" != "-" ]] || { echo "ERROR: --bin-dir requires DIR" >&2; exit 2; }
      BIN_DIR_ARG="$2"; shift 2;;
    --bin-dir=*) BIN_DIR_ARG="${1#*=}"; shift;;
    -h|--help)   show_help; exit 0;;
    *)           echo "ERROR: unknown arg: $1" >&2; show_help; exit 2;;
  esac
done

# --- resolve script dir & BIN_DIR (CLI > autodetect ../bin > empty) -----------

# Get the absolute path to the directory containing this script
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# Prefer git to find the real root; fallback to scripts/..
REPO_ROOT="$(git -C "${SCRIPT_DIR}" rev-parse --show-toplevel 2>/dev/null || true)"
[[ -n "${REPO_ROOT}" ]] || REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# --- resolve BIN_DIR (CLI > repo-root/bin > empty) ----------------------------
if [[ -n "${BIN_DIR_ARG}" ]]; then
  # Absolute path stays absolute; relative is interpreted from REPO_ROOT
  if [[ "${BIN_DIR_ARG}" = /* ]]; then
    CAND="${BIN_DIR_ARG}"
  else
    CAND="${REPO_ROOT}/${BIN_DIR_ARG}"
  fi
  # Normalize & verify
  if BIN_DIR="$(cd "${CAND}" 2>/dev/null && pwd)"; then
    :
  else
    echo "[CONFIGURE] ERROR: --bin-dir '${BIN_DIR_ARG}' not found under ${REPO_ROOT}" >&2
    exit 1
  fi
elif [[ -d "${SCRIPT_DIR}/../bin" ]]; then
  BIN_DIR="$(cd "${SCRIPT_DIR}/../bin" && pwd)"
else
  BIN_DIR=""
fi

if [[ -n "${BIN_DIR}" ]]; then
  echo "[CONFIGURE] Using BIN_DIR=${BIN_DIR}"
else
  echo "[CONFIGURE] ERROR ! BIN_DIR not provided andcould not be resolved"
  exit 1
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
NM_UI_SRC="${BIN_DIR}/nm-ui"
NM_UI_DST="${RELEASE_DIR}/nm-ui"

install_supernode() {
  if [ -n "${BIN_DIR}" ] && [ -f "${BIN_DIR}/${SN}" ]; then
    echo "[CONFIGURE] Copying supernode binary from ${BIN_DIR} to ${RELEASE_DIR}"
    cp -f "${BIN_DIR}/${SN}" "${RELEASE_DIR}/"
    chmod 755 "${RELEASE_DIR}/${SN}"
  fi
}

install_nm() {
  if [ -n "${BIN_DIR}" ] && [ -f "${BIN_DIR}/${NM}" ]; then
    # if nm-config.toml is missing - return an error
    if [ ! -f "${NM_CFG}" ]; then
      echo "[CONFIGURE] Missing ${NM_CFG}"
      exit 1
    fi
    echo "[CONFIGURE] Copying network-maker file from ${BIN_DIR} to ${RELEASE_DIR}"
    cp -f "${BIN_DIR}/${NM}" "${NM_CFG}" "${RELEASE_DIR}/"
    chmod 755 "${RELEASE_DIR}/${NM}"

    if [ -d "${NM_UI_SRC}" ]; then
      echo "[CONFIGURE] Copying network-maker UI from ${NM_UI_SRC} to ${NM_UI_DST}"
      rm -rf "${NM_UI_DST}"
      cp -r "${NM_UI_SRC}" "${NM_UI_DST}"
    else
      echo "[CONFIGURE] network-maker UI not found at ${NM_UI_SRC}; skipping UI copy"
    fi
  fi
}

install_sncli() {
  if [ -n "${BIN_DIR}" ] && [ -f "${BIN_DIR}/${SNCLI}" ]; then
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
