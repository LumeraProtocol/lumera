#!/bin/bash
#
# Shared helpers for persisting validator-local devnet accounts in a single
# JSON registry: /shared/status/<moniker>/accounts.json

ACCOUNTS_DISPLAY_DENOM_DEFAULT="${ACCOUNTS_DISPLAY_DENOM_DEFAULT:-lume}"
ACCOUNTS_DISPLAY_EXPONENT_DEFAULT="${ACCOUNTS_DISPLAY_EXPONENT_DEFAULT:-6}"

accounts_registry_init() {
	local node_status_dir="$1"
	local cfg_chain="${2:-}"

	ACCOUNTS_FILE="${node_status_dir}/accounts.json"
	ACCOUNTS_BASE_DENOM="ulume"
	ACCOUNTS_DISPLAY_DENOM="${ACCOUNTS_DISPLAY_DENOM_DEFAULT}"
	ACCOUNTS_DISPLAY_EXPONENT="${ACCOUNTS_DISPLAY_EXPONENT_DEFAULT}"

	if command -v jq >/dev/null 2>&1 && [[ -n "${cfg_chain}" && -f "${cfg_chain}" ]]; then
		local base_denom
		base_denom="$(jq -r '.chain.denom.bond // empty' "${cfg_chain}" 2>/dev/null || true)"
		if [[ -n "${base_denom}" && "${base_denom}" != "null" ]]; then
			ACCOUNTS_BASE_DENOM="${base_denom}"
		fi
	fi
}

ensure_accounts_registry() {
	if [[ ! -f "${ACCOUNTS_FILE}" ]]; then
		printf '[]\n' >"${ACCOUNTS_FILE}"
	fi
	chmod 644 "${ACCOUNTS_FILE}"
}

accounts_registry_parse_coin() {
	local coin="$1"
	local amount denom

	if [[ -z "${coin}" ]]; then
		printf '\t\n'
		return 0
	fi

	if [[ "${coin}" =~ ^([0-9]+)([[:alpha:]][[:alnum:]/:_-]*)$ ]]; then
		amount="${BASH_REMATCH[1]}"
		denom="${BASH_REMATCH[2]}"
		printf '%s\t%s\n' "${amount}" "${denom}"
		return 0
	fi

	echo "[ACCOUNTS] WARN: could not parse coin amount '${coin}'" >&2
	printf '\t\n'
}

accounts_registry_format_display_amount() {
	local base_amount="$1"
	local exponent="${2:-${ACCOUNTS_DISPLAY_EXPONENT}}"

	if [[ -z "${base_amount}" || ! "${base_amount}" =~ ^[0-9]+$ ]]; then
		printf '%s' "${base_amount}"
		return 0
	fi
	if [[ -z "${exponent}" || ! "${exponent}" =~ ^[0-9]+$ ]]; then
		printf '%s' "${base_amount}"
		return 0
	fi
	if ((exponent == 0)); then
		printf '%s' "${base_amount}"
		return 0
	fi

	local length whole fraction
	length="${#base_amount}"
	if ((length <= exponent)); then
		fraction="$(printf "%0*d%s" "$((exponent - length))" 0 "${base_amount}")"
		while [[ "${fraction}" == *0 ]]; do
			fraction="${fraction%0}"
		done
		if [[ -z "${fraction}" ]]; then
			printf '0'
		else
			printf '0.%s' "${fraction}"
		fi
		return 0
	fi

	whole="${base_amount:0:length-exponent}"
	fraction="${base_amount:length-exponent}"
	while [[ "${fraction}" == *0 ]]; do
		fraction="${fraction%0}"
	done
	if [[ -z "${fraction}" ]]; then
		printf '%s' "${whole}"
	else
		printf '%s.%s' "${whole}" "${fraction}"
	fi
}

accounts_registry_upsert() {
	local name="$1"
	local address="$2"
	local mnemonic="$3"
	local account_type="$4"
	local funded_coin="$5"
	local funding_key="$6"
	local funding_txhash="$7"
	local created_at tmp_file funded_base="" funded_base_denom="" funded_display=""

	ensure_accounts_registry
	created_at="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
	IFS=$'\t' read -r funded_base funded_base_denom < <(accounts_registry_parse_coin "${funded_coin}")
	if [[ -z "${funded_base_denom}" ]]; then
		funded_base_denom="${ACCOUNTS_BASE_DENOM}"
	fi
	if [[ -n "${funded_base}" ]]; then
		funded_display="$(accounts_registry_format_display_amount "${funded_base}" "${ACCOUNTS_DISPLAY_EXPONENT}")"
	fi

	tmp_file="$(mktemp "${ACCOUNTS_FILE}.tmp.XXXXXX")"
	jq \
		--arg name "${name}" \
		--arg address "${address}" \
		--arg mnemonic "${mnemonic}" \
		--arg type "${account_type}" \
		--arg funded_base "${funded_base}" \
		--arg funded_display "${funded_display}" \
		--arg base_denom "${funded_base_denom}" \
		--arg display_denom "${ACCOUNTS_DISPLAY_DENOM}" \
		--arg funding_key "${funding_key}" \
		--arg txhash "${funding_txhash}" \
		--arg created_at "${created_at}" \
		'
		(map(select(.name == $name)) | first) as $existing
		| map(select(.name != $name))
		+ [{
			name: $name,
			address: (if $address != "" then $address else ($existing.address // "") end),
			mnemonic: (if $mnemonic != "" then $mnemonic else ($existing.mnemonic // "") end),
			type: (if $type != "" then $type else ($existing.type // "cosmos") end),
			funded: {
				display_amount: (if $funded_display != "" then $funded_display else ($existing.funded.display_amount // "0") end),
				display_denom: (if $display_denom != "" then $display_denom else ($existing.funded.display_denom // "lume") end),
				base_amount: (if $funded_base != "" then $funded_base else ($existing.funded.base_amount // "0") end),
				base_denom: (if $base_denom != "" then $base_denom else ($existing.funded.base_denom // "ulume") end)
			},
			funding_key: (if $funding_key != "" then $funding_key else ($existing.funding_key // "") end),
			funding_txhash: (if $txhash != "" then $txhash else ($existing.funding_txhash // "") end),
			created_at: ($existing.created_at // $created_at)
		}]
		| sort_by(.name)
		' "${ACCOUNTS_FILE}" >"${tmp_file}"
	chmod 644 "${tmp_file}"
	mv "${tmp_file}" "${ACCOUNTS_FILE}"
	chmod 644 "${ACCOUNTS_FILE}"
}

accounts_registry_get_field() {
	local name="$1"
	local field_path="$2"

	ensure_accounts_registry
	jq -r \
		--arg name "${name}" \
		--arg field_path "${field_path}" \
		'
		(map(select(.name == $name)) | first) as $entry
		| if $entry == null then
			empty
		  else
			($field_path | split(".")) as $path
			| ($entry | getpath($path)) // empty
		  end
		' "${ACCOUNTS_FILE}" 2>/dev/null || true
}
