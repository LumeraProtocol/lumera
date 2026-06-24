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
#     "balance_base": "10000ulume",      # or "1lume" — display denom auto-
#     "balance_increment": "5000ulume",  # converted; both fields must agree
#     "multisig": true   # optional, default false; if true, creates 2-of-3 multisig accounts instead of single-sig
#   }
#
# Supported denoms: "ulume" (bond/base denom, used verbatim) and "lume"
# (display denom, multiplied by 10^6 up front and passed downstream as
# ulume). Mixing denoms across balance_base/balance_increment is rejected.
#
# Behavior:
#   - Exits 0 (no-op) if the block is missing or count <= 0.
#   - Waits for lumerad RPC to be reachable before provisioning.
#   - Funds account i with balance_base + i * balance_increment, normalizing
#     lume → ulume up front so funder budgeting and helper invocation both
#     operate in base units (e.g. "10000ulume", "15000ulume", ...).
#   - If test_accounts.multisig is true, creates 2-of-3 multisig accounts
#     instead of single-sig accounts.
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
TA_MULTISIG="$(printf '%s' "${VAL_REC_JSON}" | jq -r 'try .test_accounts.multisig // false')"

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
# from the other. Callers typically specify both as "Nulume" or "Nlume".
if [ -n "${BASE_DENOM_SFX}" ] && [ -n "${INCR_DENOM_SFX}" ] && [ "${BASE_DENOM_SFX}" != "${INCR_DENOM_SFX}" ]; then
	log "ERROR: balance_base denom (${BASE_DENOM_SFX}) != balance_increment denom (${INCR_DENOM_SFX})."
	exit 1
fi
DENOM_SFX="${BASE_DENOM_SFX:-${INCR_DENOM_SFX}}"

# Normalize the denom up front so funder budgeting and helper invocation
# both operate in the chain's base denom (ulume). Lumera's display denom is
# `lume` with exponent 6 (1 lume = 10^6 ulume); these values match
# lumera-helper.sh's load_config() fallback when bank.denom_metadata is not
# yet available. If Lumera's display denom ever changes, update both this
# script and lumera-helper.sh.
TA_BASE_DENOM="ulume"
TA_DISPLAY_DENOM="lume"
TA_DISPLAY_EXPONENT=6

case "${DENOM_SFX}" in
"${TA_BASE_DENOM}" | "")
	# Already in base units. Empty falls here too — happens when both base
	# and increment are bare numbers (e.g. balance_increment defaults to "0"
	# without a suffix); downstream math then runs in ulume by convention.
	:
	;;
"${TA_DISPLAY_DENOM}")
	# 1 lume = 10^TA_DISPLAY_EXPONENT ulume. Bash arithmetic supports `**`
	# directly. Even tens of millions of lume per account stay well inside
	# 64-bit signed int range.
	mult=$((10 ** TA_DISPLAY_EXPONENT))
	BASE_NUM=$((BASE_NUM * mult))
	INCR_NUM=$((INCR_NUM * mult))
	DENOM_SFX="${TA_BASE_DENOM}"
	;;
*)
	log "ERROR: unsupported denom '${DENOM_SFX}' in test_accounts; expected ${TA_BASE_DENOM} or ${TA_DISPLAY_DENOM}."
	exit 1
	;;
esac

wait_for_lumera() {
	log "Waiting for lumerad RPC at ${LUMERA_RPC_ADDR}..."
	local rpc_up=0
	for _ in $(seq 1 300); do
		if curl -sf "${LUMERA_RPC_ADDR}/status" >/dev/null 2>&1; then
			rpc_up=1
			break
		fi
		sleep 1
	done
	if [ "${rpc_up}" -ne 1 ]; then
		log "ERROR: lumerad RPC did not become ready in time."
		return 1
	fi

	# RPC is up, but tx submission fails with "lumera is not ready; please
	# wait for first block: invalid height" until the chain commits block 1.
	log "lumerad RPC is up; waiting for first committed block..."
	for _ in $(seq 1 300); do
		local height
		height="$(curl -sf "${LUMERA_RPC_ADDR}/status" 2>/dev/null |
			jq -r '.result.sync_info.latest_block_height // "0"' 2>/dev/null)"
		if [[ "${height}" =~ ^[0-9]+$ ]] && [ "${height}" -ge 1 ]; then
			log "First block committed (height=${height})."
			return 0
		fi
		sleep 1
	done
	log "ERROR: lumerad did not produce a block in time."
	return 1
}

