#!/bin/bash

##################################################################################################
# Usage: ./submit_upgrade_proposal.sh <version> <upgrade_height>
#
# Expected upgrade sequence:
# - Submit proposal → Goes into DEPOSIT_PERIOD
# - Fund deposit → Moves to VOTING_PERIOD
# - Voting happens → Waits until voting_end_time
# - Proposal is tallied → If passes, status becomes PROPOSAL_STATUS_PASSED
# - At block height = upgrade.height, validators automatically stop with panic:

# Configuration
VERSION="$1"
UPGRADE_HEIGHT="$2"
CHAIN_ID="lumera-devnet-1"
SERVICE="supernova_validator_1" # primary validator
LUMERA_SHARED="/tmp/${CHAIN_ID}/shared"
KEYRING="test"
DENOM="ulume"
DEFAULT_DEPOSIT="10000000$DENOM"
HOST_PROPOSAL_FILE="${LUMERA_SHARED}/upgrade_${VERSION}.json"
CONTAINER_PROPOSAL_FILE="/shared/upgrade_${VERSION}.json"
COMPOSE_FILE="../docker-compose.yml"

get_current_height() {
	docker compose -f "$COMPOSE_FILE" exec "$SERVICE" \
		lumerad status 2>/dev/null | jq -r '.sync_info.latest_block_height // empty'
}

