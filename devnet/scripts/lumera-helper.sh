#!/usr/bin/env bash
set -euo pipefail

SHARED_DIR="${SHARED_DIR:-/shared}"
CFG_DIR="${CFG_DIR:-${SHARED_DIR}/config}"
CFG_CHAIN="${CFG_CHAIN:-${CFG_DIR}/config.json}"
CFG_VALS="${CFG_VALS:-${CFG_DIR}/validators.json}"

require_cmd() {
	local name="$1"
	if ! command -v "${name}" >/dev/null 2>&1; then
		echo "ERROR: required command not found: ${name}" >&2
		exit 1
	fi
}

usage() {
	cat <<'EOF'
Usage:
  lumera-helper.sh new-account N
  lumera-helper.sh list-accounts

Commands:
  new-account N   Create a new keyring account, fund it with N LUME from this
                  node's genesis account, and print the mnemonic + address.
  list-accounts   List generated user accounts as "key: address".
EOF
}

trim() {
	printf '%s' "$1" | tr -d '\r\n'
}

ensure_config() {
	require_cmd jq
	require_cmd curl
	require_cmd awk

	if [ ! -f "${CFG_CHAIN}" ]; then
		echo "ERROR: missing chain config: ${CFG_CHAIN}" >&2
		exit 1
	fi
	if [ ! -f "${CFG_VALS}" ]; then
		echo "ERROR: missing validators config: ${CFG_VALS}" >&2
		exit 1
	fi
}

load_config() {
	ensure_config

	: "${MONIKER:?MONIKER environment variable must be set}"

	CHAIN_ID="$(jq -r '.chain.id' "${CFG_CHAIN}")"
	BASE_DENOM="$(jq -r '.chain.denom.bond' "${CFG_CHAIN}")"
	KEYRING_BACKEND="$(jq -r '.daemon.keyring_backend' "${CFG_CHAIN}")"
	DAEMON="$(jq -r '.daemon.binary' "${CFG_CHAIN}")"
	DAEMON_HOME_BASE="$(jq -r '.paths.base.container' "${CFG_CHAIN}")"
	DAEMON_DIR="$(jq -r '.paths.directories.daemon' "${CFG_CHAIN}")"
	MIN_GAS_PRICE="$(jq -r '.chain.denom.minimum_gas_price // "0.025ulume"' "${CFG_CHAIN}")"

	VAL_REC_JSON="$(jq -c --arg m "${MONIKER}" '[.[] | select(.moniker==$m)][0]' "${CFG_VALS}")"
	if [ -z "${VAL_REC_JSON}" ] || [ "${VAL_REC_JSON}" = "null" ]; then
		echo "ERROR: validator with moniker=${MONIKER} not found in ${CFG_VALS}" >&2
		exit 1
	fi

	FUNDER_KEY_NAME="$(printf '%s' "${VAL_REC_JSON}" | jq -r '.key_name')"
	DAEMON_HOME="${DAEMON_HOME_BASE}/${DAEMON_DIR}"
	GENESIS_LOCAL="${DAEMON_HOME}/config/genesis.json"
	RPC_PORT="${LUMERA_RPC_PORT:-26657}"
	NODE_ADDR="http://127.0.0.1:${RPC_PORT}"
	NODE_STATUS_DIR="${SHARED_DIR}/status/${MONIKER}"
	USER_ACCOUNTS_FILE="${NODE_STATUS_DIR}/user-accounts.json"

	DISPLAY_DENOM="lume"
	DISPLAY_EXPONENT="6"
	if [ -f "${GENESIS_LOCAL}" ]; then
		local metadata
		metadata="$(
			jq -c --arg base "${BASE_DENOM}" '
				.app_state.bank.denom_metadata[]? | select(.base == $base)
			' "${GENESIS_LOCAL}" | head -n1
		)"
		if [ -n "${metadata}" ]; then
			DISPLAY_DENOM="$(printf '%s' "${metadata}" | jq -r '.display // "lume"')"
			DISPLAY_EXPONENT="$(
				printf '%s' "${metadata}" | jq -r --arg display "${DISPLAY_DENOM}" '
					(.denom_units[]? | select(.denom == $display) | .exponent) // "6"
				'
			)"
		fi
	fi

	if [ -z "${CHAIN_ID}" ] || [ -z "${BASE_DENOM}" ] || [ -z "${KEYRING_BACKEND}" ] || [ -z "${DAEMON}" ] || [ -z "${DAEMON_HOME}" ]; then
		echo "ERROR: incomplete devnet configuration" >&2
		exit 1
	fi

	mkdir -p "${NODE_STATUS_DIR}"
}

