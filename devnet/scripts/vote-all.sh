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
COMPOSE_FILE="../docker-compose.yml"
FEES="5000ulume"
# Gas configuration — use a fixed gas amount by default. `--gas auto` simulates
# a gov vote at ~57.9k and even with a 1.3x bump lands right at the real usage
# (~58k), so votes fail nondeterministically with "out of gas" (code 11). A fixed
# amount with headroom is deterministic for both single-sig and multisig votes
# (the multisig `tx multisign` path can't use --gas auto at all).
USE_GAS_AUTO="false" # "true" to use --gas auto with --gas-adjustment 1.3
GAS_AMOUNT="250000"  # fixed gas; headroom over a vote's real ~58k usage
PRIMARY_KEY="supernova_validator_1_key" # fee-buffer source for vesting voters
FEE_TOPUP="1000000ulume"                # sent to a voter with no spendable balance
FEE_MIN="5000"                          # minimum spendable ulume needed to pay a vote fee
PRIMARY_FUNDING_KEY="$PRIMARY_KEY"
PRIMARY_FUNDING_ADDRESS=""

account_exists() {
	local svc="$1" address="$2"
	[[ -n "$address" ]] || return 1
	docker compose -f "$COMPOSE_FILE" exec -T "$svc" \
		lumerad query auth account "$address" --output json >/dev/null 2>&1
}

key_address() {
	local svc="$1" key_name="$2"
	docker compose -f "$COMPOSE_FILE" exec -T "$svc" \
		lumerad keys show "$key_name" -a --keyring-backend "$KEYRING_BACKEND" 2>/dev/null |
		tr -d '\r\n'
}