# Usage:
#   get_proposal_id_by_version_with_retry "v1.8.0"         # default 1 attempt
#   get_proposal_id_by_version_with_retry "v1.8.0" 5       # 5 retry attempts
get_proposal_id_by_version_with_retry() {
	local version="$1"
	local attempts="${2:-1}" # Default to 1 attempt if not provided
	local height_filter="$3"
	local sleep_interval=2
	local proposal_id=""
	local proposals_json

	for ((i = 1; i <= attempts; i++)); do
		echo "🔄 Checking for proposal with version: $version (attempt $i of $attempts)..." >&2

		proposals_json=$(docker compose -f "$COMPOSE_FILE" exec "$SERVICE" \
			lumerad query gov proposals --output json 2>/dev/null)

		echo "Proposals JSON:" >&2
		echo "$proposals_json" | jq >&2

		if echo "$proposals_json" | jq -e '.proposals | type == "array"' >/dev/null; then
			proposal_id=$(echo "$proposals_json" | jq -r --arg version "$version" --arg height "$height_filter" '
      .proposals
        | map(select(
            [
              .messages[]?
              | select(
                  ((.type // .["@type"]) == "/cosmos.upgrade.v1beta1.MsgSoftwareUpgrade")
                  and (.value.plan.name == $version)
                  and (
                    ($height == "")
                    or ((.value.plan.height // "0") | tonumber) >= ($height | tonumber)
                  )
                )
            ] | length > 0
          ))
        | sort_by(.id | tonumber)
        | last
        | .id // empty
      ')
		fi

		if [[ -n "$proposal_id" ]]; then
			echo "✅ Found proposal ID for version $version: $proposal_id" >&2
			echo "$proposal_id"
			return 0
		fi

		[[ $i -lt $attempts ]] && sleep "$sleep_interval"
	done

	if [[ -z "$proposals_json" ]] || [[ "$proposals_json" != *"proposals"* ]]; then
		return 1
	fi

	echo "❌ Could not find proposal for version $version after $attempts attempt(s)." >&2
	return 1
}

get_proposal_status_by_id() {
	local proposal_id="$1"
	docker compose -f "$COMPOSE_FILE" exec "$SERVICE" \
		lumerad query gov proposal "$proposal_id" --output json 2>/dev/null |
		jq -r '.proposal.status'
}

get_proposal_deposit_by_id() {
	local proposal_id="$1"
	docker compose -f "$COMPOSE_FILE" exec "$SERVICE" \
		lumerad query gov proposal "$proposal_id" --output json 2>/dev/null |
		jq -r '.proposal.total_deposit[0].amount + .proposal.total_deposit[0].denom'
}

# Returns the planned upgrade height for the given proposal ID (or empty)
get_proposal_height_by_id() {
	local proposal_id="$1"
	docker compose -f "$COMPOSE_FILE" exec "$SERVICE" \
		lumerad query gov proposal "$proposal_id" --output json 2>/dev/null |
		jq -r '.proposal.messages[]?.value.plan.height // empty'
}

validate_height_param() {
	CURRENT_HEIGHT=$(get_current_height)
	if ! [[ "$CURRENT_HEIGHT" =~ ^[0-9]+$ ]]; then
		echo "⚠️  Could not retrieve current block height. Is the chain running?"
		exit 1
	fi
	echo "Lumera current height: $CURRENT_HEIGHT"

	# Sanity check: warn if upgrade height is in the past
	if [ "$UPGRADE_HEIGHT" -lt "$CURRENT_HEIGHT" ]; then
		echo "❗ UPGRADE_HEIGHT ($UPGRADE_HEIGHT) is less than CURRENT_HEIGHT ($CURRENT_HEIGHT)."
		echo "⚠️  The upgrade proposal will be ineffective and ignored by the chain!"
		echo "💡 Consider choosing a future block height."
		exit 1
	fi
}

get_min_deposit_from_gov_params() {
	local output
	output=$(docker compose -f "$COMPOSE_FILE" exec "$SERVICE" \
		lumerad query gov params --output json 2>/dev/null |
		jq -r '.params.min_deposit[0].amount + .params.min_deposit[0].denom')
	echo "$output"
}

submit_proposal() {
	AUTHORITY_ADDRESS=$(docker compose -f "$COMPOSE_FILE" exec "$SERVICE" \
		lumerad query auth module-accounts --output json |
		jq -r '.accounts[] | select(.value.name=="gov") | .value.address')

	MIN_DEPOSIT=$(get_min_deposit_from_gov_params)

	echo "Generating proposal file at $HOST_PROPOSAL_FILE..." >&2
	cat >"$HOST_PROPOSAL_FILE" <<EOF
{
  "title": "Upgrade to ${VERSION}",
  "description": "Upgrading the chain to version ${VERSION} at height ${UPGRADE_HEIGHT}.",
  "summary": "Upgrade to ${VERSION}",
  "deposit": "${MIN_DEPOSIT}",
  "messages": [
    {
      "@type": "/cosmos.upgrade.v1beta1.MsgSoftwareUpgrade",
      "authority": "${AUTHORITY_ADDRESS}",
      "plan": {
        "name": "${VERSION}",
        "height": "${UPGRADE_HEIGHT}",
        "info": "",
        "upgraded_client_state": null
      }
    }
  ]
}
EOF

	echo "🔍 Proposal JSON content at $HOST_PROPOSAL_FILE:" >&2
	cat "$HOST_PROPOSAL_FILE" | jq || cat "$HOST_PROPOSAL_FILE"

	echo "Submitting software upgrade proposal for version $VERSION at height ${UPGRADE_HEIGHT}..." >&2

	# Run inside supernova_validator_1 container
	PROPOSAL_OUTPUT=$(docker compose -f "$COMPOSE_FILE" exec "$SERVICE" \
		lumerad tx gov submit-proposal "$CONTAINER_PROPOSAL_FILE" \
		--from "$GOV_ADDRESS" \
		--chain-id "$CHAIN_ID" \
		--keyring-backend "$KEYRING" \
		--fees 5000ulume \
		--broadcast-mode sync \
		--output json \
		--yes 2>/dev/null)

	echo "🔍 Proposal submission output:" >&2
	echo "$PROPOSAL_OUTPUT" | jq . >&2

	TXHASH=$(echo "$PROPOSAL_OUTPUT" | jq -r '.txhash // empty')
	LATEST_PROPOSAL_ID=$(get_proposal_id_by_version_with_retry "$VERSION" 5 "$UPGRADE_HEIGHT")

	if [[ -n "$LATEST_PROPOSAL_ID" ]]; then
		echo "✅ Proposal submitted successfully with ID: $LATEST_PROPOSAL_ID" >&2
	else
		echo "❌ Could not retrieve proposal ID. Attempting to fetch rejection reason from tx..." >&2
		if [[ -n "$TXHASH" ]]; then
			RAW_LOG=$(docker compose -f "$COMPOSE_FILE" exec "$SERVICE" \
				lumerad query tx "$TXHASH" --output json 2>/dev/null | jq -r '.raw_log // empty')
			echo "🚫 Rejection reason from tx $TXHASH:" >&2
			echo "$RAW_LOG" >&2
		else
			echo "🚫 Proposal submission failed. No tx hash captured." >&2
		fi
		exit 1
	fi
}

submit_proposal_deposit() {
	echo "✅ Proposal is in deposit period. Funding it." >&2

	# Get how much has already been deposited
	CURRENT_DEPOSIT=$(get_proposal_deposit_by_id "$EXISTING_PROPOSAL_ID")
	MIN_DEPOSIT=$(get_min_deposit_from_gov_params)
	echo "Current deposit for proposal $EXISTING_PROPOSAL_ID: $CURRENT_DEPOSIT" >&2

	CURRENT_AMOUNT=${CURRENT_DEPOSIT%ulume}
	REQUIRED_AMOUNT=${MIN_DEPOSIT%ulume}
	TO_DEPOSIT=$((REQUIRED_AMOUNT - CURRENT_AMOUNT))

	if ((TO_DEPOSIT <= 0)); then
		echo "✅ Proposal already fully funded. No additional deposit required." >&2
		return
	fi

	DEPOSIT_AMOUNT="${TO_DEPOSIT}ulume"
	echo "➡️ Submitting deposit of $DEPOSIT_AMOUNT" >&2

	DEPOSIT_OUTPUT=$(docker compose -f "$COMPOSE_FILE" exec "$SERVICE" \
		lumerad tx gov deposit "$EXISTING_PROPOSAL_ID" "$DEPOSIT_AMOUNT" \
		--from "$GOV_ADDRESS" \
		--chain-id "$CHAIN_ID" \
		--keyring-backend "$KEYRING" \
		--fees 5000ulume \
		--broadcast-mode sync \
		--output json \
		--yes 2>/dev/null)
	echo "🔍 Proposal deposit JSON content:" >&2
	echo "$DEPOSIT_OUTPUT" | jq

	DEPOSIT_TXHASH=$(echo "$DEPOSIT_OUTPUT" | jq -r '.txhash // empty')
	DEPOSIT_CODE=$(echo "$DEPOSIT_OUTPUT" | jq -r '.code // 0')
	DEPOSIT_RAW_LOG=$(echo "$DEPOSIT_OUTPUT" | jq -r '.raw_log // empty')

	if [[ "$DEPOSIT_CODE" != "0" || -n "$DEPOSIT_RAW_LOG" ]]; then
		echo "🚫 Deposit failed (txhash: $DEPOSIT_TXHASH)" >&2
		echo "🔍 Rejection reason:" >&2
		echo "$DEPOSIT_RAW_LOG" >&2
		exit 1
	fi

	echo "✅ Deposit transaction succeeded (txhash: $DEPOSIT_TXHASH)" >&2
}

show_proposal_status() {
	local version="$1"
	local proposal_id="$2"

	if [[ -z "$proposal_id" ]]; then
		echo "ℹ️  No upgrade proposal found for version $version."
		return 1
	fi

	local status=$(get_proposal_status_by_id "$proposal_id")
	echo "❗ Proposal for version $version already exists with ID: $proposal_id and status: $status"

	case "$status" in
	PROPOSAL_STATUS_REJECTED)
		echo "✅ Previous proposal was rejected."
		;;
	PROPOSAL_STATUS_REJECTED_WITH_VETO)
		echo "❗ Previous proposal was rejected with veto."
		return 1
		;;
	PROPOSAL_STATUS_FAILED)
		echo "❗ Previous proposal failed."
		return 0
		;;
	PROPOSAL_STATUS_PASSED)
		echo "✅ Proposal already passed. No need to resubmit."
		exit 0
		;;
	PROPOSAL_STATUS_DEPOSIT_PERIOD)
		echo "📥 Proposal is in deposit period. Deposit may be required."
		exit 0
		;;
	PROPOSAL_STATUS_VOTING_PERIOD)
		echo "⏳ Proposal is still active. Please wait or vote on it."
		exit 0
		;;
	*)
		echo "⚠️  Proposal in unknown state: $status"
		exit 1
		;;
	esac
}