assert_chain_ready() {
	require_cmd "${DAEMON}"

	if ! curl -sf "${NODE_ADDR}/status" >/dev/null 2>&1; then
		echo "ERROR: lumerad RPC is not reachable at ${NODE_ADDR}" >&2
		exit 1
	fi

	if ! "${DAEMON}" status --node "${NODE_ADDR}" >/dev/null 2>&1; then
		echo "ERROR: ${DAEMON} cannot query node status via ${NODE_ADDR}" >&2
		exit 1
	fi
}

wait_for_tx_confirmation() {
	local txhash="$1"
	local timeout="${2:-90}"
	local out code height deadline

	if [ -z "${txhash}" ]; then
		echo "ERROR: missing tx hash for confirmation" >&2
		exit 1
	fi

	if out="$("${DAEMON}" q wait-tx "${txhash}" --node "${NODE_ADDR}" --output json --timeout "${timeout}s" 2>/dev/null)"; then
		code="$(printf '%s' "${out}" | jq -r 'try .code // "0"')"
		if [ "${code}" = "0" ] || [ "${code}" = "null" ]; then
			return 0
		fi
		echo "ERROR: tx ${txhash} failed during confirmation wait: code=${code}" >&2
		exit 1
	fi

	deadline=$((SECONDS + timeout))
	while ((SECONDS < deadline)); do
		out="$("${DAEMON}" q tx "${txhash}" --node "${NODE_ADDR}" --output json 2>/dev/null || true)"
		if jq -e . >/dev/null 2>&1 <<<"${out}"; then
			code="$(printf '%s' "${out}" | jq -r 'try .code // "0"')"
			height="$(printf '%s' "${out}" | jq -r 'try .height // "0"')"
			if [ "${height}" != "0" ] && [ "${code}" = "0" ]; then
				return 0
			fi
			if [ "${height}" != "0" ] && [ "${code}" != "0" ]; then
				echo "ERROR: tx ${txhash} failed after broadcast: code=${code}" >&2
				exit 1
			fi
		fi
		sleep 3
	done

	echo "ERROR: tx ${txhash} was not confirmed within ${timeout}s" >&2
	exit 1
}

parse_lume_amount_to_base() {
	local amount="$1"
	local exponent="$2"

	if ! [[ "${amount}" =~ ^[0-9]+([.][0-9]+)?$ ]]; then
		echo "ERROR: amount must be a positive number of ${DISPLAY_DENOM}, got: ${amount}" >&2
		exit 1
	fi

	awk -v amount="${amount}" -v exponent="${exponent}" '
		BEGIN {
			split(amount, parts, ".")
			whole = parts[1]
			frac = (length(parts) > 1) ? parts[2] : ""

			if (length(frac) > exponent) {
				printf "ERROR: amount has more than %d decimal places\n", exponent > "/dev/stderr"
				exit 1
			}

			while (length(frac) < exponent) {
				frac = frac "0"
			}

			value = whole frac
			sub(/^0+/, "", value)
			if (value == "") {
				value = "0"
			}

			print value
		}
	'
}

account_name_prefix() {
	local validator_num

	validator_num="$(printf '%s' "${MONIKER}" | grep -oE '[0-9]+$' || true)"
	if [ -z "${validator_num}" ]; then
		validator_num="1"
	fi
	printf 'user-account-val%s' "${validator_num}"
}

next_account_name() {
	local prefix
	local max_id="0"
	local names name suffix

	prefix="$(account_name_prefix)"

	names="$("${DAEMON}" --home "${DAEMON_HOME}" keys list --keyring-backend "${KEYRING_BACKEND}" --output json 2>/dev/null | jq -r '.[].name // empty' || true)"
	while IFS= read -r name; do
		[ -z "${name}" ] && continue
		case "${name}" in
		"${prefix}"-*)
			suffix="${name#${prefix}-}"
			if [[ "${suffix}" =~ ^[0-9]+$ ]] && ((suffix > max_id)); then
				max_id="${suffix}"
			fi
			;;
		esac
	done <<<"${names}"

	printf '%s-%03d' "${prefix}" "$((max_id + 1))"
}

