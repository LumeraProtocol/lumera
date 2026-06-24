#!/usr/bin/env bash
#
# submit-params-proposal.sh — submit a MsgUpdateParams governance proposal that
# changes a single module parameter. Companion to submit-upgrade-proposal.sh.
#
# MsgUpdateParams REPLACES the module's entire Params message, so this script
# reads the current params, patches just <key>=<value>, and submits the full set.
# It funds the min deposit so the proposal goes straight to the voting period.
# Vote on it with vote-all.sh.
#
# Usage:
#   ./submit-params-proposal.sh <module> <key> <value> [title] [summary]
#   e.g. ./submit-params-proposal.sh evmigration max_validator_delegations 8000
#
# Env overrides: CHAIN_ID, SERVICE, KEYRING, GOV_KEY, GAS, FEES, PARAMS_MSG_TYPE
#
set -euo pipefail

CHAIN_ID="${CHAIN_ID:-lumera-devnet-1}"
SERVICE="${SERVICE:-lumera-supernova_validator_1}"
KEYRING="${KEYRING:-test}"
HOME_DIR="${HOME_DIR:-/root/.lumera}"
GOV_KEY="${GOV_KEY:-governance_key}"
GAS="${GAS:-400000}"        # submit-proposal needs more than the 200k default
FEES="${FEES:-5000ulume}"

if [ $# -lt 3 ]; then
	echo "Usage: $0 <module> <key> <value> [title] [summary]" >&2
	echo "  e.g. $0 evmigration max_validator_delegations 8000" >&2
	exit 1
fi
MODULE="$1"; KEY="$2"; VALUE="$3"
TITLE="${4:-Update ${MODULE} param ${KEY}}"
SUMMARY="${5:-Set ${MODULE}.${KEY} = ${VALUE}}"

dx() { docker exec -i "$SERVICE" "$@"; }
lq() { dx lumerad query "$@" --home "$HOME_DIR" --output json 2>/dev/null; }

# MsgUpdateParams type URL per module (override with PARAMS_MSG_TYPE for others).
msg_type() {
	if [[ -n "${PARAMS_MSG_TYPE:-}" ]]; then printf '%s' "$PARAMS_MSG_TYPE"; return 0; fi
	case "$MODULE" in
		evmigration) printf '/lumera.evmigration.MsgUpdateParams' ;;
		action)      printf '/lumera.action.v1.MsgUpdateParams' ;;
		supernode)   printf '/lumera.supernode.v1.MsgUpdateParams' ;;
		*) echo "❌ Unknown module '$MODULE'; set PARAMS_MSG_TYPE=/your.module.MsgUpdateParams" >&2; return 1 ;;
	esac
}
TYPE="$(msg_type)" || exit 1

echo "==> Reading current ${MODULE} params..." >&2
PARAMS="$(lq "$MODULE" params | jq -c '.params')"
if [[ -z "$PARAMS" || "$PARAMS" == "null" ]]; then
	echo "❌ Could not read ${MODULE} params (does 'lumerad query ${MODULE} params' work?)" >&2
	exit 1
fi

# Patch the key. Use a JSON value when it parses as JSON (numbers/bools), else a
# string. uint64 params render as JSON strings on-chain, so quote bare integers
# only when the existing value is a string.
if [[ "$(echo "$PARAMS" | jq -r --arg k "$KEY" '.[$k] | type')" == "string" ]]; then
	NEW_PARAMS="$(echo "$PARAMS" | jq -c --arg k "$KEY" --arg v "$VALUE" '.[$k] = $v')"
elif jq -e -n --argjson v "$VALUE" 'true' >/dev/null 2>&1; then
	NEW_PARAMS="$(echo "$PARAMS" | jq -c --arg k "$KEY" --argjson v "$VALUE" '.[$k] = $v')"
else
	NEW_PARAMS="$(echo "$PARAMS" | jq -c --arg k "$KEY" --arg v "$VALUE" '.[$k] = $v')"
fi
echo "    before: $PARAMS" >&2
echo "    after:  $NEW_PARAMS" >&2

AUTHORITY="$(lq auth module-account gov | jq -r '.account.value.address // .account.base_account.address // .account.address')"
DEPOSIT="$(lq gov params | jq -r '.params.min_deposit[0] | .amount + .denom')"
echo "==> authority=${AUTHORITY} deposit=${DEPOSIT} type=${TYPE}" >&2

PROPOSAL_FILE="/tmp/params-proposal-${MODULE}-${KEY}.json"
dx sh -c "cat > ${PROPOSAL_FILE}" <<EOF
{
  "messages": [
    { "@type": "${TYPE}", "authority": "${AUTHORITY}", "params": ${NEW_PARAMS} }
  ],
  "metadata": "",
  "deposit": "${DEPOSIT}",
  "title": "${TITLE}",
  "summary": "${SUMMARY}"
}
EOF

echo "==> Submitting proposal..." >&2
OUT="$(dx lumerad tx gov submit-proposal "${PROPOSAL_FILE}" \
	--from "${GOV_KEY}" --chain-id "${CHAIN_ID}" --keyring-backend "${KEYRING}" --home "${HOME_DIR}" \
	--gas "${GAS}" --fees "${FEES}" --broadcast-mode sync -y --output json 2>/dev/null)"
TXHASH="$(echo "$OUT" | jq -r '.txhash // empty')"
CODE="$(echo "$OUT" | jq -r '.code // 0')"
if [[ -z "$TXHASH" || "$CODE" != "0" ]]; then
	echo "❌ submit failed: $(echo "$OUT" | jq -r '.raw_log // .')" >&2
	exit 1
fi
echo "    txhash=${TXHASH}" >&2
sleep 6
RES="$(dx lumerad query tx "$TXHASH" --home "$HOME_DIR" --output json 2>/dev/null)"
RCODE="$(echo "$RES" | jq -r '.code // 1')"
if [[ "$RCODE" != "0" ]]; then
	echo "❌ proposal tx failed (code=${RCODE}): $(echo "$RES" | jq -r '.raw_log')" >&2
	exit 1
fi
PROPOSAL_ID="$(lq gov proposals | jq -r '.proposals | sort_by(.id|tonumber) | last | (.id // .proposal_id)')"
echo "✅ Submitted ${MODULE}.${KEY}=${VALUE} as proposal ${PROPOSAL_ID} (now in voting). Vote with vote-all.sh ${PROPOSAL_ID}."
echo "${PROPOSAL_ID}"
