#!/bin/bash

# Usage: ./vote_all.sh <proposal_id>
if [ -z "$1" ]; then
	echo "Usage: $0 <proposal_id>"
	exit 1
fi

# Configuration
CHAIN_ID="lumera-devnet-1"
KEYRING_BACKEND="test"
PROPOSAL_ID="$1"
SERVICE_NAME="supernova_validator_1"
LUMERA_SHARED="/tmp/lumera-devnet/shared"
COMPOSE_FILE="../docker-compose.yml"
FEES="5000ulume"
# Gas configuration — multisig path requires a fixed gas amount because
# `tx multisign` can't run `--gas auto` (simulation needs a signed tx).
USE_GAS_AUTO="true" # "true" to use --gas auto with --gas-adjustment 1.3
GAS_AMOUNT="120000" # Used when USE_GAS_AUTO="false" and for multisig votes.

# is_multisig_validator reads /shared/config/validators.json inside the target
# container to decide whether the validator's --from key is a multisig
# composite. Falls back to single-sig if the config is missing/malformed.
is_multisig_validator() {
	local svc="$1"
	local enabled
	enabled=$(docker compose -f "$COMPOSE_FILE" exec -T "$svc" \
		jq -r --arg m "$svc" '.[] | select(.moniker==$m) | .multisig.enabled // false' \
		/shared/config/validators.json 2>/dev/null | tr -d '\r\n')
	[[ "$enabled" == "true" ]]
}

# cast_vote_multisig runs the offline 2-of-N flow inside the target container:
# generate unsigned tx → sign with threshold signers → multisign → broadcast.
# Echoes a broadcast-response-shaped JSON on stdout (matches the single-sig
# path so the caller can parse txhash/code uniformly).
cast_vote_multisig() {
	local svc="$1" proposal="$2"
	local key_name="${svc}_key"
	docker compose -f "$COMPOSE_FILE" exec -T "$svc" bash -s -- \
		"$key_name" "$proposal" "$CHAIN_ID" "$KEYRING_BACKEND" "$GAS_AMOUNT" "$FEES" <<'CONTAINER_SCRIPT'
set -euo pipefail
KEY_NAME="$1"
PROPOSAL="$2"
CHAIN_ID="$3"
KEYRING_BACKEND="$4"
GAS_AMOUNT="$5"
FEES="$6"

# multisig_sign_unsigned reads ${DAEMON}, ${KEYRING_BACKEND}, ${CHAIN_ID} from
# the ambient shell; vote-all.sh isn't one of the setup scripts that normally
# exports these, so set them here before sourcing common.sh.
DAEMON="lumerad"
export DAEMON KEYRING_BACKEND CHAIN_ID

source /root/scripts/common.sh

MULTISIG_ADDR="$(lumerad keys show "$KEY_NAME" -a --keyring-backend "$KEYRING_BACKEND" | tr -d '\r\n')"
ACCT_JSON="$(lumerad q auth account "$MULTISIG_ADDR" --output json 2>/dev/null)"
ACC_NUM="$(printf '%s' "$ACCT_JSON" | jq -r '.. | objects | select(has("account_number")) | .account_number' | head -n1)"
SEQ="$(printf '%s' "$ACCT_JSON" | jq -r '.. | objects | select(has("account_number")) | (.sequence // "0")' | head -n1)"
SEQ="${SEQ:-0}"

UNSIGNED="$(mktemp /tmp/vote-unsigned.XXXXXX.json)"
SIGNED="$(mktemp /tmp/vote-signed.XXXXXX.json)"
trap 'rm -f "$UNSIGNED" "$SIGNED"' EXIT

lumerad tx gov vote "$PROPOSAL" yes \
	--from "$MULTISIG_ADDR" \
	--chain-id "$CHAIN_ID" \
	--keyring-backend "$KEYRING_BACKEND" \
	--gas "$GAS_AMOUNT" \
	--fees "$FEES" \
	--account-number "$ACC_NUM" --sequence "$SEQ" \
	--generate-only --output json >"$UNSIGNED"

multisig_sign_unsigned "$UNSIGNED" "$KEY_NAME" "$MULTISIG_ADDR" \
	"${KEY_NAME}-signer-1" "${KEY_NAME}-signer-2" \
	"$ACC_NUM" "$SEQ" >"$SIGNED"

lumerad tx broadcast "$SIGNED" --broadcast-mode sync --output json
CONTAINER_SCRIPT
}

# Checking the votes with:
#    lumerad query gov votes <proposal_id> --output json | jq
check_votes() {
	echo "🔍 Checking current votes for proposal ID: $PROPOSAL_ID"
	VOTES_JSON=$(docker compose -f "$COMPOSE_FILE" exec "$SERVICE_NAME" \
		lumerad query gov votes "$PROPOSAL_ID" --output json)
	echo "$VOTES_JSON" | jq

	if echo "$VOTES_JSON" | jq -e '.votes' >/dev/null; then
		echo "ℹ️  Current Votes for Proposal $PROPOSAL_ID:"
		echo "$VOTES_JSON" | jq '.votes[] | {voter, option: .options[0].option}'
	else
		echo "ℹ️  No votes available yet."
	fi
}

