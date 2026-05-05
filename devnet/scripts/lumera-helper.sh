#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=/dev/null
source "${SCRIPT_DIR}/common.sh"

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
  lumera-helper.sh new-account [--multisig] AMOUNT
  lumera-helper.sh list-accounts
  lumera-helper.sh generate-evm-accounts [--count N]

Commands:
  new-account [--multisig] AMOUNT
                           Create a new keyring account, fund it from this node's
                           genesis account, and print the mnemonic + address.
                           With --multisig, create a 2-of-3 multisig account
                           plus three signer keys, fund the multisig address,
                           and record signer metadata in user-accounts.json.
                           AMOUNT can be either:
                             - a bare number, interpreted as display units (LUME)
                               e.g. "5" → 5 LUME
                             - a number with the chain base denom suffix
                               e.g. "10000ulume" → 10000ulume (base units)
  list-accounts            List generated user accounts as "key: address".
  generate-evm-accounts [--count N]
                           For every selected cosmos account in user-accounts.json,
                           import the same mnemonic under coin-type 60 /
                           eth_secp256k1 as "user-account-evm-val<N>-<NNN>",
                           rewrite the JSON entry so "address" holds the new
                           EVM address, the legacy bech32 moves to
                           "legacy_address", "name" tracks the new key, and
                           "type" becomes "evm". Supports multisig entries by
                           deriving EVM keys for each signer and creating a
                           new 2-of-3 EVM multisig composite. Requires an
                           EVM-migrated chain. --count limits processing to
                           the first N user-accounts.json entries.
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

	# Honor a caller-supplied FUNDER_KEY_NAME (e.g. test-accounts-setup routes
	# funding through a dedicated temp key to avoid sequence races on the
	# shared validator genesis key). Default to the validator's own key.
	if [ -z "${FUNDER_KEY_NAME:-}" ]; then
		FUNDER_KEY_NAME="$(printf '%s' "${VAL_REC_JSON}" | jq -r '.key_name')"
	fi
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
	local out code height deadline raw_log codespace

	if [ -z "${txhash}" ]; then
		echo "ERROR: missing tx hash for confirmation" >&2
		exit 1
	fi

	if out="$("${DAEMON}" q wait-tx "${txhash}" --node "${NODE_ADDR}" --output json --timeout "${timeout}s" 2>/dev/null)"; then
		code="$(printf '%s' "${out}" | jq -r 'try .code // "0"')"
		if [ "${code}" = "0" ] || [ "${code}" = "null" ]; then
			return 0
		fi
		raw_log="$(printf '%s' "${out}" | jq -r 'try .raw_log // try .log // empty')"
		codespace="$(printf '%s' "${out}" | jq -r 'try .codespace // empty')"
		echo "ERROR: tx ${txhash} failed during confirmation wait: code=${code} codespace=${codespace:-N/A}" >&2
		[ -n "${raw_log}" ] && echo "ERROR: raw_log: ${raw_log}" >&2
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
				raw_log="$(printf '%s' "${out}" | jq -r 'try .raw_log // try .log // empty')"
				codespace="$(printf '%s' "${out}" | jq -r 'try .codespace // empty')"
				echo "ERROR: tx ${txhash} failed after broadcast: code=${code} codespace=${codespace:-N/A}" >&2
				[ -n "${raw_log}" ] && echo "ERROR: raw_log: ${raw_log}" >&2
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
			suffix="${name#"${prefix}"-}"
			# Force base-10 parsing: suffixes like "008"/"009" would otherwise be
			# interpreted as octal inside (( )) and fail with "value too great for base".
			if [[ "${suffix}" =~ ^[0-9]+$ ]] && ((10#${suffix} > 10#${max_id})); then
				max_id="${suffix}"
			fi
			;;
		esac
	done <<<"${names}"

	printf '%s-%03d' "${prefix}" "$((10#${max_id} + 1))"
}

cmd_list_accounts() {
	local accounts_count idx
	local name address acct_type legacy_address legacy_key

	load_config

	# Source of truth is user-accounts.json (per-validator, written by
	# new-account / generate-evm-accounts). Iterating the keyring directly
	# would lose the legacy↔EVM pairing recorded only in JSON.
	if [ ! -f "${USER_ACCOUNTS_FILE}" ]; then
		return 0
	fi

	accounts_count="$(jq 'length' "${USER_ACCOUNTS_FILE}" 2>/dev/null || echo 0)"
	if [ -z "${accounts_count}" ] || [ "${accounts_count}" = "0" ]; then
		return 0
	fi

	for ((idx = 0; idx < accounts_count; idx++)); do
		name="$(jq -r --argjson i "${idx}" '.[$i].name // empty' "${USER_ACCOUNTS_FILE}")"
		address="$(jq -r --argjson i "${idx}" '.[$i].address // empty' "${USER_ACCOUNTS_FILE}")"
		acct_type="$(jq -r --argjson i "${idx}" '.[$i].type // "cosmos"' "${USER_ACCOUNTS_FILE}")"
		legacy_address="$(jq -r --argjson i "${idx}" '.[$i].legacy_address // empty' "${USER_ACCOUNTS_FILE}")"
		legacy_key="$(jq -r --argjson i "${idx}" '.[$i].legacy_key // empty' "${USER_ACCOUNTS_FILE}")"

		if [ -z "${name}" ] || [ -z "${address}" ]; then
			continue
		fi

		# Migrated entry: print a paired view so the user can see both
		# the original cosmos key/address and the post-migration EVM
		# key/address that share the same mnemonic.
		if [ -n "${legacy_address}" ] && [ -n "${legacy_key}" ]; then
			printf '%s(cosmos): %s\n' "${legacy_key}" "${legacy_address}"
			printf '%s(evm): %s\n' "${name}" "${address}"
			continue
		fi

		printf '%s(%s): %s\n' "${name}" "${acct_type}" "${address}"
	done
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
		--gas-adjustment 2.0 \
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

query_account_number_sequence() {
	local address="$1"
	local out

	out="$("${DAEMON}" q auth account "${address}" --node "${NODE_ADDR}" --output json 2>/dev/null || true)"
	jq -r '
		.. | objects
		| select(has("account_number"))
		| "\(.account_number)\t\(.sequence // "0")"
	' <<<"${out}" | head -n1
}

verify_multisig_pubkey_seeded() {
	local address="$1"
	local out pubkey_type deadline

	deadline=$((SECONDS + 20))
	while :; do
		out="$("${DAEMON}" q auth account "${address}" --node "${NODE_ADDR}" --output json 2>/dev/null || true)"
		pubkey_type="$(jq -r '
			[
				.account.pub_key?,
				.account.base_account.pub_key?,
				.account.base_vesting_account.base_account.pub_key?,
				.account.value.public_key?,
				(.account? | objects | select(has("pubkey")) | .pubkey),
				(.. | objects | select(has("pub_key") or has("pubkey") or has("public_key")) | (.pub_key // .pubkey // .public_key))
			]
			| map(select(. != null))
			| (.[0] // null)
			| if . == null then empty
			  else
				(if type == "string" then (fromjson? // {}) else . end)
				| (."@type" // .type_url // .type // empty)
			  end
		' <<<"${out}" | head -n1)"

		if [ "${pubkey_type}" = "/cosmos.crypto.multisig.LegacyAminoPubKey" ]; then
			return 0
		fi
		if ((SECONDS >= deadline)); then
			break
		fi
		sleep 2
	done

	echo "ERROR: multisig pubkey was not seeded on-chain for ${address}" >&2
	echo "ERROR: expected /cosmos.crypto.multisig.LegacyAminoPubKey, got: ${pubkey_type:-none}" >&2
	if jq -e . >/dev/null 2>&1 <<<"${out}"; then
		echo "ERROR: auth account response:" >&2
		jq -c . <<<"${out}" >&2 || true
	elif [ -n "${out}" ]; then
		echo "ERROR: auth account response was not JSON: ${out}" >&2
	else
		echo "ERROR: auth account query returned no output" >&2
	fi
	exit 1
}

signed_tx_pubkey_type() {
	local signed_file="$1"

	jq -r '
		[
			.auth_info.signer_infos[0].public_key?,
			.auth_info.signer_infos[0].publicKey?
		]
		| map(select(. != null))
		| (.[0] // null)
		| if . == null then empty
		  else
			(if type == "string" then (fromjson? // {}) else . end)
			| (."@type" // .type_url // empty)
		  end
	' "${signed_file}" 2>/dev/null | head -n1
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
	local multisig_members_json="${8:-[]}"
	local multisig_pubkey_txhash="${9:-}"
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
		--arg multisig_pubkey_txhash "${multisig_pubkey_txhash}" \
		--arg created_at "$(date -u +"%Y-%m-%dT%H:%M:%SZ")" \
		--argjson multisig_members "${multisig_members_json}" \
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
		} + (
			if $type == "multisig" then
				{
					multisig: {
						threshold: 2,
						signer_count: 3,
						members: $multisig_members,
						pubkey_txhash: $multisig_pubkey_txhash
					}
				}
			else
				{}
			end
		)]
		' "${USER_ACCOUNTS_FILE}" >"${tmp_file}"
	chmod 644 "${tmp_file}"
	mv "${tmp_file}" "${USER_ACCOUNTS_FILE}"
	chmod 644 "${USER_ACCOUNTS_FILE}"
}

create_multisig_key() {
	local key_name="$1"
	local signer_count=3
	local threshold=2
	local signer_names=()
	local members_json="[]"
	local idx signer_name signer_json signer_addr signer_mnemonic joined_members

	for idx in $(seq 1 "${signer_count}"); do
		signer_name="${key_name}-signer-${idx}"
		signer_names+=("${signer_name}")

		if key_exists "${signer_name}"; then
			signer_addr="$(trim "$("${DAEMON}" --home "${DAEMON_HOME}" keys show "${signer_name}" -a --keyring-backend "${KEYRING_BACKEND}")")"
			signer_mnemonic=""
		else
			signer_json="$("${DAEMON}" --home "${DAEMON_HOME}" keys add "${signer_name}" --keyring-backend "${KEYRING_BACKEND}" --output json)"
			signer_addr="$(trim "$(printf '%s' "${signer_json}" | jq -r '.address // empty')")"
			signer_mnemonic="$(printf '%s' "${signer_json}" | jq -r '.mnemonic // empty')"
		fi

		if [ -z "${signer_addr}" ]; then
			echo "ERROR: failed to create/read multisig signer ${signer_name}" >&2
			exit 1
		fi

		members_json="$(
			jq -c \
				--arg name "${signer_name}" \
				--arg address "${signer_addr}" \
				--arg mnemonic "${signer_mnemonic}" \
				'. + [{name: $name, address: $address, mnemonic: $mnemonic}]' \
				<<<"${members_json}"
		)"
	done

	if ! key_exists "${key_name}"; then
		joined_members="$(IFS=,; printf '%s' "${signer_names[*]}")"
		"${DAEMON}" --home "${DAEMON_HOME}" keys add "${key_name}" \
			--multisig "${joined_members}" \
			--multisig-threshold "${threshold}" \
			--nosort \
			--keyring-backend "${KEYRING_BACKEND}" >/dev/null
	fi

	NEW_ACCOUNT_NAME="${key_name}"
	NEW_ACCOUNT_ADDRESS="$(trim "$("${DAEMON}" --home "${DAEMON_HOME}" keys show "${key_name}" -a --keyring-backend "${KEYRING_BACKEND}")")"
	NEW_ACCOUNT_MNEMONIC=""
	NEW_ACCOUNT_MULTISIG_MEMBERS_JSON="${members_json}"
	NEW_ACCOUNT_MULTISIG_SIGNER_1="${signer_names[0]}"
	NEW_ACCOUNT_MULTISIG_SIGNER_2="${signer_names[1]}"

	if [ -z "${NEW_ACCOUNT_ADDRESS}" ]; then
		echo "ERROR: failed to read multisig account address for ${key_name}" >&2
		exit 1
	fi
}

register_multisig_pubkey() {
	local key_name="$1"
	local address="$2"
	local signer1="$3"
	local signer2="$4"
	local unsigned_file signed_file tx_json txhash code raw_log acc_num seq

	IFS=$'\t' read -r acc_num seq < <(query_account_number_sequence "${address}")
	if [ -z "${acc_num}" ] || [ -z "${seq}" ]; then
		echo "ERROR: failed to query multisig account number/sequence for ${address}" >&2
		exit 1
	fi

	unsigned_file="$(mktemp /tmp/test-msig-selfsend-unsigned.XXXXXX.json)"
	signed_file="$(mktemp /tmp/test-msig-selfsend-signed.XXXXXX.json)"
	trap 'rm -f "${unsigned_file:-}" "${signed_file:-}"' RETURN

	"${DAEMON}" tx bank send "${key_name}" "${address}" "1${BASE_DENOM}" \
		--home "${DAEMON_HOME}" \
		--chain-id "${CHAIN_ID}" \
		--node "${NODE_ADDR}" \
		--keyring-backend "${KEYRING_BACKEND}" \
		--gas 500000 \
		--gas-prices "${MIN_GAS_PRICE}" \
		--account-number "${acc_num}" \
		--sequence "${seq}" \
		--generate-only \
		--output json >"${unsigned_file}"

	if ! multisig_sign_unsigned "${unsigned_file}" "${key_name}" "${address}" "${signer1}" "${signer2}" "${acc_num}" "${seq}" >"${signed_file}"; then
		trap - RETURN
		echo "ERROR: failed to sign multisig pubkey registration tx for ${address}" >&2
		echo "ERROR: unsigned tx: ${unsigned_file}" >&2
		echo "ERROR: partial signed tx output: ${signed_file}" >&2
		exit 1
	fi

	local signed_pk_type
	signed_pk_type="$(signed_tx_pubkey_type "${signed_file}")"
	if [ "${signed_pk_type}" != "/cosmos.crypto.multisig.LegacyAminoPubKey" ]; then
		trap - RETURN
		echo "ERROR: signed multisig pubkey registration tx does not contain a multisig public key" >&2
		echo "ERROR: expected /cosmos.crypto.multisig.LegacyAminoPubKey, got: ${signed_pk_type:-none}" >&2
		echo "ERROR: unsigned tx: ${unsigned_file}" >&2
		echo "ERROR: signed tx: ${signed_file}" >&2
		jq -c '.auth_info.signer_infos // empty' "${signed_file}" >&2 || true
		exit 1
	fi

	tx_json="$("${DAEMON}" tx broadcast "${signed_file}" \
		--node "${NODE_ADDR}" \
		--broadcast-mode sync \
		--output json 2>&1)" || true
	if ! jq -e . >/dev/null 2>&1 <<<"${tx_json}"; then
		trap - RETURN
		echo "ERROR: multisig pubkey registration broadcast failed before returning JSON: ${tx_json}" >&2
		echo "ERROR: signed tx: ${signed_file}" >&2
		exit 1
	fi
	code="$(printf '%s' "${tx_json}" | jq -r '.code // 0')"
	raw_log="$(printf '%s' "${tx_json}" | jq -r '.raw_log // empty')"
	txhash="$(printf '%s' "${tx_json}" | jq -r '.txhash // empty')"

	if [ "${code}" != "0" ]; then
		trap - RETURN
		echo "ERROR: multisig pubkey registration tx failed (code=${code}): ${raw_log}" >&2
		echo "ERROR: signed tx: ${signed_file}" >&2
		exit 1
	fi
	if [ -z "${txhash}" ]; then
		trap - RETURN
		echo "ERROR: multisig pubkey registration broadcast returned no tx hash" >&2
		echo "ERROR: signed tx: ${signed_file}" >&2
		exit 1
	fi

	wait_for_tx_confirmation "${txhash}"
	echo "INFO: multisig pubkey registration tx ${txhash} included; signed tx pubkey type=${signed_pk_type}" >&2
	verify_multisig_pubkey_seeded "${address}"
	MULTISIG_PUBKEY_TXHASH="${txhash}"
	rm -f "${unsigned_file}" "${signed_file}"
	trap - RETURN
}

assert_evm_chain_ready() {
	require_cmd "${DAEMON}"

	if ! curl -sf "${NODE_ADDR}/status" >/dev/null 2>&1; then
		echo "ERROR: lumerad RPC is not reachable at ${NODE_ADDR}" >&2
		exit 1
	fi

	if ! "${DAEMON}" status --node "${NODE_ADDR}" >/dev/null 2>&1; then
		echo "ERROR: ${DAEMON} cannot query node status via ${NODE_ADDR}" >&2
		exit 1
	fi

	# `q evm params` only succeeds on a chain where the cosmos-evm module
	# (Go-side: x/vm; CLI-side: "evm") is live — i.e. one that has already
	# gone through the EVM upgrade. A pre-upgrade chain rejects the query,
	# and a pre-upgrade binary doesn't even register the subcommand. Either
	# failure means we cannot derive eth_secp256k1 accounts that match what
	# users will see on chain.
	if ! "${DAEMON}" q evm params --node "${NODE_ADDR}" --output json >/dev/null 2>&1; then
		echo "ERROR: chain at ${NODE_ADDR} is not EVM-migrated (lumerad q evm params failed)" >&2
		exit 1
	fi
}

evm_account_name_for() {
	local cosmos_name="$1"

	# Insert "evm-" between the "user-account-" prefix and the validator
	# suffix so e.g. "user-account-val4-001" becomes
	# "user-account-evm-val4-001". For names that don't follow the standard
	# devnet pattern, fall back to a leading "evm-" tag so they're still
	# distinguishable from their legacy counterpart in the keyring.
	case "${cosmos_name}" in
	user-account-val*)
		printf 'user-account-evm-%s' "${cosmos_name#user-account-}"
		;;
	*)
		printf 'evm-%s' "${cosmos_name}"
		;;
	esac
}

import_evm_key_from_mnemonic() {
	local key_name="$1"
	local mnemonic="$2"

	# `--recover` makes lumerad consume the mnemonic from stdin (one line).
	# Coin-type 60 / eth_secp256k1 are EVM-account defaults, but we pass
	# them explicitly so the call stays correct on chains where the binary
	# default has not yet flipped or has been overridden via config.
	printf '%s\n' "${mnemonic}" | "${DAEMON}" --home "${DAEMON_HOME}" \
		keys add "${key_name}" \
		--recover \
		--coin-type 60 \
		--algo eth_secp256k1 \
		--keyring-backend "${KEYRING_BACKEND}" \
		--output json
}

ensure_evm_key_from_mnemonic() {
	local evm_name="$1"
	local mnemonic="$2"
	local source_name="$3"
	local source_address="$4"
	local key_json

	EVM_KEY_ADDRESS=""
	EVM_KEY_ACTION=""

	if key_exists "${evm_name}"; then
		EVM_KEY_ADDRESS="$(trim "$("${DAEMON}" --home "${DAEMON_HOME}" keys show "${evm_name}" -a --keyring-backend "${KEYRING_BACKEND}" 2>/dev/null || true)")"
		if [ -z "${EVM_KEY_ADDRESS}" ]; then
			echo "ERROR: ${evm_name} exists in keyring but its address could not be read" >&2
			exit 1
		fi
		echo "Reusing existing EVM key ${evm_name}: ${EVM_KEY_ADDRESS}"
		EVM_KEY_ACTION="reused"
		return 0
	fi

	if ! key_json="$(import_evm_key_from_mnemonic "${evm_name}" "${mnemonic}")"; then
		echo "ERROR: failed to import ${evm_name} from mnemonic of ${source_name}" >&2
		exit 1
	fi
	EVM_KEY_ADDRESS="$(trim "$(printf '%s' "${key_json}" | jq -r '.address // empty')")"
	if [ -z "${EVM_KEY_ADDRESS}" ]; then
		echo "ERROR: failed to read new EVM address for ${evm_name}" >&2
		exit 1
	fi
	echo "Imported EVM key ${evm_name}: ${EVM_KEY_ADDRESS} (legacy ${source_address})"
	EVM_KEY_ACTION="imported"
}

ensure_evm_multisig_composite() {
	local evm_name="$1"
	local threshold="$2"
	local member_names_csv="$3"

	EVM_MULTISIG_ADDRESS=""

	if ! key_exists "${evm_name}"; then
		"${DAEMON}" --home "${DAEMON_HOME}" keys add "${evm_name}" \
			--multisig "${member_names_csv}" \
			--multisig-threshold "${threshold}" \
			--nosort \
			--keyring-backend "${KEYRING_BACKEND}" >/dev/null
	fi

	EVM_MULTISIG_ADDRESS="$(trim "$("${DAEMON}" --home "${DAEMON_HOME}" keys show "${evm_name}" -a --keyring-backend "${KEYRING_BACKEND}" 2>/dev/null || true)")"
	if [ -z "${EVM_MULTISIG_ADDRESS}" ]; then
		echo "ERROR: failed to read EVM multisig address for ${evm_name}" >&2
		exit 1
	fi
}

update_account_to_evm() {
	local idx="$1"
	local new_name="$2"
	local new_address="$3"
	local legacy_address="$4"
	local legacy_key="$5"
	local tmp_file

	tmp_file="$(mktemp)"
	jq \
		--argjson i "${idx}" \
		--arg new_name "${new_name}" \
		--arg new_address "${new_address}" \
		--arg legacy_address "${legacy_address}" \
		--arg legacy_key "${legacy_key}" \
		'
		.[$i] |= (
			.name = $new_name
			| .address = $new_address
			| .legacy_address = $legacy_address
			| .legacy_key = $legacy_key
			| .type = "evm"
		)
		' "${USER_ACCOUNTS_FILE}" >"${tmp_file}"
	chmod 644 "${tmp_file}"
	mv "${tmp_file}" "${USER_ACCOUNTS_FILE}"
	chmod 644 "${USER_ACCOUNTS_FILE}"
}

update_multisig_account_to_evm() {
	local idx="$1"
	local new_name="$2"
	local new_address="$3"
	local legacy_address="$4"
	local legacy_key="$5"
	local threshold="$6"
	local signer_count="$7"
	local members_json="$8"
	local tmp_file

	tmp_file="$(mktemp)"
	jq \
		--argjson i "${idx}" \
		--arg new_name "${new_name}" \
		--arg new_address "${new_address}" \
		--arg legacy_address "${legacy_address}" \
		--arg legacy_key "${legacy_key}" \
		--argjson threshold "${threshold}" \
		--argjson signer_count "${signer_count}" \
		--argjson members "${members_json}" \
		'
		.[$i] |= (
			.name = $new_name
			| .address = $new_address
			| .legacy_address = $legacy_address
			| .legacy_key = $legacy_key
			| .type = "evm"
			| .multisig.threshold = $threshold
			| .multisig.signer_count = $signer_count
			| .multisig.members = $members
		)
		' "${USER_ACCOUNTS_FILE}" >"${tmp_file}"
	chmod 644 "${tmp_file}"
	mv "${tmp_file}" "${USER_ACCOUNTS_FILE}"
	chmod 644 "${USER_ACCOUNTS_FILE}"
}

process_multisig_account_to_evm() {
	local idx="$1"
	local legacy_name="$2"
	local legacy_address="$3"
	local threshold signer_count member_count member_idx
	local evm_name evm_members_json="[]" evm_member_names=()
	local member_name member_address member_mnemonic evm_member_name evm_member_address joined_members

	threshold="$(jq -r --argjson i "${idx}" '.[$i].multisig.threshold // 2' "${USER_ACCOUNTS_FILE}")"
	signer_count="$(jq -r --argjson i "${idx}" '.[$i].multisig.signer_count // (.[$i].multisig.members | length)' "${USER_ACCOUNTS_FILE}")"
	member_count="$(jq -r --argjson i "${idx}" '.[$i].multisig.members | length' "${USER_ACCOUNTS_FILE}")"

	if ! [[ "${threshold}" =~ ^[0-9]+$ ]] || ! [[ "${signer_count}" =~ ^[0-9]+$ ]] || ! [[ "${member_count}" =~ ^[0-9]+$ ]] || ((member_count == 0)); then
		echo "Skipping ${legacy_name}: invalid multisig metadata" >&2
		skipped_other=$((skipped_other + 1))
		return 0
	fi

	for ((member_idx = 0; member_idx < member_count; member_idx++)); do
		member_name="$(jq -r --argjson i "${idx}" --argjson m "${member_idx}" '.[$i].multisig.members[$m].name // empty' "${USER_ACCOUNTS_FILE}")"
		member_address="$(jq -r --argjson i "${idx}" --argjson m "${member_idx}" '.[$i].multisig.members[$m].address // empty' "${USER_ACCOUNTS_FILE}")"
		member_mnemonic="$(jq -r --argjson i "${idx}" --argjson m "${member_idx}" '.[$i].multisig.members[$m].mnemonic // empty' "${USER_ACCOUNTS_FILE}")"

		if [ -z "${member_name}" ] || [ -z "${member_address}" ] || [ -z "${member_mnemonic}" ]; then
			echo "Skipping ${legacy_name}: multisig member #${member_idx} is missing name/address/mnemonic" >&2
			skipped_other=$((skipped_other + 1))
			return 0
		fi

		evm_member_name="$(evm_account_name_for "${member_name}")"
		ensure_evm_key_from_mnemonic "${evm_member_name}" "${member_mnemonic}" "${member_name}" "${member_address}"
		evm_member_address="${EVM_KEY_ADDRESS}"
		case "${EVM_KEY_ACTION}" in
		imported) imported=$((imported + 1)) ;;
		reused) reused=$((reused + 1)) ;;
		esac
		evm_member_names+=("${evm_member_name}")
		evm_members_json="$(
			jq -c \
				--arg name "${evm_member_name}" \
				--arg address "${evm_member_address}" \
				--arg legacy_key "${member_name}" \
				--arg legacy_address "${member_address}" \
				--arg mnemonic "${member_mnemonic}" \
				'
				. + [{
					name: $name,
					address: $address,
					legacy_key: $legacy_key,
					legacy_address: $legacy_address,
					mnemonic: $mnemonic,
					type: "evm"
				}]
				' <<<"${evm_members_json}"
		)"
	done

	evm_name="$(evm_account_name_for "${legacy_name}")"
	joined_members="$(IFS=,; printf '%s' "${evm_member_names[*]}")"
	ensure_evm_multisig_composite "${evm_name}" "${threshold}" "${joined_members}"
	update_multisig_account_to_evm "${idx}" "${evm_name}" "${EVM_MULTISIG_ADDRESS}" "${legacy_address}" "${legacy_name}" "${threshold}" "${signer_count}" "${evm_members_json}"
	echo "Updated multisig ${legacy_name} -> ${evm_name}: ${EVM_MULTISIG_ADDRESS} (legacy ${legacy_address})"
	processed=$((processed + 1))
}

cmd_generate_evm_accounts() {
	local count_limit=""
	while (( $# > 0 )); do
		case "$1" in
		--count)
			if (( $# < 2 )) || [[ "${2:-}" == --* ]]; then
				echo "ERROR: --count requires a positive integer" >&2
				exit 1
			fi
			count_limit="$2"
			shift 2
			;;
		*)
			echo "ERROR: unknown generate-evm-accounts flag: $1" >&2
			usage
			exit 1
			;;
		esac
	done
	if [ -n "${count_limit}" ] && { ! [[ "${count_limit}" =~ ^[0-9]+$ ]] || ((count_limit == 0)); }; then
		echo "ERROR: --count requires a positive integer" >&2
		exit 1
	fi

	local accounts_count idx process_count
	local processed=0 imported=0 reused=0 skipped_evm=0 skipped_other=0

	load_config
	assert_evm_chain_ready

	if [ ! -f "${USER_ACCOUNTS_FILE}" ]; then
		echo "ERROR: user accounts file not found: ${USER_ACCOUNTS_FILE}" >&2
		exit 1
	fi

	accounts_count="$(jq 'length' "${USER_ACCOUNTS_FILE}" 2>/dev/null || echo 0)"
	if [ -z "${accounts_count}" ] || [ "${accounts_count}" = "0" ]; then
		echo "No accounts in ${USER_ACCOUNTS_FILE}; nothing to do."
		return 0
	fi

	process_count="${accounts_count}"
	if [ -n "${count_limit}" ] && ((count_limit < accounts_count)); then
		process_count="${count_limit}"
	fi

	for ((idx = 0; idx < process_count; idx++)); do
		local cosmos_name cosmos_address mnemonic acct_type existing_legacy
		local evm_name new_address

		cosmos_name="$(jq -r --argjson i "${idx}" '.[$i].name // empty' "${USER_ACCOUNTS_FILE}")"
		cosmos_address="$(jq -r --argjson i "${idx}" '.[$i].address // empty' "${USER_ACCOUNTS_FILE}")"
		mnemonic="$(jq -r --argjson i "${idx}" '.[$i].mnemonic // empty' "${USER_ACCOUNTS_FILE}")"
		acct_type="$(jq -r --argjson i "${idx}" '.[$i].type // "cosmos"' "${USER_ACCOUNTS_FILE}")"
		existing_legacy="$(jq -r --argjson i "${idx}" '.[$i].legacy_address // empty' "${USER_ACCOUNTS_FILE}")"

		if [ -z "${cosmos_name}" ] || [ -z "${cosmos_address}" ]; then
			echo "Skipping entry #${idx}: missing name/address" >&2
			skipped_other=$((skipped_other + 1))
			continue
		fi

		if [ "${acct_type}" = "evm" ]; then
			if [ -n "${existing_legacy}" ]; then
				echo "Skipping ${cosmos_name}: already EVM (legacy=${existing_legacy})"
			else
				echo "Skipping ${cosmos_name}: already EVM (no legacy address to record)"
			fi
			skipped_evm=$((skipped_evm + 1))
			continue
		fi

		if [ "${acct_type}" = "multisig" ]; then
			process_multisig_account_to_evm "${idx}" "${cosmos_name}" "${cosmos_address}"
			continue
		fi

		if [ -z "${mnemonic}" ]; then
			echo "Skipping ${cosmos_name}: no mnemonic recorded" >&2
			skipped_other=$((skipped_other + 1))
			continue
		fi

		evm_name="$(evm_account_name_for "${cosmos_name}")"
		ensure_evm_key_from_mnemonic "${evm_name}" "${mnemonic}" "${cosmos_name}" "${cosmos_address}"
		new_address="${EVM_KEY_ADDRESS}"
		case "${EVM_KEY_ACTION}" in
		imported) imported=$((imported + 1)) ;;
		reused) reused=$((reused + 1)) ;;
		esac

		update_account_to_evm "${idx}" "${evm_name}" "${new_address}" "${cosmos_address}" "${cosmos_name}"
		processed=$((processed + 1))
	done

	cat <<EOF

Summary:
  Total entries:          ${accounts_count}
  Selected entries:       ${process_count}
  Updated to EVM:         ${processed}
    Newly imported keys:  ${imported}
    Reused existing keys: ${reused}
  Skipped (already EVM):  ${skipped_evm}
  Skipped (other):        ${skipped_other}
  Registry:               ${USER_ACCOUNTS_FILE}
EOF
}

cmd_new_account() {
	local amount_arg="$1"
	local create_multisig=0
	local amount_base amount_display key_name account_type actual_after_pubkey

	if [ "${amount_arg}" = "--multisig" ]; then
		create_multisig=1
		amount_arg="${2:-}"
	fi

	load_config
	assert_chain_ready

	# Accept either a bare number (interpreted as display units / LUME) or
	# an explicit base-denom amount like "10000ulume" (taken verbatim).
	if [[ "${amount_arg}" =~ ^([0-9]+)${BASE_DENOM}$ ]]; then
		amount_base="${BASH_REMATCH[1]}"
		amount_display="$(awk -v v="${amount_base}" -v e="${DISPLAY_EXPONENT}" '
			BEGIN {
				n = length(v)
				if (n <= e) {
					pad = ""
					for (i = 0; i < e - n; i++) pad = pad "0"
					frac = pad v
					whole = "0"
				} else {
					whole = substr(v, 1, n - e)
					frac = substr(v, n - e + 1)
				}
				sub(/0+$/, "", frac)
				if (frac == "") print whole
				else print whole "." frac
			}
		')"
	else
		amount_display="${amount_arg}"
		amount_base="$(parse_lume_amount_to_base "${amount_display}" "${DISPLAY_EXPONENT}")"
	fi

	if [ "${amount_base}" = "0" ]; then
		echo "ERROR: amount must be greater than zero" >&2
		exit 1
	fi

	key_name="$(next_account_name)"
	ensure_account_absent "${key_name}"
	NEW_ACCOUNT_MULTISIG_MEMBERS_JSON="[]"
	if ((create_multisig == 1)); then
		create_multisig_key "${key_name}"
		account_type="multisig"
	else
		create_key "${key_name}"
		account_type="$(account_type_for_key "${key_name}")"
	fi
	fund_account "${amount_base}" "${NEW_ACCOUNT_ADDRESS}"
	MULTISIG_PUBKEY_TXHASH=""
	if ((create_multisig == 1)); then
		register_multisig_pubkey "${NEW_ACCOUNT_NAME}" "${NEW_ACCOUNT_ADDRESS}" "${NEW_ACCOUNT_MULTISIG_SIGNER_1}" "${NEW_ACCOUNT_MULTISIG_SIGNER_2}"
		actual_after_pubkey="$(query_balance_amount "${NEW_ACCOUNT_ADDRESS}")"
		[[ -z "${actual_after_pubkey}" ]] && actual_after_pubkey="0"
		if ((actual_after_pubkey < amount_base)); then
			fund_account "$((amount_base - actual_after_pubkey))" "${NEW_ACCOUNT_ADDRESS}"
		fi
	fi
	verify_funding "${NEW_ACCOUNT_ADDRESS}" "${amount_base}"
	write_user_account_record "${NEW_ACCOUNT_NAME}" "${NEW_ACCOUNT_ADDRESS}" "${NEW_ACCOUNT_MNEMONIC}" "${account_type}" "${amount_base}" "${amount_display}" "${FUNDING_TXHASH}" "${NEW_ACCOUNT_MULTISIG_MEMBERS_JSON}" "${MULTISIG_PUBKEY_TXHASH}"

	cat <<EOF
Created account:
  Name: ${NEW_ACCOUNT_NAME}
  Address: ${NEW_ACCOUNT_ADDRESS}
  Type: ${account_type}
  Funded: ${amount_display} ${DISPLAY_DENOM} (${amount_base}${BASE_DENOM})
  Funding key: ${FUNDER_KEY_NAME}
  Tx hash: ${FUNDING_TXHASH}
  Registry: ${USER_ACCOUNTS_FILE}
EOF
	if ((create_multisig == 1)); then
		jq -r '
			"  Multisig: 2-of-3",
			(.[] | "  Signer: \(.name) -> \(.address)")
		' <<<"${NEW_ACCOUNT_MULTISIG_MEMBERS_JSON}"
		printf '  Pubkey tx hash: %s\n' "${MULTISIG_PUBKEY_TXHASH}"
	else
		cat <<EOF
Mnemonic:
${NEW_ACCOUNT_MNEMONIC}
EOF
	fi
}

main() {
	if [ $# -lt 1 ]; then
		usage
		exit 1
	fi

	case "$1" in
	new-account)
		[ $# -eq 2 ] || { [ $# -eq 3 ] && [ "$2" = "--multisig" ]; } || {
			echo "ERROR: new-account requires AMOUNT or --multisig AMOUNT (bare number = LUME, or e.g. '10000ulume')" >&2
			usage
			exit 1
		}
		if [ "$2" = "--multisig" ]; then
			cmd_new_account "$2" "$3"
		else
			cmd_new_account "$2"
		fi
		;;
	list-accounts)
		[ $# -eq 1 ] || {
			echo "ERROR: list-accounts does not accept arguments" >&2
			usage
			exit 1
		}
		cmd_list_accounts
		;;
	generate-evm-accounts)
		shift
		cmd_generate_evm_accounts "$@"
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
