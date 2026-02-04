#!/bin/bash

set -euo pipefail

##################################################################################################
# Usage: ./submit-param-proposal.sh <subspace> <key> <value> [title] [description]
#
# Example:
#   ./submit-param-proposal.sh action max_actions_per_block 25 \
#     "Raise action throughput" \
#     "Increase action module max_actions_per_block to 25"
#
# The script will:
#   - Generate a proposal JSON under /tmp/<chain-id>/shared
#   - Submit a MsgUpdateParams governance proposal via supernova_validator_1
#   - Print the resulting proposal ID (if submission succeeds)
##################################################################################################

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

CHAIN_ID="lumera-devnet-1"
SERVICE="supernova_validator_1"
COMPOSE_FILE="$SCRIPT_DIR/../docker-compose.yml"
KEYRING="test"
DENOM="ulume"
LUMERA_SHARED="/tmp/${CHAIN_ID}/shared"
FEES="5000${DENOM}"

if [ $# -lt 3 ]; then
	cat <<EOF
Usage: $0 <subspace> <key> <value> [title] [description]

Example:
  $0 action max_actions_per_block 25
EOF
	exit 1
fi

SUBSPACE="$1"
KEY="$2"
VALUE="$3"
TITLE="${4:-"Update ${SUBSPACE} parameter ${KEY}"}"
DESCRIPTION="${5:-"Update ${SUBSPACE}.${KEY} to ${VALUE}"}"
SUMMARY="$TITLE"

# Ensure shared directory exists
mkdir -p "$LUMERA_SHARED"

# Helper to sanitize filenames
sanitize() {
	local s
	s="$(printf '%s' "$1" | tr '[:upper:]' '[:lower:]' | tr -c '[:alnum:]' '_')"
	# collapse duplicate underscores and trim leading/trailing ones
	s="$(echo "$s" | sed -e 's/_\{2,\}/_/g' -e 's/^_//;s/_$//')"
	echo "$s"
}

SAFE_SUBSPACE="$(sanitize "$SUBSPACE")"
SAFE_KEY="$(sanitize "$KEY")"
HOST_PROPOSAL_FILE="${LUMERA_SHARED}/param_${SAFE_SUBSPACE}_${SAFE_KEY}.json"
CONTAINER_PROPOSAL_FILE="/shared/param_${SAFE_SUBSPACE}_${SAFE_KEY}.json"

get_min_deposit_from_gov_params() {
	docker compose -f "$COMPOSE_FILE" exec "$SERVICE" \
		lumerad query gov params --output json 2>/dev/null |
		jq -r '.params.min_deposit[0].amount + .params.min_deposit[0].denom'
}

get_lumerad_version() {
	docker compose -f "$COMPOSE_FILE" exec "$SERVICE" \
		lumerad version 2>/dev/null | tr -d '\r'
}

fetch_current_params() {
	case "$SUBSPACE" in
	action)
		docker compose -f "$COMPOSE_FILE" exec "$SERVICE" \
			lumerad query action params --output json 2>/dev/null
		;;
	*)
		echo "❌ Unsupported subspace '$SUBSPACE' for automatic parameter retrieval" >&2
		exit 1
		;;
	esac
}

update_params_value() {
	local params_json="$1"
	local updated

	if jq -e -n --argjson tmp "$VALUE" 'true' >/dev/null 2>&1; then
		updated=$(echo "$params_json" | jq --arg key "$KEY" --argjson val "$VALUE" '.params[$key] = $val')
	else
		updated=$(echo "$params_json" | jq --arg key "$KEY" --arg val "$VALUE" '.params[$key] = $val')
	fi

	if [ -z "$updated" ] || [ "$updated" = "null" ]; then
		echo "❌ Failed to update parameter $KEY" >&2
		exit 1
	fi

	echo "$updated"
}

derive_msg_type() {
	local version major minor
	version="$(get_lumerad_version)"
	IFS='.' read -r major minor _ <<<"$version"
	if [[ -n "$major" && -n "$minor" && "$major" =~ ^[0-9]+$ && "$minor" =~ ^[0-9]+$ ]]; then
		if ((major > 1)) || ((major == 1 && minor >= 8)); then
			echo "/lumera.action.v1.MsgUpdateParams"
			return
		fi
	fi
	echo "/lumera.action.MsgUpdateParams"
}

# Read the governance account address (set during devnet init)
GOV_KEY_FILE="${LUMERA_SHARED}/governance_address"
if [ ! -f "$GOV_KEY_FILE" ]; then
	echo "❌ Governance key address file not found at $GOV_KEY_FILE"
	exit 1
