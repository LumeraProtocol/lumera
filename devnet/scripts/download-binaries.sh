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
# and downloads lumerad, libwasmvm, supernode, and lumera-uploader (or network-maker for <v1.11.0) into the
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
# 3. Lumera Uploader / Network-maker (tarball with binary + ui)
#    >= v1.11.0 the project is called "lumera-uploader"; older uses "network-maker".
# ---------------------------------------------------------------------------

# Simple host-side version comparison (no common.sh dependency).
_version_strip_v() { local v="${1#v}"; printf '%s' "$v"; }
_host_version_ge() {
	local cur="$(_version_strip_v "$1")" floor="$(_version_strip_v "$2")"
	printf '%s\n' "$floor" "$cur" | sort -V | head -n 1 | grep -q "^${floor}\$"
}

UPLOADER_TAG=""
UPLOADER_BIN_NAME=""
UPLOADER_REPO=""

# Try new name first (lumera_uploader), fall back to old (network_maker)
LU_TAG="$(echo "${VERSION_ENTRY}" | jq -r '.lumera_uploader.tag // empty')"
NM_TAG="$(echo "${VERSION_ENTRY}" | jq -r '.network_maker.tag // empty')"

if [[ -n "${LU_TAG}" ]] || _host_version_ge "${VERSION}" "v1.11.0"; then
	UPLOADER_TAG="${LU_TAG}"
	UPLOADER_BIN_NAME="lumera-uploader"
	UPLOADER_REPO="lumera-uploader"
elif [[ -n "${NM_TAG}" ]]; then
	UPLOADER_TAG="${NM_TAG}"
	UPLOADER_BIN_NAME="network-maker"
	UPLOADER_REPO="network-maker"
fi

if [[ -n "${UPLOADER_TAG}" ]]; then
	UL_ASSET="${UPLOADER_BIN_NAME}_${UPLOADER_TAG}_linux_amd64.tar.gz"
	UL_TAR="${TMPDIR}/${UL_ASSET}"

	echo "--- ${UPLOADER_BIN_NAME} ${UPLOADER_TAG} ---"
	download_asset "${UPLOADER_REPO}" "${UPLOADER_TAG}" "${UL_ASSET}" "${UL_TAR}"

	echo "  Extracting ${UPLOADER_BIN_NAME}..."
	UL_EXTRACT="${TMPDIR}/ul"
	mkdir -p "${UL_EXTRACT}"
	tar xzf "${UL_TAR}" -C "${UL_EXTRACT}"

	# The tarball may contain either the old or new binary name
	for candidate in "${UPLOADER_BIN_NAME}" "network-maker" "lumera-uploader"; do
		if [[ -f "${UL_EXTRACT}/${candidate}" ]]; then
			cp -f "${UL_EXTRACT}/${candidate}" "${BIN_DIR}/${UPLOADER_BIN_NAME}"
			chmod +x "${BIN_DIR}/${UPLOADER_BIN_NAME}"
			break
		fi
	done
	[[ -f "${UL_EXTRACT}/config.toml" ]] && cp -f "${UL_EXTRACT}/config.toml" "${BIN_DIR}/uploader-config.toml"
	if [[ -d "${UL_EXTRACT}/ui" ]]; then
		rm -rf "${BIN_DIR}/uploader-ui"
		cp -rf "${UL_EXTRACT}/ui" "${BIN_DIR}/uploader-ui"
	fi
	echo "  -> ${UPLOADER_BIN_NAME} installed"
fi

echo ""
echo "==> Done. Contents of ${BIN_DIR}:"
ls -lh "${BIN_DIR}"
