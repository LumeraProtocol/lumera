#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 2 ]]; then
	echo "Usage: $0 <binaries-dir> <expected-release-name>" >&2
	exit 1
fi

BINARIES_DIR="$1"
EXPECTED_RELEASE_NAME="$2"
if [[ ! -d "${BINARIES_DIR}" ]]; then
	echo "Binaries directory not found: ${BINARIES_DIR}" >&2
	exit 1
fi
BINARIES_DIR="$(cd "${BINARIES_DIR}" && pwd)"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEVNET_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
COMPOSE_FILE="${DEVNET_ROOT}/docker-compose.yml"

if [[ ! -f "${COMPOSE_FILE}" ]]; then
	echo "docker-compose.yml not found at ${COMPOSE_FILE}" >&2
	exit 1
fi

DEVNET_RUNTIME_DIR="${DEVNET_DIR:-/tmp/lumera-devnet-1}"
RELEASE_DIR="${DEVNET_RUNTIME_DIR}/shared/release"
SOURCE_LUMERAD="${BINARIES_DIR}/lumerad"
SHARED_LUMERAD="${RELEASE_DIR}/lumerad"

normalize_version() {
	local version="${1:-}"
	version="${version#"${version%%[![:space:]]*}"}"
	version="${version%"${version##*[![:space:]]}"}"
	version="${version#v}"
	printf '%s\n' "${version}"
}

binary_version() {
	local binary="$1"
	local version

	if [[ ! -x "${binary}" ]]; then
		echo "Binary is not executable: ${binary}" >&2
		return 1
	fi

	version="$("${binary}" version 2>/dev/null | head -n 1 | tr -d '\r')"
	version="$(normalize_version "${version}")"
	if [[ -z "${version}" ]]; then
		echo "Failed to determine version for binary: ${binary}" >&2
		return 1
	fi
	printf '%s\n' "${version}"
}

EXPECTED_VERSION="$(normalize_version "${EXPECTED_RELEASE_NAME}")"
if [[ -z "${EXPECTED_VERSION}" ]]; then
	echo "Expected release name is empty or invalid: ${EXPECTED_RELEASE_NAME}" >&2
	exit 1
fi

if [[ ! -f "${SOURCE_LUMERAD}" ]]; then
	echo "Source lumerad binary not found: ${SOURCE_LUMERAD}" >&2
	exit 1
fi

SOURCE_VERSION="$(binary_version "${SOURCE_LUMERAD}")"
if [[ "${SOURCE_VERSION}" != "${EXPECTED_VERSION}" ]]; then
	echo "Source lumerad version mismatch: expected ${EXPECTED_RELEASE_NAME}, got ${SOURCE_VERSION} from ${SOURCE_LUMERAD}" >&2
	exit 1
fi

echo "Verified source lumerad version ${SOURCE_VERSION} at ${SOURCE_LUMERAD}"

echo "Stopping devnet containers..."
docker compose -f "${COMPOSE_FILE}" stop

echo "Copying binaries from ${BINARIES_DIR} to ${RELEASE_DIR}..."
mkdir -p "${RELEASE_DIR}"
shopt -s nullglob
copied=0
for file in "${BINARIES_DIR}"/*; do
	if [[ -f "${file}" ]]; then
		cp -Sf "${file}" "${RELEASE_DIR}/"
		copied=1
	fi
done
shopt -u nullglob

if [[ "${copied}" -eq 0 ]]; then
	echo "No files were copied from ${BINARIES_DIR}" >&2
	exit 1
fi

if [[ -f "${RELEASE_DIR}/lumerad" ]]; then
	chmod +x "${RELEASE_DIR}/lumerad"
fi

if [[ ! -f "${SHARED_LUMERAD}" ]]; then
	echo "Copied shared lumerad binary not found: ${SHARED_LUMERAD}" >&2
	exit 1
fi

SHARED_VERSION="$(binary_version "${SHARED_LUMERAD}")"
if [[ "${SHARED_VERSION}" != "${EXPECTED_VERSION}" ]]; then
	echo "Shared lumerad version mismatch after copy: expected ${EXPECTED_RELEASE_NAME}, got ${SHARED_VERSION} from ${SHARED_LUMERAD}" >&2
	exit 1
fi

echo "Verified shared lumerad version ${SHARED_VERSION} at ${SHARED_LUMERAD}"

echo "Restarting devnet containers..."
START_MODE=run docker compose -f "${COMPOSE_FILE}" up -d

echo "Binaries upgrade complete using ${BINARIES_DIR}."