wait_for_lumera || exit 1

# ── Dedicated temp funder ──────────────────────────────────────────────
# We provision all test accounts via a dedicated temp funder key rather
# than the validator's genesis key. Without this, test-accounts-setup and
# supernode-setup race on the validator-genesis sequence (both background
# scripts issue tx bank sends concurrently once the first block lands),
# and one side loses with "account sequence mismatch".
# By sending a single bootstrap tx genesis → temp-funder and then funding
# all N test accounts from the temp funder, we collapse the contention
# window from N txs to 1.
CHAIN_ID="$(jq -r '.chain.id' "${CFG_CHAIN}")"
BASE_DENOM="$(jq -r '.chain.denom.bond' "${CFG_CHAIN}")"
KEYRING_BACKEND="$(jq -r '.daemon.keyring_backend' "${CFG_CHAIN}")"
DAEMON="$(jq -r '.daemon.binary' "${CFG_CHAIN}")"
DAEMON_HOME_BASE="$(jq -r '.paths.base.container' "${CFG_CHAIN}")"
DAEMON_DIR="$(jq -r '.paths.directories.daemon' "${CFG_CHAIN}")"
DAEMON_HOME="${DAEMON_HOME_BASE}/${DAEMON_DIR}"
MIN_GAS_PRICE="$(jq -r '.chain.denom.minimum_gas_price // "0.025ulume"' "${CFG_CHAIN}")"
VAL_KEY_NAME="$(printf '%s' "${VAL_REC_JSON}" | jq -r '.key_name')"
VALIDATOR_MULTISIG_ENABLED="$(printf '%s' "${VAL_REC_JSON}" | jq -r 'try .multisig.enabled // false')"
if [ "${VALIDATOR_MULTISIG_ENABLED}" = "true" ]; then
	VAL_KEY_NAME="prepare-funder-${MONIKER}"
	log "Validator key is multisig; using single-sig prepare funder ${VAL_KEY_NAME} to bootstrap test accounts."
	if ! "${DAEMON}" --home "${DAEMON_HOME}" keys show "${VAL_KEY_NAME}" \
		--keyring-backend "${KEYRING_BACKEND}" >/dev/null 2>&1; then
		log "ERROR: ${VAL_KEY_NAME} not found in keyring. validator-setup.sh must create it before test account provisioning."
		exit 1
	fi
fi

# Bind the shared /shared/status/<moniker>/accounts.json registry so we can
# persist the temp funder alongside other validator-local accounts (survives
# the EVM upgrade; visible to tools that read the registry post-upgrade).
NODE_STATUS_DIR="${SHARED_DIR}/status/${MONIKER}"
mkdir -p "${NODE_STATUS_DIR}"
accounts_registry_init "${NODE_STATUS_DIR}" "${CFG_CHAIN}"

# Closed-form sum: count*base + incr*count*(count-1)/2. Pad per-tx gas
# headroom (bank send with --gas auto typically uses ~5–6k ulume at
# 0.03ulume/gas; 10k × count is a comfortable margin).
TA_TOTAL=$((TA_COUNT * BASE_NUM + INCR_NUM * TA_COUNT * (TA_COUNT - 1) / 2))
TA_GAS_HEADROOM=$((TA_COUNT * 10000))
if [ "${TA_MULTISIG}" = "true" ]; then
	# Multisig accounts perform an extra signed self-send to publish the
	# composite pubkey, then get topped back up to the target balance.
	TA_GAS_HEADROOM=$((TA_COUNT * 30000))
