#!/bin/bash
#
# Shared helpers for devnet bash scripts. Keep this limited to behavior that is
# already duplicated and identical across scripts.

COMMON_SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

if [[ -f "${COMMON_SCRIPT_DIR}/account-registry.sh" ]]; then
	# shellcheck source=/dev/null
	source "${COMMON_SCRIPT_DIR}/account-registry.sh"
fi

run() {
	echo "+ $*"
	"$@"
}

run_capture() {
	echo "+ $*" >&2
	"$@"
}

have() {
	command -v "$1" >/dev/null 2>&1
}

wait_for_file() {
	while [[ ! -s "$1" ]]; do
		sleep 1
	done
}

normalize_version() {
	local version="${1:-}"
	version="${version#"${version%%[![:space:]]*}"}"
	version="${version%"${version##*[![:space:]]}"}"
	version="${version#v}"
	printf '%s\n' "${version}"
}

version_ge() {
	local current floor
	current="$(normalize_version "${1:-}")"
	floor="$(normalize_version "${2:-}")"
	printf '%s\n' "${floor}" "${current}" | sort -V | head -n 1 | grep -q "^${floor}\$"
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
	if [[ -n "${expected_core}" && "${expected}" == "${expected_core}" && "${actual_core}" == "${expected_core}" ]]; then
		return 0
	fi

	return 1
}

get_lumerad_version() {
	local log_prefix="${1:-${VERSION_LOG_PREFIX:-}}"
	local version=""
	local env_version="${LUMERA_VERSION:-}"
	local config_version=""

	version="$(${DAEMON} version 2>/dev/null | grep -Eo 'v?[0-9]+\.[0-9]+\.[0-9]+([-+][0-9A-Za-z.-]+)?' | head -n1 || true)"
	version="$(normalize_version "${version}")"
	env_version="$(normalize_version "${env_version}")"
	if [[ -n "${version}" ]]; then
		if [[ -n "${env_version}" && "${env_version}" != "null" && "${env_version}" != "${version}" && -n "${log_prefix}" ]]; then
			echo "${log_prefix} Ignoring stale LUMERA_VERSION=v${env_version}; detected lumerad binary version ${version}." >&2
		fi
		printf '%s' "${version}"
		return 0
	fi

	if [[ -n "${env_version}" && "${env_version}" != "null" ]]; then
		printf '%s' "${env_version}"
		return 0
	fi

	if [[ -n "${CFG_CHAIN:-}" && -f "${CFG_CHAIN}" ]]; then
		config_version="$(jq -r '.chain.version // empty' "${CFG_CHAIN}" 2>/dev/null || true)"
	fi
	config_version="$(normalize_version "${config_version}")"

	if [[ -n "${config_version}" && "${config_version}" != "null" ]]; then
		printf '%s' "${config_version}"
		return 0
	fi

	printf '%s' "${version}"
}

get_first_evm_version() {
	local version=""

	if [[ -n "${LUMERA_FIRST_EVM_VERSION:-}" && "${LUMERA_FIRST_EVM_VERSION}" != "null" ]]; then
		version="${LUMERA_FIRST_EVM_VERSION}"
	elif [[ -n "${CFG_CHAIN:-}" && -f "${CFG_CHAIN}" ]]; then
		version="$(jq -r '.chain.evm_from_version // empty' "${CFG_CHAIN}" 2>/dev/null || true)"
	fi

	if [[ -z "${version}" || "${version}" == "null" ]]; then
		version="v1.20.0"
	fi

	printf '%s' "$(normalize_version "${version}")"
}

lumera_supports_evm() {
	local current_version first_evm_version log_prefix verbose
	log_prefix="${LUMERA_SUPPORTS_EVM_LOG_PREFIX:-${VERSION_LOG_PREFIX:-}}"
	verbose="${LUMERA_SUPPORTS_EVM_VERBOSE:-0}"

	current_version="$(get_lumerad_version)"
	first_evm_version="$(get_first_evm_version)"

	if [[ -z "${current_version}" || "${current_version}" == "null" ]]; then
		if [[ "${verbose}" == "1" && -n "${log_prefix}" ]]; then
			echo "${log_prefix} Unable to determine lumerad version; assuming no EVM migration support." >&2
		fi
		return 1
	fi

	if version_ge "${current_version}" "${first_evm_version}"; then
		if [[ "${verbose}" == "1" && -n "${log_prefix}" ]]; then
			echo "${log_prefix} Lumera version v${current_version} has EVM support (cutover v${first_evm_version})." >&2
		fi
		return 0
	fi

	if [[ "${verbose}" == "1" && -n "${log_prefix}" ]]; then
		echo "${log_prefix} Lumera version v${current_version} is pre-EVM (cutover v${first_evm_version}); skipping EVM key migration setup." >&2
	fi
	return 1
}

# ─── Uploader Binary Name Resolution ─────────────────────────────────────────
# Starting from Lumera v1.11.0 the "network-maker" project was renamed to
# "lumera-uploader". Scripts that need the binary/log/home names should call
# resolve_uploader_name() to get the correct value for the running version.
#
# The threshold version can be overridden via LUMERA_FIRST_UPLOADER_VERSION.
LUMERA_FIRST_UPLOADER_VERSION="${LUMERA_FIRST_UPLOADER_VERSION:-1.11.0}"

# resolve_uploader_name [version]
#   Prints "lumera-uploader" if the given (or detected) lumerad version >= 1.11.0,
#   otherwise "network-maker".
resolve_uploader_name() {
	local ver="${1:-}"
	if [[ -z "${ver}" ]]; then
		ver="$(get_lumerad_version 2>/dev/null || true)"
	fi
	ver="$(normalize_version "${ver}")"
	if [[ -n "${ver}" ]] && version_ge "${ver}" "${LUMERA_FIRST_UPLOADER_VERSION}"; then
		printf "lumera-uploader"
	else
		printf "network-maker"
	fi
}


wait_for_tx() {
	local txhash="$1"
	local timeout="${2:-90}"
	local interval="${3:-3}"
	local log_prefix="${TX_WAIT_LOG_PREFIX:-${VERSION_LOG_PREFIX:-[TX]}}"

	if [[ -z "${txhash}" ]]; then
		echo "${log_prefix} wait_for_tx: missing tx hash" >&2
		return 2
	fi

	echo "${log_prefix} Waiting for tx ${txhash} (up to ${timeout}s) via WebSocket…"
	local wait_args=("${DAEMON}" q wait-tx "${txhash}" --output json --timeout "${timeout}s")
	[[ -n "${LUMERA_RPC_ADDR:-}" ]] && wait_args+=(--node "${LUMERA_RPC_ADDR}")

	local out rc=0
	out="$("${wait_args[@]}" 2>&1)"
	rc=$?
	if [[ ${rc} -eq 0 ]] && jq -e . >/dev/null 2>&1 <<<"${out}"; then
		local code height gas_used gas_wanted raw_log ts
		code="$(jq -r 'try .code // "null"' <<<"${out}")"
		height="$(jq -r 'try .height // "0"' <<<"${out}")"
		gas_used="$(jq -r 'try .gas_used // ""' <<<"${out}")"
		gas_wanted="$(jq -r 'try .gas_wanted // ""' <<<"${out}")"
		raw_log="$(jq -r 'try .raw_log // ""' <<<"${out}")"
		ts="$(jq -r 'try .timestamp // ""' <<<"${out}")"

		if [[ "${code}" == "0" || "${code}" == "null" ]]; then
			echo "${log_prefix} Tx ${txhash} confirmed at height ${height} (gas ${gas_used}/${gas_wanted}) ${ts}"
			return 0
		fi

		echo "${log_prefix} Tx ${txhash} FAILED at height ${height}: code=${code}" >&2
		[[ -n "${raw_log}" ]] && echo "${log_prefix} raw_log: ${raw_log}" >&2
		return 1
	fi

	echo "${log_prefix} WebSocket wait failed/timeout; falling back to RPC polling…"

	local deadline=$((SECONDS + timeout))
	while ((SECONDS < deadline)); do
		local tx_args=("${DAEMON}" q tx "${txhash}" --output json)
		[[ -n "${LUMERA_RPC_ADDR:-}" ]] && tx_args+=(--node "${LUMERA_RPC_ADDR}")

		out="$("${tx_args[@]}" 2>&1)" || true
		if jq -e . >/dev/null 2>&1 <<<"${out}"; then
			local height code codespace raw_log gas_used gas_wanted
			height="$(jq -r 'try .height // "0"' <<<"${out}")"
			code="$(jq -r 'try .code // "null"' <<<"${out}")"
			codespace="$(jq -r 'try .codespace // ""' <<<"${out}")"
			raw_log="$(jq -r 'try .raw_log // ""' <<<"${out}")"
			gas_used="$(jq -r 'try .gas_used // ""' <<<"${out}")"
			gas_wanted="$(jq -r 'try .gas_wanted // ""' <<<"${out}")"

			if [[ "${height}" != "0" && "${height}" != "null" ]]; then
				if [[ "${code}" == "0" || "${code}" == "null" ]]; then
					echo "${log_prefix} Tx ${txhash} confirmed at height ${height} (gas ${gas_used}/${gas_wanted})"
					return 0
				fi

				echo "${log_prefix} Tx ${txhash} FAILED at height ${height}: code=${code} codespace=${codespace:-N/A}" >&2
				[[ -n "${raw_log}" ]] && echo "${log_prefix} raw_log: ${raw_log}" >&2
				return 1
			fi
		fi

		sleep "${interval}"
	done

	echo "${log_prefix} Timeout: tx ${txhash} not found/committed after ${timeout}s." >&2
	echo "${log_prefix} Hints: ensure RPC reachable (check \$LUMERA_RPC_ADDR), and node is not lagging." >&2
	return 2
}

recover_key_from_mnemonic() {
	local key_name="$1"
	local mnemonic="$2"

	run "${DAEMON}" keys delete "${key_name}" --keyring-backend "${KEYRING_BACKEND}" -y >/dev/null 2>&1 || true
	printf '%s\n' "${mnemonic}" | run "${DAEMON}" keys add "${key_name}" --recover --keyring-backend "${KEYRING_BACKEND}" >/dev/null
}

# multisig_sign_unsigned collects 2-of-N threshold signatures for an unsigned
# cosmos tx JSON and writes a fully-signed multisig tx JSON to stdout.
# Callers redirect stdout to a file for subsequent `tx broadcast`.
#
# Positional args:
#   $1 unsigned_file  input path produced by `tx <cmd> --generate-only`
#   $2 multisig_key   keyring name of the multisig composite key
#   $3 multisig_addr  bech32 address of the multisig account
#   $4 signer1        keyring name of first sub-key to sign with
#   $5 signer2        keyring name of second sub-key to sign with
#   $6 account_num    multisig account's auth account_number
#   $7 sequence       multisig account's current sequence
#
# Uses ${DAEMON}, ${KEYRING_BACKEND}, ${CHAIN_ID} from the sourcing script's
# environment. Aborts (via set -e) if any step fails. Temp sig files are
# cleaned up before returning.
multisig_sign_unsigned() {
	local unsigned_file="$1"
	local multisig_key="$2"
	local multisig_addr="$3"
	local signer1="$4"
	local signer2="$5"
	local acc_num="$6"
	local seq="$7"
	local sig1 sig2 rc
	sig1="$(mktemp /tmp/multisig-sig1.XXXXXX.json)"
	sig2="$(mktemp /tmp/multisig-sig2.XXXXXX.json)"

	# --offline is required so `tx sign` trusts the caller-supplied
	# --account-number and --sequence instead of reaching out to the chain
	# (which may not be up yet during the gentx ceremony). Per SDK docs,
	# without --offline those two flags are silently ignored and overwritten
	# with values fetched from a full node.
	rc=0
	{
		run_capture ${DAEMON} tx sign "${unsigned_file}" \
			--from "${signer1}" \
			--multisig "${multisig_addr}" \
			--keyring-backend "${KEYRING_BACKEND}" \
			--chain-id "${CHAIN_ID}" \
			--account-number "${acc_num}" --sequence "${seq}" \
			--sign-mode amino-json --offline \
			--output json >"${sig1}" &&
		run_capture ${DAEMON} tx sign "${unsigned_file}" \
			--from "${signer2}" \
			--multisig "${multisig_addr}" \
			--keyring-backend "${KEYRING_BACKEND}" \
			--chain-id "${CHAIN_ID}" \
			--account-number "${acc_num}" --sequence "${seq}" \
			--sign-mode amino-json --offline \
			--output json >"${sig2}" &&
		run_capture ${DAEMON} tx multisign "${unsigned_file}" "${multisig_key}" \
			"${sig1}" "${sig2}" \
			--keyring-backend "${KEYRING_BACKEND}" \
			--chain-id "${CHAIN_ID}" \
			--offline --account-number "${acc_num}" --sequence "${seq}" \
			--output json
	} || rc=$?

	rm -f "${sig1}" "${sig2}"
	return "${rc}"
}