CURRENT_HEIGHT=$(get_current_height)
if [ $# -eq 1 ]; then
	EXISTING_PROPOSAL_ID=$(get_proposal_id_by_version_with_retry "$VERSION")
	if [[ -n "$EXISTING_PROPOSAL_ID" ]]; then
		show_proposal_status "$VERSION" "$EXISTING_PROPOSAL_ID"
	fi
fi

if [ $# -ne 2 ]; then
	if [[ "$CURRENT_HEIGHT" =~ ^[0-9]+$ ]]; then
		echo "💡 Lumera chain is running, current height: $CURRENT_HEIGHT"
	fi

	echo "Usage: $0 <version> <upgrade_height>"
	exit 1
fi

# Read the governance address
GOV_ADDRESS_FILE="${LUMERA_SHARED}/governance_address"
if [ ! -f "$GOV_ADDRESS_FILE" ]; then
	echo "❌ Governance address file not found at $GOV_ADDRESS_FILE"
	exit 1
fi
GOV_ADDRESS=$(cat "$GOV_ADDRESS_FILE")
GOV_KEY_NAME="governance_key"

echo "Governance address: ${GOV_ADDRESS}"

validate_height_param

EXISTING_PROPOSAL_ID=$(get_proposal_id_by_version_with_retry "$VERSION")
if [[ -n "$EXISTING_PROPOSAL_ID" ]]; then
	show_proposal_status "$VERSION" "$EXISTING_PROPOSAL_ID"

	STATUS=$(get_proposal_status_by_id "$EXISTING_PROPOSAL_ID")
	case "$STATUS" in
	PROPOSAL_STATUS_DEPOSIT_PERIOD)
		submit_proposal_deposit
		exit 0
		;;
	PROPOSAL_STATUS_FAILED)
		# Only resubmit if the new requested height is greater than the failed one
		FAILED_HEIGHT=$(get_proposal_height_by_id "$EXISTING_PROPOSAL_ID")
		CURRENT_HEIGHT=$(get_current_height)
		if [[ -z "$FAILED_HEIGHT" ]]; then
			echo "⚠️  Could not read failed proposal height; refusing to auto-resubmit."
			exit 1
		fi
		echo "ℹ️  Failed proposal height: $FAILED_HEIGHT; requested new height: $UPGRADE_HEIGHT"
		# check that UPGRADE_HEIGHT is greater than CURRENT_HEIGHT
		if ((UPGRADE_HEIGHT <= CURRENT_HEIGHT)); then
			echo "🚫 New height ($UPGRADE_HEIGHT) must be greater than current height ($CURRENT_HEIGHT)."
			exit 1
		fi
		if ((UPGRADE_HEIGHT > FAILED_HEIGHT)); then
			echo "✅ New height is greater than failed one — submitting a new proposal…"
			submit_proposal
			exit 0
		else
			echo "🚫 New height ($UPGRADE_HEIGHT) must be greater than failed height ($FAILED_HEIGHT)."
			exit 1
		fi
		;;
	PROPOSAL_STATUS_REJECTED)
		# Rejected (no veto): allow re-submit at caller’s chosen height
		echo "ℹ️  Previous proposal was rejected. Submitting a new proposal…"
		submit_proposal
		exit 0
		;;
	PROPOSAL_STATUS_REJECTED_WITH_VETO)
		# Keep current safety behavior for veto
		echo "❗ Previous proposal was rejected with veto. Not resubmitting automatically."
		exit 1
		;;
	esac
else
	echo "🔄 No existing proposal found for version $VERSION. Proceeding to submit a new upgrade proposal"
	submit_proposal
fi
