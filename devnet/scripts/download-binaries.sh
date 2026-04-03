#!/usr/bin/env bash
# download-binaries.sh — Populate a devnet bin directory for a given lumera version
# using download URLs derived from devnet/config/binaries.json.
#
# Usage:
#   ./devnet/scripts/download-binaries.sh <lumera-version>
#
# Example:
#   ./devnet/scripts/download-binaries.sh v1.11.1
#
# The script reads devnet/config/binaries.json, looks up the requested version,
# and downloads lumerad, libwasmvm, supernode, and network-maker into the
# corresponding bin-<version> directory under devnet/.

set -euo pipefail

if [[ $# -ne 1 ]]; then
	echo "Usage: $0 <lumera-version>" >&2
	echo "  e.g. $0 v1.11.1" >&2
	exit 1
fi

VERSION="$1"
# Normalise: ensure version starts with 'v'
case "${VERSION}" in
	v*) ;;
	*)  VERSION="v${VERSION}" ;;
esac

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEVNET_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
CONFIG_FILE="${DEVNET_DIR}/config/binaries.json"

if [[ ! -f "${CONFIG_FILE}" ]]; then
	echo "Config file not found: ${CONFIG_FILE}" >&2
	exit 1
fi

if ! command -v jq &>/dev/null; then
	echo "jq is required but not installed. Install it with: apt-get install jq" >&2
	exit 1
fi

# ---------------------------------------------------------------------------
# Read version entry from JSON
# ---------------------------------------------------------------------------
VERSION_ENTRY="$(jq -r --arg v "${VERSION}" '.versions[$v] // empty' "${CONFIG_FILE}")"
if [[ -z "${VERSION_ENTRY}" ]]; then
	echo "Version ${VERSION} not found in ${CONFIG_FILE}" >&2
	echo "Available versions:" >&2
	jq -r '.versions | keys[]' "${CONFIG_FILE}" >&2
	exit 1
fi

GITHUB_ORG="$(jq -r '.github_org' "${CONFIG_FILE}")"
BIN_DIR_NAME="$(echo "${VERSION_ENTRY}" | jq -r '.bin_dir')"
BIN_DIR="${DEVNET_DIR}/${BIN_DIR_NAME}"

echo "==> Downloading binaries for ${VERSION} into ${BIN_DIR}"
mkdir -p "${BIN_DIR}"

TMPDIR="$(mktemp -d)"
trap 'rm -rf "${TMPDIR}"' EXIT

# ---------------------------------------------------------------------------
# Helper: download a GitHub release asset
# ---------------------------------------------------------------------------
download_asset() {
	local repo="$1" tag="$2" asset="$3" dest="$4"
	local url="https://github.com/${GITHUB_ORG}/${repo}/releases/download/${tag}/${asset}"

	echo "  Downloading ${url}"
	if ! curl -fSL --retry 3 --retry-delay 2 -o "${dest}" "${url}"; then
		echo "  FAILED to download ${url}" >&2
		return 1
	fi
}

# ---------------------------------------------------------------------------
# 1. Lumera (lumerad + libwasmvm from tarball)
# ---------------------------------------------------------------------------
LUMERA_TAG="$(echo "${VERSION_ENTRY}" | jq -r '.lumera.tag // empty')"
if [[ -n "${LUMERA_TAG}" ]]; then
	LUMERA_ASSET="lumera_${LUMERA_TAG}_linux_amd64.tar.gz"
	LUMERA_TAR="${TMPDIR}/${LUMERA_ASSET}"

	echo "--- lumera ${LUMERA_TAG} ---"
	download_asset "lumera" "${LUMERA_TAG}" "${LUMERA_ASSET}" "${LUMERA_TAR}"

	echo "  Extracting lumerad and libwasmvm..."
	tar xzf "${LUMERA_TAR}" -C "${TMPDIR}"
	# tarball must contain ./lumerad and ./libwasmvm.x86_64.so at root
	if [[ ! -f "${TMPDIR}/lumerad" ]]; then
		echo "  ERROR: expected lumerad in ${LUMERA_ASSET}, but ${TMPDIR}/lumerad was not found after extraction" >&2
		exit 1
	fi
	if [[ ! -f "${TMPDIR}/libwasmvm.x86_64.so" ]]; then
		echo "  ERROR: expected libwasmvm.x86_64.so in ${LUMERA_ASSET}, but ${TMPDIR}/libwasmvm.x86_64.so was not found after extraction" >&2
		exit 1
	fi
	cp -f "${TMPDIR}/lumerad" "${BIN_DIR}/lumerad"
	cp -f "${TMPDIR}/libwasmvm.x86_64.so" "${BIN_DIR}/libwasmvm.x86_64.so"
	chmod +x "${BIN_DIR}/lumerad"
	echo "  -> lumerad $(${BIN_DIR}/lumerad version 2>/dev/null | head -1 || echo '(version check skipped)')"
	# clean up tarball extracts for next component
	rm -f "${TMPDIR}/lumerad" "${TMPDIR}/libwasmvm.x86_64.so" "${TMPDIR}/install.sh"
fi

# ---------------------------------------------------------------------------
# 2. Supernode (direct binary)
# ---------------------------------------------------------------------------
SN_TAG="$(echo "${VERSION_ENTRY}" | jq -r '.supernode.tag // empty')"
if [[ -n "${SN_TAG}" ]]; then
	echo "--- supernode ${SN_TAG} ---"
	download_asset "supernode" "${SN_TAG}" "supernode-linux-amd64" "${BIN_DIR}/supernode-linux-amd64"
	chmod +x "${BIN_DIR}/supernode-linux-amd64"
	echo "  -> supernode-linux-amd64 installed"
fi

# ---------------------------------------------------------------------------
# 3. Network-maker (tarball with binary + ui)
# ---------------------------------------------------------------------------
NM_TAG="$(echo "${VERSION_ENTRY}" | jq -r '.network_maker.tag // empty')"
if [[ -n "${NM_TAG}" ]]; then
	NM_ASSET="network-maker_${NM_TAG}_linux_amd64.tar.gz"
	NM_TAR="${TMPDIR}/${NM_ASSET}"

	echo "--- network-maker ${NM_TAG} ---"
	download_asset "network-maker" "${NM_TAG}" "${NM_ASSET}" "${NM_TAR}"

	echo "  Extracting network-maker..."
	NM_EXTRACT="${TMPDIR}/nm"
	mkdir -p "${NM_EXTRACT}"
	tar xzf "${NM_TAR}" -C "${NM_EXTRACT}"

	[[ -f "${NM_EXTRACT}/network-maker" ]] && cp -f "${NM_EXTRACT}/network-maker" "${BIN_DIR}/network-maker" && chmod +x "${BIN_DIR}/network-maker"
	[[ -f "${NM_EXTRACT}/config.toml" ]] && cp -f "${NM_EXTRACT}/config.toml" "${BIN_DIR}/nm-config.toml"
	if [[ -d "${NM_EXTRACT}/ui" ]]; then
		rm -rf "${BIN_DIR}/nm-ui"
		cp -rf "${NM_EXTRACT}/ui" "${BIN_DIR}/nm-ui"
	fi
	echo "  -> network-maker installed"
fi

echo ""
echo "==> Done. Contents of ${BIN_DIR}:"
ls -lh "${BIN_DIR}"