# Checking participation with:
#    lumerad query gov tally <proposal_id> --output json | jq
check_tally() {
	echo "🔍 Checking current tally for proposal ID: $PROPOSAL_ID"
	TALLY_JSON=$(docker compose -f "$COMPOSE_FILE" exec "$SERVICE_NAME" \
		lumerad query gov tally "$PROPOSAL_ID" --output json)
	echo "$TALLY_JSON" | jq

	YES_COUNT=$(echo "$TALLY_JSON" | jq -r '.tally.yes_count // "0" | tonumber')
	NO_COUNT=$(echo "$TALLY_JSON" | jq -r '.tally.no_count // "0" | tonumber')
	ABSTAIN_COUNT=$(echo "$TALLY_JSON" | jq -r '.tally.abstain_count // "0" | tonumber')
	NO_WITH_VETO_COUNT=$(echo "$TALLY_JSON" | jq -r '.tally.no_with_veto_count // "0" | tonumber')

	TOTAL_VOTES=$((YES_COUNT + NO_COUNT + ABSTAIN_COUNT + NO_WITH_VETO_COUNT))
	echo "📈 Total Votes Cast: $TOTAL_VOTES"
}

vote_all() {
	echo "🔍 Discovering validator services..."

	# Get all docker compose services and filter out the primary validator (_1)
	VALIDATOR_SERVICES=$(docker compose -f "$COMPOSE_FILE" config --services | grep supernova_validator_ | grep -v '_1$')

	TX_HASHES=()

	for SERVICE in $VALIDATOR_SERVICES; do
		echo ""
		echo "🔍 Processing $SERVICE..."

		KEY_NAME="${SERVICE}_key"
		VOTER_ADDRESS=$(docker compose -f "$COMPOSE_FILE" exec "$SERVICE" \
			lumerad keys show $KEY_NAME -a --keyring-backend "$KEYRING_BACKEND" 2>/dev/null)

		echo "🗳️  Voting YES on behalf of $SERVICE (address: $VOTER_ADDRESS)..."

		if is_multisig_validator "$SERVICE"; then
			echo "  ($SERVICE is multisig; using offline 2-of-N signing flow)"
			VOTE_JSON=$(cast_vote_multisig "$SERVICE" "$PROPOSAL_ID")
		else
			if [ "$USE_GAS_AUTO" = "true" ]; then
				GAS_FLAGS=(--gas auto --gas-adjustment 1.3)
			else
				GAS_FLAGS=(--gas "$GAS_AMOUNT")
			fi

			VOTE_JSON=$(docker compose -f "$COMPOSE_FILE" exec "$SERVICE" \
				lumerad tx gov vote "$PROPOSAL_ID" yes \
				--from $VOTER_ADDRESS \
				--chain-id "$CHAIN_ID" \
				--keyring-backend "$KEYRING_BACKEND" \
				"${GAS_FLAGS[@]}" \
				--fees "$FEES" \
				--output json \
				--broadcast-mode sync \
				--yes)
		fi

		if [ -z "$VOTE_JSON" ]; then
			echo "❌ No JSON response received. The transaction command may have failed to execute."
		else
			echo "Vote transaction for $SERVICE:"
			echo "$VOTE_JSON" | jq
		fi

		TX_CODE_RAW=$(echo "$VOTE_JSON" | jq -r '.code // empty')
		TX_HASH=$(echo "$VOTE_JSON" | jq -r '.txhash // ""')

		if [[ -z "$TX_CODE_RAW" ]]; then
			TX_CODE=0
		else
			TX_CODE=$TX_CODE_RAW
		fi

		if ! [[ "$TX_CODE" =~ ^[0-9]+$ ]]; then
			echo "⚠️  TX_CODE is not a valid number: $TX_CODE"
			TX_CODE=1
		fi

		if [ "$TX_CODE" -ne 0 ] || [ -z "$TX_HASH" ]; then
			RAW_LOG=$(echo "$VOTE_JSON" | jq -r '.raw_log // "unknown error"')
			if [ -z "$TX_HASH" ]; then
				echo "❌ Vote failed: $RAW_LOG"
			else
				echo "❌ Vote failed (txhash: $TX_HASH): $RAW_LOG"
			fi
		else
			TX_HASHES+=("$TX_HASH")
		fi
	done

	# Wait before checking transaction results
	echo "⏳ Waiting for transactions to be processed..."
	sleep 5

	echo "🔍 Verifying vote transactions..."
	for TX_HASH in "${TX_HASHES[@]}"; do
		RESULT=$(docker compose -f "$COMPOSE_FILE" exec "$SERVICE_NAME" \
			lumerad query tx "$TX_HASH" --output json 2>/dev/null)

		TX_CODE=$(echo "$RESULT" | jq -r '.code // 0')
		RAW_LOG=$(echo "$RESULT" | jq -r '.raw_log // ""')

		if [[ "$TX_CODE" == "0" ]]; then
			echo "✅ Transaction $TX_HASH succeeded"
		else
			echo "❌ Transaction $TX_HASH failed with code $TX_CODE: $RAW_LOG"
		fi
	done
}

check_votes
check_tally
vote_all
check_votes
check_tally
