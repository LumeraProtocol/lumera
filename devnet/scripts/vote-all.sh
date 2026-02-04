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
# Gas configuration
USE_GAS_AUTO="true"     # "true" to use --gas auto with --gas-adjustment 1.3
GAS_AMOUNT="120000"     # Used when USE_GAS_AUTO="false"

# Checking the votes with:
#    lumerad query gov votes <proposal_id> --output json | jq
check_votes() {
  echo "🔍 Checking current votes for proposal ID: $PROPOSAL_ID"
  VOTES_JSON=$(docker compose -f "$COMPOSE_FILE" exec "$SERVICE_NAME" \
    lumerad query gov votes "$PROPOSAL_ID" --output json)
  echo "$VOTES_JSON" | jq

  if echo "$VOTES_JSON" | jq -e '.votes' > /dev/null; then
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
