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
MINIMUM_GAS_PRICES="${MINIMUM_GAS_PRICES:-${SIMD_MINIMUM_GAS_PRICES:-0.0025${DENOM}}}"
ACCOUNT_BALANCE="${ACCOUNT_BALANCE:-${SIMD_GENESIS_BALANCE:-100000000000${DENOM}}}"
STAKING_AMOUNT="${STAKING_AMOUNT:-${SIMD_STAKE_AMOUNT:-50000000000${DENOM}}}"
TEST_KEY_NAME="${TEST_KEY_NAME:-${SIMD_TEST_KEY_NAME:-simd-test}}"
TEST_ACCOUNT_BALANCE="${TEST_ACCOUNT_BALANCE:-${SIMD_TEST_ACCOUNT_BALANCE:-100000000${DENOM}}}"
SIMAPP_SHARED_DIR="${SIMAPP_SHARED_DIR:-/shared}"

GENESIS_FILE="${SIMAPP_HOME}/config/genesis.json"

prefixed_name() {
	local prefix="$1"
	local name="$2"
	case "${name}" in
	"${prefix}"*) printf '%s' "${name}" ;;
	*) printf '%s%s' "${prefix}" "${name}" ;;
	esac
}

wait_for_file() {
	local file="$1"
	local timeout="${2:-60}"
	local elapsed=0
	while [ ! -s "${file}" ]; do
		if [ "${elapsed}" -ge "${timeout}" ]; then
			return 1
		fi
		sleep 1
		elapsed=$((elapsed + 1))
	done
	return 0
}

key_mnemonic_file() {
	local name="$1"
	name="$(prefixed_name "simd-" "${name}")"
	printf '%s/hermes/%s.mnemonic' "${SIMAPP_SHARED_DIR}" "${name}"
}

key_address_file() {
	local name="$1"
	name="$(prefixed_name "simd-" "${name}")"
	printf '%s/hermes/%s.address' "${SIMAPP_SHARED_DIR}" "${name}"
}

record_key_mnemonic() {
	local name="$1"
	local mnemonic="$2"
	[ -z "${mnemonic}" ] && return 0
	local file
	file="$(key_mnemonic_file "${name}")"
	mkdir -p "$(dirname "${file}")"
	printf '%s\n' "${mnemonic}" >"${file}"
}

record_key_address() {
	local name="$1"
	local addr file
	addr="$("${SIMD_BIN}" --home "${SIMAPP_HOME}" keys show "${name}" -a --keyring-backend "${KEYRING}" 2>/dev/null || true)"
	addr="$(printf '%s' "${addr}" | tr -d '\r\n')"
	if [ -n "${addr}" ]; then
		log "Recorded ${name} address: ${addr}"
		file="$(key_address_file "${name}")"
		mkdir -p "$(dirname "${file}")"
		printf '%s\n' "${addr}" >"${file}"
	else
		log "Failed to resolve address for key ${name}"
	fi
}

LUMERA_RELAYER_MNEMONIC_FILE="${SIMAPP_SHARED_DIR}/hermes/lumera-hermes-relayer.mnemonic"

record_relayer_mnemonic() {
	local mnemonic="$1"
	[ -z "${mnemonic}" ] && return 0
	log "Recording relayer mnemonic for key ${RELAYER_KEY_NAME}"
	record_key_mnemonic "${RELAYER_KEY_NAME}" "${mnemonic}"
}

if [ -s "${LUMERA_RELAYER_MNEMONIC_FILE}" ]; then
	if [ -z "${SIMAPP_KEY_RELAYER_MNEMONIC:-}" ]; then
		log "Using relayer mnemonic from ${LUMERA_RELAYER_MNEMONIC_FILE}"
		SIMAPP_KEY_RELAYER_MNEMONIC="$(cat "${LUMERA_RELAYER_MNEMONIC_FILE}")"
		export SIMAPP_KEY_RELAYER_MNEMONIC
	else
		existing_relayer_mnemonic="$(cat "${LUMERA_RELAYER_MNEMONIC_FILE}")"
		if [ "${SIMAPP_KEY_RELAYER_MNEMONIC}" != "${existing_relayer_mnemonic}" ]; then
			SIMAPP_KEY_RELAYER_MNEMONIC="${existing_relayer_mnemonic}"
			export SIMAPP_KEY_RELAYER_MNEMONIC
		fi
	fi
fi