fi
TA_FUNDER_AMOUNT=$((TA_TOTAL + TA_GAS_HEADROOM))
TA_FUNDER_KEY="ta-funder-${MONIKER}"

log "Temp funder ${TA_FUNDER_KEY} target balance: ${TA_FUNDER_AMOUNT}${BASE_DENOM} (accounts=${TA_TOTAL}, gas=${TA_GAS_HEADROOM})"

TA_FUNDER_MNEMONIC=""
if ! "${DAEMON}" --home "${DAEMON_HOME}" keys show "${TA_FUNDER_KEY}" \
	--keyring-backend "${KEYRING_BACKEND}" >/dev/null 2>&1; then
	log "Creating temp funder key ${TA_FUNDER_KEY}..."
	# `keys add --output json` emits a JSON object on stdout (with mnemonic)
	# and a banner warning on stderr; capture stdout only to keep parsing clean.
	TA_FUNDER_ADD_JSON="$("${DAEMON}" --home "${DAEMON_HOME}" keys add "${TA_FUNDER_KEY}" \
		--keyring-backend "${KEYRING_BACKEND}" --output json)"
	TA_FUNDER_MNEMONIC="$(printf '%s' "${TA_FUNDER_ADD_JSON}" | jq -r '.mnemonic // empty' 2>/dev/null || true)"
fi
TA_FUNDER_ADDR="$("${DAEMON}" --home "${DAEMON_HOME}" keys show "${TA_FUNDER_KEY}" -a \
	--keyring-backend "${KEYRING_BACKEND}")"

# Idempotent top-up: only send the delta if current balance is short.
current_balance="$("${DAEMON}" q bank balances "${TA_FUNDER_ADDR}" --output json 2>/dev/null |
	jq -r --arg denom "${BASE_DENOM}" '([.balances[]? | select(.denom == $denom) | .amount] | first) // "0"')"
[[ -z "${current_balance}" ]] && current_balance="0"

TA_FUNDER_TXHASH=""
if ((current_balance < TA_FUNDER_AMOUNT)); then
	topup=$((TA_FUNDER_AMOUNT - current_balance))
	# Retry loop: if the tx races with supernode-setup on the validator
	# genesis sequence, CheckTx may pass but the tx gets dropped from the
	# mempool when the other side commits first. Detect that via a short
	# wait_for_tx timeout and re-sign+re-broadcast with a fresh sequence.
	# Explicit --gas skips the gRPC Simulate roundtrip (which on a freshly-
	# started node can stall for tens of seconds).
	TA_MAX_FUND_ATTEMPTS="${TA_MAX_FUND_ATTEMPTS:-5}"
	TA_WAIT_PER_ATTEMPT="${TA_WAIT_PER_ATTEMPT:-20}"
	attempt=0
	while :; do
		attempt=$((attempt + 1))
		log "[attempt ${attempt}/${TA_MAX_FUND_ATTEMPTS}] Funding ${TA_FUNDER_KEY} (${TA_FUNDER_ADDR}) with ${topup}${BASE_DENOM} from ${VAL_KEY_NAME}..."
		fund_out="$("${DAEMON}" tx bank send "${VAL_KEY_NAME}" "${TA_FUNDER_ADDR}" "${topup}${BASE_DENOM}" \
			--home "${DAEMON_HOME}" \
			--chain-id "${CHAIN_ID}" \
			--keyring-backend "${KEYRING_BACKEND}" \
			--gas 200000 \
			--gas-prices "${MIN_GAS_PRICE}" \
			--broadcast-mode sync \
			--output json --yes 2>&1)" || true
		code="$(printf '%s' "${fund_out}" | jq -r 'try .code // 1' 2>/dev/null || echo 1)"
		raw_log="$(printf '%s' "${fund_out}" | jq -r 'try .raw_log // empty' 2>/dev/null || true)"
		TA_FUNDER_TXHASH="$(printf '%s' "${fund_out}" | jq -r 'try .txhash // empty' 2>/dev/null || true)"
		if [ "${code}" = "0" ] && [ -n "${TA_FUNDER_TXHASH}" ]; then
			if TX_WAIT_LOG_PREFIX="${LOG_PREFIX}" wait_for_tx "${TA_FUNDER_TXHASH}" "${TA_WAIT_PER_ATTEMPT}" 2; then
				break
			fi
			log "[attempt ${attempt}] tx ${TA_FUNDER_TXHASH} did not confirm in ${TA_WAIT_PER_ATTEMPT}s (likely dropped from mempool); retrying."
		elif printf '%s' "${raw_log}${fund_out}" | grep -q 'account sequence mismatch'; then
			log "[attempt ${attempt}] sequence mismatch at CheckTx; retrying."
		else
			log "ERROR: temp-funder fund tx failed (code=${code}) and is not a retriable sequence issue: ${fund_out}"
			exit 1
		fi
		if ((attempt >= TA_MAX_FUND_ATTEMPTS)); then
			log "ERROR: failed to fund ${TA_FUNDER_KEY} after ${attempt} attempts."
			exit 1
		fi
		sleep $(((RANDOM % 3) + 2))
	done