cmd_list_accounts() {
	local prefix names name address

	load_config
	prefix="$(account_name_prefix)"
	names="$("${DAEMON}" --home "${DAEMON_HOME}" keys list --keyring-backend "${KEYRING_BACKEND}" --output json 2>/dev/null | jq -r '.[].name // empty' || true)"

	while IFS= read -r name; do
		[ -z "${name}" ] && continue
		case "${name}" in
		"${prefix}"-*)
			address="$(trim "$("${DAEMON}" --home "${DAEMON_HOME}" keys show "${name}" -a --keyring-backend "${KEYRING_BACKEND}" 2>/dev/null || true)")"
			[ -n "${address}" ] && printf '%s: %s\n' "${name}" "${address}"
			;;
		esac
	done <<<"${names}"
}

key_exists() {
	local key_name="$1"
	"${DAEMON}" --home "${DAEMON_HOME}" keys show "${key_name}" --keyring-backend "${KEYRING_BACKEND}" >/dev/null 2>&1
}

key_pubkey_type() {
	local key_name="$1"
	local out

	if ! out="$("${DAEMON}" --home "${DAEMON_HOME}" keys show "${key_name}" --keyring-backend "${KEYRING_BACKEND}" --output json 2>/dev/null)"; then
		return 1
	fi

	jq -r '
		.pubkey
		| (if type == "string" then (fromjson? // {}) else . end)
		| .["@type"] // empty
	' <<<"${out}"
}

account_type_for_key() {
	local key_name="$1"
	local pubkey_type

	pubkey_type="$(key_pubkey_type "${key_name}" || true)"
	if [[ -n "${pubkey_type}" && "${pubkey_type}" == *"ethsecp256k1"* ]]; then
		printf 'evm'
		return
	fi
	printf 'cosmos'
}

ensure_account_absent() {
	local key_name="$1"

	if key_exists "${key_name}"; then
		echo "Account already exists in keyring: ${key_name}" >&2
		exit 0
	fi

	if [ -f "${USER_ACCOUNTS_FILE}" ] && jq -e --arg name "${key_name}" '.[]? | select(.name == $name)' "${USER_ACCOUNTS_FILE}" >/dev/null 2>&1; then
		echo "Account already exists in ${USER_ACCOUNTS_FILE}: ${key_name}" >&2
		exit 0
	fi
}

create_key() {
	local key_name="$1"
	local key_json

	key_json="$("${DAEMON}" --home "${DAEMON_HOME}" keys add "${key_name}" --keyring-backend "${KEYRING_BACKEND}" --output json)"
	NEW_ACCOUNT_NAME="${key_name}"
	NEW_ACCOUNT_ADDRESS="$(trim "$(printf '%s' "${key_json}" | jq -r '.address // empty')")"
	NEW_ACCOUNT_MNEMONIC="$(printf '%s' "${key_json}" | jq -r '.mnemonic // empty')"

	if [ -z "${NEW_ACCOUNT_ADDRESS}" ] || [ -z "${NEW_ACCOUNT_MNEMONIC}" ]; then
		echo "ERROR: failed to parse new account address/mnemonic from key creation output" >&2
		exit 1
	fi
}

fund_account() {
	local amount_base="$1"
	local recipient="$2"
	local tx_json txhash raw_log code

	tx_json="$("${DAEMON}" tx bank send "${FUNDER_KEY_NAME}" "${recipient}" "${amount_base}${BASE_DENOM}" \
		--home "${DAEMON_HOME}" \
		--chain-id "${CHAIN_ID}" \
		--node "${NODE_ADDR}" \
		--keyring-backend "${KEYRING_BACKEND}" \
		--gas auto \
		--gas-adjustment 1.3 \
		--gas-prices "${MIN_GAS_PRICE}" \
		--broadcast-mode sync \
		--output json \
		--yes)"

	code="$(printf '%s' "${tx_json}" | jq -r '.code // 0')"
	raw_log="$(printf '%s' "${tx_json}" | jq -r '.raw_log // empty')"
	txhash="$(printf '%s' "${tx_json}" | jq -r '.txhash // empty')"

	if [ "${code}" != "0" ]; then
		echo "ERROR: funding tx failed (code=${code}): ${raw_log}" >&2
		exit 1
	fi

	FUNDING_TXHASH="${txhash}"
	wait_for_tx_confirmation "${FUNDING_TXHASH}"
}

query_balance_amount() {
	local address="$1"

	"${DAEMON}" q bank balances "${address}" \
		--node "${NODE_ADDR}" \
		--output json 2>/dev/null | jq -r --arg denom "${BASE_DENOM}" '
			([.balances[]? | select(.denom == $denom) | .amount] | first) // "0"
		'
}

verify_funding() {
	local recipient="$1"
	local expected_amount="$2"
	local actual_amount

	actual_amount="$(query_balance_amount "${recipient}")"
	[[ -z "${actual_amount}" ]] && actual_amount="0"

	if ! [[ "${actual_amount}" =~ ^[0-9]+$ ]]; then
		echo "ERROR: invalid balance response for ${recipient}: ${actual_amount}" >&2
		exit 1
	fi

	if (( actual_amount < expected_amount )); then
		echo "ERROR: funding verification failed for ${recipient}: expected at least ${expected_amount}${BASE_DENOM}, got ${actual_amount}${BASE_DENOM}" >&2
		exit 1
	fi
}

write_user_account_record() {
	local key_name="$1"
	local address="$2"
	local mnemonic="$3"
	local account_type="$4"
	local funded_base="$5"
	local funded_display="$6"
	local txhash="$7"
	local tmp_file

	if [ ! -f "${USER_ACCOUNTS_FILE}" ]; then
		printf '[]\n' >"${USER_ACCOUNTS_FILE}"
		chmod 644 "${USER_ACCOUNTS_FILE}"
	fi

	tmp_file="$(mktemp)"
	jq \
		--arg name "${key_name}" \
		--arg address "${address}" \
		--arg mnemonic "${mnemonic}" \
		--arg type "${account_type}" \
		--arg funded_base "${funded_base}" \
		--arg funded_display "${funded_display}" \
		--arg base_denom "${BASE_DENOM}" \
		--arg display_denom "${DISPLAY_DENOM}" \
		--arg funding_key "${FUNDER_KEY_NAME}" \
		--arg txhash "${txhash}" \
		--arg created_at "$(date -u +"%Y-%m-%dT%H:%M:%SZ")" \
		'
		. + [{
			name: $name,
			address: $address,
			mnemonic: $mnemonic,
			type: $type,
			funded: {
				display_amount: $funded_display,
				display_denom: $display_denom,
				base_amount: $funded_base,
				base_denom: $base_denom
			},
			funding_key: $funding_key,
			funding_txhash: $txhash,
			created_at: $created_at
		}]
		' "${USER_ACCOUNTS_FILE}" >"${tmp_file}"
	chmod 644 "${tmp_file}"
	mv "${tmp_file}" "${USER_ACCOUNTS_FILE}"
	chmod 644 "${USER_ACCOUNTS_FILE}"
}

cmd_new_account() {
	local amount_display="$1"
	local amount_base key_name account_type

	load_config
	assert_chain_ready

	amount_base="$(parse_lume_amount_to_base "${amount_display}" "${DISPLAY_EXPONENT}")"
	if [ "${amount_base}" = "0" ]; then
		echo "ERROR: amount must be greater than zero" >&2
		exit 1
	fi

	key_name="$(next_account_name)"
	ensure_account_absent "${key_name}"
	create_key "${key_name}"
	account_type="$(account_type_for_key "${key_name}")"
	fund_account "${amount_base}" "${NEW_ACCOUNT_ADDRESS}"
	verify_funding "${NEW_ACCOUNT_ADDRESS}" "${amount_base}"
	write_user_account_record "${NEW_ACCOUNT_NAME}" "${NEW_ACCOUNT_ADDRESS}" "${NEW_ACCOUNT_MNEMONIC}" "${account_type}" "${amount_base}" "${amount_display}" "${FUNDING_TXHASH}"

	cat <<EOF
Created account:
  Name: ${NEW_ACCOUNT_NAME}
  Address: ${NEW_ACCOUNT_ADDRESS}
  Type: ${account_type}
  Funded: ${amount_display} ${DISPLAY_DENOM} (${amount_base}${BASE_DENOM})
  Funding key: ${FUNDER_KEY_NAME}
  Tx hash: ${FUNDING_TXHASH}
  Registry: ${USER_ACCOUNTS_FILE}

Mnemonic:
${NEW_ACCOUNT_MNEMONIC}
EOF
}

main() {
	if [ $# -lt 1 ]; then
		usage
		exit 1
	fi

	case "$1" in
	new-account)
		[ $# -eq 2 ] || {
			echo "ERROR: new-account requires exactly one amount argument" >&2
			usage
			exit 1
		}
		cmd_new_account "$2"
		;;
	list-accounts)
		[ $# -eq 1 ] || {
			echo "ERROR: list-accounts does not accept arguments" >&2
			usage
			exit 1
		}
		cmd_list_accounts
		;;
	-h | --help | help)
		usage
		;;
	*)
		echo "ERROR: unknown command: $1" >&2
		usage
		exit 1
		;;
	esac
}

main "$@"