already_init=0
if [ -f "${GENESIS_FILE}" ]; then
	already_init=1
	if [ -n "${SIMAPP_KEY_RELAYER_MNEMONIC:-}" ]; then
		record_relayer_mnemonic "${SIMAPP_KEY_RELAYER_MNEMONIC}"
	fi
	log "simd already initialised at ${GENESIS_FILE}"
else
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
fi

run "${SIMD_BIN}" --home "${SIMAPP_HOME}" config set client chain-id "${CHAIN_ID}"
run "${SIMD_BIN}" --home "${SIMAPP_HOME}" config set client keyring-backend "${KEYRING}"
run "${SIMD_BIN}" --home "${SIMAPP_HOME}" config set app api.enable true || true
run "${SIMD_BIN}" --home "${SIMAPP_HOME}" config set app minimum-gas-prices "${MINIMUM_GAS_PRICES}"

if [ "${already_init}" -eq 1 ]; then
	record_key_address "${KEY_NAME}"
	if [ "${RELAYER_KEY_NAME}" != "${KEY_NAME}" ]; then
		record_key_address "${RELAYER_KEY_NAME}"
	fi
	if [ -n "${TEST_KEY_NAME}" ]; then
		record_key_address "${TEST_KEY_NAME}"
	fi
	exit 0
fi

ensure_key() {
	local name="$1"
	local upper var mnemonic json mnemonic_file
	upper=$(echo "${name}" | tr '[:lower:]-' '[:upper:]_')
	var="SIMAPP_KEY_${upper}_MNEMONIC"
	mnemonic="${!var:-}"
	mnemonic_file="$(key_mnemonic_file "${name}")"

	if [ "${name}" = "${RELAYER_KEY_NAME}" ] && [ -n "${LUMERA_RELAYER_MNEMONIC_FILE:-}" ]; then
		log "Waiting for Lumera relayer mnemonic ${LUMERA_RELAYER_MNEMONIC_FILE}"
		if wait_for_file "${LUMERA_RELAYER_MNEMONIC_FILE}" "${SIMAPP_RELAYER_MNEMONIC_WAIT_SECONDS:-60}"; then
			mnemonic="$(cat "${LUMERA_RELAYER_MNEMONIC_FILE}")"
			printf -v "${var}" '%s' "${mnemonic}"
			export "${var}"
		else
			log "Lumera relayer mnemonic ${LUMERA_RELAYER_MNEMONIC_FILE} not available; proceeding without it"
		fi
	fi

	if [ -z "${mnemonic}" ] && [ -s "${mnemonic_file}" ]; then
		log "Loading mnemonic for key ${name} from ${mnemonic_file}"
		mnemonic="$(cat "${mnemonic_file}")"
		printf -v "${var}" '%s' "${mnemonic}"
		export "${var}"
	fi

	if "${SIMD_BIN}" --home "${SIMAPP_HOME}" keys show "${name}" --keyring-backend "${KEYRING}" >/dev/null 2>&1; then
		log "Key ${name} already exists in keyring"
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
	record_key_address "${name}"

	if [ "${name}" = "${RELAYER_KEY_NAME}" ]; then
		record_relayer_mnemonic "${mnemonic}"
	fi
}

ensure_key "${KEY_NAME}"
if [ "${RELAYER_KEY_NAME}" != "${KEY_NAME}" ]; then
	ensure_key "${RELAYER_KEY_NAME}"
fi
if [ -n "${TEST_KEY_NAME}" ]; then
	ensure_key "${TEST_KEY_NAME}"
fi

run "${SIMD_BIN}" --home "${SIMAPP_HOME}" genesis add-genesis-account "${KEY_NAME}" "${ACCOUNT_BALANCE}" --keyring-backend "${KEYRING}"
if [ "${RELAYER_KEY_NAME}" != "${KEY_NAME}" ]; then
	run "${SIMD_BIN}" --home "${SIMAPP_HOME}" genesis add-genesis-account "${RELAYER_KEY_NAME}" "${ACCOUNT_BALANCE}" --keyring-backend "${KEYRING}"
fi
if [ -n "${TEST_KEY_NAME}" ]; then
	run "${SIMD_BIN}" --home "${SIMAPP_HOME}" genesis add-genesis-account "${TEST_KEY_NAME}" "${TEST_ACCOUNT_BALANCE}" --keyring-backend "${KEYRING}"
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