key_name_for_address() {
	local svc="$1" address="$2"
	[[ -n "$address" ]] || return 1
	docker compose -f "$COMPOSE_FILE" exec -T "$svc" \
		lumerad keys list --keyring-backend "$KEYRING_BACKEND" --output json 2>/dev/null |
		jq -r --arg address "$address" '
			(if type == "array" then . else (.keys // []) end)
			| map(select(.address == $address))
			| first
			| .name // empty
		' 2>/dev/null | head -n 1
}

migration_new_address() {
	local svc="$1" legacy_address="$2"
	[[ -n "$legacy_address" ]] || return 1
	docker compose -f "$COMPOSE_FILE" exec -T "$svc" \
		lumerad query evmigration migration-record "$legacy_address" --output json 2>/dev/null |
		jq -r '.record.new_address // .migration_record.new_address // .new_address // empty' 2>/dev/null
}

resolve_live_key() {
	local svc="$1" key_name="$2"
	local address migrated_address migrated_key_name

	address="$(key_address "$svc" "$key_name")"
	if account_exists "$svc" "$address"; then
		printf '%s\t%s\n' "$key_name" "$address"
		return 0
	fi

	migrated_address="$(migration_new_address "$svc" "$address")"
	if [[ -z "$migrated_address" || "$migrated_address" == "null" ]]; then
		return 1
	fi

	migrated_key_name="$(key_name_for_address "$svc" "$migrated_address")"
	if [[ -z "$migrated_key_name" ]]; then
		return 1
	fi
	if ! account_exists "$svc" "$migrated_address"; then
		return 1
	fi

	printf '%s\t%s\n' "$migrated_key_name" "$migrated_address"
}

resolve_primary_funding_key() {
	local resolved
	if ! resolved="$(resolve_live_key "$SERVICE_NAME" "$PRIMARY_KEY")"; then
		echo "❌ Unable to resolve live primary funding key from ${SERVICE_NAME}/${PRIMARY_KEY}" >&2
		return 1
	fi
	IFS=$'\t' read -r PRIMARY_FUNDING_KEY PRIMARY_FUNDING_ADDRESS <<<"$resolved"
}

# ensure_fee_funds tops up a voter that cannot self-pay the tx fee. Validators
# whose operator account is a PermanentLocked (or otherwise vesting) account have
# 0 *spendable* balance, so they fail with "spendable balance 0ulume ... insufficient
# funds". Send a small spendable buffer from the primary so the vote can proceed.
ensure_fee_funds() {
	local svc="$1" addr="$2" spendable balances_json topup_json topup_rc topup_code tx_hash raw_log
	if ! balances_json=$(docker compose -f "$COMPOSE_FILE" exec -T "$svc" \
		lumerad query bank spendable-balances "$addr" --output json 2>/dev/null); then
		echo "  ❌ $svc: failed to query spendable balance for $addr" >&2
		return 1
	fi
	spendable=$(echo "$balances_json" | jq -r '[.balances[]?|select(.denom=="ulume")|.amount][0] // "0"' | tr -d '\r\n')
	spendable=${spendable:-0}
	if [[ "$spendable" =~ ^[0-9]+$ ]] && [ "$spendable" -ge "$FEE_MIN" ]; then
		return 0
	fi
	echo "  💧 $svc voter has ${spendable}ulume spendable (< ${FEE_MIN}); topping up ${FEE_TOPUP} from ${SERVICE_NAME}/${PRIMARY_FUNDING_KEY}"
	topup_json=$(docker compose -f "$COMPOSE_FILE" exec -T "$SERVICE_NAME" \
		lumerad tx bank send "$PRIMARY_FUNDING_KEY" "$addr" "$FEE_TOPUP" \
		--keyring-backend "$KEYRING_BACKEND" --chain-id "$CHAIN_ID" \
		--gas "$GAS_AMOUNT" --gas-prices 0.025ulume -y -o json 2>&1)
	topup_rc=$?
	if [ "$topup_rc" -ne 0 ]; then
		echo "  ❌ $svc: fee top-up command failed with exit code $topup_rc" >&2
		echo "$topup_json" >&2
		return 1
	fi

	if ! topup_code=$(echo "$topup_json" | jq -r '.code // 0' 2>/dev/null); then
		echo "  ❌ $svc: fee top-up did not return valid JSON" >&2
		echo "$topup_json" >&2
		return 1
	fi
	if ! [[ "$topup_code" =~ ^[0-9]+$ ]]; then
		echo "  ❌ $svc: fee top-up returned non-numeric code: $topup_code" >&2
		echo "$topup_json" | jq . >&2
		return 1
	fi

	tx_hash=$(echo "$topup_json" | jq -r '.txhash // ""')
	if [ "$topup_code" -ne 0 ] || [ -z "$tx_hash" ]; then
		raw_log=$(echo "$topup_json" | jq -r '.raw_log // "unknown error"')
		if [ -z "$tx_hash" ]; then
			echo "  ❌ $svc: fee top-up failed: $raw_log" >&2
		else
			echo "  ❌ $svc: fee top-up failed (txhash: $tx_hash): $raw_log" >&2
		fi
		return 1
	fi

	echo "  ✅ $svc fee top-up accepted (txhash: $tx_hash)"
	sleep 6
}

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
	local svc="$1" proposal="$2" key_name="$3" voter_address="$4"
	docker compose -f "$COMPOSE_FILE" exec -T "$svc" bash -s -- \
		"$key_name" "$voter_address" "$proposal" "$CHAIN_ID" "$KEYRING_BACKEND" "$GAS_AMOUNT" "$FEES" <<'CONTAINER_SCRIPT'
set -euo pipefail
KEY_NAME="$1"
MULTISIG_ADDR="$2"
PROPOSAL="$3"
CHAIN_ID="$4"
KEYRING_BACKEND="$5"
GAS_AMOUNT="$6"
FEES="$7"

# multisig_sign_unsigned reads ${DAEMON}, ${KEYRING_BACKEND}, ${CHAIN_ID} from
# the ambient shell; vote-all.sh isn't one of the setup scripts that normally
# exports these, so set them here before sourcing common.sh.
DAEMON="lumerad"
export DAEMON KEYRING_BACKEND CHAIN_ID

source /root/scripts/common.sh

ACCT_JSON="$(lumerad q auth account "$MULTISIG_ADDR" --output json 2>/dev/null)"
ACC_NUM="$(printf '%s' "$ACCT_JSON" | jq -r '.. | objects | select(has("account_number")) | .account_number' | head -n1)"
SEQ="$(printf '%s' "$ACCT_JSON" | jq -r '.. | objects | select(has("account_number")) | (.sequence // "0")' | head -n1)"
SEQ="${SEQ:-0}"
SIGNER_1="${KEY_NAME}-signer-1"
SIGNER_2="${KEY_NAME}-signer-2"
if [[ "$KEY_NAME" == *-new-msig ]]; then
	BASE_KEY="${KEY_NAME%-new-msig}"
	SIGNER_1="${BASE_KEY}-new-signer-1"
	SIGNER_2="${BASE_KEY}-new-signer-2"
	if ! lumerad keys show "$SIGNER_1" --keyring-backend "$KEYRING_BACKEND" >/dev/null 2>&1; then
		SIGNER_1="${BASE_KEY}-newmsig-1"
		SIGNER_2="${BASE_KEY}-newmsig-2"
	fi
fi

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
	"$SIGNER_1" "$SIGNER_2" \
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
	resolve_primary_funding_key || exit 1
	echo "💧 Fee top-up source: ${SERVICE_NAME}/${PRIMARY_FUNDING_KEY} (${PRIMARY_FUNDING_ADDRESS})"

	# Vote from ALL validators including the primary (_1). The proposer is NOT
	# auto-voted by x/gov, so excluding _1 silently dropped the primary's (often
	# largest) stake from the tally — a frequent cause of upgrades not reaching
	# quorum.
	VALIDATOR_SERVICES=$(docker compose -f "$COMPOSE_FILE" config --services | grep supernova_validator_)

	TX_HASHES=()

	for SERVICE in $VALIDATOR_SERVICES; do
		echo ""
		echo "🔍 Processing $SERVICE..."

		KEY_NAME="${SERVICE}_key"
		RESOLVED_VOTER="$(resolve_live_key "$SERVICE" "$KEY_NAME" || true)"
		if [[ -z "$RESOLVED_VOTER" ]]; then
			echo "❌ Unable to resolve live voter key for $SERVICE/$KEY_NAME; skipping vote"
			continue
		fi
		IFS=$'\t' read -r VOTER_KEY_NAME VOTER_ADDRESS <<<"$RESOLVED_VOTER"

		echo "🗳️  Voting YES on behalf of $SERVICE (key: $VOTER_KEY_NAME, address: $VOTER_ADDRESS)..."

		# Make sure the voter can pay the fee (vesting accounts have 0 spendable).
		if ! ensure_fee_funds "$SERVICE" "$VOTER_ADDRESS"; then
			echo "❌ Skipping vote for $SERVICE because fee top-up failed"
			continue
		fi

		if is_multisig_validator "$SERVICE"; then
			echo "  ($SERVICE is multisig; using offline 2-of-N signing flow)"
			VOTE_JSON=$(cast_vote_multisig "$SERVICE" "$PROPOSAL_ID" "$VOTER_KEY_NAME" "$VOTER_ADDRESS")
		else
			if [ "$USE_GAS_AUTO" = "true" ]; then
				GAS_FLAGS=(--gas auto --gas-adjustment 1.3)
			else
				GAS_FLAGS=(--gas "$GAS_AMOUNT")
			fi

			VOTE_JSON=$(docker compose -f "$COMPOSE_FILE" exec "$SERVICE" \
				lumerad tx gov vote "$PROPOSAL_ID" yes \
				--from "$VOTER_KEY_NAME" \
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