fi
PROPOSER_ADDRESS="$(cat "$GOV_KEY_FILE")"
GOV_KEY_NAME="governance_key"

get_gov_module_address() {
	docker compose -f "$COMPOSE_FILE" exec "$SERVICE" \
		lumerad query auth module-accounts --output json 2>/dev/null |
		jq -r '.accounts[] | select(.value.name=="gov") | .value.address'
}

AUTHORITY_ADDRESS="$(get_gov_module_address)"
if [[ -z "$AUTHORITY_ADDRESS" || "$AUTHORITY_ADDRESS" == "null" ]]; then
	echo "❌ Unable to determine gov module account address"
	exit 1
fi

echo "Proposer address: $PROPOSER_ADDRESS"
echo "Gov module authority: $AUTHORITY_ADDRESS"

MIN_DEPOSIT="$(get_min_deposit_from_gov_params)"
if [[ -z "$MIN_DEPOSIT" ]]; then
	echo "❌ Failed to determine min deposit from gov params"
	exit 1
fi

CURRENT_PARAMS_JSON="$(fetch_current_params)"

UPDATED_PARAMS_JSON="$(update_params_value "$CURRENT_PARAMS_JSON")"

MSG_TYPE="$(derive_msg_type)"

echo "$UPDATED_PARAMS_JSON" | jq --arg title "$TITLE" \
	--arg description "$DESCRIPTION" \
	--arg summary "$SUMMARY" \
	--arg deposit "$MIN_DEPOSIT" \
	--arg type "$MSG_TYPE" \
	--arg authority "$AUTHORITY_ADDRESS" \
	'{
    title: $title,
    description: $description,
    summary: $summary,
    deposit: $deposit,
    messages: [
      {
        "@type": $type,
        authority: $authority,
        params: .params
      }
    ]
  }' >"$HOST_PROPOSAL_FILE"

echo "🔧 Generated proposal JSON at $HOST_PROPOSAL_FILE"
cat "$HOST_PROPOSAL_FILE" | jq || cat "$HOST_PROPOSAL_FILE"

SUBMIT_OUTPUT="$(docker compose -f "$COMPOSE_FILE" exec "$SERVICE" \
	lumerad tx gov submit-proposal "$CONTAINER_PROPOSAL_FILE" \
	--from "$GOV_KEY_NAME" \
	--chain-id "$CHAIN_ID" \
	--keyring-backend "$KEYRING" \
	--fees "$FEES" \
	--broadcast-mode sync \
	--output json \
	--yes 2>/dev/null)"

echo "📝 Transaction response:"
echo "$SUBMIT_OUTPUT" | jq || echo "$SUBMIT_OUTPUT"

TXHASH="$(echo "$SUBMIT_OUTPUT" | jq -r '.txhash // empty')"
if [[ -z "$TXHASH" ]]; then
	echo "❌ No transaction hash returned. Submission may have failed."
	exit 1
fi

echo "⏳ Waiting for transaction to be included..."
WAIT_TX_OUTPUT="$(docker compose -f "$COMPOSE_FILE" exec "$SERVICE" \
	lumerad query wait-tx "$TXHASH" --output json --timeout 60s 2>/dev/null)"

echo "$WAIT_TX_OUTPUT" | jq || echo "$WAIT_TX_OUTPUT"

CODE="$(echo "$WAIT_TX_OUTPUT" | jq -r '.code // 0')"
RAW_LOG="$(echo "$WAIT_TX_OUTPUT" | jq -r '.raw_log // empty')"
if [[ "$CODE" != "0" ]]; then
	echo "🚫 Transaction failed (code: $CODE)"
	echo "$RAW_LOG"
	exit 1
fi

PROPOSAL_ID="$(echo "$WAIT_TX_OUTPUT" | jq -r '
  (.events[]? | select(.type == "submit_proposal").attributes[]? | select(.key == "proposal_id").value) // empty
')"

if [[ -z "$PROPOSAL_ID" ]]; then
	echo "⚠️  Unable to extract proposal ID from wait-tx output."
	exit 1
fi

STATUS="$(docker compose -f "$COMPOSE_FILE" exec "$SERVICE" \
	lumerad query gov proposal "$PROPOSAL_ID" --output json 2>/dev/null |
	jq -r '.proposal.status // empty')"

echo "✅ Params proposal submitted with ID: $PROPOSAL_ID (status: $STATUS)"
exit 0
