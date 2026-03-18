#!/bin/bash
#
# Host-side devnet configuration script.
#
# This script runs on the HOST (not inside Docker) as part of `make devnet-build-*`.
# It prepares the shared volume (/tmp/<chain-id>/shared/) that all validator
# containers will mount. Specifically:
#
#   1. Copies config.json + validators.json into /shared/config/
#   2. Copies optional binaries (supernode, sncli, network-maker, test binaries)
#      from BIN_DIR into /shared/release/ so containers can install them
#
# Usage:
#   CONFIG_JSON=path/to/config.json VALIDATORS_JSON=path/to/validators.json \
#     ./configure.sh [--bin-dir devnet/bin]
#
# The shared volume layout after this script:
#   /tmp/<chain-id>/shared/
#     config/config.json        ← chain config
#     config/validators.json    ← validator specs
#     release/supernode-linux-amd64  ← optional
#     release/sncli                  ← optional
#     release/sncli-config.toml      ← optional
#     release/network-maker          ← optional
#     release/nm-config.toml         ← optional (required if NM binary present)
#     release/nm-ui/                 ← optional (NM static web UI)
#     release/tests_*                ← optional test binaries
#
set -euo pipefail

echo "Configuring Lumera for docker compose ..."

# ─── Argument Parsing ─────────────────────────────────────────────────────────

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
	-b | --bin-dir)
		[[ -n "${2:-}" && "${2:0:1}" != "-" ]] || {
			echo "ERROR: --bin-dir requires DIR" >&2
			exit 2
		}
		BIN_DIR_ARG="$2"
		shift 2
		;;
	--bin-dir=*)
		BIN_DIR_ARG="${1#*=}"
		shift
		;;
	-h | --help)
		show_help
		exit 0
		;;
	*)
		echo "ERROR: unknown arg: $1" >&2
		show_help
		exit 2
		;;
	esac
done

# ─── Resolve Paths ────────────────────────────────────────────────────────────
# BIN_DIR resolution order: --bin-dir flag > devnet/bin/ (auto-detected) > error
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# Prefer git to find the real root; fallback to scripts/..
REPO_ROOT="$(git -C "${SCRIPT_DIR}" rev-parse --show-toplevel 2>/dev/null || true)"
[[ -n "${REPO_ROOT}" ]] || REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Resolve BIN_DIR: CLI arg takes precedence, else auto-detect from repo layout
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
	echo "[CONFIGURE] ERROR ! BIN_DIR not provided and could not be resolved"
	exit 1
fi

# ─── Validate Inputs ──────────────────────────────────────────────────────────

: "${CONFIG_JSON:?CONFIG_JSON environment variable must be set}"
echo "[CONFIGURE] Lumera chain config is $CONFIG_JSON"

: "${VALIDATORS_JSON:?VALIDATORS_JSON environment variable must be set}"
echo "[CONFIGURE] Lumera validators config is $VALIDATORS_JSON"

if [ ! -f "${CONFIG_JSON}" ]; then
	echo "[CONFIGURE] Missing ${CONFIG_JSON}"
	exit 1
fi

if [ ! -f "${VALIDATORS_JSON}" ]; then
	echo "[CONFIGURE] Missing ${VALIDATORS_JSON}"
	exit 1
fi

if ! command -v jq >/dev/null 2>&1; then
	echo "[CONFIGURE] jq is missing"
fi

# ─── Shared Volume Setup ──────────────────────────────────────────────────────
# The shared directory lives on the host at /tmp/<chain-id>/ and is bind-mounted
# to /shared/ inside each Docker container.

CHAIN_ID="$(jq -r '.chain.id' "${CONFIG_JSON}")"
echo "[CONFIGURE] Lumera chain ID is $CHAIN_ID"

SHARED_DIR="/tmp/${CHAIN_ID}/shared"
CFG_DIR="${SHARED_DIR}/config"
RELEASE_DIR="${SHARED_DIR}/release"

# Binary names and config paths in BIN_DIR
SN="supernode-linux-amd64"
NM="network-maker"
NM_CFG="${BIN_DIR}/nm-config.toml"
SNCLI="sncli"
SNCLI_CFG="${BIN_DIR}/sncli-config.toml"
NM_UI_SRC="${BIN_DIR}/nm-ui"
NM_UI_DST="${RELEASE_DIR}/nm-ui"

# ─── Binary Copy Functions ────────────────────────────────────────────────────
# Each function copies a binary (+ optional config) from BIN_DIR to RELEASE_DIR.
# All are optional — scripts in-container handle missing binaries gracefully.

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

# Copy devnet test binaries (used by `make devnet-evmigration-*` etc.)
install_tests() {
	local test_bins=("tests_validator" "tests_hermes" "tests_evmigration")
	local bin
	for bin in "${test_bins[@]}"; do
		if [ -n "${BIN_DIR}" ] && [ -f "${BIN_DIR}/${bin}" ]; then
			echo "[CONFIGURE] Copying ${bin} binary from ${BIN_DIR} to ${RELEASE_DIR}"
			cp -f "${BIN_DIR}/${bin}" "${RELEASE_DIR}/"
			chmod 755 "${RELEASE_DIR}/${bin}"
		fi
	done
}

# ─── Execute ──────────────────────────────────────────────────────────────────

mkdir -p "${CFG_DIR}" "${RELEASE_DIR}"

# Copy the two config files that drive all container-side setup scripts
cp -f "${CONFIG_JSON}" "${VALIDATORS_JSON}" "${CFG_DIR}/"
echo "[CONFIGURE] Configuration files copied to ${CFG_DIR}"

# Copy optional binaries from BIN_DIR into the shared release directory
install_supernode
install_sncli
install_nm
install_tests

echo "[CONFIGURE] Lumera configuration completed successfully."
