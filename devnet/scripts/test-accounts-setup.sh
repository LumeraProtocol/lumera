#!/usr/bin/env bash
# test-accounts-setup.sh
#
# Reads the per-validator "test_accounts" block from validators.json and, if
# present and enabled, creates the configured number of funded test accounts
# by delegating to lumera-helper.sh's new-account command.
#
# Expected validators.json shape (for this validator's entry):
#
#   "test_accounts": {
#     "count": 10,
#     "balance_base": "10000ulume",
#     "balance_increment": "5000ulume"
#   }
#
# Behavior:
#   - Exits 0 (no-op) if the block is missing or count <= 0.
#   - Waits for lumerad RPC to be reachable before provisioning.
#   - Funds account i with balance_base + i * balance_increment, passing the
#     amount straight to lumera-helper.sh (so the units come from config,
#     unmodified — e.g. "10000ulume", "15000ulume", "20000ulume", ...).
#
# Runs idempotently: lumera-helper.sh skips already-provisioned accounts.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=/dev/null
source "${SCRIPT_DIR}/common.sh"

: "${MONIKER:?MONIKER environment variable must be set}"

SHARED_DIR="${SHARED_DIR:-/shared}"
CFG_DIR="${CFG_DIR:-${SHARED_DIR}/config}"
CFG_CHAIN="${CFG_CHAIN:-${CFG_DIR}/config.json}"
CFG_VALS="${CFG_VALS:-${CFG_DIR}/validators.json}"

LUMERA_RPC_PORT="${LUMERA_RPC_PORT:-26657}"
LUMERA_RPC_ADDR="${LUMERA_RPC_ADDR:-http://127.0.0.1:${LUMERA_RPC_PORT}}"

LOG_PREFIX="[TA]"

log() { echo "${LOG_PREFIX} $*"; }

have jq || {
	log "ERROR: jq not available; cannot provision test accounts."
	exit 1
}

if [ ! -f "${CFG_VALS}" ]; then
	log "Missing ${CFG_VALS}; skipping."
	exit 0
fi

VAL_REC_JSON="$(jq -c --arg m "${MONIKER}" '[.[] | select(.moniker==$m)][0]' "${CFG_VALS}")"
if [ -z "${VAL_REC_JSON}" ] || [ "${VAL_REC_JSON}" = "null" ]; then
	log "Validator moniker ${MONIKER} not found in validators.json; skipping."
	exit 0
fi

TA_COUNT="$(printf '%s' "${VAL_REC_JSON}" | jq -r 'try .test_accounts.count // 0')"
TA_BASE="$(printf '%s' "${VAL_REC_JSON}" | jq -r 'try .test_accounts.balance_base // ""')"
TA_INCR="$(printf '%s' "${VAL_REC_JSON}" | jq -r 'try .test_accounts.balance_increment // "0"')"

if ! [[ "${TA_COUNT}" =~ ^[0-9]+$ ]] || [ "${TA_COUNT}" -eq 0 ]; then
	log "No test_accounts.count configured for ${MONIKER}; skipping."
	exit 0
fi

if [ -z "${TA_BASE}" ]; then
	log "ERROR: test_accounts.balance_base is required when count > 0."
	exit 1
fi

# Accept either "<n><denom>" (e.g. "10000ulume") or a bare number. Split the
# numeric part and the denom suffix so we can compute balance_base + i * incr
# while preserving the denom unchanged in the final argument to lumera-helper.
split_amount() {
	local raw="$1"
	local numeric denom
	if [[ "${raw}" =~ ^([0-9]+)([a-zA-Z][a-zA-Z0-9/:._-]*)?$ ]]; then
		numeric="${BASH_REMATCH[1]}"
		denom="${BASH_REMATCH[2]:-}"
		printf '%s\t%s\n' "${numeric}" "${denom}"
		return 0
	fi
	return 1
}

IFS=$'\t' read -r BASE_NUM BASE_DENOM_SFX < <(split_amount "${TA_BASE}") || {
	log "ERROR: balance_base='${TA_BASE}' is not a valid amount."
	exit 1
}

IFS=$'\t' read -r INCR_NUM INCR_DENOM_SFX < <(split_amount "${TA_INCR}") || {
	log "ERROR: balance_increment='${TA_INCR}' is not a valid amount."
	exit 1
}

# If both sides carry a denom, they must agree. If one side omits it, inherit
# from the other. Callers typically specify both as "Nulume".
if [ -n "${BASE_DENOM_SFX}" ] && [ -n "${INCR_DENOM_SFX}" ] && [ "${BASE_DENOM_SFX}" != "${INCR_DENOM_SFX}" ]; then
	log "ERROR: balance_base denom (${BASE_DENOM_SFX}) != balance_increment denom (${INCR_DENOM_SFX})."
	exit 1
fi
DENOM_SFX="${BASE_DENOM_SFX:-${INCR_DENOM_SFX}}"

wait_for_lumera() {
	log "Waiting for lumerad RPC at ${LUMERA_RPC_ADDR}..."
	for _ in $(seq 1 300); do
		if curl -sf "${LUMERA_RPC_ADDR}/status" >/dev/null 2>&1; then
			log "lumerad RPC is up."
			return 0
		fi
		sleep 1
	done
	log "ERROR: lumerad RPC did not become ready in time."
	return 1
}

wait_for_lumera || exit 1

log "Provisioning ${TA_COUNT} test account(s): base=${TA_BASE}, incr=${TA_INCR}"

HELPER="${SCRIPT_DIR}/lumera-helper.sh"
if [ ! -x "${HELPER}" ]; then
	log "ERROR: ${HELPER} not executable."
	exit 1
fi

for i in $(seq 0 $((TA_COUNT - 1))); do
	amount_num=$((BASE_NUM + i * INCR_NUM))
	amount_arg="${amount_num}${DENOM_SFX}"

	log "[$((i + 1))/${TA_COUNT}] Creating account funded with ${amount_arg}..."
	if ! "${HELPER}" new-account "${amount_arg}"; then
		log "ERROR: failed to create/fund test account #$((i + 1)) with ${amount_arg}."
		exit 1
	fi
done

log "Provisioned ${TA_COUNT} test account(s) for ${MONIKER}."
