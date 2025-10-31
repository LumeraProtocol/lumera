#!/bin/bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CHAIN_ID="lumera-devnet-1"
SERVICE="supernova_validator_1"
COMPOSE_FILE="$SCRIPT_DIR/../docker-compose.yml"

JSON_OUTPUT="${JSON_OUTPUT:-false}"

print_usage() {
  cat <<EOF
Usage: ${0##*/} [--json]

Options:
  --json    Print the latest proposal JSON instead of a summary line

Displays the most recently submitted governance proposal, or reports that none exist.
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --json)
      JSON_OUTPUT=true
      shift
      ;;
    -h|--help)
      print_usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      print_usage >&2
      exit 1
      ;;
  esac
done

PROPOSALS_JSON="$(docker compose -f "$COMPOSE_FILE" exec "$SERVICE" \
  lumerad query gov proposals --output json 2>/dev/null)"

COUNT="$(echo "$PROPOSALS_JSON" | jq '.proposals | length // 0')"

if [[ "$COUNT" -eq 0 ]]; then
  echo "No governance proposals found."
  exit 0
fi

LATEST="$(echo "$PROPOSALS_JSON" | jq '.proposals | sort_by(.id | tonumber) | last')"

if [[ "$JSON_OUTPUT" == "true" ]]; then
  echo "$LATEST" | jq
  exit 0
fi

ID="$(echo "$LATEST" | jq -r '.id')"
TITLE="$(echo "$LATEST" | jq -r '.title // .summary // ""')"
STATUS="$(echo "$LATEST" | jq -r '.status')"
PROPOSER="$(echo "$LATEST" | jq -r '.proposer // ""')"

printf "Latest proposal: ID=%s, Status=%s" "$ID" "$STATUS"
if [[ -n "$TITLE" && "$TITLE" != "" ]]; then
  printf ", Title=%s" "$TITLE"
fi
if [[ -n "$PROPOSER" && "$PROPOSER" != "" ]]; then
  printf ", Proposer=%s" "$PROPOSER"
fi
printf '\n'