else
	log "Temp funder ${TA_FUNDER_KEY} already has ${current_balance}${BASE_DENOM} (>= ${TA_FUNDER_AMOUNT})."
fi

# Persist into the shared accounts registry. Upsert preserves existing
# fields (mnemonic, first-seen txhash) when we pass empty strings, so on
# rerun we don't clobber data captured on the initial bootstrap.
accounts_registry_upsert \
	"${TA_FUNDER_KEY}" \
	"${TA_FUNDER_ADDR}" \
	"${TA_FUNDER_MNEMONIC}" \
	"cosmos" \
	"${TA_FUNDER_AMOUNT}${BASE_DENOM}" \
	"${VAL_KEY_NAME}" \
	"${TA_FUNDER_TXHASH}"
log "Registered ${TA_FUNDER_KEY} in ${NODE_STATUS_DIR}/accounts.json"

# Route all subsequent `lumera-helper.sh new-account` calls through the
# temp funder so no further tx hits the validator genesis sequence.
export FUNDER_KEY_NAME="${TA_FUNDER_KEY}"

if [ "${TA_MULTISIG}" = "true" ]; then
	log "Provisioning ${TA_COUNT} multisig test account(s): base=${TA_BASE}, incr=${TA_INCR}"
else
	log "Provisioning ${TA_COUNT} test account(s): base=${TA_BASE}, incr=${TA_INCR}"
fi

HELPER="${SCRIPT_DIR}/lumera-helper.sh"
if [ ! -x "${HELPER}" ]; then
	log "ERROR: ${HELPER} not executable."
	exit 1
fi

for i in $(seq 0 $((TA_COUNT - 1))); do
	amount_num=$((BASE_NUM + i * INCR_NUM))
	amount_arg="${amount_num}${DENOM_SFX}"

	if [ "${TA_MULTISIG}" = "true" ]; then
		log "[$((i + 1))/${TA_COUNT}] Creating 2-of-3 multisig account funded with ${amount_arg}..."
		if ! "${HELPER}" new-account --multisig "${amount_arg}"; then
			log "ERROR: failed to create/fund multisig test account #$((i + 1)) with ${amount_arg}."
			exit 1
		fi
		continue
	fi

	log "[$((i + 1))/${TA_COUNT}] Creating account funded with ${amount_arg}..."
	if ! "${HELPER}" new-account "${amount_arg}"; then
		log "ERROR: failed to create/fund test account #$((i + 1)) with ${amount_arg}."
		exit 1
	fi
done

log "Provisioned ${TA_COUNT} test account(s) for ${MONIKER}."
