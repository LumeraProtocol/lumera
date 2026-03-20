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
COMPOSE_STOP_TIMEOUT="${COMPOSE_STOP_TIMEOUT:-30}"
COMPOSE_UP_TIMEOUT="${COMPOSE_UP_TIMEOUT:-120}"
COMPOSE_READY_TIMEOUT="${COMPOSE_READY_TIMEOUT:-90}"

normalize_version() {
	local version="${1:-}"
	version="${version#"${version%%[![:space:]]*}"}"
	version="${version%"${version##*[![:space:]]}"}"
	version="${version#v}"
	printf '%s\n' "${version}"
}

release_core_version() {
	local version
	version="$(normalize_version "${1:-}")"
	printf '%s\n' "${version}" | grep -Eo '^[0-9]+\.[0-9]+\.[0-9]+' | head -n 1
}

versions_match() {
	local expected actual expected_core actual_core
	expected="$(normalize_version "${1:-}")"
	actual="$(normalize_version "${2:-}")"

	if [[ -z "${expected}" || -z "${actual}" ]]; then
		return 1
	fi

	if [[ "${expected}" == "${actual}" ]]; then
		return 0
	fi

	expected_core="$(release_core_version "${expected}")"
	actual_core="$(release_core_version "${actual}")"

	# Accept local/dev builds like 1.12.0-<gitsha> when the requested
	# upgrade target is the stable release 1.12.0.
	if [[ -n "${expected_core}" && "${expected}" == "${expected_core}" && "${actual_core}" == "${expected_core}" ]]; then
		return 0
	fi

	return 1
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

compose_services() {
	docker compose -f "${COMPOSE_FILE}" config --services
}

running_services() {
	docker compose -f "${COMPOSE_FILE}" ps --status running --services 2>/dev/null || true
}

all_services_running() {
	local expected running

	expected="$(compose_services | sort)"
	running="$(running_services | sort)"

	[[ -n "${expected}" && "${expected}" == "${running}" ]]
}

wait_for_all_services_running() {
	local deadline
	deadline=$((SECONDS + COMPOSE_READY_TIMEOUT))

	while ((SECONDS < deadline)); do
		if all_services_running; then
			return 0
		fi
		sleep 2
	done

	return 1
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
if ! versions_match "${EXPECTED_VERSION}" "${SOURCE_VERSION}"; then
	echo "Source lumerad version mismatch: expected ${EXPECTED_RELEASE_NAME}, got ${SOURCE_VERSION} from ${SOURCE_LUMERAD}" >&2
	exit 1
fi

echo "Verified source lumerad version ${SOURCE_VERSION} at ${SOURCE_LUMERAD}"

echo "Stopping devnet containers..."
docker compose -f "${COMPOSE_FILE}" stop -t "${COMPOSE_STOP_TIMEOUT}"

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
if ! versions_match "${EXPECTED_VERSION}" "${SHARED_VERSION}"; then
	echo "Shared lumerad version mismatch after copy: expected ${EXPECTED_RELEASE_NAME}, got ${SHARED_VERSION} from ${SHARED_LUMERAD}" >&2
	exit 1
fi

echo "Verified shared lumerad version ${SHARED_VERSION} at ${SHARED_LUMERAD}"

echo "Restarting devnet containers..."
if ! timeout "${COMPOSE_UP_TIMEOUT}" env START_MODE=run docker compose -f "${COMPOSE_FILE}" up -d --no-build; then
	echo "docker compose up -d did not complete within ${COMPOSE_UP_TIMEOUT}; checking container state..." >&2
	if all_services_running; then
		echo "All devnet services are running despite compose timeout; continuing."
	else
		echo "Timed out restarting devnet containers and not all services are running." >&2
		docker compose -f "${COMPOSE_FILE}" ps >&2 || true
		exit 1
	fi
fi

echo "Waiting for all devnet services to report running status..."
if ! wait_for_all_services_running; then
	echo "Timed out waiting for all devnet services to reach running state after restart." >&2
	docker compose -f "${COMPOSE_FILE}" ps >&2 || true
	exit 1
fi

docker compose -f "${COMPOSE_FILE}" ps

echo "Binaries upgrade complete using ${BINARIES_DIR}."
